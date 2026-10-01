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
  // Decided at write time below: a rebuild that lands mid-gesture must not
  // overwrite the viewer's position, so the input epoch is captured here,
  // before the row wipe, and compared when the restore is written.
  const prevInputEpoch = typeof scrollRootInputEpoch==="function" ? scrollRootInputEpoch(scroll) : 0;
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
      if(wx){ const w2=el("span","wx"),hi=el("b",null,wx.hi+"°"); w2.append(hi,document.createTextNode("/"+wx.lo+"°")); dnum.appendChild(w2); }
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
      // A gesture that arrived during this rebuild owns the position now, and
      // the scroll container itself survives the row wipe — so the correct
      // action is to leave its live offset alone instead of writing a captured
      // one back over the viewer's finger. (The agenda takes the other branch of
      // the same rule: its list is cleared, so it restores numerically. Neither
      // pane may overwrite a live gesture.)
      const gestureActive = typeof scrollRootInputEpoch==="function" && scrollRootInputEpoch(scroll)!==prevInputEpoch;
      if(!gestureActive){
        if(wasAtHome){
          scroll.scrollTop = homeTop;
        } else {
          // keep the viewer where they were (content height is stable)
          scroll.scrollTop = prevScroll;
        }
      }
    }
    scroll.dataset.built="1";
  }
  _calendarRenderSerial++;
  requestCalendarLayoutFit("renderCalendar",{force:true});
  if(typeof applySeasonalDecor==="function" && typeof seasonalDecorEnabledForCurrentTheme==="function" && seasonalDecorEnabledForCurrentTheme()) applySeasonalDecor();
  else if(typeof clearSeasonalDecor==="function") clearSeasonalDecor();
}
