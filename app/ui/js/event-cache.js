function cacheEventToRuntime(e){
  const cal=e.cal||{};
  return {
    id:e.id||("cache-"+e.start+"-"+(e.title||"")),
    cal:{url:cal.url||e.calUrl||"",name:cal.name||"",color:cal.color||"",tag:cal.tag||"",owner:cal.owner||e.appOwner||""},
    title:e.title||"(no title)", desc:e.desc||"", location:e.location||"",
    start:new Date(e.start), end:e.end?new Date(e.end):null, allDay:!!e.allDay,
    uid:e.uid||"", appOwner:e.appOwner||cal.owner||"",
    managedSchedule:(e.managedSchedule&&typeof e.managedSchedule==="object"?e.managedSchedule:null)
  };
}
let MAP_PREWARM_LAST=0;
function maybePrewarmEventMaps(winStart,winEnd){
  const now=Date.now();
  // Fire-and-forget. Server-side cache cleanup keeps storage bounded; this
  // just warms visible-range event maps so popups open with an instant image.
  if(now-MAP_PREWARM_LAST<10*60000) return;
  MAP_PREWARM_LAST=now;
  const prof=String(CONFIG.profile||"balanced").toLowerCase();
  if(["lite","zero2","low","low-power"].includes(prof) && typeof BOOT_TS!=="undefined" && Date.now()-BOOT_TS<120000) return;
  const limit=(prof==="enhanced"||prof==="maximum")?36:(prof==="balanced")?24:12;
  fetch("/api/maps/prewarm",{method:"POST",headers:{"Content-Type":"application/json"},
    body:JSON.stringify({windowStart:+winStart,windowEnd:+winEnd,limit,eventMaps:true,interactiveMaps:!!CONFIG.showInteractiveMaps})}).catch(()=>{});
}

async function loadEventsCache(winStart,winEnd){
  try{
    const res=await fetch("cache/events.cache.json?t="+Date.now(),{cache:"no-store"});
    if(!res.ok) return null;
    const cache=await res.json();
    if(!cache || cache.version!==6 || !Array.isArray(cache.events)) return null;
    if(cache.windowStart>+winStart || cache.windowEnd<+winEnd) return null;
    const all=[];
    for(const raw of cache.events){
      if(!raw || raw.start==null) continue;
      const ev=cacheEventToRuntime(raw);
      if((ev.end||ev.start)>=winStart && ev.start<=winEnd) all.push(ev);
    }
    for(const ev of birthdayEvents(winStart,winEnd)) all.push(ev);
    all.sort((a,b)=>a.start-b.start);
    EVENT_CACHE_INFO={source:"cache", using:true, generatedAt:cache.generatedAt||0,
      windowStart:cache.windowStart||0, windowEnd:cache.windowEnd||0,
      eventCount:cache.events.length, issues:cache.issues||[]};
    return all;
  }catch(err){
    console.warn("event cache unavailable, falling back to ICS",err);
    return null;
  }
}
function commitCalendarEvents(all,sigExtra){
  const sig=(sigExtra||"")+"::"+all.map(e=>(e.title||"")+"|"+(+e.start)+"|"+(e.end?+e.end:0)+"|"+
                       ((e.location||"").length)+"|"+((e.desc||"").length)+"|"+
                       ((e.cal&&e.cal.name)||"")+"|"+((e.cal&&e.cal.color)||"")+"|"+((e.cal&&e.cal.tag)||"")+"|"+
                       (e.appOwner||((e.cal&&e.cal.owner)||""))+"|"+JSON.stringify(e.managedSchedule||null)).join("~");
  // A routine cache refresh often returns byte-for-byte equivalent events.
  // Keep the current EVENTS array and per-day index intact in that case; there
  // is no DOM work to perform and rebuilding the index would only allocate on
  // the kiosk's idle path.
  if(sig===loadCalendars._sig){ return false; }
  EVENTS=all;
  rebuildDayIndex();
  loadCalendars._sig=sig;
  try{ localStorage.setItem("dashboard:lastEvents",JSON.stringify({ts:Date.now(),events:all.map(e=>({
    id:e.id,title:e.title,desc:e.desc,location:e.location,start:+e.start,end:e.end?+e.end:null,allDay:!!e.allDay,appOwner:e.appOwner||"",managedSchedule:e.managedSchedule||null,cal:e.cal||{}
  }))})); }catch(_){ }
  const paint=()=>{ renderCalendar(); renderAgenda(); };
  if(typeof deferDashboardWork==="function" && deferDashboardWork("calendar-render",paint)) return true;
  paint();
  return true;
}
function renderLastKnownEvents(){
  try{
    const saved=JSON.parse(localStorage.getItem("dashboard:lastEvents")||"null");
    if(!saved || !Array.isArray(saved.events) || !saved.events.length) return false;
    const all=saved.events.map(cacheEventToRuntime).filter(e=>e.start);
    EVENT_CACHE_INFO={source:"localStorage", using:false, generatedAt:saved.ts||0, eventCount:all.length};
    commitCalendarEvents(all,"localStorage:"+(saved.ts||0));
    return true;
  }catch(_){ return false; }
}
