#!/usr/bin/env bash
# Source-level contracts for the fresh-device and installed-device novice lanes.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
INSTALL="$ROOT/../installer/install.sh"
[ -f "$INSTALL" ] || { echo 'FAIL: installer missing' >&2; exit 1; }
bash -n "$INSTALL"
need(){ grep -Fq -- "$1" "$INSTALL" || { echo "FAIL: missing contract: $1" >&2; exit 1; }; }
reject(){ ! grep -Fq -- "$1" "$INSTALL" || { echo "FAIL: retired contract remains: $1" >&2; exit 1; }; }
need 'country="${label##*, }"'
need 'temp_unit="fahrenheit"; wind_unit="mph"'
need 'temp_unit="celsius"; wind_unit="kmh"'
need 'change anytime in Dashboard Control.'
need 'installer_cli --geocode --name'
need 'Choose a match [1-$((i-1)), b=back]'
need 'pick="${pick%)}"'
need 'Dashboard on this device: http://localhost:8090'
need 'From phones and other computers: currently OFF (private by default).'
need 'offer_display_config(){'
need '.dash-go.bak'
need 'If the screen is black after reboot'
need 'Update Dash-Go (recommended)'
need 'Show the full menu (updates, settings, tools)'
need 'Run these $selected_count tasks now? [Y/n]'
need 'Run these $selected_count tasks now? This includes a system update. [y/N]'
need 'Custom mode — choose the tasks you want. Type y or n; b goes back; q returns to the menu without selecting anything.'
need 'local -a answers=()'
need 'printf '\''  %2s) %-28s %s\n'\'''
need '  REMOVE'
need 'Remove Dash-Go'
need 'Choose option 2 (Update the app) from the menu'
reject 'python3 -c'
printf 'PASS: first-launch and second-launch novice contracts\n'
