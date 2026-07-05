#!/usr/bin/env bash
# Source smoke for Dash-Go managed Debian security-maintenance posture.
# It models APT's already-authenticated InRelease metadata with a tiny gpgv
# fixture so mirror identity decisions stay offline, deterministic, and
# independent of external repository hostnames.
set -euo pipefail

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
LIB="$ROOT/bin/dashboard-security-maintenance.sh"
INSTALLER="$(CDPATH= cd -- "$ROOT/.." && pwd)/installer/install.sh"
DOCTOR="$ROOT/bin/doctor.sh"
PLAN="$ROOT/bin/dashboard-doctor-plan.sh"

fail(){ printf 'FAIL: %s\n' "$*" >&2; exit 1; }
need(){ "$@" || fail "command failed: $*"; }
contains(){ printf '%s\n' "$1" | grep -Fq -- "$2" || fail "expected '$2' in: $1"; }

bash -n "$LIB"
bash -n "$INSTALLER"
bash -n "$DOCTOR"
bash -n "$PLAN"

TMP="$(mktemp -d "${TMPDIR:-/tmp}/dash-go-security-maintenance-test.XXXXXX")"
trap 'rm -rf "$TMP"' EXIT
mkdir -p "$TMP/etc/apt/sources.list.d" "$TMP/etc/apt/preferences.d" "$TMP/etc/apt/apt.conf.d" \
  "$TMP/etc/systemd/system/apt-daily.timer.d" "$TMP/etc/systemd/system/apt-daily-upgrade.timer.d" \
  "$TMP/etc/systemd/system/apt-daily.service.d" "$TMP/etc/systemd/system/apt-daily-upgrade.service.d" \
  "$TMP/apt-lists" "$TMP/fake-bin"
cat > "$TMP/etc/os-release" <<'EOF_OS'
ID=debian
VERSION_CODENAME=trixie
EOF_OS
cat > "$TMP/etc/apt/sources.list.d/debian.sources" <<'EOF_SOURCES'
Types: deb
URIs: https://mirror.example.invalid/debian
Suites: trixie trixie-updates
Components: main contrib non-free non-free-firmware
EOF_SOURCES
cat > "$TMP/apt-lists/mirror_dists_trixie_InRelease" <<'EOF_BASE'
-----BEGIN PGP SIGNED MESSAGE-----
Hash: SHA256

Origin: Debian
Label: Debian
Suite: stable
Codename: trixie
Signature-Status: valid
EOF_BASE
cat > "$TMP/fake-bin/gpgv" <<'EOF_GPGV'
#!/usr/bin/env bash
set -euo pipefail
file="${!#}"
grep -Fqx 'Signature-Status: valid' "$file"
EOF_GPGV
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
chmod +x "$TMP/fake-bin/gpgv" "$TMP/fake-bin/dpkg-query" "$TMP/fake-bin/systemctl"
: > "$TMP/debian-archive-keyring.gpg"

# shellcheck disable=SC1090
export DASHGO_APT_ROOT="$TMP"
export DASHGO_SECURITY_APT_METADATA_DIR="$TMP/apt-lists"
export DASHGO_SECURITY_KEYRING="$TMP/debian-archive-keyring.gpg"
export DASHGO_SECURITY_GPGV="$TMP/fake-bin/gpgv"
export DASHGO_SECURITY_ARCH=amd64
export PATH="$TMP/fake-bin:$PATH"
. "$LIB"

# A country alias, caching proxy, or properly mirrored local endpoint now
# qualifies through authenticated Debian metadata, never by hostname.
[ "$(dashboard_security_base_source_state)" = configured ] || fail "Deb822 mirror base source was not discovered"
need dashboard_security_base_repository_verified
! dashboard_security_maintenance_supported || fail "security maintenance must require trixie-security too"
need dashboard_security_security_repair_eligible
contains "$(dashboard_security_maintenance_support_reason)" 'trixie-security source is configured'

# The explicit repair target is canonical, but only a verified base makes it
# eligible. Add the source/metadata that an APT refresh would authenticate.
target="$(dashboard_security_managed_path security-source)"
mkdir -p "$(dirname "$target")"
dashboard_security_expected_file security-source > "$target"
cat > "$TMP/apt-lists/security_dists_trixie-security_InRelease" <<'EOF_SECURITY'
-----BEGIN PGP SIGNED MESSAGE-----
Hash: SHA256

Origin: Debian
Label: Debian-Security
Suite: stable-security
Codename: trixie-security
Signature-Status: valid
EOF_SECURITY
for kind in backports-source backports-pin auto-upgrades unattended-policy daily-timer upgrade-timer daily-service upgrade-service; do
  target="$(dashboard_security_managed_path "$kind")"
  mkdir -p "$(dirname "$target")"
  dashboard_security_expected_file "$kind" > "$target"
done

need dashboard_security_security_source_present
need dashboard_security_security_repository_verified
need dashboard_security_maintenance_supported
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

# .list entries through a local proxy are equally valid when the current
# authenticated metadata proves Debian identity and the selected suite.
rm -f "$TMP/etc/apt/sources.list.d/debian.sources"
cat > "$TMP/etc/apt/sources.list.d/local-mirror.list" <<'EOF_LIST'
deb http://apt-cache.lan:3142/debian trixie main contrib non-free non-free-firmware
EOF_LIST
need dashboard_security_base_repository_verified
need dashboard_security_maintenance_supported

# Wrong-suite and insecure entries fail before any repair can be offered.
cat > "$TMP/etc/apt/sources.list.d/local-mirror.list" <<'EOF_WRONG'
deb http://apt-cache.lan:3142/debian bookworm main
EOF_WRONG
! dashboard_security_base_repository_verified || fail "wrong-suite base source qualified"
contains "$(dashboard_security_maintenance_support_reason)" 'select another suite'
! dashboard_security_security_repair_eligible || fail "wrong-suite source became repair-eligible"
cat > "$TMP/etc/apt/sources.list.d/local-mirror.list" <<'EOF_INSECURE'
deb [trusted=yes] http://apt-cache.lan:3142/debian trixie main
EOF_INSECURE
[ "$(dashboard_security_base_source_state)" = insecure ] || fail "trusted=yes source was not classified insecure"
! dashboard_security_base_repository_verified || fail "trusted=yes base source qualified"
contains "$(dashboard_security_maintenance_support_reason)" 'marked trusted or insecure'
! dashboard_security_security_repair_eligible || fail "insecure source became repair-eligible"
cat > "$TMP/etc/apt/sources.list.d/local-mirror.list" <<'EOF_RESTORE'
deb http://apt-cache.lan:3142/debian trixie main
EOF_RESTORE
need dashboard_security_maintenance_supported

# Signature/identity checks are mandatory even with an otherwise correct URI.
printf '%s\n' 'Signature-Status: invalid' > "$TMP/apt-lists/security_dists_trixie-security_InRelease"
! dashboard_security_security_repository_verified || fail "invalid security metadata qualified"
contains "$(dashboard_security_maintenance_support_reason)" 'Debian-Security signed Release metadata'
cat > "$TMP/apt-lists/security_dists_trixie-security_InRelease" <<'EOF_SECURITY_RESTORED'
-----BEGIN PGP SIGNED MESSAGE-----
Hash: SHA256

Origin: Debian
Label: Debian-Security
Suite: stable-security
Codename: trixie-security
Signature-Status: valid
EOF_SECURITY_RESTORED
need dashboard_security_maintenance_supported

cat > "$TMP/etc/apt/apt.conf.d/99-local-origin-override" <<'EOF_OVERRIDE'
Unattended-Upgrade::Origins-Pattern {
        "origin=Example";
};
EOF_OVERRIDE
need dashboard_security_later_origin_policy_present
! dashboard_security_managed_files_current || fail "later origin override must make health state non-current"
rm -f "$TMP/etc/apt/apt.conf.d/99-local-origin-override"
need dashboard_security_managed_files_current

# Raspberry Pi OS remains protected: 32-bit Raspbian must never borrow Debian
# security policy; a 64-bit layout still requires the same verified metadata.
cat > "$TMP/etc/os-release" <<'EOF_RASPBIAN'
ID=raspbian
VERSION_CODENAME=trixie
EOF_RASPBIAN
DASHGO_SECURITY_ARCH=armhf
! dashboard_security_maintenance_supported || fail "32-bit Raspbian must not receive Debian security repositories"
contains "$(dashboard_security_maintenance_support_reason)" '32-bit Raspbian'
DASHGO_SECURITY_ARCH=arm64
need dashboard_security_maintenance_supported

# Installer/Doctor repair contract: only --repair --system may add the source;
# it snapshots changes, refreshes APT, validates signed metadata, and rolls
# back Dash-Go-owned files if the candidate cannot be authenticated.
grep -Fq 'dashboard_security_security_repair_eligible' "$INSTALLER" || fail "installer lacks verified-base repair eligibility"
grep -Fq '[ "$reason" = repair ] && dashboard_security_security_repair_eligible' "$INSTALLER" || fail "installer may add a security source outside explicit repair"
grep -Fq 'Normal' "$INSTALLER" || fail "installer lacks normal-update source-preservation explanation"
grep -Fq 'dashboard_security_maintenance_supported && dashboard_security_managed_files_current' "$INSTALLER" || fail "installer does not validate repository metadata after repair"
grep -Fq 'restoring Dash-Go-owned policy files' "$INSTALLER" || fail "installer rollback is missing"
grep -Fq 'security-maintenance-source' "$DOCTOR" || fail "Doctor lacks opt-in security-source repair item"
grep -Fq 'security-maintenance|security-maintenance-source' "$PLAN" || fail "Doctor plan does not route source repair to system tier"
grep -A4 -F 'for svc in hciuart exim4 ModemManager nfs-blkmap e2scrub_reap' "$INSTALLER" | grep -Fq 'unattended-upgrades' && fail "Pi trim still disables unattended-upgrades"

printf 'security-maintenance smoke passed\n'
