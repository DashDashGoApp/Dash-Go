// Agenda and Weather already use content-visibility:auto for their individual
// rows. Keep a small direction-aware warm window around the visible slice so
// WebKit has upcoming rows ready before a touch scroll reaches them. This is
// intentionally profile-neutral: the two lists stay small and benefit on every
// device, while the raw scroll path consumes only cached geometry.
const DASHBOARD_LIST_OVERSCAN_IDLE_ROWS=1;
const DASHBOARD_LIST_OVERSCAN_BEHIND_ROWS=1;
const DASHBOARD_LIST_OVERSCAN_AHEAD_ROWS=2;
const DASHBOARD_LIST_OVERSCAN_WARM_MS=250;
const DASHBOARD_LIST_OVERSCAN_SCROLL_EPSILON=1;
const DASHBOARD_LIST_OVERSCAN_STATE=new WeakMap();
function dashboardListOverscanCancel(state,name){
  if(state[name]) clearTimeout(state[name]);
  state[name]=0;
}
function dashboardListOverscanClearMarkers(items){
  for(const item of items||[]) if(item&&item.dataset) delete item.dataset.listCullNear;
}
function dashboardListOverscanClear(root){
  const state=root&&DASHBOARD_LIST_OVERSCAN_STATE.get(root);
  if(!state)return;
  if(state.raf&&typeof cancelAnimationFrame==="function")cancelAnimationFrame(state.raf);
  state.raf=0;
  dashboardListOverscanCancel(state,"idleTimer");
  dashboardListOverscanCancel(state,"warmTimer");
  if(state.onScroll&&root.removeEventListener)root.removeEventListener("scroll",state.onScroll);
  dashboardListOverscanClearMarkers(state.items);
  DASHBOARD_LIST_OVERSCAN_STATE.delete(root);
}
function dashboardListOverscanVisibleRange(state){
  const count=state.metrics.length;
  if(!count)return {first:0,last:-1};
  const top=Math.max(0,Number(state.root.scrollTop)||0),bottom=top+state.viewportHeight;
  let first=0;while(first<count&&state.metrics[first].bottom<=top)first++;
  let last=count-1;while(last>=first&&state.metrics[last].top>=bottom)last--;
  if(first>=count)first=count-1;
  if(last<first)last=first;
  return {first,last};
}
function dashboardListOverscanTarget(state,includeWarm){
  const range=dashboardListOverscanVisibleRange(state);
  let before=DASHBOARD_LIST_OVERSCAN_BEHIND_ROWS,after=DASHBOARD_LIST_OVERSCAN_BEHIND_ROWS;
  if(state.direction>0)after=DASHBOARD_LIST_OVERSCAN_AHEAD_ROWS;
  else if(state.direction<0)before=DASHBOARD_LIST_OVERSCAN_AHEAD_ROWS;
  const target=new Set();
  for(let i=Math.max(0,range.first-before);i<=Math.min(state.items.length-1,range.last+after);i++)target.add(i);
  if(includeWarm)for(const index of state.warm)target.add(index);
  return target;
}
function dashboardListOverscanApply(state,includeWarm){
  if(!state.items.length)return;
  const target=dashboardListOverscanTarget(state,includeWarm);
  for(const index of state.active){
    if(!target.has(index)&&state.items[index]?.dataset)delete state.items[index].dataset.listCullNear;
  }
  for(const index of target){
    if(!state.active.has(index)&&state.items[index]?.dataset)state.items[index].dataset.listCullNear="1";
  }
  state.active=target;
}
function dashboardListOverscanReleaseWarm(root){
  const state=DASHBOARD_LIST_OVERSCAN_STATE.get(root);if(!state)return;
  state.warmTimer=0;state.warm.clear();dashboardListOverscanApply(state,false);
}
function dashboardListOverscanSettle(root){
  const state=DASHBOARD_LIST_OVERSCAN_STATE.get(root);if(!state)return;
  state.idleTimer=0;state.warm=new Set(state.active);state.direction=0;
  dashboardListOverscanApply(state,true);
  dashboardListOverscanCancel(state,"warmTimer");
  state.warmTimer=setTimeout(()=>dashboardListOverscanReleaseWarm(root),DASHBOARD_LIST_OVERSCAN_WARM_MS);
}
function dashboardListOverscanScheduleSettle(root){
  const state=DASHBOARD_LIST_OVERSCAN_STATE.get(root);if(!state)return;
  dashboardListOverscanCancel(state,"idleTimer");
  state.idleTimer=setTimeout(()=>dashboardListOverscanSettle(root),DASHBOARD_LIST_OVERSCAN_WARM_MS);
}
function dashboardListOverscanOnScroll(root){
  const state=DASHBOARD_LIST_OVERSCAN_STATE.get(root);if(!state)return;
  const next=Math.max(0,Number(root.scrollTop)||0),delta=next-state.lastScrollTop;
  if(Math.abs(delta)>=DASHBOARD_LIST_OVERSCAN_SCROLL_EPSILON)state.direction=delta>0?1:-1;
  state.lastScrollTop=next;
  dashboardListOverscanCancel(state,"warmTimer");state.warm.clear();
  if(!state.raf)state.raf=requestAnimationFrame(()=>{
    const current=DASHBOARD_LIST_OVERSCAN_STATE.get(root);if(!current)return;
    current.raf=0;dashboardListOverscanApply(current,false);
  });
  dashboardListOverscanScheduleSettle(root);
}
// Capture one post-render geometry snapshot. No geometry reads occur from the
// passive scroll listener above, and the snapshot is refreshed after every
// explicit Agenda/Weather render transaction.
function dashboardListOverscanEnable(root,itemSelector){
  if(!root||!itemSelector)return;
  dashboardListOverscanClear(root);
  const items=Array.from(root.querySelectorAll?root.querySelectorAll(itemSelector):[]);
  if(!items.length)return;
  const rootRect=root.getBoundingClientRect();
  const state={root,items,metrics:items.map(item=>{
    const rect=item.getBoundingClientRect(),top=rect.top-rootRect.top+(Number(root.scrollTop)||0);
    return {top,bottom:top+Math.max(1,rect.height)};
  }),viewportHeight:Math.max(0,Number(root.clientHeight)||0),lastScrollTop:Math.max(0,Number(root.scrollTop)||0),direction:0,active:new Set(),warm:new Set(),raf:0,idleTimer:0,warmTimer:0,onScroll:null};
  state.onScroll=()=>dashboardListOverscanOnScroll(root);
  DASHBOARD_LIST_OVERSCAN_STATE.set(root,state);
  root.addEventListener&&root.addEventListener("scroll",state.onScroll,{passive:true});
  dashboardListOverscanApply(state,false);
}
function dashboardListOverscanAfterRender(root,itemSelector){
  if(!root)return;
  const defer=typeof requestAnimationFrame==="function"?requestAnimationFrame:fn=>setTimeout(fn,0);
  defer(()=>dashboardListOverscanEnable(root,itemSelector));
}
