#!/usr/bin/env bash
# Release-blocking regression guard for Dashboard Control's GitHub Release
# self-update preflight. It must use the modern release-bundle contract, never
# retired nginx catalog metadata or a credential gate.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
UPDATE_GO="$ROOT/cmd/dashboard-control-server/dashboard_update.go"
UPDATE_PREFLIGHT_GO="$ROOT/cmd/dashboard-control-server/dashboard_update_preflight.go"
STATUS_GO="$ROOT/cmd/dashboard-control-server/update_status.go"
HEALTH_JS="$ROOT/ui/js/control-status-health.js"
for file in "$UPDATE_GO" "$UPDATE_PREFLIGHT_GO" "$STATUS_GO" "$HEALTH_JS"; do
  [ -f "$file" ] || { echo "FAIL: missing updater source: $file" >&2; exit 1; }
done
UPDATE_FILES=("$UPDATE_GO" "$UPDATE_PREFLIGHT_GO")
need(){ grep -Fq -- "$2" "$1" || { echo "FAIL: missing $3" >&2; exit 1; }; }
need_any(){ local token="$1" label="$2" file; for file in "${UPDATE_FILES[@]}"; do grep -Fq -- "$token" "$file" && return 0; done; echo "FAIL: missing $label" >&2; exit 1; }
absent(){ ! grep -Fq -- "$2" "$1" || { echo "FAIL: retired $3" >&2; exit 1; }; }
absent_all(){ local token="$1" label="$2" file; for file in "${UPDATE_FILES[@]}"; do if grep -Fq -- "$token" "$file"; then echo "FAIL: retired $label" >&2; exit 1; fi; done; }
need_any 'func githubReleaseCatalogProblems' 'GitHub Release preflight helper'
for token in 'releaseAsset' 'releaseDigest' 'checksumsAsset' 'checksumsDigest' 'releaseUrl' 'immutable'; do
  need_any "$token" "GitHub Release metadata field $token"
done
need_any 'updateTrackProfilePresent' 'informational update-track profile state'
need "$STATUS_GO" 'updateTrackProfilePresent' 'public status track-profile field'
need "$HEALTH_JS" 'local update service' 'token-free updater setup message'
for token in 'credentialsPresent' 'updateCredentialsPresent' 'saved update credentials' '"tarball"' '"manifest"' 'shaPresent' 'installerShaPresent'; do
  absent_all "$token" "$token"
  absent "$STATUS_GO" "$token"
  absent "$HEALTH_JS" "$token"
done
echo 'PASS: Dashboard Control update preflight follows immutable GitHub Release bundle metadata without a credential gate'
