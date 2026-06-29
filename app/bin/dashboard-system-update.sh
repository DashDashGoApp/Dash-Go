#!/bin/sh
set -eu
DASH="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
CACHE_DIR="$DASH/cache"; LOG_DIR="$DASH/logs"; STATUS_FILE="$CACHE_DIR/system-update-status.json"; LOG_FILE="$LOG_DIR/system-update.log"; LOCK_DIR="$CACHE_DIR/system-update.lock"
mkdir -p "$CACHE_DIR" "$LOG_DIR"
write_status(){ "$DASH/bin/dashboard-control-server" --write-status --file "$STATUS_FILE" --state "$1" --label "${2:-System update}" --detail "${3:-}" --rc "${4:-}" --command-pid "$$" >/dev/null 2>&1 || true; }
pid_is_running(){ case "${1:-}" in ''|*[!0-9]*) return 1;; esac; kill -0 "$1" 2>/dev/null; }
release_lock(){ rm -rf "$LOCK_DIR" 2>/dev/null || true; }
acquire_lock(){
  if mkdir "$LOCK_DIR" 2>/dev/null; then
    if printf '%s\n' "$$" > "$LOCK_DIR/pid" 2>/dev/null; then return 0; fi
    release_lock
    write_status failed "System update" "could not record update ownership"
    return 1
  fi
  owner="$(cat "$LOCK_DIR/pid" 2>/dev/null || true)"
  case "$owner" in
    ''|*[!0-9]*) return 1;;
    *) if ! pid_is_running "$owner"; then
         rm -rf "$LOCK_DIR" 2>/dev/null || return 1
         if mkdir "$LOCK_DIR" 2>/dev/null; then
           if printf '%s\n' "$$" > "$LOCK_DIR/pid" 2>/dev/null; then return 0; fi
           release_lock
           write_status failed "System update" "could not record update ownership"
         fi
       fi;;
  esac
  return 1
}
if ! acquire_lock; then
  write_status running "System update" "another update is already running"
  exit 75
fi
trap release_lock EXIT
write_status running "System update" "starting apt update"
set +e
{
  echo "== Dash-Go system update $(date) =="
  sudo -n /usr/bin/apt-get update
  rc=$?
  if [ "$rc" -eq 0 ]; then
    write_status running "System update" "running apt upgrade"
    sudo -n /usr/bin/apt-get -y upgrade
    rc=$?
  fi
} >> "$LOG_FILE" 2>&1
set -e
if [ "$rc" -eq 0 ]; then write_status complete "System update" "complete" "$rc"; else write_status failed "System update" "failed; see logs/system-update.log" "$rc"; fi
exit "$rc"
