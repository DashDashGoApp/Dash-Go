#!/usr/bin/env bash
# Source-level release contracts for the dedicated updater, rollback, and
# post-update health sequence. Package/Pi smoke owns real service execution.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
INSTALLER="${1:-$ROOT/../installer/install.sh}"
RUNNER="$ROOT/bin/dashboard-update-runner.sh"
VERIFIER="$ROOT/bin/dashboard-post-update-verify.sh"
UPDATE_GO="$ROOT/cmd/dashboard-control-server/dashboard_update.go"
UPDATE_RECOVERY_GO="$ROOT/cmd/dashboard-control-server/dashboard_update_recovery.go"
MANIFEST_CLI_GO="$ROOT/cmd/dashboard-control-server/release_manifest_cli.go"
PURGE_CLI_GO="$ROOT/cmd/dashboard-control-server/release_stale_purge_cli.go"
RECORDS_CLI_GO="$ROOT/cmd/dashboard-control-server/update_records_cli.go"
ACTION_HISTORY_GO="$ROOT/cmd/dashboard-control-server/action_history.go"
CAPABILITIES_CLI_GO="$ROOT/cmd/dashboard-control-server/updater_capabilities_cli.go"
BRIDGE_SMOKE="$ROOT/tests/updater-capability-bridge-smoke.sh"
[ -f "$INSTALLER" ] || { echo "installer not found: $INSTALLER" >&2; exit 1; }
[ -x "$RUNNER" ] || { echo "updater runner missing or not executable" >&2; exit 1; }
[ -x "$VERIFIER" ] || { echo "post-update verifier missing or not executable" >&2; exit 1; }
bash -n "$INSTALLER" "$RUNNER" "$VERIFIER"
require_inst(){ grep -Fq -- "$1" "$INSTALLER" || { echo "FAIL: missing installer update contract: $1" >&2; exit 1; }; }
require_go(){ grep -Fq -- "$1" "$UPDATE_GO" || { echo "FAIL: missing Go update contract: $1" >&2; exit 1; }; }
require_recovery_go(){ grep -Fq -- "$1" "$UPDATE_RECOVERY_GO" || { echo "FAIL: missing Go update-recovery contract: $1" >&2; exit 1; }; }
require_manifest_cli(){ grep -Fq -- "$1" "$MANIFEST_CLI_GO" || { echo "FAIL: missing release-manifest CLI contract: $1" >&2; exit 1; }; }
require_purge_cli(){ grep -Fq -- "$1" "$PURGE_CLI_GO" || { echo "FAIL: missing stale-cleanup CLI contract: $1" >&2; exit 1; }; }
require_records_cli(){ grep -Fq -- "$1" "$RECORDS_CLI_GO" || { echo "FAIL: missing update-record CLI contract: $1" >&2; exit 1; }; }
require_action_history(){ grep -Fq -- "$1" "$ACTION_HISTORY_GO" || { echo "FAIL: missing update-action history contract: $1" >&2; exit 1; }; }
require_inst 'dash-go-update.service'
require_inst 'ensure_dashboard_update_service'
require_inst 'DASH_UPDATE_LOCK_HELD=1'
require_inst 'acquire_dashboard_update_lock'
require_inst 'rollback_release_transaction'
require_inst 'preserve_failed_update_transaction'
require_inst 'run_post_update_verifier'
require_inst 'pause_kiosk_for_runtime_transition'
require_inst 'restart_dashboard_server_for_update'
require_inst 'resume_kiosk_after_runtime_transition'
require_inst 'restart_kiosk'
require_inst 'release_server_for_host'
require_go 'func (a *app) updatePreflight()'
require_go 'catalogReady'
require_go 'createConfigBackup("pre-update"'
require_go 'dash-go-update.service'
require_go 'errDashboardUpdateRunning'
require_go 'context.WithTimeout'
grep -Fq 'updateStatusFresh' "$ROOT/cmd/dashboard-control-server/update_status.go" || { echo 'FAIL: update status lacks an explicit fresh catalog-check path' >&2; exit 1; }
grep -Fq 'r.URL.Query().Get("fresh")' "$ROOT/cmd/dashboard-control-server/http_routes_get.go" || { echo 'FAIL: update status route cannot request a fresh catalog check' >&2; exit 1; }
require_go 'executableRegularFile'
[ -f "$MANIFEST_CLI_GO" ] && [ -f "$PURGE_CLI_GO" ] && [ -f "$RECORDS_CLI_GO" ] && [ -f "$ACTION_HISTORY_GO" ] && [ -f "$CAPABILITIES_CLI_GO" ] || { echo 'FAIL: split update CLI source file is missing' >&2; exit 1; }
[ -x "$BRIDGE_SMOKE" ] || { echo 'FAIL: updater capability bridge smoke is missing' >&2; exit 1; }
[ ! -e "$ROOT/cmd/dashboard-control-server/release_update_cli.go" ] || { echo 'FAIL: retired update CLI monolith still exists' >&2; exit 1; }
require_manifest_cli 'runVerifyReleaseManifestCLI'
require_manifest_cli 'runReleaseFileListCLI'
require_manifest_cli 'maxReleaseManifestBytes'
require_manifest_cli 'io.LimitReader'
require_purge_cli 'runPurgeStaleManagedCLI'
require_records_cli 'writeJSONPrivateFile'
require_records_cli 'runUpdateRecordCLI'
require_action_history 'recordUpdateAction'
require_action_history 'finalizeUpdateActionHistoryFile'
require_action_history 'reconcileUpdateActionHistoryFile'
if grep -Fq 'legacyUpdateOutcomeUnknownMsg' "$ACTION_HISTORY_GO"; then
  echo 'FAIL: action-history reconciliation must not text-match unsupported legacy update rows' >&2; exit 1
fi
require_action_history 'updateJobId'
grep -Fq 'runFinalizeUpdateActionCLI' "$ROOT/cmd/dashboard-control-server/main.go" || { echo 'FAIL: updater finalizer CLI is not dispatched by the server' >&2; exit 1; }
grep -Fq 'runUpdaterCapabilitiesCLI' "$CAPABILITIES_CLI_GO" || { echo 'FAIL: updater capability CLI is missing' >&2; exit 1; }
grep -Fq 'update-action-history-v1' "$CAPABILITIES_CLI_GO" || { echo 'FAIL: updater does not advertise update-history finalization' >&2; exit 1; }
grep -Fq 'runWriteUpdaterMigrationCLI' "$CAPABILITIES_CLI_GO" || { echo 'FAIL: updater migration receipt writer is missing' >&2; exit 1; }
grep -Fq 'LoadState' "$UPDATE_GO" || { echo 'FAIL: updater preflight does not distinguish a missing systemd unit' >&2; exit 1; }
if grep -Fq 'systemd-run --user' "$INSTALLER" || grep -Fq 'relaunch_update_outside_dashboard_service' "$INSTALLER"; then
  echo 'FAIL: updater still depends on a per-user systemd handoff' >&2; exit 1
fi
if grep -Fq 'python3' "$VERIFIER"; then
  echo 'FAIL: post-update verifier still requires Python' >&2; exit 1
fi
if grep -Fq 'dashboard-health-guard.sh' "$VERIFIER"; then
  echo 'FAIL: post-update verifier still depends on a guard that intentionally skips while update.lock is held' >&2; exit 1
fi
for token in 'service_is_active' 'kiosk_pause_released' 'browser_observed' 'rollback-attempted' 'rollback-succeeded'; do
  grep -Fq "$token" "$VERIFIER" || { echo "FAIL: post-update verifier lacks $token" >&2; exit 1; }
done
for token in '/usr/sbin/visudo' '$SUDO -k' 'dashboard_update_service_is_ready' 'update_exit_cleanup' 'write_updater_migration_receipt'; do
  require_inst "$token"
done
for token in 'repair_reconcile_abandoned_update_state' 'update_lock_is_held' 'Repair needs graphical follow-up' 'Confirming kiosk browser recovery'; do
  require_inst "$token"
done
if ! grep -Fq 'DASH_POST_UPDATE_VERIFY_SECONDS' "$VERIFIER" || ! grep -Fq '[ "$verify_seconds" -le 300 ]' "$VERIFIER"; then
  echo 'FAIL: post-update verifier lacks a bounded deadline' >&2; exit 1
fi
if ! grep -Fq 'flock -n 9' "$RUNNER"; then
  echo 'FAIL: dedicated updater lacks a cross-entrypoint lock' >&2; exit 1
fi
for token in 'ACTION_HISTORY=' 'finalize_action' '--finalize-update-action' 'job-linked history row'; do
  grep -Fq -- "$token" "$RUNNER" || { echo "FAIL: dedicated updater lacks update-history finalization token $token" >&2; exit 1; }
done
if sed -n '/if ! flock -n 9; then/,/^fi/p' "$RUNNER" | grep -Fq 'job --state failed'; then
  echo 'FAIL: rejected duplicate updater runner must not overwrite the active job record' >&2; exit 1
fi
installer_duplicate_block="$(sed -n '/if ! acquire_dashboard_update_lock; then/,/^  fi/p' "$INSTALLER")"
if printf '%s\n' "$installer_duplicate_block" | grep -Eq 'write_update_(status|job)'; then
  echo 'FAIL: rejected duplicate installer update must not overwrite the active transaction records' >&2; exit 1
fi
update_block="$(sed -n '/if \[ "$UPDATE_MODE" = "1" \]; then/,/# --- Pre-flight checks/p' "$INSTALLER")"
for token in 'acquire_dashboard_update_lock' 'pause_kiosk_for_runtime_transition' 'restart_dashboard_server_for_update' 'resume_kiosk_after_runtime_transition' 'restart_kiosk' 'run_post_update_verifier'; do
  printf '%s\n' "$update_block" | grep -Fq "$token" || { echo "FAIL: update path lacks $token" >&2; exit 1; }
done
pause_at="$(printf '%s\n' "$update_block" | grep -n -m1 'pause_kiosk_for_runtime_transition' | cut -d: -f1)"
restart_at="$(printf '%s\n' "$update_block" | grep -n -m1 'restart_dashboard_server_for_update' | cut -d: -f1)"
resume_at="$(printf '%s\n' "$update_block" | grep -n -m1 'resume_kiosk_after_runtime_transition' | cut -d: -f1)"
surf_at="$(printf '%s\n' "$update_block" | grep -n -m1 'if ! restart_kiosk' | cut -d: -f1)"
verify_at="$(printf '%s\n' "$update_block" | grep -n -m1 'if ! run_post_update_verifier' | cut -d: -f1)"
[ "$pause_at" -lt "$restart_at" ] && [ "$restart_at" -lt "$resume_at" ] && [ "$resume_at" -lt "$surf_at" ] && [ "$surf_at" -lt "$verify_at" ] || {
  echo 'FAIL: updater must pause -> verify server -> release -> recycle -> verify health in order' >&2; exit 1
}
bash "$BRIDGE_SMOKE" "$INSTALLER"
echo 'PASS: dedicated updater launch, Go capability bridge, fail-closed rollback, and bounded final runtime verification contracts are present'
