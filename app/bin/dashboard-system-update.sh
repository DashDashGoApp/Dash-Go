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
SYSTEM_UPDATE_LOCK_RETRIES="${DASHGO_SYSTEM_UPDATE_LOCK_RETRIES:-12}"
SYSTEM_UPDATE_LOCK_DELAY="${DASHGO_SYSTEM_UPDATE_LOCK_DELAY:-15}"
case "$SYSTEM_UPDATE_LOCK_RETRIES" in ''|*[!0-9]*) SYSTEM_UPDATE_LOCK_RETRIES=12;; esac
case "$SYSTEM_UPDATE_LOCK_DELAY" in ''|*[!0-9]*) SYSTEM_UPDATE_LOCK_DELAY=15;; esac
[ "$SYSTEM_UPDATE_LOCK_RETRIES" -ge 1 ] || SYSTEM_UPDATE_LOCK_RETRIES=1
APT_LOCK_WAIT_EXPIRED=0
apt_lock_error(){ grep -Eqi 'Could not get lock|Unable to acquire the dpkg frontend lock|Could not open lock file' "$1"; }
run_exact_apt_with_lock_retry(){
  attempt=1
  while :; do
    tmp="$(mktemp "$CACHE_DIR/system-update-apt.XXXXXX")" || return 1
    # Keep this exact sudo invocation shape. Existing Dash-Go sudoers rules
    # authorize only apt-get update and apt-get -y upgrade, not arbitrary args.
    sudo -n /usr/bin/apt-get "$@" > "$tmp" 2>&1
    rc=$?
    cat "$tmp"
    if [ "$rc" -eq 0 ]; then rm -f "$tmp"; return 0; fi
    if apt_lock_error "$tmp"; then
      rm -f "$tmp"
      if [ "$attempt" -lt "$SYSTEM_UPDATE_LOCK_RETRIES" ]; then
        write_status running "System update" "APT is busy with background maintenance; waiting for the package lock"
        printf '%s\n' "APT is busy with background maintenance; retrying in ${SYSTEM_UPDATE_LOCK_DELAY}s (${attempt}/${SYSTEM_UPDATE_LOCK_RETRIES})."
        sleep "$SYSTEM_UPDATE_LOCK_DELAY"
        attempt=$((attempt + 1))
        continue
      fi
      APT_LOCK_WAIT_EXPIRED=1
      return 75
    fi
    rm -f "$tmp"
    return "$rc"
  done
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
  run_exact_apt_with_lock_retry update
  rc=$?
  if [ "$rc" -eq 0 ]; then
    write_status running "System update" "running apt upgrade"
    run_exact_apt_with_lock_retry -y upgrade
    rc=$?
  fi
} >> "$LOG_FILE" 2>&1
set -e
if [ "$rc" -eq 0 ]; then
  write_status complete "System update" "complete" "$rc"
elif [ "$rc" -eq 75 ] && [ "$APT_LOCK_WAIT_EXPIRED" -eq 1 ]; then
  write_status failed "System update" "APT was busy with background maintenance for too long. Try again in a few minutes." "$rc"
else
  write_status failed "System update" "failed; see logs/system-update.log" "$rc"
fi
exit "$rc"
