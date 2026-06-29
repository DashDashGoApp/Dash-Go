#!/bin/bash
# Dash-Go kiosk launcher for LXDE/Openbox on X11.
# Launched either by the dashboard-openbox LightDM session or, on legacy/full
# desktop sessions, by one selected autostart entry. A lock below prevents
# accidental duplicate browser chains if an old autostart entry survives.

export DISPLAY=:0
export XAUTHORITY="$HOME/.Xauthority"

# Resolve the dashboard folder from this script's own location, so the same
# file works for ANY username (a hardcoded /home/<user> path silently broke
# the boot-time holiday refresh + manifest regen on machines with a
# different user — the installer downloads this file verbatim).
DASH="$(cd "$(dirname "$0")" && pwd)"
BIN_DIR="$DASH/bin"
CACHE_DIR="$DASH/cache"
LOG_DIR="$DASH/logs"
KIOSK_LOG="$LOG_DIR/kiosk.log"
PAUSE_FILE="$CACHE_DIR/kiosk-paused"
# This sentinel deliberately lives outside the application tree.  An explicit
# uninstall must stop the kiosk/session instead of letting the normal recovery
# loop relaunch Surf while $DASH is being removed.
DASH_STATE_DIR="${XDG_STATE_HOME:-$HOME/.local/state}/dash-go"
DASH_REMOVE_SENTINEL="$DASH_STATE_DIR/remove-requested"
removal_requested(){ [ -f "$DASH_REMOVE_SENTINEL" ]; }

KIOSK_LOCK_DIR="$CACHE_DIR/kiosk.lock"
KIOSK_LOCK_PID="$KIOSK_LOCK_DIR/pid"
KIOSK_STOPPING=0
SURF_PID=""
FULLSCREEN_PID=""
LITE_DEFERRED_PID=""


# The helper library is intentionally sourced after globals are initialized.
# It owns reusable process, lock, profile, and deferred-work functions only.
# shellcheck source=bin/dashboard-kiosk-lib.sh
. "$BIN_DIR/dashboard-kiosk-lib.sh"
# shellcheck source=bin/dashboard-resilience-lib.sh
[ -r "$BIN_DIR/dashboard-resilience-lib.sh" ] && . "$BIN_DIR/dashboard-resilience-lib.sh"
trap cleanup_kiosk_lock EXIT
trap stop_kiosk_now INT TERM HUP
if removal_requested; then
  kiosk_log "uninstall requested before kiosk launch; exiting without starting Surf"
  exit 0
fi
acquire_kiosk_lock
kiosk_log "kiosk session started pid=$$ display=${DISPLAY:-:0}"

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
    sleep 15
    # Refresh current-boot storage evidence early enough to clear a stale warning
    # after a clean restart, but low-priority and after the first paint window.
    [ -x "$BIN_DIR/dashboard-storage-wear.sh" ] && "$BIN_DIR/dashboard-lowprio.sh" "$BIN_DIR/dashboard-storage-wear.sh" >/dev/null 2>&1 || true
    sleep 20
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

if is_lite_profile; then
  # Pi Zero 2 W: keep the kiosk on the software compositor. Forced GPU
  # compositing can allocate GPU-backed layers for each canvas and push the
  # small VideoCore CMA split over the WebKit/Suf crash threshold.
  export NO_AT_BRIDGE=1
  export GTK_MODULES=
  export GIO_USE_VFS=local
  export GIO_USE_VOLUME_MONITOR=unix
  export MALLOC_ARENA_MAX=2
  export MALLOC_TRIM_THRESHOLD_=131072
  export G_SLICE=always-malloc
  export WEBKIT_DISABLE_COMPOSITING_MODE=1
  unset WEBKIT_FORCE_COMPOSITING_MODE
else
  # Larger-memory profiles retain the existing GPU compositing experiment.
  export WEBKIT_FORCE_COMPOSITING_MODE=1
  export WEBKIT_DISABLE_COMPOSITING_MODE=0
fi

# Disable WebKit's bwrap sandbox. Recent Debian/RPi OS restrict unprivileged
# user namespaces (enforced by AppArmor), which the WebKit sandbox needs --
# the denial makes surf crash on launch. We load only our own trusted local
# dashboard, so the sandbox provides no real benefit here. Disabling it
# sidesteps the AppArmor/userns denial entirely.
unset WEBKIT_FORCE_SANDBOX
export WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS=1

# The production dashboard uses 8090. A test-only override lets the launcher
# integration smoke test use an isolated fake local server without colliding
# with a running dashboard on the validation host.
PORT="${DASHBOARD_PORT:-8090}"
dashboard_version(){
  local dash_version
  dash_version="$(cat "$DASH/VERSION" 2>/dev/null | head -1)"
  printf '%s\n' "${dash_version:-current}"
}
dashboard_url(){
  local dash_version
  dash_version="$(dashboard_version)"
  # The local Go service intentionally binds IPv4 loopback. Do not use the
  # resolver-dependent localhost alias here: an ::1 preference can turn a
  # healthy 127.0.0.1-only service into a white Surf connection-refused page.
  printf 'http://127.0.0.1:%s/?v=%s&launch=%s' "$PORT" "$dash_version" "$(date +%s)"
}
dashboard_ready_url(){
  printf 'http://127.0.0.1:%s/api/ready' "$PORT"
}
dashboard_server_ready(){
  local expected response
  expected="$(dashboard_version)"
  response="$(curl -fsS --max-time 2 "$(dashboard_ready_url)" 2>/dev/null || true)"
  [ -n "$response" ] || return 1
  printf '%s' "$response" | grep -q '"goServer"[[:space:]]*:[[:space:]]*true' || return 1
  # Versions are constrained release identifiers. A fixed-string check avoids
  # accepting an old runtime that happens to answer during a payload swap.
  printf '%s' "$response" | grep -Eq "\"version\"[[:space:]]*:[[:space:]]*\"$expected\""
}
# Surf prefixes its X11 title with a status string (for example
# "@cgDISM:- | "), and the dashboard was renamed from Dash-Go to
# Dash-Go. Prefer the actual Surf PID so a title rename cannot break fullscreen
# again; retain both names only as a startup fallback if the window manager
# reports a different client PID while the window is mapping.
find_surf_window_id(){
  local surf_pid="${1:-}" id
  command -v wmctrl >/dev/null 2>&1 || return 1
  if [ -n "$surf_pid" ]; then
    id="$(wmctrl -lp 2>/dev/null | awk -v pid="$surf_pid" '$3 == pid {print $1; exit}')"
    if [ -n "$id" ]; then
      printf '%s\n' "$id"
      return 0
    fi
  fi
  wmctrl -l 2>/dev/null | grep -Ei 'Dash-Go' | awk '{print $1; exit}'
}

wait_if_paused(){
  while [ "$KIOSK_STOPPING" -eq 0 ] && [ -f "$PAUSE_FILE" ]; do
    removal_requested && return 1
    sleep 5
  done
  [ "$KIOSK_STOPPING" -eq 0 ] && ! removal_requested
}

sleep_unless_stopping(){
  local seconds="${1:-1}" i
  for i in $(seq 1 "$seconds" 2>/dev/null || echo 1); do
    [ "$KIOSK_STOPPING" -eq 0 ] || return 1
    sleep 1
  done
  [ "$KIOSK_STOPPING" -eq 0 ]
}

# 1. A Pi without an RTC may boot with a wildly wrong clock. Show a static
# local splash for a bounded time rather than rendering a confidently wrong
# calendar and poisoning date-sensitive caches. A permanently offline device
# always falls through and marks the uncertainty for Doctor/the dashboard.
show_static_splash(){
  local page="$1" seconds="$2" pid="" elapsed=0
  command -v surf >/dev/null 2>&1 || { sleep "$seconds"; return 0; }
  surf "file://$page" >/dev/null 2>&1 & pid=$!
  while [ "$KIOSK_STOPPING" -eq 0 ] && [ "$elapsed" -lt "$seconds" ]; do
    sleep 2; elapsed=$((elapsed+2))
    dash_clock_verified && break
  done
  [ -n "$pid" ] && kill "$pid" 2>/dev/null || true
  [ -n "$pid" ] && wait "$pid" 2>/dev/null || true
}
wait_for_initial_clock(){
  [ "${DASH_KIOSK_SKIP_TIME_GATE:-0}" = 1 ] && return 0
  dash_clock_verified && { dash_record_clock_confirmed kiosk >/dev/null 2>&1 || true; dash_clear_clock_unverified; return 0; }
  local deadline=$(( $(date +%s) + ${DASH_TIME_SYNC_WAIT_SECONDS:-75} ))
  local splash_started=0 splash_pid=""
  if command -v surf >/dev/null 2>&1; then
    surf "file://$DASH/ui/time-sync.html" >/dev/null 2>&1 & splash_pid=$!; splash_started=1
  fi
  while [ "$KIOSK_STOPPING" -eq 0 ] && [ "$(date +%s)" -lt "$deadline" ]; do
    dash_clock_verified && { dash_record_clock_confirmed kiosk >/dev/null 2>&1 || true; dash_clear_clock_unverified; break; }
    sleep 2
  done
  if ! dash_clock_verified; then dash_mark_clock_unverified; else dash_record_clock_confirmed kiosk >/dev/null 2>&1 || true; dash_clear_clock_unverified; fi
  [ "$splash_started" -eq 1 ] && kill "$splash_pid" 2>/dev/null || true
  [ "$splash_started" -eq 1 ] && wait "$splash_pid" 2>/dev/null || true
}
wait_for_local_server(){
  local deadline=$(( $(date +%s) + ${DASH_SERVER_START_WAIT_SECONDS:-75} ))
  while [ "$KIOSK_STOPPING" -eq 0 ] && [ "$(date +%s)" -lt "$deadline" ]; do
    removal_requested && return 2
    dashboard_server_ready && return 0
    sleep 2
  done
  return 1
}
run_safe_mode_cycle(){
  local reason="$1" page retry now pid=""
  dash_enter_safe_mode "$reason"
  page="$(dash_safe_mode_page_path)"
  retry="$(dash_safe_mode_retry_after)"; now="$(date +%s)"
  kiosk_log "safe-mode entered reason=$reason retry_after=$retry"
  command -v surf >/dev/null 2>&1 && surf "file://$page" >/dev/null 2>&1 & pid=$!
  while [ "$KIOSK_STOPPING" -eq 0 ] && [ "$(date +%s)" -lt "$retry" ]; do sleep 2; done
  [ -n "$pid" ] && kill "$pid" 2>/dev/null || true
  [ -n "$pid" ] && wait "$pid" 2>/dev/null || true
  dash_clear_safe_mode
  kiosk_log "safe-mode cooldown complete; attempting one clean normal start"
}

removal_requested && exit 0
wait_for_initial_clock
while ! wait_for_local_server; do
  [ "$KIOSK_STOPPING" -eq 0 ] || exit 0
  removal_requested && { kiosk_log "uninstall requested while waiting for server; exiting"; exit 0; }
  run_safe_mode_cycle "The local dashboard service did not answer after a bounded startup wait"
done

# 1b/1c. Lightweight boot-time calendar prep.
# On low-memory profiles, defer heavier helper work until WebKit has completed
# its initial page allocation/paint. This reduces the launch-time RAM cliff on
# Pi Zero 2 W class devices. Other profiles keep the traditional eager refresh.
"$BIN_DIR/gen-calendars.sh" >/dev/null 2>&1 || true
if is_lite_profile; then
  run_lite_deferred
else
  # The boot-aware Go reader suppresses stale kernel-log evidence immediately;
  # this tiny low-priority run overwrites it with current-boot evidence shortly
  # after the server is ready on larger profiles too.
  ( [ -x "$BIN_DIR/dashboard-storage-wear.sh" ] && "$BIN_DIR/dashboard-lowprio.sh" "$BIN_DIR/dashboard-storage-wear.sh" >/dev/null 2>&1 ) &
  [ -x "$BIN_DIR/update-holidays.sh" ] && "$BIN_DIR/update-holidays.sh" >>"$LOG_DIR/holiday-update.log" 2>&1 &
  [ -x "$BIN_DIR/gen-default-calendars.sh" ] && "$BIN_DIR/gen-default-calendars.sh" >/dev/null 2>&1 &
  [ -x "$BIN_DIR/dashboard-control-server" ] && "$BIN_DIR/dashboard-lowprio.sh" "$BIN_DIR/dashboard-control-server" --gen-events-cache >/dev/null 2>&1 &
fi

# 2. Keep the display awake and prevent locker/screensaver fallback.
# The session guard also suppresses user-level locker autostarts; keep the
# direct xset calls as a no-dependency fallback.
if [ -x "$BIN_DIR/dashboard-session-guard.sh" ]; then
  "$BIN_DIR/dashboard-session-guard.sh" apply >/dev/null 2>&1 || true
fi
xset s off
xset s noblank
xset -dpms

# 2b. Hide the mouse cursor after inactivity, re-hiding after each touch.
# REQUIRES unclutter-xfixes (NOT classic unclutter, which gets stuck visible
# after a touch on touchscreens):  sudo apt install unclutter-xfixes
# Key flags for touch:
#   --timeout 60   : hide after 60s idle
#   --jitter 10    : ignore pointer moves under 10px, so the phantom/residual
#                    motion a touch driver emits after you lift your finger
#                    doesn't keep resetting the idle timer (this is the fix
#                    for "cursor never re-hides after a touch")
#   --fork         : run in background cleanly
pkill -x unclutter 2>/dev/null
if command -v unclutter >/dev/null; then
  if unclutter --help 2>&1 | grep -q -- '--timeout'; then
    unclutter --timeout 60 --jitter 10 --fork           # xfixes variant (wanted)
  else
    # Classic unclutter fallback -- known to stay visible after touch; if you
    # see that, install unclutter-xfixes and remove classic unclutter.
    unclutter -idle 60 -root &
  fi
fi

# 2c. Optional global terminal shortcut for VNC, remote console, or a
# locally attached keyboard. The wrapper handles PIN prompting when launched
# outside an already-unlocked Dashboard Control session. Browser-side JS also
# catches Ctrl+Alt+T as a fallback while surf has focus.
start_terminal_shortcut(){
  [ -x "$BIN_DIR/dashboard-terminal.sh" ] || return 0
  command -v xbindkeys >/dev/null 2>&1 || return 0
  mkdir -p "$CACHE_DIR"
  local conf="$CACHE_DIR/dashboard-xbindkeys.conf"
  cat > "$conf" <<EOF
"$BIN_DIR/dashboard-terminal.sh"
  control+alt + t
EOF
  pkill -f "xbindkeys.*dashboard-xbindkeys.conf" 2>/dev/null || true
  xbindkeys -f "$conf" >/dev/null 2>&1 &
}
start_terminal_shortcut

# Force the surf window fullscreen, retrying until it actually sticks.
# At boot the window can map before Openbox is ready to honor the hint, so a
# single attempt is unreliable -- we re-issue it until wmctrl reports the
# window spanning (near-)full screen width.
fullscreen_surf() {
  local surf_pid="${1:-${SURF_PID:-}}" id geom w tries=0
  command -v wmctrl >/dev/null 2>&1 || { kiosk_log "fullscreen helper unavailable: wmctrl is not installed"; return 1; }
  while [ "$KIOSK_STOPPING" -eq 0 ] && [ $tries -lt 60 ]; do
    id="$(find_surf_window_id "$surf_pid" 2>/dev/null || true)"
    if [ -n "$id" ]; then
      wmctrl -i -r "$id" -b add,fullscreen >/dev/null 2>&1 || true
      sleep 0.7
      geom="$(wmctrl -lG 2>/dev/null | awk -v win="$id" '$1 == win {print; exit}')"
      w="$(printf '%s\n' "$geom" | awk '{print $5}')"
      if [ -n "$w" ] && [ "$w" -ge 1024 ]; then
        kiosk_log "fullscreen applied window=$id surf_pid=${surf_pid:-unknown} width=$w"
        return 0
      fi
    fi
    tries=$((tries+1))
    sleep 0.5
  done
  kiosk_log "fullscreen helper could not find/apply Surf window for pid=${surf_pid:-unknown}"
  return 1
}

# 3. Launch surf, keep asking Openbox to fullscreen it in the background,
# and immediately wait on the browser PID. Older builds ran fullscreen_surf
# in front of wait; if surf exited during the retry window, relaunch could be
# delayed long enough to look broken on a kiosk. The browser loop should be
# boring: surf exits, helper stops, log the reason, relaunch a few seconds later.
# TERM/INT/HUP means the kiosk/session is shutting down; do not treat that as
# a browser crash or relaunch surf while LightDM/Xorg are stopping.
while [ "$KIOSK_STOPPING" -eq 0 ]; do
  removal_requested && { kiosk_log "uninstall requested; leaving kiosk without launching Surf"; break; }
  wait_if_paused || break
  removal_requested && { kiosk_log "uninstall requested after pause; leaving kiosk"; break; }
  if dash_safe_mode_active; then
    run_safe_mode_cycle "A previous Dash-Go recovery attempt is cooling down"
    continue
  fi
  # Every browser launch—not just initial boot—must wait for the local Go
  # runtime to answer with this payload's version. This closes the update race
  # where Surf could reach a just-restarted port before the service was ready
  # and remain parked on a connection-refused error page.
  if ! wait_for_local_server; then
    removal_requested && break
    run_safe_mode_cycle "The local dashboard service did not answer before Surf relaunch"
    continue
  fi
  if ! dash_note_kiosk_launch; then
    run_safe_mode_cycle "The dashboard browser restarted repeatedly in a short window"
    continue
  fi
  URL="$(dashboard_url)"
  LAUNCH_STARTED="$(date +%s)"
  kiosk_log "launching surf url=$URL"
  surf "$URL" &
  SURF_PID=$!
  kiosk_log "surf launched pid=$SURF_PID"
  (
    fullscreen_surf "$SURF_PID" || true
  ) &
  FULLSCREEN_PID=$!
  kiosk_log "fullscreen helper started pid=$FULLSCREEN_PID surf_pid=$SURF_PID"
  wait "$SURF_PID"
  SURF_RC=$?
  kiosk_log "surf exited pid=$SURF_PID rc=$SURF_RC"
  LAUNCH_AGE=$(( $(date +%s) - ${LAUNCH_STARTED:-0} ))
  case "$SURF_RC" in
    0|143) : ;;
    *) kiosk_log "surf exit looked unexpected rc=$SURF_RC"; browser_snapshot ;;
  esac
  # A browser that stayed up for the whole restart window proves the loop was
  # not a crash storm; clear stale launch history to avoid false safe-mode.
  if [ "$LAUNCH_AGE" -ge "${DASH_KIOSK_RESTART_WINDOW:-120}" ] 2>/dev/null; then dash_clear_kiosk_restart_state; fi
  SURF_PID=""
  cleanup_stale_webkit
  if [ -n "${FULLSCREEN_PID:-}" ] && kill -0 "$FULLSCREEN_PID" 2>/dev/null; then
    kill_tree "$FULLSCREEN_PID" TERM
    wait "$FULLSCREEN_PID" 2>/dev/null || true
  fi
  FULLSCREEN_PID=""
  [ "$KIOSK_STOPPING" -eq 0 ] || break
  removal_requested && { kiosk_log "uninstall requested after Surf exit; not relaunching"; break; }
  kiosk_log "surf relaunching in 3s"
  sleep_unless_stopping 3 || break
  # Surf exits are recovered here. Normal updates recycle Surf only, keeping
  # this launcher and the surrounding LightDM/Openbox session alive.
done

cleanup_kiosk_lock
exit 0
