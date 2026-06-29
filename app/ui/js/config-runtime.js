// 01-config-09-runtime.js — apply themes and installer/local overrides.
"use strict";
// Theme variables are replaced in one dedicated stylesheet operation. Runtime inline
// sizing values remain on :root and are never cleared during a color swap.
const THEME_VARS=(()=>{const v=new Set();for(const t of Object.values(THEMES))for(const k of Object.keys(t))v.add(k);return [...v];})();
let CURRENT_THEME="basic";
// Early source modules load before 08-settings-00-runtime.js declares its lexical
// SETTINGS binding. Read the deliberately mirrored window property here so an
// early visual/theme path cannot enter that binding's temporal dead zone.
function dashboardRuntimeSettings(){
  const w=typeof window!=="undefined"?window:null;
  const s=w&&w.DASHGO_RUNTIME_SETTINGS;
  return s&&typeof s==="object"?s:null;
}
function dashboardThemeStyle(){
  let node=document.getElementById("dashboard-theme-vars");
  if(!node){node=document.createElement("style");node.id="dashboard-theme-vars";document.head.appendChild(node);}return node;
}
function themeCssText(theme){return ":root{"+Object.entries(theme||{}).map(([k,v])=>k+":"+String(v)+";").join("")+"}";}
function applyTheme(name){
  const t=THEMES[name];if(!t)return false;
  dashboardThemeStyle().textContent=themeCssText(t);
  document.documentElement.setAttribute("data-theme",name);CURRENT_THEME=name;
  // Theme color application must remain safe during the early config-local pass.
  // Seasonal decor depends on runtime settings, which initialize later in the
  // sorted bundle, so reconcile it on the next frame only after that module is
  // available. The reconciliation path patches decor nodes only; it never forces
  // a calendar rebuild for an ordinary color-theme swap.
  if(dashboardRuntimeSettings()&&typeof scheduleSeasonalDecorReconcile==="function")scheduleSeasonalDecorReconcile();
  return true;
}

// CACHE-PROOF live theme: the first check reads config.local.js, then later
// checks use a conditional HEAD request. The local Go server supplies a
// content revision for this one mutable file, so an unchanged minute costs no
// JavaScript download or parse while set-theme.sh still appears within the
// existing one-minute window. A forced check is used after Control writes a
// theme so it applies immediately. A ?theme= URL override always wins.
let _urlThemeOverride=false;
let _themeConfigRevision="";
let _themeCheckBusy=false;
try{ _urlThemeOverride=!!new URLSearchParams(location.search).get("theme"); }catch(e){}
async function checkTheme(force){
  if(_urlThemeOverride || _themeCheckBusy) return;
  _themeCheckBusy=true;
  try{
    const path="config/config.local.js";
    if(!force && _themeConfigRevision){
      const probe=await fetch(path,{method:"HEAD",headers:{"If-None-Match":_themeConfigRevision},cache:"no-store"});
      if(probe.status===304) return;
      if(!probe.ok) return;
      const revision=probe.headers.get("ETag")||"";
      // A compatible/older local server may reply 200 to conditional HEAD.
      // Its matching revision is still enough to skip a body download.
      if(revision && revision===_themeConfigRevision) return;
    }
    const res=await fetch(path+"?t="+Date.now(),{cache:"no-store"});
    if(!res.ok) return;
    _themeConfigRevision=res.headers.get("ETag")||"";
    const m=(await res.text()).match(/theme:\s*"([^"]*)"/);
    const name=m?m[1]:"basic";
    if(name!==CURRENT_THEME && THEMES[name]) applyTheme(name);
  }catch(err){ /* transient — next minute */ }
  finally{ _themeCheckBusy=false; }
}

(function applyLocal(){
  const L = (typeof window!=="undefined") && window.DASHBOARD_LOCAL;
  // Theme can come from ?theme= in the URL (live override) or config.local.js.
  let theme = (L && L.theme) || "basic";
  try{ const u=new URLSearchParams(location.search); if(u.get("theme")) theme=u.get("theme"); }catch(e){}
  applyTheme(theme);
  if(!L) return;
  for(const k of ["lat","lon","tempUnit","windUnit","locationName",
                  "nightDim","staleCalHours","showBirthdaysOnCalendar",
                  "weatherAlerts","showUV","showAQI","clock24","showSeconds",
                  "weeksAbove","weeksBelow","firstDayOfWeek","showInteractiveMaps","mapImageStyle","profile","pauseWhileOpen",
                  "wxApi","aqApi","nwsApi","apiKey","weatherProviders","weatherProviderKeys","demoMode",
                  "weatherPrimary","fontPreset","weatherIconStyle","seasonalDecor"]){
    if(L[k]!==undefined) CONFIG[k]=L[k];
  }
  // Existing high-performance installs may have a profile in config.local.js
  // from before interactive maps existed. If no explicit interactive-map
  // preference is present, enable it automatically for the profiles that the
  // installer now treats as capable enough. A saved settings.json value still
  // wins later in loadSettings(), so users can turn it off permanently.
  // Keep "maximum" here as a legacy alias for existing config.local.js files;
  // the server normalizes it to Enhanced for Dashboard Control and profile saves.
  const highPerfProfiles = new Set(["enhanced","maximum","x86","x86_64","amd64","debian-x86","debian-x86-trixie"]);
  if(L.showInteractiveMaps===undefined && highPerfProfiles.has(String(CONFIG.profile||"").toLowerCase())){
    CONFIG.showInteractiveMaps=true;
  }
  if(L.demoMode!==undefined){
    CONFIG.demoMode=!!L.demoMode;
    document.documentElement.classList.toggle("demo-mode",!!L.demoMode);
  }
  if(Number.isFinite(+L.rowHeight)){
    CONFIG.rowHeight=+L.rowHeight;
    document.documentElement.style.setProperty("--dash-rowheight-preference",CONFIG.rowHeight+"px");
  }
  if(Number.isFinite(+L.sidebarWidth)){
    CONFIG.sidebarWidth=+L.sidebarWidth;
    document.documentElement.style.setProperty("--dash-sidebarwidth-preference",CONFIG.sidebarWidth+"px");
  }
  // Retired performance controls are fixed automatic behavior, even when an
  // old config.local.js still contains their former keys.
  CONFIG.refreshCalMinutes=10;
  CONFIG.complimentSeconds=18;
  CONFIG.complimentFadeMs=750;
  CONFIG.showEventMaps=true;
  CONFIG.pixelShift=2;
  CONFIG.radarProvider="auto";
  document.documentElement.style.setProperty("--compfade","750ms");
  if(Array.isArray(L.birthdays)){
    for(const b of L.birthdays){
      if(b && b.name && b.date)
        CONFIG.compliments.push({ text:`Happy Birthday ${b.name}!`, date:b.date, weight:30, _bday:true });
    }
  }
})();
