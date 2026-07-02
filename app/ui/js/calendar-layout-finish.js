// Calendar fit completion callbacks stay outside the grid renderer so the
// renderer remains focused on DOM construction and the builder source cap.
const CALENDAR_LAYOUT_FIT_WAITERS=[];
function calendarAfterLayoutFit(renderSerial,callback){
  if(typeof callback!=="function")return;
  CALENDAR_LAYOUT_FIT_WAITERS.push({renderSerial:Math.max(0,Number(renderSerial)||0),callback});
}
function calendarLayoutFitDidComplete(renderSerial){
  const complete=Math.max(0,Number(renderSerial)||0);
  const ready=CALENDAR_LAYOUT_FIT_WAITERS.splice(0,CALENDAR_LAYOUT_FIT_WAITERS.length);
  for(const waiter of ready){
    if(waiter.renderSerial<=complete){
      try{waiter.callback();}catch(err){console.warn("calendar fit completion failed",err);}
    }else{
      CALENDAR_LAYOUT_FIT_WAITERS.push(waiter);
    }
  }
}

/* Day-cell autofit phases. finishCalendarDayEvents (calendar-grid.js) batches
   prepare (writes) → measure (reads) → apply (writes) across every event list
   so all measurements share one forced layout; these helpers live here to keep
   the grid renderer under its split-source navigability cap. */
function prepareDayEventListForMeasure(evlist){
  const total=+(evlist.dataset.totalEvents||0);
  const events=Array.from(evlist.children).filter(x=>x.classList && x.classList.contains("ev"));
  const more=Array.from(evlist.children).find(x=>x.dataset && x.dataset.moreRow==="1");
  const item={evlist,total,events,more,skip:false,available:0,heights:null,moreH:0};
  if(!total || !events.length){ item.skip=true; return item; }
  for(const e of events) e.classList.remove("autofit-hidden");
  if(more){
    more.classList.remove("autofit-hidden");
    more.style.visibility="hidden";
  }
  return item;
}
function measureDayEventList(item){
  if(item.skip) return;
  item.available=item.evlist.clientHeight;
  item.heights=item.events.map(e=>e.offsetHeight||0);
  item.moreH=item.more?(item.more.offsetHeight||0):0;
}
function applyDayEventListFit(item,gap){
  const {evlist,total,events,more}=item;
  if(item.skip){
    if(more) more.classList.add("autofit-hidden");
    return;
  }
  if(more) more.style.visibility="";
  const cap=Math.max(1,+(evlist.dataset.cap||CALENDAR_AUTOFIT_CANDIDATE_CAP));
  const limit=Math.min(events.length,cap);
  gap=Number.isFinite(gap)?gap:0;
  const available=item.available;
  const heights=item.heights;
  const moreH=item.moreH;
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
function fitDayEventList(evlist,gap){
  // Single-list compatibility path: same three phases against one list. The
  // calendar-wide pass in finishCalendarDayEvents batches these phases across
  // every list so all measurements share one forced layout.
  if(!evlist) return;
  const item=prepareDayEventListForMeasure(evlist);
  measureDayEventList(item);
  applyDayEventListFit(item,gap);
}
