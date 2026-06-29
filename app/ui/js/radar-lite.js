// 06-radar-02-lite.js — bounded Lite radar canvas, progressive frames, and transitions.
function radarLiteCanvas(){ return document.getElementById("radarlitecanvas"); }
function radarLiteMaxPx(){ return RADAR_LITE_RENDER_PX; }
function radarLiteFrameLimit(){ return radarFrameCount("rainviewer"); }
function radarLitePx(){
  const st=radarStage(),max=radarLiteMaxPx();let base=max;
  if(st){
    let width=st.clientWidth,height=st.clientHeight;
    try{const cs=getComputedStyle(st);width-=parseFloat(cs.paddingLeft||0)+parseFloat(cs.paddingRight||0);height-=parseFloat(cs.paddingTop||0)+parseFloat(cs.paddingBottom||0);}catch(_){}
    base=Math.min(width,height);
  }
  return radarClamp(Math.round(base)||RADAR_LITE_MIN_PX,RADAR_LITE_MIN_PX,max);
}
function radarLiteScratch(px){
  let canvas=RADAR_STATE.liteScratch;
  if(!canvas){canvas=document.createElement("canvas");RADAR_STATE.liteScratch=canvas;}
  if(canvas.width!==px||canvas.height!==px)canvas.width=canvas.height=px;
  return canvas;
}
function radarFreeLiteScratch(){
  const canvas=RADAR_STATE.liteScratch;if(canvas){canvas.width=canvas.height=1;}RADAR_STATE.liteScratch=null;
}
function radarReleaseLiteFrame(frame){
  try{
    if(frame&&typeof frame.close==="function")frame.close();
    else if(frame&&typeof frame.width==="number")frame.width=frame.height=1;
  }catch(_){}
}
function radarLiteBaseConcurrency(){ return 2; }
function radarLiteRadarConcurrency(){ return 3; }
function radarLiteFrameIndices(){
  const out=[],frames=RADAR_STATE.liteFrames||[];
  for(let i=0;i<frames.length;i++)if(frames[i])out.push(i);
  return out;
}
function radarLiteAvailableFrameCount(){ return radarLiteFrameIndices().length; }
function radarLiteResolveFrameIndex(index){
  const available=radarLiteFrameIndices();if(!available.length)return -1;
  const wanted=radarClamp(Number(index)||0,0,Math.max(0,(RADAR_STATE.liteFrames||[]).length-1));
  if((RADAR_STATE.liteFrames||[])[wanted])return wanted;
  let best=available[0],distance=Math.abs(best-wanted);
  for(const i of available){const d=Math.abs(i-wanted);if(d<distance||(d===distance&&i>best)){best=i;distance=d;}}
  return best;
}
function radarLiteStepIndex(delta){
  const available=radarLiteFrameIndices();if(!available.length)return -1;
  const current=radarLiteResolveFrameIndex(RADAR_STATE.liteFrameIdx),at=Math.max(0,available.indexOf(current));
  return available[(at+delta+available.length)%available.length];
}
function radarLiteBaseSignature(z,px){ return [z,px,radarLatitude().toFixed(5),radarLongitude().toFixed(5)].join("|"); }
function radarLiteBaseIsCached(canvas){
  if(!canvas)return false;
  const cache=RADAR_STATE.liteBaseCache||{};
  return Object.values(cache).some(entry=>entry&&entry.canvas===canvas);
}
function radarFreeLiteBaseCache(preserveCanvas){
  const cache=RADAR_STATE.liteBaseCache||{},seen=new Set();
  for(const entry of Object.values(cache)){
    const canvas=entry&&entry.canvas;
    if(!canvas||canvas===preserveCanvas||seen.has(canvas))continue;
    seen.add(canvas);try{canvas.width=canvas.height=1;}catch(_){}
  }
  RADAR_STATE.liteBaseCache={};
}
async function radarLiteBaseFor(z,px,token){
  const cache=RADAR_STATE.liteBaseCache||(RADAR_STATE.liteBaseCache={}),key=String(z),signature=radarLiteBaseSignature(z,px),hit=cache[key];
  if(hit&&hit.signature===signature&&hit.px===px&&hit.canvas&&hit.canvas.width===px&&hit.canvas.height===px)return hit;
  if(hit){
    delete cache[key];
    if(hit.canvas&&hit.canvas!==RADAR_STATE.liteBase){try{hit.canvas.width=hit.canvas.height=1;}catch(_){}}
  }
  const canvas=document.createElement("canvas");
  try{
    const built=await radarBuildLiteBase(token,px,z,canvas);
    if(token!==RADAR_STATE.token){canvas.width=canvas.height=1;return null;}
    const record={canvas,plan:built.plan,baseLoaded:built.baseLoaded,px,signature};cache[key]=record;return record;
  }catch(err){canvas.width=canvas.height=1;throw err;}
}
function radarStopLiteAnim(){
  if(RADAR_STATE.liteTimer){clearInterval(RADAR_STATE.liteTimer);RADAR_STATE.liteTimer=null;}
  RADAR_STATE.litePlaying=false;
  const b=document.getElementById("radarplay");
  if(b){b.textContent="Play";b.setAttribute("aria-pressed","false");b.disabled=!radarLiteCanAnimate();}
}
function radarFreeLiteFrames(){
  radarStopLiteAnim();
  for(const frame of RADAR_STATE.liteFrames||[])radarReleaseLiteFrame(frame);
  RADAR_STATE.liteFrames=[];RADAR_STATE.liteFrameMeta=[];RADAR_STATE.liteFrameIdx=0;RADAR_STATE.litePx=0;
  const base=RADAR_STATE.liteBase;if(base&&!radarLiteBaseIsCached(base)){try{base.width=base.height=1;}catch(_){}}
  RADAR_STATE.liteBase=null;
}
function radarClearLiteCanvas(){
  const canvas=radarLiteCanvas();if(!canvas)return;
  // A 1×1 backing store releases the visible canvas allocation on close.
  canvas.width=1;canvas.height=1;canvas.hidden=true;
}
function radarSetLiteControls(on){
  const root=document.getElementById("radarfull");if(root)root.classList.toggle("radar-lite",!!on);
  const multi=radarLiteAvailableFrameCount()>1;
  for(const id of ["radarprev","radarplay","radarnext","radarnow"]){
    const e=document.getElementById(id);if(e){e.hidden=!on;e.disabled=!on||!multi;}
  }
  const scrub=document.getElementById("radarscrub");
  if(scrub){scrub.hidden=!on;scrub.disabled=!on||!multi;if(on){scrub.max=String(Math.max(0,(RADAR_STATE.liteFrames||[]).length-1));scrub.value=String(radarLiteResolveFrameIndex(RADAR_STATE.liteFrameIdx));}}
  const zoom=document.getElementById("radarzoom");
  if(zoom){zoom.hidden=!on;zoom.disabled=!on;zoom.textContent=RADAR_STATE.liteZoom===7?"Zoom out":"Zoom in";}
  const refresh=document.getElementById("radarrefresh");if(refresh){refresh.hidden=!on;refresh.disabled=!on;}
}
function radarYield(){return new Promise(resolve=>setTimeout(resolve,0));}
function radarLiteNowMs(){return typeof performance!=="undefined"&&typeof performance.now==="function"?performance.now():Date.now();}
function radarCancelLiteRequests(){
  const pending=RADAR_STATE.liteRequests||new Set();
  for(const request of pending){try{request.cancel();}catch(_){}}
  pending.clear();
}
function radarLoadDetachedImage(url,token){
  return new Promise(resolve=>{
    const img=new Image();let settled=false;
    const request={cancel(){if(settled)return;img.onload=img.onerror=null;img.src="";finish(null);}};
    const finish=value=>{if(settled)return;settled=true;(RADAR_STATE.liteRequests||new Set()).delete(request);resolve(value);};
    (RADAR_STATE.liteRequests||(RADAR_STATE.liteRequests=new Set())).add(request);
    img.decoding="async";
    img.onload=()=>{if(token!==RADAR_STATE.token){img.src="";finish(null);return;}finish(img);};
    img.onerror=()=>finish(null);img.src=url;
  });
}
function radarLiteFrameLimitForZoom(){
  // Source frames are the timeline contract at both zooms; tile work stays
  // sequential/progressive and each composite is released when radar closes.
  return radarLiteFrameLimit();
}
function radarReleasePendingLiteFrames(frames){
  const owned=new Set(RADAR_STATE.liteFrames||[]);
  for(const frame of frames||[])if(frame&&!owned.has(frame))radarReleaseLiteFrame(frame);
}
async function radarBuildLiteBase(token,px,z,base){
  if(!base)base=document.createElement("canvas");if(base.width!==px||base.height!==px)base.width=base.height=px;
  const ctx=base.getContext("2d",{alpha:false});if(!ctx)throw new Error("radar canvas is unavailable");
  ctx.clearRect(0,0,px,px);ctx.fillStyle="#0d161f";ctx.fillRect(0,0,px,px);
  const plan=radarLiteTilePlan(px,radarLatitude(),radarLongitude(),z),coverage={planned:plan.planned,settled:0,drawn:0,failed:0};let budget=radarLiteNowMs();
  await radarPool(plan.slots,async slot=>{
    if(token!==RADAR_STATE.token)return;
    const img=await radarLoadDetachedImage(radarBaseTileURL(slot),token);coverage.settled++;
    if(!img||token!==RADAR_STATE.token){coverage.failed++;return;}
    ctx.drawImage(img,plan.ox+slot.col*RADAR_TILE,plan.oy+slot.row*RADAR_TILE,RADAR_TILE,RADAR_TILE);img.src="";coverage.drawn++;
    if(radarLiteNowMs()-budget>12){await radarYield();budget=radarLiteNowMs();}
  },radarLiteBaseConcurrency());
  if(token!==RADAR_STATE.token)return {canvas:base,plan,baseLoaded:0,coverage,cancelled:true};
  if(coverage.drawn!==coverage.planned){base.width=base.height=1;throw new Error("complete base map coverage is unavailable ("+coverage.drawn+"/"+coverage.planned+")");}
  ctx.fillStyle=RADAR_LITE_BASE_DIM;ctx.fillRect(0,0,px,px);
  return {canvas:base,plan,baseLoaded:coverage.drawn,coverage,complete:true};
}
function radarCommitLiteView(baseCanvas,bitmaps,seq,px,showIdx){
  const old={base:RADAR_STATE.liteBase,frames:RADAR_STATE.liteFrames};
  radarStopLiteAnim();
  RADAR_STATE.liteBase=baseCanvas;RADAR_STATE.liteFrames=bitmaps.slice();RADAR_STATE.liteFrameMeta=seq;RADAR_STATE.litePx=px;RADAR_STATE.liteFrameIdx=showIdx;
  const keep=new Set(bitmaps.filter(Boolean));
  for(const frame of old.frames||[])if(frame&&!keep.has(frame))radarReleaseLiteFrame(frame);
  if(old.base&&old.base!==baseCanvas&&!radarLiteBaseIsCached(old.base)){try{old.base.width=old.base.height=1;}catch(_){}}
  radarDrawLiteFrame(showIdx);
}
async function radarBuildLiteFrames(token){
  const px=radarLitePx(),z=RADAR_STATE.liteZoom||RADAR_LITE_ZOOM,base=await radarLiteBaseFor(z,px,token);
  if(!base||token!==RADAR_STATE.token)return false;
  const scratch=radarLiteScratch(px),sctx=scratch.getContext("2d",{alpha:true});if(!sctx)throw new Error("radar canvas is unavailable");
  const seq=(RADAR_STATE.frames||[]).slice(-radarLiteFrameLimitForZoom(z));if(!seq.length)throw new Error("no radar frames available");
  const newest=seq.length-1,order=[newest];for(let i=0;i<newest;i++)order.push(i);
  const bitmaps=new Array(seq.length).fill(null);let committed=false;
  try{
    for(const fi of order){
      if(token!==RADAR_STATE.token){radarReleasePendingLiteFrames(bitmaps);return false;}
      sctx.clearRect(0,0,px,px);const coverage={planned:base.plan.planned,settled:0,drawn:0,failed:0};let budget=radarLiteNowMs();
      await radarPool(base.plan.slots,async slot=>{
        if(token!==RADAR_STATE.token)return;
        const url=radarTileURL(RADAR_STATE.provider,seq[fi],slot),img=url?await radarLoadDetachedImage(url,token):null;coverage.settled++;
        if(!img||token!==RADAR_STATE.token){coverage.failed++;return;}
        sctx.drawImage(img,base.plan.ox+slot.col*RADAR_TILE,base.plan.oy+slot.row*RADAR_TILE,RADAR_TILE,RADAR_TILE);img.src="";coverage.drawn++;
        if(radarLiteNowMs()-budget>12){await radarYield();budget=radarLiteNowMs();}
      },radarLiteRadarConcurrency());
      if(token!==RADAR_STATE.token){radarReleasePendingLiteFrames(bitmaps);return false;}
      // The first (newest) still is transactional: a partial tile set never replaces a complete old view.
      if(coverage.drawn!==coverage.planned){
        if(!committed)throw new Error("complete radar coverage is unavailable ("+coverage.drawn+"/"+coverage.planned+")");
        continue;
      }
      let bitmap=null;try{if(typeof createImageBitmap==="function")bitmap=await createImageBitmap(scratch);}catch(_){bitmap=null;}
      if(!bitmap){const copy=document.createElement("canvas");copy.width=copy.height=px;const copyCtx=copy.getContext("2d",{alpha:true});if(!copyCtx)throw new Error("radar canvas is unavailable");copyCtx.drawImage(scratch,0,0);bitmap=copy;}
      if(token!==RADAR_STATE.token){radarReleaseLiteFrame(bitmap);radarReleasePendingLiteFrames(bitmaps);return false;}
      bitmaps[fi]=bitmap;
      if(!committed){radarCommitLiteView(base.canvas,bitmaps,seq,px,fi);committed=true;radarSetBusy(false);radarSetLiteControls(true);}
      else{RADAR_STATE.liteFrames=bitmaps.slice();RADAR_STATE.liteFrameMeta=seq;if(radarLiteAvailableFrameCount()===2)radarSetLiteControls(true);}
    }
    if(!committed)throw new Error("complete radar coverage is unavailable");
    RADAR_STATE.liteFrames=bitmaps;RADAR_STATE.liteFrameMeta=seq;return true;
  }catch(err){radarReleasePendingLiteFrames(bitmaps);if(committed)return true;throw err;}
}
function radarDrawLiteFrame(index){
  const target=radarLiteCanvas(),frames=RADAR_STATE.liteFrames||[],base=RADAR_STATE.liteBase;if(!target||!frames.length)return;
  const idx=radarLiteResolveFrameIndex(index);if(idx<0)return;
  const px=RADAR_STATE.litePx||radarLitePx();
  if(target.width!==px||target.height!==px)target.width=target.height=px;
  const out=target.getContext("2d",{alpha:false,desynchronized:true});if(!out)return;
  if(base)out.drawImage(base,0,0);else{out.fillStyle="#0d161f";out.fillRect(0,0,px,px);}
  out.drawImage(frames[idx],0,0);target.hidden=false;RADAR_STATE.liteFrameIdx=idx;
  const frame=RADAR_STATE.liteFrameMeta[idx]||{},available=radarLiteAvailableFrameCount();
  const counter=frames.length>1?(available===frames.length?" · "+(idx+1)+"/"+frames.length:" · loading "+available+"/"+frames.length):"";
  const scope=RADAR_STATE.liteZoom===7?"local snapshot":"regional snapshot";
  radarSetText("radarstamp",radarFrameLabel(frame)+" · "+scope+counter);
  const scrub=document.getElementById("radarscrub");if(scrub&&radarIsLite())scrub.value=String(idx);
}
function radarLiteCanAnimate(){return radarIsLite()&&radarLiteAvailableFrameCount()>1;}
function radarLiteSetPlaying(on){
  radarStopLiteAnim();
  RADAR_STATE.litePlaying=!!on&&radarLiteCanAnimate();
  const b=document.getElementById("radarplay");
  if(b){b.textContent=RADAR_STATE.litePlaying?"Pause":"Play";b.setAttribute("aria-pressed",RADAR_STATE.litePlaying?"true":"false");b.disabled=!radarLiteCanAnimate();}
  if(RADAR_STATE.litePlaying)RADAR_STATE.liteTimer=setInterval(()=>{
    if(!radarIsOpen()||!radarLiteCanAnimate()){radarStopLiteAnim();return;}
    const next=radarLiteStepIndex(1);if(next!==RADAR_STATE.liteFrameIdx)radarDrawLiteFrame(next);noteRadarInput();
  },RADAR_LITE_FRAME_MS);
}
function radarLiteStep(delta){
  if(!radarLiteCanAnimate())return;
  radarLiteSetPlaying(false);radarDrawLiteFrame(radarLiteStepIndex(delta));armRadarAutoClose();
}
function radarLiteNow(){
  const available=radarLiteFrameIndices();if(!available.length)return;
  radarLiteSetPlaying(false);radarDrawLiteFrame(available[available.length-1]);armRadarAutoClose();
}
async function radarRenderLiteSnapshot(token){
  if(!radarLiteCanvas()||token!==RADAR_STATE.token||RADAR_STATE.liteRendering)return false;
  RADAR_STATE.liteRendering=true;
  try{return await radarBuildLiteFrames(token);}
  finally{
    RADAR_STATE.liteRendering=false;
    if(token===RADAR_STATE.token){radarSetLiteControls(radarIsLite());if(radarIsOpen()&&radarLitePx()!==RADAR_STATE.litePx)radarQueueResize();}
  }
}
async function radarOpenLite(token,meta,reuseFrames){
  radarSetLiteControls(true);radarClearFrameLayers();radarClearGrid(document.getElementById("radarbase"));
  let frames;
  const recent=!!reuseFrames&&Array.isArray(RADAR_STATE.frames)&&RADAR_STATE.frames.length>0&&Date.now()-Number(RADAR_STATE.liteFramesFetchedAt||0)<RADAR_LITE_REFRESH_MS;
  if(meta.kind==="rainviewer"){
    frames=recent?RADAR_STATE.frames:await radarRainViewerFrames();
    if(!recent)RADAR_STATE.liteFramesFetchedAt=Date.now();
  }else{frames=[{time:0,host:"",path:""}];RADAR_STATE.liteFramesFetchedAt=0;}
  if(token!==RADAR_STATE.token)return false;
  RADAR_STATE.frames=frames;RADAR_STATE.frame=Math.max(0,frames.length-1);
  const rendered=await radarRenderLiteSnapshot(token);if(rendered)radarStopLiteAnim();return rendered;
}
// A refresh or zoom retains a complete old view until the first complete new
// base/radar composite is ready. The generation token prevents stale callbacks
// from committing into the new view.
function radarBeginLiteRebuild(){
  radarStopLiteAnim();radarCancelLiteRequests();
  const token=++RADAR_STATE.token;RADAR_STATE.liteRendering=false;return token;
}
function radarRunLiteRebuild(token,reason,reuseFrames){
  radarSetBusy(true);radarSetError("");radarSetLiteControls(true);
  return radarOpenLite(token,radarMeta(RADAR_STATE.provider),!!reuseFrames).catch(err=>{
    if(token===RADAR_STATE.token)radarSetError(reason+" — "+(err&&err.message?err.message:"network error"));
    return false;
  }).finally(()=>{if(token===RADAR_STATE.token)radarSetBusy(false);});
}
function radarRefreshLite(){
  if(!radarIsLite()||!radarIsOpen())return Promise.resolve(false);
  const now=Date.now();
  if(now-RADAR_STATE.liteLastRefreshAt<RADAR_LITE_REFRESH_MS){
    const sec=Math.ceil((RADAR_LITE_REFRESH_MS-(now-RADAR_STATE.liteLastRefreshAt))/1000);
    radarSetText("radarstamp","Refresh available in "+sec+" sec");return Promise.resolve(false);
  }
  RADAR_STATE.liteLastRefreshAt=now;
  return radarRunLiteRebuild(radarBeginLiteRebuild(),"Radar refresh failed",false);
}
function radarLiteToggleZoom(){
  if(!radarIsLite()||!radarIsOpen())return Promise.resolve(false);
  const token=radarBeginLiteRebuild();
  RADAR_STATE.liteZoom=RADAR_STATE.liteZoom===7?RADAR_LITE_ZOOM:7;
  radarSetAttribution(radarMeta(RADAR_STATE.provider));
  return radarRunLiteRebuild(token,"Zoom failed",true);
}
