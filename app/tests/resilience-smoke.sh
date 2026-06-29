#!/usr/bin/env bash
# Release-blocking smoke for bounded static recovery, storage state, and
# health-state readers. It needs no network, systemd, X11, or root access.
set -eu
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TMP="$(mktemp -d)"; DASH_ROOT="$TMP/dashboard"
cleanup(){ rm -rf "$TMP"; }
trap cleanup EXIT INT TERM
mkdir -p "$DASH_ROOT/cache" "$DASH_ROOT/ui" "$DASH_ROOT/bin" "$DASH_ROOT/logs" "$DASH_ROOT/fakebin"
printf '1.4.1-beta.13\n' > "$DASH_ROOT/VERSION"
printf '{"buildEpoch":%s}\n' "$(date +%s)" > "$DASH_ROOT/manifest.json"
cat > "$DASH_ROOT/fakebin/timedatectl" <<'EOF'
#!/bin/sh
printf 'no\n'
EOF
chmod +x "$DASH_ROOT/fakebin/timedatectl"
cp "$ROOT/bin/dashboard-resilience-lib.sh" "$DASH_ROOT/bin/"
cp "$ROOT/bin/dashboard-storage-wear.sh" "$DASH_ROOT/bin/"
cp "$ROOT/ui/safe-mode.html" "$DASH_ROOT/ui/"
# shellcheck source=/dev/null
DASH="$DASH_ROOT"; CACHE_DIR="$DASH_ROOT/cache"; export DASH CACHE_DIR
. "$DASH_ROOT/bin/dashboard-resilience-lib.sh"
PATH="$DASH_ROOT/fakebin:$PATH" dash_clock_verified
[ -s "$DASH_ROOT/cache/clock-confirmed.json" ]
DASH_KIOSK_RESTART_WINDOW=60 DASH_KIOSK_RESTART_BURST=2 dash_note_kiosk_launch
DASH_KIOSK_RESTART_WINDOW=60 DASH_KIOSK_RESTART_BURST=2 dash_note_kiosk_launch
if DASH_KIOSK_RESTART_WINDOW=60 DASH_KIOSK_RESTART_BURST=2 dash_note_kiosk_launch; then
  echo 'FAIL: rapid restart burst did not trip' >&2; exit 1
fi
DASH_SAFE_MODE_COOLDOWN=1 dash_enter_safe_mode 'smoke recovery reason'
[ -s "$DASH_ROOT/cache/safe-mode-state.json" ]
grep -q 'smoke recovery reason' "$DASH_ROOT/cache/safe-mode-active.html"
DASH="$DASH_ROOT" "$DASH_ROOT/bin/dashboard-storage-wear.sh"
python3 - "$DASH_ROOT/cache/storage-wear-state.json" <<'PY'
import json,sys
x=json.load(open(sys.argv[1])); assert x['level'] in ('ok','watch','warn','failing'); assert 'canary' in x
PY
echo 'PASS: bounded safe-mode marker, persistent clock trust, and housekeeping storage state are valid'
