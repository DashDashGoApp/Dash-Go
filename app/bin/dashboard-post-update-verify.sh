#!/usr/bin/env bash
# dashboard-post-update-verify.sh — bounded final update verifier.
# It verifies only the just-updated local runtime while the updater lock is
# intentionally held. The normal health guard is designed to skip under that
# lock and must never be used as this transaction's success criterion.
set -u
HOME="${HOME:-$(getent passwd "$(id -u)" 2>/dev/null | awk -F: '{print $6}' || true)}"
[ -n "$HOME" ] || exit 1
DASH="${DASH:-$HOME/dashboard}"
CACHE_DIR="$DASH/cache"
BIN_DIR="$DASH/bin"
MARKER="$CACHE_DIR/post-update-verify.json"
LOG="$DASH/logs/update.log"
SERVER="$BIN_DIR/dashboard-control-server"
log(){ printf '%s post-update-verify: %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*" >> "$LOG" 2>/dev/null || true; }
[ -r "$MARKER" ] || exit 0
[ -x "$SERVER" ] || { log "cannot read update marker: dashboard control server is missing"; exit 1; }
json_get(){ "$SERVER" --json-get "$1" "$2" 2>/dev/null || true; }
record(){ "$SERVER" --update-job --file "$MARKER" "$@" >/dev/null 2>&1 || return 1; }
stage="$(json_get "$MARKER" stage)"
target="$(json_get "$MARKER" target)"
previous="$(json_get "$MARKER" previousVersion)"
[ -n "$target" ] || target="$(cat "$DASH/VERSION" 2>/dev/null || true)"
runtime_ready_for_target(){
  local tmp go version
  tmp="$(mktemp "$CACHE_DIR/.post-update-ready.XXXXXX" 2>/dev/null)" || return 1
  if ! curl -fsS --max-time 3 http://127.0.0.1:8090/api/ready > "$tmp" 2>/dev/null; then rm -f "$tmp"; return 1; fi
  go="$(json_get "$tmp" goServer)"; version="$(json_get "$tmp" version)"
  rm -f "$tmp"
  [ "$go" = "true" ] && [ "$version" = "$target" ]
}
health_api_is_safe(){
  local tmp device
  tmp="$(mktemp "$CACHE_DIR/.post-update-health.XXXXXX" 2>/dev/null)" || return 1
  if ! curl -fsS --max-time 3 http://127.0.0.1:8090/api/health > "$tmp" 2>/dev/null; then rm -f "$tmp"; return 1; fi
  device="$(json_get "$tmp" device)"
  rm -f "$tmp"
  [ "$device" != "failing" ]
}
service_is_active(){
  command -v systemctl >/dev/null 2>&1 || return 1
  systemctl is-active --quiet dashboard-server.service 2>/dev/null
}
kiosk_pause_released(){ [ ! -e "$CACHE_DIR/kiosk-paused" ]; }
browser_observed(){ pgrep -u "$(id -u)" -x surf >/dev/null 2>&1; }

verify_seconds="${DASH_POST_UPDATE_VERIFY_SECONDS:-75}"
case "$verify_seconds" in
  ''|*[!0-9]*) verify_seconds=75 ;;
esac
[ "$verify_seconds" -ge 15 ] 2>/dev/null || verify_seconds=15
[ "$verify_seconds" -le 300 ] 2>/dev/null || verify_seconds=300
deadline=$(( $(date +%s) + verify_seconds ))
good=0
browser_seen=0
while [ "$(date +%s)" -lt "$deadline" ]; do
  if runtime_ready_for_target && health_api_is_safe && service_is_active && kiosk_pause_released; then
    good=1
    browser_observed && browser_seen=1
    break
  fi
  sleep 3
done
if [ "$good" = 1 ]; then
  detail="The updated local server passed bounded readiness, service, API, and kiosk-pause checks."
  if [ "$browser_seen" = 1 ]; then
    detail="$detail Dash-Go Surf was observed after the kiosk transition."
  else
    detail="$detail The kiosk browser was not observed yet; its existing loop will continue normal bounded retry."
  fi
  record --state success --label "Post-update runtime check passed" --detail "$detail" --health-checked true --rolled-back false --rollback-attempted false --rollback-succeeded false --code 0 || true
  [ -n "$stage" ] && rm -rf "$stage" 2>/dev/null || true
  log "runtime verified for ${target:-update}; retained rollback snapshot removed; browser_seen=$browser_seen"
  exit 0
fi
log "runtime verification failed for ${target:-update}; attempting one local rollback to ${previous:-previous release}"
record --state rollback-requested --label "Runtime verification failed" --detail "The updated release did not pass its bounded local runtime check; attempting one local rollback." --health-checked true --rolled-back false --rollback-attempted true --rollback-succeeded false --code 1 || true
if [ -n "$stage" ] && [ -x "$HOME/install.sh" ]; then
  log "running local rollback stage=$stage"
  if DASH_UPDATE_ROLLBACK_ATTEMPTED=1 "$HOME/install.sh" --rollback-update "$stage" >>"$LOG" 2>&1; then
    record --state rolledback --label "Rolled back" --detail "The updated release failed bounded runtime verification and the prior release was restored and verified." --health-checked true --rolled-back true --rollback-attempted true --rollback-succeeded true --code 1 || true
    log "rollback verified for ${previous:-previous release}"
    exit 1
  fi
fi
record --state failed --label "Rollback failed" --detail "The updated release failed bounded runtime verification and the prior release could not be restored automatically. Review update.log." --health-checked true --rolled-back false --rollback-attempted true --rollback-succeeded false --code 1 || true
log "rollback failed or was unavailable"
exit 1
