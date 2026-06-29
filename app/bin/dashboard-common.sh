#!/usr/bin/env bash
# Shared Dash-Go shell helpers.
# Safe to source: no prompts, no automatic system changes.

: "${DASH:=$HOME/dashboard}"
: "${BIN_DIR:=$DASH/bin}"
: "${CONFIG_DIR:=$DASH/config}"
: "${SUDO:=sudo}"

_dash_common_ok(){ if declare -F ok >/dev/null 2>&1; then ok "$*"; else printf '   %s\n' "$*"; fi; }
_dash_common_warn(){ if declare -F warn >/dev/null 2>&1; then warn "$*"; else printf '!! %s\n' "$*"; fi; }

read_device_model(){
  local f
  for f in /proc/device-tree/model /sys/firmware/devicetree/base/model; do
    if [ -r "$f" ]; then
      tr -d '\0' < "$f" 2>/dev/null
      return 0
    fi
  done
  awk -F: '/^(Model|Hardware|model name)[[:space:]]*:/{sub(/^[[:space:]]+/,"",$2); print $2; exit}' /proc/cpuinfo 2>/dev/null || true
}

load_os_release(){
  OS_ID="unknown"; OS_CODENAME=""; OS_VERSION_ID=""
  if [ -r /etc/os-release ]; then
    # shellcheck disable=SC1091
    . /etc/os-release
    OS_ID="${ID:-unknown}"
    OS_CODENAME="${VERSION_CODENAME:-}"
    OS_VERSION_ID="${VERSION_ID:-}"
  fi
}

detect_platform(){
  DASH_ARCH="$(uname -m 2>/dev/null || echo unknown)"
  DEVICE_MODEL="$(read_device_model)"
  load_os_release
  IS_PI=0; IS_DEBIAN=0; IS_DEBIAN_TRIXIE=0; IS_X86=0
  case "$DEVICE_MODEL" in *"Raspberry Pi"*) IS_PI=1;; esac
  case "$OS_ID" in debian|raspbian) IS_DEBIAN=1;; esac
  [ "$OS_ID" = "debian" ] && [ "$OS_CODENAME" = "trixie" ] && IS_DEBIAN_TRIXIE=1
  case "$DASH_ARCH" in x86_64|amd64|i386|i686) IS_X86=1;; esac
  if [ "$IS_PI" = "1" ]; then
    PLATFORM_LABEL="Raspberry Pi (${DEVICE_MODEL:-unknown model})"
  elif [ "$IS_DEBIAN_TRIXIE" = "1" ] && [ "$IS_X86" = "1" ]; then
    PLATFORM_LABEL="Debian Trixie x86 ($DASH_ARCH)"
  elif [ "$IS_DEBIAN" = "1" ] && [ "$IS_X86" = "1" ]; then
    PLATFORM_LABEL="Debian x86 ($OS_CODENAME $DASH_ARCH)"
  elif [ "$IS_X86" = "1" ]; then
    PLATFORM_LABEL="x86 Linux ($OS_ID $OS_CODENAME)"
  else
    PLATFORM_LABEL="Linux device ($OS_ID $OS_CODENAME, $DASH_ARCH)"
  fi
}

detect_xsession(){
  local f base
  for base in dashboard-openbox dashboard-lite openbox LXDE lxde xfce xfce4; do
    [ -f "/usr/share/xsessions/$base.desktop" ] && { echo "$base"; return 0; }
  done
  for f in /usr/share/xsessions/*.desktop; do
    [ -e "$f" ] || continue
    base="$(basename "$f" .desktop)"
    [ -n "$base" ] && { echo "$base"; return 0; }
  done
  echo "openbox"
}

lxsession_autostart_dir(){
  local sess="${1:-LXDE}"
  case "$sess" in
    dashboard-openbox|dashboard-lite) echo "";;
    openbox|Openbox) echo "$HOME/.config/openbox";;
    LXDE|lxde) echo "$HOME/.config/lxsession/$sess";;
    *) echo "$HOME/.config/lxsession/LXDE";;
  esac
}

mem_total_mb(){
  awk '/^MemTotal:/ {printf "%d\n", int(($2+1023)/1024); exit}' /proc/meminfo 2>/dev/null || echo 0
}

cpu_count(){
  local n
  n="$(getconf _NPROCESSORS_ONLN 2>/dev/null || nproc 2>/dev/null || echo 1)"
  printf '%s\n' "${n:-1}"
}

classify_device_profile(){
  local model arch mem cpu
  model="${DEVICE_MODEL:-$(read_device_model)}"
  arch="${DASH_ARCH:-$(uname -m 2>/dev/null || echo unknown)}"
  mem="$(mem_total_mb)"
  cpu="$(cpu_count)"
  case "$mem" in ''|*[!0-9]*) mem=0;; esac
  case "$cpu" in ''|*[!0-9]*) cpu=1;; esac

  case "$model" in
    *"Raspberry Pi Zero"*|*"Raspberry Pi Model A"*|*"Raspberry Pi Model B"*|*"Raspberry Pi 1"*|*"Raspberry Pi 2"*|*"Raspberry Pi 3 Model A"*|*"Raspberry Pi 3A"*) echo lite; return 0;;
    *"Raspberry Pi 3 Model B"*|*"Raspberry Pi 3B"*) echo balanced; return 0;;
  esac

  if [ "$mem" -gt 0 ] && [ "$mem" -lt 900 ]; then echo lite; return 0; fi
  if [ "$mem" -ge 4096 ] && [ "$cpu" -ge 4 ]; then
    case "$arch" in x86_64|amd64|aarch64|arm64) echo enhanced; return 0;; esac
  fi
  if [ "$mem" -ge 2250 ] && [ "$cpu" -ge 2 ]; then echo enhanced; return 0; fi
  if [ "$mem" -ge 900 ]; then echo balanced; return 0; fi
  echo balanced
}

dashboard_profile(){
  local p f
  p="${PROFILE:-}"
  if [ -z "$p" ]; then
    f="$CONFIG_DIR/config.local.js"
    if [ -f "$f" ]; then
      p="$(sed -nE 's/.*profile[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/p' "$f" | head -1 | tr '[:upper:]' '[:lower:]')"
    fi
  fi
  [ -n "$p" ] || p="$(classify_device_profile)"
  p="$(printf '%s\n' "$p" | tr '[:upper:]' '[:lower:]')"
  case "$p" in maximum) p="enhanced";; standard|default) p="balanced";; zero2|low|low-power) p="lite";; esac
  printf '%s\n' "$p"
}

profile_is_lite(){
  case "$(dashboard_profile)" in lite|zero2|low|low-power) return 0;; *) return 1;; esac
}

profile_prefers_openbox_session(){
  case "$(dashboard_profile)" in lite|zero2|low|low-power|balanced) return 0;; *) return 1;; esac
}

# LightDM reads vendor drop-ins, then /etc drop-ins, then lightdm.conf. Keep
# our readback in that order so diagnostics report the effective last value.
lightdm_config_value(){
  local key="$1" f value="" found root vendor
  root="${DASH_LIGHTDM_ROOT:-/etc/lightdm}"
  vendor="${DASH_LIGHTDM_VENDOR_DIR:-/usr/share/lightdm/lightdm.conf.d}"
  for f in "$vendor"/*.conf "$root"/lightdm.conf.d/*.conf "$root"/lightdm.conf; do
    [ -f "$f" ] || continue
    found="$(sed -nE "s/^[[:space:]]*${key}[[:space:]]*=[[:space:]]*(.*)$/\1/p" "$f" 2>/dev/null | tail -1)"
    [ -n "$found" ] && value="$found"
  done
  printf '%s\n' "$value"
}

lightdm_autologin_session(){ lightdm_config_value autologin-session; }
lightdm_autologin_user(){ lightdm_config_value autologin-user; }

valid_lightdm_user(){
  case "${1:-}" in ''|*[!A-Za-z0-9_.-]*) return 1;; *) return 0;; esac
}

lightdm_dashboard_xsession_ok(){
  local session_name="${1:-dashboard-openbox}" xs script
  xs="${DASH_XSESSIONS_DIR:-/usr/share/xsessions}/${session_name}.desktop"
  script="$BIN_DIR/dashboard-lite-session.sh"
  [ -x "$script" ] && [ -f "$xs" ] && grep -Fqx "Exec=$script" "$xs" 2>/dev/null
}

lightdm_dashboard_autologin_ready(){
  local expected_user="${1:-$USER_NAME}" expected_session="${2:-dashboard-openbox}"
  [ "$(lightdm_autologin_user)" = "$expected_user" ] || return 1
  [ "$(lightdm_autologin_session)" = "$expected_session" ] || return 1
  lightdm_dashboard_xsession_ok "$expected_session"
}

write_dashboard_openbox_xsession(){
  local dir script xs compat
  dir="${DASH_XSESSIONS_DIR:-/usr/share/xsessions}"
  xs="$dir/dashboard-openbox.desktop"
  compat="$dir/dashboard-lite.desktop"
  script="$BIN_DIR/dashboard-lite-session.sh"
  [ -x "$script" ] || { _dash_common_warn "dashboard-lite-session.sh is missing or not executable; cannot enable dashboard-openbox"; return 1; }
  $SUDO mkdir -p "$dir" || return 1
  cat <<EOFOPENBOX | $SUDO tee "$xs" >/dev/null || return 1
[Desktop Entry]
Name=Dash-Go Openbox
Comment=Minimal Dash-Go kiosk session using Openbox and Surf
Exec=$script
TryExec=$script
Type=Application
DesktopNames=DashGo
EOFOPENBOX
  $SUDO chown root:root "$xs" 2>/dev/null || true
  $SUDO chmod 0644 "$xs" 2>/dev/null || true
  cat <<EOFLITE | $SUDO tee "$compat" >/dev/null || return 1
[Desktop Entry]
Name=Dash-Go Lite
Comment=Compatibility alias for the Dash-Go Openbox session
Exec=$script
TryExec=$script
Type=Application
DesktopNames=DashGo
EOFLITE
  $SUDO chown root:root "$compat" 2>/dev/null || true
  $SUDO chmod 0644 "$compat" 2>/dev/null || true
  _dash_common_ok "Dash-Go Openbox X session installed ($xs; compatibility alias $compat)"
}

write_dashboard_lite_xsession(){ write_dashboard_openbox_xsession; }

# Keep boot-critical writes out of a storage-failure recovery path. Doctor calls
# this before any LightDM/X-session repair so a read-only or failing card cannot
# turn a recoverable configuration mismatch into a keyboardless login prompt.
dashboard_boot_config_write_safe(){
  local target="${1:-/etc/lightdm}" opts="" state="" level=""
  state="${CACHE_DIR:-$DASH/cache}/storage-wear-state.json"
  if [ -r "$state" ]; then
    level="$(sed -nE 's/.*"level"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/p' "$state" 2>/dev/null | head -1)"
    case "$level" in
      failing|failed)
        _dash_common_warn "storage health is failing; refusing boot-configuration rewrite until storage is repaired"
        return 1
        ;;
    esac
  fi
  if command -v findmnt >/dev/null 2>&1; then
    opts="$(findmnt -no OPTIONS -T "$target" 2>/dev/null || true)"
    case ",$opts," in *,ro,*|*,ro) _dash_common_warn "filesystem for $target is read-only; refusing boot-configuration rewrite"; return 1;; esac
  fi
  if [ -d "$target" ] && [ ! -w "$target" ] && [ "$(id -u 2>/dev/null || echo 1)" -eq 0 ]; then
    _dash_common_warn "$target is not writable; refusing boot-configuration rewrite"
    return 1
  fi
  return 0
}

# Keep the dashboard-owned autologin decision in one root-owned /etc drop-in.
# Older installers wrote these three keys into lightdm.conf, which is read
# after drop-ins and could silently override newer managed settings. Migrate
# those keys only when configuring dashboard autologin again.
write_dashboard_lightdm_autologin(){
  local user_name="$1" session_name="$2" root conf_dir conf root_conf conf_tmp root_tmp
  local root_backup conf_backup root_changed=0 had_conf=0
  valid_lightdm_user "$user_name" || { _dash_common_warn "invalid LightDM autologin user: $user_name"; return 1; }
  id "$user_name" >/dev/null 2>&1 || { _dash_common_warn "LightDM autologin user does not exist: $user_name"; return 1; }
  lightdm_dashboard_xsession_ok "$session_name" || { _dash_common_warn "Dash-Go X session '$session_name' is not ready; autologin was not changed"; return 1; }
  root="${DASH_LIGHTDM_ROOT:-/etc/lightdm}"
  conf_dir="$root/lightdm.conf.d"
  conf="$conf_dir/90-dash-go-autologin.conf"
  root_conf="$root/lightdm.conf"
  conf_tmp="$conf_dir/.90-dash-go-autologin.conf.tmp.$$"
  root_tmp="$root/.lightdm.conf.dash-go.tmp.$$"
  root_backup="$root_conf.dash-go-autologin.bak"
  conf_backup="$conf.dash-go-autologin.bak"
  dashboard_boot_config_write_safe "$root" || return 1
  $SUDO mkdir -p "$conf_dir" || return 1

  # Build and validate the managed drop-in before it can replace any existing
  # autologin source. A failed temporary write leaves the live configuration
  # untouched.
  if ! $SUDO tee "$conf_tmp" >/dev/null <<EOFLIGHTDM
# Managed by Dash-Go. This file owns only the dashboard autologin choice.
[Seat:*]
autologin-user=$user_name
autologin-user-timeout=0
autologin-session=$session_name
EOFLIGHTDM
  then
    $SUDO rm -f "$conf_tmp" 2>/dev/null || true
    _dash_common_warn "could not stage managed LightDM autologin drop-in; existing configuration was preserved"
    return 1
  fi
  if ! $SUDO grep -Fqx "autologin-user=$user_name" "$conf_tmp" || ! $SUDO grep -Fqx "autologin-session=$session_name" "$conf_tmp"; then
    $SUDO rm -f "$conf_tmp" 2>/dev/null || true
    _dash_common_warn "staged LightDM autologin drop-in did not validate; existing configuration was preserved"
    return 1
  fi
  if $SUDO test -f "$conf"; then
    had_conf=1
    $SUDO cp -p "$conf" "$conf_backup" || { $SUDO rm -f "$conf_tmp"; return 1; }
  fi
  if ! $SUDO mv -f "$conf_tmp" "$conf"; then
    $SUDO rm -f "$conf_tmp" 2>/dev/null || true
    _dash_common_warn "could not activate managed LightDM autologin drop-in; existing configuration was preserved"
    return 1
  fi
  $SUDO chown root:root "$conf" 2>/dev/null || true
  $SUDO chmod 0644 "$conf" 2>/dev/null || true

  # Only after the working drop-in exists do we migrate legacy main-config
  # keys. The main file is replaced atomically and has a rollback copy.
  if $SUDO test -f "$root_conf" && $SUDO grep -Eq '^[[:space:]]*autologin-(user|user-timeout|session)[[:space:]]*=' "$root_conf"; then
    if ! $SUDO cp -p "$root_conf" "$root_backup"; then
      # The managed drop-in was staged successfully, but the legacy file is
      # still the live authority. Restore the previous drop-in state as well
      # so this failed migration leaves no partially-applied ownership change.
      [ "$had_conf" = 1 ] && $SUDO cp -p "$conf_backup" "$conf" || $SUDO rm -f "$conf"
      _dash_common_warn "could not back up existing LightDM config; previous autologin configuration was preserved"
      return 1
    fi
    if ! $SUDO sh -c '
      sed \
        -e "/^[[:space:]]*autologin-user[[:space:]]*=/d" \
        -e "/^[[:space:]]*autologin-user-timeout[[:space:]]*=/d" \
        -e "/^[[:space:]]*autologin-session[[:space:]]*=/d" \
        "$1" > "$2"
    ' sh "$root_conf" "$root_tmp"; then
      [ "$had_conf" = 1 ] && $SUDO cp -p "$conf_backup" "$conf" || $SUDO rm -f "$conf"
      _dash_common_warn "could not stage LightDM legacy-key migration; previous autologin configuration was preserved"
      return 1
    fi
    if ! $SUDO mv -f "$root_tmp" "$root_conf"; then
      $SUDO rm -f "$root_tmp" 2>/dev/null || true
      [ "$had_conf" = 1 ] && $SUDO cp -p "$conf_backup" "$conf" || $SUDO rm -f "$conf"
      _dash_common_warn "could not commit LightDM legacy-key migration; previous autologin configuration was preserved"
      return 1
    fi
    root_changed=1
  fi

  $SUDO groupadd -f autologin 2>/dev/null || true
  $SUDO gpasswd -a "$user_name" autologin >/dev/null 2>&1 || true
  if lightdm_dashboard_autologin_ready "$user_name" "$session_name"; then
    _dash_common_ok "LightDM autologin verified for $user_name using '$session_name'"
    return 0
  fi

  # A failed read-back must leave a bootable prior configuration, not a stripped
  # main file plus an untrusted drop-in.
  if [ "$root_changed" = 1 ] && $SUDO test -f "$root_backup"; then
    $SUDO cp -p "$root_backup" "$root_conf" 2>/dev/null || true
  fi
  if [ "$had_conf" = 1 ] && $SUDO test -f "$conf_backup"; then
    $SUDO cp -p "$conf_backup" "$conf" 2>/dev/null || true
  else
    $SUDO rm -f "$conf" 2>/dev/null || true
  fi
  _dash_common_warn "autologin change failed verification and was rolled back; previous LightDM configuration was preserved"
  return 1
}

install_kiosk_no_logout_guard(){
  local logind_drop=/etc/systemd/logind.conf.d/10-dashboard-kiosk.conf
  mkdir -p "$HOME/.config/autostart"
  for app in light-locker xscreensaver xautolock xss-lock gnome-screensaver mate-screensaver cinnamon-screensaver lxlock; do
    cat > "$HOME/.config/autostart/$app.desktop" <<EOFLOCKER
[Desktop Entry]
Type=Application
Name=$app
Hidden=true
X-GNOME-Autostart-enabled=false
EOFLOCKER
  done
  if [ -x "$BIN_DIR/dashboard-session-guard.sh" ]; then
    DISPLAY="${DISPLAY:-:0}" XAUTHORITY="${XAUTHORITY:-$HOME/.Xauthority}" "$BIN_DIR/dashboard-session-guard.sh" apply >/dev/null 2>&1 || true
  fi
  $SUDO mkdir -p /etc/systemd/logind.conf.d || return 1
  cat <<EOFLOGIND | $SUDO tee "$logind_drop" >/dev/null || return 1
# Written by Dash-Go.
# Prevent a touch-only kiosk from idling to a lock/login state.
[Login]
IdleAction=ignore
IdleActionSec=0
HandleLidSwitch=ignore
HandleLidSwitchExternalPower=ignore
HandleLidSwitchDocked=ignore
EOFLOGIND
  $SUDO chmod 0644 "$logind_drop" 2>/dev/null || true
  _dash_common_ok "kiosk anti-lock/autologout guard installed"
  echo "    Takes full effect after reboot or systemd-logind restart; current X session was updated when possible."
}

sudoers_file_is_parsed(){
  local f base
  f="$1"; base="$(basename "$f")"
  [ -f "$f" ] || return 1
  case "$f" in
    /etc/sudoers) return 0;;
    /etc/sudoers.d/*)
      case "$base" in *.*|*~|README*) return 1;; esac
      return 0;;
  esac
  return 1
}

find_broad_nopasswd_files(){
  local f tmp user
  user="${USER_NAME:-$(id -un 2>/dev/null || whoami)}"
  for f in /etc/sudoers /etc/sudoers.d/*; do
    sudoers_file_is_parsed "$f" || continue
    tmp="$($SUDO awk -v user="$user" '
      $0 !~ /^[[:space:]]*#/ &&
      $0 ~ "^[[:space:]]*" user "[[:space:]]+ALL[[:space:]]*=" &&
      $0 ~ /NOPASSWD:[[:space:]]*ALL([[:space:]]|$|,)/ {print FILENAME; exit}
    ' "$f" 2>/dev/null || true)"
    [ -n "$tmp" ] && printf '%s\n' "$f"
  done
}

write_dashboard_scoped_sudoers(){
  local user_name reboot_file update_file
  user_name="${USER_NAME:-$(id -un 2>/dev/null || whoami)}"
  reboot_file=/etc/sudoers.d/010-dashboard-reboot
  update_file=/etc/sudoers.d/011-dashboard-system-update
  echo "$user_name ALL=(root) NOPASSWD: /sbin/reboot, /sbin/poweroff" | $SUDO tee "$reboot_file" >/dev/null || return 1
  $SUDO chmod 0440 "$reboot_file" || return 1
  echo "$user_name ALL=(root) NOPASSWD: /usr/bin/apt-get update, /usr/bin/apt-get -y upgrade" | $SUDO tee "$update_file" >/dev/null || return 1
  $SUDO chmod 0440 "$update_file" || return 1
  $SUDO visudo -cf /etc/sudoers >/dev/null 2>&1
}

write_scoped_reboot_sudoers(){
  local user f
  user="${USER_NAME:-$(id -un 2>/dev/null || whoami)}"
  f=/etc/sudoers.d/010-dashboard-reboot
  echo "$user ALL=(root) NOPASSWD: /sbin/reboot, /sbin/poweroff" | $SUDO tee "$f" >/dev/null || return 1
  $SUDO chmod 0440 "$f" || return 1
  $SUDO visudo -cf /etc/sudoers >/dev/null 2>&1
}

write_scoped_system_update_sudoers(){
  local user f
  user="${USER_NAME:-$(id -un 2>/dev/null || whoami)}"
  f=/etc/sudoers.d/011-dashboard-system-update
  echo "$user ALL=(root) NOPASSWD: /usr/bin/apt-get update, /usr/bin/apt-get -y upgrade" | $SUDO tee "$f" >/dev/null || return 1
  $SUDO chmod 0440 "$f" || return 1
  $SUDO visudo -cf /etc/sudoers >/dev/null 2>&1
}

# Canonical dashboard-managed cron lines.  Installer, Doctor, and repair use
# this one implementation so a successful repair cannot create a schedule that
# Doctor immediately considers noncanonical.  The optional nightly browser
# restart is preserved unless a caller explicitly requests on/off.
dashboard_cron_event_spec(){
  case "$(dashboard_profile)" in
    lite|zero2|low|low-power) printf '%s\n' '*/20 * * * *' ;;
    *) printf '%s\n' '*/10 * * * *' ;;
  esac
}

dashboard_cron_nightly_enabled(){
  local cron="${1:-}"
  [ -n "$cron" ] || cron="$(crontab -l 2>/dev/null || true)"
  printf '%s\n' "$cron" | grep -q 'dashboard-nightly-browser-restart'
}

dashboard_cron_owned_filter(){
  local source="${1:-}"
  awk -v bindir="$BIN_DIR" -v bin="$BIN_DIR/dashboard-control-server" -v dash="$DASH" '
    index($0, bindir "/update-holidays.sh") ||
    index($0, dash "/update-holidays.sh") ||
    index($0, bindir "/update-iss-passes.sh") ||
    index($0, dash "/update-iss-passes.sh") ||
    index($0, bindir "/gen-default-calendars.sh") ||
    index($0, dash "/gen-default-calendars.sh") ||
    index($0, bindir "/gen-sky-calendars.sh") ||
    index($0, dash "/gen-sky-calendars.sh") ||
    index($0, bindir "/gen-calendars.sh") ||
    index($0, dash "/gen-calendars.sh") ||
    index($0, bindir "/log-memory.sh") ||
    index($0, dash "/log-memory.sh") ||
    index($0, bindir "/dashboard-startup-memory.sh") ||
    index($0, dash "/dashboard-startup-memory.sh") ||
    index($0, bindir "/dashboard-housekeeping.sh") ||
    index($0, dash "/dashboard-housekeeping.sh") ||
    index($0, bindir "/dashboard-health-guard.sh") ||
    index($0, dash "/dashboard-health-guard.sh") ||
    index($0, bindir "/dashboard-lowprio.sh") ||
    index($0, bin " --gen-events-cache") ||
    index($0, bin " --update-message-feeds") ||
    index($0, "# dash-go-doctor") ||
    index($0, "dashboard-startup-memory") ||
    index($0, "dashboard-housekeeping") ||
    index($0, "dashboard-health-guard") ||
    index($0, "dashboard-nightly-browser-restart") { next }
    { print }
  ' "$source"
}

dashboard_cron_expected_lines(){
  local cron="${1:-}" nightly_mode="${2:-preserve}" lowprio cadence nightly=0
  lowprio="$BIN_DIR/dashboard-lowprio.sh"
  cadence="$(dashboard_cron_event_spec)"
  case "$nightly_mode" in
    on|yes|1) nightly=1 ;;
    off|no|0) nightly=0 ;;
    *) dashboard_cron_nightly_enabled "$cron" && nightly=1 ;;
  esac
  [ -x "$BIN_DIR/update-holidays.sh" ] && printf '0 4 1 * * %s/update-holidays.sh\n' "$BIN_DIR"
  [ -x "$BIN_DIR/update-iss-passes.sh" ] && printf '22 4 */3 * * %s/update-iss-passes.sh >/dev/null 2>&1\n' "$BIN_DIR"
  [ -x "$BIN_DIR/gen-default-calendars.sh" ] && printf '37 4 * * * %s %s/gen-default-calendars.sh >/dev/null 2>&1\n' "$lowprio" "$BIN_DIR"
  [ -x "$BIN_DIR/dashboard-control-server" ] && printf '17 5 * * * %s %s/dashboard-control-server --update-message-feeds >/dev/null 2>&1\n' "$lowprio" "$BIN_DIR"
  [ -x "$BIN_DIR/dashboard-control-server" ] && printf '%s %s %s/dashboard-control-server --gen-events-cache >/dev/null 2>&1\n' "$cadence" "$lowprio" "$BIN_DIR"
  [ -x "$BIN_DIR/dashboard-housekeeping.sh" ] && printf '7 3 * * * %s %s/dashboard-housekeeping.sh >/dev/null 2>&1\n' "$lowprio" "$BIN_DIR"
  [ -x "$BIN_DIR/dashboard-health-guard.sh" ] && printf '*/30 * * * * %s/dashboard-health-guard.sh >/dev/null 2>&1\n' "$BIN_DIR"
  [ "$nightly" = 1 ] && printf '%s\n' '55 1 * * * pkill -x surf >/dev/null 2>&1 # dashboard-nightly-browser-restart'
}

dashboard_cron_reconcile(){
  local nightly_mode="${1:-preserve}" existing tmp
  command -v crontab >/dev/null 2>&1 || return 1
  existing="$(mktemp)" || return 1
  tmp="$(mktemp)" || { rm -f "$existing"; return 1; }
  crontab -l 2>/dev/null > "$existing" || true
  dashboard_cron_owned_filter "$existing" > "$tmp" || { rm -f "$existing" "$tmp"; return 1; }
  dashboard_cron_expected_lines "$(cat "$existing")" "$nightly_mode" >> "$tmp"
  crontab "$tmp"
  local rc=$?
  rm -f "$existing" "$tmp"
  return "$rc"
}
