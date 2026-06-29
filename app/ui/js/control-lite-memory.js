// 09-control-14-lite-memory.js — bounded Control-page retention on Lite.
// The cache budget uses the existing /api/status snapshot: no interval, no
// background pressure poll. Control open reuses a cached snapshot first, then may make one delayed, abortable
// status request no more often than every 90 seconds while the panel is used.
let CTRL_PAGE_LRU=[];
const CTRL_CACHE_BUDGET_POLL_MS=90000;
const CTRL_CACHE_BUDGET={tier:"conservative",memAvailMB:0,swapUsedMB:0,previousSwapUsedMB:null,observedAt:0,lastProbeAt:0};
const CTRL_RETAINABLE_LITE_PAGES=new Set(["overview","display","calendars","control"]);
const CTRL_HEAVY_LAZY_KEYS=new Set(["profile","mapcache","content","builtins","birthdays","celebrations","feeds","sources","tempmsg","schedmsg","update","systemupdate","terminal","history","diagnostics","logs"]);

function ctrlAfterNextPaint(){return new Promise(resolve=>{if(typeof ctrlAfterPaint==="function")ctrlAfterPaint(resolve);else setTimeout(resolve,0);});}
function ctrlBudgetNumber(value){const n=Number(value);return Number.isFinite(n)&&n>=0?Math.round(n):null;}
function ctrlCacheBudgetTierFor(status,previousSwapUsedMB){
  const available=ctrlBudgetNumber(status&&status.mem_avail_mb),swapUsed=ctrlBudgetNumber(status&&status.swap_used_mb);
  if(available==null)return "conservative";
  const swapRising=swapUsed!=null&&previousSwapUsedMB!=null&&swapUsed>=previousSwapUsedMB+8;
  if(available<120||swapRising)return "conservative";
  if(available>=170)return "roomy";
  return "normal";
}
function ctrlHibernatedPageLimit(){
  if(!ctrlLiteProfile())return Number.MAX_SAFE_INTEGER;
  return CTRL_CACHE_BUDGET.tier==="roomy"?2:1;
}
function ctrlPageName(page){return String((page&&page.id)||"").replace("ctrlpage-","");}
function ctrlPageHasHeavyResidentContent(page){
  if(!page)return true;
  for(const d of page.querySelectorAll("details.ctrlsec[data-lazy]")){
    if(d.dataset&&d.dataset.loaded==="1"&&CTRL_HEAVY_LAZY_KEYS.has(d.dataset.lazy||""))return true;
  }
  return !!page.querySelector(".ctrloutputconsole[style*='block']");
}
function ctrlPageCanHibernate(page){
  const name=ctrlPageName(page);
  return CTRL_RETAINABLE_LITE_PAGES.has(name)&&!ctrlPageHasHeavyResidentContent(page);
}
function ctrlTrimHibernatedPages(){
  const limit=ctrlHibernatedPageLimit();
  while(CTRL_PAGE_LRU.length>limit){
    const evict=CTRL_PAGE_LRU.pop(),node=document.getElementById("ctrlpage-"+evict);
    if(node&&typeof ctrlEvictPage==="function")ctrlEvictPage(node,"evict");
  }
}
function ctrlObserveCacheBudgetStatus(status){
  const available=ctrlBudgetNumber(status&&status.mem_avail_mb),swapUsed=ctrlBudgetNumber(status&&status.swap_used_mb);
  if(available==null)return CTRL_CACHE_BUDGET.tier;
  const prior=CTRL_CACHE_BUDGET.previousSwapUsedMB;
  const tier=ctrlCacheBudgetTierFor(status,prior);
  CTRL_CACHE_BUDGET.tier=tier;CTRL_CACHE_BUDGET.memAvailMB=available;CTRL_CACHE_BUDGET.swapUsedMB=swapUsed==null?0:swapUsed;
  CTRL_CACHE_BUDGET.previousSwapUsedMB=swapUsed;CTRL_CACHE_BUDGET.observedAt=Date.now();
  try{document.documentElement.dataset.ctrlCacheBudget=tier;}catch(_){}
  ctrlTrimHibernatedPages();
  return tier;
}
function ctrlScheduleCacheBudgetProbe(){
  if(!ctrlLiteProfile()||!CTRL_OPEN)return;
  const cached=typeof CTRL_CACHE!=="undefined"&&CTRL_CACHE["/api/status"];
  if(cached)ctrlObserveCacheBudgetStatus(cached);
  const now=Date.now();if(now-CTRL_CACHE_BUDGET.lastProbeAt<CTRL_CACHE_BUDGET_POLL_MS)return;
  CTRL_CACHE_BUDGET.lastProbeAt=now;
  const probe=async()=>{
    if(!CTRL_OPEN)return;
    try{
      const status=await api("/api/status","GET",null,null);
      if(!status)return;
      CTRL_CACHE["/api/status"]=status;ctrlObserveCacheBudgetStatus(status);
    }catch(_){} // Control remains fully functional with the conservative default.
  };
  const later=()=>setTimeout(probe,450);
  if(typeof ctrlAfterPaint==="function")ctrlAfterPaint(later);else later();
}
function ctrlCancelPageDeferred(page){if(!page)return;page.querySelectorAll("details.ctrlsec[data-lazy]").forEach(d=>{if(d._lazyTimer){clearTimeout(d._lazyTimer);d._lazyTimer=0;}});}
function ctrlRememberHibernatedPage(page){
  if(!page)return;ctrlCancelPageDeferred(page);const name=ctrlPageName(page);if(!name)return;
  if(!ctrlLiteProfile())return;
  if(!ctrlPageCanHibernate(page)){
    CTRL_PAGE_LRU=CTRL_PAGE_LRU.filter(x=>x!==name);
    if(typeof ctrlEvictPage==="function")ctrlEvictPage(page,"evict");
    return;
  }
  CTRL_PAGE_LRU=[name,...CTRL_PAGE_LRU.filter(x=>x!==name)];ctrlTrimHibernatedPages();
}
function ctrlClearDeferredWork(){if(typeof CTRL_PAGE_RENDER_TIMER!=="undefined")clearTimeout(CTRL_PAGE_RENDER_TIMER);document.querySelectorAll("#ctrl details.ctrlsec[data-lazy]").forEach(d=>{if(d._lazyTimer)clearTimeout(d._lazyTimer);});}
function ctrlCloseMemorySettle(){
  ctrlClearDeferredWork();if(typeof ctrlAbortRequests==="function")ctrlAbortRequests();CTRL_PAGE_LRU=[];
  if(!ctrlLiteProfile())return;["ctrlmsg","ctrlmemory"].forEach(id=>{const n=$("#"+id);if(n)n.textContent="";});
}
