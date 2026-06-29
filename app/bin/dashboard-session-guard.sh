#!/bin/sh
set -eu
DASH="${DASH:-$HOME/dashboard}"
CACHE_DIR="$DASH/cache"; LOG_DIR="$DASH/logs"
mkdir -p "$CACHE_DIR" "$LOG_DIR" 2>/dev/null || true
LOG_FILE="$LOG_DIR/session-guard.log"
DISPLAY="${DISPLAY:-:0}"; XAUTHORITY="${XAUTHORITY:-$HOME/.Xauthority}"; export DISPLAY XAUTHORITY
log(){ printf '%s %s\n' "$(date '+%Y-%m-%d %H:%M:%S' 2>/dev/null || date)" "$*" >> "$LOG_FILE" 2>/dev/null || true; }
write_status(){ "$DASH/bin/dashboard-control-server" --write-status --kind session-guard --file "$CACHE_DIR/session-guard-status.json" --state "$1" --detail "$2" >/dev/null 2>&1 || true; }
hide_autostart_lockers(){ dir="$HOME/.config/autostart"; mkdir -p "$dir" 2>/dev/null || return 0; for app in light-locker xscreensaver xautolock xss-lock gnome-screensaver mate-screensaver cinnamon-screensaver lxlock; do cat > "$dir/$app.desktop" <<EOFHIDE 2>/dev/null || true
[Desktop Entry]
Type=Application
Name=$app
Hidden=true
X-GNOME-Autostart-enabled=false
EOFHIDE
done; }
kill_lockers(){ for p in light-locker xscreensaver xautolock xss-lock gnome-screensaver mate-screensaver cinnamon-screensaver lxlock; do pkill -x "$p" >/dev/null 2>&1 || true; done; }
apply_x11_no_blank(){ if command -v xset >/dev/null 2>&1 && [ -e /tmp/.X11-unix/X0 ]; then xset s off >/dev/null 2>&1 || true; xset s noblank >/dev/null 2>&1 || true; xset -dpms >/dev/null 2>&1 || true; log "applied X11 no-blank/no-DPMS settings on $DISPLAY"; else log "X11 no-blank skipped"; fi; }
case "${1:-apply}" in
 apply|--apply) hide_autostart_lockers; kill_lockers; apply_x11_no_blank; write_status ok "session guard applied";;
 status|--status) cat "$CACHE_DIR/session-guard-status.json" 2>/dev/null || echo '{}';;
 *) echo "Usage: $0 [apply|status]" >&2; exit 64;;
esac
