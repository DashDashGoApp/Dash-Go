function scheduleNextCompliment(text, overrideMs){
  if(_compRotateTimer){ clearTimeout(_compRotateTimer); _compRotateTimer=null; }
  const baseDelay=Number.isFinite(+overrideMs) ? Math.max(500,+overrideMs) : complimentDelayMs(text);
  const delay=clockSafeComplimentDelay(baseDelay);
  _compRotateTimer=setTimeout(rotateCompliment,delay);
}
function showInitialCompliment(){
  const t=$("#comptext");
  if(!t) return;
  const text=pickCompliment();
  fitCompliment(text);
  scheduleNextCompliment(text);
}
function rotateCompliment(){
  if(complimentsPaused()){ pauseComplimentMotion(); scheduleNextCompliment(_lastCompText||"",5000); return; }
  const t=$("#comptext");
  if(!t) return;
  if(_compFadeTimer){ clearTimeout(_compFadeTimer); _compFadeTimer=null; }
  if(_compRotateTimer){ clearTimeout(_compRotateTimer); _compRotateTimer=null; }
  const fadeMs=complimentFadeDelayMs();
  const swap=()=>{
    _compFadeTimer=null;
    if(complimentsPaused()){ fitCompliment(); t.style.opacity=1; scheduleNextCompliment(_lastCompText||"",5000); return; }
    const text=pickCompliment();
    fitCompliment(text);
    t.style.opacity=1;
    scheduleNextCompliment(text);
  };
  if(fadeMs<=0){ swap(); return; }
  t.style.opacity=0;
  _compFadeTimer=setTimeout(()=>{
    if(typeof requestAnimationFrame==="function") requestAnimationFrame(swap);
    else swap();
  },fadeMs);
}
