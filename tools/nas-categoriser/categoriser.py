#!/usr/bin/env python3
"""Cleans descriptions and fills in missing categories for ezbookkeeping quick add transactions.

quick_add tags every transaction it creates with "needs-ai". This script picks those up, asks
Claude (through `claude -p`, so it runs on the Claude subscription the container is logged in
with) for a "Merchant, City" description and, for uncategorised ones, a category, then writes
the result back and removes the tag.

Transactions tagged "fx-unconverted" were saved at face value because quick_add could not convert
them: the currency symbol was ambiguous ("$", "¥", "Rs") or the ECB has no rate for it. Claude
works out the currency from where the purchase and the user's recent purchases were made, and the
amount is converted at the open.er-api.com rate.

    categoriser.py --loop                 run forever, every EZB_INTERVAL seconds
    categoriser.py --once                 process the pending queue once
    categoriser.py --backfill             print cleaned versions of old messy descriptions
    categoriser.py --backfill --apply     write them

Configuration comes from the environment: EZB_URL, EZB_TOKEN (an api type token), and
optionally EZB_INTERVAL (default 300), EZB_UTC_OFFSET (minutes, default 60) and CLAUDE_MODEL
(default haiku).
"""

import argparse
import json
import os
import re
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

PENDING_TAG = "needs-ai"
UNCONVERTED_TAG = "fx-unconverted"
EXPENSE = 3
INCOME = 2
BATCH_SIZE = 25
EXAMPLE_COUNT = 150

EZB_URL = os.environ.get("EZB_URL", "http://127.0.0.1:8080").rstrip("/")
EZB_TOKEN = os.environ.get("EZB_TOKEN", "")
UTC_OFFSET = os.environ.get("EZB_UTC_OFFSET", "60")
CLAUDE_MODEL = os.environ.get("CLAUDE_MODEL", "haiku")

CONTEXT_COUNT = 60

# the original foreign amount quick_add keeps at the end of a description, e.g. "(€ 7,21)" or "(Rs 1 250,00)"
ORIGINAL_AMOUNT = re.compile(r"\(([^\d()]+?) ?(\d[\d .,]*)\)$")

# descriptions written by quick_add before it learnt to clean them
MESSY_DESCRIPTION = re.compile(r"Bank Norwegian\n|^[^:\n]+: [A-Z]{3} [\d .,]+$")

SYSTEM_PROMPT = """You tidy up transactions in a Norwegian person's personal bookkeeping app.

For every transaction, return a clean description:
- Format it as "Merchant, City" when the input names a city, otherwise just "Merchant". Never invent a city.
- Use the merchant's usual brand name and capitalisation. Drop payment-processor prefixes and noise such as "PayPal *", "PAYPAL *", "AER*", "SumUp *", "iZ *", "Vipps", store or terminal numbers, order references, and the region after the city.
- Drop branch names that follow a chain's name, e.g. "AER* MENY Hoeyden" is "MENY", "Rema 1000 Dyrlokkeveien" is "REMA 1000". Spell chains the way the chain does, the same way every time: REMA 1000, MENY, OBS, Extra, Joker, Kiwi, Coop, Uno-X, YX.
- Keep names that are already a proper brand as they are (e.g. "NEON.TECH", "bull-ski-kajakk.no (KCO)"); do not re-case them.
- Expand an abbreviation only when you are sure, e.g. "Bk 5073 Vestby" is Burger King in Vestby. When unsure, keep the original wording rather than guess.
- Raw inputs may have the account name "Bank Norwegian" glued to the location and the amount on a second line; drop both, but keep a foreign (non-NOK) amount as a suffix in parentheses written exactly as in the input, e.g. "Uber (€ 7,21)" or "NEON.TECH (USD 3,08)". An existing "(...)" amount suffix stays unchanged. NOK amounts ("kr ...") are dropped.
- If a description is already clean, return it unchanged.

When needsCategory is true, also pick the category: exactly one name from the category list, or null when you cannot tell. The user's own past transactions are the strongest signal. When needsCategory is false, return null for category."""

CURRENCY_SYSTEM_PROMPT = """You work out which currency a card purchase was made in, for a Norwegian person who is travelling.

Each purchase has its description, which ends with the original amount as the bank notification showed it, e.g. "(Rs 1 250,00)" or "($ 12,00)". The symbol alone was not enough to tell the currency.

Decide the ISO 4217 code from, in order of strength:
1. The city or country in the description.
2. Where the user's recent purchases were made around the same date (listed with their dates, newest first). Someone who has been paying in Kathmandu for days is most likely still in Nepal.
3. The merchant: an international online service (AWS, Netflix, Apple, Steam) charging "$" is usually USD wherever the user is.

"Rs" or "रू" is NPR in Nepal and INR in India; "$" may be USD, SGD, AUD, HKD, NZD, CAD and so on; "¥" is JPY or CNY. Return null when the evidence does not point clearly to one currency."""

CURRENCY_SCHEMA = {
    "type": "object",
    "properties": {
        "items": {
            "type": "array",
            "items": {
                "type": "object",
                "properties": {
                    "id": {"type": "string"},
                    "currency": {"type": ["string", "null"]},
                },
                "required": ["id", "currency"],
                "additionalProperties": False,
            },
        }
    },
    "required": ["items"],
    "additionalProperties": False,
}

RESPONSE_SCHEMA = {
    "type": "object",
    "properties": {
        "items": {
            "type": "array",
            "items": {
                "type": "object",
                "properties": {
                    "id": {"type": "string"},
                    "description": {"type": "string"},
                    "category": {"type": ["string", "null"]},
                },
                "required": ["id", "description", "category"],
                "additionalProperties": False,
            },
        }
    },
    "required": ["items"],
    "additionalProperties": False,
}


def log(message):
    print(time.strftime("%Y-%m-%d %H:%M:%S"), message, flush=True)


def api(path, params=None, body=None):
    url = f"{EZB_URL}/api/v1/{path}"

    if params:
        url += "?" + urllib.parse.urlencode(params)

    request = urllib.request.Request(url, method="POST" if body is not None else "GET")
    request.add_header("Authorization", f"Bearer {EZB_TOKEN}")
    request.add_header("X-Timezone-Offset", UTC_OFFSET)
    data = None

    if body is not None:
        request.add_header("Content-Type", "application/json")
        data = json.dumps(body).encode()

    try:
        with urllib.request.urlopen(request, data=data, timeout=60) as response:
            payload = json.load(response)
    except urllib.error.HTTPError as error:
        payload = json.load(error)

    if not payload.get("success"):
        raise RuntimeError(f"{path}: {payload.get('errorMessage')}")

    return payload["result"]


def list_transactions(params, limit=None):
    transactions = []
    page = 1

    while True:
        items = api("transactions/list.json", {"count": 50, "page": page, "trim_account": "true", **params})["items"]
        transactions += items

        if len(items) < 50 or (limit and len(transactions) >= limit):
            return transactions[:limit] if limit else transactions

        page += 1


def load_categories():
    """Returns {transaction type: {"Parent > Child": id}} for the visible sub-categories."""
    categories = {EXPENSE: {}, INCOME: {}}
    category_type_to_transaction_type = {"2": EXPENSE, "1": INCOME}

    for category_type, primaries in api("transaction/categories/list.json").items():
        transaction_type = category_type_to_transaction_type.get(category_type)

        if transaction_type is None:
            continue

        for primary in primaries:
            for sub in primary.get("subCategories") or []:
                if not primary["hidden"] and not sub["hidden"]:
                    categories[transaction_type][f"{primary['name']} > {sub['name']}"] = sub["id"]

    return categories


def load_examples(categories):
    """Recent categorised expenses with clean descriptions, as "description => category" lines."""
    names_by_id = {category_id: name for name, category_id in categories[EXPENSE].items()}
    examples = {}

    for transaction in list_transactions({"type": EXPENSE}, limit=EXAMPLE_COUNT * 2):
        description = transaction.get("comment", "").strip()
        category = names_by_id.get(transaction.get("categoryId"))

        if description and category and not MESSY_DESCRIPTION.search(description):
            examples.setdefault(description, category)

    return [f"{description} => {category}" for description, category in list(examples.items())[:EXAMPLE_COUNT]]


def ask_claude(items, categories, examples):
    prompt = "\n\n".join([
        "Expense categories:\n" + "\n".join(categories[EXPENSE]),
        "Income categories:\n" + "\n".join(categories[INCOME]),
        "Past transactions (description => category):\n" + "\n".join(examples),
        "Transactions to process:\n" + json.dumps(items, ensure_ascii=False, indent=1),
    ])

    return run_claude(prompt, SYSTEM_PROMPT, RESPONSE_SCHEMA)


def run_claude(prompt, system_prompt, schema):
    """Runs claude -p without tools and returns its answer keyed by item id."""
    result = subprocess.run(
        ["claude", "-p", "--model", CLAUDE_MODEL, "--tools", "", "--strict-mcp-config",
         "--no-session-persistence", "--output-format", "json",
         "--system-prompt", system_prompt, "--json-schema", json.dumps(schema)],
        input=prompt, capture_output=True, text=True, timeout=600,
    )

    if result.returncode != 0:
        raise RuntimeError(f"claude exited with {result.returncode}: {result.stderr.strip()[:500]}")

    output = json.loads(result.stdout)

    if output.get("is_error") or not output.get("structured_output"):
        raise RuntimeError(f"claude returned no structured output: {output.get('result', '')[:500]}")

    return {item["id"]: item for item in output["structured_output"]["items"]}


def modify_transaction(transaction_id, comment=None, category_id=None, amount=None, remove_tag_ids=()):
    current = api("transactions/get.json", {"id": transaction_id, "trim_account": "true", "trim_category": "true", "trim_tag": "true", "with_pictures": "true"})

    body = {
        "id": current["id"],
        "type": current["type"],
        "categoryId": category_id or current["categoryId"],
        "time": current["time"],
        "utcOffset": current["utcOffset"],
        "sourceAccountId": current["sourceAccountId"],
        "destinationAccountId": current.get("destinationAccountId", "0"),
        "sourceAmount": amount if amount is not None else current["sourceAmount"],
        "destinationAmount": current.get("destinationAmount") or 0,
        "hideAmount": current.get("hideAmount", False),
        "tagIds": [tag_id for tag_id in current.get("tagIds") or [] if tag_id not in remove_tag_ids],
        "pictureIds": [picture["pictureId"] for picture in current.get("pictures") or []],
        "comment": (comment if comment is not None else current.get("comment", ""))[:255],
    }

    if current.get("geoLocation"):
        body["geoLocation"] = current["geoLocation"]

    api("transactions/modify.json", body=body)


def parse_amount(text):
    """Same rules as quick_add's parseAmountString: the later of "." and "," is the decimal
    separator, and a lone separator followed by exactly three digits is a thousands separator."""
    text = re.sub(r"[^\d.,-]", "", text)
    last_dot, last_comma = text.rfind("."), text.rfind(",")

    if last_dot >= 0 and last_comma >= 0:
        thousands = "." if last_comma > last_dot else ","
        text = text.replace(thousands, "").replace(",", ".")
    elif max(last_dot, last_comma) >= 0:
        separator = text[max(last_dot, last_comma)]

        if text.count(separator) > 1 or len(text) - max(last_dot, last_comma) - 1 == 3:
            text = text.replace(separator, "")
        else:
            text = text.replace(",", ".")

    return float(text)


def resolve_unconverted(transactions, unconverted_tag_id):
    """Works out the currency of purchases quick_add could not convert, then converts them."""
    items = []

    for transaction in transactions:
        if not ORIGINAL_AMOUNT.search(transaction.get("comment", "")):
            log(f"{transaction['id']}: no original amount in {transaction.get('comment')!r}, leaving it tagged")
            continue

        items.append({
            "id": transaction["id"],
            "date": time.strftime("%Y-%m-%d %H:%M", time.gmtime(transaction["time"] + transaction["utcOffset"] * 60)),
            "description": transaction["comment"],
        })

    if not items:
        return

    pending_ids = {item["id"] for item in items}
    recent = [
        f"{time.strftime('%Y-%m-%d', time.gmtime(t['time']))}  {t.get('comment', '')}"
        for t in list_transactions({}, limit=CONTEXT_COUNT)
        if t["id"] not in pending_ids and t.get("comment")
    ]

    prompt = "\n\n".join([
        "Recent purchases (newest first):\n" + "\n".join(recent),
        "Purchases to work out the currency for:\n" + json.dumps(items, ensure_ascii=False, indent=1),
    ])
    answers = run_claude(prompt, CURRENCY_SYSTEM_PROMPT, CURRENCY_SCHEMA)

    with urllib.request.urlopen("https://open.er-api.com/v6/latest/NOK", timeout=30) as response:
        rates = json.load(response)["rates"]

    for item in items:
        currency = (answers.get(item["id"], {}).get("currency") or "").upper()

        if currency not in rates:
            log(f"{item['id']}: could not tell the currency of {item['description']!r} (got {currency or 'nothing'}), leaving it tagged")
            continue

        match = ORIGINAL_AMOUNT.search(item["description"])
        original_text = match.group(2).strip()
        amount = round(parse_amount(original_text) / rates[currency] * 100)
        # name the currency that was decided on, so a wrong guess is easy to spot later
        description = item["description"][:match.start()] + f"({currency} {original_text})"

        modify_transaction(item["id"], comment=description, amount=amount, remove_tag_ids=(unconverted_tag_id,))
        log(f"{item['id']}: {item['description']!r} is {currency}, converted to {amount / 100:.2f} NOK")


def process_pending():
    tag_ids = {tag["name"]: tag["id"] for tag in api("transaction/tags/list.json")}
    pending_tag_id = tag_ids.get(PENDING_TAG)
    unconverted_tag_id = tag_ids.get(UNCONVERTED_TAG)

    if unconverted_tag_id:
        resolve_unconverted(list_transactions({"tag_filter": f"0:{unconverted_tag_id}"}), unconverted_tag_id)

    if not pending_tag_id:
        return

    pending = list_transactions({"tag_filter": f"0:{pending_tag_id}"})

    if not pending:
        return

    categories = load_categories()
    examples = load_examples(categories)
    category_names_by_type = {transaction_type: {name.split(" > ")[-1].lower(): category_id for name, category_id in by_name.items()} | {name.lower(): category_id for name, category_id in by_name.items()} for transaction_type, by_name in categories.items()}

    for start in range(0, len(pending), BATCH_SIZE):
        batch = pending[start:start + BATCH_SIZE]
        items = [{
            "id": transaction["id"],
            "description": transaction.get("comment", ""),
            "amountNok": transaction["sourceAmount"] / 100,
            "type": "income" if transaction["type"] == INCOME else "expense",
            "needsCategory": transaction.get("categoryId") in ("0", 0, None),
        } for transaction in batch]

        answers = ask_claude(items, categories, examples)

        for item, transaction in zip(items, batch):
            answer = answers.get(item["id"])

            if not answer:
                log(f"{item['id']}: no answer from claude, leaving it tagged")
                continue

            category_id = None

            if item["needsCategory"] and answer.get("category"):
                category_id = category_names_by_type.get(transaction["type"], {}).get(answer["category"].lower())

                if not category_id:
                    log(f"{item['id']}: claude picked unknown category {answer['category']!r}, leaving it uncategorised")

            description = answer["description"].strip() or item["description"]
            modify_transaction(item["id"], comment=description, category_id=category_id, remove_tag_ids=(pending_tag_id,))
            log(f"{item['id']}: {item['description']!r} -> {description!r}" + (f" [{answer['category']}]" if category_id else ""))


def backfill(apply):
    categories = load_categories()
    examples = load_examples(categories)
    messy = [t for t in list_transactions({"type": EXPENSE}) if MESSY_DESCRIPTION.search(t.get("comment", ""))]
    log(f"{len(messy)} messy descriptions")

    for start in range(0, len(messy), BATCH_SIZE):
        batch = messy[start:start + BATCH_SIZE]
        items = [{"id": t["id"], "description": t["comment"], "amountNok": t["sourceAmount"] / 100, "type": "expense", "needsCategory": False} for t in batch]
        answers = ask_claude(items, categories, examples)

        for item in items:
            answer = answers.get(item["id"])

            if not answer or not answer["description"].strip():
                continue

            print(f"{item['id']}\t{item['description']!r}\t{answer['description'].strip()!r}", flush=True)

            if apply:
                modify_transaction(item["id"], comment=answer["description"].strip())


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    mode = parser.add_mutually_exclusive_group(required=True)
    mode.add_argument("--loop", action="store_true")
    mode.add_argument("--once", action="store_true")
    mode.add_argument("--backfill", action="store_true")
    parser.add_argument("--apply", action="store_true", help="with --backfill, write the cleaned descriptions")
    args = parser.parse_args()

    if not EZB_TOKEN:
        sys.exit("EZB_TOKEN is not set")

    if args.backfill:
        backfill(args.apply)
        return

    interval = int(os.environ.get("EZB_INTERVAL", "300"))

    while True:
        try:
            process_pending()
        except Exception as error:  # keep the loop alive through ezbookkeeping or claude hiccups
            log(f"cycle failed: {error}")

        if args.once:
            return

        time.sleep(interval)


if __name__ == "__main__":
    main()
