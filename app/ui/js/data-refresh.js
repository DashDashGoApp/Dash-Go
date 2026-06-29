async function loadCalendars(){
  if(typeof deferDashboardWork==="function" && deferDashboardWork("calendar-refresh",()=>loadCalendars())) return;
  if(loadCalendars._busy) return;
  loadCalendars._busy=true;
  try{
  const today=startOfDay(new Date());
  const winStart=addDays(startOfWeek(today),-CONFIG.weeksAbove*7);          // DST-safe
  const winEnd=addDays(startOfWeek(today),(CONFIG.weeksBelow+1)*7);
  const cachedEvents=await loadEventsCache(winStart,winEnd);
  if(cachedEvents){
    CAL_ISSUES=(EVENT_CACHE_INFO.issues||[]).slice();
    lastCalOK=Date.now();
    commitCalendarEvents(cachedEvents,"events.cache:"+(EVENT_CACHE_INFO.generatedAt||0));
    maybePrewarmEventMaps(winStart,winEnd);
    return;
  }
  EVENT_CACHE_INFO={source:"ics", using:false, generatedAt:0, eventCount:0};
  // Fetch every calendar in parallel — total wait is the slowest file, not the
  // sum. Failures are isolated per calendar.
  // Parse cache: most 10-min refreshes find identical files, and re-parsing a
  // big .ics is the dominant recurring CPU cost on low-power boards. Reuse the
  // parsed events when Last-Modified matches (fast path, skips even the text
  // decode) or when the text is byte-identical (catches servers without LM).
  const CACHE = loadCalendars._cache || (loadCalendars._cache=new Map());
  const results=await Promise.all(ACTIVE_CALENDARS.map(async cal=>{
    try{
      const res=await fetch(cal.url+"?t="+Date.now(),{cache:"no-store"});
      if(!res.ok) throw new Error(res.status);
      const lm=res.headers&&res.headers.get?res.headers.get("last-modified"):null;
      const cached=CACHE.get(cal.url);
      let events;
      if(cached && lm && cached.lm===lm){
        events=cached.events;                  // unchanged on disk — reuse parse
      } else {
        const txt=await res.text();
        if(cached && cached.txt===txt){
          events=cached.events; cached.lm=lm;  // same content, LM rolled/absent
        } else {
          events=parseICS(txt,cal);
          // Cache the text for equality checks only when it's modest; huge
          // calendars still benefit from the Last-Modified fast path.
          CACHE.set(cal.url,{lm, txt: txt.length<2_000_000?txt:null, events});
        }
      }
      // Re-bind to the CURRENT calendar entry so a color/name change in
      // calendars.json shows up even on a cache hit.
      for(const ev of events){
        ev.cal=cal;
        if(!ev.appOwner && cal.owner)ev.appOwner=cal.owner;
      }
      return { cal, events, lm };
    }catch(err){ console.warn("calendar failed:",cal.url,err); return null; }
  }));
  // Drop cache entries for calendars that no longer exist.
  const live=new Set(ACTIVE_CALENDARS.map(c=>c.url));
  for(const k of CACHE.keys()) if(!live.has(k)) CACHE.delete(k);
  // Track data health for the stale indicator: any successful load refreshes
  // lastCalOK; per-file Last-Modified ages catch a sync that has quietly died
  // server-side (the cron keeps the old file, so the fetch itself succeeds).
  // Zero configured calendars is a valid state, not a stale one.
  if(ACTIVE_CALENDARS.length===0 || results.some(r=>r)) lastCalOK=Date.now();
  CAL_ISSUES=[];
  results.forEach((r,i)=>{
    const cal=ACTIVE_CALENDARS[i], nm=(cal.name||cal.url);
    if(!r){ CAL_ISSUES.push(nm); return; }
    if(CONFIG.staleCalHours>0 && r.lm && cal.tag!=="holiday"){
      const ageH=(Date.now()-Date.parse(r.lm))/3600000;
      if(isFinite(ageH) && ageH>CONFIG.staleCalHours) CAL_ISSUES.push(nm);
    }
  });
  const all=[];
  for(const r of results){
    if(!r) continue;
    for(const ev of r.events){
      for(const inst of expand(ev,winStart,winEnd)){
        if((inst.end||inst.start)>=winStart && inst.start<=winEnd) all.push(inst);
      }
    }
  }
  for(const ev of birthdayEvents(winStart,winEnd)) all.push(ev);
  all.sort((a,b)=>a.start-b.start);
  commitCalendarEvents(all,"ics:"+ACTIVE_CALENDARS.map(c=>[c.url,c.name||"",c.color||"",c.tag||"",c.owner||""].join("|")).join("~"));
  maybePrewarmEventMaps(winStart,winEnd);
  } finally {
    loadCalendars._busy=false;
  }
}
