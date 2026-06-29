function minutesOfDay(hhmm){
  const m=String(hhmm||"").match(/^(\d{1,2}):(\d{2})$/); if(!m) return null;
  return Math.max(0,Math.min(1439,(+m[1])*60+(+m[2])));
}
function inClockRange(nowMin,startMin,endMin){
  if(startMin==null || endMin==null || startMin===endMin) return false;
  return startMin<endMin ? (nowMin>=startMin && nowMin<endMin) : (nowMin>=startMin || nowMin<endMin);
}
let DISPLAY_SLEEPING=false;
async function checkDisplaySleep(){
  if(!SETTINGS.displaySleepEnabled) return;
  const now=new Date(), nowMin=now.getHours()*60+now.getMinutes();
  const off=minutesOfDay(SETTINGS.displaySleepOff), on=minutesOfDay(SETTINGS.displaySleepOn);
  const shouldSleep=inClockRange(nowMin,off,on);
  if(shouldSleep && !DISPLAY_SLEEPING && !CTRL_OPEN){
    DISPLAY_SLEEPING=true;
    api("/api/display/off","POST",{automatic:true}).catch(()=>{});
  } else if(!shouldSleep && DISPLAY_SLEEPING){
    DISPLAY_SLEEPING=false;
    api("/api/display/on","POST",{automatic:true}).catch(()=>{});
  }
}
