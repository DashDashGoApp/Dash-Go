#!/usr/bin/env bash
# Static contracts for the novice-safe installer path. These guards deliberately
# inspect wording and control flow rather than attempting a privileged install.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
INSTALLER="$ROOT/../installer/install.sh"
[ -f "$INSTALLER" ] || { echo "FAIL: installer missing" >&2; exit 1; }
bash -n "$INSTALLER"
require(){ grep -Fq -- "$1" "$INSTALLER" || { echo "FAIL: missing installer hardening contract: $1" >&2; exit 1; }; }
reject(){ if grep -Fq -- "$1" "$INSTALLER"; then echo "FAIL: unsafe/retired installer contract remains: $1" >&2; exit 1; fi; }

# Before any setup choice, validate the actual distribution route and the
# common Pi first-run failure modes.
require 'run_startup_preflight || exit 1'
require 'https://github.com/DashDashGoApp/Dash-Go/releases'
require 'need at least 500 MB'
require 'sudo timedatectl set-ntp true'
require 'DNS can find GitHub'
require 'Dash-Go release host is reachable'
require 'Dash-Go must be started as the normal kiosk user, not with sudo.'
require 'SUDO_USER'
require 'Fix the items marked ✗ and run ~/install.sh again. Nothing was changed.'

# Express is the first human-sized decision; it must not silently force a
# system upgrade or long customization menu.
require 'Express setup (recommended)'
require 'Show the full menu (updates, settings, tools)'
require 'EXPRESS_MODE=1'
require 'Express setup uses safe display, weather, and maintenance defaults.'
require 'run_express_customization'
require 'Only one detail is needed now: where this dashboard lives.'
require 'showSeconds: true,'
require 'weatherProviders: ["openmeteo"],'
require 'DO_PKGS=1; DO_FILES=1; DO_FONTS=1; DO_CUSTOM=1; DO_SERVICE=1; DO_AUTOLOGIN=1; DO_AUTOSTART=1; DOC_AT_END=1'

# Wrong answers and interruption need one concrete recovery action.
require 'PINs did not match. Try both entries again.'
require 'restore_candidate_looks_usable'
require 'Path to backup/preserved archive/directory [blank=skip, b=back]'
require 'That path is not a readable Dash-Go backup.'
require 'installer_interrupt'
require 'Install interrupted at ${INSTALLER_STAGE}.'
require 'installer_exit_summary'
require 'Install stopped at ${INSTALLER_STAGE}.'
require 'installer_build_stage_plan'
require 'installer_stage "Checking the finished dashboard"'

# Destructive work remains opt-in even after the uninstall command was chosen.
require 'remove_confirmation_matches'
require 'Type exactly: UNINSTALL DASH-GO'
require 'Remove Dash-Go application data and private credentials after wiring is removed? [y/N]'
require 'Type exactly: PURGE DASH-GO'
reject 'Remove Dash-Go application data and private credentials after wiring is removed? [Y/n]'

# Final output proves local readiness and never claims success after a failed
# runtime check; the local-only service must not be advertised as an open LAN URL.
require 'installer_final_dashboard_check'
require 'Dashboard is running locally: http://localhost:8090'
require 'Dash-Go did not confirm that it is running. Running Doctor now; the installer will not report success.'
require 'Dashboard Control will start automatically on boot'
require 'Next: open Dashboard Control on the kiosk'

echo 'PASS: installer has preflight, Express, safe recovery, destructive-default, progress, and truthful final-readiness contracts'
