#!/bin/bash
# Dash-Go — minimal X11/Openbox session for lite and balanced profiles.
# Used by LightDM as the dashboard-openbox xsession. It avoids the full LXDE
# desktop stack and starts only a tiny window manager plus the dashboard kiosk loop.
set -u
export DISPLAY="${DISPLAY:-:0}"
export XAUTHORITY="${XAUTHORITY:-$HOME/.Xauthority}"
DASH="$(cd "$(dirname "$0")/.." 2>/dev/null && pwd)"
LOG_DIR="$DASH/logs"
mkdir -p "$LOG_DIR" 2>/dev/null || true
LOG_FILE="$LOG_DIR/openbox-session.log"
# Explicit uninstall is the one case where a normal kiosk exit must not be
# recovered. The sentinel is outside $DASH so it survives application teardown.
DASH_STATE_DIR="${XDG_STATE_HOME:-$HOME/.local/state}/dash-go"
DASH_REMOVE_SENTINEL="$DASH_STATE_DIR/remove-requested"
removal_requested(){ [ -f "$DASH_REMOVE_SENTINEL" ]; }
SESSION_REMOVE_REQUESTED=0
log(){ printf '%s %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*" >> "$LOG_FILE" 2>/dev/null || true; }

log "openbox session starting for user=$(id -un 2>/dev/null || echo unknown) display=$DISPLAY"

# Kiosk/lite profile: avoid spawning desktop helper services that are not
# useful on the touchscreen appliance and cost RAM on Pi Zero-class devices.
export NO_AT_BRIDGE=1
export GTK_MODULES=
export GIO_USE_VFS=local
export GIO_USE_VOLUME_MONITOR=unix

session_dashboard_profile(){
  local cfg="$DASH/config/config.local.js" profile=""
  if [ -r "$cfg" ]; then
    profile="$(sed -nE 's/.*profile[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/p' "$cfg" | head -1 | tr '[:upper:]' '[:lower:]')"
  fi
  if [ -z "$profile" ]; then
    case "$(tr -d '\000' </proc/device-tree/model 2>/dev/null || true)" in
      *"Zero"*|*"Zero 2"*) profile="lite" ;;
      *) profile="balanced" ;;
    esac
  fi
  printf '%s\n' "$profile"
}
session_is_lite_profile(){
  case "$(session_dashboard_profile)" in lite|zero2|low|low-power) return 0;; *) return 1;; esac
}
if session_is_lite_profile; then
  # Match kiosk.sh's Lite allocator bounds for session-owned WebKit helpers.
  export MALLOC_ARENA_MAX=2
  export MALLOC_TRIM_THRESHOLD_=131072
  export G_SLICE=always-malloc
fi

SESSION_STOPPING=0
SESSION_CLEANED=0
OB_PID=""
KIOSK_PID=""

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

stop_dashboard_children(){
  local uid
  uid="$(id -u 2>/dev/null || echo '')"
  SESSION_STOPPING=1

  if [ -n "${KIOSK_PID:-}" ] && kill -0 "$KIOSK_PID" 2>/dev/null; then
    log "stopping kiosk pid=$KIOSK_PID"
    kill_tree "$KIOSK_PID" TERM
    wait_for_exit "$KIOSK_PID" 6 || kill_tree "$KIOSK_PID" KILL
  fi

  # Safety cleanup for helpers launched by the kiosk session. xbindkeys can
  # become orphaned under pid 1, so match its dashboard-specific config file.
  pkill -f "xbindkeys.*dashboard-xbindkeys.conf" >/dev/null 2>&1 || true
  pkill -x unclutter >/dev/null 2>&1 || true

  # During an explicit uninstall the kiosk trap has already stopped its own
  # Surf child. Do not sweep every user-owned Surf/WebKit process on the way
  # out; a user may be using another local WebKit application.
  if [ "${SESSION_REMOVE_REQUESTED:-0}" != 1 ] && [ -n "$uid" ]; then
    pkill -TERM -u "$uid" -x surf >/dev/null 2>&1 || true
    pkill -TERM -u "$uid" -f '/WebKitNetworkProcess' >/dev/null 2>&1 || true
    pkill -TERM -u "$uid" -f '/WebKitWebProcess' >/dev/null 2>&1 || true
  fi
}

cleanup(){
  local rc=$?
  [ "$SESSION_CLEANED" -eq 0 ] || return "$rc"
  SESSION_CLEANED=1
  SESSION_STOPPING=1
  log "openbox session cleanup starting rc=$rc"
  stop_dashboard_children

  if [ -n "${OB_PID:-}" ] && kill -0 "$OB_PID" 2>/dev/null; then
    log "stopping openbox pid=$OB_PID"
    kill "$OB_PID" >/dev/null 2>&1 || true
    wait_for_exit "$OB_PID" 3 || kill -KILL "$OB_PID" >/dev/null 2>&1 || true
  fi

  # Stop direct child helpers if present.
  for child in $(pgrep -P "$$" 2>/dev/null || true); do
    kill_tree "$child" TERM
  done

  [ -n "${DBUS_SESSION_BUS_PID:-}" ] && kill "$DBUS_SESSION_BUS_PID" >/dev/null 2>&1 || true
  log "openbox session exiting rc=$rc"
  return "$rc"
}

request_stop(){
  log "openbox session stop requested"
  SESSION_STOPPING=1
  cleanup
  exit 0
}

trap cleanup EXIT
trap request_stop INT TERM HUP

# If LightDM did not give us a DBus session, start a small one. This helps
# WebKitGTK/Openbox avoid noisy warnings without launching a full desktop.
if [ -z "${DBUS_SESSION_BUS_ADDRESS:-}" ] && command -v dbus-launch >/dev/null 2>&1; then
  # shellcheck disable=SC2046
  eval "$(dbus-launch --sh-syntax --exit-with-session 2>/dev/null)" || true
  export DBUS_SESSION_BUS_ADDRESS DBUS_SESSION_BUS_PID
fi

# Keep any desktop-session leftovers from a previous LXDE login from confusing
# the kiosk. Do not touch avahi-daemon, networking, ssh, cron, or systemd units.
pkill -x lxpanel >/dev/null 2>&1 || true
pkill -x pcmanfm >/dev/null 2>&1 || true
pkill -x xcompmgr >/dev/null 2>&1 || true
pkill -x diodon >/dev/null 2>&1 || true

# Apply kiosk anti-lock/no-blank settings before anything visible starts.
# This prevents the minimal session from idling or locking back to LightDM.
if [ -x "$DASH/bin/dashboard-session-guard.sh" ]; then
  "$DASH/bin/dashboard-session-guard.sh" apply >/dev/null 2>&1 || true
else
  command -v xset >/dev/null 2>&1 && { xset s off; xset s noblank; xset -dpms; } >/dev/null 2>&1 || true
fi

# Blank background; avoid desktop wallpaper/file-manager work.
command -v xsetroot >/dev/null 2>&1 && xsetroot -solid black >/dev/null 2>&1 || true

# Start the smallest window manager we expect to have. Plain openbox is used
# instead of openbox-session so ~/.config/openbox/autostart is not run a second
# time; kiosk.sh launches the browser and terminal shortcut itself.
if command -v openbox >/dev/null 2>&1; then
  openbox >/dev/null 2>&1 &
  OB_PID=$!
  log "openbox started pid=$OB_PID"
else
  OB_PID=""
  log "openbox not found; surf may not fullscreen correctly"
fi

# The X session owns the appliance. A kiosk.sh exit — including a clean exit
# caused by an old updater stopping the launcher — is never a logout request
# by itself. Only LightDM/session shutdown signals set SESSION_STOPPING and
# permit the X session to end.
while [ "$SESSION_STOPPING" -eq 0 ]; do
  if removal_requested; then
    SESSION_REMOVE_REQUESTED=1
    log "uninstall requested; ending Dash-Go X session without restarting kiosk"
    break
  fi
  if [ -x "$DASH/bin/dashboard-session-guard.sh" ]; then
    "$DASH/bin/dashboard-session-guard.sh" apply >/dev/null 2>&1 || true
  fi
  "$DASH/kiosk.sh" >> "$LOG_FILE" 2>&1 &
  KIOSK_PID=$!
  wait "$KIOSK_PID"
  rc=$?
  KIOSK_PID=""
  if removal_requested; then
    SESSION_REMOVE_REQUESTED=1
    log "uninstall requested after kiosk exit; ending Dash-Go X session"
    break
  fi
  [ "$SESSION_STOPPING" -eq 0 ] || break
  if [ "$rc" -eq 0 ]; then
    log "kiosk.sh exited cleanly outside session teardown; restarting in 5s"
  else
    log "kiosk.sh exited rc=$rc; restarting in 5s to keep dashboard session alive"
  fi
  for _i in 1 2 3 4 5; do
    [ "$SESSION_STOPPING" -eq 0 ] || break
    sleep 1
  done
done

exit 0
