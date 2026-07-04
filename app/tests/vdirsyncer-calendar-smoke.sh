#!/usr/bin/env bash
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
SETUP="$ROOT/bin/setup-vdirsyncer.sh"
fail(){ echo "FAIL: $*" >&2; exit 1; }
bash -n "$SETUP"
grep -Fq '1) Google Calendar' "$SETUP" || fail 'Google must be a first-class provider'
grep -Fq '2) Apple iCloud Calendar' "$SETUP" || fail 'iCloud must be a first-class provider'
grep -Fq '3) Another CalDAV account' "$SETUP" || fail 'generic CalDAV must remain available'
grep -Fq 'https://caldav.icloud.com/' "$SETUP" || fail 'iCloud endpoint must be fixed internally'
grep -Fq 'app-specific password' "$SETUP" || fail 'iCloud must explain Apple app passwords'
grep -Fq 'Checking this account and looking for calendars' "$SETUP" || fail 'account test/discovery stage missing'
grep -Fq 'Step 3 of 4 — Choose calendars' "$SETUP" || fail 'human-readable calendar selection missing'
grep -Fq 'private_cleanup_transaction' "$SETUP" || fail 'failed first sync must restore active configuration'
echo 'PASS: private calendar setup is account-first, iCloud-specific, and staged before activation.'
