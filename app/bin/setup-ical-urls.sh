#!/bin/bash
# =====================================================================
#  setup-ical-urls.sh — pull calendars from "secret iCal" .ics URLs.
#
#  Writes calendar files into ~/dashboard/calendars and logs into
#  ~/dashboard/logs, keeping the dashboard root tidy.
# =====================================================================
set -u
DASH="$HOME/dashboard"
BIN_DIR="$DASH/bin"
CAL_DIR="$DASH/calendars"
LOG_DIR="$DASH/logs"
MAP="$HOME/.dashboard-ical-urls"      # stores name|color|tag|url per calendar
say(){ printf '\n\033[1;36m== %s\033[0m\n' "$*"; }
warn(){ printf '\033[1;33m!! %s\033[0m\n' "$*"; }
ok(){ printf '\033[1;32m   %s\033[0m\n' "$*"; }

say "Calendar sync via iCal (.ics) URLs"
echo "Paste any .ics calendar URL — Google secret addresses, Outlook,"
echo "Nextcloud shares, webcal:// links, or a plain hosted .ics file."
echo
mkdir -p "$DASH" "$BIN_DIR" "$CAL_DIR" "$LOG_DIR"
touch "$MAP"; chmod 600 "$MAP"
if [ -s "$MAP" ]; then
  echo "(Existing iCal calendars are kept; new ones are added. To start over,"
  echo " delete $MAP and re-run.)"
  echo
fi

name_key(){ printf '%s' "$1" | tr '[:upper:]' '[:lower:]'; }
calendar_files_for_name(){
  local want base f
  want="$(name_key "$1")"
  for f in "$CAL_DIR"/*.ics "$DASH"/*.ics; do
    [ -e "$f" ] || continue
    base="$(basename "$f")"; base="${base%%.*}"
    [ "$(name_key "$base")" = "$want" ] && printf '%s\n' "$f"
  done
}
map_has_name(){
  local want nm
  want="$(name_key "$1")"
  [ -f "$MAP" ] || return 1
  while IFS='|' read -r nm _; do
    [ -n "$nm" ] || continue
    [ "$(name_key "$nm")" = "$want" ] && return 0
  done < "$MAP"
  return 1
}
name_taken(){
  [ -n "$(calendar_files_for_name "$1")" ] && return 0
  map_has_name "$1" && return 0
  return 1
}
remove_calendar_name(){
  local want tmp f
  want="$(name_key "$1")"
  for f in $(calendar_files_for_name "$1"); do
    rm -f "$f" 2>/dev/null || true
  done
  if [ -f "$MAP" ]; then
    tmp="$(mktemp)"
    awk -F'|' -v want="$want" 'tolower($1) != want { print }' "$MAP" > "$tmp" && mv "$tmp" "$MAP"
    chmod 600 "$MAP" 2>/dev/null || true
  fi
}
describe_collision(){
  local f
  echo "    Existing calendar/source for '$1':"
  for f in $(calendar_files_for_name "$1"); do echo "      file: $f"; done
  map_has_name "$1" && echo "      source: $MAP"
}
resolve_calendar_collision(){
  # returns: 0 = proceed, 1 = ask again, 2 = skip this calendar
  name_taken "$1" || return 0
  warn "    A calendar named '$1' already exists."
  describe_collision "$1"
  echo "    Choose what to do:"
  echo "      1) Overwrite this calendar/source"
  echo "      2) Change name/color"
  echo "      3) Skip"
  while true; do
    read -rp "    Choose [1/2/3]: " ans
    case "$ans" in
      1) remove_calendar_name "$1"; ok "    old '$1' calendar/source removed; continuing"; return 0;;
      2) return 1;;
      3) return 2;;
      *) warn "    Please choose 1, 2, or 3.";;
    esac
  done
}
valid_name(){ printf '%s' "$1" | grep -qE '^[A-Za-z0-9_-]+$'; }
valid_color(){
  case "$(printf '%s' "$1" | tr '[:upper:]' '[:lower:]')" in
    green|blue|red|gold|violet|purple|amber|teal|orange|slate) return 0;;
  esac
  printf '%s' "$1" | grep -qiE '^#?[0-9a-f]{6}$'
}

i=0
while true; do
  read -rp "  Calendar name (blank to finish): " n; [ -z "$n" ] && break
  if ! valid_name "$n"; then warn "    Use only letters, numbers, hyphen, underscore."; continue; fi
  resolve_calendar_collision "$n"
  collision_action="$?"
  [ "$collision_action" = "1" ] && continue
  [ "$collision_action" = "2" ] && { warn "    skipped $n"; continue; }
  while true; do
    read -rp "    color [green]: " c; c="${c:-green}"
    if valid_color "$c"; then break; fi
    warn "    Color must be a palette name or a 6-digit hex."
  done
  read -rp "    holiday calendar? [y/N]: " h
  read -rp "    .ics URL: " url
  if [ -z "$url" ]; then warn "    no URL given, skipping"; continue; fi
  case "$url" in webcals://*) url="https://${url#webcals://}";; webcal://*) url="https://${url#webcal://}";; esac
  echo "    checking URL..."
  if curl -fsSL -A "Mozilla/5.0" --max-time 30 "$url" 2>/dev/null | grep -q "BEGIN:VCALENDAR"; then
    ok "    URL returns a valid calendar"
  else
    warn "    That URL did not return a valid .ics calendar."
    read -rp "    Add it anyway? [y/N]: " force
    [ "$force" = "y" ] || [ "$force" = "Y" ] || { warn "    skipped $n"; continue; }
  fi
  if [ "$h" = "y" ] || [ "$h" = "Y" ]; then tag="holiday"; else tag=""; fi
  echo "$n|$c|$tag|$url" >> "$MAP"
  i=$((i+1)); ok "    added $n"
done
[ "$i" -eq 0 ] && { warn "no calendars defined — re-run when ready"; exit 0; }

say "Writing sync script (bin/sync-ical.sh)"
cat > "$BIN_DIR/sync-ical.sh" <<WRAP
#!/bin/bash
set -u
DASH="$DASH"
BIN_DIR="\$DASH/bin"
CAL_DIR="\$DASH/calendars"
LOG_DIR="\$DASH/logs"
MAP="$MAP"
UA="Mozilla/5.0"
MAX_JOBS="\${MAX_JOBS:-4}"
mkdir -p "\$CAL_DIR" "\$LOG_DIR"
pull_one(){
  local name="\$1" color="\$2" tag="\$3" url="\$4" dest tmp
  if [ -n "\$tag" ]; then dest="\$CAL_DIR/\$name.\$color.\$tag.ics"; else dest="\$CAL_DIR/\$name.\$color.ics"; fi
  tmp="\$(mktemp)"
  if curl -fsSL -A "\$UA" --max-time 60 -o "\$tmp" "\$url" && grep -q "BEGIN:VCALENDAR" "\$tmp"; then
    mv "\$tmp" "\$dest"
  else
    rm -f "\$tmp"
    echo "\$(date): failed to pull \$name" >> "\$LOG_DIR/ical-sync.log"
  fi
}
while IFS='|' read -r name color tag url; do
  [ -z "\$url" ] && continue
  pull_one "\$name" "\$color" "\$tag" "\$url" &
  while [ "\$(jobs -rp | wc -l)" -ge "\$MAX_JOBS" ]; do sleep 0.2; done
done < "\$MAP"
wait
if [ -f "\$LOG_DIR/ical-sync.log" ] && [ "\$(wc -l < "\$LOG_DIR/ical-sync.log")" -gt 400 ]; then
  tail -n 200 "\$LOG_DIR/ical-sync.log" > "\$LOG_DIR/ical-sync.log.tmp" && mv "\$LOG_DIR/ical-sync.log.tmp" "\$LOG_DIR/ical-sync.log"
fi
[ -x "\$BIN_DIR/gen-calendars.sh" ] && "\$BIN_DIR/gen-calendars.sh" >/dev/null 2>&1
[ -x "\$BIN_DIR/dashboard-control-server" ] && "\$BIN_DIR/dashboard-control-server" --gen-events-cache >/dev/null 2>&1
WRAP
chmod +x "$BIN_DIR/sync-ical.sh"
ok "sync-ical.sh written"

say "Pulling calendars now"
"$BIN_DIR/sync-ical.sh" && ok "calendars pulled + manifest regenerated"
echo "Files in $CAL_DIR:"; ls -1 "$CAL_DIR"/*.ics 2>/dev/null | sed 's/^/   /'

say "Scheduling sync (every 10 min)"
CT="$(mktemp)"
crontab -l 2>/dev/null | grep -v "$DASH/sync-ical.sh" | grep -v "$BIN_DIR/sync-ical.sh" > "$CT" || true
echo "*/10 * * * * $BIN_DIR/sync-ical.sh" >> "$CT"
crontab "$CT"; rm -f "$CT"
ok "cron installed"

say "iCal URL sync complete"
echo "Calendars refresh every 10 min into: $CAL_DIR/*.ics"
echo "Re-run setup-ical-urls.sh to change the set of calendars."
