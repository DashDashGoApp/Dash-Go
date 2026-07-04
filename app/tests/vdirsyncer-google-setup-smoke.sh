#!/usr/bin/env bash
# Exercise the Google Calendar branch of setup-vdirsyncer.sh without network
# or real OAuth. Proves the generated vdirsyncer config uses google_calendar
# storage with private token and client-secret files outside the webroot, the
# writeback registry enrolls the exact Google collection, and the generated
# sync wrapper skips an unauthorized Google pair instead of blocking cron.
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

# A fake interpreter lets google_support_present() pass its aiohttp-oauthlib
# import probe while exercising the common /usr/bin/env interpreter shebang.
cat > "$TMP/fake-bin/fakepython" <<'PY'
#!/usr/bin/env bash
set -eu
if [ "${1:-}" = "-c" ]; then exit 0; fi
script="$1"; shift
exec bash "$script" "$@"
PY
chmod +x "$TMP/fake-bin/fakepython"

cat > "$TMP/fake-bin/vdirsyncer" <<VDIR
#!/usr/bin/env fakepython
set -eu
if [ "\${1:-}" = "--version" ]; then printf 'vdirsyncer, version 0.20.0\n'; exit 0; fi
: "\${FAKE_VDIR_LOG:?}"
printf '%s\n' "\$*" >> "\$FAKE_VDIR_LOG"
root="\$(dirname "\${VDIRSYNCER_CONFIG:-\$HOME/.dashboard-vdirsyncer/config}")"
case " \$* " in
  *' sync dash_gcal '*)
    [ "\${FAIL_GCAL_SYNC:-0}" = "1" ] && exit 1
    mkdir -p "\$root/collections/gcal/home@gmail.com"
    printf 'BEGIN:VCALENDAR\nVERSION:2.0\nBEGIN:VEVENT\nUID:g1\nSUMMARY:Google event\nEND:VEVENT\nEND:VCALENDAR\n' > "\$root/collections/gcal/home@gmail.com/g1.ics"
    ;;
  *' sync dash_ical '*)
    mkdir -p "\$root/collections/ical/collection-1"
    printf 'BEGIN:VCALENDAR\nVERSION:2.0\nBEGIN:VEVENT\nUID:i1\nSUMMARY:iCloud event\nEND:VEVENT\nEND:VCALENDAR\n' > "\$root/collections/ical/collection-1/i1.ics"
    ;;
esac
exit 0
VDIR
chmod +x "$TMP/fake-bin/vdirsyncer"

cat > "$TMP/fake-bin/crontab" <<'CRON'
#!/usr/bin/env bash
set -eu
: "${FAKE_CRONTAB:?}"
if [ "${1:-}" = "-l" ]; then [ -f "$FAKE_CRONTAB" ] && cat "$FAKE_CRONTAB"; exit 0; fi
cp "$1" "$FAKE_CRONTAB"
CRON
chmod +x "$TMP/fake-bin/crontab"
cat > "$HOME/dashboard/bin/gen-calendars.sh" <<'GEN'
#!/usr/bin/env bash
exit 0
GEN
cat > "$HOME/dashboard/bin/dashboard-control-server" <<'SERVER'
#!/usr/bin/env bash
set -eu
if [ "${1:-}" = "--google-oauth" ] && [ "${2:-}" = "authorize" ]; then
  : "${FAKE_OAUTH_LOG:?}"
  token_file=""
  while [ "$#" -gt 0 ]; do
    case "$1" in
      -token-file) token_file="$2"; shift 2;;
      *) shift;;
    esac
  done
  [ -n "$token_file" ] || exit 2
  mkdir -p "$(dirname "$token_file")"
  printf '{"access_token":"fixture","refresh_token":"fixture","expires_in":3600,"token_type":"Bearer"}
' > "$token_file"
  chmod 600 "$token_file"
  printf '%s
' '--google-oauth authorize' >> "$FAKE_OAUTH_LOG"
  exit 0
fi
exit 0
SERVER
chmod +x "$HOME/dashboard/bin/gen-calendars.sh" "$HOME/dashboard/bin/dashboard-control-server"

export PATH="$TMP/fake-bin:$PATH" FAKE_CRONTAB="$TMP/crontab" FAKE_VDIR_LOG="$TMP/vdir-calls.log" FAKE_OAUTH_LOG="$TMP/oauth-calls.log" DASH_VDIRSYNCER_BIN="$TMP/fake-bin/vdirsyncer" DASH_VDIRSYNCER_PYTHON="$TMP/fake-bin/fakepython"
touch "$FAKE_VDIR_LOG" "$FAKE_OAUTH_LOG"

# Two Google calendars (exact IDs, writable) and one CalDAV calendar (exact
# collection, writable). After the blank line that ends the add loop, the
# authorization prompts are answered in pairs-file order: 'y' authorizes gcal
# through the Go helper and 'n' declines gcal2 — proving prompts read the user,
# not the pairs file.
printf 'gcal\ngreen\nn\n2\ndesktop\n1234-fixture.apps.googleusercontent.com\ngsecret-fixture\nhome@gmail.com\ny\ngcal2\nred\nn\n2\ndesktop\n5678-fixture.apps.googleusercontent.com\ngsecret2-fixture\nsecond@gmail.com\ny\nical\nblue\nn\n1\nhttps://caldav.example/\nfamily@example.com\nicloud-password-fixture\ncollection-1\ny\n\ny\nn\n' \
  | "$HOME/dashboard/bin/setup-vdirsyncer.sh" >"$TMP/setup.out" 2>&1 || { cat "$TMP/setup.out" >&2; exit 1; }

CFG="$HOME/.dashboard-vdirsyncer/config"
REG="$HOME/dashboard/config/calendar-writeback.json"
WRAPPER="$HOME/dashboard/bin/sync-vdir.sh"
TOKENS="$HOME/.dashboard-vdirsyncer/google-tokens"

fail(){ echo "FAIL: $*" >&2; cat "$TMP/setup.out" >&2; exit 1; }

# 1) Generated config: google_calendar storage with private token and secret.
grep -q 'type = "google_calendar"' "$CFG" || fail 'google_calendar storage missing'
grep -q "token_file = \"$TOKENS/gcal.json\"" "$CFG" || fail 'token_file not under private vdir home'
grep -q 'client_id = "1234-fixture.apps.googleusercontent.com"' "$CFG" || fail 'client_id missing'
grep -Fq "client_secret.fetch = [\"command\", \"cat\", \"$HOME/.dashboard-vdirsyncer/passwords/gcal.google-client-secret\"]" "$CFG" || fail 'client secret must be fetched from the private passwords dir'
grep -q 'type = "caldav"' "$CFG" || fail 'caldav pair lost its storage type'

# 2) Secrets: outside the webroot, owner-only, never in the dashboard tree.
SECRET="$HOME/.dashboard-vdirsyncer/passwords/gcal.google-client-secret"
[ -f "$SECRET" ] || fail 'google client secret file missing'
[ "$(stat -c '%a' "$SECRET")" = "600" ] || fail 'google client secret must be 0600'
grep -Frq 'gsecret-fixture' "$HOME/dashboard" && fail 'google client secret leaked into the dashboard tree'

# 3) One-time authorization ran through the dashboard control server before
#    the normal non-interactive discovery. The declined pair was prompted but
#    not discovered and has no token.
grep -Fxq -- '--google-oauth authorize' "$FAKE_OAUTH_LOG" || fail 'setup did not invoke Go-native Google authorization'
grep -q 'discover dash_gcal$' "$FAKE_VDIR_LOG" || fail 'setup did not run non-interactive Google discovery after authorization'
[ -s "$TOKENS/gcal.json" ] || fail 'google token missing after authorization'
[ "$(stat -c '%a' "$TOKENS/gcal.json")" = "600" ] || fail 'google token must be 0600'
grep -q "skipped; 'gcal2' stays read-only-idle until authorized" "$TMP/setup.out" || fail 'second google pair was never offered and declined'
grep -q 'discover dash_gcal2' "$FAKE_VDIR_LOG" && fail 'declined google pair must not run discover'
[ -e "$TOKENS/gcal2.json" ] && fail 'declined google pair must have no token'

# 3b) --authorize is a focused recovery pass: it must not ask for a new
# calendar name, start a synchronization, or change cron. It can re-run the
# one-time authorization/discovery work for saved records only.
: > "$FAKE_VDIR_LOG"
printf 'n
' | "$HOME/dashboard/bin/setup-vdirsyncer.sh" --authorize >"$TMP/authorize.out" 2>&1 || { cat "$TMP/authorize.out" >&2; exit 1; }
grep -q 'Calendar name' "$TMP/authorize.out" && fail '--authorize must not prompt to add calendars'
grep -q 'sync ' "$FAKE_VDIR_LOG" && fail '--authorize must not run a private-calendar sync'
grep -q 'Google authorization pass complete' "$TMP/authorize.out" || fail '--authorize did not report its focused completion'

# 4) Registry enrolls the exact Google collection as writable; the declined
#    pair never materialized its exact vdir and must stay out of the registry.
grep -Fq '"source":"calendars/gcal.green.ics"' "$REG" || fail 'google calendar missing from writeback registry'
grep -Fq "collections/gcal/home@gmail.com" "$REG" || fail 'registry must point at the exact google collection'
grep -Fq '"source":"calendars/ical.blue.ics"' "$REG" || fail 'caldav calendar missing from writeback registry'
grep -Fq 'gcal2' "$REG" && fail 'unmaterialized declined google pair must not enter the registry'
grep -q 'private collection gcal2 was not materialized' "$TMP/setup.out" || fail 'setup must warn that the declined pair stayed read-only'
python3 -c 'import json,sys; json.load(open(sys.argv[1]))' "$REG" 2>/dev/null || fail 'writeback registry must remain valid JSON when a pair is left unmaterialized'

# 5) Wrapper guard: with the token present both pairs sync independently;
#    a failed Google pair cannot stop CalDAV and cannot overwrite its prior
#    mirror. Removing the token then skips only the Google pair.
grep -q 'awaits its one-time authorization' "$WRAPPER" || fail 'wrapper lacks the google token guard'
grep -q 'PAIR_RESULT' "$WRAPPER" || fail 'wrapper must track each pair independently'
: > "$FAKE_VDIR_LOG"
"$WRAPPER" >/dev/null 2>&1 || true
grep -Eq 'sync .*dash_gcal( |$)' "$FAKE_VDIR_LOG" || fail 'authorized google pair did not sync'
grep -Eq 'sync .*dash_ical' "$FAKE_VDIR_LOG" || fail 'caldav pair did not sync alongside google'
grep -Eq 'sync .*dash_gcal2' "$FAKE_VDIR_LOG" && fail 'never-authorized google pair must be excluded from sync'
grep -q 'gcal2 awaits its one-time authorization' "$HOME/dashboard/logs/vdir-sync.log" || fail 'wrapper must log the skipped unauthorized pair'

# Keep a known mirror, then force just the Google pair to fail. CalDAV must
# continue and the failed Google source must preserve this existing data.
printf 'BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:kept
SUMMARY:Existing Google mirror
END:VEVENT
END:VCALENDAR
' > "$HOME/dashboard/calendars/gcal.green.ics"
: > "$FAKE_VDIR_LOG"
FAIL_GCAL_SYNC=1 "$WRAPPER" >/dev/null 2>&1 || true
grep -Eq 'sync .*dash_gcal( |$)' "$FAKE_VDIR_LOG" || fail 'failed google pair was not attempted'
grep -Eq 'sync .*dash_ical' "$FAKE_VDIR_LOG" || fail 'caldav pair must keep syncing after google failure'
grep -q 'Existing Google mirror' "$HOME/dashboard/calendars/gcal.green.ics" || fail 'failed google pair overwrote its previous mirror'
grep -q 'gcal failed remote sync; kept previous calendar file' "$HOME/dashboard/logs/vdir-sync.log" || fail 'failed google pair must be logged without blocking others'

rm -f "$TOKENS/gcal.json"
: > "$FAKE_VDIR_LOG"
"$WRAPPER" >/dev/null 2>&1 || true
grep -Eq 'sync .*dash_gcal' "$FAKE_VDIR_LOG" && fail 'unauthorized google pair must be skipped'
grep -Eq 'sync .*dash_ical' "$FAKE_VDIR_LOG" || fail 'caldav pair must keep syncing when a google pair is skipped'
grep -q 'awaits its one-time authorization' "$HOME/dashboard/logs/vdir-sync.log" || fail 'skip must be logged'

# 6) Merged mirrors exist for both calendars.
[ -f "$HOME/dashboard/calendars/gcal.green.ics" ] || fail 'google mirror missing'
grep -q 'Existing Google mirror' "$HOME/dashboard/calendars/gcal.green.ics" || fail 'skipped google pair should retain its previous mirror'
[ -f "$HOME/dashboard/calendars/ical.blue.ics" ] || fail 'caldav mirror missing'

echo 'PASS: Google Calendar setup keeps OAuth material private, enrolls exact collections, and cron-proofs unauthorized pairs'
