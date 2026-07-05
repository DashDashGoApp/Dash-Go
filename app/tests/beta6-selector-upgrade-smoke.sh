#!/usr/bin/env bash
# Simulate a beta.6-shaped local tree: its old updater may have left a generic
# selector that does not match the next manifest. The current release must first
# refresh the updater through the local bridge, then replace the selector from the verified
# release payload without rewriting it after the post-commit manifest check.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
INSTALLER="${DASHGO_INSTALLER_UNDER_TEST:-$ROOT/../installer/install.sh}"
[ -f "$INSTALLER" ] || { echo "FAIL: installer not found: $INSTALLER" >&2; exit 1; }
bash -n "$INSTALLER"
VERSION="$(tr -d '[:space:]' < "$ROOT/VERSION")"
case "$VERSION" in *-beta.*) TRACK=beta ;; *) TRACK=stable ;; esac
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT INT TERM

# The one-time bridge must copy this exact reviewed installer before it runs an
# update. Dry-run exits at that safe handoff boundary for this smoke.
mkdir -p "$TMP/bridge-bundle/app/release" "$TMP/bridge-home"
cp "$INSTALLER" "$TMP/bridge-bundle/install.sh"
printf '%s\n' "$VERSION" > "$TMP/bridge-bundle/app/VERSION"
printf '{"version":"%s","track":"%s"}\n' "$VERSION" "$TRACK" > "$TMP/bridge-bundle/app/release/release.json"
HOME="$TMP/bridge-home" DASHGO_SELECTOR_BRIDGE_DRY_RUN=1 bash "$TMP/bridge-bundle/install.sh" --bootstrap-selector-integrity
cmp -s "$TMP/bridge-bundle/install.sh" "$TMP/bridge-home/install.sh" || {
  echo 'FAIL: selector-integrity bridge did not install the reviewed updater bytes' >&2
  exit 1
}
[ -x "$TMP/bridge-home/install.sh" ] || { echo 'FAIL: selector-integrity bridge did not preserve executable mode' >&2; exit 1; }

# Extract the real transaction function but replace its external effects with
# narrow deterministic stubs. The managed file replacement loop stays real.
awk '
  /^update_commit_required_free_mb\(\)/ { capture=1 }
  capture && /^# Runtime rollback is update-only/ { exit }
  capture { print }
' "$INSTALLER" > "$TMP/install-release-payload.sh"
[ -s "$TMP/install-release-payload.sh" ] || { echo 'FAIL: could not extract install_release_payload' >&2; exit 1; }
# shellcheck disable=SC1090
source "$TMP/install-release-payload.sh"

DASH="$TMP/dashboard"
BIN_DIR="$DASH/bin"; CONFIG_DIR="$DASH/config"; CAL_DIR="$DASH/calendars"; CACHE_DIR="$DASH/cache"; LOG_DIR="$DASH/logs"; FONT_DIR="$DASH/ui/fonts"; RUNTIME_FONT_DIR="$DASH/runtime-fonts"; BASE_DIR="$DASH/base"; INSTALLER="$TMP/home/install.sh"
mkdir -p "$DASH/bin" "$TMP/home"
printf '#!/usr/bin/env sh\necho beta6-drifted-selector\n' > "$DASH/bin/dashboard-control-server"
chmod 755 "$DASH/bin/dashboard-control-server"

PAYLOAD="$TMP/payload/release/app"
mkdir -p "$PAYLOAD/bin" "$PAYLOAD/ui/js" "$PAYLOAD/ui" "$PAYLOAD/cmd/dashboard-control-server"
printf '<!doctype html>\n' > "$PAYLOAD/index.html"
printf '#!/usr/bin/env sh\n' > "$PAYLOAD/kiosk.sh"
printf '%s\n' "$VERSION" > "$PAYLOAD/VERSION"
printf '{"version":"%s","files":[]}\n' "$VERSION" > "$PAYLOAD/manifest.json"
printf '/* css */\n' > "$PAYLOAD/ui/dashboard.css"
printf '/* css */\n' > "$PAYLOAD/ui/control-layout.css"
printf '// js\n' > "$PAYLOAD/ui/js/app.bundle.js"
printf '// js\n' > "$PAYLOAD/ui/js/app.control.bundle.js"
printf '#!/usr/bin/env sh\n' > "$PAYLOAD/bin/dashboard-common.sh"
printf '#!/usr/bin/env bash\n' > "$PAYLOAD/bin/doctor.sh"
printf 'module example.invalid/dash-go\n' > "$PAYLOAD/go.mod"
printf 'package main\n' > "$PAYLOAD/cmd/dashboard-control-server/main.go"
cat > "$PAYLOAD/bin/dashboard-control-server" <<'SELECTOR'
#!/usr/bin/env sh
exec "$(dirname "$0")/dashboard-control-server-linux-amd64" "$@"
SELECTOR
chmod 755 "$PAYLOAD/bin/dashboard-control-server" "$PAYLOAD/kiosk.sh" "$PAYLOAD/bin/dashboard-common.sh" "$PAYLOAD/bin/doctor.sh"
for arch in 386 amd64 arm64 armv6 armv7; do
  cat > "$PAYLOAD/bin/dashboard-control-server-linux-$arch" <<'SERVER'
#!/usr/bin/env sh
[ "${1:-}" = '--verify-generated-assets' ] && exit 0
exit 0
SERVER
  chmod 755 "$PAYLOAD/bin/dashboard-control-server-linux-$arch"
done
cp "$PAYLOAD/bin/dashboard-control-server" "$TMP/canonical-selector"
printf '#!/usr/bin/env bash\n' > "$TMP/canonical-installer"
chmod 755 "$TMP/canonical-installer"
mkdir -p "$TMP/tar-root/release"
cp -a "$TMP/payload/release/app" "$TMP/tar-root/release/app"
cp "$TMP/canonical-installer" "$TMP/tar-root/release/install.sh"
tar -C "$TMP/tar-root" -czf "$TMP/release.tar.gz" release

# Transaction dependencies intentionally validate the selector bytes after the
# real replacement loop; any post-commit selector rewrite fails the smoke.
warn(){ printf '%s\n' "$*" >&2; }
ok(){ printf '%s\n' "$*"; }
write_update_phase(){ :; }
snapshot_personal_settings(){ :; }
find_payload_root(){ printf '%s\n' "$1/release/app"; }
validate_download(){ [ -f "$2" ]; }
update_cli_supports(){ return 1; }
manifest_verify_shell(){ return 0; }
manifest_file_list_shell(){ cat <<'FILES'
index.html
kiosk.sh
VERSION
manifest.json
ui/dashboard.css
ui/control-layout.css
ui/js/app.bundle.js
ui/js/app.control.bundle.js
bin/dashboard-common.sh
bin/doctor.sh
bin/dashboard-control-server
bin/dashboard-control-server-linux-386
bin/dashboard-control-server-linux-amd64
bin/dashboard-control-server-linux-arm64
bin/dashboard-control-server-linux-armv6
bin/dashboard-control-server-linux-armv7
go.mod
cmd/dashboard-control-server/main.go
FILES
}
release_server_for_host(){ printf '%s/bin/dashboard-control-server-linux-amd64\n' "$1"; }
atomic_replace_file(){ mkdir -p "$(dirname "$2")"; cp -p "$1" "$2"; }
purge_stale_managed_sources(){ :; }
restore_personal_settings(){ :; }
refresh_private_calendar_configuration_after_payload(){ :; }
snapshot_canonical_installer(){ :; }
install_canonical_installer(){ cp -p "$1" "$INSTALLER"; chmod 700 "$INSTALLER"; }
verify_installed_release_manifest(){ cmp -s "$TMP/canonical-selector" "$DASH/bin/dashboard-control-server"; }
ensure_go_dashboard_service_unit(){ :; }
ensure_dashboard_update_service(){ :; }
verify_go_updater_capabilities(){ :; }
write_updater_migration_receipt(){ :; }
retain_update_rollback_stage(){ rm -rf "$1"; }
rollback_release_transaction(){ echo "FAIL: unexpected rollback: $2" >&2; return 1; }

install_release_payload "$TMP/release.tar.gz" "$VERSION" '' "$TMP/canonical-installer"
cmp -s "$TMP/canonical-selector" "$DASH/bin/dashboard-control-server" || {
  echo "FAIL: beta.6→$VERSION transaction did not leave the manifest-owned selector byte-identical to the payload" >&2
  exit 1
}
[ "$(head -c 2 "$DASH/bin/dashboard-control-server")" = '#!' ] || { echo "FAIL: selector was not replaced from the $VERSION shell wrapper" >&2; exit 1; }
printf '%s\n' 'PASS: beta.6 bridge refreshes the updater and the current transaction self-heals selector drift with manifest-matching payload bytes'
