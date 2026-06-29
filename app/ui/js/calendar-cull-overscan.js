// Lite Calendar row culling is a steady-state paint optimization. It runs only
// after the existing event/span geometry pass and keeps a small warm window in
// the scroll direction so WebKitGTK has rows ready before they enter view.
const CALENDAR_WEEK_CULL_IDLE_OVERSCAN=1;
const CALENDAR_WEEK_CULL_BEHIND_OVERSCAN=1;
const CALENDAR_WEEK_CULL_AHEAD_OVERSCAN=2;
const CALENDAR_WEEK_CULL_WARM_MS=250;
const CALENDAR_WEEK_CULL_SCROLL_EPSILON=1;
const CALENDAR_WEEK_CULL_STATE={
  scroll:null,rows:[],metrics:[],viewportHeight:0,lastScrollTop:0,direction:0,
  active:new Set(),warm:new Set(),prewarm:new Set(),raf:0,idleTimer:0,warmTimer:0,onScroll:null
};
function calendarWeekCullLiteProfile(){
  return typeof liteVisualProfile==="function"&&!!liteVisualProfile();
}
function calendarWeekCullCancelTimer(name){
  const state=CALENDAR_WEEK_CULL_STATE;
  if(state[name])clearTimeout(state[name]);
  state[name]=0;
}
function calendarWeekCullClearMarkers(rows){
  for(const row of rows||[])if(row&&row.dataset)delete row.dataset.cullNear;
}
function calendarWeekCullDisable(scroll){
  const state=CALENDAR_WEEK_CULL_STATE;
  if(state.raf&&typeof cancelAnimationFrame==="function")cancelAnimationFrame(state.raf);
  state.raf=0;
  calendarWeekCullCancelTimer("idleTimer");
  calendarWeekCullCancelTimer("warmTimer");
  if(state.scroll&&state.onScroll&&state.scroll.removeEventListener)state.scroll.removeEventListener("scroll",state.onScroll);
  calendarWeekCullClearMarkers(state.rows);
  state.scroll=null;state.rows=[];state.metrics=[];state.viewportHeight=0;
  state.lastScrollTop=0;state.direction=0;state.active.clear();state.warm.clear();state.prewarm.clear();state.onScroll=null;
}
function calendarWeekCullRangeAt(state,top){
  const count=state.metrics.length;
  if(!count)return {first:0,last:-1};
  const start=Math.max(0,Number(top)||0),bottom=start+Math.max(0,state.viewportHeight);
  let first=0;
  while(first<count&&state.metrics[first].bottom<=start)first++;
  let last=count-1;
  while(last>=first&&state.metrics[last].top>=bottom)last--;
  if(first>=count)first=count-1;
  if(last<first)last=first;
  return {first,last};
}
function calendarWeekCullVisibleRange(state){
  return calendarWeekCullRangeAt(state,Number(state.scroll&&state.scroll.scrollTop)||0);
}
function calendarWeekCullRowsAt(state,top,before,after){
  const range=calendarWeekCullRangeAt(state,top),target=new Set();
  for(let i=Math.max(0,range.first-Math.max(0,before||0));i<=Math.min(state.rows.length-1,range.last+Math.max(0,after||0));i++)target.add(i);
  return target;
}
function calendarWeekCullTarget(state,includeWarm){
  const range=calendarWeekCullVisibleRange(state);
  let before=CALENDAR_WEEK_CULL_BEHIND_OVERSCAN,after=CALENDAR_WEEK_CULL_BEHIND_OVERSCAN;
  if(state.direction>0)after=CALENDAR_WEEK_CULL_AHEAD_OVERSCAN;
  else if(state.direction<0)before=CALENDAR_WEEK_CULL_AHEAD_OVERSCAN;
  const target=calendarWeekCullRowsAt(state,Number(state.scroll&&state.scroll.scrollTop)||0,before,after);
  if(includeWarm)for(const i of state.warm)target.add(i);
  for(const i of state.prewarm)target.add(i);
  return target;
}
function calendarWeekCullApply(includeWarm){
  const state=CALENDAR_WEEK_CULL_STATE;
  if(!state.scroll||!state.rows.length)return;
  const target=calendarWeekCullTarget(state,includeWarm);
  for(let i=0;i<state.rows.length;i++){
    const row=state.rows[i];
    if(!row||!row.dataset)continue;
    if(target.has(i))row.dataset.cullNear="1";
    else delete row.dataset.cullNear;
  }
  state.active=target;
}
function calendarWeekCullReleaseWarmRows(){
  const state=CALENDAR_WEEK_CULL_STATE;
  state.warmTimer=0;state.warm.clear();calendarWeekCullApply(false);
}
function calendarWeekCullSettle(){
  const state=CALENDAR_WEEK_CULL_STATE;
  state.idleTimer=0;state.warm=new Set(state.active);state.direction=0;
  calendarWeekCullApply(true);
  calendarWeekCullCancelTimer("warmTimer");
  state.warmTimer=setTimeout(calendarWeekCullReleaseWarmRows,CALENDAR_WEEK_CULL_WARM_MS);
}
function calendarWeekCullScheduleSettle(){
  const state=CALENDAR_WEEK_CULL_STATE;
  calendarWeekCullCancelTimer("idleTimer");
  state.idleTimer=setTimeout(calendarWeekCullSettle,CALENDAR_WEEK_CULL_WARM_MS);
}
function calendarWeekCullOnScroll(){
  const state=CALENDAR_WEEK_CULL_STATE;
  const next=Math.max(0,Number(state.scroll&&state.scroll.scrollTop)||0),delta=next-state.lastScrollTop;
  if(Math.abs(delta)>=CALENDAR_WEEK_CULL_SCROLL_EPSILON)state.direction=delta>0?1:-1;
  state.lastScrollTop=next;
  calendarWeekCullCancelTimer("warmTimer");state.warm.clear();
  if(!state.raf)state.raf=requestAnimationFrame(()=>{state.raf=0;calendarWeekCullApply(false);});
  calendarWeekCullScheduleSettle();
}
function calendarWeekCullViewportHeight(){return Math.max(0,Number(CALENDAR_WEEK_CULL_STATE.viewportHeight)||0);}
function calendarWeekCullCanPrewarmAt(top){
  const state=CALENDAR_WEEK_CULL_STATE;
  return !!(state.scroll&&state.rows.length&&state.metrics.length&&Number.isFinite(Number(top)));
}
function calendarWeekCullPrewarmAt(top,options){
  const state=CALENDAR_WEEK_CULL_STATE;
  if(!calendarWeekCullCanPrewarmAt(top))return false;
  const before=Math.max(0,Number(options&&options.before)||2),after=Math.max(0,Number(options&&options.after)||2);
  state.prewarm=calendarWeekCullRowsAt(state,top,before,after);
  calendarWeekCullApply(true);
  return state.prewarm.size>0;
}
function calendarWeekCullClearPrewarm(){
  const state=CALENDAR_WEEK_CULL_STATE;
  if(!state.prewarm.size)return;
  state.prewarm.clear();calendarWeekCullApply(false);
}
function calendarWeekCullCommitAt(top){
  const state=CALENDAR_WEEK_CULL_STATE;
  if(!state.scroll)return;
  state.lastScrollTop=Math.max(0,Number(top)||0);state.direction=0;state.warm.clear();state.prewarm.clear();
  calendarWeekCullApply(false);
}
function calendarWeekCullEnable(scroll){
  if(!scroll||!calendarWeekCullLiteProfile())return;
  const state=CALENDAR_WEEK_CULL_STATE;
  if(state.scroll)calendarWeekCullDisable(state.scroll);
  const rows=Array.from(scroll.querySelectorAll?scroll.querySelectorAll(".weekrow"):[]);
  state.scroll=scroll;state.rows=rows;
  // Capture row geometry once after Calendar's existing fit pipeline. Scroll
  // work below consumes these cached offsets/heights and never forces layout.
  state.metrics=rows.map(row=>{
    const top=Number(row.offsetTop)||0,height=Math.max(1,Number(row.offsetHeight)||0);
    return {top,bottom:top+height};
  });
  state.viewportHeight=Math.max(0,Number(scroll.clientHeight)||0);
  state.lastScrollTop=Math.max(0,Number(scroll.scrollTop)||0);state.direction=0;
  state.onScroll=calendarWeekCullOnScroll;
  scroll.addEventListener&&scroll.addEventListener("scroll",state.onScroll,{passive:true});
  calendarWeekCullApply(false);
}
// `content-visibility:auto` can defer off-screen layout. Keep it disabled until
// Calendar's span/event fitting is done; the CSS marker remains Lite-scoped.
function calendarSetWeekCullReady(ready,scroll){
  const target=scroll||$("#calscroll");
  if(!target||!target.dataset)return;
  if(ready){
    target.dataset.weekCullReady="1";
    calendarWeekCullEnable(target);
  }else{
    delete target.dataset.weekCullReady;
    calendarWeekCullDisable(target);
  }
}
