#!/bin/sh
set -eu
DASH="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
BIN_DIR="$DASH/bin"; CACHE_DIR="$DASH/cache"; LOG_DIR="$DASH/logs"
STATUS_FILE="$CACHE_DIR/maintenance-status.json"; LOG_FILE="$LOG_DIR/maintenance.log"; PAUSE_FILE="$CACHE_DIR/kiosk-paused"
mkdir -p "$CACHE_DIR" "$LOG_DIR"
log(){ printf '%s %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*" | tee -a "$LOG_FILE"; }
write_status(){ "$BIN_DIR/dashboard-control-server" --write-status --kind maintenance --file "$STATUS_FILE" --state "$1" --label "${2:-maintenance}" --detail "${3:-}" --rc "${4:-}" >/dev/null 2>&1 || true; }
profile(){ sed -nE 's/.*profile[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/p' "$DASH/config/config.local.js" 2>/dev/null | head -1 | tr '[:upper:]' '[:lower:]'; }
is_low(){ case "$(profile)" in lite|zero2|low|low-power) return 0;; *) return 1;; esac; }
lowprio(){ if [ -x "$BIN_DIR/dashboard-lowprio.sh" ]; then "$BIN_DIR/dashboard-lowprio.sh" "$@"; elif command -v nice >/dev/null 2>&1; then nice -n 10 "$@"; else "$@"; fi; }
run_task(){ task="$1"; shift || true; case "$task" in
 system-update) lowprio "$BIN_DIR/dashboard-system-update.sh" "$@";;
 doctor|doctor-quick) lowprio "$BIN_DIR/doctor.sh" "$@";;
 housekeeping) lowprio "$BIN_DIR/dashboard-housekeeping.sh" "$@";;
 rebuild-event-cache|event-cache) lowprio "$BIN_DIR/dashboard-control-server" --gen-events-cache "$@" >>"$LOG_FILE" 2>&1;;
 repair) lowprio "$HOME/install.sh" --repair "$@";;
 *) echo "Unknown maintenance task: $task" >&2; return 64;;
esac; }
usage(){ echo "Usage: $0 TASK [args...]"; echo "Tasks: system-update, doctor, doctor-quick, housekeeping, rebuild-event-cache, repair"; }
[ "${1:-}" = "--help" ] && { usage; exit 0; }
[ $# -ge 1 ] || { usage; exit 64; }
task="$1"; shift || true
write_status running "$task" "starting"
log "maintenance start task=$task"
if is_low; then touch "$PAUSE_FILE" 2>/dev/null || true; fi
set +e
run_task "$task" "$@"
rc=$?
set -e
rm -f "$PAUSE_FILE" 2>/dev/null || true
if [ "$rc" -eq 0 ]; then write_status complete "$task" "complete" "$rc"; else write_status failed "$task" "failed" "$rc"; fi
log "maintenance end task=$task rc=$rc"
exit "$rc"
