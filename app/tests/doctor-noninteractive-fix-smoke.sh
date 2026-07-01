#!/usr/bin/env bash
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
DOCTOR="$ROOT/bin/doctor.sh"
grep -Fq '[ "$FIX" -eq 1 ] && [ -z "$FIX_ONLY" ] && [ ! -t 0 ]' "$DOCTOR"
grep -Fq 'SAFE_ONLY=1' "$DOCTOR"
grep -Fq 'Non-interactive --fix is limited to safe repairs' "$DOCTOR"
printf 'PASS: redirected Doctor --fix is safe-only unless explicit --only keys are supplied
'
