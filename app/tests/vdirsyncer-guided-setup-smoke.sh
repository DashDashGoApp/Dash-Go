#!/usr/bin/env bash
# Exercise the account-first Google and iCloud setup paths without network.
# The fixture proves discovery happens before activation, iCloud never asks
# for server/collection internals, and a healthy managed vdirsyncer skips the
# pipx install/repair path.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
SETUP="$ROOT/bin/setup-vdirsyncer.sh"
TMP="$(mktemp -d)"
cleanup(){ rm -rf "$TMP"; }
trap cleanup EXIT
fail(){ echo "FAIL: $*" >&2; for f in "$TMP"/*.out; do [ -f "$f" ] && { echo "--- $f" >&2; cat "$f" >&2; }; done; exit 1; }

make_fixture(){
  local home="$1"
  mkdir -p "$home/dashboard/bin" "$TMP/fake-bin"
  cp "$SETUP" "$home/dashboard/bin/setup-vdirsyncer.sh"
  cp "$ROOT/bin/private-calendar-discovery.sh" "$home/dashboard/bin/private-calendar-discovery.sh"
  cp "$ROOT/bin/private-calendar-selection.sh" "$home/dashboard/bin/private-calendar-selection.sh"
  chmod +x "$home/dashboard/bin/"*.sh
  cat > "$TMP/fake-bin/fakepython" <<'PY'
#!/usr/bin/env bash
set -eu
[ "${1:-}" = "-c" ] && exit 0
exec bash "$@"
PY
  cat > "$TMP/fake-bin/vdirsyncer" <<'VDIR'
#!/usr/bin/env bash
set -eu
if [ "${1:-}" = "--version" ]; then printf 'vdirsyncer, version 0.20.0\n'; exit 0; fi
cfg=""
args=("$@")
for ((i=0;i<${#args[@]};i++)); do [ "${args[$i]}" = "-c" ] && { cfg="${args[$((i+1))]:-}"; break; }; done
root="$(dirname "${cfg:?}")"
case " $* " in
  *' metasync '*)
    # draft discovery uses the credential reference as its stage collection key.
    if grep -q 'type = "google_calendar"' "$cfg"; then key=google; title='Google Family'; else key=icloud; title='Family'; fi
    mkdir -p "$root/collections/$key/primary"
    printf '%s\n' "$title" > "$root/collections/$key/primary/displayname"
    printf 'blue\n' > "$root/collections/$key/primary/color"
    ;;
  *' sync '*)
    # A successful targeted sync is sufficient for setup to promote the exact
    # mapping. The generated wrapper later merges this bounded local mirror.
    path="$(grep -E '^path = ' "$cfg" | tail -n1 | sed -E 's/^path = "(.*)"$/\1/')"
    [ -n "$path" ] && mkdir -p "$path"
    ;;
esac
exit 0
VDIR
  cat > "$TMP/fake-bin/qrencode" <<'QR'
#!/usr/bin/env bash
# Present only so the optional QR package prompt is not part of this focused
# guided-flow fixture. The OAuth shim itself owns the token result.
exit 0
QR
  chmod +x "$TMP/fake-bin/qrencode"
  cat > "$TMP/fake-bin/crontab" <<'CRON'
#!/usr/bin/env bash
set -eu
: "${FAKE_CRONTAB:?}"
if [ "${1:-}" = "-l" ]; then [ -f "$FAKE_CRONTAB" ] && cat "$FAKE_CRONTAB"; exit 0; fi
cp "$1" "$FAKE_CRONTAB"
CRON
  cat > "$home/dashboard/bin/gen-calendars.sh" <<'GEN'
#!/usr/bin/env bash
exit 0
GEN
  cat > "$home/dashboard/bin/dashboard-lowprio.sh" <<'LOW'
#!/usr/bin/env bash
exec "$@"
LOW
  cat > "$home/dashboard/bin/dashboard-control-server" <<'SERVER'
#!/usr/bin/env bash
set -eu
if [ "${1:-}" = "--google-oauth" ] && [ "${2:-}" = "authorize" ]; then
  : "${FAKE_OAUTH_LOG:?}"
  token=""; shift 2
  while [ "$#" -gt 0 ]; do
    case "$1" in
      -token-file) token="$2"; shift 2;;
      *) shift;;
    esac
  done
  [ -n "$token" ] || exit 2
  mkdir -p "$(dirname "$token")"
  printf '{"access_token":"fixture","refresh_token":"fixture","expires_in":3600,"token_type":"Bearer"}\n' > "$token"
  chmod 600 "$token"
  printf '%s\n' '--google-oauth authorize' >> "$FAKE_OAUTH_LOG"
  exit 0
fi
exit 0
SERVER
  chmod +x "$TMP/fake-bin/"* "$home/dashboard/bin/"*
}

HOME="$TMP/home"
export HOME
make_fixture "$HOME"
export PATH="$TMP/fake-bin:$PATH" \
  DASH_VDIRSYNCER_BIN="$TMP/fake-bin/vdirsyncer" \
  DASH_VDIRSYNCER_PYTHON="$TMP/fake-bin/fakepython" \
  FAKE_CRONTAB="$TMP/crontab" \
  FAKE_OAUTH_LOG="$TMP/oauth.log"
: > "$FAKE_OAUTH_LOG"

# iCloud is first-class: only Apple identity/app password are requested;
# discovery runs before color/write permission and no active pair exists on
# failed/cancelled drafts.
printf '2\n\napple@example.com\nabcd-efgh-ijkl-mnop\n1\n\n\n\n' |
  "$HOME/dashboard/bin/setup-vdirsyncer.sh" > "$TMP/icloud.out" 2>&1 || fail 'guided iCloud setup failed'
ICLOUD_PAIRS="$HOME/.dashboard-vdirsyncer/pairs"
grep -Fq 'https://caldav.icloud.com/' "$ICLOUD_PAIRS" || fail 'iCloud endpoint was not fixed internally'
grep -Fq '|primary|' "$ICLOUD_PAIRS" || fail 'selected iCloud collection was not promoted after discovery'
! grep -Fq 'CalDAV server URL' "$TMP/icloud.out" || fail 'guided iCloud path exposed a generic server URL'
! grep -qi 'collection UUID' "$TMP/icloud.out" || fail 'guided iCloud path exposed a collection UUID'
grep -Fq 'app-specific password' "$TMP/icloud.out" || fail 'iCloud path did not explain Apple app-specific passwords'
grep -Fq 'Checking this account and looking for calendars' "$TMP/icloud.out" || fail 'iCloud did not perform discovery before activation'
grep -Fq 'Step 3 of 4 — Choose calendars' "$TMP/icloud.out" || fail 'iCloud did not show human-readable calendar selection'
! grep -Fq 'Install or repair the private-calendar tools now?' "$TMP/icloud.out" || fail 'healthy pipx must not offer pipx repair'
! grep -Fq 'Install Dash-Go’s isolated calendar component now?' "$TMP/icloud.out" || fail 'healthy managed vdirsyncer must not offer an install'

# Google uses one visible Desktop-app credential instruction and phone QR
# paste-back mode without a desktop/web architecture prompt.
HOME="$TMP/google-home"
export HOME
make_fixture "$HOME"
: > "$FAKE_OAUTH_LOG"
printf '1\n\n1234-fixture.apps.googleusercontent.com\nGOCSPX-client-secret\n2\n1\n\n\n\n' |
  "$HOME/dashboard/bin/setup-vdirsyncer.sh" > "$TMP/google.out" 2>&1 || fail 'guided Google setup failed'
GOOGLE_PAIRS="$HOME/.dashboard-vdirsyncer/pairs"
grep -Fq '|google|' "$GOOGLE_PAIRS" || fail 'Google selected calendar was not promoted'
grep -Fq 'Choose application type: Desktop app.' "$TMP/google.out" || fail 'Google path did not state the exact Google client type'
! grep -Fq 'desktop/web' "$TMP/google.out" || fail 'Google path exposed desktop/web architecture jargon'
! grep -Fq 'Web application enables' "$TMP/google.out" || fail 'Google path exposed the retired web explanation'
grep -Fq 'Phone or tablet' "$TMP/google.out" || fail 'Google path did not offer the phone fallback'
grep -Fxq -- '--google-oauth authorize' "$FAKE_OAUTH_LOG" || fail 'Google setup did not invoke the Go OAuth helper'
[ -s "$HOME/.dashboard-vdirsyncer/google-tokens/google.json" ] || fail 'Google token draft was not promoted owner-only'
[ "$(stat -c '%a' "$HOME/.dashboard-vdirsyncer/google-tokens/google.json")" = 600 ] || fail 'Google token must remain 0600'

# Once Google authorization has completed, harmless display-color typos must
# stay at that prompt. They must not roll back the token or repeat sign-in.
HOME="$TMP/google-color-retry-home"
export HOME
make_fixture "$HOME"
: > "$FAKE_OAUTH_LOG"
printf '1

1234-fixture.apps.googleusercontent.com
GOCSPX-client-secret
2
1
not-a-color
still-not-a-color
teal

' |
  "$HOME/dashboard/bin/setup-vdirsyncer.sh" > "$TMP/google-color-retry.out" 2>&1 || fail 'Google color retry setup failed'
[ "$(grep -Fc -- '--google-oauth authorize' "$FAKE_OAUTH_LOG")" -eq 1 ] || fail 'color typos repeated completed Google authorization'
grep -Fq '|teal|' "$HOME/.dashboard-vdirsyncer/pairs" || fail 'valid retry color was not activated'
grep -Fq 'Use a palette color or six-digit hex value.' "$TMP/google-color-retry.out" || fail 'color retry did not show a focused correction hint'

# Explicit cancellation leaves no active connection or scheduled wrapper.
HOME="$TMP/cancel-home"
export HOME
make_fixture "$HOME"
printf '2\nq\n' | "$HOME/dashboard/bin/setup-vdirsyncer.sh" > "$TMP/cancel.out" 2>&1 || true
[ ! -s "$HOME/.dashboard-vdirsyncer/pairs" ] || fail 'cancelled iCloud draft created an active pair'
[ ! -e "$HOME/dashboard/bin/sync-vdir.sh" ] || fail 'cancelled iCloud draft created a sync wrapper'

echo 'PASS: guided Google/iCloud setup stages credentials, discovers before activation, and keeps normal prompts free of OAuth/CalDAV internals.'
