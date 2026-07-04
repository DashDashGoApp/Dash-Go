#!/usr/bin/env bash
# The top-level menu has one named source of truth. Its displayed order and
# dispatch must remain coupled so later additions cannot strand a visible item.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
INSTALL="$ROOT/../installer/install.sh"
[ -f "$INSTALL" ] || { echo "FAIL: installer missing: $INSTALL" >&2; exit 1; }
bash -n "$INSTALL"
menu="$(awk '/echo "  INSTALL & UPDATE"/{capture=1} capture{print} /read -rp "  Choose \[1-25, q=exit; Enter=/{exit}' "$INSTALL")"
[ -n "$menu" ] || { echo 'FAIL: top-level installer menu was not found' >&2; exit 1; }

expected_names=(
  OPT_FULL OPT_UPDATE OPT_UPDATE_RECONFIGURE OPT_RECONFIGURE
  OPT_WEATHER_DISPLAY OPT_WEATHER_SOURCES OPT_RADAR OPT_CALENDARS OPT_ICAL
  OPT_VDIR OPT_MESSAGES OPT_TODO OPT_THEME OPT_SEASONAL OPT_PIN OPT_SERVICE
  OPT_SSH OPT_DOCTOR OPT_TOUR OPT_DEMO OPT_CUSTOM OPT_NOTIFICATIONS
  OPT_TERMINAL OPT_REMOVE OPT_EXIT
)
mapfile -t shown_names < <(printf '%s\n' "$menu" | sed -nE 's/.*"\$(OPT_[A-Z_]+)".*/\1/p')
[ "${shown_names[*]}" = "${expected_names[*]}" ] || {
  echo "FAIL: menu identities are out of order: ${shown_names[*]}" >&2
  exit 1
}

# Display order and numeric order are intentionally identical. Remove remains
# isolated directly above Exit, so every displayed number is the dispatched one.
for name in "${expected_names[@]}"; do
  grep -Eq "^${name}=[0-9]+$" "$INSTALL" || {
    echo "FAIL: missing canonical menu identity $name" >&2
    exit 1
  }
done

for required in \
  'OPT_PIN=15' \
  'OPT_SERVICE=16' \
  'OPT_SSH=17' \
  'OPT_NOTIFICATIONS=22' \
  'OPT_TERMINAL=23' \
  'OPT_REMOVE=24' \
  'OPT_EXIT=25' \
  '  "$OPT_PIN") DO_PIN=1;;' \
  '  "$OPT_SERVICE") DO_SERVICE=1;;' \
  '  "$OPT_SSH") DO_SSH=1;;' \
  '  "$OPT_NOTIFICATIONS") configure_apprise_notifications; exit $?;;' \
  '  "$OPT_TERMINAL") configure_terminal_access; exit $?;;' \
  '  "$OPT_EXIT"|q|Q|quit|exit)' \
  'DOC_AT_END=1'; do
  grep -Fq -- "$required" "$INSTALL" || { echo "FAIL: missing menu/dispatch contract: $required" >&2; exit 1; }
done

if printf '%s\n' "$menu" | grep -Fq '  MORE'; then
  echo 'FAIL: a secondary More group may not follow the final Exit action' >&2
  exit 1
fi
if printf '%s\n' "$menu" | grep -Eq '[[:space:]][0-9]+[[:alpha:]]\)'; then
  echo 'FAIL: top-level installer menu contains a letter-suffixed selection' >&2
  exit 1
fi
dispatch="$(awk '/^case "\$MODE" in$/{capture=1} capture{print} /^esac$/{if(capture){exit}}' "$INSTALL")"
if printf '%s\n' "$dispatch" | grep -Fq 'invalid choice'; then
  echo 'FAIL: generic invalid-choice wording remains in the top-level installer flow' >&2
  exit 1
fi
grep -Fq 'configure_terminal_access(){' "$INSTALL" || { echo 'FAIL: Terminal access installer helper is missing' >&2; exit 1; }
grep -Fq '"$server" --terminal-access status' "$INSTALL" || { echo 'FAIL: Terminal access installer menu does not read current state' >&2; exit 1; }
grep -Fq '"$server" --terminal-access "$next"' "$INSTALL" || { echo 'FAIL: Terminal access installer menu does not apply its toggle through the server CLI' >&2; exit 1; }
ical_line="$(printf '%s\n' "$menu" | grep -F '"$OPT_ICAL"' || true)"
vdir_line="$(printf '%s\n' "$menu" | grep -F '"$OPT_VDIR"' || true)"
printf '%s' "$ical_line" | grep -Fq 'Read-only calendar link' || { echo 'FAIL: option 9 must clearly identify a read-only calendar address' >&2; exit 1; }
printf '%s' "$ical_line" | grep -Fq 'never change that calendar' || { echo 'FAIL: option 9 must clearly remain read-only' >&2; exit 1; }
printf '%s' "$vdir_line" | grep -Fq 'Personal calendar sync' || { echo 'FAIL: option 10 must describe personal calendar sync' >&2; exit 1; }
printf '%s' "$vdir_line" | grep -Fq 'view-only or two-way sync' || { echo 'FAIL: option 10 must state that view-only and two-way sync are available' >&2; exit 1; }
printf 'PASS: canonical top-level menu identities, clear public/private calendar routes, visible ordering, exit placement, and dispatch are synchronized\n'
