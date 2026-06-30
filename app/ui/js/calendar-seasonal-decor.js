// 04-calendar-03d-seasonal-decor.js — static calendar decoration placement.
// Art lives in focused source modules. This runtime only selects a reviewed set
// and places static decals in empty current-month calendar cells.
const SEASONAL_DECOR_SVGS=Object.freeze({
  ...SEASONAL_DECOR_SEASONS,
  ...SEASONAL_DECOR_SEASONAL,
  ...SEASONAL_DECOR_OBSERVANCES
});
const THEME_DECOR_MAP=Object.freeze({
  christmas:'christmas', halloween:'halloween', thanksgiving:'thanksgiving',
  winter:'winter', spring:'spring', autumn:'autumn', summer:'summer',
  valentine:'valentine', stpatricks:'stpatricks', easter:'easter', newyear:'newyear',
  america:'america', pride:'pride', mardigras:'mardigras', lunar:'lunar',
  oktoberfest:'oktoberfest', earthday:'earthday', cincodemayo:'cincodemayo',
  juneteenth:'juneteenth', muertos:'muertos', memorialday:'memorialday',
  laborday:'laborday', veterans:'veterans', mothersday:'mothersday',
  fathersday:'fathersday', hanukkah:'hanukkah', kwanzaa:'kwanzaa'
});
const SEASONAL_DECOR_COUNTS=Object.freeze({subtle:5,standard:10});
let _SEASONAL_DECOR_SIGNATURE='';
let _SEASONAL_DECOR_RECONCILE=0;

function seasonalDecorKind(){
  const theme=String(CURRENT_THEME||document.documentElement.getAttribute('data-theme')||'').toLowerCase();
  return THEME_DECOR_MAP[theme]||'';
}
function seasonalDecorMode(){
  const settings=typeof dashboardRuntimeSettings==='function'?dashboardRuntimeSettings():null;
  return (settings&&settings.seasonalDecor)||CONFIG.seasonalDecor||'off';
}
function seasonalDecorEnabledForCurrentTheme(){
  const kind=seasonalDecorKind();
  return seasonalDecorMode()!=='off' && Array.isArray(SEASONAL_DECOR_SVGS[kind]) && SEASONAL_DECOR_SVGS[kind].length===5;
}
function seasonalDecorSignature(){
  return seasonalDecorEnabledForCurrentTheme()?`${seasonalDecorMode()}:${seasonalDecorKind()}`:'off';
}
function clearSeasonalDecor(){
  const root=document.querySelector('#calscroll');
  if(!root) return false;
  root.querySelectorAll('.seasonal-decor').forEach(node=>node.remove());
  _SEASONAL_DECOR_SIGNATURE='off';
  return true;
}
function seasonalDecorHash(value){
  let hash=5381;
  for(let index=0;index<value.length;index++) hash=((hash*33)^value.charCodeAt(index))|0;
  return hash>>>0;
}
function seasonalDecorRotationOffset(kind,length,now){
  if(!length) return 0;
  const date=now instanceof Date?now:new Date();
  const monthId=date.getFullYear()*12+date.getMonth();
  return (seasonalDecorHash(kind)+monthId)%length;
}
function seasonalDecorCells(root){
  return [...root.querySelectorAll('.daycell')].filter(cell=>{
    if(cell.classList.contains('today')||cell.classList.contains('other')) return false;
    if(cell.dataset.cellLanes&&+cell.dataset.cellLanes>0) return false;
    const eventList=cell.querySelector('.evlist');
    return eventList&&!eventList.querySelector('.ev,.more:not(.autofit-hidden)');
  });
}
function applySeasonalDecor(){
  const root=document.querySelector('#calscroll');
  if(!root) return false;
  const signature=seasonalDecorSignature();
  clearSeasonalDecor();
  if(signature==='off') return true;
  const kind=seasonalDecorKind();
  const svgs=SEASONAL_DECOR_SVGS[kind];
  const cells=seasonalDecorCells(root);
  if(!cells.length){_SEASONAL_DECOR_SIGNATURE=signature;return true;}
  const mode=seasonalDecorMode();
  const count=Math.min(SEASONAL_DECOR_COUNTS[mode]||0,cells.length);
  const rotation=seasonalDecorRotationOffset(kind,svgs.length,new Date());
  const used=new Set();
  for(let index=0;index<count;index++){
    let cellIndex=Math.round((index+1)*cells.length/(count+1))-1;
    cellIndex=Math.max(0,Math.min(cells.length-1,cellIndex));
    while(used.has(cellIndex)&&cellIndex<cells.length-1) cellIndex++;
    used.add(cellIndex);
    const decal=el('div',`seasonal-decor decor-${kind}${mode==='subtle'?' decor-subtle':''}`);
    decal.setAttribute('aria-hidden','true');
    decal.innerHTML=svgs[(rotation+index)%svgs.length];
    cells[cellIndex].appendChild(decal);
  }
  _SEASONAL_DECOR_SIGNATURE=signature;
  return true;
}
// Theme and visual-setting changes are reconciled once after the actual change.
// There is no periodic decoration timer, network request, or background scan.
function reconcileSeasonalDecor(){
  _SEASONAL_DECOR_RECONCILE=0;
  const next=seasonalDecorSignature();
  if(next===_SEASONAL_DECOR_SIGNATURE) return false;
  return next==='off'?clearSeasonalDecor():applySeasonalDecor();
}
function scheduleSeasonalDecorReconcile(){
  if(_SEASONAL_DECOR_RECONCILE) return;
  const run=()=>reconcileSeasonalDecor();
  _SEASONAL_DECOR_RECONCILE=typeof requestAnimationFrame==='function'?requestAnimationFrame(run):setTimeout(run,0);
}
