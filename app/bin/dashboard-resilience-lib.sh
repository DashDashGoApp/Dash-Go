#!/bin/sh
# Dash-Go resilience helpers. This file owns only tiny cache-state files and
# bounded recovery markers. It never starts services, edits preferences, or
# assumes network access.

_dash_res_now(){ date +%s 2>/dev/null || printf '0\n'; }
_dash_res_cache(){ printf '%s\n' "${CACHE_DIR:-${DASH:-$HOME/dashboard}/cache}"; }
_dash_res_json(){
  # $1 path $2 JSON supplied on stdin; atomic rename keeps readers safe.
  _drj_path="$1"; _drj_tmp="${_drj_path}.tmp.$$"
  mkdir -p "$(dirname "$_drj_path")" 2>/dev/null || return 1
  cat > "$_drj_tmp" 2>/dev/null || return 1
  mv -f "$_drj_tmp" "$_drj_path" 2>/dev/null || { rm -f "$_drj_tmp"; return 1; }
}

dash_clock_confirmed_path(){ printf '%s/clock-confirmed.json\n' "$(_dash_res_cache)"; }
_dash_clock_confirmed_at(){
  _dcca_path="$(dash_clock_confirmed_path)"
  [ -r "$_dcca_path" ] || { printf '0\n'; return 0; }
  python3 - "$_dcca_path" <<'PYJSON' 2>/dev/null
import json,sys
try:
    value=int(json.load(open(sys.argv[1], encoding='utf-8')).get('confirmedAt', 0) or 0)
    print(value if value > 0 else 0)
except Exception:
    print(0)
PYJSON
}
dash_record_clock_confirmed(){
  _drc_source="${1:-ntp}"; _drc_now="$(_dash_res_now)"; _drc_path="$(dash_clock_confirmed_path)"
  case "$_drc_now" in *[!0-9]*|'') return 1;; esac
  printf '{"confirmedAt":%s,"source":"%s"}\n' "$_drc_now" "$_drc_source" | _dash_res_json "$_drc_path"
}
_dash_clock_floor_epoch(){
  _dcf_dash="${DASH:-$HOME/dashboard}"; _dcf_manifest="$_dcf_dash/manifest.json"; _dcf_version="$_dcf_dash/VERSION"; _dcf_epoch=0
  if [ -r "$_dcf_manifest" ]; then
    _dcf_epoch="$(python3 - "$_dcf_manifest" <<'PYJSON' 2>/dev/null
import json,sys
try: print(int(json.load(open(sys.argv[1], encoding='utf-8')).get('buildEpoch', 0) or 0))
except Exception: print(0)
PYJSON
)"
  fi
  case "$_dcf_epoch" in ''|*[!0-9]*) _dcf_epoch=0;; esac
  if [ "$_dcf_epoch" -le 0 ] 2>/dev/null; then _dcf_epoch="$(stat -c %Y "$_dcf_version" 2>/dev/null || printf 0)"; fi
  case "$_dcf_epoch" in ''|*[!0-9]*) printf '0\n'; return 0;; esac
  [ "$_dcf_epoch" -gt 86400 ] 2>/dev/null || { printf '0\n'; return 0; }
  printf '%s\n' $(( _dcf_epoch - 86400 ))
}
dash_ntp_synchronized(){
  command -v timedatectl >/dev/null 2>&1 || return 1
  if command -v timeout >/dev/null 2>&1; then
    _dns_value="$(timeout 3 timedatectl show -p NTPSynchronized --value 2>/dev/null || true)"
  else
    _dns_value="$(timedatectl show -p NTPSynchronized --value 2>/dev/null || true)"
  fi
  [ "$_dns_value" = "yes" ]
}
dash_clock_verified(){
  if dash_ntp_synchronized; then
    dash_record_clock_confirmed ntp >/dev/null 2>&1 || true
    return 0
  fi
  _dcv_now="$(_dash_res_now)"; _dcv_confirmed="$(_dash_clock_confirmed_at)"
  case "$_dcv_now:$_dcv_confirmed" in *[!0-9:]*|:*) ;; esac
  if [ "$_dcv_confirmed" -gt 0 ] 2>/dev/null && [ "$_dcv_now" -ge "$_dcv_confirmed" ] 2>/dev/null; then return 0; fi
  _dcv_floor="$(_dash_clock_floor_epoch)"
  if [ "$_dcv_floor" -gt 0 ] 2>/dev/null && [ "$_dcv_now" -ge "$_dcv_floor" ] 2>/dev/null; then
    dash_record_clock_confirmed floor >/dev/null 2>&1 || true
    return 0
  fi
  return 1
}
dash_clock_unverified_path(){ printf '%s/clock-unverified\n' "$(_dash_res_cache)"; }
dash_mark_clock_unverified(){ : > "$(dash_clock_unverified_path)" 2>/dev/null || true; }
dash_clear_clock_unverified(){ rm -f "$(dash_clock_unverified_path)" 2>/dev/null || true; }

dash_restart_state_path(){ printf '%s/kiosk-restart-state\n' "$(_dash_res_cache)"; }
dash_clear_kiosk_restart_state(){ rm -f "$(dash_restart_state_path)" 2>/dev/null || true; }
# Record a launch in a bounded rolling window. Return 0 while normal and 1
# once the configured burst is exceeded. Environment values make smoke tests
# fast without changing appliance defaults.
dash_note_kiosk_launch(){
  _drs_path="$(dash_restart_state_path)"; _drs_now="$(_dash_res_now)"
  _drs_window="${DASH_KIOSK_RESTART_WINDOW:-120}"; _drs_burst="${DASH_KIOSK_RESTART_BURST:-5}"
  mkdir -p "$(dirname "$_drs_path")" 2>/dev/null || return 0
  _drs_tmp="${_drs_path}.tmp.$$"
  { [ -f "$_drs_path" ] && awk -v now="$_drs_now" -v win="$_drs_window" '$1 ~ /^[0-9]+$/ && now-$1 <= win {print $1}' "$_drs_path" 2>/dev/null || true; printf '%s\n' "$_drs_now"; } > "$_drs_tmp" 2>/dev/null || return 0
  mv -f "$_drs_tmp" "$_drs_path" 2>/dev/null || true
  _drs_count="$(wc -l < "$_drs_path" 2>/dev/null || printf 0)"
  [ "$_drs_count" -le "$_drs_burst" ] 2>/dev/null
}

dash_safe_mode_state_path(){ printf '%s/safe-mode-state.json\n' "$(_dash_res_cache)"; }
dash_safe_mode_page_path(){ printf '%s/safe-mode-active.html\n' "$(_dash_res_cache)"; }
dash_safe_mode_active(){
  _dsa_path="$(dash_safe_mode_state_path)"; [ -r "$_dsa_path" ] || return 1
  python3 - "$_dsa_path" "$(_dash_res_now)" <<'PY' 2>/dev/null
import json,sys
try:
    x=json.load(open(sys.argv[1], encoding='utf-8'))
    raise SystemExit(0 if x.get('active') and int(x.get('retryAfter', 0) or 0)>int(sys.argv[2]) else 1)
except Exception: raise SystemExit(1)
PY
}
dash_safe_mode_retry_after(){
  python3 - "$(dash_safe_mode_state_path)" <<'PY' 2>/dev/null
import json,sys
try: print(int(json.load(open(sys.argv[1], encoding='utf-8')).get('retryAfter',0) or 0))
except Exception: print(0)
PY
}
dash_enter_safe_mode(){
  _dsm_reason="${1:-dashboard recovery is in progress}"; _dsm_now="$(_dash_res_now)"; _dsm_cooldown="${DASH_SAFE_MODE_COOLDOWN:-600}"; _dsm_state="$(dash_safe_mode_state_path)"; _dsm_page="$(dash_safe_mode_page_path)"
  mkdir -p "$(dirname "$_dsm_state")" 2>/dev/null || true
  python3 - "$_dsm_state" "$_dsm_page" "${DASH:-$HOME/dashboard}/ui/safe-mode.html" "$_dsm_now" "$_dsm_cooldown" "$_dsm_reason" "$(hostname 2>/dev/null || printf dash-go)" <<'PY'
import html,json,os,shutil,sys,tempfile
state,page,template,now,cooldown,reason,host=sys.argv[1:]
now=int(now); cooldown=max(1,int(cooldown))
payload={"active":True,"level":"recovering","state":"safe-mode","reason":reason[:220],"started":now,"retryAfter":now+cooldown,"updated":now,"host":host}
tmp=state+".tmp"
os.makedirs(os.path.dirname(state),exist_ok=True)
with open(tmp,"w",encoding="utf-8") as f: json.dump(payload,f,indent=2); f.write("\n")
os.replace(tmp,state)
try: text=open(template,encoding="utf-8").read()
except Exception: text="<html><body><h1>Dashboard is recovering</h1><p>{{REASON}}</p><p>{{HOST}}</p></body></html>"
text=text.replace("{{REASON}}",html.escape(reason[:220])).replace("{{HOST}}",html.escape(host)).replace("{{RETRY_AFTER}}",str(now+cooldown))
tmp=page+".tmp"
with open(tmp,"w",encoding="utf-8") as f: f.write(text)
os.replace(tmp,page)
PY
}
dash_clear_safe_mode(){ rm -f "$(dash_safe_mode_state_path)" "$(dash_safe_mode_page_path)" 2>/dev/null || true; dash_clear_kiosk_restart_state; }

dash_network_likely_available(){
  # A default route is high-confidence; absence is inconclusive on unusual
  # setups, so return success unless the kernel explicitly exposes only loopback.
  [ -r /proc/net/route ] || return 0
  awk '$1 != "lo" && $2 == "00000000" {found=1} END {exit found?0:1}' /proc/net/route 2>/dev/null && return 0
  if command -v ip >/dev/null 2>&1; then ip route get 1.1.1.1 >/dev/null 2>&1 && return 0; fi
  return 1
}
dash_write_network_state(){
  _dwn_state="$(_dash_res_cache)/network-state.json"; _dwn_now="$(_dash_res_now)"
  if dash_network_likely_available; then _dwn_level=ok; _dwn_reason="network route available"; else _dwn_level=degraded; _dwn_reason="no network route currently visible"; fi
  printf '{"level":"%s","reason":"%s","updated":%s}\n' "$_dwn_level" "$_dwn_reason" "$_dwn_now" | _dash_res_json "$_dwn_state"
  [ "$_dwn_level" = ok ]
}
