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
function fitDayEventList(evlist,gap){
  if(!evlist) return;
  const total=+(evlist.dataset.totalEvents||0);
  const events=Array.from(evlist.children).filter(x=>x.classList && x.classList.contains("ev"));
  const more=Array.from(evlist.children).find(x=>x.dataset && x.dataset.moreRow==="1");
  if(!total || !events.length){
    if(more) more.classList.add("autofit-hidden");
    return;
  }
  const cap=Math.max(1,+(evlist.dataset.cap||CALENDAR_AUTOFIT_CANDIDATE_CAP));
  const limit=Math.min(events.length,cap);
  for(const e of events) e.classList.remove("autofit-hidden");
  if(more){
    more.classList.remove("autofit-hidden");
    more.style.visibility="hidden";
  }
  const available=evlist.clientHeight;
  gap=Number.isFinite(gap)?gap:0;
  const heights=events.map(e=>e.offsetHeight||0);
  const moreH=more?(more.offsetHeight||0):0;
  if(more) more.style.visibility="";
  const sumHeights=(n)=>{
    let h=0;
    for(let i=0;i<n;i++) h+=heights[i]||0;
    if(n>1) h+=gap*(n-1);
    return h;
  };
  // If every event fits and the safety cap did not hide any events, no "+N"
  // row is needed.
  if(total<=limit && sumHeights(total)<=available){
    for(let i=0;i<events.length;i++) events[i].classList.toggle("autofit-hidden",i>=total);
    if(more) more.classList.add("autofit-hidden");
    return;
  }
  // Reserve one row for "+N more" before choosing visible events, so the
  // indicator never becomes a surprise extra row that overflows the cell.
  let visible=0;
  const maxVisible=Math.max(0,Math.min(limit,total)-1);
  for(let n=0;n<=maxVisible;n++){
    const h=sumHeights(n)+moreH+(n>0?gap:0);
    if(h<=available || n===0) visible=n;
    else break;
  }
  for(let i=0;i<events.length;i++) events[i].classList.toggle("autofit-hidden",i>=visible);
  if(more){
    const hidden=Math.max(0,total-visible);
    more.textContent="+"+hidden+" more";
    more.classList.toggle("autofit-hidden",hidden<=0);
  }
}
function resetCalendarSpanOffsets(){
  document.querySelectorAll("#calscroll .evlist").forEach(evlist=>{
    evlist.style.marginTop="";
    evlist.style.setProperty("--cell-span-offset","0px");
  });
}
function collectCalendarSpanOffsets(){
  // Day event lists are absolute-positioned at the same top origin as
  // multi-day span bars. For cells crossed by spans, push only that cell's
  // list down to just under the real rendered span stack.
  const updates=[];
  const rows=document.querySelectorAll("#calscroll .weekrow");
  rows.forEach(row=>{
    const children=Array.from(row.children||[]);
    const bars=children.filter(x=>x.classList && x.classList.contains("spanbar"));
    const cells=children.filter(x=>x.classList && x.classList.contains("daycell"));
    if(!bars.length || !cells.length) return;
    const rowTop=row.getBoundingClientRect().top;
    const barRects=bars.map(bar=>({
      c0:+(bar.dataset.c0||0),
      c1:+(bar.dataset.c1||-1),
      bottom:bar.getBoundingClientRect().bottom-rowTop
    }));
    cells.forEach((cell,idx)=>{
      const evlist=cell.querySelector && cell.querySelector(".evlist");
      if(!evlist || +(cell.dataset.cellLanes||0)<=0) return;
      const evTop=evlist.getBoundingClientRect().top-rowTop;
      let spanBottom=0;
      for(const br of barRects){
        if(idx<br.c0 || idx>br.c1) continue;
        if(br.bottom>spanBottom) spanBottom=br.bottom;
      }
      const offset=spanBottom?Math.max(0,Math.ceil(spanBottom+2-evTop)):0;
      updates.push({evlist,offset});
    });
  });
  return updates;
}
function applyCalendarSpanOffsets(updates){
  for(const u of updates){
    u.evlist.style.marginTop="";
    u.evlist.style.setProperty("--cell-span-offset",(u.offset||0)+"px");
  }
}
function fitCalendarSpanOffsets(){
  resetCalendarSpanOffsets();
  const updates=collectCalendarSpanOffsets();
  applyCalendarSpanOffsets(updates);
  return updates.length;
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
  lists.forEach(evlist=>fitDayEventList(evlist,fitGap));
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
function renderCalendar(opts){
  const scroll=$("#calscroll");
  if(!scroll) return;
  const renderSig=calendarLayoutSignature(scroll);
  const force=!!(opts && (opts.force || opts==="force"));
  const deferHome=!!(opts&&opts.deferHome);
  if(!force && scroll.dataset.built==="1" && renderSig===_calendarRenderSig){
    return;
  }
  _calendarRenderSig=renderSig;
  // A new row transaction always starts uncullled so the fit pipeline measures
  // the actual row and event geometry before Lite steady-state culling resumes.
  calendarSetWeekCullReady(false,scroll);
  renderCalHead();
  // Capture where the viewer was BEFORE we wipe the rows, so a background
  // refresh can restore their position instead of snapping to today.
  const prevScroll = scroll.scrollTop;
  const prevHome = $("#currentweek");
  // Read this once during the render transaction. The scroll handler consumes
  // the cached numeric position via calendarScrollHomeTop().
  const prevHomeTop = typeof calendarScrollHomeTop==="function" ? calendarScrollHomeTop() : (prevHome ? prevHome.offsetTop : 0);
  scroll.innerHTML="";
  const today=startOfDay(new Date());
  const firstWeek=addDays(startOfWeek(today),-CONFIG.weeksAbove*7);  // DST-safe
  const totalWeeks=CONFIG.weeksAbove+1+CONFIG.weeksBelow;
  const mfmt=FMT.monthS;
  const spans=buildSpanLayout(firstWeek,totalWeeks);
  const style=getComputedStyle(document.documentElement);
  const spanTop=Math.max(0,parseFloat(style.getPropertyValue("--cell-head"))||34);
  const spanLaneStep=Math.max(1,parseFloat(style.getPropertyValue("--spanbar-lane-step"))||30);
  const frag=document.createDocumentFragment();   // batch: one insertion below

  for(let w=0;w<totalWeeks;w++){
    const row=el("div","weekrow");
    const weekStart=addDays(firstWeek,w*7);                          // DST-safe
    // stripe alternate weeks by absolute week number so it stays consistent
    if(Math.round(+startOfDay(weekStart)/ (7*DAY)) % 2 === 0) row.classList.add("alt");
    if(w===CONFIG.weeksAbove) row.id="currentweek";
    // multi-day bars for this week. Each day cell reserves space only for
    // span lanes that actually cross THAT day, so unrelated days in the
    // same week do not lose a usable event line.
    const wk=spans.weeks[w];
    row.style.setProperty("--lanes", wk.items.length? wk.laneCount : 0);
    for(const it of wk.items){
      const segOther = (()=>{
        for(let d=it.c0; d<=it.c1; d++){
          if(addDays(weekStart,d).getMonth()===today.getMonth()) return false;
        }
        return true;
      })();
      const bar=el("div","spanbar "+classify(it.ev)+(segOther?" other":""));
      // Match each multi-day bar to the same left/right inset used by normal
      // per-day event rows, so the colored rails line up within a day cell.
      bar.style.left="calc("+(it.c0/7*100)+"% + 4px)";
      bar.style.width="calc("+((it.c1-it.c0+1)/7*100)+"% - 8px)";
      bar.style.top=(spanTop+it.lane*spanLaneStep)+"px";
      bar.dataset.c0=String(it.c0); bar.dataset.c1=String(it.c1);
      bar.style.borderLeftColor=it.ev.cal.color||"var(--accent)";
      fillSpanBar(bar,it);
      bar.addEventListener("click",(e)=>{ e.stopPropagation(); showEventPopup(it.ev); });
      row.appendChild(bar);
    }
    for(let i=0;i<7;i++){
      const day=addDays(weekStart,i);                                // DST-safe
      const dow=day.getDay();
      const cell=el("div","daycell"+
        (sameDay(day,today)?" today":"")+
        (dow===6?" sat":dow===0?" sun":"")+
        (day.getMonth()!==today.getMonth()?" other":""));
      const cellLanes = wk.items.reduce((m,it)=>(i>=it.c0 && i<=it.c1)?Math.max(m,it.lane+1):m,0);
      if(cellLanes){ cell.dataset.cellLanes=String(cellLanes); cell.style.setProperty("--cell-lanes", cellLanes); }
      // day-number header + tiny weather
      const dnum=el("div","dnum");
      const showMonth=(day.getDate()===1);
      const dlabel=showMonth?mfmt.format(day)+" "+day.getDate():String(day.getDate());
      const dwrap=el("span","dwrap");
      dwrap.appendChild(el("span","d",dlabel));
      if(i===0 && CONFIG.showIsoWeekNumbers){
        const badge=el("span","weeknum",isoWeekLabel(weekStart));
        badge.setAttribute("aria-label","ISO week "+isoWeekInfo(weekStart).week);
        dwrap.appendChild(badge);
      }
      dnum.appendChild(dwrap);
      const wx=wxForDay(day);
      if(wx){ const w2=el("span","wx"); w2.innerHTML=`<b>${wx.hi}°</b>/${wx.lo}°`; dnum.appendChild(w2); }
      cell.appendChild(dnum);
      // events — render candidates, then auto-fit them to the real cell height.
      // "+N more" reserves its own row when needed; tapping the cell (or the
      // more line) opens a popup listing the whole day.
      const evlist=el("div","evlist");
      evlist.dataset.autoFit="1";
      const evs=eventsOnDay(day);
      // Events shown as a span bar above are excluded from the cell list
      // (the day popup still lists everything).
      const cellEvs=evs.filter(e=>!spans.spanSet.has(e));
      const rows=calendarCellDisplayRows(cellEvs);
      const cap=CALENDAR_AUTOFIT_CANDIDATE_CAP;
      const candidates=rows.slice(0,cap);
      evlist.dataset.totalEvents=String(rows.length);
      evlist.dataset.cap=String(cap);
      for(const rowData of candidates){
        const isGroup=rowData.kind==="app-group",ev=rowData.event;
        const e=el("div","ev "+(isGroup?"appcalgroup":classify(ev)));
        e.style.borderLeftColor=isGroup?rowData.color:ev.cal.color||"var(--accent)";
        if(isGroup){
          e.dataset.appOwner=rowData.owner;
          e.appendChild(el("span","etitle",appCalendarGroupTitle(rowData)));
          e.appendChild(el("span","appgroup-hint","Open"));
          e.addEventListener("click",event=>{ event.stopPropagation(); showAppCalendarGroupPopup(day,rowData); });
        }else{
          if(!ev.allDay){
            const t=el("span","t",FMT.time.format(ev.start));
            e.appendChild(t);
          }
          e.appendChild(el("span","etitle",ev.title||"(no title)"));
          e.addEventListener("click",(ev2)=>{ ev2.stopPropagation(); showEventPopup(ev); });
        }
        evlist.appendChild(e);
      }
      if(rows.length){
        const more=el("div","more autofit-hidden","+0 more");
        more.dataset.moreRow="1";
        evlist.appendChild(more);
      }
      cell.appendChild(evlist);
      // whole-cell tap → day popup (only when there are events)
      if(evs.length){
        cell.addEventListener("click",()=>showDayPopup(day,evs));
      }
      row.appendChild(cell);
    }
    frag.appendChild(row);
  }
  scroll.appendChild(frag);
  // Position so the current week is the top visible row — but ONLY on the
  // first build or when the viewer is already parked at "today". This stops
  // background refreshes (weather/calendar/midnight) from yanking the view
  // back while someone is scrolling through future or past weeks.
  const cw=$("#currentweek");
  if(cw){
    // This is the only current-week layout read. It happens after the batched
    // fragment insert, never in a raw scroll callback. A Control geometry
    // transaction defers publication/restoration until the post-fit callback
    // so it writes the final home offset exactly once.
    const homeTop=cw.offsetTop;
    if(!deferHome){
      if(typeof setCalendarScrollHomeTop==="function")setCalendarScrollHomeTop(homeTop);
      const wasAtHome = (scroll.dataset.built!=="1") ||
                        Math.abs(prevScroll - prevHomeTop) < 6;
      if(wasAtHome){
        scroll.scrollTop = homeTop;
      } else {
        // keep the viewer where they were (content height is stable)
        scroll.scrollTop = prevScroll;
      }
    }
    scroll.dataset.built="1";
  }
  _calendarRenderSerial++;
  requestCalendarLayoutFit("renderCalendar",{force:true});
  if(typeof applySeasonalDecor==="function" && typeof seasonalDecorEnabledForCurrentTheme==="function" && seasonalDecorEnabledForCurrentTheme()) applySeasonalDecor();
  else if(typeof clearSeasonalDecor==="function") clearSeasonalDecor();
}
