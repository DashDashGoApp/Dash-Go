/* =====================================================================
   ============================  AGENDA  ===============================
   One hot native list root. Rebuilds preserve the first visible keyed row
   after a refresh, but only when no newer user input happened mid-render.
   ===================================================================== */
function agendaBindDelegatedOpen(list){
  if(!list||list.dataset.agendaDelegated==="1")return;
  list.dataset.agendaDelegated="1";
  list.addEventListener("click",e=>{
    const row=e.target.closest("[data-agenda-evid]");
    if(!row||!list.contains(row))return;
    const ev=list._agendaEvents&&list._agendaEvents[Number(row.dataset.agendaEvid)];
    if(ev){e.stopPropagation();showEventPopup(ev);}
  });
}
function agendaAllDayLabel(ev){
  const owner=String((ev&&ev.appOwner)||(ev&&ev.cal&&ev.cal.owner)||"").trim().toLowerCase();
  return owner==="chore-wheel"?"Chore":owner==="maintenance"?"Maint":owner==="routines"?"Routine":"all day";
}
function agendaAllDayNode(ev){
  const label=agendaAllDayLabel(ev),node=el("span","at allday",label);
  if(label==="Chore")node.setAttribute("aria-label","All-day chore assignment");
  else if(label==="Maint")node.setAttribute("aria-label","All-day maintenance task");
  else if(label==="Routine")node.setAttribute("aria-label","All-day routine schedule");
  return node;
}
function agendaDayKey(day){
  const m=String(day.getMonth()+1).padStart(2,"0"),d=String(day.getDate()).padStart(2,"0");
  return day.getFullYear()+"-"+m+"-"+d;
}
function agendaEventKey(ev,dayKey,ordinal){
  return [dayKey,ev&& (ev.uid||ev.id||ev.calendarId||ev.calId||""),ev&&ev.start?+new Date(ev.start):"",ev&&ev.end?+new Date(ev.end):"",ev&&ev.title||"",ordinal].join("|");
}
// Month cells deliberately put compact household app groups last so ordinary
// calendar detail has the scarce row budget. Agenda is the complementary
// household-action surface: Chores, Maintenance, and Routines are always first.
// This affects Agenda presentation only; timeline/cache/ICS order stays intact.
function agendaOwnerRank(ev){
  const info=appCalendarGroupInfo(appCalendarOwner(ev));
  return info&&Number.isFinite(+info.agendaRank)?+info.agendaRank:1000;
}
function agendaEventStableKey(ev){
  return String(ev&&(ev.uid||ev.id||ev.calendarId||ev.calId)||"");
}
function agendaEventComparator(left,right){
  const rankDelta=agendaOwnerRank(left)-agendaOwnerRank(right);
  if(rankDelta)return rankDelta;
  const leftAllDay=!!(left&&left.allDay),rightAllDay=!!(right&&right.allDay);
  if(leftAllDay!==rightAllDay)return leftAllDay?-1:1;
  const leftStart=left&&left.start?+new Date(left.start):0;
  const rightStart=right&&right.start?+new Date(right.start):0;
  if(leftStart!==rightStart)return leftStart-rightStart;
  const titleDelta=String(left&&left.title||"").localeCompare(String(right&&right.title||""));
  if(titleDelta)return titleDelta;
  return agendaEventStableKey(left).localeCompare(agendaEventStableKey(right));
}
function agendaOrderedEvents(events){
  const source=Array.isArray(events)?events:[];
  return source.length>1?[...source].sort(agendaEventComparator):source;
}
function agendaVisibleDays(){
  const weeks=Math.max(1,Math.round(Number(CONFIG.weeksBelow)||8));
  return weeks*7;
}
function renderAgenda(){
  const list=$("#agendalist"); if(!list)return;
  if(typeof scrollRootState==="function")scrollRootState(list,"hot-list");
  const anchor=typeof captureScrollAnchor==="function"?captureScrollAnchor(list,"[data-agenda-key]","agendaKey"):null;
  if(typeof dashboardListOverscanClear==="function")dashboardListOverscanClear(list);
  list.replaceChildren(); agendaBindDelegatedOpen(list);
  const frag=document.createDocumentFragment(); list._agendaEvents=[];
  const today=startOfDay(new Date());
  const relfmt=(d,i)=>{
    if(i===0)return "TODAY";
    if(i===1)return "TOMORROW";
    return "IN "+i+" DAYS";
  };
  for(let i=0,days=agendaVisibleDays();i<days;i++){
    const day=new Date(+today+i*DAY),dayKey=agendaDayKey(day);
    const dow=day.getDay(),evs=agendaOrderedEvents(eventsOnDay(day));
    const block=el("div","agday"+(i===0?" today":"")+(dow===6?" sat":dow===0?" sun":""));
    block.dataset.agendaDay=dayKey;block.dataset.agendaKey="day:"+dayKey;
    const wd=FMT.agDay.format(day);
    block.appendChild(el("div","lbl",relfmt(day,i)+" · "+wd));
    if(evs.length===0){
      // Skip empty future days to save space, but always show today.
      if(i!==0)continue;
      const empty=el("div","agev empty","nothing scheduled");
      empty.dataset.agendaKey="empty:"+dayKey;
      block.appendChild(empty);
    }
    let ordinal=0;
    for(const ev of evs){
      const row=el("div","agev");
      if(!ev.allDay){
        const tfmt=FMT.time,times=el("span","at");
        times.appendChild(el("span","start",tfmt.format(ev.start)));
        if(ev.end&&+ev.end!==+ev.start)times.appendChild(el("span","end",tfmt.format(ev.end)));
        row.appendChild(times);
      }else row.appendChild(agendaAllDayNode(ev));
      row.appendChild(el("span","agtitle",ev.title||"(no title)"));
      row.dataset.agendaEvid=String(list._agendaEvents.push(ev)-1);
      row.dataset.agendaKey="event:"+agendaEventKey(ev,dayKey,ordinal++);
      block.appendChild(row);
    }
    frag.appendChild(block);
  }
  list.appendChild(frag);
  if(typeof restoreScrollAnchor==="function")restoreScrollAnchor(list,anchor,"[data-agenda-key]","agendaKey");
  if(typeof dashboardListOverscanAfterRender==="function")dashboardListOverscanAfterRender(list,".agev");
}
