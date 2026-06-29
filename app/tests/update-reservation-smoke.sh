#!/usr/bin/env bash
# Confirms that the short Control-to-systemd handoff reservation rejects only
# fresh active jobs. The installer integration smoke separately proves that a
# rejected direct entrypoint cannot overwrite the active transaction records.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
INSTALL="${1:-$ROOT/../installer/install.sh}"
[ -f "$INSTALL" ] || { echo "FAIL: installer not found: $INSTALL" >&2; exit 1; }
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT INT TERM
awk '
  /^update_job_reservation_is_active\(\)/ { take=1 }
  /^# Detect non-interactive modes early/ { take=0 }
  take { print }
' "$INSTALL" > "$TMP/reservation-function.sh"
[ -s "$TMP/reservation-function.sh" ] || { echo 'FAIL: reservation function could not be extracted' >&2; exit 1; }
# shellcheck disable=SC1090
source "$TMP/reservation-function.sh"
CACHE_DIR="$TMP/cache"
mkdir -p "$CACHE_DIR"
NOW=1800000000
mkdir -p "$TMP/fakebin"
cat > "$TMP/fakebin/date" <<DATE
#!/usr/bin/env bash
[ "\${1:-}" = '+%s' ] && { printf '%s\n' "$NOW"; exit 0; }
exec /bin/date "\$@"
DATE
chmod +x "$TMP/fakebin/date"
PATH="$TMP/fakebin:$PATH"; export PATH
update_cli_supports(){ [ "${1:-}" = '--update-job' ]; }
fixture_state=""
fixture_updated=""
update_cli(){
  [ "${1:-}" = '--json-get' ] || return 2
  case "${3:-}" in
    state) printf '%s\n' "$fixture_state" ;;
    updatedAt) printf '%s\n' "$fixture_updated" ;;
    *) return 2 ;;
  esac
}
fixture_state=queued fixture_updated=$((NOW-120))
update_job_reservation_is_active || { echo 'FAIL: fresh queued reservation was not active' >&2; exit 1; }
fixture_state=running fixture_updated=$((NOW-121))
if update_job_reservation_is_active; then
  echo 'FAIL: stale reservation remained active after grace' >&2; exit 1
fi
fixture_state=success fixture_updated=$((NOW-1))
if update_job_reservation_is_active; then
  echo 'FAIL: terminal reservation was active' >&2; exit 1
fi
fixture_state=queued fixture_updated=not-a-time
if update_job_reservation_is_active; then
  echo 'FAIL: malformed reservation timestamp was active' >&2; exit 1
fi
printf 'PASS: Control handoff reservation is bounded, active-state-only, and stale-safe\n'
