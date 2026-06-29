#!/usr/bin/env bash
# Release-blocking contract: boot-critical LightDM changes are staged before
# legacy keys are touched, and a staging/storage failure leaves the previous
# autologin state intact. No root, systemd, or host LightDM is required.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TMP="$(mktemp -d)"
cleanup(){ rm -rf "$TMP"; }
trap cleanup EXIT INT TERM
DASH="$TMP/dashboard"; CACHE_DIR="$DASH/cache"; BIN_DIR="$DASH/bin"
LIGHTDM="$TMP/lightdm"; XSESSIONS="$TMP/xsessions"; FAKEBIN="$TMP/fakebin"
USER_NAME="$(id -un)"
mkdir -p "$CACHE_DIR" "$BIN_DIR" "$LIGHTDM/lightdm.conf.d" "$XSESSIONS" "$FAKEBIN"
cp "$ROOT/bin/dashboard-common.sh" "$BIN_DIR/dashboard-common.sh"
cp "$ROOT/bin/dashboard-lite-session.sh" "$BIN_DIR/dashboard-lite-session.sh"
chmod +x "$BIN_DIR/dashboard-lite-session.sh"
cat > "$XSESSIONS/dashboard-openbox.desktop" <<EOF_XS
[Desktop Entry]
Name=Dash-Go Openbox
Exec=$BIN_DIR/dashboard-lite-session.sh
TryExec=$BIN_DIR/dashboard-lite-session.sh
Type=Application
DesktopNames=DashGo
EOF_XS
legacy(){ cat > "$LIGHTDM/lightdm.conf" <<EOF_L
[Seat:*]
autologin-user=$USER_NAME
autologin-user-timeout=0
autologin-session=dashboard-openbox
EOF_L
}
legacy
export DASH CACHE_DIR BIN_DIR USER_NAME
export DASH_LIGHTDM_ROOT="$LIGHTDM" DASH_LIGHTDM_VENDOR_DIR="$TMP/vendor" DASH_XSESSIONS_DIR="$XSESSIONS"
SUDO=""
# shellcheck source=/dev/null
. "$BIN_DIR/dashboard-common.sh"

# A successful migration is still supported.
write_dashboard_lightdm_autologin "$USER_NAME" dashboard-openbox
[ -f "$LIGHTDM/lightdm.conf.d/90-dash-go-autologin.conf" ]
! grep -q '^autologin-user=' "$LIGHTDM/lightdm.conf"

# If staging the managed drop-in fails, legacy autologin must remain exactly
# intact. Fake only tee after the session file already exists.
legacy
rm -f "$LIGHTDM/lightdm.conf.d/90-dash-go-autologin.conf"
cp "$LIGHTDM/lightdm.conf" "$TMP/original-lightdm.conf"
cat > "$FAKEBIN/sudo" <<'EOF_SUDO'
#!/usr/bin/env bash
exec "$@"
EOF_SUDO
cat > "$FAKEBIN/tee" <<'EOF_TEE'
#!/usr/bin/env bash
case "${1:-}" in "${DASH_LIGHTDM_ROOT}/lightdm.conf.d/.90-dash-go-autologin.conf.tmp."*) exit 1;; esac
exec /usr/bin/tee "$@"
EOF_TEE
chmod +x "$FAKEBIN/sudo" "$FAKEBIN/tee"
PATH="$FAKEBIN:$PATH"
hash -r
SUDO="$FAKEBIN/sudo"
if write_dashboard_lightdm_autologin "$USER_NAME" dashboard-openbox; then
  echo 'FAIL: staged drop-in write unexpectedly succeeded' >&2; exit 1
fi
cmp -s "$TMP/original-lightdm.conf" "$LIGHTDM/lightdm.conf"
[ ! -f "$LIGHTDM/lightdm.conf.d/90-dash-go-autologin.conf" ]

# A known failing storage state must refuse before changing either configuration.
printf '{"level":"failing","reason":"fixture"}\n' > "$CACHE_DIR/storage-wear-state.json"
if write_dashboard_lightdm_autologin "$USER_NAME" dashboard-openbox; then
  echo 'FAIL: boot config rewrite ran despite failing storage' >&2; exit 1
fi
cmp -s "$TMP/original-lightdm.conf" "$LIGHTDM/lightdm.conf"
echo 'PASS: autologin staging and storage refusal preserve the prior LightDM configuration'
