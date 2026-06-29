#!/usr/bin/env bash
# Check that Doctor's visible numbers and selectable parser cannot drift apart.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TMP="$(mktemp -d)"; trap 'rm -rf "$TMP"' EXIT INT TERM
DOCTOR_PLAN_FILE="$TMP/plan.tsv"; export DOCTOR_PLAN_FILE
# shellcheck source=/dev/null
. "$ROOT/bin/dashboard-doctor-plan.sh"
warn(){ printf 'WARN %s\n' "$*" >&2; }
doctor_plan_add safe legacy-autostart 'Repair legacy autostart' 'fixture' 'fixed' 'prefs'
doctor_plan_add admin autologin-session 'Repair LightDM autologin' 'fixture' 'fixed' 'prefs'
doctor_plan_add repair refresh-app 'Refresh application files' 'fixture' 'installer replaces app' 'prefs'
doctor_plan_add manual storage 'Review storage health' 'fixture' 'manual review' 'prefs'
out="$TMP/render.txt"; doctor_plan_render > "$out"
grep -Fq '[1] Repair legacy autostart' "$out"
grep -Fq '[2] Repair LightDM autologin' "$out"
grep -Fq '[info] Refresh application files' "$out"
grep -Fq '[info] Review storage health' "$out"
[ "$(doctor_plan_ids_from_numbers '1,2')" = 'legacy-autostart,autologin-session' ]
if doctor_plan_ids_from_numbers '3' >/dev/null 2>&1; then
  echo 'FAIL: non-selectable plan item received a number' >&2; exit 1
fi
hint="$TMP/hint.txt"; doctor_plan_selection_hint > "$hint"
grep -Fq 'run ~/install.sh --repair' "$hint"
if doctor_plan_post_repair_summary > "$TMP/post.txt"; then
  echo 'FAIL: remaining repair/manual items did not require follow-up' >&2; exit 1
fi
grep -Fq 'Remaining installer repair' "$TMP/post.txt"
grep -Fq 'Remaining manual review' "$TMP/post.txt"
echo 'PASS: Doctor numbers only selectable repairs and clearly hands off installer/manual work'
