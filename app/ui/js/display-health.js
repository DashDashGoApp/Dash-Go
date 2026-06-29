/* =====================================================================
   ====================  DATA HEALTH + NIGHT DIM  ======================
   ===================================================================== */
// Stale-data indicator (bottom-right). Data freshness remains separate from
// appliance health so a user may temporarily hide only a bounded data reminder.
let lastCalOK=0, lastWxOK=0, lastMessageOK=0, CAL_ISSUES=[];
let DEVICE_HEALTH=null;
let ACTIVE_STALE_WARNINGS=[];
let MESSAGE_FEED_FRESHNESS_ENABLED=null;
let ALERTS=[], ALERT_IDX=0;     // active NWS alerts (sorted by severity)
let AQI=null;                   // latest air-quality response
const BOOT_TS=Date.now();
const MESSAGE_CHECK_SOON_MS=36*60*60000;
function fmtAge(ms){
  const m=Math.round(ms/60000);
  if(m<60) return m+"m";
  const h=Math.round(m/60);
  if(h<48) return h+"h";
  return Math.round(h/24)+"d";
}
function normalizeEpochMs(raw){
  let value=Number(raw)||0;
  if(value>0 && value<100000000000) value*=1000;
  return value;
}
function staleClockMinutes(raw){ const m=String(raw||"").match(/^(\d{1,2}):(\d{2})$/); if(!m)return null;const h=+m[1],n=+m[2];return h>=0&&h<24&&n>=0&&n<60?h*60+n:null; }
function staleSleepGrace(now,refreshMin){
  const settings=typeof dashboardRuntimeSettings==="function"?dashboardRuntimeSettings():null;
  if((settings&&settings.displaySleepEnabled===false)||(CONFIG&&CONFIG.displaySleepEnabled===false))return false;
  const off=staleClockMinutes((settings&&settings.displaySleepOff)||CONFIG.displaySleepOff),on=staleClockMinutes((settings&&settings.displaySleepOn)||CONFIG.displaySleepOn);if(off==null||on==null||off===on)return false;
  const d=new Date(now),m=d.getHours()*60+d.getMinutes(),asleep=off<on?(m>=off&&m<on):(m>=off||m<on);if(asleep)return true;
  const wake=new Date(d);wake.setHours(Math.floor(on/60),on%60,0,0);if(d<wake)wake.setDate(wake.getDate()-1);return now-wake.getTime()<Math.max(1,refreshMin)*2*60000;
}
function setMessageFeedFreshnessEnabled(enabled){
  if(typeof enabled==="boolean"){ MESSAGE_FEED_FRESHNESS_ENABLED=enabled; return; }
  MESSAGE_FEED_FRESHNESS_ENABLED=Array.isArray(enabled) ? enabled.length>0 : null;
}
function healthMonitorsMessageFeeds(){
  if(typeof MESSAGE_FEED_FRESHNESS_ENABLED==="boolean") return MESSAGE_FEED_FRESHNESS_ENABLED;
  const facts=Array.isArray(DEVICE_HEALTH&&DEVICE_HEALTH.facts)?DEVICE_HEALTH.facts:[];
  return facts.some(f=>f&&f.name==="messages"&&f.tier==="data");
}
function healthWarningMuted(key,now){
  const state=DEVICE_HEALTH&&DEVICE_HEALTH.warningSilences;
  const record=state&&typeof state==="object"?state[key]:null;
  return Number(record&&record.until||0)>now;
}
function staleWarning(key,text,opts){
  return {key:String(key||""),text:String(text||""),tier:(opts&&opts.tier)||"data",silenceable:!!(opts&&opts.silenceable),ageMs:Number(opts&&opts.ageMs||0)};
}
function addStaleWarning(entries,key,text,opts){
  if(!text) return;
  entries.push(staleWarning(key,text,opts));
}
const DEVICE_WARNING_SILENCEABLE_KEYS=new Set(["storage","clock","config","update","postUpdate","healthGuard"]);
function deviceHealthWarningEntries(){
  const facts=Array.isArray(DEVICE_HEALTH&&DEVICE_HEALTH.facts)?DEVICE_HEALTH.facts:[];
  return facts.filter(f=>f&&f.tier==="device"&&(f.level==="degraded"||f.level==="failing")).map(f=>{
    const key=String(f.name||"device");
    const text=String(f.reason||((key||"device")+" is "+String(f.level||"degraded"))).trim();
    return staleWarning(key,text,{tier:"device",silenceable:f.level==="degraded"&&DEVICE_WARNING_SILENCEABLE_KEYS.has(key)});
  });
}
function updateStale(){
  const e=$("#stale"); if(!e) return;
  const now=Date.now(),entries=[],calMin=Math.max(1,Number(CONFIG.refreshCalMinutes)||10),wxMin=Math.max(1,typeof effectiveWeatherRefreshMinutes==="function"?effectiveWeatherRefreshMinutes():Number(CONFIG.refreshWxMinutes)||30);
  // A low-power box can take several cadence windows to load its first caches.
  const booted=now-BOOT_TS>Math.max(10,wxMin*2,calMin*2)*60000;
  const calHasData=(Array.isArray(EVENTS)&&EVENTS.length>0)||!!(EVENT_CACHE_INFO&&EVENT_CACHE_INFO.using);
  const wxHasData=!!(WX&&WX.current);
  const calMax=calMin*8*60000,calQuiet=staleSleepGrace(now,calMin);
  if(!calQuiet && (lastCalOK ? (now-lastCalOK>calMax) : (booted&&!calHasData)))
    addStaleWarning(entries,"calendar","cal "+(lastCalOK?fmtAge(now-lastCalOK):"–"),{tier:"data",silenceable:true,ageMs:lastCalOK?now-lastCalOK:0});
  if(CAL_ISSUES.length&&!calQuiet)
    addStaleWarning(entries,"calendar",CAL_ISSUES.length===1 ? CAL_ISSUES[0]+" stale" : CAL_ISSUES.length+" cals stale",{tier:"data",silenceable:true});
  const wxMax=wxMin*8*60000,wxQuiet=staleSleepGrace(now,wxMin);
  if(!wxQuiet && (lastWxOK ? (now-lastWxOK>wxMax) : (booted&&!wxHasData)))
    addStaleWarning(entries,"weather","weather updated "+(lastWxOK?fmtAge(now-lastWxOK):"–")+" ago",{tier:"data",silenceable:true,ageMs:lastWxOK?now-lastWxOK:0});
  // Current source selection is authoritative. An old cache alone never makes
  // defaults, personal, birthday, temporary, or scheduled messages stale.
  if(healthMonitorsMessageFeeds() && lastMessageOK && now-lastMessageOK>MESSAGE_CHECK_SOON_MS)
    addStaleWarning(entries,"messages","messages updated "+fmtAge(now-lastMessageOK)+" ago",{tier:"data",silenceable:true,ageMs:now-lastMessageOK});
  const dev=(DEVICE_HEALTH&&DEVICE_HEALTH.device)||"ok";
  const deviceEntries=deviceHealthWarningEntries();
  if(deviceEntries.length) entries.push(...deviceEntries);
  else {
    const sline=(DEVICE_HEALTH && typeof DEVICE_HEALTH.statusLine==="string") ? DEVICE_HEALTH.statusLine.trim() : "";
    const cleanLine=(!sline || sline==="<nil>" || sline==="All systems normal") ? "" : sline;
    if((dev==="failing"||dev==="degraded") && cleanLine) addStaleWarning(entries,"device",cleanLine,{tier:"device",silenceable:false});
  }
  const visible=entries.filter(entry=>!healthWarningMuted(entry.key,now));
  ACTIVE_STALE_WARNINGS=visible;
  if(visible.length){
    const text="Check soon: "+visible.map(entry=>entry.text).join(" · ");
    e.textContent=text; e.hidden=false; e.classList.add("show"); e.classList.toggle("severe",dev==="failing");
    e.dataset.warningKeys=[...new Set(visible.map(entry=>entry.key))].join(",");
    e.dataset.silenceableWarningKeys=[...new Set(visible.filter(entry=>entry.silenceable).map(entry=>entry.key))].join(",");
    e.setAttribute("aria-label",text+(visible.some(entry=>entry.silenceable)?". Triple tap to open temporary silence controls.":". Critical device notices remain visible."));
  }else{
    e.textContent=""; e.hidden=true; e.classList.remove("show","severe","tap-pulse");
    delete e.dataset.warningKeys; delete e.dataset.silenceableWarningKeys;
    e.setAttribute("aria-label","Check soon status");
  }
  // The Family Board urgent surface is a separate flexible lane. Reflow it
  // after operational warning visibility changes so neither control overlaps
  // or steals the other's touch target at compact widths.
  if(typeof window.familyBoardFooterReflow==="function")window.familyBoardFooterReflow();
  // A visible warning pill shares the footer's horizontal space. Lite keeps a
  // cached message-box snapshot, so notify it only after this visibility pass;
  // the capture is coalesced and does not run during ordinary message rotation.
  if(typeof complimentLiteNotifyLayoutChange==="function")complimentLiteNotifyLayoutChange("warning-pill");
}
async function loadDeviceHealth(){
  try{
    const r=await fetch("/api/health",{cache:"no-store"});
    if(r.ok){ DEVICE_HEALTH=await r.json(); MESSAGE_FEED_FRESHNESS_ENABLED=null; }
  }catch(_){ /* optional local status is unavailable during early boot */ }
  updateStale();
}

// Night dimming uses one permanent opaque overlay, never a body brightness filter.
// Whole-page filters force an expensive compositing/repaint path on Lite WebKit.
let NIGHT_DIM_KEY="";
function nightDimOpacity(hour,alerts){
  const nd=CONFIG.nightDim,settings=typeof dashboardRuntimeSettings==="function"?dashboardRuntimeSettings():null;
  if((settings&&settings.nightDimEnabled===false)||!nd||!nd.level||nd.level>=1)return 0;
  if((alerts||[]).some(a=>a.severity==="extreme"))return 0;
  const active=nd.start>nd.end?(hour>=nd.start||hour<nd.end):(hour>=nd.start&&hour<nd.end);
  return active?Math.max(0,Math.min(.92,1-Number(nd.level))):0;
}
function applyNightDim(){
  const overlay=document.getElementById("nightdim"),opacity=nightDimOpacity(new Date().getHours(),ALERTS),key=opacity.toFixed(3);
  // Migration cleanup: old releases left this inline value behind after a live update.
  if(document.body&&document.body.style&&document.body.style.filter)document.body.style.removeProperty("filter");
  if(!overlay||key===NIGHT_DIM_KEY)return;
  NIGHT_DIM_KEY=key;overlay.style.setProperty("--nightdim-opacity",key);overlay.classList.toggle("show",opacity>0);
}

// Burn-in mitigation: translate <body> around a small closed loop, one step
// per minute. Integer offsets keep text crisp; the jump is a single cheap
// composite (transform isn't in body's CSS transition list, so no slow
// sub-pixel animation). Returns to (0,0) within each cycle.
const SHIFT_PATTERN=[[0,0],[1,0],[2,0],[2,1],[2,2],[1,2],[0,2],[-1,2],[-2,2],[-2,1],
                     [-2,0],[-2,-1],[-2,-2],[-1,-2],[0,-2],[1,-2],[2,-2],[2,-1],[1,1],[-1,-1]];
function shiftOffset(step,maxPx){
  if(!maxPx) return [0,0];
  const [x,y]=SHIFT_PATTERN[step%SHIFT_PATTERN.length];
  const c=v=>Math.max(-maxPx,Math.min(maxPx,v));
  return [c(x),c(y)];
}
let _shiftStep=0;
function applyPixelShift(){
  const px=Math.max(1,CONFIG.pixelShift|0);
  _shiftStep++;
  const [x,y]=shiftOffset(_shiftStep,px);
  document.body.style.transform="translate("+x+"px,"+y+"px)";
}
