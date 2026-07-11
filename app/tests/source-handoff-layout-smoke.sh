#!/usr/bin/env bash
set -euo pipefail
repo="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
fail(){ printf 'FAIL: %s\n' "$*" >&2; exit 1; }
for path in .gitattributes .gitignore .github/workflows/verify.yml README.md CURRENT_STATE.md CHANGELOG.md CONTRIBUTING.md RELEASING.md SECURITY.md LICENSE app/VERSION app/release/release.json app/go.mod app/tests/run-all.sh installer/install.sh; do
  [[ -f "$repo/$path" ]] || fail "required source-handoff file is missing: $path"
done
for dir in app/cmd app/internal app/ui; do [[ -d "$repo/$dir" ]] || fail "$dir is required"; done
for forbidden in AI.md .git config calendars cache logs releases github-release-assets; do [[ ! -e "$repo/$forbidden" ]] || fail "source handoff must not contain $forbidden"; done
for misplaced in app/README.md app/CURRENT_STATE.md app/CHANGELOG.md; do [[ ! -e "$repo/$misplaced" ]] || fail "maintainer documentation belongs at repository root, not $misplaced"; done
for forbidden_glob in 'app/ui/js/app.bundle.js' 'app/ui/js/app.control.bundle.js' 'app/ui/dashboard.css' 'app/ui/control-layout.css' 'app/bin/dashboard-control-server' 'app/bin/dashboard-control-server-linux-*' 'SHA256SUMS' 'latest.json' 'install.sh.sha256'; do
  if compgen -G "$repo/$forbidden_glob" >/dev/null; then fail "source handoff contains generated or release-owned content: $forbidden_glob"; fi
done
printf '%s\n' 'PASS: repository-ready source-handoff layout contract holds'
