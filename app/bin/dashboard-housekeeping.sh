#!/bin/sh
set -eu
DASH="${DASH:-$HOME/dashboard}"
CACHE_DIR="$DASH/cache"; LOG_DIR="$DASH/logs"; STATUS="$CACHE_DIR/housekeeping-status.json"; LOG="$LOG_DIR/housekeeping.log"
mkdir -p "$CACHE_DIR" "$LOG_DIR"
log(){ printf '%s %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*" >> "$LOG" 2>/dev/null || true; }
write_status(){ "$DASH/bin/dashboard-control-server" --write-status --file "$STATUS" --state "$1" --detail "$2" >/dev/null 2>&1 || true; }
write_status running "housekeeping started"; log "start"
removed=0
for d in "$CACHE_DIR"/update-stage-* "$CACHE_DIR"/repair-stage-* "$CACHE_DIR"/tmp-*; do [ -e "$d" ] || continue; rm -rf "$d" 2>/dev/null || true; removed=$((removed+1)); done
for f in "$LOG_DIR"/*.log; do [ -f "$f" ] || continue; size=$(wc -c < "$f" 2>/dev/null || echo 0); if [ "$size" -gt 4194304 ]; then tail -c 1048576 "$f" > "$f.tmp" 2>/dev/null && mv "$f.tmp" "$f" || true; fi; done
SYSTEM_UPDATE_LOCK="$CACHE_DIR/system-update.lock"
if [ -d "$SYSTEM_UPDATE_LOCK" ]; then
  system_update_pid="$(cat "$SYSTEM_UPDATE_LOCK/pid" 2>/dev/null || true)"
  case "$system_update_pid" in
    ''|*[!0-9]*) log "preserved system update lock without a usable owner pid";;
    *) if kill -0 "$system_update_pid" 2>/dev/null; then
         log "preserved active system update lock (pid=$system_update_pid)"
       elif rm -rf "$SYSTEM_UPDATE_LOCK" 2>/dev/null; then
         removed=$((removed+1)); log "removed stale system update lock (pid=$system_update_pid)"
       fi;;
  esac
fi
# Resilience canary is folded into existing housekeeping; it is tiny and non-fatal.
[ -x "$DASH/bin/dashboard-storage-wear.sh" ] && "$DASH/bin/dashboard-storage-wear.sh" >> "$LOG" 2>&1 || true
write_status ok "housekeeping complete; removed $removed stale item(s)"; log "complete removed=$removed"
