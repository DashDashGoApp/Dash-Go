#!/usr/bin/env bash
# Both Lite and non-Lite kiosk holiday refresh paths must keep Go JSON out of
# the X-session log/terminal while retaining it in the bounded dashboard log.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
for file in "$ROOT/kiosk.sh" "$ROOT/bin/dashboard-kiosk-lib.sh"; do
  grep -Fq 'holiday-update.log' "$file" || { echo "FAIL: $file does not route holiday output to a log" >&2; exit 1; }
  if grep -Fq '"$BIN_DIR/update-holidays.sh" &' "$file"; then
    echo "FAIL: $file still backgrounds update-holidays.sh without redirection" >&2
    exit 1
  fi
done
printf 'PASS: kiosk holiday refresh output is redirected in every profile path\n'
