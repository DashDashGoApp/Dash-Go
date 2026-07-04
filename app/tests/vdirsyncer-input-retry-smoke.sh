#!/usr/bin/env bash
# Focused prompt-level regression coverage: a typo must stay at that field and
# headless SSH must choose the phone/QR path by default.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
SETUP="$ROOT/bin/setup-vdirsyncer.sh"
[ -x "$SETUP" ] || { echo 'FAIL: setup-vdirsyncer missing' >&2; exit 1; }
# shellcheck disable=SC1090
DASHGO_TEST_LIBRARY_ONLY=1 source "$SETUP"

is_good(){ [ "$1" = good ]; }
TARGET=""
prompt_retry TARGET 'Test value' is_good 'type good' 0 < <(printf 'bad\ngood\n')
[ "$TARGET" = good ] || { echo 'FAIL: prompt_retry did not retry the field' >&2; exit 1; }

PRIVATE_SELECTION=""
private_prompt_calendar_selection 3 < <(printf '99\n2\n')
[ "$PRIVATE_SELECTION" = 2 ] || { echo 'FAIL: calendar selection did not recover from invalid number' >&2; exit 1; }

PRIVATE_USERNAME="" PRIVATE_SECRET=""
private_icloud_credentials_prompt < <(printf 'person@example.com \n abcd-efgh-ijkl-mnop \n')
[ "$PRIVATE_USERNAME" = 'person@example.com' ] || { echo 'FAIL: iCloud email whitespace was not trimmed' >&2; exit 1; }
[ "$PRIVATE_SECRET" = 'abcd-efgh-ijkl-mnop' ] || { echo 'FAIL: iCloud password whitespace was not trimmed' >&2; exit 1; }

SSH_CONNECTION='198.51.100.9 12345 192.0.2.20 22'
DISPLAY='' WAYLAND_DISPLAY='' PRIVATE_GOOGLE_MODE=''
private_google_signin_mode_prompt < <(printf '\n')
[ "$PRIVATE_GOOGLE_MODE" = phone ] || { echo 'FAIL: headless SSH default did not choose phone/QR mode' >&2; exit 1; }

for required in \
  'private_cleanup_transaction 2>/dev/null || true' \
  'trap cleanup_setup_artifacts EXIT HUP INT TERM' \
  'Calendar discovery took too long' \
  'timeout 180 "$BIN_DIR/sync-vdir.sh"' \
  'This account looks already connected as'; do
  grep -Fq -- "$required" "$SETUP" || { echo "FAIL: missing setup recovery contract: $required" >&2; exit 1; }
done
printf 'PASS: private-calendar retry, trim, SSH default, timeout, and cleanup contracts\n'
