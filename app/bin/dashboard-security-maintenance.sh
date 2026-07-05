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

# APT source eligibility deliberately does not recognize repositories by
# hostname. Country aliases, HTTPS caching proxies, and local mirrors are
# legitimate when APT has current Debian-signed Release metadata for the
# installed suite. Insecure source options never qualify.
dashboard_security_source_file_state(){
  local file="$1" suite="$2"
  case "$file" in
    *.sources)
      awk -v wanted_suite="$suite" '
        BEGIN { RS=""; IGNORECASE=1 }
        function value_for(name,    n,i,line,key) {
          n=split($0, lines, "\\n")
          for (i=1; i<=n; i++) {
            line=lines[i]; sub(/^[[:space:]]*/, "", line)
            key="^" name ":[[:space:]]*"
            if (line ~ key) { sub(key, "", line); return line }
          }
          return ""
        }
        function enabled(value) { value=tolower(value); return !(value=="no" || value=="false" || value=="0") }
        function unsafe(value) { value=tolower(value); return value ~ /(^|[[:space:]])(yes|true|1)([[:space:]]|$)/ }
        function has_word(value, word) { return (" " value " ") ~ ("(^|[[:space:]])" word "([[:space:]]|$)") }
        {
          types=value_for("Types"); suites=value_for("Suites"); enabled_value=value_for("Enabled")
          trusted=value_for("Trusted"); allow_insecure=value_for("Allow-Insecure"); downgrade=value_for("Allow-Downgrade-To-Insecure")
          if (has_word(types, "deb") && has_word(suites, wanted_suite) && enabled(enabled_value)) {
            if (unsafe(trusted) || unsafe(allow_insecure) || unsafe(downgrade)) insecure=1
            else configured=1
          }
        }
        END {
          if (insecure) print "insecure"
          else if (configured) print "configured"
        }
      ' "$file"
      ;;
    *)
      awk -v wanted_suite="$suite" '
        /^[[:space:]]*#/ { next }
        {
          line=$0; sub(/[[:space:]]*#.*/, "", line); sub(/^[[:space:]]*/, "", line)
          if (line=="") next
          n=split(line, field, /[[:space:]]+/)
          if (field[1] != "deb") next
          pos=2; options=""
          if (field[pos] ~ /^\[/) {
            while (pos<=n) {
              options=options " " field[pos]
              if (field[pos] ~ /\]$/) { pos++; break }
              pos++
            }
          }
          if (pos+1>n) next
          suite=field[pos+1]
          if (suite != wanted_suite) next
          lower=tolower(options)
          if (lower ~ /(^|[[:space:]\[])(trusted|allow-insecure|allow-downgrade-to-insecure)[[:space:]]*=[[:space:]]*(yes|true|1)([[:space:]\]]|$)/) insecure=1
          else configured=1
        }
        END {
          if (insecure) print "insecure"
          else if (configured) print "configured"
        }
      ' "$file"
      ;;
  esac
}

dashboard_security_source_state(){
  local suite="$1" file state="missing" result
  while IFS= read -r file; do
    [ -n "$file" ] || continue
    result="$(dashboard_security_source_file_state "$file" "$suite" 2>/dev/null || true)"
    case "$result" in
      insecure) printf '%s\n' insecure; return 0 ;;
      configured) state=configured ;;
    esac
  done < <(dashboard_security_apt_source_files)
  printf '%s\n' "$state"
}

dashboard_security_any_binary_source_present(){
  local file
  while IFS= read -r file; do
    [ -n "$file" ] || continue
    case "$file" in
      *.sources) awk 'BEGIN { RS=""; IGNORECASE=1 } /(^|\n)[[:space:]]*Types:[^\n]*\<deb\>/ { found=1 } END { exit(found ? 0 : 1) }' "$file" && return 0 ;;
      *) awk '/^[[:space:]]*deb([[:space:]]|$)/ { found=1 } END { exit(found ? 0 : 1) }' "$file" && return 0 ;;
    esac
  done < <(dashboard_security_apt_source_files)
  return 1
}

dashboard_security_metadata_dir(){
  if [ -n "${DASHGO_SECURITY_APT_METADATA_DIR:-}" ]; then
    printf '%s\n' "$DASHGO_SECURITY_APT_METADATA_DIR"
  else
    _dashboard_security_path /var/lib/apt/lists
  fi
}

dashboard_security_debian_keyring(){
  if [ -n "${DASHGO_SECURITY_KEYRING:-}" ]; then
    printf '%s\n' "$DASHGO_SECURITY_KEYRING"
  else
    _dashboard_security_path /usr/share/keyrings/debian-archive-keyring.gpg
  fi
}

dashboard_security_gpgv(){
  printf '%s\n' "${DASHGO_SECURITY_GPGV:-gpgv}"
}

dashboard_security_metadata_files(){
  local dir
  dir="$(dashboard_security_metadata_dir)"
  [ -d "$dir" ] || return 0
  find "$dir" -maxdepth 1 -type f -name '*InRelease' -print 2>/dev/null | sort
}

dashboard_security_metadata_signature_valid(){
  local file="$1" keyring verifier
  keyring="$(dashboard_security_debian_keyring)"
  verifier="$(dashboard_security_gpgv)"
  [ -r "$file" ] && [ -r "$keyring" ] || return 1
  command -v "$verifier" >/dev/null 2>&1 || return 1
  "$verifier" --keyring "$keyring" "$file" >/dev/null 2>&1
}

dashboard_security_metadata_identity_matches(){
  local file="$1" wanted_suite="$2" wanted_label="$3"
  awk -v wanted_suite="$wanted_suite" -v wanted_label="$wanted_label" '
    function clean(value) { sub(/^[[:space:]]*/, "", value); sub(/[[:space:]]*\r$/, "", value); return value }
    /^Origin:[[:space:]]*/ { value=$0; sub(/^Origin:[[:space:]]*/, "", value); origin=clean(value) }
    /^Label:[[:space:]]*/ { value=$0; sub(/^Label:[[:space:]]*/, "", value); label=clean(value) }
    /^Suite:[[:space:]]*/ { value=$0; sub(/^Suite:[[:space:]]*/, "", value); suite=clean(value) }
    /^Codename:[[:space:]]*/ { value=$0; sub(/^Codename:[[:space:]]*/, "", value); codename=clean(value) }
    END {
      good_origin=(tolower(origin)=="debian")
      good_label=(tolower(label)==tolower(wanted_label))
      good_suite=(tolower(suite)==tolower(wanted_suite) || tolower(codename)==tolower(wanted_suite))
      exit(good_origin && good_label && good_suite ? 0 : 1)
    }
  ' "$file"
}

dashboard_security_verified_metadata_present(){
  local suite="$1" label="$2" file
  while IFS= read -r file; do
    [ -n "$file" ] || continue
    dashboard_security_metadata_signature_valid "$file" || continue
    dashboard_security_metadata_identity_matches "$file" "$suite" "$label" && return 0
  done < <(dashboard_security_metadata_files)
  return 1
}

dashboard_security_platform_eligible(){
  dashboard_security_load_platform
  case "$DASHBOARD_SECURITY_OS_ID" in
    debian|raspbian) ;;
    *) return 1 ;;
  esac
  dashboard_security_supported_codename "$DASHBOARD_SECURITY_CODENAME" || return 1
  # Raspberry Pi OS 32-bit uses the independent Raspbian archive. Do not add
  # Debian security/backports sources to it. A 64-bit Pi layout remains
  # eligible only when authenticated Debian repository metadata is present.
  if [ "$DASHBOARD_SECURITY_OS_ID" = raspbian ]; then
    case "$DASHBOARD_SECURITY_ARCH" in armhf|armv6l|armv7l) return 1;; esac
  fi
}

dashboard_security_base_source_state(){
  dashboard_security_load_platform
  dashboard_security_source_state "$DASHBOARD_SECURITY_CODENAME"
}

dashboard_security_security_source_state(){
  dashboard_security_load_platform
  dashboard_security_source_state "${DASHBOARD_SECURITY_CODENAME}-security"
}

dashboard_security_base_repository_verified(){
  dashboard_security_platform_eligible || return 1
  [ "$(dashboard_security_base_source_state)" = configured ] || return 1
  dashboard_security_verified_metadata_present "$DASHBOARD_SECURITY_CODENAME" Debian
}

dashboard_security_security_repository_verified(){
  dashboard_security_platform_eligible || return 1
  [ "$(dashboard_security_security_source_state)" = configured ] || return 1
  dashboard_security_verified_metadata_present "${DASHBOARD_SECURITY_CODENAME}-security" Debian-Security
}

dashboard_security_security_repair_eligible(){
  dashboard_security_base_repository_verified || return 1
  [ "$(dashboard_security_security_source_state)" = missing ]
}

dashboard_security_maintenance_supported(){
  dashboard_security_base_repository_verified && dashboard_security_security_repository_verified
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
  case "$(dashboard_security_base_source_state)" in
    missing)
      if dashboard_security_any_binary_source_present; then
        printf '%s\n' "no active Debian ${DASHBOARD_SECURITY_CODENAME} base source was found; enabled APT entries select another suite or are not binary Debian entries"
      else
        printf '%s\n' "no active Debian ${DASHBOARD_SECURITY_CODENAME} base source was found"
      fi
      return 0
      ;;
    insecure)
      printf '%s\n' "the active Debian ${DASHBOARD_SECURITY_CODENAME} base source is marked trusted or insecure"
      return 0
      ;;
  esac
  if ! dashboard_security_base_repository_verified; then
    printf '%s\n' "the active Debian ${DASHBOARD_SECURITY_CODENAME} base source lacks current Debian-signed Release metadata"
    return 0
  fi
  case "$(dashboard_security_security_source_state)" in
    missing)
      printf '%s\n' "a verified Debian ${DASHBOARD_SECURITY_CODENAME} base source is present, but no active ${DASHBOARD_SECURITY_CODENAME}-security source is configured; run ~/install.sh --repair --system to add Dash-Go's canonical security source"
      return 0
      ;;
    insecure)
      printf '%s\n' "the active ${DASHBOARD_SECURITY_CODENAME}-security source is marked trusted or insecure"
      return 0
      ;;
  esac
  if ! dashboard_security_security_repository_verified; then
    printf '%s\n' "the active ${DASHBOARD_SECURITY_CODENAME}-security source lacks current Debian-Security signed Release metadata"
    return 0
  fi
  printf '%s\n' supported
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
  [ "$(dashboard_security_security_source_state)" = configured ]
}

dashboard_security_backports_source_present(){
  dashboard_security_load_platform
  [ "$(dashboard_security_source_state "${DASHBOARD_SECURITY_CODENAME}-backports")" = configured ]
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
  dashboard_security_maintenance_supported || return 1
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
