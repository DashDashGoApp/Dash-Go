#!/usr/bin/env bash
# Exercises the exact installer rollback helper against a small staged tree.
# A late post-commit failure must restore pre-existing files, remove newly
# introduced files, and return retired managed source files from its backup.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
INSTALLER="${1:-$ROOT/../installer/install.sh}"
[ -f "$INSTALLER" ] || { echo "installer not found: $INSTALLER" >&2; exit 1; }
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
DASH="$TMP/dashboard"
BIN_DIR="$DASH/bin"
STAGE="$TMP/stage"
mkdir -p "$DASH/ui/js" "$BIN_DIR" "$STAGE/backup/ui/js" "$STAGE/backup/stale-managed/ui/js" "$STAGE/personal"
printf 'new payload\n' > "$DASH/managed.txt"
printf 'introduced\n' > "$DASH/introduced.txt"
printf 'old payload\n' > "$STAGE/backup/managed.txt"
printf 'retired source\n' > "$STAGE/backup/stale-managed/ui/js/retired.js"
printf 'managed.txt\nintroduced.txt\n' > "$STAGE/payload-files.txt"
printf 'managed.txt\n' > "$STAGE/preexisting-files.txt"
cat > "$TMP/verifier" <<'VERIFY'
#!/usr/bin/env bash
[ "${1:-}" = "--verify-generated-assets" ]
VERIFY
chmod +x "$TMP/verifier"
# Extract the actual rollback helper, then provide only its environment-facing
# dependencies. The assertions below therefore cover the installer logic, not
# a duplicate test implementation.
awk '
  /^rollback_update_payload\(\)/ { copy=1 }
  copy && /^rollback_release_transaction\(\)/ { exit }
  copy { print }
' "$INSTALLER" > "$TMP/rollback-function.sh"
[ -s "$TMP/rollback-function.sh" ] || { echo "could not extract rollback helper" >&2; exit 1; }
# shellcheck disable=SC1090
source "$TMP/rollback-function.sh"
atomic_replace_file(){
  local src="$1" dest="$2"
  mkdir -p "$(dirname "$dest")"
  cp -p "$src" "$dest"
}
restore_personal_settings(){ return 0; }
RESTORED_CANONICAL_INSTALLER=0
restore_canonical_installer(){ RESTORED_CANONICAL_INSTALLER=1; return 0; }
ensure_go_selector_wrapper_installed(){ return 0; }
release_server_for_host(){ printf '%s\n' "$TMP/verifier"; }
rollback_update_payload "$STAGE"
[ "$RESTORED_CANONICAL_INSTALLER" = 1 ] || { echo 'FAIL: rollback did not restore the canonical installer' >&2; exit 1; }
[ "$(cat "$DASH/managed.txt")" = "old payload" ] || { echo "FAIL: old managed file was not restored" >&2; exit 1; }
[ ! -e "$DASH/introduced.txt" ] || { echo "FAIL: newly introduced file survived rollback" >&2; exit 1; }
[ "$(cat "$DASH/ui/js/retired.js")" = "retired source" ] || { echo "FAIL: retired managed source was not restored" >&2; exit 1; }
echo 'PASS: late update rollback restores old files, deletes introduced files, restores retired managed source, and verifies the recovered payload'
