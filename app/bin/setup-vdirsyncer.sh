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
OAUTH_DISPLAY_DIR="$VDIR_HOME/oauth-display"
VDIR_PENDING="$VDIR_HOME/pending-connections"
VDIR_INSTALL_METHOD="$VDIR_HOME/install-method"
VDIR_VENV="$VDIR_HOME/pip-fallback-venv"
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
# Access intent comes from the installer’s plain-language calendar choice.
# “guided” keeps the existing per-calendar confirmation for direct terminal use.
PRIVATE_ACCESS_MODE="guided"
case "${1:-}" in
  "") ;;
  --refresh) REFRESH_ONLY=1 ;;
  --authorize) AUTHORIZE_ONLY=1 ;;
  --view-only) PRIVATE_ACCESS_MODE="view_only" ;;
  --two-way) PRIVATE_ACCESS_MODE="two_way" ;;
  --help|-h)
    cat <<'USAGE'
Usage: setup-vdirsyncer.sh [--refresh|--authorize|--view-only|--two-way]

Without arguments, add a private CalDAV or Google calendar interactively.
--view-only keeps every selected private calendar visible in Dash-Go while
preventing Dash-Go from changing the provider calendar.
--two-way starts a signed-in setup intended for calendars Dash-Go may edit;
each selected calendar still receives a final confirmation.
--refresh regenerates Dash-Go-managed configuration from saved private state
without contacting a provider or changing selected calendars.
--authorize reconnects saved Google accounts, then lists their discovered
calendars without asking for names, colors, or new calendar selections.
USAGE
    exit 0
    ;;
  *) printf 'Unknown option: %s\n' "$1" >&2; exit 2 ;;
esac

say(){ printf '\n\033[1;36m== %s\033[0m\n' "$*"; }
warn(){ printf '\033[1;33m!! %s\033[0m\n' "$*"; }
ok(){ printf '\033[1;32m   %s\033[0m\n' "$*"; }
have(){ command -v "$1" >/dev/null 2>&1; }
cleanup_oauth_display(){ rm -f "$OAUTH_DISPLAY_DIR/pending" "$OAUTH_DISPLAY_DIR/display.json" "$OAUTH_DISPLAY_DIR/qr.png" 2>/dev/null || true; }
cleanup_setup_artifacts(){ private_cleanup_transaction 2>/dev/null || true; cleanup_oauth_display; [ -z "${PRIVATE_DRAFT:-}" ] || rm -rf "$PRIVATE_DRAFT" 2>/dev/null || true; PRIVATE_DRAFT=""; }
trap cleanup_setup_artifacts EXIT HUP INT TERM

mkdir -p "$DASH" "$BIN_DIR" "$CAL_DIR" "$CONFIG_DIR" "$LOG_DIR" "$VDIR_HOME" "$VDIR_STATUS" "$VDIR_COLLECTIONS" "$VDIR_PASSWORDS" "$GOOGLE_TOKENS" "$VDIR_OAUTH_MODE" "$OAUTH_DISPLAY_DIR" "$VDIR_PENDING" "$VDIR_PIPX_HOME" "$VDIR_PIPX_BIN"
chmod 700 "$VDIR_HOME" "$VDIR_STATUS" "$VDIR_COLLECTIONS" "$VDIR_PASSWORDS" "$GOOGLE_TOKENS" "$VDIR_OAUTH_MODE" "$OAUTH_DISPLAY_DIR" "$VDIR_PENDING" "$VDIR_PIPX_HOME" "$VDIR_PIPX_BIN" 2>/dev/null || true
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
    rm -f "$VDIR_PASSWORDS/$name" "$VDIR_PASSWORDS/$name.google-client-secret" "$GOOGLE_TOKENS/$name.json" "$VDIR_OAUTH_MODE/$name" 2>/dev/null || true
    if [ -n "$tag" ]; then rm -f "$CAL_DIR/$name.$color.$tag.ics"; else rm -f "$CAL_DIR/$name.$color.ics"; fi
  done < "$old"
  rm -f "$old"
  tmp="$(mktemp)" || return 1
  awk -F'|' -v target="$target" 'tolower($1) != target { print }' "$MAP" > "$tmp" && mv "$tmp" "$MAP" || { rm -f "$tmp"; return 1; }
  tmp="$(mktemp)" || return 1
  awk -F'|' -v target="$target" 'tolower($1) != target { print }' "$VDIR_PAIRS" > "$tmp" && mv "$tmp" "$VDIR_PAIRS" || { rm -f "$tmp"; return 1; }
  chmod 600 "$MAP" "$VDIR_PAIRS" 2>/dev/null || true
}

managed_vdirsyncer_path(){
  if [ -n "${DASH_VDIRSYNCER_BIN:-}" ]; then
    printf '%s\n' "$DASH_VDIRSYNCER_BIN"
    return 0
  fi
  local method user_base
  method="$(cat "$VDIR_INSTALL_METHOD" 2>/dev/null || true)"
  case "$method" in
    venv)
      printf '%s\n' "$VDIR_VENV/bin/vdirsyncer"
      return 0
      ;;
    user-pip)
      user_base="$(python3 -m site --user-base 2>/dev/null || true)"
      [ -n "$user_base" ] && printf '%s\n' "$user_base/bin/vdirsyncer"
      return 0
      ;;
  esac
  printf '%s\n' "$VDIR_PIPX_BIN/vdirsyncer"
}
refresh_vdirsyncer_bin(){ VDIRSYNCER_BIN="$(managed_vdirsyncer_path)"; export DASH_VDIRSYNCER_BIN="$VDIRSYNCER_BIN"; }
vdirsyncer_version(){
  refresh_vdirsyncer_bin
  [ -x "$VDIRSYNCER_BIN" ] || return 1
  "$VDIRSYNCER_BIN" --version 2>/dev/null | grep -Eo '[0-9]+\.[0-9]+\.[0-9]+' | head -n1
}
vdirsyncer_is_pinned(){ [ "$(vdirsyncer_version 2>/dev/null || true)" = "$VDIRSYNCER_VERSION" ]; }
pipx_run(){ PIPX_HOME="$VDIR_PIPX_HOME" PIPX_BIN_DIR="$VDIR_PIPX_BIN" pipx "$@"; }
pipx_works(){ have pipx && pipx_run --version >/dev/null 2>&1; }
python_venv_works(){ have python3 && python3 -c 'import venv' >/dev/null 2>&1; }
python_pip_works(){ have python3 && python3 -m pip --version >/dev/null 2>&1; }
apt_codename(){
  local os_release
  if [ -n "${DASH_VDIR_APT_CODENAME:-}" ]; then printf '%s\n' "$DASH_VDIR_APT_CODENAME"; return 0; fi
  os_release="${DASH_VDIR_OS_RELEASE:-/etc/os-release}"
  [ -r "$os_release" ] || return 0
  sed -nE 's/^VERSION_CODENAME=//p; s/^DEBIAN_CODENAME=//p' "$os_release" | head -n1 | tr -d '"'
}
install_pipx_apt(){
  local codename
  have apt-get || return 1
  have sudo || { warn "sudo is required to install the missing private-calendar tools"; return 1; }
  codename="$(apt_codename)"
  if [ "$codename" = "bullseye" ]; then
    echo "  This Bullseye device uses pipx and Python virtual-environment support from bullseye-backports."
    sudo apt-get update && sudo apt-get install -y -t bullseye-backports pipx python3-venv || return 1
  else
    sudo apt-get update && sudo apt-get install -y pipx python3-venv || return 1
  fi
  pipx_works
}
offer_optional_qrencode(){
  local answer
  have qrencode && return 0
  [ -e "$VDIR_HOME/qrencode-prompted" ] && return 0
  have apt-get && have sudo || return 0
  echo "  Optional: install qrencode so Google setup can show a scannable QR code on the dashboard display."
  read -rp "  Install the optional QR helper now? [Y/n]: " answer || return 0
  : > "$VDIR_HOME/qrencode-prompted"; chmod 600 "$VDIR_HOME/qrencode-prompted" 2>/dev/null || true
  case "${answer:-y}" in
    n|N|no|NO) echo "  QR setup will still work by showing a link to type on your phone."; return 0;;
  esac
  if sudo apt-get install -y qrencode; then
    ok "optional QR helper installed"
  else
    warn "qrencode could not be installed. Google setup will show a link to type on your phone instead."
  fi
}
ensure_pipx(){
  if pipx_works; then return 0; fi
  if have pipx; then warn "pipx was found but could not run successfully."; else warn "pipx is not installed on this device."; fi
  have apt-get || return 1
  read -rp "  Install or repair the private-calendar tools now? [Y/n]: " answer
  case "${answer:-y}" in n|N|no|NO) return 1;; esac
  install_pipx_apt || { warn "Dash-Go could not prepare pipx through APT."; return 1; }
  ok "pipx is installed and working"
}
write_install_method(){ printf '%s\n' "$1" > "$VDIR_INSTALL_METHOD" && chmod 600 "$VDIR_INSTALL_METHOD"; }
install_pinned_vdirsyncer_pipx(){
  mkdir -p "$VDIR_PIPX_HOME" "$VDIR_PIPX_BIN"; chmod 700 "$VDIR_PIPX_HOME" "$VDIR_PIPX_BIN" 2>/dev/null || true
  pipx_run install --force "vdirsyncer[google]==$VDIRSYNCER_VERSION" || { warn "pipx could not install vdirsyncer $VDIRSYNCER_VERSION"; return 1; }
  pipx_run pin vdirsyncer >/dev/null 2>&1 || warn "this pipx cannot record an explicit pin; Dash-Go still keeps the exact installed version"
  write_install_method pipx || return 1
  refresh_vdirsyncer_bin
  vdirsyncer_is_pinned || { warn "Dash-Go requires vdirsyncer $VDIRSYNCER_VERSION"; return 1; }
}
install_pinned_vdirsyncer_venv(){
  python_venv_works || { warn "Python virtual-environment support is unavailable."; return 1; }
  rm -rf "$VDIR_VENV"
  python3 -m venv "$VDIR_VENV" || return 1
  "$VDIR_VENV/bin/python" -m pip install --disable-pip-version-check --no-input "vdirsyncer[google]==$VDIRSYNCER_VERSION" || return 1
  write_install_method venv || return 1
  refresh_vdirsyncer_bin
  vdirsyncer_is_pinned
}
install_pinned_vdirsyncer_user_pip(){
  local user_base existing
  python_pip_works || { warn "Python's user-level pip is unavailable."; return 1; }
  user_base="$(python3 -m site --user-base 2>/dev/null || true)"
  [ -n "$user_base" ] || return 1
  existing="$user_base/bin/vdirsyncer"
  if [ -e "$existing" ]; then
    echo "  A user-level vdirsyncer already exists at $existing."
    read -rp "  Replace it with Dash-Go's pinned calendar tool? [y/N]: " replace
    case "$replace" in y|Y|yes|YES) ;; *) return 1;; esac
  fi
  python3 -m pip install --user --disable-pip-version-check --no-input --upgrade "vdirsyncer[google]==$VDIRSYNCER_VERSION" || return 1
  write_install_method user-pip || return 1
  refresh_vdirsyncer_bin
  vdirsyncer_is_pinned
}
# vdirsyncer's Google storage lives behind the optional [google] extra. Probe
# the interpreter that owns the selected Dash-Go-managed executable, not an
# unrelated system vdirsyncer on PATH.
vdirsyncer_python(){
  if [ -n "${DASH_VDIRSYNCER_PYTHON:-}" ]; then [ -x "$DASH_VDIRSYNCER_PYTHON" ] && printf '%s\n' "$DASH_VDIRSYNCER_PYTHON"; return; fi
  local script shebang index; local -a words
  refresh_vdirsyncer_bin; script="$VDIRSYNCER_BIN"; [ -x "$script" ] || return 1
  shebang="$(head -n1 "$script" 2>/dev/null)"
  case "$shebang" in
    '#!'*)
      read -r -a words <<< "${shebang#\#!}"
      case "${words[0]:-}" in
        */env)
          index=1; [ "${words[$index]:-}" = "-S" ] && index=$((index + 1))
          while [ "$index" -lt "${#words[@]}" ] && [[ "${words[$index]}" = -* ]]; do index=$((index + 1)); done
          [ "$index" -lt "${#words[@]}" ] && command -v "${words[$index]}" || return 1
          ;;
        *python*) [ -x "${words[0]}" ] && printf '%s\n' "${words[0]}" || command -v "${words[0]}" ;;
        *) command -v python3;;
      esac
      ;;
    *) command -v python3;;
  esac
}
google_support_present(){ local py; py="$(vdirsyncer_python)" || return 1; [ -x "$py" ] || return 1; "$py" -c 'import aiohttp_oauthlib' >/dev/null 2>&1; }
private_tool_ready(){
  local provider="$1"
  vdirsyncer_is_pinned || return 1
  [ "$provider" != "google" ] || google_support_present
}
install_private_calendar_tool(){
  local provider="$1" answer
  if pipx_works; then
    ok "pipx is installed and working"
    echo "  Dash-Go's private calendar component is not installed yet."
    read -rp "  Install Dash-Go's isolated calendar component now? [Y/n]: " answer
    case "${answer:-y}" in n|N|no|NO) return 1;; esac
    install_pinned_vdirsyncer_pipx
    return
  fi
  echo "  Dash-Go could not use pipx on this device."
  if ensure_pipx && install_pinned_vdirsyncer_pipx; then return 0; fi
  echo "  Dash-Go can instead create its own isolated fallback environment."
  read -rp "  Use the isolated fallback now? [Y/n]: " answer
  case "${answer:-y}" in
    y|Y|yes|YES|'') install_pinned_vdirsyncer_venv && return 0;;
  esac
  echo "  Final fallback: install the pinned calendar tool only for this user."
  echo "  This never uses sudo or changes the operating system Python."
  read -rp "  Use this final user-level pip fallback? [y/N]: " answer
  case "$answer" in y|Y|yes|YES) install_pinned_vdirsyncer_user_pip;; *) return 1;; esac
}
ensure_vdirsyncer(){
  local provider="$1"
  if private_tool_ready "$provider"; then
    if pipx_works && [ "$(cat "$VDIR_INSTALL_METHOD" 2>/dev/null || true)" != "venv" ] && [ "$(cat "$VDIR_INSTALL_METHOD" 2>/dev/null || true)" != "user-pip" ]; then
      ok "pipx is installed and working"
    fi
    ok "Dash-Go private calendar sync is ready (vdirsyncer $VDIRSYNCER_VERSION)"
    return 0
  fi
  if [ -n "${DASH_VDIRSYNCER_BIN:-}" ]; then
    warn "The explicitly selected vdirsyncer must be version $VDIRSYNCER_VERSION$( [ "$provider" = google ] && printf ' with Google support' )."
    return 1
  fi
  say "Checking private calendar tools"
  if [ -x "$VDIRSYNCER_BIN" ]; then
    warn "Dash-Go's private calendar component needs repair or Google support."
  else
    echo "  Dash-Go needs one small private-calendar component."
  fi
  install_private_calendar_tool "$provider" || { warn "Private calendar setup was not changed."; return 1; }
  private_tool_ready "$provider" || { warn "Dash-Go could not prepare the required vdirsyncer $VDIRSYNCER_VERSION environment."; return 1; }
  ok "Dash-Go private calendar sync is ready"
}
ensure_google_support(){
  if private_tool_ready google; then
    ok "Google Calendar support is ready"
    offer_optional_qrencode
    return 0
  fi
  ensure_vdirsyncer google || return 1
  offer_optional_qrencode
}
private_pair_writable(){
  # Exact selected calendar rows own the access policy. Connection rows do not:
  # one account may intentionally contain a mix of view-only and two-way calendars.
  local pair="$1"
  awk -F'|' -v pair="$pair" '$4==pair {print $6; exit}' "$MAP" 2>/dev/null || true
}

write_vdirsyncer_config(){
  local temp name color tag pair ignored_path url username remote_id provider client_id display_name credential_ref local_id remote local_path coll_spec writable
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
      writable="$(private_pair_writable "$pair")"
      printf '[pair %s]\n' "$pair"
      printf 'a = %s\n' "$(toml_quote "$remote")"
      printf 'b = %s\n' "$(toml_quote "$local_path")"
      printf 'collections = %s\n' "$coll_spec"
      # A view-only private calendar is protected in both places: Dashboard
      # Control removes it from writeback and vdirsyncer treats the provider as
      # read-only, reverting any unexpected local mirror edits.
      [ "$writable" = "1" ] || printf 'partial_sync = "revert"\n'
      printf '\n'

      printf '[storage %s]\n' "$remote"
      if [ "$provider" = "google" ]; then
        printf 'type = "google_calendar"\n'
        printf 'token_file = %s\n' "$(toml_quote "$GOOGLE_TOKENS/$credential_ref.json")"
        printf 'client_id = %s\n' "$(toml_quote "$client_id")"
        printf 'client_secret.fetch = ["command", "cat", %s]\n' "$(toml_quote "$VDIR_PASSWORDS/$credential_ref.google-client-secret")"
      else
        printf 'type = "caldav"\n'
        printf 'url = %s\n' "$(toml_quote "$url")"
        printf 'username = %s\n' "$(toml_quote "$username")"
        printf 'password.fetch = ["command", "cat", %s]\n' "$(toml_quote "$VDIR_PASSWORDS/$credential_ref")"
      fi
      [ "$writable" = "1" ] || printf 'read_only = true\n'
      printf '\n'

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


# Guided private-calendar setup keeps credentials in a private draft until a
# real discovery and first exact-calendar sync have succeeded. The existing
# Dashboard Control selection helper remains the single source of truth for
# safe exact mappings, writeback metadata, and first-sync behavior.
private_connection_ref(){
  local prefix="$1" index=1 candidate
  while :; do
    candidate="$prefix"; [ "$index" -eq 1 ] || candidate="${prefix}_${index}"
    grep -qE "^[^|]*\|[^|]*\|[^|]*\|[^|]*\|[^|]*\|[^|]*\|[^|]*\|[^|]*\|[^|]*\|[^|]*\|[^|]*\|${candidate}\|" "$VDIR_PAIRS" 2>/dev/null || { printf '%s\n' "$candidate"; return 0; }
    index=$((index + 1))
  done
}
private_connection_label(){
  case "$1" in google) printf 'Google Calendar';; icloud) printf 'Apple iCloud Calendar';; *) printf 'CalDAV calendar account';; esac
}
private_provider_code(){ case "$1" in google) printf 'google\n';; *) printf 'caldav\n';; esac; }
private_connection_prefix(){ case "$1" in google) printf 'google\n';; icloud) printf 'icloud\n';; *) printf 'caldav\n';; esac; }
private_connection_draft(){ printf '%s/%s-%s' "$VDIR_PENDING" "$1" "$$"; }
private_remove_draft(){ [ -n "${PRIVATE_DRAFT:-}" ] && rm -rf "$PRIVATE_DRAFT" 2>/dev/null || true; PRIVATE_DRAFT=""; }
private_make_draft(){
  PRIVATE_DRAFT="$(private_connection_draft "$1")" || return 1
  mkdir -p "$PRIVATE_DRAFT/passwords" "$PRIVATE_DRAFT/google-tokens" "$PRIVATE_DRAFT/collections" "$PRIVATE_DRAFT/status" || return 1
  chmod 700 "$PRIVATE_DRAFT" "$PRIVATE_DRAFT/passwords" "$PRIVATE_DRAFT/google-tokens" "$PRIVATE_DRAFT/collections" "$PRIVATE_DRAFT/status" 2>/dev/null || true
}
private_cleanup_transaction(){
  local name color tag pair collection _
  [ -n "${PRIVATE_TX:-}" ] || return 0
  [ -r "$PRIVATE_TX/pairs.before" ] && cp "$PRIVATE_TX/pairs.before" "$VDIR_PAIRS"
  [ -r "$PRIVATE_TX/map.before" ] && cp "$PRIVATE_TX/map.before" "$MAP"
  [ -r "$PRIVATE_TX/config.before" ] && cp "$PRIVATE_TX/config.before" "$VDIR_CFG" || rm -f "$VDIR_CFG"
  [ -r "$PRIVATE_TX/wrapper.before" ] && cp "$PRIVATE_TX/wrapper.before" "$BIN_DIR/sync-vdir.sh" || rm -f "$BIN_DIR/sync-vdir.sh"
  [ -r "$PRIVATE_TX/registry.before" ] && cp "$PRIVATE_TX/registry.before" "$WRITEBACK_REGISTRY" || rm -f "$WRITEBACK_REGISTRY"
  if [ -r "$PRIVATE_TX/new-names" ]; then
    while IFS='|' read -r name color tag pair collection _; do
      [ -n "$name" ] || continue
      rm -rf "$collection" 2>/dev/null || true
      rm -f "$CAL_DIR/$name".*.ics 2>/dev/null || true
    done < "$PRIVATE_TX/new-names"
  fi
  [ -n "${PRIVATE_CONNECTION_REF:-}" ] && rm -f "$VDIR_PASSWORDS/$PRIVATE_CONNECTION_REF" "$VDIR_PASSWORDS/$PRIVATE_CONNECTION_REF.google-client-secret" "$GOOGLE_TOKENS/$PRIVATE_CONNECTION_REF.json" "$VDIR_OAUTH_MODE/$PRIVATE_CONNECTION_REF" 2>/dev/null || true
  chmod 600 "$MAP" "$VDIR_PAIRS" 2>/dev/null || true
}
private_begin_transaction(){
  PRIVATE_TX="$PRIVATE_DRAFT/transaction"; mkdir -p "$PRIVATE_TX" || return 1
  cp "$VDIR_PAIRS" "$PRIVATE_TX/pairs.before" || return 1
  cp "$MAP" "$PRIVATE_TX/map.before" || return 1
  [ ! -e "$VDIR_CFG" ] || cp "$VDIR_CFG" "$PRIVATE_TX/config.before"
  [ ! -e "$BIN_DIR/sync-vdir.sh" ] || cp "$BIN_DIR/sync-vdir.sh" "$PRIVATE_TX/wrapper.before"
  [ ! -e "$WRITEBACK_REGISTRY" ] || cp "$WRITEBACK_REGISTRY" "$PRIVATE_TX/registry.before"
  : > "$PRIVATE_TX/new-names"; chmod 600 "$PRIVATE_TX"/* 2>/dev/null || true
}
private_commit_transaction(){ PRIVATE_TX=""; }
private_stage_connection_row(){
  local provider="$1" url="$2" username="$3" client_id="$4" label="$5" pair
  pair="connect_${PRIVATE_CONNECTION_REF}"
  PRIVATE_BASE_PAIR="$pair"
  printf '%s|blue||%s|%s/collections/%s|%s|%s||%s|%s|%s|%s|\n' \
    "$PRIVATE_CONNECTION_REF" "$pair" "$VDIR_HOME" "$PRIVATE_CONNECTION_REF" "$url" "$username" "$provider" "$client_id" "$label" "$PRIVATE_CONNECTION_REF" >> "$VDIR_PAIRS"
  chmod 600 "$VDIR_PAIRS"
}
private_remove_base_pair(){
  local temp
  temp="$(mktemp "$VDIR_HOME/pairs.XXXXXX")" || return 1
  awk -F'|' -v pair="$PRIVATE_BASE_PAIR" '$4 != pair {print}' "$VDIR_PAIRS" > "$temp" && mv "$temp" "$VDIR_PAIRS" || { rm -f "$temp"; return 1; }
  chmod 600 "$VDIR_PAIRS" 2>/dev/null || true
}
private_stage_secret(){
  local provider="$1" secret="$2"
  if [ "$provider" = google ]; then
    printf '%s' "$secret" > "$VDIR_PASSWORDS/$PRIVATE_CONNECTION_REF.google-client-secret"
    chmod 600 "$VDIR_PASSWORDS/$PRIVATE_CONNECTION_REF.google-client-secret"
  else
    printf '%s' "$secret" > "$VDIR_PASSWORDS/$PRIVATE_CONNECTION_REF"
    chmod 600 "$VDIR_PASSWORDS/$PRIVATE_CONNECTION_REF"
  fi
}
private_input_trim(){
  local value="$1"
  value="${value#"${value%%[![:space:]]*}"}"
  value="${value%"${value##*[![:space:]]}"}"
  printf '%s' "$value"
}
prompt_retry(){
  # prompt_retry VAR "prompt" validator "specific corrective hint" [secret]
  # Every typed field gets three focused retries. q cancels safely and never
  # discards a completed authorization merely because a later field was mistyped.
  local target="$1" prompt="$2" validator="$3" hint="$4" secret="${5:-0}" value attempt
  for attempt in 1 2 3; do
    if [ "$secret" = 1 ]; then
      read -rsp "  ${prompt} [q=cancel]: " value || return 2
      echo
    else
      read -rp "  ${prompt} [q=cancel]: " value || return 2
    fi
    value="$(private_input_trim "$value")"
    case "$value" in q|Q) return 2;; esac
    if "$validator" "$value"; then
      printf -v "$target" '%s' "$value"
      return 0
    fi
    warn "$hint"
  done
  warn "Too many attempts. No connection was added. Run $BIN_DIR/setup-vdirsyncer.sh again when you are ready."
  return 1
}
valid_nonempty_single_line(){ [ -n "$1" ] && valid_single_line "$1"; }
valid_google_client_id(){ valid_client_id "$1" && printf "%s" "$1" | grep -q "apps.googleusercontent.com$"; }
valid_color_or_blank(){ [ -z "$1" ] || valid_color "$1"; }
valid_icloud_email_input(){ valid_nonempty_single_line "$1"; }
private_icloud_secret_shape(){ printf '%s' "$1" | grep -qE '^[a-z]{4}-[a-z]{4}-[a-z]{4}-[a-z]{4}$'; }
private_google_secret_prompt(){
  local choice
  while :; do
    prompt_retry PRIVATE_SECRET "Google Client Secret" valid_nonempty_single_line "Paste the Google Client Secret, not the Client ID or project ID." 1 || return $?
    if [ "$PRIVATE_SECRET" = "$PRIVATE_CLIENT_ID" ]; then
      warn "That is the same value as the Client ID. Paste the separate Google Client Secret instead."
      continue
    fi
    if ! printf '%s' "$PRIVATE_SECRET" | grep -q '^GOCSPX-'; then
      warn "Newer Google client secrets usually start with GOCSPX-. Double-check that you copied the Client Secret."
      read -rp "  Press Enter to keep it, r to re-enter it, or q to cancel: " choice || return 2
      case "$choice" in q|Q) return 2;; r|R) continue;; esac
    fi
    return 0
  done
}
private_icloud_credentials_prompt(){
  local choice
  prompt_retry PRIVATE_USERNAME "Apple Account email" valid_icloud_email_input "Enter the email address used with your Apple Account." 0 || return $?
  if ! printf '%s' "$PRIVATE_USERNAME" | grep -q '@'; then
    warn "That does not look like an email address. Apple Account emails normally contain @."
    read -rp "  Press Enter to keep it, r to re-enter it, or q to cancel: " choice || return 2
    case "$choice" in q|Q) return 2;; r|R) private_icloud_credentials_prompt; return $?;; esac
  fi
  while :; do
    prompt_retry PRIVATE_SECRET "Apple app-specific password" valid_nonempty_single_line "Paste the app-specific password Apple generated for Dash-Go." 1 || return $?
    if ! private_icloud_secret_shape "$PRIVATE_SECRET"; then
      warn "This does not look like an Apple app-specific password. They usually look like abcd-efgh-ijkl-mnop."
      echo "  Generate one at account.apple.com → Sign-In and Security → App-Specific Passwords."
      read -rp "  Press Enter to keep it, r to re-enter it, or q to cancel: " choice || return 2
      case "$choice" in q|Q) return 2;; r|R) continue;; esac
    fi
    return 0
  done
}
private_headless_ssh(){ [ -n "${SSH_CONNECTION:-}" ] && [ -z "${DISPLAY:-}${WAYLAND_DISPLAY:-}" ]; }
private_google_signin_mode_prompt(){
  local completion="" tunnel="" default="" mode_label=""
  echo ""
  echo "Step 2 of 4 — Choose where you will sign in"
  if private_headless_ssh; then
    echo "You are connected by SSH without a local screen. A phone or tablet is the easy path."
    echo "  1) Phone or tablet — scan the dashboard QR, then paste the final browser address here (recommended)"
    echo "  2) Browser on the computer you are typing on — uses one temporary SSH tunnel (advanced)"
    default=1
  else
    echo "  1) Browser on this device — completes automatically"
    echo "  2) Phone or tablet — scan the dashboard QR, then paste the final browser address here"
    default=1
  fi
  while :; do
    read -rp "  Choose [$default, q=cancel]: " completion || return 2
    completion="$(private_input_trim "${completion:-$default}")"
    case "$completion" in
      q|Q) return 2;;
      1)
        if private_headless_ssh; then
          PRIVATE_GOOGLE_MODE="phone"
        else
          PRIVATE_GOOGLE_MODE="loopback"
          echo "Dash-Go will wait for the local browser callback on this device."
        fi
        return 0
        ;;
      2)
        if private_headless_ssh; then
          tunnel="$(private_tunnel_command || true)"
          if [ -z "$tunnel" ]; then
            warn "Dash-Go could not build the temporary SSH tunnel command. Choose the phone option instead."
            continue
          fi
          echo ""
          echo "Open a second terminal on the computer you are typing on and paste:"
          echo "  $tunnel"
          echo "Keep it open, then press Enter here. If the sign-in page never loads, choose the phone option instead."
          read -rp "  Press Enter to continue, or q to cancel: " mode_label || return 2
          mode_label="$(private_input_trim "$mode_label")"
          case "$mode_label" in q|Q) return 2;; esac
          PRIVATE_GOOGLE_MODE="loopback"
        else
          PRIVATE_GOOGLE_MODE="phone"
        fi
        return 0
        ;;
      *) warn "Choose 1 or 2. Your Google details are still waiting in this setup.";;
    esac
  done
}
private_google_prepare(){
  local ready=""
  ensure_google_support || return 1
  echo ""
  echo "Step 1 of 4 — Prepare Google Calendar"
  echo "Google requires one small setup in your own Google Cloud account. You do this once for this Dash-Go device."
  echo "  1. Enable Google Calendar API."
  echo "  2. Configure the Google consent screen."
  echo "  3. Create an OAuth Client ID."
  echo "  4. Choose application type: Desktop app."
  echo "Choose Desktop app exactly. Do not choose Web application."
  echo "For unattended household sync, leave the consent screen In production when Google permits it."
  read -rp "Press Enter when you are ready to paste the Client ID (q cancels): " ready || return 2
  case "$ready" in q|Q) return 2;; esac
  prompt_retry PRIVATE_CLIENT_ID "Google Client ID" valid_google_client_id "Paste the Google Desktop App Client ID ending in apps.googleusercontent.com." 0 || return $?
  private_google_secret_prompt || return $?
  private_google_signin_mode_prompt
}
private_icloud_prepare(){
  local ready=""
  echo ""
  echo "Step 1 of 4 — Prepare Apple iCloud Calendar"
  echo "You need the email address used with your Apple Account and a new app-specific password."
  echo "Do not enter your normal Apple Account password."
  echo "At account.apple.com: Sign-In and Security → App-Specific Passwords → Generate Password."
  read -rp "Press Enter when you have an app-specific password (q cancels): " ready || return 2
  case "$ready" in q|Q) return 2;; esac
  private_icloud_credentials_prompt || return $?
  PRIVATE_URL="https://caldav.icloud.com/"
}
private_caldav_prepare(){
  echo ""
  echo "Step 1 of 4 — Enter CalDAV account details"
  prompt_retry PRIVATE_LABEL "Account label (letters, numbers, hyphen, underscore)" valid_name "Use only letters, numbers, hyphen, and underscore for an account label." 0 || return $?
  prompt_retry PRIVATE_URL "CalDAV server URL" valid_caldav_url "Use a single-line http(s) CalDAV URL without spaces or | characters." 0 || return $?
  prompt_retry PRIVATE_USERNAME "Account username" valid_nonempty_single_line "Enter a single-line account username." 0 || return $?
  prompt_retry PRIVATE_SECRET "Account password or app-specific password" valid_nonempty_single_line "Paste the account password or app-specific password." 1 || return $?
}
private_discover_draft(){
  local out="$PRIVATE_DRAFT/discovery.out" provider="$1" rc
  if timeout 180 env DASH_VDIR_HOME="$PRIVATE_DRAFT" DASH_VDIR_PAIRS="$PRIVATE_DRAFT/pairs" DASH_VDIR_PASSWORDS="$PRIVATE_DRAFT/passwords" DASH_VDIR_GOOGLE_TOKENS="$PRIVATE_DRAFT/google-tokens" DASH_VDIRSYNCER_BIN="$VDIRSYNCER_BIN" "$BIN_DIR/private-calendar-discovery.sh" > "$out" 2>&1; then
    rc=0
  else
    rc=$?
  fi
  awk -F '\t' '$1=="calendar" && NF==7 {print}' "$out" > "$PRIVATE_DRAFT/candidates.tsv"
  if [ "$rc" -eq 124 ]; then
    warn "Calendar discovery took too long — check the network and try again; nothing was changed."
    return 1
  fi
  if [ ! -s "$PRIVATE_DRAFT/candidates.tsv" ]; then
    echo ""
    warn "Dash-Go could not list calendars for this account. Existing dashboard calendars were not changed."
    sed -n 's/^notice[[:space:]]*//p' "$out" | sed 's/^/  /' || true
    return 1
  fi
}
private_candidate_is_active(){
  local remote="$1"
  awk -F'|' -v remote="$remote" '$7==remote {found=1} END {exit found?0:1}' "$VDIR_PAIRS"
}
private_prompt_calendar_selection(){
  local total="$1" answer="" index all_answer
  while :; do
    read -rp "Enter calendar numbers to add (space separated; Enter = all; q cancels): " answer || return 2
    answer="$(private_input_trim "$answer")"
    case "$answer" in q|Q) return 2;; esac
    if [ -z "$answer" ]; then
      read -rp "Add all $total calendars? [Y/n, q=cancel]: " all_answer || return 2
      case "$all_answer" in q|Q) return 2;; n|N|no|NO) continue;; esac
      answer="$(seq 1 "$total" | tr '\n' ' ')"
    fi
    for index in $answer; do
      case "$index" in ''|*[!0-9]*) warn "Use calendar numbers separated by spaces."; continue 2;; esac
      [ "$index" -ge 1 ] && [ "$index" -le "$total" ] || { warn "Calendar number $index is not in the list. Choose a number shown above."; continue 2; }
    done
    PRIVATE_SELECTION="$answer"
    return 0
  done
}
private_choose_and_activate(){
  local provider="$1" total index pair remote display color editable selected=0 output source name line="" chosen_color="" edit_answer="" access_mode="$PRIVATE_ACCESS_MODE"
  # Google’s ordinary view-only route is a tokenized iCal link. A signed-in
  # Google connection is created for two-way sync; an existing secure source
  # can later be safety-locked view-only from Calendar Manager without forcing
  # a risky source migration during setup.
  if [ "$provider" = google ] && [ "$access_mode" = guided ]; then
    access_mode="two_way"
    echo "  Google secure sign-in is for two-way sync. For a Google calendar link that Dash-Go can only read, use installer option 9."
  fi
  total="$(wc -l < "$PRIVATE_DRAFT/candidates.tsv")"
  echo ""
  echo "Step 3 of 4 — Choose calendars"
  index=0
  while IFS=$'\t' read -r _ pair _ _ remote display color; do
    index=$((index + 1))
    if private_candidate_is_active "$remote"; then
      printf '  [%s] %s (already selected)\n' "$index" "$display"
    else
      printf '  [%s] %s\n' "$index" "$display"
    fi
  done < "$PRIVATE_DRAFT/candidates.tsv"
  private_prompt_calendar_selection "$total" || return $?
  for index in $PRIVATE_SELECTION; do
    line="$(sed -n "${index}p" "$PRIVATE_DRAFT/candidates.tsv")"
    IFS=$'\t' read -r _ pair _ _ remote display color <<< "$line"
    color="${color:-blue}"; valid_color "$color" || color="blue"
    prompt_retry chosen_color "Display color for $display [$color]" valid_color_or_blank "Use a palette color or six-digit hex value." 0 || return $?
    chosen_color="${chosen_color:-$color}"
    case "$access_mode" in
      view_only)
        editable=0
        echo "  $display will be view-only. Dash-Go will not send event changes back."
        ;;
      two_way)
        if [ "$provider" = google ]; then
          editable=1
          echo "  $display will use two-way sync. For a Google calendar link that Dash-Go can only read, use installer option 9."
        else
          while :; do
            read -rp "  Enable two-way sync for $display? [Y/n, q=cancel]: " edit_answer || return 2
            edit_answer="$(private_input_trim "$edit_answer")"
            case "$edit_answer" in q|Q) return 2;; n|N|no|NO) editable=0; echo "  $display will stay view-only."; break;; y|Y|yes|YES|'') editable=1; break;; *) warn "Press Enter for two-way sync, n for view-only, or q to cancel.";; esac
          done
        fi
        ;;
      *)
        while :; do
          read -rp "  Allow Dash-Go to add, edit, or skip events in $display? [y/N, q=cancel]: " edit_answer || return 2
          edit_answer="$(private_input_trim "$edit_answer")"
          case "$edit_answer" in q|Q) return 2;; y|Y|yes|YES|' '| '') editable=0; case "$edit_answer" in y|Y|yes|YES) editable=1;; esac; break;; *) warn "Answer y for two-way sync or press Enter to keep $display view-only.";; esac
        done
        ;;
    esac
    output="$(DASH_VDIRSYNCER_BIN="$VDIRSYNCER_BIN" "$BIN_DIR/private-calendar-selection.sh" --activate "$PRIVATE_BASE_PAIR" "$remote" "$display" "$chosen_color" "$editable" 2>&1)" || { warn "Could not activate $display safely. Try again from Calendar Manager."; printf '%s\n' "$output" | sed 's/^/  /'; return 1; }
    case "$output" in
      activated$'\t'*)
        IFS=$'\t' read -r _ source _ _ _ _ initial <<< "$output"
        if [ "$initial" != ready ]; then warn "$display did not complete its first sync. No new calendars were activated. Re-run $BIN_DIR/setup-vdirsyncer.sh --authorize or retry from Calendar Manager."; return 1; fi
        name="$(awk -F'|' -v source="$source" 'function sf(n,c,t){return t!=""?"calendars/"n"."c"."t".ics":"calendars/"n"."c".ics"} sf($1,$2,$3)==source{print $1"|"$2"|"$3"|"$4"|"$5; exit}' "$MAP")"
        [ -n "$name" ] && printf '%s\n' "$name" >> "$PRIVATE_TX/new-names"
        selected=$((selected + 1))
        ;;
      existing$'\t'*) warn "$display is already selected; it was left unchanged.";;
      *) warn "Dash-Go received an unexpected calendar selection result for $display. Retry from Calendar Manager."; return 1;;
    esac
  done
  [ "$selected" -gt 0 ] || { warn "No new calendars were selected. Existing calendars were left unchanged."; return 1; }
}
private_finish_connection(){
  private_remove_base_pair || return 1
  write_vdirsyncer_config || return 1
  write_sync_wrapper || return 1
  if ! timeout 180 "$BIN_DIR/sync-vdir.sh"; then warn "The final private-calendar sync did not finish. No new calendars were activated. Re-run $BIN_DIR/setup-vdirsyncer.sh --authorize or retry from Calendar Manager."; return 1; fi
  write_writeback_registry || return 1
  ok "first sync completed"
}
private_duplicate_connection_ref(){
  local provider="$1" username="$2" client_id="$3"
  awk -F'|' -v provider="$provider" -v username="$username" -v client_id="$client_id" '
    $9==provider && provider=="google" && $10==client_id {print ($12!=""?$12:$1); exit}
    $9=="caldav" && provider!="google" && $6=="https://caldav.icloud.com/" && $7==username {print ($12!=""?$12:$1); exit}
  ' "$VDIR_PAIRS"
}
private_refresh_caldav_saved_ref(){
  local credential_ref="$1" provider="$2" backup="" tmp
  [ -n "$credential_ref" ] || return 1
  backup="$VDIR_PASSWORDS/$credential_ref.backup.$$"
  [ -f "$VDIR_PASSWORDS/$credential_ref" ] && cp -p "$VDIR_PASSWORDS/$credential_ref" "$backup" || true
  printf '%s' "$PRIVATE_SECRET" > "$VDIR_PASSWORDS/$credential_ref" || { rm -f "$backup"; return 1; }
  chmod 600 "$VDIR_PASSWORDS/$credential_ref"
  if timeout 180 "$BIN_DIR/sync-vdir.sh"; then
    rm -f "$backup"
    ok "saved a fresh password for the existing $(private_connection_label "$provider") account"
    return 0
  fi
  [ -f "$backup" ] && mv -f "$backup" "$VDIR_PASSWORDS/$credential_ref" || rm -f "$VDIR_PASSWORDS/$credential_ref"
  warn "The new password did not complete a sync. Your previous private-calendar password was restored. Try again from Calendar Manager."
  return 1
}
private_connect_account(){
  local choice provider label duplicate reconnect rc
  say "Connect a private calendar account"
  echo "  1) Google Calendar"
  echo "  2) Apple iCloud Calendar"
  echo "  3) Another CalDAV account — Nextcloud, Fastmail, Radicale, and similar services"
  while :; do
    read -rp "Choose [1] (blank to finish, q cancels): " choice || return 2
    choice="$(private_input_trim "$choice")"
    case "$choice" in ''|q|Q) return 2;; 1) provider=google; break;; 2) provider=icloud; break;; 3) provider=caldav; break;; *) warn "Choose 1, 2, or 3.";; esac
  done
  if [ "$provider" = google ] && [ "$PRIVATE_ACCESS_MODE" = view_only ]; then
    warn "Google view-only calendars use a calendar link. Return to the installer and choose option 9: Read-only calendar link."
    return 2
  fi
  ensure_vdirsyncer "$(private_provider_code "$provider")" || return 1
  PRIVATE_SECRET=""; PRIVATE_USERNAME=""; PRIVATE_URL=""; PRIVATE_CLIENT_ID=""; PRIVATE_LABEL="$(private_connection_label "$provider")"
  case "$provider" in google) private_google_prepare || return $?;; icloud) private_icloud_prepare || return $?;; caldav) private_caldav_prepare || return $?;; esac
  duplicate="$(private_duplicate_connection_ref "$provider" "$PRIVATE_USERNAME" "$PRIVATE_CLIENT_ID")"
  if [ -n "$duplicate" ]; then
    echo "This account looks already connected as '$duplicate'."
    read -rp "Reconnect it instead? [Y/n, q=cancel]: " reconnect || return 2
    case "$reconnect" in q|Q) return 2;; n|N|no|NO) warn "Existing account was left unchanged."; return 2;; esac
    if [ "$provider" = google ]; then
      private_google_authorize_saved_ref "$duplicate" "$PRIVATE_CLIENT_ID"
      return $?
    fi
    private_refresh_caldav_saved_ref "$duplicate" "$provider"
    return $?
  fi
  PRIVATE_CONNECTION_REF="$(private_connection_ref "$(private_connection_prefix "$provider")")"
  private_make_draft "$PRIVATE_CONNECTION_REF" || return 1
  if [ "$provider" = google ]; then
    printf '%s' "$PRIVATE_SECRET" > "$PRIVATE_DRAFT/passwords/$PRIVATE_CONNECTION_REF.google-client-secret"; chmod 600 "$PRIVATE_DRAFT/passwords/$PRIVATE_CONNECTION_REF.google-client-secret"
    printf '%s\n' "$PRIVATE_CONNECTION_REF|blue||connect_$PRIVATE_CONNECTION_REF|$PRIVATE_DRAFT/collections/$PRIVATE_CONNECTION_REF||||google|$PRIVATE_CLIENT_ID|$PRIVATE_LABEL|$PRIVATE_CONNECTION_REF|" > "$PRIVATE_DRAFT/pairs"
    chmod 600 "$PRIVATE_DRAFT/pairs"
    say "Step 3 of 4 — Sign in to Google"
    local oauth_args=(--google-oauth authorize -client-id "$PRIVATE_CLIENT_ID" -client-secret-file "$PRIVATE_DRAFT/passwords/$PRIVATE_CONNECTION_REF.google-client-secret" -token-file "$PRIVATE_DRAFT/google-tokens/$PRIVATE_CONNECTION_REF.json" -qr)
    if [ "$PRIVATE_GOOGLE_MODE" = loopback ]; then oauth_args+=(-loopback); else oauth_args+=(-display-dir "$OAUTH_DISPLAY_DIR"); fi
    [ -x "$CONTROL_SERVER_BIN" ] || { warn "dashboard control server is unavailable; no calendars were added. Re-run $BIN_DIR/setup-vdirsyncer.sh after Dashboard Control is repaired."; private_remove_draft; return 1; }
    "$CONTROL_SERVER_BIN" "${oauth_args[@]}" || { rc=$?; warn "Google sign-in did not complete. No calendars were added. Re-run $BIN_DIR/setup-vdirsyncer.sh --authorize when ready."; private_remove_draft; return "$rc"; }
    PRIVATE_URL=""; PRIVATE_USERNAME=""; label="Google Calendar"
  else
    label="$(private_connection_label "$provider")"
    printf '%s' "$PRIVATE_SECRET" > "$PRIVATE_DRAFT/passwords/$PRIVATE_CONNECTION_REF"; chmod 600 "$PRIVATE_DRAFT/passwords/$PRIVATE_CONNECTION_REF"
    printf '%s\n' "$PRIVATE_CONNECTION_REF|blue||connect_$PRIVATE_CONNECTION_REF|$PRIVATE_DRAFT/collections/$PRIVATE_CONNECTION_REF|$PRIVATE_URL|$PRIVATE_USERNAME||caldav||$PRIVATE_LABEL|$PRIVATE_CONNECTION_REF|" > "$PRIVATE_DRAFT/pairs"
    chmod 600 "$PRIVATE_DRAFT/pairs"
  fi
  echo ""
  echo "Checking this account and looking for calendars…"
  private_discover_draft "$provider" || { private_remove_draft; return 1; }
  private_begin_transaction || { private_remove_draft; return 1; }
  if [ "$provider" = google ]; then
    cp "$PRIVATE_DRAFT/passwords/$PRIVATE_CONNECTION_REF.google-client-secret" "$VDIR_PASSWORDS/$PRIVATE_CONNECTION_REF.google-client-secret"; cp "$PRIVATE_DRAFT/google-tokens/$PRIVATE_CONNECTION_REF.json" "$GOOGLE_TOKENS/$PRIVATE_CONNECTION_REF.json"; chmod 600 "$VDIR_PASSWORDS/$PRIVATE_CONNECTION_REF.google-client-secret" "$GOOGLE_TOKENS/$PRIVATE_CONNECTION_REF.json"; private_stage_connection_row google "" "" "$PRIVATE_CLIENT_ID" "$label"; printf 'desktop\n' > "$VDIR_OAUTH_MODE/$PRIVATE_CONNECTION_REF"; chmod 600 "$VDIR_OAUTH_MODE/$PRIVATE_CONNECTION_REF"
  else
    cp "$PRIVATE_DRAFT/passwords/$PRIVATE_CONNECTION_REF" "$VDIR_PASSWORDS/$PRIVATE_CONNECTION_REF"; chmod 600 "$VDIR_PASSWORDS/$PRIVATE_CONNECTION_REF"; private_stage_connection_row caldav "$PRIVATE_URL" "$PRIVATE_USERNAME" "" "$label"
  fi
  write_vdirsyncer_config || { private_cleanup_transaction; private_remove_draft; return 1; }
  private_choose_and_activate "$provider"; rc=$?
  if [ "$rc" -ne 0 ]; then private_cleanup_transaction; private_remove_draft; return "$rc"; fi
  private_finish_connection || { private_cleanup_transaction; private_remove_draft; return 1; }
  private_commit_transaction
  private_remove_draft
  say "Private calendar connection complete"
  echo "  Selected calendars are now syncing every 15 minutes at low priority."
  return 0
}
private_google_authorize_saved_ref(){
  local credential_ref="$1" client_id="$2" args rc
  [ -n "$credential_ref" ] && [ -n "$client_id" ] || return 1
  [ -r "$VDIR_PASSWORDS/$credential_ref.google-client-secret" ] || { warn "Google connection '$credential_ref' is missing its private client secret. Reconnect it from Calendar Manager."; return 1; }
  echo ""
  echo "Reconnect Google Calendar: $credential_ref"
  echo "Google requires the consent screen to remain In production for reliable unattended calendar sync."
  private_google_signin_mode_prompt || return $?
  args=(--google-oauth authorize -client-id "$client_id" -client-secret-file "$VDIR_PASSWORDS/$credential_ref.google-client-secret" -token-file "$GOOGLE_TOKENS/$credential_ref.json" -qr)
  if [ "$PRIVATE_GOOGLE_MODE" = loopback ]; then args+=(-loopback); else args+=(-display-dir "$OAUTH_DISPLAY_DIR"); fi
  [ -x "$CONTROL_SERVER_BIN" ] || { warn "dashboard control server is unavailable; '$credential_ref' was not changed. Repair Dashboard Control, then try again."; return 1; }
  "$CONTROL_SERVER_BIN" "${args[@]}" || { rc=$?; return "$rc"; }
}
authorize_google_pairs(){
  local name color tag pair collection writable remote_id provider client_id display_name credential_ref local_id _ seen="|" found=0 rc=0
  while IFS='|' read -r name color tag pair collection writable remote_id provider client_id display_name credential_ref local_id _; do
    [ "$provider" = google ] || continue
    [ -n "$credential_ref" ] || credential_ref="$name"
    [ -n "$client_id" ] || { warn "Google connection '$display_name' has no saved Client ID."; rc=1; continue; }
    case "$seen" in *"|$credential_ref|"*) continue;; esac
    seen="${seen}${credential_ref}|"; found=1
    private_google_authorize_saved_ref "$credential_ref" "$client_id" || {
      status=$?
      [ "$status" -eq 2 ] && return 2
      warn "Google authorization for '$display_name' did not complete; existing calendars were left unchanged."
      rc=1
    }
  done < "$VDIR_PAIRS"
  [ "$found" -eq 1 ] || { warn "no saved Google calendar connection exists"; return 1; }
  return "$rc"
}
discover_private_pairs(){
  local out
  out="$(DASH_VDIRSYNCER_BIN="$VDIRSYNCER_BIN" "$BIN_DIR/private-calendar-discovery.sh" 2>&1 || true)"
  if printf '%s\n' "$out" | grep -q $'^calendar\t'; then
    printf '%s\n' "$out" | awk -F '\t' '$1=="calendar" {printf "  • %s\n", $6}'
  else
    printf '%s\n' "$out" | sed -n 's/^notice[[:space:]]*/  /p'
  fi
}
legacy_icloud_notice(){
  grep -Fq '|https://caldav.icloud.com/|' "$VDIR_PAIRS" 2>/dev/null || return 0
  echo "  Existing iCloud connection found. It was kept exactly as it is."
  echo "  New iCloud accounts use the guided Apple setup above."
}
# Tests can source the prompt helpers without entering the interactive setup.
# Production execution never sets this flag.
if [ "${DASHGO_TEST_LIBRARY_ONLY:-0}" = "1" ]; then
  return 0 2>/dev/null || exit 0
fi
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

say "Private calendar setup"
case "$PRIVATE_ACCESS_MODE" in
  view_only) echo "Connect an iCloud or CalDAV account and keep selected calendars view-only. Google view-only calendars use installer option 9: Read-only calendar link." ;;
  two_way) echo "Connect a personal Google, Apple iCloud, or CalDAV account for two-way sync. Dash-Go will ask for a final confirmation for each calendar." ;;
  *) echo "Connect calendars that need a sign-in: Google, Apple iCloud, or another CalDAV account. Public view-only .ics links use installer option 9 instead." ;;
esac
legacy_icloud_notice

if [ "$AUTHORIZE_ONLY" -eq 1 ]; then
  ensure_vdirsyncer google || exit 0
  say "Google authorization recovery"
  authorize_google_pairs
  status=$?
  if [ "$status" -ne 0 ]; then
    [ "$status" -eq 2 ] && { warn "Google authorization recovery cancelled. Existing calendars were left unchanged."; exit 0; }
    warn "Google authorization recovery did not complete for every account. Existing calendars were left unchanged."
    exit 1
  fi
  say "Discovering authorized private calendar collections"
  discover_private_pairs
  write_vdirsyncer_config || { warn "could not refresh $VDIR_CFG"; exit 1; }
  write_sync_wrapper || { warn "could not write $BIN_DIR/sync-vdir.sh"; exit 1; }
  write_writeback_registry || { warn "could not write calendar writeback registry"; exit 1; }
  ok "Google authorization recovery complete"
  exit 0
fi

added=0
while true; do
  if private_connect_account; then
    added=$((added + 1))
  else
    rc=$?
    [ "$rc" -eq 2 ] && break
  fi
  read -rp "Connect another private calendar account? [y/N, q=finish]: " again
  case "$again" in y|Y|yes|YES) ;; *) break;; esac
done

if [ "$added" -eq 0 ]; then
  if grep -q '[^[:space:]]' "$VDIR_PAIRS"; then
    ok "No new account was added. Existing private calendar configuration was left unchanged."
  else
    warn "No private calendar account was added."
  fi
  exit 0
fi

write_sync_wrapper || { warn "could not write $BIN_DIR/sync-vdir.sh"; exit 1; }
write_writeback_registry || { warn "could not write calendar writeback registry"; exit 1; }
say "Scheduling private calendar sync (every 15 minutes, low priority)"
if install_vdir_cron; then ok "automatic private-calendar sync is enabled"; else warn "cron was not installed; run $BIN_DIR/sync-vdir.sh manually or repair cron"; fi
say "Private calendar setup complete"
echo "Your selected private calendars sync every 15 minutes into $CAL_DIR at gentle CPU/I/O priority."
echo "Credentials, tokens, drafts, and the pinned calendar tool remain only in $VDIR_HOME (owner-only)."
