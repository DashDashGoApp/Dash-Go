#!/usr/bin/env bash
# Source smoke for Dash-Go managed Debian security-maintenance posture.
set -euo pipefail

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
LIB="$ROOT/bin/dashboard-security-maintenance.sh"
INSTALLER="$(CDPATH= cd -- "$ROOT/.." && pwd)/installer/install.sh"
DOCTOR="$ROOT/bin/doctor.sh"
PLAN="$ROOT/bin/dashboard-doctor-plan.sh"

fail(){ printf 'FAIL: %s\n' "$*" >&2; exit 1; }
need(){ "$@" || fail "command failed: $*"; }

bash -n "$LIB"
bash -n "$INSTALLER"
bash -n "$DOCTOR"
bash -n "$PLAN"

TMP="$(mktemp -d "${TMPDIR:-/tmp}/dash-go-security-maintenance-test.XXXXXX")"
trap 'rm -rf "$TMP"' EXIT
mkdir -p "$TMP/etc/apt/sources.list.d" "$TMP/etc/apt/preferences.d" "$TMP/etc/apt/apt.conf.d" \
  "$TMP/etc/systemd/system/apt-daily.timer.d" "$TMP/etc/systemd/system/apt-daily-upgrade.timer.d" \
  "$TMP/etc/systemd/system/apt-daily.service.d" "$TMP/etc/systemd/system/apt-daily-upgrade.service.d" "$TMP/fake-bin"
cat > "$TMP/etc/os-release" <<'EOF_OS'
ID=debian
VERSION_CODENAME=trixie
EOF_OS
cat > "$TMP/etc/apt/sources.list.d/debian.sources" <<'EOF_SOURCES'
Types: deb
URIs: https://deb.debian.org/debian
Suites: trixie trixie-updates
Components: main contrib non-free non-free-firmware
EOF_SOURCES
cat > "$TMP/fake-bin/dpkg-query" <<'EOF_DPKG'
#!/usr/bin/env bash
printf 'install ok installed\n'
EOF_DPKG
cat > "$TMP/fake-bin/systemctl" <<'EOF_SYSTEMCTL'
#!/usr/bin/env bash
case "${1:-}" in
  is-enabled|is-active) exit 0 ;;
  is-failed) exit 1 ;;
  *) exit 0 ;;
esac
EOF_SYSTEMCTL
chmod +x "$TMP/fake-bin/dpkg-query" "$TMP/fake-bin/systemctl"

# shellcheck disable=SC1090
export DASHGO_APT_ROOT="$TMP"
export DASHGO_SECURITY_ARCH=amd64
export PATH="$TMP/fake-bin:$PATH"
. "$LIB"
need dashboard_security_maintenance_supported
! dashboard_security_security_source_present || fail "security source should be absent before Dash-Go creates it"
! dashboard_security_backports_source_present || fail "backports source should be absent before Dash-Go creates it"

for kind in security-source backports-source backports-pin auto-upgrades unattended-policy daily-timer upgrade-timer daily-service upgrade-service; do
  target="$(dashboard_security_managed_path "$kind")"
  mkdir -p "$(dirname "$target")"
  dashboard_security_expected_file "$kind" > "$target"
done

need dashboard_security_security_source_present
need dashboard_security_backports_source_present
need dashboard_security_managed_files_current
need dashboard_security_package_installed
need dashboard_security_timers_ready

grep -Fqx 'Pin-Priority: 100' "$TMP/etc/apt/preferences.d/90-dash-go-backports" || fail "backports pin is not 100"
grep -Fqx '#clear Unattended-Upgrade::Origins-Pattern;' "$TMP/etc/apt/apt.conf.d/52dash-go-unattended-upgrades" || fail "security-only policy does not clear stock origins"
grep -Fq 'codename=trixie-security' "$TMP/etc/apt/apt.conf.d/52dash-go-unattended-upgrades" || fail "security-only policy has wrong codename"
grep -Fqx 'Unattended-Upgrade::Automatic-Reboot "false";' "$TMP/etc/apt/apt.conf.d/52dash-go-unattended-upgrades" || fail "automatic reboot is not disabled"
grep -Fqx 'Unattended-Upgrade::Remove-Unused-Dependencies "false";' "$TMP/etc/apt/apt.conf.d/52dash-go-unattended-upgrades" || fail "unattended autoremove is not disabled"
grep -Fqx 'OnCalendar=*-*-* 02:45' "$TMP/etc/systemd/system/apt-daily-upgrade.timer.d/90-dash-go-security-maintenance.conf" || fail "security timer is not scheduled before housekeeping"
grep -Fqx 'Nice=19' "$TMP/etc/systemd/system/apt-daily-upgrade.service.d/90-dash-go-security-maintenance.conf" || fail "APT job priority is not lowered"

cat > "$TMP/etc/apt/apt.conf.d/99-local-origin-override" <<'EOF_OVERRIDE'
Unattended-Upgrade::Origins-Pattern {
        "origin=Example";
};
EOF_OVERRIDE
need dashboard_security_later_origin_policy_present
! dashboard_security_managed_files_current || fail "later origin override must make health state non-current"
rm -f "$TMP/etc/apt/apt.conf.d/99-local-origin-override"
need dashboard_security_managed_files_current

cat > "$TMP/etc/os-release" <<'EOF_RASPBIAN'
ID=raspbian
VERSION_CODENAME=trixie
EOF_RASPBIAN
DASHGO_SECURITY_ARCH=armhf
! dashboard_security_maintenance_supported || fail "32-bit Raspbian must not receive Debian security repositories"
dashboard_security_maintenance_support_reason | grep -Fq '32-bit Raspbian' || fail "32-bit Raspbian reason is not explicit"
DASHGO_SECURITY_ARCH=arm64
need dashboard_security_maintenance_supported

# Installer/Doctor contracts: installer retains no old service-disable conflict,
# updates reconcile without a general OS upgrade, and Doctor routes repair to
# the explicit system tier.
grep -Fq 'configure_security_maintenance(){' "$INSTALLER" || fail "installer does not own security-maintenance reconciliation"
grep -Fq 'ensure_security_maintenance_once update' "$INSTALLER" || fail "normal installer update does not reconcile security maintenance"
grep -A4 -F 'for svc in hciuart exim4 ModemManager nfs-blkmap e2scrub_reap' "$INSTALLER" | grep -Fq 'unattended-upgrades' && fail "Pi trim still disables unattended-upgrades"
grep -Fq 'check_security_maintenance(){' "$DOCTOR" || fail "Doctor has no security-maintenance check"
grep -Fq 'security-maintenance "Restore managed Debian security maintenance"' "$DOCTOR" || fail "Doctor repair plan is missing"
grep -Fq "security-maintenance) printf '%s\\n' '~/install.sh --repair --system'" "$PLAN" || fail "Doctor plan does not route to the system repair tier"

printf 'security-maintenance smoke passed\n'
