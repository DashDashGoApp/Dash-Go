#!/usr/bin/env bash
# Source-level contract for beta.9 first-install reliability guardrails.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
INSTALLER="$ROOT/../installer/install.sh"
DOCTOR="$ROOT/bin/doctor.sh"
README="$ROOT/../README.md"
need(){ grep -Fq -- "$1" "$2" || { echo "FAIL: expected '$1' in $2" >&2; exit 1; }; }
need 'acquire_interactive_installer_lock' "$INSTALLER"
need 'Another Dash-Go setup is already running in this account.' "$INSTALLER"
need 'start_sudo_keepalive' "$INSTALLER"
need 'DPkg::Lock::Timeout' "$INSTALLER"
need 'installer_writable_probe' "$INSTALLER"
need 'bootstrap_mem_total_mb' "$INSTALLER"
need 'prompt_timezone_if_needed' "$INSTALLER"
need 'offer_reboot_after_system_changes' "$INSTALLER"
need 'Ctrl+Alt+F2' "$INSTALLER"
need 'greetd' "$INSTALLER"
need 'Wayland desktop' "$INSTALLER"
need 'a Wayland compositor is active while Dash-Go expects its X11/LightDM kiosk session' "$DOCTOR"
need '/etc/X11/default-display-manager' "$DOCTOR"
need 'Local rescue for a black or frozen screen' "$README"
need 'Ctrl`+`Alt`+`F2' "$README"
if grep -Fq 'Forgot the PIN?' "$INSTALLER" "$ROOT/bin/dashboard-terminal.sh" "$README"; then
  echo 'FAIL: beta.9 unexpectedly added a forgotten-PIN recovery route' >&2
  exit 1
fi
echo 'PASS: beta.9 fool-proofing contracts are present and Control-PIN recovery behavior is unchanged'
