function scheduleNextCompliment(text, overrideMs){
  if(_compRotateTimer){ clearTimeout(_compRotateTimer); _compRotateTimer=null; }
  const baseDelay=Number.isFinite(+overrideMs) ? Math.max(500,+overrideMs) : complimentDelayMs(text);
  const delay=clockSafeComplimentDelay(baseDelay);
  _compRotateTimer=setTimeout(rotateCompliment,delay);
}
function showInitialCompliment(){
  const t=$("#comptext");
  if(!t) return;
  if(typeof compApplyFadeSteps==="function")compApplyFadeSteps(complimentFadeDelayMs());
  if(typeof compProbePresentBudget==="function")compProbePresentBudget();
  const chosen=pickReadableCompliment();
  const text=chosen?chosen.text:"";
  fitCompliment(text,{priority:!!(chosen&&chosen.priority)});
  if(chosen)recordMessagePick(chosen);
  scheduleNextCompliment(text);
}
function rotateCompliment(){
  if(complimentsPaused()){ pauseComplimentMotion(); scheduleNextCompliment(_lastCompText||"",5000); return; }
  const t=$("#comptext");
  if(!t) return;
  compClearFadeTimer();
  if(_compRotateTimer){ clearTimeout(_compRotateTimer); _compRotateTimer=null; }
  const fadeMs=complimentFadeDelayMs();
  // Heavy work runs while the old text is fully visible and static: the pick
  // carries its own bounded Canvas fit, and on Lite the display fit is pure
  // Canvas here, so a 30–80 ms stall never disturbs the visible line.
  const previousText=_lastCompText||"";
  const previousPriority=!!(_lastCompItem&&_lastCompItem.priority);
  const chosen=pickReadableCompliment();
  if(!chosen){ scheduleNextCompliment(previousText,5000); return; }
  const text=chosen.text,priority=!!chosen.priority;
  const planned=complimentLiteProfile()?computeComplimentFit(text,t,{priority}):null;
  const beginFadeIn=()=>{ if(complimentsPaused())return; compSetVisible(t,true); };
  const swap=()=>{
    compClearFadeTimer();
    if(complimentsPaused()){
      // Paused mid-fade: discard the pick, refit the previous text at its own priority, fade back.
      pauseComplimentMotion();
      fitCompliment(previousText,{priority:previousPriority});
      compSetVisible(t,true);
      scheduleNextCompliment(previousText,5000);
      return;
    }
    const result=planned||computeComplimentFit(text,t,{priority});
    applyComplimentFitResult(t,result,beginFadeIn);
    recordMessagePick(chosen);
    scheduleNextCompliment(text);
  };
  if(fadeMs<=0){ swap(); return; }
  compSetVisible(t,false);
  const doSwap=()=>{
    if(typeof requestAnimationFrame==="function") requestAnimationFrame(swap);
    else swap();
  };
  if(complimentLiteProfile()&&CONFIG.showSeconds&&typeof clockAfterTickAt==="function"){
    // Land the text swap on the next clock tick: the change shares the tick's
    // present instead of forcing its own, and the stepped fade-in starts on a
    // boundary-aligned frame.
    _compFadeTimer=clockAfterTickAt(Date.now()+fadeMs,doSwap);
  }else{
    _compFadeTimer=setTimeout(doSwap,fadeMs);
  }
}
