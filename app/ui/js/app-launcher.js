// 11-app-launcher.js — static local Apps overlay and retained Chalkboard lazy loader.
// The registry is code-owned by the release. It is intentionally not a remote
// catalog, plugin system, or background app scheduler.
let _chalkboardLoading = null;
let APP_LAUNCHER_LAST_FOCUS = null;
let _listsLoading = null;
let TODO_STATUS = {source:"local",state:"local",syncMode:"local",map:{todo:"local-todo",grocery:"local-grocery"},lists:[{id:"local-todo",displayName:"To Do"},{id:"local-grocery",displayName:"Grocery"}]};
let TODO_STATUS_AT = 0;

function appSvgElement(name,attrs){
  const node=document.createElementNS("http://www.w3.org/2000/svg",name);
  for(const [key,value] of Object.entries(attrs||{})) node.setAttribute(key,String(value));
  return node;
}
function appIconFrame(){
  const svg=appSvgElement("svg",{viewBox:"0 0 64 64",focusable:"false","aria-hidden":"true"});
  svg.setAttribute("fill","none");svg.setAttribute("stroke","currentColor");svg.setAttribute("stroke-width","4");svg.setAttribute("stroke-linecap","round");svg.setAttribute("stroke-linejoin","round");
  return svg;
}
function appIconChalkboard(){
  const svg=appIconFrame();
  svg.append(
    appSvgElement("rect",{x:"10",y:"13",width:"44",height:"36",rx:"4"}),
    appSvgElement("path",{d:"M18 38c5-10 10 5 16-7 4-8 8-4 12-10"}),
    appSvgElement("path",{d:"M46 21v6"}),
    appSvgElement("path",{d:"M43 24h6"}),
    appSvgElement("path",{d:"M44 22l4 4"}),
    appSvgElement("path",{d:"M48 22l-4 4"})
  );
  return svg;
}
function appIconRadar(){
  // A spacious radar-pin mark: open sweep arcs frame a map pin instead of an
  // eye-like concentric center. The expanded viewBox leaves stroke clearance
  // on every side so the launcher never clips the outer radar sweep.
  const svg=appIconFrame();
  svg.setAttribute("viewBox","0 0 72 72");
  svg.setAttribute("stroke-width","4.5");
  svg.append(
    appSvgElement("path",{d:"M18 58A28 28 0 1 1 54 58"}),
    appSvgElement("path",{d:"M23 54A21 21 0 1 1 49 54"}),
    appSvgElement("path",{d:"M28 50A14 14 0 1 1 44 50"}),
    appSvgElement("path",{d:"M36 24c-6.1 0-11 4.8-11 10.9 0 8.2 11 21.1 11 21.1s11-12.9 11-21.1C47 28.8 42.1 24 36 24z"}),
    appSvgElement("circle",{cx:"36",cy:"34.5",r:"3.5",fill:"currentColor",stroke:"none"})
  );
  return svg;
}
function appIconCheckGrid(){
  // Selected To Do mark: a calm three-row task grid. The final open box keeps
  // the mark readable as an active list rather than a completed-only badge.
  const svg=appIconFrame();
  svg.append(
    appSvgElement("rect",{x:"11",y:"10",width:"42",height:"44",rx:"8"}),
    appSvgElement("rect",{x:"18",y:"18",width:"7",height:"7",rx:"1.6"}),
    appSvgElement("path",{d:"M19.4 21.5l1.7 1.8 3.2-3.6"}),
    appSvgElement("path",{d:"M31 21.5h14"}),
    appSvgElement("rect",{x:"18",y:"30",width:"7",height:"7",rx:"1.6"}),
    appSvgElement("path",{d:"M19.4 33.5l1.7 1.8 3.2-3.6"}),
    appSvgElement("path",{d:"M31 33.5h14"}),
    appSvgElement("rect",{x:"18",y:"42",width:"7",height:"7",rx:"1.6"}),
    appSvgElement("path",{d:"M31 45.5h10"})
  );
  return svg;
}
function appIconCart(){
  const svg=appIconFrame();
  svg.append(
    appSvgElement("path",{d:"M14 17h6l4 25h23l5-17H24"}),
    appSvgElement("path",{d:"M27 30h20"}),
    appSvgElement("circle",{cx:"30",cy:"50",r:"3",fill:"currentColor",stroke:"none"}),
    appSvgElement("circle",{cx:"45",cy:"50",r:"3",fill:"currentColor",stroke:"none"}),
    appSvgElement("path",{d:"M31 16l4 5 8-10"})
  );
  return svg;
}
function appIconRepeatCheck(){
  // Selected Routines mark: two repeat arrows wrap a completed step, keeping
  // cadence and completion visible in one compact, high-contrast silhouette.
  const svg=appIconFrame();
  svg.append(
    appSvgElement("path",{d:"M20 21a17 17 0 0 1 26 2"}),
    appSvgElement("path",{d:"M46 15v8h-8"}),
    appSvgElement("path",{d:"M44 43a17 17 0 0 1-26-2"}),
    appSvgElement("path",{d:"M18 49v-8h8"}),
    appSvgElement("circle",{cx:"32",cy:"32",r:"10"}),
    appSvgElement("path",{d:"M27.5 32l3.1 3.2 6.4-7"})
  );
  return svg;
}
function appIconControl(){
  const svg=appIconFrame();
  svg.append(
    appSvgElement("path",{d:"M14 18h36M14 32h36M14 46h36"}),
    appSvgElement("circle",{cx:"25",cy:"18",r:"5",fill:"var(--panel)"}),
    appSvgElement("circle",{cx:"40",cy:"32",r:"5",fill:"var(--panel)"}),
    appSvgElement("circle",{cx:"30",cy:"46",r:"5",fill:"var(--panel)"})
  );
  return svg;
}

async function refreshTodoStatus(force){
  if(!force && Date.now()-TODO_STATUS_AT<15000) return TODO_STATUS;
  try{ const r=await fetch("/api/todo/status",{cache:"no-store"}); const next=await r.json(); if(next&&typeof next==="object")TODO_STATUS=next; TODO_STATUS_AT=Date.now(); }
  catch(_){ /* Keep local-first defaults available while loopback starts. */ }
  return TODO_STATUS;
}

const DASHBOARD_APPS=Object.freeze([
  Object.freeze({
    id:"chalkboard",
    order:10,
    title:"Chalkboard",
    description:"Draw notes and quick sketches.",
    ariaLabel:"Open Chalkboard",
    icon:appIconChalkboard,
    available:()=>true,
    launch:()=>openChalkboard(),
  }),
  Object.freeze({
    id:"radar",
    order:20,
    title:"Weather Radar",
    description:"View local radar and recent motion.",
    ariaLabel:"Open Weather Radar",
    icon:appIconRadar,
    available:()=>true,
    launch:()=>openRadar(),
  }),
  Object.freeze({
    id:"todo",
    order:30,
    title:"To Do",
    description:"Your device-local task list.",
    ariaLabel:"Open To Do",
    icon:appIconCheckGrid,
    available:()=>true,
    launch:()=>openListsApp("todo"),
  }),
  Object.freeze({
    id:"family-board",
    order:45,
    title:"Family Message Board",
    description:"Short household notes and reminders.",
    ariaLabel:"Open Family Message Board",
    icon:()=>appIconFamilyBoard(),
    available:()=>true,
    launch:()=>openFamilyBoard(),
  }),
  Object.freeze({
    id:"chore-wheel",
    order:50,
    title:"Chore Wheel",
    description:"Fair, calendar-backed chore rotations.",
    ariaLabel:"Open Chore Wheel",
    icon:()=>appIconChoreWheel(),
    available:()=>true,
    launch:()=>openChoreWheel(),
  }),
  Object.freeze({
    id:"grocery",
    order:40,
    title:"Grocery",
    description:"Your device-local shopping list.",
    ariaLabel:"Open Grocery",
    icon:appIconCart,
    available:()=>true,
    launch:()=>openListsApp("grocery"),
  }),
  Object.freeze({
    id:"maintenance",
    order:60,
    title:"Maintenance",
    description:"Recurring home and vehicle upkeep.",
    ariaLabel:"Open Maintenance",
    icon:()=>appIconMaintenance(),
    available:()=>true,
    launch:()=>openMaintenance(),
  }),
  Object.freeze({
    id:"routines",
    order:70,
    title:"Routines",
    description:"Person-centered scheduled checklists.",
    ariaLabel:"Open Routines",
    icon:appIconRepeatCheck,
    available:()=>true,
    launch:()=>openRoutines(),
  }),
  Object.freeze({
    id:"dashboard-control",
    order:80,
    title:"Dashboard Control",
    description:"Display, calendar, system, and household settings.",
    ariaLabel:"Open Dashboard Control",
    icon:appIconControl,
    available:()=>true,
    launch:()=>openDashboardControlFromLauncher(),
  }),
]);

function appLauncherIsOpen(){
  return !!((document.getElementById("applauncher")||{}).classList?.contains("show"));
}
function availableDashboardApps(){ return DASHBOARD_APPS.slice().sort((a,b)=>a.order-b.order); }
function updateAppLauncherTrigger(){
  const trigger=document.getElementById("cblaunch"),footer=document.getElementById("compliment");
  if(trigger){
    trigger.hidden=false;
    trigger.style.display="inline-flex";
    trigger.setAttribute("aria-hidden","false");
    trigger.setAttribute("aria-label","Open apps");
  }
  if(footer) footer.classList.add("has-app-launcher");
}

function appendLauncherStyle(src,dataName){
  const selector=`link[data-${dataName}="1"]`;
  const prior=document.querySelector(selector);
  if(prior&&prior.dataset.ready==="1")return Promise.resolve();
  if(prior&&prior.dataset.failed==="1")prior.remove();
  return new Promise((resolve,reject)=>{
    const link=document.querySelector(selector)||document.createElement("link");
    const ready=()=>{link.dataset.ready="1";delete link.dataset.failed;resolve();};
    const fail=()=>{link.dataset.failed="1";link.remove();reject(new Error("app stylesheet failed to load"));};
    if(!link.getAttribute(`data-${dataName}`)){link.rel="stylesheet";link.href=src;link.setAttribute(`data-${dataName}`,"1");document.head.appendChild(link);}
    if(link.sheet){ready();return;}
    link.addEventListener("load",ready,{once:true});
    link.addEventListener("error",fail,{once:true});
  });
}
function appendLauncherScript(src,dataName){
  const selector=`script[data-${dataName}="1"]`;
  const prior=document.querySelector(selector);
  if(prior&&prior.dataset.ready==="1")return Promise.resolve();
  if(prior&&prior.dataset.failed==="1")prior.remove();
  return new Promise((resolve,reject)=>{
    const script=document.querySelector(selector)||document.createElement("script");
    const fail=()=>{script.dataset.failed="1";script.remove();reject(new Error("app assets failed to load"));};
    if(!script.getAttribute(`data-${dataName}`)){script.src=src;script.setAttribute(`data-${dataName}`,"1");document.body.appendChild(script);}
    script.addEventListener("load",()=>{script.dataset.ready="1";delete script.dataset.failed;resolve();},{once:true});
    script.addEventListener("error",fail,{once:true});
  });
}
function loadChalkboardAssets(){
  if(window.openChalkboardImpl) return Promise.resolve();
  if(_chalkboardLoading) return _chalkboardLoading;
  const version=CONFIG.version||"1.5.2-beta.3";
  _chalkboardLoading=appendLauncherStyle("ui/chalkboard.css?v="+version,"chalkboard").then(()=>appendLauncherScript("ui/chalkboard.js?v="+version,"chalkboard-script"));
  _chalkboardLoading.catch(()=>{_chalkboardLoading=null;});return _chalkboardLoading;
}
function openChalkboard(){
  return loadChalkboardAssets().then(()=>{
    if(typeof window.openChalkboardImpl!=="function") throw new Error("chalkboard did not initialize");
    return window.openChalkboardImpl();
  });
}
function appendListsScript(src,dataName){
  const attr="data-"+dataName,selector=`script[${attr}="1"]`,prior=document.querySelector(selector);
  if(prior&&prior.dataset.ready==="1")return Promise.resolve();
  if(prior&&prior.dataset.failed==="1")prior.remove();
  return new Promise((resolve,reject)=>{
    const script=document.querySelector(selector)||document.createElement("script");
    const fail=()=>{script.dataset.failed="1";script.remove();reject(new Error("Lists app assets failed to load"));};
    if(!script.getAttribute(attr)){script.src=src;script.setAttribute(attr,"1");document.body.appendChild(script);}
    script.addEventListener("load",()=>{script.dataset.ready="1";delete script.dataset.failed;resolve();},{once:true});
    script.addEventListener("error",fail,{once:true});
  });
}
function loadListsAssets(){
  if(window.openListsImpl&&window.DashGoGroceryQuickAdd&&window.DashGoListsPeople) return Promise.resolve();
  if(_listsLoading) return _listsLoading;
  const version=CONFIG.version||"1.5.2-beta.3";
  _listsLoading=appendLauncherStyle("ui/lists.css?v="+version,"listsapp").then(()=>appendListsScript("ui/lists-core.js?v="+version,"listsapp-core-script")).then(()=>appendListsScript("ui/lists-actions.js?v="+version,"listsapp-actions-script")).then(()=>appendListsScript("ui/lists-people.js?v="+version,"lists-people-script")).then(()=>appendListsScript("ui/lists-grocery.js?v="+version,"lists-grocery-script"));
  _listsLoading.catch(()=>{_listsLoading=null;});return _listsLoading;
}
function openListsApp(slot){
  return loadListsAssets().then(()=>{
    if(typeof window.openListsImpl!=="function") throw new Error("Lists app did not initialize");
    return window.openListsImpl(slot);
  });
}
function openListsAppWithAdd(slot){
  return loadListsAssets().then(()=>{
    if(typeof window.openListsForAdd!=="function") throw new Error("Lists add prompt did not initialize");
    return window.openListsForAdd(slot);
  });
}
window.openListsApp=openListsApp;
window.openListsAppWithAdd=openListsAppWithAdd;

async function openDashboardControlFromLauncher(){
  const trigger=document.getElementById("cblaunch");
  window.DASH_CONTROL_RETURN_FOCUS=trigger&&!trigger.hidden?trigger:document.activeElement;
  if(typeof lazyOpenCtrl!=="function") throw new Error("Dashboard Control is unavailable");
  return lazyOpenCtrl();
}
window.openDashboardControl=openDashboardControlFromLauncher;
function setAppLauncherStatus(text){
  const status=document.getElementById("applauncherstatus");
  if(status) status.textContent=String(text||"");
}
function resetAppLauncherShell(){
  const root=document.getElementById("applauncher");
  if(!root) return;
  root.setAttribute("aria-busy","false");
  root.querySelectorAll(".app-launcher-tile,.app-launcher-close").forEach(control=>{
    control.disabled=false;
    control.removeAttribute("aria-disabled");
  });
}
function appLauncherTile(app){
  const button=document.createElement("button");
  button.type="button";
  button.className="app-launcher-tile";
  button.dataset.appId=app.id;
  button.setAttribute("aria-label",app.ariaLabel);

  const icon=document.createElement("span");
  icon.className="app-launcher-icon";
  icon.setAttribute("aria-hidden","true");
  icon.appendChild(app.icon());
  const title=document.createElement("span");
  title.className="app-launcher-tile-title";
  title.textContent=app.title;
  const detail=document.createElement("span");
  detail.className="app-launcher-tile-detail";
  detail.textContent=app.description;
  button.append(icon,title,detail);
  bindTap(button,()=>launchDashboardApp(app.id));
  return button;
}
function renderAppLauncherApps(apps){
  const grid=document.getElementById("applaunchergrid");
  if(!grid) return;
  grid.replaceChildren(...apps.map(appLauncherTile));
  setAppLauncherStatus("");
}
function openAppLauncher(){
  const root=document.getElementById("applauncher");
  if(!root || appLauncherIsOpen()) return;
  if(typeof overlayIsOpen==="function" && overlayIsOpen()) return;
  if(typeof mapFullIsOpen==="function" && mapFullIsOpen()) return;
  if(typeof chalkboardIsOpen==="function" && chalkboardIsOpen()) return;
  if(typeof radarIsOpen==="function" && radarIsOpen()) return;
  if(typeof choreWheelIsOpen==="function" && choreWheelIsOpen()) return;
  if(typeof familyBoardIsOpen==="function" && familyBoardIsOpen()) return;
  if(typeof maintenanceIsOpen==="function" && maintenanceIsOpen()) return;
  if(typeof routinesIsOpen==="function" && routinesIsOpen()) return;
  if(typeof listsAppIsOpen==="function" && listsAppIsOpen()) return;
  const apps=availableDashboardApps();
  APP_LAUNCHER_LAST_FOCUS=document.activeElement;
  renderAppLauncherApps(apps);
  resetAppLauncherShell();
  // The launcher opens from local defaults immediately. Refresh cache-only status
  // after it is visible; never make Dashboard landing wait for To Do state.
  if(typeof fetch==="function")refreshTodoStatus(true).then(()=>{
    if(appLauncherIsOpen())renderAppLauncherApps(availableDashboardApps());
  }).catch(()=>{});
  root.classList.add("show");
  root.setAttribute("aria-hidden","false");
  root.setAttribute("aria-busy","false");
  if(typeof pauseUiAnimations==="function") pauseUiAnimations();
  if(typeof armOverlayAutoClose==="function") armOverlayAutoClose();
  requestAnimationFrame(()=>{if(appLauncherIsOpen())root.querySelector(".app-launcher-tile")?.focus();});
}
function closeAppLauncher(){
  const root=document.getElementById("applauncher");
  if(!root) return;
  resetAppLauncherShell();
  root.classList.remove("show");
  root.setAttribute("aria-hidden","true");
  if(typeof overlayIsOpen==="function" && overlayIsOpen()){
    if(typeof armOverlayAutoClose==="function") armOverlayAutoClose();
  }else if(typeof disarmOverlayAutoClose==="function") disarmOverlayAutoClose();
  if(typeof resumeUiAfterOverlay==="function") resumeUiAfterOverlay();
  const trigger=document.getElementById("cblaunch");
  const focusTarget=trigger&&!trigger.hidden?trigger:APP_LAUNCHER_LAST_FOCUS;
  focusTarget?.focus?.();
}
function bindAppLauncherShell(){
  const root=document.getElementById("applauncher"),close=document.getElementById("applauncherclose");
  if(!root) return;
  if(close) bindTap(close,()=>closeAppLauncher());
  bindTap(root,event=>{if(event.target===root)closeAppLauncher();});
  document.addEventListener("keydown",event=>{
    if(event.key!=="Escape" || !appLauncherIsOpen()) return;
    event.preventDefault();
    closeAppLauncher();
  });
}
async function launchDashboardApp(id){
  const app=availableDashboardApps().find(candidate=>candidate.id===id);
  if(!app){renderAppLauncherApps(availableDashboardApps());setAppLauncherStatus("That app could not be opened. Try again.");return;}
  closeAppLauncher();
  try{await app.launch();}
  catch(error){openAppLauncher();setAppLauncherStatus("Unable to open "+app.title+". Try again.");console.warn("dashboard app launch failed",app.id,error);}
}
document.addEventListener("DOMContentLoaded",bindAppLauncherShell);
