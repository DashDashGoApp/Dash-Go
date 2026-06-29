#!/usr/bin/env bash
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
INSTALL="$ROOT/../installer/install.sh"
SET_THEME="$ROOT/bin/set-theme.sh"
CATALOG="$ROOT/themes.list"
[ -f "$INSTALL" ] || { echo "FAIL: installer missing" >&2; exit 1; }
[ -x "$SET_THEME" ] || { echo "FAIL: set-theme missing" >&2; exit 1; }
[ -r "$CATALOG" ] || { echo "FAIL: themes.list missing" >&2; exit 1; }
bash -n "$INSTALL"; bash -n "$SET_THEME"
require(){ grep -Fq -- "$1" "$INSTALL" || { echo "FAIL: missing installer contract: $1" >&2; exit 1; }; }
reject(){ if grep -Fq -- "$1" "$INSTALL"; then echo "FAIL: retired installer text/code remains: $1" >&2; exit 1; fi; }
require 'This version is too old to update automatically. Run --repair --system first, then update.'
reject 'installed Go updater does not expose release-manifest-v1'
require 'dashboard server binary missing or not executable for $ARCH: $BIN'
reject 'dashboard Go server binary missing or not executable for $ARCH: $BIN'
require 'warn "set-theme.sh not found in $DASH. Run option 2 (Update the app) first."'
require 'warn "seasonal-themes.sh not found in $DASH. Run option 2 (Update the app) first."'
require 'warn "doctor.sh not found in $DASH. Run option 2 (Update the app) first."'
require 'exit 1;;'
reject 'invalid numeric choice'
require '*) warn "invalid choice"; exit 1;;'
reject 'read -rp "Continue? [y/N] " go'
require 'read -rp "Continue? [y/N] " proceed'
reject 'migrate-compliments.py'
[ "$(grep -Ec '^(say|warn|ok)\(\)' "$INSTALL")" -eq 3 ] || { echo "FAIL: duplicate output helpers" >&2; exit 1; }
require 'Keep current location ('
require 'Choose [1/2, Enter=current ${TEMPU}/${WINDU}]'
require 'THEME_CATALOG="$DASH/themes.list"'
reject 'THEMES_LIST='
home="$(mktemp -d)"; trap 'rm -rf "$home"' EXIT
mkdir -p "$home/dashboard/config"
cp "$CATALOG" "$home/dashboard/themes.list"
HOME="$home" "$SET_THEME" paper >/dev/null
[ -f "$home/dashboard/config/config.local.js" ] || { echo "FAIL: set-theme did not create config" >&2; exit 1; }
grep -Fq 'theme: "paper"' "$home/dashboard/config/config.local.js" || { echo "FAIL: shared catalog did not accept paper" >&2; exit 1; }
if HOME="$home" "$SET_THEME" not-a-theme >/dev/null 2>&1; then echo "FAIL: set-theme accepted unknown catalog name" >&2; exit 1; fi
printf 'PASS: installer polish, idempotent defaults, truthful failure handling, and shared theme catalog\n'
