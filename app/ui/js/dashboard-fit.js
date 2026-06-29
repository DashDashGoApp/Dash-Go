// 07a-dashboard-fit.js — responsive dashboard tiers and dock sheets.
// CSS owns all ordinary geometry. Full measured docking is reserved for capable
// profiles and genuine viewport changes; Lite/Zero uses the same CSS tiers but
// deliberately avoids layout reads, dock churn, and geometry-triggered rerenders.
const DASHBOARD_FIT={
  timer:null,
  booted:false,
  applied:"",
  controls:"",
  geometry:"",
  sheet:null,
  renderQueued:false,
};
const DASHBOARD_FIT_MIN_CALENDAR_COLUMN=96;
const DASHBOARD_FIT_EPSILON=4;
function dashboardFitViewport(){
  const root=document.documentElement||{};
  const width=Math.max(1,Math.round(Number(window.innerWidth)||Number(root.clientWidth)||0));
  const height=Math.max(1,Math.round(Number(window.innerHeight)||Number(root.clientHeight)||0));
  // innerWidth/innerHeight are CSS pixels. A 4K desktop at 200% therefore
  // correctly behaves like its effective 1080p viewport.
  const dpr=Math.max(1,Number(window.devicePixelRatio)||1);
  return {width,height,dpr};
}
function dashboardFitTier(view){
  const v=view||dashboardFitViewport();
  const early=typeof window!=="undefined"&&typeof window.dashboardFitTierFromViewport==="function"
    ?window.dashboardFitTierFromViewport(v.width,v.height):"";
  if(["xl","base","compact","dense","min"].includes(early))return early;
  if(v.width>=2400||v.height>=1500)return "xl";
  if(v.width>=1280&&v.height>=720)return "base";
  if(v.width>=1024&&v.height>=600)return "compact";
  if(v.width>=860&&v.height>=520)return "dense";
  return "min";
}
function dashboardFitLiteProfile(){
  try{
    if(typeof liteVisualProfile==="function")return !!liteVisualProfile();
    if(typeof startupLiteProfile==="function")return !!startupLiteProfile();
  }catch(_){}
  const profile=typeof CONFIG!=="undefined"&&CONFIG?String(CONFIG.profile||"").toLowerCase():"";
  return ["lite","zero2","low","low-power"].includes(profile);
}
function dashboardFitGeometryPreference(key,fallback){
  const value=typeof CONFIG!=="undefined"&&CONFIG?Number(CONFIG[key]):NaN;
  return Number.isFinite(value)?Math.round(value):fallback;
}
function dashboardFitApplyGeometryPreferences(){
  const root=document.documentElement;
  if(!root)return;
  // Profile defaults and user edits both become durable geometry preferences.
  // Tier CSS constrains them only when a smaller viewport needs room for the
  // Calendar; no legacy layout mode may erase a saved value after a resize.
  const row=`${dashboardFitGeometryPreference("rowHeight",210)}px`;
  const sidebar=`${dashboardFitGeometryPreference("sidebarWidth",380)}px`;
  const signature=`${row}|${sidebar}`;
  if(signature===DASHBOARD_FIT.geometry)return;
  root.style.setProperty("--dash-rowheight-preference",row);
  root.style.setProperty("--dash-sidebarwidth-preference",sidebar);
  DASHBOARD_FIT.geometry=signature;
}
function dashboardFitApplyViewportTier(root,view,tier){
  if(root.dataset.fit!==tier)root.dataset.fit=tier;
  const dpr=String(Math.round(view.dpr*100)/100);
  if(root.dataset.fitDpr!==dpr)root.dataset.fitDpr=dpr;
  dashboardFitApplyGeometryPreferences();
}
function dashboardFitBlankState(tier){
  return {tier,dockWeather:false,dockSidebar:false,stack:false};
}
function dashboardFitStateSignature(tier,state,view){
  return [tier,state.dockWeather?1:0,state.dockSidebar?1:0,state.stack?1:0,Math.round(view.width),Math.round(view.height)].join("|");
}
function dashboardFitControlsSignature(state){
  return [state.dockWeather?1:0,state.dockSidebar?1:0,state.stack?1:0].join("|");
}
function dashboardFitDockClassSignature(app){
  if(!app||!app.classList)return "000";
  return [app.classList.contains("fit-dock-weather")?1:0,app.classList.contains("fit-dock-sidebar")?1:0,app.classList.contains("fit-stack")?1:0].join("");
}
function dashboardFitApplyDockClasses(app,state){
  if(!app||!app.classList)return false;
  const desired=[state.dockWeather?1:0,state.dockSidebar?1:0,state.stack?1:0].join("");
  if(dashboardFitDockClassSignature(app)===desired)return false;
  app.classList.toggle("fit-dock-weather",!!state.dockWeather);
  app.classList.toggle("fit-dock-sidebar",!!state.dockSidebar);
  app.classList.toggle("fit-stack",!!state.stack);
  return true;
}
function dashboardFitSyncControls(state){
  const signature=dashboardFitControlsSignature(state);
  if(signature===DASHBOARD_FIT.controls)return;
  DASHBOARD_FIT.controls=signature;
  dashboardFitControls(state);
}
function dashboardFitSchedule(reason,delay){
  if(DASHBOARD_FIT.timer){clearTimeout(DASHBOARD_FIT.timer);DASHBOARD_FIT.timer=null;}
  // Fixed Lite kiosks still need responsive CSS tokens after a real resize,
  // but they never schedule a measured controller pass from render hot paths.
  if(dashboardFitLiteProfile()){
    dashboardFitController(reason||"lite-settled");
    return;
  }
  const wait=Number(delay);
  DASHBOARD_FIT.timer=setTimeout(()=>{
    DASHBOARD_FIT.timer=null;
    dashboardFitController(reason||"settled");
  },Number.isFinite(wait)?Math.max(0,wait):120);
}
function dashboardFitSheetIsOpen(){
  const root=document.getElementById("fitdocksheet");
  return !!(root&&!root.hidden&&root.classList.contains("show"));
}
// Exported under the overlay naming convention so the shared idle/autoclose
// lifecycle treats this dock sheet exactly like the other Dashboard overlays.
function fitDockSheetIsOpen(){return dashboardFitSheetIsOpen();}
function dashboardFitSetButtonState(button,visible,expanded){
  if(!button)return;
  button.hidden=!visible;
  button.setAttribute("aria-expanded",String(!!expanded&&visible));
}
function dashboardFitControls(state){
  const forecast=document.getElementById("fitdock-weather-toggle");
  const compact=document.getElementById("fitdock-weather-compact");
  const tabs=document.getElementById("fitdocktabs");
  const agenda=document.getElementById("fitdock-agenda-tab");
  const weather=document.getElementById("fitdock-weather-tab");
  dashboardFitSetButtonState(forecast,!!state.dockWeather&&!state.dockSidebar&&!state.stack,false);
  dashboardFitSetButtonState(compact,!!state.dockSidebar&&!state.stack,false);
  if(tabs){tabs.hidden=!state.stack;tabs.setAttribute("aria-hidden",String(!state.stack));}
  dashboardFitSetButtonState(agenda,!!state.stack,false);
  dashboardFitSetButtonState(weather,!!state.stack,false);
}
function dashboardFitMeasure(){
  const app=document.getElementById("app"),sidebar=document.getElementById("sidebar");
  const calwrap=document.getElementById("calwrap"),agenda=document.getElementById("agenda");
  const weather=document.getElementById("weather"),clock=document.getElementById("clock"),wxnow=document.getElementById("wxnow");
  const calWidth=calwrap?calwrap.getBoundingClientRect().width:0;
  const columnWidth=Math.max(0,(calWidth-8)/7);
  const sidebarHeight=sidebar?sidebar.clientHeight:0;
  const agendaHeight=agenda?agenda.clientHeight:0;
  const weatherHeight=weather?weather.clientHeight:0;
  const clockHeight=clock?clock.scrollHeight:0;
  const currentWeatherHeight=wxnow?wxnow.scrollHeight:0;
  const expectedSidebarOverflow=sidebar?sidebar.scrollHeight>sidebar.clientHeight+DASHBOARD_FIT_EPSILON:false;
  return {
    app,sidebar,calwrap,agenda,weather,clock,wxnow,columnWidth,sidebarHeight,agendaHeight,weatherHeight,clockHeight,currentWeatherHeight,expectedSidebarOverflow
  };
}
function dashboardFitState(tier,measure){
  const m=measure||dashboardFitMeasure();
  const dockEligible=tier==="dense"||tier==="min";
  const calendarTooNarrow=m.columnWidth>0&&m.columnWidth<DASHBOARD_FIT_MIN_CALENDAR_COLUMN;
  const weatherTooShort=!!(m.weather&&m.wxnow&&m.weatherHeight<(m.currentWeatherHeight+62));
  const agendaTooShort=!!(m.agenda&&m.clock&&m.agendaHeight<(m.clockHeight+78));
  const sidebarTooShort=!!(m.sidebar&&m.sidebarHeight<(m.clockHeight+m.currentWeatherHeight+116));
  const measuredOverflow=dockEligible&&m.expectedSidebarOverflow;
  let dockWeather=tier==="min"||(dockEligible&&weatherTooShort)||measuredOverflow;
  let stack=dockEligible&&calendarTooNarrow;
  let dockSidebar=!stack&&dockWeather&&(agendaTooShort||sidebarTooShort||measuredOverflow);
  if(stack){dockSidebar=true;dockWeather=true;}
  return {tier,dockWeather,dockSidebar,stack,calendarTooNarrow,weatherTooShort,agendaTooShort,sidebarTooShort};
}
function dashboardFitQueueGeometryRender(){
  if(dashboardFitLiteProfile()||DASHBOARD_FIT.renderQueued)return;
  DASHBOARD_FIT.renderQueued=true;
  requestAnimationFrame(()=>{
    DASHBOARD_FIT.renderQueued=false;
    if(dashboardFitSheetIsOpen())return;
    if(typeof renderCalendar==="function")renderCalendar();
    if(typeof renderAgenda==="function")renderAgenda();
  });
}
function dashboardFitPrimeBootSignature(){
  if(DASHBOARD_FIT.applied)return;
  const root=document.documentElement,app=document.getElementById("app");
  if(!root||!app)return;
  const view=dashboardFitViewport(),tier=dashboardFitTier(view);
  // index.html owns the first-paint tier. When the page is still in its plain
  // no-dock state, register that exact signature before first render work so a
  // base/compact boot does not repaint Calendar and Agenda a second time.
  if(root.dataset.fit!==tier||dashboardFitDockClassSignature(app)!=="000")return;
  DASHBOARD_FIT.applied=dashboardFitStateSignature(tier,dashboardFitBlankState(tier),view);
}
function dashboardFitController(reason){
  const root=document.documentElement,app=document.getElementById("app");
  if(!root||!app)return;
  const view=dashboardFitViewport(),tier=dashboardFitTier(view);
  dashboardFitApplyViewportTier(root,view,tier);
  if(dashboardFitLiteProfile()){
    // Lite/Zero has one normally fixed display. Responsive CSS remains active,
    // but measured docking is intentionally disabled so agenda/weather refreshes
    // cannot force layout or enqueue a second Calendar/Agenda render.
    if(dashboardFitSheetIsOpen())closeDashboardFitSheet(false);
    const state=dashboardFitBlankState(tier);
    dashboardFitApplyDockClasses(app,state);
    dashboardFitSyncControls(state);
    DASHBOARD_FIT.applied=dashboardFitStateSignature(tier,state,view);
    return;
  }
  if(dashboardFitSheetIsOpen())return;

  // Only undo a previous dock stage before taking the ordinary-layout measure.
  // On the usual base/compact path there is no write before the reads below.
  if(dashboardFitDockClassSignature(app)!=="000"){
    app.classList.remove("fit-dock-weather","fit-dock-sidebar","fit-stack");
  }
  const state=dashboardFitState(tier,dashboardFitMeasure());
  dashboardFitApplyDockClasses(app,state);
  dashboardFitSyncControls(state);
  const signature=dashboardFitStateSignature(tier,state,view);
  if(signature!==DASHBOARD_FIT.applied){
    DASHBOARD_FIT.applied=signature;
    dashboardFitQueueGeometryRender();
  }
}

function dashboardFitWeatherDayLimit(){
  if(dashboardFitLiteProfile()||dashboardFitSheetIsOpen())return 0;
  const app=document.getElementById("app");
  return app&&app.classList.contains("fit-dock-weather")?3:0;
}
function dashboardFitSheetSource(kind){
  if(kind==="forecast")return document.getElementById("wx14");
  if(kind==="weather")return document.getElementById("weather");
  if(kind==="agenda")return document.getElementById("agendalist");
  return null;
}
function dashboardFitSheetTitle(kind){
  return kind==="forecast"?"Weather forecast":kind==="weather"?"Weather":"Agenda";
}
function dashboardFitSetSheetExpanded(expanded){
  for(const id of ["fitdock-weather-toggle","fitdock-weather-compact","fitdock-agenda-tab","fitdock-weather-tab"]){
    const button=document.getElementById(id);
    if(button&&!button.hidden)button.setAttribute("aria-expanded",String(!!expanded));
  }
}
function openDashboardFitSheet(kind,trigger){
  const source=dashboardFitSheetSource(kind),root=document.getElementById("fitdocksheet"),body=document.getElementById("fitdockbody");
  const title=document.getElementById("fitdocktitle");
  if(!source||!root||!body)return;
  if(dashboardFitSheetIsOpen())closeDashboardFitSheet(false);
  const parent=source.parentNode,next=source.nextSibling;
  DASHBOARD_FIT.sheet={kind,source,parent,next,trigger:trigger||document.activeElement};
  if(title)title.textContent=dashboardFitSheetTitle(kind);
  body.replaceChildren(source);
  root.hidden=false;root.classList.add("show");root.setAttribute("aria-hidden","false");
  dashboardFitSetSheetExpanded(true);
  if(typeof pauseUiAnimations==="function")pauseUiAnimations();
  if(typeof armOverlayAutoClose==="function")armOverlayAutoClose();
  if(kind==="forecast"||kind==="weather"){
    // A docked inline list deliberately renders only a short preview. Once it
    // moves into the sheet, rebuild the normal full forecast without another fetch.
    if(typeof renderWeather==="function")renderWeather();
  }
  if(window.DashGoAppDialog)window.DashGoAppDialog.focusInitial(root,"#fitdockclose");
  else requestAnimationFrame(()=>document.getElementById("fitdockclose")?.focus?.());
}
function closeDashboardFitSheet(restoreFocus){
  const root=document.getElementById("fitdocksheet"),body=document.getElementById("fitdockbody"),sheet=DASHBOARD_FIT.sheet;
  if(!root||!sheet)return;
  const {source,parent,next,trigger,kind}=sheet;
  if(parent){
    if(next&&next.parentNode===parent)parent.insertBefore(source,next);
    else parent.appendChild(source);
  }
  body?.replaceChildren();
  root.classList.remove("show");root.hidden=true;root.setAttribute("aria-hidden","true");
  DASHBOARD_FIT.sheet=null;
  dashboardFitSetSheetExpanded(false);
  if(kind==="forecast"||kind==="weather"){
    // Restore the bounded inline preview after the source returns to the sidebar.
    if(typeof renderWeather==="function")renderWeather();
  }
  if(typeof overlayIsOpen==="function"&&overlayIsOpen()){
    if(typeof armOverlayAutoClose==="function")armOverlayAutoClose();
  }else{
    if(typeof disarmOverlayAutoClose==="function")disarmOverlayAutoClose();
    if(typeof resumeUiAfterOverlay==="function")resumeUiAfterOverlay();
  }
  if(restoreFocus!==false){
    if(window.DashGoAppDialog)window.DashGoAppDialog.restoreFocus(trigger,document.getElementById("cblaunch"));
    else trigger?.focus?.();
  }
  dashboardFitSchedule("sheet-close",0);
}
function dashboardFitBoot(){
  if(DASHBOARD_FIT.booted)return;
  DASHBOARD_FIT.booted=true;
  const open=(kind,event)=>openDashboardFitSheet(kind,event&&event.currentTarget);
  const forecast=document.getElementById("fitdock-weather-toggle");
  const compact=document.getElementById("fitdock-weather-compact");
  const agenda=document.getElementById("fitdock-agenda-tab");
  const weather=document.getElementById("fitdock-weather-tab");
  const close=document.getElementById("fitdockclose"),root=document.getElementById("fitdocksheet");
  if(forecast)bindTap(forecast,event=>open("forecast",event));
  if(compact)bindTap(compact,event=>open("weather",event));
  if(agenda)bindTap(agenda,event=>open("agenda",event));
  if(weather)bindTap(weather,event=>open("weather",event));
  if(close)bindTap(close,()=>closeDashboardFitSheet());
  if(root)bindTap(root,()=>closeDashboardFitSheet(),{ignore:event=>event.target!==root});
  document.addEventListener("keydown",event=>{
    if(event.key!=="Escape"||!dashboardFitSheetIsOpen())return;
    event.preventDefault();closeDashboardFitSheet();
  });
  dashboardFitPrimeBootSignature();
  dashboardFitController("boot");
}
