package api

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/models"
)

func TestParseQuickAddInput_WalletNotificationInNok(t *testing.T) {
	merchant := "Bella Vista  Drøbak, AkershusBank Norwegian\nkr 478,00"
	parsed := parseQuickAddInput(merchant, "kr 478,00", "Bank Norwegian")

	assert.Equal(t, "Bella Vista", parsed.Name)
	assert.Equal(t, "Bella Vista, Drøbak", parsed.Description)
	assert.Equal(t, "NOK", parsed.Currency)
	assert.Equal(t, 478.0, parsed.Amount)
	assert.Equal(t, "", parsed.OriginalAmountText)
}

func TestParseQuickAddInput_WalletNotificationInEuro(t *testing.T) {
	merchant := "Albert Heijn  The Hague, South HollandBank Norwegian\n€ 29,05"
	parsed := parseQuickAddInput(merchant, "€ 29,05", "Bank Norwegian")

	assert.Equal(t, "Albert Heijn", parsed.Name)
	assert.Equal(t, "Albert Heijn, The Hague", parsed.Description)
	assert.Equal(t, "EUR", parsed.Currency)
	assert.Equal(t, 29.05, parsed.Amount)
	assert.Equal(t, "€ 29,05", parsed.OriginalAmountText)
}

func TestParseQuickAddInput_WalletNotificationWithoutLocation(t *testing.T) {
	merchant := "UberBank Norwegian\n€ 7,21"
	parsed := parseQuickAddInput(merchant, "€ 7,21", "Bank Norwegian")

	assert.Equal(t, "Uber", parsed.Name)
	assert.Equal(t, "Uber", parsed.Description)
	assert.Equal(t, "EUR", parsed.Currency)
	assert.Equal(t, 7.21, parsed.Amount)
}

func TestParseQuickAddInput_WalletNotificationWithThousandsSeparator(t *testing.T) {
	merchant := "bull-ski-kajakk.no (KCO)Bank Norwegian\nkr 1 537,20"
	parsed := parseQuickAddInput(merchant, "kr 1 537,20", "Bank Norwegian")

	assert.Equal(t, "bull-ski-kajakk.no (KCO)", parsed.Description)
	assert.Equal(t, "NOK", parsed.Currency)
	assert.Equal(t, 1537.2, parsed.Amount)
}

func TestParseQuickAddInput_SwedishKronor(t *testing.T) {
	merchant := "Systembolaget  Strömstad, Västra Götaland CountyBank Norwegian\nSEK 165,50"
	parsed := parseQuickAddInput(merchant, "SEK 165,50", "Bank Norwegian")

	assert.Equal(t, "Systembolaget, Strömstad", parsed.Description)
	assert.Equal(t, "SEK", parsed.Currency)
	assert.Equal(t, 165.5, parsed.Amount)
	assert.Equal(t, "SEK 165,50", parsed.OriginalAmountText)
}

func TestParseQuickAddInput_BankNorwegianNotificationInNok(t *testing.T) {
	// the Bank Norwegian shortcut splits "<merchant>: NOK <amount>" itself
	parsed := parseQuickAddInput("Billettservice AS", "1 887,00", "Bank Norwegian")

	assert.Equal(t, "Billettservice AS", parsed.Description)
	assert.Equal(t, "NOK", parsed.Currency)
	assert.Equal(t, 1887.0, parsed.Amount)
}

func TestParseQuickAddInput_BankNorwegianNotificationInForeignCurrency(t *testing.T) {
	// the shortcut only splits on ": NOK", so a foreign purchase arrives whole in both fields
	parsed := parseQuickAddInput("NEON.TECH: USD 3,08", "NEON.TECH: USD 3,08", "Bank Norwegian")

	assert.Equal(t, "NEON.TECH", parsed.Name)
	assert.Equal(t, "NEON.TECH", parsed.Description)
	assert.Equal(t, "USD", parsed.Currency)
	assert.Equal(t, 3.08, parsed.Amount)
	assert.Equal(t, "USD 3,08", parsed.OriginalAmountText)
}

func TestParseQuickAddInput_PlainNumbers(t *testing.T) {
	parsed := parseQuickAddInput("Kiwi", "42.5", "Brukskonto")

	assert.Equal(t, "Kiwi", parsed.Description)
	assert.Equal(t, "NOK", parsed.Currency)
	assert.Equal(t, 42.5, parsed.Amount)
}

func TestParseQuickAddInput_UnambiguousSymbols(t *testing.T) {
	for symbol, currency := range map[string]string{"฿": "THB", "₫": "VND", "₭": "LAK", "៛": "KHR", "₹": "INR", "Rp": "IDR", "RM": "MYR"} {
		parsed := parseQuickAddInput("Shop  Somewhere, RegionBank Norwegian\n"+symbol+" 450,00", symbol+" 450,00", "Bank Norwegian")

		assert.Equal(t, currency, parsed.Currency, symbol)
		assert.Equal(t, 450.0, parsed.Amount, symbol)
		assert.Equal(t, symbol+" 450,00", parsed.OriginalAmountText, symbol)
	}
}

func TestParseQuickAddInput_AmbiguousSymbolsLeaveCurrencyUnknown(t *testing.T) {
	// "$" and "¥" cover several currencies and "Rs" is used for both Nepalese and Indian rupees,
	// so the categoriser decides from context instead
	for _, symbol := range []string{"$", "¥", "Rs", "Rs.", "रू", "₨"} {
		parsed := parseQuickAddInput("Thamel Momo  Kathmandu, BagmatiBank Norwegian\n"+symbol+" 1 250,00", symbol+" 1 250,00", "Bank Norwegian")

		assert.Equal(t, "", parsed.Currency, symbol)
		assert.Equal(t, 1250.0, parsed.Amount, symbol)
		assert.Equal(t, symbol+" 1 250,00", parsed.OriginalAmountText, symbol)
		assert.Equal(t, "Thamel Momo, Kathmandu", parsed.Description, symbol)
	}
}

func TestParseAmountString_Separators(t *testing.T) {
	assert.Equal(t, 1887.0, parseAmountString("1 887,00"))
	assert.Equal(t, 1250.5, parseAmountString("1,250.50"))
	assert.Equal(t, 1250.5, parseAmountString("1.250,50"))
	assert.Equal(t, 1250.0, parseAmountString("1,250"))
	assert.Equal(t, 1250000.0, parseAmountString("1,250,000"))
	assert.Equal(t, 3.08, parseAmountString("3,08"))
	assert.Equal(t, 42.5, parseAmountString("42.5"))
}

func TestConvertQuickAddAmount_ThroughEuroBase(t *testing.T) {
	rates := &models.LatestExchangeRateResponse{
		BaseCurrency: "EUR",
		ExchangeRates: models.LatestExchangeRateSlice{
			{Currency: "NOK", Rate: "11.0"},
			{Currency: "USD", Rate: "1.1"},
		},
	}

	amount, ok := convertQuickAddAmount(29.05, "EUR", "NOK", rates)
	assert.True(t, ok)
	assert.InDelta(t, 319.55, amount, 0.001)

	amount, ok = convertQuickAddAmount(3.08, "USD", "NOK", rates)
	assert.True(t, ok)
	assert.InDelta(t, 30.8, amount, 0.001)

	amount, ok = convertQuickAddAmount(100, "NOK", "NOK", rates)
	assert.True(t, ok)
	assert.Equal(t, 100.0, amount)
}

func TestConvertQuickAddAmount_UnknownCurrency(t *testing.T) {
	rates := &models.LatestExchangeRateResponse{
		BaseCurrency:  "EUR",
		ExchangeRates: models.LatestExchangeRateSlice{{Currency: "NOK", Rate: "11.0"}},
	}

	_, ok := convertQuickAddAmount(100000, "VND", "NOK", rates)
	assert.False(t, ok)

	_, ok = convertQuickAddAmount(10, "EUR", "NOK", nil)
	assert.False(t, ok)
}
