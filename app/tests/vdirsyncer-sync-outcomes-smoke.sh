#!/usr/bin/env bash
# Prove the truthful private-sync outcome pipeline end to end without network:
# the wrapper reports structured per-pair RESULT lines and a durable results
# file, classifies conflicts and the emptied-collection guard, propagates a
# dashboard delete of the final event only with explicit permission, Control
# activation discovers its new exact pair before the first sync, and a setup
# refresh preserves per-calendar edit flags written by the Go server's
# indented registry format.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
SETUP="$ROOT/bin/setup-vdirsyncer.sh"
SELECT="$ROOT/bin/private-calendar-selection.sh"
TMP="$(mktemp -d)"
cleanup(){ rm -rf "$TMP"; }
trap cleanup EXIT
HOME="$TMP/home"
export HOME
mkdir -p "$HOME/dashboard/bin" "$TMP/fake-bin" "$TMP/state"
cp "$SETUP" "$HOME/dashboard/bin/setup-vdirsyncer.sh"
cp "$SELECT" "$HOME/dashboard/bin/private-calendar-selection.sh"
chmod +x "$HOME/dashboard/bin/setup-vdirsyncer.sh" "$HOME/dashboard/bin/private-calendar-selection.sh"

cat > "$TMP/fake-bin/vdir-python" <<'PY'
#!/usr/bin/env bash
set -eu
[ "${1:-}" = "-c" ] && exit 0
exec /usr/bin/env bash "$@"
PY
# The fake vdirsyncer models the real tool's contract: sync refuses a pair
# that was never discovered, a conflicted item fails with the upstream
# message, and an emptied local collection is blocked unless --force-delete
# accompanies the run.
cat > "$TMP/fake-bin/vdirsyncer" <<'VDIR'
#!/usr/bin/env bash
set -eu
if [ "${1:-}" = "--version" ]; then printf 'vdirsyncer, version 0.20.0\n'; exit 0; fi
: "${FAKE_VDIR_LOG:?}" "${FAKE_STATE:?}"
printf '%s\n' "$*" >> "$FAKE_VDIR_LOG"
cfg=""
args=("$@")
for ((i=0; i<${#args[@]}; i++)); do
  [ "${args[$i]}" = "-c" ] && { cfg="${args[$((i + 1))]:-}"; break; }
done
root="$(dirname "${cfg:?}")"
force=0
pair=""
verb=""
for a in "$@"; do
  case "$a" in
    --force-delete) force=1;;
    discover|sync|metasync) verb="$a";;
    dash_*|discover_*) pair="$a";;
  esac
done
case "$verb" in
  discover)
    touch "$FAKE_STATE/discovered.$pair"
    exit 0
    ;;
  sync)
    [ -e "$FAKE_STATE/discovered.$pair" ] || { echo "critical: Please run \`vdirsyncer discover $pair\`  before synchronization." >&2; exit 1; }
    # Model the real parser's strictness: every pair keeps its section header,
    # and pair-only options may not leak into [general] or a storage section.
    # A one-shot resolution policy counts only when it sits inside this exact
    # pair's own section. This is what catches a config generator that drops
    # headers or injects into the wrong section.
    awk -v want="$pair" '
      /^\[/ { section=$0 }
      section=="[general]" && /^(a|b|collections|conflict_resolution)[[:space:]]*=/ { bad=1 }
      /^conflict_resolution[[:space:]]*=/ && section!="[pair " want "]" && section ~ /^\[pair / { misplaced=1 }
      $0=="[pair " want "]" { seen=1 }
      END { exit (seen && !bad) ? 0 : 1 }
    ' "$cfg" || { echo "critical: Error during reading config $cfg: Invalid general section." >&2; exit 1; }
    if [ -e "$FAKE_STATE/conflict.$pair" ]; then
      policy="$(awk -v want="$pair" '
        /^\[/ { active=($0=="[pair " want "]") }
        active && /^conflict_resolution[[:space:]]*=/ { print; exit }
      ' "$cfg")"
      case "$policy" in
        *'"a wins"'*) printf 'remote\n' > "$FAKE_STATE/resolved.$pair";;
        *'"b wins"'*) printf 'dashboard\n' > "$FAKE_STATE/resolved.$pair";;
        *)
          echo "error: $pair/x: One item changed on both sides. Resolve this conflict manually, or by setting the \`conflict_resolution\` parameter in your config file." >&2
          exit 1
          ;;
      esac
    fi
    if [ -e "$FAKE_STATE/empty.$pair" ] && [ "$force" -ne 1 ]; then
      echo "error: $pair/x: Storage \"${pair}_local/x\" was completely emptied. If you want to delete ALL entries on BOTH sides, then use \`vdirsyncer sync --force-delete\`." >&2
      exit 1
    fi
    case "$pair" in
      dash_family)
        mkdir -p "$root/collections/family/collection-1"
        printf 'BEGIN:VCALENDAR\nVERSION:2.0\nBEGIN:VEVENT\nUID:one\nSUMMARY:Private event\nEND:VEVENT\nEND:VCALENDAR\n' > "$root/collections/family/collection-1/one.ics"
        ;;
      dash_pc_*)
        name="${pair#dash_}"
        dir="$(find "$root/collections/$name" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | head -n1)"
        [ -n "$dir" ] || dir="$root/collections/$name/collection_fixture"
        mkdir -p "$dir"
        printf 'BEGIN:VCALENDAR\nVERSION:2.0\nBEGIN:VEVENT\nUID:two\nSUMMARY:Selected event\nEND:VEVENT\nEND:VCALENDAR\n' > "$dir/two.ics"
        ;;
    esac
    exit 0
    ;;
esac
exit 0
VDIR
cat > "$TMP/fake-bin/crontab" <<'CRON'
#!/usr/bin/env bash
exit 0
CRON
cat > "$HOME/dashboard/bin/dashboard-lowprio.sh" <<'LOWPRIO'
#!/usr/bin/env bash
exec "$@"
LOWPRIO
printf '#!/usr/bin/env bash\nexit 0\n' > "$HOME/dashboard/bin/gen-calendars.sh"
printf '#!/usr/bin/env bash\nexit 0\n' > "$HOME/dashboard/bin/dashboard-control-server"
chmod +x "$TMP/fake-bin/vdir-python" "$TMP/fake-bin/vdirsyncer" "$TMP/fake-bin/crontab" \
  "$HOME/dashboard/bin/dashboard-lowprio.sh" "$HOME/dashboard/bin/gen-calendars.sh" "$HOME/dashboard/bin/dashboard-control-server"

export PATH="$TMP/fake-bin:$PATH" \
  DASH_VDIRSYNCER_BIN="$TMP/fake-bin/vdirsyncer" \
  DASH_VDIRSYNCER_PYTHON="$TMP/fake-bin/vdir-python" \
  FAKE_VDIR_LOG="$TMP/vdir.log" \
  FAKE_STATE="$TMP/state"
: > "$FAKE_VDIR_LOG"

printf 'family\nblue\nn\n1\nhttps://caldav.example/\nfamily@example.com\nfixture-password\ncollection-1\ny\n\n' \
  | "$HOME/dashboard/bin/setup-vdirsyncer.sh" > "$TMP/setup.out" 2>&1 || { cat "$TMP/setup.out" >&2; exit 1; }

fail(){ echo "FAIL: $*" >&2; cat "$TMP/setup.out" >&2; exit 1; }
WRAPPER="$HOME/dashboard/bin/sync-vdir.sh"
RESULTS="$HOME/.dashboard-vdirsyncer/last-sync-results"
REG="$HOME/dashboard/config/calendar-writeback.json"
[ -x "$WRAPPER" ] || fail 'generated sync wrapper missing'

# 1) Healthy targeted run: structured RESULT on stdout plus a durable row.
out="$("$WRAPPER" --pair dash_family)" || fail 'healthy targeted sync failed'
printf '%s\n' "$out" | grep -q $'^RESULT\tdash_family\tsynced$' || fail 'wrapper must report a structured synced RESULT line'
grep -q '^dash_family|synced|' "$RESULTS" || fail 'results file missing the synced row'

# 2) Conflict: classified on stdout and durable, calendar mirror retained.
touch "$FAKE_STATE/conflict.dash_family"
out="$("$WRAPPER" --pair dash_family)" && fail 'conflicted sync must exit nonzero'
printf '%s\n' "$out" | grep -q $'^RESULT\tdash_family\tconflict$' || fail 'conflict must be classified on stdout for the dashboard queue'
grep -q '^dash_family|conflict|' "$RESULTS" || fail 'results file missing the conflict row'
grep -q 'Private event' "$HOME/dashboard/calendars/family.blue.ics" || fail 'conflict must keep the previous mirror'
# One-shot conflict winners must never appear in the normal generated config.
! grep -q '^conflict_resolution' "$HOME/.dashboard-vdirsyncer/config" || fail 'normal config must not retain a conflict winner'
out="$("$WRAPPER" --pair dash_family --resolve-conflict remote)" || fail 'targeted remote conflict resolution failed'
printf '%s\n' "$out" | grep -q $'^RESULT\tdash_family\tsynced$' || fail 'remote conflict resolution must report synced'
[ "$(cat "$FAKE_STATE/resolved.dash_family")" = remote ] || fail 'remote winner policy did not reach only the temporary config'
! find "$HOME/.dashboard-vdirsyncer" -maxdepth 1 -name 'resolve-vdir.*' -print -quit | grep -q . || fail 'temporary conflict config must be removed after remote resolution'
out="$("$WRAPPER" --pair dash_family --resolve-conflict dashboard)" || fail 'targeted Dashboard conflict resolution failed'
printf '%s\n' "$out" | grep -q $'^RESULT\tdash_family\tsynced$' || fail 'Dashboard conflict resolution must report synced'
[ "$(cat "$FAKE_STATE/resolved.dash_family")" = dashboard ] || fail 'Dashboard winner policy did not reach only the temporary config'
! find "$HOME/.dashboard-vdirsyncer" -maxdepth 1 -name 'resolve-vdir.*' -print -quit | grep -q . || fail 'temporary conflict config must be removed after Dashboard resolution'
rm -f "$FAKE_STATE/conflict.dash_family"

# 3) Emptied collection: blocked without permission, classified as attention;
#    propagates only with the explicit --allow-empty-once permission.
touch "$FAKE_STATE/empty.dash_family"
out="$("$WRAPPER" --pair dash_family)" && fail 'emptied collection must be blocked without permission'
printf '%s\n' "$out" | grep -q $'^RESULT\tdash_family\tattention-empty$' || fail 'empty-guard must classify as attention-empty'
out="$("$WRAPPER" --pair dash_family --allow-empty-once)" || fail 'final-delete run must succeed with explicit permission'
printf '%s\n' "$out" | grep -q $'^RESULT\tdash_family\tsynced$' || fail 'permitted final-delete run must report synced'
grep -q -- '--force-delete' "$FAKE_VDIR_LOG" || fail 'permitted final-delete run must pass --force-delete to vdirsyncer'
[ "$(grep -c -- '--force-delete' "$FAKE_VDIR_LOG")" -eq 1 ] || fail 'force-delete must be a one-run permission'
rm -f "$FAKE_STATE/empty.dash_family"

# 4) Control activation must discover its new exact pair, then sync ready.
out="$("$HOME/dashboard/bin/private-calendar-selection.sh" --activate dash_family remote-two "Second" green 1)" || fail 'activation failed'
printf '%s\n' "$out" | grep -q $'^activated\t' || fail 'activation must report activated'
printf '%s\n' "$out" | grep -q $'\tready$' || fail 'activation with discovery must reach the ready state'
newpair="$(printf '%s\n' "$out" | cut -f5)"
grep -q "discover $newpair" "$FAKE_VDIR_LOG" || fail 'activation must discover the new exact pair before its first sync'

# 5) An exact-pair repair discovers and synchronizes only the selected pair.
newsource="$(printf '%s\n' "$out" | cut -f2)"
rm -f "$FAKE_STATE/discovered.$newpair"
: > "$FAKE_VDIR_LOG"
out="$($HOME/dashboard/bin/private-calendar-selection.sh --repair "$newsource")" || fail 'targeted exact-pair repair failed'
printf '%s\n' "$out" | grep -Fqx "$(printf 'repaired\t%s\t%s' "$newsource" "$newpair")" || fail 'repair must return only its selected source and pair'
grep -q "discover $newpair" "$FAKE_VDIR_LOG" || fail 'repair must discover only the selected pair'
! grep -q 'discover dash_family' "$FAKE_VDIR_LOG" || fail 'repair must not broad-discover another pair'

# 6) A full run merges rows: both pairs present in the durable results file.
"$WRAPPER" >/dev/null || fail 'full sync run failed'
grep -q '^dash_family|synced|' "$RESULTS" || fail 'full run lost the family row'
grep -q "^$newpair|synced|" "$RESULTS" || fail 'full run lost the selected-pair row'

# 7) Server-formatted registry flags survive a setup refresh.
python3 - "$REG" <<'PYSET'
import json,sys
reg=json.load(open(sys.argv[1]))
reg["enabled"]=True
for cal in reg["calendars"]:
    if cal["name"]=="family":
        cal["enabled"]=False
open(sys.argv[1],"w").write(json.dumps(reg,indent=" ")+"\n")
PYSET
"$HOME/dashboard/bin/setup-vdirsyncer.sh" --refresh >/dev/null 2>&1 || fail 'setup refresh failed'
python3 - "$REG" <<'PYCHK'
import json,sys
reg=json.load(open(sys.argv[1]))
assert reg["enabled"] is True, "master edit switch lost by refresh"
rows={c["name"]:c for c in reg["calendars"]}
assert rows["family"]["enabled"] is False, "per-calendar disabled flag lost by refresh"
PYCHK

echo 'PASS: structured sync outcomes, one-shot conflict resolution, guarded final deletes, activation discovery, and refresh-safe edit flags hold'
