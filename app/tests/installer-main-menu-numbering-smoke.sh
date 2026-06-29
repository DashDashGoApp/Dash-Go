#!/usr/bin/env bash
# The visible top-level installer menu is intentionally all numeric. Microsoft
# To Do / Graph setup must stay explicit so the Azure CLI app-registration path
# cannot be hidden behind a lettered suffix again.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
INSTALL="$ROOT/../installer/install.sh"
[ -f "$INSTALL" ] || { echo "FAIL: installer missing: $INSTALL" >&2; exit 1; }
menu="$(awk '/echo "  INSTALL & UPDATE"/{capture=1} capture{print} /read -rp "  Choose \[1-25; Enter=/{exit}' "$INSTALL")"
[ -n "$menu" ] || { echo 'FAIL: top-level installer menu was not found' >&2; exit 1; }
for required in \
  '1) Full install' \
  '7) Weather radar' \
  '12) Microsoft To Do / Graph local Lists, client ID, and Azure CLI app setup' \
  '22) Uninstall Dash-Go' \
  '23) Exit installer' \
  '24) Notifications (Apprise-Go) configure private outbound delivery routes' \
  '25) Terminal access         toggle Dashboard Control Terminal card'; do
  printf '%s\n' "$menu" | grep -Fq -- "$required" || { echo "FAIL: missing numeric menu entry: $required" >&2; exit 1; }
done
mapfile -t shown_menu_numbers < <(printf '%s\n' "$menu" | sed -nE 's/^echo \"[[:space:]]*([0-9]+)\).*/\1/p')
expected_menu_numbers=()
for number in $(seq 1 25); do expected_menu_numbers+=("$number"); done
[ "${shown_menu_numbers[*]}" = "${expected_menu_numbers[*]}" ] || { echo "FAIL: top-level installer menu is not in strict numeric order: ${shown_menu_numbers[*]}" >&2; exit 1; }
printf '%s\n' "$menu" | grep -Fq '  MORE' || { echo 'FAIL: options 24/25 must appear in the More group after option 23' >&2; exit 1; }
if printf '%s\n' "$menu" | grep -Eq '[[:space:]][0-9]+[[:alpha:]]\)'; then
  echo 'FAIL: top-level installer menu contains a letter-suffixed selection' >&2
  exit 1
fi
if printf '%s\n' "$menu" | grep -Eq '6[bB]|10[bB]'; then
  echo 'FAIL: legacy lettered selection remains visible in top-level installer menu' >&2
  exit 1
fi
grep -Fq '  7) DO_RADAR=1;;' "$INSTALL" || { echo 'FAIL: numeric option 7 does not dispatch radar setup' >&2; exit 1; }
grep -Fq ' 12) DO_APP_SETUP=1;;' "$INSTALL" || { echo 'FAIL: numeric option 12 does not dispatch Microsoft To Do / Graph setup' >&2; exit 1; }
grep -Fq ' 22) run_remove_install; exit $?;;' "$INSTALL" || { echo 'FAIL: numeric option 22 does not dispatch uninstall' >&2; exit 1; }
grep -Fq ' 23)' "$INSTALL" || { echo 'FAIL: numeric option 23 does not dispatch exit' >&2; exit 1; }
grep -Fq ' 24) configure_apprise_notifications; exit $?;;' "$INSTALL" || { echo 'FAIL: numeric option 24 does not dispatch Apprise-Go notifications' >&2; exit 1; }
grep -Fq ' 25) configure_terminal_access; exit $?;;' "$INSTALL" || { echo 'FAIL: numeric option 25 does not dispatch Terminal access' >&2; exit 1; }
grep -Fq 'configure_terminal_access(){' "$INSTALL" || { echo 'FAIL: Terminal access installer helper is missing' >&2; exit 1; }
grep -Fq '"$server" --terminal-access status' "$INSTALL" || { echo 'FAIL: Terminal access installer menu does not read current state' >&2; exit 1; }
grep -Fq '"$server" --terminal-access "$next"' "$INSTALL" || { echo 'FAIL: Terminal access installer menu does not apply its toggle through the server CLI' >&2; exit 1; }
printf 'PASS: top-level installer menu is fully numeric and keeps Microsoft To Do / Graph, Apprise-Go, and terminal-access toggles discoverable\n'
