#!/bin/sh
set -eu
export DISPLAY="${DISPLAY:-:0}" XAUTHORITY="${XAUTHORITY:-$HOME/.Xauthority}"
DASH="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
AUTH=0; MODE=normal
case "${1:-}" in --authorized|--control-authorized) AUTH=1;; --authorized-shell) MODE=authorized-shell;; --pin-shell) MODE=pin-shell;; esac
pin_enabled(){ [ -f "$HOME/.dashboard-control.env" ] && grep -q '^DASH_CONTROL_PIN_ENABLED=1' "$HOME/.dashboard-control.env" 2>/dev/null; }
verify_pin(){ PIN_VALUE="$1" "$DASH/bin/dashboard-control-server" --pin-check >/dev/null 2>&1; }
terminal_shell(){ clear 2>/dev/null || true; echo "Dash-Go terminal"; echo "Type 'exit' or close this window to return to the fullscreen dashboard."; echo; cd "$HOME" 2>/dev/null || cd "$DASH" || true; exec sh -l; }
[ "$MODE" = authorized-shell ] && terminal_shell
if [ "$MODE" = pin-shell ]; then if pin_enabled; then clear 2>/dev/null || true; echo "Dash-Go terminal"; printf 'Dashboard Control PIN: '; stty -echo 2>/dev/null || true; read PIN; stty echo 2>/dev/null || true; echo; verify_pin "$PIN" || { echo "Wrong PIN. Closing in 3 seconds."; sleep 3; exit 1; }; fi; terminal_shell; fi
[ -e /tmp/.X11-unix/X0 ] || { echo "No X display :0 is available." >&2; exit 1; }
command -v xterm >/dev/null 2>&1 || { echo "xterm is not installed." >&2; exit 1; }
if [ "$AUTH" -eq 1 ] || ! pin_enabled; then exec xterm -T "Dash-Go Terminal" -geometry 120x36+20+20 -e "$0" --authorized-shell; fi
exec xterm -T "Dash-Go Terminal" -geometry 120x36+20+20 -e "$0" --pin-shell
