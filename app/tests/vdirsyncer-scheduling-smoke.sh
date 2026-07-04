#!/usr/bin/env bash
# Prove private-calendar discovery stays explicit and routine sync remains low-priority.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
SETUP="$ROOT/bin/setup-vdirsyncer.sh"
SELECT="$ROOT/bin/private-calendar-selection.sh"
fail(){ echo "FAIL: $*" >&2; exit 1; }
bash -n "$SETUP"
bash -n "$SELECT"
grep -Fq 'private_discover_draft' "$SETUP" || fail 'guided setup must use a private discovery draft'
grep -Fq 'private_choose_and_activate' "$SETUP" || fail 'calendar selection must follow discovery'
grep -Fq 'write_sync_wrapper' "$SETUP" || fail 'setup must still generate the bounded sync wrapper'
grep -Fq 'dashboard-lowprio.sh' "$SETUP" || fail 'generated wrapper must retain Dash-Go low-priority execution'
! grep -Eq 'run_bounded .*discover' "$SETUP" || fail 'routine wrapper must not rediscover remote calendars'
grep -Fq 'discover "$pair"' "$SELECT" || fail 'only explicit selection/repair may discover an exact pair'
grep -Fq '"$SYNC" --pair "$pair"' "$SELECT" || fail 'selected exact pair must receive first sync before promotion'
echo 'PASS: discovery remains an explicit setup/selection action and routine sync stays bounded and low priority.'
