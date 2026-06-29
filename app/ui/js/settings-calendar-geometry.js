/* =====================================================================
   ==============  CALENDAR GEOMETRY PREVIEW + COMMIT  =================
   Dashboard Control previews row/sidebar geometry through CSS immediately.
   It defers the expensive calendar rebuild, event/span fit pass, and Today
   anchor update until the Control overlay has actually closed.
   ===================================================================== */
const CALENDAR_GEOMETRY_TRANSACTION={pending:false,committing:false,anchor:null,serial:0};
function calendarGeometryCommitPending(){
  return !!(CALENDAR_GEOMETRY_TRANSACTION.pending||CALENDAR_GEOMETRY_TRANSACTION.committing);
}
function calendarGeometryClampRatio(value){
  const n=Number(value);
  return Number.isFinite(n)?Math.max(0,Math.min(1,n)):0;
}
function calendarGeometryCaptureAnchor(scroll){
  const target=scroll||$("#calscroll");
  if(!target)return {kind:"home"};
  const home=typeof calendarScrollHomeTop==="function"?calendarScrollHomeTop():0;
  const top=Math.max(0,Number(target.scrollTop)||0);
  if(!CALENDAR_SCROLL_HOME.ready||Math.abs(top-home)<8)return {kind:"home"};
  const rows=Array.from(target.querySelectorAll(".weekrow"));
  const currentIndex=rows.findIndex(row=>row.id==="currentweek");
  let rowIndex=rows.findIndex(row=>{
    const start=Number(row.offsetTop)||0,end=start+Math.max(1,Number(row.offsetHeight)||1);
    return top>=start&&top<end;
  });
  if(rowIndex<0)rowIndex=rows.length?Math.max(0,Math.min(rows.length-1,rows.findIndex(row=>(Number(row.offsetTop)||0)>top)-1)):0;
  const row=rows[rowIndex];
  if(!row||currentIndex<0)return {kind:"raw",top};
  const rowTop=Number(row.offsetTop)||0,rowHeight=Math.max(1,Number(row.offsetHeight)||1);
  return {kind:"week",weekOffset:rowIndex-currentIndex,ratio:calendarGeometryClampRatio((top-rowTop)/rowHeight),fallbackTop:top};
}
function calendarGeometryPreviewAllowed(){
  const ctrl=$("#ctrl");
  return !!(ctrl&&ctrl.classList&&ctrl.classList.contains("show"));
}
function calendarGeometryBeginPreview(){
  if(calendarGeometryCommitPending())return true;
  if(!calendarGeometryPreviewAllowed())return false;
  const scroll=$("#calscroll");
  CALENDAR_GEOMETRY_TRANSACTION.anchor=calendarGeometryCaptureAnchor(scroll);
  CALENDAR_GEOMETRY_TRANSACTION.pending=true;
  CALENDAR_GEOMETRY_TRANSACTION.serial++;
  if(typeof calendarSetWeekCullReady==="function")calendarSetWeekCullReady(false,scroll);
  if(typeof calendarScrollSnapSuspend==="function")calendarScrollSnapSuspend(true);
  return true;
}
function calendarGeometryRestoreAnchor(anchor,scroll){
  const target=scroll||$("#calscroll"),current=$("#currentweek");
  if(!target||!current)return;
  const home=Math.max(0,Number(current.offsetTop)||0);
  if(typeof setCalendarScrollHomeTop==="function")setCalendarScrollHomeTop(home);
  if(!anchor||anchor.kind==="home"){
    target.scrollTop=home;
    return;
  }
  if(anchor.kind==="week"){
    const rows=Array.from(target.querySelectorAll(".weekrow"));
    const currentIndex=rows.indexOf(current);
    const requested=currentIndex+Math.round(Number(anchor.weekOffset)||0);
    const row=rows[Math.max(0,Math.min(rows.length-1,requested))];
    if(row){
      const rowTop=Math.max(0,Number(row.offsetTop)||0);
      const rowHeight=Math.max(1,Number(row.offsetHeight)||1);
      target.scrollTop=Math.max(0,rowTop+rowHeight*calendarGeometryClampRatio(anchor.ratio));
      return;
    }
  }
  target.scrollTop=Math.max(0,Number(anchor.fallbackTop)||0);
}
function calendarGeometryFinishCommit(serial){
  if(serial!==CALENDAR_GEOMETRY_TRANSACTION.serial)return;
  const scroll=$("#calscroll");
  calendarGeometryRestoreAnchor(CALENDAR_GEOMETRY_TRANSACTION.anchor,scroll);
  CALENDAR_GEOMETRY_TRANSACTION.pending=false;
  CALENDAR_GEOMETRY_TRANSACTION.committing=false;
  CALENDAR_GEOMETRY_TRANSACTION.anchor=null;
  if(typeof calendarScrollSnapSuspend==="function")calendarScrollSnapSuspend(false);
  else if(typeof calendarScrollSnapReconcile==="function")calendarScrollSnapReconcile();
}
function calendarGeometryCommitAfterControlClose(){
  if(!CALENDAR_GEOMETRY_TRANSACTION.pending||CALENDAR_GEOMETRY_TRANSACTION.committing)return false;
  const serial=CALENDAR_GEOMETRY_TRANSACTION.serial;
  CALENDAR_GEOMETRY_TRANSACTION.pending=false;
  CALENDAR_GEOMETRY_TRANSACTION.committing=true;
  requestAnimationFrame(()=>requestAnimationFrame(()=>{
    if(serial!==CALENDAR_GEOMETRY_TRANSACTION.serial)return;
    const scroll=$("#calscroll");
    if(!scroll||typeof renderCalendar!=="function"||typeof calendarAfterLayoutFit!=="function"){
      calendarGeometryFinishCommit(serial);
      return;
    }
    renderCalendar({force:true,deferHome:true});
    const renderSerial=typeof calendarRenderSerial==="function"?calendarRenderSerial():0;
    calendarAfterLayoutFit(renderSerial,()=>calendarGeometryFinishCommit(serial));
  }));
  return true;
}
