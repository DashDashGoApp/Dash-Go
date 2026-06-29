#!/usr/bin/env bash
# Release-blocking contract for private/offline device rehearsal bundles.
# It proves an extracted bundle reports its own exact release track before a
# fresh device persists update state, and that --bundle-info is side-effect-free.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
INSTALLER="${1:-${DASHGO_INSTALLER_UNDER_TEST:-$ROOT/../installer/install.sh}}"
[ -f "$INSTALLER" ] || { echo "FAIL: installer not found: $INSTALLER" >&2; exit 1; }
bash -n "$INSTALLER"
need(){ grep -Fq -- "$2" "$1" || { echo "FAIL: missing $3" >&2; exit 1; }; }
need "$INSTALLER" 'bundle_release_version(){' 'local release-bundle version reader'
need "$INSTALLER" 'bundle_release_track(){' 'local release-bundle track reader'
need "$INSTALLER" 'the local Dash-Go release bundle is $bundled_track' 'conflicting track correction'
need "$INSTALLER" 'show_local_release_bundle_info(){' 'side-effect-free bundle info command'
need "$INSTALLER" '--bundle-info) BUNDLE_INFO_MODE=1;;' 'bundle-info parser entry'
need "$INSTALLER" 'if [ "$BUNDLE_INFO_MODE" = "1" ]; then' 'bundle-info early exit'
need "$INSTALLER" 'No GitHub token belongs on the device.' 'private-device rehearsal guidance'

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT INT TERM
VERSION="$(tr -d '[:space:]' < "$ROOT/VERSION")"
case "$VERSION" in *-beta.*) EXPECTED_TRACK=beta ;; *) EXPECTED_TRACK=stable ;; esac
BUNDLE="$TMP/Dash-Go_$VERSION"
mkdir -p "$BUNDLE/app" "$TMP/home/dashboard"
printf '%s\n' "$VERSION" > "$BUNDLE/app/VERSION"
printf '<!doctype html>\n' > "$BUNDLE/app/index.html"
printf '{"schema":1}\n' > "$BUNDLE/app/manifest.json"
cp "$INSTALLER" "$BUNDLE/install.sh"
chmod +x "$BUNDLE/install.sh"

# Metadata inspection must not need sudo, mutate update state, or use a network.
info="$(HOME="$TMP/home" DASH_TRACK=stable bash "$BUNDLE/install.sh" --bundle-info)"
printf '%s\n' "$info" | grep -Fqx 'Dash-Go local release bundle'
printf '%s\n' "$info" | grep -Fqx "Version: $VERSION"
printf '%s\n' "$info" | grep -Fqx "Track: $EXPECTED_TRACK"
[ ! -e "$TMP/home/.dashboard-update.env" ] || { echo 'FAIL: --bundle-info wrote update state' >&2; exit 1; }
[ ! -e "$TMP/home/.dashboard-update-profile.json" ] || { echo 'FAIL: --bundle-info wrote update profile' >&2; exit 1; }

# Extract just the track helpers so the contract can be exercised without
# starting the complete interactive installer.
awk '
  /^normalize_release_track\(\)/ { capture=1 }
  /^show_local_release_bundle_info\(\)/ { exit }
  capture { print }
' "$INSTALLER" > "$TMP/track-helpers.sh"
[ -s "$TMP/track-helpers.sh" ] || { echo 'FAIL: could not extract bundle track helpers' >&2; exit 1; }
# shellcheck disable=SC1090
source "$TMP/track-helpers.sh"
INSTALLER_SOURCE_DIR="$BUNDLE"
DASH="$TMP/home/dashboard"
DEFAULT_RELEASE_TRACK=stable
warn(){ printf '%s\n' "$*" >&2; }
DASH_TRACK=stable
resolve_release_track
[ "$RELEASE_TRACK" = "$EXPECTED_TRACK" ] || { echo "FAIL: bundled track resolved $RELEASE_TRACK instead of $EXPECTED_TRACK" >&2; exit 1; }

# Exercise the opposite semantic track without depending on the current beta number.
if [ "$EXPECTED_TRACK" = beta ]; then
  printf '1.5.0\n' > "$BUNDLE/app/VERSION"
  DASH_TRACK=beta
  resolve_release_track
  [ "$RELEASE_TRACK" = stable ] || { echo "FAIL: stable bundle resolved $RELEASE_TRACK instead of stable" >&2; exit 1; }
else
  printf '1.5.0-beta.0\n' > "$BUNDLE/app/VERSION"
  DASH_TRACK=stable
  resolve_release_track
  [ "$RELEASE_TRACK" = beta ] || { echo "FAIL: beta bundle resolved $RELEASE_TRACK instead of beta" >&2; exit 1; }
fi

echo 'PASS: local Dash-Go release bundles expose metadata without side effects and force their exact beta/stable update track during private/offline device rehearsal'
