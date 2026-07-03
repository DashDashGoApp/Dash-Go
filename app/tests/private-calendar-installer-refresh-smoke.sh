#!/usr/bin/env bash
# A payload update must regenerate only derived private-calendar files after
# protected user state is restored. It must never run discovery or a network
# sync as part of an update.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
INSTALL="$ROOT/../installer/install.sh"
TMP="$(mktemp -d)"
cleanup(){ rm -rf "$TMP"; }
trap cleanup EXIT

awk '/^refresh_private_calendar_configuration_after_payload\(\)/{take=1} take{print} take && /^}/{exit}' "$INSTALL" > "$TMP/function.sh"
warn(){ printf 'WARN: %s\n' "$*" >&2; }
ok(){ printf 'OK: %s\n' "$*"; }
# shellcheck source=/dev/null
source "$TMP/function.sh"

HOME="$TMP/home"; export HOME
DASH="$HOME/dashboard"; BIN_DIR="$DASH/bin"; LOG_DIR="$DASH/logs"
mkdir -p "$BIN_DIR" "$LOG_DIR" "$HOME/.dashboard-vdirsyncer"
printf 'one exact selected pair\n' > "$HOME/.dashboard-vdirsyncer/pairs"
cat > "$BIN_DIR/setup-vdirsyncer.sh" <<'EOS'
#!/usr/bin/env bash
[ "${1:-}" = "--refresh" ] || exit 64
printf 'refresh-only\n' >> "${DASH_REFRESH_LOG:?}"
EOS
chmod +x "$BIN_DIR/setup-vdirsyncer.sh"
export DASH_REFRESH_LOG="$TMP/refresh.log"
refresh_private_calendar_configuration_after_payload
[ "$(cat "$TMP/refresh.log")" = 'refresh-only' ]
grep -Fq 'refresh_private_calendar_configuration_after_payload' "$INSTALL"
if grep -Eq '(sync-vdir|[[:space:]]discover([[:space:]]|$)|[[:space:]]sync([[:space:]]|$))' "$TMP/function.sh"; then
  echo 'FAIL: payload refresh attempted remote discovery or sync' >&2
  exit 1
fi
printf 'PASS: update refreshes derived private-calendar configuration locally without provider discovery or sync\n'
