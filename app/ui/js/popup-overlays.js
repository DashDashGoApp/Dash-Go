// 05-popups-00-overlays.js — shared overlay lifecycle and first-paint popup transactions.
const POPUP_IDLE_MS=3*60000;
let POPUP_IDLE_TIMER=null;
let POPUP_RENDER_TOKEN=0;
let POPUP_DEFERRED=new Set();
function overlayIsOpen(){
  return !!((document.getElementById("ctrl")||{}).classList?.contains("show") ||
            (document.getElementById("scrim")||{}).classList?.contains("show") ||
            (document.getElementById("applauncher")||{}).classList?.contains("show") ||
            (document.getElementById("chorewheel")||{}).classList?.contains("show") ||
            (document.getElementById("familyboard")||{}).classList?.contains("show") ||
            (document.getElementById("maintenance")||{}).classList?.contains("show") ||
            (document.getElementById("routines")||{}).classList?.contains("show") ||
            (document.getElementById("listsapp")||{}).classList?.contains("show") ||
            (typeof fitDockSheetIsOpen==="function" && fitDockSheetIsOpen()));
}
function disarmOverlayAutoClose(){
  if(POPUP_IDLE_TIMER){ clearTimeout(POPUP_IDLE_TIMER); POPUP_IDLE_TIMER=null; }
}
function closeOverlaysForIdle(){
  const scrim=document.getElementById("scrim"),ctrl=document.getElementById("ctrl");
  if(scrim&&scrim.classList.contains("show"))closeScrim();
  if(ctrl&&ctrl.classList.contains("show"))closeCtrl();
  if(typeof appLauncherIsOpen==="function" && appLauncherIsOpen())closeAppLauncher();
  if(typeof choreWheelIsOpen==="function" && choreWheelIsOpen())closeChoreWheel();
  if(typeof familyBoardIsOpen==="function" && familyBoardIsOpen())closeFamilyBoard();
  if(typeof maintenanceIsOpen==="function" && maintenanceIsOpen())closeMaintenance();
  if(typeof routinesIsOpen==="function" && routinesIsOpen())closeRoutines();
  if(typeof listsAppIsOpen==="function" && listsAppIsOpen())closeListsApp();
  if(typeof fitDockSheetIsOpen==="function" && fitDockSheetIsOpen() && typeof closeDashboardFitSheet==="function")closeDashboardFitSheet(false);
  disarmOverlayAutoClose();
}
function armOverlayAutoClose(){
  disarmOverlayAutoClose();
  if(overlayIsOpen())POPUP_IDLE_TIMER=setTimeout(closeOverlaysForIdle,POPUP_IDLE_MS);
}
function noteOverlayInput(){
  if(mapFullIsOpen()){noteMapFullInput();return;}
  if(overlayIsOpen())armOverlayAutoClose();
}
["pointerdown","mousedown","touchstart","wheel","keydown"].forEach(t=>document.addEventListener(t,noteOverlayInput,{capture:true,passive:true}));
function setPopupMode(cls){
  if(cls!=="messagepop"&&typeof releaseMessagePopupRotationPause==="function")releaseMessagePopupRotationPause();
  const pop=$("#pop");if(!pop)return;
  pop.classList.remove("daytimelinepop","weatherpop","messagepop","eventpop","managedschedulepop","healthwarningpop","calendareventform","calendarconfirm");
  if(cls)pop.classList.add(cls);
}
function popupNextFrame(fn){
  if(typeof requestAnimationFrame==="function")requestAnimationFrame(fn);
  else setTimeout(fn,0);
}
// Popup latency marks. Marks only: no timer, no resident consumer, no network.
// A bounded ring holds the last few milestones — bounded because the kiosk never
// reloads — so a device measurement can read them and a smoke can assert that the
// shell never waits on the network.
const POPUP_TIMING_BUDGET={shellMs:100,contentMs:250,settledMs:800,hydrateMs:1500};
const POPUP_TIMING_KEEP=8;
let POPUP_TIMING_CLOCK=null,POPUP_TIMING_OPEN=-1,POPUP_TIMING_LAST=[];
function popupTimingEnabled(){
  if(POPUP_TIMING_CLOCK===null)
    POPUP_TIMING_CLOCK=typeof performance!=="undefined"&&typeof performance.now==="function"&&typeof performance.mark==="function"&&
      (typeof dashboardLiteProfile!=="function"||dashboardLiteProfile());
  return POPUP_TIMING_CLOCK;
}
function popupTimingBudget(hop){
  if(hop==="shell")return POPUP_TIMING_BUDGET.shellMs;
  if(hop==="content")return POPUP_TIMING_BUDGET.contentMs;
  if(hop==="settled")return POPUP_TIMING_BUDGET.settledMs;
  return POPUP_TIMING_BUDGET.hydrateMs;
}
function popupTimingBegin(){
  // -1, not 0: a clock legitimately reads 0 at the very start of a session, and a
  // falsy sentinel would silently drop every mark of that first open.
  POPUP_TIMING_OPEN=popupTimingEnabled()?performance.now():-1;
}
function popupTimingMark(hop){
  if(!popupTimingEnabled()||POPUP_TIMING_OPEN<0)return 0;
  const elapsed=performance.now()-POPUP_TIMING_OPEN;
  POPUP_TIMING_LAST.push({hop,ms:Math.round(elapsed),budget:popupTimingBudget(hop)});
  if(POPUP_TIMING_LAST.length>POPUP_TIMING_KEEP)POPUP_TIMING_LAST.splice(0,POPUP_TIMING_LAST.length-POPUP_TIMING_KEEP);
  try{performance.mark("popup:"+hop);}catch(_){}
  return elapsed;
}
function popupTimingRecent(){return POPUP_TIMING_LAST.slice();}
function popupTimingOverBudget(){return POPUP_TIMING_LAST.filter(entry=>entry.ms>entry.budget);}
function popupInvalidateWork(){
  POPUP_RENDER_TOKEN++;
  POPUP_LIVE_REFRESH=null;
  for(const task of POPUP_DEFERRED){try{task.cancel();}catch(_){}}
  POPUP_DEFERRED.clear();
  return POPUP_RENDER_TOKEN;
}
function popupIsCurrent(token){
  return token===POPUP_RENDER_TOKEN&&!!((document.getElementById("scrim")||{}).classList?.contains("show"));
}
// An open popup can hold data that a background refresh replaces. The popup
// registers one refresher for the token it opened with; a calendar commit calls
// popupNotifyDataCommit() and the popup rebuilds from current data instead of
// showing events that no longer match the grid and agenda behind it. The
// registration dies with the token: popupInvalidateWork() clears it on close and
// on any newer popup, so a stale refresher can never run.
let POPUP_LIVE_REFRESH=null;
function popupRegisterLiveRefresh(token,refresh){
  if(!popupIsCurrent(token))return false;
  POPUP_LIVE_REFRESH={token,refresh};
  return true;
}
function popupNotifyDataCommit(){
  const live=POPUP_LIVE_REFRESH;
  if(!live)return false;
  if(!popupIsCurrent(live.token)){POPUP_LIVE_REFRESH=null;return false;}
  try{
    return live.refresh()===true;
  }catch(err){
    // A refresh must never take down the popup that is already on screen.
    console.warn("popup live refresh failed",err);
    return false;
  }
}
function popupDefer(token,work){
  const cancelers=[];
  const task={
    cancelled:false,
    cancel(){
      if(this.cancelled)return;
      this.cancelled=true;
      for(const fn of cancelers.splice(0)){try{fn();}catch(_){}}
    }
  };
  POPUP_DEFERRED.add(task);
  popupNextFrame(()=>{
    if(task.cancelled||!popupIsCurrent(token)){POPUP_DEFERRED.delete(task);return;}
    try{
      work({
        isCurrent:()=>!task.cancelled&&popupIsCurrent(token),
        onCancel:fn=>{if(typeof fn==="function")cancelers.push(fn);}
      });
    }catch(_){/* a deferred visual must never break the popup shell */}
  });
  return task;
}
function popupReplaceWhen(content){
  const when=$("#popwhen");if(!when)return;
  const value=typeof content==="function"?content():content;
  if(value==null){when.replaceChildren();return;}
  if(typeof value==="string")when.textContent=value;
  else when.replaceChildren(value);
}
function popupLoadingBody(text){
  const skeleton=el("div","popup-skeleton");
  skeleton.setAttribute("role","status");skeleton.textContent=text||"Loading…";
  return skeleton;
}
// Paint header + scrim before any heavy body construction. Builders may return a
// node or fragment; stale builders are ignored after a close or newer popup.
function popupOpenTransaction(opts,build){
  popupTimingBegin();
  const token=popupInvalidateWork();
  opts=opts||{};
  if(typeof setPopupMode==="function")setPopupMode(opts.mode||"");
  const title=$("#poptitle");if(title)title.textContent=opts.title||"";
  popupReplaceWhen(opts.when);
  const body=$("#popbody");if(body)body.replaceChildren(popupLoadingBody(opts.loading));
  openScrim();
  popupTimingMark("shell");
  popupNextFrame(()=>{
    if(!popupIsCurrent(token)||!body)return;
    try{
      const content=build&&build(token);
      if(!popupIsCurrent(token)||content==null)return;
      body.replaceChildren(content);
      popupTimingMark("content");
      if(typeof opts.afterCommit==="function")opts.afterCommit(token,body);
    }catch(err){
      if(!popupIsCurrent(token))return;
      body.replaceChildren(el("div","popup-error","Unable to open this item."));
      console.warn("popup render failed",err);
    }
  });
  return token;
}
function openScrim(){ $("#scrim").classList.add("show");pauseUiAnimations();armOverlayAutoClose(); }
function closeScrim(){
  popupInvalidateWork();
  $("#scrim").classList.remove("show");
  if(typeof releaseMessagePopupRotationPause==="function")releaseMessagePopupRotationPause();
  if(!overlayIsOpen())disarmOverlayAutoClose();
  resumeUiAfterOverlay();
}
bindTap($("#popclose"),closeScrim);
const _scrimEl=$("#scrim");
if(_scrimEl)_scrimEl.addEventListener("click",e=>{if(e.target.id==="scrim")closeScrim();});
