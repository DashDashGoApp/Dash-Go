#!/usr/bin/env bash
# Release-blocking source contract for the GitHub Release device path.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
INSTALLER="${DASHGO_INSTALLER_UNDER_TEST:-$ROOT/../installer/install.sh}"
DOCTOR="$ROOT/bin/doctor.sh"
CLI="$ROOT/cmd/dashboard-control-server/release_github_cli.go"
CAPS="$ROOT/cmd/dashboard-control-server/updater_capabilities_cli.go"
[ -f "$INSTALLER" ] && [ -f "$DOCTOR" ] && [ -f "$CLI" ] && [ -f "$CAPS" ] || { echo 'FAIL: GitHub migration source files are missing' >&2; exit 1; }
need(){ grep -Fq -- "$2" "$1" || { echo "FAIL: missing $3" >&2; exit 1; }; }
absent(){ ! grep -Fq -- "$2" "$1" || { echo "FAIL: forbidden $3" >&2; exit 1; }; }

bash -n "$INSTALLER"
bash -n "$DOCTOR"
need "$INSTALLER" 'REPAIR_UPDATE_REQUESTED=0' 'explicit repair-update flag'
need "$INSTALLER" 'REPAIR_TARGET="$(installed_dashboard_version)"' 'exact installed-version repair target'
need "$INSTALLER" 'repair --update to request the newest eligible release' 'explicit newest-release repair guidance'
need "$INSTALLER" 'github_digest_sha256(){' 'GitHub asset digest parser'
need "$INSTALLER" 'github_checksum_for_name(){' 'SHA256SUMS exact-name parser'
need "$INSTALLER" 'resolve_github_release(){' 'canonical resolver bridge'
need "$INSTALLER" 'args+=(--version "$target")' 'exact-version resolver request'
need "$INSTALLER" '--cache-file "$CACHE_DIR/github-release-cache.json"' 'private bounded resolver metadata cache'
need "$INSTALLER" 'validate_github_release_resolution(){' 'resolved metadata contract'
need "$INSTALLER" 'DashDashGoApp/Dash-Go' 'compiled canonical repository check'
need "$INSTALLER" 'install_local_release_bundle(){' 'self-contained fresh-install bundle path'
need "$INSTALLER" 'download_release_payload(){' 'GitHub asset download path'
need "$INSTALLER" 'verify_sha256 "$release_digest" "$release_file"' 'GitHub reported release digest verification'
need "$INSTALLER" 'github_checksum_for_name "$sums_file" "$release_name"' 'checksum manifest release verification'
need "$INSTALLER" 'install_release_payload "$release_file" "$version"' 'verified staged replacement entrypoint'
need "$DOCTOR" 'github-release-resolution-v3' 'Doctor canonical resolver capability requirement'
need "$DOCTOR" 'check_online_release(){' 'Doctor GitHub online check'
need "$DOCTOR" 'GitHub Release asset digest or SHA256SUMS verification failed' 'Doctor digest failure diagnosis'
need "$CLI" '--version X.Y.Z[-beta.N]' 'exact-version resolver CLI help'
need "$CLI" '--cache-file FILE' 'resolver cache CLI help'
need "$CAPS" 'github-release-resolution-v3' 'resolver capability v2'
for forbidden in 'BASE_URL' 'DASH_TOKEN' 'DASH_USERPASS' 'maybe_self_update_installer' 'Fetching release catalog' 'Fetching release file manifest'; do
  absent "$INSTALLER" "$forbidden" "legacy updater token $forbidden"
done

echo 'PASS: installer, repair, and Doctor use the canonical GitHub Release bundle/digest/checksum transaction without arbitrary update hosts or credentials'
