#!/usr/bin/env bash
# Minimal /etc/os-release files may omit VERSION_CODENAME. Bookworm/Trixie
# must retain their normal support levels when VERSION_ID is still present.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
DOCTOR="$ROOT/bin/doctor.sh"
[ -f "$DOCTOR" ] || { echo "FAIL: doctor not found: $DOCTOR" >&2; exit 1; }
bash -n "$DOCTOR"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT INT TERM
awk '
  /^doctor_os_support\(\)/ { capture=1 }
  capture && /^check_platform_compatibility\(\)/ { exit }
  capture { print }
' "$DOCTOR" > "$TMP/doctor-os-support.sh"
[ -s "$TMP/doctor-os-support.sh" ] || { echo 'FAIL: unable to extract doctor_os_support' >&2; exit 1; }
# shellcheck disable=SC1090
source "$TMP/doctor-os-support.sh"
printf 'ID=debian\nVERSION_ID="12"\n' > "$TMP/bookworm-os-release"
printf 'ID=raspbian\nVERSION_ID="13"\n' > "$TMP/trixie-os-release"
[ "$(DOCTOR_OS_RELEASE="$TMP/bookworm-os-release" doctor_os_support | cut -f1)" = supported ] || { echo 'FAIL: codename-less Debian 12 did not remain supported' >&2; exit 1; }
[ "$(DOCTOR_OS_RELEASE="$TMP/trixie-os-release" doctor_os_support | cut -f1)" = recommended ] || { echo 'FAIL: codename-less Raspbian 13 did not remain recommended' >&2; exit 1; }
printf '%s\n' 'PASS: codename-less Bookworm/Trixie retain supported/recommended Doctor states'
