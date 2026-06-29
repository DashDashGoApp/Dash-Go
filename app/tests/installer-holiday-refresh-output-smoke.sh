#!/usr/bin/env bash
# The interactive first-install holiday refresh must keep the Go helper's
# structured JSON in a diagnostic log instead of leaking it into the terminal.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
INSTALL="$ROOT/../installer/install.sh"
TMP="$(mktemp -d)"
cleanup(){ rm -rf "$TMP"; }
trap cleanup EXIT

awk '/^refresh_holidays_for_installer\(\)/{take=1} /^configure_app_setup\(\)/{take=0} take{print}' "$INSTALL" > "$TMP/holiday-helper.sh"
[ -s "$TMP/holiday-helper.sh" ] || { echo 'FAIL: holiday refresh helper could not be extracted' >&2; exit 1; }

ok(){ printf 'OK: %s\n' "$*"; }
warn(){ printf 'WARN: %s\n' "$*"; }
export BIN_DIR="$TMP/bin" LOG_DIR="$TMP/logs"
mkdir -p "$BIN_DIR" "$LOG_DIR"
cat > "$BIN_DIR/update-holidays.sh" <<'HOLIDAY'
#!/usr/bin/env bash
printf '{"changed":1,"errors":[],"generator":"go","ok":true}\n'
HOLIDAY
chmod +x "$BIN_DIR/update-holidays.sh"
# shellcheck source=/dev/null
source "$TMP/holiday-helper.sh"

refresh_holidays_for_installer >"$TMP/success.out" 2>&1
if grep -Fq '"generator":"go"' "$TMP/success.out"; then
  echo 'FAIL: holiday JSON leaked into interactive installer output' >&2
  exit 1
fi
grep -Fq 'OK: Holiday calendar refreshed' "$TMP/success.out"
grep -Fq '"generator":"go"' "$LOG_DIR/holiday-refresh-install.log"

cat > "$BIN_DIR/update-holidays.sh" <<'HOLIDAY_FAIL'
#!/usr/bin/env bash
printf '{"changed":0,"errors":["offline"],"generator":"go","ok":false}\n'
exit 1
HOLIDAY_FAIL
chmod +x "$BIN_DIR/update-holidays.sh"
set +e
refresh_holidays_for_installer >"$TMP/failure.out" 2>&1
status=$?
set -e
[ "$status" -eq 1 ] || { echo "FAIL: expected holiday helper failure, got $status" >&2; exit 1; }
if grep -Fq '"generator":"go"' "$TMP/failure.out"; then
  echo 'FAIL: failed holiday JSON leaked into interactive installer output' >&2
  exit 1
fi
grep -Fq 'WARN: Holiday calendar refresh failed (will retry via cron; details:' "$TMP/failure.out"
grep -Fq '"errors":["offline"]' "$LOG_DIR/holiday-refresh-install.log"
printf 'PASS: holiday refresh keeps structured helper output in an installer diagnostic log\n'
