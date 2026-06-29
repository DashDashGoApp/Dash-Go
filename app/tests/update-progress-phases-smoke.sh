#!/usr/bin/env bash
# Source-level contract for GitHub Release update progress wording. The dashboard
# can display an update while the release that owns it is being replaced, so
# exact progress stays in labels while durable state names remain compatible.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
INSTALL="$ROOT/../installer/install.sh"
UPDATES="$ROOT/ui/js/control-updates.js"
[ -f "$INSTALL" ] || { echo 'FAIL: installer missing' >&2; exit 1; }
[ -f "$UPDATES" ] || { echo 'FAIL: update card source missing' >&2; exit 1; }
require_inst(){ grep -Fq -- "$1" "$INSTALL" || { echo "FAIL: missing update phase: $1" >&2; exit 1; }; }
require_ui(){ grep -Fq -- "$1" "$UPDATES" || { echo "FAIL: missing update card phase presentation: $1" >&2; exit 1; }; }
require_inst 'write_update_phase(){'
require_inst '[ "${UPDATE_MODE:-0}" = "1" ] || return 0'
require_inst 'write_update_status "$state" "$label" "$detail" 0'
require_inst 'write_update_job "$state" "$label" "$detail" 0'
require_inst 'github_digest_sha256(){'
require_inst 'github_checksum_for_name(){'
require_inst 'resolve_github_release(){'
require_inst 'install_local_release_bundle(){'
require_inst 'write_update_phase validating-payload "Resolving GitHub Release"'
require_inst 'write_update_phase validating-payload "Downloading GitHub Release assets"'
require_inst 'write_update_phase validating-payload "Verifying GitHub Release assets"'
require_inst 'write_update_phase validating-payload "Extracting verified release"'
require_inst 'write_update_phase validating-payload "Verifying staged release files"'
require_inst 'write_update_phase committing "Preparing safe replacement"'
require_inst 'write_update_phase committing "Installing verified release"'
require_inst '"Refreshing dashboard data"'
if grep -Fq 'Fetching release catalog' "$INSTALL" || grep -Fq 'Fetching release file manifest' "$INSTALL"; then
  echo 'FAIL: legacy catalog progress wording remains in the installer' >&2
  exit 1
fi
payload_block="$(sed -n '/^download_release_payload(){/,/^install_local_release_bundle(){/p' "$INSTALL")"
phase_line(){ printf '%s\n' "$payload_block" | grep -n -m1 -F -- "$1" | cut -d: -f1; }
resolve_at="$(phase_line 'Resolving GitHub Release')"
download_at="$(phase_line 'Downloading GitHub Release assets')"
verify_at="$(phase_line 'Verifying GitHub Release assets')"
[ -n "$resolve_at" ] && [ -n "$download_at" ] && [ -n "$verify_at" ] || { echo 'FAIL: GitHub Release phase ordering tokens are incomplete' >&2; exit 1; }
[ "$resolve_at" -lt "$download_at" ] && [ "$download_at" -lt "$verify_at" ] || {
  echo 'FAIL: resolve -> download -> digest verification phases are out of order' >&2
  exit 1
}
install_block="$(sed -n '/^install_release_payload(){/,/^retain_update_rollback_stage(){/p' "$INSTALL")"
phase_line_install(){ printf '%s\n' "$install_block" | grep -n -m1 -F -- "$1" | cut -d: -f1; }
extract_at="$(phase_line_install 'Extracting verified release')"
files_at="$(phase_line_install 'Verifying staged release files')"
prepare_at="$(phase_line_install 'Preparing safe replacement')"
commit_at="$(phase_line_install 'Installing verified release')"
[ -n "$extract_at" ] && [ -n "$files_at" ] && [ -n "$prepare_at" ] && [ -n "$commit_at" ] || { echo 'FAIL: staged payload phase tokens are incomplete' >&2; exit 1; }
[ "$extract_at" -lt "$files_at" ] && [ "$files_at" -lt "$prepare_at" ] && [ "$prepare_at" -lt "$commit_at" ] || {
  echo 'FAIL: extraction -> file verification -> rollback preparation -> install phases are out of order' >&2
  exit 1
}
require_ui 'function updateJobPresentation(progress)'
require_ui 'ctrlUpdateSetStat(ui.rows.job,"Job",job.label,job.state);'
require_ui '["job","Job",updateJobPresentation(progress).label,updateJobPresentation(progress).state]'
require_ui 'selected GitHub Release'
if grep -Fq 'ctrlUpdateSetStat(ui.rows.job,"Job",progress.state' "$UPDATES"; then
  echo 'FAIL: Update card still exposes the coarse state key as the Job value' >&2
  exit 1
fi
echo 'PASS: update progress labels distinguish GitHub resolution, asset download, digest verification, staging, commit, and runtime work while retaining backward-compatible active states'
