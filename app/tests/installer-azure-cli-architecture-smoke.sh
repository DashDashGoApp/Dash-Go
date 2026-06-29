#!/usr/bin/env bash
# App Setup must not add an unsupported Azure CLI apt source on 32-bit Pi armhf.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
INSTALL="$ROOT/../installer/install.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
mkdir -p "$TMP/fakebin"
cat > "$TMP/fakebin/dpkg" <<'DPKG'
#!/usr/bin/env bash
[ "${1:-}" = "--print-architecture" ] && { printf 'armhf\n'; exit 0; }
exit 64
DPKG
cat > "$TMP/fakebin/apt-get" <<'APT'
#!/usr/bin/env bash
printf 'FAIL: apt-get must not run on unsupported architecture\n' >&2
exit 99
APT
chmod +x "$TMP/fakebin"/*
awk '/^valid_microsoft_client_id\(\)/{take=1} /^validate_download\(\)/{take=0} take{print}' "$INSTALL" > "$TMP/app-setup-functions.sh"
warn(){ printf '%s\n' "$*" >&2; }
ok(){ :; }
say(){ :; }
source "$TMP/app-setup-functions.sh"
PATH="$TMP/fakebin:$PATH"; export PATH
set +e
output="$(install_azure_cli_for_todo_registration 2>&1)"; status=$?
set -e
[ "$status" -eq 2 ] || { echo "FAIL: expected safe unsupported-architecture status 2, got $status" >&2; exit 1; }
printf '%s' "$output" | grep -Fq 'armhf'
printf '%s' "$output" | grep -Fq 'client ID created on another computer'
printf 'PASS: App Setup refuses unsupported 32-bit Azure CLI package installation without invoking apt\n'
