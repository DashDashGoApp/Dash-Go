#!/usr/bin/env bash
# Runtime contract: no az -> explicit consented Azure CLI install -> private app.
# Uses fake local commands only; no Microsoft, apt, sudo, or /etc access occurs.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
INSTALL="$ROOT/../installer/install.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
mkdir -p "$TMP/fakebin" "$TMP/home" "$TMP/etc/apt/keyrings" "$TMP/etc/apt/sources.list.d"
cat > "$TMP/fakebin/sudo" <<'SUDO'
#!/usr/bin/env bash
set -euo pipefail
[ "${1:-}" = "-v" ] && exit 0
exec "$@"
SUDO
cat > "$TMP/fakebin/dpkg" <<'DPKG'
#!/usr/bin/env bash
[ "${1:-}" = "--print-architecture" ] && { printf 'arm64\n'; exit 0; }
exit 64
DPKG
cat > "$TMP/fakebin/apt-get" <<'APT'
#!/usr/bin/env bash
set -euo pipefail
printf 'apt-get %s\n' "$*" >> "$AZ_INSTALL_LOG"
case " $* " in
  *" azure-cli "*)
    cat > "$FAKEBIN/az" <<'AZ'
#!/usr/bin/env bash
set -euo pipefail
: "${AZ_LOG:?}"
printf '%s\n' "${AZURE_CONFIG_DIR:-}" >> "$AZ_LOG"
case "${1:-}:${2:-}:${3:-}" in
  version:*) printf '{"azure-cli":"fake"}\n' ;;
  login:*) exit 0 ;;
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
  *) printf 'unexpected fake az invocation: %s\n' "$*" >&2; exit 64 ;;
esac
AZ
    chmod +x "$FAKEBIN/az"
    ;;
esac
exit 0
APT
cat > "$TMP/fakebin/curl" <<'CURL'
#!/usr/bin/env bash
set -euo pipefail
out=""
while [ "$#" -gt 0 ]; do case "$1" in -o) out="$2"; shift 2;; *) shift;; esac; done
[ -n "$out" ] || exit 64
printf 'fake Microsoft signing key\n' > "$out"
CURL
cat > "$TMP/fakebin/gpg" <<'GPG'
#!/usr/bin/env bash
set -euo pipefail
cat
GPG
cat > "$TMP/fakebin/hostname" <<'HOST'
#!/usr/bin/env bash
printf 'test-kiosk\n'
HOST
chmod +x "$TMP/fakebin"/*
awk '/^valid_microsoft_client_id\(\)/{take=1} /^validate_download\(\)/{take=0} take{print}' "$INSTALL" > "$TMP/app-setup-functions.sh"
warn(){ printf 'WARN: %s\n' "$*" >&2; }
ok(){ printf 'OK: %s\n' "$*"; }
say(){ printf '== %s ==\n' "$*"; }
source "$TMP/app-setup-functions.sh"
export HOME="$TMP/home" SETTINGS_FILE="$TMP/home/settings.json" PATH="$TMP/fakebin:$PATH" SUDO="$TMP/fakebin/sudo" OS_CODENAME=trixie
export TODO_AZURE_CLI_KEYRING="$TMP/etc/apt/keyrings/dash-go-azure-cli.gpg" TODO_AZURE_CLI_SOURCE_FILE="$TMP/etc/apt/sources.list.d/dash-go-azure-cli.sources"
export AZ_INSTALL_LOG="$TMP/apt.log" AZ_LOG="$TMP/az.log" FAKEBIN="$TMP/fakebin"
printf 'y\nCREATE PRIVATE APP\n' | register_todo_private_app_with_azure_cli
python3 - "$SETTINGS_FILE" <<'PYDONE'
import json, pathlib, sys
settings=json.loads(pathlib.Path(sys.argv[1]).read_text())
todo=settings['todo']
assert 'enabled' not in todo and todo['source'] == 'local' and todo['syncMode'] == 'microsoft'
assert todo['clientId'] == '12345678-1234-1234-1234-1234567890ab'
assert todo['map'] == {'todo':'local-todo','grocery':'local-grocery'}
PYDONE
grep -Fq 'apt-get update' "$AZ_INSTALL_LOG"
grep -Fq 'apt-get install -y azure-cli' "$AZ_INSTALL_LOG"
grep -Fq 'https://packages.microsoft.com/repos/azure-cli/' "$TODO_AZURE_CLI_SOURCE_FILE"
grep -Fqx 'Suites: bookworm' "$TODO_AZURE_CLI_SOURCE_FILE"
[ -s "$TODO_AZURE_CLI_KEYRING" ]
[ "$(wc -l < "$AZ_LOG")" -eq 3 ] || { echo 'FAIL: expected az version/login/create calls' >&2; exit 1; }
AZ_DIR="$(sed -n '2p' "$AZ_LOG")"
[ -n "$AZ_DIR" ] && [ ! -e "$AZ_DIR" ] || { echo 'FAIL: temporary Azure CLI profile was retained' >&2; exit 1; }
printf 'PASS: App Setup maps Debian Trixie to the Bookworm Azure CLI source, then creates a private public-client registration while keeping local Lists write-first\n'
