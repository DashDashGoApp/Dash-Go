#!/usr/bin/env bash
# Demo-mode Go helpers are structured CLI commands. Installer output must stay
# human-readable while retaining exact diagnostics in an owner-local log.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
INSTALL="$ROOT/../installer/install.sh"
TMP="$(mktemp -d)"
cleanup(){ rm -rf "$TMP"; }
trap cleanup EXIT

awk '/^run_demo_mode_cli_for_installer\(\)/{take=1} /^installer_demo_defaults_active\(\)/{take=0} take{print}' "$INSTALL" > "$TMP/demo-helper.sh"
[ -s "$TMP/demo-helper.sh" ] || { echo 'FAIL: could not extract demo helper functions' >&2; exit 1; }

say(){ printf 'SAY: %s\n' "$*"; }
ok(){ printf 'OK: %s\n' "$*"; }
warn(){ printf 'WARN: %s\n' "$*"; }
export BIN_DIR="$TMP/bin" LOG_DIR="$TMP/logs" CACHE_DIR="$TMP/cache" CAL_DIR="$TMP/calendars" CONFIG_DIR="$TMP/config"
mkdir -p "$BIN_DIR" "$LOG_DIR"
cat > "$BIN_DIR/dashboard-control-server" <<'CLI'
#!/usr/bin/env bash
printf '{"changed":1,"generator":"go","ok":true}\n'
CLI
chmod +x "$BIN_DIR/dashboard-control-server"
# shellcheck source=/dev/null
source "$TMP/demo-helper.sh"

DEMO_RESET_REQUESTED=1
DEMO_WIPE_CALENDARS_REQUESTED=0
reset_demo_mode_if_requested >"$TMP/reset.out" 2>&1
if grep -Fq '"generator":"go"' "$TMP/reset.out"; then
  echo 'FAIL: demo reset JSON leaked into installer output' >&2
  exit 1
fi
grep -Fq '"generator":"go"' "$LOG_DIR/demo-mode-install.log"

enable_demo_mode >"$TMP/enable.out" 2>&1
if grep -Fq '"generator":"go"' "$TMP/enable.out"; then
  echo 'FAIL: demo enable JSON leaked into installer output' >&2
  exit 1
fi

action_fail(){ cat > "$BIN_DIR/dashboard-control-server" <<'CLI'
#!/usr/bin/env bash
printf '{"changed":0,"errors":["fixture failure"],"generator":"go","ok":false}\n'
exit 1
CLI
chmod +x "$BIN_DIR/dashboard-control-server"; }
action_fail
set +e
enable_demo_mode >"$TMP/failure.out" 2>&1
status=$?
set -e
[ "$status" -eq 1 ] || { echo "FAIL: expected demo enable failure, got $status" >&2; exit 1; }
if grep -Fq '"generator":"go"' "$TMP/failure.out"; then
  echo 'FAIL: failed demo JSON leaked into installer output' >&2
  exit 1
fi
grep -Fq 'WARN: Demo Mode helper reported an issue (details:' "$TMP/failure.out"
grep -Fq 'fixture failure' "$LOG_DIR/demo-mode-install.log"
printf 'PASS: demo helper output is captured in an installer diagnostic log\n'
