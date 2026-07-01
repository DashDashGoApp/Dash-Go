// 09-control.js — generated from dashboard.js for maintainability.
/* =====================================================================
   ====================  CONTROL OVERLAY  ==============================
   Triple-tap the moon phase to open. Talks to the
   loopback API in dashboard-control-server. All actions are local-only.
   ===================================================================== */
// Tap and triple-tap helpers live in 00-tap.js. Keep a tiny guard here so
// a stale index that somehow skipped 00-tap.js fails loudly instead of
// silently binding incomplete controls.
if(typeof bindTap!=="function" || typeof bindTripleTap!=="function"){
  throw new Error("Dashboard tap helpers failed to load before Dashboard Control");
}
// GET cache for the control overlay: render instantly from the last-known
// response, then refresh in the background. On a slow board the status call
// (wifi/temp/etc) takes a couple of seconds — without this the panel sat
// empty that whole time. The cache is also warmed once shortly after boot
// so even the FIRST open is instant.
const CTRL_CACHE={};
const SAFE_SESSION=(()=>{
  const mem={};
  return {
    get(k){ try{return sessionStorage.getItem(k)||"";}catch(_){return mem[k]||"";} },
    set(k,v){ mem[k]=String(v||""); try{sessionStorage.setItem(k,mem[k]);}catch(_){} },
    remove(k){ delete mem[k]; try{sessionStorage.removeItem(k);}catch(_){} }
  };
})();
let CTRL_TOKEN=SAFE_SESSION.get("dashboardControlToken");
let CTRL_LOCK_STATUS=null;
let CTRL_OPEN=false;
const CTRL_LAZY_LOADED={};
function ctrlActivePageName(){
  const p=document.querySelector(".ctrlpage.show");
  return p&&p.id?p.id.replace("ctrlpage-",""):"overview";
}
function ctrlLiteProfile(){ return ["lite","zero2","low","low-power"].includes(String(CONFIG.profile||"").toLowerCase()); }
let CTRL_PAGE_RENDER_SEQ=0;
function ctrlClearNode(node){
  if(!node) return;
  if(node.id==="ctrlprofile"&&typeof ctrlResetProfileEditor==="function")ctrlResetProfileEditor(node);
  if(node.tagName==="PRE"){ ctrlHideOutputConsole(node.id); return; }
  if(node.classList && node.classList.contains("ctrloutputconsole")){
    const pre=node.querySelector("pre");
    if(pre) ctrlHideOutputConsole(pre.id);
    else node.style.display="none";
    return;
  }
  node.innerHTML="";
  if(node.dataset){
    delete node.dataset.rendered;
    delete node.dataset.loaded;
  }
}
function ctrlOutputHost(target){
  if(!target) return null;
  if(typeof target==="string") return $(target.charAt(0)==="#"?target:"#"+target);
  return target.nodeType===1?target:null;
}
function ensureCtrlOutputConsole(id,label,target){
  let pre=$("#"+id);
  // Some output consoles are intentionally moved into lazily-rendered cards.
  // If a card re-render removed the old node, recreate the hidden <pre> so the
  // next health/log run still has a stable output target.
  if(!pre){
    pre=document.createElement("pre");
    pre.id=id;
    pre.dataset.scrollPolicy="console";
    pre.style.display="none";
    const panel=$("#ctrlpanel") || document.body;
    panel.appendChild(pre);
  }
  let wrap=pre.parentElement && pre.parentElement.classList && pre.parentElement.classList.contains("ctrloutputconsole") ? pre.parentElement : null;
  if(!wrap){
    wrap=el("div","ctrloutputconsole");
    wrap.id=id+"wrap";
    wrap.style.display="none";
    const head=el("div","ctrloutputhead");
    head.appendChild(el("div","ctrloutputtitle",label||"Output"));
    const close=cbtn("× Close","ctrloutputclose",()=>ctrlHideOutputConsole(id));
    head.appendChild(close);
    pre.parentNode.insertBefore(wrap,pre);
    wrap.appendChild(head);
    wrap.appendChild(pre);
  }else{
    const title=wrap.querySelector(".ctrloutputtitle");
    if(title && label) title.textContent=label;
  }
  const host=ctrlOutputHost(target);
  if(host && wrap.parentElement!==host) host.appendChild(wrap);
  if(typeof scrollRootState==="function")scrollRootState(pre,"console");
  return {wrap,pre};
}
function ctrlShowOutputConsole(id,label,text,target){
  const c=ensureCtrlOutputConsole(id,label,target);
  if(!c) return;
  document.querySelectorAll(".ctrlsecbody.hasoutputconsole").forEach(n=>n.classList.remove("hasoutputconsole"));
  c.wrap.style.display="block";
  c.pre.style.display="block";
  c.pre.textContent=text||"";
  if(c.wrap.parentElement && c.wrap.parentElement.classList) c.wrap.parentElement.classList.add("hasoutputconsole");
}
function ctrlHideOutputConsole(id){
  if(!id) return;
  const pre=$("#"+id);
  if(!pre) return;
  pre.textContent="";
  pre.style.display="none";
  const wrap=pre.parentElement;
  if(wrap && wrap.classList && wrap.classList.contains("ctrloutputconsole")){
    if(wrap.parentElement && wrap.parentElement.classList) wrap.parentElement.classList.remove("hasoutputconsole");
    wrap.style.display="none";
  }
}
function ctrlHideAllOutputConsoles(){
  ["ctrldoctor","ctrlupdatelog","ctrlsystemupdatelog","ctrlmemory"].forEach(ctrlHideOutputConsole);
  document.querySelectorAll(".ctrlsecbody.hasoutputconsole").forEach(n=>n.classList.remove("hasoutputconsole"));
}
function ctrlEvictPage(page,reason){
  if(!page)return;
  page.querySelectorAll("details.ctrlsec").forEach(d=>{d.open=false;if(d.dataset&&d.dataset.lazy)d.dataset.loaded="0";});
  page.querySelectorAll(".ctrlsecbody [id]").forEach(ctrlClearNode);
  if(reason!=="tab"){const doctor=$("#ctrldoctor");if(doctor)ctrlClearNode(doctor);}
}
function cleanupCtrlPage(page,reason){
  if(!page)return;
  if(reason==="tab"){
    if(typeof ctrlClosePageSectionsForSession==="function")ctrlClosePageSectionsForSession(page,true);
    else page.querySelectorAll("details.ctrlsec, details.ctrlbackupcard").forEach(d=>d.open=false);
    if(typeof ctrlRememberHibernatedPage==="function")ctrlRememberHibernatedPage(page);
    return;
  }
  ctrlEvictPage(page,reason);
}
function cleanupInactiveCtrlPages(activeName){
  if(!ctrlLiteProfile()) return;
  // A new Control session starts with no history. Do not seed the adaptive LRU
  // with pages merely because they exist in markup; only pages actually left
  // during this session become candidates for short-lived retention.
  document.querySelectorAll(".ctrlpage").forEach(p=>{
    const name=p.id?p.id.replace("ctrlpage-",""):"";
    if(name && name!==activeName) ctrlEvictPage(p,"open");
  });
}
function cleanupCtrlTemporaryContent(){
  ctrlHideAllOutputConsoles();
  if(!ctrlLiteProfile()) return;
  ["ctrlhistory","ctrldiag","ctrlmapcache","ctrlcalhealth","ctrlcomp","ctrlterminal","ctrlupdate","ctrlsystemupdate"].forEach(id=>ctrlClearNode($("#"+id)));
}
