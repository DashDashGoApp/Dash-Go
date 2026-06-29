#!/usr/bin/env bash
# Debian Trixie must replace beta.48's stale Trixie Azure CLI source with the
# compatible Bookworm suite, while restoring the old source if install fails.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
INSTALL="$ROOT/../installer/install.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
mkdir -p "$TMP/fakebin" "$TMP/etc/apt/keyrings" "$TMP/etc/apt/sources.list.d"
cat > "$TMP/fakebin/sudo" <<'SUDO'
#!/usr/bin/env bash
set -euo pipefail
[ "${1:-}" = "-v" ] && exit 0
exec "$@"
SUDO
cat > "$TMP/fakebin/dpkg" <<'DPKG'
#!/usr/bin/env bash
[ "${1:-}" = "--print-architecture" ] && { printf 'amd64\n'; exit 0; }
exit 64
DPKG
cat > "$TMP/fakebin/apt-get" <<'APT'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "$AZ_INSTALL_LOG"
if [ "${1:-}" = "update" ]; then
  grep '^Suites:' "$AZ_SOURCE_FILE" >> "$AZ_SUITE_LOG"
fi
case " $* " in *" azure-cli "*) exit 42;; esac
exit 0
APT
cat > "$TMP/fakebin/curl" <<'CURL'
#!/usr/bin/env bash
set -euo pipefail
out=""
while [ "$#" -gt 0 ]; do case "$1" in -o) out="$2"; shift 2;; *) shift;; esac; done
printf 'fake Microsoft signing key\n' > "$out"
CURL
cat > "$TMP/fakebin/gpg" <<'GPG'
#!/usr/bin/env bash
set -euo pipefail
cat
GPG
chmod +x "$TMP/fakebin"/*
awk '/^valid_microsoft_client_id\(\)/{take=1} /^validate_download\(\)/{take=0} take{print}' "$INSTALL" > "$TMP/app-setup-functions.sh"
warn(){ printf '%s\n' "$*" >&2; }
ok(){ :; }
say(){ :; }
# shellcheck source=/dev/null
source "$TMP/app-setup-functions.sh"
export PATH="$TMP/fakebin:$PATH" SUDO="$TMP/fakebin/sudo" OS_CODENAME=trixie
export TODO_AZURE_CLI_KEYRING="$TMP/etc/apt/keyrings/dash-go-azure-cli.gpg"
export TODO_AZURE_CLI_SOURCE_FILE="$TMP/etc/apt/sources.list.d/dash-go-azure-cli.sources"
export AZ_INSTALL_LOG="$TMP/apt.log" AZ_SUITE_LOG="$TMP/suite.log" AZ_SOURCE_FILE="$TODO_AZURE_CLI_SOURCE_FILE"
cat > "$TODO_AZURE_CLI_SOURCE_FILE" <<'SOURCE'
Types: deb
URIs: https://packages.microsoft.com/repos/azure-cli/
Suites: trixie
Components: main
Architectures: amd64
Signed-By: /tmp/old-key.gpg
SOURCE
cp "$TODO_AZURE_CLI_SOURCE_FILE" "$TMP/before.sources"
set +e
printf 'y\n' | install_azure_cli_for_todo_registration >"$TMP/out" 2>&1
status=$?
set -e
[ "$status" -eq 1 ] || { cat "$TMP/out" >&2; echo "FAIL: expected install failure status 1, got $status" >&2; exit 1; }
grep -Fqx 'Suites: bookworm' "$AZ_SUITE_LOG" || { echo 'FAIL: apt refresh did not observe the Bookworm fallback source' >&2; exit 1; }
cmp -s "$TODO_AZURE_CLI_SOURCE_FILE" "$TMP/before.sources" || { echo 'FAIL: stale Trixie source was not restored after install failure' >&2; exit 1; }
grep -Fq 'install -y azure-cli' "$AZ_INSTALL_LOG"
printf 'PASS: Trixie Azure CLI Bookworm fallback updates stale source transactionally and restores it on package failure\n'
