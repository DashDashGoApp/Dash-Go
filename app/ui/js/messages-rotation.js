function scheduleNextCompliment(text, overrideMs){
  if(_compRotateTimer){ clearTimeout(_compRotateTimer); _compRotateTimer=null; }
  const baseDelay=Number.isFinite(+overrideMs) ? Math.max(500,+overrideMs) : complimentDelayMs(text);
  const delay=clockSafeComplimentDelay(baseDelay);
  _compRotateTimer=setTimeout(rotateCompliment,delay);
}
function showInitialCompliment(){
  const t=$("#comptext");
  if(!t) return;
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
  if(_compFadeTimer){ clearTimeout(_compFadeTimer); _compFadeTimer=null; }
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
    _compFadeTimer=null;
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
  _compFadeTimer=setTimeout(()=>{
    if(typeof requestAnimationFrame==="function") requestAnimationFrame(swap);
    else swap();
  },fadeMs);
}
