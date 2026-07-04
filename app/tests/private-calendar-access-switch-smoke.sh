#!/usr/bin/env bash
# Exercise the exact selected-calendar access switch. It must snapshot before
# locking a calendar, reconfigure through setup, and validate a targeted sync
# before allowing two-way mode again.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
SELECT="$ROOT/bin/private-calendar-selection.sh"
TMP="$(mktemp -d)"
cleanup(){ rm -rf "$TMP"; }
trap cleanup EXIT
fail(){ echo "FAIL: $*" >&2; exit 1; }
DASH="$TMP/dashboard"; HOME="$TMP/home"; VDIR="$HOME/.dashboard-vdirsyncer"
mkdir -p "$DASH/bin" "$DASH/calendars" "$VDIR/collections/family/collection_primary"
printf 'BEGIN:VCALENDAR\nEND:VCALENDAR\n' > "$DASH/calendars/family.blue.ics"
printf '%s\n' "family|blue||dash_family|$VDIR/collections/family|1|remote-primary|Family|caldav|icloud-account|collection_primary" > "$VDIR/calendars.map"
printf '%s\n' "family|blue||dash_family|$VDIR/collections/family|https://caldav.icloud.com/|person@example.com|remote-primary|caldav||Family|icloud-account|collection_primary" > "$VDIR/pairs"
cat > "$DASH/bin/setup-vdirsyncer.sh" <<'SETUP'
#!/usr/bin/env bash
[ "${1:-}" = --refresh ] || exit 2
printf '%s\n' refresh >> "${FAKE_LOG:?}"
SETUP
cat > "$DASH/bin/sync-vdir.sh" <<'SYNC'
#!/usr/bin/env bash
[ "${1:-}" = --pair ] || exit 2
printf '%s %s\n' "$1" "$2" >> "${FAKE_LOG:?}"
SYNC
chmod +x "$DASH/bin/setup-vdirsyncer.sh" "$DASH/bin/sync-vdir.sh"
export FAKE_LOG="$TMP/actions.log"
DASH="$DASH" HOME="$HOME" DASH_VDIR_HOME="$VDIR" "$SELECT" --set-editable calendars/family.blue.ics 0 > "$TMP/off.out" || fail 'view-only switch failed'
grep -Fq '|0|remote-primary|' "$VDIR/calendars.map" || fail 'view-only mode was not saved'
find "$VDIR/access-mode-backups" -name README.txt -type f | grep -q . || fail 'view-only switch did not retain a private snapshot'
grep -Fxq refresh "$FAKE_LOG" || fail 'view-only switch did not refresh generated configuration'
DASH="$DASH" HOME="$HOME" DASH_VDIR_HOME="$VDIR" "$SELECT" --set-editable calendars/family.blue.ics 1 > "$TMP/on.out" || fail 'two-way switch failed'
grep -Fq '|1|remote-primary|' "$VDIR/calendars.map" || fail 'two-way mode was not saved'
grep -Fq -- '--pair dash_family' "$FAKE_LOG" || fail 'two-way switch did not verify targeted provider sync'
echo 'PASS: private calendar view-only/two-way switching is staged and verified'
