#!/usr/bin/env bash
# Release-blocking source contract: the executable path is GitHub-only,
# cache-bounded, and has no integrity bypass or retired distribution residue.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
INSTALLER="${DASHGO_INSTALLER_UNDER_TEST:-$ROOT/../installer/install.sh}"
DOCTOR="$ROOT/bin/doctor.sh"
RELEASE="$ROOT/internal/release/github.go"
CLI="$ROOT/cmd/dashboard-control-server/release_github_cli.go"
MANIFEST="$ROOT/cmd/dashboard-control-server/release_manifest_cli.go"
CAPS="$ROOT/cmd/dashboard-control-server/updater_capabilities_cli.go"
for file in "$INSTALLER" "$DOCTOR" "$RELEASE" "$CLI" "$MANIFEST" "$CAPS"; do
  [ -f "$file" ] || { echo "FAIL: expected release-hardening source file is missing: $file" >&2; exit 1; }
done
need(){ grep -Fq -- "$2" "$1" || { echo "FAIL: missing $3" >&2; exit 1; }; }
absent(){ ! grep -Fq -- "$2" "$1" || { echo "FAIL: forbidden $3" >&2; exit 1; }; }

bash -n "$INSTALLER"
bash -n "$DOCTOR"
need "$RELEASE" 'ListCached' 'bounded ETag list helper'
need "$RELEASE" 'If-None-Match' 'conditional GitHub request header'
need "$RELEASE" 'http.StatusNotModified' '304 cache reuse handling'
need "$RELEASE" 'maxReleaseCacheAge' 'bounded cache age'
need "$RELEASE" 'RateLimitError' 'calm rate-limit classification'
need "$CLI" '--cache-file FILE' 'private cache CLI flag'
need "$CAPS" 'github-release-resolution-v3' 'cache-aware resolver capability'
need "$INSTALLER" '--cache-file "$CACHE_DIR/github-release-cache.json"' 'installer private cache path'
need "$INSTALLER" 'GitHub Release asset digest mismatch' 'GitHub digest failure path'
need "$INSTALLER" 'SHA256SUMS does not verify the release bundle' 'checksum-manifest failure path'
need "$INSTALLER" 'rollback_release_transaction' 'atomic replacement rollback'
need "$DOCTOR" 'github-release-resolution-v3' 'Doctor cache-aware resolver capability'
absent "$INSTALLER" '--unsafe' 'checksum-bypass flag'
absent "$INSTALLER" 'DASH_UNSAFE' 'checksum-bypass environment variable'
absent "$MANIFEST" 'latest.json' 'retired catalog metadata dependency'

# Runtime/installer source is intentionally separated from legacy-migration
# fixtures. These terms may appear in isolated migration tests, but never in an
# executable Go/sh file after the GitHub-only transition.
for forbidden in BASE_URL DASH_TOKEN DASH_USERPASS 'install.sh.sha256' 'latest.json' 'basic-auth' 'nginx'; do
  if grep -RInF --include='*.go' --exclude='*_test.go' -- "$forbidden" "$ROOT/cmd" "$ROOT/internal" >/dev/null; then
    echo "FAIL: retired distribution token remains in executable Go source: $forbidden" >&2
    exit 1
  fi
  if grep -Fq -- "$forbidden" "$INSTALLER" || grep -Fq -- "$forbidden" "$DOCTOR"; then
    echo "FAIL: retired distribution token remains in installer/Doctor source: $forbidden" >&2
    exit 1
  fi
done

echo 'PASS: GitHub Release path keeps discovery GitHub-only, ETag-bounded, checksum-enforced, rollback-safe, and free of retired distribution controls'
