// 10-boot.js — generated from dashboard.js for maintainability.
/* =====================================================================
   ============================  BOOT  =================================
   ===================================================================== */
function startupProfileName(){ return String(CONFIG.profile||"balanced").toLowerCase(); }
function startupLiteProfile(){
  return ["lite","zero2","low","low-power"].includes(startupProfileName());
}
function startupBalancedProfile(){ return startupProfileName()==="balanced"; }
function deferStartup(fn,ms){ setTimeout(()=>{ try{ fn(); }catch(e){ console.warn("startup task failed",e); } }, Math.max(0,ms||0)); }
let DASH_REFRESH_TIMERS={calendar:null,weather:null,alerts:null};
function armDashboardRefreshSchedules(){
  for(const key of Object.keys(DASH_REFRESH_TIMERS)){
    if(DASH_REFRESH_TIMERS[key]){clearInterval(DASH_REFRESH_TIMERS[key]);DASH_REFRESH_TIMERS[key]=null;}
  }
  DASH_REFRESH_TIMERS.calendar=setInterval(()=>runOrDeferDashboardWork("calendar-refresh",()=>discoverCalendars().then(loadCalendars)),Math.max(3,Number(CONFIG.refreshCalMinutes)||15)*60000);
  DASH_REFRESH_TIMERS.weather=setInterval(()=>runOrDeferDashboardWork("weather-refresh",loadWeather),(typeof effectiveWeatherRefreshMinutes==="function"?effectiveWeatherRefreshMinutes():Math.max(15,Number(CONFIG.refreshWxMinutes)||45))*60000);
  const alerts=(CONFIG.weatherAlerts&&CONFIG.weatherAlerts.refreshMinutes)||5;
  DASH_REFRESH_TIMERS.alerts=setInterval(()=>runOrDeferDashboardWork("alerts-refresh",loadAlerts),Math.max(3,Number(alerts)||5)*60000);
}
function pulseTapAffordance(elm){
  if(!elm) return;
  elm.classList.remove("tap-pulse");
  void elm.offsetWidth;
  elm.classList.add("tap-pulse");
  setTimeout(()=>elm.classList.remove("tap-pulse"),520);
}
function boot(){
  const lite=startupLiteProfile(), balanced=startupBalancedProfile();
  if(lite && typeof complimentLitePrimeGeometry==="function") complimentLitePrimeGeometry();
  showInitialCompliment();
  setupMessageLongPress();
  if(document.fonts && document.fonts.ready) document.fonts.ready.then(()=>{
    if(lite && typeof complimentLiteInvalidateGeometry==="function") complimentLiteInvalidateGeometry("fonts");
    else fitCompliment();
    if(typeof dashboardFitSchedule==="function")dashboardFitSchedule("fonts",0);
  }).catch(()=>{});
  $("#sun").querySelector(".moon").innerHTML=moonSVG();  // moon needs no weather data
  armClockTimer();
  const hadLastKnown=renderLastKnownEvents();
  // Discover calendars from calendars.json first, THEN load them. On lite
  // profiles, a last-known render gives the screen something useful quickly;
  // refresh the real cache after the first paint/settle instead of competing
  // with WebKit's cold-start allocations.
  const calendarLoad=()=>discoverCalendars().then(loadCalendars);
  deferStartup(calendarLoad, lite && hadLastKnown ? 12000 : (lite ? 2500 : 0));
  loadSettings().then(()=>{
    if(typeof dashboardListsDockSettingsChanged==="function")dashboardListsDockSettingsChanged();
    armDashboardRefreshSchedules();
    deferStartup(loadWeather, lite ? 9000 : (balanced ? 2500 : 0));
    deferStartup(loadAlerts, lite ? 35000 : (balanced ? 12000 : 0));
    deferStartup(checkDisplaySleep, lite ? 20000 : (balanced ? 15000 : 0));
  });
  deferStartup(loadCompliments, lite ? 25000 : 0);
  setInterval(()=>runOrDeferDashboardWork("compliments-refresh",loadCompliments), 10*60000);   // SSH/CLI edits appear within 10 min

  let _compResizeT=null;
  window.addEventListener("resize",()=>{
    clearTimeout(_compResizeT);
    _compResizeT=setTimeout(()=>{
      if(lite && typeof complimentLiteInvalidateGeometry==="function") complimentLiteInvalidateGeometry("resize");
      else runOrDeferDashboardWork("compliment-fit",fitCompliment);
      if(typeof dashboardFitSchedule==="function")dashboardFitSchedule("resize",0);
    },150);
  },{passive:true});
  // Warm the control-overlay cache off the critical path on capable profiles.
  // Lite/Zero skips this entirely so first page launch stays smooth; Control
  // itself renders cached sections/lazy data on first open.
  if(!lite && !balanced){
    setTimeout(()=>{for(const p of ["/api/status","/api/themes","/api/calendars","/api/compliments","/api/cache/status","/api/maps/status"])cachedApi(p,()=>{}).catch(()=>{});},20000);
  }
  // Asset warmup downloads only the lazy Control CSS/JS after dashboard first paint;
  // it never renders Control or makes Control API calls during boot.
  if(typeof scheduleControlAssetWarmup==="function")requestAnimationFrame(()=>requestAnimationFrame(scheduleControlAssetWarmup));
  checkTheme();   // cache-proof theme pickup (covers a stale config.local.js)
  // Control overlay: triple-tap the moon phase only. The clock is display-only.
  // Do not prewarm Dashboard Control on the first moon tap; real kiosk testing
  // showed it made the first triple-tap feel less predictable on lite hardware.
  const moonButton=$("#sun") && $("#sun").querySelector(".moon");
  bindTripleTap(moonButton,openCtrl,650,{onFirstTap:()=>pulseTapAffordance(moonButton)});
  const wxPanel=document.getElementById("wxnow");
  if(wxPanel && typeof openRadar==="function") bindTripleTap(wxPanel,openRadar,650,{onFirstTap:()=>pulseTapAffordance(wxPanel),ignore:e=>!!(e&&e.target&&e.target.closest&&e.target.closest("button,a,input,textarea,select,[data-no-radar-action]"))});
  const cbButton=$("#cblaunch");
  bindTap(cbButton,openAppLauncher);
  if(typeof bindHealthWarningPill==="function") bindHealthWarningPill();
  updateAppLauncherTrigger();
  if(typeof dashboardFitBoot==="function")dashboardFitBoot();
  if(typeof familyBoardFooterBoot==="function")familyBoardFooterBoot();
  bindTap($("#ctrlclose"),closeCtrl);
  $("#ctrl").addEventListener("click",e=>{
    if(e.target.id==="ctrl"){
      if(typeof shouldIgnoreCtrlBackdropClose==="function" && shouldIgnoreCtrlBackdropClose()){
        e.preventDefault(); e.stopPropagation(); return;
      }
      closeCtrl();
    }
  });
  armDashboardRefreshSchedules();
  // Alert banner rotation is armed only while multiple visible alerts exist.
  // Rotating message timing is adaptive; scheduleNextCompliment() uses
  // complimentSeconds as a minimum and extends longer text for readability.
  // Re-render and refresh weather when the local date changes. A cache written
  // just before midnight must not leave yesterday labeled as Today until the
  // next ordinary weather interval.
  applyNightDim(); updateStale(); loadDeviceHealth();
  setInterval(()=>runOrDeferDashboardWork("device-health",loadDeviceHealth),10*60000);
  let lastDay=typeof weatherLocalDateKey==="function"?weatherLocalDateKey(new Date()):new Date().toDateString();
  setInterval(()=>{
    const visualMinute=()=>{
      const now=new Date();
      const d=typeof weatherLocalDateKey==="function"?weatherLocalDateKey(now):now.toDateString();
      if(d!==lastDay){
        lastDay=d; renderCalendar(); renderAgenda();
        clearTimeout(loadWeather._timer); loadWeather._retry=0;
        if(!(typeof deferDashboardWork==="function" && deferDashboardWork("weather-day-rollover",loadWeather))) loadWeather();
      }
      applyNightDim(); updateStale(); applyPixelShift(); checkTheme();
    };
    if(!(typeof deferDashboardWork==="function" && deferDashboardWork("minute-visuals",visualMinute))) visualMinute();
    checkDisplaySleep();
  }, 60000);
  // Prime the calendar scroll machinery so the FIRST user gesture is responsive
  // (otherwise WebKit defers layer/scroll setup until first touch, causing the
  // "first scroll doesn't take" lag). Poll until content exists, then nudge.
  const prime=()=>{
    (function primeScroll(tries){
      const s=$("#calscroll");
      if(s && s.scrollHeight>s.clientHeight){
        const home=(function(){ const cw=$("#currentweek"); return cw?cw.offsetTop:0; })();
        s.scrollTop = home + 1;        // tiny nudge to force scroll init
        requestAnimationFrame(()=>{ s.scrollTop = home; });  // settle back to today
      } else if((tries||0) < 40){
        setTimeout(()=>primeScroll((tries||0)+1), lite ? 300 : 150);   // wait for first render
      }
    })(0);
  };
  deferStartup(prime, lite ? 18000 : 0);
}
function bootSafe(){
  try{ boot(); }
  catch(err){
    console.error("dashboard boot failed",err);
    const q=(typeof $==="function") ? $ : (s)=>document.querySelector(s);
    const wx=q("#wxnow"), cal=q("#calscroll"), ct=q("#ctime");
    const msg="dashboard startup error — check browser console / index.html";
    if(wx) wx.innerHTML='<div class="loading">'+msg+'</div>';
    if(cal) cal.innerHTML='<div class="loading" style="padding:20px">'+msg+'</div>';
    if(ct) ct.textContent="—";
  }
}
document.addEventListener("DOMContentLoaded",bootSafe);
