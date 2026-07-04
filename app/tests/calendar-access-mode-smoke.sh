#!/usr/bin/env bash
# Verify the access model is real provider policy, not just a UI label: a
# private view-only source emits vdirsyncer read_only + partial_sync=revert and
# never enters the writeback registry. Switching back restores the exact pair.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
SETUP="$ROOT/bin/setup-vdirsyncer.sh"
INSTALL="$ROOT/../installer/install.sh"
UI="$ROOT/ui/js/control-private-calendars.js"
TMP="$(mktemp -d)"
cleanup(){ rm -rf "$TMP"; }
trap cleanup EXIT
fail(){ echo "FAIL: $*" >&2; exit 1; }
DASH="$TMP/dashboard"
HOME="$TMP/home"
VDIR="$HOME/.dashboard-vdirsyncer"
mkdir -p "$DASH/bin" "$DASH/config" "$DASH/calendars" "$DASH/logs" "$VDIR/collections/private_google/collection_primary"
cp "$SETUP" "$DASH/bin/setup-vdirsyncer.sh"
chmod +x "$DASH/bin/setup-vdirsyncer.sh"
# One exact Google collection. Field six in calendars.map is the durable
# per-calendar access mode: 0=view-only, 1=two-way.
printf '%s\n' "private_google|blue||dash_private_google|$VDIR/collections/private_google|0|remote-primary|Family|google|google-account|collection_primary" > "$VDIR/calendars.map"
printf '%s\n' "private_google|blue||dash_private_google|$VDIR/collections/private_google||||remote-primary|google|fixture.apps.googleusercontent.com|Family|google-account|collection_primary" > "$VDIR/pairs"
HOME="$HOME" DASH="$DASH" DASH_VDIR_HOME="$VDIR" "$DASH/bin/setup-vdirsyncer.sh" --refresh >/dev/null || fail 'view-only refresh failed'
grep -Fq 'read_only = true' "$VDIR/config" || fail 'view-only config is missing vdirsyncer read_only'
grep -Fq 'partial_sync = "revert"' "$VDIR/config" || fail 'view-only config is missing revert policy'
! grep -Fq 'calendars/private_google.blue.ics' "$DASH/config/calendar-writeback.json" || fail 'view-only calendar entered writeback registry'
awk -F'|' 'BEGIN{OFS="|"}{$6="1";print}' "$VDIR/calendars.map" > "$VDIR/calendars.map.tmp" && mv "$VDIR/calendars.map.tmp" "$VDIR/calendars.map"
HOME="$HOME" DASH="$DASH" DASH_VDIR_HOME="$VDIR" "$DASH/bin/setup-vdirsyncer.sh" --refresh >/dev/null || fail 'two-way refresh failed'
! grep -Fq 'read_only = true' "$VDIR/config" || fail 'two-way config still has vdirsyncer read_only'
grep -Fq 'calendars/private_google.blue.ics' "$DASH/config/calendar-writeback.json" || fail 'two-way calendar is absent from writeback registry'
grep -Fq 'Read-only calendar link' "$INSTALL" || fail 'installer option 9 wording missing'
grep -Fq 'Personal calendar sync' "$INSTALL" || fail 'installer option 10 wording missing'
grep -Fq 'view-only or two-way sync' "$INSTALL" || fail 'installer option 10 does not explain both access modes'
grep -Fq 'How should Dash-Go use a personal calendar?' "$INSTALL" || fail 'initial access choice missing'
grep -Fq '"$BIN_DIR/setup-vdirsyncer.sh" --view-only' "$INSTALL" || fail 'iCloud/CalDAV view-only path missing'
grep -Fq '"$BIN_DIR/setup-vdirsyncer.sh" --two-way' "$INSTALL" || fail 'initial two-way personal path missing'
grep -Fq '"$BIN_DIR/setup-vdirsyncer.sh"' "$INSTALL" || fail 'guided personal calendar setup path missing'
grep -Fq 'Google secure sign-in is for two-way sync' "$SETUP" || fail 'Google secure setup is not restricted to two-way sync'
grep -Fq 'Switch to two-way sync' "$UI" || fail 'Calendar Manager upgrade action missing'
grep -Fq 'Switch to view-only' "$UI" || fail 'Calendar Manager downgrade action missing'
grep -Fq 'Google calendar link later' "$UI" || fail 'Google link-migration guidance missing'
echo 'PASS: access mode drives vdirsyncer, writeback, installer, and Calendar Manager behavior'
