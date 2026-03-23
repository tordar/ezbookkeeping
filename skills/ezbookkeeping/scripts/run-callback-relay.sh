#!/usr/bin/env sh
# Run the callback-relay server for Enable Banking OAuth. Expose it with ngrok
# so the bank can redirect to your relay, which forwards the code to the local backend.
#
# Usage:
#   1. Start the main app: npm run dev (or npm run dev:backend)
#   2. In another terminal: ./scripts/run-callback-relay.sh
#   3. In a third terminal: ngrok http 9999
#   4. In Enable Banking Control Panel, set redirect URI to https://YOUR-NGROK-URL/callback
#   5. In conf/ezbookkeeping.dev.ini set enablebanking_callback_url = https://YOUR-NGROK-URL/callback

set -e
cd "$(dirname "$0")/.."

PORT="${CALLBACK_RELAY_PORT:-9999}"
BACKEND="${CALLBACK_RELAY_BACKEND:-http://localhost:8080}"

if command -v go >/dev/null 2>&1; then
  echo "Starting callback-relay on port $PORT, forwarding to $BACKEND"
  echo "Expose with: ngrok http $PORT"
  exec go run ./scripts/callback-relay --port "$PORT" --backend "$BACKEND"
else
  echo "Error: Go not in PATH. Install Go or run: go run ./scripts/callback-relay --port $PORT --backend $BACKEND"
  exit 1
fi
