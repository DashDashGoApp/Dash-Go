// 08-settings-alerts-scroll.js — generated from dashboard.js for maintainability.
/* =====================================================================
   ====================  RUNTIME SETTINGS (settings.json)  =============
   Written by dashboard-control-server when the on-screen overlay changes
   something; read here at boot so choices survive the nightly browser
   restart. The overlay also applies changes live via applySettings().
   ===================================================================== */
let SETTINGS={ alertsMutedUntil:0, alertsDocked:false, alertsDockedRank:0,
               nightDimEnabled:true, pixelShiftEnabled:true,
               rowHeight:210, sidebarWidth:380, weatherDetailMode:"expanded",
               weeksAbove:2, weeksBelow:10, firstDayOfWeek:0, showIsoWeekNumbers:false,
               displaySleepEnabled:false, displaySleepOff:"22:30", displaySleepOn:"06:00",
               fontPreset:"default", weatherIconStyle:"soft", seasonalDecor:"off",
               calendarTextWeight:700, calendarTextSize:0, calendarTextFont:"default",
               clockTextSize:0, clockTextWeight:600, clockTextFont:"default",
               weatherTextSize:0, weatherTextWeight:600, weatherTextFont:"default",
               messageTextSize:0, messageTextWeight:800, messageTextFont:"default" };
// Early modules use this mirrored object instead of touching the later lexical
// SETTINGS binding. Keep it synchronized whenever runtime settings are replaced.
function syncDashboardRuntimeSettings(){
  if(typeof window!=="undefined") window.DASHGO_RUNTIME_SETTINGS=SETTINGS;
}
syncDashboardRuntimeSettings();
const FONT_PRESETS={
  default:{label:"Default",summary:"Bundled Libre Franklin + DM Mono.",sans:"'Libre Franklin','Libre Franklin Fallback',system-ui,'Noto Sans','DejaVu Sans',sans-serif",mono:"'DM Mono','DM Mono Fallback','DejaVu Sans Mono',ui-monospace,monospace"},
  system:{label:"System",summary:"Native Pi/Debian UI stack.",sans:"system-ui,'Noto Sans','DejaVu Sans',sans-serif",mono:"ui-monospace,'Noto Sans Mono','DejaVu Sans Mono',monospace"},
  rounded:{label:"Rounded",summary:"Nunito when downloaded; Libre Franklin until then.",sans:"'Nunito','Libre Franklin','Libre Franklin Fallback',system-ui,sans-serif",mono:"'Nunito','Libre Franklin','Libre Franklin Fallback',system-ui,sans-serif"},
  readable:{label:"Readable",summary:"Atkinson Hyperlegible when downloaded; clear system fallback until then.",sans:"'Atkinson Hyperlegible','DejaVu Sans',system-ui,sans-serif",mono:"'Atkinson Hyperlegible','DejaVu Sans',system-ui,sans-serif"},
  mono:{label:"Mono",summary:"Bundled DM Mono for structured text.",sans:"'DM Mono','DM Mono Fallback','DejaVu Sans Mono',ui-monospace,monospace",mono:"'DM Mono','DM Mono Fallback','DejaVu Sans Mono',ui-monospace,monospace"}
};
let DASHBOARD_FONT_STATUS={}; const DASHBOARD_FONT_DOWNLOADS=new Map();
function dashboardFontInfo(key){ return DASHBOARD_FONT_STATUS[key]||{state:key==="rounded"||key==="readable"?"missing":"bundled",downloadable:key==="rounded"||key==="readable"}; }
function dashboardFontAvailable(key){ return ["bundled","downloaded","system"].includes(String(dashboardFontInfo(key).state||"")); }
async function refreshDashboardFontStatus(){ try{ const r=await fetch("/api/fonts/status",{cache:"no-store"}); const j=await r.json(); if(r.ok&&j&&j.fonts) DASHBOARD_FONT_STATUS=j.fonts; }catch(_){} return DASHBOARD_FONT_STATUS; }
function reloadDashboardFontFaces(){ const link=document.getElementById("dynfonts"); if(link) link.href="/api/fonts/face.css?v="+encodeURIComponent(CONFIG.version||Date.now())+"&t="+Date.now(); try{ if(document.fonts&&document.fonts.ready) document.fonts.ready.then(()=>applyDashboardTypographySettings()).catch(()=>{}); }catch(_){} }
function downloadDashboardFont(key){
  if(DASHBOARD_FONT_DOWNLOADS.has(key))return DASHBOARD_FONT_DOWNLOADS.get(key);
  const run=(async()=>{ let j; if(typeof api==="function") j=await api("/api/fonts/download","POST",{key}); else { const r=await fetch("/api/fonts/download",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({key})}); j=await r.json().catch(()=>({})); if(!r.ok) throw new Error(j.error||"download failed"); } DASHBOARD_FONT_STATUS=j.fonts||DASHBOARD_FONT_STATUS; reloadDashboardFontFaces(); return j; })();
  DASHBOARD_FONT_DOWNLOADS.set(key,run);
  return run.finally(()=>DASHBOARD_FONT_DOWNLOADS.delete(key));
}
const WEATHER_ICON_STYLES={
  soft:{label:"Soft",summary:"Current softened dashboard SVGs."},
  bold:{label:"Bold",summary:"Thicker shapes for distance."},
  outline:{label:"Outline",summary:"Line-art weather symbols."},
  contrast:{label:"High contrast",summary:"Brighter sun/cloud/rain colors."},
  playful:{label:"Playful",summary:"Rounder, more colorful icons."}
};
const SEASONAL_DECOR_MODES={
  off:{label:"Off",summary:"No holiday decoration layer."},
  subtle:{label:"Subtle",summary:"A few faint SVG accents in empty cells."},
  standard:{label:"Standard",summary:"More visible accents, still content-safe."}
};
function visualChoice(map,value,fallback){ return map[value] ? value : fallback; }
// Dashboard typography is intentionally target-scoped. Legacy fontPreset still
// owns the general dashboard/control baseline, while these values override only
// the user-selected dashboard text area.
const DASHBOARD_TYPOGRAPHY_FONT_KEYS=["calendarTextFont","clockTextFont","weatherTextFont","messageTextFont"];
const DASHBOARD_TYPOGRAPHY_NUMERIC_VALUES={
  calendarTextSize:[-1,-0.5,0,0.5,1], calendarTextWeight:[400,600,700,800,900],
  clockTextSize:[-2,-1,0,1,2], clockTextWeight:[400,500,600,700,800],
  weatherTextSize:[-2,-1,0,1,2], weatherTextWeight:[400,500,600,700,800],
  messageTextSize:[-2,-1,0,1,2], messageTextWeight:[600,700,800,850,900]
};
const DASHBOARD_TYPOGRAPHY_DEFAULTS={
  calendarTextSize:0, calendarTextWeight:700, calendarTextFont:"default",
  clockTextSize:0, clockTextWeight:600, clockTextFont:"default",
  weatherTextSize:0, weatherTextWeight:600, weatherTextFont:"default",
  messageTextSize:0, messageTextWeight:800, messageTextFont:"default"
};
const DASHBOARD_TYPOGRAPHY_EXPLICIT_FONT_KEYS=new Set();
const CALENDAR_TEXT_WEIGHTS={400:"Light",600:"Regular",700:"Default",800:"Strong",900:"Bold"};
const CALENDAR_TEXT_SIZES={"-1":"Compact","-0.5":"Small","0":"Default","0.5":"Large","1":"Extra large"};
function dashboardTypographyNumber(key,value){
  const n=Number(value), allowed=DASHBOARD_TYPOGRAPHY_NUMERIC_VALUES[key]||[];
  return allowed.includes(n)?n:DASHBOARD_TYPOGRAPHY_DEFAULTS[key];
}
function dashboardTypographyFont(value){ return visualChoice(FONT_PRESETS,String(value||""),"default"); }
function dashboardTypographyFontIsExplicit(key){ return DASHBOARD_TYPOGRAPHY_EXPLICIT_FONT_KEYS.has(key); }
function dashboardTypographySetExplicitFont(key,explicit){
  if(explicit) DASHBOARD_TYPOGRAPHY_EXPLICIT_FONT_KEYS.add(key);
  else DASHBOARD_TYPOGRAPHY_EXPLICIT_FONT_KEYS.delete(key);
}
function dashboardTypographyEffectiveFont(key){
  const own=dashboardTypographyFontIsExplicit(key);
  return own?dashboardTypographyFont(SETTINGS[key]):dashboardTypographyFont(SETTINGS.fontPreset||CONFIG.fontPreset||"default");
}
function dashboardTypographyFontLabel(key){ return (FONT_PRESETS[dashboardTypographyEffectiveFont(key)]||FONT_PRESETS.default).label; }
function calendarTextWeight(value){ return dashboardTypographyNumber("calendarTextWeight",value); }
function calendarTextSize(value){ return dashboardTypographyNumber("calendarTextSize",value); }
function calendarTextSizeLabel(value){ return CALENDAR_TEXT_SIZES[String(calendarTextSize(value))]||"Default"; }
function dashboardTypographyLabel(key,value){
  if(key==="calendarTextSize") return calendarTextSizeLabel(value);
  if(key==="calendarTextWeight") return CALENDAR_TEXT_WEIGHTS[String(calendarTextWeight(value))]||"Default";
  const labels={clockTextSize:{"-2":"Compact","-1":"Small","0":"Default","1":"Large","2":"Extra large"},weatherTextSize:{"-2":"Compact","-1":"Small","0":"Default","1":"Large","2":"Extra large"},messageTextSize:{"-2":"Compact","-1":"Small","0":"Default","1":"Large","2":"Extra large"},clockTextWeight:{400:"Light",500:"Regular",600:"Default",700:"Strong",800:"Bold"},weatherTextWeight:{400:"Light",500:"Regular",600:"Default",700:"Strong",800:"Bold"},messageTextWeight:{600:"Light",700:"Regular",800:"Default",850:"Strong",900:"Bold"}};
  const map=labels[key]||{};
  const normalized=key.endsWith("Font")?dashboardTypographyFont(value):dashboardTypographyNumber(key,value);
  return key.endsWith("Font")?(FONT_PRESETS[normalized]||FONT_PRESETS.default).label:(map[String(normalized)]||"Default");
}
function dashboardTypographySummary(keys){
  return dashboardTypographyLabel(keys.size,SETTINGS[keys.size])+" size · "+dashboardTypographyLabel(keys.weight,SETTINGS[keys.weight])+" weight · "+dashboardTypographyFontLabel(keys.font)+" font";
}
function dashboardTypographyClockFont(fontKey){
  const preset=FONT_PRESETS[dashboardTypographyEffectiveFont(fontKey)]||FONT_PRESETS.default;
  return ["default","mono"].includes(dashboardTypographyEffectiveFont(fontKey))?preset.mono:preset.sans;
}
function applyDashboardTypographySettings(){
  const root=document.documentElement;
  const calSize=dashboardTypographyNumber("calendarTextSize",SETTINGS.calendarTextSize);
  const calWeight=dashboardTypographyNumber("calendarTextWeight",SETTINGS.calendarTextWeight);
  root.style.setProperty("--dash-calendar-event-font",(FONT_PRESETS[dashboardTypographyEffectiveFont("calendarTextFont")]||FONT_PRESETS.default).sans);
  root.style.setProperty("--dash-calendar-event-weight",String(calWeight));
  root.style.setProperty("--dash-calendar-event-size-offset",calSize+"px");
  // Retained aliases keep any existing calendar helper styles compatible.
  root.style.setProperty("--calendar-event-weight",String(calWeight));
  root.style.setProperty("--calendar-event-size-offset",calSize+"px");

  const clockSize=dashboardTypographyNumber("clockTextSize",SETTINGS.clockTextSize);
  const clockWeight=dashboardTypographyNumber("clockTextWeight",SETTINGS.clockTextWeight);
  const clockSizes={"-2":[24,72,24],"-1":[26,77,26],"0":[28,82,28],"1":[30,87,30],"2":[32,92,32]}[String(clockSize)]||[28,82,28];
  root.style.setProperty("--dash-clock-font",dashboardTypographyClockFont("clockTextFont"));
  root.style.setProperty("--dash-clock-date-size",clockSizes[0]+"px");
  root.style.setProperty("--dash-clock-time-size",clockSizes[1]+"px");
  root.style.setProperty("--dash-clock-suffix-size",clockSizes[2]+"px");
  root.style.setProperty("--dash-clock-weight",String(clockWeight));

  const weatherSize=dashboardTypographyNumber("weatherTextSize",SETTINGS.weatherTextSize);
  const weatherWeight=dashboardTypographyNumber("weatherTextWeight",SETTINGS.weatherTextWeight);
  const weatherSizes={
    "-2":{big:"clamp(44px,2.55vw,48px)",meta:15,sub:13,day:18,desc:16,temp:19},
    "-1":{big:"clamp(47px,2.72vw,51px)",meta:16,sub:14,day:19,desc:17,temp:20},
    "0":{big:"clamp(50px,2.9vw,54px)",meta:17,sub:15,day:20,desc:18,temp:21},
    "1":{big:"clamp(53px,3.08vw,57px)",meta:18,sub:16,day:21,desc:19,temp:22},
    "2":{big:"clamp(56px,3.25vw,60px)",meta:19,sub:17,day:22,desc:20,temp:23}
  }[String(weatherSize)]||{big:"clamp(50px,2.9vw,54px)",meta:17,sub:15,day:20,desc:18,temp:21};
  const primary={400:400,500:400,600:500,700:600,800:700}[weatherWeight]||500;
  const meta={400:400,500:400,600:400,700:500,800:600}[weatherWeight]||400;
  const emphasis={400:600,500:600,600:700,700:800,800:900}[weatherWeight]||700;
  const weatherPreset=FONT_PRESETS[dashboardTypographyEffectiveFont("weatherTextFont")]||FONT_PRESETS.default;
  root.style.setProperty("--dash-weather-font",weatherPreset.sans);
  root.style.setProperty("--dash-weather-display-font",["default","mono"].includes(dashboardTypographyEffectiveFont("weatherTextFont"))?weatherPreset.mono:weatherPreset.sans);
  root.style.setProperty("--dash-weather-big-size",weatherSizes.big);
  root.style.setProperty("--dash-weather-meta-size",weatherSizes.meta+"px");
  root.style.setProperty("--dash-weather-sub-size",weatherSizes.sub+"px");
  root.style.setProperty("--dash-weather-day-size",weatherSizes.day+"px");
  root.style.setProperty("--dash-weather-description-size",weatherSizes.desc+"px");
  root.style.setProperty("--dash-weather-temp-size",weatherSizes.temp+"px");
  root.style.setProperty("--dash-weather-primary-weight",String(primary));
  root.style.setProperty("--dash-weather-meta-weight",String(meta));
  root.style.setProperty("--dash-weather-emphasis-weight",String(emphasis));
  root.style.setProperty("--dash-weather-secondary-weight",String(weatherWeight));

  const messageWeight=dashboardTypographyNumber("messageTextWeight",SETTINGS.messageTextWeight);
  root.style.setProperty("--dash-message-font",(FONT_PRESETS[dashboardTypographyEffectiveFont("messageTextFont")]||FONT_PRESETS.default).sans);
  root.style.setProperty("--dash-message-weight",String(messageWeight));
  root.style.setProperty("--message-weight",String(messageWeight));
}
function messageTypographySizeMultiplier(){
  const size=dashboardTypographyNumber("messageTextSize",SETTINGS.messageTextSize);
  return ({"-2":0.84,"-1":0.92,"0":1,"1":1.08,"2":1.16})[String(size)]||1;
}
function applyDashboardTypographyTarget(target){
  applyDashboardTypographySettings();
  if(target==="calendar"){ if(typeof renderCalendar==="function") renderCalendar(); if(typeof renderAgenda==="function") renderAgenda(); return; }
  if(target==="clock"){ _clockEls=null; if(typeof tickClock==="function") tickClock(); if(typeof armClockTimer==="function") armClockTimer(); return; }
  if(target==="weather"){ if(typeof renderWeather==="function") renderWeather(); return; }
  if(target==="messages") refitComplimentForVisualChange();
}
function liteVisualProfile(){
  return ["lite","zero2","low","low-power"].includes(String(CONFIG.profile||"").toLowerCase());
}
let _VISUAL_STATE={fontPreset:null, weatherIconStyle:null, seasonalDecor:null, profile:null};
function refitComplimentForVisualChange(){
  const run=()=>{
    try{
      if(typeof COMP_FIT_CACHE!=="undefined" && COMP_FIT_CACHE && typeof COMP_FIT_CACHE.clear==="function") COMP_FIT_CACHE.clear();
      if(typeof complimentLiteProfile==="function" && complimentLiteProfile() && typeof complimentLiteInvalidateGeometry==="function"){
        complimentLiteInvalidateGeometry("typography");
        return;
      }
      if(typeof fitCompliment==="function") fitCompliment();
    }catch(_){}
  };
  if(typeof requestAnimationFrame==="function") requestAnimationFrame(run); else setTimeout(run,0);
  try{
    if(document.fonts && document.fonts.ready) document.fonts.ready.then(run).catch(()=>{});
  }catch(_){}
}
function applyVisualSettings(){
  const root=document.documentElement;
  const fp=visualChoice(FONT_PRESETS,SETTINGS.fontPreset||CONFIG.fontPreset||"default","default");
  const wi=visualChoice(WEATHER_ICON_STYLES,SETTINGS.weatherIconStyle||CONFIG.weatherIconStyle||"soft","soft");
  const dec=visualChoice(SEASONAL_DECOR_MODES,SETTINGS.seasonalDecor||CONFIG.seasonalDecor||"off","off");
  const profile=String(CONFIG.profile||"balanced").toLowerCase();
  const changed={
    font:_VISUAL_STATE.fontPreset!==fp,
    icon:_VISUAL_STATE.weatherIconStyle!==wi,
    decor:_VISUAL_STATE.seasonalDecor!==dec,
    profile:_VISUAL_STATE.profile!==profile
  };
  CONFIG.fontPreset=fp; CONFIG.weatherIconStyle=wi; CONFIG.seasonalDecor=dec;
  if(!(changed.font||changed.icon||changed.decor||changed.profile)){
    return {changed:false,font:false,icon:false,decor:false,profile:false};
  }
  if(changed.font){
    const preset=FONT_PRESETS[fp];
    root.style.setProperty("--sans",preset.sans);
    root.style.setProperty("--mono",preset.mono);
    root.setAttribute("data-font-preset",fp);
    // Font stacks change glyph widths. Keep message text sizing under the
    // measured rotating-message rule rather than retaining a stale inline size.
    refitComplimentForVisualChange();
  }
  if(changed.icon) root.setAttribute("data-wx-icon-style",wi);
  if(changed.decor) root.setAttribute("data-seasonal-decor",dec);
  if(changed.profile){
    root.setAttribute("data-profile",profile);
    root.classList.toggle("profile-lite",liteVisualProfile());
  }
  _VISUAL_STATE={fontPreset:fp, weatherIconStyle:wi, seasonalDecor:dec, profile};
  return {changed:true,...changed};
}
function applySettings(s){
  if(!s) return;
  SETTINGS={...SETTINGS, ...s};
  syncDashboardRuntimeSettings();
  if(typeof s.profile==="string" && s.profile) CONFIG.profile=s.profile;
  if(Number.isFinite(+s.lat) && +s.lat>=-85 && +s.lat<=85) CONFIG.lat=+s.lat;
  if(Number.isFinite(+s.lon) && +s.lon>=-180 && +s.lon<=180) CONFIG.lon=+s.lon;
  for(const k of ["clock24","showSeconds","showInteractiveMaps","showIsoWeekNumbers"]) if(k in s) CONFIG[k]=!!s[k];
  // Weather detail replaces the two tiny UV/AQI switches. Legacy saved flags
  // migrate once on read; both renderer helpers remain derived from this mode.
  let detailMode=String(s.weatherDetailMode||"").toLowerCase();
  if(detailMode!=="standard"&&detailMode!=="expanded") detailMode=(s.showUV===false&&s.showAQI===false)?"standard":"expanded";
  SETTINGS.weatherDetailMode=detailMode;CONFIG.weatherDetailMode=detailMode;
  CONFIG.showUV=detailMode==="expanded";CONFIG.showAQI=detailMode==="expanded";
  // Source/provider choice is automatic; historical saved choices are inert.
  CONFIG.radarProvider="auto";
  if(typeof s.radarCustomTiles==="string") CONFIG.radarCustomTiles=s.radarCustomTiles;
  if(typeof s.radarCustomWms==="string") CONFIG.radarCustomWms=s.radarCustomWms;
  if(s.tempUnit==="fahrenheit"||s.tempUnit==="celsius") CONFIG.tempUnit=s.tempUnit;
  if(["standard","hybrid"].includes(s.mapImageStyle)) CONFIG.mapImageStyle=s.mapImageStyle;
  if(FONT_PRESETS[s.fontPreset]) SETTINGS.fontPreset=s.fontPreset;
  if(WEATHER_ICON_STYLES[s.weatherIconStyle]) SETTINGS.weatherIconStyle=s.weatherIconStyle;
  if(SEASONAL_DECOR_MODES[s.seasonalDecor]) SETTINGS.seasonalDecor=s.seasonalDecor;
  for(const key of DASHBOARD_TYPOGRAPHY_FONT_KEYS){
    if(Object.prototype.hasOwnProperty.call(s,key)) dashboardTypographySetExplicitFont(key,true);
    SETTINGS[key]=dashboardTypographyFont(SETTINGS[key]);
  }
  for(const key of Object.keys(DASHBOARD_TYPOGRAPHY_NUMERIC_VALUES)) SETTINGS[key]=dashboardTypographyNumber(key,SETTINGS[key]);
  // Timing, refresh cadence, forecast horizon, static maps, and pixel shift
  // are automatic defaults. Only geometry remains a deliberate profile control.
  for(const k of ["rowHeight","sidebarWidth"]) if(Number.isFinite(+s[k])) CONFIG[k]=+s[k];
  SETTINGS.pixelShiftEnabled=true;CONFIG.pixelShift=2;CONFIG.showEventMaps=true;
  if(s.weatherAlerts && typeof s.weatherAlerts==="object") CONFIG.weatherAlerts={...(CONFIG.weatherAlerts||{}),...s.weatherAlerts,refreshMinutes:5};
  applyVisualSettings();
  for(const k of ["weeksAbove","weeksBelow","firstDayOfWeek"]) if(Number.isFinite(+s[k])) CONFIG[k]=+s[k];
  const root=document.documentElement;
  if(Number.isFinite(+CONFIG.rowHeight)) root.style.setProperty("--dash-rowheight-preference",(+CONFIG.rowHeight)+"px");
  if(Number.isFinite(+CONFIG.sidebarWidth)) root.style.setProperty("--dash-sidebarwidth-preference",(+CONFIG.sidebarWidth)+"px");
  if(Number.isFinite(+CONFIG.complimentFadeMs)) root.style.setProperty("--compfade",(+CONFIG.complimentFadeMs)+"ms");
  applyDashboardTypographySettings();
  buildFormatters();                  // time format may have changed
  _clockEls=null; tickClock(); if(typeof armClockTimer==="function") armClockTimer();        // rebuild clock structure
  if(typeof updateAppLauncherTrigger==="function") updateAppLauncherTrigger();
  if(typeof dashboardListsDockSettingsChanged==="function") dashboardListsDockSettingsChanged();
  renderWeather(); renderCalendar(); renderAgenda(); renderAlerts();
  if(typeof dashboardFitSchedule==="function")dashboardFitSchedule("settings",0);
  checkDisplaySleep();
}
async function loadSettings(){
  try{
    const res=await fetch("config/settings.json?t="+Date.now(),{cache:"no-store"});
    if(res.ok) applySettings(await res.json());
  }catch(err){ /* server may not be the control server yet — defaults apply */ }
}
function markDashboardTypographyFontsPersisted(){
  for(const key of DASHBOARD_TYPOGRAPHY_FONT_KEYS) dashboardTypographySetExplicitFont(key,true);
}
async function postSettings(){
  // A normal successful complete settings save is the lazy migration point for
  // legacy fontPreset installs. Build its payload from the effective fallback
  // values, then mark the target keys explicit only after persistence succeeds.
  const payload={clock24:CONFIG.clock24,weatherDetailMode:CONFIG.weatherDetailMode||"expanded",
                 alertsMutedUntil:SETTINGS.alertsMutedUntil||0,
                 alertsDocked:!!SETTINGS.alertsDocked,
                 alertsDockedRank:SETTINGS.alertsDockedRank||0,
                 nightDimEnabled:SETTINGS.nightDimEnabled!==false,
                 showSeconds:!!CONFIG.showSeconds,
                 showInteractiveMaps:!!CONFIG.showInteractiveMaps,
                 mapImageStyle:CONFIG.mapImageStyle||"standard",
                 fontPreset:SETTINGS.fontPreset||CONFIG.fontPreset||"default",
                 weatherIconStyle:SETTINGS.weatherIconStyle||CONFIG.weatherIconStyle||"soft",
                 seasonalDecor:SETTINGS.seasonalDecor||CONFIG.seasonalDecor||"off",
                 calendarTextWeight:calendarTextWeight(SETTINGS.calendarTextWeight),
                 calendarTextSize:calendarTextSize(SETTINGS.calendarTextSize),
                 calendarTextFont:dashboardTypographyEffectiveFont("calendarTextFont"),
                 clockTextSize:dashboardTypographyNumber("clockTextSize",SETTINGS.clockTextSize),
                 clockTextWeight:dashboardTypographyNumber("clockTextWeight",SETTINGS.clockTextWeight),
                 clockTextFont:dashboardTypographyEffectiveFont("clockTextFont"),
                 weatherTextSize:dashboardTypographyNumber("weatherTextSize",SETTINGS.weatherTextSize),
                 weatherTextWeight:dashboardTypographyNumber("weatherTextWeight",SETTINGS.weatherTextWeight),
                 weatherTextFont:dashboardTypographyEffectiveFont("weatherTextFont"),
                 messageTextSize:dashboardTypographyNumber("messageTextSize",SETTINGS.messageTextSize),
                 messageTextWeight:dashboardTypographyNumber("messageTextWeight",SETTINGS.messageTextWeight),
                 messageTextFont:dashboardTypographyEffectiveFont("messageTextFont"),
                 showIsoWeekNumbers:!!SETTINGS.showIsoWeekNumbers,
                 tempUnit:CONFIG.tempUnit||"",
                 rowHeight:+(SETTINGS.rowHeight||210),
                 sidebarWidth:+(SETTINGS.sidebarWidth||380),
                 weeksAbove:+(CONFIG.weeksAbove||2),
                 weeksBelow:+(CONFIG.weeksBelow||10),
                 firstDayOfWeek:+(CONFIG.firstDayOfWeek||0),
                 displaySleepEnabled:!!SETTINGS.displaySleepEnabled,
                 displaySleepOff:SETTINGS.displaySleepOff||"22:30",
                 displaySleepOn:SETTINGS.displaySleepOn||"06:00"};
  try{
    // Use the Dashboard Control API wrapper when available so settings saves
    // include the active PIN/session token. A raw fetch silently failed on
    // PIN-protected installs because /api/settings is intentionally locked.
    if(typeof api === "function"){
      await api("/api/settings","POST",payload);
      markDashboardTypographyFontsPersisted();
      return true;
    }
    const headers={"Content-Type":"application/json"};
    try{ if(typeof CTRL_TOKEN !== "undefined" && CTRL_TOKEN) headers["X-Dashboard-Token"]=CTRL_TOKEN; }catch(_){ }
    const res=await fetch("/api/settings",{method:"POST",headers,body:JSON.stringify(payload)});
    if(!res.ok) throw new Error("settings save failed: HTTP "+res.status);
    markDashboardTypographyFontsPersisted();
    return true;
  }catch(err){ console.warn("settings save failed",err); return false; }
}

applyVisualSettings();
