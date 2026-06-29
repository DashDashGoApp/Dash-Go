#!/usr/bin/env bash
# Ensures a source handoff is structurally safe to seed into a public Git repository.
# Exact private-history markers are intentionally enforced by a local-builder-only
# audit. They must not be embedded in public source or public source tests.
set -euo pipefail

root="$(cd "$(dirname "$0")/../.." && pwd)"

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

for forbidden in AI.md .git config calendars cache logs releases github-release-assets; do
  if [[ -e "$root/$forbidden" ]]; then
    fail "source tree must not contain $forbidden"
  fi
done

for forbidden_glob in '*.tar.gz' '*.zip' 'SHA256SUMS' '*.sha256' '*.sbom.json' '*.spdx.json' 'dashboard-control-server-linux-*'; do
  if find "$root" -xdev -type f -name "$forbidden_glob" -print -quit | grep -q .; then
    fail "source tree contains forbidden generated artifact matching $forbidden_glob"
  fi
done

[[ -f "$root/RELEASING.md" ]] || fail 'RELEASING.md is required for the release workflow'
[[ -f "$root/.gitignore" ]] || fail '.gitignore is required for public-source import'
[[ -f "$root/.gitattributes" ]] || fail '.gitattributes is required for public-source import'

grep -Fq 'Report a vulnerability' "$root/SECURITY.md" || \
  fail 'SECURITY.md must describe the GitHub private-reporting boundary'
grep -Fq 'Public release transition' "$root/README.md" || \
  fail 'README.md must document the public release transition'
grep -Fq 'DashDashGoApp/Dash-Go' "$root/RELEASING.md" || \
  fail 'RELEASING.md must name the canonical repository'

echo 'PASS: public-source repository structural hygiene contract holds'
