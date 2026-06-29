// 07-compliments-00b-lite-fit.js — cached-geometry Canvas fitting for Lite.
// This path reads layout only after browser-reported geometry changes. Rotating a
// message uses an in-memory snapshot plus Canvas text metrics, never a DOM probe.
const COMP_LITE_GEOMETRY={metrics:null,revision:0,cacheKey:"",captureTimer:0,observer:null,target:null,parent:null,sun:null,stale:null,resizeBound:false,fontBound:false,observed:[]};
let COMP_LITE_CANVAS=null;
function complimentLiteTier(){return (document.documentElement&&document.documentElement.dataset&&document.documentElement.dataset.fit)||"base";}
function complimentLiteViewportMetrics(){
  // Emergency boot fallback only. A real cached snapshot replaces this on the
  // next animation frame / observer callback; normal rotation never reaches
  // this path after the dashboard has settled.
  const width=Math.max(320,Math.round(Number(window.innerWidth)||800)),height=Math.max(240,Math.round(Number(window.innerHeight)||480));
  const tier=complimentLiteTier();let band=tier==="min"?84:tier==="dense"?96:tier==="compact"?112:124;
  if(tier==="xl")band=Math.max(124,Math.min(200,Math.round(height*.12)));
  const outerWidth=Math.max(150,Math.min(Math.round(width*.82),width-360)),contentWidth=Math.max(120,outerWidth-48),contentHeight=Math.max(42,band-24);
  return {outerWidth,outerHeight:contentHeight,contentWidth,contentHeight,elPadV:0,elPadH:48,fontFamily:"sans-serif",fontWeight:"800",fontStyle:"normal",letterSpacing:0,tier,revision:0,cacheKey:"viewport:"+tier+":"+width+"x"+height};
}
function complimentLiteStyleNumber(style,property){return parseFloat(style&&style[property])||0;}
function complimentLiteCustomNumber(style,property){return parseFloat(style&&style.getPropertyValue&&style.getPropertyValue(property))||0;}
function complimentLiteSnapshotKey(metrics){
  return [Math.round(metrics.contentWidth),Math.round(metrics.contentHeight),metrics.fontFamily||"",metrics.fontWeight||"",metrics.fontStyle||"",Math.round((metrics.letterSpacing||0)*100)/100,metrics.tier||""].join("|");
}
function complimentLiteReadGeometry(el){
  const parent=el&&el.parentElement;
  if(!el||!parent)return null;
  const parentStyle=getComputedStyle(parent),elStyle=getComputedStyle(el);
  const parentPad=complimentLiteStyleNumber(parentStyle,"paddingTop")+complimentLiteStyleNumber(parentStyle,"paddingBottom");
  const actualElPadV=complimentLiteStyleNumber(elStyle,"paddingTop")+complimentLiteStyleNumber(elStyle,"paddingBottom");
  const basePadY=complimentLiteCustomNumber(elStyle,"--comptext-base-pad-y");
  // Keep the cached snapshot tied to the normal one-line inset. A selected
  // semantic layout may temporarily reclaim vertical padding, but that must not
  // poison the next rotation's cached geometry revision.
  const elPadV=basePadY>0?basePadY*2:actualElPadV;
  const elPadH=complimentLiteStyleNumber(elStyle,"paddingLeft")+complimentLiteStyleNumber(elStyle,"paddingRight");
  const outerWidth=Math.max(1,el.clientWidth||Math.round(el.getBoundingClientRect().width)||320);
  const outerHeight=Math.max(42,(parent.clientHeight||124)-parentPad);
  const metrics={outerWidth,outerHeight,contentWidth:Math.max(1,outerWidth-elPadH),contentHeight:Math.max(18,outerHeight-elPadV),elPadV,elPadH,fontFamily:elStyle.fontFamily||"sans-serif",fontWeight:elStyle.fontWeight||"800",fontStyle:elStyle.fontStyle||"normal",letterSpacing:complimentLiteStyleNumber(elStyle,"letterSpacing"),tier:complimentLiteTier()};
  metrics.cacheKey=complimentLiteSnapshotKey(metrics);
  return metrics;
}
function complimentLiteCommitGeometry(el){
  const next=complimentLiteReadGeometry(el);if(!next)return false;
  if(next.cacheKey===COMP_LITE_GEOMETRY.cacheKey)return false;
  COMP_LITE_GEOMETRY.revision++;
  next.revision=COMP_LITE_GEOMETRY.revision;
  next.cacheKey+="@"+next.revision;
  COMP_LITE_GEOMETRY.metrics=next;COMP_LITE_GEOMETRY.cacheKey=complimentLiteSnapshotKey(next);
  return true;
}
function complimentLiteCaptureNow(el){
  const changed=complimentLiteCommitGeometry(el);
  if(changed&&el&&el.textContent&&typeof fitCompliment==="function")fitCompliment();
  return changed;
}
function complimentLiteScheduleGeometryCapture(el,reason){
  if(!complimentLiteProfile())return;
  const target=el||COMP_LITE_GEOMETRY.target||document.getElementById("comptext");
  if(!target||COMP_LITE_GEOMETRY.captureTimer)return;
  const run=()=>{COMP_LITE_GEOMETRY.captureTimer=0;complimentLiteCaptureNow(target);};
  COMP_LITE_GEOMETRY.captureTimer=typeof requestAnimationFrame==="function"?requestAnimationFrame(run):setTimeout(run,0);
}
function complimentLiteDisconnectGeometryObserver(){
  if(COMP_LITE_GEOMETRY.observer)COMP_LITE_GEOMETRY.observer.disconnect();
  COMP_LITE_GEOMETRY.observer=null;COMP_LITE_GEOMETRY.observed=[];
}
function complimentLiteEnsureGeometry(el){
  if(!el)return;
  const parent=el.parentElement,sun=document.getElementById("sun"),stale=document.getElementById("stale");
  if(COMP_LITE_GEOMETRY.target!==el||COMP_LITE_GEOMETRY.parent!==parent){
    complimentLiteDisconnectGeometryObserver();
    COMP_LITE_GEOMETRY.target=el;COMP_LITE_GEOMETRY.parent=parent;COMP_LITE_GEOMETRY.sun=sun;COMP_LITE_GEOMETRY.stale=stale;
    if(typeof ResizeObserver==="function"){
      COMP_LITE_GEOMETRY.observer=new ResizeObserver(()=>complimentLiteScheduleGeometryCapture(el,"resize-observer"));
      // Do not observe #comptext itself: each rotating message changes its
      // intrinsic height, which would turn normal rotation back into layout work.
      for(const node of [parent,sun,stale])if(node){COMP_LITE_GEOMETRY.observer.observe(node);COMP_LITE_GEOMETRY.observed.push(node);}
    }
  }
  if(!COMP_LITE_GEOMETRY.resizeBound&&typeof window!=="undefined"){
    COMP_LITE_GEOMETRY.resizeBound=true;
    window.addEventListener("resize",()=>complimentLiteInvalidateGeometry("viewport"),{passive:true});
  }
  if(!COMP_LITE_GEOMETRY.fontBound&&document.fonts&&document.fonts.ready){
    COMP_LITE_GEOMETRY.fontBound=true;
    document.fonts.ready.then(()=>complimentLiteInvalidateGeometry("fonts")).catch(()=>{});
  }
  if(!COMP_LITE_GEOMETRY.metrics)complimentLiteScheduleGeometryCapture(el,"boot");
}
function complimentLiteMetricsForFit(el){
  complimentLiteEnsureGeometry(el);
  return COMP_LITE_GEOMETRY.metrics||complimentLiteViewportMetrics();
}
function complimentLitePrimeGeometry(){
  if(!complimentLiteProfile())return;
  const el=document.getElementById("comptext");if(el){complimentLiteEnsureGeometry(el);complimentLiteScheduleGeometryCapture(el,"boot");}
}
function complimentLiteInvalidateGeometry(reason){
  if(!complimentLiteProfile())return;
  COMP_LITE_GEOMETRY.metrics=null;COMP_LITE_GEOMETRY.cacheKey="";
  complimentLiteScheduleGeometryCapture(COMP_LITE_GEOMETRY.target||document.getElementById("comptext"),reason||"invalidate");
}
function complimentLiteNotifyLayoutChange(reason){
  if(!complimentLiteProfile())return;
  complimentLiteScheduleGeometryCapture(COMP_LITE_GEOMETRY.target||document.getElementById("comptext"),reason||"layout");
}
function complimentLiteCanvasContext(){
  if(COMP_LITE_CANVAS)return COMP_LITE_CANVAS;
  try{const canvas=document.createElement("canvas");COMP_LITE_CANVAS=canvas&&canvas.getContext&&canvas.getContext("2d");}catch(_){}
  return COMP_LITE_CANVAS;
}
function complimentLiteFont(metrics,size){return String(metrics.fontStyle||"normal")+" "+String(metrics.fontWeight||"800")+" "+Math.max(1,Math.round(size))+"px "+String(metrics.fontFamily||"sans-serif");}
function complimentLiteWidth(context,text,size,metrics){
  const chars=Array.from(String(text||"")).length;
  const measured=context?context.measureText(String(text||"")).width:chars*size*.52;
  return measured+Math.max(0,chars-1)*(metrics.letterSpacing||0);
}
function complimentLiteTokenChunks(context,token,size,metrics,maxWidth){
  if(complimentLiteWidth(context,token,size,metrics)<=maxWidth)return [token];
  const out=[];let current="";
  for(const glyph of Array.from(token)){
    const next=current+glyph;
    if(current&&complimentLiteWidth(context,next,size,metrics)>maxWidth){out.push(current);current=glyph;}else current=next;
  }
  if(current)out.push(current);
  return out.length?out:[token];
}
function complimentLiteLineCount(text,size,metrics,limit){
  const display=typeof complimentDisplayText==="function"?complimentDisplayText(text):String(text||"");if(!display)return 1;
  const context=complimentLiteCanvasContext();if(context)context.font=complimentLiteFont(metrics,size);
  const maxWidth=Math.max(1,metrics.contentWidth);let lines=0;
  const addSourceLine=source=>{
    let current="";lines++;if(lines>limit)return false;
    const add=part=>{
      const candidate=current?current+" "+part:part;
      if(complimentLiteWidth(context,candidate,size,metrics)<=maxWidth){current=candidate;return true;}
      if(current){lines++;current="";if(lines>limit)return false;}
      if(complimentLiteWidth(context,part,size,metrics)<=maxWidth){current=part;return true;}
      const chunks=complimentLiteTokenChunks(context,part,size,metrics,maxWidth);
      for(let i=0;i<chunks.length;i++){if(i>0){lines++;if(lines>limit)return false;}current=chunks[i];}
      return true;
    };
    for(const word of complimentCleanText(source).split(" "))if(word&&!add(word))return false;
    return true;
  };
  for(const source of display.split("\n"))if(!addSourceLine(source))return lines;
  return Math.max(1,lines);
}
function complimentLiteReadingFloors(metrics){
  const base={xl:[64,44,32],base:[48,34,24],compact:[40,28,20],dense:[34,24,18],min:[30,22,16]}[metrics.tier]||[48,34,24];
  const multiplier=complimentTypographyMultiplier();
  return base.map(value=>Math.max(COMP_FIT.hardFloor,Math.round(value*multiplier)));
}
function complimentLiteVisualCaps(metrics){
  const base={xl:92,base:72,compact:58,dense:48,min:40}[metrics.tier]||72;
  const multiplier=complimentTypographyMultiplier();
  return [base,base*.84,base*.70].map((value,index)=>{
    const lines=index+1,ratio=lines<=1?1.03:1.08;
    const reserve=complimentVerticalFitReserve(value*multiplier,lines,true);
    return Math.max(COMP_FIT.hardFloor,Math.floor(Math.min(value*multiplier,Math.max(18,metrics.contentHeight-reserve)/(lines*ratio))));
  });
}
function complimentLiteMetricsForLines(metrics,lines){
  if(!metrics||lines<=1)return metrics;
  const outer=Math.max(Number(metrics.outerHeight)||0,(Number(metrics.contentHeight)||0)+(Number(metrics.elPadV)||0));
  const compactPad=Math.min(Math.max(0,Number(metrics.elPadV)||0),12);
  if(!outer||compactPad>=Number(metrics.elPadV)||0)return metrics;
  return {...metrics,elPadV:compactPad,contentHeight:Math.max(18,outer-compactPad),layoutLines:lines};
}
function complimentLiteLargestForLines(text,metrics,target,floor,cap){
  let low=floor,high=Math.max(floor,cap),best=0;
  const fits=size=>{
    const lines=complimentLiteLineCount(text,size,metrics,target);
    const ratio=lines<=1?1.03:1.08;
    const available=Math.max(18,metrics.contentHeight-complimentVerticalFitReserve(size,lines,true));
    return lines<=target&&lines*size*ratio<=available;
  };
  if(!fits(low))return 0;
  while(low<=high){const mid=Math.floor((low+high)/2);if(fits(mid)){best=mid;low=mid+1;}else high=mid-1;}
  return best;
}
function complimentLiteFit(text,metrics){
  const floors=complimentLiteReadingFloors(metrics),caps=complimentLiteVisualCaps(metrics);
  for(let target=1;target<=3;target++){
    const floor=floors[target-1],size=complimentLiteLargestForLines(text,metrics,target,floor,caps[target-1]);
    if(size)return {size,lines:target,maxLines:3,fits:true,preferredFloor:floor,targetLines:target,box:metrics,lite:true};
  }
  // Exceptional content (very long pasted material or a hostile token) keeps
  // the tier's three-line reading floor where possible, then uses the existing
  // WebKit line clamp/ellipsis rather than silently shrinking ordinary prose.
  const preferredFloor=floors[2],reserve=complimentVerticalFitReserve(preferredFloor,3,true),emergency=Math.max(COMP_FIT.hardFloor,Math.min(preferredFloor,Math.floor(Math.max(18,metrics.contentHeight-reserve)/(3*1.08))));
  return {size:emergency,lines:3,maxLines:3,fits:false,preferredFloor,targetLines:3,box:metrics,lite:true};
}
