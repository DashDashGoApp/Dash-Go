#!/usr/bin/env bash
# dash-go-update-runner.sh — dedicated updater service entry point.
# Runs in dash-go-update.service, never as a child of dashboard-server.service.
set -u
HOME="${HOME:-$(getent passwd "$(id -u)" 2>/dev/null | awk -F: '{print $6}' || true)}"
DASH="${DASH:-$HOME/dashboard}"
CACHE_DIR="$DASH/cache"
LOG_DIR="$DASH/logs"
INSTALLER="$HOME/install.sh"
SERVER="$DASH/bin/dashboard-control-server"
JOB="$CACHE_DIR/update-job.json"
ACTION_HISTORY="$CACHE_DIR/action-history.json"
LOCK="$CACHE_DIR/update.lock"
mkdir -p "$CACHE_DIR" "$LOG_DIR" 2>/dev/null || exit 1
log(){ printf '%s update-runner: %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*" >> "$LOG_DIR/update.log" 2>/dev/null || true; }
job(){
  [ -x "$SERVER" ] || return 0
  "$SERVER" --update-job --file "$JOB" "$@" >/dev/null 2>&1 || true
}
job_id(){
  [ -x "$SERVER" ] || return 0
  "$SERVER" --json-get "$JOB" id 2>/dev/null || true
}
finalize_action(){
  [ -x "$SERVER" ] || return 0
  "$SERVER" --finalize-update-action --history "$ACTION_HISTORY" --job "$JOB" >/dev/null 2>&1 || {
    log "could not finalize job-linked Recent Actions entry"
    return 1
  }
}
terminal_state(){
  local state
  [ -x "$SERVER" ] || return 1
  state="$("$SERVER" --json-get "$JOB" state 2>/dev/null || true)"
  case "$state" in success|rolledback|failed) return 0;; esac
  return 1
}
if ! command -v flock >/dev/null 2>&1; then
  job --state failed --label "Updater dependency missing" --detail "flock is required for the dedicated updater lock." --code 1 --source control
  finalize_action || true
  log "flock is missing"
  exit 1
fi
exec 9>"$LOCK"
if ! flock -n 9; then
  # Another runner owns the transaction record. Do not overwrite that live
  # job or its single Recent Actions row with this rejected duplicate launch.
  log "rejected duplicate update because lock is held"
  exit 75
fi
if [ ! -x "$INSTALLER" ]; then
  job --state failed --label "Installer missing" --detail "The canonical installer is missing from the dashboard account home directory." --code 1 --source control
  finalize_action || true
  log "installer missing: $INSTALLER"
  exit 1
fi
id="$(job_id)"
[ -n "$id" ] || id="control-$(date +%s)-$$"
job --job-id "$id" --state running --label "Running update" --detail "Dedicated updater service is downloading and validating the selected release." --source control
log "starting dedicated control update job=$id"
DASH_UPDATE_SOURCE=control DASH_UPDATE_EXTERNAL_RUNNER=1 DASH_UPDATE_LOCK_HELD=1 DASH_UPDATE_JOB_ID="$id" \
  /bin/bash "$INSTALLER" --update --latest
rc=$?
# The installer is the transaction state machine. Preserve its terminal
# success, rollback, or failure record instead of replacing it with a generic
# runner outcome after a runtime or post-update verifier result.
if ! terminal_state; then
  if [ "$rc" -eq 0 ]; then
    job --job-id "$id" --state success --label "Update complete" --detail "The dedicated updater completed successfully." --code 0 --source control
  else
    job --job-id "$id" --state failed --label "Update failed" --detail "The dedicated updater exited with code $rc. Review the update log." --code "$rc" --source control
  fi
fi
# The installer owns the final job outcome. Finalize exactly the matching
# job-linked history row after that durable record is terminal; startup/API
# reconciliation repairs the narrow replacement/restart window if this call is
# interrupted after the record write.
finalize_action || true
if [ "$rc" -eq 0 ]; then log "completed successfully"; else log "finished with exit code $rc"; fi
exit "$rc"
