// 11a-household-apps.js — local lazy Family Board and Maintenance app assets.
// This stays separate from the App Launcher shell so household app lifecycle
// code cannot turn the registry into a monolith.
function appIconFamilyBoard(){
  const svg=appIconFrame();
  svg.append(appSvgElement("rect",{x:"12",y:"13",width:"40",height:"36",rx:"5"}),appSvgElement("path",{d:"M20 23h24M20 32h24M20 41h15"}),appSvgElement("path",{d:"M17 18v8M47 18v8"}),appSvgElement("circle",{cx:"48",cy:"15",r:"4",fill:"currentColor",stroke:"none"}));
  return svg;
}
function appIconMaintenance(){
  // Selected Maintenance mark: Calendar + cog. Keep the shared single-color
  // launcher style so it matches the rest of Dash-Go's line icons while giving
  // Maintenance a clearer, calmer identity than the previous tool mashups.
  const svg=appIconFrame();
  const gear=appSvgElement("g",{transform:"translate(15 16) scale(0.55)"});
  gear.append(
    appSvgElement("path",{d:"M43.10 35.39 L43.50 30.20 L48.50 30.20 L48.90 35.39 L51.45 36.45 L55.40 33.06 L58.94 36.60 L55.55 40.55 L56.61 43.10 L61.80 43.50 L61.80 48.50 L56.61 48.90 L55.55 51.45 L58.94 55.40 L55.40 58.94 L51.45 55.55 L48.90 56.61 L48.50 61.80 L43.50 61.80 L43.10 56.61 L40.55 55.55 L36.60 58.94 L33.06 55.40 L36.45 51.45 L35.39 48.90 L30.20 48.50 L30.20 43.50 L35.39 43.10 L36.45 40.55 L33.06 36.60 L36.60 33.06 L40.55 36.45 Z"}),
    appSvgElement("circle",{cx:"46",cy:"46",r:"4"})
  );
  svg.append(
    appSvgElement("rect",{x:"11",y:"16",width:"38",height:"34",rx:"4"}),
    appSvgElement("path",{d:"M11 26h38"}),
    appSvgElement("path",{d:"M21 12v8"}),
    appSvgElement("path",{d:"M39 12v8"}),
    gear
  );
  return svg;
}
function appIconChoreWheel(){
  const svg=appIconFrame();
  svg.append(appSvgElement("circle",{cx:"32",cy:"32",r:"22"}),appSvgElement("path",{d:"M32 10v44M10 32h44M16.5 16.5l31 31M47.5 16.5l-31 31"}),appSvgElement("path",{d:"M32 5l-4 8h8z",fill:"currentColor",stroke:"none"}));
  return svg;
}
let _choreWheelLoading=null;
let _familyBoardLoading=null;
let _maintenanceLoading=null;
let _routinesLoading=null;
function appendLazyScript(src,dataName){
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
function appendLazyStyle(src,dataName){
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
function loadChoreWheelAssets(){
  if(window.openChoreWheelImpl) return Promise.resolve();
  if(_choreWheelLoading) return _choreWheelLoading;
  _choreWheelLoading=Promise.all([appendLazyStyle("ui/chore-wheel.css?v="+(CONFIG.version||"1.5.3-beta.1"),"chorewheel"),appendLazyScript("ui/chore-wheel-core.js?v="+(CONFIG.version||"1.5.3-beta.1"),"chorewheel-core")]).then(()=>appendLazyScript("ui/chore-wheel.js?v="+(CONFIG.version||"1.5.3-beta.1"),"chorewheel-script"));
  _choreWheelLoading.catch(()=>{_choreWheelLoading=null;});return _choreWheelLoading;
}
function openChoreWheel(){
  return loadChoreWheelAssets().then(()=>{
    if(typeof window.openChoreWheelImpl!=="function")throw new Error("Chore Wheel did not initialize");
    return window.openChoreWheelImpl();
  });
}
function loadFamilyBoardAssets(){
  if(window.openFamilyBoardImpl) return Promise.resolve();
  if(_familyBoardLoading) return _familyBoardLoading;
  _familyBoardLoading=Promise.all([appendLazyStyle("ui/family-board.css?v="+(CONFIG.version||"1.5.3-beta.1"),"familyboard"),appendLazyScript("ui/family-board-core.js?v="+(CONFIG.version||"1.5.3-beta.1"),"familyboard-core")]).then(()=>appendLazyScript("ui/family-board.js?v="+(CONFIG.version||"1.5.3-beta.1"),"familyboard-script"));
  _familyBoardLoading.catch(()=>{_familyBoardLoading=null;});return _familyBoardLoading;
}
function openFamilyBoard(){
  return loadFamilyBoardAssets().then(()=>{
    if(typeof window.openFamilyBoardImpl!=="function")throw new Error("Family Message Board did not initialize");
    return window.openFamilyBoardImpl();
  });
}
function loadMaintenanceAssets(){
  if(window.openMaintenanceImpl) return Promise.resolve();
  if(_maintenanceLoading) return _maintenanceLoading;
  _maintenanceLoading=Promise.all([appendLazyStyle("ui/maintenance.css?v="+(CONFIG.version||"1.5.3-beta.1"),"maintenance"),appendLazyScript("ui/maintenance-core.js?v="+(CONFIG.version||"1.5.3-beta.1"),"maintenance-core")]).then(()=>appendLazyScript("ui/maintenance.js?v="+(CONFIG.version||"1.5.3-beta.1"),"maintenance-script"));
  _maintenanceLoading.catch(()=>{_maintenanceLoading=null;});return _maintenanceLoading;
}
function openMaintenance(){
  return loadMaintenanceAssets().then(()=>{
    if(typeof window.openMaintenanceImpl!=="function")throw new Error("Maintenance did not initialize");
    return window.openMaintenanceImpl();
  });
}
window.openChoreWheel=openChoreWheel;
window.openFamilyBoard=openFamilyBoard;
window.openMaintenance=openMaintenance;

function loadRoutinesAssets(){
  if(window.openRoutinesImpl)return Promise.resolve();
  if(_routinesLoading)return _routinesLoading;
  _routinesLoading=Promise.all([appendLazyStyle("ui/routines.css?v="+(CONFIG.version||"1.5.3-beta.1"),"routines"),appendLazyScript("ui/routines-core.js?v="+(CONFIG.version||"1.5.3-beta.1"),"routines-core")]).then(()=>appendLazyScript("ui/routines.js?v="+(CONFIG.version||"1.5.3-beta.1"),"routines-script"));
  _routinesLoading.catch(()=>{_routinesLoading=null;});return _routinesLoading;
}
function openRoutines(options){return loadRoutinesAssets().then(()=>{if(typeof window.openRoutinesImpl!=="function")throw new Error("Routines did not initialize");return window.openRoutinesImpl(options||{});});}
window.openRoutines=openRoutines;
