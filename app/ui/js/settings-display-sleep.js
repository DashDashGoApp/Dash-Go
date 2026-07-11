function minutesOfDay(hhmm){
  const m=String(hhmm||"").match(/^(\d{1,2}):(\d{2})$/);
  if(!m) return null;
  return Math.max(0,Math.min(1439,(+m[1])*60+(+m[2])));
}
function inClockRange(nowMin,startMin,endMin){
  if(startMin==null || endMin==null || startMin===endMin) return false;
  return startMin<endMin ? (nowMin>=startMin && nowMin<endMin) : (nowMin>=startMin || nowMin<endMin);
}

let DISPLAY_SLEEPING=false;
let DISPLAY_SLEEP_PENDING="";
let DISPLAY_SLEEP_RECONCILE_PENDING=false;
let DISPLAY_SLEEP_WAKE_OVERRIDE=false;
let DISPLAY_SLEEP_REQUEST_SERIAL=0;
let DISPLAY_SLEEP_SCHEDULE_SIGNATURE="";

function markDisplaySleepReconcile(){ DISPLAY_SLEEP_RECONCILE_PENDING=true; }
function reconcileDisplayWake(){
  if(!DISPLAY_SLEEP_RECONCILE_PENDING) return;
  DISPLAY_SLEEP_RECONCILE_PENDING=false;
  renderCalendar();
  renderAgenda();
  clearTimeout(loadWeather._timer);
  loadWeather._retry=0;
  loadWeather();
  applyNightDim();
  updateStale();
  applyPixelShift();
  checkTheme();
}
function displaySleepScheduleSignature(){
  return [!!SETTINGS.displaySleepEnabled,SETTINGS.displaySleepOff||"",SETTINGS.displaySleepOn||""].join("|");
}
function displaySleepInScheduledRange(now){
  const current=now||new Date();
  const off=minutesOfDay(SETTINGS.displaySleepOff);
  const on=minutesOfDay(SETTINGS.displaySleepOn);
  return !!SETTINGS.displaySleepEnabled && inClockRange(current.getHours()*60+current.getMinutes(),off,on);
}
function displaySleepSetAwake({override=false,reconcile=true}={}){
  const wasSleeping=DISPLAY_SLEEPING || DISPLAY_SLEEP_PENDING==="off";
  DISPLAY_SLEEPING=false;
  DISPLAY_SLEEP_PENDING="";
  if(override && displaySleepInScheduledRange()) DISPLAY_SLEEP_WAKE_OVERRIDE=true;
  if(wasSleeping) markDisplaySleepReconcile();
  if(reconcile) reconcileDisplayWake();
}
async function requestDisplaySleep({automatic=false}={}){
  if(DISPLAY_SLEEPING || DISPLAY_SLEEP_PENDING==="off") return true;
  const serial=++DISPLAY_SLEEP_REQUEST_SERIAL;
  DISPLAY_SLEEP_PENDING="off";
  try{
    await api("/api/display/off","POST",automatic?{automatic:true}:{});
    if(serial!==DISPLAY_SLEEP_REQUEST_SERIAL) return false;
    DISPLAY_SLEEP_PENDING="";
    DISPLAY_SLEEPING=true;
    return true;
  }catch(err){
    if(serial===DISPLAY_SLEEP_REQUEST_SERIAL){
      DISPLAY_SLEEP_PENDING="";
      DISPLAY_SLEEPING=false;
    }
    throw err;
  }
}
async function requestDisplayWake({automatic=false,override=false}={}){
  if(override && displaySleepInScheduledRange()) DISPLAY_SLEEP_WAKE_OVERRIDE=true;
  const serial=++DISPLAY_SLEEP_REQUEST_SERIAL;
  const wasSleeping=DISPLAY_SLEEPING || DISPLAY_SLEEP_PENDING==="off";
  DISPLAY_SLEEP_PENDING="on";
  try{
    await api("/api/display/on","POST",automatic?{automatic:true}:{});
    if(serial!==DISPLAY_SLEEP_REQUEST_SERIAL) return false;
    DISPLAY_SLEEP_PENDING="";
    DISPLAY_SLEEPING=false;
    if(wasSleeping) markDisplaySleepReconcile();
    reconcileDisplayWake();
    return true;
  }catch(err){
    if(serial===DISPLAY_SLEEP_REQUEST_SERIAL){
      DISPLAY_SLEEP_PENDING="";
      DISPLAY_SLEEPING=wasSleeping;
    }
    throw err;
  }
}
function noteDisplayWakeInteraction(){
  if(!DISPLAY_SLEEPING && DISPLAY_SLEEP_PENDING!=="off") return;
  const interruptedOff=DISPLAY_SLEEP_PENDING==="off";
  ++DISPLAY_SLEEP_REQUEST_SERIAL;
  displaySleepSetAwake({override:true,reconcile:true});
  // A touch can race the asynchronous screen-off command. A corrective on
  // request preserves the user's wake action without trusting completion order.
  if(interruptedOff) requestDisplayWake({automatic:true,override:true}).catch(()=>{});
}
function bindDisplayWakeReconciliation(){
  const wake=()=>noteDisplayWakeInteraction();
  document.addEventListener("touchstart",wake,{capture:true,passive:true});
  document.addEventListener("pointerdown",wake,{capture:true,passive:true});
  document.addEventListener("keydown",wake,{capture:true,passive:true});
  document.addEventListener("visibilitychange",()=>{
    if(!document.hidden) noteDisplayWakeInteraction();
  },{passive:true});
}
async function checkDisplaySleep(){
  const signature=displaySleepScheduleSignature();
  if(signature!==DISPLAY_SLEEP_SCHEDULE_SIGNATURE){
    DISPLAY_SLEEP_SCHEDULE_SIGNATURE=signature;
    DISPLAY_SLEEP_WAKE_OVERRIDE=false;
  }

  if(!SETTINGS.displaySleepEnabled){
    DISPLAY_SLEEP_WAKE_OVERRIDE=false;
    if(DISPLAY_SLEEPING || DISPLAY_SLEEP_PENDING==="off"){
      try{ await requestDisplayWake({automatic:true}); }catch(_){ }
    }else{
      displaySleepSetAwake({reconcile:true});
    }
    return;
  }

  const shouldSleep=displaySleepInScheduledRange();
  if(!shouldSleep){
    DISPLAY_SLEEP_WAKE_OVERRIDE=false;
    if(DISPLAY_SLEEPING || DISPLAY_SLEEP_PENDING==="off"){
      try{ await requestDisplayWake({automatic:true}); }catch(_){ }
    }else{
      displaySleepSetAwake({reconcile:true});
    }
    return;
  }

  if(DISPLAY_SLEEP_WAKE_OVERRIDE || CTRL_OPEN || DISPLAY_SLEEP_PENDING==="on") return;
  if(!DISPLAY_SLEEPING && DISPLAY_SLEEP_PENDING!=="off"){
    try{ await requestDisplaySleep({automatic:true}); }catch(_){ }
  }
}

bindDisplayWakeReconciliation();
