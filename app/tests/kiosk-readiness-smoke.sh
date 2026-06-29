#!/usr/bin/env bash
# Proves a kiosk launch waits for the version-matched local readiness endpoint
# instead of opening Surf while the local service is still refusing connections.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TMP="$(mktemp -d)"; FIXTURE="$TMP/dashboard"; FAKE_BIN="$TMP/fake-bin"; SURF_RECORD="$TMP/surf-launches"
KIOSK_PID=""; SERVER_PID=""
cleanup(){
  [ -n "$KIOSK_PID" ] && kill -0 "$KIOSK_PID" 2>/dev/null && kill -TERM "$KIOSK_PID" 2>/dev/null || true
  [ -n "$KIOSK_PID" ] && wait "$KIOSK_PID" 2>/dev/null || true
  [ -n "$SERVER_PID" ] && kill -0 "$SERVER_PID" 2>/dev/null && kill -TERM "$SERVER_PID" 2>/dev/null || true
  [ -n "$SERVER_PID" ] && wait "$SERVER_PID" 2>/dev/null || true
  rm -rf "$TMP"
}
trap cleanup EXIT INT TERM
mkdir -p "$FIXTURE/bin" "$FIXTURE/config" "$FIXTURE/ui" "$FAKE_BIN"
cp "$ROOT/kiosk.sh" "$FIXTURE/kiosk.sh"
cp "$ROOT/bin/dashboard-kiosk-lib.sh" "$ROOT/bin/dashboard-resilience-lib.sh" "$FIXTURE/bin/"
cp "$ROOT/ui/time-sync.html" "$ROOT/ui/safe-mode.html" "$FIXTURE/ui/"
printf '1.4.1-beta.19\n' > "$FIXTURE/VERSION"
printf 'window.DASHBOARD_CONFIG = { profile: "balanced" };\n' > "$FIXTURE/config/config.local.js"
for helper in gen-calendars.sh update-holidays.sh gen-default-calendars.sh; do printf '#!/bin/bash\nexit 0\n' > "$FIXTURE/bin/$helper"; done
cat > "$FAKE_BIN/surf" <<'FAKE_SURF'
#!/bin/bash
printf '%s\n' "$*" >> "$SURF_RECORD"
trap 'exit 0' INT TERM HUP
while :; do sleep 1; done
FAKE_SURF
for tool in wmctrl xset pkill unclutter; do printf '#!/bin/bash\n[ "${1:-}" = "--help" ] && echo --timeout\nexit 0\n' > "$FAKE_BIN/$tool"; done
chmod +x "$FIXTURE/kiosk.sh" "$FIXTURE/bin/"*.sh "$FAKE_BIN/"*
PORT="$(python3 - <<'PY_PORT'
import socket
with socket.socket() as s:
    s.bind(('127.0.0.1',0)); print(s.getsockname()[1])
PY_PORT
)"
PATH="$FAKE_BIN:$PATH" SURF_RECORD="$SURF_RECORD" DASHBOARD_PORT="$PORT" DASH_KIOSK_SKIP_TIME_GATE=1 DASH_SERVER_START_WAIT_SECONDS=8 "$FIXTURE/kiosk.sh" > "$TMP/kiosk.log" 2>&1 &
KIOSK_PID=$!
sleep 1
[ ! -s "$SURF_RECORD" ] || { echo 'FAIL: Surf launched before local readiness was available' >&2; exit 1; }
cat > "$TMP/server.py" <<'PY_SERVER'
import json, os
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
class H(BaseHTTPRequestHandler):
 def log_message(self,*a): pass
 def do_GET(self):
  if self.path.split('?',1)[0] == '/api/ready':
   b=json.dumps({'goServer':True,'version':'1.4.1-beta.19'}).encode(); self.send_response(200); self.send_header('Content-Length',str(len(b))); self.end_headers(); self.wfile.write(b); return
  self.send_response(404); self.end_headers()
ThreadingHTTPServer(('127.0.0.1', int(os.environ['PORT'])), H).serve_forever()
PY_SERVER
PORT="$PORT" python3 "$TMP/server.py" > "$TMP/server.log" 2>&1 & SERVER_PID=$!
i=0
while [ ! -s "$SURF_RECORD" ]; do
  i=$((i+1)); [ "$i" -lt 80 ] || { echo 'FAIL: Surf did not launch after version-matched readiness' >&2; cat "$TMP/kiosk.log" >&2; exit 1; }; sleep 0.1
done
grep -Fq "http://127.0.0.1:$PORT/" "$SURF_RECORD" || { echo 'FAIL: Surf did not use explicit IPv4 URL' >&2; exit 1; }
echo 'PASS: kiosk waits for version-matched local readiness before launching Surf'
