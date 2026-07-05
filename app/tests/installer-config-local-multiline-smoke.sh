#!/usr/bin/env bash
# Config-local editing is intentionally source-aware enough to replace a
# formatted top-level object without leaving orphaned nested lines behind.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
BIN="${DASHGO_CONTROL_SERVER_BIN:-}"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT INT TERM
if [ -z "$BIN" ]; then
  BIN="$TMP/dashboard-control-server"
  (cd "$ROOT" && go build -o "$BIN" ./cmd/dashboard-control-server)
fi
[ -x "$BIN" ] || { echo "FAIL: control server binary is unavailable: $BIN" >&2; exit 1; }
node --version >/dev/null
CFG="$TMP/config.local.js"
cat > "$CFG" <<'CONFIG'
window.DASHBOARD_LOCAL = {
  weatherAlerts: {
    enabled: false,
    refreshMinutes: 90,
    nested: { preserve: true },
  },
  birthdays: [],
  keep: "unchanged",
};
CONFIG
"$BIN" --installer-config-local --file "$CFG" --mode display --temp-unit celsius --wind-unit kph --weather-days 10 --refresh-wx 25 --alert-refresh 5 --alert-min severe
node --check "$CFG"
grep -Fq 'weatherAlerts: { enabled: true, refreshMinutes: 5, minSeverity: "severe" },' "$CFG" || { echo 'FAIL: display edit did not replace the multiline weatherAlerts field' >&2; exit 1; }
if grep -Fq 'nested: { preserve: true }' "$CFG"; then
  echo 'FAIL: multiline config edit left orphaned nested weatherAlerts content' >&2
  exit 1
fi
grep -Fq 'keep: "unchanged"' "$CFG" || { echo 'FAIL: config edit damaged an unrelated field' >&2; exit 1; }
printf '%s\n' 'PASS: multiline config.local.js values are replaced as complete assignments and remain valid JavaScript'
