#!/usr/bin/env bash
# Verifies that a verified release bundle atomically refreshes ~/install.sh and
# that a later transaction rollback restores the exact prior launcher.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
INSTALLER="${1:-${DASHGO_INSTALLER_UNDER_TEST:-$ROOT/../installer/install.sh}}"
[ -f "$INSTALLER" ] || { echo "FAIL: installer not found: $INSTALLER" >&2; exit 1; }
bash -n "$INSTALLER"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT INT TERM

# Extract the file-validation/replacement and canonical-installer helpers from
# the real installer. Supply only the no-op environment hooks they do not own.
awk '
  /^validate_download\(\)/ { copy=1 }
  copy && /^# Personal dashboard state is mutable/ { exit }
  copy { print }
' "$INSTALLER" > "$TMP/helpers.sh"
[ -s "$TMP/helpers.sh" ] || { echo 'FAIL: could not extract installer helpers' >&2; exit 1; }
# shellcheck disable=SC1090
source "$TMP/helpers.sh"

HOME="$TMP/home"
INSTALLER="$HOME/install.sh"
mkdir -p "$HOME" "$TMP/bundle" "$TMP/stage"
printf '#!/bin/bash\necho legacy\n' > "$INSTALLER"
chmod 700 "$INSTALLER"
printf '#!/bin/bash\necho verified-release\n' > "$TMP/bundle/install.sh"
chmod 755 "$TMP/bundle/install.sh"

snapshot_canonical_installer "$TMP/stage" "$TMP/bundle/install.sh"
install_canonical_installer "$TMP/bundle/install.sh"
cmp -s "$TMP/bundle/install.sh" "$INSTALLER" || { echo 'FAIL: canonical installer was not refreshed from the verified bundle' >&2; exit 1; }
[ "$(stat -c '%a' "$INSTALLER")" = 700 ] || { echo 'FAIL: canonical installer mode is not owner-only executable' >&2; exit 1; }
restore_canonical_installer "$TMP/stage"
printf '#!/bin/bash\necho legacy\n' > "$TMP/expected-legacy"
cmp -s "$TMP/expected-legacy" "$INSTALLER" || { echo 'FAIL: canonical installer rollback did not restore the prior launcher' >&2; exit 1; }

rm -f "$INSTALLER"
rm -rf "$TMP/stage"
mkdir -p "$TMP/stage"
snapshot_canonical_installer "$TMP/stage" "$TMP/bundle/install.sh"
install_canonical_installer "$TMP/bundle/install.sh"
restore_canonical_installer "$TMP/stage"
[ ! -e "$INSTALLER" ] || { echo 'FAIL: rollback did not remove a newly introduced canonical installer' >&2; exit 1; }

mkdir -p "$TMP/release/app"
printf '<!doctype html>\n' > "$TMP/release/app/index.html"
printf '#!/bin/bash\necho verified-release\n' > "$TMP/release/install.sh"
chmod 755 "$TMP/release/install.sh"
found="$(find_release_bundle_installer "$TMP/release" "$TMP/release/app")"
[ "$found" = "$TMP/release/install.sh" ] || { echo "FAIL: bundle installer resolver returned $found" >&2; exit 1; }

echo 'PASS: verified release bundles refresh the canonical installer atomically and rollback restores the exact prior launcher'
