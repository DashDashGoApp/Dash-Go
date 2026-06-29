// 09-control-13a-lifecycle.js — Dashboard Control overlay lifecycle.
// This stays separate from location/PIN rendering so opening and closing the
// Control shell can remain small, auditable, and independently bounded.

async function renderCtrlAll(){
  try{ await renderCtrlPage(ctrlActivePageName(),CTRL_PAGE_RENDER_SEQ); }
  catch(e){
    if(String(e.message).toLowerCase().includes("locked")){ showPinLock(); return; }
    ctrlMsg("Control panel unavailable: "+e.message+" (is dashboard-control-server installed?)");
  }
}
async function openCtrl(){
  if(typeof markCtrlOpenGesture==="function") markCtrlOpenGesture(900);
  CTRL_OPEN=true;
  if(typeof CTRL_LAST_OPEN_SECTION!=="undefined")CTRL_LAST_OPEN_SECTION.clear();
  CTRL_PAGE_RENDER_SEQ++;
  $("#ctrl").classList.add("show");
  $("#ctrl").classList.remove("ctrl-loading","ctrl-load-error");
  if(typeof hideCtrlLoadingShell==="function")hideCtrlLoadingShell();
  document.body.classList.toggle("ctrl-lite",ctrlLiteProfile());
  pauseUiAnimations();
  const overviewPage=document.querySelector("#ctrlpage-overview");
  if(overviewPage){
    setCtrlPageVisual("overview",overviewPage);
    if(typeof scrollRootState==="function")scrollRootState(overviewPage,"control-page");
  }
  collapseCtrlSections();
  cleanupInactiveCtrlPages("overview");
  armOverlayAutoClose();
  ctrlMsg("");
  const pin=$("#ctrlpin");
  if(pin) pin.classList.remove("show");
  const panel=$("#ctrlpanel"); if(panel) panel.classList.remove("pinlocked");
  setCtrlMainVisible(true);
  if(await controlLockEnabled()){
    // Trust the session token if we have one; the first API call will ask for
    // the PIN again if the server restarted or the token expired.
    if(!CTRL_TOKEN){ showPinLock(); return; }
  }
  await renderCtrlAll();
  if(typeof ctrlScheduleCacheBudgetProbe==="function")ctrlScheduleCacheBudgetProbe();
}
function closeCtrl(){
  if(typeof stopCtrlUpdatePoll==="function")stopCtrlUpdatePoll();
  if(typeof closePeopleInboxPINKeypad==="function")closePeopleInboxPINKeypad({restoreFocus:false});
  if(typeof hideOSK==="function")hideOSK();
  const active=typeof ctrlActivePageName==="function"?ctrlActivePageName():"";
  if(active&&typeof ctrlAbortPageRequests==="function")ctrlAbortPageRequests(active);
  CTRL_PAGE_RENDER_SEQ++;
  if(typeof ctrlCloseMemorySettle==="function") ctrlCloseMemorySettle();
  ctrlHideAllOutputConsoles();
  cleanupCtrlTemporaryContent();
  document.querySelectorAll(".ctrlpage").forEach(p=>cleanupCtrlPage(p,"close"));
  const overviewPage=document.querySelector("#ctrlpage-overview");
  if(overviewPage) setCtrlPageVisual("overview",overviewPage);
  $("#ctrl").classList.remove("show"); document.body.classList.remove("ctrl-lite"); if(typeof hideOSK==="function") hideOSK(); else _oskTarget=null;
  // Geometry previews remain visible while Control is open. Once it has closed,
  // commit exactly one Calendar rebuild/fit/home-anchor transaction.
  if(typeof calendarGeometryCommitAfterControlClose==="function")calendarGeometryCommitAfterControlClose();
  const pin=$("#ctrlpin"), panel=$("#ctrlpanel"); if(pin) pin.classList.remove("show"); if(panel) panel.classList.remove("pinlocked"); setCtrlMainVisible(true);
  CTRL_OPEN=false;
  if(typeof CTRL_LAST_OPEN_SECTION!=="undefined")CTRL_LAST_OPEN_SECTION.clear();
  if(CTRL_LOCK_STATUS && CTRL_LOCK_STATUS.timeout==="every_open" && CTRL_TOKEN){
    const tok=CTRL_TOKEN;
    CTRL_TOKEN=""; SAFE_SESSION.remove("dashboardControlToken");
    fetch("/api/lock/revoke",{method:"POST",headers:{"X-Dashboard-Token":tok,"Content-Type":"application/json"},body:"{}"}).catch(()=>{});
  }
  if(!overlayIsOpen()) disarmOverlayAutoClose();
  _lastTickMin=-1;   // minute may have rolled over while paused
  tickClock();       // repaint immediately rather than at the next tick
  const returnFocus=window.DASH_CONTROL_RETURN_FOCUS;
  const returnAction=window.DASH_CONTROL_RETURN_ACTION;
  window.DASH_CONTROL_RETURN_FOCUS=null;
  window.DASH_CONTROL_RETURN_ACTION=null;
  resumeUiAfterOverlay();
  if(typeof returnAction==="function"){
    requestAnimationFrame(()=>{ try{returnAction();}catch(_){ } });
  }else if(returnFocus&&returnFocus.isConnected){
    requestAnimationFrame(()=>returnFocus.focus?.());
  }
}
