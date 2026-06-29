#!/usr/bin/env bash
# Debian 13/Trixie must use the compatible Azure CLI Bookworm repository suite.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
INSTALL="$ROOT/../installer/install.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
awk '/^valid_microsoft_client_id\(\)/{take=1} /^validate_download\(\)/{take=0} take{print}' "$INSTALL" > "$TMP/app-setup-functions.sh"
warn(){ printf '%s\n' "$*" >&2; }
ok(){ :; }
say(){ :; }
# shellcheck source=/dev/null
source "$TMP/app-setup-functions.sh"
OS_CODENAME=trixie
[ "$(todo_azure_cli_repository_suite)" = "bookworm" ] || { echo "FAIL: Debian Trixie must fall back to the Azure CLI Bookworm repository suite" >&2; exit 1; }
OS_CODENAME=unknown
set +e
todo_azure_cli_repository_suite >/dev/null 2>&1
status=$?
set -e
[ "$status" -ne 0 ] || { echo "FAIL: unreviewed suites must not be accepted" >&2; exit 1; }
printf 'PASS: Debian Trixie maps to the compatible Azure CLI Bookworm apt repository suite\n'
