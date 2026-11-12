// 07-clock-compliments.js — generated from dashboard.js for maintainability.
/* =====================================================================
   ============================  CLOCK  ================================
   ===================================================================== */
let _clockEls=null, _lastDateStr="";
const _dateFmt=new Intl.DateTimeFormat(LOCALE,{weekday:"short",month:"short",day:"numeric",year:"numeric"});
// Pure helper (testable): big-clock pieces for either format.
function clockParts(d,clock24){
  const mm=String(d.getMinutes()).padStart(2,"0");
  if(clock24) return { hm:String(d.getHours()).padStart(2,"0")+":"+mm, ap:"" };
  let h=d.getHours()%12; if(h===0) h=12;
  return { hm:h+":"+mm, ap:d.getHours()<12?"AM":"PM" };
}
// Write-on-change text helper: keeps a tick that did not change a glyph from
// touching the DOM at all, and writes changed glyphs through the existing
// text node instead of replacing node children.
function clockWriteText(node,value){
  if(!node)return;
  if(node.__lastText===value)return;
  node.__lastText=value;
  if(node.firstChild&&node.firstChild.nodeType===3)node.firstChild.nodeValue=value;
  else node.textContent=value;
}
let _lastTickMin=-1;
function tickClock(){
  if(CONFIG.pauseWhileOpen!==false && typeof uiOverlayActive === "function" && uiOverlayActive()) return;
  const d=new Date();
  // Build the time structure once, then only poke text nodes each tick.
  if(!_clockEls){
    const ct=$("#ctime");
    if(!ct) return;
    ct.innerHTML='<span class="hm"></span><sup class="ss"></sup><span class="ap"></span>';
    _clockEls={ hm:ct.querySelector(".hm"), ss:ct.querySelector(".ss"), ap:ct.querySelector(".ap") };
    _lastTickMin=-1;
    _lastTickClockParts={hm:"",ss:"",ap:""};
  }
  // Seconds hidden -> nothing on screen changes until the minute rolls over,
  // so skip ALL DOM writes (style/layout work) for 59 of every 60 ticks.
  const min=d.getHours()*60+d.getMinutes();
  if(!CONFIG.showSeconds && min===_lastTickMin) return;
  _lastTickMin=min;
  const parts=clockParts(d,CONFIG.clock24);
  const ss=CONFIG.showSeconds?String(d.getSeconds()).padStart(2,"0"):"";
  // hm/ap change once a minute, ss once a second: write only the pieces that
  // actually moved so an unchanged piece costs nothing.
  clockWriteText(_clockEls.hm,parts.hm);
  clockWriteText(_clockEls.ss,ss);
  clockWriteText(_clockEls.ap,parts.ap);
  _lastTickClockParts={hm:parts.hm,ss:ss,ap:parts.ap};
  // Date string changes once a day — only reformat when it actually differs.
  const ds=_dateFmt.format(d).toUpperCase();
  if(ds!==_lastDateStr){
    const dateNode=$("#cdate");
    if(dateNode){ _lastDateStr=ds; dateNode.textContent=ds; }
  }
  clockRunTickHooks(d.getTime());
}
let _lastTickClockParts={hm:"",ss:"",ap:""};
// One-shot hooks that fire on the first clock tick at or after `afterMs`.
// The message rotation uses this to land the text swap on the tick's own
// present instead of forcing a separate one. The returned handle is cleared
// through compClearFadeTimer() so the tick path and the fallback timeout can
// never double-fire.
let _clockTickHooks=[];
function clockAfterTickAt(afterMs,fn){
  let fired=false;
  const entry={
    afterMs,
    run(){
      if(fired)return;
      fired=true;
      const i=_clockTickHooks.indexOf(entry);
      if(i>=0)_clockTickHooks.splice(i,1);
      try{ fn(); }catch(e){ console.warn("clock tick hook failed",e); }
    },
    cancel(){
      if(fired)return;
      fired=true;
      const i=_clockTickHooks.indexOf(entry);
      if(i>=0)_clockTickHooks.splice(i,1);
      if(entry.timer){ clearTimeout(entry.timer); entry.timer=null; }
    }
  };
  _clockTickHooks.push(entry);
  // Fallback for a stalled tick chain (long page stall, throttled tab): the
  // swap still happens, just unaligned.
  entry.timer=setTimeout(entry.run,Math.max(100,(Number(afterMs)||Date.now())-Date.now())+3000);
  return entry;
}
function clockRunTickHooks(nowMs){
  if(!_clockTickHooks.length)return;
  const due=_clockTickHooks.filter(h=>h.afterMs<=nowMs);
  if(!due.length)return;
  due.forEach(h=>h.run());
}
let _clockTimer=null;
function armClockTimer(){
  if(_clockTimer){ clearTimeout(_clockTimer); _clockTimer=null; }
  const run=()=>{
    tickClock();
    // Schedule each tick at the wall-clock second boundary (+15 ms so the
    // update lands after the second has actually changed). The chain
    // recomputes from Date.now() every tick, so it self-corrects after
    // stalls, NTP steps, and overlay pauses instead of drifting a few ms per
    // tick and wrapping out of phase.
    const now=Date.now();
    const ms=CONFIG.showSeconds ? 1000-(now%1000)+15 : Math.max(250,60000-(now%60000)+25);
    _clockTimer=setTimeout(run,ms);
  };
  run();
}
// Re-arm the tick chain after an overlay pause so the next tick lands on a
// boundary again; the immediate run shows the current time at once.
function clockResyncTimer(){ armClockTimer(); }
