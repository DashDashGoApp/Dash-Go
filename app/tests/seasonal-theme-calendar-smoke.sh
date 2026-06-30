#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$ROOT/bin/seasonal-themes.sh"
SERVER="$ROOT/cmd/dashboard-control-server/main.go"
AVAIL="$ROOT/cmd/dashboard-control-server/theme_availability.go"
for required in '--seasonal-theme' 'events.cache.json' 'jewish-holidays.' 'kwanzaa' 'memorialday' 'mothersday'; do
  grep -Fq -- "$required" "$AVAIL" || { echo "FAIL: missing calendar-aware seasonal token $required" >&2; exit 1; }
done
grep -Fq 'dashboard-control-server" --seasonal-theme' "$SCRIPT" || { echo "FAIL: seasonal helper must ask the local server for exact holiday matches" >&2; exit 1; }
grep -Fq '.dashboard-seasonal-themes' "$SCRIPT" || { echo "FAIL: seasonal helper must retain an explicit enabled marker" >&2; exit 1; }
grep -Fq 'case "--seasonal-theme"' "$SERVER" || { echo "FAIL: server must expose the seasonal resolver CLI" >&2; exit 1; }
if grep -Eqi 'back.?to.?school|game.?day' "$ROOT/themes.list" "$ROOT/ui/js/config-theme-meta.js"; then
  echo "FAIL: excluded themes remain in the source catalog" >&2
  exit 1
fi
printf 'PASS: seasonal helper uses local event-cache resolver without fixed-date guesses for optional observances\n'
