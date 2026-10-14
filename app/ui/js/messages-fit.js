// 07-compliments-00a-fit.js — shared/fully measured rotating-message fitting.
// Lite-specific cached geometry and Canvas fitting live in 00b so ordinary
// cached rotations stay layout-read-free on Pi Zero class devices. A new cache
// entry receives one bounded rAF verification before it is trusted thereafter.
const COMP_FIT={hardFloor:16,maxLines:4,maxTargetLines:3,minVisualSize:42,maxVisualSize:72};
// Readable-size contract for both fit paths: preferred [1,2,3]-line floors,
// the drop threshold a non-priority message may never render below, and the
// fourth-line floor for priority text. The multiplier may raise floors, never lower drop.
const COMP_READABLE={
  xl:     {floors:[64,46,36],drop:36,priority:30},
  base:   {floors:[48,36,28],drop:28,priority:24},
  compact:{floors:[40,30,22],drop:22,priority:20},
  dense:  {floors:[34,26,20],drop:20,priority:18},
  min:    {floors:[30,22,18],drop:18,priority:16}};
const COMP_FIT_CACHE_LIMIT=120;
const COMP_FIT_CACHE=new Map();
// Session-only unreadable set, keyed per geometry revision so a lane change
// re-admits a dropped pick. Keys only; message text is never stored here.
const COMP_UNREADABLE=new Set();
const COMP_UNREADABLE_LIMIT=200;
// Session-only diagnostics for Dashboard Control: clips, corrections, and
// dropped unreadable picks. Counts and timestamps only, never message text.
const COMP_FIT_DIAGNOSTICS={clipped:0,corrected:0,lastClippedAt:0,dropped:0,lastDroppedAt:0};
function complimentFitDiagnostics(){ return {...COMP_FIT_DIAGNOSTICS}; }
const compUnreadableRevision=()=>{const g=typeof COMP_LITE_GEOMETRY!=="undefined"?COMP_LITE_GEOMETRY:null;return g?Number(g.revision)||0:0;};
function compUnreadableAdd(key){
  COMP_UNREADABLE.add(compUnreadableRevision()+"|"+String(key||""));
  while(COMP_UNREADABLE.size>COMP_UNREADABLE_LIMIT)COMP_UNREADABLE.delete(COMP_UNREADABLE.values().next().value);
}
function compUnreadableHas(key){return COMP_UNREADABLE.has(compUnreadableRevision()+"|"+String(key||""));}
function compUnreadableClear(){ COMP_UNREADABLE.clear(); }
function noteComplimentDropped(){COMP_FIT_DIAGNOSTICS.dropped+=1;COMP_FIT_DIAGNOSTICS.lastDroppedAt=Date.now();}
function noteComplimentFitClipped(fit){
  if(!fit||fit.fits!==false||fit.clipDiagnosticRecorded)return;
  fit.clipDiagnosticRecorded=true;COMP_FIT_DIAGNOSTICS.clipped+=1;COMP_FIT_DIAGNOSTICS.lastClippedAt=Date.now();
}
function complimentCleanText(text){ return String(text||"").trim().replace(/\s+/g," "); }
function complimentNumber(style,property){ return parseFloat(style&&style[property])||0; }
function complimentBoxMetrics(el){
  const parent=el&&el.parentElement;
  const parentStyle=parent?getComputedStyle(parent):null;
  const elStyle=el?getComputedStyle(el):null;
  const parentPad=complimentNumber(parentStyle,"paddingTop")+complimentNumber(parentStyle,"paddingBottom");
  const elPadV=complimentNumber(elStyle,"paddingTop")+complimentNumber(elStyle,"paddingBottom");
  const elPadH=complimentNumber(elStyle,"paddingLeft")+complimentNumber(elStyle,"paddingRight");
  const outerWidth=Math.max(1,(el&&el.clientWidth)||(el&&el.getBoundingClientRect().width)||320);
  const outerHeight=Math.max(42,(parent&&parent.clientHeight?parent.clientHeight:124)-parentPad);
  return {outerWidth,outerHeight,contentWidth:Math.max(1,outerWidth-elPadH),contentHeight:Math.max(18,outerHeight-elPadV),parentPad,elPadV,elPadH};
}
function complimentVisualCap(metrics){
  // CSS-pixel box measurements already reflect monitor scaling/zoom. Start from
  // the actual available text box rather than a viewport-height tier.
  const widthScale=Math.sqrt(Math.max(1,metrics.contentWidth)/800);
  const heightScale=Math.sqrt(Math.max(1,metrics.contentHeight)/74);
  const scale=Math.max(.58,Math.min(1,widthScale,heightScale));
  return Math.round(Math.max(COMP_FIT.minVisualSize,Math.min(COMP_FIT.maxVisualSize,COMP_FIT.maxVisualSize*scale)));
}
function complimentLineTarget(text,metrics,minimumLines){
  const display=typeof complimentDisplayText==="function"?complimentDisplayText(text):String(text||"");
  const clean=complimentCleanText(display);if(!clean)return 1;
  const forced=Math.max(1,display.split("\n").filter(Boolean).length),cap=complimentVisualCap(metrics);
  const minimum=Math.max(1,Math.min(COMP_FIT.maxTargetLines,Number(minimumLines)||1));
  const charsPerLine=Math.max(8,Math.floor(metrics.contentWidth/Math.max(1,cap*.52)));
  return Math.max(forced,Math.min(COMP_FIT.maxTargetLines,Math.max(minimum,Math.ceil(clean.length/charsPerLine))));
}
function complimentTypographyMultiplier(){
  const user=messageTypographySizeMultiplier();
  return Math.max(0.55,Math.min(1.45,Math.max(user,Number(CONFIG.messageTypographyMultiplier)||1)));
}
function complimentReadableTier(metrics){
  const tier=metrics&&metrics.tier?String(metrics.tier):(typeof document!=="undefined"&&document.documentElement&&document.documentElement.dataset&&document.documentElement.dataset.fit)||"";
  return COMP_READABLE[tier]||COMP_READABLE.base;
}
// Raised by the typography multiplier when it is above 1; it may raise floors
// but never lowers the tier's drop threshold below its table value.
function complimentReadableThreshold(metrics,kind){
  const table=complimentReadableTier(metrics),base=kind==="priority"?table.priority:table.drop,m=complimentTypographyMultiplier();
  return Math.max(base,Math.round(base*(m>1?m:1)));
}
function complimentRenderedLineHeight(lines){return Number(lines)<=1?1.03:1.08;}
function complimentFitLineHeight(lines){return Number(lines)<=1?1.06:1.13;}
function complimentVerticalFitReserve(size,lines,lite){
  // Budget a little more than the applied line-height. WebKit's final line box,
  // bold descenders, and semantic breaks can exceed Canvas' optimistic estimate.
  const count=Math.max(1,Math.min(COMP_FIT.maxLines,Number(lines)||1));
  const px=Math.max(1,Number(size)||0);
  const base=count<=1?5:count===2?8:count===3?11:13;
  const scale=Math.max(0,Math.min(3,Math.ceil(px/40)-1));
  // The Lite path deliberately keeps a touch more headroom because Canvas is a
  // predictor, not WebKit's final wrapping engine.
  return base+scale+(lite&&px>=28?2:0);
}
function complimentStartSizeFor(text,metrics,minimumLines){
  const lines=complimentLineTarget(text,metrics,minimumLines),clean=complimentCleanText(text);
  const ratio=complimentFitLineHeight(lines);
  const visualCap=complimentVisualCap(metrics)*complimentTypographyMultiplier();
  const expectedChars=Math.max(1,Math.ceil(clean.length/lines));
  const widthCap=metrics.contentWidth/Math.max(1,expectedChars*.52);
  const heightCap=metrics.contentHeight/(lines*ratio);
  return Math.max(COMP_FIT.hardFloor,Math.floor(Math.min(visualCap,widthCap,heightCap)));
}
function complimentFloorSize(metrics){
  // Preserve a resolution/box-scaled reading floor whenever it fits. The
  // absolute hard floor (16px) is reserved for genuinely constrained boxes.
  return Math.max(COMP_FIT.hardFloor,Math.round(complimentVisualCap(metrics)*.39*complimentTypographyMultiplier()));
}
function complimentLiteProfile(){ return dashboardLiteProfile(); }
function complimentTypographyBucket(){
  const root=document.documentElement;
  const preset=root.getAttribute("data-font-preset")||String(CONFIG.fontPreset||"default");
  const saved=typeof SETTINGS!=="undefined"&&SETTINGS?SETTINGS:{};
  const messageFont=typeof dashboardTypographyEffectiveFont==="function"?dashboardTypographyEffectiveFont("messageTextFont"):preset;
  const messageSize=typeof dashboardTypographyNumber==="function"?dashboardTypographyNumber("messageTextSize",saved.messageTextSize):0;
  const messageWeight=typeof dashboardTypographyNumber==="function"?dashboardTypographyNumber("messageTextWeight",saved.messageTextWeight):800;
  return String(CONFIG.profile||"balanced").toLowerCase()+":"+preset+":msg:"+messageFont+":"+messageSize+":"+messageWeight+":"+(document.fonts&&document.fonts.status||"font")+":dpr:"+(window.devicePixelRatio||1);
}
function cacheComplimentFit(key,fit){
  if(!key)return;
  if(COMP_FIT_CACHE.has(key))COMP_FIT_CACHE.delete(key);
  COMP_FIT_CACHE.set(key,fit);
  while(COMP_FIT_CACHE.size>COMP_FIT_CACHE_LIMIT)COMP_FIT_CACHE.delete(COMP_FIT_CACHE.keys().next().value);
}
function resetComplimentFit(el){
  for(const prop of ["font-size","line-height","letter-spacing","-webkit-line-clamp"])el.style.removeProperty(prop);
  delete el.dataset.fitLines;delete el.dataset.fitSize;delete el.dataset.fitClipped;delete el.dataset.fitLayout;delete el.dataset.fitCorrected;
}
function complimentBoxBucket(metrics){return Math.round(metrics.contentWidth/2)*2+"x"+Math.round(metrics.contentHeight/2)*2;}
function complimentFitKey(text,start,floor,lite,metrics,priority){
  // Lite keys exact normalized text to a stable geometry revision; priority
  // fits get their own key space so the two kinds of fits never cross-apply.
  const geometry=lite?String((metrics&&metrics.cacheKey)||(metrics&&metrics.revision)||complimentBoxBucket(metrics)):complimentBoxBucket(metrics);
  return [lite?"lite":"full",complimentTypographyBucket(),geometry,start,floor,priority?"priority":"",complimentCleanText(text)].join("|");
}
function applyComplimentFit(el,fit,metrics){
  // Non-priority text never falls below the tier drop threshold; priority text uses the 16px hard floor as its fourth-line last resort.
  const minSize=fit&&fit.priority?COMP_FIT.hardFloor:Math.max(COMP_FIT.hardFloor,complimentReadableThreshold(metrics,"drop"));
  const size=Math.max(minSize,Math.round((fit&&fit.size)||complimentFloorSize(metrics||complimentBoxMetrics(el))));
  const lines=Math.max(1,Math.round(Number(fit&&fit.lines)||1));
  el.style.fontSize=size+"px";
  el.style.lineHeight=String(complimentRenderedLineHeight(lines));
  el.style.letterSpacing=lines<=1?"-0.025em":"-0.014em";
  el.dataset.fitLines=String(lines);
  el.dataset.fitSize=String(size);
  el.dataset.fitClipped=(fit&&fit.fits===false)?"true":"false";
  el.dataset.fitLayout=String((fit&&fit.layout)||"single");
  el.dataset.fitCorrected=String(Math.max(0,Number(fit&&fit.correctionAttempts)||0));
  // Clamp to the fit that was actually selected. A conservative fallback is
  // ellipsis inside the footer, never text escaping below the viewport.
  el.style.webkitLineClamp=String(lines);
}
function complimentRenderedLineCount(el){
  try{
    const range=document.createRange();range.selectNodeContents(el);
    const rects=[...range.getClientRects()].filter(r=>r.width>1&&r.height>1);range.detach&&range.detach();
    if(rects.length){const tops=[];for(const r of rects){const t=Math.round(r.top);if(!tops.some(x=>Math.abs(x-t)<=2))tops.push(t);}return Math.max(1,tops.length);}
  }catch(_){}
  const sz=parseFloat(el.style.fontSize)||parseFloat(getComputedStyle(el).fontSize)||32;
  const lh=parseFloat(getComputedStyle(el).lineHeight)||sz*1.08;
  return Math.max(1,Math.ceil((el.scrollHeight-1)/lh));
}
function measureComplimentFit(el,text,start,floor,metrics,minimumLines,priority){
  const box=metrics||complimentBoxMetrics(el),maxH=Math.max(42,box.outerHeight),maxW=Math.max(1,box.outerWidth);
  // Non-priority text stops at the tier drop threshold; priority text may take a fourth line down to the priority floor, then the hard floor.
  const drop=complimentReadableThreshold(box,"drop");
  const priorityFloor=complimentReadableThreshold(box,"priority");
  const lineCap=priority?COMP_FIT.maxLines:COMP_FIT.maxTargetLines;
  const targetLines=Math.min(lineCap,complimentLineTarget(text,box,minimumLines));
  const old={fontSize:el.style.fontSize,lineHeight:el.style.lineHeight,webkitLineClamp:el.style.webkitLineClamp,letterSpacing:el.style.letterSpacing,visibility:el.style.visibility};
  el.style.visibility="hidden";el.style.webkitLineClamp="unset";el.style.lineHeight=String(complimentRenderedLineHeight(targetLines));el.style.letterSpacing=targetLines<=1?"-0.025em":"-0.014em";
  const measureAt=sz=>{
    el.style.fontSize=sz+"px";
    let lines=complimentRenderedLineCount(el);
    el.style.lineHeight=String(complimentRenderedLineHeight(lines));
    lines=complimentRenderedLineCount(el);
    const budget=complimentFitLineHeight(lines)*lines*sz+box.elPadV;
    return {lines,requiredH:Math.max(el.scrollHeight,budget)};
  };
  const fitsIn=(sz,lineMax)=>{
    const measured=measureAt(sz);
    const availableH=Math.max(18,maxH-complimentVerticalFitReserve(sz,measured.lines,false));
    return measured.lines<=lineMax&&Math.max(measured.requiredH,el.scrollHeight)<=availableH&&el.scrollWidth<=maxW+1;
  };
  const findLargest=(low,high,lineMax)=>{let best=low;while(low<=high){const mid=Math.floor((low+high)/2);if(fitsIn(mid,lineMax)){best=mid;low=mid+1;}else high=mid-1;}return best;};
  let best=0;
  if(start>=drop&&fitsIn(start,COMP_FIT.maxTargetLines))best=start;
  else if(fitsIn(drop,COMP_FIT.maxTargetLines))best=findLargest(drop,Math.max(start,drop),COMP_FIT.maxTargetLines);
  if(priority&&!best){
    if(fitsIn(priorityFloor,COMP_FIT.maxLines))best=findLargest(priorityFloor,Math.max(start,priorityFloor),COMP_FIT.maxLines);
    else if(fitsIn(COMP_FIT.hardFloor,COMP_FIT.maxLines))best=findLargest(COMP_FIT.hardFloor,Math.max(COMP_FIT.hardFloor,priorityFloor-1),COMP_FIT.maxLines);
  }
  let finalFits=false,lines=targetLines;
  if(best>0){
    el.style.fontSize=best+"px";
    finalFits=fitsIn(best,lineCap);
    lines=Math.min(lineCap,measureAt(best).lines);
  }
  el.style.fontSize=old.fontSize;el.style.lineHeight=old.lineHeight;el.style.webkitLineClamp=old.webkitLineClamp;el.style.letterSpacing=old.letterSpacing;el.style.visibility=old.visibility;
  return {size:best,lines,maxH,maxW,fits:finalFits,readable:priority?true:best>=drop,preferredFloor:floor,targetLines,box};
}
function complimentRenderedOverflow(el,fit){
  const planned=Math.max(1,Number(fit&&fit.lines)||1);
  if(!el||el.clientWidth<1||el.clientHeight<1)return {overflow:false,lines:planned};
  const clamp=el.style.webkitLineClamp;
  try{
    // Measure unclamped inside one animation frame, then restore synchronously
    // before paint. This exposes a Canvas/WebKit wrapping disagreement without
    // letting that disagreement become visible at the screen edge.
    el.style.webkitLineClamp="unset";
    const lines=complimentRenderedLineCount(el);
    const vertical=el.scrollHeight>el.clientHeight+1;
    const horizontal=el.scrollWidth>el.clientWidth+1;
    return {overflow:lines>planned||vertical||horizontal,lines:Math.max(planned,lines)};
  }finally{el.style.webkitLineClamp=clamp;}
}
function complimentCorrectRenderedFit(el,key,fit,metrics,lite,observedLines,onVerified){
  const minimum=Math.max(1,Math.min(fit.priority?COMP_FIT.maxLines:lite?3:COMP_FIT.maxLines,Number(observedLines)||1));
  let next;
  if(lite){
    const candidateMetrics=typeof complimentLiteMetricsForLines==="function"?complimentLiteMetricsForLines(metrics,minimum):metrics;
    next=complimentLiteFit(fit.displayText,candidateMetrics,minimum,{priority:!!fit.priority});
  }else{
    const candidateMetrics=complimentBoxMetrics(el);
    const start=complimentStartSizeFor(fit.displayText,candidateMetrics,minimum);
    next=measureComplimentFit(el,fit.displayText,start,complimentFloorSize(candidateMetrics),candidateMetrics,minimum,!!fit.priority);
  }
  // A re-fit below the drop threshold cannot be applied: end with the current fit as the final bounded clip.
  if(next&&next.readable===false&&!next.priority){
    fit.renderVerified=true;fit.fits=false;noteComplimentFitClipped(fit);cacheComplimentFit(key,fit);
    if(typeof onVerified!=="function")return;try{onVerified();}catch(_){}
    return;
  }
  next={...next,displayText:fit.displayText,layout:fit.layout||"single",priority:!!fit.priority,correctionAttempts:Math.max(0,Number(fit.correctionAttempts)||0)+1};
  COMP_FIT_DIAGNOSTICS.corrected+=1;
  cacheComplimentFit(key,next);el.textContent=next.displayText;complimentApplyFitAndVerify(el,key,next,metrics,lite,onVerified);
}
function complimentScheduleRenderedVerification(el,key,fit,metrics,lite,onVerified){
  if(!el||!key||fit.renderVerified||fit.renderVerifyPending)return;
  fit.renderVerifyPending=true;
  const notify=()=>{
    if(typeof onVerified!=="function")return;
    try{onVerified();}catch(_){}
  };
  const verify=()=>{
    fit.renderVerifyPending=false;
    if(el.__dashComplimentFitKey!==key||el.__dashComplimentFit!==fit){notify();return;}
    const observed=complimentRenderedOverflow(el,fit);
    if(!observed.overflow){fit.renderVerified=true;noteComplimentFitClipped(fit);cacheComplimentFit(key,fit);notify();return;}
    if(Number(fit.correctionAttempts)||0){fit.renderVerified=true;fit.fits=false;noteComplimentFitClipped(fit);cacheComplimentFit(key,fit);notify();return;}
    complimentCorrectRenderedFit(el,key,fit,metrics,lite,observed.lines,notify);
  };
  if(typeof requestAnimationFrame==="function")requestAnimationFrame(verify);else setTimeout(verify,0);
}
function complimentApplyFitAndVerify(el,key,fit,metrics,lite,onVerified){
  el.__dashComplimentFitKey=key;el.__dashComplimentFit=fit;
  applyComplimentFit(el,fit,metrics);complimentScheduleRenderedVerification(el,key,fit,metrics,lite,onVerified);
}
function fitCompliment(rawText,opts){
  const el=document.getElementById("comptext");if(!el)return;
  const text=complimentCleanText(rawText===undefined?(el.__dashComplimentRawText||el.textContent||""):rawText);
  el.__dashComplimentRawText=text;
  // Without explicit options the refit keeps the displayed item's priority, so a
  // four-line priority fit is never measured against the non-priority drop rule.
  const priority=opts?!!opts.priority:!!(typeof _lastCompItem!=="undefined"&&_lastCompItem&&_lastCompItem.priority);
  // Never let a previous message's inline size determine the next fit.
  resetComplimentFit(el);
  applyComplimentFitResult(el,computeComplimentFit(text,el,{priority}));
}
function computeComplimentFit(text,el,opts){
  const lite=complimentLiteProfile(),metrics=lite?complimentLiteMetricsForFit(el):complimentBoxMetrics(el);
  const priority=!!(opts&&opts.priority);
  const baseline=lite?complimentLiteFit(text,metrics,1,{priority}):null;
  const start=lite?baseline.size:complimentStartSizeFor(text,metrics);
  const floor=lite?baseline.preferredFloor:complimentFloorSize(metrics);
  const key=complimentFitKey(text,start,floor,lite,metrics,priority),cached=COMP_FIT_CACHE.get(key);
  if(cached)return {fit:cached,metrics,lite,key};
  const assess=(displayText,candidate)=>{
    const plannedLines=Math.max(1,Number(candidate&&candidate.lines)||1);
    if(lite)return complimentLiteFit(displayText,typeof complimentLiteMetricsForLines==="function"?complimentLiteMetricsForLines(metrics,plannedLines):metrics,plannedLines,{priority});
    if(plannedLines>1)el.dataset.fitLines=String(plannedLines);else delete el.dataset.fitLines;
    const candidateMetrics=complimentBoxMetrics(el),candidateStart=complimentStartSizeFor(displayText,candidateMetrics,plannedLines),candidateFloor=complimentFloorSize(candidateMetrics);
    el.textContent=displayText;return measureComplimentFit(el,displayText,candidateStart,candidateFloor,candidateMetrics,plannedLines,priority);
  };
  const decision=typeof complimentLayoutChoose==="function"?complimentLayoutChoose(text,assess):{candidate:{displayText:text,kind:"single"},fit:assess(text)};
  const fit={...(decision.fit||baseline||{}),displayText:(decision.candidate&&decision.candidate.displayText)||text,layout:(decision.candidate&&decision.candidate.kind)||"single",priority};
  return {fit,metrics,lite,key};
}
function applyComplimentFitResult(el,result,onVerified){
  const fit=result.fit,metrics=result.metrics,lite=result.lite,key=result.key;
  el.__dashComplimentFitKey=key;el.__dashComplimentFit=fit;
  // A non-priority message that cannot reach the tier drop threshold is never rendered:
  // the lane stays empty for this rotation, and the tap target #compliment stays intact.
  if(fit&&fit.readable===false){
    el.textContent="";applyComplimentFit(el,fit,metrics);
    if(typeof onVerified!=="function")return;try{onVerified();}catch(_){}
    return;
  }
  el.textContent=(fit&&fit.displayText)||"";
  cacheComplimentFit(key,fit);
  applyComplimentFit(el,fit,metrics);
  if(fit.renderVerified||fit.renderVerifyPending){
    // Another verification already owns this element's fit: one fallback rAF keeps the
    // double-rAF rule — the fade-in starts only after the element painted at opacity 0.
    if(typeof onVerified!=="function")return;
    if(typeof requestAnimationFrame==="function")requestAnimationFrame(onVerified);else setTimeout(onVerified,0);
    return;
  }
  complimentScheduleRenderedVerification(el,key,fit,metrics,lite,onVerified);
}
// Hidden Lite fade-mode selector: opacity is the default and leaves the base
// CSS untouched; color / color-stepped fade the text colour instead of an
// opacity layer. Owner-set in settings.json only — the fade contract forbids UI.
function liteMessageFadeMode(raw){
  const value=typeof raw==="string"?raw:(typeof SETTINGS!=="undefined"&&SETTINGS?SETTINGS.liteMessageFade:"");
  const mode=String(value||"").toLowerCase();
  return mode==="color"||mode==="color-stepped"?mode:"opacity";
}
// Single funnel for every #comptext visibility change, so pause, force, and
// instant swaps behave identically in all fade modes.
function compSetVisible(el,visible){
  if(!el||!el.style)return;
  if(liteMessageFadeMode()==="opacity"){
    el.style.opacity=visible?"1":"0";
    if(el.classList)el.classList.remove("comp-fading");
  }else{
    el.style.opacity="";
    if(el.classList)el.classList.toggle("comp-fading",!visible);
  }
}
function complimentFadeValueMs(value){const raw=Number(value);return Number.isFinite(raw)?Math.max(0,Math.min(5000,Math.round(raw))):750;}
function complimentFadeLabel(value){const ms=complimentFadeValueMs(value);if(ms===0)return "Instant";if(ms<=150)return "Very fast";if(ms<=350)return "Fast";if(ms<=800)return "Normal";if(ms<=1500)return "Slow";return "Very slow";}
function complimentFadeSummary(value){const ms=complimentFadeValueMs(value);return ms===0?"Instant (0 ms)":complimentFadeLabel(ms)+" ("+ms+" ms)";}
function complimentFadeDelayMs(){return complimentFadeValueMs(CONFIG.complimentFadeMs);}
function clockSafeComplimentDelay(delay){
  delay=Math.max(500,+delay||500);
  if(!(CONFIG.showSeconds&&complimentLiteProfile()))return delay;
  const projected=(Date.now()+delay)%1000;
  if(projected<180)return delay+(220-projected);
  if(projected>840)return delay+(1220-projected);
  return delay;
}
