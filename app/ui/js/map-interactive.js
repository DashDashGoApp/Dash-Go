// 05-popups-01-interactive-map.js — split from 05-popups-maps.js.
/* Full-screen interactive map. It is intentionally optional because it embeds
   Google Maps in an iframe and keeps a large keyboard onscreen; higher-power
   installer profiles can enable it by default. */
const MAPFULL_IDLE_MS = 5*60000;
const MAP_MIN_ZOOM = 3;
const MAP_MAX_ZOOM = 20;
const MAP_ZOOM_PRESETS = [
  {label:"Wide",zoom:8,title:"Wide area view"},
  {label:"City",zoom:11,title:"City-level view"},
  {label:"Area",zoom:14,title:"Neighborhood / local area view"},
  {label:"Street",zoom:17,title:"Street-level view"},
  {label:"Close",zoom:19,title:"Close-in detail view"},
];
let MAPFULL_IDLE_TIMER=null, _mapSearchTarget=null;
let MAP_STATE={query:"",label:"",lat:null,lon:null,baseLat:null,baseLon:null,zoom:15,seq:0};
function mapFullIsOpen(){ return !!((document.getElementById("mapfull")||{}).classList?.contains("show")); }
function chalkboardIsOpen(){ return !!((document.getElementById("chalkboard")||{}).classList?.contains("show")); }
function uiOverlayActive(){ return overlayIsOpen() || mapFullIsOpen() || chalkboardIsOpen() || (typeof appLauncherHandoffActive==="function" && appLauncherHandoffActive()); }
let DASH_DEFERRED_WORK=new Map();
function chalkboardFocusActive(){
  return CONFIG.pauseWhileOpen!==false && chalkboardIsOpen();
}
function deferDashboardWork(key,fn){
  if(!chalkboardFocusActive()) return false;
  if(typeof fn==="function") DASH_DEFERRED_WORK.set(String(key||"work"),fn);
  return true;
}
function runOrDeferDashboardWork(key,fn){
  if(deferDashboardWork(key,fn)) return undefined;
  return (typeof fn==="function") ? fn() : undefined;
}
function flushDeferredDashboardWork(){
  if(chalkboardFocusActive() || !DASH_DEFERRED_WORK.size) return;
  const tasks=Array.from(DASH_DEFERRED_WORK.values());
  DASH_DEFERRED_WORK.clear();
  tasks.forEach((fn,i)=>setTimeout(()=>{
    try{ fn(); }catch(e){ console.warn("deferred dashboard work failed",e); }
  },i*80));
}
function pauseUiAnimations(){
  // Popup/control/map overlays cover the rotating compliment area on many
  // layouts. Stop compliment fades and clock/seconds repaint work while any
  // overlay is open so popups appear immediately and stay visually quiet.
  // The chalkboard is a full-screen drawing surface, so heavier background
  // dashboard refreshes use deferDashboardWork() and catch up after close.
  if(typeof pauseComplimentMotion === "function") pauseComplimentMotion();
}
function resumeUiAfterOverlay(){
  if(uiOverlayActive()) return;
  if(typeof _lastTickMin !== "undefined"){ _lastTickMin=-1; tickClock(); }
  flushDeferredDashboardWork();
}
function mapClamp(n,min,max){ return Math.max(min,Math.min(max,n)); }
function mapCoord(n){ return Number.isFinite(Number(n)); }
function mapEmbedUrl(q,z){
  // Use the older maps.google.com embed form. It is less feature-rich than
  // the modern www.google.com/maps URL, but it has been more reliable in
  // Surf/WebKitGTK kiosk sessions and honors the zoom parameter better.
  const params=new URLSearchParams({output:"embed", q:String(q||""), z:String(z||15), hl:"en", iwloc:"addr"});
  return "https://maps.google.com/maps?"+params.toString();
}
function mapEmbedStateUrl(){
  if(mapCoord(MAP_STATE.lat) && mapCoord(MAP_STATE.lon)){
    return mapEmbedUrl(Number(MAP_STATE.lat).toFixed(6)+","+Number(MAP_STATE.lon).toFixed(6),MAP_STATE.zoom);
  }
  return mapEmbedUrl(MAP_STATE.query,MAP_STATE.zoom);
}
function disarmMapFullAutoClose(){ if(MAPFULL_IDLE_TIMER){ clearTimeout(MAPFULL_IDLE_TIMER); MAPFULL_IDLE_TIMER=null; } }
function armMapFullAutoClose(){ disarmMapFullAutoClose(); if(mapFullIsOpen()) MAPFULL_IDLE_TIMER=setTimeout(closeInteractiveMap,MAPFULL_IDLE_MS); }
function noteMapFullInput(){ if(mapFullIsOpen()) armMapFullAutoClose(); }
function mapUpdateFrame(){
  const frame=$("#mapframe"), title=$("#maptitle");
  if(title) title.textContent=MAP_STATE.query ? "Google Maps" : "Map";
  if(frame && MAP_STATE.query) frame.src=mapEmbedStateUrl();
  if(typeof updateMapTools==="function") updateMapTools();
}
async function mapResolveQuery(query,seq){
  if(!query || query.length<3) return;
  try{
    const res=await fetch("/api/event-map?q="+encodeURIComponent(query),{cache:"no-store"});
    const m=await res.json().catch(()=>({}));
    if(seq!==MAP_STATE.seq || !res.ok || !m.ok || !mapCoord(m.lat) || !mapCoord(m.lon)) return;
    MAP_STATE.lat=Number(m.lat); MAP_STATE.lon=Number(m.lon);
    MAP_STATE.baseLat=MAP_STATE.lat; MAP_STATE.baseLon=MAP_STATE.lon;
    MAP_STATE.label=m.label||query;
    mapUpdateFrame();
  }catch(_){ /* Keep text-search embed if local geocode fails. */ }
}
function setInteractiveMapQuery(q){
  const input=$("#mapsearch"), frame=$("#mapframe"), title=$("#maptitle");
  const query=String(q||"").trim();
  MAP_STATE={query,label:query,lat:null,lon:null,baseLat:null,baseLon:null,zoom:15,seq:(MAP_STATE.seq||0)+1};
  if(input) input.value=query;
  if(title) title.textContent=query ? "Google Maps" : "Map";
  if(frame && query) frame.src=mapEmbedStateUrl();
  if(typeof updateMapTools==="function") updateMapTools();
  mapResolveQuery(query,MAP_STATE.seq);
}
function openInteractiveMap(q){
  if(!CONFIG.showInteractiveMaps || !q) return;
  pauseUiAnimations();
  disarmOverlayAutoClose();
  const m=$("#mapfull"); if(!m) return;
  m.classList.add("show"); m.setAttribute("aria-hidden","false");
  _mapOskLayer="letters";
  _mapOskShift=true;
  _mapOskCapsLock=false;
  _mapOskLastShiftTap=0;
  buildMapKeyboard();
  if(typeof buildMapTools==="function") buildMapTools();
  setInteractiveMapQuery(q);
  const input=$("#mapsearch"); if(input) _mapSearchTarget=input;
  armMapFullAutoClose();
}
function closeInteractiveMap(){
  const m=$("#mapfull"), frame=$("#mapframe");
  if(frame) frame.src="about:blank";
  if(m){ m.classList.remove("show"); m.setAttribute("aria-hidden","true"); }
  disarmMapFullAutoClose();
  _mapSearchTarget=null;
  if(overlayIsOpen()) armOverlayAutoClose();
  resumeUiAfterOverlay();
}
function runInteractiveMapSearch(){
  const q=($("#mapsearch")||{}).value||"";
  if(q.trim()) setInteractiveMapQuery(q);
  armMapFullAutoClose();
}
function mapKeyType(ch){
  const i=_mapSearchTarget || $("#mapsearch"); if(!i) return;
  if(ch==="\b") i.value=i.value.slice(0,-1);
  else if(ch==="clear") i.value="";
  else i.value+=ch;
  armMapFullAutoClose();
}
function mapZoomBy(delta){
  MAP_STATE.zoom=mapClamp((Number(MAP_STATE.zoom)||15)+delta,MAP_MIN_ZOOM,MAP_MAX_ZOOM);
  mapUpdateFrame(); armMapFullAutoClose();
}
function mapSetZoom(level){
  MAP_STATE.zoom=mapClamp(Number(level)||15,MAP_MIN_ZOOM,MAP_MAX_ZOOM);
  mapUpdateFrame(); armMapFullAutoClose();
}
function mapRecenter(){
  if(!mapCoord(MAP_STATE.baseLat) || !mapCoord(MAP_STATE.baseLon)) return;
  MAP_STATE.lat=MAP_STATE.baseLat; MAP_STATE.lon=MAP_STATE.baseLon;
  mapUpdateFrame(); armMapFullAutoClose();
}
bindTap($("#mapclose"),closeInteractiveMap);
bindTap($("#mapclose2"),closeInteractiveMap);
bindTap($("#mapgo"),runInteractiveMapSearch);
const _mapSearchEl=$("#mapsearch");
if(_mapSearchEl){
  _mapSearchEl.addEventListener("click",()=>{ _mapSearchTarget=_mapSearchEl; armMapFullAutoClose(); });
  _mapSearchEl.addEventListener("keydown",e=>{ if(e.key==="Enter") runInteractiveMapSearch(); });
}
