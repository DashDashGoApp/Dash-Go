// 00-tap.js — one pointer-first tap primitive for kiosk touch, pen, and mouse.
// Public binding signatures stay stable so existing call sites remain small.
// One document-level visibility listener plus one DOM-removal observer owns
// gesture cleanup. bindTap() is used on controls rebuilt by every app render;
// registering document listeners per control would otherwise retain those
// detached controls for the life of the kiosk session.
const DASHGO_TAP_BINDINGS=new Set();
let DASHGO_TAP_VISIBILITY_READY=false;
let DASHGO_TAP_REMOVAL_OBSERVER=null;
function pruneDashGoTapBindings(){
  for(const binding of DASHGO_TAP_BINDINGS){
    if(!binding||binding.disposed||binding.elm?.isConnected===false)DASHGO_TAP_BINDINGS.delete(binding);
  }
}
function resetDashGoTapBindings(){
  for(const binding of DASHGO_TAP_BINDINGS){
    if(!binding||binding.disposed||binding.elm?.isConnected===false){DASHGO_TAP_BINDINGS.delete(binding);continue;}
    binding.cancel();
  }
}
function ensureDashGoTapCleanup(){
  if(!DASHGO_TAP_VISIBILITY_READY){
    DASHGO_TAP_VISIBILITY_READY=true;
    document.addEventListener("visibilitychange",()=>{if(document.hidden)resetDashGoTapBindings();});
  }
  if(!DASHGO_TAP_REMOVAL_OBSERVER&&typeof MutationObserver==="function"){
    const target=document.documentElement||document.body;
    if(target){
      DASHGO_TAP_REMOVAL_OBSERVER=new MutationObserver(()=>pruneDashGoTapBindings());
      DASHGO_TAP_REMOVAL_OBSERVER.observe(target,{childList:true,subtree:true});
    }
  }
}

function attachTaps(elm,opts){
  if(!elm) return ()=>{};
  opts=opts||{};
  const gap=Math.max(160,Number(opts.gap)||500);
  // moveTol applies only inside one press/release cycle. Keep it tight so a
  // drag or swipe never becomes a tap merely because the control is large.
  const moveTol=Math.max(4,Number(opts.moveTol)||24);
  // Consecutive taps belong to one gesture when they occur on this bound
  // element within the gap. A wide control must accept taps anywhere on its
  // surface, so inter-tap clustering is floored to the element diagonal.
  // attachTaps receives events only from elm (or its descendants), making the
  // wider cluster radius safe. opts.clusterTol remains an explicit override.
  const clusterTol=()=>{
    const rect=elm.getBoundingClientRect&&elm.getBoundingClientRect();
    const diagonal=rect?Math.hypot(Number(rect.width)||0,Number(rect.height)||0):0;
    return Math.max(moveTol,Number(opts.clusterTol)||0,diagonal);
  };
  const holdMax=Math.max(250,Number(opts.holdMax)||1200);
  const need=Math.max(1,Number(opts.maxTaps)||1);
  let count=0,lastAt=0,lastX=0,lastY=0;
  let downAt=0,downX=0,downY=0,ignoreDown=false,pointerActive=false;
  let suppressClickUntil=0;
  const reset=()=>{ count=0; lastAt=0; lastX=0; lastY=0; };
  const isTapDisabled=node=>!!(node&&(
    node.disabled===true ||
    node.getAttribute?.("aria-disabled")==="true" ||
    node.matches?.(":disabled")
  ));
  const cancelGesture=()=>{
    ignoreDown=true;
    pointerActive=false;
    downAt=0; downX=0; downY=0;
    reset();
    suppressClickUntil=0;
  };
  const ignored=e=>{
    try{ return typeof opts.ignore==="function" && !!opts.ignore(e); }
    catch(_){ return false; }
  };
  const register=(x,y,event,source)=>{
    const now=Date.now();
    const px=Number.isFinite(+x)?+x:0, py=Number.isFinite(+y)?+y:0;
    const inWindow=now-lastAt<=gap && Math.hypot(px-lastX,py-lastY)<=clusterTol();
    count=inWindow?count+1:1;
    lastAt=now; lastX=px; lastY=py;
    if(typeof opts.onAnyTap==="function"){
      try{ opts.onAnyTap(count,{x:px,y:py,event,source:source||"tap"}); }catch(_){}
    }
    if(count>=need){
      reset();
      try{ if(typeof opts.onTaps==="function") opts.onTaps(event); }catch(e){ setTimeout(()=>{ throw e; },0); }
    }
  };
  const validRelease=(x,y)=>Date.now()-downAt<=holdMax && Math.hypot((+x||0)-downX,(+y||0)-downY)<=moveTol;
  const isPrimaryPointer=e=>!e || (e.isPrimary!==false && (e.button==null || e.button===0));
  const onPointerDown=e=>{
    // Treat one primary pointer stream as the authoritative gesture. Some
    // WebKitGTK/Surf touch stacks expose a real touch as a mouse-compatible
    // PointerEvent and then omit or delay the follow-up click. Do not make
    // controls depend on that synthetic click.
    if(!isPrimaryPointer(e) || isTapDisabled(elm) || ignored(e)){
      cancelGesture();
      return;
    }
    ignoreDown=false;
    pointerActive=true;
    downAt=Date.now(); downX=e.clientX||0; downY=e.clientY||0;
  };
  const onPointerUp=e=>{
    if(ignoreDown || !isPrimaryPointer(e) || isTapDisabled(elm) || ignored(e)){
      cancelGesture();
      return;
    }
    if(!validRelease(e.clientX,e.clientY)){
      cancelGesture();
      return;
    }
    // Commit every valid primary PointerEvent on release, including
    // mouse-compatible touch streams. The resulting browser click (if any) is
    // scoped to this one bound element and suppressed below, so actions such
    // as Close run exactly once. Keyboard/programmatic clicks still have no
    // preceding pointer release and continue through onClick.
    suppressClickUntil=Date.now()+700;
    // A bound summary owns its <details> state through register(). Cancel the
    // native toggle path for every pointer kind, including mouse-compatible
    // touchscreen streams; otherwise its follow-up click re-closes the card.
    if((opts.preventDefault || e.pointerType==="touch" || e.pointerType==="pen") && e.cancelable) e.preventDefault();
    // Some touch stacks emit lostpointercapture after a successful release. It
    // must not clear the duplicate window after this committed gesture.
    pointerActive=false;
    downAt=0;
    register(e.clientX,e.clientY,e,e.pointerType||"pointer");
  };
  const onPointerCancel=()=>{ if(pointerActive) cancelGesture(); };
  const onTouchStart=e=>{
    if(isTapDisabled(elm) || ignored(e)){
      cancelGesture();
      return;
    }
    ignoreDown=false;
    pointerActive=true;
    const t=e.touches&&e.touches[0]; if(!t){ cancelGesture(); return; }
    downAt=Date.now(); downX=t.clientX||0; downY=t.clientY||0;
  };
  const onTouchEnd=e=>{
    if(ignoreDown || isTapDisabled(elm) || ignored(e)){
      cancelGesture();
      return;
    }
    const t=e.changedTouches&&e.changedTouches[0];
    if(!t || !validRelease(t.clientX,t.clientY)){
      cancelGesture();
      return;
    }
    suppressClickUntil=Date.now()+700;
    if(e.cancelable) e.preventDefault();
    pointerActive=false;
    downAt=0;
    register(t.clientX,t.clientY,e,"touch");
  };
  const onClick=e=>{
    if(!isPrimaryPointer(e) || isTapDisabled(elm)) return;
    // Surf/WebKitGTK can report a follow-up mouse-compatible touch click with
    // detail 0. Once a real pointer release has committed, suppress every click
    // on this exact bound control during the short duplicate window. Keyboard
    // and programmatic activation remain available whenever no pointer release
    // is pending, which is the only reliable distinction across kiosk stacks.
    const duplicate=Date.now()<suppressClickUntil;
    // Suppressing duplicate handler work is not sufficient for native controls
    // such as <summary>: their click default still toggles <details>. Prevent
    // that default before returning so one pointer release yields one toggle.
    if(opts.preventDefault && e.cancelable) e.preventDefault();
    if(duplicate || ignored(e)) return;
    register(e.clientX,e.clientY,e,"click");
  };
  if(window.PointerEvent){
    elm.addEventListener("pointerdown",onPointerDown,{passive:true});
    elm.addEventListener("pointerup",onPointerUp,{passive:false});
    elm.addEventListener("pointercancel",onPointerCancel,{passive:true});
    elm.addEventListener("lostpointercapture",onPointerCancel,{passive:true});
  }else{
    elm.addEventListener("touchstart",onTouchStart,{passive:true});
    elm.addEventListener("touchend",onTouchEnd,{passive:false});
    elm.addEventListener("touchcancel",onPointerCancel,{passive:true});
  }
  elm.addEventListener("click",onClick);
  const binding={elm,disposed:false,cancel:cancelGesture};
  DASHGO_TAP_BINDINGS.add(binding);
  ensureDashGoTapCleanup();
  return ()=>{
    binding.disposed=true;
    DASHGO_TAP_BINDINGS.delete(binding);
    if(window.PointerEvent){
      elm.removeEventListener("pointerdown",onPointerDown);
      elm.removeEventListener("pointerup",onPointerUp);
      elm.removeEventListener("pointercancel",onPointerCancel);
      elm.removeEventListener("lostpointercapture",onPointerCancel);
    }
    else {
      elm.removeEventListener("touchstart",onTouchStart);
      elm.removeEventListener("touchend",onTouchEnd);
      elm.removeEventListener("touchcancel",onPointerCancel);
    }
    elm.removeEventListener("click",onClick);
  };
}
function bindTap(elm,fn,opts){
  return attachTaps(elm,{...(opts||{}),maxTaps:1,onTaps:fn});
}
function bindTripleTap(elm,fn,gap,opts){
  if(gap && typeof gap==="object"){ opts=gap; gap=opts.gap; }
  opts=opts||{};
  return attachTaps(elm,{...opts,maxTaps:3,gap:gap||650,onTaps:fn,onAnyTap:(n,meta)=>{
    if(n===1 && typeof opts.onFirstTap==="function"){ try{ opts.onFirstTap(meta); }catch(_){} }
    if(typeof opts.onAnyTap==="function"){ try{ opts.onAnyTap(n,meta); }catch(_){} }
  }});
}
function bindSingleDoubleTap(elm,singleFn,doubleFn,gap,opts){
  const wait=Math.max(180,Number(gap)||460);
  let timer=null;
  return attachTaps(elm,{...(opts||{}),maxTaps:2,gap:wait,onAnyTap:n=>{
    if(n!==1) return;
    clearTimeout(timer);
    timer=setTimeout(()=>{ timer=null; if(typeof singleFn==="function") singleFn(); },wait+40);
  },onTaps:()=>{
    clearTimeout(timer); timer=null;
    if(typeof doubleFn==="function") doubleFn();
  }});
}
