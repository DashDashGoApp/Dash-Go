#!/usr/bin/env bash
# Exercises the same normalization used by both first and later installer menus.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
INSTALL="$ROOT/../installer/install.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
[ -f "$INSTALL" ] || { echo 'FAIL: installer missing' >&2; exit 1; }
bash -n "$INSTALL"
extract_function(){
  awk -v name="$1" '
    $0 ~ "^" name "\\(\\)\\{" { printing=1 }
    printing { print }
    printing && $0 == "}" { exit }
  ' "$INSTALL"
}
{
  extract_function trim_input
  printf '\n'
  extract_function normalize_menu_choice
} > "$TMP/functions.sh"
# shellcheck disable=SC1090
source "$TMP/functions.sh"
[[ "$(normalize_menu_choice '  1)  ')" == 1 ]] || { echo 'FAIL: whitespace/trailing parenthesis did not normalize to 1' >&2; exit 1; }
[[ "$(normalize_menu_choice ' zz ')" == zz ]] || { echo 'FAIL: invalid text should remain visible to validation' >&2; exit 1; }
[[ "$(normalize_menu_choice '26')" == 26 ]] || { echo 'FAIL: out-of-range number should remain visible to validation' >&2; exit 1; }
choose_menu(){
  local raw MODE=""
  while IFS= read -r raw; do
    MODE="$(normalize_menu_choice "${raw:-1}")"
    case "$MODE" in
      q|Q|quit|exit) printf 'exit\n'; return 0 ;;
      [1-9]|1[0-9]|2[0-5]) printf '%s\n' "$MODE"; return 0 ;;
      *) : ;;
    esac
  done
  return 1
}
[[ "$(printf 'zz\n26\n1)\n' | choose_menu)" == 1 ]] || { echo 'FAIL: zz, 26, then 1) did not re-prompt to a valid action' >&2; exit 1; }
grep -Fq 'Update Dash-Go (recommended)' "$INSTALL" || { echo 'FAIL: installed start lane lacks truthful Update wording' >&2; exit 1; }
grep -Fq 'Dash-Go ${installed_version:-is} already installed.' "$INSTALL" || { echo 'FAIL: installed start lane condition is missing' >&2; exit 1; }
grep -Fq 'That menu choice could not be used. Returning to the menu; nothing was changed.' "$INSTALL" || { echo 'FAIL: defensive dispatch fallback is not novice-safe' >&2; exit 1; }
! grep -Fq '*) warn "Choose a listed action or q to exit."; exit 1;;' "$INSTALL" || { echo 'FAIL: old fatal dispatch fallback remains' >&2; exit 1; }
grep -Fq 'exec bash "$0" "${INSTALLER_ORIGINAL_ARGS[@]}"' "$INSTALL" || { echo 'FAIL: interactive restart does not preserve original arguments' >&2; exit 1; }
echo 'PASS: installer menus normalize typos safely, keep the installed Update lane truthful, and never use the stale fatal dispatch path.'
