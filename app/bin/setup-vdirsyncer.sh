#!/usr/bin/env bash
# =====================================================================
# setup-vdirsyncer.sh — opt-in CalDAV sync for Dash-Go.
#
# Pulls CalDAV collections into a private local vdir, merges them into the
# dashboard's one-file-per-calendar format, then rebuilds the normal manifest
# and event cache. Credentials remain outside ~/dashboard/ at all times.
# =====================================================================
set -u

DASH="${DASH:-$HOME/dashboard}"
BIN_DIR="$DASH/bin"
CAL_DIR="$DASH/calendars"
LOG_DIR="$DASH/logs"
VDIR_HOME="${DASH_VDIR_HOME:-$HOME/.dashboard-vdirsyncer}"
VDIR_CFG="$VDIR_HOME/config"
VDIR_STATUS="$VDIR_HOME/status"
VDIR_COLLECTIONS="$VDIR_HOME/collections"
VDIR_PAIRS="$VDIR_HOME/pairs"
VDIR_PASSWORDS="$VDIR_HOME/passwords"
MAP="${DASH_VDIR_MAP:-$VDIR_HOME/calendars.map}"
SYNC_LOG="$LOG_DIR/vdir-sync.log"

# pip --user and pipx commonly place vdirsyncer here. Keep cron and interactive
# runs consistent without modifying a user's shell profile.
export PATH="$HOME/.local/bin:$PATH"
umask 077

say(){ printf '\n\033[1;36m== %s\033[0m\n' "$*"; }
warn(){ printf '\033[1;33m!! %s\033[0m\n' "$*"; }
ok(){ printf '\033[1;32m   %s\033[0m\n' "$*"; }
have(){ command -v "$1" >/dev/null 2>&1; }

mkdir -p "$DASH" "$BIN_DIR" "$CAL_DIR" "$LOG_DIR" "$VDIR_HOME" "$VDIR_STATUS" "$VDIR_COLLECTIONS" "$VDIR_PASSWORDS"
chmod 700 "$VDIR_HOME" "$VDIR_STATUS" "$VDIR_COLLECTIONS" "$VDIR_PASSWORDS" 2>/dev/null || true
touch "$MAP" "$VDIR_PAIRS"
chmod 600 "$MAP" "$VDIR_PAIRS" 2>/dev/null || true

name_key(){ printf '%s' "$1" | tr '[:upper:]' '[:lower:]'; }
valid_name(){ printf '%s' "$1" | grep -qE '^[A-Za-z0-9_-]+$'; }
valid_color(){
  case "$(printf '%s' "$1" | tr '[:upper:]' '[:lower:]')" in
    green|blue|red|gold|violet|purple|amber|teal|orange|slate) return 0;;
  esac
  printf '%s' "$1" | grep -qiE '^#?[0-9a-f]{6}$'
}
valid_single_line(){
  case "$1" in *$'\n'*|*$'\r'*|*'|'*) return 1;; esac
  return 0
}
valid_caldav_url(){
  valid_single_line "$1" && printf '%s' "$1" | grep -qE '^https?://[^[:space:]]+$'
}
valid_collection_id(){
  [ -z "$1" ] && return 0
  valid_single_line "$1" && printf '%s' "$1" | grep -qE '^[A-Za-z0-9._-]+$'
}
toml_quote(){
  # We reject line breaks in values before persisting. Quote remaining TOML
  # metacharacters so a user name or URL cannot alter vdirsyncer config syntax.
  local value="$1"
  value="${value//\\/\\\\}"
  value="${value//\"/\\\"}"
  printf '"%s"' "$value"
}
calendar_file_exists(){
  local want base file
  want="$(name_key "$1")"
  for file in "$CAL_DIR"/*.ics; do
    [ -e "$file" ] || continue
    base="$(basename "$file")"; base="${base%%.*}"
    [ "$(name_key "$base")" = "$want" ] && return 0
  done
  return 1
}
map_has_name(){
  local want name _
  want="$(name_key "$1")"
  while IFS='|' read -r name _; do
    [ -n "$name" ] || continue
    [ "$(name_key "$name")" = "$want" ] && return 0
  done < "$MAP"
  return 1
}
remove_own_calendar(){
  local target tmp old name color tag pair collection _
  target="$(name_key "$1")"
  old="$(mktemp)" || return 1
  awk -F'|' -v target="$target" 'tolower($1) == target { print }' "$MAP" > "$old" || { rm -f "$old"; return 1; }
  while IFS='|' read -r name color tag pair collection _; do
    [ -n "$name" ] || continue
    rm -rf "$collection" 2>/dev/null || true
    rm -f "$VDIR_PASSWORDS/$name" 2>/dev/null || true
    if [ -n "$tag" ]; then rm -f "$CAL_DIR/$name.$color.$tag.ics"; else rm -f "$CAL_DIR/$name.$color.ics"; fi
  done < "$old"
  rm -f "$old"
  tmp="$(mktemp)" || return 1
  awk -F'|' -v target="$target" 'tolower($1) != target { print }' "$MAP" > "$tmp" && mv "$tmp" "$MAP" || { rm -f "$tmp"; return 1; }
  tmp="$(mktemp)" || return 1
  awk -F'|' -v target="$target" 'tolower($1) != target { print }' "$VDIR_PAIRS" > "$tmp" && mv "$tmp" "$VDIR_PAIRS" || { rm -f "$tmp"; return 1; }
  chmod 600 "$MAP" "$VDIR_PAIRS" 2>/dev/null || true
}

ensure_vdirsyncer(){
  if have vdirsyncer; then
    ok "vdirsyncer found: $(command -v vdirsyncer)"
    return 0
  fi
  warn "vdirsyncer is not installed."
  echo "  How would you like to install it?"
  echo "    1) apt        sudo apt-get install vdirsyncer   (recommended on Debian / Raspberry Pi OS)"
  echo "    2) pipx       pipx install vdirsyncer           (isolated; needs pipx)"
  echo "    3) pip --user pip3 install --user vdirsyncer    (needs python3-pip)"
  echo "    4) Skip       install it yourself and stop here"
  read -rp "  Choose [1/2/3/4]: " install_choice
  case "$install_choice" in
    1)
      have apt-get || { warn "apt-get is not available on this system"; return 1; }
      sudo apt-get update && sudo apt-get install -y vdirsyncer || { warn "apt install failed"; return 1; }
      ;;
    2)
      have pipx || { warn "pipx is not installed (try: sudo apt-get install pipx)"; return 1; }
      pipx install vdirsyncer || { warn "pipx install failed"; return 1; }
      ;;
    3)
      have pip3 || { warn "pip3 is not installed (try: sudo apt-get install python3-pip)"; return 1; }
      pip3 install --user vdirsyncer || { warn "pip --user install failed"; return 1; }
      ;;
    *) warn "Install vdirsyncer, then re-run this option."; return 1;;
  esac
  have vdirsyncer || { warn "vdirsyncer is not on PATH. For pip --user, ensure ~/.local/bin is available, then re-run."; return 1; }
  ok "vdirsyncer installed"
}

write_vdirsyncer_config(){
  local temp name color tag pair collection url username remote local_path coll_spec
  temp="$(mktemp)" || return 1
  {
    printf '[general]\n'
    printf 'status_path = %s\n\n' "$(toml_quote "$VDIR_STATUS")"
    while IFS='|' read -r name color tag pair local_path url username collection; do
      [ -n "$name" ] || continue
      if [ -n "$collection" ]; then
        coll_spec="[$(toml_quote "$collection")]"
      else
        coll_spec='["from a"]'
      fi
      remote="${pair}_remote"
      local_path="${pair}_local"
      printf '[pair %s]\n' "$pair"
      printf 'a = %s\n' "$(toml_quote "$remote")"
      printf 'b = %s\n' "$(toml_quote "$local_path")"
      printf 'collections = %s\n' "$coll_spec"
      printf 'conflict_resolution = "a wins"\n\n'

      printf '[storage %s]\n' "$remote"
      printf 'type = "caldav"\n'
      printf 'url = %s\n' "$(toml_quote "$url")"
      printf 'username = %s\n' "$(toml_quote "$username")"
      printf 'password.fetch = ["command", "cat", %s]\n\n' "$(toml_quote "$VDIR_PASSWORDS/$name")"

      printf '[storage %s]\n' "$local_path"
      printf 'type = "filesystem"\n'
      printf 'path = %s\n' "$(toml_quote "$VDIR_COLLECTIONS/$name/")"
      printf 'fileext = ".ics"\n\n'
    done < "$VDIR_PAIRS"
  } > "$temp"
  chmod 600 "$temp" || { rm -f "$temp"; return 1; }
  mv "$temp" "$VDIR_CFG"
  chmod 600 "$VDIR_CFG" 2>/dev/null || true
}

write_sync_wrapper(){
  cat > "$BIN_DIR/sync-vdir.sh" <<WRAPPER
#!/usr/bin/env bash
# Generated by setup-vdirsyncer.sh. Pulls private CalDAV data, merges each
# local vdir into one Dash-Go calendar file, and rebuilds derived indexes.
set -u
DASH=$(printf '%q' "$DASH")
BIN_DIR="\$DASH/bin"
CAL_DIR="\$DASH/calendars"
LOG_DIR="\$DASH/logs"
VDIR_HOME=$(printf '%q' "$VDIR_HOME")
VDIR_CFG=$(printf '%q' "$VDIR_CFG")
VDIR_COLLECTIONS=$(printf '%q' "$VDIR_COLLECTIONS")
MAP=$(printf '%q' "$MAP")
LOG="\$LOG_DIR/vdir-sync.log"
LOCK_DIR="\$VDIR_HOME/sync.lock"
export PATH="\$HOME/.local/bin:\$PATH"
export VDIRSYNCER_CONFIG="\$VDIR_CFG"
umask 077
mkdir -p "\$CAL_DIR" "\$LOG_DIR" "\$VDIR_HOME"
log(){ printf '%s %s\\n' "\$(date '+%Y-%m-%d %H:%M:%S')" "\$*" >> "\$LOG"; }

acquire_lock(){
  if mkdir "\$LOCK_DIR" 2>/dev/null; then echo "\$\$" > "\$LOCK_DIR/pid"; return 0; fi
  if [ -r "\$LOCK_DIR/pid" ]; then
    pid="\$(cat "\$LOCK_DIR/pid" 2>/dev/null || true)"
    case "\$pid" in
      ''|*[!0-9]*) ;;
      *) if kill -0 "\$pid" 2>/dev/null; then log 'sync already running; skipped overlapping run'; return 1; fi;;
    esac
  fi
  rm -rf "\$LOCK_DIR" 2>/dev/null || return 1
  mkdir "\$LOCK_DIR" 2>/dev/null || return 1
  echo "\$\$" > "\$LOCK_DIR/pid"
}
acquire_lock || exit 0
trap 'rm -rf "\$LOCK_DIR"' EXIT

command -v vdirsyncer >/dev/null 2>&1 || { log 'vdirsyncer not on PATH'; exit 0; }
[ -r "\$VDIR_CFG" ] && [ -r "\$MAP" ] || { log 'vdirsyncer configuration is incomplete'; exit 1; }

# Discovery is noninteractive because this wrapper is run by cron. It remains
# idempotent and lets a manually added collection appear below the configured
# collection root without changing stored credentials.
yes | vdirsyncer -c "\$VDIR_CFG" discover >> "\$LOG" 2>&1 || log 'vdirsyncer discover reported an issue'
sync_rc=0
vdirsyncer -c "\$VDIR_CFG" sync >> "\$LOG" 2>&1 || { sync_rc=1; log 'vdirsyncer sync reported errors; retaining previous calendar data where needed'; }

merge_collection(){
  src="\$1"; dest="\$2"; tmp="\$(mktemp)"
  if ! find "\$src" -type f -name '*.ics' -print -quit 2>/dev/null | grep -q .; then
    rm -f "\$tmp"; return 1
  fi
  {
    printf 'BEGIN:VCALENDAR\\nVERSION:2.0\\nPRODID:-//Dash-Go//vdirsyncer//EN\\nCALSCALE:GREGORIAN\\n'
    find "\$src" -type f -name '*.ics' -print0 2>/dev/null | LC_ALL=C sort -z | xargs -0r awk '
      { sub(/\\r\$/, "") }
      /^BEGIN:VCALENDAR/ { next }
      /^END:VCALENDAR/   { next }
      /^BEGIN:V/ { depth++; print; next }
      /^END:V/   { print; if (depth>0) depth--; next }
      depth>0 { print }
    '
    printf 'END:VCALENDAR\\n'
  } > "\$tmp"
  if grep -q 'BEGIN:VEVENT' "\$tmp"; then mv "\$tmp" "\$dest"; return 0; fi
  if [ ! -s "\$dest" ]; then mv "\$tmp" "\$dest"; else rm -f "\$tmp"; fi
  return 0
}

while IFS='|' read -r name color tag pair collection; do
  [ -n "\$name" ] || continue
  if [ -n "\$tag" ]; then dest="\$CAL_DIR/\$name.\$color.\$tag.ics"; else dest="\$CAL_DIR/\$name.\$color.ics"; fi
  if merge_collection "\$collection" "\$dest"; then
    log "merged \$name -> \$(basename "\$dest")"
  else
    log "no local CalDAV data for \$name; kept previous calendar file"
  fi
done < "\$MAP"

if [ -f "\$LOG" ] && [ "\$(wc -l < "\$LOG")" -gt 400 ]; then
  tail -n 200 "\$LOG" > "\$LOG.tmp" && mv "\$LOG.tmp" "\$LOG"
fi
[ -x "\$BIN_DIR/gen-calendars.sh" ] && "\$BIN_DIR/gen-calendars.sh" >/dev/null 2>&1 || true
[ -x "\$BIN_DIR/dashboard-control-server" ] && "\$BIN_DIR/dashboard-control-server" --gen-events-cache >/dev/null 2>&1 || true
exit "\$sync_rc"
WRAPPER
  chmod 700 "$BIN_DIR/sync-vdir.sh"
}

install_vdir_cron(){
  local cron_tmp
  have crontab || { warn "crontab is unavailable; run $BIN_DIR/sync-vdir.sh manually or install cron"; return 1; }
  cron_tmp="$(mktemp)" || return 1
  crontab -l 2>/dev/null | grep -Fv "$BIN_DIR/sync-vdir.sh" > "$cron_tmp" || true
  printf '*/15 * * * * %s/sync-vdir.sh >/dev/null 2>&1\n' "$BIN_DIR" >> "$cron_tmp"
  crontab "$cron_tmp"
  local rc=$?
  rm -f "$cron_tmp"
  return "$rc"
}

say "CalDAV calendar sync via vdirsyncer"
echo "Pull calendars directly onto this device from iCloud, Nextcloud, Fastmail,"
echo "Radicale, or another standard CalDAV server. Credentials remain outside"
echo "the dashboard webroot in $VDIR_HOME (owner-only permissions)."
ensure_vdirsyncer || exit 0

added=0
while true; do
  read -rp "  Calendar name (blank to finish): " name
  [ -n "$name" ] || break
  if ! valid_name "$name"; then
    warn "    Use only letters, numbers, hyphen, and underscore."
    continue
  fi
  if map_has_name "$name"; then
    warn "    A Dash-Go CalDAV calendar named '$name' already exists."
    read -rp "    Replace its saved CalDAV setup? [y/N]: " replace
    case "$replace" in
      y|Y) remove_own_calendar "$name" || { warn "    could not replace $name"; continue; };;
      *) warn "    skipped $name"; continue;;
    esac
  elif calendar_file_exists "$name"; then
    warn "    A different calendar file already uses '$name'. Choose a different name so this CalDAV sync cannot overwrite it."
    continue
  fi

  while true; do
    read -rp "    Color [blue]: " color
    color="${color:-blue}"
    valid_color "$color" && break
    warn "    Use a palette color or six-digit hex value."
  done
  read -rp "    Holiday calendar? [y/N]: " holiday
  case "$holiday" in y|Y) tag="holiday";; *) tag="";; esac

  echo "    CalDAV server base URL:"
  echo "      iCloud:    https://caldav.icloud.com/   (default)"
  echo "      Nextcloud: https://HOST/remote.php/dav/"
  echo "      Fastmail:  https://caldav.fastmail.com/dav/"
  read -rp "    URL [https://caldav.icloud.com/]: " url
  url="${url:-https://caldav.icloud.com/}"
  if ! valid_caldav_url "$url"; then warn "    Use a single-line http(s) CalDAV URL without spaces or | characters."; continue; fi
  read -rp "    Username (for example, Apple ID email): " username
  if [ -z "$username" ] || ! valid_single_line "$username"; then warn "    no valid username given, skipping"; continue; fi
  read -rsp "    App-specific password: " password; echo
  if [ -z "$password" ] || ! valid_single_line "$password"; then warn "    no valid password given, skipping"; unset password; continue; fi
  echo "    Optionally limit to one collection UUID (blank = sync all discovered collections)."
  read -rp "    Collection UUID [all]: " collection_id
  if ! valid_collection_id "$collection_id"; then warn "    collection UUID may use only letters, numbers, dot, hyphen, and underscore."; unset password; continue; fi

  pair="dash_${name}"
  collection_path="$VDIR_COLLECTIONS/$name"
  mkdir -p "$collection_path"
  printf '%s|%s|%s|%s|%s\n' "$name" "$color" "$tag" "$pair" "$collection_path" >> "$MAP"
  printf '%s|%s|%s|%s|%s|%s|%s|%s\n' "$name" "$color" "$tag" "$pair" "$collection_path" "$url" "$username" "$collection_id" >> "$VDIR_PAIRS"
  printf '%s' "$password" > "$VDIR_PASSWORDS/$name"
  chmod 600 "$VDIR_PASSWORDS/$name" "$MAP" "$VDIR_PAIRS" 2>/dev/null || true
  unset password
  added=$((added + 1))
  ok "    queued $name"
done

[ "$added" -gt 0 ] || { warn "no CalDAV calendars defined — re-run when ready"; exit 0; }

say "Writing private vdirsyncer configuration"
if ! write_vdirsyncer_config; then
  warn "could not safely write $VDIR_CFG"
  exit 1
fi
ok "config written (credentials remain outside the dashboard webroot)"

say "Writing CalDAV sync wrapper"
write_sync_wrapper || { warn "could not write $BIN_DIR/sync-vdir.sh"; exit 1; }
ok "sync-vdir.sh written"

say "Pulling CalDAV calendars now"
if "$BIN_DIR/sync-vdir.sh"; then
  ok "initial CalDAV sync completed"
else
  warn "initial CalDAV sync reported an issue; existing local calendar files were kept. See $SYNC_LOG"
fi
echo "Files in $CAL_DIR:"
ls -1 "$CAL_DIR"/*.ics 2>/dev/null | sed 's/^/   /' || true

say "Scheduling CalDAV sync (every 15 minutes)"
if install_vdir_cron; then
  ok "cron installed"
else
  warn "cron was not installed; run $BIN_DIR/sync-vdir.sh manually or repair cron"
fi

say "CalDAV/vdirsyncer setup complete"
echo "Calendars sync every 15 minutes into $CAL_DIR/*.ics."
echo "Re-run setup-vdirsyncer.sh to add or replace Dash-Go CalDAV calendars."
echo "Credentials and vdir state remain only in $VDIR_HOME (owner-only)."
