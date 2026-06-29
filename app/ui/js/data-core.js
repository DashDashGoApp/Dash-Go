// 03-data.js — generated from dashboard.js for maintainability.
/* =====================================================================
   ============================  DATE HELPERS  =========================
   ===================================================================== */
const DAY=86400000;
function startOfDay(d){ return new Date(d.getFullYear(),d.getMonth(),d.getDate()); }
function startOfWeek(d){
  const x=startOfDay(d); const diff=(x.getDay()-CONFIG.firstDayOfWeek+7)%7;
  // Date components, not ms subtraction — ms math lands at 23:00/01:00 when
  // the span crosses a DST changeover, mislabeling every cell after it.
  return new Date(x.getFullYear(),x.getMonth(),x.getDate()-diff);
}
function addDays(d,n){ return new Date(d.getFullYear(),d.getMonth(),d.getDate()+n); }
function isoWeek(d){
  const t=new Date(Date.UTC(d.getFullYear(),d.getMonth(),d.getDate()));
  const day=(t.getUTCDay()+6)%7; t.setUTCDate(t.getUTCDate()-day+3);
  const first=new Date(Date.UTC(t.getUTCFullYear(),0,4));
  const fday=(first.getUTCDay()+6)%7; first.setUTCDate(first.getUTCDate()-fday+3);
  return 1+Math.round((t-first)/(7*DAY));
}
function sameDay(a,b){ return a.getFullYear()===b.getFullYear()&&a.getMonth()===b.getMonth()&&a.getDate()===b.getDate(); }

/* =====================================================================
   ============================  STATE  ================================
   ===================================================================== */
let EVENTS=[];          // expanded, in-window events
let EVENT_CACHE_INFO={source:"ics", using:false, generatedAt:0, eventCount:0};
let WX=null;            // weather json

// Daily weather dates are reused by calendar cells and Weather popups. Build
// these small indexes once whenever WX changes instead of repeatedly scanning
// the forecast arrays with indexOf() on every render/tap.
let WX_DAILY_INDEX=new Map();
let WX_DAILY_INDEX_SOURCE=null;
let WX_SOURCE_DAILY_INDEX=new WeakMap();
function weatherDateKey(value){ return String(value||"").slice(0,10); }
function buildWeatherDateIndex(times){
  const index=new Map();
  if(Array.isArray(times)) times.forEach((value,i)=>{ const key=weatherDateKey(value); if(key && !index.has(key)) index.set(key,i); });
  return index;
}
function rebuildWeatherDayIndex(wx){
  const times=wx&&wx.daily&&Array.isArray(wx.daily.time)?wx.daily.time:[];
  WX_DAILY_INDEX=buildWeatherDateIndex(times);
  WX_DAILY_INDEX_SOURCE=times;
  WX_SOURCE_DAILY_INDEX=new WeakMap();
  for(const src of (wx&&Array.isArray(wx._sources)?wx._sources:[])){
    if(!src || typeof src!=="object") continue;
    const sourceTimes=src.daily&&Array.isArray(src.daily.time)?src.daily.time:[];
    WX_SOURCE_DAILY_INDEX.set(src,{times:sourceTimes,index:buildWeatherDateIndex(sourceTimes)});
  }
}
function setWeatherPayload(wx){ WX=wx; rebuildWeatherDayIndex(WX); return WX; }
function weatherDailyIndexFor(date){
  const times=WX&&WX.daily&&Array.isArray(WX.daily.time)?WX.daily.time:[];
  if(times!==WX_DAILY_INDEX_SOURCE) rebuildWeatherDayIndex(WX);
  const index=WX_DAILY_INDEX.get(weatherDateKey(date));
  return index==null?-1:index;
}
function weatherSourceDailyIndexFor(src,date){
  if(!src || typeof src!=="object") return -1;
  const times=src.daily&&Array.isArray(src.daily.time)?src.daily.time:[];
  let cached=WX_SOURCE_DAILY_INDEX.get(src);
  if(!cached || cached.times!==times){
    cached={times,index:buildWeatherDateIndex(times)};
    WX_SOURCE_DAILY_INDEX.set(src,cached);
  }
  const index=cached.index.get(weatherDateKey(date));
  return index==null?-1:index;
}
const $=s=>document.querySelector(s);
const el=(tag,cls,txt)=>{const e=document.createElement(tag);if(cls)e.className=cls;if(txt!=null)e.textContent=txt;return e;};

// Shared date/time formatters — constructing Intl.DateTimeFormat is expensive
// (a few ms each on slow hardware), so build each ONCE and reuse everywhere
// instead of per-event inside render loops. Rebuilt (rarely) when the
// 12/24-hour setting changes, so event/agenda/weather times follow the clock.
let FMT=null;
function buildFormatters(){
  const h12=!CONFIG.clock24;
  FMT={
    time:   new Intl.DateTimeFormat(LOCALE,{hour:"numeric",minute:"2-digit",hour12:h12}),
    hour:   new Intl.DateTimeFormat(LOCALE,{hour:"numeric",hour12:h12}),
    monthS: new Intl.DateTimeFormat(LOCALE,{month:"short"}),
    dayLong:new Intl.DateTimeFormat(LOCALE,{weekday:"long",month:"long",day:"numeric"}),
    agDay:  new Intl.DateTimeFormat(LOCALE,{weekday:"long",month:"short",day:"numeric"}),
    popDay: new Intl.DateTimeFormat(LOCALE,{weekday:"short",month:"short",day:"numeric"}),
    wxDay:  new Intl.DateTimeFormat(LOCALE,{weekday:"short",month:"numeric",day:"numeric"}),
    weekday:new Intl.DateTimeFormat(LOCALE,{weekday:"long"}),
    hm2:    new Intl.DateTimeFormat(LOCALE,{hour:"2-digit",minute:"2-digit",hour12:h12}),
  };
}
buildFormatters();

// Per-day event index: maps localDateKey -> events touching that day, built
// once whenever EVENTS changes. Replaces the old per-cell full scan
// (O(cells × events)) with an O(events) pass + O(1) lookups during render.
let DAY_INDEX=new Map();
function rebuildDayIndex(){
  DAY_INDEX=new Map();
  const add=(key,ev)=>{ const a=DAY_INDEX.get(key); if(a) a.push(ev); else DAY_INDEX.set(key,[ev]); };
  for(const ev of EVENTS){
    if(ev.allDay){
      // All-day events span whole local days. iCal DTEND is exclusive, but
      // exporters are inconsistent (same-day end, missing end, midnight times),
      // so normalize: event covers [startDay .. lastDay] inclusive.
      const sDay=startOfDay(ev.start);
      let lastDay;
      if(ev.end){
        const endDay=startOfDay(ev.end);
        const endAtMidnight=(+ev.end===+endDay);
        lastDay=endAtMidnight ? addDays(endDay,-1) : endDay;   // DST-safe
        if(lastDay<sDay) lastDay=sDay;
      } else {
        lastDay=sDay;
      }
      for(let d=sDay; d<=lastDay; d=addDays(d,1)){
        add(localDateKey(d),ev);
      }
    } else {
      const e=ev.end||ev.start;
      let d=startOfDay(ev.start);
      let stop=startOfDay(new Date(+e-1));  // end is exclusive at exact midnight
      if(stop<d) stop=d;                    // malformed end<start: keep start day
      for(; d<=stop; d=addDays(d,1)){
        add(localDateKey(d),ev);
      }
    }
  }
}
