#!/usr/bin/env bash
# Regression: an update refuses insufficient space before any live replacement.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
INSTALLER="$ROOT/../installer/install.sh"
TMP="$(mktemp -d)"
cleanup(){ rm -rf "$TMP"; }
trap cleanup EXIT INT TERM
mkdir -p "$TMP/fake" "$TMP/payload" "$TMP/dashboard"
extract(){ awk -v name="$1" '$0 ~ "^" name "\\(\\)\\{" {on=1} on {print} on && /^}$/ {exit}' "$INSTALLER"; }
{ extract update_commit_required_free_mb; echo; extract require_update_commit_space; } > "$TMP/space.sh"
cat > "$TMP/fake/du" <<'EOF'
#!/usr/bin/env bash
printf '10 %s\n' "${@: -1}"
EOF
cat > "$TMP/fake/df" <<'EOF'
#!/usr/bin/env bash
printf 'Filesystem 1M-blocks Used Available Use%% Mounted on\n'
printf '/dev/fake 1000 100 %s 10%% %s\n' "${FREE_MB:?}" "${@: -1}"
EOF
chmod +x "$TMP/fake/du" "$TMP/fake/df"
# shellcheck disable=SC1090
source "$TMP/space.sh"
warn(){ printf '%s\n' "$*" >> "$TMP/warnings"; }
PATH="$TMP/fake:$PATH" FREE_MB=500 require_update_commit_space "$TMP/dashboard" "$TMP/payload"
set +e
PATH="$TMP/fake:$PATH" FREE_MB=299 require_update_commit_space "$TMP/dashboard" "$TMP/payload"
rc=$?
set -e
[ "$rc" -ne 0 ] || { echo "FAIL: pre-commit space guard accepted unsafe free space" >&2; exit 1; }
grep -F 'Nothing was changed.' "$TMP/warnings" >/dev/null || { echo "FAIL: space refusal lacks safe recovery wording" >&2; exit 1; }
guard_line="$(grep -n 'require_update_commit_space "$DASH" "$src"' "$INSTALLER" | cut -d: -f1)"
commit_line="$(grep -n 'write_update_phase committing "Preparing safe replacement"' "$INSTALLER" | cut -d: -f1)"
[ -n "$guard_line" ] && [ -n "$commit_line" ] && [ "$guard_line" -lt "$commit_line" ] || { echo "FAIL: storage guard is not before commit" >&2; exit 1; }
echo 'PASS: update pre-commit storage guard is conservative and runs before live replacement'
