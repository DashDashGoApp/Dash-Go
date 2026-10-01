const CALENDAR_AUTOFIT_CANDIDATE_CAP=16;

function isoWeekInfo(d){
  const x=new Date(d.getFullYear(), d.getMonth(), d.getDate());
  x.setHours(0,0,0,0);
  x.setDate(x.getDate()+3-((x.getDay()+6)%7));
  const week1=new Date(x.getFullYear(),0,4);
  const week=1+Math.round(((x-week1)/DAY-3+((week1.getDay()+6)%7))/7);
  return {year:x.getFullYear(), week};
}
function isoWeekLabel(d){
  const iso=isoWeekInfo(d);
  return "W"+String(iso.week).padStart(2,"0");
}
function calendarFlexGap(elm){
  const cs=getComputedStyle(elm);
  const raw=cs.rowGap||cs.gap||"0";
  const n=parseFloat(raw);
  return Number.isFinite(n)?n:0;
}
function calendarEventSignature(){
  let h=2166136261;
  const mix=(v)=>{ h^=(v>>>0); h=Math.imul(h,16777619); };
  mix(EVENTS.length||0);
  for(const ev of EVENTS){
    mix(Math.floor((+ev.start||0)/60000));
    mix(Math.floor((+(ev.end||ev.start)||0)/60000));
    mix((ev.title||"").length);
    mix((ev.uid||"").length);
    mix(((ev.cal&&ev.cal.name)||"").length);
    mix(String(ev.appOwner||((ev.cal&&ev.cal.owner)||"")).length);
    if(ev.allDay) mix(17);
  }
  return (h>>>0).toString(36);
}
function calendarWeatherSignature(){
  if(!WX || !WX.daily || !Array.isArray(WX.daily.time)) return "nowx";
  const n=Math.min((CONFIG.weeksAbove+1+CONFIG.weeksBelow)*7, WX.daily.time.length, Math.max(1,Number(CONFIG.weatherForecastMaxDays)||16));
  const d=WX.daily;
  let out="";
  for(let i=0;i<n;i+=1){
    out+=`${d.time[i]||""}:${Math.round(d.temperature_2m_max&&d.temperature_2m_max[i]||0)}/${Math.round(d.temperature_2m_min&&d.temperature_2m_min[i]||0)};`;
  }
  return out;
}
function calendarLayoutSignature(scroll){
  scroll=scroll||$("#calscroll");
  const todayKey=localDateKey(startOfDay(new Date()));
  const settings=typeof dashboardRuntimeSettings==="function"?dashboardRuntimeSettings():null;
  const font=(settings&&settings.fontPreset)||CONFIG.fontPreset||"default";
  const decor=typeof seasonalDecorSignature==="function"?seasonalDecorSignature():((settings&&settings.seasonalDecor)||CONFIG.seasonalDecor||"off");
  const profile=String((settings&&settings.profile)||CONFIG.profile||"").toLowerCase();
  return [
    todayKey, CONFIG.weeksAbove, CONFIG.weeksBelow, CONFIG.firstDayOfWeek,
    CALENDAR_AUTOFIT_CANDIDATE_CAP, CONFIG.showIsoWeekNumbers?1:0, CONFIG.clock24?1:0,
    font, decor, profile,
    scroll?scroll.clientWidth:0, scroll?scroll.clientHeight:0,
    calendarEventSignature(), calendarWeatherSignature()
  ].join("|");
}
let _fitDayEventsTimer=0;
let _fitDayEventsReadTimer=0;
let _fitDayEventsWriteTimer=0;
let _fitDayEventsFinishTimer=0;
let _fitDayEventsDebounce=0;
let _calendarFitSig="";
let _calendarRenderSig="";
let _calendarRenderSerial=0;
function calendarRenderSerial(){return _calendarRenderSerial;}
function cancelCalendarFitTimers(){
  if(_fitDayEventsTimer) cancelAnimationFrame(_fitDayEventsTimer);
  if(_fitDayEventsReadTimer) cancelAnimationFrame(_fitDayEventsReadTimer);
  if(_fitDayEventsWriteTimer) cancelAnimationFrame(_fitDayEventsWriteTimer);
  if(_fitDayEventsFinishTimer) cancelAnimationFrame(_fitDayEventsFinishTimer);
  _fitDayEventsTimer=_fitDayEventsReadTimer=_fitDayEventsWriteTimer=_fitDayEventsFinishTimer=0;
}
function finishCalendarDayEvents(){
  _fitDayEventsFinishTimer=0;
  const lists=Array.from(document.querySelectorAll("#calscroll .evlist[data-auto-fit='1']"));
  const fitGap=lists.length?calendarFlexGap(lists[0]):0;
  // Batched write→read→write: unclamp every list first, take every
  // measurement against ONE forced layout, then apply every clamp decision.
  // Interleaving these phases per cell forced a reflow per day cell, which
  // was the largest remaining per-fit cost on single-core boards.
  const items=lists.map(prepareDayEventListForMeasure);
  for(const item of items) measureDayEventList(item);
  for(const item of items) applyDayEventListFit(item,fitGap);
  // All geometry-dependent reads/writes are complete. Lite may now let the
  // browser skip paint/layout work for week rows outside the scroll viewport.
  calendarSetWeekCullReady(true);
  if(typeof calendarLayoutFitDidComplete==="function")calendarLayoutFitDidComplete(_calendarRenderSerial);
  if(typeof calendarScrollSnapReconcile==="function")calendarScrollSnapReconcile();
}
function runCalendarFitPipeline(reason){
  _fitDayEventsTimer=0;
  resetCalendarSpanOffsets();
  _fitDayEventsReadTimer=requestAnimationFrame(()=>{
    _fitDayEventsReadTimer=0;
      const updates=collectCalendarSpanOffsets();
    _fitDayEventsWriteTimer=requestAnimationFrame(()=>{
      _fitDayEventsWriteTimer=0;
      applyCalendarSpanOffsets(updates);
      _fitDayEventsFinishTimer=requestAnimationFrame(()=>finishCalendarDayEvents());
    });
  });
}
function requestCalendarLayoutFit(reason,opts){
  const scroll=$("#calscroll");
  if(!scroll) return;
  const sig=calendarLayoutSignature(scroll);
  const force=!!(opts&&opts.force);
  if(!force && sig===_calendarFitSig){
    return;
  }
  _calendarFitSig=sig;
  // Force every row live before a span/event measurement pass. Without this,
  // an off-screen content-visibility placeholder could yield incomplete rects.
  calendarSetWeekCullReady(false,scroll);
  cancelCalendarFitTimers();
  _fitDayEventsTimer=requestAnimationFrame(()=>runCalendarFitPipeline(reason||"scheduled"));
}
function debounceCalendarLayoutFit(reason,delay){
  clearTimeout(_fitDayEventsDebounce);
  _fitDayEventsDebounce=setTimeout(()=>requestCalendarLayoutFit(reason||"debounced"),delay==null?90:delay);
}
if(typeof window!=="undefined"){
  window.addEventListener("resize",()=>{
    debounceCalendarLayoutFit("resize",100);
  });
}
