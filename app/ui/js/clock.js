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
let _lastTickMin=-1;
function tickClock(){
  if(CONFIG.pauseWhileOpen!==false && typeof uiOverlayActive === "function" && uiOverlayActive()) return;
  const d=new Date();
  // Build the time structure once, then only poke text nodes each tick.
  if(!_clockEls){
    const ct=$("#ctime"); ct.innerHTML='<span class="hm"></span><sup class="ss"></sup><span class="ap"></span>';
    _clockEls={ hm:ct.querySelector(".hm"), ss:ct.querySelector(".ss"), ap:ct.querySelector(".ap") };
    _lastTickMin=-1;
  }
  // Seconds hidden -> nothing on screen changes until the minute rolls over,
  // so skip ALL DOM writes (style/layout work) for 59 of every 60 ticks.
  const min=d.getHours()*60+d.getMinutes();
  if(!CONFIG.showSeconds && min===_lastTickMin) return;
  _lastTickMin=min;
  const parts=clockParts(d,CONFIG.clock24);
  _clockEls.hm.textContent=parts.hm;
  _clockEls.ss.textContent=CONFIG.showSeconds?String(d.getSeconds()).padStart(2,"0"):"";
  _clockEls.ap.textContent=parts.ap;
  // Date string changes once a day — only reformat when it actually differs.
  const ds=_dateFmt.format(d).toUpperCase();
  if(ds!==_lastDateStr){ _lastDateStr=ds; $("#cdate").textContent=ds; }
}

let _clockTimer=null;
function armClockTimer(){
  if(_clockTimer){ clearTimeout(_clockTimer); _clockTimer=null; }
  const run=()=>{
    tickClock();
    const ms=CONFIG.showSeconds ? 1000 : Math.max(250,60000-(Date.now()%60000)+25);
    _clockTimer=setTimeout(run,ms);
  };
  run();
}
