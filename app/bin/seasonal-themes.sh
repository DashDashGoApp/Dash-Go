#!/bin/bash
# =====================================================================
#  seasonal-themes.sh — make the dashboard dress up for holidays/seasons
#  automatically. It maps date ranges to themes and switches via
#  set-theme.sh. Between holidays it returns to your BASE theme.
#
#  Usage:
#    seasonal-themes.sh apply        # set the theme appropriate for today
#    seasonal-themes.sh install      # add a daily cron that calls 'apply'
#    seasonal-themes.sh uninstall    # remove the cron
#    seasonal-themes.sh base <name>  # set the between-holidays base theme
#    seasonal-themes.sh show         # print the schedule + today's pick
#
#  The schedule below is easy to edit — each line is:  MM-DD MM-DD theme
#  (start and end inclusive, by month-day; ranges may span into a month).
# =====================================================================
set -u
DASH="$HOME/dashboard"
CONFIG_DIR="$DASH/config"
SETTHEME="$DASH/bin/set-theme.sh"
BASEFILE="$HOME/.dashboard-base-theme"   # remembers the non-holiday theme
say(){ printf '\n\033[1;36m== %s\033[0m\n' "$*"; }
warn(){ printf '\033[1;33m!! %s\033[0m\n' "$*"; }
ok(){ printf '\033[1;32m   %s\033[0m\n' "$*"; }

# --- Seasonal schedule: "START_MMDD END_MMDD theme" -----------------
# Ranges are inclusive and compared by month-day. Earlier lines win if
# two ranges overlap (order matters) — that's why juneteenth (a single
# day inside the pride range) is listed BEFORE pride. Edit freely.
# Floating observances are resolved by the local dashboard-control-server from
# the enabled, already cached holiday calendars. There is no network request or
# date guess here; if no matching event is loaded, this fixed-date schedule
# remains the fallback.
SCHEDULE="
01-01 01-02 newyear
01-29 02-04 lunar
02-05 02-09 mardigras
02-12 02-15 valentine
03-15 03-18 stpatricks
03-29 04-12 easter
04-20 04-22 earthday
05-03 05-05 cincodemayo
06-19 06-19 juneteenth
06-10 06-27 pride
06-28 07-07 america
07-08 09-18 summer
09-19 10-05 oktoberfest
10-18 10-31 halloween
11-01 11-02 muertos
11-20 11-28 thanksgiving
12-10 12-26 christmas
12-29 12-31 newyear
"

# Determine today's MMDD and pick the matching theme (or base).
pick_theme(){
  local today; today="$(date +%m-%d)"
  local base; base="basic"
  [ -f "$BASEFILE" ] && base="$(cat "$BASEFILE" 2>/dev/null)"
  # Exact enabled-calendar observances outrank a fixed date range. The Go CLI
  # only reads events.cache.json and prints a catalog theme name or nothing.
  local event_theme=""
  if [ -x "$DASH/bin/dashboard-control-server" ]; then
    event_theme="$("$DASH/bin/dashboard-control-server" --seasonal-theme 2>/dev/null || true)"
  fi
  if [ -n "$event_theme" ]; then echo "$event_theme"; return; fi
  # Walk the schedule; handle ranges that wrap the year end (start>end).
  local s e th
  while read -r s e th; do
    [ -z "$th" ] && continue
    # numeric compare on MMDD (strip the dash)
    local t="${today/-/}" sn="${s/-/}" en="${e/-/}"
    if [ "$sn" -le "$en" ]; then
      # normal range within the year
      if [ "$t" -ge "$sn" ] && [ "$t" -le "$en" ]; then echo "$th"; return; fi
    else
      # wraps year-end (e.g. 12-27 .. 01-02)
      if [ "$t" -ge "$sn" ] || [ "$t" -le "$en" ]; then echo "$th"; return; fi
    fi
  done <<< "$SCHEDULE"
  echo "$base"
}

CMD="${1:-apply}"
case "$CMD" in
  apply)
    [ -x "$SETTHEME" ] || { warn "set-theme.sh not found/executable at $SETTHEME"; exit 1; }
    TH="$(pick_theme)"
    say "Seasonal theme for $(date +%m-%d): $TH"
    "$SETTHEME" "$TH"
    ;;
  base)
    NEW="${2:-}"
    [ -z "$NEW" ] && { warn "usage: seasonal-themes.sh base <theme>"; exit 1; }
    printf '%s\n' "$NEW" > "$BASEFILE"
    ok "between-holidays base theme set to: $NEW"
    echo "Run 'seasonal-themes.sh apply' (or wait for the daily cron) to use it now."
    ;;
  install)
    # Capture the CURRENT theme as the base (so non-holiday days keep your pick),
    # unless a base was already chosen.
    if [ ! -f "$BASEFILE" ]; then
      cur="basic"
      if [ -f "$CONFIG_DIR/config.local.js" ]; then
        c="$(grep -oE 'theme:[[:space:]]*"[^"]*"' "$CONFIG_DIR/config.local.js" | head -1 | sed -E 's/.*"([^"]*)".*/\1/')"
        [ -n "$c" ] && cur="$c"
      fi
      printf '%s\n' "$cur" > "$BASEFILE"
      ok "base theme remembered as: $cur (change with: seasonal-themes.sh base <name>)"
    fi
    # Daily at 00:05, set the theme appropriate for the date.
    printf '1\n' > "$HOME/.dashboard-seasonal-themes"
    CT="$(mktemp)"
    crontab -l 2>/dev/null | grep -v "seasonal-themes.sh apply" > "$CT" || true
    echo "5 0 * * * $DASH/bin/seasonal-themes.sh apply >/dev/null 2>&1" >> "$CT"
    crontab "$CT"; rm -f "$CT"
    ok "installed daily cron (00:05) — the dashboard will theme itself by date"
    # Apply right now too.
    "$DASH/bin/seasonal-themes.sh" apply
    ;;
  uninstall)
    CT="$(mktemp)"
    crontab -l 2>/dev/null | grep -v "seasonal-themes.sh apply" > "$CT" || true
    crontab "$CT"; rm -f "$CT"
    rm -f "$HOME/.dashboard-seasonal-themes"
    ok "seasonal cron removed (current theme unchanged)"
    ;;
  show)
    base="basic"; [ -f "$BASEFILE" ] && base="$(cat "$BASEFILE")"
    say "Seasonal schedule (between holidays: $base)"
    printf '%s\n' "$SCHEDULE" | while read -r s e th; do
      [ -z "$th" ] && continue
      printf '   %s .. %s  -> %s\n' "$s" "$e" "$th"
    done
    echo "   today ($(date +%m-%d)) -> $(pick_theme)"
    ;;
  *)
    warn "usage: seasonal-themes.sh [apply|install|uninstall|base <name>|show]"
    exit 1;;
esac
