#!/bin/sh
set -eu
DASH="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
"$DASH/bin/dashboard-control-server" --gen-default-calendars "$@"
"$DASH/bin/dashboard-control-server" --gen-calendars >/dev/null 2>&1 || true
"$DASH/bin/dashboard-control-server" --gen-events-cache >/dev/null 2>&1 || true
