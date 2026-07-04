#!/usr/bin/env bash
# Dash-Go managed Debian security-maintenance policy helpers.
# Safe to source: this file only inspects state and renders deterministic
# Dash-Go-owned configuration. The installer owns all privileged writes.

: "${DASHGO_APT_ROOT:=}"

_dashboard_security_path(){
  local path="$1"
  printf '%s%s\n' "${DASHGO_APT_ROOT:-}" "$path"
}

dashboard_security_os_release_path(){
  if [ -n "${DASHGO_SECURITY_OS_RELEASE:-}" ]; then
    printf '%s\n' "$DASHGO_SECURITY_OS_RELEASE"
  else
    _dashboard_security_path /etc/os-release
  fi
}

dashboard_security_load_platform(){
  local os_release
  DASHBOARD_SECURITY_OS_ID="unknown"
  DASHBOARD_SECURITY_CODENAME=""
  DASHBOARD_SECURITY_ARCH="${DASHGO_SECURITY_ARCH:-}"
  os_release="$(dashboard_security_os_release_path)"
  if [ -r "$os_release" ]; then
    DASHBOARD_SECURITY_OS_ID="$(sed -nE 's/^ID="?([^"[:space:]]+).*/\1/p' "$os_release" | head -n 1)"
    DASHBOARD_SECURITY_CODENAME="$(sed -nE 's/^VERSION_CODENAME="?([^"[:space:]]+).*/\1/p' "$os_release" | head -n 1)"
  fi
  [ -n "$DASHBOARD_SECURITY_OS_ID" ] || DASHBOARD_SECURITY_OS_ID=unknown
  if [ -z "$DASHBOARD_SECURITY_ARCH" ]; then
    DASHBOARD_SECURITY_ARCH="$(dpkg --print-architecture 2>/dev/null || uname -m 2>/dev/null || true)"
  fi
  export DASHBOARD_SECURITY_OS_ID DASHBOARD_SECURITY_CODENAME DASHBOARD_SECURITY_ARCH
}

dashboard_security_supported_codename(){
  case "${1:-}" in bookworm|trixie) return 0;; *) return 1;; esac
}

dashboard_security_apt_source_files(){
  local root f
  root="${DASHGO_APT_ROOT:-}"
  for f in "$root/etc/apt/sources.list" "$root/etc/apt/sources.list.d"/*.list "$root/etc/apt/sources.list.d"/*.sources; do
    [ -r "$f" ] && printf '%s\n' "$f"
  done
}

dashboard_security_debian_source_present(){
  local codename="$1" file
  while IFS= read -r file; do
    [ -n "$file" ] || continue
    case "$file" in
      *.sources)
        if awk -v codename="$codename" '
          BEGIN { RS=""; IGNORECASE=1 }
          {
            types=""; uris=""; suites=""
            n=split($0, lines, "\n")
            for (i=1; i<=n; i++) {
              line=lines[i]
              sub(/^[[:space:]]*/, "", line)
              if (line ~ /^Types:[[:space:]]*/) { sub(/^Types:[[:space:]]*/, "", line); types=types " " line }
              if (line ~ /^URIs:[[:space:]]*/) { sub(/^URIs:[[:space:]]*/, "", line); uris=uris " " line }
              if (line ~ /^Suites:[[:space:]]*/) { sub(/^Suites:[[:space:]]*/, "", line); suites=suites " " line }
            }
            if (types ~ /(^|[[:space:]])deb([[:space:]]|$)/ &&
                uris ~ /(^|[[:space:]])https?:\/\/(deb\.debian\.org|ftp\.debian\.org)\/debian([[:space:]]|$)/ &&
                suites ~ ("(^|[[:space:]])" codename "([[:space:]]|$)")) found=1
          }
          END { exit(found ? 0 : 1) }
        ' "$file"; then return 0; fi
        ;;
      *)
        if awk -v codename="$codename" '
          /^[[:space:]]*#/ { next }
          {
            line=$0; sub(/[[:space:]]*#.*/, "", line); sub(/^[[:space:]]*/, "", line)
            n=split(line, field, /[[:space:]]+/)
            if (field[1] == "deb" && field[2] ~ /^https?:\/\/(deb\.debian\.org|ftp\.debian\.org)\/debian\/?$/ && field[3] == codename) found=1
          }
          END { exit(found ? 0 : 1) }
        ' "$file"; then return 0; fi
        ;;
    esac
  done < <(dashboard_security_apt_source_files)
  return 1
}

dashboard_security_archive_source_present(){
  local suite="$1" host_path="$2" file
  while IFS= read -r file; do
    [ -n "$file" ] || continue
    case "$file" in
      *.sources)
        if awk -v suite="$suite" -v host_path="$host_path" '
          BEGIN { RS=""; IGNORECASE=1 }
          {
            types=""; uris=""; suites=""
            n=split($0, lines, "\n")
            for (i=1; i<=n; i++) {
              line=lines[i]
              sub(/^[[:space:]]*/, "", line)
              if (line ~ /^Types:[[:space:]]*/) { sub(/^Types:[[:space:]]*/, "", line); types=types " " line }
              if (line ~ /^URIs:[[:space:]]*/) { sub(/^URIs:[[:space:]]*/, "", line); uris=uris " " line }
              if (line ~ /^Suites:[[:space:]]*/) { sub(/^Suites:[[:space:]]*/, "", line); suites=suites " " line }
            }
            uri_re="(^|[[:space:]])https?://" host_path "([[:space:]]|$)"
            suite_re="(^|[[:space:]])" suite "([[:space:]]|$)"
            if (types ~ /(^|[[:space:]])deb([[:space:]]|$)/ && uris ~ uri_re && suites ~ suite_re) found=1
          }
          END { exit(found ? 0 : 1) }
        ' "$file"; then return 0; fi
        ;;
      *)
        if awk -v suite="$suite" -v host_path="$host_path" '
          /^[[:space:]]*#/ { next }
          {
            line=$0; sub(/[[:space:]]*#.*/, "", line); sub(/^[[:space:]]*/, "", line)
            n=split(line, field, /[[:space:]]+/)
            uri_re="^https?://" host_path "/?$"
            if (field[1] == "deb" && field[2] ~ uri_re && field[3] == suite) found=1
          }
          END { exit(found ? 0 : 1) }
        ' "$file"; then return 0; fi
        ;;
    esac
  done < <(dashboard_security_apt_source_files)
  return 1
}

dashboard_security_maintenance_supported(){
  dashboard_security_load_platform
  case "$DASHBOARD_SECURITY_OS_ID" in
    debian|raspbian) ;;
    *) return 1 ;;
  esac
  dashboard_security_supported_codename "$DASHBOARD_SECURITY_CODENAME" || return 1
  # Raspberry Pi OS 32-bit uses the independent Raspbian archive. Do not add
  # Debian security/backports sources to it. A 64-bit Pi layout is accepted
  # only when its active sources already prove it uses the Debian archive.
  if [ "$DASHBOARD_SECURITY_OS_ID" = raspbian ]; then
    case "$DASHBOARD_SECURITY_ARCH" in armhf|armv6l|armv7l) return 1;; esac
  fi
  dashboard_security_debian_source_present "$DASHBOARD_SECURITY_CODENAME"
}

dashboard_security_maintenance_support_reason(){
  dashboard_security_load_platform
  case "$DASHBOARD_SECURITY_OS_ID" in
    debian|raspbian) ;;
    *) printf '%s\n' "unsupported OS: ${DASHBOARD_SECURITY_OS_ID}"; return 0;;
  esac
  if ! dashboard_security_supported_codename "$DASHBOARD_SECURITY_CODENAME"; then
    printf '%s\n' "unsupported Debian-family codename: ${DASHBOARD_SECURITY_CODENAME:-unknown}"
    return 0
  fi
  if [ "$DASHBOARD_SECURITY_OS_ID" = raspbian ]; then
    case "$DASHBOARD_SECURITY_ARCH" in
      armhf|armv6l|armv7l)
        printf '%s\n' '32-bit Raspbian uses a separate package archive; Dash-Go will not add Debian security sources'
        return 0
        ;;
    esac
  fi
  if ! dashboard_security_debian_source_present "$DASHBOARD_SECURITY_CODENAME"; then
    printf '%s\n' "no active official Debian ${DASHBOARD_SECURITY_CODENAME} base source was found"
    return 0
  fi
  printf '%s\n' 'supported'
}

dashboard_security_managed_path(){
  local kind="$1"
  case "$kind" in
    security-source) _dashboard_security_path /etc/apt/sources.list.d/dash-go-security-maintenance.sources ;;
    backports-source) _dashboard_security_path /etc/apt/sources.list.d/dash-go-backports.sources ;;
    backports-pin) _dashboard_security_path /etc/apt/preferences.d/90-dash-go-backports ;;
    auto-upgrades) _dashboard_security_path /etc/apt/apt.conf.d/20dash-go-auto-upgrades ;;
    unattended-policy) _dashboard_security_path /etc/apt/apt.conf.d/52dash-go-unattended-upgrades ;;
    daily-timer) _dashboard_security_path /etc/systemd/system/apt-daily.timer.d/90-dash-go-security-maintenance.conf ;;
    upgrade-timer) _dashboard_security_path /etc/systemd/system/apt-daily-upgrade.timer.d/90-dash-go-security-maintenance.conf ;;
    daily-service) _dashboard_security_path /etc/systemd/system/apt-daily.service.d/90-dash-go-security-maintenance.conf ;;
    upgrade-service) _dashboard_security_path /etc/systemd/system/apt-daily-upgrade.service.d/90-dash-go-security-maintenance.conf ;;
    *) return 1;;
  esac
}

dashboard_security_expected_file(){
  local kind="$1" codename="${2:-$DASHBOARD_SECURITY_CODENAME}"
  case "$kind" in
    security-source)
      cat <<EOF_SECURITY_SOURCE
# Managed by Dash-Go. Official Debian security archive for the installed codename.
Types: deb
URIs: https://security.debian.org/debian-security
Suites: ${codename}-security
Components: main contrib non-free non-free-firmware
EOF_SECURITY_SOURCE
      ;;
    backports-source)
      cat <<EOF_BACKPORTS_SOURCE
# Managed by Dash-Go. Backports remain dormant unless an administrator explicitly selects one.
Types: deb
URIs: https://deb.debian.org/debian
Suites: ${codename}-backports
Components: main contrib non-free non-free-firmware
EOF_BACKPORTS_SOURCE
      ;;
    backports-pin)
      cat <<EOF_BACKPORTS_PIN
# Managed by Dash-Go. Keep Debian backports opt-in only.
Package: *
Pin: release a=${codename}-backports
Pin-Priority: 100
EOF_BACKPORTS_PIN
      ;;
    auto-upgrades)
      cat <<'EOF_AUTO_UPGRADES'
// Managed by Dash-Go. APT lists and security-only unattended upgrades run daily.
APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Download-Upgradeable-Packages "1";
APT::Periodic::Unattended-Upgrade "1";
APT::Periodic::AutocleanInterval "7";
APT::Periodic::RandomSleep "0";
EOF_AUTO_UPGRADES
      ;;
    unattended-policy)
      cat <<EOF_UNATTENDED_POLICY
// Managed by Dash-Go. Clear package defaults before allowing only this exact Debian security archive.
#clear Unattended-Upgrade::Origins-Pattern;
Unattended-Upgrade::Origins-Pattern {
        "origin=Debian,codename=${codename}-security,label=Debian-Security";
};
Unattended-Upgrade::Automatic-Reboot "false";
Unattended-Upgrade::Automatic-Reboot-WithUsers "false";
Unattended-Upgrade::InstallOnShutdown "false";
Unattended-Upgrade::Remove-Unused-Dependencies "false";
Unattended-Upgrade::Remove-New-Unused-Dependencies "false";
Unattended-Upgrade::Remove-Unused-Kernel-Packages "false";
Unattended-Upgrade::MinimalSteps "true";
EOF_UNATTENDED_POLICY
      ;;
    daily-timer)
      cat <<'EOF_DAILY_TIMER'
# Managed by Dash-Go. Refresh package lists after the nightly browser restart.
[Timer]
OnCalendar=
OnCalendar=*-*-* 02:15
RandomizedDelaySec=10m
AccuracySec=1m
Persistent=true
EOF_DAILY_TIMER
      ;;
    upgrade-timer)
      cat <<'EOF_UPGRADE_TIMER'
# Managed by Dash-Go. Install security updates before housekeeping begins.
[Timer]
OnCalendar=
OnCalendar=*-*-* 02:45
RandomizedDelaySec=10m
AccuracySec=1m
Persistent=true
EOF_UPGRADE_TIMER
      ;;
    daily-service|upgrade-service)
      cat <<'EOF_SERVICE_PRIORITY'
# Managed by Dash-Go. Keep APT maintenance behind the kiosk on small devices.
[Service]
Nice=19
IOSchedulingClass=idle
IOSchedulingPriority=7
EOF_SERVICE_PRIORITY
      ;;
    *) return 1;;
  esac
}

dashboard_security_file_matches(){
  local kind="$1" path
  path="$(dashboard_security_managed_path "$kind")" || return 1
  [ -r "$path" ] || return 1
  cmp -s "$path" <(dashboard_security_expected_file "$kind")
}

dashboard_security_security_source_present(){
  dashboard_security_load_platform
  dashboard_security_archive_source_present "${DASHBOARD_SECURITY_CODENAME}-security" 'security\.debian\.org/debian-security'
}

dashboard_security_backports_source_present(){
  dashboard_security_load_platform
  dashboard_security_archive_source_present "${DASHBOARD_SECURITY_CODENAME}-backports" 'deb\.debian\.org/debian'
}

dashboard_security_package_installed(){
  command -v dpkg-query >/dev/null 2>&1 || return 1
  dpkg-query -W -f='${Status}' unattended-upgrades 2>/dev/null | grep -Fqx 'install ok installed'
}

dashboard_security_timers_ready(){
  command -v systemctl >/dev/null 2>&1 || return 1
  systemctl is-enabled --quiet apt-daily.timer 2>/dev/null &&
    systemctl is-active --quiet apt-daily.timer 2>/dev/null &&
    systemctl is-enabled --quiet apt-daily-upgrade.timer 2>/dev/null &&
    systemctl is-active --quiet apt-daily-upgrade.timer 2>/dev/null
}

dashboard_security_later_origin_policy_present(){
  local dir file base managed
  dir="$(_dashboard_security_path /etc/apt/apt.conf.d)"
  managed='52dash-go-unattended-upgrades'
  [ -d "$dir" ] || return 1
  for file in "$dir"/*; do
    [ -r "$file" ] || continue
    base="$(basename "$file")"
    [ "$base" = "$managed" ] && continue
    [[ "$base" > "$managed" ]] || continue
    grep -Eq '^[[:space:]]*(#clear[[:space:]]+)?Unattended-Upgrade::Origins-Pattern' "$file" 2>/dev/null && return 0
  done
  return 1
}

dashboard_security_managed_files_current(){
  local kind path
  dashboard_security_security_source_present || return 1
  dashboard_security_backports_source_present || return 1
  for kind in backports-pin auto-upgrades unattended-policy daily-timer upgrade-timer daily-service upgrade-service; do
    dashboard_security_file_matches "$kind" || return 1
  done
  # A Dash-Go-created source remains valid only while its full managed content
  # is intact. Existing distribution sources are intentionally not rewritten.
  for kind in security-source backports-source; do
    path="$(dashboard_security_managed_path "$kind")"
    [ ! -e "$path" ] || dashboard_security_file_matches "$kind" || return 1
  done
  ! dashboard_security_later_origin_policy_present
}
