#!/usr/bin/env bash
# Keep current Debian/Raspberry Pi OS policy intentional: Trixie is recommended,
# Bookworm is supported, and Bullseye-or-older never reaches fresh provisioning.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
INSTALL="$ROOT/../installer/install.sh"
SETUP="$ROOT/bin/setup-vdirsyncer.sh"
COMMON="$ROOT/bin/dashboard-common.sh"
DOCTOR="$ROOT/bin/doctor.sh"
for file in "$INSTALL" "$SETUP" "$COMMON" "$DOCTOR"; do [ -f "$file" ] || { echo "FAIL: missing $file" >&2; exit 1; }; bash -n "$file"; done
need(){ grep -Fq -- "$1" "$2" || { echo "FAIL: missing contract in $(basename "$2"): $1" >&2; exit 1; }; }
reject(){ ! grep -Eiq -- "$1" "$2" || { echo "FAIL: retired/unsafe contract remains in $(basename "$2"): $1" >&2; exit 1; }; }
need 'bookworm)' "$INSTALL"
need 'trixie)' "$INSTALL"
need 'bullseye|buster|stretch|jessie|wheezy)' "$INSTALL"
need 'OS_SUPPORT_LEVEL="unsupported"' "$INSTALL"
need 'installer_required_package_candidates(){' "$INSTALL"
need 'Required kiosk package unavailable:' "$INSTALL"
need 'install_runtime_packages(){' "$INSTALL"
need 'No autologin, service, or kiosk settings were changed.' "$INSTALL"
need 'doctor_os_support(){' "$DOCTOR"
need 'check_platform_compatibility(){' "$DOCTOR"
need 'legacy Dash-Go FKMS display override' "$DOCTOR"
need 'classify_os_support(){' "$COMMON"
need 'apt-get install -y pipx python3-venv' "$SETUP"
reject 'bullseye-backports|DASH_VDIR_APT_CODENAME' "$SETUP"
reject 'dtoverlay=vc4-fkms-v3d' "$INSTALL"
need 'KMS settings stay unchanged' "$INSTALL"
need 'Create a reversible backup before any manual display troubleshooting?' "$INSTALL"
echo 'PASS: Bookworm/Trixie policy, required-package fail-closed behavior, safe display handling, and post-Bullseye private-calendar setup hold.'
