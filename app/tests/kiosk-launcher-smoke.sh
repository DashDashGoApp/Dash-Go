#!/bin/bash
# Release-blocking integration coverage for the packaged kiosk launcher.
set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TMP="$(mktemp -d)"
FIXTURE="$TMP/dashboard"
FAKE_BIN="$TMP/fake-bin"
WEBROOT="$TMP/webroot"
SURF_RECORD="$TMP/surf-launches"
KIOSK_PID=""
SERVER_PID=""

cleanup(){
  if [ -n "$KIOSK_PID" ] && kill -0 "$KIOSK_PID" 2>/dev/null; then
    kill -TERM "$KIOSK_PID" 2>/dev/null || true
    wait "$KIOSK_PID" 2>/dev/null || true
  fi
  if [ -n "$SERVER_PID" ] && kill -0 "$SERVER_PID" 2>/dev/null; then
    kill -TERM "$SERVER_PID" 2>/dev/null || true
    wait "$SERVER_PID" 2>/dev/null || true
  fi
  rm -rf "$TMP"
}
trap cleanup EXIT INT TERM

for tool in bash curl pgrep python3; do
  command -v "$tool" >/dev/null 2>&1 || {
    echo "FAIL: required test tool is missing: $tool" >&2
    exit 1
  }
done

mkdir -p "$FIXTURE/bin" "$FIXTURE/config" "$FAKE_BIN" "$WEBROOT"
cp "$ROOT/kiosk.sh" "$FIXTURE/kiosk.sh"
cp "$ROOT/bin/dashboard-kiosk-lib.sh" "$FIXTURE/bin/dashboard-kiosk-lib.sh"
cp "$ROOT/bin/dashboard-resilience-lib.sh" "$FIXTURE/bin/dashboard-resilience-lib.sh"
mkdir -p "$FIXTURE/ui"
cp "$ROOT/ui/time-sync.html" "$ROOT/ui/safe-mode.html" "$FIXTURE/ui/"
cp "$ROOT/VERSION" "$FIXTURE/VERSION"
printf 'window.DASHBOARD_CONFIG = { profile: "balanced" };\n' > "$FIXTURE/config/config.local.js"

for helper in gen-calendars.sh update-holidays.sh gen-default-calendars.sh; do
  printf '#!/bin/bash\nexit 0\n' > "$FIXTURE/bin/$helper"
done

cat > "$FAKE_BIN/surf" <<'FAKE_SURF'
#!/bin/bash
printf '%s %s\n' "$$" "$*" >> "$SURF_RECORD"
trap 'exit 0' INT TERM HUP
while :; do sleep 1; done
FAKE_SURF

cat > "$FAKE_BIN/wmctrl" <<'FAKE_WMCTRL'
#!/bin/bash
exit 0
FAKE_WMCTRL

cat > "$FAKE_BIN/xset" <<'FAKE_XSET'
#!/bin/bash
exit 0
FAKE_XSET

cat > "$FAKE_BIN/pkill" <<'FAKE_PKILL'
#!/bin/bash
# Keep the integration test isolated from the validation host's X session.
exit 0
FAKE_PKILL

cat > "$FAKE_BIN/unclutter" <<'FAKE_UNCLUTTER'
#!/bin/bash
case "${1:-}" in --help) echo '--timeout';; esac
exit 0
FAKE_UNCLUTTER

chmod +x "$FIXTURE/kiosk.sh" "$FIXTURE/bin/"*.sh "$FAKE_BIN/"*
printf '<!doctype html><title>Dash-Go launcher smoke</title>\n' > "$WEBROOT/index.html"

PORT="$(python3 - <<'PY_PORT'
import socket
with socket.socket() as sock:
    sock.bind(('127.0.0.1', 0))
    print(sock.getsockname()[1])
PY_PORT
)"
cat > "$TMP/ready-server.py" <<'PY_READY_SERVER'
import json, os
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
class Handler(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args): pass
    def do_GET(self):
        if self.path.split('?', 1)[0] == '/api/ready':
            body=json.dumps({'goServer': True, 'version': os.environ['DASH_VERSION']}).encode()
            self.send_response(200); self.send_header('Content-Type','application/json'); self.send_header('Content-Length',str(len(body))); self.end_headers(); self.wfile.write(body); return
        body=b'<!doctype html><title>Dash-Go launcher smoke</title>\n'
        self.send_response(200); self.send_header('Content-Type','text/html'); self.send_header('Content-Length',str(len(body))); self.end_headers(); self.wfile.write(body)
ThreadingHTTPServer(('127.0.0.1', int(os.environ['DASH_PORT'])), Handler).serve_forever()
PY_READY_SERVER
DASH_PORT="$PORT" DASH_VERSION="$(cat "$FIXTURE/VERSION")" python3 "$TMP/ready-server.py" > "$TMP/server.log" 2>&1 &
SERVER_PID=$!

i=0
until curl -sf --max-time 1 "http://127.0.0.1:$PORT/api/ready" >/dev/null 2>&1; do
  i=$((i+1))
  [ "$i" -lt 50 ] || { echo 'FAIL: fake local server did not start' >&2; exit 1; }
  sleep 0.1
done

PATH="$FAKE_BIN:$PATH" SURF_RECORD="$SURF_RECORD" DASHBOARD_PORT="$PORT" DASH_KIOSK_SKIP_TIME_GATE=1 \
  "$FIXTURE/kiosk.sh" > "$TMP/kiosk.stdout" 2>&1 &
KIOSK_PID=$!

i=0
while [ ! -s "$SURF_RECORD" ]; do
  if ! kill -0 "$KIOSK_PID" 2>/dev/null; then
    echo 'FAIL: packaged kiosk.sh exited before launching fake Surf' >&2
    cat "$TMP/kiosk.stdout" >&2 || true
    exit 1
  fi
  i=$((i+1))
  [ "$i" -lt 100 ] || {
    echo 'FAIL: packaged kiosk.sh did not launch fake Surf' >&2
    cat "$TMP/kiosk.stdout" >&2 || true
    exit 1
  }
  sleep 0.1
done

lock_pid="$(cat "$FIXTURE/cache/kiosk.lock/pid" 2>/dev/null || true)"
[ "$lock_pid" = "$KIOSK_PID" ] || {
  echo "FAIL: kiosk lock owner is '$lock_pid', expected '$KIOSK_PID'" >&2
  exit 1
}

sleep 1
launches="$(wc -l < "$SURF_RECORD" | tr -d ' ')"
[ "$launches" = 1 ] || {
  echo "FAIL: expected one fake Surf launch, observed $launches" >&2
  exit 1
}
if ! grep -Fq "http://127.0.0.1:$PORT/" "$SURF_RECORD"; then
  echo 'FAIL: kiosk did not launch Surf through explicit IPv4 loopback' >&2
  cat "$SURF_RECORD" >&2 || true
  exit 1
fi
if grep -q 'duplicate launcher will exit' "$TMP/kiosk.stdout"; then
  echo 'FAIL: launcher rejected its own PID as a duplicate' >&2
  exit 1
fi

kill -TERM "$KIOSK_PID"
wait "$KIOSK_PID" 2>/dev/null || true
KIOSK_PID=""
[ ! -d "$FIXTURE/cache/kiosk.lock" ] || {
  echo 'FAIL: kiosk lock remained after launcher shutdown' >&2
  exit 1
}

echo 'PASS: packaged kiosk.sh acquired one lock and launched one fake Surf process'
