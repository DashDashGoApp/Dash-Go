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
  pop.classList.remove("daytimelinepop","weatherpop","messagepop","eventpop","healthwarningpop");
  if(cls)pop.classList.add(cls);
}
function popupNextFrame(fn){
  if(typeof requestAnimationFrame==="function")requestAnimationFrame(fn);
  else setTimeout(fn,0);
}
function popupInvalidateWork(){
  POPUP_RENDER_TOKEN++;
  for(const task of POPUP_DEFERRED){try{task.cancel();}catch(_){}}
  POPUP_DEFERRED.clear();
  return POPUP_RENDER_TOKEN;
}
function popupIsCurrent(token){
  return token===POPUP_RENDER_TOKEN&&!!((document.getElementById("scrim")||{}).classList?.contains("show"));
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
  const token=popupInvalidateWork();
  opts=opts||{};
  if(typeof setPopupMode==="function")setPopupMode(opts.mode||"");
  const title=$("#poptitle");if(title)title.textContent=opts.title||"";
  popupReplaceWhen(opts.when);
  const body=$("#popbody");if(body)body.replaceChildren(popupLoadingBody(opts.loading));
  openScrim();
  popupNextFrame(()=>{
    if(!popupIsCurrent(token)||!body)return;
    try{
      const content=build&&build(token);
      if(!popupIsCurrent(token)||content==null)return;
      body.replaceChildren(content);
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
