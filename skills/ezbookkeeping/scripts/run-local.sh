#!/usr/bin/env sh
# Run ezBookkeeping locally (backend only). For full dev with frontend hot-reload,
# run the backend with this script, then in another terminal: npm run serve

set -e
cd "$(dirname "$0")/.."

# Ensure Go is on PATH (common install locations when not set in npm's env)
for go_path in /opt/homebrew/bin /opt/homebrew/opt/go/libexec/bin /usr/local/go/bin /usr/local/bin /usr/local/opt/go/libexec/bin; do
  if [ -x "$go_path/go" ]; then
    export PATH="$go_path:$PATH"
    break
  fi
done

# Create directories used by the app (relative to project root)
mkdir -p data storage log

# Use dev config so backend serves from dist/ and runs in development mode
CONF_PATH="conf/ezbookkeeping.dev.ini"
if [ ! -f "$CONF_PATH" ]; then
  echo "Error: $CONF_PATH not found."
  exit 1
fi

# Build frontend if dist/ is missing (backend needs static files)
if [ ! -d "dist" ]; then
  echo "Building frontend (dist/ missing)..."
  npm run build
fi

# Run backend (config must be passed: default path is relative to cwd and may not resolve)
if command -v go >/dev/null 2>&1; then
  echo "Starting backend (Go) on http://localhost:8080 ..."
  go run . server run --conf-path "$CONF_PATH"
else
  if [ -f "./ezbookkeeping" ]; then
    echo "Starting backend (binary) on http://localhost:8080 ..."
    ./ezbookkeeping server run --conf-path "$CONF_PATH"
  else
    echo "Error: Go not in PATH and no ./ezbookkeeping binary. Install Go and run from repo root, or build first: ./build.sh backend --no-lint --no-test"
    exit 1
  fi
fi
