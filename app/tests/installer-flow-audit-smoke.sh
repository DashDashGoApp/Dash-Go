#!/usr/bin/env bash
# Regression contracts for the installer/menu audit. These assert the safety
# and recovery semantics directly, while the complete release builder runs the
# broader installer and package suite.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
INSTALL="$ROOT/../installer/install.sh"
[ -f "$INSTALL" ] || { echo "FAIL: installer missing: $INSTALL" >&2; exit 1; }
bash -n "$INSTALL"
need(){ grep -Fq -- "$2" "$1" || { echo "FAIL: missing $3" >&2; exit 1; }; }
absent(){ ! grep -Fq -- "$2" "$1" || { echo "FAIL: forbidden $3" >&2; exit 1; }; }

need "$INSTALL" '1) Keep Demo Mode and continue to the selected installer action (default)' 'safe Demo Mode continuation'
need "$INSTALL" 'Choose [1/2/3/4, Enter=1]' 'safe Demo Mode default'
need "$INSTALL" 'No Demo Mode data was changed.' 'Demo Mode invalid-input protection'
need "$INSTALL" '[ "$MODE" = "$OPT_DEMO" ] || prompt_demo_mode_reset' 'post-selection Demo Mode gate'
need "$INSTALL" 'if [ "$REMOVE_MODE" = "1" ]; then' 'Demo Mode full-removal handoff'
need "$INSTALL" 'run_remove_install' 'Demo Mode full-removal execution'
need "$INSTALL" 'run_interactive_preflight' 'deferred interactive preflight'
need "$INSTALL" 'control_pin_timeout_choice(){' 'timeout choice mapper'
need "$INSTALL" 'Choose [1-7, Enter=${default_choice}]' 'current timeout default'
need "$INSTALL" 'run_customization(){' 'customization function boundary'
need "$INSTALL" 'Settings were not written. Let' 'customization restart message'
need "$INSTALL" 'return 2' 'geocode search-again status'
need "$INSTALL" 'Search again selected.' 'geocode search-again feedback'
need "$INSTALL" 'Skipped $ONAME — no birthday date given.' 'birthday skip feedback'
need "$INSTALL" 'Enable every listed provider, including paid/trial sources? [y/N]' 'paid provider bulk confirmation'
need "$INSTALL" '4) Tomorrow.io' 'sequential weather number 4'
need "$INSTALL" '8) Xweather' 'sequential weather number 8'
need "$INSTALL" '9) OpenWeather' 'sequential weather number 9'
need "$INSTALL" '10) Google Weather' 'sequential weather number 10'
need "$INSTALL" '11) AccuWeather' 'sequential weather number 11'
need "$INSTALL" '4) [ "$sel_tomorrow" = "1" ]' 'weather map follows printed number 4'
need "$INSTALL" '9) [ "$sel_openweather" = "1" ]' 'weather map follows printed number 9'
need "$INSTALL" 'TRIPLE-TAP the moon-phase icon next to the weather' 'correct dashboard Control gesture'
need "$INSTALL" 'iCal URL setup is unavailable. Run Update the app first, then try again.' 'iCal missing-helper guidance'
need "$INSTALL" 'CalDAV/vdirsyncer setup is unavailable. Run Update the app first, then try again.' 'CalDAV missing-helper guidance'
absent "$INSTALL" 'TRIPLE-TAP the clock' 'incorrect clock gesture'
absent "$INSTALL" 'clock or the moon' 'ambiguous gesture'

if [ "$(id -u)" -ne 0 ]; then
  TMP="$(mktemp -d)"
  trap 'rm -rf "$TMP"' EXIT
  mkdir -p "$TMP/fakebin" "$TMP/home"
  cat > "$TMP/fakebin/curl" <<'CURL'
#!/usr/bin/env bash
touch "${DASHGO_CURL_WATCH:?}"
exit 1
CURL
  chmod +x "$TMP/fakebin/curl"
  set +e
  printf 'q\n' | HOME="$TMP/home" DASHGO_CURL_WATCH="$TMP/curl.called" PATH="$TMP/fakebin:$PATH" bash "$INSTALL" >"$TMP/exit.out" 2>&1
  status=$?
  set -e
  [ "$status" -eq 0 ] || { cat "$TMP/exit.out" >&2; echo "FAIL: q must exit successfully, got $status" >&2; exit 1; }
  grep -Fq 'installer closed without making changes' "$TMP/exit.out" || { cat "$TMP/exit.out" >&2; echo 'FAIL: q exit acknowledgement missing' >&2; exit 1; }
  [ ! -e "$TMP/curl.called" ] || { echo 'FAIL: exit menu choice ran the internet preflight' >&2; exit 1; }

  help_out="$(HOME="$TMP/home" bash "$INSTALL" --help)"
  [ "$(printf '%s\n' "$help_out" | grep -c '^Interactive menu:$')" -eq 1 ] || { echo 'FAIL: installer help must have one Interactive menu heading' >&2; exit 1; }
  printf '%s\n' "$help_out" | grep -Fq 'Removal commands:' || { echo 'FAIL: help lacks removal command section' >&2; exit 1; }
  printf '%s\n' "$help_out" | grep -Fq 'Type q, quit, or exit' || { echo 'FAIL: help lacks truthful main-menu exit wording' >&2; exit 1; }
else
  echo 'NOTE: interactive installer probe skipped because installer correctly refuses root execution.'
fi
printf 'PASS: installer audit safety, flow, wording, menu, and weather-picker contracts\n'
