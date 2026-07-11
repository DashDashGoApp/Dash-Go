function cacheEventToRuntime(e){
  const cal=e.cal||{};
  return {
    id:e.id||("cache-"+e.start+"-"+(e.title||"")),
    cal:{url:cal.url||e.calUrl||"",name:cal.name||"",color:cal.color||"",tag:cal.tag||"",owner:cal.owner||e.appOwner||""},
    title:e.title||"(no title)", desc:e.desc||"", location:e.location||"",
    start:new Date(e.start), end:e.end?new Date(e.end):null, allDay:!!e.allDay,
    uid:e.uid||"", appOwner:e.appOwner||cal.owner||"",
    managedSchedule:(e.managedSchedule&&typeof e.managedSchedule==="object"?e.managedSchedule:null),
    writeback:(e.writeback&&typeof e.writeback==="object"?e.writeback:null)
  };
}
let MAP_PREWARM_LAST=0;
function maybePrewarmEventMaps(winStart,winEnd){
  const now=Date.now();
  // Fire-and-forget. Server-side cache cleanup keeps storage bounded; this
  // just warms visible-range event maps so popups open with an instant image.
  if(now-MAP_PREWARM_LAST<10*60000) return;
  MAP_PREWARM_LAST=now;
  const prof=dashboardProfileName();
  if(dashboardLiteProfile(prof) && typeof BOOT_TS!=="undefined" && Date.now()-BOOT_TS<120000) return;
  const limit=(prof==="enhanced"||prof==="maximum")?36:(prof==="balanced")?24:12;
  fetch("/api/maps/prewarm",{method:"POST",headers:{"Content-Type":"application/json"},
    body:JSON.stringify({windowStart:+winStart,windowEnd:+winEnd,limit,eventMaps:true,interactiveMaps:!!CONFIG.showInteractiveMaps})}).catch(()=>{});
}

let EVENT_CACHE_ETAG="";
let EVENT_CACHE_BASE_EVENTS=null;
let EVENT_CACHE_WINDOW_START=0;
let EVENT_CACHE_WINDOW_END=0;
function eventCacheWindowEvents(winStart,winEnd){
  if(!Array.isArray(EVENT_CACHE_BASE_EVENTS)) return null;
  if(EVENT_CACHE_WINDOW_START>+winStart||EVENT_CACHE_WINDOW_END<+winEnd) return null;
  const all=EVENT_CACHE_BASE_EVENTS.filter(ev=>(ev.end||ev.start)>=winStart&&ev.start<=winEnd);
  for(const ev of birthdayEvents(winStart,winEnd)) all.push(ev);
  all.sort((a,b)=>a.start-b.start);
  return all;
}
async function loadEventsCache(winStart,winEnd,retryWithoutETag){
  try{
    const headers=EVENT_CACHE_ETAG?{"If-None-Match":EVENT_CACHE_ETAG}:{};
    const res=await fetch("cache/events.cache.json",{cache:"no-store",headers});
    if(res.status===304){
      const prior=eventCacheWindowEvents(winStart,winEnd);
      if(prior) return prior;
      if(!retryWithoutETag){
        EVENT_CACHE_ETAG="";
        return loadEventsCache(winStart,winEnd,true);
      }
      return null;
    }
    if(!res.ok) return null;
    const cache=await res.json();
    if(!cache || cache.version!==10 || !Array.isArray(cache.events)) return null;
    if(cache.windowStart>+winStart || cache.windowEnd<+winEnd) return null;
    EVENT_CACHE_BASE_EVENTS=[];
    for(const raw of cache.events){
      if(!raw || raw.start==null) continue;
      EVENT_CACHE_BASE_EVENTS.push(cacheEventToRuntime(raw));
    }
    EVENT_CACHE_WINDOW_START=Number(cache.windowStart)||0;
    EVENT_CACHE_WINDOW_END=Number(cache.windowEnd)||0;
    EVENT_CACHE_ETAG=(res.headers&&res.headers.get&&res.headers.get("ETag"))||"";
    EVENT_CACHE_INFO={source:"cache",using:true,generatedAt:cache.generatedAt||0,
      windowStart:cache.windowStart||0,windowEnd:cache.windowEnd||0,
      eventCount:cache.events.length,issues:cache.issues||[],etag:EVENT_CACHE_ETAG};
    return eventCacheWindowEvents(winStart,winEnd);
  }catch(err){
    console.warn("event cache unavailable, falling back to ICS",err);
    return null;
  }
}
let LAST_KNOWN_EVENTS_PERSIST_QUEUED=false;
let LAST_KNOWN_EVENTS_PENDING=null;
function queueLastKnownEventsPersist(events){
  // Calendar commits can arrive in a short burst (last-known first paint,
  // then fresh ICS). Serialize only the newest snapshot so deferred work
  // cannot overwrite newer data and repeated refreshes share one idle slot.
  LAST_KNOWN_EVENTS_PENDING=events;
  if(LAST_KNOWN_EVENTS_PERSIST_QUEUED)return;
  LAST_KNOWN_EVENTS_PERSIST_QUEUED=true;
  const persist=()=>{
    LAST_KNOWN_EVENTS_PERSIST_QUEUED=false;
    const current=LAST_KNOWN_EVENTS_PENDING;
    LAST_KNOWN_EVENTS_PENDING=null;
    if(!current)return;
    try{ localStorage.setItem("dashboard:lastEvents",JSON.stringify({ts:Date.now(),events:current.map(e=>({
      id:e.id,title:e.title,desc:e.desc,location:e.location,start:+e.start,end:e.end?+e.end:null,allDay:!!e.allDay,appOwner:e.appOwner||"",managedSchedule:e.managedSchedule||null,writeback:e.writeback||null,cal:e.cal||{}
    }))})); }catch(_){ }
  };
  if(typeof requestIdleCallback==="function")requestIdleCallback(persist,{timeout:4000});
  else setTimeout(persist,600);
}
function calendarStableJSON(value){
  if(value===null||typeof value!=="object") return JSON.stringify(value);
  if(Array.isArray(value)) return "["+value.map(calendarStableJSON).join(",")+"]";
  return "{"+Object.keys(value).sort().map(key=>JSON.stringify(key)+":"+calendarStableJSON(value[key])).join(",")+"}";
}
function calendarSignatureMix(hash,value){
  const text=String(value==null?"":value);
  for(let i=0;i<text.length;i++){
    hash^=text.charCodeAt(i);
    hash=Math.imul(hash,16777619)>>>0;
  }
  hash^=31;
  return Math.imul(hash,16777619)>>>0;
}
function calendarEventsSignature(all,sigExtra){
  let hash=2166136261;
  hash=calendarSignatureMix(hash,sigExtra||"");
  for(const event of all){
    const cal=event.cal||{};
    for(const value of [event.id,event.uid,event.title,event.desc,event.location,+event.start,event.end?+event.end:0,event.allDay?1:0,cal.url,cal.name,cal.color,cal.tag,cal.owner,event.appOwner]){
      hash=calendarSignatureMix(hash,value);
    }
    hash=calendarSignatureMix(hash,calendarStableJSON(event.managedSchedule||null));
    hash=calendarSignatureMix(hash,calendarStableJSON(event.writeback||null));
  }
  return hash.toString(16)+":"+all.length;
}
function commitCalendarEvents(all,sigExtra){
  const sig=calendarEventsSignature(all,sigExtra);
  // A routine cache refresh often returns byte-for-byte equivalent events.
  // Keep the current EVENTS array and per-day index intact in that case; there
  // is no DOM work to perform and rebuilding the index would only allocate on
  // the kiosk's idle path.
  if(sig===loadCalendars._sig){ return false; }
  EVENTS=all;
  rebuildDayIndex();
  loadCalendars._sig=sig;
  // Persist the last-known snapshot after the visible update is scheduled.
  // JSON.stringify of a full multi-week window plus a synchronous
  // localStorage write only benefits the NEXT boot, so it stays off the
  // fresh-data-to-paint path and coalesces with any following refresh.
  queueLastKnownEventsPersist(all);
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
