// 07-compliments-00a-fit.js — shared/fully measured rotating-message fitting.
// Lite-specific cached geometry and Canvas fitting live in 00b so ordinary
// cached rotations stay layout-read-free on Pi Zero class devices. A new cache
// entry receives one bounded rAF verification before it is trusted thereafter.
const COMP_FIT={hardFloor:12,maxLines:4,maxTargetLines:3,minVisualSize:42,maxVisualSize:72};
const COMP_FIT_CACHE_LIMIT=120;
const COMP_FIT_CACHE=new Map();
// Session-only diagnostics expose rare final CSS-clamp clips in Dashboard
// Control without storing message text or adding work during rotation.
const COMP_FIT_DIAGNOSTICS={clipped:0,corrected:0,lastClippedAt:0};
function complimentFitDiagnostics(){ return {...COMP_FIT_DIAGNOSTICS}; }
function noteComplimentFitClipped(fit){
  if(!fit||fit.fits!==false||fit.clipDiagnosticRecorded) return;
  fit.clipDiagnosticRecorded=true;
  COMP_FIT_DIAGNOSTICS.clipped+=1;
  COMP_FIT_DIAGNOSTICS.lastClippedAt=Date.now();
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
  const raw=typeof messageTypographySizeMultiplier==="function"?messageTypographySizeMultiplier():1;
  return Number.isFinite(+raw)?Math.max(.60,Math.min(1.50,+raw)):1;
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
  // absolute 12px fallback is reserved for a genuinely constrained box.
  return Math.max(COMP_FIT.hardFloor,Math.round(complimentVisualCap(metrics)*.39*complimentTypographyMultiplier()));
}
function complimentLiteProfile(){
  try{
    if(typeof liteVisualProfile==="function")return liteVisualProfile();
    if(typeof startupLiteProfile==="function")return startupLiteProfile();
  }catch(_){}
  return ["lite","zero2","low","low-power"].includes(String(CONFIG.profile||"").toLowerCase());
}
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
function complimentFitKey(text,start,floor,lite,metrics){
  // Lite must cache exact normalized text against a stable geometry revision;
  // same-length messages can wrap very differently, especially with long words.
  const geometry=lite?String((metrics&&metrics.cacheKey)||(metrics&&metrics.revision)||complimentBoxBucket(metrics)):complimentBoxBucket(metrics);
  return [lite?"lite":"full",complimentTypographyBucket(),geometry,start,floor,complimentCleanText(text)].join("|");
}
function applyComplimentFit(el,fit,metrics){
  const size=Math.max(COMP_FIT.hardFloor,Math.round((fit&&fit.size)||complimentFloorSize(metrics||complimentBoxMetrics(el))));
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
function measureComplimentFit(el,text,start,floor,metrics,minimumLines){
  const box=metrics||complimentBoxMetrics(el),maxH=Math.max(42,box.outerHeight),maxW=Math.max(1,box.outerWidth);
  const targetLines=complimentLineTarget(text,box,minimumLines);
  const preferredFloor=Math.max(COMP_FIT.hardFloor,Math.min(Math.max(COMP_FIT.hardFloor,start-1),floor));
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
  const fits=sz=>{
    const measured=measureAt(sz);
    const availableH=Math.max(18,maxH-complimentVerticalFitReserve(sz,measured.lines,false));
    return measured.lines<=COMP_FIT.maxLines&&Math.max(measured.requiredH,el.scrollHeight)<=availableH&&el.scrollWidth<=maxW+1;
  };
  const findLargest=(low,high)=>{let best=low;while(low<=high){const mid=Math.floor((low+high)/2);if(fits(mid)){best=mid;low=mid+1;}else high=mid-1;}return best;};
  let best=COMP_FIT.hardFloor;
  if(fits(start))best=start;
  else if(fits(preferredFloor))best=findLargest(preferredFloor,start-1);
  else if(fits(COMP_FIT.hardFloor))best=findLargest(COMP_FIT.hardFloor,Math.max(COMP_FIT.hardFloor,preferredFloor-1));
  el.style.fontSize=best+"px";const finalFits=fits(best),lines=Math.min(COMP_FIT.maxLines,measureAt(best).lines);
  el.style.fontSize=old.fontSize;el.style.lineHeight=old.lineHeight;el.style.webkitLineClamp=old.webkitLineClamp;el.style.letterSpacing=old.letterSpacing;el.style.visibility=old.visibility;
  return {size:best,lines,maxH,maxW,fits:finalFits,preferredFloor,targetLines,box};
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
function complimentCorrectRenderedFit(el,key,fit,metrics,lite,observedLines){
  const minimum=Math.max(1,Math.min(lite?3:COMP_FIT.maxLines,Number(observedLines)||1));
  let next;
  if(lite){
    const candidateMetrics=typeof complimentLiteMetricsForLines==="function"?complimentLiteMetricsForLines(metrics,minimum):metrics;
    next=complimentLiteFit(fit.displayText,candidateMetrics,minimum);
  }else{
    const candidateMetrics=complimentBoxMetrics(el);
    const start=complimentStartSizeFor(fit.displayText,candidateMetrics,minimum);
    next=measureComplimentFit(el,fit.displayText,start,complimentFloorSize(candidateMetrics),candidateMetrics,minimum);
  }
  next={...next,displayText:fit.displayText,layout:fit.layout||"single",correctionAttempts:Math.max(0,Number(fit.correctionAttempts)||0)+1};
  COMP_FIT_DIAGNOSTICS.corrected+=1;
  cacheComplimentFit(key,next);el.textContent=next.displayText;complimentApplyFitAndVerify(el,key,next,metrics,lite);
}
function complimentScheduleRenderedVerification(el,key,fit,metrics,lite){
  if(!el||!key||fit.renderVerified||fit.renderVerifyPending)return;
  fit.renderVerifyPending=true;
  const verify=()=>{
    fit.renderVerifyPending=false;
    if(el.__dashComplimentFitKey!==key||el.__dashComplimentFit!==fit)return;
    const observed=complimentRenderedOverflow(el,fit);
    if(!observed.overflow){fit.renderVerified=true;noteComplimentFitClipped(fit);cacheComplimentFit(key,fit);return;}
    if(Number(fit.correctionAttempts)||0){fit.renderVerified=true;fit.fits=false;noteComplimentFitClipped(fit);cacheComplimentFit(key,fit);return;}
    complimentCorrectRenderedFit(el,key,fit,metrics,lite,observed.lines);
  };
  if(typeof requestAnimationFrame==="function")requestAnimationFrame(verify);else setTimeout(verify,0);
}
function complimentApplyFitAndVerify(el,key,fit,metrics,lite){
  el.__dashComplimentFitKey=key;el.__dashComplimentFit=fit;
  applyComplimentFit(el,fit,metrics);complimentScheduleRenderedVerification(el,key,fit,metrics,lite);
}
function fitCompliment(rawText){
  const el=document.getElementById("comptext");if(!el)return;
  const text=complimentCleanText(rawText===undefined?(el.__dashComplimentRawText||el.textContent||""):rawText);
  el.__dashComplimentRawText=text;
  // Never let a previous message's inline size determine the next fit.
  resetComplimentFit(el);
  const lite=complimentLiteProfile(),metrics=lite?complimentLiteMetricsForFit(el):complimentBoxMetrics(el);
  const baseline=lite?complimentLiteFit(text,metrics):null;
  const start=lite?baseline.size:complimentStartSizeFor(text,metrics);
  const floor=lite?baseline.preferredFloor:complimentFloorSize(metrics);
  const key=complimentFitKey(text,start,floor,lite,metrics),cached=COMP_FIT_CACHE.get(key);
  if(cached){el.textContent=cached.displayText||text;complimentApplyFitAndVerify(el,key,cached,metrics,lite);return;}
  const assess=(displayText,candidate)=>{
    const plannedLines=Math.max(1,Number(candidate&&candidate.lines)||1);
    if(lite)return complimentLiteFit(displayText,typeof complimentLiteMetricsForLines==="function"?complimentLiteMetricsForLines(metrics,plannedLines):metrics,plannedLines);
    if(plannedLines>1)el.dataset.fitLines=String(plannedLines);else delete el.dataset.fitLines;
    const candidateMetrics=complimentBoxMetrics(el),candidateStart=complimentStartSizeFor(displayText,candidateMetrics,plannedLines),candidateFloor=complimentFloorSize(candidateMetrics);
    el.textContent=displayText;return measureComplimentFit(el,displayText,candidateStart,candidateFloor,candidateMetrics,plannedLines);
  };
  const decision=typeof complimentLayoutChoose==="function"?complimentLayoutChoose(text,assess):{candidate:{displayText:text,kind:"single"},fit:assess(text)};
  const fit={...(decision.fit||baseline||{}),displayText:(decision.candidate&&decision.candidate.displayText)||text,layout:(decision.candidate&&decision.candidate.kind)||"single"};
  el.textContent=fit.displayText;cacheComplimentFit(key,fit);complimentApplyFitAndVerify(el,key,fit,metrics,lite);
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
