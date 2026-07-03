#!/usr/bin/env bash
# Prove private-calendar discovery stays in setup/repair while every normal
# sync entry point re-execs through Dash-Go's gentle CPU/I/O launcher.
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

cat > "$TMP/fake-bin/vdir-python" <<'PY'
#!/usr/bin/env bash
set -eu
[ "${1:-}" = "-c" ] && exit 0
exec /usr/bin/env bash "$@"
PY
cat > "$TMP/fake-bin/vdirsyncer" <<'VDIR'
#!/usr/bin/env bash
set -eu
if [ "${1:-}" = "--version" ]; then printf 'vdirsyncer, version 0.20.0\n'; exit 0; fi
: "${FAKE_VDIR_LOG:?}"
printf '%s\n' "$*" >> "$FAKE_VDIR_LOG"
cfg=""
args=("$@")
for ((i=0; i<${#args[@]}; i++)); do
  [ "${args[$i]}" = "-c" ] && { cfg="${args[$((i + 1))]:-}"; break; }
done
root="$(dirname "${cfg:?}")"
case " $* " in
  *' sync dash_family '*)
    mkdir -p "$root/collections/family/collection-1"
    printf 'BEGIN:VCALENDAR\nVERSION:2.0\nBEGIN:VEVENT\nUID:one\nSUMMARY:Private event\nEND:VEVENT\nEND:VCALENDAR\n' > "$root/collections/family/collection-1/one.ics"
    ;;
esac
VDIR
cat > "$TMP/fake-bin/crontab" <<'CRON'
#!/usr/bin/env bash
set -eu
: "${FAKE_CRONTAB:?}"
if [ "${1:-}" = "-l" ]; then [ -f "$FAKE_CRONTAB" ] && cat "$FAKE_CRONTAB"; exit 0; fi
cp "$1" "$FAKE_CRONTAB"
CRON
cat > "$HOME/dashboard/bin/dashboard-lowprio.sh" <<'LOWPRIO'
#!/usr/bin/env bash
set -eu
: "${FAKE_LOWPRIO_LOG:?}"
printf '%s\n' "$*" >> "$FAKE_LOWPRIO_LOG"
exec "$@"
LOWPRIO
cat > "$HOME/dashboard/bin/gen-calendars.sh" <<'GEN'
#!/usr/bin/env bash
exit 0
GEN
cat > "$HOME/dashboard/bin/dashboard-control-server" <<'SERVER'
#!/usr/bin/env bash
exit 0
SERVER
chmod +x "$TMP/fake-bin/vdir-python" "$TMP/fake-bin/vdirsyncer" "$TMP/fake-bin/crontab" \
  "$HOME/dashboard/bin/dashboard-lowprio.sh" "$HOME/dashboard/bin/gen-calendars.sh" "$HOME/dashboard/bin/dashboard-control-server"

export PATH="$TMP/fake-bin:$PATH" \
  DASH_VDIRSYNCER_BIN="$TMP/fake-bin/vdirsyncer" \
  DASH_VDIRSYNCER_PYTHON="$TMP/fake-bin/vdir-python" \
  FAKE_VDIR_LOG="$TMP/vdir.log" \
  FAKE_LOWPRIO_LOG="$TMP/lowprio.log" \
  FAKE_CRONTAB="$TMP/crontab"
: > "$FAKE_VDIR_LOG"
: > "$FAKE_LOWPRIO_LOG"
printf 'family\nblue\nn\n1\nhttps://caldav.example/\nfamily@example.com\nfixture-password\ncollection-1\n\n' \
  | "$HOME/dashboard/bin/setup-vdirsyncer.sh" > "$TMP/setup.out" 2>&1 || { cat "$TMP/setup.out" >&2; exit 1; }

fail(){ echo "FAIL: $*" >&2; cat "$TMP/setup.out" >&2; exit 1; }
WRAPPER="$HOME/dashboard/bin/sync-vdir.sh"
[ -x "$WRAPPER" ] || fail 'generated sync wrapper missing'
grep -Fq 'dashboard-lowprio.sh' "$WRAPPER" || fail 'wrapper must use Dash-Go low-priority launcher'
grep -Fq 'DASH_VDIR_LOWPRIO_ACTIVE' "$WRAPPER" || fail 'wrapper must guard its low-priority re-exec'
! grep -Eq 'VDIRSYNCER_BIN.*discover|run_bounded .* discover' "$WRAPPER" || fail 'routine wrapper must not rediscover remote collections'
grep -Fq 'sync-vdir.sh >/dev/null 2>&1' "$FAKE_CRONTAB" || fail 'cron must invoke the shared sync wrapper'
[ "$(grep -Ec '(^| )discover dash_family$' "$FAKE_VDIR_LOG")" -eq 1 ] || fail 'setup must discover the configured pair once'
[ "$(grep -Ec '(^| )sync dash_family$' "$FAKE_VDIR_LOG")" -eq 1 ] || fail 'initial setup must sync the configured pair once'
grep -Fq "$WRAPPER" "$FAKE_LOWPRIO_LOG" || fail 'initial sync must pass through dashboard-lowprio.sh'

# A regular scheduled/manual wrapper run synchronizes again, but must not
# perform discovery a second time. The helper marker prevents re-exec loops.
"$WRAPPER" >/dev/null 2>&1 || fail 'routine wrapper failed'
[ "$(grep -Ec '(^| )discover dash_family$' "$FAKE_VDIR_LOG")" -eq 1 ] || fail 'routine sync must not rediscover the known pair'
[ "$(grep -Ec '(^| )sync dash_family$' "$FAKE_VDIR_LOG")" -eq 2 ] || fail 'routine sync must execute exactly one normal vdirsyncer sync'
[ "$(wc -l < "$FAKE_LOWPRIO_LOG")" -eq 2 ] || fail 'each wrapper entry must pass through low priority exactly once'

echo 'PASS: private calendar discovery is setup-only and syncs inherit Dash-Go low priority'
