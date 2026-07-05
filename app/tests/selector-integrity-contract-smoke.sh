#!/usr/bin/env bash
# Release-blocking selector ownership contract. The package-generated generic
# selector is manifest-managed; installer recovery may create it only when a
# legacy installation has no selector at all, and every payload transaction
# verifies the live manifest before it reports success.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
INSTALLER="${1:-${DASHGO_INSTALLER_UNDER_TEST:-$ROOT/../installer/install.sh}}"
[ -f "$INSTALLER" ] || { echo "FAIL: installer not found: $INSTALLER" >&2; exit 1; }
bash -n "$INSTALLER"
VERSION="$(tr -d '[:space:]' < "$ROOT/VERSION")"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT INT TERM

need(){ grep -Fq -- "$2" "$1" || { echo "FAIL: missing $3" >&2; exit 1; }; }
need "$INSTALLER" 'ensure_missing_go_selector_wrapper(){' 'missing-selector-only recovery helper'
need "$INSTALLER" '[ -e "$selector" ] && return 0' 'existing selector preservation guard'
need "$INSTALLER" 'for arch in amd64 386 arm64 armv7 armv6; do' 'all-architecture legacy recovery guard'
need "$INSTALLER" 'verify_installed_release_manifest(){' 'post-install release-manifest verifier'
need "$INSTALLER" 'Post-install package integrity check failed' 'transactional post-install integrity rollback'
if grep -Fq 'ensure_go_selector_wrapper_installed' "$INSTALLER"; then
  echo 'FAIL: installer still contains the unconditional selector rewrite helper' >&2
  exit 1
fi

# Extract and run the exact recovery helper. A present selector must be byte
# preserved; a missing selector is reconstructed for every shipped target.
awk '
  /^ensure_missing_go_selector_wrapper\(\)/ { capture=1 }
  capture && /^# Verify the live managed tree/ { exit }
  capture { print }
' "$INSTALLER" > "$TMP/selector-helper.sh"
[ -s "$TMP/selector-helper.sh" ] || { echo 'FAIL: could not extract selector helper' >&2; exit 1; }
# shellcheck disable=SC1090
source "$TMP/selector-helper.sh"

# Preservation remains unconditional regardless of architecture binaries.
DASH="$TMP/preserved"
mkdir -p "$DASH/bin"
printf '#!/usr/bin/env sh\nprintf preserved\n' > "$DASH/bin/dashboard-control-server"
chmod 755 "$DASH/bin/dashboard-control-server"
cp "$DASH/bin/dashboard-control-server" "$TMP/preserved-selector"
ensure_missing_go_selector_wrapper
cmp -s "$TMP/preserved-selector" "$DASH/bin/dashboard-control-server" || { echo 'FAIL: recovery helper rewrote a present selector' >&2; exit 1; }

# Simulate every architecture the canonical selector dispatches. The legacy
# tree deliberately contains only that architecture's binary, proving the
# presence guard is not accidentally limited to amd64/armv7.
while IFS='|' read -r uname_arch target_arch; do
  DASH="$TMP/$target_arch"
  mkdir -p "$DASH/bin" "$DASH/fake-bin"
  cat > "$DASH/fake-bin/uname" <<EOF
#!/usr/bin/env sh
printf '%s\\n' '$uname_arch'
EOF
  chmod 755 "$DASH/fake-bin/uname"
  target="$DASH/bin/dashboard-control-server-linux-$target_arch"
  cat > "$target" <<'SERVER'
#!/usr/bin/env sh
printf '%s\n' "${1:-}" > "${SELECTOR_TEST_LOG:?}"
SERVER
  chmod 755 "$target"
  PATH="$DASH/fake-bin:$PATH" ensure_missing_go_selector_wrapper
  [ -x "$DASH/bin/dashboard-control-server" ] || { echo "FAIL: recovery helper did not create selector for $target_arch" >&2; exit 1; }
  SELECTOR_TEST_LOG="$DASH/selector-log" PATH="$DASH/fake-bin:$PATH" "$DASH/bin/dashboard-control-server" --selector-smoke
  [ "$(cat "$DASH/selector-log")" = '--selector-smoke' ] || { echo "FAIL: recovered selector did not dispatch for $target_arch" >&2; exit 1; }
done <<'ARCHES'
x86_64|amd64
i686|386
aarch64|arm64
armv7l|armv7
armv6l|armv6
ARCHES

# Exercise the exact post-install verifier helper with a fake host binary. It
# must pass the installed manifest, root, version, and host target explicitly.
awk '
  /^verify_installed_release_manifest\(\)/ { capture=1 }
  capture && /^service_unit_section_has_setting\(/ { exit }
  capture { print }
' "$INSTALLER" > "$TMP/manifest-helper.sh"
[ -s "$TMP/manifest-helper.sh" ] || { echo 'FAIL: could not extract manifest verifier helper' >&2; exit 1; }
# shellcheck disable=SC1090
source "$TMP/manifest-helper.sh"
DASH="$TMP/manifest-dashboard"
mkdir -p "$DASH/bin"
printf '{"version":"%s","files":[]}' "$VERSION" > "$DASH/manifest.json"
cat > "$TMP/manifest-verifier" <<'VERIFY'
#!/usr/bin/env bash
printf '%s\n' "$*" > "${MANIFEST_TEST_LOG:?}"
[ "${1:-}" = '--verify-release-manifest' ]
VERIFY
chmod +x "$TMP/manifest-verifier"
warn(){ printf '%s\n' "$*" >> "$TMP/warnings"; }
release_server_for_host(){ printf '%s\n' "$TMP/manifest-verifier"; }
MANIFEST_TEST_LOG="$TMP/manifest-log" verify_installed_release_manifest "$VERSION"
for token in '--verify-release-manifest' '--manifest' "$DASH/manifest.json" '--root' "$DASH" '--version' "$VERSION" '--target-bin' 'bin/dashboard-control-server-linux-'; do
  grep -Fq -- "$token" "$TMP/manifest-log" || { echo "FAIL: installed manifest verifier omitted $token" >&2; exit 1; }
done

# The post-commit check must precede generated-asset verification. A fresh
# payload must never be reported successful before the manifest still matches.
post_line="$(grep -n -m1 'if ! verify_installed_release_manifest "\$version"; then' "$INSTALLER" | cut -d: -f1)"
generated_line="$(grep -n -m1 'installed_verifier="\$(release_server_for_host "\$DASH"' "$INSTALLER" | cut -d: -f1)"
[ -n "$post_line" ] && [ -n "$generated_line" ] && [ "$post_line" -lt "$generated_line" ] || {
  echo 'FAIL: post-install manifest verification is missing or ordered after generated-asset verification' >&2
  exit 1
}
rollback_block="$(sed -n '/^rollback_update_payload(){/,/^rollback_release_transaction(){/p' "$INSTALLER")"
printf '%s\n' "$rollback_block" | grep -Fq 'verify_installed_release_manifest "$restored_version"' || {
  echo 'FAIL: rollback does not verify the restored release manifest' >&2
  exit 1
}

printf '%s\n' 'PASS: selector ownership is manifest-safe; legacy recovery covers all shipped architectures; update/rollback verify the live payload'
