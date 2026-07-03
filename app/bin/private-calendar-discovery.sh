#!/usr/bin/env bash
# Private-calendar discovery is intentionally inventory-only. It builds an
# isolated vdirsyncer config/status/filesystem under ~/.dashboard-vdirsyncer,
# runs discover + metadata sync there, prints safe TSV rows, and removes the
# staging tree. It never reads or writes the active pair config/status, mirror
# files, event cache, or cron schedule.
set -u

DASH="${DASH:-$HOME/dashboard}"
VDIR_HOME="${DASH_VDIR_HOME:-$HOME/.dashboard-vdirsyncer}"
VDIR_PAIRS="${DASH_VDIR_PAIRS:-$VDIR_HOME/pairs}"
VDIR_PASSWORDS="${DASH_VDIR_PASSWORDS:-$VDIR_HOME/passwords}"
GOOGLE_TOKENS="${DASH_VDIR_GOOGLE_TOKENS:-$VDIR_HOME/google-tokens}"
VDIRSYNCER_BIN="${DASH_VDIRSYNCER_BIN:-$VDIR_HOME/bin/vdirsyncer}"
umask 077

valid_name(){ printf '%s' "$1" | grep -qE '^[A-Za-z0-9_-]{1,96}$'; }
valid_single_line(){ case "$1" in *$'\n'*|*$'\r'*|*$'\t'*|*'|'*) return 1;; esac; return 0; }
toml_quote(){ local v="$1"; v="${v//\\/\\\\}"; v="${v//\"/\\\"}"; printf '"%s"' "$v"; }
clean_field(){ printf '%s' "$1" | tr '\t\r\n' '   ' | sed 's/^ *//;s/ *$//' ; }
notice(){ printf 'notice\t%s\n' "$(clean_field "$1")"; }

[ -x "$VDIRSYNCER_BIN" ] || { notice "Private calendar sync is unavailable."; exit 1; }
[ -r "$VDIR_PAIRS" ] || { notice "No private calendar connection is configured yet."; exit 0; }
if [ -d "$VDIR_HOME/sync.lock" ]; then
  notice "A calendar sync is already running. Try discovery again shortly."
  exit 75
fi

STAGE="$(mktemp -d "$VDIR_HOME/discovery.XXXXXX")" || exit 1
trap 'rm -rf "$STAGE"' EXIT
mkdir -p "$STAGE/status" "$STAGE/collections"
CFG="$STAGE/config"

# A connection is keyed by its private credential reference. Several selected
# Google calendars may share an OAuth token; discover it once, not once per
# selected calendar. First matching saved connection wins because a credential
# reference is an explicit Dash-Go invariant.
declare -A SEEN=()
{
  printf '[general]\nstatus_path = %s\n\n' "$(toml_quote "$STAGE/status")"
  while IFS='|' read -r name color tag pair ignored url username remote_id provider client_id display_name credential_ref local_id _; do
    [ -n "$name" ] || continue
    [ -n "$provider" ] || provider="caldav"
    [ -n "$credential_ref" ] || credential_ref="$name"
    valid_name "$credential_ref" || { notice "Saved private connection '$name' has an invalid credential reference."; continue; }
    [ -z "${SEEN[$credential_ref]:-}" ] || continue
    SEEN[$credential_ref]=1
    stage_pair="discover_${credential_ref}"
    remote="${stage_pair}_remote"
    local_store="${stage_pair}_local"
    printf '[pair %s]\n' "$stage_pair"
    printf 'a = %s\nb = %s\ncollections = ["from a"]\nmetadata = ["displayname", "color"]\n\n' "$(toml_quote "$remote")" "$(toml_quote "$local_store")"
    printf '[storage %s]\n' "$remote"
    if [ "$provider" = "google" ]; then
      if [ ! -s "$GOOGLE_TOKENS/$credential_ref.json" ]; then
        notice "Google connection '$display_name' needs authorization before calendars can be discovered."
        continue
      fi
      printf 'type = "google_calendar"\n'
      printf 'token_file = %s\n' "$(toml_quote "$GOOGLE_TOKENS/$credential_ref.json")"
      printf 'client_id = %s\n' "$(toml_quote "$client_id")"
      printf 'client_secret.fetch = ["command", "cat", %s]\n\n' "$(toml_quote "$VDIR_PASSWORDS/$credential_ref.google-client-secret")"
    else
      printf 'type = "caldav"\n'
      printf 'url = %s\nusername = %s\n' "$(toml_quote "$url")" "$(toml_quote "$username")"
      printf 'password.fetch = ["command", "cat", %s]\n\n' "$(toml_quote "$VDIR_PASSWORDS/$credential_ref")"
    fi
    printf '[storage %s]\n' "$local_store"
    printf 'type = "filesystem"\npath = %s\nfileext = ".ics"\n\n' "$(toml_quote "$STAGE/collections/$credential_ref/")"
  done < "$VDIR_PAIRS"
} > "$CFG"
chmod 600 "$CFG"

for connection in "${!SEEN[@]}"; do
  stage_pair="discover_${connection}"
  # Discover only updates the isolated stage. metasync copies collection
  # displayname/color metadata into that stage; it does not transfer events.
  if ! yes | "$VDIRSYNCER_BIN" -c "$CFG" discover "$stage_pair" >/dev/null 2>&1; then
    notice "Could not discover calendars for '$connection'. Existing selections were not changed."
    continue
  fi
  "$VDIRSYNCER_BIN" -c "$CFG" metasync "$stage_pair" >/dev/null 2>&1 || true
  # Recover provider/base-pair metadata from the persisted private connection.
  base_pair=""; provider="caldav"
  while IFS='|' read -r name color tag pair ignored url username remote_id pprovider client_id display_name credential_ref local_id _; do
    [ "$credential_ref" = "$connection" ] || continue
    base_pair="$pair"; provider="${pprovider:-caldav}"; break
  done < "$VDIR_PAIRS"
  [ -n "$base_pair" ] || continue
  for dir in "$STAGE/collections/$connection"/*; do
    [ -d "$dir" ] || continue
    remote_id="$(basename "$dir")"
    display_name="$remote_id"
    color="blue"
    [ -r "$dir/displayname" ] && display_name="$(cat "$dir/displayname" 2>/dev/null || printf '%s' "$remote_id")"
    [ -r "$dir/color" ] && color="$(cat "$dir/color" 2>/dev/null || printf 'blue')"
    valid_single_line "$remote_id" && valid_single_line "$display_name" && valid_single_line "$color" || continue
    printf 'calendar\t%s\t%s\t%s\t%s\t%s\t%s\n' \
      "$(clean_field "$base_pair")" "$(clean_field "$provider")" "$(clean_field "$connection")" "$(clean_field "$remote_id")" "$(clean_field "$display_name")" "$(clean_field "$color")"
  done
done
