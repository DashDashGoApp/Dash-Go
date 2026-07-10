#!/bin/bash
# =====================================================================
#  install.sh — Dash-Go kiosk installer (Raspberry Pi + Debian x86)
#
#  Sets up the dashboard on a fresh device: downloads app files, asks a few
#  customization questions, writes per-device settings, installs fonts,
#  sets permissions, and configures the web server, autostart, and cron.
#
#  USAGE:   ~/install.sh                         interactive menu
#           ~/install.sh --help                  concise command help
#           ~/install.sh update                  unattended update to latest
#           ~/install.sh --update --latest       unattended update to latest
#           ~/install.sh --repair                repair install: fresh app
#                                                files, restored settings and
#                                                calendars (preserves settings)
#           ~/install.sh --repair --reset-profile
#                                                explicitly reapply detected
#                                                performance-profile defaults
#           ~/install.sh --doctor [--full]       run the installed doctor
#                                                directly from SSH/terminal
#           ~/install.sh --remove                uninstall Dash-Go offline
#                                                (verified archive optional)
#                                                (diagnostic/emergency only)
#
#  The on-screen "Update dashboard" button runs the latest-version form.
#  Re-runnable: safe to run again to change settings (it backs up and
#  overwrites). System files are backed up to *.bak before editing.
#
#  Releases are resolved only from the compiled canonical Dash-Go GitHub
#  repository. The only user-selectable durable update setting is DASH_TRACK
#  (stable or beta); legacy updater records are sanitized during migration.
UPDATE_ENV="$HOME/.dashboard-update.env"
UPDATE_PROFILE="$HOME/.dashboard-update-profile.json"
CONTROL_ENV="$HOME/.dashboard-control.env"
WEATHER_ENV="$HOME/.dashboard-weather.env"
MESSAGE_ENV="$HOME/.dashboard-message.env"
RADAR_ENV="$HOME/.dashboard-radar.env"
ENV_DASH_TRACK="${DASH_TRACK:-}"

# beta.34 migrates legacy per-device updater state without sourcing it. Older
# files may contain arbitrary hosts or credentials; accept only the literal
# stable/beta choice and never evaluate the file as shell code.
legacy_saved_update_track(){
  local raw=""
  if [ -r "$UPDATE_PROFILE" ]; then
    raw="$(sed -nE 's/^[[:space:]]*"track"[[:space:]]*:[[:space:]]*"(stable|beta)"[[:space:]]*,?[[:space:]]*$/\1/p' "$UPDATE_PROFILE" 2>/dev/null | head -n 1 || true)"
  fi
  if [ -z "$raw" ] && [ -r "$UPDATE_ENV" ]; then
    raw="$(sed -nE 's/^[[:space:]]*(export[[:space:]]+)?DASH_TRACK[[:space:]]*=[[:space:]]*(stable|beta)[[:space:]]*$/\2/p' "$UPDATE_ENV" 2>/dev/null | head -n 1 || true)"
  fi
  case "$raw" in stable|beta) printf '%s\n' "$raw";; esac
}
DASH_TRACK="${ENV_DASH_TRACK:-$(legacy_saved_update_track)}"
DASH_TRACK="${DASH_TRACK:-}"
# =====================================================================
set -u
# Keep the original invocation shape for interactive-only reassurance and
# preflight behavior. Argument parsing below may shift positional parameters.
# Interactive restarts must preserve a caller's shortcut flags as well.
INSTALLER_ORIGINAL_ARGC=$#
INSTALLER_ORIGINAL_ARGS=("$@")
INSTALLER_SOURCE_DIR="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
DASH="$HOME/dashboard"
BIN_DIR="$DASH/bin"
CONFIG_DIR="$DASH/config"
SETTINGS_FILE="$CONFIG_DIR/settings.json"
CAL_DIR="$DASH/calendars"
CACHE_DIR="$DASH/cache"
LOG_DIR="$DASH/logs"
# Repair archives live outside the application tree they are protecting.
DASHGO_STATE_DIR="${XDG_STATE_HOME:-$HOME/.local/state}/dash-go"
REPAIR_BACKUP_DIR="$DASHGO_STATE_DIR/repair-backups"
FONT_DIR="$DASH/ui/fonts"
RUNTIME_FONT_DIR="$DASH/fonts"
BASE_DIR="$DASH/base"
INSTALLER="$HOME/install.sh"   # canonical launcher; app files live in $DASH
USER_NAME="$(whoami)"
# Shared update status/logging used by both SSH-run updates and Dashboard Control updates.
# This keeps Control > Update / Backup / Restore in sync no matter where the
# update was launched from.
UPDATE_STATUS_FILE="$CACHE_DIR/update-status.json"
UPDATE_LOG_FILE="$LOG_DIR/update.log"
DEMO_STATUS_FILE="$CACHE_DIR/demo-mode.json"
DO_DEMO=0
DEMO_RESET_REQUESTED=0
UPDATER_CAPABILITY_FLOOR="1.4.3-beta.72"
UPDATER_MIGRATION_FILE="$CACHE_DIR/updater-migration-v1.json"

update_cli_for_host(){
  release_server_for_host "$DASH" 2>/dev/null || true
}
update_cli(){
  local bin
  bin="$(update_cli_for_host)"
  [ -n "$bin" ] && [ -x "$bin" ] || return 1
  "$bin" "$@"
}

# The installer never depends on a system Python interpreter. Runtime JSON,
# PIN, and geocoding work uses the installed Go control server; the small
# shell fallbacks below are used only before a trusted Go binary exists.
installer_cli(){ update_cli "$@"; }
trim_input(){
  local value="${1:-}"
  value="${value#"${value%%[![:space:]]*}"}"
  value="${value%"${value##*[![:space:]]}"}"
  printf '%s' "$value"
}
json_escape_shell(){
  printf '%s' "${1:-}" | sed 's/\\/\\\\/g; s/"/\\"/g; s/\r/\\r/g; s/\n/\\n/g'
}
json_basic_valid(){
  # Bootstrap-only structural guard. Release manifests receive the stronger
  # path/hash verification below; normal installs use --json-validate.
  [ -s "$1" ] || return 1
  awk 'BEGIN{RS=""} {gsub(/[[:space:]]/, "", $0); if (($0 ~ /^\{.*\}$/) || ($0 ~ /^\[.*\]$/)) exit 0; exit 1}' "$1"
}
installer_json_validate(){
  installer_cli --json-validate "$1" >/dev/null 2>&1 || json_basic_valid "$1"
}
# This parser is deliberately limited to the trusted resolver's compact JSON
# shape. It is only a first-install bootstrap fallback; normal updates use the
# Go --json-get command and never parse release metadata with shell text tools.
json_field_shell(){
  local file="$1" field="$2" raw group key
  raw="$(tr -d '\n\r' < "$file" 2>/dev/null || true)"
  [ -n "$raw" ] || return 1
  case "$field" in
    assets.release.*) group='release'; key="${field#assets.release.}" ;;
    assets.checksums.*) group='checksums'; key="${field#assets.checksums.}" ;;
    *) group=''; key="$field" ;;
  esac
  if [ -n "$group" ]; then
    raw="$(printf '%s' "$raw" | sed -nE 's/.*"'"$group"'"[[:space:]]*:[[:space:]]*\\{([^}]*)\\}.*/\\1/p')"
  fi
  case "$key" in
    immutable) printf '%s' "$raw" | sed -nE 's/.*"immutable"[[:space:]]*:[[:space:]]*(true|false).*/\1/p' | head -n1 ;;
    *) printf '%s' "$raw" | sed -nE 's/.*"'"$key"'"[[:space:]]*:[[:space:]]*"([^"]*)".*/\1/p' | head -n1 ;;
  esac
}
json_field(){
  local file="$1" field="$2"
  if installer_cli --json-get "$file" "$field" 2>/dev/null; then return 0; fi
  json_field_shell "$file" "$field"
}
manifest_entries_shell(){
  # Emit path<TAB>sha256 from the builder's stable pretty JSON. Entries with
  # missing values are rejected by callers before any staged file is trusted.
  awk '
    /"path"[[:space:]]*:/ { path=$0; sub(/.*"path"[[:space:]]*:[[:space:]]*"/, "", path); sub(/".*/, "", path) }
    /"sha256"[[:space:]]*:/ { hash=$0; sub(/.*"sha256"[[:space:]]*:[[:space:]]*"/, "", hash); sub(/".*/, "", hash) }
    /}[[:space:]]*,?[[:space:]]*$/ { if (path != "" || hash != "") { if (path != "" && hash != "") print path "\t" hash; path=""; hash="" } }
  ' "$1"
}
manifest_verify_shell(){
  local manifest="$1" root="$2" version="$3" target_bin="$4" path expected actual saw=0
  [ "$(json_field_shell "$manifest" version 2>/dev/null || true)" = "$version" ] || return 1
  while IFS=$'\t' read -r path expected; do
    [ -n "$path" ] && [ -n "$expected" ] || return 1
    case "$path" in /*|*'..'*|*$'\n'*|*$'\r'*) return 1;; esac
    [ -f "$root/$path" ] || return 1
    saw=1
    case "$path" in
      manifest.json) continue;;
      bin/dashboard-control-server-linux-*) [ -n "$target_bin" ] && [ "$path" != "$target_bin" ] && continue;;
    esac
    actual="$(sha256sum "$root/$path" 2>/dev/null | awk '{print $1}')" || return 1
    [ "$actual" = "$expected" ] || return 1
  done < <(manifest_entries_shell "$manifest")
  [ "$saw" = 1 ]
}
manifest_file_list_shell(){
  local manifest="$1" path expected
  while IFS=$'\t' read -r path expected; do
    [ -n "$path" ] || continue
    case "$path" in
      config/*|calendars/*|cache/*|logs/*|releases/*|.git/*|install.sh|AI.md) continue;;
    esac
    printf '%s\n' "$path"
  done < <(manifest_entries_shell "$manifest") | awk '!seen[$0]++'
}
json_array_lines(){
  local first=1 value
  printf '['
  while IFS= read -r value; do
    [ -n "$value" ] || continue
    [ "$first" = 1 ] || printf ','
    printf '"%s"' "$(json_escape_shell "$value")"
    first=0
  done
  printf ']'
}
zip_has_entries(){ unzip -Z1 "$1" 2>/dev/null | grep -q .; }
zip_restore_config_fallback(){
  local archive="$1" config="$2" entry base dest tmp
  while IFS= read -r entry; do
    entry="${entry#./}"; case "$entry" in config/*) ;; *) continue;; esac
    base="${entry#config/}"; case "$base" in */|*/*) continue;; esac
    case "$base" in settings.json|compliments.json|message-sources.json|message-cache-overrides.json|temp-messages.json|scheduled-messages.json|chalkboard.json|map-provider.json|config.local.js) ;; *) continue;; esac
    dest="$config/$base"; [ -f "$dest" ] && installer_json_validate "$dest" && continue
    tmp="${dest}.restore.$$"; mkdir -p "$config" || return 1
    unzip -p "$archive" "$entry" > "$tmp" 2>/dev/null || { rm -f "$tmp"; continue; }
    case "$base" in *.js) :;; *) installer_json_validate "$tmp" || { rm -f "$tmp"; continue; };; esac
    mv -f "$tmp" "$dest" || { rm -f "$tmp"; return 1; }
  done < <(unzip -Z1 "$archive" 2>/dev/null)
}
zip_restore_config_and_calendars(){
  local archive="$1" entry clean top rel root dest tmp
  while IFS= read -r entry; do
    clean="${entry#./}"; case "$clean" in config/*|calendars/*) ;; *) continue;; esac
    case "$clean" in */|/*|*'/../'*|../*|*'\\'*) continue;; esac
    top="${clean%%/*}"; rel="${clean#*/}"
    case "$top" in config) root="$CONFIG_DIR";; calendars) root="$CAL_DIR";; esac
    [ -n "$rel" ] || continue
    dest="$root/$rel"; case "$dest" in "$root"/*) ;; *) continue;; esac
    mkdir -p "$(dirname "$dest")" || return 1
    tmp="${dest}.restore.$$"
    unzip -p "$archive" "$entry" > "$tmp" 2>/dev/null || { rm -f "$tmp"; continue; }
    mv -f "$tmp" "$dest" || { rm -f "$tmp"; return 1; }
  done < <(unzip -Z1 "$archive" 2>/dev/null)
}
installed_dashboard_version(){
  head -n 1 "$DASH/VERSION" 2>/dev/null | tr -d '[:space:]'
}
# Private calendar selections are mutable user state outside the release tree.
# A beta.7 update may replace the helper that renders their exact mappings, so
# regenerate only the derived config/wrapper/registry after the protected state
# has been restored. --refresh is deliberately local-only: it does not contact
# a provider, discover collections, change cron, or change selections. A
# failure is non-fatal because the previous local mirrors remain usable and the
# user can retry from Calendar Manager after the update.
refresh_private_calendar_configuration_after_payload(){
  local private_home="${DASH_VDIR_HOME:-$HOME/.dashboard-vdirsyncer}" setup="$BIN_DIR/setup-vdirsyncer.sh"
  [ -s "$private_home/pairs" ] || return 0
  if [ ! -x "$setup" ]; then
    warn "private calendar configuration was retained but the refresh helper is unavailable; open Calendar Manager to retry"
    return 0
  fi
  if "$setup" --refresh >> "$LOG_DIR/update.log" 2>&1; then
    ok "refreshed private calendar configuration without contacting a provider"
  else
    warn "private calendar configuration was retained but could not be refreshed; open Calendar Manager to retry"
  fi
  return 0
}
# A pre-beta.72 server would treat an unknown CLI switch as a normal server
# start. Never probe it with --updater-capabilities. The floor is only a safe
# bootstrap boundary; every normal decision below uses live Go CLI output.
version_at_least(){
  local candidate="$1" floor="$2" cmaj cmin cpatch cbeta fmaj fmin fpatch fbeta index
  [[ "$candidate" =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)(-beta\.([0-9]+))?$ ]] || return 1
  cmaj="${BASH_REMATCH[1]}"; cmin="${BASH_REMATCH[2]}"; cpatch="${BASH_REMATCH[3]}"; cbeta="${BASH_REMATCH[5]:-}"
  [[ "$floor" =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)(-beta\.([0-9]+))?$ ]] || return 1
  fmaj="${BASH_REMATCH[1]}"; fmin="${BASH_REMATCH[2]}"; fpatch="${BASH_REMATCH[3]}"; fbeta="${BASH_REMATCH[5]:-}"
  for index in 1 2 3; do
    local c f
    case "$index" in
      1) c="$cmaj"; f="$fmaj";;
      2) c="$cmin"; f="$fmin";;
      *) c="$cpatch"; f="$fpatch";;
    esac
    ((10#$c > 10#$f)) && return 0
    ((10#$c < 10#$f)) && return 1
  done
  [ -z "$cbeta" ] && return 0
  [ -z "$fbeta" ] && return 1
  ((10#$cbeta >= 10#$fbeta))
}

# release_version_relation compares candidate against installed Dash-Go versions.
# It prints one of newer, equal, or older. Stable releases sort after prereleases
# with the same numeric base (for example 1.5.6 > 1.5.6-beta.9). Invalid values
# deliberately fail closed so a damaged install uses the verified recovery path.
release_version_relation(){
  local candidate="$1" installed="$2" cmaj cmin cpatch cbeta imaj imin ipatch ibeta index c i
  [[ "$candidate" =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)(-beta\.([0-9]+))?$ ]] || return 2
  cmaj="${BASH_REMATCH[1]}"; cmin="${BASH_REMATCH[2]}"; cpatch="${BASH_REMATCH[3]}"; cbeta="${BASH_REMATCH[5]:-}"
  [[ "$installed" =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)(-beta\.([0-9]+))?$ ]] || return 2
  imaj="${BASH_REMATCH[1]}"; imin="${BASH_REMATCH[2]}"; ipatch="${BASH_REMATCH[3]}"; ibeta="${BASH_REMATCH[5]:-}"
  for index in 1 2 3; do
    case "$index" in
      1) c="$cmaj"; i="$imaj";;
      2) c="$cmin"; i="$imin";;
      *) c="$cpatch"; i="$ipatch";;
    esac
    if ((10#$c > 10#$i)); then printf 'newer\n'; return 0; fi
    if ((10#$c < 10#$i)); then printf 'older\n'; return 0; fi
  done
  if [ -z "$cbeta" ] && [ -n "$ibeta" ]; then printf 'newer\n'; return 0; fi
  if [ -n "$cbeta" ] && [ -z "$ibeta" ]; then printf 'older\n'; return 0; fi
  if [ -z "$cbeta" ]; then printf 'equal\n'; return 0; fi
  if ((10#$cbeta > 10#$ibeta)); then printf 'newer\n'; return 0; fi
  if ((10#$cbeta < 10#$ibeta)); then printf 'older\n'; return 0; fi
  printf 'equal\n'
}

valid_release_version(){
  [[ "${1:-}" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-beta\.[0-9]+)?$ ]]
}
updater_capability_query_is_safe(){
  version_at_least "$(installed_dashboard_version)" "$UPDATER_CAPABILITY_FLOOR"
}
updater_capability_for_feature(){
case "${1:-}" in
    --verify-release-manifest) printf '%s\n' release-manifest-v1;;
    --release-file-list) printf '%s\n' release-file-list-v1;;
    --purge-stale-managed) printf '%s\n' stale-source-purge-v1;;
    --update-status) printf '%s\n' update-status-v1;;
    --update-job) printf '%s\n' update-job-v1;;
    --resolve-github-release) printf '%s\n' github-release-resolution-v3;;
    *) return 1;;
  esac
}
update_cli_capabilities(){
  local bin
  updater_capability_query_is_safe || return 2
  bin="$(update_cli_for_host)"
  [ -n "$bin" ] && [ -x "$bin" ] || return 1
  if command -v timeout >/dev/null 2>&1; then
    timeout 3 "$bin" --updater-capabilities
  else
    "$bin" --updater-capabilities
  fi
}
update_cli_supports(){
  local feature="$1" capability capabilities
  capability="$(updater_capability_for_feature "$feature" 2>/dev/null || true)"
  [ -n "$capability" ] || return 1
  capabilities="$(update_cli_capabilities 2>/dev/null || true)"
  printf '%s\n' "$capabilities" | grep -Fqx -- "$capability"
}
write_update_status(){
  local state="${1:-unknown}" label="${2:-}" detail="${3:-}" code="${4:-0}" track
  mkdir -p "$CACHE_DIR" "$LOG_DIR" 2>/dev/null || return 1
  track="${RELEASE_TRACK:-stable}"
  if update_cli_supports '--update-status'; then
    update_cli --update-status --file "$UPDATE_STATUS_FILE" --state "$state" --label "$label" --detail "$detail" --code "$code" \
      --source "${DASH_UPDATE_SOURCE:-ssh}" --target "${UPDATE_TARGET:-latest}" --track "$track" \
      --version "${DASH_INSTALLED_VERSION:-}" --previous-version "${UPDATE_PREVIOUS_VERSION:-}" \
      --job-id "${DASH_UPDATE_JOB_ID:-}" --health-checked "${DASH_UPDATE_HEALTH_CHECKED:-0}" --rolled-back "${DASH_UPDATE_ROLLED_BACK:-0}" \
      --rollback-attempted "${DASH_UPDATE_ROLLBACK_ATTEMPTED:-}" --rollback-succeeded "${DASH_UPDATE_ROLLBACK_SUCCEEDED:-}" >/dev/null 2>&1
    return $?
  fi
  # Pre-beta.72 Go payloads have no safe capability query. Their trusted host
  # binary still owns the older status-write command while the bridge runs.
  update_cli --write-status --file "$UPDATE_STATUS_FILE" --state "$state" --label "$label" --detail "$detail" --rc "$code" --kind update >/dev/null 2>&1
}
write_update_job(){
  local state="$1" label="$2" detail="$3" code="${4:-0}"
  update_cli_supports '--update-job' || return 0
  update_cli --update-job --file "$CACHE_DIR/update-job.json" --state "$state" --label "$label" --detail "$detail" --code "$code" \
    --source "${DASH_UPDATE_SOURCE:-ssh}" --target "${UPDATE_TARGET:-latest}" --track "${RELEASE_TRACK:-stable}" \
    --version "${DASH_INSTALLED_VERSION:-}" --previous-version "${UPDATE_PREVIOUS_VERSION:-}" --job-id "${DASH_UPDATE_JOB_ID:-}" \
    --rollback-attempted "${DASH_UPDATE_ROLLBACK_ATTEMPTED:-}" --rollback-succeeded "${DASH_UPDATE_ROLLBACK_SUCCEEDED:-}" --rolled-back "${DASH_UPDATE_ROLLED_BACK:-}" >/dev/null 2>&1
}

# Keep the durable state deliberately coarse and already-known to prior
# dashboard builds. A device may be displaying an update while it is replacing
# itself, so introducing a new state name here could make an older UI mistake
# an active phase for a terminal result. Human-readable labels carry the
# precise catalog/download/verify/extract/commit phase instead.
write_update_phase(){
  local state="$1" label="$2" detail="$3"
  [ "${UPDATE_MODE:-0}" = "1" ] || return 0
  write_update_status "$state" "$label" "$detail" 0 || true
  write_update_job "$state" "$label" "$detail" 0 || true
}
require_update_compatibility_tools(){
  # A beta.72+ binary must prove its real release-manifest capability. Older
  # Dash-Go builds use the installer’s bounded shell verifier for this one
  # bridge update; no system scripting runtime is required.
  if update_cli_supports '--verify-release-manifest'; then return 0; fi
  if updater_capability_query_is_safe; then
    warn "This version is too old to update automatically. Run ~/install.sh --repair --system first, then update."
    return 1
  fi
  say "One-time compatibility step: upgrading the installed updater to Go-native release verification."
  # Exported for the child updater bridge; it is intentionally not read in this shell.
  export DASH_UPDATE_LEGACY_BRIDGE=1
  return 0
}
write_updater_migration_receipt(){
  local arch
  update_cli_capabilities >/dev/null 2>&1 || return 1
  arch="$(uname -m 2>/dev/null || echo unknown)"
  update_cli --write-updater-migration --file "$UPDATER_MIGRATION_FILE" --previous-version "${UPDATE_PREVIOUS_VERSION:-}" --architecture "$arch" >/dev/null 2>&1
}
verify_go_updater_capabilities(){
  local feature
  for feature in --verify-release-manifest --release-file-list --purge-stale-managed --update-status --update-job --resolve-github-release; do
    update_cli_supports "$feature" || return 1
  done
}

start_update_logging(){
  mkdir -p "$LOG_DIR" "$CACHE_DIR" 2>/dev/null || true
  if [ "${DASH_UPDATE_LOG_ACTIVE:-0}" != "1" ]; then
    : > "$UPDATE_LOG_FILE" 2>/dev/null || true
    export DASH_UPDATE_LOG_ACTIVE=1
    # A runner explicitly sets DASH_UPDATE_SOURCE=control. Treat every other
    # invocation, including redirected SSH commands, as a terminal update.
    if [ -z "${DASH_UPDATE_SOURCE:-}" ]; then DASH_UPDATE_SOURCE="ssh"; export DASH_UPDATE_SOURCE; fi
    if [ -t 1 ]; then
      exec > >(tee -a "$UPDATE_LOG_FILE") 2>&1
    else
      exec >> "$UPDATE_LOG_FILE" 2>&1
    fi
  fi
}

acquire_dashboard_update_lock(){
  [ "${DASH_UPDATE_LOCK_HELD:-0}" = "1" ] && return 0
  command -v flock >/dev/null 2>&1 || { warn "Updater dependency missing: flock is required for the update lock."; return 1; }
  mkdir -p "$CACHE_DIR" 2>/dev/null || return 1
  # Keep this shell's descriptor open for the whole transaction so direct SSH
  # updates and the dedicated control runner cannot overlap.
  exec 9>"$CACHE_DIR/update.lock" || return 1
  if ! flock -n 9; then
    warn "another Dash-Go update transaction is already running"
    return 1
  fi
  DASH_UPDATE_LOCK_HELD=1; export DASH_UPDATE_LOCK_HELD
  return 0
}

read_dashboard_update_job_id(){
  update_cli --json-get "$CACHE_DIR/update-job.json" id 2>/dev/null || true
}

# A Control request writes its queued job before systemd's runner has acquired
# the shared flock. Direct SSH updates must honor that brief reservation or they
# could win the lock and replace the Control job/action identity. An aged record
# without a lock is abandoned state and is intentionally not a permanent block.
update_job_reservation_is_active(){
  local state updated now age
  update_cli_supports '--update-job' || return 1
  state="$(update_cli --json-get "$CACHE_DIR/update-job.json" state 2>/dev/null || true)"
  case "$state" in
    preflight|queued|starting|running|validating-payload|committing|checking-runtime|recycling-browser|post-verify-pending|rollback-requested) ;;
    *) return 1 ;;
  esac
  updated="$(update_cli --json-get "$CACHE_DIR/update-job.json" updatedAt 2>/dev/null || true)"
  case "$updated" in
    ''|*[!0-9]*) return 1 ;;
  esac
  now="$(date +%s)"
  age=$((now-updated))
  [ "$age" -ge 0 ] && [ "$age" -le 120 ]
}

# Detect non-interactive modes early. A normal update follows exactly one
# selected catalog: beta or stable. It never crosses tracks or overwrites user
# settings. Fresh installations default to stable; an explicit DASH_TRACK or
# --track value is saved for future unattended updates.
UPDATE_MODE=0
UPDATE_ROLLBACK_ONLY=0
UPDATE_ROLLBACK_STAGE=""
SELECTOR_INTEGRITY_BOOTSTRAP=0
UPDATE_PREVIOUS_VERSION=""
DASH_UPDATE_RUNTIME_ROLLBACK="${DASH_UPDATE_RUNTIME_ROLLBACK:-0}"
DASH_UPDATE_PAUSE_OWNED="${DASH_UPDATE_PAUSE_OWNED:-0}"
DASH_UPDATE_ROLLBACK_ATTEMPTED="${DASH_UPDATE_ROLLBACK_ATTEMPTED:-}"
DASH_UPDATE_ROLLBACK_SUCCEEDED="${DASH_UPDATE_ROLLBACK_SUCCEEDED:-}"
REPAIR_MODE=0
REPAIR_UPDATE_REQUESTED=0
DOCTOR_MODE=0
REMOVE_MODE=0
REMOVE_DRY_RUN=0
REMOVE_PRESERVE_REQUESTED=-1
REMOVE_PURGE_REQUESTED=0
REMOVE_KEEP_INSTALLER=0
REMOVE_REBOOT_AFTER=0
HELP_MODE=0
BUNDLE_INFO_MODE=0
RESET_PROFILE=0
# Repair tiers are deliberately opt-in. Plain --repair remains the narrow
# application-file recovery path; --system and --packages widen recovery.
REPAIR_SYSTEM=0
REPAIR_PACKAGES=0
REPAIR_FROM_DOCTOR=0
REPAIR_SYSTEM_ROOT=0
REPAIR_WARNINGS=""
DOCTOR_ARGS=()
UPDATE_TARGET="latest"
REPAIR_TARGET="latest"
DEFAULT_RELEASE_TRACK="stable"
TRACK_REQUEST=""

# Basic output helpers are declared before argument compatibility handling so
# unsupported legacy flags never turn into a shell-level command-not-found error.
say(){ printf '\n\033[1;36m== %s\033[0m\n' "$*"; }
warn(){ printf '\033[1;33m!! %s\033[0m\n' "$*"; }
ok(){ printf '\033[1;32m   %s\033[0m\n' "$*"; }

# Interactive setup reports its current high-level stage. This makes a
# screenshot or a Ctrl-C interruption useful without exposing noisy commands.
INSTALLER_STAGE="starting"
INSTALLER_STAGE_INDEX=0
INSTALLER_STAGE_TOTAL=0
INSTALLER_INTERRUPTED=0
INTERACTIVE_INSTALLER=0
INSTALLER_STAGE_NAMES=()
# Reliability controls are deliberately independent of Control PIN behavior.
# They protect setup work without changing dashboard-access policy.
INTERACTIVE_INSTALL_LOCK_HELD=0
SUDO_KEEPALIVE_PID=""
SYSTEM_REBOOT_RECOMMENDED=0

mark_reboot_recommended(){ SYSTEM_REBOOT_RECOMMENDED=1; }

stop_sudo_keepalive(){
  if [ -n "${SUDO_KEEPALIVE_PID:-}" ]; then
    kill "$SUDO_KEEPALIVE_PID" >/dev/null 2>&1 || true
    wait "$SUDO_KEEPALIVE_PID" 2>/dev/null || true
    SUDO_KEEPALIVE_PID=""
  fi
  return 0
}

start_sudo_keepalive(){
  local parent_pid
  if [ -n "${SUDO_KEEPALIVE_PID:-}" ] && kill -0 "$SUDO_KEEPALIVE_PID" 2>/dev/null; then
    return 0
  fi
  SUDO_KEEPALIVE_PID=""
  if ! $SUDO -v; then
    warn "Dash-Go needs administrator permission for the selected system steps. Enter your password when prompted, then run ~/install.sh again."
    return 1
  fi
  parent_pid="$$"
  (
    while kill -0 "$parent_pid" 2>/dev/null; do
      sleep 50
      $SUDO -n -v >/dev/null 2>&1 || exit 0
    done
  ) &
  SUDO_KEEPALIVE_PID="$!"
  return 0
}

interactive_run_needs_sudo(){
  [ "${DO_SYSTEM:-0}" = "1" ] || [ "${DO_PKGS:-0}" = "1" ] || [ "${DO_SERVICE:-0}" = "1" ] || \
    [ "${DO_AUTOLOGIN:-0}" = "1" ] || [ "${DO_AUTOSTART:-0}" = "1" ] || [ "${DO_SSH:-0}" = "1" ]
}

acquire_interactive_installer_lock(){
  local lock_file
  [ "$INTERACTIVE_INSTALL_LOCK_HELD" = "1" ] && return 0
  command -v flock >/dev/null 2>&1 || { warn "This system is missing flock, so Dash-Go cannot safely prevent two setup windows from changing the same dashboard."; return 1; }
  mkdir -p "$DASHGO_STATE_DIR" 2>/dev/null || { warn "Could not create Dash-Go's setup lock folder: $DASHGO_STATE_DIR"; return 1; }
  lock_file="$DASHGO_STATE_DIR/install.lock"
  # FD 7 stays open for this installer and is released automatically at exit.
  exec 7>"$lock_file" || { warn "Could not open Dash-Go's setup lock: $lock_file"; return 1; }
  if ! flock -n 7; then
    warn "Another Dash-Go setup is already running in this account. Let it finish, or close the other window, then try again."
    return 1
  fi
  INTERACTIVE_INSTALL_LOCK_HELD=1
  return 0
}

offer_reboot_after_system_changes(){
  local answer
  [ "$SYSTEM_REBOOT_RECOMMENDED" = "1" ] || return 0
  echo
  ok "Setup is complete. The kiosk starts on the next boot."
  if [ ! -t 0 ] || [ ! -t 1 ]; then
    echo "   Reboot later with: sudo reboot"
    return 0
  fi
  read -r -p "Reboot now to launch the dashboard? [Y/n] " answer || answer=n
  case "${answer:-y}" in
    y|Y|yes|YES) $SUDO reboot || { warn "Could not start a reboot. Reboot later with: sudo reboot"; return 1; } ;;
    *) echo "   Reboot later with: sudo reboot" ;;
  esac
}

installer_build_stage_plan(){
  INSTALLER_STAGE_NAMES=()
  [ "${DO_SYSTEM:-0}" = "1" ] && INSTALLER_STAGE_NAMES+=("Preparing the operating system")
  [ "${DO_PKGS:-0}" = "1" ] && INSTALLER_STAGE_NAMES+=("Installing dashboard components")
  [ "${DO_FILES:-0}" = "1" ] && INSTALLER_STAGE_NAMES+=("Installing Dash-Go")
  [ "${RESTORE_FROM_BACKUP:-0}" = "1" ] && INSTALLER_STAGE_NAMES+=("Restoring saved dashboard data")
  [ "${DO_CUSTOM:-0}" = "1" ] && INSTALLER_STAGE_NAMES+=("Setting up this dashboard")
  [ "${DO_ICAL:-0}" = "1" ] && INSTALLER_STAGE_NAMES+=("Adding public calendars")
  [ "${DO_VDIR:-0}" = "1" ] && INSTALLER_STAGE_NAMES+=("Connecting private calendars")
  [ "${DO_CALENDARS:-0}" = "1" ] && INSTALLER_STAGE_NAMES+=("Setting up built-in calendars")
  [ "${DO_SERVICE:-0}" = "1" ] && INSTALLER_STAGE_NAMES+=("Starting Dashboard Control")
  [ "${DO_PIN:-0}" = "1" ] && INSTALLER_STAGE_NAMES+=("Setting the dashboard PIN")
  [ "${DO_AUTOLOGIN:-0}" = "1" ] && INSTALLER_STAGE_NAMES+=("Preparing automatic sign-in")
  [ "${DO_AUTOSTART:-0}" = "1" ] && INSTALLER_STAGE_NAMES+=("Preparing automatic startup")
  [ "${DO_SSH:-0}" = "1" ] && INSTALLER_STAGE_NAMES+=("Preparing remote access")
  INSTALLER_STAGE_NAMES+=("Checking the finished dashboard")
  INSTALLER_STAGE_TOTAL=${#INSTALLER_STAGE_NAMES[@]}
  INSTALLER_STAGE_INDEX=0
}

installer_stage(){
  local label="$1"
  INSTALLER_STAGE_INDEX=$((INSTALLER_STAGE_INDEX + 1))
  INSTALLER_STAGE="step ${INSTALLER_STAGE_INDEX}/${INSTALLER_STAGE_TOTAL}: ${label}"
  say "Step ${INSTALLER_STAGE_INDEX}/${INSTALLER_STAGE_TOTAL} — ${label}"
}

installer_interrupt(){
  INSTALLER_INTERRUPTED=1
  printf '\n'
  warn "Install interrupted at ${INSTALLER_STAGE}. Your existing dashboard files and settings were left in place where possible. Run ~/install.sh again anytime to continue safely."
  exit 130
}

installer_exit_summary(){
  local code=$?
  trap - EXIT
  stop_sudo_keepalive
  if [ "$INTERACTIVE_INSTALLER" = "1" ] && [ "$code" -ne 0 ] && [ "$INSTALLER_INTERRUPTED" != "1" ]; then
    warn "Install stopped at ${INSTALLER_STAGE}. Existing settings were preserved where possible; run ~/install.sh again to resume or choose a smaller repair action."
  fi
  exit "$code"
}

# Keep optional installer stages resilient without pretending that a required
# configuration write succeeded.  Stages may continue, but failures are named
# again in the final recap so a re-run has an obvious target.
INSTALL_STEP_FAILURES=()
mark_install_step_failed(){
  local label="$1" detail="${2:-}"
  INSTALL_STEP_FAILURES+=("${label}${detail:+ — $detail}")
  warn "$label did not complete${detail:+: $detail}"
}


normalize_release_track(){
  local raw
  raw="$(printf '%s' "${1:-}" | tr '[:upper:]' '[:lower:]')"
  case "$raw" in
    beta|stable) printf '%s\n' "$raw" ;;
    *) return 1 ;;
  esac
}

# A release bundle contains the installer plus the exact validated app payload.
# Its semantic version is authoritative for local/SSH device rehearsal so an
# extracted beta archive cannot accidentally persist a stable update track (or
# vice versa). An installed standalone ~/install.sh has no sibling app/ tree
# and therefore keeps normal saved/default track behavior.
bundle_release_version(){
  local version=""
  [ -r "$INSTALLER_SOURCE_DIR/app/VERSION" ] || return 1
  version="$(tr -d '\r\n[:space:]' < "$INSTALLER_SOURCE_DIR/app/VERSION" 2>/dev/null || true)"
  [[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-beta\.[0-9]+)?$ ]] || return 1
  printf '%s\n' "$version"
}

bundle_release_track(){
  local version
  version="$(bundle_release_version 2>/dev/null || true)"
  [ -n "$version" ] || return 1
  case "$version" in
    *-beta.*) printf 'beta\n' ;;
    *) printf 'stable\n' ;;
  esac
}

infer_release_track(){
  local installed="" bundled_track=""
  [ -f "$DASH/VERSION" ] && installed="$(tr -d '\r\n' < "$DASH/VERSION" 2>/dev/null || true)"
  case "$installed" in
    *-beta*|*-alpha*|*-rc*) printf 'beta\n'; return 0 ;;
    [0-9]*.[0-9]*.[0-9]*) printf 'stable\n'; return 0 ;;
  esac
  bundled_track="$(bundle_release_track 2>/dev/null || true)"
  if [ -n "$bundled_track" ]; then
    printf '%s\n' "$bundled_track"
    return 0
  fi
  printf '%s\n' "$DEFAULT_RELEASE_TRACK"
}

resolve_release_track(){
  local requested="${DASH_TRACK:-}" resolved="" bundled_track=""
  if [ -n "$requested" ]; then
    resolved="$(normalize_release_track "$requested" 2>/dev/null || true)"
    if [ -z "$resolved" ]; then
      warn "unsupported DASH_TRACK '$requested'; using the installed/default track instead"
    fi
  fi
  bundled_track="$(bundle_release_track 2>/dev/null || true)"
  if [ -n "$bundled_track" ]; then
    if [ -n "$resolved" ] && [ "$resolved" != "$bundled_track" ]; then
      warn "the local Dash-Go release bundle is $bundled_track; using its exact bundled track instead of '$resolved'"
    fi
    resolved="$bundled_track"
  fi
  [ -n "$resolved" ] || resolved="$(infer_release_track)"
  RELEASE_TRACK="$resolved"
  DASH_TRACK="$RELEASE_TRACK"
  export DASH_TRACK RELEASE_TRACK
}

show_local_release_bundle_info(){
  local version track app="$INSTALLER_SOURCE_DIR/app"
  version="$(bundle_release_version 2>/dev/null || true)"
  track="$(bundle_release_track 2>/dev/null || true)"
  [ -n "$version" ] && [ -n "$track" ] || { echo "ERROR: this installer is not inside a valid Dash-Go release bundle" >&2; return 1; }
  [ -f "$app/index.html" ] || { echo "ERROR: local release bundle is missing app/index.html" >&2; return 1; }
  [ -f "$app/manifest.json" ] || { echo "ERROR: local release bundle is missing app/manifest.json" >&2; return 1; }
  printf 'Dash-Go local release bundle\nVersion: %s\nTrack: %s\nPayload: validated app tree present\n' "$version" "$track"
}

# beta.6's standalone updater predates selector-integrity protection and can
# rewrite a manifest-owned selector after a successful payload transaction.
# This one-time bridge copies the reviewed current installer to ~/install.sh,
# then immediately runs the beta update through the corrected updater. It is
# intentionally local-only: it never downloads or accepts a different script.
bootstrap_selector_integrity_updater(){
  local source="$INSTALLER_SOURCE_DIR/install.sh" target="$INSTALLER" tmp track
  [ -r "$source" ] || { echo "ERROR: selector-integrity bootstrap must run from an extracted Dash-Go release bundle" >&2; return 1; }
  track="$(bundle_release_track 2>/dev/null || true)"
  case "$track" in
    beta|stable) ;;
    *) echo "ERROR: selector-integrity bootstrap requires a release bundle with a valid beta or stable track" >&2; return 1 ;;
  esac
  mkdir -p "$(dirname "$target")" || return 1
  tmp="${target}.selector-integrity.tmp.$$"
  (umask 077; cp "$source" "$tmp") || { rm -f "$tmp"; return 1; }
  chmod 700 "$tmp" 2>/dev/null || { rm -f "$tmp"; return 1; }
  mv -f "$tmp" "$target" || { rm -f "$tmp"; return 1; }
  cmp -s "$source" "$target" || { echo "ERROR: selector-integrity bootstrap could not verify $target" >&2; return 1; }
  ok "Selector-integrity updater bridge installed at $target"
  if [ "${DASHGO_SELECTOR_BRIDGE_DRY_RUN:-0}" = 1 ]; then
    return 0
  fi
  echo "Starting the corrected $track updater."
  exec "$target" --update --track "$track"
}

case "${1:-}" in
  help|--help|-h) HELP_MODE=1;;
  --bundle-info) BUNDLE_INFO_MODE=1;;
  --bootstrap-selector-integrity) SELECTOR_INTEGRITY_BOOTSTRAP=1;;
  update|--update) UPDATE_MODE=1;;
  repair|--repair|--repair-install) REPAIR_MODE=1;;
  --rollback-update) UPDATE_ROLLBACK_ONLY=1; UPDATE_ROLLBACK_STAGE="${2:-}"; shift || true;;
  doctor|--doctor|health|--health|check|--check) DOCTOR_MODE=1; DOCTOR_ARGS=("${@:2}");;
  remove|--remove|uninstall|--uninstall) REMOVE_MODE=1;;
esac
expect_track_value=0
for arg in "$@"; do
  if [ "$expect_track_value" = 1 ]; then
    TRACK_REQUEST="$arg"
    expect_track_value=0
    continue
  fi
  case "$arg" in
    remove|--remove|uninstall|--uninstall) REMOVE_MODE=1;;
    update|--update) [ "$REPAIR_MODE" = "1" ] && REPAIR_UPDATE_REQUESTED=1;;
    --reset-profile) RESET_PROFILE=1;;
    --system) REPAIR_SYSTEM=1;;
    --packages) REPAIR_SYSTEM=1; REPAIR_PACKAGES=1;;
    --full) [ "$REPAIR_MODE" = "1" ] && { REPAIR_SYSTEM=1; REPAIR_PACKAGES=1; };;
    --from-doctor) REPAIR_SYSTEM=1; REPAIR_FROM_DOCTOR=1;;
    --dry-run) REMOVE_DRY_RUN=1;;
    --preserve) REMOVE_PRESERVE_REQUESTED=1;;
    --purge) REMOVE_PURGE_REQUESTED=1;;
    --keep-installer) REMOVE_KEEP_INSTALLER=1;;
    --reboot) REMOVE_REBOOT_AFTER=1;;
    --track) expect_track_value=1;;
    --track=*) TRACK_REQUEST="${arg#--track=}";;
    --version|version|track|swap|--swap)
      warn_pending_args="${warn_pending_args:-} $arg";;
  esac
done
if [ "$expect_track_value" = 1 ]; then
  warn "--track needs beta or stable; using the installed/default track"
fi
[ -n "$TRACK_REQUEST" ] && DASH_TRACK="$TRACK_REQUEST"
resolve_release_track
if [ "$REPAIR_MODE" = "1" ] && [ "$REPAIR_UPDATE_REQUESTED" != "1" ]; then
  REPAIR_TARGET="$(installed_dashboard_version)"
  if ! [[ "$REPAIR_TARGET" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-beta\.[0-9]+)?$ ]]; then
    warn "plain --repair requires a valid installed Dash-Go version; use --repair --update to request the newest eligible release"
    exit 1
  fi
fi

if [ "$BUNDLE_INFO_MODE" = "1" ]; then
  show_local_release_bundle_info
  exit $?
fi

if [ "$SELECTOR_INTEGRITY_BOOTSTRAP" = "1" ]; then
  bootstrap_selector_integrity_updater
  exit $?
fi

if [ -n "${warn_pending_args:-}" ]; then
  warn "version-target arguments are no longer supported; using the latest release in the selected $RELEASE_TRACK track"
fi

dashboard_demo_mode_detected(){
  [ -f "$DEMO_STATUS_FILE" ] && return 0
  [ -f "$CONFIG_DIR/config.local.js" ] && grep -q 'demoMode[[:space:]]*:[[:space:]]*true' "$CONFIG_DIR/config.local.js" 2>/dev/null && return 0
  return 1
}

prompt_demo_mode_reset(){
  [ "$UPDATE_MODE" = "1" ] && return 0
  [ "$REPAIR_MODE" = "1" ] && return 0
  [ "$DOCTOR_MODE" = "1" ] && return 0
  [ "$REMOVE_MODE" = "1" ] && return 0
  [ "$HELP_MODE" = "1" ] && return 0
  dashboard_demo_mode_detected || return 0
  while :; do
    echo
    warn "This dashboard is currently in DEMO MODE."
    echo "Demo Mode uses Chicago sample calendars, demo messages, and key-free weather settings."
    echo "  1) Keep Demo Mode and continue to the selected installer action (default)"
    echo "  2) Remove all Demo Mode data (including calendar data and demo messages), then run fresh setup"
    echo "  3) Completely remove Dash-Go (same flow as install.sh --remove), then restart"
    echo "  4) Exit"
    read -rp "Choose [1/2/3/4, Enter=1]: " dmode
    dmode="${dmode:-1}"
    case "$dmode" in
      1)
        ok "Keeping Demo Mode; continuing to the selected installer action"
        return 0
        ;;
      2)
        DEMO_RESET_REQUESTED=1
        DEMO_WIPE_CALENDARS_REQUESTED=1
        warn "All Demo Mode data, demo messages, and calendar data will be removed before setup continues"
        return 0
        ;;
      3)
        REMOVE_MODE=1
        warn "Dash-Go full removal selected; this will use the install.sh --remove flow"
        return 0
        ;;
      4|q|Q|quit|exit)
        ok "installer closed without changing Demo Mode"
        exit 0
        ;;
      *) warn "Choose 1, 2, 3, or 4. No Demo Mode data was changed.";;
    esac
  done
}

run_demo_mode_cli_for_installer(){
  # The Go helper returns structured diagnostics on stdout/stderr. Keep the
  # interactive installer readable while retaining exact details for repair.
  local demo_log="$LOG_DIR/demo-mode-install.log"
  mkdir -p "$LOG_DIR" 2>/dev/null || true
  "$BIN_DIR/dashboard-control-server" "$@" >>"$demo_log" 2>&1
}

reset_demo_mode_if_requested(){
  [ "${DEMO_RESET_REQUESTED:-0}" = "1" ] || return 0
  say "Resetting Demo Mode"
  if [ -x "$BIN_DIR/dashboard-control-server" ]; then
    if [ "${DEMO_WIPE_CALENDARS_REQUESTED:-0}" = "1" ]; then
      run_demo_mode_cli_for_installer --setup-demo-mode --clear --wipe-calendars || warn "Demo Mode reset helper reported an issue (details: $LOG_DIR/demo-mode-install.log)"
    else
      run_demo_mode_cli_for_installer --setup-demo-mode --clear || warn "Demo Mode reset helper reported an issue (details: $LOG_DIR/demo-mode-install.log)"
    fi
  else
    mkdir -p "$CACHE_DIR" "$CAL_DIR" "$CONFIG_DIR"
    ts="$(date +%Y%m%d-%H%M%S)"
    if [ -d "$CONFIG_DIR" ] || [ -d "$CAL_DIR" ]; then
      mkdir -p "$CACHE_DIR/demo-backups"
      tar -czf "$CACHE_DIR/demo-backups/demo-reset-$ts.tar.gz" -C "$DASH" config calendars cache/demo-mode.json cache/events.cache.json cache/events.cache.meta.json 2>/dev/null || true
    fi
    rm -f "$CAL_DIR"/demo-*.ics "$CACHE_DIR/demo-mode.json" "$CACHE_DIR/events.cache.json" "$CACHE_DIR/events.cache.meta.json"
    if [ "${DEMO_WIPE_CALENDARS_REQUESTED:-0}" = "1" ]; then
      rm -rf -- "${CAL_DIR:?}/"*
      mkdir -p "$CAL_DIR"
      rm -f "$CACHE_DIR"/events.cache.json "$CACHE_DIR"/events.cache.meta.json "$CACHE_DIR"/events.json "$CACHE_DIR"/events-cache.json
    fi
    if [ -f "$CONFIG_DIR/config.local.js" ] && grep -q 'demoMode[[:space:]]*:[[:space:]]*true' "$CONFIG_DIR/config.local.js" 2>/dev/null; then
      rm -f "$CONFIG_DIR/config.local.js"
    fi
  fi
  DO_CUSTOM=1; DO_WEATHER_DISPLAY=1; DO_WEATHER=1; DO_RADAR=1; DO_MESSAGE_SOURCES=1; DO_CALENDARS=1
  if [ "${DEMO_WIPE_CALENDARS_REQUESTED:-0}" = "1" ]; then
    ok "Demo Mode and calendar data reset. Continue through setup to write real settings and calendars."
  else
    ok "Demo Mode reset. Continue through setup to write real settings."
  fi
}

enable_demo_mode(){
  say "Enabling Demo Mode"
  if [ -x "$BIN_DIR/dashboard-control-server" ]; then
    run_demo_mode_cli_for_installer --setup-demo-mode || {
      warn "Demo Mode helper reported an issue (details: $LOG_DIR/demo-mode-install.log)"
      return 1
    }
  else
    warn "Demo Mode helper is missing. Run Update the app first, then choose Demo Mode again."
    return 1
  fi
  ok "Demo Mode seeded: Chicago location, sample ICS calendars, and demo messages"
}

installer_demo_defaults_active(){
  [ "${DO_DEMO:-0}" = "1" ]
}

installer_demo_prompt_defaults_active(){
  [ "${DO_DEMO:-0}" = "1" ] && return 0
  dashboard_demo_mode_detected && return 0
  return 1
}

demo_default_note(){
  printf '\033[1;32m   Demo Mode default: %s\033[0m\n' "$*"
}


# /api/ready is intentionally tiny and public: it proves that the loopback
# Go runtime is bound and serving the expected payload version without opening
# Dashboard Control status or settings before authentication.  It is the
# critical readiness contract for kiosk relaunch and post-update rollback.
dashboard_api_ready_for_version(){
  local expected="$1" status
  [ -n "$expected" ] || return 1
  status="$(curl -fsS --max-time 3 http://127.0.0.1:8090/api/ready 2>/dev/null || true)"
  [ -n "$status" ] || return 1
  printf '%s' "$status" | grep -q '"goServer"[[:space:]]*:[[:space:]]*true' || return 1
  printf '%s' "$status" | grep -Eq "\"version\"[[:space:]]*:[[:space:]]*\"$expected\""
}

# Confirm the live server.  Update/rollback callers pass an expected version so
# an old process answering during an atomic payload swap cannot be mistaken for
# the newly committed runtime.  Legacy setup callers retain the permissive
# header/status fallback until their first beta.18 update adds /api/ready.
dashboard_server_confirm_live(){
  command -v curl >/dev/null 2>&1 || return 1
  local expected="${1:-}" i hdr status tries
  tries="${DASH_SERVER_READY_WAIT_SECONDS:-45}"
  case "$tries" in *[!0-9]*|'') tries=45;; esac
  for i in $(seq 1 "$tries" 2>/dev/null || echo 1); do
    if [ -n "$expected" ]; then
      dashboard_api_ready_for_version "$expected" && return 0
    else
      hdr="$(curl -sI --max-time 3 http://127.0.0.1:8090/ 2>/dev/null || true)"
      if printf '%s\n' "$hdr" | grep -qi '^Cache-Control:'; then
        return 0
      fi
      status="$(curl -fsS --max-time 3 http://127.0.0.1:8090/api/status 2>/dev/null || true)"
      if printf '%s' "$status" | grep -q '"goServer"[[:space:]]*:[[:space:]]*true'; then
        return 0
      fi
    fi
    sleep 1
  done
  return 1
}

dashboard_server_failure_hint(){
  if command -v systemctl >/dev/null 2>&1; then
    if ! systemctl is-active dashboard-server.service >/dev/null 2>&1; then
      warn "dashboard-server.service is not active; run: sudo journalctl -u dashboard-server -n 80 --no-pager"
      return 0
    fi
  fi
  warn "server did not confirm as dashboard-control-server on 127.0.0.1:8090"
  if command -v ss >/dev/null 2>&1; then
    local owners
    owners="$(ss -ltnp 2>/dev/null | awk '$4 ~ /:8090$/ {print}' || true)"
    [ -n "$owners" ] && warn "port 8090 listener: $owners"
  fi
}

installer_final_dashboard_check(){
  local ip
  installer_stage "Checking the finished dashboard"
  say "Final dashboard check"

  # A focused configuration-only action does not necessarily start/restart the
  # service. For an install, update, or service action, readiness is proof of
  # life and must pass before the installer says it is done.
  if [ "$DO_SERVICE" = "1" ] || [ "$DO_FILES" = "1" ]; then
    if ! dashboard_server_confirm_live; then
      warn "Dash-Go did not confirm that it is running. Running Doctor now; the installer will not report success."
      if [ -x "$BIN_DIR/doctor.sh" ]; then
        bash "$BIN_DIR/doctor.sh" || true
      else
        warn "Doctor is not available yet. Run ~/install.sh --repair --system after checking the warnings above."
      fi
      return 1
    fi
    ok "Dashboard is running locally: http://localhost:8090"
    if command -v systemctl >/dev/null 2>&1 && systemctl is-enabled dashboard-server.service >/dev/null 2>&1; then
      ok "Dashboard Control will start automatically on boot"
    else
      warn "Dashboard Control is running now, but automatic startup was not confirmed. Run ~/install.sh and choose Dashboard service."
    fi
    ip="$(hostname -I 2>/dev/null | awk '{print $1}')"
    [ -n "$ip" ] && echo "   Device address for SSH: $USER_NAME@$ip"
  else
    ok "Selected settings were saved. No dashboard-server restart was needed."
  fi
  echo "Next: open Dashboard Control on the kiosk (triple-tap the moon icon) to add calendars or change any default."
  return 0
}

show_install_help(){
  cat <<'HELP'
Dash-Go installer

Usage:
  ./install.sh                         Open the interactive setup menu
  ./install.sh --help                  Show this help and exit

Update, repair, and health commands:
  ./install.sh --update                Update within the saved beta/stable track
  ./install.sh --update --latest       Alias for --update
  ./install.sh --update --track beta   Explicitly use and save the beta track
  ./install.sh --update --track stable Explicitly use and save the stable track
  ./install.sh --repair                Restore the exact installed release while preserving personal settings and calendars
  ./install.sh --repair --update       Repair using the newest eligible GitHub Release
  ./install.sh --repair --reset-profile
                                       Refresh app files and explicitly restore detected profile defaults
  ./install.sh --repair --system       Also recover service, autologin, kiosk wiring, and scheduler
  ./install.sh --repair --system --packages
                                       Also install missing runtime packages (requires network + sudo)
  ./install.sh --repair --full         Alias for --repair --system --packages
  ./install.sh --repair --from-doctor  Limit system recovery to current Doctor findings when possible
  ./install.sh --doctor                Quiet scan, then offer Fix all
  ./install.sh --doctor --full         Detailed scan with individual fixes
  ./install.sh --bundle-info           Inspect this extracted local release bundle; changes nothing
  ./install.sh --bootstrap-selector-integrity
                                       One-time beta.6 updater bridge from a verified local release bundle

Removal commands:
  ./install.sh --remove                Offline Dash-Go uninstall (interactive)
  ./install.sh --remove --dry-run      Show project-owned artifacts; change nothing
  ./install.sh --remove --preserve     Require a verified private recovery archive
  ./install.sh --remove --purge        Confirm removal of Dash-Go data/credentials
  ./install.sh --remove --keep-installer
                                       Leave ~/install.sh after a successful uninstall
  ./install.sh --remove --reboot       Reboot after verified teardown

Interactive menu:
  Run ./install.sh with no command. The menu is safe to re-run and includes
  focused actions for updates, dashboard setup, Control PIN, remote access,
  notifications, terminal access, Demo Mode, and Exit. Type q, quit, or exit
  at the main menu to leave without changes.

  Notifications (Apprise-Go) configure private outbound delivery routes through
  SSH. Destination URLs never appear in Dashboard Control or normal logs.

Environment variables:
  DASH_TRACK      beta or stable; defaults to the installed/default stable track.
                  When this installer is inside a versioned local release bundle,
                  the bundle's own beta/stable version is authoritative.
  DASH_RADAR_ENV  optional override for the private radar-key file
  PROFILE         lite, balanced, or enhanced (legacy maximum resolves to enhanced)

Local device rehearsal:
  A validated Dash-Go GitHub Release bundle contains install.sh and its app payload.
  On a private-repository or offline test device, transfer the builder's four assets
  over SSH, verify SHA256SUMS, extract Dash-Go_*_release.tar.gz, then run this
  installer. Existing Dash-Go devices use --update; a fresh device uses the normal
  interactive Full install flow. No GitHub token belongs on the device.

Run as the normal kiosk user, not root. The installer uses sudo only for
system-level setup steps.
HELP
}

if [ "$HELP_MODE" = "1" ]; then
  show_install_help
  exit 0
fi

# This installer must be run as the NON-ROOT user that will own and run the
# kiosk (its files, autologin, autostart, and cron are all set up for that
# user). It uses sudo only for the few system-level steps. Running as root
# would put everything under /root and create the service for root, which is
# not what you want. Starting it as `sudo ./install.sh` has the same harmful
# effect, so catch both variants before any interactive question is asked.
if [ "$(id -u)" -eq 0 ] || [ -n "${SUDO_USER:-}" ]; then
  echo "ERROR: Dash-Go must be started as the normal kiosk user, not with sudo."
  if [ -n "${SUDO_USER:-}" ]; then
    echo "You started this with sudo. Log in as ${SUDO_USER} and run:"
  else
    echo "You are currently root. Log in as the normal kiosk user and run:"
  fi
  echo "    ~/install.sh"
  echo "Dash-Go will ask for sudo only when a system-level step needs it."
  exit 1
fi
if [ "$UPDATE_MODE" != "1" ] && [ "$REPAIR_MODE" != "1" ] && [ "$DOCTOR_MODE" != "1" ] && ! command -v sudo >/dev/null 2>&1; then
  echo "ERROR: 'sudo' is not installed, but it is required for the system"
  echo "steps (web service, autologin, optional trim)."
  echo "(Unattended update and doctor shortcut modes do NOT need sudo just to start.)"
  echo "Install it first (as root):  apt install sudo"
  echo "then add your user to the sudo group:  usermod -aG sudo <user>"
  exit 1
fi
SUDO="sudo"

# APT can legitimately be busy with Debian's own scheduled maintenance. Use a
# bounded lock wait for installer-owned package work so a novice sees one clear
# retry message rather than a raw dpkg lock failure. Dashboard Control keeps
# its pre-existing exact sudoers command shape in dashboard-system-update.sh.
APT_LOCK_TIMEOUT_SECONDS="${DASHGO_APT_LOCK_TIMEOUT_SECONDS:-180}"
apt_managed_as(){
  local apt_sudo="$1" rc
  shift
  case "$APT_LOCK_TIMEOUT_SECONDS" in ''|*[!0-9]*) APT_LOCK_TIMEOUT_SECONDS=180;; esac
  if "$apt_sudo" env DEBIAN_FRONTEND=noninteractive apt-get -o "DPkg::Lock::Timeout=$APT_LOCK_TIMEOUT_SECONDS" "$@"; then
    return 0
  fi
  rc=$?
  warn "APT could not finish after waiting ${APT_LOCK_TIMEOUT_SECONDS} seconds for background system updates. Try again in a few minutes."
  return "$rc"
}
apt_managed(){ apt_managed_as "$SUDO" "$@"; }

# --- Platform detection -------------------------------------------------
# The dashboard app itself is portable, but the appliance setup has a few
# platform-specific pieces:
#   * Raspberry Pi: optional /boot/config.txt watchdog, Pi-friendly trim.
#   * Debian x86/Trixie: install/check the full X11 + LightDM + LXDE stack;
#     skip Pi boot config and Pi-style de-bloat unless explicitly requested.
bootstrap_read_device_model(){
  local f
  for f in /proc/device-tree/model /sys/firmware/devicetree/base/model; do
    if [ -r "$f" ]; then
      tr -d '\0' < "$f" 2>/dev/null
      return 0
    fi
  done
  awk -F: '/^(Model|Hardware|model name)[[:space:]]*:/{sub(/^[[:space:]]+/,"",$2); print $2; exit}' /proc/cpuinfo 2>/dev/null || true
}

bootstrap_load_os_release(){
  OS_ID="unknown"; OS_CODENAME=""; OS_VERSION_ID=""
  if [ -r /etc/os-release ]; then
    # shellcheck disable=SC1091
    . /etc/os-release
    OS_ID="${ID:-unknown}"
    OS_CODENAME="${VERSION_CODENAME:-}"
    OS_VERSION_ID="${VERSION_ID:-}"
  fi
}

# Dash-Go supports current Debian-family installations deliberately rather than
# treating every /etc/os-release value as interchangeable. Bookworm is kept as
# a compatibility target, Trixie is the recommended base, and newer releases
# stay conservative: ordinary kiosk setup works but Dash-Go avoids unreviewed
# firmware/display changes. Bullseye and older are maintenance-only so a fresh
# installer cannot begin changing an end-of-life operating system.
bootstrap_classify_os_support(){
  local major="${OS_VERSION_ID%%.*}"
  OS_SUPPORT_LEVEL="unsupported"
  OS_SUPPORT_LABEL="${OS_ID:-unknown} ${OS_CODENAME:-unknown}"
  OS_SUPPORT_HINT="Install Raspberry Pi OS Bookworm or Trixie, then run ~/install.sh again."
  case "$OS_ID" in
    debian|raspbian) ;;
    *) return 0 ;;
  esac
  case "$OS_CODENAME" in
    bookworm)
      OS_SUPPORT_LEVEL="supported"
      OS_SUPPORT_LABEL="${OS_ID} Bookworm — supported compatibility mode"
      OS_SUPPORT_HINT="Bookworm is supported. Trixie is recommended for new devices."
      ;;
    trixie)
      OS_SUPPORT_LEVEL="recommended"
      OS_SUPPORT_LABEL="${OS_ID} Trixie — recommended"
      OS_SUPPORT_HINT="Trixie is the recommended Dash-Go base."
      ;;
    bullseye|buster|stretch|jessie|wheezy)
      OS_SUPPORT_LEVEL="unsupported"
      OS_SUPPORT_LABEL="${OS_ID} ${OS_CODENAME} — unsupported for a fresh Dash-Go install"
      OS_SUPPORT_HINT="Upgrade this device to Bookworm or Trixie, then run ~/install.sh again."
      ;;
    *)
      case "$major" in
        ''|*[!0-9]*)
          OS_SUPPORT_LEVEL="conservative"
          OS_SUPPORT_LABEL="${OS_ID} ${OS_CODENAME:-newer release} — conservative mode"
          OS_SUPPORT_HINT="Dash-Go will avoid unreviewed distribution-specific system changes."
          ;;
        0|1|2|3|4|5|6|7|8|9|10|11)
          OS_SUPPORT_LEVEL="unsupported"
          OS_SUPPORT_LABEL="${OS_ID} ${OS_CODENAME:-$OS_VERSION_ID} — unsupported for a fresh Dash-Go install"
          OS_SUPPORT_HINT="Upgrade this device to Bookworm or Trixie, then run ~/install.sh again."
          ;;
        *)
          OS_SUPPORT_LEVEL="conservative"
          OS_SUPPORT_LABEL="${OS_ID} ${OS_CODENAME:-$OS_VERSION_ID} — conservative mode"
          OS_SUPPORT_HINT="Dash-Go will avoid unreviewed distribution-specific system changes."
          ;;
      esac
      ;;
  esac
}

bootstrap_detect_platform(){
  DASH_ARCH="$(uname -m 2>/dev/null || echo unknown)"
  DEVICE_MODEL="$(bootstrap_read_device_model)"
  bootstrap_load_os_release
  IS_PI=0; IS_DEBIAN=0; IS_X86=0
  case "$DEVICE_MODEL" in *"Raspberry Pi"*) IS_PI=1;; esac
  case "$OS_ID" in debian|raspbian) IS_DEBIAN=1;; esac
  bootstrap_classify_os_support
  case "$DASH_ARCH" in x86_64|amd64|i386|i686) IS_X86=1;; esac
  if [ "$IS_PI" = "1" ]; then
    PLATFORM_LABEL="Raspberry Pi OS ${OS_CODENAME:-unknown} (${DEVICE_MODEL:-unknown model}, $DASH_ARCH)"
  elif [ "$IS_DEBIAN" = "1" ] && [ "$IS_X86" = "1" ]; then
    PLATFORM_LABEL="Debian ${OS_CODENAME:-unknown} x86 ($DASH_ARCH)"
  elif [ "$IS_X86" = "1" ]; then
    PLATFORM_LABEL="x86 Linux ($OS_ID $OS_CODENAME)"
  else
    PLATFORM_LABEL="Linux device ($OS_ID $OS_CODENAME, $DASH_ARCH)"
  fi
}

bootstrap_detect_xsession(){
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

bootstrap_mem_total_mb(){
  awk '/^MemTotal:/ {printf "%d\n", int(($2+1023)/1024); exit}' /proc/meminfo 2>/dev/null || echo 0
}

bootstrap_cpu_count(){
  local n
  n="$(getconf _NPROCESSORS_ONLN 2>/dev/null || nproc 2>/dev/null || echo 1)"
  printf '%s\n' "${n:-1}"
}

bootstrap_classify_device_profile(){
  local model arch mem cpu
  model="${DEVICE_MODEL:-$(bootstrap_read_device_model)}"
  arch="${DASH_ARCH:-$(uname -m 2>/dev/null || echo unknown)}"
  mem="$(bootstrap_mem_total_mb)"
  cpu="$(bootstrap_cpu_count)"
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

bootstrap_dashboard_profile(){
  local p f
  p="${PROFILE:-}"
  if [ -z "$p" ]; then
    f="$CONFIG_DIR/config.local.js"
    if [ -f "$f" ]; then
      p="$(sed -nE 's/.*profile[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/p' "$f" | head -1 | tr '[:upper:]' '[:lower:]')"
    fi
  fi
  [ -n "$p" ] || p="$(bootstrap_classify_device_profile)"
  p="$(printf '%s\n' "$p" | tr '[:upper:]' '[:lower:]')"
  case "$p" in maximum) p="enhanced";; standard|default) p="balanced";; zero2|low|low-power) p="lite";; esac
  printf '%s\n' "$p"
}

bootstrap_profile_prefers_openbox_session(){
  case "$(bootstrap_dashboard_profile)" in lite|zero2|low|low-power|balanced) return 0;; *) return 1;; esac
}

# Runtime helper names used by later installer sections. Keep these in the
# standalone installer too; fresh x86/Debian installs cannot rely on
# bin/dashboard-common.sh existing until after the app payload downloads.
detect_xsession(){
  bootstrap_detect_xsession
}

profile_prefers_openbox_session(){
  bootstrap_profile_prefers_openbox_session
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

# LightDM reads vendor drop-ins, then /etc drop-ins, then lightdm.conf. Keep
# our readback in that order so diagnostics report the effective last value.
lightdm_config_value(){
  local key="$1" f value="" found
  for f in /usr/share/lightdm/lightdm.conf.d/*.conf /etc/lightdm/lightdm.conf.d/*.conf /etc/lightdm/lightdm.conf; do
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
  xs="/usr/share/xsessions/${session_name}.desktop"
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
  dir=/usr/share/xsessions
  xs="$dir/dashboard-openbox.desktop"
  compat="$dir/dashboard-lite.desktop"
  script="$BIN_DIR/dashboard-lite-session.sh"
  [ -x "$script" ] || { warn "dashboard-lite-session.sh is missing or not executable; cannot enable dashboard-openbox"; return 1; }
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
  ok "Dash-Go Openbox X session installed ($xs; compatibility alias $compat)"
}

write_dashboard_lite_xsession(){ write_dashboard_openbox_xsession; }

# Keep the dashboard-owned autologin decision in one root-owned /etc drop-in.
# Older installers wrote these three keys into lightdm.conf, which is read
# after drop-ins and could silently override newer managed settings. Migrate
# those keys only when configuring dashboard autologin again.
write_dashboard_lightdm_autologin(){
  local user_name="$1" session_name="$2" conf_dir conf root_conf
  valid_lightdm_user "$user_name" || { warn "invalid LightDM autologin user: $user_name"; return 1; }
  id "$user_name" >/dev/null 2>&1 || { warn "LightDM autologin user does not exist: $user_name"; return 1; }
  lightdm_dashboard_xsession_ok "$session_name" || { warn "Dash-Go X session '$session_name' is not ready; autologin was not changed"; return 1; }
  conf_dir=/etc/lightdm/lightdm.conf.d
  conf="$conf_dir/90-dash-go-autologin.conf"
  root_conf=/etc/lightdm/lightdm.conf
  $SUDO mkdir -p "$conf_dir" || return 1
  if [ -f "$root_conf" ] && $SUDO grep -Eq '^[[:space:]]*autologin-(user|user-timeout|session)[[:space:]]*=' "$root_conf"; then
    $SUDO cp -p "$root_conf" "$root_conf.dash-go-autologin.bak" 2>/dev/null || true
    $SUDO sed -i \
      -e '/^[[:space:]]*autologin-user[[:space:]]*=/d' \
      -e '/^[[:space:]]*autologin-user-timeout[[:space:]]*=/d' \
      -e '/^[[:space:]]*autologin-session[[:space:]]*=/d' \
      "$root_conf" || return 1
  fi
  cat <<EOFLIGHTDM | $SUDO tee "$conf" >/dev/null || return 1
# Managed by Dash-Go. This file owns only the dashboard autologin choice.
[Seat:*]
autologin-user=$user_name
autologin-user-timeout=0
autologin-session=$session_name
EOFLIGHTDM
  $SUDO chown root:root "$conf" 2>/dev/null || true
  $SUDO chmod 0644 "$conf" 2>/dev/null || true
  $SUDO groupadd -f autologin 2>/dev/null || true
  $SUDO gpasswd -a "$user_name" autologin >/dev/null 2>&1 || true
  if lightdm_dashboard_autologin_ready "$user_name" "$session_name"; then
    ok "LightDM autologin verified for $user_name using '$session_name'"
  else
    warn "LightDM autologin file was written but verification did not match; inspect $conf"
    return 1
  fi
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
    [ -n "$tmp" ] && printf '%s
' "$f"
  done
}

install_kiosk_no_logout_guard(){
  local logind_drop=/etc/systemd/logind.conf.d/10-dashboard-kiosk.conf app
  mkdir -p "$HOME/.config/autostart" || return 1
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
  ok "kiosk anti-lock/autologout guard installed"
  echo "    Takes full effect after reboot or systemd-logind restart; current X session was updated when possible."
}

# Debian display-manager packages ask a shared debconf question. Preseed it
# before installs, then enforce it afterward, so a fresh Debian/GNOME/Trixie
# box cannot accidentally stay on gdm3/lxdm while the dashboard config is
# written for LightDM. We only disable competing DMs for the next boot; we do
# not stop the active graphical session mid-install.
preseed_lightdm_default(){
  if command -v debconf-set-selections >/dev/null 2>&1; then
    printf 'shared/default-x-display-manager shared/default-x-display-manager select lightdm\n' | $SUDO debconf-set-selections 2>/dev/null || true
    printf 'lightdm shared/default-x-display-manager select lightdm\n' | $SUDO debconf-set-selections 2>/dev/null || true
  fi
}
wayland_session_is_active(){
  [ -n "${WAYLAND_DISPLAY:-}" ] && return 0
  command -v pgrep >/dev/null 2>&1 && { pgrep -x labwc >/dev/null 2>&1 || pgrep -x wayfire >/dev/null 2>&1 || pgrep -x sway >/dev/null 2>&1; }
}

ensure_lightdm_default(){
  local dm default_dm=""
  if [ ! -x /usr/sbin/lightdm ] && ! command -v lightdm >/dev/null 2>&1; then
    warn "LightDM is not installed yet; cannot set it as the default display manager"
    return 0
  fi
  if wayland_session_is_active; then
    warn "This device is currently running a Wayland desktop. Dash-Go's kiosk uses X11/LightDM and will take over on the next reboot."
  fi
  preseed_lightdm_default
  if [ -d /etc/X11 ]; then
    echo '/usr/sbin/lightdm' | $SUDO tee /etc/X11/default-display-manager >/dev/null || return 1
  fi
  $SUDO systemctl set-default graphical.target >/dev/null 2>&1 || true
  $SUDO systemctl enable lightdm >/dev/null 2>&1 || true
  for dm in gdm3 lxdm sddm xdm wdm greetd; do
    if systemctl list-unit-files "${dm}.service" 2>/dev/null | grep -q "^${dm}\.service"; then
      $SUDO systemctl disable "${dm}.service" >/dev/null 2>&1 || true
    fi
  done
  default_dm="$(cat /etc/X11/default-display-manager 2>/dev/null || true)"
  if [ "$default_dm" != "/usr/sbin/lightdm" ]; then
    warn "LightDM is not the recorded default display manager yet; run ~/install.sh --repair --system before rebooting."
    return 1
  fi
  if ! systemctl is-enabled lightdm >/dev/null 2>&1; then
    warn "LightDM could not be enabled for the next boot; run ~/install.sh --repair --system before rebooting."
    return 1
  fi
  ok "LightDM selected as Dash-Go's X11 display manager for next boot"
}
bootstrap_detect_platform

# A novice should learn about a broken clock, missing storage, unsupported
# device, or unreachable release host before answering setup questions. This
# preflight intentionally checks the canonical GitHub Release route rather
# than an unrelated provider. Direct --doctor/--remove/update commands retain
# their own narrow behavior and do not call this interactive-only helper.
preflight_item(){
  local state="$1" label="$2" detail="${3:-}"
  printf '  %s %s%s\n' "$state" "$label" "${detail:+ — $detail}"
}

installer_supported_platform(){
  case "${OS_ID:-unknown}" in
    raspbian) [ "${IS_PI:-0}" = "1" ] || return 1 ;;
    debian) [ "${IS_PI:-0}" = "1" ] || [ "${IS_X86:-0}" = "1" ] || return 1 ;;
    *) return 1 ;;
  esac
  [ "${OS_SUPPORT_LEVEL:-unsupported}" != "unsupported" ]
}

installer_required_kiosk_packages(){
  # Keep this short and durable for the preflight. install_runtime_packages
  # adds a profile-specific session stack later, after APT candidates are known.
  printf '%s\n' surf openbox wmctrl x11-xserver-utils curl
}

installer_required_package_candidates(){
  local pkg candidate missing=""
  command -v apt-cache >/dev/null 2>&1 || return 0
  for pkg in $(installer_required_kiosk_packages); do
    candidate="$(apt-cache policy "$pkg" 2>/dev/null | awk '/Candidate:/ {print $2; exit}')"
    case "$candidate" in ''|none|"(none)") missing="$missing $pkg";; esac
  done
  [ -z "$missing" ] || { printf '%s\n' "${missing# }"; return 1; }
}

installer_clock_is_sane(){
  local year
  year="$(date +%Y 2>/dev/null || true)"
  case "$year" in ''|*[!0-9]*) return 1;; esac
  [ "$year" -ge 2024 ] && [ "$year" -le 2100 ]
}

installer_timezone(){
  local zone=""
  if command -v timedatectl >/dev/null 2>&1; then
    zone="$(timedatectl show -p Timezone --value 2>/dev/null || true)"
  fi
  [ -n "$zone" ] || zone="$(tr -d '\r\n' < /etc/timezone 2>/dev/null || true)"
  printf '%s\n' "$zone"
}
installer_timezone_needs_attention(){
  case "${1:-}" in ''|UTC|Etc/UTC) return 0;; esac
  return 1
}
installer_timezone_is_valid(){
  local zone="$1"
  case "$zone" in ''|/*|*'..'*|*'//'*) return 1;; esac
  [ -f "/usr/share/zoneinfo/$zone" ]
}
prompt_timezone_if_needed(){
  local current requested
  current="$(installer_timezone)"
  installer_timezone_needs_attention "$current" || return 0
  echo
  warn "Time zone is ${current:-unset}. Calendar times and weather use this device setting."
  if ! command -v timedatectl >/dev/null 2>&1; then
    warn "This system cannot set a time zone automatically. Set one later with: sudo timedatectl set-timezone Region/City"
    return 0
  fi
  read -r -p "  Time zone (for example America/Chicago; Enter leaves it unchanged): " requested || return 0
  [ -n "$requested" ] || return 0
  if ! installer_timezone_is_valid "$requested"; then
    warn "'$requested' is not a valid IANA time-zone name. Leave it unchanged, then try again with a name such as America/Chicago."
    return 0
  fi
  start_sudo_keepalive || return 1
  if $SUDO timedatectl set-timezone "$requested"; then
    ok "Time zone set to $requested"
  else
    warn "Could not set the time zone. Try later with: sudo timedatectl set-timezone $requested"
  fi
}

installer_writable_probe(){
  local target="$1" probe
  [ -d "$target" ] || target="$(dirname "$target")"
  probe="$target/.dash-go-write-test.$$"
  if ( umask 077; : > "$probe" && { sync -f "$probe" 2>/dev/null || sync >/dev/null 2>&1; } ); then
    rm -f "$probe" 2>/dev/null || true
    return 0
  fi
  rm -f "$probe" 2>/dev/null || true
  return 1
}

run_startup_preflight(){
  local failed=0 free_mb year mem_mb zone dashboard_probe
  say "Checking your device"
  echo "This quick check runs before setup questions so problems are easy to fix."

  if installer_supported_platform; then
    preflight_item '✓' "$PLATFORM_LABEL"
    case "${OS_SUPPORT_LEVEL:-}" in
      supported) preflight_item '✓' "$OS_SUPPORT_LABEL" ;;
      recommended) preflight_item '✓' "$OS_SUPPORT_LABEL" ;;
      conservative) preflight_item '!' "$OS_SUPPORT_LABEL" "$OS_SUPPORT_HINT" ;;
    esac
  else
    if [ "${IS_DEBIAN:-0}" = "1" ] && [ "${OS_SUPPORT_LEVEL:-}" = "unsupported" ]; then
      preflight_item '✗' "$OS_SUPPORT_LABEL" "$OS_SUPPORT_HINT"
    else
      preflight_item '✗' "Unsupported platform: $PLATFORM_LABEL" "Dash-Go supports Raspberry Pi OS and Debian x86/Pi installations"
    fi
    failed=1
  fi

  if command -v sudo >/dev/null 2>&1; then
    preflight_item '✓' 'Normal user with sudo available'
  else
    preflight_item '✗' 'sudo is missing' 'install it as root, add this user to sudo, then run ~/install.sh again'
    failed=1
  fi

  free_mb="$(df -Pm "$HOME" 2>/dev/null | awk 'NR==2{print $4}')"
  case "$free_mb" in
    ''|*[!0-9]*)
      preflight_item '✗' 'Free disk space could not be checked' 'make sure your home folder is available, then run ~/install.sh again'
      failed=1
      ;;
    *)
      if [ "$free_mb" -ge 500 ]; then
        preflight_item '✓' "Free disk space: ${free_mb} MB"
      else
        preflight_item '✗' "Free disk space: ${free_mb} MB" 'need at least 500 MB; free space, then run ~/install.sh again'
        failed=1
      fi
      ;;
  esac

  mem_mb="$(bootstrap_mem_total_mb)"
  case "$mem_mb" in ''|*[!0-9]*) mem_mb=0;; esac
  if [ "$mem_mb" -ge 900 ]; then
    preflight_item '✓' "RAM: ${mem_mb} MB"
  elif [ "$mem_mb" -ge 450 ]; then
    preflight_item '✓' "RAM: ${mem_mb} MB (Lite profile is recommended on lower-memory devices)"
  elif [ "$mem_mb" -ge 400 ]; then
    preflight_item '!' "RAM: ${mem_mb} MB" 'below the recommended level; the dashboard may load slowly, and the Lite profile is recommended'
  else
    preflight_item '!' "RAM: ${mem_mb} MB" 'very low memory; a Raspberry Pi Zero 2 W or newer is recommended'
  fi

  if installer_writable_probe "$HOME"; then
    preflight_item '✓' 'Home folder is writable'
  else
    preflight_item '✗' 'Home folder is not writable' 'the SD card or filesystem may be read-only or failing; repair, re-flash, or replace the card, then run ~/install.sh again'
    failed=1
  fi
  dashboard_probe="$DASH"
  if [ -e "$DASH" ] && ! installer_writable_probe "$dashboard_probe"; then
    preflight_item '✗' 'Dashboard folder is not writable' 'the SD card or filesystem may be read-only or failing; repair storage, then run ~/install.sh again'
    failed=1
  elif [ -e "$DASH" ]; then
    preflight_item '✓' 'Dashboard folder is writable'
  fi

  zone="$(installer_timezone)"
  if installer_timezone_needs_attention "$zone"; then
    preflight_item '!' "Time zone: ${zone:-unset}" 'calendar times and weather use this setting; setup can help you set it'
  else
    preflight_item '✓' "Time zone: $zone"
  fi

  year="$(date +%Y 2>/dev/null || true)"
  if installer_clock_is_sane; then
    preflight_item '✓' "Clock: $(date '+%Y-%m-%d %H:%M' 2>/dev/null || true)"
  else
    preflight_item '✗' "Clock looks wrong (${year:-unknown})" 'connect to the network, run sudo timedatectl set-ntp true, then run ~/install.sh again'
    failed=1
  fi

  if command -v getent >/dev/null 2>&1 && getent ahosts github.com >/dev/null 2>&1; then
    preflight_item '✓' 'DNS can find GitHub'
  else
    preflight_item '✗' 'DNS cannot find GitHub' 'connect this device to the internet, then run ~/install.sh again'
    failed=1
  fi

  if ! command -v curl >/dev/null 2>&1; then
    preflight_item '✗' 'curl is missing' 'install curl, then run ~/install.sh again'
    failed=1
  elif curl -fsSIL --connect-timeout 5 --max-time 12 --proto '=https' --tlsv1.2 \
      'https://github.com/DashDashGoApp/Dash-Go/releases' >/dev/null 2>&1; then
    preflight_item '✓' 'Dash-Go release host is reachable'
  else
    preflight_item '✗' 'Dash-Go release host is not reachable' 'check Wi-Fi/internet and the clock, then run ~/install.sh again'
    failed=1
  fi

  if missing_packages="$(installer_required_package_candidates 2>/dev/null)"; then
    preflight_item '✓' 'Required kiosk packages are available from configured APT sources'
  else
    preflight_item '✗' "Required kiosk package unavailable: ${missing_packages:-unknown}" 'check your Debian software sources or upgrade the device, then run ~/install.sh again'
    failed=1
  fi

  if [ "$failed" = "1" ]; then
    echo
    warn 'Fix the items marked ✗ and run ~/install.sh again. Nothing was changed.'
    return 1
  fi
  return 0
}

# Downloads used by installer recovery are public GitHub Release assets or
# pinned public font sources. Dash-Go deliberately does not accept an arbitrary
# update host or send any household credential during this path.
UA="Dash-Go installer"
fetch(){
  local url="$1" out="$2" attempt=1 attempts="${DASHGO_FETCH_ATTEMPTS:-3}" delay="${DASHGO_FETCH_RETRY_DELAY:-2}"
  case "$url" in
    https://*) ;;
    *) return 1;;
  esac
  case "$attempts" in ''|*[!0-9]*) attempts=3;; esac
  [ "$attempts" -ge 1 ] || attempts=1
  case "$delay" in ''|*[!0-9]*) delay=2;; esac
  while [ "$attempt" -le "$attempts" ]; do
    if curl --fail --silent --show-error --location --proto '=https' --tlsv1.2 \
      --retry 1 --retry-delay "$delay" --retry-connrefused --retry-all-errors \
      --connect-timeout 15 --max-time 180 -A "$UA" "$url" -o "$out"; then
      return 0
    fi
    rm -f "$out" 2>/dev/null || true
    if [ "$attempt" -lt "$attempts" ]; then
      warn "Download interrupted; retrying (${attempt}/${attempts})."
      sleep "$delay"
    fi
    attempt=$((attempt + 1))
  done
  warn "Download failed after retries. Move closer to Wi-Fi or use Ethernet, then run ~/install.sh again."
  return 1
}

# Base dashboard faces are runtime dependencies rather than release payload
# files. Keep them out of the source handoff, download only when missing, and
# stage every missing file before replacing any live face. Optional user-picked
# fonts live separately in $RUNTIME_FONT_DIR and are never touched here.
font_file_valid(){
  local path="$1" size magic
  [ -s "$path" ] || return 1
  size="$(wc -c < "$path" 2>/dev/null || echo 0)"
  [ "$size" -ge 4096 ] && [ "$size" -le 3000000 ] || return 1
  ! head -c 160 "$path" | grep -qiE '<!doctype|<html' || return 1
  magic="$(dd if="$path" bs=1 count=4 2>/dev/null | od -An -tx1 | tr -d ' \n')"
  case "$magic" in 00010000|4f54544f|74727565|74746366) ;; *) return 1;; esac
  command -v fc-scan >/dev/null 2>&1 && ! fc-scan "$path" >/dev/null 2>&1 && return 1
  return 0
}
font_download(){
  local url="$1" out="$2"
  curl --fail --location --retry 2 --retry-all-errors --connect-timeout 15 --max-time 120 \
    --proto '=https' --tlsv1.2 -A 'Dash-Go font recovery' -o "$out" "$url"
}
ensure_dashboard_fonts(){
  local stage name url target downloaded=0
  mkdir -p "$FONT_DIR" "$RUNTIME_FONT_DIR" || return 1
  stage="$(mktemp -d "$DASH/.font-stage.XXXXXX")" || return 1
  trap 'rm -rf "$stage"' RETURN
  while IFS='|' read -r name url; do
    target="$FONT_DIR/$name"
    if font_file_valid "$target"; then continue; fi
    downloaded=1
    font_download "$url" "$stage/$name" && font_file_valid "$stage/$name" || { rm -rf "$stage"; trap - RETURN; return 1; }
  done <<'EOFONTS'
LibreFranklin-400.ttf|https://raw.githubusercontent.com/impallari/Libre-Franklin/master/fonts/TTF/LibreFranklin-Regular.ttf
LibreFranklin-600.ttf|https://raw.githubusercontent.com/impallari/Libre-Franklin/master/fonts/TTF/LibreFranklin-SemiBold.ttf
LibreFranklin-700.ttf|https://raw.githubusercontent.com/impallari/Libre-Franklin/master/fonts/TTF/LibreFranklin-Bold.ttf
LibreFranklin-800.ttf|https://raw.githubusercontent.com/impallari/Libre-Franklin/master/fonts/TTF/LibreFranklin-ExtraBold.ttf
DMMono-500.ttf|https://raw.githubusercontent.com/google/fonts/main/ofl/dmmono/DMMono-Medium.ttf
EOFONTS
  if [ "$downloaded" = 1 ]; then
    for name in LibreFranklin-400.ttf LibreFranklin-600.ttf LibreFranklin-700.ttf LibreFranklin-800.ttf DMMono-500.ttf; do
      [ -f "$stage/$name" ] && install -m 0644 "$stage/$name" "$FONT_DIR/$name" || true
    done
  fi
  rm -rf "$stage"; trap - RETURN
  for name in LibreFranklin-400.ttf LibreFranklin-600.ttf LibreFranklin-700.ttf LibreFranklin-800.ttf DMMono-500.ttf; do font_file_valid "$FONT_DIR/$name" || return 1; done
  return 0
}

# Remove only duplicate kiosk loops created by stale autostarts. Leave one
# current session loop alive so it can relaunch Surf after updates.
prune_duplicate_kiosk_loops(){
  local pids count keep pid
  pids="$(pgrep -f "$DASH/kiosk.sh" 2>/dev/null | awk -v self="$$" '$1 != self {print}' | sort -n || true)"
  count="$(printf '%s\n' "$pids" | awk 'NF{c++} END{print c+0}')"
  [ "$count" -le 1 ] && return 0
  keep="$(printf '%s\n' "$pids" | awk 'NF{print; exit}')"
  printf '%s\n' "$pids" | while IFS= read -r pid; do
    [ -n "$pid" ] && [ "$pid" != "$keep" ] && kill "$pid" 2>/dev/null || true
  done
  sleep 1
  printf '%s\n' "$pids" | while IFS= read -r pid; do
    [ -n "$pid" ] && [ "$pid" != "$keep" ] && kill -9 "$pid" 2>/dev/null || true
  done
  ok "pruned duplicate kiosk launch loops; kept pid $keep"
}

cleanup_kiosk_autostarts(){
  local f tmp changed=0
  rm -f "$HOME/.config/autostart/dashboard-kiosk.desktop" 2>/dev/null && changed=1 || true
  for f in "$HOME"/.config/lxsession/*/autostart "$HOME/.config/openbox/autostart"; do
    [ -f "$f" ] || continue
    tmp="$(mktemp)" || continue
    grep -vE 'dashboard/kiosk\.sh|/kiosk\.sh' "$f" > "$tmp" || true
    if ! cmp -s "$f" "$tmp"; then
      cp -p "$f" "$f.dashboard-bak-$(date +%Y%m%d-%H%M%S)" 2>/dev/null || true
      mv "$tmp" "$f" 2>/dev/null || rm -f "$tmp"
      changed=1
    else
      rm -f "$tmp"
    fi
  done
  [ "$changed" = "1" ] && ok "removed stale duplicate kiosk autostart entries" || ok "no stale duplicate kiosk autostart entries found"
}

remove_obsolete_shutdown_cleanup_service(){
  local svc=/etc/systemd/system/dashboard-shutdown-cleanup.service
  rm -f "$BIN_DIR/dashboard-shutdown-cleanup.sh" "$DASH/bin/dashboard-shutdown-cleanup.sh" 2>/dev/null || true
  if command -v systemctl >/dev/null 2>&1 && [ -f "$svc" ] && sudo -n true >/dev/null 2>&1; then
    sudo systemctl disable dashboard-shutdown-cleanup.service >/dev/null 2>&1 || true
    sudo rm -f "$svc" >/dev/null 2>&1 || true
    sudo systemctl daemon-reload >/dev/null 2>&1 || true
  fi
}

# Recycle only the Surf process owned by the Dash-Go kiosk after an update or
# setup change.  Do not use a global `pkill -x surf`: another local Surf
# instance is not Dash-Go's browser to terminate.
dashboard_kiosk_pids(){
  pgrep -f "$DASH/kiosk.sh" 2>/dev/null | sort -n || true
}

dashboard_surf_pids(){
  local kiosk child comm
  for kiosk in $(dashboard_kiosk_pids); do
    for child in $(pgrep -P "$kiosk" 2>/dev/null || true); do
      comm="$(ps -p "$child" -o comm= 2>/dev/null | tr -d '[:space:]')"
      [ "$comm" = "surf" ] && printf '%s\n' "$child"
    done
  done | sort -nu
}

restart_kiosk(){
  local old_pids current_pids replacement_pid="" pid i=0 wait_seconds
  old_pids="$(dashboard_surf_pids | tr '\n' ' ' || true)"
  wait_seconds="${DASH_SURF_RELAUNCH_TIMEOUT:-30}"
  case "$wait_seconds" in *[!0-9]*|'') wait_seconds=30;; esac
  mkdir -p "$LOG_DIR" 2>/dev/null || true
  if [ -z "$old_pids" ]; then
    if [ -e "$CACHE_DIR/kiosk-paused" ]; then
      warn "kiosk browser cannot be verified while the Dash-Go kiosk pause marker is present"
      return 1
    fi
    if ! dashboard_kiosk_pids | grep -q .; then
      warn "no Dash-Go kiosk loop is running; browser recovery requires a graphical-session restart"
      return 1
    fi
    say "Confirming kiosk browser recovery"
    printf '%s installer found no existing Dash-Go Surf; waiting for the active kiosk loop to launch one\n' "$(date '+%Y-%m-%d %H:%M:%S')" >> "$LOG_DIR/kiosk.log" 2>/dev/null || true
    while [ "$i" -lt "$wait_seconds" ]; do
      replacement_pid="$(dashboard_surf_pids | head -n 1 || true)"
      if [ -n "$replacement_pid" ]; then
        printf '%s installer verified Dash-Go Surf pid=%s after recovery wait\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$replacement_pid" >> "$LOG_DIR/kiosk.log" 2>/dev/null || true
        ok "kiosk browser is running as pid $replacement_pid"
        return 0
      fi
      i=$((i+1)); sleep 1
    done
    warn "kiosk loop is running, but no Dash-Go Surf PID appeared within ${wait_seconds}s"
    printf '%s installer could not verify Dash-Go Surf after recovery wait\n' "$(date '+%Y-%m-%d %H:%M:%S')" >> "$LOG_DIR/kiosk.log" 2>/dev/null || true
    return 1
  fi
  say "Refreshing kiosk browser so changes take effect"
  printf '%s installer requested targeted Dash-Go Surf recycle; keeping kiosk/session alive\n' "$(date '+%Y-%m-%d %H:%M:%S')" >> "$LOG_DIR/kiosk.log" 2>/dev/null || true
  for pid in $old_pids; do kill -TERM "$pid" 2>/dev/null || true; done
  while [ "$i" -lt "$wait_seconds" ]; do
    current_pids="$(dashboard_surf_pids | tr '\n' ' ' || true)"
    for pid in $current_pids; do
      case " $old_pids " in
        *" $pid "*) ;;
        *) replacement_pid="$pid"; break ;;
      esac
    done
    if [ -n "$replacement_pid" ]; then
      printf '%s installer verified replacement Dash-Go Surf pid=%s after recycle\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$replacement_pid" >> "$LOG_DIR/kiosk.log" 2>/dev/null || true
      ok "browser recycled; replacement Dash-Go Surf is running as pid $replacement_pid"
      return 0
    fi
    i=$((i+1)); sleep 1
  done
  if dashboard_kiosk_pids | grep -q .; then
    warn "kiosk loop is running, but no replacement Dash-Go Surf PID appeared within ${wait_seconds}s"
  else
    warn "Dash-Go Surf was stopped but no kiosk loop was detected; it will return at the next login/reboot"
  fi
  printf '%s installer could not verify a replacement Dash-Go Surf PID after recycle\n' "$(date '+%Y-%m-%d %H:%M:%S')" >> "$LOG_DIR/kiosk.log" 2>/dev/null || true
  return 1
}

pause_kiosk_for_runtime_transition(){
  mkdir -p "$CACHE_DIR" 2>/dev/null || return 1
  : > "$CACHE_DIR/kiosk-paused" || return 1
  DASH_UPDATE_PAUSE_OWNED=1; export DASH_UPDATE_PAUSE_OWNED
  printf '%s installer paused kiosk relaunch pending local runtime readiness\n' "$(date '+%Y-%m-%d %H:%M:%S')" >> "$LOG_DIR/kiosk.log" 2>/dev/null || true
}

resume_kiosk_after_runtime_transition(){
  rm -f "$CACHE_DIR/kiosk-paused" 2>/dev/null || true
  DASH_UPDATE_PAUSE_OWNED=0; export DASH_UPDATE_PAUSE_OWNED
  printf '%s installer released kiosk pause after local runtime readiness\n' "$(date '+%Y-%m-%d %H:%M:%S')" >> "$LOG_DIR/kiosk.log" 2>/dev/null || true
}

update_exit_cleanup(){
  local rc="${1:-0}"
  if [ "${DASH_UPDATE_PAUSE_OWNED:-0}" = 1 ]; then
    warn "update ended while the kiosk was paused; releasing the Dash-Go kiosk pause marker"
    resume_kiosk_after_runtime_transition
  fi
  update_plan_cleanup || true
  return "$rc"
}

dashboard_service_main_pid(){
  local pid cmd
  command -v systemctl >/dev/null 2>&1 || return 1
  pid="$(systemctl show --property=MainPID --value dashboard-server.service 2>/dev/null || true)"
  case "$pid" in ''|0|*[!0-9]*) return 1;; esac
  [ -r "/proc/$pid/cmdline" ] || return 1
  cmd="$(tr '\0' ' ' < "/proc/$pid/cmdline" 2>/dev/null || true)"
  case "$cmd" in
    *"$BIN_DIR/dashboard-control-server-linux-"*) printf '%s\n' "$pid" ;;
    *) return 1 ;;
  esac
}

# The updater must survive the server restart in order to confirm the local
# API and only then recycle Surf. Control-launched updates are moved into the
# user manager below; SSH updates are already outside dashboard-server.service.
restart_dashboard_server_for_update(){
  local expected="$1" pid
  pid="$(dashboard_service_main_pid 2>/dev/null || true)"
  if [ -z "$pid" ]; then
    warn "could not identify the running Dash-Go server process for a safe restart"
    return 1
  fi
  kill -TERM "$pid" 2>/dev/null || return 1
  dashboard_server_confirm_live "$expected"
}

# Dashboard Control launches updates through dash-go-update.service. Do not
# move updates through a per-user systemd manager: it is optional on kiosk
# devices and cannot be the safety boundary for a server restart.

control_pin_timeout_label(){
  case "${1:-1800}" in
    every_open|0) echo "Every dashboard control open";;
    60) echo "1 minute";;
    300) echo "5 minutes";;
    900) echo "15 minutes";;
    1800|"") echo "30 minutes";;
    3600) echo "1 hour";;
    until_reboot) echo "Until reboot";;
    *) echo "30 minutes";;
  esac
}

current_control_pin_timeout(){
  if [ -f "$CONTROL_ENV" ]; then
    awk -F= '/^DASH_CONTROL_PIN_TIMEOUT=/{print $2; found=1; exit} END{if(!found) print "1800"}' "$CONTROL_ENV" 2>/dev/null
  else
    echo "1800"
  fi
}

control_pin_timeout_choice(){
  case "${1:-1800}" in
    every_open|0) printf '1';;
    60) printf '2';;
    300) printf '3';;
    900) printf '4';;
    1800|"") printf '5';;
    3600) printf '6';;
    until_reboot) printf '7';;
    *) printf '5';;
  esac
}

prompt_control_pin_timeout(){
  local cur choice default_choice val
  cur="$(current_control_pin_timeout)"
  default_choice="$(control_pin_timeout_choice "$cur")"
  echo >&2
  echo "How long should the control panel stay unlocked after a correct PIN?" >&2
  echo "  1) Every dashboard control open" >&2
  echo "  2) 1 minute" >&2
  echo "  3) 5 minutes" >&2
  echo "  4) 15 minutes" >&2
  echo "  5) 30 minutes" >&2
  echo "  6) 1 hour" >&2
  echo "  7) Until reboot / dashboard-server restart" >&2
  echo "Current/default: $(control_pin_timeout_label "$cur")" >&2
  read -rp "  Choose [1-7, Enter=${default_choice}]: " choice
  choice="${choice:-$default_choice}"
  case "$choice" in
    1) val="every_open";;
    2) val="60";;
    3) val="300";;
    4) val="900";;
    5) val="1800";;
    6) val="3600";;
    7) val="until_reboot";;
    *) printf '\033[1;33m!! invalid choice; keeping %s\033[0m\n' "$(control_pin_timeout_label "$cur")" >&2; val="$cur";;
  esac
  printf '%s' "$val"
}

set_control_pin_timeout_only(){
  local val tmp
  val="$1"
  [ -f "$CONTROL_ENV" ] || { warn "$CONTROL_ENV does not exist yet. Enable the PIN first."; return 1; }
  if ! grep -q '^DASH_CONTROL_PIN_ENABLED=1' "$CONTROL_ENV" 2>/dev/null; then
    warn "PIN lock is not enabled. Enable or reset the PIN first."
    return 1
  fi
  tmp="$(mktemp)"
  awk -v new="$val" '
    BEGIN{done=0}
    /^DASH_CONTROL_PIN_TIMEOUT=/{print "DASH_CONTROL_PIN_TIMEOUT=" new; done=1; next}
    {print}
    END{if(!done) print "DASH_CONTROL_PIN_TIMEOUT=" new}
  ' "$CONTROL_ENV" > "$tmp"
  mv "$tmp" "$CONTROL_ENV"
  chmod 600 "$CONTROL_ENV"
  ok "PIN unlock duration set to $(control_pin_timeout_label "$val")"
}


# --- Sudo hardening ------------------------------------------------------
# Raspberry Pi OS images sometimes ship a broad passwordless sudo drop-in such
# as /etc/sudoers.d/010_pi-nopasswd. That is convenient for first boot, but too
# broad for a dedicated kiosk. The dashboard only needs scoped NOPASSWD rules
# for reboot/poweroff and optional apt package maintenance.
harden_dashboard_sudoers(){
  local files f tmp stamp bak changed=0
  files="$(find_broad_nopasswd_files | sort -u)"
  if [ -z "$files" ]; then
    ok "no broad passwordless sudo rule found for $USER_NAME"
    return 0
  fi

  echo
  warn "Broad passwordless sudo detected for $USER_NAME:"
  printf '%s\n' "$files" | sed 's/^/  - /'
  echo
  echo "This is often created by Raspberry Pi OS as 010_pi-nopasswd. It allows"
  echo "$USER_NAME to run ANY command as root without a password. The dashboard"
  echo "only needs scoped rules for reboot/poweroff and optional apt-get update/upgrade."
  echo
  echo "If you continue, the installer will:"
  echo "  1) write scoped dashboard sudoers rules,"
  echo "  2) back up the broad sudoers file(s),"
  echo "  3) comment out only the broad NOPASSWD: ALL line(s),"
  echo "  4) validate sudoers before keeping the change."
  echo
  echo "Normal SSH/admin sudo may ask for the user's password afterward."
  read -rp "Replace broad passwordless sudo with scoped dashboard rules? [y/N] " ans
  case "$ans" in y|Y) ;; *) warn "left broad passwordless sudo unchanged"; return 0;; esac

  write_dashboard_scoped_sudoers || return 1
  stamp="$(date +%Y%m%d-%H%M%S)"
  for f in $files; do
    sudoers_file_is_parsed "$f" || continue
    tmp="$(mktemp)" || return 1
    $SUDO awk -v user="$USER_NAME" -v stamp="$stamp" '
      $0 !~ /^[[:space:]]*#/ &&
      $0 ~ "^[[:space:]]*" user "[[:space:]]+ALL[[:space:]]*=" &&
      $0 ~ /NOPASSWD:[[:space:]]*ALL([[:space:]]|$)/ {
        print "# disabled by Dash-Go sudo hardening " stamp ": " $0
        next
      }
      {print}
    ' "$f" > "$tmp"
    bak="$f.dashboard-bak-$stamp"
    $SUDO cp -p "$f" "$bak" || { rm -f "$tmp"; warn "could not back up $f"; return 1; }
    if $SUDO cp "$tmp" "$f"; then
      $SUDO chmod 0440 "$f" 2>/dev/null || true
      changed=1
    else
      rm -f "$tmp"
      warn "could not update $f"
      return 1
    fi
    rm -f "$tmp"
  done

  if $SUDO visudo -cf /etc/sudoers >/dev/null 2>&1; then
    [ "$changed" = "1" ] && ok "broad passwordless sudo disabled; scoped dashboard rules kept"
    return 0
  fi

  warn "sudoers validation failed after hardening — restoring backups"
  for f in $files; do
    bak="$f.dashboard-bak-$stamp"
    [ -f "$bak" ] && $SUDO cp -p "$bak" "$f" 2>/dev/null || true
  done
  $SUDO visudo -cf /etc/sudoers >/dev/null 2>&1 || warn "sudoers still needs manual review with visudo"
  return 1
}

# Configure, reset, or disable the optional on-screen control-panel PIN.
# Kept as its own installer task so it can be changed later without re-running
# the web-service/autologin/system setup. The server reads this file outside the
# served dashboard directory; it contains only a salted PBKDF2 hash, never the
# plaintext PIN. Restarting the service after a change clears any old unlock
# session tokens immediately.
configure_control_pin(){
  say "Control-panel PIN lock"
  echo "The on-screen control panel can be locked behind a phone-style PIN."
  echo "The PIN hash is stored in $CONTROL_ENV (chmod 600), not in the served"
  echo "dashboard folder. Use this task any time to set, reset, disable it, or"
  echo "choose how long a successful unlock remains active."
  echo

  CURLOCK="off"
  CURTIME="$(current_control_pin_timeout)"
  if [ -f "$CONTROL_ENV" ] && grep -q '^DASH_CONTROL_PIN_ENABLED=1' "$CONTROL_ENV" 2>/dev/null; then
    CURLOCK="on"
  fi
  echo "Current control-panel PIN lock: $CURLOCK"
  if [ "$CURLOCK" = "on" ]; then
    echo "Current unlock duration: $(control_pin_timeout_label "$CURTIME")"
  fi
  echo "  1) Keep current setting"
  echo "  2) Enable or reset PIN"
  echo "  3) Change unlock duration only"
  echo "  4) Disable PIN lock"
  read -rp "  Choose [1/2/3/4, Enter=1]: " PINMODE
  PINMODE="${PINMODE:-1}"

  case "$PINMODE" in
    1)
      ok "left control-panel PIN lock unchanged"
      ;;
    2)
      while true; do
        read -rsp "  New PIN (4-8 digits): " PIN1; echo
        read -rsp "  Confirm PIN: " PIN2; echo
        if [ "$PIN1" != "$PIN2" ]; then warn "PINs did not match. Try both entries again."; continue; fi
        if ! printf '%s' "$PIN1" | grep -qE '^[0-9]{4,8}$'; then warn "Use 4-8 digits only."; continue; fi
        break
      done
      PIN_TIMEOUT="$(prompt_control_pin_timeout)"
      PIN_DATA="$(PIN_VALUE="$PIN1" installer_cli --pin-hash 2>/dev/null)" || {
        warn "Dash-Go could not secure the PIN on this device. Choose Update the app, then try Control PIN again."
        return 1
      }
      {
        echo "# saved by install.sh — optional dashboard control PIN lock"
        echo "DASH_CONTROL_PIN_TIMEOUT=$PIN_TIMEOUT"
        echo "$PIN_DATA"
      } > "$CONTROL_ENV"
      chmod 600 "$CONTROL_ENV"
      ok "control-panel PIN lock enabled/reset ($(control_pin_timeout_label "$PIN_TIMEOUT"))"
      ;;
    3)
      PIN_TIMEOUT="$(prompt_control_pin_timeout)"
      set_control_pin_timeout_only "$PIN_TIMEOUT" || true
      ;;
    4)
      {
        echo "# saved by install.sh — optional dashboard control PIN lock"
        echo "DASH_CONTROL_PIN_ENABLED=0"
        echo "DASH_CONTROL_PIN_TIMEOUT=$(current_control_pin_timeout)"
      } > "$CONTROL_ENV"
      chmod 600 "$CONTROL_ENV"
      ok "control-panel PIN lock disabled"
      ;;
    *)
      warn "invalid choice; left control-panel PIN lock unchanged"
      ;;
  esac

  if systemctl is-active dashboard-server.service >/dev/null 2>&1; then
    if $SUDO systemctl restart dashboard-server.service >/dev/null 2>&1; then
      ok "dashboard-server restarted so PIN/session changes take effect now"
    else
      warn "could not restart dashboard-server; PIN file is saved, but a reboot or service restart may be needed"
    fi
  fi
}
# ---------------------------------------------------------------------
# An interactive run gets one visible device check before its first question.
# Command modes already have narrower, operation-specific preflights.
if [ "$INSTALLER_ORIGINAL_ARGC" -eq 0 ]; then
  INTERACTIVE_INSTALLER=1
  trap 'installer_interrupt' INT TERM
  trap 'installer_exit_summary' EXIT
  run_startup_preflight || exit 1
  # The preflight write probe should be able to explain read-only storage
  # before this setup lock attempts its own state-file write.
  acquire_interactive_installer_lock || exit 1
fi

say "Dash-Go installer"
cat <<'WELCOME'
Dash-Go (pronounced "Dash Dash Go") is a local, touch-first household kiosk.

This sets up the Dash-Go kiosk: calendar, weather, maps,
and rotating messages.

  * Safe to re-run; choose only what you want to change.
  * First install can take 10-30 minutes on a Pi Zero 2 W.
  * Press Ctrl-C to stop a step.
  * The main menu includes Exit. You can also type exit, quit, or q.
WELCOME
echo
echo "This installs the dashboard into: $DASH"
echo "Updates resolve only the canonical Dash-Go GitHub Releases (track: $RELEASE_TRACK)."

# --- Persist only the release track -------------------------------------
# Older updater records can contain retired connection material.
# This one-time migration overwrites both private files with the only durable
# state the GitHub Release model accepts: stable or beta. Values are never read
# back, logged, or sent over the network.
write_update_profile_json(){
  local tmp
  tmp="$(mktemp "${UPDATE_PROFILE}.tmp.XXXXXX")" || return 1
  {
    printf '{\n'
    printf '  "schema": 2,\n'
    printf '  "track": "%s"\n' "$RELEASE_TRACK"
    printf '}\n'
  } > "$tmp" || { rm -f "$tmp"; return 1; }
  chmod 600 "$tmp" || { rm -f "$tmp"; return 1; }
  mv -f "$tmp" "$UPDATE_PROFILE" || { rm -f "$tmp"; return 1; }
  chmod 600 "$UPDATE_PROFILE" || return 1
}

migrate_update_track_state(){
  [ "$REMOVE_MODE" != "1" ] || return 0
  [ "${DOCTOR_MODE:-0}" != "1" ] || return 0
  local tmp
  tmp="$(mktemp "${UPDATE_ENV}.tmp.XXXXXX")" || return 1
  {
    echo "# Dash-Go update track; canonical GitHub Releases are compiled into Dash-Go."
    printf 'DASH_TRACK=%s\n' "$RELEASE_TRACK"
  } > "$tmp" || { rm -f "$tmp"; return 1; }
  chmod 600 "$tmp" || { rm -f "$tmp"; return 1; }
  mv -f "$tmp" "$UPDATE_ENV" || { rm -f "$tmp"; return 1; }
  chmod 600 "$UPDATE_ENV" || return 1
  write_update_profile_json
}

if ! migrate_update_track_state; then
  warn "could not write the private Dash-Go update-track state; fix home-directory storage permissions and rerun install.sh"
  exit 1
fi

# Validate a downloaded file before it is allowed to replace the live copy.
# This catches captive portals, auth failures, and HTML error pages served with
# HTTP 200 — all of which otherwise look like a successful non-empty download.

# --- Apps: local Lists default + optional Microsoft To Do setup ----------------
valid_microsoft_client_id(){
  printf '%s' "${1:-}" | grep -Eq '^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}$'
}
write_todo_app_settings(){
  local sync_mode="$1" client_id="${2:-}"
  installer_cli --installer-todo-settings --file "$SETTINGS_FILE" --mode "$sync_mode" --client-id "$client_id"
}

# beta.37 retires app-visibility switches. The app shells are permanently
# present; this one-time update cleanup removes legacy switches and repairs only
# empty To Do/Grocery slot mappings. It preserves valid mappings, task data,
# integrations, and every unrelated preference.
normalize_app_visibility_preferences(){
  local settings_path="$SETTINGS_FILE" local_path="$CONFIG_DIR/config.local.js"
  installer_cli --installer-normalize-app-visibility --settings "$settings_path" --config-local "$local_path" ||     warn "Dash-Go could not normalize older app-visibility settings. Choose Update the app and try again."
}

todo_azure_cli_supported_architecture(){
  # Microsoft documents supported Azure CLI apt packages for amd64 and arm64.
  # Do not offer a package-repository install on a 32-bit Pi/armhf image that
  # cannot receive the documented package; the existing client-ID path remains
  # the safe fallback in that case.
  local arch
  arch="$(dpkg --print-architecture 2>/dev/null || true)"
  case "$arch" in
    amd64|arm64) printf '%s\n' "$arch"; return 0 ;;
  esac
  return 1
}
todo_azure_cli_repository_suite(){
  # The installer already loaded /etc/os-release into OS_CODENAME. Fall back to
  # lsb_release only when that value is unavailable. Keep the set explicit so
  # a future or unusual distribution does not receive an unreviewed repo entry.
  # Azure CLI currently documents Debian 11/Bullseye and Debian 12/Bookworm
  # packages, not a Debian 13/Trixie suite. Debian Trixie is intentionally
  # mapped to the compatible Bookworm package suite here; this is not a claim
  # that Microsoft publishes or supports a native Trixie repository.
  local suite="${OS_CODENAME:-}"
  if [ -z "$suite" ] && command -v lsb_release >/dev/null 2>&1; then
    suite="$(lsb_release -cs 2>/dev/null || true)"
  fi
  case "$suite" in
    trixie) printf '%s\n' bookworm; return 0 ;;
    bullseye|bookworm|jammy|noble) printf '%s\n' "$suite"; return 0 ;;
  esac
  return 1
}
install_azure_cli_for_todo_registration(){
  # Explicit, one-time installer action for the advanced private-registration
  # path. It is never run by normal install/update/repair flows. The package
  # remains installed only after a user confirms; Dash-Go runtime itself never
  # invokes az or receives app-management permissions.
  (
    umask 077
    if command -v az >/dev/null 2>&1; then return 0; fi

    local todo_arch todo_suite todo_choice todo_sudo
    todo_arch="$(todo_azure_cli_supported_architecture 2>/dev/null || true)"
    if [ -z "$todo_arch" ]; then
      local reported_arch
      reported_arch="$(dpkg --print-architecture 2>/dev/null || uname -m 2>/dev/null || echo unknown)"
      warn "Azure CLI is not installed, and Dash-Go cannot safely install it on architecture: $reported_arch"
      warn "The documented Azure CLI apt packages are amd64/arm64; this commonly affects 32-bit Raspberry Pi images."
      warn "Use Microsoft To Do / Graph setup with a client ID created on another computer, or choose the registration guide."
      return 2
    fi
    if ! command -v apt-get >/dev/null 2>&1; then
      warn "Azure CLI is not installed and this device does not provide apt-get."
      warn "Use an existing private client ID or create the app on another computer."
      return 2
    fi
    todo_suite="$(todo_azure_cli_repository_suite 2>/dev/null || true)"
    if [ -z "$todo_suite" ]; then
      warn "Azure CLI automatic install is offered only for reviewed Debian/Ubuntu bases (bullseye, bookworm, trixie via bookworm fallback, jammy, noble)."
      warn "This device reports codename: ${OS_CODENAME:-unknown}. Use an existing private client ID or the guide."
      return 2
    fi

    echo "Azure CLI is not installed. It is needed only for this one-time private app registration."
    if [ "${OS_CODENAME:-}" = "trixie" ] && [ "$todo_suite" = "bookworm" ]; then
      echo "Debian 13/Trixie uses the Azure CLI Debian 12/Bookworm package suite because Microsoft does not currently publish a native Trixie suite."
    fi
    echo "Installing it adds Microsoft's signed Azure CLI apt source and installs the azure-cli package."
    echo "It will remain installed afterward for future private-registration maintenance; Dash-Go itself will not use it at runtime."
    read -rp "  Install Azure CLI now? [y/N]: " todo_choice
    case "${todo_choice:-}" in
      y|Y|yes|YES) ;;
      *) echo "Azure CLI installation cancelled; local Lists remain unchanged."; return 2 ;;
    esac

    todo_sudo="${SUDO:-sudo}"
    if ! "$todo_sudo" -v; then
      warn "Administrator authorization was not granted. Azure CLI was not installed."
      return 1
    fi

    local todo_tmp todo_key_asc todo_key_gpg todo_keyring todo_source todo_source_tmp todo_source_backup
    local todo_source_created=0 todo_source_updated=0 todo_key_created=0 todo_install_ok=0
    todo_keyring="${TODO_AZURE_CLI_KEYRING:-/etc/apt/keyrings/dash-go-azure-cli.gpg}"
    todo_source="${TODO_AZURE_CLI_SOURCE_FILE:-/etc/apt/sources.list.d/dash-go-azure-cli.sources}"
    todo_tmp="$(mktemp -d "${TMPDIR:-/tmp}/dash-go-azure-cli.XXXXXX")" || { warn "Could not create a temporary Azure CLI installer workspace."; return 1; }
    todo_key_asc="$todo_tmp/microsoft.asc"
    todo_key_gpg="$todo_tmp/microsoft.gpg"
    todo_source_tmp="$todo_tmp/azure-cli.sources"
    todo_source_backup="$todo_tmp/azure-cli.sources.previous"
    todo_azure_cli_cleanup(){
      if [ "$todo_install_ok" != "1" ]; then
        if [ "$todo_source_created" = "1" ]; then
          "$todo_sudo" rm -f "$todo_source" >/dev/null 2>&1 || true
        elif [ "$todo_source_updated" = "1" ] && [ -f "$todo_source_backup" ]; then
          "$todo_sudo" install -m 0644 "$todo_source_backup" "$todo_source" >/dev/null 2>&1 || true
        fi
        [ "$todo_key_created" = "1" ] && "$todo_sudo" rm -f "$todo_keyring" >/dev/null 2>&1 || true
      fi
      rm -rf "$todo_tmp"
    }
    trap todo_azure_cli_cleanup EXIT HUP INT TERM

    cat > "$todo_source_tmp" <<TODO_AZURE_CLI_SOURCE
Types: deb
URIs: https://packages.microsoft.com/repos/azure-cli/
Suites: $todo_suite
Components: main
Architectures: $todo_arch
Signed-By: $todo_keyring
TODO_AZURE_CLI_SOURCE
    # Repair beta.48's stale Trixie source before the first apt refresh. An
    # unsupported Suite: trixie entry can itself make that refresh fail, which
    # would otherwise prevent this fallback from ever being applied.
    if [ -f "$todo_source" ]; then
      if ! grep -Fq 'https://packages.microsoft.com/repos/azure-cli/' "$todo_source"; then
        warn "Refusing to overwrite an existing non-Azure CLI source file: $todo_source"
        return 1
      fi
      if [ "${OS_CODENAME:-}" = "trixie" ] && grep -Fqx 'Suites: trixie' "$todo_source"; then
        "$todo_sudo" cp "$todo_source" "$todo_source_backup" || return 1
        "$todo_sudo" install -m 0644 "$todo_source_tmp" "$todo_source" || return 1
        todo_source_updated=1
      fi
    fi

    if ! apt_managed_as "$todo_sudo" update; then
      warn "Could not refresh apt before Azure CLI installation. Any temporary Azure CLI source/key changes were rolled back."
      return 1
    fi
    if ! apt_managed_as "$todo_sudo" install -y apt-transport-https ca-certificates curl gnupg lsb-release; then
      warn "Could not install Azure CLI prerequisites. No Dash-Go settings changed."
      return 1
    fi
    if ! curl --fail --silent --show-error --location https://packages.microsoft.com/keys/microsoft.asc -o "$todo_key_asc"; then
      warn "Could not download Microsoft's Azure CLI signing key. No Dash-Go settings changed."
      return 1
    fi
    if ! gpg --dearmor < "$todo_key_asc" > "$todo_key_gpg"; then
      warn "Could not prepare Microsoft's Azure CLI signing key. No Dash-Go settings changed."
      return 1
    fi
    if [ ! -f "$todo_keyring" ]; then
      "$todo_sudo" install -d -m 0755 "$(dirname "$todo_keyring")" || return 1
      "$todo_sudo" install -m 0644 "$todo_key_gpg" "$todo_keyring" || return 1
      todo_key_created=1
    fi

    if [ ! -f "$todo_source" ]; then
      "$todo_sudo" install -d -m 0755 "$(dirname "$todo_source")" || return 1
      "$todo_sudo" install -m 0644 "$todo_source_tmp" "$todo_source" || return 1
      todo_source_created=1
    fi

    if ! apt_managed_as "$todo_sudo" update; then
      warn "Could not refresh the Azure CLI package source. The temporary source/key changes were rolled back."
      return 1
    fi
    if ! apt_managed_as "$todo_sudo" install -y azure-cli; then
      warn "Azure CLI installation failed. The temporary source/key changes were rolled back."
      return 1
    fi
    hash -r 2>/dev/null || true
    if ! command -v az >/dev/null 2>&1; then
      warn "Azure CLI package installation finished without an az command. No Dash-Go settings changed."
      return 1
    fi
    if ! az version --only-show-errors >/dev/null 2>&1; then
      warn "Azure CLI installed but did not pass its local version check. No Dash-Go settings changed."
      return 1
    fi
    todo_install_ok=1
    ok "Azure CLI installed for the optional private Microsoft app registration."
    return 0
  )
}
register_todo_private_app_with_azure_cli(){
  # This path is deliberately opt-in and uses Azure CLI as the administrator's
  # temporary bootstrap client. Dash-Go itself never receives app-management
  # permissions, and no Azure CLI login state is retained after this action.
  (
    umask 077
    if ! command -v az >/dev/null 2>&1; then
      install_azure_cli_for_todo_registration
      todo_az_install_status=$?
      case "$todo_az_install_status" in
        0) ;;
        2) exit 0 ;;
        *) exit 1 ;;
      esac
    fi
    echo "This creates one private Microsoft Entra public-client registration in the tenant"
    echo "of the Microsoft account you sign into with Azure CLI. No client secret is created."
    echo "The signed-in account must be allowed to create app registrations in that tenant."
    read -rp "  Type CREATE PRIVATE APP to continue [blank cancels]: " todo_create_confirm
    if [ "${todo_create_confirm:-}" != "CREATE PRIVATE APP" ]; then
      echo "Private app creation cancelled; local Lists remain unchanged."
      exit 0
    fi

    todo_az_dir="$(mktemp -d "${TMPDIR:-/tmp}/dash-go-entra.XXXXXX")" || { warn "Could not create a temporary Azure CLI profile."; exit 1; }
    trap 'rm -rf "$todo_az_dir"' EXIT HUP INT TERM
    todo_manifest="$todo_az_dir/required-resource-accesses.json"
    cat > "$todo_manifest" <<'TODO_AZ_MANIFEST'
[
  {
    "resourceAppId": "00000003-0000-0000-c000-000000000000",
    "resourceAccess": [
      {
        "id": "2219042f-cab5-40cc-b0d2-16b1540b4c5f",
        "type": "Scope"
      }
    ]
  }
]
TODO_AZ_MANIFEST

    say "Private Microsoft To Do app registration"
    echo "Azure CLI will show a device-code sign-in. Use the Entra account that should own this registration."
    echo "A temporary Azure CLI profile is used and removed before this installer action ends."
    if ! AZURE_CONFIG_DIR="$todo_az_dir" az login --use-device-code --allow-no-subscriptions --only-show-errors; then
      warn "Azure CLI sign-in did not complete. Nothing changed in Dash-Go."
      exit 1
    fi

    todo_app_name="Dash-Go To Do $(hostname 2>/dev/null || echo kiosk)"
    if ! todo_client_id="$(AZURE_CONFIG_DIR="$todo_az_dir" az ad app create \
      --display-name "$todo_app_name" \
      --sign-in-audience AzureADandPersonalMicrosoftAccount \
      --is-fallback-public-client true \
      --required-resource-accesses "@$todo_manifest" \
      --query appId --output tsv --only-show-errors)"; then
      warn "Azure CLI could not create the private Entra app. The signed-in tenant may block app registration."
      warn "Use the registration guide or an existing private client ID instead."
      exit 1
    fi
    if ! valid_microsoft_client_id "$todo_client_id"; then
      warn "Azure CLI did not return a valid Application (client) ID. Dash-Go settings were not changed."
      exit 1
    fi
    if ! write_todo_app_settings microsoft "$todo_client_id"; then
      warn "The private app was created, but Dash-Go could not save its client ID."
      warn "Application (client) ID: $todo_client_id"
      warn "Copy it and use Microsoft To Do / Graph setup after resolving the settings-file issue."
      exit 1
    fi
    say "Microsoft To Do app registration complete"
    ok "Your private Microsoft app was created and its application ID was saved in Dash-Go."
    echo "   You do not need to copy the client ID or create a client secret."
    echo
    echo "   Next steps"
    echo "   1. Choose 5 to return to the installer, then finish installation."
    echo "   2. On the dashboard, open Dashboard Control > Settings > Lists / optional Microsoft To Do."
    echo "   3. Select Link Microsoft account and complete the device-code sign-in on a phone or computer."
    echo "   4. Refresh Microsoft lists, then choose the To Do and Grocery destinations you want to mirror."
    echo
    echo "   Local Lists remain write-first. Microsoft sync is optional and Azure CLI is not used by Dash-Go at runtime."
  )
}
write_todo_app_registration_guide(){
  local guide="$HOME/Dash-Go_Microsoft_To_Do_Setup.txt"
  cat > "$guide" <<'TODO_APP_GUIDE'
Dash-Go optional Microsoft To Do setup
======================================

Dash-Go Lists are local-first by default. This guide is only needed if you
choose to mirror a local list with Microsoft To Do on your phone.

Microsoft To Do / Graph setup choices
---------------------------
1. Local To Do + Grocery only (recommended/default): no Microsoft account,
   internet connection, or app registration is required.
2. Use an existing private Application (client) ID that you registered.
3. Create a private public-client registration through Azure CLI (advanced):
   If `az` is missing, Microsoft To Do / Graph setup offers an explicit install action on supported
   Debian/Ubuntu amd64 or arm64 systems. It adds Microsoft's signed Azure CLI
   apt source only after confirmation. A 32-bit Raspberry Pi image is not
   offered this package install because the documented package architectures do
   not include armhf. The registration uses a temporary Azure CLI profile and
   removes it after the action; Dash-Go does not keep its administrator session.

Manual private-registration path
--------------------------------
1. In Microsoft Entra admin center, create an App registration owned by you.
2. Select accounts in any organizational directory and personal Microsoft
   accounts.
3. Under Authentication, enable Allow public client flows.
4. Add Microsoft Graph delegated permissions:
   - Tasks.ReadWrite
   - offline_access
   - openid
   - profile
5. Do not create or save a client secret. Dash-Go uses device-code sign-in.
6. Copy the Application (client) ID.
7. Re-run ~/install.sh, choose Microsoft To Do / Graph setup, then
   choose Optional Microsoft To Do sync and enter that client ID. On the dashboard open Dashboard Control >
   Lists / optional Microsoft To Do and choose Link Microsoft account. Complete
   the code on the phone or computer signed into the same Microsoft To Do account.
8. After linking, refresh Microsoft lists and map the To Do/Grocery icons to
   the remote lists you want to mirror.

Security boundary
-----------------
- Dash-Go runtime never receives Microsoft app-management permission such as
  Application.ReadWrite.All. The advanced Azure CLI choice is a user-started,
  temporary administrator action under the operator's own Microsoft account.
  Azure CLI, when explicitly installed, remains a separate system package and
  is not used by the Dash-Go runtime after registration.
- The client ID is not secret. The Microsoft refresh token is written only by
  the local dashboard control server to ~/.dashboard-todo.json with 0600 mode.
- Local Lists remain available if Microsoft is offline, unlinked, or later
  disabled. Choosing local-only pauses cloud sync without deleting local tasks.
TODO_APP_GUIDE
  chmod 600 "$guide" 2>/dev/null || true
  ok "Microsoft To Do setup guide written to $guide"
}
refresh_holidays_for_installer(){
  local holiday_log="$LOG_DIR/holiday-refresh-install.log"
  mkdir -p "$LOG_DIR" 2>/dev/null || true
  if "$BIN_DIR/update-holidays.sh" >"$holiday_log" 2>&1; then
    ok "Holiday calendar refreshed"
    return 0
  fi
  warn "Holiday calendar refresh failed (will retry via cron; details: $holiday_log)"
  return 1
}

configure_app_setup(){
  while :; do
    say "Microsoft To Do / Graph setup"
    echo "Local To Do and Grocery are the default: they save on this device and"
    echo "need neither a Microsoft account nor a network connection. Microsoft"
    echo "To Do is optional and mirrors only after local writes are safely saved."
    echo
    echo "  1) Use local To Do + Grocery only  recommended/default"
    echo "  2) Enable optional Microsoft To Do sync  enter a private client ID"
    echo "  3) Create private Microsoft Graph app  Azure CLI registration (advanced)"
    echo "  4) Microsoft Graph registration guide  write/show setup instructions"
    echo "  5) Back to installer"
    echo
    read -rp "  Choose [1-5, Enter=5]: " app_choice
    app_choice="${app_choice:-5}"
    case "$app_choice" in
      1)
        if write_todo_app_settings local ""; then
          ok "Local To Do and Grocery are active; local lists are the source of truth."
        else
          warn "Could not safely write local Lists settings."
        fi
        ;;
      2)
        echo "Microsoft To Do is opt-in. Keep a private public-client app registration"
        echo "under your ownership; do not enter a client secret because Dash-Go never uses one."
        echo "The dashboard will still save locally first and Microsoft remains a mirror."
        read -rp "  Application (client) ID [blank cancels]: " todo_client_id
        if [ -z "${todo_client_id:-}" ]; then
          echo "No client ID entered; local Lists remain unchanged."
        elif ! valid_microsoft_client_id "$todo_client_id"; then
          warn "That does not look like an Application (client) ID UUID. Nothing changed."
        elif write_todo_app_settings microsoft "$todo_client_id"; then
          ok "Optional Microsoft To Do sync is prepared."
          echo "   Next: Dashboard Control > Settings > Lists / optional Microsoft To Do > Link Microsoft account."
          echo "   Then refresh Microsoft lists and map each launcher icon as desired."
        else
          warn "Could not safely write Microsoft To Do settings."
        fi
        ;;
      3)
        register_todo_private_app_with_azure_cli || true
        ;;
      4)
        write_todo_app_registration_guide
        echo
        sed -n '1,260p' "$HOME/Dash-Go_Microsoft_To_Do_Setup.txt"
        ;;
      5|q|Q|quit|Quit|exit|Exit) return 0;;
      *) warn "Choose 1, 2, 3, 4, or 5.";;
    esac
  done
}

validate_download(){
  local name="$1" path="$2"
  [ -s "$path" ] || return 1
  case "$name" in
    install.sh)
      head -c 80 "$path" | grep -q '^#!/bin/bash'
      ;;
    *.sh)
      ! head -c 120 "$path" | grep -qi '<html\|<!doctype' \
        && bash -n "$path" >/dev/null 2>&1
      ;;
    index.html)
      grep -qiE 'Dash-Go' "$path"
      ;;
    *.css)
      # Accept focused/split CSS files as long as they are not HTML error pages
      # and contain normal CSS syntax. Earlier 1.1.0 rejected
      # ui/control-layout.css because it did not include :root or #app.
      ! head -c 120 "$path" | grep -qi '<html\|<!doctype' \
        && grep -q '[{}]' "$path"
      ;;
    *.js)
      # Reject HTML error/captive-portal pages; otherwise keep JS validation
      # broad because the dashboard is now split across focused files.
      ! head -c 80 "$path" | grep -qi '<html\|<!doctype' && grep -qE 'function|const|let|var|DOMContentLoaded|Dash-Go|CONFIG\.|window\.|=>|use strict' "$path"
      ;;
    *.html)
      grep -qi '<!DOCTYPE html\|<html' "$path"
      ;;
    *.manifest.json|manifest.json|*.json)
      # Fresh bootstrap has no installed Go binary yet. The payload manifest is
      # subsequently checked path-by-path and hash-by-hash before replacement.
      installer_json_validate "$path"
      ;;
    *.tar.gz|*.tgz)
      tar -tzf "$path" >/dev/null 2>&1
      ;;
    *.sha256)
      grep -Eq '^[[:space:]]*[A-Fa-f0-9]{64}([[:space:]]|$)' "$path"
      ;;
    *.ttf)
      # Font downloads are best-effort, but reject HTML/captive-portal pages.
      ! head -c 120 "$path" | grep -qi '<html\|<!doctype' && [ -s "$path" ]
      ;;
    *)
      return 0
      ;;
  esac
}

# GitHub Release metadata and payload helpers. json_field is defined near
# update_cli: Go is authoritative when installed, with a bounded bootstrap
# reader for first install only.
# The bootstrap verifier runs at low priority and remains memory-bounded; it is
# intentionally generous enough for Python startup but bounded on the Pi.
DASH_MANIFEST_VERIFY_VMEM_KB="${DASH_MANIFEST_VERIFY_VMEM_KB:-262144}"
manifest_verify_vmem_kb(){
  case "$DASH_MANIFEST_VERIFY_VMEM_KB" in
    ''|*[!0-9]*) printf '%s\n' 262144 ;;
    *) printf '%s\n' "$DASH_MANIFEST_VERIFY_VMEM_KB" ;;
  esac
}

sha_from_file(){ awk '/[A-Fa-f0-9]{64}/{print $1; exit}' "$1" 2>/dev/null; }
verify_sha256(){
  local expected="$1" file="$2" actual
  [ -n "$expected" ] || return 1
  actual="$(sha256sum "$file" 2>/dev/null | awk '{print $1}')"
  [ -n "$actual" ] || return 1
  [ "$(printf '%s' "$actual" | tr 'A-F' 'a-f')" = "$(printf '%s' "$expected" | tr 'A-F' 'a-f')" ]
}

find_payload_root(){
  local extracted="$1" d
  [ -f "$extracted/index.html" ] && { printf '%s\n' "$extracted"; return 0; }
  [ -f "$extracted/app/index.html" ] && { printf '%s\n' "$extracted/app"; return 0; }
  for d in "$extracted"/*; do
    [ -d "$d" ] && [ -f "$d/index.html" ] && { printf '%s\n' "$d"; return 0; }
    [ -d "$d/app" ] && [ -f "$d/app/index.html" ] && { printf '%s\n' "$d/app"; return 0; }
  done
  return 1
}

atomic_replace_file(){
  local src="$1" dest="$2" dest_dir base tmp
  [ -f "$src" ] || return 1
  dest_dir="$(dirname "$dest")"; base="$(basename "$dest")"
  mkdir -p "$dest_dir" || return 1
  tmp="$dest_dir/.${base}.tmp.$$.$RANDOM"
  rm -f "$tmp" 2>/dev/null || true
  cp -p "$src" "$tmp" && mv -f "$tmp" "$dest"
}

# The Dashboard Control runner deliberately invokes ~/install.sh rather than
# an app-tree copy. Every verified release bundle must therefore refresh this
# canonical launcher as part of the same rollback-capable transaction as the
# app payload. A bundle installer is covered by the verified release archive;
# this helper only accepts the installer beside that extracted app payload.
find_release_bundle_installer(){
  local extracted="$1" payload_root="$2" candidate
  for candidate in     "$extracted/install.sh"     "$(dirname "$payload_root")/install.sh"; do
    [ -f "$candidate" ] || continue
    validate_download install.sh "$candidate" || continue
    printf '%s\n' "$candidate"
    return 0
  done
  return 1
}

snapshot_canonical_installer(){
  local stage="$1" source="$2" backup
  backup="$stage/canonical-installer"
  [ -f "$source" ] && validate_download install.sh "$source" || return 1
  mkdir -p "$backup" || return 1
  if [ -e "$INSTALLER" ]; then
    [ -f "$INSTALLER" ] || return 1
    cp -p "$INSTALLER" "$backup/previous" || return 1
    printf '%s\n' present > "$backup/state" || return 1
  else
    printf '%s\n' absent > "$backup/state" || return 1
  fi
}

restore_canonical_installer(){
  local stage="$1" backup state=""
  backup="$stage/canonical-installer"
  [ -f "$backup/state" ] || return 0
  state="$(tr -d '\r\n[:space:]' < "$backup/state" 2>/dev/null || true)"
  case "$state" in
    present)
      [ -f "$backup/previous" ] && validate_download install.sh "$backup/previous" || return 1
      atomic_replace_file "$backup/previous" "$INSTALLER" || return 1
      chmod 700 "$INSTALLER" || return 1
      ;;
    absent)
      rm -f "$INSTALLER" || return 1
      ;;
    *) return 1;;
  esac
}

install_canonical_installer(){
  local source="$1"
  [ -f "$source" ] && validate_download install.sh "$source" || return 1
  atomic_replace_file "$source" "$INSTALLER" || return 1
  chmod 700 "$INSTALLER" || return 1
  cmp -s "$source" "$INSTALLER"
}

# Personal dashboard state is mutable user data. A release payload must never
# replace it. Keep a small explicit guard as well as excluding these paths from
# the tarball/manifest install list.
snapshot_personal_settings(){
  local root="$1" rel
  mkdir -p "$root/files" || return 1
  : > "$root/state"
  for rel in \
    config/config.local.js config/settings.json config/compliments.json \
    config/message-sources.json config/message-cache-overrides.json \
    config/temp-messages.json config/scheduled-messages.json \
    config/chalkboard.json config/map-provider.json config/notification-preferences.json calendars/calendars.json; do
    if [ -f "$DASH/$rel" ]; then
      mkdir -p "$root/files/$(dirname "$rel")"
      cp -p "$DASH/$rel" "$root/files/$rel" || return 1
      printf 'present %s\n' "$rel" >> "$root/state"
    else
      printf 'absent %s\n' "$rel" >> "$root/state"
    fi
  done
  # Local To Do/Grocery data is user-owned state. It is a directory of hashed
  # per-list caches, so preserve/restore it as one unit rather than treating it
  # as a generated release asset or a single preference file.
  if [ -d "$DASH/config/todo" ]; then
    mkdir -p "$root/files/config"
    cp -a "$DASH/config/todo" "$root/files/config/todo" || return 1
    printf 'present_dir config/todo\n' >> "$root/state"
  else
    printf 'absent_dir config/todo\n' >> "$root/state"
  fi
  # Apprise destinations are private, home-side data. Preserve them during a
  # payload swap without printing their URLs or putting them in app assets.
  if [ -f "$HOME/.config/dash-go/apprise/routes.json" ]; then
    mkdir -p "$root/files/home/.config/dash-go/apprise"
    cp -p "$HOME/.config/dash-go/apprise/routes.json" "$root/files/home/.config/dash-go/apprise/routes.json" || return 1
    printf 'present_home .config/dash-go/apprise/routes.json\n' >> "$root/state"
  else
    printf 'absent_home .config/dash-go/apprise/routes.json\n' >> "$root/state"
  fi

}
restore_personal_settings(){
  local root="$1" state rel kind
  [ -f "$root/state" ] || return 0
  while read -r kind rel; do
    [ -n "${rel:-}" ] || continue
    case "$kind" in
      present)
        mkdir -p "$DASH/$(dirname "$rel")"
        cp -p "$root/files/$rel" "$DASH/$rel" || return 1
        ;;
      absent)
        rm -f "$DASH/$rel"
        ;;
      present_dir)
        rm -rf "$DASH/$rel"
        mkdir -p "$DASH/$(dirname "$rel")"
        cp -a "$root/files/$rel" "$DASH/$rel" || return 1
        ;;
      absent_dir)
        rm -rf "$DASH/$rel"
        ;;
      present_home)
        mkdir -p "$HOME/$(dirname "$rel")"
        cp -p "$root/files/home/$rel" "$HOME/$rel" || return 1
        chmod 600 "$HOME/$rel" 2>/dev/null || true
        if [ "$rel" = ".config/dash-go/apprise/routes.json" ]; then
          chmod 700 "$HOME/.config" "$HOME/.config/dash-go" "$HOME/.config/dash-go/apprise" 2>/dev/null || true
        fi
        ;;
      absent_home)
        rm -f "$HOME/$rel"
        ;;
    esac
  done < "$root/state"
}

# Return the exact staged/runtime Go binary for this host.  Generated-asset
# verification must never execute a generic selector from a newly extracted
# release payload: a selector/package mix-up can otherwise hide an ARM/x86
# mismatch until the device sees an Exec format error.
release_server_for_host(){
  local root="$1" arch bin
  case "$(uname -m 2>/dev/null || true)" in
    x86_64|amd64) arch="amd64" ;;
    i?86) arch="386" ;;
    aarch64|arm64) arch="arm64" ;;
    armv6l) arch="armv6" ;;
    armv7l|armv8l) arch="armv7" ;;
    *) return 1 ;;
  esac
  bin="$root/bin/dashboard-control-server-linux-$arch"
  [ -x "$bin" ] || return 1
  printf '%s\n' "$bin"
}

# Compatibility recovery only: modern release payloads own this selector and
# its manifest hash. Never rewrite an existing selector after payload commit,
# rollback, or ordinary repair; doing so would make the installed bytes differ
# from the signed release manifest. This fallback exists solely for a very old
# installation which has architecture binaries but no generic selector at all.
ensure_missing_go_selector_wrapper(){
  local selector="$DASH/bin/dashboard-control-server" tmp arch have_arch_bin=0
  [ -d "$DASH/bin" ] || return 0
  [ -e "$selector" ] && return 0
  # Legacy reconstruction is missing-only, but it must recognize every
  # architecture shipped by the canonical selector. A pre-selector rollback on
  # arm64, armv6, or 386 must remain bootable just like amd64 and armv7.
  for arch in amd64 386 arm64 armv7 armv6; do
    if [ -x "$DASH/bin/dashboard-control-server-linux-$arch" ]; then
      have_arch_bin=1
      break
    fi
  done
  [ "$have_arch_bin" = 1 ] || return 0
  tmp="$selector.tmp.$$"
  cat > "$tmp" <<'EOSERVERWRAP'
#!/usr/bin/env sh
set -eu
DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ARCH=$(uname -m 2>/dev/null || echo unknown)
case "$ARCH" in
  x86_64|amd64) BIN="$DIR/dashboard-control-server-linux-amd64" ;;
  i386|i486|i586|i686|x86) BIN="$DIR/dashboard-control-server-linux-386" ;;
  aarch64|arm64) BIN="$DIR/dashboard-control-server-linux-arm64" ;;
  armv7l|armv7*|armv8l) BIN="$DIR/dashboard-control-server-linux-armv7" ;;
  armv6l|armv6*) BIN="$DIR/dashboard-control-server-linux-armv6" ;;
  *) echo "unsupported dashboard Go server architecture: $ARCH" >&2; exit 127 ;;
esac
if [ ! -x "$BIN" ]; then
  echo "dashboard server binary missing or not executable for $ARCH: $BIN" >&2
  exit 126
fi
exec "$BIN" "$@"
EOSERVERWRAP
  chmod 755 "$tmp" 2>/dev/null || { rm -f "$tmp"; return 1; }
  mv -f "$tmp" "$selector"
}

# Verify the live managed tree only after a payload transaction has finished
# replacing files. This closes the gap where a later installer helper could
# mutate a manifest-managed file after staging validation had already passed.
verify_installed_release_manifest(){
  local version="$1" verifier arch target
  [ -n "$version" ] || { warn "post-install package integrity check has no installed version"; return 1; }
  [ -s "$DASH/manifest.json" ] || { warn "post-install package integrity check cannot find $DASH/manifest.json"; return 1; }
  case "$(uname -m 2>/dev/null || true)" in
    x86_64|amd64) arch="amd64" ;;
    i?86) arch="386" ;;
    aarch64|arm64) arch="arm64" ;;
    armv6l) arch="armv6" ;;
    armv7l|armv8l) arch="armv7" ;;
    *) warn "post-install package integrity check does not support $(uname -m 2>/dev/null || echo unknown)"; return 1 ;;
  esac
  target="bin/dashboard-control-server-linux-$arch"
  verifier="$(release_server_for_host "$DASH" 2>/dev/null || true)"
  [ -n "$verifier" ] || { warn "post-install package integrity verifier is missing for $arch"; return 1; }
  if ! "$verifier" --verify-release-manifest --manifest "$DASH/manifest.json" --root "$DASH" --version "$version" --target-bin "$target"; then
    warn "post-install package integrity verification failed"
    return 1
  fi
  return 0
}

service_unit_section_has_setting(){
  local path="$1" section="$2" setting="$3"
  [ -f "$path" ] || return 1
  awk -v section="$section" -v setting="$setting" '
    /^\[/ { inside=($0 == "[" section "]"); next }
    inside && $0 == setting { found=1 }
    END { exit(found ? 0 : 1) }
  ' "$path" 2>/dev/null
}

ensure_go_dashboard_service_unit(){
  [ -x "$DASH/bin/dashboard-control-server" ] || return 0
  command -v systemctl >/dev/null 2>&1 || return 0
  [ -n "${SUDO:-}" ] || SUDO=""
  local svc=/etc/systemd/system/dashboard-server.service
  [ -f "$svc" ] || return 0
  if service_unit_section_has_setting "$svc" Unit 'StartLimitIntervalSec=120' \
    && service_unit_section_has_setting "$svc" Unit 'StartLimitBurst=5' \
    && service_unit_section_has_setting "$svc" Service "ExecStart=$DASH/bin/dashboard-control-server" \
    && service_unit_section_has_setting "$svc" Service 'Environment=GOMEMLIMIT=96MiB' \
    && service_unit_section_has_setting "$svc" Service 'Restart=always' \
    && service_unit_section_has_setting "$svc" Service 'RestartSec=3'; then
    return 0
  fi
  $SUDO cp "$svc" "$svc.bak" 2>/dev/null || true
  $SUDO tee "$svc" >/dev/null <<UNIT
[Unit]
Description=Dash-Go local web server
After=network.target
StartLimitIntervalSec=120
StartLimitBurst=5

[Service]
Type=simple
User=$USER_NAME
WorkingDirectory=$DASH
Environment=GOMEMLIMIT=96MiB
ExecStart=$DASH/bin/dashboard-control-server
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
UNIT
  $SUDO systemctl daemon-reload >/dev/null 2>&1 || true
}


find_visudo_path(){
  local candidate
  for candidate in /usr/sbin/visudo /usr/bin/visudo; do
    [ -x "$candidate" ] && { printf '%s\n' "$candidate"; return 0; }
  done
  command -v visudo 2>/dev/null || return 1
}

dashboard_update_service_is_ready(){
  local systemctl_path="$1" output
  output="$($SUDO -n "$systemctl_path" show dash-go-update.service 2>/dev/null || true)"
  printf '%s\n' "$output" | grep -Fqx 'LoadState=loaded'
}

ensure_dashboard_update_service(){
  local svc=/etc/systemd/system/dash-go-update.service sudoers=/etc/sudoers.d/012-dashboard-app-update
  local systemctl_path visudo_path svc_tmp sudoers_tmp output
  [ -x "$BIN_DIR/dashboard-update-runner.sh" ] || { warn "update-service runner is missing or not executable"; return 1; }
  command -v systemctl >/dev/null 2>&1 || { warn "update-service provisioning requires systemctl"; return 1; }
  systemctl_path="$(command -v systemctl 2>/dev/null || true)"
  [ -n "$systemctl_path" ] || { warn "update-service provisioning could not resolve systemctl"; return 1; }
  visudo_path="$(find_visudo_path 2>/dev/null || true)"
  [ -n "$visudo_path" ] || { warn "update-service provisioning could not find visudo at /usr/sbin/visudo, /usr/bin/visudo, or PATH"; return 1; }
  mkdir -p "$CACHE_DIR" 2>/dev/null || return 1
  svc_tmp="$(mktemp "$CACHE_DIR/.dash-go-update-service.XXXXXX")" || return 1
  sudoers_tmp="$(mktemp "$CACHE_DIR/.dash-go-update-sudoers.XXXXXX")" || { rm -f "$svc_tmp"; return 1; }
  cleanup_update_service_temp(){ rm -f "$svc_tmp" "$sudoers_tmp" 2>/dev/null || true; }
  cat > "$svc_tmp" <<UNIT
[Unit]
Description=Dash-Go dedicated application updater
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
User=$USER_NAME
WorkingDirectory=$HOME
Environment=HOME=$HOME
ExecStart=$BIN_DIR/dashboard-update-runner.sh
Nice=10
IOSchedulingClass=idle

[Install]
WantedBy=multi-user.target
UNIT
  {
    echo "# Written by Dash-Go install.sh. Allows Dashboard Control to start/query only its dedicated application updater."
    echo "$USER_NAME ALL=(root) NOPASSWD: $systemctl_path start --no-block dash-go-update.service, $systemctl_path show dash-go-update.service"
  } > "$sudoers_tmp"
  chmod 0600 "$svc_tmp" "$sudoers_tmp" 2>/dev/null || true
  if ! $SUDO "$visudo_path" -cf "$sudoers_tmp" >/dev/null 2>&1; then
    cleanup_update_service_temp
    warn "update-service sudoers staging validation failed at $visudo_path"
    return 1
  fi
  if ! $SUDO mkdir -p /etc/systemd/system /etc/sudoers.d; then
    cleanup_update_service_temp
    warn "update-service provisioning could not create system directories"
    return 1
  fi
  if ! $SUDO install -m 0644 "$svc_tmp" "$svc"; then
    cleanup_update_service_temp
    warn "update-service provisioning could not install $svc"
    return 1
  fi
  if ! $SUDO install -m 0440 "$sudoers_tmp" "$sudoers"; then
    cleanup_update_service_temp
    warn "update-service provisioning could not install $sudoers"
    return 1
  fi
  if ! $SUDO "$visudo_path" -cf "$sudoers" >/dev/null 2>&1; then
    cleanup_update_service_temp
    warn "update-service sudoers validation failed after install"
    return 1
  fi
  if ! $SUDO systemctl daemon-reload; then
    cleanup_update_service_temp
    warn "update-service provisioning could not reload systemd"
    return 1
  fi
  # Deliberately drop any cached terminal authorization before checking the
  # exact Control command. A successful query proves the scoped NOPASSWD rule,
  # not merely a password cached by repair/install earlier in this session.
  $SUDO -k >/dev/null 2>&1 || true
  output="$($SUDO -n "$systemctl_path" show dash-go-update.service 2>&1 || true)"
  if ! printf '%s\n' "$output" | grep -Fqx 'LoadState=loaded'; then
    cleanup_update_service_temp
    warn "update-service scoped sudo verification failed: ${output:-no systemctl response}"
    return 1
  fi
  cleanup_update_service_temp
  ok "dedicated Dash-Go update service is installed and its scoped start/query permission was verified"
}


# Release manifests describe the current package, but older payloads may have
# left retired split CSS/JS/Go source files behind. Generated-asset validation
# globs those package-owned source folders, so remove only unmanifested source
# files from the three immutable code trees after a successful staged commit.
# User-owned config, calendars, cache, logs, fonts, and other runtime data are
# intentionally outside this narrow cleanup.
purge_stale_managed_sources(){
  local root="$1" manifest="$2" backup="$3"
  update_cli_supports '--purge-stale-managed' || {
    warn "installed updater is missing the Go stale-source cleanup operation"
    return 1
  }
  update_cli --purge-stale-managed --root "$root" --manifest "$manifest" --backup "$backup"
}

update_commit_required_free_mb(){
  local payload_root="$1" payload_mb required
  payload_mb="$(du -sm "$payload_root" 2>/dev/null | awk 'NR==1{print $1}')"
  case "$payload_mb" in ''|*[!0-9]*) payload_mb=0;; esac
  # Stage, rollback snapshot, and atomic replacement need more room than the
  # archive alone. Keep a conservative floor for small releases.
  required=$((payload_mb * 2 + 64))
  [ "$required" -ge 300 ] || required=300
  printf '%s\n' "$required"
}

require_update_commit_space(){
  local target="$1" payload_root="$2" free_mb required
  required="$(update_commit_required_free_mb "$payload_root")"
  free_mb="$(df -Pm "$target" 2>/dev/null | awk 'NR==2{print $4}')"
  case "$free_mb" in
    ''|*[!0-9]*)
      warn "Could not check free space before the update commit. Nothing was changed."
      return 1
      ;;
  esac
  if [ "$free_mb" -lt "$required" ]; then
    warn "Not enough free space to update safely (${required} MB needed; ${free_mb} MB available). Free space, then retry. Nothing was changed."
    return 1
  fi
  return 0
}

install_release_payload(){
  local tarball="$1" version="$2" manifest_src="${3:-}" installer_src="${4:-}" stage extract src backup personal file_list rel failed=0 manifest_file generated_check canonical_installer=""
  mkdir -p "$DASH" "$BIN_DIR" "$CONFIG_DIR" "$CAL_DIR" "$CACHE_DIR" "$LOG_DIR" "$FONT_DIR" "$RUNTIME_FONT_DIR" "$BASE_DIR" || return 1
  stage="$(mktemp -d "$DASH/.release.XXXXXX")" || return 1
  extract="$stage/extract"; backup="$stage/backup"; personal="$stage/personal"
  mkdir -p "$extract" "$backup" || { rm -rf "$stage"; return 1; }
  snapshot_personal_settings "$personal" || { rm -rf "$stage"; return 1; }
  write_update_phase validating-payload "Extracting verified release" "Extracting the verified release into a private staging area. Live dashboard files are still unchanged."
  if ! tar -xzf "$tarball" -C "$extract"; then
    warn "release tarball could not be extracted"; rm -rf "$stage"; return 1
  fi
  src="$(find_payload_root "$extract")" || { warn "release tarball does not look like a dashboard payload (index.html missing)"; rm -rf "$stage"; return 1; }
  canonical_installer="$installer_src"
  if [ -z "$canonical_installer" ]; then
    canonical_installer="$(find_release_bundle_installer "$extract" "$src" 2>/dev/null || true)"
  fi
  if [ -z "$canonical_installer" ] || ! validate_download install.sh "$canonical_installer"; then
    warn "verified Dash-Go release bundle is missing its canonical installer"; rm -rf "$stage"; return 1
  fi
  [ -f "$src/VERSION" ] || printf '%s\n' "$version" > "$src/VERSION"
  if [ -n "$manifest_src" ] && [ -s "$manifest_src" ] && [ ! -f "$src/manifest.json" ]; then cp -p "$manifest_src" "$src/manifest.json" || true; fi
  manifest_file="$src/manifest.json"; file_list="$stage/payload-files.txt"
  [ -s "$manifest_file" ] || { warn "release payload is missing manifest.json"; rm -rf "$stage"; return 1; }
  write_update_phase validating-payload "Verifying staged release files" "Checking the manifest, host architecture binary, generated assets, and every managed release file before replacement."
  local manifest_arch="" manifest_target_bin=""
  case "$(uname -m 2>/dev/null || true)" in
    x86_64|amd64) manifest_arch="amd64" ;;
    i?86) manifest_arch="386" ;;
    aarch64|arm64) manifest_arch="arm64" ;;
    armv6l) manifest_arch="armv6" ;;
    armv7l|armv8l) manifest_arch="armv7" ;;
  esac
  [ -z "$manifest_arch" ] || manifest_target_bin="bin/dashboard-control-server-linux-$manifest_arch"
  # Verify the downloaded manifest through the already-installed, trusted Go
  # binary. A beta.69 device uses Python only for this one bridge update before
  # beta.70 installs the trusted Go verifier. We never execute an unverified
  # staged binary merely to validate its own manifest.
  if update_cli_supports '--verify-release-manifest'; then
    if ! update_cli --verify-release-manifest --manifest "$manifest_file" --root "$src" --version "$version" --target-bin "$manifest_target_bin"; then
      warn "release manifest verification failed"; rm -rf "$stage"; return 1
    fi
  else
    # First install has no trusted resident Go binary. Verify the known
    # builder manifest with a bounded shell reader before any staged binary is
    # executed; every managed path and expected SHA-256 is checked below.
    if ! manifest_verify_shell "$manifest_file" "$src" "$version" "$manifest_target_bin"; then
      warn "release manifest verification failed; run ~/install.sh again after checking the downloaded release."
      rm -rf "$stage"; return 1
    fi
  fi
  # The generic server path is a portable shell selector, never a host-built
  # ELF. Validate that release contract before executing anything from staging;
  # then run the selected target directly so a malformed selector cannot mask a
  # target-architecture packaging error.
  if [ ! -x "$src/bin/dashboard-control-server" ] || ! head -c 2 "$src/bin/dashboard-control-server" 2>/dev/null | grep -Fqx '#!'; then
    warn "release payload dashboard-control-server is not the required architecture-selector shell wrapper"; rm -rf "$stage"; return 1
  fi
  local staged_verifier
  staged_verifier="$(release_server_for_host "$src" 2>/dev/null || true)"
  if [ -z "$staged_verifier" ]; then
    warn "release payload generated asset verifier is missing or not executable for $(uname -m 2>/dev/null || echo unknown)"; rm -rf "$stage"; return 1
  fi
  if ! generated_check="$("$staged_verifier" --verify-generated-assets 2>&1)"; then
    warn "release payload generated asset verification failed before install"
    [ -z "$generated_check" ] || printf '   verifier: %s\n' "$generated_check" >&2
    rm -rf "$stage"; return 1
  fi
  if update_cli_supports '--release-file-list'; then
    if ! update_cli --release-file-list --manifest "$manifest_file" > "$file_list"; then
      warn "could not derive release file list from manifest"; rm -rf "$stage"; return 1
    fi
  else
    if ! manifest_file_list_shell "$manifest_file" > "$file_list"; then
      warn "could not derive release file list from manifest; run ~/install.sh again."
      rm -rf "$stage"; return 1
    fi
    grep -Fqx 'manifest.json' "$file_list" || printf 'manifest.json
' >> "$file_list"
  fi
  local critical_files="index.html kiosk.sh VERSION manifest.json ui/dashboard.css ui/control-layout.css ui/js/app.bundle.js ui/js/app.control.bundle.js bin/dashboard-common.sh bin/doctor.sh bin/dashboard-control-server bin/dashboard-control-server-linux-386 bin/dashboard-control-server-linux-amd64 bin/dashboard-control-server-linux-arm64 bin/dashboard-control-server-linux-armv6 bin/dashboard-control-server-linux-armv7 go.mod cmd/dashboard-control-server/main.go"
  for rel in $critical_files; do
    [ -f "$src/$rel" ] || { warn "release payload is missing critical file: $rel"; failed=1; }
  done
  while IFS= read -r rel; do
    [ -n "$rel" ] || continue
    validate_download "$rel" "$src/$rel" || { warn "release payload failed validation: $rel"; failed=1; }
  done < "$file_list"
  [ "$failed" = 0 ] || { rm -rf "$stage"; return 1; }
  if ! require_update_commit_space "$DASH" "$src"; then
    rm -rf "$stage"
    return 1
  fi
  write_update_phase committing "Preparing safe replacement" "The release is verified. Saving current managed files for rollback before any live file is replaced."
  : > "$stage/preexisting-files.txt"
  while IFS= read -r rel; do
    [ -n "$rel" ] && [ -e "$DASH/$rel" ] && printf '%s\n' "$rel" >> "$stage/preexisting-files.txt"
  done < "$file_list"
  while IFS= read -r rel; do
    [ -n "$rel" ] || continue
    if [ -e "$DASH/$rel" ]; then
      mkdir -p "$backup/$(dirname "$rel")" || { warn "could not prepare rollback backup for $rel"; rm -rf "$stage"; return 1; }
      cp -p "$DASH/$rel" "$backup/$rel" || { warn "could not back up $rel before commit"; rm -rf "$stage"; return 1; }
    fi
  done < "$file_list"
  if ! snapshot_canonical_installer "$stage" "$canonical_installer"; then
    warn "could not prepare the canonical installer for rollback"; rm -rf "$stage"; return 1
  fi
  write_update_phase committing "Installing verified release" "Replacing managed dashboard files from the verified staging area. Personal settings, calendars, cache, logs, and backups stay preserved."
  while IFS= read -r rel; do
    [ -n "$rel" ] || continue
    if ! atomic_replace_file "$src/$rel" "$DASH/$rel"; then
      rollback_release_transaction "$stage" "Commit failed while replacing $rel" || true
      return 1
    fi
  done < "$file_list"
  if ! purge_stale_managed_sources "$DASH" "$manifest_file" "$backup/stale-managed" >/dev/null; then
    rollback_release_transaction "$stage" "Could not remove stale managed source files after update" || true
    return 1
  fi

  chmod +x "$DASH/bin"/*.sh "$DASH/bin"/dashboard-control-server* "$DASH/kiosk.sh" 2>/dev/null || true
  if ! restore_personal_settings "$personal"; then
    rollback_release_transaction "$stage" "Could not restore protected personal settings after update" || true
    return 1
  fi
  refresh_private_calendar_configuration_after_payload
  write_update_phase committing "Refreshing local updater" "Installing the matching release installer for future Dashboard Control and SSH updates."
  if ! install_canonical_installer "$canonical_installer"; then
    rollback_release_transaction "$stage" "Could not install the canonical Dash-Go updater" || true
    return 1
  fi
  if ! verify_installed_release_manifest "$version"; then
    rollback_release_transaction "$stage" "Post-install package integrity check failed" || true
    return 1
  fi
  local installed_verifier
  installed_verifier="$(release_server_for_host "$DASH" 2>/dev/null || true)"
  if [ -z "$installed_verifier" ] || ! generated_check="$("$installed_verifier" --verify-generated-assets 2>&1)"; then
    [ -z "${generated_check:-}" ] || printf '   verifier: %s\n' "$generated_check" >&2
    rollback_release_transaction "$stage" "Post-install generated asset check failed" || true
    return 1
  fi
  ensure_go_dashboard_service_unit || true
  # A Control-launched update is already inside the restricted dedicated unit.
  # It may query that unit but must never attempt to rewrite system files from
  # the service account. SSH/repair paths provision or repair it explicitly.
  if [ "${DASH_UPDATE_SOURCE:-ssh}" = "control" ] && [ "${DASH_UPDATE_EXTERNAL_RUNNER:-0}" = "1" ]; then
    if ! dashboard_update_service_is_ready "$(command -v systemctl 2>/dev/null || true)"; then
      warn "dedicated updater service could not be verified after payload install; Dashboard Control remains blocked until repair --system succeeds"
    fi
  elif ! ensure_dashboard_update_service; then
    warn "dedicated updater service was not installed; Dashboard Control updates remain blocked until repair --system succeeds"
  fi
  # The payload has now replaced the trusted host binary. Prove that the new
  # binary, not shipped Go source, exposes every Go-owned updater primitive
  # before retaining the transaction as a runtime rollback candidate.
  if ! verify_go_updater_capabilities; then
    rollback_release_transaction "$stage" "The newly installed Go updater did not expose the required release capabilities" || true
    return 1
  fi
  if ! write_updater_migration_receipt; then
    rollback_release_transaction "$stage" "The newly installed Go updater could not record its verified migration receipt" || true
    return 1
  fi
  retain_update_rollback_stage "$stage"
  ok "installed dashboard release $version (personal settings preserved)"
  return 0
}

# Runtime rollback is update-only. The normal staged commit still rolls back
# immediately on file errors; these helpers retain one verified pre-update
# snapshot long enough for a bounded runtime probe after a browser/server
# recycle.
retain_update_rollback_stage(){
  local stage="$1" stamp dest
  [ "${DASH_UPDATE_RUNTIME_ROLLBACK:-0}" = 1 ] || { rm -rf "$stage"; return 0; }
  stamp="$(date +%Y%m%d-%H%M%S)"; dest="$CACHE_DIR/update-rollback/pending-$stamp"
  mkdir -p "$CACHE_DIR/update-rollback" 2>/dev/null || { rm -rf "$stage"; return 0; }
  mv "$stage" "$dest" 2>/dev/null || { rm -rf "$stage"; return 0; }
  UPDATE_ROLLBACK_STAGE="$dest"; export UPDATE_ROLLBACK_STAGE
  # Keep only the two newest completed/pending snapshots; active snapshot is
  # excluded from cleanup by name until the verifier resolves it.
  find "$CACHE_DIR/update-rollback" -mindepth 1 -maxdepth 1 -type d -name 'pending-*' -printf '%T@ %p\n' 2>/dev/null | sort -nr | awk 'NR>2{print $2}' | xargs -r rm -rf 2>/dev/null || true
}

preserve_failed_update_transaction(){
  local stage="$1" reason="$2" stamp dest
  [ -d "$stage" ] || return 0
  stamp="$(date +%Y%m%d-%H%M%S)"; dest="$CACHE_DIR/update-rollback/failed-$stamp"
  mkdir -p "$CACHE_DIR/update-rollback" 2>/dev/null || return 0
  printf '%s\n' "$reason" > "$stage/failure-reason.txt" 2>/dev/null || true
  mv "$stage" "$dest" 2>/dev/null || return 0
  find "$CACHE_DIR/update-rollback" -mindepth 1 -maxdepth 1 -type d -name 'failed-*' -printf '%T@ %p\n' 2>/dev/null | sort -nr | awk 'NR>2{print $2}' | xargs -r rm -rf 2>/dev/null || true
}
rollback_update_payload(){
  local stage="$1" file_list="$1/payload-files.txt" existed="$1/preexisting-files.txt" backup="$1/backup" rel stale_list
  [ -d "$stage" ] && [ -f "$file_list" ] && [ -f "$existed" ] || return 1
  while IFS= read -r rel; do
    [ -n "$rel" ] || continue
    if grep -Fqx "$rel" "$existed" 2>/dev/null; then
      [ -e "$backup/$rel" ] && atomic_replace_file "$backup/$rel" "$DASH/$rel" || return 1
    else
      rm -f "$DASH/$rel" || return 1
    fi
  done < "$file_list"
  if [ -d "$backup/stale-managed" ]; then
    stale_list="$stage/stale-managed-files.txt"
    (cd "$backup/stale-managed" && find . -type f -print0) > "$stale_list" || return 1
    while IFS= read -r -d '' rel; do
      rel="${rel#./}"; mkdir -p "$(dirname "$DASH/$rel")" || return 1
      atomic_replace_file "$backup/stale-managed/$rel" "$DASH/$rel" || return 1
    done < "$stale_list"
  fi
  restore_personal_settings "$stage/personal" || return 1
  restore_canonical_installer "$stage" || return 1
  # A very old rollback target may predate the packaged selector. Reconstruct
  # only when it is entirely absent; never replace an existing manifest file.
  ensure_missing_go_selector_wrapper || true
  chmod +x "$BIN_DIR"/*.sh "$BIN_DIR"/dashboard-control-server* "$DASH/kiosk.sh" 2>/dev/null || true
  local restored_version
  restored_version="$(cat "$DASH/VERSION" 2>/dev/null | head -1 || true)"
  [ -n "$restored_version" ] || return 1
  verify_installed_release_manifest "$restored_version" || return 1
  local rollback_verifier
  rollback_verifier="$(release_server_for_host "$DASH" 2>/dev/null || true)"
  [ -n "$rollback_verifier" ] || return 1
  "$rollback_verifier" --verify-generated-assets >/dev/null 2>&1 || return 1
  return 0
}
rollback_release_transaction(){
  local stage="$1" reason="$2"
  warn "$reason — restoring the verified previous dashboard payload"
  if rollback_update_payload "$stage"; then
    preserve_failed_update_transaction "$stage" "$reason (rollback verified)"
    write_update_status rolledback "Rolled back" "$reason; the previous release payload was restored and verified." 1 || true
    write_update_job rolledback "Rolled back" "$reason; the previous release payload was restored and verified." 1 || true
    return 0
  fi
  preserve_failed_update_transaction "$stage" "$reason (rollback failed)"
  write_update_status failed "Rollback failed" "$reason; rollback could not verify the prior payload. Inspect update evidence immediately." 1 || true
  write_update_job failed "Rollback failed" "$reason; rollback could not verify the prior payload. Inspect update evidence immediately." 1 || true
  return 1
}

write_post_update_verify_marker(){
  local stage="$1" target="$2" previous="$3" marker="$CACHE_DIR/post-update-verify.json"
  update_cli_supports '--update-job' || return 1
  update_cli --update-job --file "$marker" --state pending --label "Post-update health check pending" \
    --detail "The updated local server is being checked before its rollback snapshot is discarded." \
    --source "${DASH_UPDATE_SOURCE:-ssh}" --target "$target" --previous-version "$previous" --stage "$stage" \
    --health-checked false --rolled-back false >/dev/null 2>&1
}

run_post_update_verifier(){
  local verifier="$BIN_DIR/dashboard-post-update-verify.sh"
  [ -n "${UPDATE_ROLLBACK_STAGE:-}" ] && [ -x "$verifier" ] || return 0
  write_post_update_verify_marker "$UPDATE_ROLLBACK_STAGE" "${DASH_INSTALLED_VERSION:-latest}" "${UPDATE_PREVIOUS_VERSION:-unknown}" || return 1
  # The dedicated updater stays alive while this bounded verifier runs. This
  # keeps status truthful and avoids an unproven user-manager/setsid escape.
  "$verifier"
}

github_digest_sha256(){
  local raw="${1:-}"
  raw="$(printf '%s' "$raw" | tr '[:upper:]' '[:lower:]')"
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

resolve_github_release(){
  local dest="$1" target="${2:-latest}" resolver args=()
  resolver="$(update_cli_for_host)"
  [ -n "$resolver" ] && [ -x "$resolver" ] || { warn "the installed dashboard server is required to resolve canonical GitHub Releases"; return 1; }
  if ! update_cli_supports '--resolve-github-release'; then
    warn "the installed dashboard server does not support GitHub Release resolution; run the downloaded release bundle installer once to complete migration"
    return 1
  fi
  args=(--resolve-github-release --track "$RELEASE_TRACK" --cache-file "$CACHE_DIR/github-release-cache.json")
  if [ "$target" != "latest" ]; then
    args+=(--version "$target")
  fi
  "$resolver" "${args[@]}" > "$dest"
}

validate_github_release_resolution(){
  local meta="$1" target="${2:-latest}" version tag track repo immutable release_name sums_name release_url sums_url release_digest sums_digest
  version="$(json_field "$meta" version 2>/dev/null || true)"
  tag="$(json_field "$meta" tag 2>/dev/null || true)"
  track="$(json_field "$meta" track 2>/dev/null || true)"
  repo="$(json_field "$meta" repository 2>/dev/null || true)"
  immutable="$(json_field "$meta" immutable 2>/dev/null || true)"
  [ -n "$version" ] && [[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-beta\.[0-9]+)?$ ]] || return 1
  [ "$tag" = "v$version" ] || return 1
  [ "$track" = "$RELEASE_TRACK" ] || return 1
  [ "$repo" = "DashDashGoApp/Dash-Go" ] || return 1
  [ "$immutable" = "true" ] || return 1
  [ "$target" = "latest" ] || [ "$version" = "$target" ] || return 1
  release_name="$(json_field "$meta" assets.release.name 2>/dev/null || true)"
  sums_name="$(json_field "$meta" assets.checksums.name 2>/dev/null || true)"
  release_url="$(json_field "$meta" assets.release.browser_download_url 2>/dev/null || true)"
  sums_url="$(json_field "$meta" assets.checksums.browser_download_url 2>/dev/null || true)"
  release_digest="$(json_field "$meta" assets.release.digest 2>/dev/null || true)"
  sums_digest="$(json_field "$meta" assets.checksums.digest 2>/dev/null || true)"
  [ "$release_name" = "Dash-Go_${version}_release.tar.gz" ] || return 1
  [ "$sums_name" = "SHA256SUMS" ] || return 1
  case "$release_url" in https://github.com/*|https://objects.githubusercontent.com/*|https://github-releases.githubusercontent.com/*) ;; *) return 1;; esac
  case "$sums_url" in https://github.com/*|https://objects.githubusercontent.com/*|https://github-releases.githubusercontent.com/*) ;; *) return 1;; esac
  github_digest_sha256 "$release_digest" >/dev/null || return 1
  github_digest_sha256 "$sums_digest" >/dev/null || return 1
  return 0
}

# update_plan_cleanup removes only the short-lived resolved metadata used by one
# locked update attempt. It never touches GitHub's bounded ETag cache or any
# staged release/rollback evidence.
update_plan_cleanup(){
  if [ -n "${UPDATE_PLAN_DIR:-}" ] && [ -d "$UPDATE_PLAN_DIR" ]; then rm -rf "$UPDATE_PLAN_DIR"; fi
  UPDATE_PLAN_DIR=""; UPDATE_PLAN_META=""; UPDATE_PLAN_KIND=""; UPDATE_PLAN_VERSION=""; UPDATE_PLAN_COMPARISON=""; UPDATE_PLAN_NOOP=0
}

update_plan_local_bundle(){
  local target="${1:-latest}" version
  [ -f "$INSTALLER_SOURCE_DIR/app/index.html" ] || return 1
  [ -f "$INSTALLER_SOURCE_DIR/app/manifest.json" ] || return 1
  version="$(bundle_release_version 2>/dev/null || true)"
  valid_release_version "$version" || return 1
  [ "$target" = "latest" ] || [ "$target" = "$version" ] || return 1
  UPDATE_PLAN_KIND="local"; UPDATE_PLAN_VERSION="$version"
  return 0
}

update_plan_github_release(){
  local target="${1:-latest}" meta
  mkdir -p "$DASH" "$CACHE_DIR" 2>/dev/null || return 1
  UPDATE_PLAN_DIR="$(mktemp -d "$CACHE_DIR/.update-release-plan.XXXXXX")" || return 1
  meta="$UPDATE_PLAN_DIR/resolved-release.json"
  if ! resolve_github_release "$meta" "$target" || ! validate_github_release_resolution "$meta" "$target"; then
    rm -rf "$UPDATE_PLAN_DIR"; UPDATE_PLAN_DIR=""; return 1
  fi
  UPDATE_PLAN_KIND="github"; UPDATE_PLAN_META="$meta"; UPDATE_PLAN_VERSION="$(json_field "$meta" version 2>/dev/null || true)"
  valid_release_version "$UPDATE_PLAN_VERSION" || { update_plan_cleanup; return 1; }
  return 0
}

# plan_normal_update_candidate resolves once while the shared update lock is
# held, then decides whether a strictly newer payload exists. A newer local
# bundle remains usable offline; an equal/older local bundle is never applied
# and falls through to the selected immutable GitHub Release. The private
# DASHGO_INSTALLER_SMOKE switch is builder-only and permits package validation
# of a same-version local bundle; it is not exposed by any user-facing flag.
plan_normal_update_candidate(){
  local target="${1:-latest}" installed relation local_kind=""
  update_plan_cleanup
  installed="$(installed_dashboard_version)"
  UPDATE_PREVIOUS_VERSION="$installed"
  DASH_INSTALLED_VERSION="$installed"
  export UPDATE_PREVIOUS_VERSION DASH_INSTALLED_VERSION

  if update_plan_local_bundle "$target"; then
    local_kind="$UPDATE_PLAN_KIND"
    if [ "${DASHGO_INSTALLER_SMOKE:-0}" = "1" ]; then
      UPDATE_PLAN_COMPARISON="smoke-same-version"
      return 0
    fi
    relation="$(release_version_relation "$UPDATE_PLAN_VERSION" "$installed" 2>/dev/null || true)"
    if [ -z "$relation" ]; then
      # A damaged/missing installed VERSION is a recovery case; a verified
      # local release bundle may safely restore it.
      UPDATE_PLAN_COMPARISON="invalid-installed"
      return 0
    fi
    if [ "$relation" = "newer" ]; then
      UPDATE_PLAN_COMPARISON="$relation"
      return 0
    fi
    # Do not use equal/older local bytes. Retain no plan so GitHub can provide
    # a newer eligible selected-track release when the network is available.
    local_kind="$UPDATE_PLAN_KIND"
    UPDATE_PLAN_KIND=""; UPDATE_PLAN_VERSION=""; UPDATE_PLAN_COMPARISON="$relation"
  fi

  if ! update_plan_github_release "$target"; then
    [ -n "$local_kind" ] && warn "the local Dash-Go release bundle is not newer, and canonical GitHub Release discovery did not complete"
    return 1
  fi
  relation="$(release_version_relation "$UPDATE_PLAN_VERSION" "$installed" 2>/dev/null || true)"
  if [ -z "$relation" ]; then
    UPDATE_PLAN_COMPARISON="invalid-installed"
    return 0
  fi
  UPDATE_PLAN_COMPARISON="$relation"
  if [ "$relation" = "newer" ]; then return 0; fi
  UPDATE_PLAN_NOOP=1
  return 0
}

record_no_update_needed(){
  local installed="$1" candidate="$2" relation="$3" label detail
  if [ "$relation" = "older" ]; then
    label="Installed version is newer"
    detail="Installed ${installed}; selected ${RELEASE_TRACK} release ${candidate} is older. Dash-Go will not downgrade, download, replace files, restart services, or recycle the kiosk."
  else
    label="Already up to date"
    detail="Installed ${installed}; selected ${RELEASE_TRACK} release ${candidate} is the same version. No download, replacement, restart, or dashboard refresh was performed."
  fi
  UPDATE_TARGET="$candidate"
  DASH_INSTALLED_VERSION="$installed"
  export UPDATE_TARGET DASH_INSTALLED_VERSION
  write_update_status no-update "$label" "$detail" 0 || true
  # Dashboard Control normally blocks a no-op before creating a job. This
  # terminal record handles only a stale/racing Control request already handed
  # to the dedicated runner, so its linked action can finish truthfully.
  if [ "${DASH_UPDATE_SOURCE:-ssh}" = "control" ] && [ -n "${DASH_UPDATE_JOB_ID:-}" ]; then
    write_update_job no-update "$label" "$detail" 0 || true
  fi
  say "$label"
  ok "$detail"
}

# Download/install from a previously validated resolution so normal updates do
# not resolve GitHub metadata a second time after the no-op decision.
download_release_payload_from_resolution(){
  local meta="$1" target="${2:-latest}" work version release_name sums_name release_url sums_url release_digest sums_digest release_file sums_file sums_expected
  [ -r "$meta" ] || return 1
  work="$(mktemp -d "$DASH/.github-release.XXXXXX")" || return 1
  if ! validate_github_release_resolution "$meta" "$target"; then
    warn "canonical GitHub Release resolution failed for the selected $RELEASE_TRACK track"
    rm -rf "$work"; return 1
  fi
  version="$(json_field "$meta" version)"
  release_name="$(json_field "$meta" assets.release.name)"; sums_name="$(json_field "$meta" assets.checksums.name)"
  release_url="$(json_field "$meta" assets.release.browser_download_url)"; sums_url="$(json_field "$meta" assets.checksums.browser_download_url)"
  release_digest="$(github_digest_sha256 "$(json_field "$meta" assets.release.digest)")" || { rm -rf "$work"; return 1; }
  sums_digest="$(github_digest_sha256 "$(json_field "$meta" assets.checksums.digest)")" || { rm -rf "$work"; return 1; }
  release_file="$work/$release_name"; sums_file="$work/$sums_name"
  write_update_phase validating-payload "Downloading GitHub Release assets" "Downloading Dash-Go $version and its checksum manifest. Live dashboard files remain unchanged until all digest and package checks pass."
  if ! fetch "$release_url" "$release_file" || ! fetch "$sums_url" "$sums_file"; then
    warn "could not download the required GitHub Release assets"; rm -rf "$work"; return 1
  fi
  if ! validate_download "$release_name" "$release_file" || ! validate_download "$sums_name" "$sums_file"; then
    warn "a downloaded GitHub Release asset failed structural validation"; rm -rf "$work"; return 1
  fi
  write_update_phase validating-payload "Verifying GitHub Release assets" "Checking GitHub-reported SHA-256 digests and SHA256SUMS before extracting the release."
  if ! verify_sha256 "$release_digest" "$release_file" || ! verify_sha256 "$sums_digest" "$sums_file"; then
    warn "GitHub Release asset digest mismatch"; rm -rf "$work"; return 1
  fi
  sums_expected="$(github_checksum_for_name "$sums_file" "$release_name" 2>/dev/null || true)"
  if [ -z "$sums_expected" ] || ! verify_sha256 "$sums_expected" "$release_file"; then
    warn "SHA256SUMS does not verify the release bundle"; rm -rf "$work"; return 1
  fi
  if install_release_payload "$release_file" "$version"; then rm -rf "$work"; return 0; fi
  rm -rf "$work"; return 1
}

download_release_payload(){
  local target="${1:-latest}" plan_dir meta rc
  plan_dir="$(mktemp -d "$CACHE_DIR/.adhoc-release-plan.XXXXXX")" || return 1
  meta="$plan_dir/resolved-release.json"
  write_update_phase validating-payload "Resolving GitHub Release" "Resolving the newest eligible immutable GitHub Release for the selected $RELEASE_TRACK track using bounded ETag metadata caching. No package has been downloaded or installed yet."
  if ! resolve_github_release "$meta" "$target"; then
    warn "canonical GitHub Release resolution failed for the selected $RELEASE_TRACK track"
    rm -rf "$plan_dir"; return 1
  fi
  download_release_payload_from_resolution "$meta" "$target"; rc=$?
  rm -rf "$plan_dir"
  return "$rc"
}

install_local_release_bundle(){
  local target="${1:-latest}" version work payload
  [ -f "$INSTALLER_SOURCE_DIR/app/index.html" ] || return 1
  [ -f "$INSTALLER_SOURCE_DIR/app/manifest.json" ] || return 1
  version="$(tr -d '[:space:]' < "$INSTALLER_SOURCE_DIR/app/VERSION" 2>/dev/null || true)"
  [[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-beta\.[0-9]+)?$ ]] || return 1
  [ "$target" = "latest" ] || [ "$target" = "$version" ] || return 1
  work="$(mktemp -d "$DASH/.local-release.XXXXXX")" || return 1
  payload="$work/Dash-Go_${version}_local_payload.tar.gz"
  if ! tar -C "$INSTALLER_SOURCE_DIR/app" -czf "$payload" .; then rm -rf "$work"; return 1; fi
  write_update_phase validating-payload "Using downloaded release bundle" "Installing the self-contained app payload included with this Dash-Go GitHub Release bundle."
  if install_release_payload "$payload" "$version" "" "$INSTALLER_SOURCE_DIR/install.sh"; then rm -rf "$work"; return 0; fi
  rm -rf "$work"; return 1
}

repair_bundle_recovery_recipe(){
  local target="${1:-latest}" version_hint
  version_hint="$target"
  [ "$version_hint" = latest ] && version_hint="the matching current ${RELEASE_TRACK} release version"
  warn "Repair recovery when the dashboard control server is missing, corrupted, or wrong for this device:"
  warn "1) On another computer, download Dash-Go_<version>_release.tar.gz and SHA256SUMS from the same Dash-Go GitHub Release (${version_hint})."
  warn "2) Copy both files to this device, then run: sha256sum -c SHA256SUMS --ignore-missing"
  warn "3) Extract the verified release, change into its extracted directory, and run: ./install.sh --repair"
  warn "The bundle installer supplies its own verified payload, so it can repair the server required by normal release resolution."
}

download_app_files(){
  local target="${1:-latest}"
  case "${UPDATE_PLAN_KIND:-}" in
    local) install_local_release_bundle "$target" && return 0 ;;
    github) download_release_payload_from_resolution "${UPDATE_PLAN_META:-}" "$target" && return 0 ;;
  esac
  if [ -n "${UPDATE_PLAN_KIND:-}" ]; then
    warn "the selected update candidate could not be staged or verified; the running dashboard was left unchanged"
    return 1
  fi
  if install_local_release_bundle "$target"; then return 0; fi
  download_release_payload "$target" && return 0
  warn "GitHub Release update failed; staged verification did not complete and the running dashboard was left unchanged"
  [ "$REPAIR_MODE" = "1" ] && repair_bundle_recovery_recipe "$target"
  return 1
}

run_doctor_mode(){
  say "Dashboard doctor"
  if [ -x "$BIN_DIR/doctor.sh" ]; then
    exec bash "$BIN_DIR/doctor.sh" "${DOCTOR_ARGS[@]}"
  fi
  warn "doctor.sh was not found at $BIN_DIR/doctor.sh"
  warn "Run ~/install.sh --update first, or complete a normal install."
  exit 1
}

# --- Offline, verified Dash-Go uninstall ---------------------------------
# Removal intentionally runs from the local installer before any network access. It removes only Dash-Go-owned wiring and state; it never
# attempts to undo shared packages, generic desktop tuning, or security
# hardening whose prior state is not known.
remove_state_prepare(){
  REMOVE_STATE_DIR="${XDG_STATE_HOME:-$HOME/.local/state}/dash-go"
  REMOVE_JOURNAL="$REMOVE_STATE_DIR/remove-state.json"
  REMOVE_SENTINEL="$REMOVE_STATE_DIR/remove-requested"
  umask 077
  mkdir -p "$REMOVE_STATE_DIR" || return 1
  chmod 700 "$REMOVE_STATE_DIR" 2>/dev/null || true
  return 0
}

remove_note(){
  local phase="${1:-unknown}" detail="${2:-}" errors="${3:-}" tmp
  tmp="${REMOVE_JOURNAL}.tmp"
  umask 077
  mkdir -p "$(dirname "$REMOVE_JOURNAL")" || return 1
  {
    printf '{
'
    printf '  "detail": "%s",
' "$(json_escape_shell "$detail")"
    printf '  "errors": "%s",
' "$(json_escape_shell "$errors")"
    printf '  "phase": "%s",
' "$(json_escape_shell "$phase")"
    printf '  "updatedAt": %s
' "$(date +%s)"
    printf '}
'
  } > "$tmp" || return 1
  chmod 600 "$tmp" 2>/dev/null || true
  mv -f "$tmp" "$REMOVE_JOURNAL"
}

remove_step(){
  local label="$1"; shift
  if "$@"; then
    ok "$label"
    return 0
  fi
  warn "$label failed"
  REMOVE_ERRORS+=("$label")
  remove_note partial "$label failed" "${REMOVE_ERRORS[*]}" || true
  return 1
}

remove_require_sudo(){
  command -v sudo >/dev/null 2>&1 || { warn "sudo is required to remove Dash-Go system wiring"; return 1; }
  sudo -v >/dev/null 2>&1 || { warn "sudo authentication was not granted; no Dash-Go files were removed"; return 1; }
}

remove_print_plan(){
  say "Uninstall Dash-Go"
  echo "This is an offline Dash-Go uninstall. It removes Dash-Go-owned service, session,"
  echo "autologin drop-in, cron, private dashboard credentials, and application files."
  echo "It does not remove shared packages, generic desktop changes, watchdog boot settings,"
  echo "or security hardening whose original state was not recorded."
  echo
  echo "Planned project paths:"
  printf '  - %s\n' "$DASH" "$HOME/.dashboard-*.env" "$HOME/.dashboard-default-calendars" "$HOME/.dashboard-disabled-calendars"
  echo "  - dashboard-server.service, Dash-Go X sessions/autologin/logind/sudoers files"
  echo "  - only recognized Dash-Go cron lines (including seasonal and nightly browser restart)"
  echo "  - no generic Surf or WebKit cache deletion"
}

remove_dashboard_cron_jobs(){
  local before middle after
  command -v crontab >/dev/null 2>&1 || return 0
  before="$(mktemp)" || return 1
  middle="$(mktemp)" || { rm -f "$before"; return 1; }
  after="$(mktemp)" || { rm -f "$before" "$middle"; return 1; }
  crontab -l 2>/dev/null > "$before" || true
  if [ -r "$BIN_DIR/dashboard-common.sh" ]; then
    # shellcheck disable=SC1090
    . "$BIN_DIR/dashboard-common.sh"
    dashboard_cron_owned_filter "$before" > "$middle" || { rm -f "$before" "$middle" "$after"; return 1; }
  else
    awk -v dash="$DASH" '
      index($0, dash "/bin/update-holidays.sh") ||
      index($0, dash "/bin/update-iss-passes.sh") ||
      index($0, dash "/bin/gen-default-calendars.sh") ||
      index($0, dash "/bin/gen-sky-calendars.sh") ||
      index($0, dash "/bin/gen-calendars.sh") ||
      index($0, dash "/bin/dashboard-housekeeping.sh") ||
      index($0, dash "/bin/dashboard-health-guard.sh") ||
      index($0, dash "/bin/dashboard-control-server --") ||
      index($0, "# dash-go-doctor") ||
      index($0, "dashboard-nightly-browser-restart") { next }
      { print }
    ' "$before" > "$middle" || { rm -f "$before" "$middle" "$after"; return 1; }
  fi
  awk -v seasonal="$BIN_DIR/seasonal-themes.sh" '
    index($0, seasonal " apply") || index($0, "seasonal-themes.sh apply") { next }
    { print }
  ' "$middle" > "$after" || { rm -f "$before" "$middle" "$after"; return 1; }
  if ! cmp -s "$before" "$after"; then
    crontab "$after" || { rm -f "$before" "$middle" "$after"; return 1; }
  fi
  rm -f "$before" "$middle" "$after"
  return 0
}

remove_make_archive(){
  local stamp archive tmpdir stage list f target
  stamp="$(date +%Y%m%d-%H%M%S)"
  archive="$HOME/dashboard-preserved/dash-go-preserve-${stamp}.tar.gz"
  tmpdir="$(mktemp -d "${TMPDIR:-/tmp}/dash-go-remove.XXXXXX")" || return 1
  stage="$tmpdir/stage"
  list="$tmpdir/archive.list"
  umask 077
  mkdir -p "$HOME/dashboard-preserved" "$stage/dashboard" "$stage/home" "$stage/meta" || { rm -rf "$tmpdir"; return 1; }
  chmod 700 "$HOME/dashboard-preserved" 2>/dev/null || true
  [ -d "$CONFIG_DIR" ] && cp -a "$CONFIG_DIR" "$stage/dashboard/config" || true
  if [ -L "$CAL_DIR" ]; then
    target="$(readlink "$CAL_DIR" 2>/dev/null || true)"
    {
      printf '{
'
      printf '  "kind": "external-symlink",
'
      printf '  "link": "%s",
' "$(json_escape_shell "$CAL_DIR")"
      printf '  "note": "External calendar target was intentionally left untouched. Recreate its symlink after reinstall if desired.",
'
      printf '  "target": "%s",
' "$(json_escape_shell "$target")"
      printf '  "targetPreserved": false
}
'
    } > "$stage/meta/calendar-source.json" || { rm -rf "$tmpdir"; return 1; }
  elif [ -d "$CAL_DIR" ]; then
    cp -a "$CAL_DIR" "$stage/dashboard/calendars" || { rm -rf "$tmpdir"; return 1; }
  fi
  for f in VERSION manifest.json; do
    [ -f "$DASH/$f" ] && cp -p "$DASH/$f" "$stage/dashboard/$f" || true
  done
  for f in .dashboard-update.env .dashboard-update-profile.json .dashboard-control.env .dashboard-weather.env .dashboard-message.env .dashboard-radar.env .dashboard-default-calendars .dashboard-disabled-calendars; do
    [ -f "$HOME/$f" ] && cp -p "$HOME/$f" "$stage/home/$f" || true
  done
  chmod 600 "$stage/home"/.dashboard-*.env "$stage/home"/.dashboard-update-profile.json 2>/dev/null || true
  {
    version="$(head -n 1 "$DASH/VERSION" 2>/dev/null | tr -d '
' || true)"
    printf '{
'
    printf '  "contents": ["settings", "calendar sources or external-calendar metadata", "private Dash-Go environment files"],
'
    printf '  "createdAt": "%s",
' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    printf '  "sourceVersion": "%s",
' "$(json_escape_shell "${version:-unknown}")"
    printf '  "type": "dash-go-preserved-uninstall"
}
'
  } > "$stage/meta/uninstall-manifest.json" || { rm -rf "$tmpdir"; return 1; }
  tar -C "$stage" -czf "$archive.tmp" . || { rm -rf "$tmpdir" "$archive.tmp"; return 1; }
  tar -tzf "$archive.tmp" > "$list" || { rm -rf "$tmpdir" "$archive.tmp"; return 1; }
  grep -qx './meta/uninstall-manifest.json' "$list" || { rm -rf "$tmpdir" "$archive.tmp"; return 1; }
  mv -f "$archive.tmp" "$archive" || { rm -rf "$tmpdir"; return 1; }
  chmod 600 "$archive" || { rm -rf "$tmpdir"; return 1; }
  sha256sum "$archive" > "$archive.sha256" || { rm -rf "$tmpdir"; return 1; }
  chmod 600 "$archive.sha256" 2>/dev/null || true
  rm -rf "$tmpdir"
  printf '%s\n' "$archive"
}

remove_request_kiosk_shutdown(){
  local pid args i
  : > "$REMOVE_SENTINEL" || return 1
  chmod 600 "$REMOVE_SENTINEL" 2>/dev/null || true
  remove_note quiesce "uninstall requested; stopping Dash-Go kiosk" || true
  pid=""
  [ -r "$CACHE_DIR/kiosk.lock/pid" ] && pid="$(tr -cd '0-9' < "$CACHE_DIR/kiosk.lock/pid" 2>/dev/null || true)"
  [ -n "$pid" ] || return 0
  kill -0 "$pid" 2>/dev/null || return 0
  args="$(ps -p "$pid" -o args= 2>/dev/null || true)"
  case "$args" in *"$DASH/kiosk.sh"*) ;; *) warn "refused to signal non-Dash-Go kiosk pid $pid"; return 1;; esac
  kill -TERM "$pid" 2>/dev/null || return 1
  for i in 1 2 3 4 5 6 7 8; do
    kill -0 "$pid" 2>/dev/null || return 0
    sleep 1
  done
  return 1
}

remove_stop_dashboard_service(){
  command -v systemctl >/dev/null 2>&1 || return 0
  $SUDO systemctl stop dashboard-server.service >/dev/null 2>&1 || true
  $SUDO systemctl disable dashboard-server.service >/dev/null 2>&1 || true
  $SUDO systemctl is-active --quiet dashboard-server.service 2>/dev/null && return 1
  return 0
}

remove_filter_legacy_autostart(){
  local f tmp
  for f in "$HOME"/.config/lxsession/*/autostart "$HOME/.config/openbox/autostart" "$HOME/.xsession" "$HOME/.xinitrc"; do
    [ -f "$f" ] || continue
    tmp="$(mktemp)" || return 1
    grep -vE 'dashboard/kiosk\.sh|/dashboard/kiosk\.sh' "$f" > "$tmp" || true
    if ! cmp -s "$f" "$tmp"; then
      mv "$tmp" "$f" || { rm -f "$tmp"; return 1; }
    else
      rm -f "$tmp"
    fi
  done
}

remove_owned_locker_overrides(){
  local app f tmp
  for app in light-locker xscreensaver xautolock xss-lock gnome-screensaver mate-screensaver cinnamon-screensaver lxlock; do
    f="$HOME/.config/autostart/$app.desktop"
    [ -f "$f" ] || continue
    tmp="$(mktemp)" || return 1
    printf '[Desktop Entry]\nType=Application\nName=%s\nHidden=true\nX-GNOME-Autostart-enabled=false\n' "$app" > "$tmp"
    if cmp -s "$f" "$tmp"; then rm -f "$f" || { rm -f "$tmp"; return 1; }; fi
    rm -f "$tmp"
  done
}

remove_owned_user_wiring(){
  rm -f "$HOME/.config/autostart/dashboard-kiosk.desktop" || return 1
  remove_filter_legacy_autostart || return 1
  remove_owned_locker_overrides || return 1
  return 0
}

remove_owned_system_wiring(){
  local f
  for f in \
    /etc/systemd/system/dashboard-server.service \
    /etc/systemd/system/dash-go-update.service \
    /usr/share/xsessions/dashboard-openbox.desktop \
    /usr/share/xsessions/dashboard-lite.desktop \
    /etc/lightdm/lightdm.conf.d/90-dash-go-autologin.conf \
    /etc/systemd/logind.conf.d/10-dashboard-kiosk.conf \
    /etc/systemd/system.conf.d/10-dashboard-watchdog.conf \
    /etc/sudoers.d/010-dashboard-reboot \
    /etc/sudoers.d/011-dashboard-system-update \
    /etc/sudoers.d/012-dashboard-app-update; do
    [ -e "$f" ] || continue
    $SUDO rm -f "$f" || return 1
  done
  $SUDO systemctl daemon-reload >/dev/null 2>&1 || return 1
  return 0
}

remove_purge_user_data(){
  local f
  [ "$REMOVE_PURGE_REQUESTED" = "1" ] || return 0
  [ -d "$DASH" ] && rm -rf "$DASH" || true
  [ ! -e "$DASH" ] || return 1
  for f in .dashboard-update.env .dashboard-update-profile.json .dashboard-control.env .dashboard-weather.env .dashboard-message.env .dashboard-radar.env .dashboard-default-calendars .dashboard-disabled-calendars; do
    rm -f "$HOME/$f" || return 1
  done
  if [ "$REMOVE_KEEP_INSTALLER" != "1" ] && [ -f "$INSTALLER" ]; then
    rm -f "$INSTALLER" || return 1
  fi
  return 0
}

remove_verify(){
  local failed=0 cron
  command -v systemctl >/dev/null 2>&1 && $SUDO systemctl is-active --quiet dashboard-server.service 2>/dev/null && { warn "dashboard server is still active"; failed=1; }
  [ -e /etc/systemd/system/dashboard-server.service ] && { warn "Dash-Go service unit remains"; failed=1; }
  [ -e /usr/share/xsessions/dashboard-openbox.desktop ] && { warn "Dash-Go X session remains"; failed=1; }
  [ -e /etc/lightdm/lightdm.conf.d/90-dash-go-autologin.conf ] && { warn "Dash-Go autologin drop-in remains"; failed=1; }
  cron="$(crontab -l 2>/dev/null || true)"
  printf '%s\n' "$cron" | grep -Eq "${DASH}/bin/|dashboard-nightly-browser-restart|seasonal-themes\.sh apply" && { warn "Dash-Go cron entries remain"; failed=1; }
  [ "$REMOVE_PURGE_REQUESTED" != "1" ] || [ ! -e "$DASH" ] || { warn "dashboard directory remains"; failed=1; }
  return "$failed"
}

remove_confirmation_matches(){
  local expected="$1" supplied="$2" normalized_expected normalized_supplied
  normalized_expected="$(printf '%s' "$expected" | tr '[:lower:]' '[:upper:]' | sed 's/^[[:space:]]*//; s/[[:space:]]*$//; s/[[:space:]][[:space:]]*/ /g')"
  normalized_supplied="$(printf '%s' "$supplied" | tr '[:lower:]' '[:upper:]' | sed 's/^[[:space:]]*//; s/[[:space:]]*$//; s/[[:space:]][[:space:]]*/ /g')"
  [ "$normalized_supplied" = "$normalized_expected" ]
}

run_remove_install(){
  local preserve answer archive=""
  REMOVE_ERRORS=()
  remove_print_plan
  if [ "$REMOVE_DRY_RUN" = "1" ]; then
    ok "dry run only; no files, services, cron entries, or network resources were changed"
    return 0
  fi
  remove_state_prepare || { warn "cannot create the Dash-Go uninstall state directory"; return 1; }
  read -rp "Type UNINSTALL DASH-GO to continue: " answer
  if ! remove_confirmation_matches "UNINSTALL DASH-GO" "$answer"; then
    warn "Uninstall was not started. Type exactly: UNINSTALL DASH-GO"
    return 1
  fi
  if [ "$REMOVE_PRESERVE_REQUESTED" = "-1" ]; then
    read -rp "Create a verified private recovery archive before removal? [Y/n] " preserve
    case "$preserve" in n|N) REMOVE_PRESERVE_REQUESTED=0;; *) REMOVE_PRESERVE_REQUESTED=1;; esac
  fi
  if [ "$REMOVE_PURGE_REQUESTED" != "1" ]; then
    read -rp "Remove Dash-Go application data and private credentials after wiring is removed? [y/N] " answer
    case "$answer" in y|Y) REMOVE_PURGE_REQUESTED=1;; *) REMOVE_PURGE_REQUESTED=0;; esac
  fi
  if [ "$REMOVE_PRESERVE_REQUESTED" != "1" ] && [ "$REMOVE_PURGE_REQUESTED" = "1" ]; then
    read -rp "No recovery archive was requested. Type PURGE DASH-GO to confirm destructive removal: " answer
    if ! remove_confirmation_matches "PURGE DASH-GO" "$answer"; then
      warn "Destructive uninstall was not started. Type exactly: PURGE DASH-GO"
      return 1
    fi
  fi
  remove_require_sudo || return 1
  if [ "$REMOVE_PRESERVE_REQUESTED" = "1" ]; then
    remove_note backup "creating verified recovery archive" || true
    archive="$(remove_make_archive)" || { warn "recovery archive failed; no Dash-Go files were removed"; return 2; }
    ok "verified recovery archive: $archive"
  fi
  remove_note start "offline uninstall started" || true
  remove_step "requested Dash-Go kiosk shutdown" remove_request_kiosk_shutdown || {
    warn "Dash-Go kiosk did not exit safely; data was left intact. Reboot, then rerun --remove."
    return 3
  }
  remove_step "removed Dash-Go scheduled jobs" remove_dashboard_cron_jobs || true
  remove_step "stopped and disabled Dash-Go server" remove_stop_dashboard_service || true
  remove_step "removed Dash-Go system wiring" remove_owned_system_wiring || true
  remove_step "removed Dash-Go user session wiring" remove_owned_user_wiring || true
  remove_step "removed Dash-Go application data and credentials" remove_purge_user_data || true
  if [ "${#REMOVE_ERRORS[@]}" -gt 0 ] || ! remove_verify; then
    warn "uninstall is incomplete; review $REMOVE_JOURNAL and rerun --remove after resolving: ${REMOVE_ERRORS[*]:-remaining post-condition}"
    remove_note partial "uninstall incomplete" "${REMOVE_ERRORS[*]}" || true
    return 3
  fi
  rm -f "$REMOVE_SENTINEL" 2>/dev/null || true
  remove_note complete "Dash-Go uninstall completed" "" || true
  ok "Dash-Go uninstall completed${archive:+; recovery archive: $archive}"
  echo "Note: shared packages, display-manager/default-target changes, watchdog boot settings, generic appliance tuning, and sudo hardening were intentionally not reversed automatically."
  if [ "$REMOVE_REBOOT_AFTER" = "1" ]; then
    $SUDO reboot
  else
    echo "Reboot after the graphical session closes to return to the normal login/session."
  fi
  return 0
}
# Removal is intentionally offline and must use the local installer copy.
if [ "$REMOVE_MODE" = "1" ]; then run_remove_install; exit $?; fi

# The installer travels inside each immutable GitHub Release bundle; it never self-updates from a separate host.
if [ "$DOCTOR_MODE" = "1" ]; then run_doctor_mode; fi

# --- Tiered repair recovery helpers --------------------------------------
# Plain --repair intentionally stays narrow.  These helpers are called only by
# explicit --system / --packages recovery tiers after the safety backup exists.
repair_add_warning(){
  local message="$*"
  REPAIR_WARNINGS="${REPAIR_WARNINGS}${REPAIR_WARNINGS:+$'\n'}${message}"
  repair_log "WARN: $message"
  warn "$message"
}

repair_stage(){
  local label="$1"; shift
  if "$@"; then
    repair_log "OK: $label"
    return 0
  fi
  repair_add_warning "$label was not fully repaired; see $LOG_DIR/repair-install.log"
  return 1
}

repair_prepare_root_access(){
  [ "${REPAIR_SYSTEM:-0}" = 1 ] || return 0
  if ! command -v sudo >/dev/null 2>&1; then
    repair_add_warning "sudo is unavailable; user-space repair will continue but service, LightDM, and package wiring are skipped"
    return 1
  fi
  if sudo -v; then
    REPAIR_SYSTEM_ROOT=1
    repair_log "root access verified for requested system recovery"
    return 0
  fi
  repair_add_warning "sudo authorization was not granted; user-space repair will continue but root-owned wiring is skipped"
  return 1
}

repair_snapshot_system_file(){
  local src="$1" dest="$2"
  [ -f "$src" ] || return 0
  [ "${REPAIR_SYSTEM_ROOT:-0}" = 1 ] || return 0
  $SUDO cp -p "$src" "$dest" 2>/dev/null || return 1
  $SUDO chown "$USER_NAME":"$(id -gn 2>/dev/null || echo "$USER_NAME")" "$dest" 2>/dev/null || true
}

repair_snapshot_system_state(){
  local tmp="$1" system_dir
  [ "${REPAIR_SYSTEM:-0}" = 1 ] || return 0
  system_dir="$tmp/system"
  mkdir -p "$system_dir" || return 1
  crontab -l > "$system_dir/crontab.bak" 2>/dev/null || :
  if [ "${REPAIR_SYSTEM_ROOT:-0}" = 1 ]; then
    repair_snapshot_system_file /etc/systemd/system/dashboard-server.service "$system_dir/dashboard-server.service" || return 1
    repair_snapshot_system_file /etc/lightdm/lightdm.conf.d/90-dash-go-autologin.conf "$system_dir/90-dash-go-autologin.conf" || return 1
    repair_snapshot_system_file /usr/share/xsessions/dashboard-openbox.desktop "$system_dir/dashboard-openbox.desktop" || return 1
    repair_snapshot_system_file /usr/share/xsessions/dashboard-lite.desktop "$system_dir/dashboard-lite.desktop" || return 1
    repair_snapshot_system_file /etc/apt/sources.list.d/dash-go-security-maintenance.sources "$system_dir/dash-go-security-maintenance.sources" || return 1
    repair_snapshot_system_file /etc/apt/sources.list.d/dash-go-backports.sources "$system_dir/dash-go-backports.sources" || return 1
    repair_snapshot_system_file /etc/apt/preferences.d/90-dash-go-backports "$system_dir/90-dash-go-backports" || return 1
    repair_snapshot_system_file /etc/apt/apt.conf.d/20dash-go-auto-upgrades "$system_dir/20dash-go-auto-upgrades" || return 1
    repair_snapshot_system_file /etc/apt/apt.conf.d/52dash-go-unattended-upgrades "$system_dir/52dash-go-unattended-upgrades" || return 1
    repair_snapshot_system_file /etc/systemd/system/apt-daily.timer.d/90-dash-go-security-maintenance.conf "$system_dir/apt-daily.timer.conf" || return 1
    repair_snapshot_system_file /etc/systemd/system/apt-daily-upgrade.timer.d/90-dash-go-security-maintenance.conf "$system_dir/apt-daily-upgrade.timer.conf" || return 1
    repair_snapshot_system_file /etc/systemd/system/apt-daily.service.d/90-dash-go-security-maintenance.conf "$system_dir/apt-daily.service.conf" || return 1
    repair_snapshot_system_file /etc/systemd/system/apt-daily-upgrade.service.d/90-dash-go-security-maintenance.conf "$system_dir/apt-daily-upgrade.service.conf" || return 1
  fi
  cat > "$system_dir/README.txt" <<EOFREPAIRSYSTEM
Dash-Go repair system snapshot.
This directory contains only Dash-Go-managed system files and the invoking
user's crontab before an explicit --repair --system operation.
EOFREPAIRSYSTEM
}

repair_source_common(){
  if [ -r "$BIN_DIR/dashboard-common.sh" ]; then
    # shellcheck disable=SC1090
    . "$BIN_DIR/dashboard-common.sh"
    return 0
  fi
  return 1
}

# --- Managed Debian security maintenance ------------------------------------
# The policy renderer lives with the app so Doctor, installer update, and
# explicit system repair all inspect exactly the same files.  The installer is
# the only writer and never changes user-owned repository definitions.
SECURITY_MAINTENANCE_ATTEMPTED=0
SECURITY_MAINTENANCE_RESULT=""

source_security_maintenance_helper(){
  local candidate
  for candidate in "$BIN_DIR/dashboard-security-maintenance.sh" "$INSTALLER_SOURCE_DIR/app/bin/dashboard-security-maintenance.sh"; do
    [ -r "$candidate" ] || continue
    # shellcheck disable=SC1090
    . "$candidate"
    return 0
  done
  return 1
}

security_maintenance_sudo(){
  if $SUDO -n true >/dev/null 2>&1; then
    return 0
  fi
  if [ -t 0 ] && [ -t 1 ]; then
    say "Administrator permission for managed Debian security maintenance"
    echo "Dash-Go will install unattended-upgrades and write only Dash-Go-owned APT/systemd policy files."
    $SUDO -v
    return $?
  fi
  return 2
}

security_maintenance_stage_file(){
  local stage="$1" kind="$2"
  dashboard_security_expected_file "$kind" > "$stage/$kind" || return 1
  chmod 600 "$stage/$kind" 2>/dev/null || true
}

security_maintenance_apply_file(){
  local stage="$1" kind="$2" target
  target="$(dashboard_security_managed_path "$kind")" || return 1
  $SUDO mkdir -p "$(dirname "$target")" || return 1
  $SUDO install -m 0644 "$stage/$kind" "$target"
}

configure_security_maintenance(){
  local reason="${1:-install}" stage backup kind target rc=0 security_source_repair=0
  local -a kinds=(backports-pin auto-upgrades unattended-policy daily-timer upgrade-timer daily-service upgrade-service)

  source_security_maintenance_helper || {
    warn "managed Debian security maintenance helper is unavailable; refresh Dash-Go app files before retrying system repair"
    SECURITY_MAINTENANCE_RESULT=unavailable
    return 1
  }
  if ! dashboard_security_maintenance_supported; then
    # A missing security suite is the only source topology Dash-Go may repair,
    # and only through the explicit system-repair path after the active base
    # suite has already been proven by Debian-signed Release metadata. Normal
    # installs and updates report the condition without rewriting APT sources.
    if [ "$reason" = repair ] && dashboard_security_security_repair_eligible; then
      security_source_repair=1
      warn "a verified Debian base exists but ${DASHBOARD_SECURITY_CODENAME}-security is missing; applying the requested reversible security-source repair"
    else
      ok "managed Debian security maintenance not applied: $(dashboard_security_maintenance_support_reason)"
      SECURITY_MAINTENANCE_RESULT=unsupported
      return 0
    fi
  fi
  if dashboard_security_later_origin_policy_present; then
    warn "a later unattended-upgrades origin policy exists; Dash-Go will not overwrite it. Review /etc/apt/apt.conf.d before enabling security-only maintenance."
    SECURITY_MAINTENANCE_RESULT=manual-review
    return 1
  fi
  if dashboard_security_managed_files_current && dashboard_security_package_installed && dashboard_security_timers_ready; then
    ok "managed Debian security maintenance is healthy (security-only; no automatic reboot)"
    SECURITY_MAINTENANCE_RESULT=healthy
    return 0
  fi

  if security_maintenance_sudo; then
    :
  else
    rc=$?
    if [ "$rc" = 2 ]; then
      warn "Dash-Go update continued without changing system APT policy; run ~/install.sh --repair --system from a terminal to finish managed security maintenance."
      SECURITY_MAINTENANCE_RESULT=deferred
      return 2
    fi
    warn "administrator permission was not granted; managed Debian security maintenance was not changed"
    SECURITY_MAINTENANCE_RESULT=permission-denied
    return 1
  fi

  stage="$(mktemp -d "${TMPDIR:-/tmp}/dash-go-security-maintenance.XXXXXX")" || return 1
  backup="$stage/backup"
  mkdir -p "$backup" || { rm -rf "$stage"; return 1; }

  # Never duplicate a user/distribution source. A canonical security source
  # is written only for the explicit, verified-base repair path or to restore
  # Dash-Go's own existing file. An authenticated administrator-owned source
  # remains untouched.
  target="$(dashboard_security_managed_path security-source)"
  if [ "$security_source_repair" = 1 ] || [ -e "$target" ]; then
    kinds+=(security-source)
  fi
  target="$(dashboard_security_managed_path backports-source)"
  if [ -e "$target" ] || ! dashboard_security_backports_source_present; then
    kinds+=(backports-source)
  fi

  for kind in "${kinds[@]}"; do
    security_maintenance_stage_file "$stage" "$kind" || { rm -rf "$stage"; return 1; }
    target="$(dashboard_security_managed_path "$kind")" || { rm -rf "$stage"; return 1; }
    if $SUDO test -e "$target"; then
      printf 'present\n' > "$backup/$kind.state"
      $SUDO cat "$target" > "$backup/$kind" || { rm -rf "$stage"; return 1; }
    else
      : > "$backup/$kind.state"
    fi
  done

  for kind in "${kinds[@]}"; do
    if ! security_maintenance_apply_file "$stage" "$kind"; then
      rc=1
      break
    fi
  done
  if [ "$rc" = 0 ]; then
    if ! apt_managed update || ! apt_managed install -y unattended-upgrades; then
      rc=1
    elif ! $SUDO systemctl daemon-reload || ! $SUDO systemctl enable --now apt-daily.timer apt-daily-upgrade.timer; then
      rc=1
    fi
  fi
  if [ "$rc" = 0 ] && dashboard_security_maintenance_supported && dashboard_security_managed_files_current && dashboard_security_package_installed && dashboard_security_timers_ready; then
    rm -rf "$stage"
    if [ "$security_source_repair" = 1 ]; then
      ok "managed Debian security maintenance enabled after verified security-source repair (security-only; no automatic reboot or backports installs)"
    else
      ok "managed Debian security maintenance enabled (security-only; no automatic reboot or backports installs)"
    fi
    SECURITY_MAINTENANCE_RESULT=enabled
    return 0
  fi

  warn "managed Debian security maintenance could not be verified after APT metadata validation; restoring Dash-Go-owned policy files"
  for kind in "${kinds[@]}"; do
    target="$(dashboard_security_managed_path "$kind")" || continue
    if [ -s "$backup/$kind.state" ]; then
      $SUDO install -m 0644 "$backup/$kind" "$target" 2>/dev/null || true
    else
      $SUDO rm -f "$target" 2>/dev/null || true
    fi
  done
  $SUDO systemctl daemon-reload >/dev/null 2>&1 || true
  rm -rf "$stage"
  SECURITY_MAINTENANCE_RESULT=failed
  return 1
}

ensure_security_maintenance_once(){
  local reason="${1:-install}" rc
  if [ "$SECURITY_MAINTENANCE_ATTEMPTED" = 1 ]; then
    [ "$SECURITY_MAINTENANCE_RESULT" = healthy ] || [ "$SECURITY_MAINTENANCE_RESULT" = enabled ] || [ "$SECURITY_MAINTENANCE_RESULT" = unsupported ]
    return
  fi
  SECURITY_MAINTENANCE_ATTEMPTED=1
  configure_security_maintenance "$reason"
  rc=$?
  return "$rc"
}

repair_scope_defaults(){
  REPAIR_SCOPE_SERVICE=1
  REPAIR_SCOPE_AUTOLOGIN=1
  REPAIR_SCOPE_AUTOSTART=1
  REPAIR_SCOPE_GUARD=1
  REPAIR_SCOPE_FONTS=1
  REPAIR_SCOPE_SECURITY=1
  REPAIR_SCOPE_PACKAGES="${REPAIR_PACKAGES:-0}"
}

repair_scope_from_doctor(){
  local report
  [ "${REPAIR_FROM_DOCTOR:-0}" = 1 ] || { repair_scope_defaults; return 0; }
  REPAIR_SCOPE_SERVICE=0; REPAIR_SCOPE_AUTOLOGIN=0; REPAIR_SCOPE_AUTOSTART=0
  REPAIR_SCOPE_GUARD=0; REPAIR_SCOPE_FONTS=0; REPAIR_SCOPE_SECURITY=0; REPAIR_SCOPE_PACKAGES="${REPAIR_PACKAGES:-0}"
  if [ ! -x "$BIN_DIR/doctor.sh" ]; then
    repair_add_warning "Doctor scope was requested but doctor.sh is unavailable; using the full requested system-recovery scope"
    repair_scope_defaults
    return 0
  fi
  report="$(bash "$BIN_DIR/doctor.sh" --full --no-prompt 2>&1 || true)"
  printf '%s\n' "$report" > "$CACHE_DIR/repair-doctor-preflight.txt" 2>/dev/null || true
  # Doctor prints successful checks too. Scope only from WARN/FAIL lines so
  # --from-doctor does not turn a healthy report into a whole-system rewrite.
  local findings
  findings="$(printf '%s\n' "$report" | grep -E '^(WARN|FAIL) ' || true)"
  if printf '%s\n' "$findings" | grep -Eiq 'dashboard-server\.service|Go API|web server'; then REPAIR_SCOPE_SERVICE=1; fi
  if printf '%s\n' "$findings" | grep -Eiq 'LightDM|autologin|X session|graphical session'; then REPAIR_SCOPE_AUTOLOGIN=1; REPAIR_SCOPE_GUARD=1; fi
  if printf '%s\n' "$findings" | grep -Eiq 'scheduled Dash-Go jobs|cron service|scheduler|kiosk launcher|autostart'; then REPAIR_SCOPE_AUTOSTART=1; fi
  if printf '%s\n' "$findings" | grep -Eiq 'Debian security maintenance|unattended-upgrades|APT policy'; then REPAIR_SCOPE_SECURITY=1; fi
  if printf '%s\n' "$findings" | grep -Eiq 'required graphical kiosk commands|required maintenance tools|surf .*missing|openbox .*missing'; then
    if [ "${REPAIR_PACKAGES:-0}" = 1 ]; then REPAIR_SCOPE_PACKAGES=1
    else repair_add_warning "Doctor found missing runtime dependencies; rerun with --repair --system --packages to install OS packages"
    fi
  fi
  if [ "$REPAIR_SCOPE_SERVICE$REPAIR_SCOPE_AUTOLOGIN$REPAIR_SCOPE_AUTOSTART$REPAIR_SCOPE_GUARD$REPAIR_SCOPE_FONTS$REPAIR_SCOPE_SECURITY$REPAIR_SCOPE_PACKAGES" = 0000000 ]; then
    repair_log "Doctor scope found no recoverable system-wiring issue; retained app-file repair only"
  else
    repair_log "Doctor-scoped recovery: service=$REPAIR_SCOPE_SERVICE autologin=$REPAIR_SCOPE_AUTOLOGIN autostart=$REPAIR_SCOPE_AUTOSTART guard=$REPAIR_SCOPE_GUARD security=$REPAIR_SCOPE_SECURITY packages=$REPAIR_SCOPE_PACKAGES"
  fi
}

provision_dashboard_service(){
  local svc=/etc/systemd/system/dashboard-server.service
  # Bootstrap only pre-manifest legacy installations that lack this file.
  # A present selector is always release-owned and is never overwritten.
  ensure_missing_go_selector_wrapper || true
  [ -x "$BIN_DIR/dashboard-control-server" ] || { warn "dashboard-control-server is missing from $BIN_DIR"; return 1; }
  command -v systemctl >/dev/null 2>&1 || { warn "systemctl is unavailable; cannot provision dashboard-server.service"; return 1; }
  $SUDO mkdir -p /etc/systemd/system || return 1
  if [ -f "$svc" ]; then $SUDO cp -p "$svc" "$svc.bak" 2>/dev/null || true; fi
  $SUDO tee "$svc" >/dev/null <<UNIT
[Unit]
Description=Dash-Go local web server
After=network.target
StartLimitIntervalSec=120
StartLimitBurst=5

[Service]
Type=simple
User=$USER_NAME
WorkingDirectory=$DASH
Environment=GOMEMLIMIT=96MiB
ExecStart=$DASH/bin/dashboard-control-server
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
UNIT
  ensure_dashboard_update_service || warn "dedicated Dashboard Control updater was not provisioned; run repair --system after confirming sudo access"
  $SUDO systemctl daemon-reload || return 1
  $SUDO systemctl enable dashboard-server.service || return 1
  $SUDO systemctl restart dashboard-server.service || return 1
  remove_obsolete_shutdown_cleanup_service || true
  if dashboard_server_confirm_live; then
    SERVICE_CONFIRMED_OK=1
    ok "dashboard-server provisioned, enabled, and confirmed live"
    return 0
  fi
  dashboard_server_failure_hint
  return 1
}

install_runtime_packages(){
  local pkg candidate missing="" path
  say "Installing required runtime packages"
  echo "Detected platform: $PLATFORM_LABEL"
  case "${OS_SUPPORT_LEVEL:-unsupported}" in
    recommended) echo "OS policy: Trixie is the recommended Dash-Go base." ;;
    supported) echo "OS policy: Bookworm is supported compatibility mode." ;;
    conservative) echo "OS policy: newer Debian-family release detected; Dash-Go will keep display and firmware configuration unchanged." ;;
    *) warn "This operating system is not supported for fresh kiosk provisioning. Upgrade to Bookworm or Trixie, then run ~/install.sh again."; return 1 ;;
  esac

  # Base packages the dashboard/kiosk needs everywhere. x11-xserver-utils
  # provides xset, used for screen blanking/wake; cron provides crontab.
  BASE_PKGS="curl cron surf wmctrl unclutter-xfixes scrot x11-xserver-utils xterm xbindkeys"
  if [ "$IS_PI" = "1" ]; then
    PKGS="$BASE_PKGS"
    echo "Raspberry Pi mode: installing the lightweight browser/X11 helper set."
    if [ ! -f /etc/lightdm/lightdm.conf ] || ! ls /usr/share/xsessions/*.desktop >/dev/null 2>&1; then
      if bootstrap_profile_prefers_openbox_session; then
        PKGS="$PKGS xorg lightdm lightdm-gtk-greeter openbox dbus-x11"
        echo "No complete graphical login/session stack detected; adding minimal Xorg + LightDM + Openbox for lite/balanced profile."
      else
        PKGS="$PKGS xorg lightdm lightdm-gtk-greeter lxde-core openbox dbus-x11"
        echo "No complete graphical login/session stack detected; adding Xorg + LightDM + LXDE for enhanced profile."
      fi
    fi
  elif [ "$IS_DEBIAN" = "1" ] && [ "$IS_X86" = "1" ]; then
    if bootstrap_profile_prefers_openbox_session; then
      PKGS="$BASE_PKGS xorg lightdm lightdm-gtk-greeter openbox dbus-x11"
      echo "Debian x86 mode: installing minimal X11 + LightDM + Openbox for lite/balanced profile."
    else
      PKGS="$BASE_PKGS xorg lightdm lightdm-gtk-greeter lxde-core openbox dbus-x11"
      echo "Debian x86 mode: installing the full X11 + LightDM + LXDE kiosk stack for enhanced profile."
    fi
    echo "This lets a minimal Debian Bookworm/Trixie install boot directly into the dashboard."
  else
    warn "Unsupported platform for kiosk provisioning. Upgrade to a supported Debian-family image, then run ~/install.sh again."
    return 1
  fi

  # Refresh package metadata once, then prove each required package has a real
  # candidate before changing display-manager, autologin, or kiosk settings.
  if ! apt_managed update; then
    warn "Could not refresh APT package metadata. No kiosk settings were changed. Check Wi-Fi and APT sources, then run ~/install.sh again."
    return 1
  fi
  for pkg in $PKGS; do
    candidate="$(apt-cache policy "$pkg" 2>/dev/null | awk '/Candidate:/ {print $2; exit}')"
    case "$candidate" in ''|none|"(none)") missing="$missing $pkg";; esac
  done
  if [ -n "$missing" ]; then
    warn "Required kiosk package unavailable:${missing}. No kiosk settings were changed. Check your Debian software sources or upgrade the device, then run ~/install.sh again."
    return 1
  fi
  if echo " $PKGS " | grep -q " lightdm "; then
    preseed_lightdm_default
  fi
  if ! apt_managed install -y $PKGS; then
    warn "Required kiosk packages could not be installed. No autologin or kiosk settings were changed. Fix the package error, then run ~/install.sh again."
    return 1
  fi
  for path in surf wmctrl xset curl; do
    if ! command -v "$path" >/dev/null 2>&1; then
      warn "Required kiosk command is still missing: $path. No autologin or kiosk settings were changed. Repair packages, then run ~/install.sh again."
      return 1
    fi
  done
  mark_reboot_recommended
  ok "required runtime packages installed and verified"
  if echo " $PKGS " | grep -q " lightdm "; then
    ensure_lightdm_default || { warn "LightDM could not be selected after packages installed. Run ~/install.sh and choose Dashboard service after checking the warning."; return 1; }
  fi

  # Raspberry Pi OS sometimes ships an AppArmor profile for /usr/bin/surf that
  # blocks EGL/glvnd graphics vendor files. It is an optional recovery detail;
  # failure never invalidates the package verification above.
  if [ -e /etc/apparmor.d/usr.bin.surf ]; then
    apt_managed install -y apparmor-utils >/dev/null 2>&1 || true
    if $SUDO aa-complain /usr/bin/surf >/dev/null 2>&1; then
      ok "surf AppArmor profile set to complain mode (was blocking rendering)"
    else
      $SUDO mkdir -p /etc/apparmor.d/disable
      $SUDO ln -sf /etc/apparmor.d/usr.bin.surf /etc/apparmor.d/disable/ 2>/dev/null || true
      $SUDO apparmor_parser -R /etc/apparmor.d/usr.bin.surf 2>/dev/null \
        && ok "surf AppArmor profile disabled" \
        || warn "could not adjust surf AppArmor profile — if Surf crashes, run: sudo aa-complain /usr/bin/surf"
    fi
  else
    ok "no restrictive surf AppArmor profile present (nothing to adjust)"
  fi

  KIOSK_SESSION="$(detect_xsession)"
  if profile_prefers_openbox_session && [ "$KIOSK_SESSION" != "dashboard-openbox" ] && [ "$KIOSK_SESSION" != "dashboard-lite" ]; then
    ok "current graphical session: $KIOSK_SESSION (dashboard-openbox will be installed if graphical autologin is configured)"
  else
    ok "preferred graphical session: $KIOSK_SESSION"
  fi
}

provision_dashboard_autologin(){
  local mode="${1:-interactive}" kiosk_session ans_openbox_session
  say "Graphical autologin (LightDM) for user: $USER_NAME"
  kiosk_session="$(detect_xsession)"
  if profile_prefers_openbox_session; then
    if [ "$mode" = interactive ]; then
      read -rp "Use minimal dashboard-openbox session for this profile? [Y/n] " ans_openbox_session
    else
      ans_openbox_session=y
    fi
    if [ "$ans_openbox_session" != n ] && [ "$ans_openbox_session" != N ]; then
      write_dashboard_openbox_xsession || return 1
      kiosk_session=dashboard-openbox
    fi
  fi
  ensure_lightdm_default || true
  if command -v lightdm >/dev/null 2>&1 || [ -x /usr/sbin/lightdm ]; then
    write_dashboard_lightdm_autologin "$USER_NAME" "$kiosk_session" || return 1
  else
    warn "LightDM is not installed; graphical autologin was not configured"
    return 1
  fi
  install_kiosk_no_logout_guard || true
  mark_reboot_recommended
  ok "dashboard graphical autologin recovery complete ($kiosk_session)"
}

provision_dashboard_autostart_cron(){
  local mode="${1:-interactive}" kiosk_session lightdm_session auto_dir auto nightly_mode=preserve nightly_answer
  say "Autostart + cron"
  kiosk_session="$(detect_xsession)"
  lightdm_session="$(lightdm_autologin_session)"
  mkdir -p "$HOME/.config/openbox" "$HOME/.config/autostart"
  cleanup_kiosk_autostarts || true
  if [ "$lightdm_session" = dashboard-openbox ] || [ "$lightdm_session" = dashboard-lite ]; then
    ok "$lightdm_session session starts kiosk.sh directly"
  else
    auto_dir="$(lxsession_autostart_dir "$kiosk_session")"
    if [ -n "$auto_dir" ]; then
      mkdir -p "$auto_dir"
      auto="$auto_dir/autostart"
      [ -f "$auto" ] && cp -p "$auto" "$auto.bak" 2>/dev/null || true
      printf '@%s/kiosk.sh\n' "$DASH" > "$auto"
      ok "session autostart -> kiosk.sh ($auto)"
    fi
  fi
  if [ "$mode" = interactive ]; then
    read -rp "Enable nightly browser restart? [Y/n] " nightly_answer
    case "$nightly_answer" in n|N) nightly_mode=off;; *) nightly_mode=on;; esac
  fi
  if declare -F dashboard_cron_reconcile >/dev/null 2>&1; then
    dashboard_cron_reconcile "$nightly_mode" || return 1
  else
    warn "shared cron helper is missing; canonical scheduler was not changed"
    return 1
  fi
  ok "canonical Dash-Go scheduled jobs restored (event cache $(case "$(dashboard_cron_event_spec)" in '*/20 '* ) echo 'every 20 minutes';; *) echo 'every 10 minutes';; esac); nightly restart $nightly_mode)"
}

repair_system_recovery(){
  [ "${REPAIR_SYSTEM:-0}" = 1 ] || return 0
  repair_source_common || { repair_add_warning "shared dashboard recovery helpers are missing after the app refresh"; return 1; }
  repair_scope_from_doctor
  if [ "${REPAIR_SCOPE_PACKAGES:-0}" = 1 ]; then
    [ "${REPAIR_SYSTEM_ROOT:-0}" = 1 ] && repair_stage "runtime package recovery" install_runtime_packages || repair_add_warning "runtime package recovery requires sudo; rerun with sudo access"
  fi
  if [ "${REPAIR_SCOPE_SECURITY:-0}" = 1 ]; then
    [ "${REPAIR_SYSTEM_ROOT:-0}" = 1 ] && repair_stage "managed Debian security maintenance" configure_security_maintenance repair || repair_add_warning "managed Debian security maintenance requires sudo; rerun with sudo access"
  fi
  [ "${REPAIR_SCOPE_FONTS:-0}" = 1 ] && repair_stage "dashboard font check" ensure_dashboard_fonts || true
  if [ "${REPAIR_SCOPE_SERVICE:-0}" = 1 ]; then
    [ "${REPAIR_SYSTEM_ROOT:-0}" = 1 ] && repair_stage "dashboard service recovery" provision_dashboard_service || repair_add_warning "dashboard service recovery requires sudo"
  fi
  if [ "${REPAIR_SCOPE_AUTOLOGIN:-0}" = 1 ]; then
    [ "${REPAIR_SYSTEM_ROOT:-0}" = 1 ] && repair_stage "LightDM and Dash-Go session recovery" provision_dashboard_autologin repair || repair_add_warning "LightDM recovery requires sudo"
  fi
  if [ "${REPAIR_SCOPE_AUTOSTART:-0}" = 1 ]; then
    repair_stage "Dash-Go autostart and scheduler recovery" provision_dashboard_autostart_cron repair || true
    if [ "${REPAIR_SYSTEM_ROOT:-0}" = 1 ] && command -v systemctl >/dev/null 2>&1; then
      $SUDO systemctl enable --now cron.service >/dev/null 2>&1 || $SUDO systemctl enable --now crond.service >/dev/null 2>&1 || repair_add_warning "cron service could not be enabled automatically"
    fi
  fi
  if [ "${REPAIR_SCOPE_GUARD:-0}" = 1 ]; then
    [ "${REPAIR_SYSTEM_ROOT:-0}" = 1 ] && repair_stage "kiosk anti-lock guard recovery" install_kiosk_no_logout_guard || true
  fi
  # Only inspect/prune launch loops when the requested recovery scope includes
  # session wiring. A healthy --from-doctor report must remain app-file-only.
  if [ "${REPAIR_SCOPE_AUTOSTART:-0}" = 1 ] || [ "${REPAIR_SCOPE_GUARD:-0}" = 1 ]; then
    prune_duplicate_kiosk_loops || true
  fi
}

repair_print_post_doctor_summary(){
  local report="$CACHE_DIR/repair-doctor-latest.txt"
  [ -f "$report" ] || return 0
  echo
  echo "Post-repair Doctor summary:"
  awk '/^== Summary/{show=1} show{print}' "$report" | tail -8
}

# --- Repair install ------------------------------------------------------
# SSH/terminal-only repair path: keep user setup, refresh app files, and
# regenerate derived data. This is intentionally NOT exposed as a Dashboard
# Control button because it is a deeper recovery operation.
repair_log(){
  mkdir -p "$LOG_DIR"
  printf '%s %s\n' "$(date -Is 2>/dev/null || date)" "$*" >> "$LOG_DIR/repair-install.log"
}

write_repair_status(){
  local state="$1" label="$2" detail="$3" backup_path="${4:-}" target="${5:-${REPAIR_TARGET:-latest}}" rc="${6:-0}" path tmp now warnings line first=1
  path="$CACHE_DIR/repair-install-status.json"
  mkdir -p "$CACHE_DIR" || return 1
  now="$(date +%s)"; tmp="${path}.tmp"; warnings="${REPAIR_WARNINGS:-}"
  {
    printf '{
'
    printf '  "backup": "%s",
' "$(json_escape_shell "$backup_path")"
    printf '  "detail": "%s",
' "$(json_escape_shell "$detail")"
    printf '  "label": "%s",
' "$(json_escape_shell "$label")"
    printf '  "rc": %s,
' "${rc:-0}"
    printf '  "state": "%s",
' "$(json_escape_shell "$state")"
    printf '  "target": "%s",
' "$(json_escape_shell "$target")"
    printf '  "updated": %s,
' "$now"
    printf '  "warnings": ['
    while IFS= read -r line; do
      [ -n "$line" ] || continue
      [ "$first" = 1 ] || printf ', '
      printf '"%s"' "$(json_escape_shell "$line")"; first=0
    done <<EOFREPAIRWARNINGS
$warnings
EOFREPAIRWARNINGS
    printf ']
}
'
  } > "$tmp" || return 1
  chmod 600 "$tmp" 2>/dev/null || true
  mv -f "$tmp" "$path"
}

make_repair_backup(){
  mkdir -p "$REPAIR_BACKUP_DIR" "$LOG_DIR" "$DASH" || return 1
  chmod 700 "$DASHGO_STATE_DIR" "$REPAIR_BACKUP_DIR" 2>/dev/null || true
  local ts tmp backup
  ts="$(date +%Y%m%d-%H%M%S)"
  tmp="$(mktemp -d "${TMPDIR:-/tmp}/dash-go-repair-backup.XXXXXX")" || return 1
  backup="$REPAIR_BACKUP_DIR/dashboard-repair-${ts}.tar.gz"
  mkdir -p "$tmp/dashboard" "$tmp/home"

  [ -d "$CONFIG_DIR" ] && cp -a "$CONFIG_DIR" "$tmp/dashboard/config" 2>/dev/null || true
  [ -d "$CAL_DIR" ] && cp -a "$CAL_DIR" "$tmp/dashboard/calendars" 2>/dev/null || true
  for f in VERSION manifest.json; do
    [ -f "$DASH/$f" ] && cp -p "$DASH/$f" "$tmp/dashboard/$f" 2>/dev/null || true
  done
  # These home files are first-class preferences: update credentials, API
  # keys, and Control PIN settings must survive a repair or archive restore.
  for f in .dashboard-update.env .dashboard-update-profile.json .dashboard-control.env \
           .dashboard-weather.env .dashboard-message.env .dashboard-radar.env \
           .dashboard-todo.json .dashboard-default-calendars .dashboard-disabled-calendars; do
    [ -f "$HOME/$f" ] && cp -p "$HOME/$f" "$tmp/home/$f" 2>/dev/null || true
  done
  if [ -f "$HOME/.config/dash-go/apprise/routes.json" ]; then
    mkdir -p "$tmp/home/.config/dash-go/apprise"
    cp -p "$HOME/.config/dash-go/apprise/routes.json" "$tmp/home/.config/dash-go/apprise/routes.json" 2>/dev/null || true
  fi
  # An explicit --system repair snapshots every Dash-Go-owned system file it
  # may change before touching it. Plain --repair remains app/user-data only.
  repair_snapshot_system_state "$tmp" || { rm -rf "$tmp"; return 1; }
  cat > "$tmp/repair-backup.json" <<EOFREPAIRBACKUP
{
  "type": "repair-install",
  "reason": "pre-repair",
  "created": "$(date -Is 2>/dev/null || date)",
  "version": "$(cat "$DASH/VERSION" 2>/dev/null || echo unknown)",
  "target": "${REPAIR_TARGET:-latest}",
  "contents": ["config including local To Do/Grocery caches", "calendars", "VERSION", "manifest.json", "calendar preference files", "dashboard environment credentials, Microsoft To Do token, and PIN configuration", "optional Dash-Go system wiring snapshot"]
}
EOFREPAIRBACKUP
  if ( umask 077; cd "$tmp" && tar -czf "$backup" . ); then
    chmod 600 "$backup" 2>/dev/null || true
    rm -rf "$tmp"
    printf '%s\n' "$backup"
    return 0
  fi
  rm -rf "$tmp"
  return 1
}

restore_repair_user_data(){
  local backup="$1" tmp old_settings
  [ -n "$backup" ] && [ -f "$backup" ] || return 1
  tmp="$(mktemp -d "${TMPDIR:-/tmp}/dash-go-repair-restore.XXXXXX")" || return 1
  if ! tar -xzf "$backup" -C "$tmp"; then
    rm -rf "$tmp"
    return 1
  fi

  mkdir -p "$CONFIG_DIR" "$CAL_DIR" "$CACHE_DIR" "$LOG_DIR"

  # Restore user config files exactly as backed up. New app defaults belong in
  # runtime defaulting; repair must not rewrite user-owned settings state.
  for name in config.local.js compliments.json message-sources.json message-cache.json message-cache-overrides.json temp-messages.json scheduled-messages.json map-provider.json notification-preferences.json; do
    if [ -f "$tmp/dashboard/config/$name" ]; then
      cp -p "$tmp/dashboard/config/$name" "$CONFIG_DIR/$name" 2>/dev/null || true
    fi
  done

  old_settings="$tmp/dashboard/config/settings.json"
  if [ -f "$old_settings" ]; then
    # settings.json is user-owned state. Repair restores it byte-for-byte;
    # profile defaults are applied only with --reset-profile after restore.
    cp -p "$old_settings" "$SETTINGS_FILE" 2>/dev/null || return 1
  fi

  # To Do/Grocery local lists are source data, not a generated cache. Restore
  # the complete hashed-cache directory so offline household tasks survive a
  # repair exactly like calendars and chalkboard state.
  if [ -d "$tmp/dashboard/config/todo" ]; then
    rm -rf "$CONFIG_DIR/todo"
    mkdir -p "$CONFIG_DIR/todo"
    cp -a "$tmp/dashboard/config/todo/." "$CONFIG_DIR/todo/" 2>/dev/null || return 1
  fi

  # Restore calendar sources, but regenerate calendars.json from the fresh
  # helper so old generated manifests do not pin stale behavior.
  if [ -d "$tmp/dashboard/calendars" ]; then
    rm -rf "$CAL_DIR"
    mkdir -p "$CAL_DIR"
    cp -a "$tmp/dashboard/calendars/." "$CAL_DIR/" 2>/dev/null || true
    rm -f "$CAL_DIR/calendars.json" "$CAL_DIR/calendars.json.tmp" 2>/dev/null || true
  fi

  for f in .dashboard-default-calendars .dashboard-disabled-calendars; do
    if [ -f "$tmp/home/$f" ]; then
      cp -p "$tmp/home/$f" "$HOME/$f" 2>/dev/null || true
    fi
  done
  for f in .dashboard-update.env .dashboard-update-profile.json .dashboard-control.env .dashboard-weather.env .dashboard-message.env .dashboard-radar.env .dashboard-todo.json; do
    if [ -f "$tmp/home/$f" ]; then
      cp -p "$tmp/home/$f" "$HOME/$f" 2>/dev/null || true
      chmod 600 "$HOME/$f" 2>/dev/null || true
    fi
  done
  if [ -f "$tmp/home/.config/dash-go/apprise/routes.json" ]; then
    mkdir -p "$HOME/.config/dash-go/apprise"
    cp -p "$tmp/home/.config/dash-go/apprise/routes.json" "$HOME/.config/dash-go/apprise/routes.json" 2>/dev/null || true
    chmod 700 "$HOME/.config" "$HOME/.config/dash-go" "$HOME/.config/dash-go/apprise" 2>/dev/null || true
    chmod 600 "$HOME/.config/dash-go/apprise/routes.json" 2>/dev/null || true
  fi

  rm -f "$CACHE_DIR/events.cache.json" "$CACHE_DIR/.events-cache.meta.json" 2>/dev/null || true
  rm -rf "$tmp"
  return 0
}

# Best-effort preference discovery is only a fallback.  The live repair backup
# always wins when it contains usable state; older archives fill individual
# missing/corrupt preferences instead of overwriting a working current setup.
repair_json_usable(){
  [ -f "$1" ] || return 1
  installer_json_validate "$1"
}

repair_archive_has_preferences(){
  local archive="$1"
  case "$archive" in
    *.tar.gz|*.tgz)
      tar -tzf "$archive" 2>/dev/null | grep -Eq '(^|/)dashboard/config/(settings\.json|config\.local\.js|todo/)|(^|/)home/\.dashboard-(update|control|weather|message|radar)\.env|(^|/)home/\.dashboard-todo\.json'
      ;;
    *.zip)
      unzip -Z1 "$archive" 2>/dev/null | grep -Eq '(^|/)config/(settings\.json|config\.local\.js)'
      ;;
    *) return 1 ;;
  esac
}

repair_newest_valid_candidate(){
  local pattern="$1" f
  # Shell glob expansion is intentional; mtime ordering is used rather than
  # filename ordering because users may have copied archives between devices.
  for f in $(ls -1t $pattern 2>/dev/null || true); do
    [ -f "$f" ] && repair_archive_has_preferences "$f" && { printf '%s\n' "$f"; return 0; }
  done
  return 1
}

discover_best_preferences(){
  local current="$1" found
  found="$(repair_newest_valid_candidate "$REPAIR_BACKUP_DIR/dashboard-repair-*.tar.gz" || true)"
  [ -n "$found" ] && [ "$found" != "$current" ] && { printf '%s\n' "$found"; return 0; }
  # Compatibility fallback for archives made before the state-directory move.
  found="$(repair_newest_valid_candidate "$CACHE_DIR/repair-backups/dashboard-repair-*.tar.gz" || true)"
  [ -n "$found" ] && [ "$found" != "$current" ] && { printf '%s\n' "$found"; return 0; }
  found="$(repair_newest_valid_candidate "$CACHE_DIR/config-backups/dashboard-config-*.zip" || true)"
  [ -n "$found" ] && { printf '%s\n' "$found"; return 0; }
  found="$(find "$HOME" -maxdepth 3 -type f \( -iname 'dashboard-repair-*.tar.gz' -o -iname 'dashboard-*backup*.zip' \) -printf '%T@ %p\n' 2>/dev/null | sort -nr | cut -d' ' -f2- | while IFS= read -r candidate; do repair_archive_has_preferences "$candidate" && { printf '%s\n' "$candidate"; break; }; done)"
  [ -n "$found" ] && [ "$found" != "$current" ] && { printf '%s\n' "$found"; return 0; }
  return 1
}

repair_restore_missing_from_tar(){
  local archive="$1" tmp
  tmp="$(mktemp -d "${TMPDIR:-/tmp}/dash-go-repair-fallback.XXXXXX")" || return 1
  tar -xzf "$archive" -C "$tmp" || { rm -rf "$tmp"; return 1; }
  for f in settings.json compliments.json message-sources.json message-cache-overrides.json temp-messages.json scheduled-messages.json chalkboard.json map-provider.json; do
    # Older archives are only a recovery source after their JSON has been
    # parsed successfully. Never trade a missing preference for a corrupt one.
    if [ -f "$tmp/dashboard/config/$f" ] && repair_json_usable "$tmp/dashboard/config/$f" \
       && { [ ! -f "$CONFIG_DIR/$f" ] || ! repair_json_usable "$CONFIG_DIR/$f"; }; then
      cp -p "$tmp/dashboard/config/$f" "$CONFIG_DIR/$f" 2>/dev/null || true
      repair_log "fallback preference restored: config/$f"
    fi
  done
  if [ -f "$tmp/dashboard/config/config.local.js" ] && [ ! -f "$CONFIG_DIR/config.local.js" ]; then
    cp -p "$tmp/dashboard/config/config.local.js" "$CONFIG_DIR/config/config.local.js" 2>/dev/null || true
    repair_log "fallback preference restored: config/config.local.js"
  fi
  if [ -d "$tmp/dashboard/config/todo" ] && [ ! -d "$CONFIG_DIR/todo" ]; then
    mkdir -p "$CONFIG_DIR/todo"
    cp -a "$tmp/dashboard/config/todo/." "$CONFIG_DIR/todo/" 2>/dev/null || true
    repair_log "fallback local Lists restored: config/todo"
  fi
  for f in .dashboard-update.env .dashboard-update-profile.json .dashboard-control.env .dashboard-weather.env .dashboard-message.env .dashboard-radar.env .dashboard-todo.json; do
    if [ -f "$tmp/home/$f" ] && [ ! -f "$HOME/$f" ]; then
      cp -p "$tmp/home/$f" "$HOME/$f" 2>/dev/null || true
      chmod 600 "$HOME/$f" 2>/dev/null || true
      repair_log "fallback preference restored: $f"
    fi
  done
  if [ -d "$tmp/dashboard/calendars" ] && ! find "$CAL_DIR" -maxdepth 1 -type f -name '*.ics' -print -quit 2>/dev/null | grep -q .; then
    cp -a "$tmp/dashboard/calendars/." "$CAL_DIR/" 2>/dev/null || true
    rm -f "$CAL_DIR/calendars.json" "$CAL_DIR/calendars.json.tmp" 2>/dev/null || true
    repair_log "fallback calendar sources restored"
  fi
  rm -rf "$tmp"
}

repair_restore_missing_from_zip(){
  local archive="$1"
  zip_restore_config_fallback "$archive" "$CONFIG_DIR"
}

repair_restore_discovered_preferences(){
  local current_backup="$1" candidate=""
  candidate="$(discover_best_preferences "$current_backup" || true)"
  [ -n "$candidate" ] || return 0
  repair_log "using best-effort fallback preferences from $candidate"
  case "$candidate" in
    *.tar.gz|*.tgz) repair_restore_missing_from_tar "$candidate" || repair_add_warning "could not read fallback preferences archive: $candidate" ;;
    *.zip) repair_restore_missing_from_zip "$candidate" || repair_add_warning "could not read fallback config backup: $candidate" ;;
  esac
}

# --- Fresh-install restore discovery ------------------------------------
# Supports repair/preserve tarballs and Dashboard Control config-backup ZIPs.
# Candidate discovery intentionally uses name patterns so users can move the
# preservation folder and still have the installer find it during a reinstall.
RESTORE_FROM_BACKUP=0
RESTORE_ARCHIVE=""
RESTORE_SKIP_SETUP="1"

restore_candidate_kind(){
  case "$1" in
    *.tar.gz|*.tgz) echo "repair/preserve archive";;
    *.zip) echo "config backup ZIP";;
    *) [ -d "$1" ] && echo "directory" || echo "file";;
  esac
}

restore_candidate_looks_usable(){
  local candidate="$1"
  [ -e "$candidate" ] || return 1
  case "$candidate" in
    *.tar.gz|*.tgz)
      tar -tzf "$candidate" >/dev/null 2>&1
      ;;
    *.zip)
      zip_has_entries "$candidate"
      ;;
    *)
      [ -d "$candidate" ] && { [ -d "$candidate/config" ] || [ -d "$candidate/calendars" ] || [ -f "$candidate/config.local.js" ]; }
      ;;
  esac
}

list_restore_candidates(){
  {
    find "$REPAIR_BACKUP_DIR" -maxdepth 1 -type f -name 'dashboard-repair-*.tar.gz' 2>/dev/null
    find "$CACHE_DIR/repair-backups" -maxdepth 1 -type f -name 'dashboard-repair-*.tar.gz' 2>/dev/null
    find "$CACHE_DIR/config-backups" -maxdepth 1 -type f -name 'dashboard-config-*.zip' 2>/dev/null
    find "$HOME" -maxdepth 5 -type f \( \
      -iname '*dashboard*preserv*.tar.gz' -o \
      -iname '*dashboard*preserve*.tgz' -o \
      -iname '*family*dashboard*preserv*.tar.gz' -o \
      -iname 'dashboard-repair-*.tar.gz' -o \
      -iname 'dashboard-config-*.zip' \
    \) 2>/dev/null
  } | awk 'NF && !seen[$0]++' | sort -r
}

select_restore_candidate(){
  local candidates count i ans path manual
  candidates="$(list_restore_candidates)"
  count=0
  if [ -n "$candidates" ]; then
    echo
    echo "Restore candidates found:"
    while IFS= read -r path; do
      [ -n "$path" ] || continue
      count=$((count+1))
      printf '  %2d) %s  (%s)\n' "$count" "$path" "$(restore_candidate_kind "$path")"
    done <<EOFRESTORECANDS
$candidates
EOFRESTORECANDS
    echo "   m) Enter another path manually"
    echo "   s) Skip restore"
    read -rp "Choose restore source [1-$count/m/s]: " ans
    case "$ans" in
      s|S|"") return 1;;
      m|M) ;;
      *)
        if printf '%s' "$ans" | grep -qE '^[0-9]+$' && [ "$ans" -ge 1 ] && [ "$ans" -le "$count" ]; then
          i=0
          while IFS= read -r path; do
            [ -n "$path" ] || continue
            i=$((i+1))
            [ "$i" = "$ans" ] && { printf '%s\n' "$path"; return 0; }
          done <<EOFRESTOREPICK
$candidates
EOFRESTOREPICK
        fi
        warn "invalid restore selection"
        return 1
        ;;
    esac
  fi
  while :; do
    read -rp "Path to backup/preserved archive/directory [blank=skip, b=back]: " manual
    case "$manual" in
      ''|b|B) return 1;;
    esac
    manual="${manual/#\~/$HOME}"
    if restore_candidate_looks_usable "$manual"; then
      printf '%s\n' "$manual"
      return 0
    fi
    warn "That path is not a readable Dash-Go backup. Check the path, choose a .tar.gz/.tgz/.zip backup or a backup folder, then try again."
  done
}

restore_from_config_zip(){
  local zipfile="$1"
  zip_restore_config_and_calendars "$zipfile"
}

restore_from_directory(){
  local src="$1"
  if [ -d "$src/dashboard/config" ] || [ -d "$src/dashboard/calendars" ]; then
    [ -d "$src/dashboard/config" ] && mkdir -p "$CONFIG_DIR" && cp -a "$src/dashboard/config/." "$CONFIG_DIR/" 2>/dev/null || true
    [ -d "$src/dashboard/calendars" ] && rm -rf "$CAL_DIR" && mkdir -p "$CAL_DIR" && cp -aL "$src/dashboard/calendars/." "$CAL_DIR/" 2>/dev/null || true
  else
    [ -d "$src/config" ] && mkdir -p "$CONFIG_DIR" && cp -a "$src/config/." "$CONFIG_DIR/" 2>/dev/null || true
    [ -d "$src/calendars" ] && rm -rf "$CAL_DIR" && mkdir -p "$CAL_DIR" && cp -aL "$src/calendars/." "$CAL_DIR/" 2>/dev/null || true
  fi
  for f in .dashboard-default-calendars .dashboard-disabled-calendars; do
    [ -f "$src/home/$f" ] && cp -p "$src/home/$f" "$HOME/$f" 2>/dev/null || true
  done
  for f in .dashboard-update.env .dashboard-update-profile.json .dashboard-control.env .dashboard-weather.env .dashboard-message.env .dashboard-radar.env .dashboard-todo.json; do
    if [ -f "$src/home/$f" ]; then
      cp -p "$src/home/$f" "$HOME/$f" 2>/dev/null || true
      chmod 600 "$HOME/$f" 2>/dev/null || true
    fi
  done
}

restore_previous_install_data(){
  local src="$1" tmp
  [ -n "$src" ] && [ -e "$src" ] || { warn "restore source not found"; return 1; }
  mkdir -p "$CONFIG_DIR" "$CAL_DIR" "$CACHE_DIR" "$LOG_DIR"
  case "$src" in
    *.tar.gz|*.tgz)
      # Reuse repair restore for compatible tarballs. If it is not compatible,
      # fall back to extracting and copying config/calendars directly.
      if restore_repair_user_data "$src"; then
        ok "restored settings/calendars from archive"
      else
        tmp="$(mktemp -d "$DASH/.restore-source.XXXXXX")" || return 1
        tar -xzf "$src" -C "$tmp" || { rm -rf "$tmp"; return 1; }
        restore_from_directory "$tmp"
        rm -rf "$tmp"
        ok "restored settings/calendars from archive directory layout"
      fi
      ;;
    *.zip)
      restore_from_config_zip "$src" >/dev/null || return 1
      ok "restored settings/calendars from config backup ZIP"
      ;;
    *)
      [ -d "$src" ] || { warn "unsupported restore source: $src"; return 1; }
      restore_from_directory "$src"
      ok "restored settings/calendars from directory"
      ;;
  esac
  rm -f "$CACHE_DIR/events.cache.json" "$CACHE_DIR/.events-cache.meta.json" "$CAL_DIR/calendars.json" 2>/dev/null || true
  [ -x "$BIN_DIR/gen-calendars.sh" ] && "$BIN_DIR/gen-calendars.sh" >/dev/null 2>&1 || true
}

prompt_fresh_restore_if_needed(){
  local ans chosen skip
  [ ! -f "$DASH/index.html" ] || return 0
  echo
  echo "Fresh install detected."
  read -rp "Restore settings/calendars from a previous backup or preserved uninstall archive? [y/N] " ans
  [ "$ans" = "y" ] || [ "$ans" = "Y" ] || return 0
  if chosen="$(select_restore_candidate)"; then
    RESTORE_FROM_BACKUP=1
    RESTORE_ARCHIVE="$chosen"
    echo "Selected restore source: $RESTORE_ARCHIVE"
    read -rp "Use restored settings/calendars instead of re-answering setup/calendar questions? [Y/n] " skip
    case "$skip" in n|N) RESTORE_SKIP_SETUP="0";; *) RESTORE_SKIP_SETUP="1";; esac
  else
    warn "no restore source selected; continuing as a normal fresh install"
  fi
}

repair_detect_profile_defaults(){
  # Repair mode should rebuild profile/display defaults around the device that
  # is running the repair, not blindly preserve an old profile from backup.
  local guess model arch
  model="${DEVICE_MODEL:-}"
  arch="${DASH_ARCH:-$(uname -m 2>/dev/null || echo unknown)}"
  guess="$(bootstrap_classify_device_profile)"

  case "$guess" in
    lite)
      R_PROFILE="lite"; R_PROFILE_LABEL="Low-memory / Pi Zero-class"
      R_SHOWSECS="false"; R_COMPSEC="18"; R_COMPFADEMS="650"
      R_MAXEVENTS="6"; R_AGENDADAYS="10"; R_WEATHERDAYS="10"; R_WEEKSABOVE="1"; R_WEEKSBELOW="8"; R_ROWHEIGHT="205"; R_SIDEBARWIDTH="370"
      R_CALREFRESH="15"; R_WXREFRESH="45"; R_ALERTREFRESH="10"; R_PIXELSHIFT="1"; R_SHOWMAPS="false"; R_SHOWINTERACTIVEMAPS="false"; R_LAYOUTPROFILE="auto";;
    enhanced)
      R_PROFILE="enhanced"; R_PROFILE_LABEL="2.2 GB+ capable device"
      R_SHOWSECS="true"; R_COMPSEC="25"; R_COMPFADEMS="450"
      R_MAXEVENTS="10"; R_AGENDADAYS="18"; R_WEATHERDAYS="16"; R_WEEKSABOVE="3"; R_WEEKSBELOW="12"; R_ROWHEIGHT="218"; R_SIDEBARWIDTH="400"
      R_CALREFRESH="7"; R_WXREFRESH="30"; R_ALERTREFRESH="5"; R_PIXELSHIFT="2"; R_SHOWMAPS="true"; R_SHOWINTERACTIVEMAPS="true"; R_LAYOUTPROFILE="auto";;
    *)
      R_PROFILE="balanced"; R_PROFILE_LABEL="2 GB class / balanced"
      R_SHOWSECS="true"; R_COMPSEC="15"; R_COMPFADEMS="600"
      R_MAXEVENTS="8"; R_AGENDADAYS="14"; R_WEATHERDAYS="14"; R_WEEKSABOVE="2"; R_WEEKSBELOW="10"; R_ROWHEIGHT="210"; R_SIDEBARWIDTH="380"
      R_CALREFRESH="10"; R_WXREFRESH="30"; R_ALERTREFRESH="5"; R_PIXELSHIFT="2"; R_SHOWMAPS="true"; R_SHOWINTERACTIVEMAPS="false"; R_LAYOUTPROFILE="auto";;
  esac
}

apply_repair_profile_defaults(){
  [ "${RESET_PROFILE:-0}" = "1" ] || return 0
  repair_detect_profile_defaults
  if [ ! -x "$BIN_DIR/dashboard-control-server" ]; then
    repair_log "profile reset requested but Go server helper is missing"
    return 1
  fi
  repair_log "explicit profile reset requested: $R_PROFILE ($R_PROFILE_LABEL)"
  "$BIN_DIR/dashboard-control-server" --apply-profile-preset "$R_PROFILE" >> "$LOG_DIR/repair-install.log" 2>&1
}

update_lock_is_held(){
  [ -e "$CACHE_DIR/update.lock" ] || return 1
  command -v flock >/dev/null 2>&1 || return 1
  (
    exec 8>"$CACHE_DIR/update.lock" || exit 0
    flock -n 8 && exit 1
    exit 0
  )
}

repair_reconcile_abandoned_update_state(){
  local pause="$CACHE_DIR/kiosk-paused" evidence
  [ -e "$pause" ] || return 0
  if update_lock_is_held; then
    repair_add_warning "Kiosk remains paused because an active Dash-Go update lock is still held; do not clear it until that update finishes"
    repair_log "left kiosk pause in place because update lock is active"
    return 1
  fi
  mkdir -p "$CACHE_DIR/update-recovery" 2>/dev/null || true
  evidence="$CACHE_DIR/update-recovery/kiosk-paused-abandoned-$(date +%Y%m%d-%H%M%S)"
  if mv "$pause" "$evidence" 2>/dev/null; then
    repair_log "moved abandoned kiosk pause marker to $evidence"
    if [ -r "$CACHE_DIR/post-update-verify.json" ]; then
      cp -p "$CACHE_DIR/post-update-verify.json" "$CACHE_DIR/update-recovery/post-update-verify-before-repair-$(date +%Y%m%d-%H%M%S).json" 2>/dev/null || true
      if update_cli_supports '--update-job'; then
        update_cli --update-job --file "$CACHE_DIR/post-update-verify.json" --state recovered --label "Repair reconciled interrupted update" --detail "Repair found no active update lock, preserved prior verifier evidence, and released the abandoned kiosk pause." --rolled-back false --rollback-attempted true --rollback-succeeded false --code 1 >/dev/null 2>&1 || true
      fi
    fi
    ok "cleared an abandoned kiosk update pause before graphical recovery"
    return 0
  fi
  repair_add_warning "Could not clear an abandoned Dash-Go kiosk pause marker; browser recovery cannot be verified"
  repair_log "FAILED: could not clear abandoned kiosk pause marker"
  return 1
}

repair_graphical_kiosk_expected(){
  command -v systemctl >/dev/null 2>&1 || return 1
  systemctl is-active --quiet lightdm.service 2>/dev/null
}

run_repair_install(){
  say "Repair install"
  echo "This creates a safety backup, refreshes the dashboard app files,"
  echo "restores your settings and calendars, then rebuilds generated data."
  if [ "${REPAIR_SYSTEM:-0}" = 1 ]; then
    if [ "${REPAIR_PACKAGES:-0}" = 1 ]; then
      echo "Requested recovery tier: system wiring and runtime packages."
    else
      echo "Requested recovery tier: system wiring."
    fi
    echo "Dash-Go-managed service, autologin, session, cron, and guard files are backed up first."
  fi
  echo "This is intended for SSH/terminal repair work, not normal touch-panel use."
  echo

  mkdir -p "$DASH" "$CACHE_DIR" "$LOG_DIR"
  cd "$DASH" || { warn "cannot enter $DASH"; exit 1; }
  : > "$LOG_DIR/repair-install.log"
  repair_log "repair install requested; target=${REPAIR_TARGET:-latest}; system=${REPAIR_SYSTEM:-0}; packages=${REPAIR_PACKAGES:-0}; from_doctor=${REPAIR_FROM_DOCTOR:-0}"
  [ "${REPAIR_SYSTEM:-0}" = 1 ] && repair_prepare_root_access || true
  write_repair_status running "Repair install running" "Creating safety backup." "" "${REPAIR_TARGET:-latest}" 0

  local backup detail
  if ! backup="$(make_repair_backup)"; then
    warn "could not create repair safety backup — repair cancelled"
    repair_log "FAILED: could not create backup"
    write_repair_status failed "Repair install failed" "Could not create the safety backup. Nothing was changed." "" "${REPAIR_TARGET:-latest}" 1
    exit 1
  fi
  ok "safety backup created: $backup"
  repair_log "backup created: $backup"
  write_repair_status running "Repair install running" "Installing fresh dashboard app files." "$backup" "${REPAIR_TARGET:-latest}" 0

  if ! download_app_files "${REPAIR_TARGET:-latest}"; then
    warn "fresh app install failed — user data backup is still available"
    repair_log "FAILED: download/install app payload failed"
    write_repair_status failed "Repair install failed" "Fresh app files could not be installed. Backup is available." "$backup" "${REPAIR_TARGET:-latest}" 2
    exit 1
  fi
  repair_log "fresh app payload installed"

  write_repair_status running "Repair install running" "Restoring settings, calendars, credentials, and PIN configuration from the safety backup." "$backup" "${REPAIR_TARGET:-latest}" 0
  if ! restore_repair_user_data "$backup"; then
    warn "could not restore user config/calendar data from repair backup"
    repair_log "FAILED: restore user data failed"
    write_repair_status failed "Repair install failed" "Fresh files installed, but user data restore failed. Backup is available." "$backup" "${REPAIR_TARGET:-latest}" 3
    exit 1
  fi
  repair_restore_discovered_preferences "$backup" || true
  repair_log "user data, credentials, and PIN configuration restored"

  if [ "${RESET_PROFILE:-0}" = "1" ]; then
    write_repair_status running "Repair install running" "Applying the explicitly requested performance profile defaults." "$backup" "${REPAIR_TARGET:-latest}" 0
    apply_repair_profile_defaults || true
  else
    repair_log "preserved personalized profile settings (no --reset-profile requested)"
  fi

  rm -f "$BIN_DIR/gen-events-cache.py" "$BIN_DIR/update-message-feeds.py" 2>/dev/null || true
  rm -rf "$DASH/server" 2>/dev/null || true
  chmod +x "$BIN_DIR"/*.sh "$BIN_DIR"/dashboard-control-server* "$DASH/kiosk.sh" 2>/dev/null || true
  [ -x "$BIN_DIR/gen-calendars.sh" ] && "$BIN_DIR/gen-calendars.sh" >> "$LOG_DIR/repair-install.log" 2>&1 || true
  [ -x "$BIN_DIR/dashboard-control-server" ] && "$BIN_DIR/dashboard-lowprio.sh" "$BIN_DIR/dashboard-control-server" --gen-events-cache >> "$LOG_DIR/repair-install.log" 2>&1 || true
  repair_log "regenerated calendars/events after fresh app install"

  if [ "${REPAIR_SYSTEM:-0}" = 1 ]; then
    repair_reconcile_abandoned_update_state || true
    write_repair_status running "Repair install running" "Recovering the requested Dash-Go system wiring." "$backup" "${REPAIR_TARGET:-latest}" 0
    repair_system_recovery || true
  fi

  # Keep Tier 1 behavior: refresh an existing unit, otherwise ask a remaining
  # user-owned kiosk/session to relaunch naturally. Tier 2 may already have
  # created/enabled the unit, so this is harmless convergence rather than a
  # second provisioning path.
  if [ -f /etc/systemd/system/dashboard-server.service ] && command -v systemctl >/dev/null 2>&1 && command -v sudo >/dev/null 2>&1; then
    ensure_go_dashboard_service_unit >> "$LOG_DIR/repair-install.log" 2>&1 || true
    sudo systemctl restart dashboard-server.service >> "$LOG_DIR/repair-install.log" 2>&1 || true
  else
    pkill -f dashboard-control-server 2>/dev/null || true
  fi
  REPAIR_KIOSK_VERIFIED=1
  if repair_graphical_kiosk_expected; then
    if ! restart_kiosk; then
      REPAIR_KIOSK_VERIFIED=0
      repair_add_warning "The graphical session is active but Dash-Go Surf was not observed after repair; inspect $LOG_DIR/kiosk.log and restart LightDM if needed"
      repair_log "FAILED: graphical recovery did not observe Dash-Go Surf"
    fi
  else
    restart_kiosk || true
  fi

  [ -x "$BIN_DIR/doctor.sh" ] && bash "$BIN_DIR/doctor.sh" --full --no-prompt > "$CACHE_DIR/repair-doctor-latest.txt" 2>&1 || true
  repair_log "ran post-repair Doctor when available"
  repair_print_post_doctor_summary

  detail="Fresh files installed; settings, calendars, credentials, and PIN configuration preserved; generated data rebuilt."
  if [ "${REPAIR_SYSTEM:-0}" = 1 ]; then detail="$detail Requested system recovery completed."; fi
  if [ -n "${REPAIR_WARNINGS:-}" ]; then detail="$detail Some requested stages need review; see repair log."; fi
  if [ "${REPAIR_KIOSK_VERIFIED:-1}" != 1 ]; then
    write_repair_status failed "Repair needs graphical follow-up" "$detail The dashboard server is available, but the active graphical session did not launch Surf." "$backup" "${REPAIR_TARGET:-latest}" 4
    warn "repair finished with graphical follow-up required; Dash-Go is not yet verified usable on the display"
    echo "Backup kept at: $backup"
    echo "Log written to: $LOG_DIR/repair-install.log"
    return 1
  fi
  write_repair_status success "Repair install complete" "$detail" "$backup" "${REPAIR_TARGET:-latest}" 0
  ok "repair install complete"
  echo "Backup kept at: $backup"
  echo "Log written to: $LOG_DIR/repair-install.log"
  [ -n "${REPAIR_WARNINGS:-}" ] && { echo "System recovery warnings:"; printf '%b\n' "$REPAIR_WARNINGS"; }
}
# Runtime rollback is deliberately synchronous for the first local API
# readiness check.  The later post-update verifier still owns deeper health
# evidence and long-tail rollback, but a browser must never be recycled toward
# a port that has not answered for the newly installed version.
rollback_update_runtime_failure(){
  local reason="$1" restored_version=""
  DASH_UPDATE_ROLLBACK_ATTEMPTED=1; DASH_UPDATE_ROLLBACK_SUCCEEDED=0; DASH_UPDATE_ROLLED_BACK=0
  export DASH_UPDATE_ROLLBACK_ATTEMPTED DASH_UPDATE_ROLLBACK_SUCCEEDED DASH_UPDATE_ROLLED_BACK
  warn "$reason"
  dashboard_server_failure_hint
  if [ -z "${UPDATE_ROLLBACK_STAGE:-}" ] || ! rollback_update_payload "$UPDATE_ROLLBACK_STAGE"; then
    resume_kiosk_after_runtime_transition
    DASH_UPDATE_HEALTH_CHECKED=1 write_update_status failed "Runtime readiness failed" "The new release did not become ready and its retained rollback snapshot could not be restored." 1
    DASH_UPDATE_HEALTH_CHECKED=1 write_update_job failed "Runtime readiness failed" "The new release did not become ready and its retained rollback snapshot could not be restored." 1 || true
    return 1
  fi

  restored_version="$(cat "$DASH/VERSION" 2>/dev/null | head -1 || true)"
  if [ -z "$restored_version" ] || ! restart_dashboard_server_for_update "$restored_version"; then
    resume_kiosk_after_runtime_transition
    DASH_UPDATE_HEALTH_CHECKED=1 write_update_status failed "Rollback runtime failed" "The previous release files were restored but the local server did not become ready; inspect dashboard-server.service." 1
    DASH_UPDATE_HEALTH_CHECKED=1 write_update_job failed "Rollback runtime failed" "The previous release files were restored but the local server did not become ready; inspect dashboard-server.service." 1 || true
    return 1
  fi
  resume_kiosk_after_runtime_transition
  restart_kiosk || true
  DASH_UPDATE_ROLLED_BACK=1; DASH_UPDATE_ROLLBACK_SUCCEEDED=1
  export DASH_UPDATE_ROLLED_BACK DASH_UPDATE_ROLLBACK_SUCCEEDED
  DASH_UPDATE_HEALTH_CHECKED=1 write_update_status rolledback "Rolled back" "The new release did not pass immediate local runtime readiness; restored ${restored_version}." 1
  DASH_UPDATE_HEALTH_CHECKED=1 write_update_job rolledback "Rolled back" "The new release did not pass immediate local runtime readiness; restored ${restored_version}." 1 || true
  preserve_failed_update_transaction "$UPDATE_ROLLBACK_STAGE" "runtime readiness rollback verified" || true
  return 0
}

if [ "$UPDATE_ROLLBACK_ONLY" = "1" ]; then
  start_update_logging
  trap 'update_exit_cleanup "$?"' EXIT
  DASH_UPDATE_ROLLBACK_ATTEMPTED=1; DASH_UPDATE_ROLLBACK_SUCCEEDED=0; DASH_UPDATE_ROLLED_BACK=0
  export DASH_UPDATE_ROLLBACK_ATTEMPTED DASH_UPDATE_ROLLBACK_SUCCEEDED DASH_UPDATE_ROLLED_BACK
  pause_kiosk_for_runtime_transition || true
  if rollback_update_payload "$UPDATE_ROLLBACK_STAGE"; then
    restored_version="$(cat "$DASH/VERSION" 2>/dev/null | head -1 || true)"
    if [ -n "$restored_version" ] && restart_dashboard_server_for_update "$restored_version"; then
      resume_kiosk_after_runtime_transition
      restart_kiosk || true
      DASH_UPDATE_ROLLED_BACK=1; DASH_UPDATE_ROLLBACK_SUCCEEDED=1
      export DASH_UPDATE_ROLLED_BACK DASH_UPDATE_ROLLBACK_SUCCEEDED
      DASH_UPDATE_HEALTH_CHECKED=1 write_update_status rolledback "Rolled back" "The new release did not pass its bounded runtime check; restored ${restored_version}." 1
      DASH_UPDATE_HEALTH_CHECKED=1 write_update_job rolledback "Rolled back" "The new release did not pass its bounded runtime check; restored ${restored_version}." 1 || true
      preserve_failed_update_transaction "$UPDATE_ROLLBACK_STAGE" "bounded runtime rollback verified" || true
      # The rollback command itself succeeded even though the original update
      # failed; callers use this result to avoid reporting a false rollback failure.
      exit 0
    fi
  fi
  resume_kiosk_after_runtime_transition
  DASH_UPDATE_HEALTH_CHECKED=1 write_update_status failed "Rollback failed" "The update runtime check failed and the previous local server could not be restored." 1
  DASH_UPDATE_HEALTH_CHECKED=1 write_update_job failed "Rollback failed" "The update runtime check failed and the previous local server could not be restored." 1 || true
  exit 1
fi
if [ "$REPAIR_MODE" = "1" ]; then
  run_repair_install
  exit $?
fi

# --- Non-interactive update mode -----------------------------------------
# Dashboard Control starts this transaction through the dedicated
# dash-go-update.service, outside dashboard-server.service. Direct SSH updates
# take the same lock and write the same durable job/status records.
if [ "$UPDATE_MODE" = "1" ]; then
  # Resolve and compare while holding the same flock used for the destructive
  # transaction. A direct SSH update therefore cannot race a second invocation
  # into reinstalling an equal release, and a no-op never creates the normal
  # update log, payload stage, data refresh, or kiosk restart.
  if ! require_update_compatibility_tools; then
    write_update_status failed "Updater preflight failed" "The installed updater is missing a required compatibility tool; no release files were changed." 1 || true
    write_update_job failed "Updater preflight failed" "The installed updater is missing a required compatibility tool; no release files were changed." 1 || true
    exit 1
  fi
  if [ "${DASH_UPDATE_SOURCE:-ssh}" != "control" ] && update_job_reservation_is_active; then
    warn "another Dash-Go update transaction has already been reserved; wait for it to start or recover"
    exit 1
  fi
  if ! acquire_dashboard_update_lock; then
    # The current lock holder owns the durable job/status records. A rejected
    # second entrypoint must never overwrite that live transaction with failed.
    warn "another Dash-Go update transaction already holds the update lock"
    exit 1
  fi
  if [ -z "${DASH_UPDATE_JOB_ID:-}" ]; then
    if [ "${DASH_UPDATE_SOURCE:-ssh}" = "control" ]; then
      DASH_UPDATE_JOB_ID="$(read_dashboard_update_job_id)"
    fi
    # The outer branch has already established that no job ID was supplied.
    # A Dashboard Control caller may have had no durable job ID yet, so use an
    # SSH-style locally unique fallback without putting an assignment inside
    # a test expression.
    DASH_UPDATE_JOB_ID="${DASH_UPDATE_JOB_ID:-ssh-$(date +%s)-$$}"
    export DASH_UPDATE_JOB_ID
  fi
  mkdir -p "$DASH" "$CACHE_DIR" || { warn "cannot prepare the Dash-Go update workspace"; exit 1; }
  if ! plan_normal_update_candidate "$UPDATE_TARGET"; then
    write_update_status failed "Release check failed" "The selected release could not be resolved and no live release files were changed." 1 || true
    if [ "${DASH_UPDATE_SOURCE:-ssh}" = "control" ]; then write_update_job failed "Release check failed" "The selected release could not be resolved and no live release files were changed." 1 || true; fi
    warn "could not resolve the selected Dash-Go release before update"
    update_plan_cleanup || true
    exit 1
  fi
  if [ "${UPDATE_PLAN_NOOP:-0}" = "1" ]; then
    record_no_update_needed "$UPDATE_PREVIOUS_VERSION" "$UPDATE_PLAN_VERSION" "$UPDATE_PLAN_COMPARISON"
    update_plan_cleanup || true
    exit 0
  fi
  # A verified recovery candidate is allowed only when VERSION is missing or
  # malformed. Normal valid installs reach this point only for a strict upgrade.
  UPDATE_TARGET="$UPDATE_PLAN_VERSION"
  export UPDATE_TARGET
  start_update_logging
  trap 'update_exit_cleanup "$?"' EXIT
  write_update_status running "Starting update" "A newer selected release was confirmed; preparing the verified update transaction." 0 || true
  write_update_job running "Starting update" "A newer selected release was confirmed; preparing the verified update transaction." 0 || true

  say "Unattended update from canonical GitHub Releases ($RELEASE_TRACK track, $UPDATE_TARGET)"
  cd "$DASH" || { write_update_status failed "Failed" "Cannot enter $DASH." 1; write_update_job failed "Failed" "Cannot enter $DASH." 1; warn "cannot enter $DASH"; exit 1; }
  if [ "${DASHGO_INSTALLER_SMOKE:-0}" != 1 ]; then DASH_UPDATE_RUNTIME_ROLLBACK=1; export DASH_UPDATE_RUNTIME_ROLLBACK; fi
  if ! download_app_files "$UPDATE_TARGET"; then
    write_update_status failed "Release validation failed" "The selected release could not be downloaded or verified; no live release files were changed." 1 || true
    write_update_job failed "Release validation failed" "The selected release could not be downloaded or verified; no live release files were changed." 1 || true
    warn "release-tarball update failed; staged release validation did not complete"
    exit 1
  fi
  if [ -f "$DASH/VERSION" ]; then DASH_INSTALLED_VERSION="$(cat "$DASH/VERSION" 2>/dev/null | head -1)"; export DASH_INSTALLED_VERSION; fi
  if [ "${DASHGO_INSTALLER_SMOKE:-0}" != "1" ]; then
    # A Dashboard Control runner has no safe interactive sudo path.  Its app
    # update stays successful while the system policy reports a clear deferred
    # repair; direct terminal updates can authenticate and repair immediately.
    ensure_security_maintenance_once update || true
  fi
  if [ "${DASHGO_INSTALLER_SMOKE:-0}" = "1" ]; then
    write_update_status success "Success" "Installer smoke validated ${DASH_INSTALLED_VERSION:-latest} from the $RELEASE_TRACK track." 0 || true
    write_update_job success "Success" "Installer smoke validated ${DASH_INSTALLED_VERSION:-latest} from the $RELEASE_TRACK track." 0 || true
    ok "installer smoke payload validation complete: ${DASH_INSTALLED_VERSION:-latest} ($RELEASE_TRACK track)"
    exit 0
  fi

  ensure_dashboard_fonts || true
  write_update_status committing "Refreshing dashboard data" "Verified release installed; refreshing generated calendars and cached dashboard data before local runtime readiness is checked." 0 || true
  write_update_job committing "Refreshing dashboard data" "Verified release installed; refreshing generated calendars and cached dashboard data before local runtime readiness is checked." 0 || true

  ensure_go_dashboard_service_unit || true
  [ -x "$BIN_DIR/update-holidays.sh" ] && "$BIN_DIR/update-holidays.sh" >/dev/null 2>&1 || true
  [ -x "$BIN_DIR/update-iss-passes.sh" ] && "$BIN_DIR/update-iss-passes.sh" >/dev/null 2>&1 || true
  [ -x "$BIN_DIR/gen-default-calendars.sh" ] && "$BIN_DIR/gen-default-calendars.sh" >/dev/null 2>&1 || true
  [ -x "$BIN_DIR/gen-sky-calendars.sh" ] && "$BIN_DIR/gen-sky-calendars.sh" >/dev/null 2>&1 || true
  [ -x "$BIN_DIR/dashboard-control-server" ] && "$BIN_DIR/dashboard-control-server" --update-message-feeds >/dev/null 2>&1 || true
  [ -x "$BIN_DIR/gen-calendars.sh" ] && "$BIN_DIR/gen-calendars.sh" >/dev/null 2>&1
  [ -x "$BIN_DIR/dashboard-control-server" ] && "$BIN_DIR/dashboard-lowprio.sh" "$BIN_DIR/dashboard-control-server" --gen-events-cache >/dev/null 2>&1
  prune_duplicate_kiosk_loops || true

  if ! pause_kiosk_for_runtime_transition; then
    write_update_status failed "Runtime transition failed" "Could not pause kiosk relaunch before restarting the local server. The update requires manual review." 1 || true
    write_update_job failed "Runtime transition failed" "Could not pause kiosk relaunch before restarting the local server. The update requires manual review." 1 || true
    exit 1
  fi
  write_update_status checking-runtime "Checking local runtime" "Restarting dashboard-server.service and waiting for ${DASH_INSTALLED_VERSION:-the installed version} on 127.0.0.1:8090." 0 || true
  write_update_job checking-runtime "Checking local runtime" "Restarting dashboard-server.service and waiting for ${DASH_INSTALLED_VERSION:-the installed version} on 127.0.0.1:8090." 0 || true
  if ! restart_dashboard_server_for_update "${DASH_INSTALLED_VERSION:-}"; then
    rollback_update_runtime_failure "The newly installed local server did not become ready before the bounded deadline." || true
    exit 1
  fi
  resume_kiosk_after_runtime_transition
  write_update_status recycling-browser "Recycling dashboard display" "The updated local server is ready; restarting the kiosk browser." 0 || true
  write_update_job recycling-browser "Recycling dashboard display" "The updated local server is ready; restarting the kiosk browser." 0 || true
  if ! restart_kiosk; then
    warn "release runtime is ready, but the Dash-Go browser relaunch was not verified"
    write_update_status post-verify-pending "Update committed" "Installed ${DASH_INSTALLED_VERSION:-latest} and verified the local server. The kiosk browser relaunch was not observed; the kiosk loop will retry while final health verification completes." 0 || true
    write_update_job post-verify-pending "Update committed" "Installed ${DASH_INSTALLED_VERSION:-latest} and verified the local server. The kiosk browser relaunch was not observed; the kiosk loop will retry while final health verification completes." 0 || true
  else
    write_update_status post-verify-pending "Update committed" "Installed ${DASH_INSTALLED_VERSION:-latest}; local runtime readiness passed and final health verification is running." 0 || true
    write_update_job post-verify-pending "Update committed" "Installed ${DASH_INSTALLED_VERSION:-latest}; local runtime readiness passed and final health verification is running." 0 || true
  fi
  if ! run_post_update_verifier; then
    # The bounded verifier has already recorded a truthful rolled-back/failed
    # state and retained evidence. Do not overwrite it here.
    exit 1
  fi
  DASH_UPDATE_HEALTH_CHECKED=1 write_update_status success "Update complete" "Installed ${DASH_INSTALLED_VERSION:-latest}; runtime and bounded post-update health checks passed." 0 || true
  DASH_UPDATE_HEALTH_CHECKED=1 write_update_job success "Update complete" "Installed ${DASH_INSTALLED_VERSION:-latest}; runtime and bounded post-update health checks passed." 0 || true
  ok "update complete — local runtime and post-update health checks passed"
  exit 0
fi

# --- Workflow-specific preflight -----------------------------------------
# The device/network/clock checks happen before the first interactive question.
# This smaller phase belongs after the user selects an action because restore
# discovery is meaningful only for a fresh installation.
run_interactive_preflight(){
  # On a fresh install, optionally restore a previous dashboard config/calendar
  # backup before the setup questions run. App files still install first; the
  # restore is applied immediately after file download so the fresh server code is
  # present for settings merge/rebuild behavior.
  prompt_fresh_restore_if_needed
}

# Notifications are SSH/terminal-owned because Apprise destination URLs are
# secrets. Dashboard Control exposes only per-person delivery preferences.
configure_apprise_notifications(){
  local server choice selected line person_id person_name configured route count enabled
  server="$(release_server_for_host "$DASH" 2>/dev/null || true)"
  if [ -z "$server" ] || [ ! -x "$server" ]; then
    warn "Apprise-Go setup needs an installed Dash-Go release for this device. Run Update the app first."
    return 1
  fi
  while :; do
    echo
    echo "== Notifications (Apprise-Go)"
    "$server" --apprise-status 2>/dev/null || warn "Could not read Apprise-Go status."
    echo
    echo "  1) Status"
    echo "  2) Configure personal route"
    echo "  3) Send a test notification"
    echo "  4) Enable or disable all external delivery"
    echo "  5) Remove all Apprise routes and secrets"
    echo "  6) Remove routes for permanently removed People"
    echo "  7) Back"
    read -rp "  Choose [1-7]: " choice
    case "${choice:-7}" in
      1) ;;
      2)
        mapfile -t APPRISE_PEOPLE < <("$server" --apprise-people 2>/dev/null)
        if [ "${#APPRISE_PEOPLE[@]}" -eq 0 ]; then
          warn "Add active People in Dashboard Control first."
          continue
        fi
        echo "  Choose a household person:"
        for i in "${!APPRISE_PEOPLE[@]}"; do
          IFS=$'\t' read -r person_id person_name configured <<< "${APPRISE_PEOPLE[$i]}"
          printf '   %d) %s (%s)\n' "$((i+1))" "$person_name" "$( [ "$configured" = true ] && printf configured || printf not-configured )"
        done
        read -rp "  Person [1-${#APPRISE_PEOPLE[@]}]: " selected
        case "$selected" in ''|*[!0-9]*) warn "Choose a listed person."; continue;; esac
        [ "$selected" -ge 1 ] 2>/dev/null && [ "$selected" -le "${#APPRISE_PEOPLE[@]}" ] 2>/dev/null || { warn "Choose a listed person."; continue; }
        IFS=$'\t' read -r person_id person_name configured <<< "${APPRISE_PEOPLE[$((selected-1))]}"
        echo "  Enter one or more Apprise URLs for $person_name."
        echo "  Supported here: apprise(s) through a full Apprise API server, gotify(s), ifttt, json(s), ntfy(s), form(s), and xml(s). Discord, email, Telegram, Slack, and Pushover require the Apprise API route."
        echo "  They are hidden while typed and never printed in normal logs."
        echo "  Leave a blank line after the final route (maximum 8)."
        APPRISE_ROUTES=()
        while [ "${#APPRISE_ROUTES[@]}" -lt 8 ]; do
          read -rsp "  Route $(( ${#APPRISE_ROUTES[@]} + 1 )): " route; echo
          [ -n "$route" ] || break
          APPRISE_ROUTES+=("$route")
        done
        if [ "${#APPRISE_ROUTES[@]}" -eq 0 ]; then warn "No routes entered; nothing changed."; continue; fi
        if printf '%s\n' "${APPRISE_ROUTES[@]}" | "$server" --apprise-route-set --person "$person_id"; then
          ok "Private Apprise route saved for $person_name."
        else
          warn "Route was not saved. Check its Apprise URL syntax and try again."
        fi
        unset APPRISE_ROUTES route
        ;;
      3)
        mapfile -t APPRISE_PEOPLE < <("$server" --apprise-people 2>/dev/null | awk -F '\t' '$3 == "true" {print}')
        if [ "${#APPRISE_PEOPLE[@]}" -eq 0 ]; then warn "No active person has a configured route."; continue; fi
        echo "  Choose a configured person:"
        for i in "${!APPRISE_PEOPLE[@]}"; do IFS=$'\t' read -r person_id person_name configured <<< "${APPRISE_PEOPLE[$i]}"; printf '   %d) %s\n' "$((i+1))" "$person_name"; done
        read -rp "  Person [1-${#APPRISE_PEOPLE[@]}]: " selected
        case "$selected" in ''|*[!0-9]*) warn "Choose a listed person."; continue;; esac
        [ "$selected" -ge 1 ] 2>/dev/null && [ "$selected" -le "${#APPRISE_PEOPLE[@]}" ] 2>/dev/null || { warn "Choose a listed person."; continue; }
        IFS=$'\t' read -r person_id person_name configured <<< "${APPRISE_PEOPLE[$((selected-1))]}"
        "$server" --apprise-test --person "$person_id" && ok "Test requested for $person_name." || warn "Test delivery did not complete. The local message board remains unaffected."
        ;;
      4)
        read -rp "  Enable external Dash-Go delivery? [Y/n] " enabled
        if [ "$enabled" = n ] || [ "$enabled" = N ]; then "$server" --apprise-set-enabled false; else "$server" --apprise-set-enabled true; fi
        ;;
      5)
        read -rp "  Remove every private Apprise route and secret? [y/N] " enabled
        if [ "$enabled" = y ] || [ "$enabled" = Y ]; then "$server" --apprise-remove-config && ok "Private Apprise routes removed."; else echo "  Left unchanged."; fi
        ;;
      6)
        read -rp "  Remove routes for People permanently removed from Dash-Go? [y/N] " enabled
        if [ "$enabled" = y ] || [ "$enabled" = Y ]; then "$server" --apprise-remove-orphaned-routes; else echo "  Left unchanged."; fi
        ;;
      7) return 0;;
      *) warn "Choose 1 through 7.";;
    esac
  done
}

# Terminal access remains administrator-owned, but repeating the full helper
# command from memory is unnecessary. This installer choice is intentionally
# terminal/SSH-only; it never creates a touchscreen setting or alters SSH.
configure_terminal_access(){
  local server current next answer
  server="$(release_server_for_host "$DASH" 2>/dev/null || true)"
  if [ -z "$server" ] || [ ! -x "$server" ]; then
    warn "Terminal access control needs an installed Dash-Go release for this device. Run Update the app first."
    return 1
  fi
  current="$("$server" --terminal-access status 2>/dev/null || true)"
  case "$current" in
    enabled) next="disable" ;;
    disabled) next="enable" ;;
    *)
      warn "Terminal access controls require Dash-Go 1.4.3-beta.105 or newer. Choose Update the app, then try again."
      return 1
      ;;
  esac
  echo
  echo "== Terminal access"
  echo "Dashboard Control terminal access is currently ${current}."
  echo "SSH itself is not changed by this setting."
  read -rp "  Toggle Dashboard Control terminal access to ${next}? [y/N] " answer
  if [ "$answer" != "y" ] && [ "$answer" != "Y" ]; then
    echo "  Left unchanged."
    return 0
  fi
  if "$server" --terminal-access "$next"; then
    if [ "$next" = "disable" ]; then
      ok "Terminal access disabled. Re-open Dashboard Control to remove the Terminal access card."
    else
      ok "Terminal access enabled. Re-open Dashboard Control to show the Terminal access card."
    fi
    return 0
  fi
  warn "Terminal access setting was not changed."
  return 1
}

# --- Run mode -------------------------------------------------------
# Each task can be turned on/off. Presets cover common cases; "Custom"
# lets you pick. On a first install choose Full; on later runs pick a
# targeted task (e.g. just re-pull the app files, or just change settings).
# Keep all top-level identities here so displayed order, dispatch, and later
# user-facing guidance cannot drift apart during a future menu change.
OPT_FULL=1
OPT_UPDATE=2
OPT_UPDATE_RECONFIGURE=3
OPT_RECONFIGURE=4
OPT_WEATHER_DISPLAY=5
OPT_WEATHER_SOURCES=6
OPT_RADAR=7
OPT_CALENDARS=8
OPT_ICAL=9
OPT_VDIR=10
OPT_MESSAGES=11
OPT_TODO=12
OPT_THEME=13
OPT_SEASONAL=14
OPT_PIN=15
OPT_SERVICE=16
OPT_SSH=17
OPT_DOCTOR=18
OPT_TOUR=19
OPT_DEMO=20
OPT_CUSTOM=21
OPT_NOTIFICATIONS=22
OPT_TERMINAL=23
OPT_REMOVE=24
OPT_EXIT=25

DO_SYSTEM=0 DO_PKGS=0 DO_FILES=0 DO_FONTS=0 DO_CUSTOM=0 DO_WEATHER=0 DO_RADAR=0 DO_WEATHER_DISPLAY=0 DO_MESSAGE_SOURCES=0 DO_APP_SETUP=0 DO_ICAL=0 DO_VDIR=0 DO_SERVICE=0 DO_AUTOLOGIN=0 DO_AUTOSTART=0 DO_CALENDARS=0 DO_PIN=0 DO_SSH=0 DOC_AT_END=0 DO_DEMO=0
EXPRESS_MODE=0
MODE=""
# Detect whether this looks like a first install and default accordingly.
if [ -f "$DASH/index.html" ]; then DEFMODE="$OPT_UPDATE"; DEFHINT="Dash-Go is already installed"; else DEFMODE="$OPT_FULL"; DEFHINT="first install detected"; fi

# Keep the first decision human-sized. The detailed menu remains available,
# but a first-time owner should not have to understand every safe default.
normalize_menu_choice(){
  local value
  value="$(trim_input "${1:-}")"
  value="${value%)}"
  printf '%s' "$value"
}
while :; do
  installed_version="$(installed_dashboard_version)"
  if [ -f "$DASH/index.html" ]; then
    echo
    echo "Start here — Dash-Go ${installed_version:-is} already installed."
    echo "  1) Update Dash-Go (recommended)"
    echo "     Download the newest version; keeps all your settings and calendars."
  else
    echo
    echo "Start here — first install detected."
    echo "  1) Express setup (recommended)"
    echo "     Use safe defaults. You will only choose your location and, if wanted, a dashboard PIN."
  fi
  echo "  2) Show the full menu (updates, settings, tools)"
  echo "  3) Exit without changing anything"
  read -rp "  Choose [1/2/3, Enter=1]: " START_LANE
  START_LANE="$(normalize_menu_choice "${START_LANE:-1}")"
  case "$START_LANE" in
    1)
      EXPRESS_MODE=1
      if [ -f "$DASH/index.html" ]; then
        MODE="$OPT_UPDATE"
        ok "Update selected — Dash-Go will download the newest selected release and confirm it starts."
      else
        MODE="$OPT_FULL"
        echo "Express setup uses safe display, weather, and maintenance defaults. Everything can be changed later in Dashboard Control."
        read -rp "  Set a Dashboard Control PIN now? [y/N] " EXPRESS_PIN
        case "$(normalize_menu_choice "$EXPRESS_PIN")" in y|Y) DO_PIN=1;; esac
      fi
      break
      ;;
    2) break ;;
    3|q|Q|quit|exit)
      ok "installer closed without making changes"
      exit 0
      ;;
    *) warn "Choose 1 for the recommended path, 2 for the full menu, or 3 to exit." ;;
  esac
done

installer_selected_tasks(){
  [ "$DO_SYSTEM" = 1 ] && printf '%s\n' "System update / optional platform trim + security maintenance"
  [ "$DO_PKGS" = 1 ] && printf '%s\n' "Install runtime packages"
  [ "$DO_FILES" = 1 ] && printf '%s\n' "Download/refresh the dashboard app files"
  [ "$DO_FONTS" = 1 ] && printf '%s\n' "Download fonts"
  [ "$DO_CUSTOM" = 1 ] && printf '%s\n' "Setup questions"
  [ "$DO_WEATHER_DISPLAY" = 1 ] && printf '%s\n' "Weather display behavior"
  [ "$DO_WEATHER" = 1 ] && printf '%s\n' "Weather source configuration"
  [ "$DO_RADAR" = 1 ] && printf '%s\n' "Weather radar provider"
  [ "$DO_MESSAGE_SOURCES" = 1 ] && printf '%s\n' "Message sources"
  [ "$DO_APP_SETUP" = 1 ] && printf '%s\n' "Microsoft To Do setup"
  [ "$DO_CALENDARS" = 1 ] && printf '%s\n' "Built-in/default calendars"
  [ "$DO_ICAL" = 1 ] && printf '%s\n' "Add a read-only calendar link"
  [ "$DO_VDIR" = 1 ] && printf '%s\n' "Connect a personal calendar (view-only or two-way)"
  [ "$DO_PIN" = 1 ] && printf '%s\n' "Control-panel PIN"
  [ "$DO_SERVICE" = 1 ] && printf '%s\n' "Dashboard service"
  [ "$DO_AUTOLOGIN" = 1 ] && printf '%s\n' "Boot straight into the dashboard"
  [ "$DO_AUTOSTART" = 1 ] && printf '%s\n' "Autostart + scheduled tasks"
  [ "$DO_SSH" = 1 ] && printf '%s\n' "Enable SSH"
  [ "$DOC_AT_END" = 1 ] && printf '%s\n' "Run a health check"
}

while [ -z "$MODE" ]; do
  echo
  echo "What do you want to do?  (${DEFHINT}; Enter = $( [ "$DEFMODE" = "$OPT_UPDATE" ] && printf 'Update the app' || printf 'Full install'))"
  echo
  echo "  INSTALL & UPDATE"
  printf '  %2s) %-28s %s\n' "$OPT_FULL" "Full install" "everything, start to finish (first time)"
  printf '  %2s) %-28s %s\n' "$OPT_UPDATE" "Update the app" "get the newest version; keeps settings and calendars"
  printf '  %2s) %-28s %s\n' "$OPT_UPDATE_RECONFIGURE" "Update + reconfigure" "new files, then re-answer setup questions"
  echo
  echo "  SETTINGS"
  printf '  %2s) %-28s %s\n' "$OPT_RECONFIGURE" "Reconfigure all" "profile, location, display, weather, messages, theme, birthdays"
  printf '  %2s) %-28s %s\n' "$OPT_WEATHER_DISPLAY" "Weather display" "units, days shown, refresh, alert severity"
  printf '  %2s) %-28s %s\n' "$OPT_WEATHER_SOURCES" "Weather sources" "guided menu for free/keyed providers"
  printf '  %2s) %-28s %s\n' "$OPT_RADAR" "Weather radar" "choose provider + optional protected key"
  printf '  %2s) %-28s %s\n' "$OPT_CALENDARS" "Built-in calendars" "holidays, sky calendars, celebrations, pickup"
  printf '  %2s) %-28s %s\n' "$OPT_ICAL" "Read-only calendar link" "show events only; Dash-Go can never change that calendar"
  printf '  %2s) %-28s %s\n' "$OPT_VDIR" "Personal calendar sync" "secure sign-in; choose view-only or two-way sync"
  printf '  %2s) %-28s %s\n' "$OPT_MESSAGES" "Message sources" "quotes, jokes, facts, prompts, API refresh"
  printf '  %2s) %-28s %s\n' "$OPT_TODO" "Microsoft To Do" "connect a Microsoft account for shared to-do lists (guided)"
  printf '  %2s) %-28s %s\n' "$OPT_THEME" "Theme" "pick from built-in color schemes"
  printf '  %2s) %-28s %s\n' "$OPT_SEASONAL" "Seasonal themes" "holiday auto-theming on or off"
  echo
  echo "  SECURITY"
  printf '  %2s) %-28s %s\n' "$OPT_PIN" "Control PIN" "set/reset/disable passcode + unlock duration"
  echo
  echo "  SYSTEM"
  printf '  %2s) %-28s %s\n' "$OPT_SERVICE" "Dashboard service" "web server + on-screen control panel"
  printf '  %2s) %-28s %s\n' "$OPT_SSH" "Remote access (SSH)" "manage the dashboard from another computer"
  echo
  echo "  HELP & ADMIN"
  printf '  %2s) %-28s %s\n' "$OPT_DOCTOR" "Health check" "verify everything is running"
  printf '  %2s) %-28s %s\n' "$OPT_TOUR" "Feature tour" "what this dashboard can do, in plain words"
  printf '  %2s) %-28s %s\n' "$OPT_DEMO" "Demo mode" "fill the screen with sample data (clearly marked DEMO; removable)"
  printf '  %2s) %-28s %s\n' "$OPT_CUSTOM" "Custom" "choose a focused set of setup/system tasks"
  printf '  %2s) %-28s %s\n' "$OPT_NOTIFICATIONS" "Phone/email alerts" "send dashboard alerts to your phone or email (advanced)"
  printf '  %2s) %-28s %s\n' "$OPT_TERMINAL" "Terminal access" "toggle Dashboard Control Terminal card"
  echo
  echo "  REMOVE"
  printf '  %2s) %-28s %s\n' "$OPT_REMOVE" "Remove Dash-Go" "uninstall from this device — offers to save a backup first"
  printf '  %2s) %-28s %s\n' "$OPT_EXIT" "Exit installer" "close without changing anything"
  echo
  read -rp "  Choose [1-25, q=exit; Enter=$DEFMODE]: " MODE
  MODE="$(normalize_menu_choice "${MODE:-$DEFMODE}")"

case "$MODE" in
    q|Q|quit|exit) ok "installer closed without making changes"; exit 0 ;;
    [1-9]|1[0-9]|2[0-5]) : ;;
    *) warn "Choose a listed action or q to exit."; MODE="" ;;
  esac
done

menu_handoff_missing(){
  local feature="$1"
  warn "$feature isn't installed yet. Choose option 2 (Update the app) from the menu, then try again."
  echo "Returning to the installer menu without changing anything."
  exec bash "$0" "${INSTALLER_ORIGINAL_ARGS[@]}"
}

custom_questionnaire(){
  local -a task_labels=(
    "System update / optional platform trim (slow; rarely needed twice)"
    "Install runtime packages and fonts"
    "Download/refresh the dashboard app files"
    "Setup questions (profile, location, units, theme, birthdays)"
    "Weather display behavior (days, refresh, alerts, units)"
    "Weather source configuration (guided provider menu)"
    "Weather radar provider (on-demand; no background polling)"
    "Message sources (quotes, jokes, facts, prompts)"
    "Microsoft To Do / Graph setup"
    "Built-in/default calendars"
    "Add a read-only calendar link"
    "Connect a personal calendar (view-only or two-way)"
    "Control-panel PIN lock"
    "Dashboard service (web server + on-screen control panel)"
    "Boot straight into the dashboard + scheduled tasks"
    "Enable SSH for remote administration"
    "Run a health check when finished"
  )
  local -a answers=()
  local index=0 answer total=${#task_labels[@]}
  # This questionnaire is deliberately stateful: b revisits and replaces the
  # previous answer instead of leaving a stale "yes" selected behind.
  DO_SYSTEM=0 DO_PKGS=0 DO_FILES=0 DO_FONTS=0 DO_CUSTOM=0 DO_WEATHER=0 DO_RADAR=0 DO_WEATHER_DISPLAY=0 DO_MESSAGE_SOURCES=0 DO_APP_SETUP=0 DO_ICAL=0 DO_VDIR=0 DO_SERVICE=0 DO_AUTOLOGIN=0 DO_AUTOSTART=0 DO_CALENDARS=0 DO_PIN=0 DO_SSH=0 DOC_AT_END=0
  echo "Custom mode — choose the tasks you want. Type y or n; b goes back; q returns to the menu without selecting anything."
  while [ "$index" -lt "$total" ]; do
    read -rp "  [$((index+1))/$total] ${task_labels[$index]}? [y/N, b=back, q=cancel] " answer || return 2
    answer="$(normalize_menu_choice "$answer")"
    case "$answer" in
      q|Q|quit|exit)
        echo "Nothing selected — returning to the menu."
        return 2
        ;;
      b|B)
        [ "$index" -gt 0 ] && index=$((index-1)) || warn "This is the first question."
        continue
        ;;
      ''|n|N) answers[$index]=0 ;;
      y|Y) answers[$index]=1 ;;
      *) warn "Please answer y, n, b, or q."; continue ;;
    esac
    index=$((index+1))
  done
  for index in "${!answers[@]}"; do
    [ "${answers[$index]:-0}" = 1 ] || continue
    case "$index" in
      0) DO_SYSTEM=1;;
      1) DO_PKGS=1; DO_FONTS=1;;
      2) DO_FILES=1;;
      3) DO_CUSTOM=1;;
      4) DO_WEATHER_DISPLAY=1;;
      5) DO_WEATHER=1;;
      6) DO_RADAR=1;;
      7) DO_MESSAGE_SOURCES=1;;
      8) DO_APP_SETUP=1;;
      9) DO_CALENDARS=1;;
      10) DO_ICAL=1;;
      11) DO_VDIR=1;;
      12) DO_PIN=1;;
      13) DO_SERVICE=1;;
      14) DO_AUTOLOGIN=1; DO_AUTOSTART=1;;
      15) DO_SSH=1;;
      16) DOC_AT_END=1;;
    esac
  done
  return 0
}

case "$MODE" in
  "$OPT_FULL")
     if [ "$EXPRESS_MODE" = "1" ]; then
       # Express avoids system upgrade, optional providers, remote access,
       # and lengthy preference menus. It remains a complete local kiosk
       # installation and every omitted choice is available later.
       DO_PKGS=1; DO_FILES=1; DO_FONTS=1; DO_CUSTOM=1; DO_SERVICE=1; DO_AUTOLOGIN=1; DO_AUTOSTART=1; DOC_AT_END=1
     else
       DO_SYSTEM=1; DO_PKGS=1; DO_FILES=1; DO_FONTS=1; DO_CUSTOM=1; DO_WEATHER_DISPLAY=1; DO_WEATHER=1; DO_RADAR=1; DO_MESSAGE_SOURCES=1; DO_SERVICE=1; DO_AUTOLOGIN=1; DO_AUTOSTART=1; DO_CALENDARS=1; DO_PIN=1; DO_SSH=1; DOC_AT_END=1
     fi
     ;;
  "$OPT_UPDATE") DO_FILES=1;;
  "$OPT_UPDATE_RECONFIGURE") DO_FILES=1; DO_CUSTOM=1; DO_WEATHER_DISPLAY=1; DO_WEATHER=1; DO_RADAR=1; DO_MESSAGE_SOURCES=1;;
  "$OPT_RECONFIGURE") DO_CUSTOM=1; DO_WEATHER_DISPLAY=1; DO_WEATHER=1; DO_RADAR=1; DO_MESSAGE_SOURCES=1;;
  "$OPT_WEATHER_DISPLAY") DO_WEATHER_DISPLAY=1;;
  "$OPT_WEATHER_SOURCES") DO_WEATHER=1;;
  "$OPT_RADAR") DO_RADAR=1;;
  "$OPT_CALENDARS") DO_CALENDARS=1;;
  "$OPT_ICAL") DO_ICAL=1;;
  "$OPT_VDIR") DO_VDIR=1;;
  "$OPT_MESSAGES") DO_MESSAGE_SOURCES=1;;
  "$OPT_TODO") DO_APP_SETUP=1;;
  "$OPT_THEME") # Theme-only: hand off to the deployed set-theme.sh (interactive) and exit.
     if [ -x "$BIN_DIR/set-theme.sh" ]; then
       "$BIN_DIR/set-theme.sh"
       exit $?
     fi
     menu_handoff_missing "Theme support";;
  "$OPT_SEASONAL") # Seasonal themes: show the current state, offer to flip it, and exit.
     if [ ! -x "$BIN_DIR/seasonal-themes.sh" ]; then
       menu_handoff_missing "Seasonal themes support"
     fi
     if crontab -l 2>/dev/null | grep -q "seasonal-themes.sh apply"; then
       echo "Seasonal auto-theming is currently ON — holiday themes apply by"
       echo "themselves and your base theme returns in between."
       read -rp "Turn it OFF? [y/N] " SOFF
       if [ "$SOFF" = "y" ] || [ "$SOFF" = "Y" ]; then
         "$BIN_DIR/seasonal-themes.sh" uninstall
       else
         echo "Left on. (Tip: you can also flip this in the on-screen control panel.)"
       fi
     else
       echo "Seasonal auto-theming is currently OFF."
       echo "Turning it on makes holiday themes (Christmas, Halloween, Pride, ...)"
       echo "apply automatically on the right dates, returning to your chosen"
       echo "base theme in between. Nothing else changes."
       read -rp "Turn it ON? [Y/n] " SON
       if [ "$SON" != "n" ] && [ "$SON" != "N" ]; then
         "$BIN_DIR/seasonal-themes.sh" install
       fi
     fi
     exit 0;;
  "$OPT_PIN") DO_PIN=1;;
  "$OPT_SERVICE") DO_SERVICE=1;;
  "$OPT_SSH") DO_SSH=1;;
  "$OPT_DOCTOR") # Doctor: run the deployed health-check script and exit.
     if [ -x "$BIN_DIR/doctor.sh" ]; then
       bash "$BIN_DIR/doctor.sh"; exit $?
     else
       menu_handoff_missing "Health Check"
     fi;;
  "$OPT_TOUR") # Feature tour: a plain-language overview, then exit.
     cat <<'TOUR'

============================ FEATURE TOUR =============================

 THE SCREEN
   * Month grid + agenda, weather with sunrise/sunset and moon phase,
     UV and air-quality pills, and friendly rotating messages.
   * "Today" is ringed in the theme color so it pops at a glance.
   * Severe-weather banners (US National Weather Service) appear on
     their own; tap for details, or HIDE to dock them to a corner tab.

 THE ON-SCREEN CONTROL PANEL  (the big one to remember)
   * TRIPLE-TAP the moon-phase icon next to the weather to open it.
     Triple-tap current weather to open radar when enabled. From there you can:
     update the dashboard to the newest version, restart the browser,
     reboot or shut down, sync calendars, rebuild the event cache, view
     logs, export diagnostics, run a health check, switch themes instantly,
     adjust performance, edit your location, and manage People, lists,
     routines, and household schedules.
   * Optional PIN lock and unlock duration: manage it from Control PIN.

 CALENDARS
   * Subscribe to public .ics URLs (Google, Outlook, Nextcloud, school,
     work) or set up a private vdirsyncer calendar, then see all events in one view.
   * Built-in holidays, sky events, celebrations, pickup schedules, and
     birthdays can be turned on/off separately. Drag/drop .ics files works too.

 WEATHER & MAPS
   * Pick a simple free forecast, a US blend, or an optional provider mix.
   * Radar is on-demand — it never polls or runs in the background.
   * Static event maps remain lightweight; interactive maps are optional on
     capable profiles.

 HOUSEHOLD TOOLS
   * Local To Do, Grocery, Routines, Chore Wheel, Maintenance, Family Board,
     and People remain useful offline. Microsoft To Do and notifications are
     opt-in enhancements, never required for normal household use.

 MAINTENANCE
   * Update the app keeps your saved settings/calendars. Doctor explains
     what is wrong and can offer the smallest safe repair. Backups are local
     and can be exported before larger repair or removal work.

 ADMINISTRATOR TOOLS
   * Notifications (Apprise-Go) are set up from SSH/terminal so destination
     URLs stay private. Terminal access can be shown or hidden in Dashboard
     Control without changing SSH.

 FILES YOU MAY WANT
   * ~/dashboard/config/             your settings, local app data, secrets
   * ~/dashboard/calendars/          local .ics calendars
   * ~/dashboard/bin/seasonal-themes.sh  holiday auto-theming
   * ~/dashboard/bin/doctor.sh           quiet health scan and fix-all prompt
   * ~/dashboard/bin/doctor.sh --full    detailed health report

 GOOD TO KNOW
   * Themes apply within a minute of any change — no restarts needed.
   * Your panel settings survive reboots and nightly browser restarts.
   * Calendars are plain .ics files in ~/dashboard/calendars named like
     family.blue.ics — drop one in and run bin/gen-calendars.sh.

=======================================================================
TOUR
     exit 0;;
  "$OPT_DEMO") DO_PKGS=1; DO_FILES=1; DO_FONTS=1; DO_SERVICE=1; DO_AUTOLOGIN=1; DO_AUTOSTART=1; DO_DEMO=1;;
  "$OPT_CUSTOM")
     if ! custom_questionnaire; then
       ok "No custom tasks selected. Returning to the installer menu without changing anything."
       exec bash "$0" "${INSTALLER_ORIGINAL_ARGS[@]}"
     fi
     ;;
  "$OPT_REMOVE") run_remove_install; exit $?;;
  "$OPT_NOTIFICATIONS") configure_apprise_notifications; exit $?;;
  "$OPT_TERMINAL") configure_terminal_access; exit $?;;
  "$OPT_EXIT"|q|Q|quit|exit)
     ok "installer closed without making changes"
     exit 0;;
  # The menu loop validates every normal input before dispatch. Keep this
  # defensive fallback non-fatal so a future menu refactor cannot strand a
  # novice after a harmless typo or silently discard their original flags.
  *)
     warn "That menu choice could not be used. Returning to the menu; nothing was changed."
     exec bash "$0" "${INSTALLER_ORIGINAL_ARGS[@]}"
     ;;
esac

# The Go control-server helper owns installer JSON, PIN, and geocoding writes.
# A focused first-run/custom selection might otherwise reach those prompts before
# a payload exists, so transparently add the safe app-file refresh it requires.
if [ "$DO_FILES" != "1" ] && { [ "$DO_CUSTOM" = "1" ] || [ "$DO_WEATHER_DISPLAY" = "1" ] || [ "$DO_WEATHER" = "1" ] || [ "$DO_RADAR" = "1" ] || [ "$DO_MESSAGE_SOURCES" = "1" ] || [ "$DO_APP_SETUP" = "1" ] || [ "$DO_PIN" = "1" ] || [ "$DO_VDIR" = "1" ]; }; then
  if [ ! -x "$BIN_DIR/dashboard-control-server" ]; then
    warn "This setup choice needs the current Dash-Go helper. Enabling Update the app first; your existing settings and calendars will be kept."
    DO_FILES=1
  fi
fi

# A detected Demo Mode is actionable only after the user has chosen an actual
# installer workflow. Health, tour, Exit, notifications, and terminal actions
# already returned above, so they are never blocked by a destructive prompt.
[ "$MODE" = "$OPT_DEMO" ] || prompt_demo_mode_reset
if [ "$REMOVE_MODE" = "1" ]; then
  run_remove_install
  exit $?
fi
run_interactive_preflight
installer_build_stage_plan

# Full install also installs packages + fonts; presets assume those exist.
[ "$MODE" = "$OPT_FULL" ] && DO_PKGS=1

if [ "${RESTORE_FROM_BACKUP:-0}" = "1" ]; then
  echo
  echo "Restore source selected: $RESTORE_ARCHIVE"
  if [ "${RESTORE_SKIP_SETUP:-1}" = "1" ]; then
    DO_CUSTOM=0
    DO_CALENDARS=0
    ok "will restore saved settings/calendars instead of re-answering setup/calendar questions"
  else
    ok "will restore saved data, then still run selected setup/calendar questions"
  fi
  if [ "$DO_FILES" != "1" ]; then
    warn "restore was requested but app-file download is not selected; enabling app-file download so restore helpers are available"
    DO_FILES=1
  fi
fi

echo
if [ "$EXPRESS_MODE" = "1" ]; then
  ok "Continuing with Express setup. You can change any default later in Dashboard Control."
else
  selected_count="$(installer_selected_tasks | wc -l | tr -d ' ')"
  echo "The following $selected_count safe, re-runnable task(s) will run:"
  installer_selected_tasks | sed 's/^/  • /'
  if [ "$DO_SYSTEM" = "1" ]; then
    read -rp "Run these $selected_count tasks now? This includes optional system maintenance plus managed security updates. [y/N] " proceed
  else
    read -rp "Run these $selected_count tasks now? [Y/n] " proceed
  fi
  case "$(normalize_menu_choice "$proceed")" in
    n|N|q|Q|quit|exit) ok "Nothing was changed — run ~/install.sh again anytime."; exit 0;;
  esac
fi

# A correct clock alone is not enough for calendar and weather semantics. Ask
# only after the user has approved the selected work, and never infer a zone.
prompt_timezone_if_needed || exit 1
if interactive_run_needs_sudo; then
  start_sudo_keepalive || exit 1
fi

APP_FILES_OK=0
DEMO_MODE_OK=0
SERVICE_CONFIRMED_OK=0

# ---------------------------------------------------------------------
configure_optional_zram_tuning(){
  [ "$IS_PI" = "1" ] || return 0

  local zram_cfg="/etc/default/zramswap" sysctl_cfg="/etc/sysctl.d/99-dash-go-zram.conf"
  local had_zram=0 managed_cfg=0 tmp clean

  say "Optional Pi memory / SD-card tuning"
  echo "zram keeps compressed swap in RAM, reducing SD-card swap I/O when"
  echo "WebKit is under memory pressure. This is useful on 512 MB Pi kiosks."
  echo "Dash-Go can configure zram-tools with a 50% RAM cap (256 MB on a"
  echo "Pi Zero 2 W) and vm.swappiness=10. It does not change your root mount."
  echo "A pre-existing non-Dash-Go zram-tools configuration is left unchanged."
  read -rp "Enable optional zram memory tuning? [y/N] " zram_choice
  case "$zram_choice" in
    y|Y) ;;
    *) ok "optional zram tuning skipped"; return 0;;
  esac

  command -v apt-get >/dev/null 2>&1 || { warn "apt-get is unavailable; could not install zram-tools"; return 1; }
  if command -v dpkg-query >/dev/null 2>&1 && dpkg-query -W -f='${Status}' zram-tools 2>/dev/null | grep -q 'install ok installed'; then
    had_zram=1
  fi
  if ! apt_managed install -y zram-tools; then
    warn "could not install zram-tools"
    return 1
  fi

  if $SUDO test -f "$zram_cfg" && $SUDO grep -Fq '# Managed by Dash-Go optional zram tuning' "$zram_cfg" 2>/dev/null; then
    managed_cfg=1
  fi
  if [ "$had_zram" = "0" ] || [ "$managed_cfg" = "1" ] || ! $SUDO test -f "$zram_cfg"; then
    tmp="$(mktemp)" || return 1
    clean="$(mktemp)" || { rm -f "$tmp"; return 1; }
    if $SUDO test -f "$zram_cfg"; then
      $SUDO cat "$zram_cfg" > "$tmp" || { rm -f "$tmp" "$clean"; warn "could not read existing zram configuration"; return 1; }
    else
      : > "$tmp"
    fi
    # Preserve any distribution/default options other than the two values this
    # opt-in owns. Uncommented and commented prior values are removed so the
    # result is deterministic without discarding unrelated configuration.
    awk '
      /^[[:space:]]*#?[[:space:]]*(PERCENT|PRIORITY)=/ { next }
      /^[[:space:]]*# Managed by Dash-Go optional zram tuning/ { next }
      /^[[:space:]]*# \/etc\/default\/zramswap to return to your preferred zram-tools settings\./ { next }
      /^[[:space:]]*# This is a maximum virtual zram size; it consumes RAM only as swap is used\./ { next }
      { print }
    ' "$tmp" > "$clean"
    cat >> "$clean" <<'ZRAMCONF'

# Managed by Dash-Go optional zram tuning. Remove this block or edit
# /etc/default/zramswap to return to your preferred zram-tools settings.
# This is a maximum virtual zram size; it consumes RAM only as swap is used.
PERCENT=50
PRIORITY=100
ZRAMCONF
    if ! $SUDO install -m 0644 "$clean" "$zram_cfg"; then
      rm -f "$tmp" "$clean"
      warn "could not write $zram_cfg"
      return 1
    fi
    rm -f "$tmp" "$clean"
    ok "zram-tools configured with a 50% RAM cap"
  else
    ok "existing non-Dash-Go zram-tools configuration left unchanged"
  fi

  if ! $SUDO tee "$sysctl_cfg" >/dev/null <<'SYSCTL'
# Managed by Dash-Go optional Pi kiosk memory tuning.
# Favor RAM until there is genuine pressure; zram remains the higher-priority swap.
vm.swappiness=10
SYSCTL
  then
    warn "zram is installed, but $sysctl_cfg could not be written"
    return 1
  fi
  if ! $SUDO sysctl -w vm.swappiness=10 >/dev/null 2>&1; then
    warn "zram configuration was written, but vm.swappiness could not be applied immediately"
    return 1
  fi
  if command -v systemctl >/dev/null 2>&1; then
    if ! $SUDO systemctl enable --now zramswap.service; then
      warn "zram-tools configuration was written, but zramswap.service did not start"
      return 1
    fi
  else
    warn "zram-tools configuration was written, but systemctl is unavailable to start it"
    return 1
  fi
  if command -v swapon >/dev/null 2>&1 && swapon --noheadings --show=NAME 2>/dev/null | grep -q '/dev/zram'; then
    ok "zram swap is active; vm.swappiness=10"
  else
    warn "zramswap.service started but /dev/zram is not yet visible; check: systemctl status zramswap"
  fi
}

# ---------------------------------------------------------------------
if [ "$DO_SYSTEM" = "1" ]; then
installer_stage "Preparing the operating system"
say "System update / platform trim and security maintenance"
echo "Detected platform: $PLATFORM_LABEL"
echo
if [ "$IS_PI" = "1" ]; then
  echo "Raspberry Pi mode: apt full-upgrade plus optional Pi-kiosk service"
  echo "trim/autostart cleanup. This is useful on dedicated Pi appliances, but"
  echo "it is invasive and can be slow on small boards."
  read -rp "Proceed with Pi update + kiosk trim? [y/N] " dosys
  if { [ "$dosys" = "y" ] || [ "$dosys" = "Y" ]; }; then
    say "  apt update && full-upgrade (this can take a while on small boards)"
    apt_managed update && apt_managed -y full-upgrade && apt_managed -y autoremove
    ok "system updated"

    say "  Disabling services not needed for a Pi kiosk"
    for svc in hciuart exim4 ModemManager nfs-blkmap e2scrub_reap \
               udisks2 \
               NetworkManager-wait-online xinetd accounts-daemon; do
      if systemctl list-unit-files 2>/dev/null | grep -q "^$svc"; then
        $SUDO systemctl disable --now "$svc" 2>/dev/null && ok "disabled $svc" || warn "could not disable $svc"
      fi
    done
    warn "Kept (needed): NetworkManager/wpa_supplicant/dhcpcd as present, ssh, lightdm,"
    warn "cron, systemd-timesyncd, avahi-daemon (.local hostname), dashboard server."

    say "  Removing a desktop extra (diodon clipboard manager)"
    apt_managed -y remove --purge diodon 2>/dev/null && ok "removed diodon" || true

    say "  Suppressing desktop autostart items (per-user, reversible)"
    mkdir -p "$HOME/.config/autostart"
    for app in nm-applet diodon-autostart lxpolkit at-spi-dbus-bus xcompmgr \
               org.gnome.Evolution-alarm-notify org.kde.korgac onboard-autostart \
               orca-autostart print-applet org.kde.kdeconnect.daemon; do
      printf '[Desktop Entry]\nHidden=true\n' > "$HOME/.config/autostart/$app.desktop"
    done
    ok "autostart trimmed (undo by deleting files in ~/.config/autostart)"
  else
    warn "Skipped Pi update + trim."
  fi
else
  echo "Non-Pi mode: the installer will NOT apply Pi-specific de-bloat or"
  echo "Raspberry Pi boot/config changes. On Debian x86/Trixie, a normal apt"
  echo "update is safe, but service trimming is skipped by default."
  read -rp "Run apt update + full-upgrade + autoremove? [y/N] " dosys
  if { [ "$dosys" = "y" ] || [ "$dosys" = "Y" ]; }; then
    apt_managed update && apt_managed -y full-upgrade && apt_managed -y autoremove
    ok "system updated"
  else
    warn "Skipped system update."
  fi
  echo
  read -rp "Apply optional generic kiosk autostart cleanup? [y/N] " dotrim
  if [ "$dotrim" = "y" ] || [ "$dotrim" = "Y" ]; then
    mkdir -p "$HOME/.config/autostart"
    for app in org.gnome.Evolution-alarm-notify org.kde.korgac diodon-autostart \
               onboard-autostart orca-autostart print-applet; do
      printf '[Desktop Entry]\nHidden=true\n' > "$HOME/.config/autostart/$app.desktop"
    done
    ok "generic autostart cleanup written (reversible by deleting files in ~/.config/autostart)"
  else
    ok "left desktop services/autostart alone"
  fi
fi
if [ "$IS_PI" = "1" ]; then
  configure_optional_zram_tuning || warn "optional zram tuning did not complete; existing dashboard behavior is unchanged"
fi
say "Managed Debian security maintenance"
if ! ensure_security_maintenance_once system; then
  mark_install_step_failed "Managed Debian security maintenance" "Run ~/install.sh --repair --system from a terminal after resolving the named APT policy or permission warning."
else
  mark_reboot_recommended
fi
fi

# ---------------------------------------------------------------------
if [ "$DO_PKGS" = "1" ]; then
  installer_stage "Installing dashboard components"
  if ! install_runtime_packages; then
    mark_install_step_failed "Required kiosk packages" "No autologin, service, or kiosk settings were changed. Fix the named package/source issue, then run ~/install.sh again."
    warn "Required kiosk provisioning stopped safely. Fix the warning above, then run ~/install.sh again."
    exit 1
  fi
  if [ "$DO_SYSTEM" != "1" ]; then
    say "Managed Debian security maintenance"
    if ! ensure_security_maintenance_once install; then
      mark_install_step_failed "Managed Debian security maintenance" "Run ~/install.sh --repair --system from a terminal after resolving the named APT policy or permission warning."
    fi
  fi
fi  # end DO_PKGS

# ---------------------------------------------------------------------
# Always ensure the dashboard dir exists (every task needs it).
mkdir -p "$DASH"
cd "$DASH" || { warn "cannot enter $DASH"; exit 1; }

# ---------------------------------------------------------------------
if [ "$DO_FILES" = "1" ]; then
installer_stage "Installing Dash-Go"
say "Downloading app files"
if download_app_files; then
  APP_FILES_OK=1
  if normalize_app_visibility_preferences; then
    ok "retired legacy app-visibility preferences; all Apps remain available"
  else
    warn "could not normalize legacy app-visibility preferences; beta.37 still ignores them at runtime"
  fi
  [ -x "$BIN_DIR/update-holidays.sh" ] && "$BIN_DIR/update-holidays.sh" >/dev/null 2>&1 || true
  [ -x "$BIN_DIR/update-iss-passes.sh" ] && "$BIN_DIR/update-iss-passes.sh" >/dev/null 2>&1 || true
  [ -x "$BIN_DIR/gen-default-calendars.sh" ] && "$BIN_DIR/gen-default-calendars.sh" >/dev/null 2>&1 || true
  [ -x "$BIN_DIR/gen-sky-calendars.sh" ] && "$BIN_DIR/gen-sky-calendars.sh" >/dev/null 2>&1 || true
  [ -x "$BIN_DIR/dashboard-control-server" ] && "$BIN_DIR/dashboard-control-server" --update-message-feeds >/dev/null 2>&1 || true
  [ -x "$BIN_DIR/gen-calendars.sh" ] && "$BIN_DIR/gen-calendars.sh" >/dev/null 2>&1
  [ -x "$BIN_DIR/dashboard-control-server" ] && "$BIN_DIR/dashboard-lowprio.sh" "$BIN_DIR/dashboard-control-server" --gen-events-cache >/dev/null 2>&1
else
  warn "some downloads failed/failed validation — existing copies kept"
fi
# The web server runs from the verified, manifest-owned selector. Restart only
# after a successful file commit so a failed manifest/download does
# not misleadingly report that the new update is live.
if [ "$APP_FILES_OK" = "1" ] && [ -f /etc/systemd/system/dashboard-server.service ] && command -v systemctl >/dev/null 2>&1; then

  ensure_go_dashboard_service_unit || true
  if $SUDO systemctl restart dashboard-server.service 2>/dev/null; then
    ok "web server restarted — the update is live"
  else
    warn "couldn't restart the web server — run: sudo systemctl restart dashboard-server"
  fi
elif [ "$DO_FILES" = "1" ] && [ "$APP_FILES_OK" != "1" ]; then
  warn "app files were not updated, so the web server was not restarted as a new update"
fi
fi

# If the app payload is now present, prefer the shared helper implementation for
# profile/session/sudo/no-lock logic. install.sh keeps bootstrap fallbacks above
# so first-run package installation can still work before the app exists.
if [ -r "$BIN_DIR/dashboard-common.sh" ]; then
  # shellcheck disable=SC1090
  . "$BIN_DIR/dashboard-common.sh"
fi

# Apply a fresh-install restore after app files are in place so current helper
# scripts and settings defaults are available. This supports archives created by
# --repair, --remove, and Dashboard Control config-backup ZIPs.
if [ "${RESTORE_FROM_BACKUP:-0}" = "1" ]; then
  installer_stage "Restoring saved dashboard data"
  say "Restoring previous dashboard settings/calendars"
  if restore_previous_install_data "$RESTORE_ARCHIVE"; then
    ok "restore source applied"
  else
    warn "restore source could not be applied; continuing with install"
  fi
fi

# ---------------------------------------------------------------------
reset_demo_mode_if_requested
if [ "$DO_DEMO" = "1" ]; then
  if enable_demo_mode; then
    DEMO_MODE_OK=1
  else
    warn "Demo Mode could not be fully enabled"
  fi
fi

# ---------------------------------------------------------------------
# Fonts are checked on every install/update. Existing fonts are left alone;
# missing fonts are downloaded best-effort from same-host paths first, then
# upstream. Missing fonts are non-fatal because CSS fallback stacks are tuned.
if ! ensure_dashboard_fonts; then
  warn "dashboard display fonts could not be recovered; readable system fallbacks will be used. Run ~/install.sh --repair --system after network access is available."
fi

# ---------------------------------------------------------------------
# Input validators (defined always; cheap function declarations).
# Latitude -90..90, longitude -180..180, decimals allowed (optionally signed).
valid_lat(){ printf '%s' "$1" | grep -qE '^-?([0-8]?[0-9](\.[0-9]+)?|90(\.0+)?)$'; }
valid_lon(){ printf '%s' "$1" | grep -qE '^-?(1[0-7][0-9]|[0-9]?[0-9])(\.[0-9]+)?$|^-?180(\.0+)?$'; }
# MM-DD: month 01-12, day 01-31 (calendar-light; doesn't reject e.g. 02-30).
valid_mmdd(){ printf '%s' "$1" | grep -qE '^(0[1-9]|1[0-2])-(0[1-9]|[12][0-9]|3[01])$'; }
# API keys and endpoint values are pasted often. Trim harmless surrounding
# whitespace before persisting them, while retaining the previous saved value
# when the user presses Enter.
store_optional_trimmed(){
  local target="$1" value
  value="$(trim_input "$2")"
  [ -n "$value" ] && printf -v "$target" '%s' "$value"
}
json_escape(){ printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'; }

write_radar_env(){
  mkdir -p "$HOME" 2>/dev/null || true
  old_umask="$(umask)"; umask 077
  {
    echo "# saved by install.sh — weather-radar keys for the local dashboard server"
    echo "# Kept outside ~/dashboard/config so keys are never served to the browser."
    printf 'DASH_RADAR_TOMORROW_KEY=%q\n' "${key_radar_tomorrow:-}"
    printf 'DASH_RADAR_WEATHERBIT_KEY=%q\n' "${key_radar_weatherbit:-}"
    printf 'DASH_RADAR_XWEATHER_ID=%q\n' "${key_radar_xweather_id:-}"
    printf 'DASH_RADAR_XWEATHER_SECRET=%q\n' "${key_radar_xweather_secret:-}"
  } > "$RADAR_ENV"
  umask "$old_umask"
  chmod 600 "$RADAR_ENV" 2>/dev/null || true
  ok "radar API keys saved to $RADAR_ENV (not config.local.js)"
}

write_radar_settings_only(){
  mkdir -p "$CONFIG_DIR"
  installer_cli --installer-config-local --file "$CONFIG_DIR/config.local.js" --mode radar \
    --provider "${RADAR_PROVIDER:-rainviewer}" --custom-tiles "${RADAR_CUSTOM_TILES:-}" || {
      warn "Could not save radar settings. Run ~/install.sh again to retry this step."
      return 1
    }
  ok "wrote radar settings to config/config.local.js"
}

prompt_radar_provider(){
  RADAR_PROVIDER="rainviewer"; RADAR_CUSTOM_TILES=""
  key_radar_tomorrow=""; key_radar_weatherbit=""; key_radar_xweather_id=""; key_radar_xweather_secret=""
  if [ -f "$RADAR_ENV" ]; then
    # shellcheck disable=SC1090
    . "$RADAR_ENV"
    key_radar_tomorrow="${DASH_RADAR_TOMORROW_KEY:-}"
    key_radar_weatherbit="${DASH_RADAR_WEATHERBIT_KEY:-}"
    key_radar_xweather_id="${DASH_RADAR_XWEATHER_ID:-}"
    key_radar_xweather_secret="${DASH_RADAR_XWEATHER_SECRET:-}"
  fi
  if [ -f "$CONFIG_DIR/config.local.js" ]; then
    RADAR_PROVIDER="$(sed -nE 's/.*radarProvider[[:space:]]*:[[:space:]]*"?([^",} ]+)"?.*/\1/p' "$CONFIG_DIR/config.local.js" | head -1)"
    RADAR_PROVIDER="${RADAR_PROVIDER:-rainviewer}"
    RADAR_CUSTOM_TILES="$(sed -nE 's/.*radarCustomTiles[[:space:]]*:[[:space:]]*"([^"]*)".*/\1/p' "$CONFIG_DIR/config.local.js" | head -1)"
  fi
  echo
  echo "Weather radar (opens only on triple-tap of current weather; no cron/background polling)"
  echo "  1) RainViewer       free · no key · global · animated (default)"
  echo "  2) NWS / NOAA       free · no key · US-only · latest frame"
  echo "  3) Tomorrow.io      key required · metered map tiles"
  echo "  4) Weatherbit Maps  key required · Maps plan"
  echo "  5) Xweather Maps    key required · metered map tiles"
  echo "  6) Custom XYZ       advanced · public HTTPS tile template"
  read -rp "  Choose [1-6, Enter=current ${RADAR_PROVIDER}]: " ans
  case "$ans" in
    2) RADAR_PROVIDER="nws";; 3) RADAR_PROVIDER="tomorrow";; 4) RADAR_PROVIDER="weatherbit";; 5) RADAR_PROVIDER="xweather";; 6) RADAR_PROVIDER="custom_xyz";; 1|"") :;; *) warn "unknown radar choice; keeping ${RADAR_PROVIDER}";;
  esac
  case "$RADAR_PROVIDER" in
    tomorrow) read -rp "  Tomorrow.io radar key [$([ -n "$key_radar_tomorrow" ] && echo saved || echo blank)]: " ans; store_optional_trimmed key_radar_tomorrow "$ans";;
    weatherbit) read -rp "  Weatherbit Maps key [$([ -n "$key_radar_weatherbit" ] && echo saved || echo blank)]: " ans; store_optional_trimmed key_radar_weatherbit "$ans";;
    xweather) read -rp "  Xweather client ID [$([ -n "$key_radar_xweather_id" ] && echo saved || echo blank)]: " ans; store_optional_trimmed key_radar_xweather_id "$ans"; read -rp "  Xweather client secret [$([ -n "$key_radar_xweather_secret" ] && echo saved || echo blank)]: " ans; store_optional_trimmed key_radar_xweather_secret "$ans";;
    custom_xyz) read -rp "  HTTPS XYZ template (use {z}/{x}/{y}): " ans; store_optional_trimmed RADAR_CUSTOM_TILES "$ans";;
  esac
  if [ "$RADAR_PROVIDER" = "tomorrow" ] && [ -z "$key_radar_tomorrow" ]; then warn "Tomorrow.io has no saved key; RainViewer will be used until a key is added."; fi
  if [ "$RADAR_PROVIDER" = "weatherbit" ] && [ -z "$key_radar_weatherbit" ]; then warn "Weatherbit Maps has no saved key; RainViewer will be used until a key is added."; fi
  if [ "$RADAR_PROVIDER" = "xweather" ] && { [ -z "$key_radar_xweather_id" ] || [ -z "$key_radar_xweather_secret" ]; }; then warn "Xweather has no complete saved key; RainViewer will be used until keys are added."; fi
  write_radar_env
  ok "radar provider=${RADAR_PROVIDER}"
}

write_weather_env(){
  mkdir -p "$HOME" 2>/dev/null || true
  old_umask="$(umask)"; umask 077
  {
    echo "# saved by install.sh — weather API keys for the local dashboard server"
    echo "# API keys are intentionally kept outside ~/dashboard/config so they are not served to the browser."
    printf 'DASH_WEATHERAPI_KEY=%q\n' "${key_weatherapi:-}"
    printf 'DASH_OPENWEATHER_KEY=%q\n' "${key_openweather:-}"
    printf 'DASH_GOOGLE_WEATHER_KEY=%q\n' "${key_googleweather:-}"
    printf 'DASH_TOMORROW_KEY=%q\n' "${key_tomorrow:-}"
    printf 'DASH_VISUALCROSSING_KEY=%q\n' "${key_visualcrossing:-}"
    printf 'DASH_WEATHERBIT_KEY=%q\n' "${key_weatherbit:-}"
    printf 'DASH_PIRATEWEATHER_KEY=%q\n' "${key_pirateweather:-}"
    printf 'DASH_ACCUWEATHER_KEY=%q\n' "${key_accuweather:-}"
    printf 'DASH_XWEATHER_KEY=%q\n' "${key_xweather:-}"
    printf 'DASH_OPENMETEO_CUSTOM_KEY=%q\n' "${key_openmeteocustom:-}"
  } > "$WEATHER_ENV"
  umask "$old_umask"
  chmod 600 "$WEATHER_ENV" 2>/dev/null || true
  ok "weather API keys saved to $WEATHER_ENV (not config.local.js)"
}

write_message_env(){
  mkdir -p "$HOME" 2>/dev/null || true
  old_umask="$(umask)"; umask 077
  {
    echo "# saved by install.sh — message-source API keys for the local dashboard server"
    echo "# API keys are intentionally kept outside ~/dashboard/config so they are not served to the browser."
    printf 'DASH_API_NINJAS_KEY=%q\n' "${key_api_ninjas:-}"
  } > "$MESSAGE_ENV"
  umask "$old_umask"
  chmod 600 "$MESSAGE_ENV" 2>/dev/null || true
  ok "message-source API keys saved to $MESSAGE_ENV (not config.local.js)"
}

prompt_message_api_keys(){
  key_api_ninjas=""
  if [ -f "$MESSAGE_ENV" ]; then
    # shellcheck disable=SC1090
    . "$MESSAGE_ENV"
    key_api_ninjas="${DASH_API_NINJAS_KEY:-}"
  fi
  echo
  echo "Message-source API keys"
  echo "  Optional. Leave blank to use only key-free providers plus local fallback."
  echo "  Currently supported keyed provider family: API Ninjas."
  read -rp "  API Ninjas key [$([ -n "$key_api_ninjas" ] && echo saved || echo blank)]: " ans
  store_optional_trimmed key_api_ninjas "$ans"
  write_message_env
}

prompt_weather_sources(){
  WEATHER_PROVIDERS="openmeteo"; WEATHER_KEYS_JS="{}"; WXAPI=""; AQAPI=""; APIKEY=""
  key_weatherapi=""; key_openweather=""; key_googleweather=""; key_tomorrow=""; key_visualcrossing=""; key_weatherbit=""; key_pirateweather=""; key_accuweather=""; key_xweather=""; key_openmeteocustom=""
  if [ -f "$WEATHER_ENV" ]; then
    # shellcheck disable=SC1090
    . "$WEATHER_ENV"
    key_weatherapi="${DASH_WEATHERAPI_KEY:-}"
    key_openweather="${DASH_OPENWEATHER_KEY:-}"
    key_googleweather="${DASH_GOOGLE_WEATHER_KEY:-${DASH_GOOGLEWEATHER_KEY:-}}"
    key_tomorrow="${DASH_TOMORROW_KEY:-}"
    key_visualcrossing="${DASH_VISUALCROSSING_KEY:-}"
    key_weatherbit="${DASH_WEATHERBIT_KEY:-${DASH_METEOSOURCE_KEY:-}}"
    key_pirateweather="${DASH_PIRATEWEATHER_KEY:-}"
    key_accuweather="${DASH_ACCUWEATHER_KEY:-}"
    key_xweather="${DASH_XWEATHER_KEY:-}"
    key_openmeteocustom="${DASH_OPENMETEO_CUSTOM_KEY:-}"
  fi

  existing="openmeteo"
  if [ -f "$CONFIG_DIR/config.local.js" ]; then
    existing="$(installer_cli --installer-weather-providers "$CONFIG_DIR/config.local.js" 2>/dev/null || printf 'openmeteo')"
  fi

  sel_openmeteo=0; sel_nws=0; sel_weatherapi=0; sel_openweather=0; sel_googleweather=0; sel_tomorrow=0; sel_visualcrossing=0; sel_weatherbit=0; sel_pirateweather=0; sel_accuweather=0; sel_xweather=0; sel_custom=0
  for p in $existing; do
    case "$p" in
      openmeteo) sel_openmeteo=1;; nws) sel_nws=1;; weatherapi) sel_weatherapi=1;; openweather) sel_openweather=1;; googleweather) sel_googleweather=1;; tomorrow) sel_tomorrow=1;; visualcrossing) sel_visualcrossing=1;; weatherbit) sel_weatherbit=1;; pirateweather) sel_pirateweather=1;; accuweather) sel_accuweather=1;; xweather) sel_xweather=1;; openmeteo-custom) sel_custom=1;;
    esac
  done
  if [ "$sel_openmeteo$sel_nws$sel_weatherapi$sel_openweather$sel_googleweather$sel_tomorrow$sel_visualcrossing$sel_weatherbit$sel_pirateweather$sel_accuweather$sel_xweather$sel_custom" = "000000000000" ]; then sel_openmeteo=1; fi

  show_weather_choice(){ [ "$1" = "1" ] && printf '[x]' || printf '[ ]'; }
  # The printed ordering is deliberate: every group uses sequential numeric
  # selections so a user can choose by its visible number without translating
  # the provider’s historical registry position.
  toggle_weather_choice(){
    case "$1" in
      1) [ "$sel_openmeteo" = "1" ] && sel_openmeteo=0 || sel_openmeteo=1;;
      2) [ "$sel_nws" = "1" ] && sel_nws=0 || sel_nws=1;;
      3) [ "$sel_weatherapi" = "1" ] && sel_weatherapi=0 || sel_weatherapi=1;;
      4) [ "$sel_tomorrow" = "1" ] && sel_tomorrow=0 || sel_tomorrow=1;;
      5) [ "$sel_visualcrossing" = "1" ] && sel_visualcrossing=0 || sel_visualcrossing=1;;
      6) [ "$sel_weatherbit" = "1" ] && sel_weatherbit=0 || sel_weatherbit=1;;
      7) [ "$sel_pirateweather" = "1" ] && sel_pirateweather=0 || sel_pirateweather=1;;
      8) [ "$sel_xweather" = "1" ] && sel_xweather=0 || sel_xweather=1;;
      9) [ "$sel_openweather" = "1" ] && sel_openweather=0 || sel_openweather=1;;
      10) [ "$sel_googleweather" = "1" ] && sel_googleweather=0 || sel_googleweather=1;;
      11) [ "$sel_accuweather" = "1" ] && sel_accuweather=0 || sel_accuweather=1;;
      12) [ "$sel_custom" = "1" ] && sel_custom=0 || sel_custom=1;;
    esac
  }
  auto_disable_keyless_weather_sources(){
    disabled=""
    if [ "$sel_weatherapi" = "1" ] && [ -z "${key_weatherapi:-}" ]; then sel_weatherapi=0; disabled="$disabled WeatherAPI.com"; fi
    if [ "$sel_openweather" = "1" ] && [ -z "${key_openweather:-}" ]; then sel_openweather=0; disabled="$disabled OpenWeather"; fi
    if [ "$sel_googleweather" = "1" ] && [ -z "${key_googleweather:-}" ]; then sel_googleweather=0; disabled="$disabled GoogleWeather"; fi
    if [ "$sel_tomorrow" = "1" ] && [ -z "${key_tomorrow:-}" ]; then sel_tomorrow=0; disabled="$disabled Tomorrow.io"; fi
    if [ "$sel_visualcrossing" = "1" ] && [ -z "${key_visualcrossing:-}" ]; then sel_visualcrossing=0; disabled="$disabled VisualCrossing"; fi
    if [ "$sel_weatherbit" = "1" ] && [ -z "${key_weatherbit:-}" ]; then sel_weatherbit=0; disabled="$disabled Weatherbit"; fi
    if [ "$sel_pirateweather" = "1" ] && [ -z "${key_pirateweather:-}" ]; then sel_pirateweather=0; disabled="$disabled PirateWeather"; fi
    if [ "$sel_accuweather" = "1" ] && [ -z "${key_accuweather:-}" ]; then sel_accuweather=0; disabled="$disabled AccuWeather"; fi
    if [ "$sel_xweather" = "1" ] && [ -z "${key_xweather:-}" ]; then sel_xweather=0; disabled="$disabled Xweather"; fi
    if [ -n "$disabled" ]; then
      warn "keyed weather source(s) without a saved key were toggled off:$disabled"
    fi
  }
  apply_weather_preset(){
    case "$1" in
      default)
        sel_openmeteo=1; sel_nws=0; sel_weatherapi=0; sel_openweather=0; sel_googleweather=0; sel_tomorrow=0; sel_visualcrossing=0; sel_weatherbit=0; sel_pirateweather=0; sel_accuweather=0; sel_xweather=0; sel_custom=0;;
      us)
        sel_openmeteo=1; sel_nws=1; sel_weatherapi=0; sel_openweather=0; sel_googleweather=0; sel_tomorrow=0; sel_visualcrossing=0; sel_weatherbit=0; sel_pirateweather=0; sel_accuweather=0; sel_xweather=0; sel_custom=0;;
    esac
  }
  build_weather_list(){
    weather_list=""
    [ "$sel_openmeteo" = "1" ] && weather_list="$weather_list openmeteo"
    [ "$sel_nws" = "1" ] && weather_list="$weather_list nws"
    [ "$sel_weatherapi" = "1" ] && weather_list="$weather_list weatherapi"
    [ "$sel_openweather" = "1" ] && weather_list="$weather_list openweather"
    [ "$sel_googleweather" = "1" ] && weather_list="$weather_list googleweather"
    [ "$sel_tomorrow" = "1" ] && weather_list="$weather_list tomorrow"
    [ "$sel_visualcrossing" = "1" ] && weather_list="$weather_list visualcrossing"
    [ "$sel_weatherbit" = "1" ] && weather_list="$weather_list weatherbit"
    [ "$sel_pirateweather" = "1" ] && weather_list="$weather_list pirateweather"
    [ "$sel_accuweather" = "1" ] && weather_list="$weather_list accuweather"
    [ "$sel_xweather" = "1" ] && weather_list="$weather_list xweather"
    [ "$sel_custom" = "1" ] && weather_list="$weather_list openmeteo-custom"
    [ -n "$weather_list" ] || weather_list=" openmeteo"
    WEATHER_PROVIDERS="$(printf '%s
' $weather_list | awk '!seen[$0]++' | tr '
' ' ' | sed 's/^ *//;s/ *$//')"
  }
  weather_review(){ build_weather_list; echo "  Selected: ${WEATHER_PROVIDERS}"; }

  auto_disable_keyless_weather_sources
  echo
  echo "Weather source setup"
  echo "  Recommended choices avoid paid/trial sources and keep the provider blend easy to understand."
  echo "  You can still choose Custom blend for every source, key, and paid/billable provider."
  echo
  echo "  1) Recommended global       Open-Meteo only · free · no key · global"
  echo "  2) Recommended US           Open-Meteo + NWS/NOAA · free · no key · NWS skips unsupported coordinates"
  echo "  3) Keep current selection   ${existing}"
  echo "  4) Custom blend             grouped provider menu with free/keyed/paid sections"
  echo "  5) Cancel"
  read -rp "  Choose weather setup [3]: " wxmode; wxmode="${wxmode:-3}"
  case "$wxmode" in
    1) apply_weather_preset default;;
    2) apply_weather_preset us;;
    3) :;;
    4) while :; do
         echo
         echo "Custom weather blend — toggle one number at a time"
         echo "  No-key sources"
         echo "  $(show_weather_choice "$sel_openmeteo")  1) Open-Meteo        free · no key · recommended default · global"
         echo "  $(show_weather_choice "$sel_nws")  2) NWS / NOAA        free · no key · US-only · skips unsupported coordinates"
         echo
         echo "  Free key sources"
         echo "  $(show_weather_choice "$sel_weatherapi")  3) WeatherAPI.com    free key · 100K/month · 3-day free forecast"
         echo "  $(show_weather_choice "$sel_tomorrow")  4) Tomorrow.io       free key · 500/day, 25/hour cap · ~5-day"
         echo "  $(show_weather_choice "$sel_visualcrossing")  5) Visual Crossing   free key · 1,000 records/day · attribution required · 15-day"
         echo "  $(show_weather_choice "$sel_weatherbit")  6) Weatherbit        free key · 50/day · NON-COMMERCIAL · 7-day"
         echo "  $(show_weather_choice "$sel_pirateweather")  7) Pirate Weather    free key · 10,000/month · 8-day"
         echo "  $(show_weather_choice "$sel_xweather")  8) Xweather          free/trial/metered · conservative 9K/month cap · US/CA · 15-day"
         echo
         echo "  Billable / trial / paid sources"
         echo "  $(show_weather_choice "$sel_openweather")  9) OpenWeather       free allowance · 1,000/day then BILLABLE · 8-day"
         echo "  $(show_weather_choice "$sel_googleweather") 10) Google Weather    PAID · Google Maps Platform billing · no normal free tier · 10-day"
         echo "  $(show_weather_choice "$sel_accuweather") 11) AccuWeather       14-DAY TRIAL then paid · 500/day during trial · 5-day"
         echo "  $(show_weather_choice "$sel_custom") 12) Custom Open-Meteo  compatible endpoint / API key"
         echo
         echo "  Type a number to toggle it. Use R=review, A=all (asks before paid), N=none, S=save, Q=cancel."
         read -rp "  Selection action [S]: " ans; ans="${ans:-S}"
         case "$ans" in
           [sS]) break;;
           [rR]) weather_review;;
           [qQ]) warn "weather source changes cancelled"; WEATHER_PROVIDERS="$existing"; WEATHER_KEYS_JS="{}"; return 0;;
           [aA])
             read -rp "  Enable every listed provider, including paid/trial sources? [y/N] " enable_all
             if [ "$enable_all" = "y" ] || [ "$enable_all" = "Y" ]; then
               sel_openmeteo=1; sel_nws=1; sel_weatherapi=1; sel_openweather=1; sel_googleweather=1; sel_tomorrow=1; sel_visualcrossing=1; sel_weatherbit=1; sel_pirateweather=1; sel_accuweather=1; sel_xweather=1
               echo "  All listed provider sources are selected. Custom Open-Meteo remains opt-in."
             else
               echo "  Left provider selections unchanged."
             fi
             ;;
           [nN]) sel_openmeteo=0; sel_nws=0; sel_weatherapi=0; sel_openweather=0; sel_googleweather=0; sel_tomorrow=0; sel_visualcrossing=0; sel_weatherbit=0; sel_pirateweather=0; sel_accuweather=0; sel_xweather=0; sel_custom=0;;
           1|2|3|4|5|6|7|8|9|10|11|12) toggle_weather_choice "$ans";;
           *) warn "choose a number, R, A, N, S, or Q";;
         esac
       done;;
    5|q|Q) warn "weather source changes cancelled"; WEATHER_PROVIDERS="$existing"; WEATHER_KEYS_JS="{}"; return 0;;
    *) warn "unknown choice; keeping current selection";;
  esac

  build_weather_list
  echo
  echo "Weather source review"
  echo "  Selected providers: ${WEATHER_PROVIDERS}"
  echo "  Keyed sources without saved keys will be toggled off after the key prompt."
  echo "  Paid/billable sources stay clearly labeled in Dashboard Control source health."
  read -rp "  Continue with this weather setup? [Y/n] " okwx
  if [ "$okwx" = "n" ] || [ "$okwx" = "N" ]; then
    warn "weather source changes cancelled"; WEATHER_PROVIDERS="$existing"; WEATHER_KEYS_JS="{}"; return 0
  fi

  echo
  echo "Weather API keys — press Enter to keep a saved key or leave blank."
  if [ "$sel_weatherapi" = "1" ]; then read -rp "  WeatherAPI.com key [$([ -n "$key_weatherapi" ] && echo saved || echo blank)]: " ans; store_optional_trimmed key_weatherapi "$ans"; fi
  if [ "$sel_openweather" = "1" ]; then read -rp "  OpenWeather key [$([ -n "$key_openweather" ] && echo saved || echo blank)]: " ans; store_optional_trimmed key_openweather "$ans"; fi
  if [ "$sel_googleweather" = "1" ]; then read -rp "  Google Weather API key [$([ -n "$key_googleweather" ] && echo saved || echo blank)]: " ans; store_optional_trimmed key_googleweather "$ans"; fi
  if [ "$sel_tomorrow" = "1" ]; then read -rp "  Tomorrow.io key [$([ -n "$key_tomorrow" ] && echo saved || echo blank)]: " ans; store_optional_trimmed key_tomorrow "$ans"; fi
  if [ "$sel_visualcrossing" = "1" ]; then read -rp "  Visual Crossing key [$([ -n "$key_visualcrossing" ] && echo saved || echo blank)]: " ans; store_optional_trimmed key_visualcrossing "$ans"; fi
  if [ "$sel_weatherbit" = "1" ]; then read -rp "  Weatherbit key [$([ -n "$key_weatherbit" ] && echo saved || echo blank)]: " ans; store_optional_trimmed key_weatherbit "$ans"; fi
  if [ "$sel_pirateweather" = "1" ]; then read -rp "  Pirate Weather key [$([ -n "$key_pirateweather" ] && echo saved || echo blank)]: " ans; store_optional_trimmed key_pirateweather "$ans"; fi
  if [ "$sel_accuweather" = "1" ]; then read -rp "  AccuWeather key [$([ -n "$key_accuweather" ] && echo saved || echo blank)]: " ans; store_optional_trimmed key_accuweather "$ans"; fi
  if [ "$sel_xweather" = "1" ]; then read -rp "  Xweather client_id:client_secret [$([ -n "$key_xweather" ] && echo saved || echo blank)]: " ans; store_optional_trimmed key_xweather "$ans"; fi
  if [ "$sel_custom" = "1" ]; then
    read -rp "  Weather API base [https://api.open-meteo.com]: " WXAPI
    WXAPI="$(trim_input "$WXAPI")"
    read -rp "  Air-quality base [https://air-quality-api.open-meteo.com]: " AQAPI
    AQAPI="$(trim_input "$AQAPI")"
    read -rp "  Open-Meteo-compatible API key [$([ -n "$key_openmeteocustom" ] && echo saved || echo blank)]: " ans; store_optional_trimmed key_openmeteocustom "$ans"
    APIKEY="$key_openmeteocustom"
  fi
  auto_disable_keyless_weather_sources
  build_weather_list
  WEATHER_KEYS_JS="{}"
  write_weather_env
  echo
  echo "Final weather source review"
  echo "  Providers: ${WEATHER_PROVIDERS}"
  echo "  Source health in Dashboard Control will show active, stale, missing-key, cooldown, unsupported, and budget states."
  ok "weather sources: ${WEATHER_PROVIDERS}"
}

write_weather_sources_only(){
  mkdir -p "$CONFIG_DIR"
  installer_cli --installer-config-local --file "$CONFIG_DIR/config.local.js" --mode weather \
    --providers "${WEATHER_PROVIDERS:-openmeteo}" --wx-api "${WXAPI:-}" --aq-api "${AQAPI:-}" || {
      warn "Could not save weather sources. Run ~/install.sh again to retry this step."
      return 1
    }
  write_weather_env
  ok "weather sources saved"
}

write_weather_display_only(){
  mkdir -p "$CONFIG_DIR"
  installer_cli --installer-config-local --file "$CONFIG_DIR/config.local.js" --mode display \
    --temp-unit "${TEMPU:-fahrenheit}" --wind-unit "${WINDU:-mph}" \
    --weather-days "${WEATHERDAYS:-14}" --refresh-wx "${WXREFRESH:-30}" \
    --alert-refresh "${ALERTREFRESH:-5}" --alert-min "${ALERTMIN:-moderate}" || {
      warn "Could not save weather display settings. Run ~/install.sh again to retry this step."
      return 1
    }
  ok "weather display settings saved"
}

ensure_message_data_files(){
  mkdir -p "$CONFIG_DIR"
  installer_cli --installer-message-files --dir "$CONFIG_DIR" || {
    warn "Could not prepare message data files. Run ~/install.sh again to retry this step."
    return 1
  }
  if [ -f "$CACHE_DIR/demo-mode.json" ]; then
    : # Demo Mode owns message-cache.json; do not replace sample messages.
  elif [ -x "$BIN_DIR/dashboard-control-server" ]; then
    "$BIN_DIR/dashboard-control-server" --update-message-feeds --no-network >/dev/null 2>&1 || true
  fi
  ok "Messages data files present"
}

prompt_message_sources(){
  local server list_file choice joined id label adult enabled idx
  local -a message_ids=() message_labels=() message_adult=()
  declare -A message_enabled=()
  say "Message sources"
  ensure_message_data_files
  echo "Choose categories for rotating quotes, jokes, facts, and prompts."
  echo "The dashboard reads a local cache; online sources refresh in the background."
  if installer_demo_prompt_defaults_active; then
    demo_default_note "keeping Demo Mode sample message rotation and skipping message-source prompts"
    return 0
  fi
  read -rp "  Configure optional message API keys? [y/N] " MKEYS
  if [ "$MKEYS" = "y" ] || [ "$MKEYS" = "Y" ]; then
    prompt_message_api_keys
  fi
  server="$BIN_DIR/dashboard-control-server"
  if [ ! -x "$server" ]; then
    mark_install_step_failed "Message-source configuration" "Go helper is missing; choose Update the app, then Message sources again"
    return 1
  fi
  list_file="$(mktemp "${TMPDIR:-/tmp}/dash-go-message-sources.XXXXXX")" || {
    mark_install_step_failed "Message-source configuration" "could not create a temporary category list"
    return 1
  }
  if ! "$server" --message-sources --list > "$list_file"; then
    rm -f "$list_file"
    mark_install_step_failed "Message-source configuration" "the Go catalog helper failed"
    return 1
  fi
  while IFS=$'\t' read -r id label adult enabled; do
    [ -n "$id" ] || continue
    message_ids+=("$id")
    message_labels+=("$label")
    message_adult+=("$adult")
    [ "$enabled" = "true" ] && message_enabled["$id"]=1
  done < "$list_file"
  rm -f "$list_file"
  if [ "${#message_ids[@]}" -eq 0 ]; then
    mark_install_step_failed "Message-source configuration" "the Go catalog returned no categories"
    return 1
  fi
  while :; do
    echo
    echo "Message categories — toggle one number at a time"
    for idx in "${!message_ids[@]}"; do
      id="${message_ids[$idx]}"; label="${message_labels[$idx]}"; adult="${message_adult[$idx]}"
      if [ "${message_enabled[$id]:-0}" = 1 ]; then mark='[x]'; else mark='[ ]'; fi
      [ "$adult" = true ] && label="$label (adult)"
      printf '  %s %d) %s\n' "$mark" "$((idx+1))" "$label"
    done
    echo "  s) Save   a) All safe   n) None   q) Keep current"
    read -rp "Choice: " choice || { echo "message-source selection unchanged (no input available)"; break; }
    choice="$(printf '%s' "$choice" | tr '[:upper:]' '[:lower:]')"
    case "$choice" in
      q|'') echo "message-source selection unchanged"; break ;;
      a)
        for idx in "${!message_ids[@]}"; do
          [ "${message_adult[$idx]}" = true ] || message_enabled["${message_ids[$idx]}"]=1
        done
        ;;
      n) message_enabled=() ;;
      s)
        joined=""
        for idx in "${!message_ids[@]}"; do
          id="${message_ids[$idx]}"
          [ "${message_enabled[$id]:-0}" = 1 ] || continue
          joined="${joined}${joined:+,}${id}"
        done
        if "$server" --message-sources --set "$joined"; then
          ok "message-source preferences saved"
        else
          mark_install_step_failed "Message-source configuration" "validated preferences were not written"
          return 1
        fi
        break
        ;;
      *)
        if [[ "$choice" =~ ^[0-9]+$ ]] && [ "$choice" -ge 1 ] && [ "$choice" -le "${#message_ids[@]}" ]; then
          idx=$((choice-1)); id="${message_ids[$idx]}"
          if [ "${message_enabled[$id]:-0}" = 1 ]; then unset 'message_enabled[$id]'; else message_enabled["$id"]=1; fi
        else
          warn "Enter one number, s, a, n, or q."
        fi
        ;;
    esac
  done
  read -rp "  Refresh message cache now? [y/N] " MNET || MNET=""
  if [ "$MNET" = "y" ] || [ "$MNET" = "Y" ]; then
    if [ -x "$BIN_DIR/dashboard-lowprio.sh" ]; then
      if "$BIN_DIR/dashboard-lowprio.sh" "$server" --update-message-feeds >/dev/null 2>&1; then
        ok "message cache refreshed"
      else
        mark_install_step_failed "Message cache refresh" "it will retry on the scheduled background refresh"
      fi
    elif "$server" --update-message-feeds >/dev/null 2>&1; then
      ok "message cache refreshed"
    else
      mark_install_step_failed "Message cache refresh" "it will retry on the scheduled background refresh"
    fi
  fi
}

run_customization(){
while :; do
say "Customization"
echo "Weather location — enter a place name to look up, or coordinates directly."

# Geocode a place name via Open-Meteo's free geocoding API (no key). Sets
# globals LAT, LON, LOCNAME on success; returns 1 if nothing usable found.
geocode(){
  local q="$1" results rc errfile i=1 la lo lab pick
  q="${q%%,*}"; q="$(trim_input "$q")"
  [ -n "$q" ] || return 1
  errfile="$(mktemp "${TMPDIR:-/tmp}/dash-go-geocode.XXXXXX")" || return 1
  results="$(installer_cli --geocode --name "$q" 2>"$errfile")"; rc=$?
  if [ "$rc" -ne 0 ]; then
    warn "Dash-Go could not reach the city lookup service. Check the network and try again, or enter latitude and longitude."
    rm -f "$errfile"; return 1
  fi
  rm -f "$errfile"
  if [ -z "$results" ]; then
    warn "Dash-Go could not find that city. Check the spelling, try a nearby city, or enter latitude and longitude."
    return 1
  fi
  echo "  Matches:"
  local -a GLAT=() GLON=() GLAB=()
  while IFS='|' read -r la lo lab; do
    [ -n "$la" ] || continue
    printf '    %d) %s  (%s, %s)\n' "$i" "$lab" "$la" "$lo"
    GLAT[$i]="$la"; GLON[$i]="$lo"; GLAB[$i]="$lab"; i=$((i+1))
  done <<EOFGEORESULTS
$results
EOFGEORESULTS
  [ "$i" -gt 1 ] || return 1
  while :; do
    read -rp "  Choose a match [1-$((i-1)), b=back]: " pick
    pick="$(trim_input "$pick")"; pick="${pick%)}"
    case "$pick" in
      b|B|0) return 2 ;;
      ''|*[!0-9]*) warn "Choose one listed number, or b to search again."; continue ;;
    esac
    if [ -n "${GLAT[$pick]:-}" ]; then
      LAT="${GLAT[$pick]}"; LON="${GLON[$pick]}"; LOCNAME="${GLAB[$pick]}"
      return 0
    fi
    warn "Choose one listed number, or b to search again."
  done
}

# --- Device performance profile ----------------------------------------
# Auto-detect the board, recommend a preset, and let the user confirm or
# override it. These are only INSTALL DEFAULTS written to config.local.js;
# the on-screen control panel can still change user-facing toggles later.
# Lower-power profiles protect scroll/touch smoothness. Higher-performance
# profiles spend the extra headroom on visibility: seconds, denser calendar
# cells, longer agenda/weather lists, longer scroll window, and smoother/faster
# compliment rotation.
# Device-tree model files only exist on Raspberry Pi / ARM boards. Check for
# readability before redirecting from them; otherwise bash prints a noisy
# "No such file or directory" before stderr redirection can suppress it.
read_pi_model(){
  local f
  for f in /proc/device-tree/model /sys/firmware/devicetree/base/model; do
    if [ -r "$f" ]; then
      tr -d '\0' < "$f" 2>/dev/null
      return 0
    fi
  done
  # Some non-DT systems expose useful model strings only through /proc/cpuinfo.
  awk -F: '/^(Model|Hardware)[[:space:]]*:/{sub(/^[[:space:]]+/,"",$2); print $2; exit}' /proc/cpuinfo 2>/dev/null || true
}
PIMODEL=""
[ "$IS_PI" = "1" ] && PIMODEL="$DEVICE_MODEL"
ARCH="$DASH_ARCH"
GUESS="$(bootstrap_classify_device_profile)"

set_profile_defaults(){
  case "$1" in
    lite)
      PROFILE="lite"; PROFILE_LABEL="Low-memory / Pi Zero-class"
      SHOWSECS=false; COMPSEC=18; COMPFADEMS=650
      MAXEVENTS=6; AGENDADAYS=10; WEATHERDAYS=10; WEEKSABOVE=1; WEEKSBELOW=8; ROWHEIGHT=205; SIDEBARWIDTH=370
      CALREFRESH=15; WXREFRESH=45; ALERTREFRESH=10; PIXELSHIFT=1; SHOWMAPS=false; SHOWINTERACTIVEMAPS=false; LAYOUTPROFILE=auto;;
    enhanced)
      PROFILE="enhanced"; PROFILE_LABEL="2.2 GB+ capable device"
      SHOWSECS=true; COMPSEC=25; COMPFADEMS=450
      MAXEVENTS=10; AGENDADAYS=18; WEATHERDAYS=16; WEEKSABOVE=3; WEEKSBELOW=12; ROWHEIGHT=218; SIDEBARWIDTH=400
      CALREFRESH=7; WXREFRESH=30; ALERTREFRESH=5; PIXELSHIFT=2; SHOWMAPS=true; SHOWINTERACTIVEMAPS=true; LAYOUTPROFILE=auto;;
    *)
      PROFILE="balanced"; PROFILE_LABEL="2 GB class / balanced"
      SHOWSECS=true; COMPSEC=15; COMPFADEMS=600
      MAXEVENTS=8; AGENDADAYS=14; WEATHERDAYS=14; WEEKSABOVE=2; WEEKSBELOW=10; ROWHEIGHT=210; SIDEBARWIDTH=380
      CALREFRESH=10; WXREFRESH=30; ALERTREFRESH=5; PIXELSHIFT=2; SHOWMAPS=true; SHOWINTERACTIVEMAPS=false; LAYOUTPROFILE=auto;;
  esac
}

first_day_label(){
  case "${FIRSTDAY:-0}" in
    6) echo "Saturday (weekend start)";;
    1) echo "Monday (weekday start)";;
    *) echo "Sunday (normal view)";;
  esac
}

profile_summary(){
  printf '     %-15s %s\n' "Clock seconds:" "$([ "$SHOWSECS" = true ] && echo shown || echo hidden)"
  printf '     %-15s every %ss, %sms fade\n' "Compliments:" "$COMPSEC" "$COMPFADEMS"
  printf '     %-15s %s events/day, %s agenda days, %s weather days\n' "Visibility:" "$MAXEVENTS" "$AGENDADAYS" "$WEATHERDAYS"
  printf '     %-15s %s profile, %s past weeks, %s future weeks, %spx rows, %spx sidebar\n' "Layout:" "$LAYOUTPROFILE" "$WEEKSABOVE" "$WEEKSBELOW" "$ROWHEIGHT" "$SIDEBARWIDTH"
  printf '     %-15s calendars %sm, weather %sm, alerts %sm\n' "Refresh:" "$CALREFRESH" "$WXREFRESH" "$ALERTREFRESH"
  printf '     %-15s %s\n' "Event maps:" "$([ "$SHOWMAPS" = true ] && echo shown || echo hidden)"
  printf '     %-15s %s\n' "Interactive map:" "$([ "${SHOWINTERACTIVEMAPS:-false}" = true ] && echo enabled || echo disabled)"
}
case "$GUESS" in lite) GDEF=1;; balanced) GDEF=2;; enhanced|maximum) GDEF=3;; esac
set_profile_defaults "$GUESS"
FIRSTDAY="${FIRSTDAY:-0}"
echo
echo "== Performance / visibility profile"
echo "   detected platform: $PLATFORM_LABEL"
echo "   recommended: $PROFILE_LABEL"
echo
echo "  1) Lite      Pi Zero / very-low-memory — protects scroll/touch smoothness"
echo "  2) Balanced  ~2 GB / modest device     — normal appliance defaults"
echo "  3) Enhanced  2.2+ GB capable device    — richer display + maps"
echo
echo "   Recommended preset details:"
profile_summary
while :; do
  read -rp "  Choose profile [1-3, Enter=$GDEF recommended, q=cancel]: " PROFSEL || return 2
  PROFSEL="$(trim_input "${PROFSEL:-$GDEF}")"
  case "$PROFSEL" in
    q|Q) warn "Customization cancelled before settings were written. Run ~/install.sh again when you are ready."; return 2;;
    1) set_profile_defaults lite; break;;
    2) set_profile_defaults balanced; break;;
    3) set_profile_defaults enhanced; break;;
    *) warn "Choose 1, 2, or 3. Your current setup answers are still waiting.";;
  esac
done
echo
echo "   Selected preset: $PROFILE_LABEL"
profile_summary
read -rp "  Fine-tune these defaults now? [y/N] " TUNEPROF
if [ "$TUNEPROF" = "y" ] || [ "$TUNEPROF" = "Y" ]; then
  read -rp "    Show clock seconds? [$( [ "$SHOWSECS" = true ] && echo Y/n || echo y/N )]: " ans
  case "$ans" in y|Y) SHOWSECS=true;; n|N) SHOWSECS=false;; esac
  read -rp "    Compliment rotation seconds [$COMPSEC]: " ans; [ -n "$ans" ] && printf '%s' "$ans" | grep -qE '^[0-9]+$' && COMPSEC="$ans"
  read -rp "    Compliment fade milliseconds [$COMPFADEMS]: " ans; [ -n "$ans" ] && printf '%s' "$ans" | grep -qE '^[0-9]+$' && COMPFADEMS="$ans"
  read -rp "    Events visible per day cell [$MAXEVENTS]: " ans; [ -n "$ans" ] && printf '%s' "$ans" | grep -qE '^[0-9]+$' && MAXEVENTS="$ans"
  read -rp "    Agenda days [$AGENDADAYS]: " ans; [ -n "$ans" ] && printf '%s' "$ans" | grep -qE '^[0-9]+$' && AGENDADAYS="$ans"
  read -rp "    Weather days [$WEATHERDAYS]: " ans; [ -n "$ans" ] && printf '%s' "$ans" | grep -qE '^[0-9]+$' && WEATHERDAYS="$ans"
  echo "    Screen layout profile: auto, compact, standard, large, xlarge, portrait"
  read -rp "    Screen layout [$LAYOUTPROFILE]: " ans; [ -n "$ans" ] && LAYOUTPROFILE="$ans"
  read -rp "    Past calendar weeks to keep available [$WEEKSABOVE]: " ans; [ -n "$ans" ] && printf '%s' "$ans" | grep -qE '^[0-9]+$' && WEEKSABOVE="$ans"
  read -rp "    Future calendar weeks to render [$WEEKSBELOW]: " ans; [ -n "$ans" ] && printf '%s' "$ans" | grep -qE '^[0-9]+$' && WEEKSBELOW="$ans"
  read -rp "    Show static maps in event popups? [$( [ "$SHOWMAPS" = true ] && echo Y/n || echo y/N )]: " ans
  case "$ans" in y|Y) SHOWMAPS=true;; n|N) SHOWMAPS=false;; esac
  read -rp "    Enable full-screen interactive Google Maps on location tap? [$( [ "${SHOWINTERACTIVEMAPS:-false}" = true ] && echo Y/n || echo y/N )]: " ans
  case "$ans" in y|Y) SHOWINTERACTIVEMAPS=true;; n|N) SHOWINTERACTIVEMAPS=false;; esac
fi

echo "Calendar start day:"
echo "  1) Sunday normal view"
echo "  2) Start Saturday (weekend start)"
echo "  3) Start Monday (weekday start)"
case "${FIRSTDAY:-0}" in 6) fddef=2;; 1) fddef=3;; *) fddef=1;; esac
read -rp "  Choose [1-3, Enter=$fddef]: " ans; ans="${ans:-$fddef}"
case "$ans" in 2) FIRSTDAY=6;; 3) FIRSTDAY=1;; *) FIRSTDAY=0;; esac
ok "calendar starts: $(first_day_label)"

ok "profile: $PROFILE ($PROFILE_LABEL)"

# Non-Pi hardware reports temperature through hwmon, which often needs the
# lm-sensors kernel-module detection run once. Pi boards don't need this.
if [ -z "$PIMODEL" ] && [ "$PROFILE" != "lite" ]; then
  if ! ls /sys/class/hwmon/hwmon*/temp1_input >/dev/null 2>&1 \
     && ! [ -e /sys/class/thermal/thermal_zone0/temp ]; then
    if command -v apt-get >/dev/null 2>&1; then
      read -rp "  No temperature sensors detected — install lm-sensors to enable the CPU-temp tile? [Y/n] " SENSOK
      if [ "$SENSOK" != "n" ] && [ "$SENSOK" != "N" ]; then
        apt_managed install -y lm-sensors >/dev/null 2>&1 \
          && $SUDO sensors-detect --auto >/dev/null 2>&1 || true
        ok "lm-sensors installed (CPU temp shows '—' on VMs that expose no sensors at all)"
      fi
    else
      warn "no apt-get — install your distro's lm-sensors package manually for CPU temp"
    fi
  fi
fi

# --- Weather data source providers are their own task -----------------------
if [ "$DO_WEATHER" = "1" ]; then
  prompt_weather_sources
fi
if [ "$DO_RADAR" = "1" ]; then
  prompt_radar_provider
fi

TEMPU="${TEMPU:-$(read_config_scalar tempUnit fahrenheit)}"
WINDU="${WINDU:-$(read_config_scalar windUnit mph)}"
CURRENT_LAT="$(read_config_scalar lat "")"
CURRENT_LON="$(read_config_scalar lon "")"
CURRENT_LOCNAME="$(read_config_string locationName "")"
CURRENT_LOCATION_READY=0
# Do not offer untouched fresh-install 0,0 defaults as a meaningful prior
# location. A saved name or a nonzero coordinate pair is a real reconfigure
# value that the user can keep with Enter.
if valid_lat "$CURRENT_LAT" && valid_lon "$CURRENT_LON" \
   && { [ -n "$CURRENT_LOCNAME" ] || ! awk -v lat="$CURRENT_LAT" -v lon="$CURRENT_LON" 'BEGIN { exit ((lat+0)==0 && (lon+0)==0) }'; }; then
  CURRENT_LOCATION_READY=1
fi
LAT=""; LON=""; LOCNAME=""
while true; do
  if [ "$CURRENT_LOCATION_READY" = "1" ]; then
    echo "  0) Keep current location (${CURRENT_LOCNAME:-$CURRENT_LAT, $CURRENT_LON})"
  fi
  echo "  1) Search by CITY NAME (e.g. 'Chicago' — just the city,"
  echo "     no state or ZIP; you'll pick from a list to disambiguate)"
  echo "  2) Enter latitude/longitude manually"
  if [ "$CURRENT_LOCATION_READY" = "1" ]; then
    read -rp "  Choose [0/1/2, Enter=0]: " locmode
    locmode="$(trim_input "${locmode:-0}")"
  else
    read -rp "  Choose [1/2, Enter=1]: " locmode
    locmode="$(trim_input "${locmode:-1}")"
  fi
  case "$locmode" in
    q|Q) warn "Customization cancelled before settings were written. Run ~/install.sh again when you are ready."; return 2;;
    b|B) warn "This is the first location step. Choose a location method, or q to cancel."; continue;;
    0)
      if [ "$CURRENT_LOCATION_READY" != "1" ]; then
        warn "  No current location is available. Choose 1 or 2."
        continue
      fi
      LAT="$CURRENT_LAT"; LON="$CURRENT_LON"; LOCNAME="$CURRENT_LOCNAME"
      ok "  Keeping: ${LOCNAME:-($LAT, $LON)}"
      break
      ;;
    1)
      read -rp "  City name [b=back, q=cancel]: " PLACE
      PLACE="$(trim_input "$PLACE")"
      case "$PLACE" in q|Q) warn "Customization cancelled before settings were written. Run ~/install.sh again when you are ready."; return 2;; b|B) continue;; esac
      geocode "$PLACE"
      geocode_status=$?
      if [ "$geocode_status" = "0" ]; then
        ok "  Using: $LOCNAME ($LAT, $LON)"
        break
      fi
      if [ "$geocode_status" = "2" ]; then
        echo "  Search again selected."
      else
        warn "  No match found (or lookup failed). Try again or enter coordinates."
      fi
      ;;
    2)
      while true; do
        read -rp "  Latitude:  " LAT
        valid_lat "$LAT" && break; warn "    Latitude must be a number between -90 and 90."
      done
      while true; do
        read -rp "  Longitude: " LON
        valid_lon "$LON" && break; warn "    Longitude must be a number between -180 and 180."
      done
      read -rp "  Location name (for display, optional): " LOCNAME
      break
      ;;
    *) warn "  Choose 0, 1, or 2.";;
  esac
done
echo "Units:"
echo "  1) Fahrenheit / mph  (US)"
echo "  2) Celsius / km/h"
while :; do
  read -rp "  Choose [1/2, Enter=current ${TEMPU}/${WINDU}, q=cancel]: " U || return 2
  U="$(trim_input "$U")"
  case "$U" in
    q|Q) warn "Customization cancelled before settings were written. Run ~/install.sh again when you are ready."; return 2;;
    2) TEMPU="celsius"; WINDU="kmh"; break;;
    1) TEMPU="fahrenheit"; WINDU="mph"; break;;
    "") break;;
    *) warn "Choose 1 or 2, or press Enter to keep ${TEMPU}/${WINDU}.";;
  esac
done
if [ "$DO_WEATHER_DISPLAY" = "1" ]; then
  prompt_weather_display 1
fi

echo
echo "Theme (color scheme) — choose from built-in palettes. Change anytime later with:"
echo "  $BIN_DIR/set-theme.sh        (or choose Theme from this installer)"
THEME_CATALOG="$DASH/themes.list"
if [ ! -r "$THEME_CATALOG" ]; then
  warn "theme catalog not found in $DASH. Run Update the app first."
  exit 1
fi
mapfile -t THEME_OPTIONS < <(sed -e 's/[[:space:]]*#.*$//' -e '/^[[:space:]]*$/d' "$THEME_CATALOG")
[ "${#THEME_OPTIONS[@]}" -gt 0 ] || { warn "theme catalog is empty: $THEME_CATALOG"; exit 1; }
for t in "${THEME_OPTIONS[@]}"; do
  [[ "$t" =~ ^[a-z][a-z0-9]*$ ]] || { warn "invalid theme catalog entry: $t"; exit 1; }
done
i=1
for t in "${THEME_OPTIONS[@]}"; do
  printf '  %2d) %-13s' "$i" "$t"
  [ $((i % 4)) -eq 0 ] && echo
  i=$((i+1))
done
NTHEMES=$((i-1))
[ $(( NTHEMES % 4 )) -ne 0 ] && echo
while :; do
  read -rp "  Choose [1-$NTHEMES, Enter=1 basic, q=cancel]: " THM || return 2
  THM="$(trim_input "${THM:-1}")"
  case "$THM" in q|Q) warn "Customization cancelled before settings were written. Run ~/install.sh again when you are ready."; return 2;; esac
  if printf '%s' "$THM" | grep -qE '^[0-9]+$'; then
    THEME="${THEME_OPTIONS[$((THM-1))]:-}"
  else
    THEME=""
  fi
  [ -n "$THEME" ] && break
  warn "Choose a listed theme number."
done

echo
echo "Primary user (for compliments)."
read -rp "  Your name [blank=skip, q=cancel]: " PNAME || return 2
PNAME="$(trim_input "$PNAME")"
case "$PNAME" in q|Q) warn "Customization cancelled before settings were written. Run ~/install.sh again when you are ready."; return 2;; esac
while true; do
  read -rp "  Your birthday (MM-DD, e.g. 01-25, blank=skip, q=cancel): " PBDAY || return 2
  PBDAY="$(trim_input "$PBDAY")"
  case "$PBDAY" in q|Q) warn "Customization cancelled before settings were written. Run ~/install.sh again when you are ready."; return 2;; esac
  [ -z "$PBDAY" ] && break
  valid_mmdd "$PBDAY" && break; warn "    Use MM-DD format, e.g. 01-25."
done

# Collect additional people + birthdays.
# Names go inside a JSON string in config.local.js — escape backslashes and
# double quotes so a name like  Sarah "Sunny" O'Neil  can't break the file.
BDAY_JSON=""
add_bday(){ # name date
  local n="$1" d="$2"
  [ -z "$n" ] && return
  [ -z "$d" ] && return
  BDAY_JSON="${BDAY_JSON:+$BDAY_JSON,}{\"name\":\"$(json_escape "$n")\",\"date\":\"$d\"}"
}
add_bday "$PNAME" "$PBDAY"
echo
echo "Add other noteworthy people + birthdays (blank name when done)."
while true; do
  read -rp "  Name (blank=finish, q=cancel): " ONAME || return 2
  ONAME="$(trim_input "$ONAME")"
  case "$ONAME" in q|Q) warn "Customization cancelled before settings were written. Run ~/install.sh again when you are ready."; return 2;; esac
  [ -z "$ONAME" ] && break
  while true; do
    read -rp "  $ONAME's birthday (MM-DD, blank=skip, q=cancel): " ODATE || return 2
    ODATE="$(trim_input "$ODATE")"
    case "$ODATE" in q|Q) warn "Customization cancelled before settings were written. Run ~/install.sh again when you are ready."; return 2;; esac
    [ -z "$ODATE" ] && break
    valid_mmdd "$ODATE" && break; warn "    Use MM-DD format, e.g. 10-04."
  done
  if [ -z "$ODATE" ]; then
    warn "  Skipped $ONAME — no birthday date given."
    continue
  fi
  add_bday "$ONAME" "$ODATE"
done

# ---------------------------------------------------------------------
# Show a summary and let the user re-run customization if something's off.
say "Review your settings"
echo "  Location : ${LOCNAME:-($LAT, $LON)}"
echo "  Coords   : $LAT, $LON"
echo "  Units    : $TEMPU / $WINDU"
echo "  Theme    : ${THEME:-basic}"
echo "  Profile  : ${PROFILE:-balanced} (${PROFILE_LABEL:-})"
echo "  Display  : seconds=${SHOWSECS:-true}, compliments=${COMPSEC:-18}s, events/day=${MAXEVENTS:-8}, agenda=${AGENDADAYS:-14}d, weather=${WEATHERDAYS:-14}d, past=${WEEKSABOVE:-2}w, future=${WEEKSBELOW:-10}w, start=$(first_day_label), layout=${LAYOUTPROFILE:-auto}"
echo "  Weather  : sources=${WEATHER_PROVIDERS:-openmeteo} (keys in ~/.dashboard-weather.env)"
echo "  Radar    : ${RADAR_PROVIDER:-rainviewer} (keys in ~/.dashboard-radar.env)"
echo "  Messages : sources/config in ~/dashboard/config; optional keys in ~/.dashboard-message.env"
echo "  Birthdays: ${BDAY_JSON:-none}"
read -rp "Look right? [Y/n] " confirm
if [ "$confirm" = "n" ] || [ "$confirm" = "N" ]; then
  warn "Settings were not written. Let's walk through customization again."
  continue
fi

say "Writing per-device settings (config.local.js)"
{
  echo "// Generated by install.sh on $(date). Re-run the installer to change."
  echo "window.DASHBOARD_LOCAL = {"
  echo "  lat: ${LAT:-0},"
  echo "  lon: ${LON:-0},"
  echo "  tempUnit: \"$TEMPU\","
  echo "  windUnit: \"$WINDU\","
  echo "  theme: \"${THEME:-basic}\","
  echo "  profile: \"${PROFILE:-balanced}\","
  echo "  showSeconds: ${SHOWSECS:-true},"
  echo "  complimentSeconds: ${COMPSEC:-15},"
  echo "  complimentFadeMs: ${COMPFADEMS:-600},"
  echo "  maxEventsPerCell: ${MAXEVENTS:-8},"
  echo "  agendaDays: ${AGENDADAYS:-14},"
  echo "  weatherDays: ${WEATHERDAYS:-14},"
  echo "  layoutProfile: \"${LAYOUTPROFILE:-auto}\","
  echo "  weeksAbove: ${WEEKSABOVE:-2},"
  echo "  weeksBelow: ${WEEKSBELOW:-10},"
  echo "  firstDayOfWeek: ${FIRSTDAY:-0},"
  echo "  rowHeight: ${ROWHEIGHT:-210},"
  echo "  sidebarWidth: ${SIDEBARWIDTH:-380},"
  echo "  refreshCalMinutes: ${CALREFRESH:-10},"
  echo "  refreshWxMinutes: ${WXREFRESH:-30},"
  echo "  showEventMaps: ${SHOWMAPS:-true},"
  echo "  showInteractiveMaps: ${SHOWINTERACTIVEMAPS:-false},"
  echo "  pixelShift: ${PIXELSHIFT:-2},"
  echo "  weatherAlerts: { enabled: true, refreshMinutes: ${ALERTREFRESH:-5}, minSeverity: \"moderate\" },"
  if [ -n "${WEATHER_PROVIDERS:-}" ]; then
    providers_json="$(json_string_array_words "$WEATHER_PROVIDERS")"
    echo "  weatherProviders: ${providers_json},"
  fi
  [ -n "${WXAPI:-}" ]  && echo "  wxApi: \"${WXAPI%/}\","
  [ -n "${AQAPI:-}" ]  && echo "  aqApi: \"${AQAPI%/}\","
  [ -n "$LOCNAME" ] && echo "  locationName: \"$(json_escape "$LOCNAME")\","
  echo "  birthdays: [ $BDAY_JSON ]"
  echo "};"
} > "$CONFIG_DIR/config.local.js"
ok "wrote config/config.local.js"
return 0
done
}

# Express setup deliberately asks only for a usable weather location. It writes
# a small, valid local override and leaves every richer preference at the
# application defaults, where Dashboard Control can change it later.
run_express_customization(){
  local place results pick count lat lon label profile tmp rc geocode_err temp_unit wind_unit country
  say "Express dashboard setup"
  echo "Only one detail is needed now: where this dashboard lives."
  echo "You can change the theme, units, calendars, weather sources, and everything else later in Dashboard Control."

  while :; do
    echo
    echo "  1) Search by city or town"
    echo "  2) Enter latitude and longitude"
    echo "  b) Back to the installer menu"
    read -rp "  Choose [1/2, Enter=1]: " pick
    pick="${pick:-1}"
    case "$pick" in
      b|B)
        warn "Express setup cancelled before settings were written. Run ~/install.sh again when you are ready."
        return 2
        ;;
      1)
        read -rp "  City or town (for example, Chicago): " place
        place="$(trim_input "$place")"
        case "$place" in
          '' ) warn "Enter a city or choose b to leave Express setup."; continue;;
          b|B|q|Q) warn "Express setup cancelled before settings were written. Run ~/install.sh again when you are ready."; return 2;;
        esac
        local geocode_err
        geocode_err="$(mktemp "${TMPDIR:-/tmp}/dash-go-express-geocode.XXXXXX")" || return 1
        results="$(installer_cli --geocode --name "$place" 2>"$geocode_err")"; rc=$?
        if [ "$rc" -ne 0 ]; then
          warn "Dash-Go could not reach the city lookup service. Check the network and try again, or choose latitude/longitude."
          rm -f "$geocode_err"; continue
        fi
        rm -f "$geocode_err"
        if [ -z "$results" ]; then
          warn "Dash-Go could not find that city. Check the spelling, try a nearby city, or choose latitude/longitude."
          continue
        fi
        echo "  Matches:"
        count=0
        while IFS='|' read -r lat lon label; do
          [ -n "$lat" ] || continue
          count=$((count + 1))
          printf '    %d) %s (%s, %s)\n' "$count" "$label" "$lat" "$lon"
        done <<EOFEXPRESSMATCHES
$results
EOFEXPRESSMATCHES
        while :; do
          read -rp "  Choose a match [1-$count, b=back]: " pick
          pick="$(trim_input "$pick")"; pick="${pick%)}"
          case "$pick" in
            b|B|0) break;;
            *[!0-9]*|'') warn "Choose one listed number or b to search again.";;
            *)
              if [ "$pick" -ge 1 ] 2>/dev/null && [ "$pick" -le "$count" ] 2>/dev/null; then
                IFS='|' read -r lat lon label <<< "$(printf '%s\n' "$results" | sed -n "${pick}p")"
                break 2
              fi
              warn "Choose one listed number or b to search again."
              ;;
          esac
        done
        ;;
      2)
        while :; do
          read -rp "  Latitude [b=back]: " lat
          [ "$lat" = b ] || [ "$lat" = B ] && break
          valid_lat "$lat" && break
          warn "Latitude must be a number from -90 to 90."
        done
        [ "$lat" = b ] || [ "$lat" = B ] && continue
        while :; do
          read -rp "  Longitude [b=back]: " lon
          [ "$lon" = b ] || [ "$lon" = B ] && break
          valid_lon "$lon" && break
          warn "Longitude must be a number from -180 to 180."
        done
        [ "$lon" = b ] || [ "$lon" = B ] && continue
        read -rp "  Location name for the screen [optional]: " label
        label="${label:-$lat, $lon}"
        break
        ;;
      *) warn "Choose 1, 2, or b.";;
    esac
  done

  profile="$(bootstrap_classify_device_profile)"
  case "$profile" in lite|balanced|enhanced) :;; *) profile="balanced";; esac
  country="${label##*, }"
  if [ "$country" = "United States" ]; then
    temp_unit="fahrenheit"; wind_unit="mph"
  else
    temp_unit="celsius"; wind_unit="kmh"
  fi
  echo "Using $([ "$temp_unit" = fahrenheit ] && printf '°F and mph' || printf '°C and km/h') for ${country:-this location} — change anytime in Dashboard Control."
  mkdir -p "$CONFIG_DIR"
  tmp="$(mktemp "$CONFIG_DIR/config.local.js.express.XXXXXX")" || { warn "Could not prepare your dashboard settings. Run ~/install.sh again."; return 1; }
  cat >"$tmp" <<EOFEXPRESSCONFIG
// Generated by Dash-Go Express setup on $(date). Change any setting later in Dashboard Control.
window.DASHBOARD_LOCAL = {
  lat: ${lat},
  lon: ${lon},
  locationName: "$(json_escape "$label")",
  tempUnit: "${temp_unit}",
  windUnit: "${wind_unit}",
  theme: "basic",
  profile: "${profile}",
  showSeconds: true,
  weatherProviders: ["openmeteo"],
  birthdays: []
};
EOFEXPRESSCONFIG
  chmod 600 "$tmp" 2>/dev/null || true
  mv -f "$tmp" "$CONFIG_DIR/config.local.js" || { rm -f "$tmp"; warn "Could not save your dashboard settings. Run ~/install.sh again."; return 1; }
  ok "Location saved: $label. Safe defaults are ready; change anything later in Dashboard Control."
  return 0
}

if [ "$DO_CUSTOM" = "1" ]; then
  installer_stage "Setting up this dashboard"
  if [ "$EXPRESS_MODE" = "1" ]; then
    if run_express_customization; then
      :
    else
      rc=$?
      if [ "$rc" -eq 2 ]; then
        ok "Express setup was cancelled before settings were written. Returning to the installer menu."
        exec bash "$0" "${INSTALLER_ORIGINAL_ARGS[@]}"
      fi
      mark_install_step_failed "Express dashboard setup" "settings were not written; run ~/install.sh again to retry this step"
    fi
  else
    if run_customization; then
      :
    else
      rc=$?
      if [ "$rc" -eq 2 ]; then
        ok "Customization was cancelled before settings were written. Returning to the installer menu."
        exec bash "$0" "${INSTALLER_ORIGINAL_ARGS[@]}"
      fi
      mark_install_step_failed "Dashboard customization" "settings were not written; run ~/install.sh again to retry this step"
    fi
  fi
fi

if [ "$DO_WEATHER_DISPLAY" = "1" ] && [ "$DO_CUSTOM" != "1" ]; then
  say "Weather display configuration"
  prompt_weather_display
  write_weather_display_only
fi

if [ "$DO_WEATHER" = "1" ] && [ "$DO_CUSTOM" != "1" ]; then
  say "Weather source configuration"
  prompt_weather_sources
  write_weather_sources_only
fi

if [ "$DO_RADAR" = "1" ]; then
  if [ "$DO_CUSTOM" != "1" ]; then
    say "Weather radar configuration"
    prompt_radar_provider
  fi
  write_radar_settings_only
fi

if [ "$DO_MESSAGE_SOURCES" = "1" ]; then
  prompt_message_sources
fi

if [ "$DO_APP_SETUP" = "1" ]; then
  configure_app_setup
fi

# ---------------------------------------------------------------------
# Permissions + initial calendar data (always — cheap and idempotent).
say "Permissions"
mkdir -p "$BIN_DIR" "$CONFIG_DIR" "$CAL_DIR" "$CACHE_DIR" "$LOG_DIR" "$FONT_DIR" "$RUNTIME_FONT_DIR" "$BASE_DIR"
chmod +x "$BIN_DIR"/*.sh "$BIN_DIR"/dashboard-control-server* "$DASH/kiosk.sh" 2>/dev/null && ok "scripts made executable"
ensure_message_data_files

# Offer seasonal auto-theming (dashboard dresses up for holidays on its own).
if [ "$DO_CUSTOM" = "1" ] && [ -x "$BIN_DIR/seasonal-themes.sh" ]; then
  echo
  read -rp "Auto-switch theme for holidays/seasons (Halloween, Christmas, etc.)? [y/N] " dosea
  if [ "$dosea" = "y" ] || [ "$dosea" = "Y" ]; then
    "$BIN_DIR/seasonal-themes.sh" install
    ok "Seasonal theming on. Between holidays it uses your chosen theme (${THEME:-default})."
    echo "    Manage later: seasonal-themes.sh [show|base <name>|uninstall]"
  else
    ok "Seasonal theming off. Enable later with: seasonal-themes.sh install"
  fi
fi

# Pull holidays now so the calendar has them on first boot (writes
# holidays.blue.holiday.ics and regenerates the manifest). The Go helper emits
# structured JSON for diagnostics, so keep that output in the installer log
# rather than leaking it into the interactive setup flow.
if [ -x "$BIN_DIR/update-holidays.sh" ]; then
  refresh_holidays_for_installer || true
fi
# Generate calendars.json from whatever .ics are present now.
[ -x "$BIN_DIR/gen-calendars.sh" ] && "$BIN_DIR/gen-calendars.sh" >/dev/null 2>&1
if [ -x "$BIN_DIR/dashboard-control-server" ]; then
  "$BIN_DIR/dashboard-lowprio.sh" "$BIN_DIR/dashboard-control-server" --gen-events-cache >/dev/null 2>&1
fi
ok "calendars.json initialized (add your own .ics files, see README)"


if [ "$DO_ICAL" = "1" ]; then
  installer_stage "Adding view-only calendar links"
  say "Read-only calendar link"
  echo "Dash-Go will show these events but can never change the remote calendar."
  if [ -x "$BIN_DIR/setup-ical-urls.sh" ]; then
    "$BIN_DIR/setup-ical-urls.sh"
  else
    warn "Read-only calendar-link setup is unavailable. Run Update the app first, then try again."
  fi
fi

if [ "$DO_VDIR" = "1" ]; then
  installer_stage "Connecting personal calendars"
  say "Personal calendar sync"
  echo "Dash-Go will use a secure sign-in. Choose view-only or two-way sync for each selected iCloud or CalDAV calendar."
  echo "Google two-way sync uses secure sign-in; Google view-only calendars use option 9."
  if [ -x "$BIN_DIR/setup-vdirsyncer.sh" ]; then
    "$BIN_DIR/setup-vdirsyncer.sh"
  else
    warn "Personal-calendar setup is unavailable. Run Update the app first, then try again."
  fi
fi

if [ "$DO_CALENDARS" = "1" ]; then
installer_stage "Setting up built-in calendars"
say "Built-in/default calendars"
CALCFG="$HOME/.dashboard-default-calendars"
CELECFG="$HOME/.dashboard-celebrations"
DEFAULT_US_HOLIDAYS=1
HOLIDAY_COUNTRY="usa"
HOLIDAY_RELIGIONS=""
DEFAULT_MOON_PHASES=0
DEFAULT_SEASONS=0
DEFAULT_DST_CHANGES=0
DEFAULT_ISO_WEEKS=0  # legacy; migrated to display overlay
DEFAULT_METEOR_SHOWERS=0
DEFAULT_SUPERMOONS=0
DEFAULT_ECLIPSES=0
TRASH_WEEKDAY=""
RECYCLING_WEEKDAY=""
RECYCLING_EVERY_WEEKS=2
PICKUP_HOLIDAY_SHIFT=0
PICKUP_SHIFT="forward"
PICKUP_SHIFT_DAYS=1
PAYDAY_MODE=""
PAYDAY_START=""
PAYDAY_DAY="1"
DEFAULT_ISS_PASSES=0
ISS_N2YO_API_KEY=""
ISS_LOOKAHEAD_DAYS=7
ISS_MIN_VISIBILITY=180
if [ -f "$CALCFG" ]; then
  CALCFG_HAS_RELIGION_MARKER=0
  grep -q '^HOLIDAY_RELIGIONS_CONFIGURED=' "$CALCFG" 2>/dev/null && CALCFG_HAS_RELIGION_MARKER=1
  # shellcheck disable=SC1090
  . "$CALCFG"
  if [ "$CALCFG_HAS_RELIGION_MARKER" != "1" ]; then
    # Older installers could leave Jewish observances selected as an implicit default.
    # Treat unmarked saved values as legacy defaults and start from None.
    HOLIDAY_RELIGIONS=""
  fi
fi
ask_on(){ local prompt="$1" cur="$2" ans; read -rp "  $prompt [$([ "$cur" = "1" ] && echo Y/n || echo y/N)]: " ans; case "$ans" in y|Y) echo 1;; n|N) echo 0;; *) echo "$cur";; esac; }
if installer_demo_prompt_defaults_active; then
  demo_default_note "using Demo Mode built-in calendar defaults: USA holidays, no religious observance layers, no extra pickup/payday/ISS prompts"
  DEFAULT_US_HOLIDAYS=1
  HOLIDAY_COUNTRY="usa"
  HOLIDAY_RELIGIONS=""
  DEFAULT_MOON_PHASES=0
  DEFAULT_SEASONS=0
  DEFAULT_DST_CHANGES=0
  DEFAULT_METEOR_SHOWERS=0
  DEFAULT_SUPERMOONS=0
  DEFAULT_ECLIPSES=0
  TRASH_WEEKDAY=""
  RECYCLING_WEEKDAY=""
  RECYCLING_EVERY_WEEKS=2
  PICKUP_HOLIDAY_SHIFT=0
  PICKUP_SHIFT="forward"
  PICKUP_SHIFT_DAYS=1
  PAYDAY_MODE=""
  PAYDAY_START=""
  PAYDAY_DAY="1"
  DEFAULT_ISS_PASSES=0
  ISS_N2YO_API_KEY=""
  ISS_LOOKAHEAD_DAYS=7
  ISS_MIN_VISIBILITY=180
else
DEFAULT_US_HOLIDAYS="$(ask_on "Public holidays" "${DEFAULT_US_HOLIDAYS:-1}")"
if [ "${DEFAULT_US_HOLIDAYS:-1}" = "1" ]; then
  echo "  Public holiday country codes: usa, uk, canada, australia, germany, france, spain, italy, japan, netherlands, newzealand, mexico"
  read -rp "  Public holiday country [${HOLIDAY_COUNTRY:-usa}]: " ans; [ -n "$ans" ] && HOLIDAY_COUNTRY="$ans"
fi
echo "  Observance layers are optional date layers only."
choose_observance_layers(){
  local cur=" ${HOLIDAY_RELIGIONS:-} " ans
  local sel_jewish=0 sel_islamic=0 sel_christian=0 sel_orthodox=0 sel_hindu=0
  case "$cur" in *jewish*) sel_jewish=1;; esac
  case "$cur" in *islamic*) sel_islamic=1;; esac
  case "$cur" in *christian*) sel_christian=1;; esac
  case "$cur" in *orthodox*) sel_orthodox=1;; esac
  case "$cur" in *hindu*) sel_hindu=1;; esac
  mark(){ [ "$1" = "1" ] && printf '[x]' || printf '[ ]'; }
  toggle_obs(){
    case "$1" in
      1) [ "$sel_jewish" = "1" ] && sel_jewish=0 || sel_jewish=1;;
      2) [ "$sel_islamic" = "1" ] && sel_islamic=0 || sel_islamic=1;;
      3) [ "$sel_christian" = "1" ] && sel_christian=0 || sel_christian=1;;
      4) [ "$sel_orthodox" = "1" ] && sel_orthodox=0 || sel_orthodox=1;;
      5) [ "$sel_hindu" = "1" ] && sel_hindu=0 || sel_hindu=1;;
    esac
  }
  while true; do
    echo "  Observance layers — toggle one number at a time"
    echo "  Selected layers appear on Calendar and enable matching rotating holiday messages."
    echo "    $(mark "$sel_jewish") 1) Jewish"
    echo "    $(mark "$sel_islamic") 2) Islamic"
    echo "    $(mark "$sel_christian") 3) Christian"
    echo "    $(mark "$sel_orthodox") 4) Orthodox"
    echo "    $(mark "$sel_hindu") 5) Hindu"
    echo "    s) Save   n) None   q) Keep current"
    read -rp "  Choice: " ans
    case "$ans" in
      s|S) break;;
      n|N) sel_jewish=0; sel_islamic=0; sel_christian=0; sel_orthodox=0; sel_hindu=0; break;;
      q|Q|'') return 0;;
      1|2|3|4|5) toggle_obs "$ans";;
      *) warn "enter one number, s, n, or q";;
    esac
  done
  HOLIDAY_RELIGIONS=""
  [ "$sel_jewish" = "1" ] && HOLIDAY_RELIGIONS="${HOLIDAY_RELIGIONS:+$HOLIDAY_RELIGIONS,}jewish"
  [ "$sel_islamic" = "1" ] && HOLIDAY_RELIGIONS="${HOLIDAY_RELIGIONS:+$HOLIDAY_RELIGIONS,}islamic"
  [ "$sel_christian" = "1" ] && HOLIDAY_RELIGIONS="${HOLIDAY_RELIGIONS:+$HOLIDAY_RELIGIONS,}christian"
  [ "$sel_orthodox" = "1" ] && HOLIDAY_RELIGIONS="${HOLIDAY_RELIGIONS:+$HOLIDAY_RELIGIONS,}orthodox"
  [ "$sel_hindu" = "1" ] && HOLIDAY_RELIGIONS="${HOLIDAY_RELIGIONS:+$HOLIDAY_RELIGIONS,}hindu"
}
choose_observance_layers
DEFAULT_MOON_PHASES="$(ask_on "Moon phases" "${DEFAULT_MOON_PHASES:-0}")"
DEFAULT_SEASONS="$(ask_on "Seasons / solstices / equinoxes" "${DEFAULT_SEASONS:-0}")"
DEFAULT_DST_CHANGES="$(ask_on "Daylight Saving Time change markers" "${DEFAULT_DST_CHANGES:-0}")"
DEFAULT_METEOR_SHOWERS="$(ask_on "Meteor showers" "${DEFAULT_METEOR_SHOWERS:-0}")"
DEFAULT_SUPERMOONS="$(ask_on "Supermoons" "${DEFAULT_SUPERMOONS:-0}")"
DEFAULT_ECLIPSES="$(ask_on "Eclipse date reminders" "${DEFAULT_ECLIPSES:-0}")"
echo "  Household Schedules in Dashboard Control can later add, pause, or correct Paydays, Trash Pickup, and Recycling Pickup without rerunning setup."
read -rp "  Weekly trash pickup weekday (blank = none) [${TRASH_WEEKDAY:-none}]: " ans; [ -n "$ans" ] && TRASH_WEEKDAY="$ans"
read -rp "  Recycling pickup weekday (blank = none) [${RECYCLING_WEEKDAY:-none}]: " ans; [ -n "$ans" ] && RECYCLING_WEEKDAY="$ans"
if [ -n "${RECYCLING_WEEKDAY:-}" ]; then
  read -rp "  Recycling frequency in weeks [${RECYCLING_EVERY_WEEKS:-2}]: " ans
  [ -n "$ans" ] && printf '%s' "$ans" | grep -qE '^[0-9]+$' && RECYCLING_EVERY_WEEKS="$ans"
fi
if [ -n "${TRASH_WEEKDAY:-}${RECYCLING_WEEKDAY:-}" ]; then
  echo "  Holiday-aware pickup shifting is best-effort; municipality rules vary."
  PICKUP_HOLIDAY_SHIFT="$(ask_on "Shift trash/recycling after public holidays" "${PICKUP_HOLIDAY_SHIFT:-0}")"
  if [ "${PICKUP_HOLIDAY_SHIFT:-0}" = "1" ]; then
    read -rp "  Shift direction forward/backward [${PICKUP_SHIFT:-forward}]: " ans; [ -n "$ans" ] && PICKUP_SHIFT="$ans"
    read -rp "  Shift by how many days [${PICKUP_SHIFT_DAYS:-1}]: " ans
    [ -n "$ans" ] && printf '%s' "$ans" | grep -qE '^[0-9]+$' && PICKUP_SHIFT_DAYS="$ans"
  fi
fi
if [ ! -f "$CELECFG" ]; then
  echo "  Celebrations/special dates can be added later in $CELECFG as: MM-DD | Label or YYYY-MM-DD | Label"
  read -rp "  Add a celebration/special date now? [y/N] " addcele
  if [ "$addcele" = "y" ] || [ "$addcele" = "Y" ]; then
    : > "$CELECFG"
    while :; do
      read -rp "    Date (MM-DD or YYYY-MM-DD, blank = done): " cdate
      [ -z "$cdate" ] && break
      read -rp "    Label: " clabel
      [ -n "$clabel" ] && printf '%s | %s\n' "$cdate" "$clabel" >> "$CELECFG"
    done
    chmod 600 "$CELECFG"
  fi
else
  echo "  Celebrations source already exists: $CELECFG"
fi
echo "  Payday calendar (initial shortcut — add multiple named paydays and business-day rules later in Dashboard Control → Calendars → Household Schedules):"
echo "    1) none"
echo "    2) weekly from a start date"
echo "    3) biweekly from a start date"
echo "    4) monthly on a day number"
case "${PAYDAY_MODE:-}" in weekly) PDEF=2;; biweekly) PDEF=3;; monthly) PDEF=4;; *) PDEF=1;; esac
read -rp "  Choose [1-4, Enter=$PDEF]: " paychoice; paychoice="${paychoice:-$PDEF}"
case "$paychoice" in
  2) PAYDAY_MODE="weekly"; read -rp "  First/known payday date YYYY-MM-DD [${PAYDAY_START:-}]: " ans; [ -n "$ans" ] && PAYDAY_START="$ans";;
  3) PAYDAY_MODE="biweekly"; read -rp "  First/known payday date YYYY-MM-DD [${PAYDAY_START:-}]: " ans; [ -n "$ans" ] && PAYDAY_START="$ans";;
  4) PAYDAY_MODE="monthly"; read -rp "  Monthly payday day number 1-28 [${PAYDAY_DAY:-1}]: " ans; [ -n "$ans" ] && PAYDAY_DAY="$ans";;
  *) PAYDAY_MODE=""; PAYDAY_START="";;
esac
echo "  ISS visible passes require a free N2YO API key and live third-party prediction service."
DEFAULT_ISS_PASSES="$(ask_on "ISS visible passes" "${DEFAULT_ISS_PASSES:-0}")"
if [ "${DEFAULT_ISS_PASSES:-0}" = "1" ]; then
  read -rp "  N2YO API key (blank = keep current) [$([ -n "${ISS_N2YO_API_KEY:-}" ] && echo saved || echo none)]: " ans; [ -n "$ans" ] && ISS_N2YO_API_KEY="$ans"
  read -rp "  ISS lookahead days 1-10 [${ISS_LOOKAHEAD_DAYS:-7}]: " ans
  [ -n "$ans" ] && printf '%s' "$ans" | grep -qE '^[0-9]+$' && ISS_LOOKAHEAD_DAYS="$ans"
  read -rp "  Minimum visibility seconds 1-300 [${ISS_MIN_VISIBILITY:-180}]: " ans
  [ -n "$ans" ] && printf '%s' "$ans" | grep -qE '^[0-9]+$' && ISS_MIN_VISIBILITY="$ans"
fi
fi
{
  echo "# saved by install.sh — built-in/generated dashboard calendars"
  echo "HOLIDAY_RELIGIONS_CONFIGURED=1"
  echo "DEFAULT_US_HOLIDAYS=$DEFAULT_US_HOLIDAYS"
  printf 'HOLIDAY_COUNTRY=%q\n' "${HOLIDAY_COUNTRY:-usa}"
  printf 'HOLIDAY_RELIGIONS=%q\n' "${HOLIDAY_RELIGIONS:-}"
  echo "DEFAULT_MOON_PHASES=$DEFAULT_MOON_PHASES"
  echo "DEFAULT_SEASONS=$DEFAULT_SEASONS"
  echo "DEFAULT_DST_CHANGES=$DEFAULT_DST_CHANGES"
  echo "DEFAULT_ISO_WEEKS=0" # legacy: ISO weeks are now a Dashboard Control display overlay
  echo "DEFAULT_METEOR_SHOWERS=${DEFAULT_METEOR_SHOWERS:-0}"
  echo "DEFAULT_SUPERMOONS=${DEFAULT_SUPERMOONS:-0}"
  echo "DEFAULT_ECLIPSES=${DEFAULT_ECLIPSES:-0}"
  printf 'TRASH_WEEKDAY=%q\n' "${TRASH_WEEKDAY:-}"
  printf 'RECYCLING_WEEKDAY=%q\n' "${RECYCLING_WEEKDAY:-}"
  echo "RECYCLING_EVERY_WEEKS=${RECYCLING_EVERY_WEEKS:-2}"
  echo "PICKUP_HOLIDAY_SHIFT=${PICKUP_HOLIDAY_SHIFT:-0}"
  printf 'PICKUP_SHIFT=%q\n' "${PICKUP_SHIFT:-forward}"
  printf 'PICKUP_SHIFT_DAYS=%q\n' "${PICKUP_SHIFT_DAYS:-1}"
  printf 'PAYDAY_MODE=%q\n' "${PAYDAY_MODE:-}"
  printf 'PAYDAY_START=%q\n' "${PAYDAY_START:-}"
  printf 'PAYDAY_DAY=%q\n' "${PAYDAY_DAY:-1}"
  echo "DEFAULT_ISS_PASSES=${DEFAULT_ISS_PASSES:-0}"
  printf 'ISS_N2YO_API_KEY=%q\n' "${ISS_N2YO_API_KEY:-}"
  printf 'ISS_LOOKAHEAD_DAYS=%q\n' "${ISS_LOOKAHEAD_DAYS:-7}"
  printf 'ISS_MIN_VISIBILITY=%q\n' "${ISS_MIN_VISIBILITY:-180}"
} > "$CALCFG"
chmod 600 "$CALCFG"
[ -f "$CELECFG" ] && chmod 600 "$CELECFG" || true
[ -x "$BIN_DIR/update-holidays.sh" ] && "$BIN_DIR/update-holidays.sh" >/dev/null 2>&1 || true
[ -x "$BIN_DIR/update-iss-passes.sh" ] && "$BIN_DIR/update-iss-passes.sh" >/dev/null 2>&1 || true
[ -x "$BIN_DIR/gen-default-calendars.sh" ] && "$BIN_DIR/gen-default-calendars.sh" >/dev/null 2>&1 || true
[ -x "$BIN_DIR/gen-sky-calendars.sh" ] && "$BIN_DIR/gen-sky-calendars.sh" >/dev/null 2>&1 || true
  [ -x "$BIN_DIR/dashboard-control-server" ] && "$BIN_DIR/dashboard-control-server" --update-message-feeds >/dev/null 2>&1 || true

ok "generated selected built-in calendars"
[ -x "$BIN_DIR/gen-calendars.sh" ] && "$BIN_DIR/gen-calendars.sh" >/dev/null 2>&1
if [ -x "$BIN_DIR/dashboard-control-server" ]; then
  "$BIN_DIR/dashboard-lowprio.sh" "$BIN_DIR/dashboard-control-server" --gen-events-cache >/dev/null 2>&1
fi
fi

if [ "$DO_CALENDARS" = "1" ]; then
if installer_demo_prompt_defaults_active; then
  ok "Demo Mode uses seeded local ICS calendars; skipped add-calendar prompts"
else
# Calendar setup starts with the one decision a normal household user can
# understand: may Dash-Go only show events, or may it send approved changes
# back? Provider-specific mechanics follow only after that answer.
while true; do
  echo
  echo "How should Dash-Go use a personal calendar?"
  echo "  1) View events only"
  echo "     Dash-Go shows the calendar but never changes it."
  echo "  2) Two-way sync"
  echo "     Dash-Go may add, edit, or skip supported events in calendars you choose."
  echo "  3) Skip for now"
  read -rp "  Choose [3]: " calaccess
  calaccess="$(normalize_menu_choice "${calaccess:-3}")"
  case "$calaccess" in
    1)
      while true; do
        echo "  View-only calendar type:"
        echo "    1) Google, Outlook, school, or public feed — paste a calendar link"
        echo "    2) Apple iCloud — sign in with an app-specific password; keep calendars view-only"
        echo "    3) Another personal CalDAV account — sign in; keep calendars view-only"
        echo "    b) Back"
        read -rp "  Choose [1]: " calmethod
        calmethod="$(normalize_menu_choice "${calmethod:-1}")"
        case "$calmethod" in
          1) [ -x "$BIN_DIR/setup-ical-urls.sh" ] && "$BIN_DIR/setup-ical-urls.sh" || warn "Read-only calendar-link setup is unavailable. Run Update the app first, then try again."; break;;
          2|3) [ -x "$BIN_DIR/setup-vdirsyncer.sh" ] && "$BIN_DIR/setup-vdirsyncer.sh" --view-only || warn "Private calendar setup is unavailable. Run Update the app first, then try again."; break;;
          b|B) break;;
          *) warn "Choose 1, 2, 3, or b to go back.";;
        esac
      done
      ;;
    2)
      if [ -x "$BIN_DIR/setup-vdirsyncer.sh" ]; then
        "$BIN_DIR/setup-vdirsyncer.sh" --two-way
      else
        warn "Two-way personal-calendar setup is unavailable. Run Update the app first, then try again."
      fi
      ;;
    3|q|Q) ok "Calendar setup skipped. You can add or change access later from installer options 9 and 10 or Calendar Manager."; break;;
    *) warn "Choose 1 for view-only, 2 for two-way sync, or 3 to skip.";;
  esac
done
fi
fi

# ---------------------------------------------------------------------
if [ "$DO_SERVICE" = "1" ]; then
installer_stage "Starting Dashboard Control"
say "Web server service (dashboard-server.service, localhost:8090)"
if ! provision_dashboard_service; then
  warn "dashboard-server service wiring was not fully provisioned"
fi

# --- On-screen control overlay: reboot permission ---------------------
# The overlay (triple-tap the moon-phase icon next to the weather) can restart
# the browser and change themes as the normal user, but reboot/shutdown needs
# root. This grants exactly ONE command — /sbin/reboot — nothing else.
echo
echo "The on-screen control overlay includes reboot and shutdown"
echo "buttons. Those two actions need a sudo rule allowing this user to run"
echo "/sbin/reboot and /sbin/poweroff (those exact commands, nothing else)."
if installer_demo_defaults_active; then
  RBOK="y"
  demo_default_note "allowing the overlay reboot/shutdown buttons"
else
  read -rp "Allow the overlay to reboot/shut down this device? [Y/n] " RBOK
fi
if [ "$RBOK" = "n" ] || [ "$RBOK" = "N" ]; then
  ok "skipped — those buttons will show a 'not permitted' message"
else
  SUDOERS_FILE=/etc/sudoers.d/010-dashboard-reboot
  echo "$USER_NAME ALL=(root) NOPASSWD: /sbin/reboot, /sbin/poweroff" | $SUDO tee "$SUDOERS_FILE" >/dev/null
  $SUDO chmod 0440 "$SUDOERS_FILE"
  # Validate before trusting it — a bad sudoers file can lock sudo out.
  if $SUDO visudo -cf "$SUDOERS_FILE" >/dev/null 2>&1; then
    ok "reboot/shutdown permission granted (scoped to those two commands only)"
  else
    $SUDO rm -f "$SUDOERS_FILE"
    warn "sudoers validation failed — entry removed, reboot/shutdown buttons disabled"
  fi
fi

# --- On-screen control overlay: system package update permission --------
# The overlay can run a bounded OS package update for maintenance:
#   apt-get update && apt-get -y upgrade
# This is separate from dashboard app updates and does not grant arbitrary sudo.
echo
echo "The on-screen control overlay can include a System update button that runs"
echo "apt-get update, then apt-get -y upgrade. This updates OS packages only;"
echo "dashboard app updates remain under Update / Restore / Backup."
echo "This requires a sudo rule for exactly those apt-get commands."
if installer_demo_defaults_active; then
  SYSUPDOK="y"
  demo_default_note "allowing Dashboard Control system package updates"
else
  read -rp "Allow Dashboard Control to run system package updates? [Y/n] " SYSUPDOK
fi
if [ "$SYSUPDOK" = "n" ] || [ "$SYSUPDOK" = "N" ]; then
  ok "skipped — the System update button will show setup/permission guidance"
else
  SYSUPD_SUDOERS=/etc/sudoers.d/011-dashboard-system-update
  APT_GET_PATH="$(command -v apt-get 2>/dev/null || echo /usr/bin/apt-get)"
  {
    echo "# Written by Dash-Go install.sh."
    echo "# Allows the local kiosk control panel to run bounded OS package maintenance."
    echo "# Kept intentionally narrow: no arbitrary sudo, only these apt-get commands."
    echo "$USER_NAME ALL=(root) NOPASSWD: $APT_GET_PATH update, $APT_GET_PATH -y upgrade"
  } | $SUDO tee "$SYSUPD_SUDOERS" >/dev/null
  $SUDO chmod 0440 "$SYSUPD_SUDOERS"
  if $SUDO visudo -cf "$SYSUPD_SUDOERS" >/dev/null 2>&1; then
    ok "system update permission granted (scoped to apt-get update + apt-get -y upgrade)"
  else
    $SUDO rm -f "$SYSUPD_SUDOERS"
    warn "sudoers validation failed — entry removed, System update button disabled"
  fi
fi

# --- Security hardening: broad passwordless sudo ----------------------
# Detect Raspberry Pi OS style 010_pi-nopasswd rules and offer to replace
# NOPASSWD: ALL with the scoped dashboard rules above. This is never silent.
echo
say "Sudo security hardening"
harden_dashboard_sudoers || true

# --- Kiosk anti-lock/autologout guard ----------------------------------
echo
say "Kiosk anti-lock / autologout guard"
install_kiosk_no_logout_guard || warn "could not fully install kiosk anti-lock/autologout guard"

# --- Hardware watchdog / runtime watchdog -----------------------------
echo
if [ "$IS_PI" = "1" ]; then
  echo "Optional: enable the Raspberry Pi hardware watchdog. If the system ever"
  echo "hard-freezes, it reboots itself within ~15 seconds instead of sitting on"
  echo "a black screen until someone notices. Recommended for an always-on Pi."
  if installer_demo_defaults_active; then
    WDOG="y"
    demo_default_note "enabling Raspberry Pi hardware watchdog"
  else
    read -rp "Enable Raspberry Pi hardware watchdog? [Y/n] " WDOG
  fi
  if [ "$WDOG" = "n" ] || [ "$WDOG" = "N" ]; then
    ok "watchdog skipped (re-run this step anytime to enable it)"
  else
    BOOTCFG=""
    for b in /boot/firmware/config.txt /boot/config.txt; do
      [ -e "$b" ] && BOOTCFG="$b" && break
    done
    if [ -n "$BOOTCFG" ]; then
      if grep -q "^dtparam=watchdog=on" "$BOOTCFG"; then
        ok "watchdog already enabled in $BOOTCFG"
      else
        {
          echo ""
          echo "# Hardware watchdog (added by dashboard installer): auto-reboot on hard freeze."
          echo "dtparam=watchdog=on"
        } | $SUDO tee -a "$BOOTCFG" >/dev/null
        ok "added dtparam=watchdog=on to $BOOTCFG (takes effect after reboot)"
      fi
    else
      warn "couldn't find the Pi boot config.txt — add 'dtparam=watchdog=on' manually if needed"
    fi
    $SUDO mkdir -p /etc/systemd/system.conf.d
    $SUDO tee /etc/systemd/system.conf.d/10-dashboard-watchdog.conf >/dev/null <<'WDCONF'
# Installed by the dashboard installer (Raspberry Pi hardware watchdog).
# systemd feeds the BCM2835 watchdog; if userspace ever hangs hard, the
# chip reboots the Pi automatically. 15s is this SoC's maximum — keep it.
[Manager]
RuntimeWatchdogSec=15
WDCONF
    $SUDO systemctl daemon-reexec
    ok "Pi watchdog configured (RuntimeWatchdogSec=15)"
  fi
else
  echo "Optional: enable a generic systemd runtime watchdog. This is NOT the"
  echo "Raspberry Pi boot/config watchdog; it only uses whatever watchdog device"
  echo "your x86/Linux system exposes. Skip this if unsure."
  if installer_demo_defaults_active; then
    WDOG="y"
    demo_default_note "enabling generic runtime watchdog"
  else
    read -rp "Enable generic runtime watchdog? [y/N] " WDOG
  fi
  if [ "$WDOG" = "y" ] || [ "$WDOG" = "Y" ]; then
    $SUDO mkdir -p /etc/systemd/system.conf.d
    $SUDO tee /etc/systemd/system.conf.d/10-dashboard-watchdog.conf >/dev/null <<'WDCONF'
# Installed by the dashboard installer (generic Linux runtime watchdog).
# This does not edit Raspberry Pi boot config. It asks systemd to feed the
# available watchdog device, if one exists on this hardware/VM.
[Manager]
RuntimeWatchdogSec=30
WDCONF
    $SUDO systemctl daemon-reexec
    ok "generic runtime watchdog configured (RuntimeWatchdogSec=30)"
  else
    ok "generic watchdog skipped"
  fi
fi
fi  # end DO_SERVICE

# ---------------------------------------------------------------------
if [ "$DO_PIN" = "1" ]; then
  installer_stage "Setting the dashboard PIN"
  configure_control_pin
fi

# ---------------------------------------------------------------------
if [ "$DO_AUTOLOGIN" = "1" ]; then
  installer_stage "Preparing automatic sign-in"
  provision_dashboard_autologin interactive || warn "graphical autologin was not fully configured"
fi  # end DO_AUTOLOGIN

# ---------------------------------------------------------------------
if [ "$DO_AUTOSTART" = "1" ]; then
  installer_stage "Preparing automatic startup"
  provision_dashboard_autostart_cron interactive || warn "autostart or canonical scheduler was not fully configured"
fi  # end DO_AUTOSTART

# ---------------------------------------------------------------------
if [ "$DO_SSH" = "1" ]; then
installer_stage "Preparing remote access"
say "Enabling SSH for remote (headless) administration"
# Install + enable the SSH server.
apt_managed install -y openssh-server >/dev/null 2>&1
$SUDO systemctl enable --now ssh 2>/dev/null || $SUDO systemctl enable --now sshd 2>/dev/null
if systemctl is-active ssh >/dev/null 2>&1 || systemctl is-active sshd >/dev/null 2>&1; then
  ok "SSH server running"
else
  warn "SSH server may not have started -- check: systemctl status ssh"
fi

echo
echo "SSH authentication method:"
echo "  1) SSH key (recommended — paste a public key; password login stays off)"
echo "  2) Password (convenient on a trusted LAN, but brute-forceable)"
read -rp "  Choose [1/2]: " sshmethod
SSHD_DROPIN=/etc/ssh/sshd_config.d/00-dashboard.conf
OLD_SSHD_DROPIN=/etc/ssh/sshd_config.d/99-dashboard.conf
EXPECTED_PASSWORD_SETTING=""
$SUDO mkdir -p /etc/ssh/sshd_config.d
# OpenSSH uses the first value it reads for most settings. Debian includes
# sshd_config.d files in lexical order, so use 00-dashboard.conf to win over
# cloud-image or distro snippets such as 50-cloud-init.conf. Remove the old
# 99-dashboard.conf from earlier installer versions so there is one source.
$SUDO rm -f "$OLD_SSHD_DROPIN"
if [ -f /etc/ssh/sshd_config ] && ! grep -Eq '^[[:space:]]*Include[[:space:]]+/etc/ssh/sshd_config\.d/\*\.conf' /etc/ssh/sshd_config; then
  $SUDO cp /etc/ssh/sshd_config /etc/ssh/sshd_config.bak.dashboard
  { echo 'Include /etc/ssh/sshd_config.d/*.conf'; cat /etc/ssh/sshd_config; } | $SUDO tee /etc/ssh/sshd_config.tmp.dashboard >/dev/null
  $SUDO mv /etc/ssh/sshd_config.tmp.dashboard /etc/ssh/sshd_config
fi
if [ "$sshmethod" = "2" ]; then
  # Password auth. Requires the user to actually have a password set.
  warn "Password SSH is less secure than keys. Only do this on a trusted network."
  pwstate="$($SUDO passwd -S "$USER_NAME" 2>/dev/null | awk '{print $2}')"
  if [ "$pwstate" != "P" ]; then
    echo "User '$USER_NAME' has no usable password set (needed for password SSH)."
    echo "Set one now:"
    $SUDO passwd "$USER_NAME"
  fi
  cat <<SSHCONF | $SUDO tee "$SSHD_DROPIN" >/dev/null
# Written by dashboard install.sh. This file intentionally sorts early:
# sshd uses the first value read for these settings, so 00-dashboard.conf
# overrides distro/cloud snippets that may disable password login.
PubkeyAuthentication yes
PasswordAuthentication yes
KbdInteractiveAuthentication yes
UsePAM yes
PermitRootLogin no
SSHCONF
  EXPECTED_PASSWORD_SETTING="yes"
  ok "Password SSH enabled for $USER_NAME"
else
  # Key-based. Collect a public key and disable password auth.
  echo "Paste your SSH PUBLIC key (one line, starts ssh-ed25519 or ssh-rsa),"
  read -rp "  or leave blank to configure keys yourself later: " PUBKEY
  if [ -n "$PUBKEY" ]; then
    install -d -m 700 -o "$USER_NAME" -g "$USER_NAME" "$HOME/.ssh"
    printf '%s\n' "$PUBKEY" >> "$HOME/.ssh/authorized_keys"
    chmod 600 "$HOME/.ssh/authorized_keys"; chown "$USER_NAME":"$USER_NAME" "$HOME/.ssh/authorized_keys"
    ok "public key added to authorized_keys"
  else
    warn "No key added. Add one to ~/.ssh/authorized_keys before disabling passwords,"
    warn "or you may lock yourself out. Leaving SSH password settings at their current value."
  fi
  # Only harden (disable passwords) if a key was actually installed.
  if [ -n "$PUBKEY" ]; then
    cat <<SSHCONF | $SUDO tee "$SSHD_DROPIN" >/dev/null
# Written by dashboard install.sh. This file intentionally sorts early:
# sshd uses the first value read for these settings, so 00-dashboard.conf
# overrides distro/cloud snippets.
PubkeyAuthentication yes
PasswordAuthentication no
KbdInteractiveAuthentication no
UsePAM yes
PermitRootLogin no
SSHCONF
    EXPECTED_PASSWORD_SETTING="no"
    ok "key-only SSH enabled (password login disabled)"
  fi
fi
# Validate config and reload so changes take effect.
if $SUDO sshd -t 2>/dev/null; then
  $SUDO systemctl reload ssh 2>/dev/null || $SUDO systemctl reload sshd 2>/dev/null || \
    $SUDO systemctl restart ssh 2>/dev/null || $SUDO systemctl restart sshd 2>/dev/null
  ok "SSH config reloaded"
  if [ -n "$EXPECTED_PASSWORD_SETTING" ]; then
    eff="$($SUDO sshd -T -C user="$USER_NAME",host="$(hostname 2>/dev/null || echo dashboard)",addr=127.0.0.1 2>/dev/null | awk '/^passwordauthentication /{print $2; exit}')"
    if [ "$eff" = "$EXPECTED_PASSWORD_SETTING" ]; then
      ok "effective SSH PasswordAuthentication=$eff"
    else
      warn "effective SSH PasswordAuthentication is '${eff:-unknown}', expected '$EXPECTED_PASSWORD_SETTING'"
      warn "Check /etc/ssh/sshd_config and /etc/ssh/sshd_config.d for earlier Match/Include rules."
    fi
  fi
  IP="$(hostname -I 2>/dev/null | awk '{print $1}')"
  echo "Reach this device at:  ssh $USER_NAME@${IP:-<device-ip>}   (or ssh $USER_NAME@$(hostname).local)"
else
  warn "sshd config test failed -- not reloading. Check $SSHD_DROPIN"
fi
fi  # end DO_SSH

# ---------------------------------------------------------------------
# If we updated what surf displays (app files) or the device settings,
# restart the running kiosk so the changes appear now without a manual reboot.
if [ "$APP_FILES_OK" = "1" ] || [ "$DO_CUSTOM" = "1" ] || [ "$DO_WEATHER" = "1" ] || [ "$DO_RADAR" = "1" ] || [ "$DO_WEATHER_DISPLAY" = "1" ] || [ "$DO_MESSAGE_SOURCES" = "1" ] || [ "$DO_APP_SETUP" = "1" ] || [ "$DEMO_MODE_OK" = "1" ]; then
  restart_kiosk
fi

# Offer a safe Pi display review as an explicit, reversible choice.
# KMS settings stay unchanged; even Express never changes firmware settings silently.
offer_display_config(){
  local cfg backup answer managed
  [ "$IS_PI" = "1" ] || return 0
  cfg="/boot/firmware/config.txt"; [ -f "$cfg" ] || cfg="/boot/config.txt"
  [ -f "$cfg" ] || { warn "Raspberry Pi display configuration file was not found. Leave display settings unchanged and run ~/install.sh again after checking the boot partition."; return 0; }
  echo
  echo "Optional Raspberry Pi display review"
  echo "Dash-Go preserves the current KMS display driver on Bookworm and Trixie. It will not switch this device to the older FKMS driver or change GPU memory automatically."
  echo "Current relevant lines in $cfg:"
  grep -E '^[[:space:]]*(dtoverlay=vc4-|gpu_mem=)' "$cfg" 2>/dev/null || echo "  (none found)"
  case "${OS_SUPPORT_LEVEL:-}" in
    recommended|supported|conservative)
      echo "Detected OS policy: ${OS_SUPPORT_LABEL:-current Debian-family release}."
      echo "For a black screen, keep the current driver and use Dashboard Doctor before changing firmware settings manually."
      ;;
    *)
      echo "This release is not a supported fresh-install target; Dash-Go will not edit display firmware settings."
      return 0
      ;;
  esac
  read -rp "Create a reversible backup before any manual display troubleshooting? [y/N] " answer
  case "$(normalize_menu_choice "$answer")" in
    y|Y) : ;;
    *) echo "Left display firmware settings unchanged."; return 0;;
  esac
  backup="${cfg}.dash-go.bak"
  if [ -f "$backup" ]; then
    ok "Existing display backup kept at $backup. Current KMS settings were not changed."
  elif $SUDO cp -p "$cfg" "$backup"; then
    ok "Saved a display backup at $backup. Current KMS settings were not changed."
  else
    warn "Could not save $backup. Leave display settings unchanged and run ~/install.sh again after checking sudo access."
    return 1
  fi
  managed="$(grep -F 'Managed by Dash-Go display setup' "$cfg" 2>/dev/null || true)"
  if [ -n "$managed" ]; then
    warn "A legacy Dash-Go display marker was found. Bookworm/Trixie keep KMS; do not apply a new driver conversion. Use Dashboard Doctor or restore $backup only if you deliberately need to undo a prior manual change."
  fi
  echo "If the screen is black after a future manual change, use another computer or the SD card to restore $backup over $cfg, then reboot."
}


dashboard_access_note(){
  local ip
  echo "Dashboard on this device: http://localhost:8090"
  ip="$(hostname -I 2>/dev/null | awk '{print $1}')"
  if ss -ltn 2>/dev/null | awk '$4 ~ /(^|:)8090$/ && $4 !~ /127\.0\.0\.1:8090$/ && $4 !~ /\[::1\]:8090$/ {found=1} END{exit !found}'; then
    echo "Dashboard from your network: http://${ip:-<device-ip>}:8090"
  else
    echo "From phones and other computers: currently OFF (private by default)."
    echo "Use Dashboard Control on the display, or set up an explicit secure proxy if you need remote administration."
  fi
}

if [ "$MODE" = "$OPT_FULL" ] && [ "$IS_PI" = "1" ]; then
  offer_display_config || mark_install_step_failed "Optional Pi display configuration" "the existing display settings were left unchanged"
fi

# ---------------------------------------------------------------------
INSTALLER_FINAL_READY=1
if ! installer_final_dashboard_check; then
  INSTALLER_FINAL_READY=0
fi

if [ "${#INSTALL_STEP_FAILURES[@]}" -gt 0 ]; then
  echo
  warn "Installer completed with ${#INSTALL_STEP_FAILURES[@]} skipped or failed step(s):"
  for _step in "${INSTALL_STEP_FAILURES[@]}"; do
    printf '   - %s\n' "$_step"
  done
  warn "Re-run ~/install.sh and choose the named option after resolving the warning."
fi
if [ "$INSTALLER_FINAL_READY" = "1" ] && [ "${#INSTALL_STEP_FAILURES[@]}" -eq 0 ]; then
  say "All done."
  dashboard_access_note
else
  say "Dash-Go needs one more check"
  warn "The installer did not claim success. Review the named action above, then run ~/install.sh again."
fi
# A short, tailored recap of what just happened and what (if anything) to do.
if [ "$DO_FILES" = "1" ] && [ "$APP_FILES_OK" = "1" ]; then
  echo "   * App files updated. The screen itself refreshes at the next browser restart (tonight, or via the panel)."
elif [ "$DO_FILES" = "1" ]; then
  echo "   * App files were not updated; review the warnings above before rebooting."
fi
[ "$DO_CUSTOM" = "1" ]    && echo "   * Settings saved — the dashboard restarts now to use them."
[ "$DO_WEATHER_DISPLAY" = "1" ] && echo "   * Weather display behavior updated."
[ "$DO_WEATHER" = "1" ] && echo "   * Weather providers updated."
[ "$DO_RADAR" = "1" ] && echo "   * Weather radar provider updated (on-demand only)."
[ "$DO_MESSAGE_SOURCES" = "1" ] && echo "   * Message sources checked/updated."
[ "$DO_APP_SETUP" = "1" ] && echo "   * Microsoft To Do / Graph setup updated; local Lists remain the default source."
if [ "$DO_DEMO" = "1" ] && [ "$DEMO_MODE_OK" = "1" ]; then
  echo "   * Demo Mode enabled — the screen shows Chicago sample data and a DEMO MODE badge."
elif [ "$DO_DEMO" = "1" ]; then
  echo "   * Demo Mode was requested but did not finish; update app files, then choose Demo Mode again."
fi
if [ "$DO_SERVICE" = "1" ] && [ "$SERVICE_CONFIRMED_OK" = "1" ]; then
  echo "   * Web server + control panel installed. Check it any time with Dashboard service."
elif [ "$DO_SERVICE" = "1" ]; then
  echo "   * Web server setup was attempted, but live server confirmation did not pass."
fi
[ "$DO_CALENDARS" = "1" ] && echo "   * Calendars configured — events appear within a few minutes."
[ "$DO_PIN" = "1" ]       && echo "   * Control-panel PIN setting updated. Manage it any time with Control PIN."
[ "$DO_SSH" = "1" ]       && echo "   * SSH enabled — connect with: ssh $USER_NAME@$(hostname 2>/dev/null || echo '<device>')"
echo "   * Tip: TRIPLE-TAP the moon-phase icon next to the weather to open Dashboard Control."
[ "$DOC_AT_END" = "1" ] && [ -x "$BIN_DIR/doctor.sh" ] && { echo; bash "$BIN_DIR/doctor.sh"; }

# The full first-boot guide only matters when boot/system pieces were touched.
if [ "$MODE" = "1" ] || [ "$DO_AUTOSTART" = "1" ] || [ "$DO_AUTOLOGIN" = "1" ]; then
if [ "$IS_PI" = "1" ]; then
cat <<NOTE

WHAT YOU SHOULD SEE: after a reboot the screen boots straight into the
dashboard, fullscreen, no desktop.

  On the device: http://localhost:8090
  From phones/other computers: the dashboard stays private by default.
  Add calendars from Dashboard Control, or re-run ~/install.sh and choose
  Personal calendar sync / Read-only calendar link when you are ready.

NEXT STEPS FOR RASPBERRY PI:

  1) DISPLAY DRIVER — the installer offered a reversible display setup above.
     If the screen is black after reboot, restore config.txt.dash-go.bak from
     another computer or the SD card, then reboot.

  2) CURSOR HIDE — sudo apt install unclutter-xfixes

  3) Optional memory/SD tuning: re-run the installer and choose System update
     / optional platform trim, then opt into its zram-tools + swappiness=10.

  4) Add calendars: use Dashboard Control or re-run ~/install.sh.

IF SOMETHING LOOKS WRONG:
  * Black/frozen screen       -> connect a keyboard, press Ctrl+Alt+F2, sign in,
                                 then run: ~/install.sh --doctor
                                 For a system/session repair: ~/install.sh --repair --system
  * Dashboard but no events   -> re-run installer, choose Built-in calendars
  * No weather                -> check WiFi/network; weather retries automatically
  * Want to change anything   -> just re-run:  ~/install.sh

The dashboard starts fullscreen after the next reboot.
NOTE
else
cat <<NOTE

WHAT YOU SHOULD SEE: after a reboot the device logs into LightDM using the
'$KIOSK_SESSION' X11 session and launches the dashboard fullscreen. To check
it locally, open http://localhost:8090 on the device.

DEBIAN x86 / NON-PI NOTES:
  * This installer does not edit Raspberry Pi /boot/config.txt display options.
  * Kiosk mode expects X11 + LightDM + Openbox, with LXDE only used as a
    fallback/full-desktop session. Package setup installs the needed pieces on
    Debian x86/Trixie if they were missing.
  * If the device boots to a different desktop or Wayland session, choose the
    dashboard-openbox or Openbox session in LightDM once, or re-run Dashboard service from the installer.
  * Add calendars by dropping .ics files in $CAL_DIR and running:
        $BIN_DIR/gen-calendars.sh

IF SOMETHING LOOKS WRONG:
  * Login screen appears      -> check /etc/lightdm/lightdm.conf autologin lines
  * Desktop appears instead   -> check ~/.config/lxsession/*/autostart and
                                 ~/.config/autostart/dashboard-kiosk.desktop
  * Black/frozen screen       -> connect a keyboard, press Ctrl+Alt+F2, sign in,
                                 then run: ~/install.sh --doctor
                                 For a system/session repair: ~/install.sh --repair --system
  * Dashboard but no events   -> re-run installer, choose Built-in calendars
  * No weather                -> check network; weather retries automatically

The dashboard starts fullscreen after the next reboot.
NOTE
fi
fi

# A failed final readiness check or a recorded optional-stage failure must not
# be reported as a successful installation. The global EXIT reminder names the
# exact high-level stage for easy recovery.
if [ "$INSTALLER_FINAL_READY" != "1" ] || [ "${#INSTALL_STEP_FAILURES[@]}" -gt 0 ]; then
  exit 1
fi
offer_reboot_after_system_changes
