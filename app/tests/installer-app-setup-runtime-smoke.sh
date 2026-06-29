#!/usr/bin/env bash
# Runtime contract for App Setup's advanced private-registration path.
# Runs only against a fake Azure CLI: no Microsoft login, network, or real
# Entra app registration is attempted by source validation.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
INSTALL="$ROOT/../installer/install.sh"
TMP="$(mktemp -d)"
cleanup(){ rm -rf "$TMP"; }
trap cleanup EXIT

mkdir -p "$TMP/fakebin" "$TMP/home"
cat > "$TMP/fakebin/az" <<'AZ'
#!/usr/bin/env bash
set -euo pipefail
: "${AZ_LOG:?}"
printf '%s\n' "${AZURE_CONFIG_DIR:-}" >> "$AZ_LOG"
case "${1:-}:${2:-}:${3:-}" in
  login:*)
    exit 0
    ;;
  ad:app:create)
    seen_public=0 seen_audience=0 seen_access=0
    for arg in "$@"; do
      [ "$arg" = "--is-fallback-public-client" ] && seen_public=1
      [ "$arg" = "--sign-in-audience" ] && seen_audience=1
      case "$arg" in @*/required-resource-accesses.json) seen_access=1;; esac
    done
    [ "$seen_public" = 1 ] && [ "$seen_audience" = 1 ] && [ "$seen_access" = 1 ]
    printf '%s\n' '12345678-1234-1234-1234-1234567890ab'
    ;;
  *)
    printf 'unexpected fake az invocation: %s\n' "$*" >&2
    exit 64
    ;;
esac
AZ
chmod +x "$TMP/fakebin/az"

# Source only the App Setup helper block; do not execute install.sh's main body.
awk '/^valid_microsoft_client_id\(\)/{take=1} /^validate_download\(\)/{take=0} take{print}' "$INSTALL" > "$TMP/app-setup-functions.sh"
warn(){ printf 'WARN: %s\n' "$*" >&2; }
ok(){ printf 'OK: %s\n' "$*"; }
say(){ printf '== %s ==\n' "$*"; }
# shellcheck source=/dev/null
source "$TMP/app-setup-functions.sh"

export HOME="$TMP/home"
export SETTINGS_FILE="$TMP/home/settings.json"
export PATH="$TMP/fakebin:$PATH"
export AZ_LOG="$TMP/az.log"
printf 'CREATE PRIVATE APP\n' | register_todo_private_app_with_azure_cli >"$TMP/out" 2>&1
grep -Fq 'Microsoft To Do app registration complete' "$TMP/out"
grep -Fq 'Your private Microsoft app was created and its application ID was saved in Dash-Go.' "$TMP/out"
grep -Fq 'You do not need to copy the client ID or create a client secret.' "$TMP/out"
grep -Fq 'Select Link Microsoft account and complete the device-code sign-in on a phone or computer.' "$TMP/out"
grep -Fq 'Refresh Microsoft lists, then choose the To Do and Grocery destinations you want to mirror.' "$TMP/out"

python3 - "$SETTINGS_FILE" <<'PY'
import json, pathlib, sys
settings=json.loads(pathlib.Path(sys.argv[1]).read_text())
todo=settings['todo']
assert 'enabled' not in todo
assert todo['source'] == 'local'
assert todo['syncMode'] == 'microsoft'
assert todo['clientId'] == '12345678-1234-1234-1234-1234567890ab'
assert todo['map'] == {'todo':'local-todo','grocery':'local-grocery'}
PY
[ "$(wc -l < "$AZ_LOG")" -eq 2 ] || { echo 'FAIL: expected temporary Azure profile for login/create' >&2; exit 1; }
AZ_DIR="$(head -n1 "$AZ_LOG")"
[ -n "$AZ_DIR" ] && [ ! -e "$AZ_DIR" ] || { echo 'FAIL: temporary Azure CLI profile was retained' >&2; exit 1; }
printf 'PASS: advanced App Setup creates a private public-client registration through a temporary Azure CLI profile and retains local Lists as source of truth\n'
