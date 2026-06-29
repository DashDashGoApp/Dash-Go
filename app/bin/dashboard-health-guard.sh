#!/usr/bin/env bash
# Dash-Go Health Guard: bounded, user-space recovery for deterministic stale state.
# It intentionally never installs packages, rewrites source calendars/settings,
# restarts services, or kills a live browser/kiosk process.
set -u

SCRIPT_ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
DASH="${DASH:-$SCRIPT_ROOT}"
CACHE_DIR="$DASH/cache"
LOG_DIR="$DASH/logs"
CONFIG_DIR="$DASH/config"
# shellcheck source=bin/dashboard-resilience-lib.sh
[ -r "$DASH/bin/dashboard-resilience-lib.sh" ] && . "$DASH/bin/dashboard-resilience-lib.sh"
LOCK="$CACHE_DIR/health-guard.lock"
STATUS="$CACHE_DIR/health-guard-status.json"
LOG="$LOG_DIR/health-guard.log"
STAMP="$(date -Is 2>/dev/null || date '+%Y-%m-%dT%H:%M:%S%z')"
ACTIONS=()
# Deferred work and audit evidence are useful to Doctor, but must not become
# dashboard warnings. WARNINGS is reserved for conditions needing follow-up.
SKIPPED=()
INFO=()
WARNINGS=()

mkdir -p "$CACHE_DIR" "$LOG_DIR" 2>/dev/null || exit 0

log(){ printf '%s %s\n' "$STAMP" "$*" >> "$LOG" 2>/dev/null || true; }
status_is_recent_healthy(){
  # A quiet guard should not rewrite flash/SD-card state every 30 minutes.
  # Keep a one-hour heartbeat so Doctor can still prove the guard is alive.
  [ -f "$STATUS" ] || return 1
  find "$STATUS" -mmin -60 -print -quit 2>/dev/null | grep -q . || return 1
  python3 - "$STATUS" <<'PY'
import json, sys
try:
    payload=json.load(open(sys.argv[1], encoding='utf-8'))
except Exception:
    raise SystemExit(1)
raise SystemExit(0 if payload.get('state') == 'healthy' and not payload.get('actions') and not payload.get('skipped') and not payload.get('warnings') else 1)
PY
}

json_array(){
  python3 - "$@" <<'PY'
import json, sys
print(json.dumps(sys.argv[1:]))
PY
}

write_status(){
  local state="$1" actions_json skipped_json info_json warnings_json reason="" tmp
  if [ "$state" = "healthy" ] && [ "${#ACTIONS[@]}" -eq 0 ] && [ "${#SKIPPED[@]}" -eq 0 ] && [ "${#WARNINGS[@]}" -eq 0 ] && status_is_recent_healthy; then
    return
  fi
  actions_json="$(json_array "${ACTIONS[@]}")"
  skipped_json="$(json_array "${SKIPPED[@]}")"
  info_json="$(json_array "${INFO[@]}")"
  warnings_json="$(json_array "${WARNINGS[@]}")"
  [ "${#WARNINGS[@]}" -gt 0 ] && reason="${WARNINGS[0]}"
  tmp="$STATUS.tmp.$$"
  python3 - "$tmp" "$STAMP" "$state" "$actions_json" "$skipped_json" "$info_json" "$warnings_json" "$reason" <<'PY'
import json, sys
path, updated, state, actions, skipped, info, warnings, reason = sys.argv[1:]
with open(path, 'w', encoding='utf-8') as handle:
    json.dump({
        "updated": updated, "state": state,
        "actions": json.loads(actions), "skipped": json.loads(skipped),
        "info": json.loads(info), "warnings": json.loads(warnings),
        "reason": reason,
    }, handle, indent=2)
    handle.write("\n")
PY
  mv -f "$tmp" "$STATUS" 2>/dev/null || rm -f "$tmp"
}

release_lock(){ rm -rf "$LOCK" 2>/dev/null || true; }
trap release_lock EXIT INT TERM

# Never contend with an installer/repair. A later scheduled run can retry.
if [ -e "$CACHE_DIR/system-update.lock" ] || [ -e "$CACHE_DIR/update.lock" ] || [ -e "$CACHE_DIR/maintenance.lock" ]; then
  SKIPPED+=("update-or-maintenance-active")
  write_status "skipped"
  exit 0
fi

if [ -d "$LOCK" ]; then
  owner="$(cat "$LOCK/pid" 2>/dev/null || true)"
  if [[ "$owner" =~ ^[0-9]+$ ]] && kill -0 "$owner" 2>/dev/null; then
    SKIPPED+=("another-health-guard-run-is-active")
    write_status "skipped"
    exit 0
  fi
  rm -rf "$LOCK" 2>/dev/null || { WARNINGS+=("stale-health-guard-lock-could-not-be-removed"); write_status "warning"; exit 0; }
  ACTIONS+=("removed-stale-health-guard-lock")
fi
mkdir -p "$LOCK" 2>/dev/null || { WARNINGS+=("health-guard-lock-unavailable"); write_status "warning"; exit 0; }
printf '%s\n' "$$" > "$LOCK/pid" 2>/dev/null || true

# A kiosk lock is safe to remove only when its claimed process is gone.
KIOSK_LOCK="$CACHE_DIR/kiosk.lock"
if [ -d "$KIOSK_LOCK" ]; then
  owner="$(cat "$KIOSK_LOCK/pid" 2>/dev/null || true)"
  if ! [[ "$owner" =~ ^[0-9]+$ ]] || ! kill -0 "$owner" 2>/dev/null; then
    if rm -rf "$KIOSK_LOCK" 2>/dev/null; then
      ACTIONS+=("removed-stale-kiosk-lock")
      log "removed stale kiosk lock (owner=${owner:-unknown})"
    fi
  fi
fi

# Maintenance pause is owned by Dash-Go. Do not remove it while an update marker exists.
PAUSE="$CACHE_DIR/kiosk-paused"
if [ -f "$PAUSE" ] && find "$PAUSE" -mmin +30 -print -quit 2>/dev/null | grep -q .; then
  if rm -f "$PAUSE" 2>/dev/null; then
    ACTIONS+=("removed-stale-kiosk-maintenance-pause")
    log "removed stale kiosk maintenance pause"
  fi
fi

# Clear only an explicitly unverified clock marker. Clock checks are cheap and
# this does not wait or alter system time.
if command -v dash_clock_verified >/dev/null 2>&1 && dash_clock_verified; then
  if [ -f "$CACHE_DIR/clock-unverified" ]; then
    dash_clear_clock_unverified
    ACTIONS+=("cleared-clock-unverified-marker")
  fi
fi

# A systemd StartLimit failure means systemd has deliberately stopped a rapid
# restart loop. Record it for Doctor/status; never restart it from this guard.
if command -v systemctl >/dev/null 2>&1 && { command -v timeout >/dev/null 2>&1 && timeout 3 systemctl is-failed --quiet dashboard-server.service 2>/dev/null || { ! command -v timeout >/dev/null 2>&1 && systemctl is-failed --quiet dashboard-server.service 2>/dev/null; }; }; then
  python3 - "$CACHE_DIR/server-restart-state.json" "$STAMP" <<'PY2'
import json,os,sys
p,ts=sys.argv[1:]
t=p+'.tmp'
with open(t,'w',encoding='utf-8') as f: json.dump({'level':'failing','reason':'dashboard-server.service is failed or StartLimit blocked','updated':ts},f,indent=2); f.write('\n')
os.replace(t,p)
PY2
  WARNINGS+=("dashboard-server-service-failed-startlimit")
else
  rm -f "$CACHE_DIR/server-restart-state.json" 2>/dev/null || true
fi

# Network state is persisted so an offline-to-online transition can gently
# nudge normal caches once. The request helpers preserve last-good data and
# have their own provider backoff, so this never becomes a tight retry loop.
was_network=""
if [ -r "$CACHE_DIR/network-state.json" ]; then was_network="$(python3 - "$CACHE_DIR/network-state.json" <<'PY2' 2>/dev/null
import json,sys
try: print(json.load(open(sys.argv[1])).get('level',''))
except Exception: pass
PY2
)"; fi
if command -v dash_write_network_state >/dev/null 2>&1 && dash_write_network_state; then
  if [ "$was_network" = "degraded" ]; then
    if [ -x "$DASH/bin/dashboard-control-server" ]; then
      "$DASH/bin/dashboard-lowprio.sh" "$DASH/bin/dashboard-control-server" --refresh-weather >/dev/null 2>&1 &
      "$DASH/bin/dashboard-lowprio.sh" "$DASH/bin/dashboard-control-server" --update-message-feeds >/dev/null 2>&1 &
      ACTIONS+=("nudged-weather-and-message-refresh-after-network-recovery")
    fi
  fi
fi

# If a static recovery page has been displayed long enough and the local API
# is healthy, release the marker. Kiosk still owns browser/restart decisions.
if [ -r "$CACHE_DIR/safe-mode-state.json" ] && curl -sf --max-time 2 http://127.0.0.1:8090/api/health >/dev/null 2>&1; then
  if python3 - "$CACHE_DIR/safe-mode-state.json" <<'PY2' 2>/dev/null
import json,sys,time
try:
  x=json.load(open(sys.argv[1])); raise SystemExit(0 if x.get('active') and time.time()-float(x.get('started',0))>=120 else 1)
except Exception: raise SystemExit(1)
PY2
  then
    dash_clear_safe_mode
    ACTIONS+=("cleared-safe-mode-after-local-api-recovered")
  fi
fi

# A durable post-update marker is a fallback when the updater could not spawn
# its verifier outside the service cgroup. Start at most one bounded verifier;
# it either confirms the new release or performs one rollback and exits.
if [ -r "$CACHE_DIR/post-update-verify.json" ] && [ -x "$DASH/bin/dashboard-post-update-verify.sh" ]; then
  pending="$(python3 - "$CACHE_DIR/post-update-verify.json" <<'PY2' 2>/dev/null
import json,sys
try: print('1' if json.load(open(sys.argv[1])).get('state')=='pending' else '0')
except Exception: print('0')
PY2
)"
  if [ "$pending" = 1 ] && ! pgrep -f '[d]ashboard-post-update-verify.sh' >/dev/null 2>&1; then
    "$DASH/bin/dashboard-post-update-verify.sh" >/dev/null 2>&1 &
    INFO+=("post-update-runtime-verifier-started")
  fi
fi

# A cheap reader-only freshness state is consumed by /api/health and Doctor.
python3 - "$CACHE_DIR/data-freshness-state.json" "$CACHE_DIR/weather-cache.json" "$CACHE_DIR/events.cache.json" "$CONFIG_DIR/message-cache.json" <<'PY2' >/dev/null 2>&1 || true
import json,os,sys,time
out,weather,events,messages=sys.argv[1:]
now=time.time(); specs=[('weather',weather,90*60),('calendar',events,45*60),('messages',messages,36*60*60)]
items=[]
for name,path,interval in specs:
    try: at=os.path.getmtime(path); age=max(0,int(now-at)); level='ok' if age<=interval*3 else ('degraded' if age<=interval*6 else 'failing')
    except OSError: at=0; age=None; level='unknown'
    items.append({'name':name,'level':level,'lastSuccess':int(at),'ageSeconds':age,'expectedSeconds':interval})
level='ok'
if any(x['level']=='failing' for x in items): level='failing'
elif any(x['level']=='degraded' for x in items): level='degraded'
elif any(x['level']=='unknown' for x in items): level='unknown'
tmp=out+'.tmp'
with open(tmp,'w',encoding='utf-8') as f: json.dump({'level':level,'updated':int(now),'items':items},f,indent=2); f.write('\n')
os.replace(tmp,out)
PY2

# Only clear a weather cache that is provably tied to 0,0 while config now has a real location.
WEATHER="$CACHE_DIR/weather-cache.json"
LOCAL="$CONFIG_DIR/config.local.js"
if [ -f "$WEATHER" ] && [ -f "$LOCAL" ] && command -v python3 >/dev/null 2>&1; then
  if python3 - "$WEATHER" "$LOCAL" <<'PY'
import json, re, sys
weather, local = sys.argv[1:]
try:
    data=json.load(open(weather, encoding='utf-8'))
except Exception:
    raise SystemExit(1)
loc=data.get('location') or data.get('meta', {}).get('location') or {}
lat=loc.get('lat', loc.get('latitude'))
lon=loc.get('lon', loc.get('lng', loc.get('longitude')))
try:
    cached_zero=float(lat) == 0.0 and float(lon) == 0.0
except (TypeError, ValueError):
    cached_zero=False
text=open(local, encoding='utf-8', errors='replace').read()
values=[]
for name in ('lat', 'latitude', 'lon', 'lng', 'longitude'):
    found=re.search(r'\b%s\s*:\s*([-+]?\d+(?:\.\d+)?)' % re.escape(name), text)
    if found:
        values.append(float(found.group(1)))
configured=any(value != 0.0 for value in values)
raise SystemExit(0 if cached_zero and configured else 1)
PY
  then
    if mv "$WEATHER" "$WEATHER.health-guard-bad-$(date '+%Y%m%d-%H%M%S')" 2>/dev/null; then
      ACTIONS+=("quarantined-zero-zero-weather-cache")
      log "quarantined weather cache generated for 0,0 after a real location was configured"
    fi
  fi
fi

state="healthy"
if [ "${#WARNINGS[@]}" -gt 0 ]; then
  state="warning"
elif [ "${#ACTIONS[@]}" -gt 0 ]; then
  state="recovered"
elif [ "${#SKIPPED[@]}" -gt 0 ]; then
  # A scheduled guard that deliberately deferred itself is diagnostic only.
  state="skipped"
fi
write_status "$state"
for action in "${ACTIONS[@]:-}"; do [ -n "$action" ] && log "$action"; done
exit 0
