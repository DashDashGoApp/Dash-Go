// 06-radar-01-overlay.js — bounded, self-contained regional radar overlay.
const RADAR_IDLE_MS=5*60000;
const RADAR_TILE=256;
const RADAR_TILE_GRID=3; // Default full-mode regional window; independent from Lite viewport coverage.
const RADAR_TILE_CACHE_LIMIT=72;
const RADAR_BASE_TILE_CACHE_LIMIT=36;
const RADAR_PREFETCH_DELAY_MS=400;
const RADAR_PREFETCH_NEAREST_FRAMES=2; // all source frames remain selectable; tile prefetch stays bounded.
const RADAR_TRANSPARENT="data:image/gif;base64,R0lGODlhAQABAAD/ACwAAAAAAQABAAACADs=";
// Lite derives a bounded 3×3/4×4 tile plan from its measured viewport and uses
// a short, user-controlled RainViewer sequence while the overlay is open.
const RADAR_LITE_MIN_PX=384;
const RADAR_LITE_MAX_PX=768; // Visible canvas cap; Lite derives source tile coverage with overscan.
const RADAR_LITE_ZOOM=6;
const RADAR_LITE_REFRESH_MS=20000;
const RADAR_LITE_BASE_DIM="rgba(12,20,28,0.40)";
const RADAR_LITE_RENDER_PX=640; // fixed current Lite canvas balance; no Dashboard Control tuning.
const RADAR_LITE_FRAME_MS=650;
const RADAR_LITE_EDGE_OVERSCAN_PX=2; // small measured source margin; plan math still covers every viewport pixel.
let RADAR_IDLE_TIMER=null,RADAR_RESIZE_TIMER=null;
let RADAR_STATE={open:false,provider:RADAR_DEFAULT_PROVIDER,frames:[],frame:0,playing:false,timer:null,token:0,status:null,
  tileCache:new Map(),tileOrder:[],baseTileCache:new Map(),baseTileOrder:[],prefetchTimer:null,retryTimer:null,
  visibleLayer:-1,rendering:false,renderQueued:false,error:"",fallbackTried:false,fullFrameFailures:0,liteLastRefreshAt:0,liteRendering:false,
  liteScratch:null,liteBase:null,liteBaseCache:{},liteRequests:new Set(),liteFrames:[],liteFrameIdx:0,litePlaying:false,liteTimer:null,liteFrameMeta:[],liteFramesFetchedAt:0,liteZoom:RADAR_LITE_ZOOM,litePx:0,priorFocus:null,
  renderTier:null};
function radarIsOpen(){ return !!((document.getElementById("radarfull")||{}).classList?.contains("show")); }
function radarTimestamp(ts){
  const n=Number(ts||0); if(!n) return "Latest";
  try{return new Intl.DateTimeFormat(undefined,{hour:"numeric",minute:"2-digit"}).format(new Date(n*1000));}catch(_){return new Date(n*1000).toLocaleTimeString();}
}
function radarFrameLabel(frame){
  const ts=Number((frame||{}).time||0); if(!ts) return "Latest available";
  const ago=Math.max(0,Math.round((Date.now()/1000-ts)/60)), age=ago<1?"just now":ago+" min ago";
  return radarTimestamp(ts)+" · "+age;
}
function radarClamp(n,lo,hi){ return Math.max(lo,Math.min(hi,n)); }
function radarLatitude(){ const n=Number((CONFIG||{}).lat); return Number.isFinite(n)&&n>=-85&&n<=85?n:41.8781; }
function radarLongitude(){ const n=Number((CONFIG||{}).lon); return Number.isFinite(n)&&n>=-180&&n<=180?n:-87.6298; }
function radarTileXY(lat,lon,z){
  const n=2**z,x=Math.floor((Number(lon)+180)/360*n),r=radarClamp(Number(lat),-85,85)*Math.PI/180;
  const y=Math.floor((1-Math.asinh(Math.tan(r))/Math.PI)/2*n);
  return {x:radarClamp(x,0,n-1),y:radarClamp(y,0,n-1),n};
}
function radarCenterFraction(lat,lon,z){
  const n=2**z,r=radarClamp(Number(lat),-85,85)*Math.PI/180;
  const x=(radarClamp(Number(lon),-180,180)+180)/360*n,y=(1-Math.asinh(Math.tan(r))/Math.PI)/2*n;
  return {fx:x-Math.floor(x),fy:y-Math.floor(y)};
}
function radarTileSlots(lat,lon,z){
  const c=radarTileXY(lat,lon,z),half=Math.floor(RADAR_TILE_GRID/2),out=[];
  for(let row=-half;row<=half;row++) for(let col=-half;col<=half;col++) out.push({x:(c.x+col+c.n)%c.n,y:radarClamp(c.y+row,0,c.n-1),z,row:row+half,col:col+half});
  return out;
}
// Lite derives its grid from the actual world-pixel viewport. A 768px canvas
// can straddle four tiles after fractional centering; fixed 3×3 coverage is not safe.
function radarLiteTilePlan(px,lat,lon,z){
  const n=2**z,r=radarClamp(Number(lat),-85,85)*Math.PI/180;
  const wx=(radarClamp(Number(lon),-180,180)+180)/360*n,wy=(1-Math.asinh(Math.tan(r))/Math.PI)/2*n;
  // Explicit edge margin prevents fractional-center rounding/seam exposure without
  // permanently inflating the Pi's tile plan by a whole extra tile on each side.
  const edge=RADAR_LITE_EDGE_OVERSCAN_PX/RADAR_TILE,span=px/RADAR_TILE;
  const leftX=wx-span/2-edge,rightX=wx+span/2+edge,topY=wy-span/2-edge,bottomY=wy+span/2+edge;
  const startX=Math.floor(leftX),endX=Math.ceil(rightX),startY=Math.floor(topY),endY=Math.ceil(bottomY),slots=[];
  for(let y=startY;y<endY;y++)for(let x=startX;x<endX;x++)slots.push({x:(x+n)%n,y:radarClamp(y,0,n-1),z,row:y-startY,col:x-startX});
  return {slots,cols:endX-startX,rows:endY-startY,planned:slots.length,ox:(startX-wx)*RADAR_TILE+px/2,oy:(startY-wy)*RADAR_TILE+px/2,edgePx:RADAR_LITE_EDGE_OVERSCAN_PX};
}

function radarZoom(){ return radarProfileTier()==="enhanced"?7:6; }
function radarIsLite(){ return radarProfileTier()==="lite"; }
function radarSkipBase(){ return radarIsLite(); }
function radarPrefetchConcurrency(){
  if(radarIsLite())return 1;
  // Enhanced rendering can be selected on a Lite-base Pi, but tile work stays
  // intentionally below the normal enhanced fan-out.
  return radarProfileTier()==="enhanced"?4:3;
}
function radarCanAnimate(){ return !radarIsLite()&&radarMeta(RADAR_STATE.provider).animate&&RADAR_STATE.frames.length>1; }
function radarSetText(id,text){ const e=document.getElementById(id); if(e) e.textContent=text||""; }
function radarSetBusy(on){ const e=document.getElementById("radarbusy"); if(e) e.hidden=!on; }
function radarSetError(text){ RADAR_STATE.error=String(text||""); const e=document.getElementById("radarerror"); if(e){e.textContent=RADAR_STATE.error;e.hidden=!RADAR_STATE.error;} }
function radarClearTimer(){ if(RADAR_STATE.timer){clearInterval(RADAR_STATE.timer);RADAR_STATE.timer=null;} }
function radarClearRetry(){ if(RADAR_STATE.retryTimer){clearTimeout(RADAR_STATE.retryTimer);RADAR_STATE.retryTimer=null;} }
function disarmRadarAutoClose(){ if(RADAR_IDLE_TIMER){clearTimeout(RADAR_IDLE_TIMER);RADAR_IDLE_TIMER=null;} }
function armRadarAutoClose(){ disarmRadarAutoClose(); if(radarIsOpen()) RADAR_IDLE_TIMER=setTimeout(closeRadar,RADAR_IDLE_MS); }
function noteRadarInput(){ if(radarIsOpen()) armRadarAutoClose(); }
function radarFrameInterval(){ return radarProfileTier()==="enhanced"?350:500; }
function radarCacheState(base){ return base?{items:RADAR_STATE.baseTileCache,order:RADAR_STATE.baseTileOrder,limit:RADAR_BASE_TILE_CACHE_LIMIT}:{items:RADAR_STATE.tileCache,order:RADAR_STATE.tileOrder,limit:RADAR_TILE_CACHE_LIMIT}; }
function radarCachedURL(key,base){ return radarCacheState(!!base).items.get(key)||""; }
function radarCachePut(key,url,base){ const c=radarCacheState(!!base); if(!c.items.has(key)) c.order.push(key); c.items.set(key,url); while(c.order.length>c.limit){const old=c.order.shift();c.items.delete(old);} }
function radarImagePreload(key,url,base){
  if(!url||radarCachedURL(key,base)) return Promise.resolve(!!url);
  return new Promise(resolve=>{let done=false,img=new Image();const finish=ok=>{if(done)return;done=true;if(ok)radarCachePut(key,url,base);resolve(!!ok);};img.decoding="async";img.onload=()=>finish(true);img.onerror=()=>finish(false);img.src=url;if(img.complete)setTimeout(()=>finish(img.naturalWidth>0),0);});
}
function radarLoadImage(img,key,url,token,base){
  if(!img||!url||token!==RADAR_STATE.token) return Promise.resolve({ok:false,cancelled:true});
  const source=radarCachedURL(key,base)||url;
  if(img.dataset.radarKey===key&&img.dataset.radarURL===source&&img.complete) return Promise.resolve({ok:img.dataset.radarFailed!=="1"&&img.naturalWidth>0,cancelled:false});
  return new Promise(resolve=>{
    let settled=false; const finish=ok=>{if(settled)return;settled=true;if(token!==RADAR_STATE.token||img.dataset.radarKey!==key||img.dataset.radarURL!==source){resolve({ok:false,cancelled:true});return;}if(ok)radarCachePut(key,source,base);resolve({ok:!!ok,cancelled:false});};
    img.dataset.radarKey=key;img.dataset.radarURL=source;delete img.dataset.radarFailed;
    img.onload=()=>finish(true);
    img.onerror=()=>{img.onerror=null;img.dataset.radarFailed="1";img.src=RADAR_TRANSPARENT;finish(false);};
    img.src=source;if(img.complete)setTimeout(()=>finish(img.naturalWidth>0),0);
  });
}
async function radarPool(items,fn,limit){ let at=0;await Promise.all(Array.from({length:Math.max(1,limit||1)},async()=>{while(at<items.length){const i=at++;await fn(items[i]);}})); }
function radarBaseTileURL(s){ return "https://tile.openstreetmap.org/"+s.z+"/"+s.x+"/"+s.y+".png"; }
function radarNwsTileURL(s){
  const n=2**s.z,lon1=s.x/n*360-180,lon2=(s.x+1)/n*360-180,lat1=Math.atan(Math.sinh(Math.PI*(1-2*(s.y+1)/n)))*180/Math.PI,lat2=Math.atan(Math.sinh(Math.PI*(1-2*s.y/n)))*180/Math.PI;
  const q=new URLSearchParams({service:"WMS",version:"1.1.1",request:"GetMap",layers:"conus_bref_qcd",styles:"",format:"image/png",transparent:"TRUE",srs:"EPSG:4326",bbox:[lon1,lat1,lon2,lat2].join(","),width:String(RADAR_TILE),height:String(RADAR_TILE)});
  return "https://opengeo.ncep.noaa.gov/geoserver/conus/conus_bref_qcd/ows?"+q;
}
// RainViewer's free personal tier serves Universal Blue (/2/) through zoom 7.
// Keep those fixed unless a separately authenticated provider contract is added.
function radarRainViewerURL(f,s){ return String(f.host||"")+String(f.path||"")+"/256/"+s.z+"/"+s.x+"/"+s.y+"/2/1_1.png"; }
function radarProxyURL(p,f,s){ const q=new URLSearchParams({provider:String(p),z:String(s.z),x:String(s.x),y:String(s.y),t:String((f&&f.time)||"now")});return "/api/radar/tile?"+q; }
function radarCustomURL(s){ const raw=String(CONFIG.radarCustomTiles||CONFIG.radarCustomWms||"").trim();return /^https:\/\//i.test(raw)?raw.replace(/\{z\}/g,s.z).replace(/\{x\}/g,s.x).replace(/\{y\}/g,s.y):""; }
function radarTileURL(p,f,s){ const m=radarMeta(p);if(m.kind==="rainviewer")return radarRainViewerURL(f,s);if(m.kind==="nws")return radarNwsTileURL(s);if(m.kind==="custom")return radarCustomURL(s);return radarProxyURL(p,f,s); }
function radarStage(){ return document.getElementById("radarstage"); }
// Two frames let the newly shown overlay settle before its first measured tile grid.
function radarNextFrame(){ return new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))); }
function radarStageReady(){ const st=radarStage(); return !!(st&&st.clientWidth>=2&&st.clientHeight>=2); }
function radarGridSignature(kind,provider){ const st=radarStage(),p=kind==="overlay"?String(provider||RADAR_STATE.provider):"base";return [radarLatitude().toFixed(5),radarLongitude().toFixed(5),radarZoom(),st?st.clientWidth:0,st?st.clientHeight:0,p].join("|"); }
function radarPaintGrid(layer,slots,imgs){
  const st=radarStage();if(!st||st.clientWidth<2||st.clientHeight<2)return false;const W=st.clientWidth,H=st.clientHeight,tilePx=Math.ceil(Math.max(W,H)/(RADAR_TILE_GRID-1)),frac=radarCenterFraction(radarLatitude(),radarLongitude(),radarZoom()),cx=W/2,cy=H/2;
  slots.forEach((slot,i)=>{const img=imgs[i],dx=(slot.col-1-frac.fx)*tilePx,dy=(slot.row-1-frac.fy)*tilePx;img.style.width=img.style.height=tilePx+"px";img.style.left=Math.round(cx+dx)+"px";img.style.top=Math.round(cy+dy)+"px";img.style.objectFit="fill";});return true;
}
function radarEnsureGrid(layer,kind,provider){
  if(!layer||!radarStageReady())return {slots:[],imgs:[]};const sig=radarGridSignature(kind,provider),oldSlots=layer._slots||[],oldImgs=layer._imgs||[];
  if(layer.dataset.gridSig===sig&&oldSlots.length===RADAR_TILE_GRID*RADAR_TILE_GRID&&oldImgs.length===oldSlots.length)return {slots:oldSlots,imgs:oldImgs};
  layer.innerHTML="";const slots=radarTileSlots(radarLatitude(),radarLongitude(),radarZoom()),imgs=[];
  slots.forEach(slot=>{const img=document.createElement("img");img.className="radar-tile radar-"+kind;img.alt="";img.decoding="async";img.draggable=false;img.dataset.slot=slot.z+"/"+slot.x+"/"+slot.y;layer.appendChild(img);imgs.push(img);});
  if(!radarPaintGrid(layer,slots,imgs)){radarClearGrid(layer);return {slots:[],imgs:[]};}layer.dataset.gridSig=sig;layer._slots=slots;layer._imgs=imgs;return {slots,imgs};
}
function radarClearGrid(layer){if(!layer)return;layer.innerHTML="";delete layer._slots;delete layer._imgs;delete layer.dataset.gridSig;}
function radarEnsureFrameLayers(){const host=document.getElementById("radarframes");if(!host)return [];let ls=[...host.querySelectorAll(".radar-frame-layer")];if(ls.length===2)return ls;host.innerHTML="";ls=[];for(let i=0;i<2;i++){const l=document.createElement("div");l.className="radar-frame-layer";l.dataset.radarLayer=String(i);host.appendChild(l);ls.push(l);}return ls;}
function radarClearFrameLayers(){const host=document.getElementById("radarframes");if(host)host.innerHTML="";RADAR_STATE.visibleLayer=-1;}
function radarSetLayerVisible(ls,n,fade){ls.forEach((l,i)=>{l.classList.toggle("radar-layer-active",i===n);l.classList.toggle("radar-layer-no-fade",!fade);});}
function radarSetAttribution(meta){
  const scope=radarIsLite()?(RADAR_STATE.liteZoom===7?" · local snapshot":" · regional snapshot"):"";
  radarSetText("radarattribution","Base map © OpenStreetMap contributors · "+(meta?.label||"Radar")+scope);
}
async function radarLoadBase(token){
  const layer=document.getElementById("radarbase");if(!layer)return;if(radarSkipBase()){radarClearGrid(layer);return;}
  const g=radarEnsureGrid(layer,"base","base");if(!g.slots.length)return;await radarPool(g.slots.map((slot,i)=>({slot,img:g.imgs[i]})),async it=>{if(token!==RADAR_STATE.token)return;const k="base:"+it.slot.z+"/"+it.slot.x+"/"+it.slot.y;await radarLoadImage(it.img,k,radarBaseTileURL(it.slot),token,true);},radarPrefetchConcurrency());
}
function radarUpdateFrameChrome(frame,frames){
  radarSetText("radarstamp",radarFrameLabel(frame));const scrub=document.getElementById("radarscrub");if(scrub){scrub.max=String(Math.max(0,frames.length-1));scrub.value=String(RADAR_STATE.frame);scrub.disabled=frames.length<2;}const now=document.getElementById("radarnow");if(now)now.disabled=frames.length<2||RADAR_STATE.frame===frames.length-1;
}
function radarHandleFrameFailures(ok,total){
  if(ok>0){RADAR_STATE.fullFrameFailures=0;return false;}RADAR_STATE.fullFrameFailures++;const reason=radarMeta(RADAR_STATE.provider).label+" frame did not load.";
  if(RADAR_STATE.fullFrameFailures<2){radarSetError(reason+" Retrying once…");radarClearRetry();RADAR_STATE.retryTimer=setTimeout(()=>{RADAR_STATE.retryTimer=null;radarRenderFrame(RADAR_STATE.token);},650);return true;}
  if(RADAR_STATE.provider!=="rainviewer")setTimeout(()=>radarUseFallback(reason),0);else radarSetError(reason+" Check the network connection and try again.");return true;
}
async function radarRenderFrame(token){
  const wanted=token||RADAR_STATE.token;if(!radarIsOpen()||wanted!==RADAR_STATE.token)return false;if(RADAR_STATE.rendering){RADAR_STATE.renderQueued=true;return false;}RADAR_STATE.rendering=true;const requested=RADAR_STATE.frame;
  try{
    const frames=RADAR_STATE.frames,frame=frames[requested]||{};radarUpdateFrameChrome(frame,frames);if(!frames.length)return false;const ls=radarEnsureFrameLayers();if(ls.length!==2)return false;
    const next=RADAR_STATE.visibleLayer===0?1:0,layer=ls[next],g=radarEnsureGrid(layer,"overlay",RADAR_STATE.provider),results=[];
    if(!g.slots.length){RADAR_STATE.renderQueued=true;return false;}
    await radarPool(g.slots.map((slot,i)=>({slot,img:g.imgs[i]})),async it=>{const url=radarTileURL(RADAR_STATE.provider,frame,it.slot);if(!url){results.push({ok:false,cancelled:false});return;}const k=RADAR_STATE.provider+":"+(frame.time||"latest")+":"+it.slot.z+"/"+it.slot.x+"/"+it.slot.y;results.push(await radarLoadImage(it.img,k,url,wanted,false));},radarPrefetchConcurrency());
    if(wanted!==RADAR_STATE.token||!radarIsOpen()||requested!==RADAR_STATE.frame)return false;const painted=results.filter(r=>!r.cancelled),ok=painted.filter(r=>r.ok).length;
    if(radarHandleFrameFailures(ok,painted.length))return false;
    radarSetLayerVisible(ls,next,radarProfileTier()!=="lite"&&RADAR_STATE.visibleLayer>=0);RADAR_STATE.visibleLayer=next;return true;
  }finally{RADAR_STATE.rendering=false;if(RADAR_STATE.renderQueued&&radarIsOpen()){RADAR_STATE.renderQueued=false;setTimeout(()=>radarRenderFrame(RADAR_STATE.token),40);}}
}
function radarCancelPrefetch(){if(RADAR_STATE.prefetchTimer){clearTimeout(RADAR_STATE.prefetchTimer);RADAR_STATE.prefetchTimer=null;}}
async function radarPrefetchFrames(token){
  if(radarIsLite())return;
  const frames=RADAR_STATE.frames,slots=radarTileSlots(radarLatitude(),radarLongitude(),radarZoom()),ordered=[];
  if(frames.length<=1)return;
  // The timeline keeps every source frame. Only the two nearest older frames are
  // preloaded; the rest load on demand so opening radar never floods the Pi or feed.
  for(let step=1;step<=Math.min(RADAR_PREFETCH_NEAREST_FRAMES,frames.length-1);step++)ordered.push(frames[(RADAR_STATE.frame-step+frames.length)%frames.length]);
  for(const frame of ordered){if(token!==RADAR_STATE.token||!radarIsOpen())return;await radarPool(slots,async slot=>{if(token!==RADAR_STATE.token||!radarIsOpen())return;const url=radarTileURL(RADAR_STATE.provider,frame,slot);if(!url)return;const k=RADAR_STATE.provider+":"+(frame.time||"latest")+":"+slot.z+"/"+slot.x+"/"+slot.y;await radarImagePreload(k,url,false);},radarPrefetchConcurrency());}
}
function radarSchedulePrefetch(token){radarCancelPrefetch();if(radarIsLite()||RADAR_STATE.frames.length<=1)return;RADAR_STATE.prefetchTimer=setTimeout(()=>{RADAR_STATE.prefetchTimer=null;radarPrefetchFrames(token).catch(()=>{});},RADAR_PREFETCH_DELAY_MS);}
async function radarRainViewerFrames(){const r=await fetch("https://api.rainviewer.com/public/weather-maps.json",{cache:"no-store"});if(!r.ok)throw new Error("RainViewer frame list unavailable (HTTP "+r.status+")");const d=await r.json(),host=String(d.host||""),past=(d.radar&&Array.isArray(d.radar.past)?d.radar.past:[]).map(x=>({host,path:x.path,time:x.time})).filter(x=>x.path&&Number(x.time)>0);if(!host||!past.length)throw new Error("RainViewer returned no usable radar frames");return past;}
async function radarStatus(){const r=await fetch("/api/radar/status",{cache:"no-store"}),data=await r.json().catch(()=>({}));if(!r.ok)throw new Error(data.error||"Radar status unavailable");return data;}
function radarFallbackProvider(status){const active=(status&&status.provider)||RADAR_STATE.provider,current=(status&&Array.isArray(status.providers)?status.providers:[]).find(x=>x.id===active);return current&&current.keyRequired&&!current.hasKey?"rainviewer":(active||RADAR_DEFAULT_PROVIDER);}
async function radarUseFallback(reason,target){
  target=String(target||"rainviewer").toLowerCase();
  if(RADAR_STATE.fallbackTried||RADAR_STATE.provider===target||!radarIsOpen())return;
  RADAR_STATE.fallbackTried=true;radarSetPlaying(false);radarCancelPrefetch();const label=(radarMeta(target)||{}).label||target;radarSetError(String(reason||"Radar tiles unavailable")+` Showing ${label} instead.`);const token=RADAR_STATE.token;
  try{RADAR_STATE.provider=target;const m=radarMeta(target);radarSetText("radartitle",m.label);radarSetText("radarprovider",m.tier);radarSetAttribution(m);radarClearFrameLayers();
    if(radarIsLite()){await radarOpenLite(token,m);return;}
    RADAR_STATE.frames=m.kind==="rainviewer"?await radarRainViewerFrames():[{time:0,host:"",path:""}];if(token!==RADAR_STATE.token)return;RADAR_STATE.frame=Math.max(0,RADAR_STATE.frames.length-1);await radarRenderFrame(token);radarSetPlaying(false);radarSchedulePrefetch(token);
  }catch(_){if(token===RADAR_STATE.token)radarSetError(String(reason||"Radar tiles unavailable")+` ${label} fallback also failed.`);}
}
function radarSetPlaying(on){
  RADAR_STATE.playing=!!on&&radarCanAnimate();radarClearTimer();const b=document.getElementById("radarplay");if(b){b.textContent=RADAR_STATE.playing?"Pause":"Play";b.setAttribute("aria-pressed",RADAR_STATE.playing?"true":"false");b.disabled=!radarCanAnimate();}
  if(RADAR_STATE.playing)RADAR_STATE.timer=setInterval(()=>{if(!radarIsOpen())return;RADAR_STATE.frame=(RADAR_STATE.frame+1)%RADAR_STATE.frames.length;radarRenderFrame(RADAR_STATE.token);noteRadarInput();},radarFrameInterval());
}
async function openRadar(){
  const root=document.getElementById("radarfull");if(!root)return;const token=++RADAR_STATE.token;
  RADAR_STATE.priorFocus=document.activeElement;RADAR_STATE.open=true;RADAR_STATE.renderTier=radarBaseProfileTier();RADAR_STATE.error="";RADAR_STATE.fallbackTried=false;RADAR_STATE.fullFrameFailures=0;RADAR_STATE.renderQueued=false;RADAR_STATE.liteLastRefreshAt=0;RADAR_STATE.liteFramesFetchedAt=0;RADAR_STATE.liteZoom=RADAR_LITE_ZOOM;radarCancelPrefetch();radarClearRetry();radarClearTimer();radarCancelLiteRequests();radarFreeLiteFrames();radarFreeLiteBaseCache();radarClearFrameLayers();radarClearGrid(document.getElementById("radarbase"));radarClearLiteCanvas();
  radarSetError("");radarSetBusy(true);root.classList.toggle("radar-lite",radarIsLite());pauseUiAnimations();disarmOverlayAutoClose();root.classList.add("show");root.setAttribute("aria-hidden","false");if(window.DashGoAppDialog)window.DashGoAppDialog.focusInitial(root,"#radarclose");else requestAnimationFrame(()=>document.getElementById("radarclose")?.focus?.());armRadarAutoClose();
  await radarNextFrame();if(!radarStageReady())await radarNextFrame();
  let status=null;try{status=await radarStatus();if(token!==RADAR_STATE.token)return;RADAR_STATE.status=status;RADAR_STATE.provider=radarFallbackProvider(status);const m=radarMeta(RADAR_STATE.provider);radarSetText("radartitle",m.label);radarSetText("radarprovider",m.tier);radarSetAttribution(m);const bad=(status.providers||[]).find(x=>x.id===status.provider&&x.keyRequired&&!x.hasKey);if(bad)radarSetError(bad.label+" needs a saved key; showing RainViewer instead.");
    if(radarIsLite()){await radarOpenLite(token,m);return;}
    radarSetLiteControls(false);if(!radarStageReady())await radarNextFrame();await radarLoadBase(token);if(token!==RADAR_STATE.token)return;RADAR_STATE.frames=m.kind==="rainviewer"?await radarRainViewerFrames():[{time:0,host:"",path:""}];if(token!==RADAR_STATE.token)return;RADAR_STATE.frame=Math.max(0,RADAR_STATE.frames.length-1);await radarRenderFrame(token);radarSetPlaying(false);radarSchedulePrefetch(token);
  }catch(e){if(token!==RADAR_STATE.token)return;const fallback=status&&status.automatic&&status.fallbackProvider;if(fallback&&RADAR_STATE.provider!==fallback){await radarUseFallback(e&&e.message?e.message:"Radar source unavailable",fallback);return;}radarSetPlaying(false);radarStopLiteAnim();radarSetError("Radar unavailable — "+(e&&e.message?e.message:"network error"));RADAR_STATE.frames=[];radarClearFrameLayers();}finally{if(token===RADAR_STATE.token)radarSetBusy(false);}
}
function closeRadar(){if(typeof completeAppLauncherHandoff==="function")completeAppLauncherHandoff();const root=document.getElementById("radarfull");if(root){root.classList.remove("show","radar-lite");root.setAttribute("aria-hidden","true");}RADAR_STATE.open=false;RADAR_STATE.renderTier=null;RADAR_STATE.frames=[];RADAR_STATE.frame=0;++RADAR_STATE.token;RADAR_STATE.renderQueued=false;radarClearTimer();radarClearRetry();radarCancelPrefetch();disarmRadarAutoClose();radarClearFrameLayers();radarClearGrid(document.getElementById("radarbase"));radarClearLiteCanvas();radarCancelLiteRequests();radarFreeLiteFrames();radarFreeLiteScratch();radarFreeLiteBaseCache();RADAR_STATE.liteFramesFetchedAt=0;radarSetLiteControls(false);if(overlayIsOpen())armOverlayAutoClose();else resumeUiAfterOverlay();const trigger=document.getElementById("cblaunch");if(window.DashGoAppDialog)window.DashGoAppDialog.restoreFocus(RADAR_STATE.priorFocus,trigger);else (trigger&&!trigger.hidden?trigger:RADAR_STATE.priorFocus)?.focus?.();}
function radarStep(delta){if(radarIsLite()||!RADAR_STATE.frames.length)return;radarSetPlaying(false);RADAR_STATE.frame=(RADAR_STATE.frame+delta+RADAR_STATE.frames.length)%RADAR_STATE.frames.length;radarRenderFrame(RADAR_STATE.token);armRadarAutoClose();}
function radarJumpLatest(){if(radarIsLite()||!RADAR_STATE.frames.length)return;radarSetPlaying(false);RADAR_STATE.frame=RADAR_STATE.frames.length-1;radarRenderFrame(RADAR_STATE.token);armRadarAutoClose();}
function radarQueueResize(){clearTimeout(RADAR_RESIZE_TIMER);RADAR_RESIZE_TIMER=setTimeout(()=>{if(!radarIsOpen())return;if(radarIsLite()){if(RADAR_STATE.liteRendering||radarLitePx()===RADAR_STATE.litePx)return;radarFreeLiteBaseCache(RADAR_STATE.liteBase);radarRunLiteRebuild(radarBeginLiteRebuild(),"Radar resize failed",true);return;}const token=RADAR_STATE.token;radarClearGrid(document.getElementById("radarbase"));radarEnsureFrameLayers().forEach(radarClearGrid);RADAR_STATE.visibleLayer=-1;radarLoadBase(token).then(()=>radarRenderFrame(token));},160);}
function radarBindControls(){
  bindTap(document.getElementById("radarclose"),closeRadar);
  bindTap(document.getElementById("radarplay"),()=>{if(radarIsLite())radarLiteSetPlaying(!RADAR_STATE.litePlaying);else radarSetPlaying(!RADAR_STATE.playing);armRadarAutoClose();});
  bindTap(document.getElementById("radarprev"),()=>{if(radarIsLite())radarLiteStep(-1);else radarStep(-1);});
  bindTap(document.getElementById("radarnext"),()=>{if(radarIsLite())radarLiteStep(1);else radarStep(1);});
  bindTap(document.getElementById("radarnow"),()=>{if(radarIsLite())radarLiteNow();else radarJumpLatest();});
  bindTap(document.getElementById("radarrefresh"),radarRefreshLite);bindTap(document.getElementById("radarzoom"),radarLiteToggleZoom);
  const scrub=document.getElementById("radarscrub");if(scrub)scrub.addEventListener("input",()=>{if(radarIsLite()){radarLiteSetPlaying(false);radarDrawLiteFrame(Number(scrub.value)||0);armRadarAutoClose();return;}radarSetPlaying(false);RADAR_STATE.frame=Number(scrub.value)||0;radarRenderFrame(RADAR_STATE.token);armRadarAutoClose();});
  const root=document.getElementById("radarfull");if(root){bindTap(root,closeRadar,{ignore:event=>event.target!==root});["pointerdown","touchstart","wheel","keydown"].forEach(t=>root.addEventListener(t,noteRadarInput,{passive:true}));}
  document.addEventListener("keydown",event=>{if(event.key!=="Escape"||!radarIsOpen())return;event.preventDefault();closeRadar();});
  window.addEventListener("resize",radarQueueResize,{passive:true});document.addEventListener("visibilitychange",()=>{if(document.hidden){radarSetPlaying(false);radarStopLiteAnim();radarCancelPrefetch();}});
}

document.addEventListener("DOMContentLoaded",radarBindControls);
