#!/usr/bin/env bash
# Release-blocking static contract for the offline, staged Dash-Go uninstall.
# It intentionally uses no host sudo/systemd/crontab state.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
INSTALLER="${DASHGO_INSTALLER_UNDER_TEST:-$ROOT/../installer/install.sh}"
[ -f "$INSTALLER" ] || { echo 'FAIL: installer under test not found' >&2; exit 1; }
need(){ grep -Fq -- "$2" "$1" || { echo "FAIL: missing $3" >&2; exit 1; }; }
absent(){ ! grep -Fq -- "$2" "$1" || { echo "FAIL: forbidden $3" >&2; exit 1; }; }
line_of(){ grep -nF -- "$2" "$1" | head -1 | cut -d: -f1; }

bash -n "$INSTALLER"
bash -n "$ROOT/kiosk.sh"
bash -n "$ROOT/bin/dashboard-lite-session.sh"

need "$INSTALLER" 'remove_state_prepare(){' 'external uninstall state helper'
need "$INSTALLER" 'remove_make_archive(){' 'verified preservation archive helper'
need "$INSTALLER" 'remove_dashboard_cron_jobs(){' 'Dash-Go-only cron cleanup helper'
need "$INSTALLER" 'remove_request_kiosk_shutdown(){' 'targeted kiosk shutdown helper'
need "$INSTALLER" 'remove_verify(){' 'uninstall post-condition verifier'
need "$INSTALLER" 'REMOVE_DRY_RUN=0' 'uninstall dry-run flag'
need "$INSTALLER" 'REMOVE_KEEP_INSTALLER=0' 'uninstall keep-installer flag'
need "$INSTALLER" 'Offline, verified Dash-Go uninstall' 'honest uninstall scope'
need "$INSTALLER" 'dashboard-post-update' 'existing installer remains unrelated to uninstall'
need "$INSTALLER" 'calendar-source.json' 'external calendar topology metadata'
need "$INSTALLER" '.dashboard-radar.env' 'radar credential archive/removal'
need "$INSTALLER" 'dashboard-nightly-browser-restart' 'nightly browser cron removal'
need "$INSTALLER" 'seasonal-themes.sh apply' 'seasonal cron removal'
need "$INSTALLER" 'sudo -v' 'sudo preflight'
need "$INSTALLER" 'recovery archive failed; no Dash-Go files were removed' 'archive failure stops destructive stages'
need "$ROOT/kiosk.sh" 'DASH_REMOVE_SENTINEL' 'kiosk external removal sentinel'
need "$ROOT/kiosk.sh" 'leaving kiosk without launching Surf' 'kiosk no-relaunch removal path'
need "$ROOT/bin/dashboard-lite-session.sh" 'SESSION_REMOVE_REQUESTED=1' 'session removal state'
need "$ROOT/bin/dashboard-lite-session.sh" 'ending Dash-Go X session without restarting kiosk' 'session no-restart removal path'

# Extract only the uninstall section: generic browser/cache removal elsewhere in
# the installer is not an uninstall violation.
start=$(line_of "$INSTALLER" '# --- Offline, verified Dash-Go uninstall')
end=$(line_of "$INSTALLER" 'if [ "$REMOVE_MODE" = "1" ]; then run_remove_install; exit $?; fi')
[ -n "$start" ] && [ -n "$end" ] && [ "$start" -lt "$end" ] || { echo 'FAIL: cannot delimit uninstall block' >&2; exit 1; }
sed -n "${start},${end}p" "$INSTALLER" > "${TMPDIR:-/tmp}/dash-go-remove-smoke.$$"
BLOCK="${TMPDIR:-/tmp}/dash-go-remove-smoke.$$"
trap 'rm -f "$BLOCK"' EXIT
absent "$BLOCK" 'pkill -x surf' 'generic Surf kill in uninstall'
absent "$BLOCK" '.cache/webkitgtk' 'generic WebKit cache purge in uninstall'
absent "$BLOCK" '.local/share/webkit' 'generic WebKit share purge in uninstall'
need "$BLOCK" 'REMOVE_SENTINEL' 'removal sentinel use'
need "$BLOCK" 'tar -tzf' 'archive readability verification'
need "$BLOCK" 'sha256sum "$archive"' 'archive checksum record'
need "$BLOCK" 'if [ "$REMOVE_DRY_RUN" = "1" ]' 'dry-run exits before mutation'
need "$BLOCK" 'remove_require_sudo || return 1' 'sudo before destructive stages'
need "$BLOCK" 'remove_step "requested Dash-Go kiosk shutdown"' 'quiesce before teardown'

# The removal dispatch must precede all GitHub Release resolution/network work.
remove_dispatch=$(line_of "$INSTALLER" 'if [ "$REMOVE_MODE" = "1" ]; then run_remove_install; exit $?; fi')
network_resolution=$(line_of "$INSTALLER" 'if ! download_app_files "${REPAIR_TARGET:-latest}"; then')
[ -n "$remove_dispatch" ] && [ -n "$network_resolution" ] && [ "$remove_dispatch" -lt "$network_resolution" ] || { echo 'FAIL: --remove is not dispatched before GitHub Release network work' >&2; exit 1; }

echo 'PASS: offline uninstall has staged archive, cron, sentinel, targeted-process, and post-condition contracts'
