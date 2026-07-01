#!/usr/bin/env bash
# Dash-Go Doctor: actionable health checks and conservative common repairs.
set -u

SCRIPT_ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
DASH="${DASH:-$SCRIPT_ROOT}"
BIN_DIR="$DASH/bin"
# Use the same persisted clock-trust model as kiosk and health rollups.
[ -r "$BIN_DIR/dashboard-resilience-lib.sh" ] && . "$BIN_DIR/dashboard-resilience-lib.sh"
BIN="$BIN_DIR/dashboard-control-server"
CONFIG_DIR="$DASH/config"
CACHE_DIR="$DASH/cache"
LOG_DIR="$DASH/logs"
CAL_DIR="$DASH/calendars"
USER_NAME="$(id -un 2>/dev/null || printf '%s' "${USER:-dashboard}")"
SERVICE_FILE="${DOCTOR_SERVICE_FILE:-/etc/systemd/system/dashboard-server.service}"
XSESSION_FILE="${DOCTOR_XSESSION_FILE:-/usr/share/xsessions/dashboard-openbox.desktop}"
AUTLOGIN_FILE="${DOCTOR_AUTOLOGIN_FILE:-/etc/lightdm/lightdm.conf.d/90-dash-go-autologin.conf}"
STAMP="$(date '+%Y%m%d-%H%M%S' 2>/dev/null || printf unknown)"

MODE=quick
FIX=0
NO_PROMPT=0
PLAN_MODE=0
INTERACTIVE_PLAN=0
FIX_ONLY=""
SAFE_ONLY=0
ONLINE=0
VERBOSE=0
SKIP_SYSTEM="${DOCTOR_SKIP_SYSTEM:-0}"
FROM_API="${DASH_DOCTOR_FROM_API:-0}"
RC=0
OK_COUNT=0
WARN_COUNT=0
FAIL_COUNT=0
FIX_COUNT=0
ACTIONABLE=0
GO_RUNTIME_TRUSTED=1
# Quick scans collect section output lazily so healthy sections do not bury the
# few warnings a kiosk owner actually needs to read.
CURRENT_SECTION=""
SECTION_EMITTED=0
# A 0,0 weather cache is one root cause even when the cache body is also an
# upstream provider-error payload. Keep the diagnosis to one actionable item.
WEATHER_ZERO_CACHE=0
# Scheduler output stays friendly in the scan; detailed plan output retains the
# precise canonical cron line for an administrator who needs it.
CRON_PROBLEM=""
CRON_PROBLEM_DETAIL=""
DOCTOR_PLAN_LIB="$BIN_DIR/dashboard-doctor-plan.sh"

usage(){
  cat <<'EOF'
Usage: doctor.sh [--quick|--full] [--plan] [--fix] [--yes|--no-prompt] [--online]

  --quick      Compact local scan; warnings and required actions are shown.
  --full       Detailed scan including package checks, scheduler, logs, and system health.
  --plan       Full read-only scan followed by a human-readable repair plan.
  --fix        Review the repair plan first, then apply selected repairs.
  --yes        Apply all automatically eligible safe repairs without prompting.
  --no-prompt Do not offer an interactive repair plan (used by Dashboard Control).
  --online     Also validate the selected canonical GitHub Release without installing it.

Safe repairs make backups before replacing invalid user JSON. Guided or
administrator repairs are never selected by the all-safe option. Package damage,
missing OS packages, and unknown locations are reported with explicit commands
instead of being guessed or silently overwritten.
EOF
}

while [ $# -gt 0 ]; do
  case "$1" in
    --quick) MODE=quick ;;
    --full) MODE=full; VERBOSE=1 ;;
    --plan) PLAN_MODE=1; MODE=full ;;
    --interactive-plan) PLAN_MODE=1; MODE=full; INTERACTIVE_PLAN=1 ;;
    --fix) FIX=1; MODE=full ;;
    --yes|-y) FIX=1; MODE=full; NO_PROMPT=1; SAFE_ONLY=1 ;;
    --only) shift; [ $# -gt 0 ] || { printf '%s\n' '--only needs a comma-separated repair key list' >&2; exit 64; }; FIX_ONLY="$1" ;;
    --online) ONLINE=1; MODE=full ;;
    --no-prompt) NO_PROMPT=1 ;;
    --help|-h) usage; exit 0 ;;
    *) printf 'Unknown doctor option: %s\n' "$1" >&2; usage >&2; exit 64 ;;
  esac
  shift
done

if [ -r "$DOCTOR_PLAN_LIB" ]; then
  # shellcheck disable=SC1090
  . "$DOCTOR_PLAN_LIB"
else
  printf 'Doctor repair-plan helper is missing: %s\n' "$DOCTOR_PLAN_LIB" >&2
  exit 1
fi
# Cron policy is shared with installer repair. This prevents a repaired
# schedule from diverging from the exact cadence Doctor verifies.
if [ -r "$BIN_DIR/dashboard-common.sh" ]; then
  # shellcheck disable=SC1090
  . "$BIN_DIR/dashboard-common.sh"
fi
trap doctor_plan_cleanup EXIT INT TERM

# A terminal repair always begins with a read-only full plan. Dashboard Control
# passes --no-prompt and retains the explicit confirmed-repair behavior.
if [ "$FIX" -eq 1 ] && [ "$NO_PROMPT" -eq 0 ] && [ -z "$FIX_ONLY" ] && [ -t 0 ]; then
  exec "$0" --full --interactive-plan
fi
# A redirected --fix must never silently apply guided or administrator repairs.
# Non-interactive callers can use --only after reviewing exact repair keys;
# otherwise they receive the same conservative safe-only behavior as --yes.
if [ "$FIX" -eq 1 ] && [ -z "$FIX_ONLY" ] && [ ! -t 0 ]; then
  SAFE_ONLY=1
  NONINTERACTIVE_SAFE_FIX=1
else
  NONINTERACTIVE_SAFE_FIX=0
fi

section(){
  CURRENT_SECTION="$*"
  SECTION_EMITTED=0
  if [ "$VERBOSE" = 1 ]; then
    printf '\n== %s\n' "$CURRENT_SECTION"
    SECTION_EMITTED=1
  fi
}
section_emit(){
  [ "$SECTION_EMITTED" = 1 ] && return 0
  [ -n "$CURRENT_SECTION" ] || return 0
  printf '\n== %s\n' "$CURRENT_SECTION"
  SECTION_EMITTED=1
}
ok(){
  OK_COUNT=$((OK_COUNT+1))
  if [ "$VERBOSE" = 1 ]; then section_emit; printf 'OK  %s\n' "$*"; fi
}
info(){ section_emit; printf 'INFO %s\n' "$*"; }
if [ "${NONINTERACTIVE_SAFE_FIX:-0}" = 1 ]; then
  info "Non-interactive --fix is limited to safe repairs; use --only after reviewing a plan to select guided or administrator repairs."
fi
# Historical log-state notes are available in --full output, but are not a
# current finding and should not make a compact scan look unhealthy.
info_quiet(){ [ "$VERBOSE" = 1 ] && info "$@" || true; }
warn(){ WARN_COUNT=$((WARN_COUNT+1)); section_emit; printf 'WARN %s\n' "$*"; }
fail(){ FAIL_COUNT=$((FAIL_COUNT+1)); ACTIONABLE=$((ACTIONABLE+1)); RC=1; section_emit; printf 'FAIL %s\n' "$*"; }
# Warnings are suggestions, not the hard failures counted as action-required
# in the summary. This keeps the severity language internally consistent.
warn_fix(){ WARN_COUNT=$((WARN_COUNT+1)); ACTIONABLE=$((ACTIONABLE+1)); section_emit; printf 'WARN %s — Suggestion: %s\n' "$1" "$2"; }
fail_fix(){ FAIL_COUNT=$((FAIL_COUNT+1)); ACTIONABLE=$((ACTIONABLE+1)); RC=1; section_emit; printf 'FAIL %s — Fix: %s\n' "$1" "$2"; }
fixed(){ FIX_COUNT=$((FIX_COUNT+1)); section_emit; printf 'FIXED %s\n' "$*"; }

doctor_plan_class(){
  local key="$1"
  [ -f "${DOCTOR_PLAN_FILE:-}" ] || return 1
  awk -F '\t' -v wanted="$key" '$2 == wanted { print $1; exit }' "$DOCTOR_PLAN_FILE"
}

can_apply_fix(){
  local key="$1" class=""
  [ "$FIX" -eq 1 ] || return 1
  if [ -n "$FIX_ONLY" ]; then
    case ",$FIX_ONLY," in *,"$key",*) return 0 ;; esac
    return 1
  fi
  if [ "$SAFE_ONLY" -eq 1 ]; then
    class="$(doctor_plan_class "$key" 2>/dev/null || true)"
    [ "$class" = safe ] || return 1
  fi
  return 0
}
go_runtime_trusted(){ [ "${GO_RUNTIME_TRUSTED:-0}" = 1 ]; }

have(){ command -v "$1" >/dev/null 2>&1; }
trim_line(){ tr '\n' ' ' | sed 's/[[:space:]][[:space:]]*/ /g; s/^ //; s/ $//'; }
file_old_minutes(){ [ -e "$1" ] && find "$1" -mmin "+$2" -print -quit 2>/dev/null | grep -q .; }

run_root(){
  if [ "$(id -u 2>/dev/null || echo 1)" -eq 0 ]; then
    "$@"
  elif have sudo && sudo -n true >/dev/null 2>&1; then
    sudo -n "$@"
  elif [ -t 0 ] && have sudo; then
    sudo "$@"
  else
    return 1
  fi
}

json_ok(){
  [ -f "$1" ] || return 1
  if [ -x "$BIN" ] && go_runtime_trusted; then
    "$BIN" --json-validate "$1" >/dev/null 2>&1
  elif have python3; then
    python3 -m json.tool "$1" >/dev/null 2>&1
  else
    return 1
  fi
}

backup_file(){
  local path="$1" suffix="${2:-doctor-bad}"
  [ -f "$path" ] || return 0
  cp -p "$path" "$path.$suffix-$STAMP" 2>/dev/null
}

repair_json(){
  local path="$1" default_json="$2"
  mkdir -p "$(dirname "$path")" || return 1
  backup_file "$path" || return 1
  printf '%s\n' "$default_json" > "$path.tmp" || return 1
  mv "$path.tmp" "$path"
}

quarantine_cache(){
  local path="$1"
  [ -e "$path" ] || return 0
  mv "$path" "$path.doctor-bad-$STAMP"
}

http_status(){
  if have curl; then
    curl -fsS --max-time 5 http://127.0.0.1:8090/api/status 2>/dev/null
  elif have wget; then
    wget -qO- -T 5 http://127.0.0.1:8090/api/status 2>/dev/null
  else
    return 1
  fi
}

restart_server(){
  have systemctl || return 1
  run_root systemctl daemon-reload >/dev/null 2>&1 || true
  run_root systemctl enable dashboard-server.service >/dev/null 2>&1 || true
  run_root systemctl restart dashboard-server.service >/dev/null 2>&1
}

unit_section_has_setting(){
  local section="$1" setting="$2"
  [ -f "$SERVICE_FILE" ] || return 1
  awk -v section="$section" -v setting="$setting" '
    /^\[/ { inside=($0 == "[" section "]"); next }
    inside && $0 == setting { found=1 }
    END { exit(found ? 0 : 1) }
  ' "$SERVICE_FILE" 2>/dev/null
}

service_unit_matches(){
  [ -f "$SERVICE_FILE" ] || return 1
  unit_section_has_setting Unit "StartLimitIntervalSec=120" || return 1
  unit_section_has_setting Unit "StartLimitBurst=5" || return 1
  unit_section_has_setting Service "Type=simple" || return 1
  unit_section_has_setting Service "User=$USER_NAME" || return 1
  unit_section_has_setting Service "WorkingDirectory=$DASH" || return 1
  unit_section_has_setting Service "ExecStart=$BIN" || return 1
  unit_section_has_setting Service "Restart=always" || return 1
  unit_section_has_setting Service "RestartSec=3" || return 1
  unit_section_has_setting Install "WantedBy=multi-user.target" || return 1
}

service_unit_syntax_ok(){
  local path="$1"
  if have systemd-analyze; then
    systemd-analyze verify "$path" >/dev/null 2>&1
  else
    return 0
  fi
}

repair_service_unit(){
  local tmp backup
  mkdir -p "$CACHE_DIR" || return 1
  tmp="$CACHE_DIR/.doctor-dashboard-server.$$.service"
  backup="$SERVICE_FILE.doctor-bak-$STAMP"
  cat > "$tmp" <<EOF
[Unit]
Description=Dash-Go local web server
After=network.target
StartLimitIntervalSec=120
StartLimitBurst=5

[Service]
Type=simple
User=$USER_NAME
WorkingDirectory=$DASH
ExecStart=$BIN
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
EOF
  if ! service_unit_syntax_ok "$tmp"; then
    rm -f "$tmp"
    return 1
  fi
  if [ -f "$SERVICE_FILE" ] && ! run_root cp -p "$SERVICE_FILE" "$backup"; then
    rm -f "$tmp"
    return 1
  fi
  run_root install -m 0644 "$tmp" "$SERVICE_FILE" || { rm -f "$tmp"; return 1; }
  rm -f "$tmp"
  run_root systemctl daemon-reload >/dev/null 2>&1 || return 1
  run_root systemctl enable dashboard-server.service >/dev/null 2>&1 || return 1
  [ "$FROM_API" = 1 ] && return 0
  restart_server
}

doctor_profile(){
  local profile=""
  if [ -n "${DOCTOR_PROFILE:-}" ]; then
    profile="$DOCTOR_PROFILE"
  elif declare -F dashboard_profile >/dev/null 2>&1; then
    profile="$(dashboard_profile 2>/dev/null || true)"
  elif [ -f "$CONFIG_DIR/config.local.js" ]; then
    profile="$(sed -nE 's/.*profile[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/p' "$CONFIG_DIR/config.local.js" | head -1)"
  fi
  profile="$(printf '%s' "${profile:-balanced}" | tr '[:upper:]' '[:lower:]')"
  case "$profile" in zero2|low|low-power) profile=lite;; standard|default) profile=balanced;; maximum) profile=enhanced;; esac
  printf '%s\n' "$profile"
}

cron_event_spec(){
  if declare -F dashboard_cron_event_spec >/dev/null 2>&1; then
    dashboard_cron_event_spec
  else
    case "$(doctor_profile)" in lite) printf '%s\n' '*/20 * * * *' ;; *) printf '%s\n' '*/10 * * * *' ;; esac
  fi
}

cron_optional_nightly_enabled(){
  local cron="${1:-}"
  if declare -F dashboard_cron_nightly_enabled >/dev/null 2>&1; then
    dashboard_cron_nightly_enabled "$cron"
  else
    [ -n "$cron" ] || cron="$(crontab -l 2>/dev/null || true)"
    printf '%s\n' "$cron" | grep -q 'dashboard-nightly-browser-restart'
  fi
}

cron_expected_lines(){
  local cron="${1:-}"
  if declare -F dashboard_cron_expected_lines >/dev/null 2>&1; then
    dashboard_cron_expected_lines "$cron" preserve
  else
    local lowprio cadence
    lowprio="$BIN_DIR/dashboard-lowprio.sh"; cadence="$(cron_event_spec)"
    [ -x "$BIN_DIR/update-holidays.sh" ] && printf '0 4 1 * * %s/update-holidays.sh\n' "$BIN_DIR"
    [ -x "$BIN_DIR/update-iss-passes.sh" ] && printf '22 4 */3 * * %s/update-iss-passes.sh >/dev/null 2>&1\n' "$BIN_DIR"
    [ -x "$BIN_DIR/gen-default-calendars.sh" ] && printf '37 4 * * * %s %s/gen-default-calendars.sh >/dev/null 2>&1\n' "$lowprio" "$BIN_DIR"
    [ -x "$BIN" ] && printf '17 5 * * * %s %s --update-message-feeds >/dev/null 2>&1\n' "$lowprio" "$BIN"
    [ -x "$BIN" ] && printf '%s %s %s --gen-events-cache >/dev/null 2>&1\n' "$cadence" "$lowprio" "$BIN"
    [ -x "$BIN_DIR/dashboard-housekeeping.sh" ] && printf '7 3 * * * %s %s/dashboard-housekeeping.sh >/dev/null 2>&1\n' "$lowprio" "$BIN_DIR"
    [ -x "$BIN_DIR/dashboard-health-guard.sh" ] && printf '*/30 * * * * %s/dashboard-health-guard.sh >/dev/null 2>&1\n' "$BIN_DIR"
    cron_optional_nightly_enabled "$cron" && printf '%s\n' '55 1 * * * pkill -x surf >/dev/null 2>&1 # dashboard-nightly-browser-restart'
  fi
}

cron_problem_label(){
  case "$1" in
    *dashboard-housekeeping.sh*) printf '%s' 'housekeeping job' ;;
    *--gen-events-cache*) printf '%s' 'event-cache refresh' ;;
    *--update-message-feeds*) printf '%s' 'message-feed refresh' ;;
    *update-holidays.sh*) printf '%s' 'holiday refresh' ;;
    *update-iss-passes.sh*) printf '%s' 'ISS-pass refresh' ;;
    *gen-default-calendars.sh*) printf '%s' 'default-calendar refresh' ;;
    *dashboard-health-guard.sh*) printf '%s' 'health-guard job' ;;
    *dashboard-nightly-browser-restart*) printf '%s' 'nightly browser restart' ;;
    *) printf '%s' 'managed dashboard job' ;;
  esac
}

cron_schedule_status(){
  local cron="" line count expected_count owned_count what filtered
  CRON_PROBLEM=""
  CRON_PROBLEM_DETAIL=""
  cron="$(crontab -l 2>/dev/null || true)"
  while IFS= read -r line; do
    [ -n "$line" ] || continue
    count="$(printf '%s\n' "$cron" | grep -Fxc "$line" 2>/dev/null || true)"
    if [ "$count" -ne 1 ] 2>/dev/null; then
      what="$(cron_problem_label "$line")"
      if [ "$count" -eq 0 ] 2>/dev/null; then
        CRON_PROBLEM="$what is missing or uses a non-canonical schedule"
      else
        CRON_PROBLEM="$what is duplicated or uses a non-canonical schedule"
      fi
      CRON_PROBLEM_DETAIL="Expected exactly one canonical cron line: $line"
      return 1
    fi
  done < <(cron_expected_lines "$cron")
  expected_count="$(cron_expected_lines "$cron" | awk 'NF{n++} END{print n+0}')"
  if declare -F dashboard_cron_owned_filter >/dev/null 2>&1; then
    filtered="$(mktemp)" || return 1
    printf '%s\n' "$cron" > "$filtered"
    owned_count="$(( $(printf '%s\n' "$cron" | awk 'NF{n++} END{print n+0}') - $(dashboard_cron_owned_filter "$filtered" | awk 'NF{n++} END{print n+0}') ))"
    rm -f "$filtered"
  else
    owned_count="$(printf '%s\n' "$cron" | awk -v bindir="$BIN_DIR" -v bin="$BIN" -v dash="$DASH" '
      index($0, bindir "/update-holidays.sh") || index($0, dash "/update-holidays.sh") ||
      index($0, bindir "/update-iss-passes.sh") || index($0, dash "/update-iss-passes.sh") ||
      index($0, bindir "/gen-default-calendars.sh") || index($0, dash "/gen-default-calendars.sh") ||
      index($0, bindir "/gen-sky-calendars.sh") || index($0, dash "/gen-sky-calendars.sh") ||
      index($0, bindir "/gen-calendars.sh") || index($0, dash "/gen-calendars.sh") ||
      index($0, bindir "/log-memory.sh") || index($0, dash "/log-memory.sh") ||
      index($0, bindir "/dashboard-startup-memory.sh") || index($0, dash "/dashboard-startup-memory.sh") ||
      index($0, bindir "/dashboard-housekeeping.sh") || index($0, dash "/dashboard-housekeeping.sh") ||
      index($0, bindir "/dashboard-health-guard.sh") || index($0, dash "/dashboard-health-guard.sh") ||
      index($0, bindir "/dashboard-lowprio.sh") || index($0, bin " --gen-events-cache") ||
      index($0, bin " --update-message-feeds") || index($0, "# dash-go-doctor") ||
      index($0, "dashboard-startup-memory") || index($0, "dashboard-housekeeping") ||
      index($0, "dashboard-nightly-browser-restart") { n++ }
      END { print n+0 }
    ')"
  fi
  if [ "$owned_count" -ne "$expected_count" ] 2>/dev/null; then
    CRON_PROBLEM="duplicate, legacy, or unexpected Dash-Go scheduled jobs were found"
    CRON_PROBLEM_DETAIL="Expected $expected_count managed job(s), found $owned_count managed/legacy Dash-Go line(s)."
    return 1
  fi
  return 0
}

repair_cron(){
  if declare -F dashboard_cron_reconcile >/dev/null 2>&1; then
    dashboard_cron_reconcile preserve
    return $?
  fi
  return 1
}

cron_service_unit(){
  [ "$SKIP_SYSTEM" = 1 ] && return 1
  have systemctl || return 1
  systemctl list-unit-files --type=service --no-legend 2>/dev/null | awk '$1 == "cron.service" || $1 == "crond.service" {print $1; exit}'
}

check_cron_service(){
  local unit=""
  [ "$SKIP_SYSTEM" = 1 ] && return 0
  have systemctl || return 0
  unit="$(cron_service_unit || true)"
  if [ -z "$unit" ]; then
    warn_fix "cron service is unavailable; scheduled dashboard jobs cannot run" "sudo apt-get install -y cron"
  elif systemctl is-enabled --quiet "$unit" 2>/dev/null && systemctl is-active --quiet "$unit" 2>/dev/null; then
    ok "$unit is enabled and active"
  else
    doctor_plan_add admin cron-service "Enable the scheduler service" "Dash-Go scheduled jobs cannot run while $unit is disabled or inactive." "Enables and starts $unit so existing managed jobs can run." "Your cron entries and unrelated services."
    if can_apply_fix cron-service && run_root systemctl enable --now "$unit" >/dev/null 2>&1; then
      fixed "enabled and started $unit for Dash-Go scheduled jobs"
    else
      warn_fix "$unit is disabled or inactive" "sudo systemctl enable --now $unit"
    fi
  fi
}

COMMON_LOADED=0
if [ -r "$BIN_DIR/dashboard-common.sh" ]; then
  # shellcheck disable=SC1090
  . "$BIN_DIR/dashboard-common.sh"
  if declare -F lightdm_autologin_user >/dev/null 2>&1 \
    && declare -F lightdm_autologin_session >/dev/null 2>&1 \
    && declare -F lightdm_dashboard_xsession_ok >/dev/null 2>&1; then
    COMMON_LOADED=1
  fi
fi

check_installation(){
  local missing="" path rel selected manifest_version version verify_out
  section "Installation and Go runtime"

  for path in "$CONFIG_DIR" "$CACHE_DIR" "$LOG_DIR" "$CAL_DIR"; do
    if [ -d "$path" ] && [ -w "$path" ]; then
      ok "runtime directory writable: ${path#$DASH/}"
    else
      doctor_plan_add safe runtime-dirs "Restore writable Dash-Go runtime folders" "A required dashboard data folder is missing or not writable." "Creates the folder and restores owner read/write/execute access where permitted." "Application code and personal files already present."
      if can_apply_fix runtime-dirs && mkdir -p "$path" 2>/dev/null && chmod u+rwx "$path" 2>/dev/null; then
        fixed "created/repaired writable runtime directory: ${path#$DASH/}"
      else
        fail_fix "runtime directory missing or not writable: $path" "sudo chown -R $USER_NAME:$USER_NAME '$path'"
      fi
    fi
  done

  for rel in index.html VERSION manifest.json kiosk.sh \
    ui/dashboard.css ui/control-layout.css ui/js/app.bundle.js ui/js/app.control.bundle.js \
    bin/dashboard-control-server bin/dashboard-common.sh bin/dashboard-kiosk-lib.sh \
    bin/dashboard-lite-session.sh bin/dashboard-session-guard.sh bin/doctor.sh \
    bin/dashboard-doctor-plan.sh bin/dashboard-health-guard.sh; do
    [ -f "$DASH/$rel" ] || missing="$missing $rel"
  done
  if [ -z "$missing" ]; then ok "required Dash-Go application files are present"; else doctor_plan_add repair installer-repair "Refresh Dash-Go application files" "Required packaged files are missing." "Downloads and verifies the selected release, then restores missing app files in a staged repair." "Personal settings, calendars, caches, and logs."; fail_fix "missing application files:$missing" "run ~/install.sh --repair"; fi

  if [ -f "$DASH/VERSION" ]; then
    version="$(head -1 "$DASH/VERSION" 2>/dev/null | trim_line)"
    [ -n "$version" ] && ok "installed version: $version" || { doctor_plan_add repair installer-repair "Refresh Dash-Go application files" "The installed version marker is empty." "Downloads and verifies the selected release, then restores packaged app files." "Personal settings, calendars, caches, and logs."; fail_fix "VERSION is empty" "run ~/install.sh --repair"; }
  fi

  if [ -x "$BIN" ]; then
    if "$BIN" --json-validate "$DASH/manifest.json" >/dev/null 2>&1; then
      ok "Go architecture selector starts successfully"
    else
      doctor_plan_add safe executable-modes "Restore executable permissions" "The Dash-Go selector cannot start normally; file mode bits may be missing." "Restores executable mode only on packaged Dash-Go scripts and binaries." "File contents and personal data."
      if can_apply_fix executable-modes && chmod +x "$BIN_DIR"/dashboard-control-server* "$BIN_DIR"/*.sh "$DASH/kiosk.sh" 2>/dev/null && "$BIN" --json-validate "$DASH/manifest.json" >/dev/null 2>&1; then
        fixed "restored executable permissions for the selected Go binary"
      else
        doctor_plan_add repair installer-repair "Refresh Dash-Go application files" "The packaged Go selector cannot start the expected binary." "Downloads and validates the selected release, then replaces application files in a staged repair." "Personal settings, calendars, caches, and logs."
        fail_fix "Go selector cannot run the packaged binary for $(uname -m 2>/dev/null || echo unknown)" "run ~/install.sh --repair"
      fi
    fi
  else
    doctor_plan_add safe executable-modes "Restore executable permissions" "The Dash-Go selector is missing executable mode bits." "Restores executable mode on packaged Dash-Go scripts and binaries where the files are present." "File contents and personal data."
    if can_apply_fix executable-modes && chmod +x "$BIN_DIR"/dashboard-control-server* "$BIN_DIR"/*.sh "$DASH/kiosk.sh" 2>/dev/null && [ -x "$BIN" ]; then
      fixed "restored executable permissions for Dash-Go scripts and binaries"
    else
      doctor_plan_add repair installer-repair "Refresh Dash-Go application files" "The Dash-Go selector is missing or cannot be made executable." "Downloads and validates the selected release, then replaces application files in a staged repair." "Personal settings, calendars, caches, and logs."
      fail_fix "Go architecture selector is missing or not executable" "run ~/install.sh --repair"
    fi
  fi

  if [ -x "$BIN" ] && [ -f "$DASH/manifest.json" ]; then
    manifest_version="$("$BIN" --json-get "$DASH/manifest.json" version '' 2>/dev/null | tr -d '"\r\n')"
    version="$(head -1 "$DASH/VERSION" 2>/dev/null | tr -d '\r\n')"
    if [ -n "$manifest_version" ] && [ "$manifest_version" = "$version" ]; then ok "VERSION and application manifest agree"; else doctor_plan_add repair installer-repair "Refresh Dash-Go application files" "VERSION and the packaged application manifest disagree." "Downloads and verifies the selected release, then restores a matched application set." "Personal settings, calendars, caches, and logs."; fail_fix "VERSION '$version' differs from manifest '$manifest_version'" "run ~/install.sh --repair"; fi
  fi

  if [ -x "$BIN" ]; then
    if verify_out="$("$BIN" --verify-generated-assets 2>&1)"; then
      ok "generated JavaScript and CSS assets match their split sources"
    else
      doctor_plan_add safe generated-assets "Regenerate browser bundles" "Generated JavaScript or CSS does not match its split source files." "Rebuilds only generated browser bundles from packaged split sources." "Your settings, calendars, messages, and source files."
      if go_runtime_trusted && can_apply_fix generated-assets && "$BIN" --verify-generated-assets --write >/dev/null 2>&1 && "$BIN" --verify-generated-assets >/dev/null 2>&1; then
        fixed "regenerated stale JavaScript and CSS browser assets"
      else
        fail_fix "generated browser assets are stale: $(printf '%s' "$verify_out" | trim_line)" "'$BIN' --verify-generated-assets --write"
      fi
    fi
  fi

  if [ "$MODE" = full ] && have python3 && [ -f "$DASH/manifest.json" ]; then
    local report manifest_rc
    report="$(python3 - "$DASH/manifest.json" "$DASH" <<'PY'
import hashlib, json, os, sys
manifest, root = sys.argv[1:]
try:
    data=json.load(open(manifest, encoding='utf-8'))
except Exception as exc:
    print('manifest unreadable:', exc); raise SystemExit(2)
bad=[]
for item in data.get('files', []):
    rel=item.get('path',''); path=os.path.join(root,rel)
    expected=str(item.get('sha256') or '')
    expected_size=item.get('size')
    if not rel or not os.path.isfile(path):
        bad.append(f"{rel or '<unnamed>'}: missing (expected sha256={expected[:12]}… size={expected_size})")
        continue
    actual_size=os.path.getsize(path)
    digest=hashlib.sha256()
    with open(path, 'rb') as fh:
        for chunk in iter(lambda: fh.read(1024 * 1024), b''):
            digest.update(chunk)
    actual=digest.hexdigest()
    if actual != expected or (isinstance(expected_size, int) and actual_size != expected_size):
        bad.append(f"{rel}: changed (expected sha256={expected[:12]}… size={expected_size}; found sha256={actual[:12]}… size={actual_size})")
print('; '.join(bad[:4]))
raise SystemExit(1 if bad else 0)
PY
)"; manifest_rc=$?
    if [ "$manifest_rc" -eq 0 ]; then
      ok "all packaged files match manifest checksums"
    else
      GO_RUNTIME_TRUSTED=0
      doctor_plan_add repair installer-repair "Refresh Dash-Go application files" "One or more packaged files differ from the release manifest." "Downloads and verifies the selected release, then stages a repair instead of overwriting code directly." "Personal settings, calendars, caches, and logs."
      fail_fix "packaged file integrity failed: ${report:-unknown mismatch}; later Go-backed repairs are skipped in this run" "run ~/install.sh --repair"
    fi
  fi
}

check_configuration(){
  local spec rel default path loc
  section "Configuration"

  while IFS='|' read -r rel default; do
    [ -n "$rel" ] || continue
    path="$CONFIG_DIR/$rel"
    if [ ! -e "$path" ]; then
      info "optional config absent: config/$rel"
    elif json_ok "$path"; then
      ok "valid JSON: config/$rel"
    else
      doctor_plan_add safe config-json "Back up and reset invalid Dash-Go settings JSON" "One or more optional dashboard JSON settings files are malformed." "Creates a timestamped backup and replaces only invalid files with safe empty defaults." "Valid settings, calendars, API keys outside these files, and all code."
      if can_apply_fix config-json && repair_json "$path" "$default"; then
        fixed "backed up and reset invalid config/$rel"
      else
        fail_fix "invalid JSON: config/$rel" "back it up, then repair it in Dashboard Control or run doctor.sh --fix"
      fi
    fi
  done <<'EOF'
settings.json|{}
compliments.json|{"version":4,"messages":[]}
message-sources.json|{}
message-cache-overrides.json|{}
temp-messages.json|[]
scheduled-messages.json|[]
chalkboard.json|{"version":1,"strokes":[]}
map-provider.json|{}
EOF

  if [ -f "$CONFIG_DIR/config.local.js" ] && [ -x "$BIN" ] && go_runtime_trusted; then
    loc="$("$BIN" --doctor-config --location-check 2>/dev/null || true)"
    if printf '%s\n' "$loc" | grep -q '^SYNTAX_KNOWN:'; then
      doctor_plan_add safe config-local "Repair the known config.local.js syntax regression" "Dashboard configuration has the exact recognized pause/profile syntax error." "Creates a timestamped backup and repairs only the known syntax pattern." "Location, weather keys, and unrelated configuration."
      if go_runtime_trusted && can_apply_fix config-local && "$BIN" --doctor-config --repair >/dev/null 2>&1; then fixed "repaired known config.local.js syntax error with a timestamped backup"; else fail_fix "config.local.js has a known syntax error" "'$BIN' --doctor-config --repair"; fi
    else
      ok "config.local.js has no known syntax regression"
    fi
    if printf '%s\n' "$loc" | grep -q '^LOCATION_OK:' && ! printf '%s\n' "$loc" | grep -q '^LOCATION_ZERO'; then
      ok "dashboard location coordinates are configured"
    elif printf '%s\n' "$loc" | grep -q '^LOCATION_ZERO\|^LOCATION_MISSING'; then
      warn_fix "dashboard location is missing or 0,0; weather and maps may be wrong" "set Location in Dashboard Control"
    fi
    if printf '%s\n' "$loc" | grep -q '^WEATHERAPI_ZERO_CACHE'; then
      WEATHER_ZERO_CACHE=1
      doctor_plan_add safe weather-zero-cache "Clear the invalid 0,0 weather cache" "Location is configured, but an old derived weather response was made for 0,0 coordinates." "Moves only the stale weather cache aside so the next normal refresh can replace it." "Location settings, weather keys, and source configuration."
      if can_apply_fix weather-zero-cache && quarantine_cache "$CACHE_DIR/weather-cache.json"; then fixed "removed cached 0,0 weather response so it can refresh"; else warn_fix "weather cache contains 0,0 coordinates" "run '$BIN_DIR/doctor.sh' --fix"; fi
    fi
  elif [ ! -f "$CONFIG_DIR/config.local.js" ]; then
    warn "config.local.js is absent; packaged defaults are active until setup is completed"
  elif ! go_runtime_trusted; then
    warn "skipped Go-backed config.local.js checks after the packaged integrity failure"
  fi

  for path in "$HOME/.dashboard-update.env" "$HOME/.dashboard-update-profile.json" "$HOME/.dashboard-weather.env" "$HOME/.dashboard-message.env" "$HOME/.dashboard-radar.env" "$HOME/.dashboard-control.env"; do
    [ -f "$path" ] || continue
    local mode
    mode="$(stat -c '%a' "$path" 2>/dev/null || echo unknown)"
    case "$mode" in 600|400) ok "private permissions on $(basename "$path")";;
      *) doctor_plan_add safe private-permissions "Restrict Dash-Go credential file permissions" "A saved update profile, weather, message, radar, or control credential file is readable by more than its owner." "Sets affected credential files to owner-only mode 600." "File contents and all other permissions."; if can_apply_fix private-permissions && chmod 600 "$path"; then fixed "secured $(basename "$path") to mode 600"; else warn_fix "$(basename "$path") permissions are $mode" "chmod 600 '$path'"; fi;;
    esac
  done

  # Radar is on-demand, so Doctor only verifies its saved configuration and
  # credential state. It deliberately does not fetch tile frames or wake a
  # metered provider during a health check.
  if [ -f "$CONFIG_DIR/config.local.js" ]; then
    radar_provider="$(sed -nE 's/.*radarProvider[[:space:]]*:[[:space:]]*"?([^",} ]+)"?.*/\1/p' "$CONFIG_DIR/config.local.js" 2>/dev/null | head -1)"
    radar_provider="${radar_provider:-rainviewer}"
    case "$radar_provider" in
      rainviewer|nws|custom_xyz) ok "radar provider $radar_provider configured";;
      tomorrow)
        if grep -qE '^DASH_RADAR_TOMORROW_KEY=.+$' "$HOME/.dashboard-radar.env" 2>/dev/null; then ok "radar provider tomorrow configured"; else warn "radar provider tomorrow needs an API key"; fi;;
      weatherbit)
        if grep -qE '^DASH_RADAR_WEATHERBIT_KEY=.+$' "$HOME/.dashboard-radar.env" 2>/dev/null; then ok "radar provider weatherbit configured"; else warn "radar provider weatherbit needs an API key"; fi;;
      xweather)
        if grep -qE '^DASH_RADAR_XWEATHER_ID=.+$' "$HOME/.dashboard-radar.env" 2>/dev/null && grep -qE '^DASH_RADAR_XWEATHER_SECRET=.+$' "$HOME/.dashboard-radar.env" 2>/dev/null; then ok "radar provider xweather configured"; else warn "radar provider xweather needs API credentials"; fi;;
      *) warn "radar provider $radar_provider is not recognized";;
    esac
  fi
}

doctor_data_report(){
  [ -x "$BIN" ] && go_runtime_trusted || return 1
  "$BIN" --doctor-data 2>/dev/null
}

doctor_data_has(){ printf '%s\n' "$1" | grep -qE "^$2"; }

check_data(){
  local path data_report event_issue=0 weather_issue=0 message_issue=0
  section "Calendars, messages, and caches"

  if ! go_runtime_trusted; then
    warn "skipped Go-backed calendar/cache validation and repair after the packaged integrity failure"
    return
  fi

  path="$CAL_DIR/calendars.json"
  if json_ok "$path"; then
    ok "calendar manifest is valid JSON"
  else
    doctor_plan_add safe calendar-manifest "Regenerate the derived calendar manifest" "calendars/calendars.json is missing or invalid." "Rebuilds the generated manifest from configured calendar inputs." "Your source ICS files and personal calendar settings."
    if go_runtime_trusted && can_apply_fix calendar-manifest && [ -x "$BIN" ] && "$BIN" --gen-calendars >/dev/null 2>&1 && json_ok "$path"; then
      fixed "regenerated calendars/calendars.json"
    else
      fail_fix "calendar manifest is missing or invalid" "'$BIN' --gen-calendars"
    fi
  fi

  data_report="$(doctor_data_report 2>&1 || true)"
  if doctor_data_has "$data_report" 'DASHBOARD_FONTS_MISSING'; then
    doctor_plan_add repair dashboard-fonts "Restore missing dashboard display fonts" "One or more bundled Dash-Go webfonts are missing, so the kiosk is using its readable system fallback." "Downloads and validates only the five required dashboard font files into ui/fonts." "Dashboard settings, calendars, downloaded optional fonts, and all unrelated user files."
    warn_fix "dashboard display fonts are missing; the kiosk is using fallback fonts" "run ~/install.sh --repair --system"
  elif doctor_data_has "$data_report" 'DASHBOARD_FONTS_OK'; then
    ok "dashboard display fonts are present"
  fi
  if doctor_data_has "$data_report" 'CALENDAR_MANIFEST_(ABSENT|INVALID)|CALENDAR_SOURCE_MISSING'; then
    doctor_plan_add guided calendar-sources "Rebuild generated calendar data after a missing source" "The derived calendar manifest references a local calendar source that is missing or invalid." "Regenerates the manifest and event cache from current configured sources." "Existing source ICS files and all unrelated calendars."
    if go_runtime_trusted && can_apply_fix calendar-sources && "$BIN" --gen-calendars >/dev/null 2>&1 && "$BIN" --gen-events-cache >/dev/null 2>&1; then
      fixed "rebuilt calendar manifest and event cache after invalid or missing calendar sources"
      data_report="$(doctor_data_report 2>&1 || true)"
    else
      warn_fix "calendar manifest references an invalid or missing source" "run '$BIN' --gen-calendars, then '$BIN' --gen-events-cache"
    fi
  elif doctor_data_has "$data_report" 'CALENDAR_MANIFEST_OK'; then
    ok "calendar manifest source entries are readable"
  elif [ -x "$BIN" ]; then
    warn "could not obtain semantic calendar-data status from the Go runtime"
  fi

  path="$CACHE_DIR/events.cache.json"
  if doctor_data_has "$data_report" 'EVENT_CACHE_(ABSENT|INVALID)'; then
    event_issue=1
  elif ! json_ok "$path"; then
    event_issue=1
  fi
  if [ "$event_issue" -eq 1 ]; then
    doctor_plan_add safe event-cache "Rebuild the derived event cache" "The event cache is missing, malformed, or uses an old schema." "Moves an invalid derived cache aside and regenerates it from current calendar data." "Source calendars and dashboard settings."
    if go_runtime_trusted && can_apply_fix event-cache && { [ ! -e "$path" ] || quarantine_cache "$path"; } && "$BIN" --gen-events-cache >/dev/null 2>&1 && json_ok "$path"; then
      fixed "quarantined and rebuilt the invalid event cache"
      data_report="$(doctor_data_report 2>&1 || true)"
    else
      fail_fix "event cache is missing, invalid, or uses an old schema" "'$BIN' --gen-events-cache"
    fi
  elif file_old_minutes "$path" 360; then
    doctor_plan_add safe event-cache-stale "Refresh an old event cache" "The generated event cache is more than six hours old." "Regenerates only derived event data from current calendars." "Source calendars and dashboard settings."
    if go_runtime_trusted && can_apply_fix event-cache-stale && "$BIN" --gen-events-cache >/dev/null 2>&1; then fixed "rebuilt event cache older than six hours"; else warn_fix "event cache is older than six hours" "'$BIN' --gen-events-cache"; fi
  else
    ok "event cache is valid, current, and uses the expected schema"
  fi

  path="$CACHE_DIR/weather-cache.json"
  if doctor_data_has "$data_report" 'WEATHER_CACHE_ABSENT'; then
    info "derived cache absent: weather-cache.json (it will be fetched when needed)"
  elif doctor_data_has "$data_report" 'WEATHER_CACHE_INVALID'; then
    weather_issue=1
  elif doctor_data_has "$data_report" 'WEATHER_CACHE_(DAY_STALE|LOCATION_MISMATCH|KEY_MISMATCH)'; then
    weather_issue=2
  elif doctor_data_has "$data_report" 'WEATHER_CACHE_ERROR_PAYLOAD'; then
    # The configuration check above already gives the single actionable
    # diagnosis for a 0,0-derived cache. Do not count its provider-error body
    # as a second unrelated warning.
    if [ "$WEATHER_ZERO_CACHE" -ne 1 ]; then
      warn "last weather cache is a provider-error response; the next normal refresh will retry"
    fi
  elif doctor_data_has "$data_report" 'WEATHER_CACHE_OK'; then
    ok "weather cache matches its current location, settings, and local day"
  elif [ -e "$path" ]; then
    warn "could not obtain semantic weather-cache status from the Go runtime"
  fi
  if [ "$weather_issue" -gt 0 ]; then
    doctor_plan_add safe weather-cache "Clear an outdated derived weather cache" "The stored weather response is malformed or no longer matches today, the active location, or weather settings." "Moves only the derived weather cache aside so the next normal refresh replaces it." "Location settings, weather keys, and source configuration."
    if can_apply_fix weather-cache && quarantine_cache "$path"; then
      fixed "cleared derived weather cache that no longer matches current day, location, or settings"
    elif [ "$weather_issue" -eq 1 ]; then
      warn_fix "weather cache has an invalid payload schema" "move '$path' aside; it will be fetched again when needed"
    else
      warn_fix "weather cache no longer matches the current day, location, or settings" "remove '$path' to force a clean refresh"
    fi
  fi

  path="$CONFIG_DIR/message-cache.json"
  if doctor_data_has "$data_report" 'MESSAGE_CACHE_ABSENT'; then
    info "derived cache absent: config/message-cache.json (it will refresh on the next scheduled feed update)"
  elif doctor_data_has "$data_report" 'MESSAGE_CACHE_INVALID'; then
    message_issue=1
  elif doctor_data_has "$data_report" 'MESSAGE_CACHE_OK'; then
    ok "generated message cache has the expected schema"
  elif [ -e "$path" ]; then
    warn "could not obtain semantic message-cache status from the Go runtime"
  fi
  if [ "$message_issue" -eq 1 ]; then
    doctor_plan_add safe message-cache "Quarantine an invalid generated message cache" "The generated message cache does not have the expected structure." "Moves only the derived cache aside; the next scheduled feed refresh rebuilds it." "Message sources, temporary messages, schedules, and settings."
    if can_apply_fix message-cache && quarantine_cache "$path"; then
      fixed "quarantined invalid generated message cache; the next scheduled refresh will rebuild it"
    else
      warn_fix "generated message cache has an invalid schema" "move '$path' aside; it will be rebuilt by the next feed refresh"
    fi
  fi

  if [ "$MODE" = full ]; then
    if have crontab; then
      if cron_schedule_status; then
        ok "canonical Dash-Go scheduled jobs are installed ($(doctor_profile) event cadence $(cron_event_spec))"
      else
        doctor_plan_add safe scheduler "Restore canonical Dash-Go scheduled jobs" "${CRON_PROBLEM:-Managed jobs need reconciliation.}" "Restores calendar, message, event-cache, holiday, housekeeping, and health-guard jobs for the active profile." "All unrelated personal cron entries and an existing nightly Surf restart." "${CRON_PROBLEM_DETAIL:-}"
        if can_apply_fix scheduler && repair_cron && cron_schedule_status; then
          fixed "restored canonical scheduled events, messages, calendars, holidays, housekeeping, and health guard"
        else
          warn_fix "scheduled Dash-Go jobs need attention: ${CRON_PROBLEM:-unknown}" "run '$BIN_DIR/doctor.sh' --fix"
        fi
      fi
      check_cron_service
    else
      warn_fix "crontab is not installed; caches will not refresh on schedule" "sudo apt-get install -y cron"
    fi
  fi
}

check_server(){
  local status="" owners="" verify_out=""
  section "Go server and local API"
  [ "$SKIP_SYSTEM" = 1 ] && { info "system service checks skipped by DOCTOR_SKIP_SYSTEM"; return; }

  if ! have systemctl; then
    fail_fix "systemctl is unavailable; dashboard-server cannot be managed" "install systemd or run the supported Debian/Raspberry Pi OS image"
    return
  fi

  if service_unit_matches; then
    ok "dashboard-server.service has the expected Go runtime and restart policy"
  else
    doctor_plan_add admin service-unit "Restore the managed dashboard-server service unit" "The systemd unit is missing or differs from the supported Dash-Go service contract." "Backs up the existing unit, validates the replacement, reloads systemd, and enables it at boot." "Dashboard settings, calendars, and user-owned files."
    if can_apply_fix service-unit && repair_service_unit; then
      if [ "$FROM_API" = 1 ]; then
        fixed "installed the correct dashboard-server.service, enabled it for boot, and deferred restart while Dashboard Control returns this result"
      else
        fixed "installed the correct dashboard-server.service, enabled it for boot, and restarted it"
      fi
    else
      fail_fix "dashboard-server.service is missing or differs from the supported Go unit" "run doctor.sh --fix from a sudo-capable terminal, or installer System pieces"
    fi
  fi

  if [ "$MODE" = full ] && [ -f "$SERVICE_FILE" ] && have systemd-analyze; then
    if service_unit_syntax_ok "$SERVICE_FILE"; then
      ok "dashboard-server.service passes systemd unit syntax verification"
    else
      verify_out="$(systemd-analyze verify "$SERVICE_FILE" 2>&1 | tail -4 | trim_line || true)"
      warn_fix "dashboard-server.service has a systemd syntax or load problem${verify_out:+: $verify_out}" "run doctor.sh --fix from a sudo-capable terminal"
    fi
  fi

  if systemctl is-enabled --quiet dashboard-server.service 2>/dev/null; then
    ok "dashboard-server.service is enabled at boot"
  else
    doctor_plan_add admin service-enable "Enable dashboard-server at boot" "The dashboard service is not enabled for startup." "Enables only dashboard-server.service at boot." "Other system services and configuration."
    if can_apply_fix service-enable && run_root systemctl enable dashboard-server.service >/dev/null 2>&1; then
      fixed "enabled dashboard-server.service at boot"
    else
      warn_fix "dashboard-server.service is not enabled at boot" "sudo systemctl enable dashboard-server.service"
    fi
  fi

  if systemctl is-active --quiet dashboard-server.service 2>/dev/null; then
    ok "dashboard-server.service is active"
  else
    doctor_plan_add admin service-restart "Restart the inactive Dash-Go server" "dashboard-server.service is not active." "Restarts only the managed Dash-Go local server after reloading its unit." "Kiosk browser session and personal dashboard data."
    if can_apply_fix service-restart && [ "$FROM_API" != 1 ] && restart_server && systemctl is-active --quiet dashboard-server.service 2>/dev/null; then
      fixed "enabled and restarted dashboard-server.service"
    elif can_apply_fix service-restart && [ "$FROM_API" = 1 ]; then
      warn_fix "dashboard-server.service is not active; restart is deferred during a Dashboard Control repair" "restart from a sudo-capable terminal: sudo systemctl restart dashboard-server"
    else
      fail_fix "dashboard-server.service is not active" "sudo systemctl restart dashboard-server; sudo journalctl -u dashboard-server -n 80 --no-pager"
    fi
  fi

  status="$(http_status || true)"
  if printf '%s' "$status" | grep -q '"goServer"[[:space:]]*:[[:space:]]*true'; then
    ok "Go API answers on http://127.0.0.1:8090"
  else
    doctor_plan_add admin api-restart "Restore the local Dash-Go API" "The service is running but the local Go API did not answer on port 8090." "Restarts only dashboard-server.service and rechecks the loopback API." "Kiosk browser session and personal dashboard data."
    if can_apply_fix api-restart && [ "$FROM_API" != 1 ] && restart_server && sleep 2 && status="$(http_status || true)" && printf '%s' "$status" | grep -q '"goServer"[[:space:]]*:[[:space:]]*true'; then
      fixed "restarted the service and restored the local Go API"
    else
      if have ss; then owners="$(ss -ltnp 2>/dev/null | awk '$4 ~ /:8090$/ {print}' | trim_line)"; fi
      if can_apply_fix api-restart && [ "$FROM_API" = 1 ]; then
        warn_fix "Go API did not answer on port 8090${owners:+; listener: $owners}; restart is deferred during this Dashboard Control repair" "restart from a sudo-capable terminal after this result is displayed"
      else
        fail_fix "Go API did not answer on port 8090${owners:+; listener: $owners}" "sudo systemctl restart dashboard-server and inspect its journal"
      fi
    fi
  fi

  if systemctl --user is-active --quiet dashboard-server.service 2>/dev/null && systemctl is-active --quiet dashboard-server.service 2>/dev/null; then
    doctor_plan_add guided duplicate-user-service "Disable a duplicate per-user dashboard service" "Both system and per-user dashboard-server services are active." "Stops and disables only the duplicate per-user service." "The managed system dashboard-server service and dashboard data."
    if can_apply_fix duplicate-user-service && systemctl --user disable --now dashboard-server.service >/dev/null 2>&1; then
      fixed "disabled duplicate per-user dashboard-server service"
    else
      warn_fix "system and per-user dashboard-server services are both active" "systemctl --user disable --now dashboard-server.service"
    fi
  fi

  if [ "$MODE" = full ] && have journalctl; then
    local errors
    errors="$(journalctl -b -u dashboard-server.service -n 120 --no-pager 2>/dev/null | grep -Ei 'panic|fatal|address already in use|permission denied|segmentation|failed' | grep -Evi 'sudo.*pam_unix|pam_unix.*conversation failed|sudo:auth' | tail -5 | trim_line || true)"
    [ -z "$errors" ] && ok "current-boot dashboard-server journal has no common fatal signatures" || warn "current-boot dashboard-server journal needs review: $errors"
  fi
}

latest_log_session(){
  # Read only the portion after the most recent session marker. Old logs are
  # preserved for --history/manual review but must not make a healthy boot look
  # broken today.
  local path="$1" marker="$2"
  [ -f "$path" ] || return 0
  awk -v marker="$marker" '
    index($0, marker) { seen=1; body=$0 ORS; next }
    seen { body=body $0 ORS }
    END { if (seen) printf "%s", body }
  ' "$path" 2>/dev/null
}

legacy_kiosk_autostart_files(){
  local path
  for path in "$HOME/.config/openbox/autostart" "$HOME/.xsession" "$HOME/.xinitrc"; do
    [ -f "$path" ] && grep -Fq "$DASH/kiosk.sh" "$path" 2>/dev/null && printf '%s\n' "$path"
  done
  while IFS= read -r path; do
    grep -Fq "$DASH/kiosk.sh" "$path" 2>/dev/null && printf '%s\n' "$path"
  done < <(find "$HOME/.config/lxsession" "$HOME/.config/autostart" -type f \( -name autostart -o -name '*.desktop' \) -print 2>/dev/null || true)
}

disable_legacy_kiosk_autostarts(){
  local path backup tmp changed=0
  while IFS= read -r path; do
    [ -n "$path" ] || continue
    backup="$path.doctor-backup-$STAMP"
    cp -p "$path" "$backup" 2>/dev/null || return 1
    tmp="${path}.doctor-tmp-$$"
    if [[ "$path" = *.desktop ]]; then
      cp -p "$path" "$tmp" 2>/dev/null || return 1
      grep -q '^Hidden=true$' "$tmp" 2>/dev/null || printf '\nHidden=true\n' >> "$tmp"
    else
      awk -v target="$DASH/kiosk.sh" '
        index($0, target) && $0 !~ /^#[[:space:]]*dash-go-doctor-disabled:/ { print "# dash-go-doctor-disabled: " $0; next }
        { print }
      ' "$path" > "$tmp" || { rm -f "$tmp"; return 1; }
    fi
    mv -f "$tmp" "$path" || { rm -f "$tmp"; return 1; }
    changed=$((changed+1))
  done < <(legacy_kiosk_autostart_files)
  [ "$changed" -gt 0 ]
}

kiosk_process_details(){
  local lines="$1" pid
  while IFS= read -r line; do
    pid="${line%% *}"
    [[ "$pid" =~ ^[0-9]+$ ]] || continue
    ps -p "$pid" -o pid=,ppid=,etimes=,args= 2>/dev/null | trim_line
  done <<< "$lines"
}

current_kiosk_log_errors(){
  # A duplicate launcher that immediately recognizes the live lock and exits
  # is normal race protection, not an active launcher failure.
  printf '%s' "$1"     | grep -Ei 'duplicate launcher|could not acquire kiosk lock|openbox.*(not found|failed)|surf.*(failed|missing|command not found)|surf exit looked unexpected|fullscreen helper (unavailable|could not find|failed)|wmctrl.*(not installed|unavailable)|cannot open display|could not connect to display'     | grep -Evi 'already running as pid [0-9]+; this duplicate launcher will exit'     | tail -5 | trim_line || true
}

check_kiosk(){
  local gui=0 gui_active=0 gui_expected=0 kiosk_lines="" kiosk_count=0 surf_count=0 lock="$CACHE_DIR/kiosk.lock" pid="" cmd="" session="" autouser="" path="" errors="" relaunches=0 missing_gui="" legacy="" detail="" surf_pid="" surf_window="" kiosk_log="" openbox_log=""
  section "Kiosk, Surf, and graphical session"

  if have pgrep && { pgrep -x openbox >/dev/null 2>&1 || pgrep -x lightdm >/dev/null 2>&1; }; then gui_active=1; fi
  if [ -d /tmp/.X11-unix ] && find /tmp/.X11-unix -maxdepth 1 -type s -name 'X*' -print -quit 2>/dev/null | grep -q .; then gui_active=1; fi
  if [ "$COMMON_LOADED" -eq 1 ] && { [ -f "$AUTLOGIN_FILE" ] || [ -f "$XSESSION_FILE" ]; }; then
    autouser="$(lightdm_autologin_user)"
    session="$(lightdm_autologin_session)"
    if [ "$autouser" = "$USER_NAME" ] && { [ "$session" = dashboard-openbox ] || [ "$session" = dashboard-lite ]; }; then gui_expected=1; fi
  fi
  if [ "$gui_active" -eq 1 ] || [ "$gui_expected" -eq 1 ]; then gui=1; fi

  for path in "$DASH/kiosk.sh" "$BIN_DIR/dashboard-lite-session.sh" "$BIN_DIR/dashboard-session-guard.sh"; do
    if [ -x "$path" ]; then
      ok "executable: ${path#$DASH/}"
    elif [ -f "$path" ]; then
      doctor_plan_add safe kiosk-executables "Restore kiosk script permissions" "A packaged kiosk script has lost its executable mode bit." "Restores executable mode only for known Dash-Go kiosk scripts." "Script contents, settings, and personal files."
      if can_apply_fix kiosk-executables && chmod +x "$path"; then fixed "restored executable permission: ${path#$DASH/}"; else fail_fix "kiosk component is not executable: $path" "select Restore kiosk script permissions or run ~/install.sh --repair"; fi
    else
      doctor_plan_add repair installer-repair "Refresh Dash-Go application files" "A required kiosk component is missing." "Downloads and verifies the selected release, then restores missing app files in a staged repair." "Personal settings, calendars, caches, and logs."
      fail_fix "missing kiosk component: $path" "run ~/install.sh --repair"
    fi
  done

  if [ "$gui" -eq 1 ]; then
    for path in surf openbox wmctrl xset curl; do have "$path" || missing_gui="$missing_gui $path"; done
    if [ -z "$missing_gui" ]; then
      ok "required graphical kiosk commands are available"
    else
      doctor_plan_add manual kiosk-dependencies "Install missing graphical kiosk dependencies" "The managed kiosk session cannot start without:${missing_gui}." "Installs the supported Surf/Openbox/X11 support packages." "Dashboard data and settings."
      fail_fix "graphical kiosk dependencies are missing:$missing_gui" "sudo apt-get install -y surf openbox wmctrl x11-xserver-utils curl"
    fi
  elif have lightdm || [ -f "$XSESSION_FILE" ]; then
    info "graphical kiosk files are installed but no managed graphical session is active"
  else
    info "no graphical kiosk session was detected"
  fi

  if [ -d "$lock" ]; then
    pid="$(cat "$lock/pid" 2>/dev/null || true)"
    if [[ "$pid" =~ ^[0-9]+$ ]] && kill -0 "$pid" 2>/dev/null; then
      cmd="$(tr '\0' ' ' < "/proc/$pid/cmdline" 2>/dev/null || true)"
      if [[ "$cmd" == *kiosk.sh* ]]; then ok "kiosk lock belongs to live kiosk PID $pid"; else doctor_plan_add manual kiosk-lock-owner "Review a kiosk lock owned by another process" "The kiosk lock PID is alive but does not identify as kiosk.sh." "No automatic change; avoids disrupting an unrelated live process." "The running process and all dashboard data."; fail_fix "kiosk lock PID $pid belongs to another process" "inspect PID $pid, then remove '$lock' only if it is unrelated"; fi
    else
      doctor_plan_add safe stale-kiosk-lock "Remove a stale kiosk lock" "The kiosk lock belongs to no live process and blocks a normal launcher restart." "Removes only the stale dashboard-owned lock." "Live kiosk/browser processes and settings."
      if can_apply_fix stale-kiosk-lock && rm -rf "$lock"; then fixed "removed stale kiosk lock"; else fail_fix "stale kiosk lock blocks the launcher" "select Remove a stale kiosk lock or run rm -rf '$lock'"; fi
    fi
  else
    info "no kiosk lock is present"
  fi

  if [ -f "$CACHE_DIR/kiosk-paused" ] && file_old_minutes "$CACHE_DIR/kiosk-paused" 10; then
    doctor_plan_add safe stale-kiosk-pause "Clear a stale kiosk maintenance pause" "The dashboard pause marker is older than ten minutes and no live update marker is present." "Removes only the stale dashboard pause marker." "Live update markers, browser processes, and dashboard data."
    if can_apply_fix stale-kiosk-pause && rm -f "$CACHE_DIR/kiosk-paused"; then fixed "removed stale kiosk maintenance pause"; else warn_fix "kiosk has been paused for more than ten minutes" "select Clear a stale kiosk maintenance pause or run rm -f '$CACHE_DIR/kiosk-paused'"; fi
  fi

  if have pgrep; then
    # pgrep receives a simple kiosk.sh search; the following fixed-string full
    # path filter is the authoritative ownership check. This avoids an ERE
    # escape mismatch that could report zero launchers on a healthy kiosk.
    kiosk_lines="$(pgrep -afu "$USER_NAME" kiosk.sh 2>/dev/null | grep -F "$DASH/kiosk.sh" | grep -v '[d]octor.sh' || true)"
    kiosk_count="$(printf '%s\n' "$kiosk_lines" | awk 'NF{n++} END{print n+0}')"
    surf_count="$(pgrep -u "$USER_NAME" -x surf 2>/dev/null | awk 'NF{n++} END{print n+0}')"
    if [ "$kiosk_count" -eq 1 ]; then
      ok "one kiosk launcher is running"
    elif [ "$kiosk_count" -gt 1 ]; then
      detail="$(kiosk_process_details "$kiosk_lines" | tr '\n' ';' | trim_line)"
      legacy="$(legacy_kiosk_autostart_files | tr '\n' ',' | sed 's/,$//' || true)"
      if [ -n "$legacy" ]; then
        doctor_plan_add guided legacy-kiosk-autostart "Disable obsolete Dash-Go kiosk autostart" "A managed dashboard session already launches kiosk.sh, while legacy autostart file(s) also reference it: $legacy." "Creates timestamped backups and disables only exact Dash-Go kiosk launch lines; restart LightDM or reboot afterward." "Other autostart entries, live kiosk processes, and settings."
        if can_apply_fix legacy-kiosk-autostart && disable_legacy_kiosk_autostarts; then
          fixed "disabled legacy Dash-Go kiosk autostart entry; restart LightDM or reboot to remove the existing duplicate"
        else
          warn_fix "$kiosk_count kiosk launchers are running${detail:+ ($detail)}" "select Disable obsolete Dash-Go kiosk autostart, then restart LightDM or reboot"
        fi
      else
        doctor_plan_add manual duplicate-kiosk "Review duplicate kiosk launchers" "More than one kiosk.sh process is active, but no exact legacy Dash-Go autostart source was found." "No automatic process termination; protects the live dashboard session." "All live kiosk/browser processes."
        warn_fix "$kiosk_count kiosk launchers are running${detail:+ ($detail)}; Doctor will not kill a live session automatically" "keep one session-owned kiosk.sh, then restart the graphical session"
      fi
    elif [ "$gui" -eq 1 ]; then
      warn_fix "no kiosk launcher is running in the managed graphical installation" "remove a stale lock if present, then restart LightDM or reboot"
    else
      info "no kiosk launcher process (normal on a headless/non-kiosk install)"
    fi

    if [ "$surf_count" -eq 1 ]; then
      ok "one user-owned Surf browser is running"
      surf_pid="$(pgrep -u "$USER_NAME" -x surf 2>/dev/null | head -1 || true)"
      if [ "$gui" -eq 1 ] && have wmctrl && [ -n "$surf_pid" ]; then
        surf_window="$(DISPLAY="${DISPLAY:-:0}" XAUTHORITY="${XAUTHORITY:-$HOME/.Xauthority}" wmctrl -lp 2>/dev/null | awk -v want="$surf_pid" '$3 == want {print $1; exit}' || true)"
        if [ -n "$surf_window" ]; then
          ok "Surf window is visible to the active X11 session"
        elif DISPLAY="${DISPLAY:-:0}" XAUTHORITY="${XAUTHORITY:-$HOME/.Xauthority}" wmctrl -m >/dev/null 2>&1; then
          warn_fix "Surf is running but no matching X11 window is visible; the kiosk may be hidden behind the desktop" "restart LightDM or reboot after reviewing logs/kiosk.log"
        else
          info "could not query Surf window visibility from this shell"
        fi
      fi
    elif [ "$surf_count" -gt 1 ]; then
      warn_fix "$surf_count user-owned Surf browser processes are running; Doctor will not kill them automatically" "keep one Surf process, then restart the graphical session"
    elif [ "$gui" -eq 1 ]; then
      if pgrep -x openbox >/dev/null 2>&1; then
        warn_fix "Openbox is running but Surf is absent; the kiosk may be stranded on the desktop background" "review kiosk.log, remove a stale lock if present, then restart LightDM or reboot"
      else
        warn_fix "Surf is not running in the managed graphical installation" "check logs/openbox-session.log, then restart LightDM or reboot"
      fi
    else
      info "Surf is not running because no graphical kiosk session was detected"
    fi
  fi

  if [ "$COMMON_LOADED" -eq 1 ] && (have lightdm || [ -f "$AUTLOGIN_FILE" ]); then
    autouser="$(lightdm_autologin_user)"
    session="$(lightdm_autologin_session)"
    if [ -z "$autouser" ] && [ -z "$session" ]; then
      info "LightDM autologin is not configured (optional on a normal desktop)"
    elif [ "$autouser" = "$USER_NAME" ] && { [ "$session" = dashboard-openbox ] || [ "$session" = dashboard-lite ]; } && lightdm_dashboard_xsession_ok "$session"; then
      ok "LightDM autologin and Dash-Go X session agree"
    else
      doctor_plan_add admin autologin-session "Restore managed LightDM autologin and Dash-Go session" "The configured autologin user/session or its X session file does not match the managed Dash-Go kiosk." "Stages and verifies a managed LightDM/X-session configuration before migrating legacy keys, then requires a graphical-session restart." "Dashboard settings, calendars, caches, unrelated desktop files, and the prior verified LightDM autologin configuration."
      if can_apply_fix autologin-session; then
        if declare -F dashboard_boot_config_write_safe >/dev/null 2>&1 && ! dashboard_boot_config_write_safe /etc/lightdm; then
          doctor_plan_add manual autologin-storage "Repair storage before rewriting LightDM autologin" "Storage health is failing or the LightDM filesystem is read-only." "No boot configuration is changed until storage is safe for an atomic write." "Existing autologin and all dashboard data."
          warn_fix "refused LightDM autologin repair because storage is unsafe" "repair/replace storage first, then rerun Doctor and select the LightDM repair"
        elif { [ -f "$AUTLOGIN_FILE" ] || [ "$session" = dashboard-openbox ] || [ "$session" = dashboard-lite ]; } && write_dashboard_openbox_xsession >/dev/null 2>&1 && write_dashboard_lightdm_autologin "$USER_NAME" dashboard-openbox >/dev/null 2>&1; then
          fixed "repaired managed LightDM autologin and dashboard-openbox session; restart LightDM or reboot to apply it"
        else
          warn_fix "LightDM autologin user/session is '${autouser:-unset}/${session:-unset}' or its X session file is stale; the previous configuration was preserved if staging failed" "select the LightDM repair from a sudo-capable terminal or use installer System pieces"
        fi
      else
        warn_fix "LightDM autologin user/session is '${autouser:-unset}/${session:-unset}' or its X session file is stale" "select the LightDM repair from a sudo-capable terminal or use installer System pieces"
      fi
    fi
  fi

  if [ "$MODE" = full ]; then
    openbox_log="$(latest_log_session "$LOG_DIR/openbox-session.log" 'openbox session starting')"
    if [ -n "$openbox_log" ]; then
      errors="$(current_kiosk_log_errors "$openbox_log")"
      [ -z "$errors" ] && ok "current Openbox session log has no common launcher failure signature" || warn "current Openbox session has launcher failures: $errors"
    elif [ -f "$LOG_DIR/openbox-session.log" ]; then
      info_quiet "openbox-session.log has no current-session marker; historical entries are not treated as active failures"
    fi

    kiosk_log="$(latest_log_session "$LOG_DIR/kiosk.log" 'kiosk session started')"
    if [ -n "$kiosk_log" ]; then
      errors="$(current_kiosk_log_errors "$kiosk_log")"
      [ -z "$errors" ] && ok "current kiosk log has no common launcher failure signature" || warn "current kiosk session has launcher failures: $errors"
      relaunches="$(printf '%s' "$kiosk_log" | grep -Eic 'surf exit looked unexpected' || true)"
      if [ "$relaunches" -ge 3 ] 2>/dev/null; then
        warn_fix "current kiosk session shows $relaunches unexpected Surf exits; Doctor will not terminate a live kiosk automatically" "review the first Surf/Openbox error in logs/kiosk.log, then restart LightDM or reboot"
      fi
    elif [ -f "$LOG_DIR/kiosk.log" ]; then
      info_quiet "kiosk.log predates current-session markers; historical entries are not treated as active failures"
    fi
  fi
}

check_resilience_state(){
  local file label level reason detail
  section "Boring resilience state"
  resilience_state(){
    file="$1"; label="$2"
    [ -f "$file" ] || return 0
    if ! json_ok "$file"; then warn "$label state file is malformed; its owning subsystem will replace it"; return 0; fi
    detail="$(python3 - "$file" <<'PY2'
import json,sys
try:
 x=json.load(open(sys.argv[1],encoding='utf-8'))
 print(str(x.get('level') or x.get('state') or 'unknown')+'\t'+str(x.get('reason') or ''))
except Exception: print('unknown\t')
PY2
)"
    level="${detail%%$'\t'*}"; reason="${detail#*$'\t'}"
    case "$level" in
      ok|healthy|success) ok "$label reports normal" ;;
      watch|unknown) info_quiet "$label state is ${reason:-not yet established}" ;;
      recovering|recovered) warn "$label is recovering${reason:+: $reason}" ;;
      warn|warning|degraded|reverted) warn "$label needs attention${reason:+: $reason}" ;;
      fail|failed|failing|error) fail_fix "$label reports a failure${reason:+: $reason}" "review Dashboard Control status or run doctor.sh --full" ;;
      *) info_quiet "$label state is $level${reason:+: $reason}" ;;
    esac
  }
  resilience_state "$CACHE_DIR/safe-mode-state.json" "safe-mode recovery"
  if [ -f "$CACHE_DIR/clock-unverified" ]; then warn "device clock has not yet been verified by network time"; else ok "clock-verification marker is clear"; fi
  resilience_state "$CACHE_DIR/storage-wear-state.json" "storage wear monitor"
  resilience_state "$CACHE_DIR/config-revert.json" "settings last-good recovery"
  resilience_state "$CACHE_DIR/server-restart-state.json" "server restart guard"
  resilience_state "$CACHE_DIR/update-status.json" "last update"
  if [ -d "$CACHE_DIR/provider-backoff" ]; then
    local active=0
    while IFS= read -r file; do
      [ -n "$file" ] || continue
      if python3 - "$file" <<'PY2' 2>/dev/null
import json,sys,time
try:
 x=json.load(open(sys.argv[1])); raise SystemExit(0 if int(x.get('until',0) or 0)>time.time() else 1)
except Exception: raise SystemExit(1)
PY2
      then active=$((active+1)); fi
    done < <(find "$CACHE_DIR/provider-backoff" -maxdepth 1 -type f -name '*.json' -print 2>/dev/null || true)
    [ "$active" -eq 0 ] && ok "providers are not in retry backoff" || warn "$active provider(s) are in bounded retry backoff; last-good data is being retained"
  fi
}

check_system(){
  local free_kb mem_kb year missing="" tool sync mount_opts="" storage_errors="" guard_status=""
  section "System readiness"
  [ "$SKIP_SYSTEM" = 1 ] && { info "system readiness checks skipped by DOCTOR_SKIP_SYSTEM"; return; }

  for tool in python3 curl systemctl; do have "$tool" || missing="$missing $tool"; done
  if [ -z "$missing" ]; then ok "required maintenance tools are installed"; else fail_fix "missing required tools:$missing" "sudo apt-get install -y python3 curl systemd"; fi

  if have findmnt; then
    mount_opts="$(findmnt -no OPTIONS --target "$DASH" 2>/dev/null || true)"
    case ",$mount_opts," in
      *,ro,*)
        doctor_plan_add manual filesystem-readonly "Resolve a read-only dashboard filesystem" "The dashboard filesystem is mounted read-only, commonly after storage or filesystem errors." "No automatic write attempt; protects data until the underlying storage/filesystem is repaired." "Existing dashboard files and the mounted filesystem."
        fail_fix "dashboard filesystem is mounted read-only" "stop using the device, inspect kernel storage errors, then repair/replace the storage before running installer repair"
        ;;
      *) [ -n "$mount_opts" ] && ok "dashboard filesystem is writable" ;;
    esac
  fi

  if have df; then
    free_kb="$(df -Pk "$DASH" 2>/dev/null | awk 'NR==2{print $4}')"
    case "$free_kb" in ''|*[!0-9]*) warn "could not determine free disk space";;
      *) if [ "$free_kb" -lt 102400 ]; then fail_fix "less than 100 MB free on the dashboard filesystem" "remove old logs/caches or expand the filesystem";
         elif [ "$free_kb" -lt 512000 ]; then warn_fix "less than 500 MB free on the dashboard filesystem" "run '$BIN_DIR/dashboard-housekeeping.sh' and review disk use";
         else ok "$((free_kb/1024)) MB free on the dashboard filesystem"; fi;;
    esac
  fi

  mem_kb="$(awk '/^MemAvailable:/ {print $2; exit}' /proc/meminfo 2>/dev/null || true)"
  case "$mem_kb" in ''|*[!0-9]*) info "memory availability unavailable";;
    *) if [ "$mem_kb" -lt 65536 ]; then warn_fix "only $((mem_kb/1024)) MB memory available" "close extra browser/desktop processes or reboot"; else ok "$((mem_kb/1024)) MB memory currently available"; fi;;
  esac

  if command -v dash_clock_verified >/dev/null 2>&1 && dash_clock_verified; then
    ok "system clock is confirmed by network time or does not predate this install"
  else
    fail_fix "system clock has not been confirmed by network time and predates this install" "enable NTP with sudo timedatectl set-ntp true"
  fi
  if have timedatectl; then
    sync="$(timeout 3 timedatectl show -p NTPSynchronized --value 2>/dev/null || true)"
    [ "$sync" = yes ] && ok "network time is synchronized" || warn_fix "network time is not yet synchronized" "sudo timedatectl set-ntp true"
  fi

  if [ -x "$BIN_DIR/dashboard-health-guard.sh" ] && [ "$MODE" = full ]; then
    if [ -f "$CACHE_DIR/health-guard-status.json" ]; then
      if json_ok "$CACHE_DIR/health-guard-status.json"; then
        if file_old_minutes "$CACHE_DIR/health-guard-status.json" 95 && [ "$(awk '{print int($1)}' /proc/uptime 2>/dev/null || echo 0)" -gt 7200 ] 2>/dev/null; then
          warn_fix "health guard has not recorded a run in more than 95 minutes" "run '$BIN_DIR/dashboard-health-guard.sh' and review the managed cron job"
        else
          ok "bounded health guard has recorded recent status"
        fi
      else
        warn "health guard status file is malformed; the next guard run will replace it"
      fi
    else
      info "health guard has not recorded status yet (normal shortly after install)"
    fi
  fi

  if [ "$MODE" = full ] && have journalctl; then
    local oom
    oom="$(journalctl -k -b -n 300 --no-pager 2>/dev/null | grep -Ei 'out of memory|oom-kill|killed process.*(surf|webkit|dashboard)' | tail -3 | trim_line || true)"
    [ -z "$oom" ] && ok "current boot has no dashboard/browser OOM signature" || warn "current-boot memory-pressure event found: $oom"
    # Normal mmcblk discovery/partition lines are expected at boot. Review only
    # concrete block-device timeout/reset/I/O signatures or filesystem failures.
    storage_error_pattern='mmcblk[^:]*:.*(I/O error|timeout|timed out|error -[0-9]+|reset)|Buffer I/O error|EXT4-fs.*(error|remounting filesystem read-only)|filesystem.*read-only'
    storage_errors="$(journalctl -k -b -n 500 --no-pager 2>/dev/null | grep -Ei "$storage_error_pattern" | tail -4 | trim_line || true)"
    if [ -n "$storage_errors" ]; then
      doctor_plan_add manual storage-errors "Review storage or filesystem errors" "The current boot kernel log contains SD-card, block I/O, or filesystem errors." "No automatic cleanup; avoids writing more data to potentially failing storage." "Existing dashboard data and system files."
      warn "current-boot storage/filesystem warning: $storage_errors"
    else
      ok "current boot has no common storage or filesystem error signature"
    fi
  fi
}

sha256_file(){
  local path="$1"
  if have sha256sum; then sha256sum "$path" | awk '{print $1}';
  elif have python3; then python3 - "$path" <<'PY'
import hashlib, sys
h=hashlib.sha256()
with open(sys.argv[1], 'rb') as f:
    for chunk in iter(lambda: f.read(1024 * 1024), b''):
        h.update(chunk)
print(h.hexdigest())
PY
  else return 1; fi
}

online_release_track(){
  local raw="${DASH_TRACK:-}" installed
  if [ -z "$raw" ] && [ -r "$HOME/.dashboard-update-profile.json" ]; then
    raw="$(sed -nE 's/^[[:space:]]*"track"[[:space:]]*:[[:space:]]*"(stable|beta)"[[:space:]]*,?[[:space:]]*$/\1/p' "$HOME/.dashboard-update-profile.json" 2>/dev/null | head -n 1 || true)"
  fi
  case "$raw" in stable|beta) printf '%s\n' "$raw"; return 0;; esac
  installed="$(tr -d '[:space:]' < "$DASH/VERSION" 2>/dev/null || true)"
  case "$installed" in *-beta.*) printf 'beta\n';; *) printf 'stable\n';; esac
}

github_digest_sha256(){
  local raw
  raw="$(printf '%s' "${1:-}" | tr '[:upper:]' '[:lower:]')"
  [[ "$raw" =~ ^sha256:[0-9a-f]{64}$ ]] || return 1
  printf '%s\n' "${raw#sha256:}"
}

github_checksum_for_name(){
  local sums="$1" wanted="$2"
  awk -v wanted="$wanted" '
    /^[0-9a-fA-F]{64}[[:space:]][[:space:]]+/ {
      hash=$1; name=$2; sub(/^\*/, "", name)
      if (name == wanted) { print tolower(hash); found=1; exit }
    }
    END { if (!found) exit 1 }
  ' "$sums"
}

github_asset_url_ok(){
  case "${1:-}" in
    https://github.com/*|https://objects.githubusercontent.com/*|https://github-releases.githubusercontent.com/*) return 0;;
    *) return 1;;
  esac
}

check_online_release(){
  local track tmp resolved version tag repo immutable bundle_name sums_name bundle_url sums_url bundle_digest sums_digest expected actual
  [ "$ONLINE" -eq 1 ] || return 0
  section "GitHub Release validation"
  if ! have curl || ! have sha256sum || [ ! -x "$BIN" ]; then
    fail_fix "--online needs curl, sha256sum, and the installed dashboard server" "repair the local installation, then rerun doctor.sh --online"
    return 0
  fi
  if ! "$BIN" --updater-capabilities 2>/dev/null | grep -Fqx 'github-release-resolution-v3'; then
    fail_fix "the installed updater does not support canonical GitHub Release validation with bounded metadata caching" "run the downloaded Dash-Go GitHub Release bundle installer once to complete the migration"
    return 0
  fi
  track="$(online_release_track)"
  tmp="$(mktemp -d)" || { warn "could not create temporary directory for GitHub Release validation"; return 0; }
  trap 'rm -rf "$tmp" 2>/dev/null || true' RETURN
  resolved="$tmp/resolved.json"
  if ! "$BIN" --resolve-github-release --track "$track" > "$resolved"; then
    fail_fix "canonical GitHub Release discovery failed for the selected $track track" "check network access and the published Dash-Go GitHub Release assets"
    trap - RETURN; rm -rf "$tmp"; return 0
  fi
  version="$("$BIN" --json-get "$resolved" version 2>/dev/null || true)"
  tag="$("$BIN" --json-get "$resolved" tag 2>/dev/null || true)"
  repo="$("$BIN" --json-get "$resolved" repository 2>/dev/null || true)"
  immutable="$("$BIN" --json-get "$resolved" immutable 2>/dev/null || true)"
  bundle_name="$("$BIN" --json-get "$resolved" assets.release.name 2>/dev/null || true)"
  sums_name="$("$BIN" --json-get "$resolved" assets.checksums.name 2>/dev/null || true)"
  bundle_url="$("$BIN" --json-get "$resolved" assets.release.browser_download_url 2>/dev/null || true)"
  sums_url="$("$BIN" --json-get "$resolved" assets.checksums.browser_download_url 2>/dev/null || true)"
  bundle_digest="$(github_digest_sha256 "$("$BIN" --json-get "$resolved" assets.release.digest 2>/dev/null || true)" 2>/dev/null || true)"
  sums_digest="$(github_digest_sha256 "$("$BIN" --json-get "$resolved" assets.checksums.digest 2>/dev/null || true)" 2>/dev/null || true)"
  if ! [[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-beta\.[0-9]+)?$ ]] || [ "$tag" != "v$version" ] || [ "$repo" != "DashDashGoApp/Dash-Go" ] || [ "$immutable" != true ] || [ "$bundle_name" != "Dash-Go_${version}_release.tar.gz" ] || [ "$sums_name" != SHA256SUMS ] || [ -z "$bundle_digest" ] || [ -z "$sums_digest" ] || ! github_asset_url_ok "$bundle_url" || ! github_asset_url_ok "$sums_url"; then
    fail_fix "canonical GitHub Release metadata violates the Dash-Go asset contract" "publish a complete immutable release with the exact Dash-Go bundle, SHA256SUMS, and GitHub SHA-256 digests"
    trap - RETURN; rm -rf "$tmp"; return 0
  fi
  if ! curl -fsSL --proto '=https' --tlsv1.2 --connect-timeout 8 --max-time 180 -A 'Dash-Go Doctor' "$bundle_url" -o "$tmp/$bundle_name" || ! curl -fsSL --proto '=https' --tlsv1.2 --connect-timeout 8 --max-time 120 -A 'Dash-Go Doctor' "$sums_url" -o "$tmp/$sums_name"; then
    fail_fix "required GitHub Release bundle or SHA256SUMS cannot be downloaded" "check the published release asset URLs and network access"
    trap - RETURN; rm -rf "$tmp"; return 0
  fi
  actual="$(sha256_file "$tmp/$bundle_name" 2>/dev/null || true)"
  expected="$(github_checksum_for_name "$tmp/$sums_name" "$bundle_name" 2>/dev/null || true)"
  if [ "$actual" != "$bundle_digest" ] || [ -z "$expected" ] || [ "$actual" != "$expected" ] || [ "$(sha256_file "$tmp/$sums_name" 2>/dev/null || true)" != "$sums_digest" ]; then
    fail_fix "GitHub Release asset digest or SHA256SUMS verification failed" "rebuild the release assets and publish a complete immutable GitHub Release"
    trap - RETURN; rm -rf "$tmp"; return 0
  fi
  if ! tar -tzf "$tmp/$bundle_name" | grep -Eq '(^|/)app/(manifest\.json|VERSION)$'; then
    fail_fix "GitHub Release bundle is missing the installable Dash-Go app payload" "rebuild the self-contained release bundle before publishing"
    trap - RETURN; rm -rf "$tmp"; return 0
  fi
  ok "canonical GitHub $track release is complete and digest-valid: $version"
  info "Canonical GitHub $track release is complete and digest-valid: $version"
  trap - RETURN; rm -rf "$tmp"
}

printf 'Dash-Go Doctor — %s scan%s\n' "$MODE" "$([ "$FIX" -eq 1 ] && printf ' with selected repairs' || true)"
printf 'Dashboard: %s\n' "$DASH"

check_installation
check_configuration
check_data
check_server
check_kiosk
check_resilience_state
check_system
check_online_release

section "Summary"
section_emit
printf 'INFO %s passed · %s repaired · %s warnings · %s action required\n' "$OK_COUNT" "$FIX_COUNT" "$WARN_COUNT" "$FAIL_COUNT"
if [ "$FAIL_COUNT" -eq 0 ] && [ "$WARN_COUNT" -eq 0 ]; then
  printf 'INFO Result: Dash-Go is healthy.\n'
elif [ "$FAIL_COUNT" -eq 0 ]; then
  printf 'INFO Result: Dash-Go is usable; review the warnings or the repair plan when convenient.\n'
else
  printf 'INFO Result: Dash-Go has unresolved problems. Review the repair plan for what Doctor can safely change and what needs installer/manual attention.\n'
fi

if [ "$FIX" -eq 1 ] && declare -F doctor_plan_post_repair_summary >/dev/null 2>&1; then
  if ! doctor_plan_post_repair_summary; then
    RC=1
    printf 'INFO Result: Doctor applied selected repairs, but the device is not fully healthy until the remaining installer/manual actions are completed.\n'
  fi
fi

if [ "$PLAN_MODE" -eq 1 ]; then
  if [ "$INTERACTIVE_PLAN" -eq 1 ]; then
    doctor_plan_interactive
  else
    doctor_plan_render
  fi
elif [ "$FIX" -eq 0 ] && [ "$NO_PROMPT" -eq 0 ] && [ "$ACTIONABLE" -gt 0 ] && [ -t 0 ]; then
  printf '\nReview the full repair plan before making any changes? [P/n] '
  read -r answer
  case "$answer" in
    ''|p|P|plan|PLAN|y|Y|yes|YES) exec "$0" --full --interactive-plan ;;
    *) info "No changes were made. Run doctor.sh --plan whenever you want the detailed repair plan." ;;
  esac
fi

exit "$RC"
