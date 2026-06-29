#!/usr/bin/env bash
# Proves the Python bridge is used only for a pre-capability installed updater.
# The fixture never runs a full installer or network request.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
INSTALLER="${1:-$ROOT/../installer/install.sh}"
TMP="$(mktemp -d)"
cleanup(){ rm -rf "$TMP"; }
trap cleanup EXIT INT TERM

awk '
  /^UPDATER_CAPABILITY_FLOOR=/{copy=1}
  copy && /^start_update_logging\(\)/{exit}
  copy {print}
' "$INSTALLER" > "$TMP/update-capability-functions.sh"
[ -s "$TMP/update-capability-functions.sh" ] || { echo 'FAIL: capability functions could not be extracted' >&2; exit 1; }

mkdir -p "$TMP/dashboard/bin" "$TMP/fake-bin"
DASH="$TMP/dashboard"
CACHE_DIR="$DASH/cache"
LOG_DIR="$DASH/logs"
UPDATE_STATUS_FILE="$CACHE_DIR/update-status.json"
RELEASE_TRACK=beta
warn(){ printf 'WARN:%s\n' "$*" >> "$TMP/messages"; }
say(){ printf 'SAY:%s\n' "$*" >> "$TMP/messages"; }
release_server_for_host(){ printf '%s\n' "$TMP/dashboard/bin/dashboard-control-server-linux-armv6"; }
cat > "$TMP/dashboard/bin/dashboard-control-server-linux-armv6" <<'SERVER'
#!/usr/bin/env bash
printf '%s\n' "${1:-}" >> "${CAP_QUERY_LOG:?}"
case "${1:-}" in
  --updater-capabilities)
    if [ "${CAP_MODE:-legacy}" = modern ]; then
      cat <<'CAPS'
dash-go-updater-capabilities-v1
release-file-list-v1
release-manifest-v1
stale-source-purge-v1
update-job-v1
update-status-v1
CAPS
      exit 0
    fi
    exit 64
    ;;
  --write-status) exit 0 ;;
esac
exit 64
SERVER
chmod +x "$TMP/dashboard/bin/dashboard-control-server-linux-armv6"
printf '#!/usr/bin/env bash\nexit 0\n' > "$TMP/fake-bin/python3"
chmod +x "$TMP/fake-bin/python3"
# shellcheck disable=SC1090
source "$TMP/update-capability-functions.sh"

printf '1.4.3-beta.71\n' > "$DASH/VERSION"
: > "$TMP/cap-queries"
: > "$TMP/messages"
CAP_QUERY_LOG="$TMP/cap-queries" CAP_MODE=legacy PATH="$TMP/fake-bin:$PATH" require_update_compatibility_tools
[ ! -s "$TMP/cap-queries" ] || { echo 'FAIL: legacy updater was probed with an unknown capability command' >&2; exit 1; }
grep -Fq 'One-time compatibility step:' "$TMP/messages" || { echo 'FAIL: legacy bridge was not labeled as one-time' >&2; exit 1; }

printf '1.4.3-beta.72\n' > "$DASH/VERSION"
: > "$TMP/cap-queries"
: > "$TMP/messages"
CAP_QUERY_LOG="$TMP/cap-queries" CAP_MODE=modern PATH="/usr/bin:/bin" require_update_compatibility_tools
 grep -Fqx -- '--updater-capabilities' "$TMP/cap-queries" || { echo 'FAIL: modern updater did not answer the live capability query' >&2; exit 1; }
[ ! -s "$TMP/messages" ] || { echo 'FAIL: modern updater emitted a bridge message' >&2; cat "$TMP/messages" >&2; exit 1; }

: > "$TMP/messages"
if CAP_QUERY_LOG="$TMP/cap-queries" CAP_MODE=legacy PATH="$TMP/fake-bin:$PATH" require_update_compatibility_tools; then
  echo 'FAIL: modern updater without release-manifest-v1 fell back instead of failing closed' >&2
  exit 1
fi
grep -Fq 'This version is too old to update automatically. Run --repair --system first, then update.' "$TMP/messages" || { echo 'FAIL: missing fail-closed capability diagnostic' >&2; exit 1; }
echo 'PASS: legacy bridge is one-time, modern Go capability discovery is live, and incomplete modern updaters fail closed'
