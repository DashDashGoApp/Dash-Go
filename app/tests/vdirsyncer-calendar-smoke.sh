#!/usr/bin/env bash
# Exercise restored CalDAV/vdirsyncer setup without network or root access.
# Proves secrets stay outside the webroot and the generated wrapper publishes
# one valid Dash-Go calendar file from a multi-file vdir.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
SETUP="$ROOT/bin/setup-vdirsyncer.sh"
TMP="$(mktemp -d)"
cleanup(){ rm -rf "$TMP"; }
trap cleanup EXIT
HOME="$TMP/home"
export HOME
mkdir -p "$HOME/dashboard/bin" "$TMP/fake-bin"
cp "$SETUP" "$HOME/dashboard/bin/setup-vdirsyncer.sh"
chmod +x "$HOME/dashboard/bin/setup-vdirsyncer.sh"

cat > "$TMP/fake-bin/vdirsyncer" <<'VDIR'
#!/usr/bin/env bash
set -eu
case " $* " in
  *' sync '*)
    root="$(dirname "${VDIRSYNCER_CONFIG:?}")"
    mkdir -p "$root/collections/family/nested"
    printf 'BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VTIMEZONE\r\nTZID:Fixture\r\nEND:VTIMEZONE\r\nBEGIN:VEVENT\r\nUID:one\r\nSUMMARY:First event\r\nDESCRIPTION:Folded\r\n detail\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n' > "$root/collections/family/one.ics"
    printf 'BEGIN:VCALENDAR\nVERSION:2.0\nBEGIN:VEVENT\nUID:two\nSUMMARY:Second event\nEND:VEVENT\nEND:VCALENDAR\n' > "$root/collections/family/nested/two.ics"
    ;;
esac
exit 0
VDIR
cat > "$TMP/fake-bin/crontab" <<'CRON'
#!/usr/bin/env bash
set -eu
: "${FAKE_CRONTAB:?}"
if [ "${1:-}" = "-l" ]; then [ -f "$FAKE_CRONTAB" ] && cat "$FAKE_CRONTAB"; exit 0; fi
cp "$1" "$FAKE_CRONTAB"
CRON
chmod +x "$TMP/fake-bin/vdirsyncer" "$TMP/fake-bin/crontab"
cat > "$HOME/dashboard/bin/gen-calendars.sh" <<'GEN'
#!/usr/bin/env bash
: "${HOME:?}"
touch "$HOME/gen-calendars.called"
GEN
cat > "$HOME/dashboard/bin/dashboard-control-server" <<'SERVER'
#!/usr/bin/env bash
: "${HOME:?}"
[ "${1:-}" = "--gen-events-cache" ] && touch "$HOME/gen-events-cache.called"
SERVER
chmod +x "$HOME/dashboard/bin/gen-calendars.sh" "$HOME/dashboard/bin/dashboard-control-server"

export PATH="$TMP/fake-bin:$PATH" FAKE_CRONTAB="$TMP/crontab"
printf 'family\nblue\nn\nhttps://caldav.example/\nfamily@example.com\nfixture-password\ncollection-1\n\n' | "$HOME/dashboard/bin/setup-vdirsyncer.sh" >"$TMP/setup.out" 2>&1

CAL="$HOME/dashboard/calendars/family.blue.ics"
[ -f "$CAL" ] || { cat "$TMP/setup.out" >&2; echo 'FAIL: merged calendar missing' >&2; exit 1; }
[ "$(grep -c '^BEGIN:VCALENDAR$' "$CAL")" -eq 1 ]
[ "$(grep -c '^END:VCALENDAR$' "$CAL")" -eq 1 ]
[ "$(grep -c '^BEGIN:VEVENT$' "$CAL")" -eq 2 ]
grep -Fq 'BEGIN:VTIMEZONE' "$CAL"
! grep -q $'\r' "$CAL"
test -f "$HOME/gen-calendars.called"
test -f "$HOME/gen-events-cache.called"
[ "$(stat -c '%a' "$HOME/.dashboard-vdirsyncer/config")" = 600 ]
[ "$(stat -c '%a' "$HOME/.dashboard-vdirsyncer/passwords/family")" = 600 ]
[ "$(stat -c '%a' "$HOME/.dashboard-vdirsyncer")" = 700 ]
grep -Fqx 'fixture-password' "$HOME/.dashboard-vdirsyncer/passwords/family"
! grep -R -Fq 'fixture-password' "$HOME/dashboard"
! grep -Fq 'fixture-password' "$HOME/.dashboard-vdirsyncer/calendars.map"
! grep -Fq 'fixture-password' "$HOME/.dashboard-vdirsyncer/pairs"
grep -Fq '*/15 * * * * ' "$FAKE_CRONTAB"
grep -Fq 'sync-vdir.sh >/dev/null 2>&1' "$FAKE_CRONTAB"
bash -n "$HOME/dashboard/bin/sync-vdir.sh"
# A second run is safe and leaves a single merged VCALENDAR wrapper.
"$HOME/dashboard/bin/sync-vdir.sh"
[ "$(grep -c '^BEGIN:VCALENDAR$' "$CAL")" -eq 1 ]
printf 'PASS: CalDAV vdirsyncer setup keeps secrets private and merges calendar files safely\n'
