/* =====================================================================
   ================  IDLE SCROLL RETURN CONTROLLER  ====================
   Known dashboard roots only. One bounded controller owns user input,
   idle rearming, return verification, and the one numeric fallback when
   WebKitGTK declines or interrupts a native smooth return. Raw scroll work
   is intentionally limited to scrollTop and controller state.
   ===================================================================== */
const CALENDAR_SCROLL_HOME={top:0,ready:false,lastInputAt:0,suspended:false};
const DASH_SCROLL_RETURN_PREWARM_MS=400;
const DASH_SCROLL_RETURN_VERIFY_MS=700;
const DASH_SCROLL_RETURN_INPUT_GRACE_MS=140;
const DASH_SCROLL_RETURN_CONTROLLERS=new WeakMap();
let _calendarScrollSnapSuspend=null;
let _calendarScrollSnapReconcile=null;
function setCalendarScrollHomeTop(top){
  const value=Number(top);
  CALENDAR_SCROLL_HOME.top=Number.isFinite(value)&&value>=0?value:0;
  CALENDAR_SCROLL_HOME.ready=true;
}
function calendarScrollHomeTop(){return CALENDAR_SCROLL_HOME.top;}
function calendarScrollSnapSuspend(next){
  CALENDAR_SCROLL_HOME.suspended=!!next;
  if(typeof _calendarScrollSnapSuspend==="function")_calendarScrollSnapSuspend(!!next);
}
function calendarScrollSnapReconcile(){
  if(typeof _calendarScrollSnapReconcile==="function")_calendarScrollSnapReconcile();
}
function dashboardScrollReturnDelay(){
  const seconds=Number(CONFIG.snapBackSeconds);
  return Math.max(1,Number.isFinite(seconds)&&seconds>0?seconds:35)*1000;
}
function dashboardScrollReturnDefer(fn){
  return typeof requestAnimationFrame==="function"?requestAnimationFrame(fn):setTimeout(fn,0);
}
function dashboardScrollReturnCancel(state,name){
  if(state[name])clearTimeout(state[name]);
  state[name]=0;
}
function dashboardScrollReturnCreate(root,options){
  if(!root)return null;
  const old=DASH_SCROLL_RETURN_CONTROLLERS.get(root);
  if(old)return old;
  const opts=options||{};
  const state={
    root,opts,away:false,suspended:false,returning:false,prewarmed:false,
    lastInputAt:0,idleTimer:0,prewarmTimer:0,verifyTimer:0,
    onScroll:null,onInput:null,onRelease:null,onVisibility:null
  };
  const tolerance=()=>Math.max(0,Number(opts.tolerance)||4);
  const homeReady=()=>typeof opts.homeReady!=="function"||!!opts.homeReady();
  const homeTop=()=>Math.max(0,Number(typeof opts.homeTop==="function"?opts.homeTop():0)||0);
  const atHome=()=>!homeReady()||Math.abs((Number(root.scrollTop)||0)-homeTop())<tolerance();
  const setAway=next=>{
    const value=!!next;
    if(state.away===value)return;
    state.away=value;
    if(typeof opts.onAway==="function")opts.onAway(value);
  };
  const clearPrewarm=()=>{
    if(!state.prewarmed)return;
    state.prewarmed=false;
    if(typeof opts.clearPrewarm==="function")opts.clearPrewarm();
  };
  const clearTimers=()=>{
    dashboardScrollReturnCancel(state,"idleTimer");
    dashboardScrollReturnCancel(state,"prewarmTimer");
  };
  const clearVerification=()=>dashboardScrollReturnCancel(state,"verifyTimer");
  const distance=()=>Math.abs((Number(root.scrollTop)||0)-homeTop());
  const isFar=()=>typeof opts.isFar==="function"&&!!opts.isFar(distance());
  const refreshAway=()=>{
    const next=!atHome();
    setAway(next);
    return next;
  };
  const finishReturn=()=>{
    clearVerification();
    state.returning=false;
    clearPrewarm();
    const away=refreshAway();
    if(away&&!state.suspended)arm();
  };
  const directReturn=()=>{
    const top=homeTop();
    root.scrollTop=top;
    if(typeof opts.afterDirectReturn==="function")opts.afterDirectReturn(top);
    dashboardScrollReturnDefer(()=>{
      if(atHome())finishReturn();
      else{
        state.returning=false;
        clearPrewarm();
        reconcile();
      }
    });
  };
  const verifyReturn=()=>{
    state.verifyTimer=0;
    if(state.suspended||!state.returning)return;
    if(atHome()){finishReturn();return;}
    // Native smooth scrolling was interrupted or never started. One direct
    // numeric fallback is deterministic and avoids leaving an unarmed list.
    directReturn();
  };
  const startReturn=(manual)=>{
    clearTimers();
    if(state.suspended||atHome()){finishReturn();return;}
    if(!manual&&Date.now()-state.lastInputAt<DASH_SCROLL_RETURN_INPUT_GRACE_MS){reconcile();return;}
    if(!manual&&isFar()&&!state.prewarmed&&typeof opts.prewarm==="function"){
      opts.prewarm(homeTop());
      state.prewarmed=true;
      state.idleTimer=setTimeout(()=>startReturn(false),DASH_SCROLL_RETURN_PREWARM_MS);
      return;
    }
    state.returning=true;
    const top=homeTop();
    if(!manual&&isFar()){
      directReturn();
      return;
    }
    if(typeof root.scrollTo==="function")root.scrollTo({top,behavior:"smooth"});
    else root.scrollTop=top;
    state.verifyTimer=setTimeout(verifyReturn,DASH_SCROLL_RETURN_VERIFY_MS);
  };
  const prewarm=()=>{
    state.prewarmTimer=0;
    if(state.suspended||state.returning||atHome()||!isFar()||typeof opts.prewarm!=="function")return;
    opts.prewarm(homeTop());
    state.prewarmed=true;
  };
  const arm=()=>{
    clearTimers();
    if(state.suspended||state.returning||!refreshAway())return;
    const delay=dashboardScrollReturnDelay();
    state.idleTimer=setTimeout(()=>startReturn(false),delay);
    if(typeof opts.prewarm==="function"&&isFar()){
      const lead=Math.min(DASH_SCROLL_RETURN_PREWARM_MS,Math.max(1,delay-1));
      state.prewarmTimer=setTimeout(prewarm,Math.max(0,delay-lead));
    }
  };
  const cancelForInput=()=>{
    state.lastInputAt=Date.now();
    CALENDAR_SCROLL_HOME.lastInputAt=state.lastInputAt;
    state.returning=false;
    clearTimers();
    clearVerification();
    clearPrewarm();
    refreshAway();
  };
  const reconcile=()=>{
    const away=refreshAway();
    if(!away){
      clearTimers();
      if(state.returning)finishReturn();
      return;
    }
    if(!state.suspended&&!state.returning)arm();
  };
  const setSuspended=next=>{
    state.suspended=!!next;
    if(state.suspended){
      state.returning=false;
      clearTimers();
      clearVerification();
      clearPrewarm();
      return;
    }
    reconcile();
  };
  state.onScroll=()=>{
    if(state.returning){
      if(atHome())finishReturn();
      else refreshAway();
      return;
    }
    reconcile();
  };
  state.onInput=cancelForInput;
  state.onRelease=()=>{if(!state.suspended)reconcile();};
  root.addEventListener("pointerdown",state.onInput,{passive:true});
  root.addEventListener("touchstart",state.onInput,{passive:true});
  root.addEventListener("wheel",state.onInput,{passive:true});
  for(const name of ["pointerup","pointercancel","lostpointercapture","touchend","touchcancel"]){
    root.addEventListener(name,state.onRelease,{passive:true});
  }
  root.addEventListener("scroll",state.onScroll,{passive:true});
  if(typeof document!=="undefined"&&document.addEventListener){
    state.onVisibility=()=>{
      if(document.visibilityState==="hidden"){
        clearTimers();
        clearVerification();
        clearPrewarm();
        return;
      }
      reconcile();
    };
    document.addEventListener("visibilitychange",state.onVisibility,{passive:true});
  }
  const controller={
    state,
    reconcile,
    setSuspended,
    returnNow:()=>startReturn(true),
    destroy:()=>{
      clearTimers();clearVerification();clearPrewarm();
      root.removeEventListener("pointerdown",state.onInput);
      root.removeEventListener("touchstart",state.onInput);
      root.removeEventListener("wheel",state.onInput);
      for(const name of ["pointerup","pointercancel","lostpointercapture","touchend","touchcancel"]){
        root.removeEventListener(name,state.onRelease);
      }
      root.removeEventListener("scroll",state.onScroll);
      if(state.onVisibility&&typeof document!=="undefined")document.removeEventListener("visibilitychange",state.onVisibility);
      DASH_SCROLL_RETURN_CONTROLLERS.delete(root);
    }
  };
  DASH_SCROLL_RETURN_CONTROLLERS.set(root,controller);
  return controller;
}
function scrollIdleReturnReconcile(root){
  const controller=root&&DASH_SCROLL_RETURN_CONTROLLERS.get(root);
  if(controller)controller.reconcile();
}
(function(){
  const scroll=$("#calscroll"),btn=$("#snapback");
  if(!scroll)return;
  const controller=dashboardScrollReturnCreate(scroll,{
    tolerance:8,
    homeReady:()=>CALENDAR_SCROLL_HOME.ready,
    homeTop:calendarScrollHomeTop,
    onAway:next=>btn&&btn.classList&&btn.classList.toggle("show",next),
    isFar:distance=>{
      const height=typeof calendarWeekCullViewportHeight==="function"?calendarWeekCullViewportHeight():0;
      return height>0&&distance>height*1.5;
    },
    prewarm:top=>{if(typeof calendarWeekCullPrewarmAt==="function")calendarWeekCullPrewarmAt(top,{before:2,after:2});},
    clearPrewarm:()=>{if(typeof calendarWeekCullClearPrewarm==="function")calendarWeekCullClearPrewarm();},
    afterDirectReturn:top=>{if(typeof calendarWeekCullCommitAt==="function")calendarWeekCullCommitAt(top);}
  });
  _calendarScrollSnapSuspend=next=>controller.setSuspended(next);
  _calendarScrollSnapReconcile=()=>controller.reconcile();
  if(btn)btn.addEventListener("click",()=>controller.returnNow());
  controller.reconcile();
})();
(function(){
  for(const selector of ["#agendalist","#wx14"]){
    const root=$(selector);
    if(root)dashboardScrollReturnCreate(root,{homeTop:()=>0,tolerance:4});
  }
})();
