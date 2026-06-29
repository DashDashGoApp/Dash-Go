#!/bin/bash
# Dash-Go kiosk launcher helpers. Sourced by kiosk.sh after it initializes the
# dashboard paths and process-state globals. Keep reusable launch/process work
# here so the executable kiosk entrypoint remains small and auditable.

kiosk_log(){
  mkdir -p "$LOG_DIR" 2>/dev/null || true
  printf '%s %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*" >> "$KIOSK_LOG" 2>/dev/null || true
}

cleanup_kiosk_lock(){
  if [ -d "$KIOSK_LOCK_DIR" ] && [ "$(cat "$KIOSK_LOCK_PID" 2>/dev/null || true)" = "$$" ]; then
    rm -rf "$KIOSK_LOCK_DIR" 2>/dev/null || true
  fi
}

kill_tree(){
  local pid="${1:-}" sig="${2:-TERM}" child
  [ -n "$pid" ] || return 0
  kill -0 "$pid" 2>/dev/null || return 0
  for child in $(pgrep -P "$pid" 2>/dev/null || true); do
    kill_tree "$child" "$sig"
  done
  kill "-$sig" "$pid" 2>/dev/null || true
}

wait_for_exit(){
  local pid="${1:-}" tries="${2:-5}" i
  [ -n "$pid" ] || return 0
  for i in $(seq 1 "$tries" 2>/dev/null || echo 1 2 3 4 5); do
    kill -0 "$pid" 2>/dev/null || return 0
    sleep 1
  done
  kill -0 "$pid" 2>/dev/null || return 0
  return 1
}

browser_snapshot(){
  kiosk_log "browser snapshot: $(free -m 2>/dev/null | awk '/^Mem:/{printf "mem=%s/%sMB avail=%sMB",$3,$2,$7}') $(zramctl --noheadings --output DATA,TOTAL --bytes 2>/dev/null | awk '{d+=$1;t+=$2} END{if(t>0)printf "zram=%.1f/%.1fMB",d/1024/1024,t/1024/1024}')"
  ps -eo pid,ppid,comm,rss,vsz,args --sort=-rss 2>/dev/null | grep -E 'surf|WebKit|dashboard|Xorg|openbox' | head -12 | while IFS= read -r line; do
    kiosk_log "browser process: $line"
  done
}

cleanup_stale_webkit(){
  # WebKitGTK on low-memory Pi/WebKit builds can occasionally leave a web or
  # network process around after surf exits.  Keep this user-scoped and only
  # run it between surf launches so normal active browsing is not affected.
  local uid stale="" pid
  uid="$(id -u 2>/dev/null || echo '')"
  [ -n "$uid" ] || return 0
  for pid in $(pgrep -u "$uid" -f '/WebKit(Web|Network)Process' 2>/dev/null || true); do
    [ -n "$pid" ] || continue
    stale="$stale $pid"
  done
  [ -n "$stale" ] || return 0
  kiosk_log "cleaning stale WebKit child processes:$stale"
  kill $stale >/dev/null 2>&1 || true
  sleep 1
  for pid in $stale; do
    kill -0 "$pid" 2>/dev/null && kill -KILL "$pid" >/dev/null 2>&1 || true
  done
}

stop_kiosk_now(){
  KIOSK_STOPPING=1
  trap - INT TERM HUP

  if [ -n "${LITE_DEFERRED_PID:-}" ] && kill -0 "$LITE_DEFERRED_PID" 2>/dev/null; then
    kill_tree "$LITE_DEFERRED_PID" TERM
  fi

  if [ -n "${FULLSCREEN_PID:-}" ] && kill -0 "$FULLSCREEN_PID" 2>/dev/null; then
    kill_tree "$FULLSCREEN_PID" TERM
    wait "$FULLSCREEN_PID" 2>/dev/null || true
    FULLSCREEN_PID=""
  fi

  if [ -n "${SURF_PID:-}" ] && kill -0 "$SURF_PID" 2>/dev/null; then
    kiosk_log "stopping surf pid=$SURF_PID"
    kill_tree "$SURF_PID" TERM
    wait_for_exit "$SURF_PID" 5 || kill_tree "$SURF_PID" KILL
  fi

  # Safety sweep for helpers started by this launcher. xbindkeys may orphan
  # itself under pid 1, so match the dashboard-specific config path too.
  pkill -f "xbindkeys.*dashboard-xbindkeys.conf" >/dev/null 2>&1 || true
  pkill -x unclutter >/dev/null 2>&1 || true
  for child in $(pgrep -P "$$" 2>/dev/null || true); do
    kill_tree "$child" TERM
  done

  cleanup_kiosk_lock
  exit 0
}

acquire_kiosk_lock(){
  mkdir -p "$CACHE_DIR" 2>/dev/null || true
  local oldpid oldcmd tries=0
  while ! mkdir "$KIOSK_LOCK_DIR" 2>/dev/null; do
    oldpid="$(cat "$KIOSK_LOCK_PID" 2>/dev/null || true)"
    if [ -n "$oldpid" ] && kill -0 "$oldpid" 2>/dev/null; then
      oldcmd="$(tr '\0' ' ' < "/proc/$oldpid/cmdline" 2>/dev/null || true)"
      case "$oldcmd" in
        *"$DASH/kiosk.sh"*|*"kiosk.sh"*)
          echo "Dash-Go kiosk already running as pid $oldpid; this duplicate launcher will exit."
          exit 0
          ;;
      esac
    fi
    rm -rf "$KIOSK_LOCK_DIR" 2>/dev/null || true
    tries=$((tries+1))
    [ "$tries" -gt 3 ] && { echo "Could not acquire kiosk lock; exiting duplicate launcher."; exit 0; }
    sleep 1
  done
  printf '%s\n' "$$" > "$KIOSK_LOCK_PID"
}

# Low-memory profile helper. Keep this self-contained so kiosk.sh can make
# launch-smoothing decisions even before the control server/installer is run.
dashboard_profile(){
  local cfg="$DASH/config/config.local.js" p=""
  if [ -r "$cfg" ]; then
    p="$(sed -nE 's/.*profile[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/p' "$cfg" | head -1 | tr '[:upper:]' '[:lower:]')"
  fi
  if [ -z "$p" ]; then
    case "$(tr -d '\000' </proc/device-tree/model 2>/dev/null || true)" in *"Zero"*|*"Zero 2"*) p="lite";; *) p="balanced";; esac
  fi
  printf '%s\n' "$p"
}
is_lite_profile(){
  case "$(dashboard_profile)" in lite|zero2|low|low-power) return 0;; *) return 1;; esac
}
run_lite_deferred(){
  # Stagger helper work after surf has painted. On Pi Zero 2 W, running cache
  # rebuilds while WebKit is allocating its first page can create a sharp RAM/
  # CPU dip. These background jobs are best-effort; normal cron refreshes still
  # handle them if boot-time smoothing skips or delays one.
  (
    sleep 35
    [ -x "$BIN_DIR/dashboard-control-server" ] && "$BIN_DIR/dashboard-lowprio.sh" "$BIN_DIR/dashboard-control-server" --gen-events-cache >/dev/null 2>&1 || true
    sleep 25
    [ -x "$BIN_DIR/gen-default-calendars.sh" ] && "$BIN_DIR/dashboard-lowprio.sh" "$BIN_DIR/gen-default-calendars.sh" >/dev/null 2>&1 || true
    sleep 20
    if [ -x "$BIN_DIR/update-holidays.sh" ]; then
      "$BIN_DIR/dashboard-lowprio.sh" "$BIN_DIR/update-holidays.sh" >>"$LOG_DIR/holiday-update.log" 2>&1 || true
    fi
  ) &
  LITE_DEFERRED_PID=$!
}
