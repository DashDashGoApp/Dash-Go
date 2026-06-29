#!/usr/bin/env bash
# Keep packaged split JavaScript source compatible with install.sh validate_download.
# The staged installer validates every manifest-listed *.js source before it
# replaces a live dashboard. A comment-only compatibility marker is valid for
# Node parsing but is deliberately rejected as a likely HTML/error payload.
set -euo pipefail
ROOT="${1:-.}"
INSTALLER="${ROOT%/}/../installer/install.sh"
if [[ ! -f "$INSTALLER" ]]; then
  INSTALLER="${ROOT%/}/installer/install.sh"
fi
[[ -f "$INSTALLER" ]] || { echo "missing installer/install.sh" >&2; exit 1; }
# This exact recognition rule belongs to validate_download() in the installer.
installer_rule="$(cat <<'RULE'
grep -qE 'function|const|let|var|DOMContentLoaded|Dash-Go|CONFIG\.|window\.|=>|use strict' "$path"
RULE
)"
grep -Fq "$installer_rule" "$INSTALLER" || {
  echo "installer JavaScript validation rule changed; update this smoke with validate_download" >&2
  exit 1
}
retired="${ROOT%/}/ui/js/09-control-12a-osk.js"
[[ ! -e "$retired" ]] || {
  echo "retired comment-only Control OSK compatibility marker must not be packaged" >&2
  exit 1
}
failed=0
while IFS= read -r -d '' path; do
  rel="${path#${ROOT%/}/}"
  if [[ ! -s "$path" ]]; then
    echo "empty packaged JavaScript source: $rel" >&2
    failed=1
    continue
  fi
  if head -c 80 "$path" | grep -qi '<html\|<!doctype'; then
    echo "HTML-like packaged JavaScript source: $rel" >&2
    failed=1
    continue
  fi
  if ! grep -qE 'function|const|let|var|DOMContentLoaded|Dash-Go|CONFIG\.|window\.|=>|use strict' "$path"; then
    echo "installer validate_download would reject packaged JavaScript source: $rel" >&2
    failed=1
  fi
done < <(find "${ROOT%/}/ui/js" -maxdepth 1 -type f -name '*.js' -print0 | sort -z)
[[ "$failed" -eq 0 ]]
