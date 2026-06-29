#!/usr/bin/env bash
# Confirms beta.34 migrates all legacy updater connection state to a small
# owner-only GitHub Release track profile without sourcing shell content.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
INSTALLER="${1:-$ROOT/../installer/install.sh}"
PROFILE_GO="$ROOT/cmd/dashboard-control-server/release_update_profile.go"
[ -f "$INSTALLER" ] || { echo "FAIL: installer not found" >&2; exit 1; }
[ -f "$PROFILE_GO" ] || { echo "FAIL: private update profile reader is missing" >&2; exit 1; }
require(){ grep -Fq -- "$1" "$INSTALLER" || { echo "FAIL: missing installer profile contract: $1" >&2; exit 1; }; }
require_go(){ grep -Fq -- "$1" "$PROFILE_GO" || { echo "FAIL: missing Go update-profile contract: $1" >&2; exit 1; }; }
for token in 'UPDATE_PROFILE="$HOME/.dashboard-update-profile.json"' 'legacy_saved_update_track(){' 'write_update_profile_json(){' 'migrate_update_track_state(){' '"schema": 2' 'DASH_TRACK=%s' 'chmod 600 "$UPDATE_PROFILE"'; do
  require "$token"
done
for token in 'const updateProfileSchema = 2' 'const legacyUpdateProfileSchema = 1' 'readPrivateUpdateProfile' 'private-json-v1-pending-migration' 'func (a *app) resolveUpdateTrack()'; do
  require_go "$token"
done
if grep -Eq '\b(BASE_URL|DASH_TOKEN|DASH_USERPASS)\b' "$INSTALLER"; then
  echo 'FAIL: installer still contains a legacy arbitrary-host or credential setting' >&2; exit 1
fi
if grep -Fq 'resolveUpdateSource' "$PROFILE_GO" || grep -Fq 'readLegacyUpdateEnv' "$PROFILE_GO"; then
  echo 'FAIL: Go server must not parse or use historical shell updater connection state' >&2; exit 1
fi
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT INT TERM
awk '
  /^write_update_profile_json\(\)/ { copy=1 }
  /^if ! migrate_update_track_state; then/ { exit }
  copy { print }
' "$INSTALLER" > "$TMP/profile-functions.sh"
[ -s "$TMP/profile-functions.sh" ] || { echo 'FAIL: profile writer functions could not be extracted' >&2; exit 1; }
# shellcheck disable=SC1090
source "$TMP/profile-functions.sh"
UPDATE_PROFILE="$TMP/.dashboard-update-profile.json"
UPDATE_ENV="$TMP/.dashboard-update.env"
REMOVE_MODE=0
RELEASE_TRACK='beta'
migrate_update_track_state
[ -f "$UPDATE_PROFILE" ] || { echo 'FAIL: JSON profile was not written' >&2; exit 1; }
[ -f "$UPDATE_ENV" ] || { echo 'FAIL: shell track file was not written' >&2; exit 1; }
[ "$(stat -c '%a' "$UPDATE_PROFILE")" = 600 ] || { echo 'FAIL: JSON profile mode is not 0600' >&2; exit 1; }
[ "$(stat -c '%a' "$UPDATE_ENV")" = 600 ] || { echo 'FAIL: shell track file mode is not 0600' >&2; exit 1; }
node - "$UPDATE_PROFILE" "$UPDATE_ENV" <<'NODE'
const fs = require('fs');
const [profilePath, envPath] = process.argv.slice(2);
const profile = JSON.parse(fs.readFileSync(profilePath, 'utf8'));
const env = fs.readFileSync(envPath, 'utf8');
if (profile.schema !== 2 || profile.track !== 'beta' || Object.keys(profile).sort().join(',') !== 'schema,track') process.exit(1);
if (!/^# Dash-Go update track[\s\S]*^DASH_TRACK=beta$/m.test(env)) process.exit(1);
if (/(BASE_URL|DASH_TOKEN|DASH_USERPASS)/.test(env)) process.exit(1);
NODE
printf 'PASS: GitHub Release update state is owner-only, track-only, and does not retain legacy host or credential fields\n'
