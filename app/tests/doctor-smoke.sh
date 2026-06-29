#!/usr/bin/env bash
# Release-blocking regression for Doctor truthfulness, repair-plan interaction,
# canonical scheduler repair, and conservative generated-data recovery.
set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TMP="$(mktemp -d)"
FIXTURE="$TMP/dashboard"
FAKE_BIN="$TMP/fake-bin"
HOME_FIXTURE="$TMP/home"
OUTPUT="$TMP/doctor.out"
PLAN_OUTPUT="$TMP/doctor-plan.out"
SECOND_PLAN_OUTPUT="$TMP/doctor-second-plan.out"

cleanup(){ rm -rf "$TMP"; }
trap cleanup EXIT INT TERM

mkdir -p "$FIXTURE/bin" "$FIXTURE/ui/js" "$FIXTURE/ui" "$FIXTURE/config" \
  "$FIXTURE/cache/kiosk.lock" "$FIXTURE/logs" "$FAKE_BIN" "$HOME_FIXTURE"
printf '1.4.1-beta.11\n' > "$FIXTURE/VERSION"
printf '{"version":"1.4.1-beta.11","files":[]}\n' > "$FIXTURE/manifest.json"
printf '<!doctype html><title>doctor fixture</title>\n' > "$FIXTURE/index.html"
printf 'body{}\n' > "$FIXTURE/ui/dashboard.css"
printf 'body{}\n' > "$FIXTURE/ui/control-layout.css"
printf 'window.fixture=true;\n' > "$FIXTURE/ui/js/app.bundle.js"
printf 'window.controlFixture=true;\n' > "$FIXTURE/ui/js/app.control.bundle.js"
printf 'window.DASHBOARD_LOCAL={pauseWhileOpen:true profile:"lite",lat:41.8781,lon:-87.6298};\n' > "$FIXTURE/config/config.local.js"
printf '{invalid settings\n' > "$FIXTURE/config/settings.json"
printf '{}\n' > "$FIXTURE/cache/weather-cache.json"
printf '999999\n' > "$FIXTURE/cache/kiosk.lock/pid"

for name in doctor.sh dashboard-kiosk-lib.sh dashboard-lite-session.sh dashboard-session-guard.sh dashboard-lowprio.sh dashboard-housekeeping.sh update-holidays.sh update-iss-passes.sh gen-default-calendars.sh; do
  printf '#!/usr/bin/env bash\n' > "$FIXTURE/bin/$name"
done
cp "$ROOT/bin/dashboard-common.sh" "$FIXTURE/bin/dashboard-common.sh"
cp "$ROOT/bin/dashboard-doctor-plan.sh" "$FIXTURE/bin/dashboard-doctor-plan.sh"
cp "$ROOT/bin/dashboard-health-guard.sh" "$FIXTURE/bin/dashboard-health-guard.sh"
printf '#!/usr/bin/env bash\nexit 0\n' > "$FIXTURE/kiosk.sh"
chmod +x "$FIXTURE/kiosk.sh" "$FIXTURE/bin/"*.sh

cat > "$FIXTURE/bin/dashboard-control-server" <<'FAKE_SERVER'
#!/usr/bin/env bash
set -eu
case "${1:-}" in
  --json-validate)
    grep -q '^\{invalid' "$2" && exit 1
    exit 0
    ;;
  --json-get)
    [ "${3:-}" = version ] && printf '%s\n' '1.4.1-beta.11'
    ;;
  --verify-generated-assets)
    if [ "${2:-}" = --write ]; then touch "$DASH/cache/generated-ok"; exit 0; fi
    [ -f "$DASH/cache/generated-ok" ]
    ;;
  --doctor-config)
    case "${2:-}" in
      --repair) touch "$DASH/cache/config-repaired" ;;
      --location-check)
        [ -f "$DASH/cache/config-repaired" ] || echo 'SYNTAX_KNOWN:fixture'
        echo 'LOCATION_OK:41.878100,-87.629800:Fixture'
        [ -f "$DASH/cache/weather-cache.json" ] && echo 'WEATHERAPI_ZERO_CACHE'
        ;;
    esac
    ;;
  --doctor-data)
    [ -f "$DASH/calendars/calendars.json" ] && echo 'CALENDAR_MANIFEST_OK:0 entries' || echo 'CALENDAR_MANIFEST_ABSENT'
    [ -f "$DASH/cache/events.cache.json" ] && echo 'EVENT_CACHE_OK' || echo 'EVENT_CACHE_ABSENT'
    [ -f "$DASH/cache/weather-cache.json" ] && echo 'WEATHER_CACHE_ERROR_PAYLOAD' || echo 'WEATHER_CACHE_ABSENT'
    [ -f "$DASH/config/message-cache.json" ] && echo 'MESSAGE_CACHE_OK' || echo 'MESSAGE_CACHE_ABSENT'
    ;;
  --gen-calendars)
    mkdir -p "$DASH/calendars"; printf '[]\n' > "$DASH/calendars/calendars.json"
    ;;
  --gen-events-cache)
    mkdir -p "$DASH/cache"; printf '{"version":2,"generatedAt":1,"windowStart":1,"windowEnd":2,"events":[],"issues":[]}\n' > "$DASH/cache/events.cache.json"
    ;;
  *) exit 0 ;;
esac
FAKE_SERVER
chmod +x "$FIXTURE/bin/dashboard-control-server"

cat > "$FAKE_BIN/crontab" <<'FAKE_CRON'
#!/usr/bin/env bash
set -eu
case "${1:-}" in
  -l) [ -f "$DASH/cache/test-crontab" ] && cat "$DASH/cache/test-crontab" || exit 1 ;;
  *) cp "$1" "$DASH/cache/test-crontab" ;;
esac
FAKE_CRON
cat > "$FAKE_BIN/pgrep" <<'FAKE_PGREP'
#!/usr/bin/env bash
set -eu
last="${!#}"
case " $* " in
  *' -x openbox '*) exit 0 ;;
  *' -afu '*)
    # The beta.6 expression passed kiosk\\.sh and matched nothing. Beta.7
    # must pass the simple token and rely on the fixed full-path filter.
    [ "$last" = kiosk.sh ] || exit 1
    printf '4242 %s/kiosk.sh\n' "$DASH"
    ;;
  *' -u '*' -x surf '*) exit 1 ;;
  *) exit 1 ;;
esac
FAKE_PGREP
for name in surf openbox wmctrl xset curl; do
  printf '#!/usr/bin/env bash\nexit 0\n' > "$FAKE_BIN/$name"
  chmod +x "$FAKE_BIN/$name"
done
chmod +x "$FAKE_BIN/crontab" "$FAKE_BIN/pgrep"

# Every canonical Lite job is present except housekeeping, intentionally using
# an old minute. The user-facing scan must name the job, not leak raw crontab.
cat > "$FIXTURE/cache/test-crontab" <<EOF_CRON
5 9 * * * echo keep-me
*/20 * * * * $FIXTURE/bin/dashboard-lowprio.sh $FIXTURE/bin/dashboard-control-server --gen-events-cache >/dev/null 2>&1
17 5 * * * $FIXTURE/bin/dashboard-lowprio.sh $FIXTURE/bin/dashboard-control-server --update-message-feeds >/dev/null 2>&1
0 4 1 * * $FIXTURE/bin/update-holidays.sh
22 4 */3 * * $FIXTURE/bin/update-iss-passes.sh >/dev/null 2>&1
37 4 * * * $FIXTURE/bin/dashboard-lowprio.sh $FIXTURE/bin/gen-default-calendars.sh >/dev/null 2>&1
8 3 */3 * * $FIXTURE/bin/dashboard-lowprio.sh $FIXTURE/bin/dashboard-housekeeping.sh >/dev/null 2>&1
*/30 * * * * $FIXTURE/bin/dashboard-health-guard.sh >/dev/null 2>&1
55 1 * * * pkill -x surf >/dev/null 2>&1 # dashboard-nightly-browser-restart
EOF_CRON

printf '%s\n' \
  '2026-06-20 20:00:00 kiosk session started pid=123 display=:0' \
  '2026-06-20 20:00:01 fullscreen helper started pid=124 surf_pid=123' > "$FIXTURE/logs/kiosk.log"
printf '%s\n' \
  '2026-06-20 20:00:00 openbox session starting' \
  'Dash-Go kiosk already running as pid 15307; this duplicate launcher will exit' > "$FIXTURE/logs/openbox-session.log"

run_doctor(){
  PATH="$FAKE_BIN:$PATH" HOME="$HOME_FIXTURE" DASH="$FIXTURE" DOCTOR_SKIP_SYSTEM=1 DOCTOR_PROFILE=lite \
    DOCTOR_XSESSION_FILE="$TMP/no-xsession" DOCTOR_AUTOLOGIN_FILE="$TMP/no-autologin" \
    bash "$ROOT/bin/doctor.sh" "$@"
}

# Plan only: no mutation, single root-cause weather warning, friendly schedule,
# no false kiosk/duplicate-exit warning.
run_doctor --plan --no-prompt > "$PLAN_OUTPUT" 2>&1 || true
grep -q '^== Repair plan$' "$PLAN_OUTPUT"
grep -q '^Safe repairs available' "$PLAN_OUTPUT"
grep -q 'Restore canonical Dash-Go scheduled jobs' "$PLAN_OUTPUT"
grep -q 'WARN scheduled Dash-Go jobs need attention: housekeeping job is missing or uses a non-canonical schedule — Suggestion:' "$PLAN_OUTPUT"
! grep -Fq "8 3 */3 * * $FIXTURE/bin/dashboard-lowprio.sh" "$PLAN_OUTPUT"
grep -q 'WARN weather cache contains 0,0 coordinates — Suggestion:' "$PLAN_OUTPUT"
! grep -q 'last weather cache is a provider-error response' "$PLAN_OUTPUT"
! grep -q '^WARN .* — Fix:' "$PLAN_OUTPUT"
! grep -q '^FIXED ' "$PLAN_OUTPUT"
! grep -q 'no kiosk launcher is running' "$PLAN_OUTPUT"
! grep -q '^WARN current Openbox session has launcher failures:' "$PLAN_OUTPUT"

# B3: helper-level test verifies one render normally and exactly two across the
# explicit Details path (normal + detailed), never an accidental third render.
plan_flow_check="$TMP/plan-flow-check.sh"
cat > "$plan_flow_check" <<'EOF_PLAN'
#!/usr/bin/env bash
set -eu
source "${DOCTOR_PLAN_ROOT:?}/bin/dashboard-doctor-plan.sh"
renders=0
doctor_plan_has(){ return 0; }
doctor_plan_render(){ renders=$((renders+1)); }
warn(){ :; }
normal="$(mktemp)"
details="$(mktemp)"
trap 'rm -f "$normal" "$details"' EXIT
printf 'n\n' > "$normal"
doctor_plan_interactive < "$normal"
[ "$renders" = 1 ]
renders=0
printf 'd\nn\n' > "$details"
doctor_plan_interactive < "$details"
[ "$renders" = 2 ]
EOF_PLAN
chmod +x "$plan_flow_check"
DOCTOR_PLAN_ROOT="$ROOT" "$plan_flow_check" >/dev/null
python3 - "$ROOT/bin/doctor.sh" <<'PYPLAN'
from pathlib import Path
import sys
text=Path(sys.argv[1]).read_text(encoding='utf-8')
required='''if [ "$PLAN_MODE" -eq 1 ]; then
  if [ "$INTERACTIVE_PLAN" -eq 1 ]; then
    doctor_plan_interactive
  else
    doctor_plan_render
  fi
'''
if required not in text:
    raise SystemExit('interactive plan dispatcher can render a duplicate plan')
for required in (
    'if [ "$VERBOSE" = 1 ]; then',
    'info_quiet "openbox-session.log has no current-session marker',
    'info_quiet "kiosk.log predates current-session markers',
):
    if required not in text:
        raise SystemExit('compact Doctor output lost its quiet-lane contract')
PYPLAN

# Safe repair must be bounded, preserve user cron, and restore canonical Lite
# cadence. This is the only mutating Doctor pass in the fixture.
run_doctor --yes --no-prompt > "$OUTPUT" 2>&1
grep -q '^FIXED regenerated stale JavaScript and CSS browser assets' "$OUTPUT"
grep -q '^FIXED repaired known config.local.js syntax error' "$OUTPUT"
grep -q '^FIXED backed up and reset invalid config/settings.json' "$OUTPUT"
grep -q '^FIXED removed cached 0,0 weather response so it can refresh' "$OUTPUT"
grep -q '^FIXED regenerated calendars/calendars.json' "$OUTPUT"
grep -q '^FIXED quarantined and rebuilt the invalid event cache' "$OUTPUT"
grep -q '^FIXED removed stale kiosk lock' "$OUTPUT"
grep -q '^FIXED restored canonical scheduled events, messages, calendars, holidays, housekeeping, and health guard' "$OUTPUT"
! grep -q '^FAIL ' "$OUTPUT"
! grep -q 'launcher failures:.*fullscreen helper started' "$OUTPUT"
grep -Fqx '{}' "$FIXTURE/config/settings.json"
grep -Fqx '[]' "$FIXTURE/calendars/calendars.json"
grep -q '"events"' "$FIXTURE/cache/events.cache.json"
[ ! -e "$FIXTURE/cache/weather-cache.json" ]
[ ! -d "$FIXTURE/cache/kiosk.lock" ]

event_line="*/20 * * * * $FIXTURE/bin/dashboard-lowprio.sh $FIXTURE/bin/dashboard-control-server --gen-events-cache >/dev/null 2>&1"
message_line="17 5 * * * $FIXTURE/bin/dashboard-lowprio.sh $FIXTURE/bin/dashboard-control-server --update-message-feeds >/dev/null 2>&1"
housekeeping_line="7 3 * * * $FIXTURE/bin/dashboard-lowprio.sh $FIXTURE/bin/dashboard-housekeeping.sh >/dev/null 2>&1"
health_guard_line="*/30 * * * * $FIXTURE/bin/dashboard-health-guard.sh >/dev/null 2>&1"
[ "$(grep -Fxc "$event_line" "$FIXTURE/cache/test-crontab")" = 1 ]
[ "$(grep -Fxc "$message_line" "$FIXTURE/cache/test-crontab")" = 1 ]
[ "$(grep -Fxc "$housekeeping_line" "$FIXTURE/cache/test-crontab")" = 1 ]
[ "$(grep -Fxc "$health_guard_line" "$FIXTURE/cache/test-crontab")" = 1 ]
grep -Fqx "0 4 1 * * $FIXTURE/bin/update-holidays.sh" "$FIXTURE/cache/test-crontab"
grep -Fqx "22 4 */3 * * $FIXTURE/bin/update-iss-passes.sh >/dev/null 2>&1" "$FIXTURE/cache/test-crontab"
grep -Fqx "37 4 * * * $FIXTURE/bin/dashboard-lowprio.sh $FIXTURE/bin/gen-default-calendars.sh >/dev/null 2>&1" "$FIXTURE/cache/test-crontab"
grep -Fqx '5 9 * * * echo keep-me' "$FIXTURE/cache/test-crontab"
[ "$(grep -Fxc '55 1 * * * pkill -x surf >/dev/null 2>&1 # dashboard-nightly-browser-restart' "$FIXTURE/cache/test-crontab")" = 1 ]

# Idempotency: a fresh plan after an untouched repair must contain no automatic
# repair candidates and must not resurrect the noncanonical housekeeping issue.
run_doctor --plan --no-prompt > "$SECOND_PLAN_OUTPUT" 2>&1 || true
grep -q '^INFO No automatic repairs are currently planned\.$' "$SECOND_PLAN_OUTPUT"
! grep -q 'scheduled Dash-Go jobs need attention' "$SECOND_PLAN_OUTPUT"

echo 'PASS: Doctor plan is singular and readable, recognizes a live kiosk launcher, ignores benign duplicate exits, repairs the named scheduler job, and is idempotent on the next scan'
