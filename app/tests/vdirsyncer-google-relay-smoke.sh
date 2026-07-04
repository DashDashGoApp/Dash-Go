#!/usr/bin/env bash
# Exercise saved Web-client authorization wiring without Google or a browser.
# The Go relay itself has focused httptest coverage; this shell smoke proves
# setup-vdirsyncer passes the exact persisted callback and owner-only spool
# directory to the Go helper, then keeps --authorize out of the name loop.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
SETUP="$ROOT/bin/setup-vdirsyncer.sh"
TMP="$(mktemp -d)"
cleanup(){ rm -rf "$TMP"; }
trap cleanup EXIT
HOME="$TMP/home"
export HOME
mkdir -p "$HOME/dashboard/bin" "$TMP/fake-bin" "$HOME/.dashboard-vdirsyncer/passwords" "$HOME/.dashboard-vdirsyncer/oauth-mode"
cp "$SETUP" "$HOME/dashboard/bin/setup-vdirsyncer.sh"
chmod +x "$HOME/dashboard/bin/setup-vdirsyncer.sh"

cat > "$TMP/fake-bin/fakepython" <<'PY'
#!/usr/bin/env bash
set -eu
[ "${1:-}" = "-c" ] && exit 0
script="$1"; shift
exec bash "$script" "$@"
PY
cat > "$TMP/fake-bin/vdirsyncer" <<'VDIR'
#!/usr/bin/env fakepython
set -eu
[ "${1:-}" = "--version" ] && { printf 'vdirsyncer, version 0.20.0\n'; exit 0; }
printf '%s\n' "$*" >> "${FAKE_VDIR_LOG:?}"
exit 0
VDIR
cat > "$TMP/fake-bin/curl" <<'CURL'
#!/usr/bin/env bash
# setup uses curl only to confirm the already-running local control server.
exit 0
CURL
cat > "$TMP/fake-bin/crontab" <<'CRON'
#!/usr/bin/env bash
set -eu
if [ "${1:-}" = "-l" ]; then exit 0; fi
exit 0
CRON
cat > "$HOME/dashboard/bin/dashboard-control-server" <<'SERVER'
#!/usr/bin/env bash
set -eu
if [ "${1:-}" = "--google-oauth" ] && [ "${2:-}" = "authorize" ]; then
  printf '%s\n' "$*" >> "${FAKE_OAUTH_LOG:?}"
  token=""
  while [ "$#" -gt 0 ]; do
    case "$1" in
      -token-file) token="$2"; shift 2;;
      *) shift;;
    esac
  done
  [ -n "$token" ]
  mkdir -p "$(dirname "$token")"
  printf '{"access_token":"fixture","refresh_token":"fixture","expires_at":4102444800,"token_type":"Bearer"}\n' > "$token"
  chmod 600 "$token"
fi
exit 0
SERVER
cat > "$HOME/dashboard/bin/gen-calendars.sh" <<'GEN'
#!/usr/bin/env bash
exit 0
GEN
chmod +x "$TMP/fake-bin/"* "$HOME/dashboard/bin/"*

VDIR_HOME="$HOME/.dashboard-vdirsyncer"
COLLECTIONS="$VDIR_HOME/collections/webcal"
mkdir -p "$COLLECTIONS"
printf '%s\n' "webcal|blue||dash_webcal|$COLLECTIONS|0|calendar-id|webcal|google|webcal|calendar-id" > "$VDIR_HOME/map"
printf '%s\n' "webcal|blue||dash_webcal|$COLLECTIONS|||calendar-id|google|fixture-client.apps.googleusercontent.com|webcal|webcal|calendar-id" > "$VDIR_HOME/pairs"
printf '%s' 'fixture-secret' > "$VDIR_HOME/passwords/webcal.google-client-secret"
printf '%s\n' web > "$VDIR_HOME/oauth-mode/webcal"
printf '%s\n' 'https://dash-go.example.test/oauth/google/callback' > "$VDIR_HOME/oauth-mode/webcal.redirect-uri"
chmod 600 "$VDIR_HOME/map" "$VDIR_HOME/pairs" "$VDIR_HOME/passwords/webcal.google-client-secret" "$VDIR_HOME/oauth-mode/webcal" "$VDIR_HOME/oauth-mode/webcal.redirect-uri"

export PATH="$TMP/fake-bin:$PATH"
export FAKE_VDIR_LOG="$TMP/vdir.log" FAKE_OAUTH_LOG="$TMP/oauth.log"
export DASH_VDIRSYNCER_BIN="$TMP/fake-bin/vdirsyncer" DASH_VDIRSYNCER_PYTHON="$TMP/fake-bin/fakepython"
export DASH_OAUTH_CONTROL_URL='http://127.0.0.1:8090'
: > "$FAKE_VDIR_LOG"; : > "$FAKE_OAUTH_LOG"
printf 'y\n' | "$HOME/dashboard/bin/setup-vdirsyncer.sh" --authorize > "$TMP/out" 2>&1 || { cat "$TMP/out" >&2; exit 1; }

fail(){ echo "FAIL: $*" >&2; cat "$TMP/out" >&2; exit 1; }
grep -q 'Calendar name' "$TMP/out" && fail '--authorize entered the add-calendar loop'
grep -Fq -- '-relay-dir /' "$FAKE_OAUTH_LOG" || fail 'web mode did not arm the relay directory'
grep -Fq -- '-redirect-uri https://dash-go.example.test/oauth/google/callback' "$FAKE_OAUTH_LOG" || fail 'web mode did not preserve the exact HTTPS callback'
grep -Fq -- '-token-file /' "$FAKE_OAUTH_LOG" || fail 'web mode omitted its owner-only token path'
grep -Eq '(^| )discover dash_webcal$' "$FAKE_VDIR_LOG" || fail 'authorized web pair was not discovered'
[ -s "$VDIR_HOME/google-tokens/webcal.json" ] || fail 'web helper did not write the token'
[ "$(stat -c '%a' "$VDIR_HOME/google-tokens/webcal.json")" = '600' ] || fail 'web token permissions are not owner-only'
for stale in pending result display.json qr.png; do [ ! -e "$VDIR_HOME/oauth-relay/$stale" ] || fail "stale relay artifact $stale remained"; done

echo 'PASS: saved Web OAuth setup preserves the exact HTTPS callback, arms the Go relay, and keeps --authorize focused.'
