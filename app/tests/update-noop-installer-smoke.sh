#!/usr/bin/env bash
# Ensures normal updates stay strictly monotonic and private-calendar setup has
# no unrelated weather-provider connectivity probe.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
INSTALLER="${1:-$ROOT/../installer/install.sh}"
UPDATES="$ROOT/ui/js/control-updates.js"
HEALTH="$ROOT/ui/js/control-status-health.js"
[ -f "$INSTALLER" ] && [ -f "$UPDATES" ] || { echo 'required source missing' >&2; exit 1; }
require(){ grep -Fq -- "$1" "$2" || { echo "FAIL: missing $1 in $2" >&2; exit 1; }; }
require 'release_version_relation(){' "$INSTALLER"
require 'plan_normal_update_candidate(){' "$INSTALLER"
require 'UPDATE_PLAN_NOOP=1' "$INSTALLER"
require 'record_no_update_needed(){' "$INSTALLER"
require 'DASHGO_INSTALLER_SMOKE' "$INSTALLER"
require 'if [ -z "${DASH_UPDATE_JOB_ID:-}" ]; then' "$INSTALLER"
require 'DASH_UPDATE_JOB_ID="${DASH_UPDATE_JOB_ID:-ssh-$(date +%s)-$$}"' "$INSTALLER"
if grep -Fq '[ -n "${DASH_UPDATE_JOB_ID:-}" ||' "$INSTALLER"; then
  echo 'FAIL: update job fallback must not embed an assignment in a test expression' >&2; exit 1
fi
require 'A newer selected release was confirmed' "$INSTALLER"
require 'Installed version is newer' "$INSTALLER"
require 'Dash-Go will use a secure sign-in. Choose view-only or two-way sync for each selected iCloud or CalDAV calendar.' "$INSTALLER"
require 'pre.canStart' "$UPDATES"
require 'r&&r.noUpdate===true' "$UPDATES"
require 'Installed version is newer' "$UPDATES"
require 'av.status==="installed-newer"' "$HEALTH"
update_block="$(sed -n '/if \[ "$UPDATE_MODE" = "1" \]; then/,/# --- Pre-flight checks/p' "$INSTALLER")"
plan_line="$(printf '%s\n' "$update_block" | awk '/plan_normal_update_candidate/{print NR; exit}')"
log_line="$(printf '%s\n' "$update_block" | awk '/start_update_logging/{print NR; exit}')"
[ -n "$plan_line" ] && [ -n "$log_line" ] && [ "$plan_line" -lt "$log_line" ] || { echo 'FAIL: no-op decision must precede update logging' >&2; exit 1; }
preflight_block="$(sed -n '/run_interactive_preflight(){/,/^}/p' "$INSTALLER")"
if printf '%s\n' "$preflight_block" | grep -Eq 'api\.open-meteo\.com|curl -fsSL.*open-meteo'; then
  echo 'FAIL: interactive preflight still uses Open-Meteo as an internet probe' >&2; exit 1
fi
if printf '%s\n' "$update_block" | grep -Fq 'Safety backup is ready;'; then
  echo 'FAIL: direct update still claims a Dashboard Control backup before a strict upgrade' >&2; exit 1
fi
echo 'PASS: strict no-op/no-downgrade update guard and workflow-specific private-calendar diagnostics hold'
