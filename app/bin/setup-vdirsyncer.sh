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
VDIR_OAUTH_MODE="$VDIR_HOME/oauth-mode"
OAUTH_RELAY="$VDIR_HOME/oauth-relay"
# Dash-Go owns one isolated, pinned vdirsyncer environment. Keep the tool and
# its Python dependencies outside ~/dashboard with the private vdir state.
VDIR_PIPX_HOME="${DASH_VDIR_PIPX_HOME:-$VDIR_HOME/pipx}"
VDIR_PIPX_BIN="${DASH_VDIR_PIPX_BIN:-$VDIR_HOME/bin}"
VDIRSYNCER_VERSION="0.20.0"
VDIRSYNCER_BIN="${DASH_VDIRSYNCER_BIN:-$VDIR_PIPX_BIN/vdirsyncer}"
CONTROL_SERVER_BIN="${DASH_CONTROL_SERVER_BIN:-$BIN_DIR/dashboard-control-server}"
MAP="${DASH_VDIR_MAP:-$VDIR_HOME/calendars.map}"
WRITEBACK_REGISTRY="$CONFIG_DIR/calendar-writeback.json"
SYNC_LOG="$LOG_DIR/vdir-sync.log"

# The generated wrapper calls the known Dash-Go-managed executable directly;
# keep its bin directory first only for interactive diagnostics and helpers.
export PATH="$VDIR_PIPX_BIN:$PATH"
umask 077

# --refresh is intentionally non-interactive. It regenerates only Dash-Go's
# generated vdirsyncer config, targeted sync wrapper, and writeback registry
# from existing private state. It never discovers remote collections, starts
# OAuth, runs a sync, changes cron, or changes selected calendars.
REFRESH_ONLY=0
AUTHORIZE_ONLY=0
case "${1:-}" in
  "") ;;
  --refresh) REFRESH_ONLY=1 ;;
  --authorize) AUTHORIZE_ONLY=1 ;;
  --help|-h)
    cat <<'USAGE'
Usage: setup-vdirsyncer.sh [--refresh|--authorize]

Without arguments, add a private CalDAV or Google calendar interactively.
--refresh regenerates Dash-Go-managed configuration from saved private state
without contacting a provider or changing selected calendars.
--authorize re-runs one-time Google authorization for saved connections, then
performs their normal one-time discovery without adding calendars or syncing.
USAGE
    exit 0
    ;;
  *) printf 'Unknown option: %s\n' "$1" >&2; exit 2 ;;
esac

say(){ printf '\n\033[1;36m== %s\033[0m\n' "$*"; }
warn(){ printf '\033[1;33m!! %s\033[0m\n' "$*"; }
ok(){ printf '\033[1;32m   %s\033[0m\n' "$*"; }
have(){ command -v "$1" >/dev/null 2>&1; }
cleanup_oauth_relay(){ rm -f "$OAUTH_RELAY/pending" "$OAUTH_RELAY/result" "$OAUTH_RELAY/display.json" "$OAUTH_RELAY/qr.png" 2>/dev/null || true; }
trap cleanup_oauth_relay EXIT HUP INT TERM

mkdir -p "$DASH" "$BIN_DIR" "$CAL_DIR" "$CONFIG_DIR" "$LOG_DIR" "$VDIR_HOME" "$VDIR_STATUS" "$VDIR_COLLECTIONS" "$VDIR_PASSWORDS" "$GOOGLE_TOKENS" "$VDIR_OAUTH_MODE" "$OAUTH_RELAY" "$VDIR_PIPX_HOME" "$VDIR_PIPX_BIN"
chmod 700 "$VDIR_HOME" "$VDIR_STATUS" "$VDIR_COLLECTIONS" "$VDIR_PASSWORDS" "$GOOGLE_TOKENS" "$VDIR_OAUTH_MODE" "$OAUTH_RELAY" "$VDIR_PIPX_HOME" "$VDIR_PIPX_BIN" 2>/dev/null || true
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
  case "$1" in *$'\n'*|*$'\r'*|*$'\t'*|*'|'*) return 1;; esac
  return 0
}
valid_caldav_url(){
  valid_single_line "$1" && printf '%s' "$1" | grep -qE '^https?://[^[:space:]]+$'
}
valid_collection_id(){
  # Provider collection IDs are opaque data in the pair/config files, never
  # filesystem components. CalDAV servers may use URL-like names, so allow
  # slash/colon/etc. while rejecting delimiters and control characters.
  [ -z "$1" ] && return 0
  [ "${#1}" -le 512 ] && valid_single_line "$1"
}
valid_local_collection_key(){
  [ -n "$1" ] && [ "${#1}" -le 160 ] && valid_single_line "$1" && case "$1" in */*|*\\*|*..*) false;; *) true;; esac
}
valid_client_id(){
  [ -n "$1" ] && valid_single_line "$1" && printf '%s' "$1" | grep -qE '^[A-Za-z0-9._-]+$'
}
valid_oauth_redirect_uri(){
  valid_single_line "$1" && printf '%s' "$1" | grep -qE '^https://[^[:space:]/?#]+(?::[0-9]{1,5})?/oauth/google/callback$'
}
oauth_mode_for_ref(){
  local ref="$1" mode
  mode="$(cat "$VDIR_OAUTH_MODE/$ref" 2>/dev/null || true)"
  case "$mode" in web|desktop) printf '%s\n' "$mode";; *) printf 'desktop\n';; esac
}
oauth_redirect_for_ref(){
  local ref="$1" uri
  uri="$(cat "$VDIR_OAUTH_MODE/$ref.redirect-uri" 2>/dev/null || true)"
  valid_oauth_redirect_uri "$uri" && printf '%s\n' "$uri"
}
oauth_web_callback_ready(){
  have curl || return 1
  curl --connect-timeout 2 --max-time 4 -fsS "${DASH_OAUTH_CONTROL_URL:-http://127.0.0.1:8090}/api/ready" >/dev/null 2>&1
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
    rm -f "$VDIR_PASSWORDS/$name" "$VDIR_PASSWORDS/$name.google-client-secret" "$GOOGLE_TOKENS/$name.json" "$VDIR_OAUTH_MODE/$name" "$VDIR_OAUTH_MODE/$name.redirect-uri" 2>/dev/null || true
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
apt_codename(){
  local os_release
  if [ -n "${DASH_VDIR_APT_CODENAME:-}" ]; then
    printf '%s\n' "$DASH_VDIR_APT_CODENAME"
    return 0
  fi
  os_release="${DASH_VDIR_OS_RELEASE:-/etc/os-release}"
  [ -r "$os_release" ] || return 0
  sed -nE 's/^VERSION_CODENAME=//p; s/^DEBIAN_CODENAME=//p' "$os_release" | head -n1 | tr -d '"'
}
ensure_pipx(){
  local codename
  if have pipx; then return 0; fi
  if ! have apt-get; then
    warn "pipx is required for Dash-Go private calendar sync. Install pipx with this system's native package manager, then re-run setup."
    return 1
  fi
  echo "  Dash-Go installs pipx through APT, then keeps vdirsyncer isolated under $VDIR_HOME."
  read -rp "  Install pipx now? [Y/n]: " install_pipx
  case "${install_pipx:-y}" in n|N|no|NO) warn "pipx is required before private calendar sync can be configured"; return 1;; esac
  have sudo || { warn "sudo is required to install pipx with APT"; return 1; }
  codename="$(apt_codename)"
  if [ "$codename" = "bullseye" ]; then
    echo "  Debian Bullseye uses pipx and python3-venv from bullseye-backports."
    sudo apt-get update && sudo apt-get install -y -t bullseye-backports pipx python3-venv || { warn "could not install pipx and python3-venv from bullseye-backports"; return 1; }
  else
    sudo apt-get update && sudo apt-get install -y pipx python3-venv || { warn "could not install pipx and python3-venv through APT"; return 1; }
  fi
  have pipx || { warn "pipx was installed but is not available on PATH; re-open the terminal and re-run setup"; return 1; }
  have qrencode || echo "  Optional: install qrencode to show a Google sign-in QR code on the dashboard display."
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
  local temp name color tag pair ignored_path url username remote_id provider client_id display_name credential_ref local_id remote local_path coll_spec
  temp="$(mktemp)" || return 1
  {
    printf '[general]\n'
    printf 'status_path = %s\n\n' "$(toml_quote "$VDIR_STATUS")"
    while IFS='|' read -r name color tag pair ignored_path url username remote_id provider client_id display_name credential_ref local_id _; do
      [ -n "$name" ] || continue
      [ -n "$provider" ] || provider="caldav"
      [ -n "$credential_ref" ] || credential_ref="$name"
      [ -n "$display_name" ] || display_name="$name"
      valid_name "$name" && valid_name "$pair" || { warn "invalid saved private calendar row '$name'; skipped" >&2; continue; }
      if [ -n "$remote_id" ]; then
        [ -n "$local_id" ] || local_id="$remote_id"
        valid_collection_id "$remote_id" && valid_local_collection_key "$local_id" || { warn "invalid saved exact private calendar row '$name'; skipped" >&2; continue; }
        # Use an explicit three-part mapping: display label, opaque remote ID,
        # and safe local vdir directory. toml_quote keeps the generated config
        # data-only even when a provider display name contains punctuation.
        coll_spec="[[ $(toml_quote "$display_name"), $(toml_quote "$remote_id"), $(toml_quote "$local_id") ]]"
      else
        # Legacy broad mirrors are deliberately retained as display-only
        # discovery mirrors. They have no single local collection directory,
        # so they must not be treated as writeback candidates.
        coll_spec='["from a"]'
      fi
      remote="${pair}_remote"
      local_path="${pair}_local"
      printf '[pair %s]\n' "$pair"
      printf 'a = %s\n' "$(toml_quote "$remote")"
      printf 'b = %s\n' "$(toml_quote "$local_path")"
      printf 'collections = %s\n\n' "$coll_spec"

      printf '[storage %s]\n' "$remote"
      if [ "$provider" = "google" ]; then
        printf 'type = "google_calendar"\n'
        printf 'token_file = %s\n' "$(toml_quote "$GOOGLE_TOKENS/$credential_ref.json")"
        printf 'client_id = %s\n' "$(toml_quote "$client_id")"
        printf 'client_secret.fetch = ["command", "cat", %s]\n\n' "$(toml_quote "$VDIR_PASSWORDS/$credential_ref.google-client-secret")"
      else
        printf 'type = "caldav"\n'
        printf 'url = %s\n' "$(toml_quote "$url")"
        printf 'username = %s\n' "$(toml_quote "$username")"
        printf 'password.fetch = ["command", "cat", %s]\n\n' "$(toml_quote "$VDIR_PASSWORDS/$credential_ref")"
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
  local temp first name color tag pair collection writable remote_id display_name provider connection local_id source exact enabled require_pin row_enabled previous disabled_sources
  enabled=false
  require_pin=false
  disabled_sources="|"
  # Rebuild source metadata without resetting the user's edit/PIN choices.
  # The dashboard server rewrites this file with indented JSON, so previous
  # per-calendar flags are read with a JSON parser; the whitespace-tolerant
  # greps remain only as a fallback for a python3-free environment.
  if [ -r "$WRITEBACK_REGISTRY" ]; then
    enabled="$(grep -Eo '"enabled"[[:space:]]*:[[:space:]]*(true|false)' "$WRITEBACK_REGISTRY" | head -n1 | sed -E 's/.*(true|false)$/\1/' || true)"
    require_pin="$(grep -Eo '"requirePin"[[:space:]]*:[[:space:]]*(true|false)' "$WRITEBACK_REGISTRY" | head -n1 | sed -E 's/.*(true|false)$/\1/' || true)"
    if have python3; then
      previous="$(python3 - "$WRITEBACK_REGISTRY" <<'PYREG' 2>/dev/null || true
import json,sys
try:
    reg=json.load(open(sys.argv[1]))
except Exception:
    sys.exit(0)
print("master_enabled=%s" % ("true" if reg.get("enabled") is True else "false"))
print("require_pin=%s" % ("true" if reg.get("requirePin") is True else "false"))
for cal in reg.get("calendars") or []:
    if isinstance(cal,dict) and cal.get("enabled") is False:
        source=str(cal.get("source",""))
        if source and "|" not in source and "\n" not in source:
            print("disabled=%s" % source)
PYREG
)"
      if [ -n "$previous" ]; then
        enabled="$(printf '%s\n' "$previous" | sed -n 's/^master_enabled=//p' | head -n1)"
        require_pin="$(printf '%s\n' "$previous" | sed -n 's/^require_pin=//p' | head -n1)"
        while IFS= read -r source; do
          [ -n "$source" ] && disabled_sources="${disabled_sources}${source}|"
        done <<EOF
$(printf '%s\n' "$previous" | sed -n 's/^disabled=//p')
EOF
      fi
    fi
  fi
  case "$enabled" in true|false) ;; *) enabled=false;; esac
  case "$require_pin" in true|false) ;; *) require_pin=false;; esac
  temp="$(mktemp)" || return 1
  first=1
  {
    printf '{\n  "version": 2,\n  "enabled": %s,\n  "requirePin": %s,\n  "calendars": [' "$enabled" "$require_pin"
    while IFS='|' read -r name color tag pair collection writable remote_id display_name provider connection local_id _; do
      [ -n "$name" ] || continue
      [ "$writable" = "1" ] && [ -n "$remote_id" ] || continue
      [ -n "$display_name" ] || display_name="$name"
      [ -n "$provider" ] || provider="caldav"
      [ -n "$connection" ] || connection="$name"
      [ -n "$local_id" ] || local_id="$remote_id"
      valid_local_collection_key "$local_id" || { warn "private collection $name has no safe local vdir key; leaving Dashboard edits off" >&2; continue; }
      exact="$collection/$local_id"
      if [ ! -d "$exact" ]; then
        warn "private collection $name was not materialized as one exact vdir; leaving Dashboard edits off" >&2
        continue
      fi
      if [ -n "$tag" ]; then source="calendars/$name.$color.$tag.ics"; else source="calendars/$name.$color.ics"; fi
      row_enabled=true
      case "$disabled_sources" in *"|$source|"*) row_enabled=false;; esac
      if [ "$row_enabled" = "true" ] && [ "$disabled_sources" = "|" ] && [ -r "$WRITEBACK_REGISTRY" ] && ! have python3; then
        previous="$(grep -F "\"source\":\"$source\"" "$WRITEBACK_REGISTRY" | head -n1 || true)"
        case "$previous" in *'"enabled":false'*) row_enabled=false;; esac
      fi
      [ "$first" -eq 1 ] || printf ','
      first=0
      esc(){ local v="$1"; v="${v//\\/\\\\}"; v="${v//\"/\\\"}"; printf '%s' "$v"; }
      printf '\n    {"source":"%s","collection":"%s","writable":true,"enabled":%s,"name":"%s","pair":"%s","provider":"%s","connection":"%s","remoteId":"%s"}' \
        "$(esc "$source")" "$(esc "$exact")" "$row_enabled" "$(esc "$display_name")" "$(esc "$pair")" "$(esc "$provider")" "$(esc "$connection")" "$(esc "$remote_id")"
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
# Generated by setup-vdirsyncer.sh. It synchronizes only explicitly selected
# private pairs, replaces a mirror only after that pair succeeds, and supports
# one targeted post-edit pair without turning routine cron into discovery.
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
LOWPRIO="\$BIN_DIR/dashboard-lowprio.sh"
export VDIRSYNCER_CONFIG="\$VDIR_CFG"
umask 077
mkdir -p "\$CAL_DIR" "\$LOG_DIR" "\$VDIR_HOME"
log(){ printf '%s %s\\n' "\$(date '+%Y-%m-%d %H:%M:%S')" "\$*" >> "\$LOG"; }

# Preserve the original command line across the low-priority re-exec below.
# Parsing shifts positional arguments, so reusing $@ after validation would
# silently turn a targeted sync or one-shot conflict resolution into a full run.
ORIGINAL_ARGS=("\$@")
TARGET_PAIR=""
ALLOW_EMPTY_ONCE=0
RESOLVE_CONFLICT=""
while [ "\$#" -gt 0 ]; do
  case "\$1" in
    --pair)
      [ "\$#" -ge 2 ] || { log 'invalid targeted sync request'; exit 2; }
      TARGET_PAIR="\$2"; shift 2
      ;;
    --allow-empty-once)
      ALLOW_EMPTY_ONCE=1; shift
      ;;
    --resolve-conflict)
      [ "\$#" -ge 2 ] || { log 'invalid conflict-resolution request'; exit 2; }
      RESOLVE_CONFLICT="\$2"; shift 2
      ;;
    *) log 'invalid sync-vdir arguments'; exit 2;;
  esac
done
case "\$TARGET_PAIR" in *[!A-Za-z0-9_-]*|'')
  if [ -n "\$TARGET_PAIR" ]; then log 'invalid targeted sync pair'; exit 2; fi
  ;;
esac
case "\$RESOLVE_CONFLICT" in ""|remote|dashboard) ;; *) log 'invalid conflict-resolution winner'; exit 2;; esac
[ -z "\$RESOLVE_CONFLICT" ] || [ -n "\$TARGET_PAIR" ] || { log 'conflict resolution requires one selected pair'; exit 2; }
[ "\$ALLOW_EMPTY_ONCE" -eq 0 ] || [ -n "\$TARGET_PAIR" ] || { log 'empty-collection permission requires one selected pair'; exit 2; }

# Every entry point enters through the shared low-priority helper. The marker
# prevents recursion after dashboard-lowprio.sh execs this script.
if [ "\${DASH_VDIR_LOWPRIO_ACTIVE:-}" != "1" ] && [ -x "\$LOWPRIO" ]; then
  export DASH_VDIR_LOWPRIO_ACTIVE=1
  exec "\$LOWPRIO" "\$0" "\${ORIGINAL_ARGS[@]}"
fi

acquire_lock(){
  if mkdir "\$LOCK_DIR" 2>/dev/null; then echo "\$\$" > "\$LOCK_DIR/pid"; return 0; fi
  if [ -r "\$LOCK_DIR/pid" ]; then
    pid="\$(cat "\$LOCK_DIR/pid" 2>/dev/null || true)"
    case "\$pid" in ''|*[!0-9]*) ;; *) if kill -0 "\$pid" 2>/dev/null; then log 'sync already running; skipped overlapping run'; return 1; fi;; esac
  fi
  rm -rf "\$LOCK_DIR" 2>/dev/null || return 1
  mkdir "\$LOCK_DIR" 2>/dev/null || return 1
  echo "\$\$" > "\$LOCK_DIR/pid"
}
if ! acquire_lock; then
  [ -n "\$TARGET_PAIR" ] && exit 75
  exit 0
fi
trap 'rm -rf "\$LOCK_DIR"' EXIT

[ -x "\$VDIRSYNCER_BIN" ] || { log 'Dash-Go pinned vdirsyncer is unavailable; kept existing calendar mirrors'; exit 1; }
[ -r "\$VDIR_CFG" ] && [ -r "\$MAP" ] && [ -r "\$VDIR_PAIRS" ] || { log 'vdirsyncer configuration is incomplete'; exit 1; }
run_bounded(){ if command -v timeout >/dev/null 2>&1; then timeout 600 "\$@"; else "\$@"; fi; }

declare -A PAIR_RESULT=()
known_target=0
sync_rc=0
ready_pairs=0
RESULTS="\$VDIR_HOME/last-sync-results"
RESOLVE_CONFIG=""
cleanup_resolve_config(){ [ -z "\$RESOLVE_CONFIG" ] || rm -f "\$RESOLVE_CONFIG"; }
trap 'cleanup_resolve_config; rm -rf "\$LOCK_DIR"' EXIT

# Conflict resolution is an explicit, one-run recovery operation. Normal pair
# configuration intentionally contains no conflict winner. This copy injects a
# vdirsyncer-native policy into only the selected pair and is deleted by trap
# on every success, failure, and interruption path.
make_resolve_config(){
  winner="\$1"; pair="\$2"
  RESOLVE_CONFIG="\$(mktemp "\$VDIR_HOME/resolve-vdir.XXXXXX")" || return 1
  case "\$winner" in
    remote) policy="a wins";;
    dashboard) policy="b wins";;
    *) rm -f "\$RESOLVE_CONFIG"; RESOLVE_CONFIG=""; return 1;;
  esac
  # Every line of the user-owned config is preserved, including section
  # headers. The policy line is inserted immediately after the selected pair's
  # own header, and any conflict_resolution already inside that one section is
  # dropped so the injected policy is authoritative for this run only. The awk
  # exit status confirms the exact pair section exists before the temporary
  # config is ever passed to vdirsyncer.
  if ! awk -v want="\$pair" -v policy="\$policy" '
    /^\[/ { inwant = (\$0 == "[pair " want "]") }
    inwant && !/^\[/ && /^conflict_resolution[[:space:]]*=/ { next }
    { print }
    \$0 == "[pair " want "]" { print "conflict_resolution = \"" policy "\""; seen = 1 }
    END { exit seen ? 0 : 1 }
  ' "\$VDIR_CFG" > "\$RESOLVE_CONFIG"; then
    rm -f "\$RESOLVE_CONFIG"; RESOLVE_CONFIG=""; return 1
  fi
  chmod 600 "\$RESOLVE_CONFIG" || { rm -f "\$RESOLVE_CONFIG"; RESOLVE_CONFIG=""; return 1; }
}

# Failures are classified from this pair's own captured output so the
# dashboard can state what actually happened instead of a generic retry
# promise. Classification is conservative: anything unrecognized is 'failed'.
classify_failure(){
  if grep -qi 'changed on both sides' "\$1"; then printf 'conflict'; return; fi
  if grep -qi 'completely emptied' "\$1"; then printf 'attention-empty'; return; fi
  if grep -qiE 'run .vdirsyncer discover' "\$1"; then printf 'attention-undiscovered'; return; fi
  if grep -qiE '401|unauthorized|invalid_grant' "\$1"; then printf 'attention-auth'; return; fi
  printf 'failed'
}
record_result(){
  # RESULT lines on stdout feed the dashboard's queued targeted sync; the
  # results file makes cron-run outcomes visible to Dashboard Control too.
  PAIR_RESULT["\$1"]="\$2"
  printf 'RESULT\t%s\t%s\n' "\$1" "\$2"
  printf '%s|%s|%s\n' "\$1" "\$2" "\$(date +%s)" >> "\$RESULTS.tmp"
}
# Preserve last-known results for pairs a targeted run does not touch.
: > "\$RESULTS.tmp"
if [ -r "\$RESULTS" ]; then
  while IFS='|' read -r rpair rstate repoch _; do
    [ -n "\$rpair" ] || continue
    [ -n "\$TARGET_PAIR" ] && [ "\$rpair" != "\$TARGET_PAIR" ] || continue
    printf '%s|%s|%s\n' "\$rpair" "\$rstate" "\$repoch" >> "\$RESULTS.tmp"
  done < "\$RESULTS"
fi

while IFS='|' read -r pname _ _ ppair _ _ _ _ pprovider _ _ credential_ref _; do
  [ -n "\$pname" ] || continue
  [ -n "\$ppair" ] || continue
  [ -z "\$TARGET_PAIR" ] || [ "\$ppair" = "\$TARGET_PAIR" ] || continue
  [ "\$ppair" = "\$TARGET_PAIR" ] && known_target=1
  [ -n "\$credential_ref" ] || credential_ref="\$pname"
  if [ "\$pprovider" = "google" ] && [ ! -s "\$GOOGLE_TOKENS/\$credential_ref.json" ]; then
    record_result "\$ppair" "skipped"
    log "google calendar \$pname awaits its one-time authorization; skipped this run"
    continue
  fi
  ready_pairs=\$((ready_pairs + 1))
  sync_args=(sync "\$ppair")
  if [ "\$ALLOW_EMPTY_ONCE" -eq 1 ] && [ "\$ppair" = "\$TARGET_PAIR" ]; then
    sync_args=(sync --force-delete "\$ppair")
  fi
  config_for_pair="\$VDIR_CFG"
  if [ -n "\$RESOLVE_CONFLICT" ] && [ "\$ppair" = "\$TARGET_PAIR" ]; then
    if ! make_resolve_config "\$RESOLVE_CONFLICT" "\$ppair"; then
      record_result "\$ppair" "failed"
      sync_rc=1
      log "could not prepare one-run conflict resolution for \$pname; retained its previous calendar data"
      continue
    fi
    config_for_pair="\$RESOLVE_CONFIG"
  fi
  CAPTURE="\$(mktemp "\$VDIR_HOME/sync-vdir.XXXXXX")" || {
    record_result "\$ppair" "failed"
    sync_rc=1
    log "could not create private-calendar sync capture for \$pname; retained its previous calendar data"
    continue
  }
  if run_bounded "\$VDIRSYNCER_BIN" -c "\$config_for_pair" "\${sync_args[@]}" > "\$CAPTURE" 2>&1; then
    record_result "\$ppair" "synced"
  else
    record_result "\$ppair" "\$(classify_failure "\$CAPTURE")"
    sync_rc=1
    log "vdirsyncer sync reported errors for \$pname; retained its previous calendar data"
  fi
  cat "\$CAPTURE" >> "\$LOG"
  rm -f "\$CAPTURE"
done < "\$VDIR_PAIRS"
mv "\$RESULTS.tmp" "\$RESULTS" 2>/dev/null || rm -f "\$RESULTS.tmp"
if [ -n "\$TARGET_PAIR" ] && [ "\$known_target" -ne 1 ]; then log "unknown selected pair \$TARGET_PAIR"; exit 2; fi
[ "\$ready_pairs" -gt 0 ] || log 'no pairs are ready to sync; retained existing local calendar data'

merge_collection(){
  src="\$1"; dest="\$2"; tmp="\$(mktemp "\$dest.tmp.XXXXXX")" || return 1
  if ! find "\$src" -type f -name '*.ics' -print -quit 2>/dev/null | grep -q .; then
    printf 'BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Dash-Go//vdirsyncer//EN\r\nCALSCALE:GREGORIAN\r\nEND:VCALENDAR\r\n' > "\$tmp"
    chmod 644 "\$tmp" && mv "\$tmp" "\$dest"; return 0
  fi
  {
    printf 'BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Dash-Go//vdirsyncer//EN\r\nCALSCALE:GREGORIAN\r\n'
    find "\$src" -type f -name '*.ics' -print0 2>/dev/null | LC_ALL=C sort -z | xargs -0r awk '
      { sub(/\\r\$/, "") }
      /^BEGIN:VCALENDAR/ { next }
      /^END:VCALENDAR/   { next }
      /^BEGIN:V/ { depth++; print; next }
      /^END:V/   { print; if (depth>0) depth--; next }
      depth>0 { print }
    ' | sed 's/$/\r/'
    printf 'END:VCALENDAR\r\n'
  } > "\$tmp"
  chmod 644 "\$tmp" && mv "\$tmp" "\$dest"
}

while IFS='|' read -r name color tag pair collection writable remote_id display_name provider connection local_id _; do
  [ -n "\$name" ] || continue
  [ -z "\$TARGET_PAIR" ] || [ "\$pair" = "\$TARGET_PAIR" ] || continue
  if [ -n "\$tag" ]; then dest="\$CAL_DIR/\$name.\$color.\$tag.ics"; else dest="\$CAL_DIR/\$name.\$color.ics"; fi
  src="\$collection"
  [ -z "\$remote_id" ] || src="\$collection/\${local_id:-\$remote_id}"
  case "\${PAIR_RESULT[\$pair]:-missing}" in
    synced)
      if merge_collection "\$src" "\$dest"; then log "merged \$name -> \$(basename "\$dest")"; else log "could not merge \$name; kept previous calendar file"; fi
      ;;
    skipped) log "calendar \$name awaits authorization; kept previous calendar file";;
    conflict) log "calendar \$name has a local/remote conflict; kept previous calendar file";;
    failed|attention-*) log "calendar \$name failed remote sync; kept previous calendar file";;
    *) log "calendar \$name has no eligible sync pair; kept previous calendar file";;
  esac
done < "\$MAP"

if [ -f "\$LOG" ] && [ "\$(wc -l < "\$LOG")" -gt 400 ]; then tail -n 200 "\$LOG" > "\$LOG.tmp" && mv "\$LOG.tmp" "\$LOG"; fi
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
  # sync-vdir.sh re-execs through dashboard-lowprio.sh before acquiring its
  # shared lock, so cron stays simple while every normal sync path is gentle.
  printf '*/15 * * * * %s/sync-vdir.sh >/dev/null 2>&1\n' "$BIN_DIR" >> "$cron_tmp"
  crontab "$cron_tmp"
  local rc=$?
  rm -f "$cron_tmp"
  return "$rc"
}

if [ "$REFRESH_ONLY" -eq 1 ]; then
  if ! grep -q '[^[:space:]]' "$VDIR_PAIRS"; then
    warn "no saved private calendar configuration exists"
    exit 0
  fi
  write_vdirsyncer_config || { warn "could not refresh $VDIR_CFG"; exit 1; }
  write_sync_wrapper || { warn "could not refresh $BIN_DIR/sync-vdir.sh"; exit 1; }
  write_writeback_registry || { warn "could not refresh $WRITEBACK_REGISTRY"; exit 1; }
  ok "private calendar configuration refreshed without contacting a provider"
  exit 0
fi

say "Private calendar sync via vdirsyncer"
echo "Pull calendars directly onto this device from iCloud, Nextcloud, Fastmail,"
echo "Radicale or another standard CalDAV server, or from Google Calendar via"
echo "OAuth. Credentials remain outside the dashboard webroot in $VDIR_HOME"
echo "(owner-only permissions)."
ensure_vdirsyncer || exit 0

added=0
if [ "$AUTHORIZE_ONLY" -eq 0 ]; then
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

  url=""; username=""; client_id=""; google_oauth_mode="desktop"; google_redirect_uri=""
  if [ "$provider" = "google" ]; then
    ensure_google_support || continue
    echo "    Google needs a one-time OAuth client from your own Google Cloud project."
    echo "    Client type: Desktop app uses reliable paste-back with no public callback."
    echo "    Web app enables automatic QR completion only through an exact HTTPS callback"
    echo "    (normally a local reverse proxy to Dash-Go's loopback control server)."
    read -rp "    OAuth client type [desktop/web, desktop]: " google_oauth_mode
    google_oauth_mode="${google_oauth_mode:-desktop}"
    case "$google_oauth_mode" in
      desktop|Desktop|DESKTOP) google_oauth_mode="desktop";;
      web|Web|WEB)
        google_oauth_mode="web"
        echo "    Register this exact HTTPS redirect URI before creating the Web client."
        echo "    It must end in /oauth/google/callback and reach this Dash-Go device through your HTTPS proxy."
        read -rp "    Exact HTTPS redirect URI [${DASH_OAUTH_WEB_REDIRECT_URI:-required}]: " google_redirect_uri
        google_redirect_uri="${google_redirect_uri:-${DASH_OAUTH_WEB_REDIRECT_URI:-}}"
        if ! valid_oauth_redirect_uri "$google_redirect_uri"; then warn "    web mode needs a single exact HTTPS callback ending in /oauth/google/callback; use desktop when none is configured"; continue; fi
        echo "    Register exactly: $google_redirect_uri"
        ;;
      *) warn "    choose desktop or web"; continue;;
    esac
    echo "    Enable the Google Calendar API. Set the OAuth consent screen to In production"
    echo "    for unattended calendar sync; test-only consent can expire before cron refreshes it."
    read -rp "    OAuth client ID: " client_id
    if ! valid_client_id "$client_id"; then warn "    no valid OAuth client ID given, skipping"; continue; fi
    read -rsp "    OAuth client secret: " password; echo
    if [ -z "$password" ] || ! valid_single_line "$password"; then warn "    no valid OAuth client secret given, skipping"; unset password; continue; fi
    echo "    Optionally limit to one Calendar ID (blank = sync all discovered calendars)."
    echo "    Your primary calendar's ID is your Gmail address; other calendars show"
    echo "    their ID under Google Calendar settings → Integrate calendar."
    read -rp "    Calendar ID [all]: " collection_id
    if ! valid_collection_id "$collection_id"; then warn "    calendar ID must be one line and may not contain a pipe character."; unset password; continue; fi
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
    if ! valid_collection_id "$collection_id"; then warn "    collection ID must be one line and may not contain a pipe character."; unset password; continue; fi
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
  # A legacy exact source used the remote ID as its local directory. Keep that
  # behavior for compatibility when it is filesystem-safe; new selections use
  # a generated local key instead.
  local_id="$collection_id"
  if [ -n "$local_id" ] && ! valid_local_collection_key "$local_id"; then
    local_id="local_${name}"
  fi
  mkdir -p "$collection_path"
  printf '%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s\n' "$name" "$color" "$tag" "$pair" "$collection_path" "$writable" "$collection_id" "$name" "$provider" "$name" "$local_id" >> "$MAP"
  printf '%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s\n' "$name" "$color" "$tag" "$pair" "$collection_path" "$url" "$username" "$collection_id" "$provider" "$client_id" "$name" "$name" "$local_id" >> "$VDIR_PAIRS"
  if [ "$provider" = "google" ]; then
    printf '%s' "$password" > "$VDIR_PASSWORDS/$name.google-client-secret"
    printf '%s\n' "$google_oauth_mode" > "$VDIR_OAUTH_MODE/$name"
    if [ "$google_oauth_mode" = "web" ]; then printf '%s\n' "$google_redirect_uri" > "$VDIR_OAUTH_MODE/$name.redirect-uri"; else rm -f "$VDIR_OAUTH_MODE/$name.redirect-uri"; fi
    chmod 600 "$VDIR_PASSWORDS/$name.google-client-secret" "$VDIR_OAUTH_MODE/$name" "$VDIR_OAUTH_MODE/$name.redirect-uri" 2>/dev/null || true
  else
    printf '%s' "$password" > "$VDIR_PASSWORDS/$name"
    chmod 600 "$VDIR_PASSWORDS/$name" 2>/dev/null || true
  fi
  chmod 600 "$MAP" "$VDIR_PAIRS" 2>/dev/null || true
  unset password
  added=$((added + 1))
  ok "    queued $name"
done
fi

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

# Pair discovery is intentionally setup-time work. Routine syncs have exact,
# already-configured pairs and must not repeatedly rediscover remote calendars.
# This keeps the 15-minute job bounded and prevents provider discovery traffic
# from competing with the kiosk during normal use.
SETUP_DISCOVERED_PAIRS="|"
mark_setup_discovered(){ SETUP_DISCOVERED_PAIRS="${SETUP_DISCOVERED_PAIRS}$1|"; }
was_setup_discovered(){ case "$SETUP_DISCOVERED_PAIRS" in *"|$1|"*) return 0;; esac; return 1; }

authorize_google_pairs(){
  # The pairs file is read on its own descriptor so the authorization prompt
  # below keeps reading the user's answers from stdin. Selected calendars may
  # share a connection credential; prompt once for that shared token.
  local gname gpair gprovider credential_ref client_id_for_ref answered any=0 mode redirect_uri
  local -a oauth_args
  local seen="|"
  while IFS='|' read -r gname _ _ gpair _ _ _ _ gprovider _ _ credential_ref _ <&3; do
    [ -n "$gname" ] || continue
    [ "$gprovider" = "google" ] || continue
    [ -n "$credential_ref" ] || credential_ref="$gname"
    case "$seen" in *"|$credential_ref|"*) continue;; esac
    seen="${seen}${credential_ref}|"
    [ -s "$GOOGLE_TOKENS/$credential_ref.json" ] && continue
    if [ "$any" -eq 0 ]; then
      say "Google authorization (one time per connected account)"
      echo "Desktop clients use paste-back: open the printed link on any phone or computer,"
      echo "approve it, then paste the full 127.0.0.1 callback address here."
      echo "Web clients can show a one-time QR on the dashboard display and finish automatically."
      any=1
    fi
    read -rp "  Authorize Google connection '$credential_ref' now? [Y/n]: " answered
    case "${answered:-y}" in
      n|N) warn "  skipped; '$gname' stays read-only-idle until authorized (re-run: setup-vdirsyncer.sh --authorize)"; continue;;
    esac
    [ -x "$CONTROL_SERVER_BIN" ] || { warn "  dashboard control server is unavailable; cannot authorize '$credential_ref'"; continue; }
    client_id_for_ref="$(awk -F'|' -v ref="$credential_ref" '$12==ref && $9=="google" {print $10; exit}' "$VDIR_PAIRS")"
    if [ -z "$client_id_for_ref" ]; then
      warn "  saved Google client ID for '$credential_ref' is unavailable; cannot authorize it"
      continue
    fi
    oauth_args=(--google-oauth authorize -client-id "$client_id_for_ref" -client-secret-file "$VDIR_PASSWORDS/$credential_ref.google-client-secret" -token-file "$GOOGLE_TOKENS/$credential_ref.json" -qr)
    mode="$(oauth_mode_for_ref "$credential_ref")"
    if [ "$mode" = "web" ]; then
      redirect_uri="$(oauth_redirect_for_ref "$credential_ref")"
      if [ -z "$redirect_uri" ]; then
        warn "  '$credential_ref' has no valid saved HTTPS callback; using Desktop paste-back"
      elif ! oauth_web_callback_ready; then
        warn "  dashboard control server is not ready on loopback; using Desktop paste-back"
      else
        oauth_args+=(-relay-dir "$OAUTH_RELAY" -redirect-uri "$redirect_uri")
        echo "  The Google QR will appear on the dashboard display for up to five minutes."
      fi
    fi
    if "$CONTROL_SERVER_BIN" "${oauth_args[@]}"; then
      chmod 600 "$GOOGLE_TOKENS/$credential_ref.json" 2>/dev/null || true
      ok "  Google connection '$credential_ref' authorized"
    else
      warn "  authorization for '$credential_ref' did not complete; it is skipped by sync until it does"
    fi
  done 3< "$VDIR_PAIRS"
}

discover_private_pairs(){
  # Discovery may create local collection folders only when an administrator
  # deliberately opens interactive setup. Routine sync and beta.7's Dashboard
  # Control discovery use separate paths and never call this function.
  local dname dpair dprovider credential_ref discovered=0 skipped=0
  while IFS='|' read -r dname _ _ dpair _ _ _ _ dprovider _ _ credential_ref _ <&3; do
    [ -n "$dname" ] || continue
    [ -n "$dpair" ] || continue
    [ -n "$credential_ref" ] || credential_ref="$dname"
    if [ "$dprovider" = "google" ] && [ ! -s "$GOOGLE_TOKENS/$credential_ref.json" ]; then
      warn "  '$dname' has no Google authorization yet; discovery is deferred until it is authorized"
      skipped=$((skipped + 1)); continue
    fi
    if was_setup_discovered "$dpair"; then continue; fi
    if yes | "$VDIRSYNCER_BIN" -c "$VDIR_CFG" discover "$dpair"; then
      mark_setup_discovered "$dpair"; discovered=$((discovered + 1)); ok "  discovered private collection(s) for $dname"
    else
      warn "  discovery for '$dname' did not complete; the prior local mirror is preserved and sync will still try its known pair"
    fi
  done 3< "$VDIR_PAIRS"
  [ "$discovered" -gt 0 ] && ok "discovered $discovered private pair(s)"
  [ "$skipped" -gt 0 ] && warn "$skipped Google pair(s) await authorization"
}

if [ "$AUTHORIZE_ONLY" -eq 1 ]; then
  say "Google authorization"
  authorize_google_pairs
  say "Discovering authorized private calendar collections"
  discover_private_pairs
  write_sync_wrapper || { warn "could not write $BIN_DIR/sync-vdir.sh"; exit 1; }
  write_writeback_registry || { warn "could not write calendar writeback registry"; exit 1; }
  ok "Google authorization pass complete"
  exit 0
fi

say "Discovering private calendar collections"
authorize_google_pairs
discover_private_pairs

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

say "Scheduling private calendar sync (every 15 minutes, low priority)"
if install_vdir_cron; then
  ok "cron installed"
else
  warn "cron was not installed; run $BIN_DIR/sync-vdir.sh manually or repair cron"
fi

say "Private calendar/vdirsyncer setup complete"
echo "Private calendars sync every 15 minutes into $CAL_DIR/*.ics at gentle CPU/I/O priority."
echo "Re-run setup-vdirsyncer.sh to add, replace, migrate, or discover newly created remote calendar collections."
echo "vdirsyncer $VDIRSYNCER_VERSION runs only from the isolated pipx environment under $VDIR_HOME."
echo "Credentials, tokens, vdir state, and the pinned tool environment remain only in $VDIR_HOME (owner-only)."
