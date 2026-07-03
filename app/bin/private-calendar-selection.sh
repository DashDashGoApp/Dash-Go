#!/usr/bin/env bash
# Explicit private-calendar activation/deactivation. This helper changes only
# saved pair/map configuration after a Dashboard Control selection. Routine
# sync never discovers remote collections. Activating a newly selected exact
# collection deliberately performs one bounded initial discovery, then never
# discovers it again during recurring sync. This helper never deletes a remote
# calendar. New selected collections are exact vdir mappings with safe local
# keys, so the dashboard can resolve writeback without a broad mirror.
set -u

DASH="${DASH:-$HOME/dashboard}"
BIN_DIR="$DASH/bin"
VDIR_HOME="${DASH_VDIR_HOME:-$HOME/.dashboard-vdirsyncer}"
VDIR_PAIRS="${DASH_VDIR_PAIRS:-$VDIR_HOME/pairs}"
MAP="${DASH_VDIR_MAP:-$VDIR_HOME/calendars.map}"
VDIR_COLLECTIONS="$VDIR_HOME/collections"
SETUP="$BIN_DIR/setup-vdirsyncer.sh"
SYNC="$BIN_DIR/sync-vdir.sh"
LOWPRIO="$BIN_DIR/dashboard-lowprio.sh"
umask 077

valid_name(){ printf '%s' "$1" | grep -qE '^[A-Za-z0-9_-]{1,96}$'; }
valid_single_line(){ case "$1" in *$'\n'*|*$'\r'*|*'|'*|*$'\t'*) return 1;; esac; return 0; }
valid_color(){ case "$(printf '%s' "$1" | tr '[:upper:]' '[:lower:]')" in green|blue|red|gold|violet|purple|amber|teal|orange|slate) return 0;; esac; printf '%s' "$1" | grep -qiE '^#?[0-9a-f]{6}$'; }
# Remote IDs are opaque provider data. The generated local ID is the only
# filesystem component, so URL-like CalDAV collection IDs are safe here.
valid_remote(){ [ -n "$1" ] && [ "${#1}" -le 512 ] && valid_single_line "$1"; }
key_for(){ printf '%s' "$1" | cksum | awk '{print $1}'; }
source_for(){ local n="$1" c="$2" t="$3"; if [ -n "$t" ]; then printf 'calendars/%s.%s.%s.ics' "$n" "$c" "$t"; else printf 'calendars/%s.%s.ics' "$n" "$c"; fi; }
valid_source(){ case "$1" in calendars/*.ics) valid_single_line "$1";; *) return 1;; esac; }

[ -r "$VDIR_PAIRS" ] && [ -r "$MAP" ] && [ -x "$SETUP" ] || { printf 'error\tPrivate calendar setup is unavailable.\n'; exit 1; }
[ -d "$VDIR_HOME/sync.lock" ] && { printf 'error\tA calendar sync is already running. Try again shortly.\n'; exit 75; }
LOCK="$VDIR_HOME/selection.lock"
if ! mkdir "$LOCK" 2>/dev/null; then printf 'error\tPrivate calendar settings are already changing. Try again shortly.\n'; exit 75; fi
trap 'rm -rf "$LOCK"' EXIT

mode="${1:-}"
case "$mode" in
  --activate)
    [ "$#" -eq 6 ] || { printf 'error\tInvalid calendar selection.\n'; exit 2; }
    base_pair="$2"; remote_id="$3"; display_name="$4"; color="$5"; editable="$6"
    valid_name "$base_pair" && valid_remote "$remote_id" && valid_single_line "$display_name" && valid_color "$color" || { printf 'error\tInvalid calendar selection.\n'; exit 2; }
    case "$editable" in 0|1) ;; *) printf 'error\tInvalid edit setting.\n'; exit 2;; esac
    # Resolve the existing connection that owns credentials. No client secret,
    # password, token path, or provider endpoint is emitted from this script.
    found=0
    while IFS='|' read -r name old_color tag pair ignored url username old_remote provider client_id old_display credential_ref old_local _; do
      [ "$pair" = "$base_pair" ] || continue
      found=1; break
    done < "$VDIR_PAIRS"
    [ "$found" -eq 1 ] || { printf 'error\tThis calendar connection is no longer available. Discover again.\n'; exit 1; }
    [ -n "${credential_ref:-}" ] || credential_ref="$name"
    [ -n "${provider:-}" ] || provider="caldav"
    # Exact same provider connection + remote collection is idempotent.
    while IFS='|' read -r ename ecolor etag epair eignored eurl euser eremote eprovider eclient edisplay ecred elocal _; do
      [ "$eprovider" = "$provider" ] && [ "$ecred" = "$credential_ref" ] && [ "$eremote" = "$remote_id" ] || continue
      while IFS='|' read -r mname mcolor mtag mpair mpath mwritable mremote mdisplay mprovider mconnection mlocal _; do
        [ "$mname" = "$ename" ] || continue
        printf 'existing\t%s\t%s\t%s\t%s\t%s\t%s\n' "$(source_for "$mname" "$mcolor" "$mtag")" "$mdisplay" "$mprovider" "$mpair" "$mremote" "$mwritable"
        exit 0
      done < "$MAP"
    done < "$VDIR_PAIRS"
    base="pc_$(key_for "$credential_ref|$remote_id")"
    name="$base"; suffix=1
    while grep -qE "^${name}\\|" "$VDIR_PAIRS"; do suffix=$((suffix+1)); name="${base}_${suffix}"; done
    pair="dash_${name}"
    local_id="collection_$(key_for "$remote_id")"
    collection_path="$VDIR_COLLECTIONS/$name"
    mkdir -p "$collection_path"
    tmp_pairs="$(mktemp "$VDIR_HOME/pairs.XXXXXX")"; tmp_map="$(mktemp "$VDIR_HOME/map.XXXXXX")"
    backup_pairs="$(mktemp "$VDIR_HOME/pairs-backup.XXXXXX")"; backup_map="$(mktemp "$VDIR_HOME/map-backup.XXXXXX")"
    cp "$VDIR_PAIRS" "$tmp_pairs"; cp "$MAP" "$tmp_map"
    cp "$VDIR_PAIRS" "$backup_pairs"; cp "$MAP" "$backup_map"
    printf '%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s\n' "$name" "$color" "" "$pair" "$collection_path" "$editable" "$remote_id" "$display_name" "$provider" "$credential_ref" "$local_id" >> "$tmp_map"
    printf '%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s\n' "$name" "$color" "" "$pair" "$collection_path" "$url" "$username" "$remote_id" "$provider" "$client_id" "$display_name" "$credential_ref" "$local_id" >> "$tmp_pairs"
    chmod 600 "$tmp_pairs" "$tmp_map" "$backup_pairs" "$backup_map"
    mv "$tmp_pairs" "$VDIR_PAIRS"; mv "$tmp_map" "$MAP"
    if ! "$SETUP" --refresh >/dev/null 2>&1; then
      cp "$backup_pairs" "$VDIR_PAIRS"; cp "$backup_map" "$MAP"
      "$SETUP" --refresh >/dev/null 2>&1 || true
      rm -f "$backup_pairs" "$backup_map"
      printf 'error\tCould not activate the selected calendar safely.\n'; exit 1
    fi
    rm -f "$backup_pairs" "$backup_map"
    # vdirsyncer refuses `sync` for a pair it has never discovered, and routine
    # syncs deliberately never discover. Activation is an explicit
    # administrator action, so run the one bounded, noninteractive discovery
    # this exact pair needs; without it the selected calendar could never sync.
    VDIRSYNCER_BIN="${DASH_VDIRSYNCER_BIN:-$VDIR_HOME/bin/vdirsyncer}"
    VDIR_CFG="$VDIR_HOME/config"
    discovered=1
    if [ -x "$VDIRSYNCER_BIN" ]; then
      if command -v timeout >/dev/null 2>&1; then
        yes | timeout 300 "$VDIRSYNCER_BIN" -c "$VDIR_CFG" discover "$pair" >/dev/null 2>&1 || discovered=0
      else
        yes | "$VDIRSYNCER_BIN" -c "$VDIR_CFG" discover "$pair" >/dev/null 2>&1 || discovered=0
      fi
    else
      discovered=0
    fi
    # Initial sync is targeted. It may fail without changing the selected
    # mapping; the UI can show the safe waiting state and offer Sync now.
    initial="ready"
    if [ "$discovered" -ne 1 ]; then
      initial="waiting"
    elif [ -x "$SYNC" ] && ! "$SYNC" --pair "$pair" >/dev/null 2>&1; then
      initial="waiting"
    fi
    "$SETUP" --refresh >/dev/null 2>&1 || true
    printf 'activated\t%s\t%s\t%s\t%s\t%s\t%s\n' "$(source_for "$name" "$color" "")" "$display_name" "$provider" "$pair" "$remote_id" "$initial"
    ;;
  --deactivate)
    [ "$#" -eq 2 ] || { printf 'error\tInvalid calendar source.\n'; exit 2; }
    source="$2"; valid_source "$source" || { printf 'error\tInvalid calendar source.\n'; exit 2; }
    found=""
    while IFS='|' read -r name color tag pair collection writable remote_id display_name provider connection local_id _; do
      [ "$(source_for "$name" "$color" "$tag")" = "$source" ] || continue
      found="$name"; break
    done < "$MAP"
    [ -n "$found" ] || { printf 'error\tThis selected calendar is no longer available.\n'; exit 1; }
    tmp_pairs="$(mktemp "$VDIR_HOME/pairs.XXXXXX")"; tmp_map="$(mktemp "$VDIR_HOME/map.XXXXXX")"
    backup_pairs="$(mktemp "$VDIR_HOME/pairs-backup.XXXXXX")"; backup_map="$(mktemp "$VDIR_HOME/map-backup.XXXXXX")"
    cp "$VDIR_PAIRS" "$backup_pairs"; cp "$MAP" "$backup_map"
    awk -F'|' -v n="$found" '$1 != n {print}' "$VDIR_PAIRS" > "$tmp_pairs"
    awk -F'|' -v n="$found" '$1 != n {print}' "$MAP" > "$tmp_map"
    chmod 600 "$tmp_pairs" "$tmp_map" "$backup_pairs" "$backup_map"
    mv "$tmp_pairs" "$VDIR_PAIRS"; mv "$tmp_map" "$MAP"
    if ! "$SETUP" --refresh >/dev/null 2>&1; then
      cp "$backup_pairs" "$VDIR_PAIRS"; cp "$backup_map" "$MAP"
      "$SETUP" --refresh >/dev/null 2>&1 || true
      rm -f "$backup_pairs" "$backup_map"
      printf 'error\tCould not update the private calendar configuration.\n'; exit 1
    fi
    rm -f "$backup_pairs" "$backup_map"
    printf 'deactivated\t%s\n' "$source"
    ;;
  --set-editable)
    [ "$#" -eq 3 ] || { printf 'error\tInvalid calendar edit setting.\n'; exit 2; }
    source="$2"; editable="$3"
    valid_source "$source" || { printf 'error\tInvalid calendar source.\n'; exit 2; }
    case "$editable" in 0|1) ;; *) printf 'error\tInvalid edit setting.\n'; exit 2;; esac
    found=""
    while IFS='|' read -r name color tag pair collection writable remote_id display_name provider connection local_id _; do
      [ "$(source_for "$name" "$color" "$tag")" = "$source" ] || continue
      found="$name"; break
    done < "$MAP"
    [ -n "$found" ] || { printf 'error\tThis selected calendar is no longer available.\n'; exit 1; }
    tmp_map="$(mktemp "$VDIR_HOME/map.XXXXXX")"; backup_map="$(mktemp "$VDIR_HOME/map-backup.XXXXXX")"
    cp "$MAP" "$backup_map"
    awk -F'|' -v n="$found" -v e="$editable" 'BEGIN{OFS="|"} $1 == n {$6=e} {print}' "$MAP" > "$tmp_map"
    chmod 600 "$tmp_map" "$backup_map"
    mv "$tmp_map" "$MAP"
    if ! "$SETUP" --refresh >/dev/null 2>&1; then
      cp "$backup_map" "$MAP"
      "$SETUP" --refresh >/dev/null 2>&1 || true
      rm -f "$backup_map"
      printf 'error\tCould not update Dashboard edit permission safely.\n'; exit 1
    fi
    rm -f "$backup_map"
    printf 'updated\t%s\t%s\n' "$source" "$editable"
    ;;
  --repair)
    [ "$#" -eq 2 ] || { printf 'error\tInvalid calendar source.\n'; exit 2; }
    source="$2"; valid_source "$source" || { printf 'error\tInvalid calendar source.\n'; exit 2; }
    pair=""
    while IFS='|' read -r name color tag row_pair collection writable remote_id display_name provider connection local_id _; do
      [ "$(source_for "$name" "$color" "$tag")" = "$source" ] || continue
      pair="$row_pair"; break
    done < "$MAP"
    [ -n "$pair" ] || { printf 'error\tThis selected calendar is no longer available.\n'; exit 1; }
    VDIRSYNCER_BIN="${DASH_VDIRSYNCER_BIN:-$VDIR_HOME/bin/vdirsyncer}"
    VDIR_CFG="$VDIR_HOME/config"
    [ -x "$VDIRSYNCER_BIN" ] && [ -r "$VDIR_CFG" ] && [ -x "$SYNC" ] || { printf 'error\tPrivate calendar repair is unavailable.\n'; exit 1; }
    # Repair is an explicit administrator action. It deliberately discovers
    # only this exact pair; it never inventories every account calendar and is
    # never called from the recurring sync wrapper.
    if [ -x "$LOWPRIO" ]; then
      discover_cmd=("$LOWPRIO" "$VDIRSYNCER_BIN" -c "$VDIR_CFG" discover "$pair")
    else
      discover_cmd=("$VDIRSYNCER_BIN" -c "$VDIR_CFG" discover "$pair")
    fi
    if command -v timeout >/dev/null 2>&1; then
      yes | timeout 300 "${discover_cmd[@]}" >/dev/null 2>&1 || { printf 'error\tCould not repair this calendar connection. Check its provider authorization, then try again.\n'; exit 1; }
    else
      yes | "${discover_cmd[@]}" >/dev/null 2>&1 || { printf 'error\tCould not repair this calendar connection. Check its provider authorization, then try again.\n'; exit 1; }
    fi
    if ! "$SYNC" --pair "$pair" >/dev/null 2>&1; then
      printf 'error\tThe calendar connection was discovered, but its first sync still needs attention.\n'; exit 1
    fi
    printf 'repaired\t%s\t%s\n' "$source" "$pair"
    ;;
  *) printf 'error\tUsage: private-calendar-selection.sh --activate ... | --deactivate source | --set-editable source 0|1 | --repair source\n'; exit 2;;
esac
