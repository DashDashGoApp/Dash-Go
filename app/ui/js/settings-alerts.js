/* =====================================================================
   ====================  NWS WEATHER ALERTS  ===========================
   ===================================================================== */
const SEV_RANK={extreme:4,severe:3,moderate:2,minor:1,unknown:0};
function sevRank(sv){ return SEV_RANK[String(sv||"").toLowerCase()]||0; }
// Filter NWS GeoJSON features to active, sufficiently-severe alerts, sorted
// most-severe first then soonest-ending. Pure function (testable).
function activeAlerts(features, now, minSeverity){
  const min=sevRank(minSeverity||"minor")||1;
  const out=[];
  for(const f of (features||[])){
    const p=f&&f.properties; if(!p) continue;
    const endsRaw=p.ends||p.expires;
    const ends=endsRaw?new Date(endsRaw):null;
    if(ends && !(ends>now)) continue;                  // already over
    if(sevRank(p.severity)<min) continue;
    out.push({
      event:p.event||"Weather Alert",
      headline:p.headline||"",
      desc:p.description||"",
      instruction:p.instruction||"",
      area:p.areaDesc||"",
      severity:(String(p.severity||"unknown").toLowerCase()),
      ends,
    });
  }
  out.sort((a,b)=> sevRank(b.severity)-sevRank(a.severity)
                || ((a.ends&&b.ends)?(a.ends-b.ends):(a.ends?-1:b.ends?1:0)));
  return out;
}
async function loadAlerts(){
  if(typeof deferDashboardWork==="function" && deferDashboardWork("alerts-refresh",()=>loadAlerts())) return;
  if(loadAlerts._busy) return;
  loadAlerts._busy=true;
  try{
  const cfg=CONFIG.weatherAlerts;
  if(!cfg || !cfg.enabled){
    ALERTS=[];
    const paint=()=>renderAlerts();
    if(!(typeof deferDashboardWork==="function" && deferDashboardWork("alerts-render",paint))) paint();
    return;
  }
  try{
    const res=await fetch(CONFIG.nwsApi+"/alerts/active?point="+
                          CONFIG.lat+","+CONFIG.lon,
                          {headers:{accept:"application/geo+json"}});
    if(!res.ok) throw new Error("HTTP "+res.status);
    const j=await res.json();
    ALERTS=activeAlerts(j.features, new Date(), cfg.minSeverity);
    ALERT_IDX=0;
  }catch(err){
    // Offline / non-US: keep what we have but drop anything now expired, so a
    // dead network can't pin an old warning on screen past its end time.
    console.warn("alerts fetch failed",err);
    ALERTS=ALERTS.filter(a=>!a.ends || a.ends>new Date());
  }
  const paint=()=>{ renderAlerts(); applyNightDim(); };
  if(!(typeof deferDashboardWork==="function" && deferDashboardWork("alerts-render",paint))) paint();
  } finally { loadAlerts._busy=false; }
}
function fmtUntil(ends){
  if(!ends) return "";
  const now=new Date();
  return " · until "+(sameDay(ends,now)?"":FMT.popDay.format(ends)+" ")+FMT.time.format(ends);
}
function alertsMuted(now){ return (now||Date.now()) < (SETTINGS.alertsMutedUntil||0); }
// Pure (testable): what the banner should show right now.
//  "hidden" — no alerts, or fully muted from the control overlay
//  "docked" — user pressed HIDE: only the corner tab shows. A NEW alert of
//             HIGHER severity than what was docked pops the bar back out.
//  "bar"    — the full banner
function alertDisplayMode(alerts,settings,now){
  const hasPreview=alerts.some(a=>a&&a._test);
  if(!alerts.length || (!hasPreview && (now||Date.now())<(settings.alertsMutedUntil||0))) return "hidden";
  const maxRank=Math.max(...alerts.map(a=>sevRank(a.severity)));
  if(!hasPreview && settings.alertsDocked && maxRank<=(settings.alertsDockedRank||0)) return "docked";
  return "bar";
}
let ALERT_ROTATE_TIMER=null;
function armAlertRotation(active){
  if(!active){
    if(ALERT_ROTATE_TIMER){ clearInterval(ALERT_ROTATE_TIMER); ALERT_ROTATE_TIMER=null; }
    return;
  }
  if(ALERT_ROTATE_TIMER) return;
  ALERT_ROTATE_TIMER=setInterval(()=>{
    if(typeof chalkboardFocusActive==="function" && chalkboardFocusActive()){
      if(typeof deferDashboardWork==="function") deferDashboardWork("alerts-render",()=>renderAlerts());
      return;
    }
    if(ALERTS.length>1){ ALERT_IDX=(ALERT_IDX+1)%ALERTS.length; renderAlerts(); }
    else armAlertRotation(false);
  },8000);
}
function renderAlerts(){
  const bar=$("#alertbar"), tab=$("#alerttab"); if(!bar||!tab) return;
  const mode=alertDisplayMode(ALERTS,SETTINGS);
  armAlertRotation(mode==="bar" && ALERTS.length>1);
  if(mode==="hidden"){ bar.classList.remove("show"); tab.classList.remove("show"); return; }
  const sevClass=a=>"sev-"+(SEV_RANK[a.severity]!==undefined?a.severity:"unknown");
  const top=ALERTS[0];   // most severe (list is sorted)
  if(mode==="docked"){
    bar.classList.remove("show");
    tab.className=sevClass(top); tab.classList.add("show");
    tab.textContent=ALERTS.length>1 ? ALERTS.length+" ALERTS" : "ALERT";
    tab.onclick=()=>{ SETTINGS.alertsDocked=false; postSettings(); renderAlerts(); };
    return;
  }
  tab.classList.remove("show");
  const a=ALERTS[ALERT_IDX%ALERTS.length];
  const key=a.event+"|"+a.ends+"|"+ALERT_IDX%ALERTS.length+"|"+ALERTS.length;
  if(bar._key===key && bar.classList.contains("show")) return;  // nothing changed
  bar._key=key;
  bar.className=sevClass(a); bar.classList.add("show");
  const count=ALERTS.length>1 ? "  ("+((ALERT_IDX%ALERTS.length)+1)+"/"+ALERTS.length+")" : "";
  bar.innerHTML="";
  const txt=el("span","abtxt",a.event.toUpperCase()+fmtUntil(a.ends)+count);
  txt.addEventListener("click",()=>showAlertPopup(a));
  const dk=el("button","abdock","HIDE");
  dk.addEventListener("click",(e)=>{
    e.stopPropagation();
    SETTINGS.alertsDocked=true;
    SETTINGS.alertsDockedRank=Math.max(...ALERTS.map(x=>sevRank(x.severity)));
    postSettings(); renderAlerts();
  });
  bar.onclick=null;
  bar.append(txt,dk);
}

function previewAlertBanner(){
  const fake={event:"Test Alert — Preview", severity:"extreme",
    headline:"This is only a preview.", area:"This dashboard",
    desc:"The banner shows live National Weather Service watches and warnings for your location. This preview removes itself in 15 seconds.",
    instruction:"", ends:new Date(Date.now()+15*60000), _test:true};
  ALERTS=[fake,...ALERTS.filter(a=>!a._test)];
  ALERT_IDX=0;
  renderAlerts(); applyNightDim();
  if(typeof closeCtrl==="function") closeCtrl();
  setTimeout(()=>{ ALERTS=ALERTS.filter(a=>!a._test); renderAlerts(); applyNightDim(); },15000);
}
function showAlertPopup(a){
  if(typeof setPopupMode==="function") setPopupMode("");
  $("#poptitle").textContent=a.event;
  $("#popwhen").innerHTML=`<span class="wstart">${escapeHTML(
    a.severity.charAt(0).toUpperCase()+a.severity.slice(1)+fmtUntil(a.ends))}</span>`;
  const body=$("#popbody"); body.innerHTML="";
  if(a.area){ const r=el("div","row"); r.innerHTML=`<span>Area</span><span>${escapeHTML(a.area)}</span>`; body.appendChild(r); }
  const txt=(a.headline?a.headline+"\n\n":"")+(a.desc||"")+(a.instruction?"\n\n"+a.instruction:"");
  const d=el("div"); d.style.marginTop="10px"; d.style.whiteSpace="pre-wrap";
  d.textContent=txt.trim()||"No details provided.";
  body.appendChild(d);
  openScrim();
}
