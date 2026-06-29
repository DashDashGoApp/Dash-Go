#!/usr/bin/env bash
# Static, release-blocking contracts for installer flow safety. The full
# selected-track installer smoke exercises update behavior separately.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
INSTALLER="${1:-$ROOT/../installer/install.sh}"
[ -f "$INSTALLER" ] || { echo "installer not found: $INSTALLER" >&2; exit 1; }
bash -n "$INSTALLER"
if grep -Eq 'from[[:space:]]+server[[:space:]]+import[[:space:]]+message_sources|sys\.path\.insert\(0, os\.environ\[.DASH.\]\)' "$INSTALLER"; then
  echo 'FAIL: installer retains deleted Python message-source helper' >&2; exit 1
fi
require(){ grep -Fq -- "$1" "$INSTALLER" || { echo "missing installer contract: $1" >&2; exit 1; }; }
require '--message-sources --list'
require '--message-sources --set'
require '--update-message-feeds'
require '22) Uninstall Dash-Go'
require '12) Microsoft To Do / Graph local Lists, client ID, and Azure CLI app setup'
if grep -Eq '[[:space:]][0-9]+[[:alpha:]]\)' "$INSTALLER"; then
  echo 'FAIL: installer retains a letter-suffixed main-menu choice' >&2; exit 1
fi
require 'run_remove_install'
require 'INSTALL_STEP_FAILURES'
require 'architecture-selector shell wrapper'
require 'manifest_target_bin'
require 'release_server_for_host'
require 'dashboard_api_ready_for_version'
require 'dash-go-update.service'
require 'ensure_dashboard_update_service'
require 'acquire_dashboard_update_lock'
require 'rollback_release_transaction'
require 'restart_dashboard_server_for_update'
require 'run_post_update_verifier'
if grep -Fq 'systemd-run --user' "$INSTALLER"; then echo 'FAIL: installer still relies on per-user systemd-run for dashboard updates' >&2; exit 1; fi
echo 'PASS: installer has numeric top-level choices, Go message-source flow, architecture-safe staging, dedicated updater provisioning, transactional rollback, and runtime readiness gates'
