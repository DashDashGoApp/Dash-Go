#!/bin/bash
# =====================================================================
#  set-theme.sh — change the dashboard color theme without reinstalling.
#  Rewrites the `theme:` line in config.local.js. The dashboard checks that
#  file every minute and applies changes LIVE — no browser restart, no
#  flicker. (Applying from the on-screen control overlay is instant.)
#  Usage:  set-theme.sh            (interactive picker)
#          set-theme.sh ocean      (set directly)
# =====================================================================
set -u
DASH="$HOME/dashboard"
CONFIG_DIR="$DASH/config"
mkdir -p "$CONFIG_DIR"
CFG="$CONFIG_DIR/config.local.js"
say(){ printf '\n\033[1;36m== %s\033[0m\n' "$*"; }
warn(){ printf '\033[1;33m!! %s\033[0m\n' "$*"; }
ok(){ printf '\033[1;32m   %s\033[0m\n' "$*"; }

# The shared themes.list file is the selectable-theme source of truth.
# Browser metadata owns labels/palettes; default and dark remain aliases of basic.
THEME_CATALOG="$DASH/themes.list"
if [ ! -r "$THEME_CATALOG" ]; then
  warn "theme catalog not found: $THEME_CATALOG"
  exit 1
fi
mapfile -t THEME_OPTIONS < <(sed -e 's/[[:space:]]*#.*$//' -e '/^[[:space:]]*$/d' "$THEME_CATALOG")
[ "${#THEME_OPTIONS[@]}" -gt 0 ] || { warn "theme catalog is empty: $THEME_CATALOG"; exit 1; }
for theme_name in "${THEME_OPTIONS[@]}"; do
  [[ "$theme_name" =~ ^[a-z][a-z0-9]*$ ]] || { warn "invalid theme catalog entry: $theme_name"; exit 1; }
done
VALID="${THEME_OPTIONS[*]} dark default"
THEME="${1:-}"
# Old names keep working: "default" (renamed to basic) and "dark" are aliases.
# Normalize BEFORE the no-op compare and the config write so config.local.js
# always carries the canonical name going forward.
case "$THEME" in default|dark) THEME="basic";; esac
if [ -z "$THEME" ]; then
  say "Choose a theme"
  i=1
  for t in "${THEME_OPTIONS[@]}"; do
    printf '  %2d) %-13s' "$i" "$t"
    [ $((i % 4)) -eq 0 ] && echo
    i=$((i+1))
  done
  N=$((i-1))
  [ $(( N % 4 )) -ne 0 ] && echo
  read -rp "  Choose [1-$N]: " n
  printf '%s' "$n" | grep -qE '^[0-9]+$' || { warn "invalid choice"; exit 1; }
  THEME="${THEME_OPTIONS[$((n-1))]:-}"
  [ -z "$THEME" ] && { warn "invalid choice (1-$N)"; exit 1; }
fi
# validate
echo " $VALID " | grep -q " $THEME " || { warn "unknown theme: $THEME (valid: $VALID)"; exit 1; }

# If the requested theme is already active, do nothing — in particular do NOT
# restart the kiosk. This matters because seasonal-themes.sh calls this script
# from a daily cron; without this check the kiosk would hard-restart (and lose
# its WebKit cache) every night even when the theme hasn't changed.
if [ -f "$CFG" ]; then
  CUR="$(grep -oE 'theme:[[:space:]]*"[^"]*"' "$CFG" | head -1 | sed -E 's/.*"([^"]*)".*/\1/')"
  if [ "$CUR" = "$THEME" ]; then
    ok "theme already set to: $THEME (nothing to do)"
    exit 0
  fi
fi

if [ ! -f "$CFG" ]; then
  # No config yet — create a minimal one with just the theme.
  printf 'window.DASHBOARD_LOCAL = { theme: "%s" };\n' "$THEME" > "$CFG"
  ok "created $CFG with theme: $THEME"
else
  if grep -q 'theme:' "$CFG"; then
    # Replace existing theme value.
    sed -i "s/theme:[[:space:]]*\"[^\"]*\"/theme: \"$THEME\"/" "$CFG"
  else
    # Insert a theme line after the opening brace.
    sed -i "s/window.DASHBOARD_LOCAL[[:space:]]*=[[:space:]]*{/&\n  theme: \"$THEME\",/" "$CFG"
  fi
  ok "set theme: $THEME in $CFG"
fi

# NO RESTART NEEDED: the dashboard polls config.local.js (cache-busted) at
# boot and once a minute, and applies a changed theme live. Restarting the
# browser here used to be the apply mechanism, but it was defeated by
# WebKit's heuristic HTTP cache serving a stale config — the live poll is
# immune to every cache layer by construction.
if pgrep -x surf >/dev/null 2>&1 || pgrep -f "kiosk.sh" >/dev/null 2>&1; then
  ok "theme will appear on the dashboard within a minute (no restart needed)"
  echo "   (applying from the on-screen control overlay is instant)"
else
  ok "theme set; will show when the kiosk next starts"
fi
