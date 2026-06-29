#!/usr/bin/env bash
# Calendar fetching/index rebuild ownership is Go/server-side. The former shell
# library was unreferenced dead code and must not quietly return.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
[ ! -e "$ROOT/bin/calendar-fetch-lib.sh" ] || { echo 'FAIL: obsolete calendar-fetch-lib.sh returned' >&2; exit 1; }
if grep -R -n -E 'calendar-fetch-lib|calendar_fetch_ics|calendar_refresh_indexes' "$ROOT/bin" "$ROOT/../installer" >/dev/null 2>&1; then
  echo 'FAIL: obsolete calendar fetch library still has a reference' >&2
  exit 1
fi
printf 'PASS: obsolete calendar fetch library is absent and unreferenced\n'
