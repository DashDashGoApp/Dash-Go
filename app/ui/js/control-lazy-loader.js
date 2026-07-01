// Lazy-load Dashboard Control implementation and its Control-only stylesheet on first open.
let CTRL_BUNDLE_PROMISE=null,CTRL_SCRIPT_PROMISE=null,CTRL_STYLES_PROMISE=null,CTRL_OPEN_GUARD_UNTIL=0,CTRL_WARM_TIMER=0,CTRL_LAZY_OPEN_TOKEN=0;
function markCtrlOpenGesture(ms){CTRL_OPEN_GUARD_UNTIL=Math.max(CTRL_OPEN_GUARD_UNTIL,Date.now()+(ms||850));}
function shouldIgnoreCtrlBackdropClose(){return Date.now()<CTRL_OPEN_GUARD_UNTIL;}
function controlAssetVersion(){return encodeURIComponent(CONFIG.version||"1.5.4");}
function controlAssetURL(path){return path+"?v="+controlAssetVersion();}
function controlLiteProfile(){return ["lite","zero2","low","low-power"].includes(String((CONFIG||{}).profile||"").toLowerCase());}
function ctrlShellStatus(text,retry){
  const node=document.getElementById("ctrlshellstatus");if(!node)return;
  node.replaceChildren(document.createTextNode(text||"Loading Dashboard Control…"));
  if(retry){const b=document.createElement("button");b.type="button";b.className="cbtn";b.textContent="Try again";b.addEventListener("click",()=>lazyOpenCtrl());node.appendChild(b);}
}
function showCtrlLoadingShell(text,error){
  const ctrl=document.getElementById("ctrl");if(!ctrl)return;
  ctrl.classList.add("show");ctrl.classList.toggle("ctrl-loading",!error);ctrl.classList.toggle("ctrl-load-error",!!error);ctrl.setAttribute("aria-busy",error?"false":"true");
  ctrlShellStatus(text||"Loading Dashboard Control…",!!error);
  if(typeof pauseUiAnimations==="function")pauseUiAnimations();if(typeof armOverlayAutoClose==="function")armOverlayAutoClose();
}
function hideCtrlLoadingShell(){
  const ctrl=document.getElementById("ctrl");if(ctrl){ctrl.classList.remove("ctrl-loading","ctrl-load-error");ctrl.setAttribute("aria-busy","false");}
  const node=document.getElementById("ctrlshellstatus");if(node)node.replaceChildren();
}
function ensureControlStyles(){
  if(CTRL_STYLES_PROMISE)return CTRL_STYLES_PROMISE;
  CTRL_STYLES_PROMISE=new Promise((resolve,reject)=>{const link=document.createElement("link");link.rel="stylesheet";link.href=controlAssetURL("ui/control-layout.css");link.dataset.controlStylesheet="1";link.onload=resolve;link.onerror=()=>{CTRL_STYLES_PROMISE=null;link.remove();reject(new Error("Dashboard Control styles failed to load"));};document.head.appendChild(link);});return CTRL_STYLES_PROMISE;
}
function ensureControlScript(){
  if(CTRL_SCRIPT_PROMISE)return CTRL_SCRIPT_PROMISE;
  CTRL_SCRIPT_PROMISE=new Promise((resolve,reject)=>{const s=document.createElement("script");s.src=controlAssetURL("ui/js/app.control.bundle.js");s.dataset.controlBundle="1";s.onload=resolve;s.onerror=()=>{CTRL_SCRIPT_PROMISE=null;s.remove();reject(new Error("Dashboard Control assets failed to load"));};document.body.appendChild(s);});return CTRL_SCRIPT_PROMISE;
}
function ensureControlBundle(){
  if(window.__dashboardControlLoaded)return Promise.resolve();if(CTRL_BUNDLE_PROMISE)return CTRL_BUNDLE_PROMISE;
  CTRL_BUNDLE_PROMISE=Promise.all([ensureControlStyles(),ensureControlScript()]).then(()=>{window.__dashboardControlLoaded=true;}).catch(err=>{CTRL_BUNDLE_PROMISE=null;throw err;});return CTRL_BUNDLE_PROMISE;
}
async function lazyOpenCtrl(){
  const token=++CTRL_LAZY_OPEN_TOKEN;
  markCtrlOpenGesture(900);showCtrlLoadingShell("Loading Dashboard Control…",false);
  try{
    await ensureControlBundle();
    if(token!==CTRL_LAZY_OPEN_TOKEN)return;
    markCtrlOpenGesture(900);hideCtrlLoadingShell();
    if(window.openCtrl&&window.openCtrl!==lazyOpenCtrl)return window.openCtrl();
  }catch(err){
    if(token!==CTRL_LAZY_OPEN_TOKEN)return;
    showCtrlLoadingShell("Dashboard Control could not load. Check the local server, then retry.",true);
    console.warn("Dashboard Control asset load failed",err);
  }
}

// Shared app-to-Control handoff for household People. Apps own their normal
// lifecycle; this helper opens the canonical editor and returns to the caller
// only after the Control overlay closes.
function openDashboardPeopleControl(options){
  const opts=options||{};
  const origin=opts.origin||document.activeElement||null;
  window.DASH_CONTROL_RETURN_FOCUS=origin;
  window.DASH_CONTROL_RETURN_ACTION=typeof opts.reopen==="function"?opts.reopen:null;
  window.DASH_CONTROL_PENDING_SECTION={page:"control",lazy:"people"};
  const open=()=>Promise.resolve(lazyOpenCtrl()).then(()=>{
    const route=()=>{
      if(typeof ctrlOpenPendingSection==="function"){
        if(ctrlOpenPendingSection())return;
        if(typeof CTRL_LOCK_STATUS!=="undefined"&&CTRL_LOCK_STATUS&&CTRL_LOCK_STATUS.enabled&&!CTRL_TOKEN)return;
      }
      setTimeout(route,20);
    };
    route();
  });
  if(typeof opts.close==="function"){
    try{ opts.close(); }catch(_){ }
    return new Promise(resolve=>setTimeout(resolve,45)).then(open);
  }
  return open();
}

function lazyCloseCtrl(){
  CTRL_LAZY_OPEN_TOKEN++;
  if(window.closeCtrl&&window.closeCtrl!==lazyCloseCtrl)return window.closeCtrl();
  const c=document.getElementById("ctrl");if(c){c.classList.remove("show","ctrl-loading","ctrl-load-error");c.setAttribute("aria-busy","false");}hideCtrlLoadingShell();
  if(typeof CTRL_OPEN!=="undefined")CTRL_OPEN=false;
  if(typeof overlayIsOpen==="function"&&typeof disarmOverlayAutoClose==="function"&&!overlayIsOpen())disarmOverlayAutoClose();
  if(typeof resumeUiAfterOverlay==="function")resumeUiAfterOverlay();
}
function controlWarmupBlocked(){
  const radar=document.getElementById("radarfull"),scrim=document.getElementById("scrim"),ctrl=document.getElementById("ctrl");
  return !!((radar&&radar.classList.contains("show"))||(scrim&&scrim.classList.contains("show"))||(ctrl&&ctrl.classList.contains("show"))||(typeof chalkboardFocusActive==="function"&&chalkboardFocusActive()));
}
function scheduleControlAssetWarmup(){
  clearTimeout(CTRL_WARM_TIMER);const delay=controlLiteProfile()?20000:8000;
  CTRL_WARM_TIMER=setTimeout(()=>{
    if(controlWarmupBlocked()){scheduleControlAssetWarmup();return;}
    const run=()=>{if(!controlWarmupBlocked())ensureControlBundle().catch(()=>{});};
    if(typeof requestIdleCallback==="function")requestIdleCallback(run,{timeout:3500});else setTimeout(run,0);
  },delay);
}
var openCtrl=lazyOpenCtrl,closeCtrl=lazyCloseCtrl;
document.addEventListener("keydown",e=>{if(e.ctrlKey&&e.altKey&&String(e.key||"").toLowerCase()==="t"){e.preventDefault();lazyOpenCtrl().then(()=>{if(!CTRL_LOCK_STATUS||!CTRL_LOCK_STATUS.enabled||CTRL_TOKEN){if(typeof openDashboardTerminal==="function")openDashboardTerminal();}else if(typeof ctrlMsg==="function")ctrlMsg("Unlock Dashboard Control, then tap Open terminal.");}).catch(()=>{});}});
