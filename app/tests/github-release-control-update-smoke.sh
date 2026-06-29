#!/usr/bin/env bash
# Release-blocking regression guard for Dashboard Control's GitHub Release
# self-update preflight. It must use the modern release-bundle contract, never
# retired nginx catalog metadata or a credential gate.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
UPDATE_GO="$ROOT/cmd/dashboard-control-server/dashboard_update.go"
STATUS_GO="$ROOT/cmd/dashboard-control-server/update_status.go"
HEALTH_JS="$ROOT/ui/js/control-status-health.js"
for file in "$UPDATE_GO" "$STATUS_GO" "$HEALTH_JS"; do
  [ -f "$file" ] || { echo "FAIL: missing updater source: $file" >&2; exit 1; }
done
need(){ grep -Fq -- "$2" "$1" || { echo "FAIL: missing $3" >&2; exit 1; }; }
absent(){ ! grep -Fq -- "$2" "$1" || { echo "FAIL: retired $3" >&2; exit 1; }; }
need "$UPDATE_GO" 'func githubReleaseCatalogProblems' 'GitHub Release preflight helper'
for token in 'releaseAsset' 'releaseDigest' 'checksumsAsset' 'checksumsDigest' 'releaseUrl' 'immutable'; do
  need "$UPDATE_GO" "$token" "GitHub Release metadata field $token"
done
need "$UPDATE_GO" 'updateTrackProfilePresent' 'informational update-track profile state'
need "$STATUS_GO" 'updateTrackProfilePresent' 'public status track-profile field'
need "$HEALTH_JS" 'local update service' 'token-free updater setup message'
for token in 'credentialsPresent' 'updateCredentialsPresent' 'saved update credentials' '"tarball"' '"manifest"' 'shaPresent' 'installerShaPresent'; do
  absent "$UPDATE_GO" "$token"
  absent "$STATUS_GO" "$token"
  absent "$HEALTH_JS" "$token"
done
echo 'PASS: Dashboard Control update preflight follows immutable GitHub Release bundle metadata without a credential gate'
