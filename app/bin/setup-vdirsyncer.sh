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
CONFIG_DIR="$DASH/config"
LOG_DIR="$DASH/logs"
VDIR_HOME="${DASH_VDIR_HOME:-$HOME/.dashboard-vdirsyncer}"
VDIR_CFG="$VDIR_HOME/config"
VDIR_STATUS="$VDIR_HOME/status"
VDIR_COLLECTIONS="$VDIR_HOME/collections"
VDIR_PAIRS="$VDIR_HOME/pairs"
VDIR_PASSWORDS="$VDIR_HOME/passwords"
GOOGLE_TOKENS="$VDIR_HOME/google-tokens"
# Dash-Go owns one isolated, pinned vdirsyncer environment. Keep the tool and
# its Python dependencies outside ~/dashboard with the private vdir state.
VDIR_PIPX_HOME="${DASH_VDIR_PIPX_HOME:-$VDIR_HOME/pipx}"
VDIR_PIPX_BIN="${DASH_VDIR_PIPX_BIN:-$VDIR_HOME/bin}"
VDIRSYNCER_VERSION="0.20.0"
VDIRSYNCER_BIN="${DASH_VDIRSYNCER_BIN:-$VDIR_PIPX_BIN/vdirsyncer}"
MAP="${DASH_VDIR_MAP:-$VDIR_HOME/calendars.map}"
WRITEBACK_REGISTRY="$CONFIG_DIR/calendar-writeback.json"
SYNC_LOG="$LOG_DIR/vdir-sync.log"

# The generated wrapper calls the known Dash-Go-managed executable directly;
# keep its bin directory first only for interactive diagnostics and helpers.
export PATH="$VDIR_PIPX_BIN:$PATH"
umask 077

say(){ printf '\n\033[1;36m== %s\033[0m\n' "$*"; }
warn(){ printf '\033[1;33m!! %s\033[0m\n' "$*"; }
ok(){ printf '\033[1;32m   %s\033[0m\n' "$*"; }
have(){ command -v "$1" >/dev/null 2>&1; }

mkdir -p "$DASH" "$BIN_DIR" "$CAL_DIR" "$CONFIG_DIR" "$LOG_DIR" "$VDIR_HOME" "$VDIR_STATUS" "$VDIR_COLLECTIONS" "$VDIR_PASSWORDS" "$GOOGLE_TOKENS" "$VDIR_PIPX_HOME" "$VDIR_PIPX_BIN"
chmod 700 "$VDIR_HOME" "$VDIR_STATUS" "$VDIR_COLLECTIONS" "$VDIR_PASSWORDS" "$GOOGLE_TOKENS" "$VDIR_PIPX_HOME" "$VDIR_PIPX_BIN" 2>/dev/null || true
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
  # Google Calendar IDs use the address form user@gmail.com or
  # hash@group.calendar.google.com, so @ is a legal collection character.
  [ -z "$1" ] && return 0
  valid_single_line "$1" && printf '%s' "$1" | grep -qE '^[A-Za-z0-9._@-]+$'
}
valid_client_id(){
  [ -n "$1" ] && valid_single_line "$1" && printf '%s' "$1" | grep -qE '^[A-Za-z0-9._-]+$'
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
    rm -f "$VDIR_PASSWORDS/$name" "$VDIR_PASSWORDS/$name.google-client-secret" "$GOOGLE_TOKENS/$name.json" 2>/dev/null || true
    if [ -n "$tag" ]; then rm -f "$CAL_DIR/$name.$color.$tag.ics"; else rm -f "$CAL_DIR/$name.$color.ics"; fi
  done < "$old"
  rm -f "$old"
  tmp="$(mktemp)" || return 1
  awk -F'|' -v target="$target" 'tolower($1) != target { print }' "$MAP" > "$tmp" && mv "$tmp" "$MAP" || { rm -f "$tmp"; return 1; }
  tmp="$(mktemp)" || return 1
  awk -F'|' -v target="$target" 'tolower($1) != target { print }' "$VDIR_PAIRS" > "$tmp" && mv "$tmp" "$VDIR_PAIRS" || { rm -f "$tmp"; return 1; }
  chmod 600 "$MAP" "$VDIR_PAIRS" 2>/dev/null || true
}

vdirsyncer_version(){
  [ -x "$VDIRSYNCER_BIN" ] || return 1
  "$VDIRSYNCER_BIN" --version 2>/dev/null | grep -Eo '[0-9]+\.[0-9]+\.[0-9]+' | head -n1
}
vdirsyncer_is_pinned(){
  [ "$(vdirsyncer_version 2>/dev/null || true)" = "$VDIRSYNCER_VERSION" ]
}
pipx_run(){
  PIPX_HOME="$VDIR_PIPX_HOME" PIPX_BIN_DIR="$VDIR_PIPX_BIN" pipx "$@"
}
ensure_pipx(){
  if have pipx; then return 0; fi
  if ! have apt-get; then
    warn "pipx is required for Dash-Go private calendar sync. Install pipx with this system's native package manager, then re-run setup."
    return 1
  fi
  echo "  Dash-Go installs pipx through APT, then keeps vdirsyncer isolated under $VDIR_HOME."
  read -rp "  Install pipx now? [Y/n]: " install_pipx
  case "${install_pipx:-y}" in n|N|no|NO) warn "pipx is required before private calendar sync can be configured"; return 1;; esac
  have sudo || { warn "sudo is required to install pipx with APT"; return 1; }
  sudo apt-get update && sudo apt-get install -y pipx || { warn "could not install pipx through APT"; return 1; }
  have pipx || { warn "pipx was installed but is not available on PATH; re-open the terminal and re-run setup"; return 1; }
}
install_pinned_vdirsyncer(){
  mkdir -p "$VDIR_PIPX_HOME" "$VDIR_PIPX_BIN"
  chmod 700 "$VDIR_PIPX_HOME" "$VDIR_PIPX_BIN" 2>/dev/null || true
  pipx_run install --force "vdirsyncer[google]==$VDIRSYNCER_VERSION" || { warn "pipx could not install vdirsyncer $VDIRSYNCER_VERSION"; return 1; }
  # pipx never upgrades applications unless asked. Pin where the installed pipx
  # supports it as a second explicit guard; an older pipx still retains the exact
  # version because Dash-Go never runs an automatic upgrade command.
  pipx_run pin vdirsyncer >/dev/null 2>&1 || warn "this pipx cannot record an explicit pin; Dash-Go still keeps vdirsyncer at the exact installed version and never auto-upgrades it"
  vdirsyncer_is_pinned || { warn "Dash-Go requires vdirsyncer $VDIRSYNCER_VERSION at $VDIRSYNCER_BIN"; return 1; }
}

# vdirsyncer's Google storage lives behind the optional [google] extra
# (aiohttp-oauthlib). Probe the interpreter that actually runs the known
# Dash-Go-managed executable, not an unrelated system vdirsyncer on PATH.
vdirsyncer_python(){
  if [ -n "${DASH_VDIRSYNCER_PYTHON:-}" ]; then
    [ -x "$DASH_VDIRSYNCER_PYTHON" ] && printf '%s\n' "$DASH_VDIRSYNCER_PYTHON"
    return
  fi
  # Resolve the interpreter that owns the installed vdirsyncer package. A
  # pipx/venv launcher usually names Python directly, while distro wrappers
  # often use `#!/usr/bin/env python3` (or `env -S python3`). The latter must
  # resolve the requested interpreter, not /usr/bin/env itself.
  local script shebang index
  local -a words
  script="$VDIRSYNCER_BIN"
  [ -x "$script" ] || return 1
  shebang="$(head -n1 "$script" 2>/dev/null)"
  case "$shebang" in
    '#!'*)
      read -r -a words <<< "${shebang#\#!}"
      case "${words[0]:-}" in
        */env)
          index=1
          [ "${words[$index]:-}" = "-S" ] && index=$((index + 1))
          while [ "$index" -lt "${#words[@]}" ] && [[ "${words[$index]}" = -* ]]; do index=$((index + 1)); done
          [ "$index" -lt "${#words[@]}" ] && command -v "${words[$index]}" || return 1
          ;;
        *python*)
          [ -x "${words[0]}" ] && printf '%s\n' "${words[0]}" || command -v "${words[0]}"
          ;;
        *) command -v python3;;
      esac
      ;;
    *) command -v python3;;
  esac
}
google_support_present(){
  local py
  py="$(vdirsyncer_python)" || return 1
  [ -x "$py" ] || return 1
  "$py" -c 'import aiohttp_oauthlib' >/dev/null 2>&1
}
ensure_vdirsyncer(){
  if vdirsyncer_is_pinned && google_support_present; then
    ok "Dash-Go vdirsyncer $VDIRSYNCER_VERSION is ready: $VDIRSYNCER_BIN"
    return 0
  fi
  if [ -n "${DASH_VDIRSYNCER_BIN:-}" ]; then
    warn "The explicit DASH_VDIRSYNCER_BIN must be vdirsyncer $VDIRSYNCER_VERSION with the [google] extra."
    return 1
  fi
  if [ -x "$VDIRSYNCER_BIN" ]; then
    warn "Dash-Go's vdirsyncer environment is missing Google support or is not the required $VDIRSYNCER_VERSION."
  else
    warn "Dash-Go private calendar sync uses a pinned vdirsyncer $VDIRSYNCER_VERSION environment managed by pipx."
  fi
  ensure_pipx || return 1
  read -rp "  Install or repair Dash-Go's pinned vdirsyncer now? [Y/n]: " install_choice
  case "${install_choice:-y}" in n|N|no|NO) warn "private calendar sync was not changed"; return 1;; esac
  install_pinned_vdirsyncer || return 1
  google_support_present || { warn "vdirsyncer $VDIRSYNCER_VERSION installed but its Google OAuth support is unavailable"; return 1; }
  ok "Dash-Go vdirsyncer $VDIRSYNCER_VERSION installed in its isolated pipx environment"
}
ensure_google_support(){
  if google_support_present; then
    ok "vdirsyncer Google support found (aiohttp-oauthlib present)"
    return 0
  fi
  [ -z "${DASH_VDIRSYNCER_BIN:-}" ] || { warn "The explicit vdirsyncer override lacks the required Google support"; return 1; }
  warn "Dash-Go's pinned vdirsyncer environment is incomplete; repairing the same pinned [google] installation."
  read -rp "  Repair Dash-Go's vdirsyncer now? [Y/n]: " repair_choice
  case "${repair_choice:-y}" in n|N|no|NO) warn "Google Calendar setup needs vdirsyncer[google]"; return 1;; esac
  ensure_pipx && install_pinned_vdirsyncer && google_support_present || { warn "could not restore Dash-Go's vdirsyncer Google support"; return 1; }
  ok "vdirsyncer Google support restored"
}
write_vdirsyncer_config(){
  local temp name color tag pair collection url username remote local_path coll_spec provider client_id
  temp="$(mktemp)" || return 1
  {
    printf '[general]\n'
    printf 'status_path = %s\n\n' "$(toml_quote "$VDIR_STATUS")"
    while IFS='|' read -r name color tag pair local_path url username collection provider client_id _; do
      [ -n "$name" ] || continue
      # Rows written before Google support carry no provider field; they are
      # plain CalDAV pairs and keep exactly their previous configuration.
      [ -n "$provider" ] || provider="caldav"
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
      if [ "$provider" = "google" ]; then
        printf 'type = "google_calendar"\n'
        printf 'token_file = %s\n' "$(toml_quote "$GOOGLE_TOKENS/$name.json")"
        printf 'client_id = %s\n' "$(toml_quote "$client_id")"
        printf 'client_secret.fetch = ["command", "cat", %s]\n\n' "$(toml_quote "$VDIR_PASSWORDS/$name.google-client-secret")"
      else
        printf 'type = "caldav"\n'
        printf 'url = %s\n' "$(toml_quote "$url")"
        printf 'username = %s\n' "$(toml_quote "$username")"
        printf 'password.fetch = ["command", "cat", %s]\n\n' "$(toml_quote "$VDIR_PASSWORDS/$name")"
      fi

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

write_writeback_registry(){
  local temp first name color tag pair collection writable collection_id source exact
  temp="$(mktemp)" || return 1
  first=1
  {
    printf '{\n  "version": 1,\n  "enabled": false,\n  "requirePin": false,\n  "calendars": ['
    while IFS='|' read -r name color tag pair collection writable collection_id _; do
      [ -n "$name" ] || continue
      # A broad discovery mirror is display-only. Writeback requires one exact
      # remote collection and a concrete local vdir below its pair root.
      [ "$writable" = "1" ] && [ -n "$collection_id" ] || continue
      exact="$collection/$collection_id"
      if [ ! -d "$exact" ]; then
        # This block's stdout is the registry file itself; the warning must
        # bypass the redirection or it corrupts the JSON for every calendar.
        warn "private collection $name was not materialized as one exact vdir; leaving Dashboard edits off" >&2
        continue
      fi
      if [ -n "$tag" ]; then source="calendars/$name.$color.$tag.ics"; else source="calendars/$name.$color.ics"; fi
      [ "$first" -eq 1 ] || printf ','
      first=0
      printf '\n    {"source":"%s","collection":"%s","writable":true,"enabled":true,"name":"%s"}' "$source" "$exact" "$name"
    done < "$MAP"
    printf '\n  ]\n}\n'
  } > "$temp" || { rm -f "$temp"; return 1; }
  chmod 600 "$temp" || { rm -f "$temp"; return 1; }
  mv "$temp" "$WRITEBACK_REGISTRY" || { rm -f "$temp"; return 1; }
  chmod 600 "$WRITEBACK_REGISTRY" 2>/dev/null || true
}

write_sync_wrapper(){
  cat > "$BIN_DIR/sync-vdir.sh" <<WRAPPER
#!/usr/bin/env bash
# Generated by setup-vdirsyncer.sh. Pulls private calendar data, merges each
# local vdir into one Dash-Go calendar file, and rebuilds derived indexes.
set -u
DASH=$(printf '%q' "$DASH")
BIN_DIR="\$DASH/bin"
CAL_DIR="\$DASH/calendars"
LOG_DIR="\$DASH/logs"
VDIR_HOME=$(printf '%q' "$VDIR_HOME")
VDIR_CFG=$(printf '%q' "$VDIR_CFG")
VDIR_COLLECTIONS=$(printf '%q' "$VDIR_COLLECTIONS")
VDIR_PAIRS=$(printf '%q' "$VDIR_PAIRS")
GOOGLE_TOKENS=$(printf '%q' "$GOOGLE_TOKENS")
VDIRSYNCER_BIN=$(printf '%q' "$VDIRSYNCER_BIN")
MAP=$(printf '%q' "$MAP")
LOG="\$LOG_DIR/vdir-sync.log"
LOCK_DIR="\$VDIR_HOME/sync.lock"
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

[ -x "\$VDIRSYNCER_BIN" ] || { log 'Dash-Go pinned vdirsyncer is unavailable; kept existing calendar mirrors'; exit 1; }
[ -r "\$VDIR_CFG" ] && [ -r "\$MAP" ] && [ -r "\$VDIR_PAIRS" ] || { log 'vdirsyncer configuration is incomplete'; exit 1; }

# A Google pair whose one-time authorization has not completed would start an
# interactive OAuth consent flow and block a cron run forever. Each eligible
# pair is discovered and synchronized independently: a revoked Google token or
# a failed remote pair cannot prevent another enrolled calendar from syncing.
# A pair's derived dashboard mirror is replaced only after that exact pair
# completes successfully, so skipped/failed pairs retain their previous data.
run_bounded(){
  if command -v timeout >/dev/null 2>&1; then timeout 600 "\$@"; else "\$@"; fi
}
declare -A PAIR_RESULT=()
sync_rc=0
ready_pairs=0
while IFS='|' read -r pname _ _ ppair _ _ _ _ pprovider _; do
  [ -n "\$pname" ] || continue
  [ -n "\$ppair" ] || continue
  if [ "\$pprovider" = "google" ] && [ ! -s "\$GOOGLE_TOKENS/\$pname.json" ]; then
    PAIR_RESULT["\$ppair"]="skipped"
    log "google calendar \$pname awaits its one-time authorization; skipped this run"
    continue
  fi
  ready_pairs=\$((ready_pairs + 1))
  # Discovery remains noninteractive under cron. A discovery warning should
  # not prevent a known pair from attempting its ordinary synchronization.
  yes | run_bounded "\$VDIRSYNCER_BIN" -c "\$VDIR_CFG" discover "\$ppair" >> "\$LOG" 2>&1 || log "vdirsyncer discover reported an issue for \$pname"
  if run_bounded "\$VDIRSYNCER_BIN" -c "\$VDIR_CFG" sync "\$ppair" >> "\$LOG" 2>&1; then
    PAIR_RESULT["\$ppair"]="synced"
  else
    PAIR_RESULT["\$ppair"]="failed"
    sync_rc=1
    log "vdirsyncer sync reported errors for \$pname; retained its previous calendar data"
  fi
done < "\$VDIR_PAIRS"
[ "\$ready_pairs" -gt 0 ] || log 'no pairs are ready to sync; retained existing local calendar data'

merge_collection(){
  src="\$1"; dest="\$2"; tmp="\$(mktemp)"
  if ! find "\$src" -type f -name '*.ics' -print -quit 2>/dev/null | grep -q .; then
    # An empty collection after a successful per-pair sync is legitimate (for
    # example after deleting its final event).
    printf 'BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//Dash-Go//vdirsyncer//EN\nCALSCALE:GREGORIAN\nEND:VCALENDAR\n' > "\$tmp"
    mv "\$tmp" "\$dest"; return 0
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
  mv "\$tmp" "\$dest"; return 0
}

while IFS='|' read -r name color tag pair collection writable _; do
  [ -n "\$name" ] || continue
  if [ -n "\$tag" ]; then dest="\$CAL_DIR/\$name.\$color.\$tag.ics"; else dest="\$CAL_DIR/\$name.\$color.ics"; fi
  case "\${PAIR_RESULT[\$pair]:-missing}" in
    synced)
      if merge_collection "\$collection" "\$dest"; then
        log "merged \$name -> \$(basename "\$dest")"
      else
        log "could not merge \$name; kept previous calendar file"
      fi
      ;;
    skipped) log "calendar \$name awaits authorization; kept previous calendar file";;
    failed) log "calendar \$name failed remote sync; kept previous calendar file";;
    *) log "calendar \$name has no eligible sync pair; kept previous calendar file";;
  esac
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

say "Private calendar sync via vdirsyncer"
echo "Pull calendars directly onto this device from iCloud, Nextcloud, Fastmail,"
echo "Radicale or another standard CalDAV server, or from Google Calendar via"
echo "OAuth. Credentials remain outside the dashboard webroot in $VDIR_HOME"
echo "(owner-only permissions)."
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
    warn "    A Dash-Go private calendar connection named '$name' already exists."
    read -rp "    Replace its saved CalDAV setup? [y/N]: " replace
    case "$replace" in
      y|Y) remove_own_calendar "$name" || { warn "    could not replace $name"; continue; };;
      *) warn "    skipped $name"; continue;;
    esac
  elif calendar_file_exists "$name"; then
    warn "    A different calendar file already uses '$name'. Choose a different name so this private calendar sync cannot overwrite it."
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

  echo "    Provider:"
  echo "      1) CalDAV server — iCloud, Nextcloud, Fastmail, Radicale, Baïkal (default)"
  echo "      2) Google Calendar — OAuth, needs your own OAuth client ID and secret"
  read -rp "    Choose [1]: " provider_choice
  provider="caldav"
  case "${provider_choice:-1}" in 2) provider="google";; esac

  url=""; username=""; client_id=""
  if [ "$provider" = "google" ]; then
    ensure_google_support || continue
    echo "    Google needs a one-time OAuth client from your own Google Cloud project"
    echo "    (Desktop-app type, CalDAV API enabled). See INTEGRATIONS.md for the recipe."
    read -rp "    OAuth client ID: " client_id
    if ! valid_client_id "$client_id"; then warn "    no valid OAuth client ID given, skipping"; continue; fi
    read -rsp "    OAuth client secret: " password; echo
    if [ -z "$password" ] || ! valid_single_line "$password"; then warn "    no valid OAuth client secret given, skipping"; unset password; continue; fi
    echo "    Optionally limit to one Calendar ID (blank = sync all discovered calendars)."
    echo "    Your primary calendar's ID is your Gmail address; other calendars show"
    echo "    their ID under Google Calendar settings → Integrate calendar."
    read -rp "    Calendar ID [all]: " collection_id
    if ! valid_collection_id "$collection_id"; then warn "    calendar ID may use only letters, numbers, dot, hyphen, underscore, and @."; unset password; continue; fi
  else
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
  fi
  writable=0
  if [ -n "$collection_id" ]; then
    read -rp "    Allow Dashboard add/edit/skip for this one collection? [y/N]: " writable_choice
    case "${writable_choice:-n}" in y|Y|yes|YES) writable=1;; esac
  else
    echo "    Broad discovered mirrors stay read-only. Choose one exact collection UUID to enable Dashboard edits later."
  fi

  pair="dash_${name}"
  collection_path="$VDIR_COLLECTIONS/$name"
  mkdir -p "$collection_path"
  printf '%s|%s|%s|%s|%s|%s|%s\n' "$name" "$color" "$tag" "$pair" "$collection_path" "$writable" "$collection_id" >> "$MAP"
  printf '%s|%s|%s|%s|%s|%s|%s|%s|%s|%s\n' "$name" "$color" "$tag" "$pair" "$collection_path" "$url" "$username" "$collection_id" "$provider" "$client_id" >> "$VDIR_PAIRS"
  if [ "$provider" = "google" ]; then
    printf '%s' "$password" > "$VDIR_PASSWORDS/$name.google-client-secret"
    chmod 600 "$VDIR_PASSWORDS/$name.google-client-secret" 2>/dev/null || true
  else
    printf '%s' "$password" > "$VDIR_PASSWORDS/$name"
    chmod 600 "$VDIR_PASSWORDS/$name" 2>/dev/null || true
  fi
  chmod 600 "$MAP" "$VDIR_PAIRS" 2>/dev/null || true
  unset password
  added=$((added + 1))
  ok "    queued $name"
done

if [ "$added" -eq 0 ] && ! grep -q '[^[:space:]]' "$VDIR_PAIRS"; then
  warn "no private calendars are defined — re-run when ready"
  exit 0
fi
[ "$added" -gt 0 ] || ok "no new private calendars; refreshing existing pipx-managed sync configuration"

say "Writing private vdirsyncer configuration"
if ! write_vdirsyncer_config; then
  warn "could not safely write $VDIR_CFG"
  exit 1
fi
ok "config written (credentials remain outside the dashboard webroot)"

authorize_google_pairs(){
  # The pairs file is read on its own descriptor so the authorization prompt
  # below keeps reading the user's answers from stdin.
  local gname gpair gprovider answered any=0
  while IFS='|' read -r gname _ _ gpair _ _ _ _ gprovider _ <&3; do
    [ -n "$gname" ] || continue
    [ "$gprovider" = "google" ] || continue
    [ -s "$GOOGLE_TOKENS/$gname.json" ] && continue
    if [ "$any" -eq 0 ]; then
      say "Google authorization (one time per calendar)"
      echo "vdirsyncer will print a Google sign-in URL that redirects to"
      echo "http://127.0.0.1:PORT on THIS device."
      echo "  - With a desktop session here, the browser opens automatically."
      echo "  - Over SSH: read PORT from redirect_uri in the printed URL, open a"
      echo "    second terminal on your computer with"
      echo "      ssh -L PORT:127.0.0.1:PORT $(whoami)@$(hostname)"
      echo "    then open the printed URL in your own browser; the final redirect"
      echo "    reaches this device through the tunnel."
      echo "  - Alternatively run this same setup on a desktop and copy"
      echo "    $GOOGLE_TOKENS/<name>.json here afterwards (0600 permissions)."
      any=1
    fi
    read -rp "  Authorize Google calendar '$gname' now? [Y/n]: " answered
    case "${answered:-y}" in
      n|N)
        warn "  skipped; '$gname' stays read-only-idle until authorized (re-run setup or: $VDIRSYNCER_BIN -c $VDIR_CFG discover $gpair)"
        continue;;
    esac
    if "$VDIRSYNCER_BIN" -c "$VDIR_CFG" discover "$gpair" && [ -s "$GOOGLE_TOKENS/$gname.json" ]; then
      chmod 600 "$GOOGLE_TOKENS/$gname.json" 2>/dev/null || true
      ok "  Google calendar '$gname' authorized"
    else
      warn "  authorization for '$gname' did not complete; it is skipped by sync until it does"
    fi
  done 3< "$VDIR_PAIRS"
}
authorize_google_pairs

say "Writing private calendar sync wrapper"
write_sync_wrapper || { warn "could not write $BIN_DIR/sync-vdir.sh"; exit 1; }
ok "sync-vdir.sh written"

say "Pulling private calendars now"
if "$BIN_DIR/sync-vdir.sh"; then
  ok "initial calendar sync completed"
else
  warn "initial private-calendar sync reported an issue; existing local calendar files were kept. See $SYNC_LOG"
fi
echo "Files in $CAL_DIR:"
ls -1 "$CAL_DIR"/*.ics 2>/dev/null | sed 's/^/   /' || true

write_writeback_registry || { warn "could not write calendar writeback registry"; exit 1; }
ok "calendar writeback registry written (Dashboard edits start disabled)"

say "Scheduling private calendar sync (every 15 minutes)"
if install_vdir_cron; then
  ok "cron installed"
else
  warn "cron was not installed; run $BIN_DIR/sync-vdir.sh manually or repair cron"
fi

say "Private calendar/vdirsyncer setup complete"
echo "Private calendars sync every 15 minutes into $CAL_DIR/*.ics."
echo "Re-run setup-vdirsyncer.sh to add, replace, or migrate Dash-Go private calendar connections."
echo "vdirsyncer $VDIRSYNCER_VERSION runs only from the isolated pipx environment under $VDIR_HOME."
echo "Credentials, tokens, vdir state, and the pinned tool environment remain only in $VDIR_HOME (owner-only)."
