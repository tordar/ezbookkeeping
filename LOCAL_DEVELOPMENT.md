# Running ezBookkeeping Locally

This guide covers running the app from source on your machine.

## Prerequisites

- **Go** 1.25+ ([golang.org](https://golang.org/))
- **Node.js** and **npm** (for the frontend)
- **GCC** (for building the Go backend with CGO, e.g. SQLite)

## Quick start (backend + built frontend)

1. **Install dependencies**
   - Frontend: `npm install`
   - Backend: Go dependencies are fetched on first build/run

2. **Create your local config** (one-time)
   ```bash
   cp conf/ezbookkeeping.dev.ini.example conf/ezbookkeeping.dev.ini
   ```
   Then edit `conf/ezbookkeeping.dev.ini`: set `secret_key` in `[security]` (required), and if you use bank integration, fill in the `[bank_integration]` values. Do not commit `ezbookkeeping.dev.ini` (it is gitignored).

3. **Run the app**
   ```bash
   npm run dev
   ```
   This builds the frontend (if needed), then starts the backend. Open **http://localhost:8080** in your browser.

## Option A: Backend only (serves built frontend)

1. Create data directories (done automatically by the script):
   ```bash
   mkdir -p data storage log
   ```

2. Build the frontend once (so the backend has static files to serve):
   ```bash
   npm run build
   ```

3. Start the backend:
   ```bash
   npm run dev:backend
   ```
   Or directly:
   ```bash
   ./scripts/run-local.sh
   ```
   Or with Go:
   ```bash
   go run . server run --conf-path conf/ezbookkeeping.dev.ini
   ```

4. Open **http://localhost:8080**

## Option B: Frontend dev server (hot reload)

Use this when you’re changing the Vue/Vite frontend and want hot reload.

1. **Terminal 1 – backend** (must be running so the frontend can call the API):
   ```bash
   npm run build
   npm run dev:backend
   ```
   Backend runs on **http://localhost:8080**.

2. **Terminal 2 – frontend dev server**:
   ```bash
   npm run serve
   ```
   Vite runs on **http://localhost:8081** and proxies `/api`, `/oauth2`, etc. to the backend.

3. Open **http://localhost:8081** in your browser. UI changes reload automatically; backend still on 8080.

## Config used for local run

- **Config file:** `conf/ezbookkeeping.dev.ini` — create it by copying the template:
  ```bash
  cp conf/ezbookkeeping.dev.ini.example conf/ezbookkeeping.dev.ini
  ```
  Then set `secret_key` in `[security]` and any optional values (e.g. `[bank_integration]`). Do not commit `ezbookkeeping.dev.ini`; it is gitignored.
- **Differences from default:** `mode = development`, `static_root_path = dist` (so the backend serves the built frontend from `dist/`).
- **Data:** SQLite DB path is `data/ezbookkeeping.db` (relative to project root). Directories `data/`, `storage/`, `log/` are in `.gitignore`.

To use the default config instead (e.g. `static_root_path = public`), run:
```bash
go run . server run --conf-path conf/ezbookkeeping.ini
```
and ensure the frontend is built into `public/` (e.g. copy `dist/` to `public/` or change the build output).

## Building the backend binary

Full build (with lint and tests):
```bash
./build.sh backend
```

Quick build (no lint/tests, may avoid static linking issues on some systems):
```bash
CGO_ENABLED=1 go build -o ezbookkeeping .
```
Then run:
```bash
./ezbookkeeping server run --conf-path conf/ezbookkeeping.dev.ini
```

## Bank integration (Enable Banking) with a public callback URL

Enable Banking redirects the user’s browser to your **redirect URI** after they authorize at the bank. That URI must be publicly reachable. Run the **callback-relay** locally; then expose it with either a second ngrok tunnel (same agent) or a different tunnel (e.g. localtunnel) so you can keep ngrok for another project.

1. **Start the main app** (backend + frontend):
   ```bash
   npm run dev
   ```
   Backend runs on **http://localhost:8080**.

2. **Start the callback-relay** (in a second terminal):
   ```bash
   ./scripts/run-callback-relay.sh
   ```
   The relay listens on port **9999** and forwards `GET /callback?code=...&state=...` to `http://localhost:8080/api/bank_integration/callback`.

3. **Expose the relay** with one of the options below.

4. **Configure Enable Banking**  
   In the [Enable Banking Control Panel](https://control.enablebanking.com/), add this redirect URI (replace with your public callback URL):
   ```
   https://YOUR-PUBLIC-URL/callback
   ```

5. **Configure the app**  
   In your local `conf/ezbookkeeping.dev.ini` (copy from `conf/ezbookkeeping.dev.ini.example` if you don’t have it yet), in the `[bank_integration]` section, set:
   ```ini
   enablebanking_app_id = your-app-id-from-control-panel
   enablebanking_private_key_path = your-key-filename.pem
   enablebanking_callback_url = https://YOUR-PUBLIC-URL/callback
   ```
   Restart the main app after changing config.

### Option A: Second tunnel in your existing ngrok (one agent, two URLs)

If you already run ngrok for another project, the free tier allows only **one agent session** but that agent can run **multiple tunnels**. Add the callback relay as a second tunnel in your ngrok config, then start all tunnels with one process.

1. Open your ngrok config (run `ngrok config check` to see its path; often `~/Library/Application Support/ngrok/ngrok.yml` on macOS).

2. Add a second endpoint for the callback relay. Example for **config version 3** (see [ngrok agent config](https://ngrok.com/docs/agent/config/)):
   ```yaml
   # In your existing ngrok.yml, under endpoints: add:
   - name: ezbookkeeping-callback
     upstream:
       url: 9999
   ```
   If your config uses the older **version 2** `tunnels:` format:
   ```yaml
   tunnels:
     # ... your existing tunnel ...
     ezbookkeeping-callback:
       proto: http
       addr: 9999
   ```

3. Start all tunnels (this single agent serves both your other project and the callback):
   ```bash
   ngrok start --all
   ```
   Or start only the callback tunnel if your other project is defined in the same file:
   ```bash
   ngrok start ezbookkeeping-callback
   ```

4. In the ngrok output, find the public URL for the callback tunnel (e.g. `https://abc123.ngrok-free.app`). Use that as **YOUR-PUBLIC-URL** in steps 4–5 above.

### Option B: Use a different tunnel for the callback (keep ngrok for the other project)

Use a separate tool only for the callback so your existing ngrok stays for the other project.

**localtunnel** (no account, quick):
```bash
npx localtunnel --port 9999
```
Use the printed URL (e.g. `https://something.loca.lt`) as **YOUR-PUBLIC-URL**. When the bank redirects there, you may see a localtunnel “Click to continue” page once; then the relay will receive the request.

**Cloudflare Tunnel** (free, stable URL with a bit of setup):
```bash
# Install: brew install cloudflare/cloudflare/cloudflared
cloudflared tunnel --url http://localhost:9999
```
Use the generated `*.trycloudflare.com` URL as **YOUR-PUBLIC-URL**.

### Option C: A route on your own website (stable URL, no tunnel)

You can use a **fixed URL on your personal site** as the callback. The route receives the redirect from the bank (with `code` and `state` in the query string) and redirects the user’s browser to your local backend with the same params. No tunnel and no changing domain.

**Flow:** Bank redirects to `https://your-site.com/path/callback?code=...&state=...` → your route responds with a redirect to `http://localhost:8080/api/bank_integration/callback?code=...&state=...` → the browser loads that URL (your app must be running on your machine).

**Requirements:**

- The route must pass through the full query string to the redirect.
- The redirect target must be reachable from the browser (use `http://localhost:8080` when you’re developing on the same machine).

**Example (Node/Express):**
```js
app.get('/path/callback', (req, res) => {
  const target = `http://localhost:8080/api/bank_integration/callback?${req.url.split('?')[1] || ''}`;
  res.redirect(302, target);
});
```

**Example (Vercel serverless function)** – save as `api/ezbookkeeping-callback.js` (or under your `api/` path):
```js
export default function handler(req, res) {
  const qs = req.url.includes('?') ? req.url.slice(req.url.indexOf('?')) : '';
  res.redirect(302, `http://localhost:8080/api/bank_integration/callback${qs}`);
}
```

**Example (Netlify function)** – save as `netlify/functions/ezbookkeeping-callback.js`:
```js
exports.handler = async (event) => ({
  statusCode: 302,
  headers: { Location: `http://localhost:8080/api/bank_integration/callback?${event.rawQuery || ''}` },
});
```

Then:

1. **Don’t run the callback-relay** – your app receives the callback directly.
2. **Enable Banking redirect URI:** `https://your-site.com/path/callback` (the path you chose).
3. **In `conf/ezbookkeeping.dev.ini`:**  
   `enablebanking_callback_url = https://your-site.com/path/callback`
4. Use the app (and complete the bank flow) on the same machine where the backend runs, so `localhost:8080` is correct.

If you want to run the app on another device, set the redirect target via an env var (e.g. `EZBOOKKEEPING_CALLBACK_TARGET`) and point it at that machine’s URL (e.g. `http://192.168.1.x:8080` on your LAN).

### Setting up ngrok

1. **Install ngrok**  
   - **macOS (Homebrew):** `brew install ngrok/ngrok/ngrok`  
   - **Other:** download from [ngrok.com/download](https://ngrok.com/download) or use the [official install script](https://ngrok.com/download).

2. **Create an account and get your auth token**  
   - Sign up at [dashboard.ngrok.com/signup](https://dashboard.ngrok.com/signup).  
   - In the dashboard go to **Your Authtoken** ([dashboard.ngrok.com/get-started/your-authtoken](https://dashboard.ngrok.com/get-started/your-authtoken)) and copy the token.

3. **Add your authtoken** (one-time):
   ```bash
   ngrok config add-authtoken YOUR_TOKEN
   ```

4. **Start a tunnel** to the port your relay (or app) is listening on:
   ```bash
   ngrok http 9999
   ```
   For the callback relay use port **9999**; to expose the main app use your `http_port` (e.g. **8080**).  
   To run **multiple tunnels** in one agent (e.g. another project + this callback), use a config file and `ngrok start --all` — see [Option A](#option-a-second-tunnel-in-your-existing-ngrok-one-agent-two-urls) above. The terminal shows the public URL; use it as **YOUR-PUBLIC-URL** in the Bank integration steps.

Each time you run `ngrok http 9999`, the free tier may assign a **new URL**. If it changes, update `enablebanking_callback_url` in `conf/ezbookkeeping.dev.ini` and the redirect URI in the Enable Banking Control Panel. Paid plans can use a fixed subdomain.

**Flow:** User clicks “Connect bank” → bank redirects to `https://YOUR-PUBLIC-URL/callback?code=...&state=...` → tunnel forwards to relay → relay redirects to `http://localhost:8080/api/bank_integration/callback` → backend completes the flow and sends the user back to the app.

## Troubleshooting

- **“cannot load configuration”**  
  Run the server from the **project root** and pass the config explicitly:  
  `--conf-path conf/ezbookkeeping.dev.ini`

- **Blank or 404 on frontend**  
  Run `npm run build` so `dist/` exists and is used by the backend (when using `conf/ezbookkeeping.dev.ini`).

- **Port 8080 in use**  
  Change `http_port` in `conf/ezbookkeeping.dev.ini`. If you use the frontend dev server (port 8081), update the proxy target in `vite.config.ts` to match.

- **SQLite / CGO build errors**  
  Install GCC (e.g. Xcode Command Line Tools on macOS: `xcode-select --install`).
