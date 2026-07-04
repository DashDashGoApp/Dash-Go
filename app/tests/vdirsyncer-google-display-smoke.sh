#!/usr/bin/env bash
# The kiosk QR is a display-only Desktop-client paste-back aid. It must not
# leave a hidden HTTPS callback mode that ordinary setup can never select.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
SETUP="$ROOT/bin/setup-vdirsyncer.sh"
OPTIONS="$ROOT/cmd/dashboard-control-server/google_oauth_options.go"
DISPLAY="$ROOT/cmd/dashboard-control-server/oauth_display.go"
SERVER="$ROOT/cmd/dashboard-control-server/http_server.go"
fail(){ echo "FAIL: $*" >&2; exit 1; }
bash -n "$SETUP"
grep -Fq 'googleOAuthDisplayOnly' "$DISPLAY" || fail 'phone QR must remain display-only'
grep -Fq 'oauthDisplayTTL' "$DISPLAY" || fail 'display lifetime must remain bounded'
grep -Fq 'OAUTH_DISPLAY_DIR' "$SETUP" || fail 'setup must use the named owner-only display directory'
! grep -Fq 'relay-dir' "$OPTIONS" || fail 'unreachable advanced callback flag remains'
! grep -Fq 'redirect-uri' "$OPTIONS" || fail 'unreachable advanced callback URI remains'
! grep -Fq '/oauth/google/callback' "$SERVER" || fail 'public OAuth callback route remains'
! grep -Fq 'OAuth client type [desktop/web' "$SETUP" || fail 'ordinary setup must not expose desktop/web choice'
! grep -Fq 'Register this exact HTTPS redirect URI' "$SETUP" || fail 'ordinary setup must not request proxy callback configuration'
echo 'PASS: phone QR is a bounded display-only paste-back aid; no hidden HTTP callback mode remains.'
