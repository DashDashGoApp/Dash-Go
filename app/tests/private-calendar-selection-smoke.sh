#!/usr/bin/env bash
# Explicit selection converts one discovered opaque collection into one exact
# vdir mapping, does a targeted initial sync, preserves the broad source, and
# can stop future sync without deleting remote/local calendar data.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
TMP="$(mktemp -d)"
cleanup(){ rm -rf "$TMP"; }
trap cleanup EXIT
HOME="$TMP/home"; export HOME
DASH="$HOME/dashboard"; export DASH
mkdir -p "$DASH/bin" "$TMP/fake-bin" "$HOME/.dashboard-vdirsyncer/passwords" "$HOME/.dashboard-vdirsyncer/google-tokens"
cp "$ROOT/bin/setup-vdirsyncer.sh" "$DASH/bin/setup-vdirsyncer.sh"
cp "$ROOT/bin/private-calendar-selection.sh" "$DASH/bin/private-calendar-selection.sh"
chmod +x "$DASH/bin/setup-vdirsyncer.sh" "$DASH/bin/private-calendar-selection.sh"
cat > "$TMP/fake-bin/vdirsyncer" <<'VDIR'
#!/usr/bin/env bash
set -euo pipefail
if [ "${1:-}" = "--version" ]; then printf 'vdirsyncer, version 0.20.0\n'; exit 0; fi
cfg="${VDIRSYNCER_CONFIG:-}"
if [ "${1:-}" = "-c" ]; then cfg="${2:-}"; fi
root="$(dirname "$cfg")"
if [[ " $* " == *' sync dash_pc_'* ]]; then
  path="$(grep -E 'path = .*collections/pc_' "$cfg" | tail -n1 | sed -E 's/.*path = "(.*)"/\1/')"
  local_id="$(grep -E '^collections = \[\[' "$cfg" | tail -n1 | sed -E 's/.*"[^"]+",[[:space:]]*"[^"]+",[[:space:]]*"([^"]+)".*/\1/')"
  mkdir -p "$path/$local_id"
  printf 'BEGIN:VCALENDAR\nVERSION:2.0\nBEGIN:VEVENT\nUID:selected\nSUMMARY:Selected family event\nEND:VEVENT\nEND:VCALENDAR\n' > "$path/$local_id/selected.ics"
fi
exit 0
VDIR
chmod +x "$TMP/fake-bin/vdirsyncer"
cat > "$DASH/bin/gen-calendars.sh" <<'SH2'
#!/usr/bin/env bash
exit 0
SH2
cat > "$DASH/bin/dashboard-control-server" <<'SH2'
#!/usr/bin/env bash
exit 0
SH2
chmod +x "$DASH/bin/gen-calendars.sh" "$DASH/bin/dashboard-control-server"
VDIR="$HOME/.dashboard-vdirsyncer"
printf 'token' > "$VDIR/google-tokens/gacct.json"
printf 'secret' > "$VDIR/passwords/gacct.google-client-secret"
printf 'base|blue||dash_base|%s/collections/base||||google|fixture.apps.googleusercontent.com|Google account|gacct|\n' "$VDIR" > "$VDIR/pairs"
printf 'base|blue||dash_base|%s/collections/base|0||Google account|google|gacct|\n' "$VDIR" > "$VDIR/calendars.map"
export PATH="$TMP/fake-bin:$PATH" DASH_VDIRSYNCER_BIN="$TMP/fake-bin/vdirsyncer" DASH_VDIRSYNCER_PYTHON=/bin/bash
OUT="$($DASH/bin/private-calendar-selection.sh --activate dash_base family/path/opaque Family teal 1)"
printf '%s\n' "$OUT" | grep -Eq $'^activated\tcalendars/pc_[0-9]+\.teal\.ics\tFamily\tgoogle\tdash_pc_[0-9]+'
REG="$DASH/config/calendar-writeback.json"
python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); assert d["version"] == 2; assert len(d["calendars"]) == 1; assert d["calendars"][0]["provider"] == "google"; assert d["calendars"][0]["remoteId"] == "family/path/opaque"; assert d["calendars"][0]["pair"].startswith("dash_pc_")' "$REG"
! grep -Fq 'conflict_resolution = "a wins"' "$VDIR/config"
# An exact selected calendar must not erase the legacy broad mirror that remains
# deliberately display-only until the user chooses to hide it.
grep -Fq '[pair dash_base]' "$VDIR/config"
grep -Fq 'collections = ["from a"]' "$VDIR/config"
grep -Fq 'TARGET_PAIR' "$DASH/bin/sync-vdir.sh"
grep -Fq 'dash_base' "$VDIR/pairs"
SOURCE="$(printf '%s\n' "$OUT" | cut -f2)"
LOCAL="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["calendars"][0]["collection"])' "$REG")"
[ -f "$LOCAL/selected.ics" ]
"$DASH/bin/private-calendar-selection.sh" --set-editable "$SOURCE" 0 | grep -Fq $'updated\t'
grep -Fq '|0|family/path/opaque|' "$VDIR/calendars.map"
"$DASH/bin/private-calendar-selection.sh" --set-editable "$SOURCE" 1 | grep -Fq $'updated\t'
grep -Fq '|1|family/path/opaque|' "$VDIR/calendars.map"
"$DASH/bin/private-calendar-selection.sh" --deactivate "$SOURCE" | grep -Fq $'deactivated\t'
[ -f "$LOCAL/selected.ics" ]
! grep -Fq 'family/path/opaque' "$VDIR/pairs"
printf 'PASS: explicit private calendar selection uses an exact target and preserves local data on stop\n'
