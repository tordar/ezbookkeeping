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
   Then edit `conf/ezbookkeeping.dev.ini`: set `secret_key` in `[security]` (required). Do not commit `ezbookkeeping.dev.ini` (it is gitignored).

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
  Then set `secret_key` in `[security]` Do not commit `ezbookkeeping.dev.ini`; it is gitignored.
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
