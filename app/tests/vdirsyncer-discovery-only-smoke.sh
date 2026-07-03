#!/usr/bin/env bash
# Discovery must inventory remote collections in an isolated stage. It may not
# alter active configuration, mirrors, cache, map, pairs, or cron state.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
DISCOVERY="$ROOT/bin/private-calendar-discovery.sh"
TMP="$(mktemp -d)"
cleanup(){ rm -rf "$TMP"; }
trap cleanup EXIT
HOME="$TMP/home"; export HOME
VDIR="$HOME/.dashboard-vdirsyncer"
mkdir -p "$VDIR/passwords" "$VDIR/google-tokens" "$VDIR/collections" "$HOME/dashboard/calendars" "$TMP/fake-bin"
printf 'token' > "$VDIR/google-tokens/gacct.json"
printf 'secret' > "$VDIR/passwords/gacct.google-client-secret"
printf 'dashboard-sentinel\n' > "$HOME/dashboard/calendars/active.ics"
printf 'active-config\n' > "$VDIR/config"
printf 'pc_base|blue||dash_pc_base|%s/collections/pc_base||||google|fixture.apps.googleusercontent.com|Google account|gacct|\n' "$VDIR" > "$VDIR/pairs"
printf 'pc_base|blue||dash_pc_base|%s/collections/pc_base|0||Google account|google|gacct|\n' "$VDIR" > "$VDIR/calendars.map"
cp "$VDIR/pairs" "$TMP/pairs.before"; cp "$VDIR/calendars.map" "$TMP/map.before"; cp "$VDIR/config" "$TMP/config.before"
cat > "$TMP/fake-bin/vdirsyncer" <<'VDIR'
#!/usr/bin/env bash
set -euo pipefail
cfg="${VDIRSYNCER_CONFIG:-}"
if [ "${1:-}" = "-c" ]; then cfg="${2:-}"; fi
root="$(dirname "$cfg")"
case " $* " in
  *' discover discover_gacct '*)
    mkdir -p "$root/collections/gacct/family-opaque" "$root/collections/gacct/work-opaque"
    printf 'Family\n' > "$root/collections/gacct/family-opaque/displayname"
    printf 'teal\n' > "$root/collections/gacct/family-opaque/color"
    printf 'Work\n' > "$root/collections/gacct/work-opaque/displayname"
    printf 'orange\n' > "$root/collections/gacct/work-opaque/color"
    ;;
esac
exit 0
VDIR
chmod +x "$TMP/fake-bin/vdirsyncer"
export DASH_VDIR_HOME="$VDIR" DASH_VDIRSYNCER_BIN="$TMP/fake-bin/vdirsyncer"
OUT="$($DISCOVERY)"
printf '%s\n' "$OUT" | grep -Fq $'calendar\tdash_pc_base\tgoogle\tgacct\tfamily-opaque\tFamily\tteal'
printf '%s\n' "$OUT" | grep -Fq $'calendar\tdash_pc_base\tgoogle\tgacct\twork-opaque\tWork\torange'
cmp "$TMP/pairs.before" "$VDIR/pairs"
cmp "$TMP/map.before" "$VDIR/calendars.map"
cmp "$TMP/config.before" "$VDIR/config"
grep -Fq 'dashboard-sentinel' "$HOME/dashboard/calendars/active.ics"
! find "$VDIR" -maxdepth 1 -type d -name 'discovery.*' -print -quit | grep -q .
printf 'PASS: private calendar discovery inventories an isolated vdir stage without activating collections\n'
