// Dashboard display and Weather & alerts keep their household choices beside
// the feature they affect. Performance Profile remains the reset surface.
function ctrlActionWithSetting(label,detail,key,on,fn){
  const b=caction(label,detail,on?"on":"",fn);if(key)b.dataset.settingKey=key;return b;
}
function ctrlMutedAlertLabel(){
  if(!alertsMuted())return {title:"Mute alerts for 12 hours",detail:"Dashboard banner stays visible until you mute it."};
  const until=new Date(SETTINGS.alertsMutedUntil||0),time=until.toLocaleTimeString([], {hour:"numeric",minute:"2-digit"});
  return {title:`Muted until ${time}`,detail:"Resume the dashboard alert banner now."};
}
function renderCtrlDashboardDisplay(){
  const wrap=$("#ctrldashboarddisplay");if(!wrap)return;wrap.replaceChildren();
  if(typeof renderCtrlDashboardTypography==="function") { renderCtrlDashboardTypography(); refreshDashboardFontStatus().then(()=>renderCtrlDashboardTypography()).catch(()=>{}); }
  const secondsOn=CONFIG.showSeconds!==false;
  const group=actionGroup("Clock & footer","Always-visible dashboard choices.","displaygroup grid-3-provider");
  group.grid.append(
    caction(`Time: ${CONFIG.clock24?"24-hour":"12-hour"}`,"Clock format","",()=>{CONFIG.clock24=!CONFIG.clock24;buildFormatters();_clockEls=null;tickClock();if(typeof armClockTimer==="function")armClockTimer();renderWeather();renderCalendar();renderAgenda();postSettings();renderCtrlDashboardDisplay();}),
    caction(`Temperature: ${CONFIG.tempUnit==="celsius"?"°C":"°F"}`,"Weather units","",()=>{CONFIG.tempUnit=CONFIG.tempUnit==="celsius"?"fahrenheit":"celsius";loadWeather();postSettings();renderCtrlDashboardDisplay();}),
    caction(`Clock seconds: ${secondsOn?"On":"Off"}`,"Show seconds in the large clock.",secondsOn?"on":"",async()=>{try{await ctrlSaveProfileOwned("showSeconds",!secondsOn,"Clock seconds","dashboarddisplay");renderCtrlDashboardDisplay();}catch(e){ctrlMsg("Could not change Clock seconds: "+(e.message||String(e)));renderCtrlDashboardDisplay();}})
  );wrap.appendChild(group.group);
}
function weatherHealthLabel(item){
  const status=String(item&&item.status||"").replace(/_/g," "),fresh=String(item&&item.freshness||"");
  if(item&&item.ok&&fresh==="fresh")return "Active · fresh";
  if(item&&item.ok&&fresh==="cached")return "Active · cached";
  if(item&&item.ok&&fresh==="stale")return "Active · stale";
  return status?status.charAt(0).toUpperCase()+status.slice(1):(item&&item.ok?"Active":"Unavailable");
}
function weatherHealthDetail(item){
  const bits=[];if(item&&item.reason)bits.push(item.reason);else if(item&&item.error)bits.push(item.error);else if(item&&item.tier)bits.push(item.tier);
  if(item&&item.daysReturned!=null)bits.push(`${item.daysReturned} days returned`);
  if(item&&item.dailyBudget)bits.push(`today ${item.dailyRequests||0}/${item.dailyBudget}`);
  if(item&&item.nextRetry)bits.push(`retry ${new Date(item.nextRetry).toLocaleString([], {month:"short",day:"numeric",hour:"numeric",minute:"2-digit"})}`);
  return bits.join(" · ");
}
async function renderCtrlWeatherAlerts(){
  const wrap=$("#ctrlweatheralerts");if(!wrap)return;ctrlSetLoading(wrap,"Loading weather & alerts…","Reading configured source health only after this card opens.");
  let payload={};try{payload=await api("/api/weather");}catch(_){payload={sourceHealth:[]};}
  const frag=document.createDocumentFragment(),list=Array.isArray(payload.sourceHealth)?payload.sourceHealth:(Array.isArray(payload.status)?payload.status:[]),healthy=list.filter(x=>x&&x.ok).length,stale=list.filter(x=>x&&x.freshness==="stale").length;
  const health=el("section","weatherhealthsummary"),head=el("div","weatherhealthhead");head.append(el("div","weatherhealthtitle",`Weather source health · ${healthy} active${stale?` · ${stale} stale`:""}`),cbtn("Refresh weather","",async()=>{try{await loadWeather();await renderCtrlWeatherAlerts();}catch(e){ctrlMsg(e.message||String(e));}}));health.appendChild(head);
  const sourceGrid=el("div","weatherhealthgrid");for(const item of list){const card=el("div","weatherhealthrow "+(item&&item.ok?"ok":"bad"));card.innerHTML=`<div class="wxhealthname">${escapeHTML((item.label||item.id||"Weather source")+" — "+weatherHealthLabel(item))}</div><div class="wxhealthdetail">${escapeHTML(weatherHealthDetail(item)||item.tier||"")}</div>`;sourceGrid.appendChild(card);}if(!list.length)sourceGrid.appendChild(ctrlStateCard("info","Source health not cached","Open the forecast once or use Refresh weather to populate provider status."));health.appendChild(sourceGrid);frag.appendChild(health);
  const alerts=(CONFIG.weatherAlerts&&typeof CONFIG.weatherAlerts==="object")?CONFIG.weatherAlerts:{enabled:true},monitoringOn=alerts.enabled!==false;
  const monitoring=actionGroup("Background alert monitoring","Automatic five-minute checks are separate from the temporary banner mute.","displaygroup grid-1-feature");
  monitoring.grid.append(caction(`Alert monitoring: ${monitoringOn?"On":"Off"}`,"Controls whether Dash-Go checks and surfaces weather alerts.",monitoringOn?"on":"",async()=>{try{await ctrlSaveProfileOwned("weatherAlerts",{...alerts,enabled:!monitoringOn,refreshMinutes:5},"Alert monitoring","weatheralerts");await renderCtrlWeatherAlerts();}catch(e){ctrlMsg("Could not change Alert monitoring: "+(e.message||String(e)));await renderCtrlWeatherAlerts();}}));frag.appendChild(monitoring.group);
  const banner=actionGroup("Dashboard alert banner","Temporary mute never disables background alert monitoring.","displaygroup grid-2-feature"),mute=ctrlMutedAlertLabel();
  banner.grid.append(caction(mute.title,mute.detail,!alertsMuted()?"on":"",()=>{SETTINGS.alertsMutedUntil=alertsMuted()?0:Date.now()+12*3600*1000;renderAlerts();applyNightDim();postSettings();renderCtrlWeatherAlerts();}),caction("Preview alert","Show a local banner without fetching or muting.","",()=>{if(typeof previewAlertBanner==="function")previewAlertBanner();}));frag.appendChild(banner.group);
  const detailMode=CONFIG.weatherDetailMode==="standard"?"standard":"expanded";
  const extras=actionGroup("Current-weather details","Standard keeps the forecast compact. Expanded adds UV and air quality when available.","displaygroup grid-2-feature");
  const setMode=mode=>{if(mode===detailMode)return;CONFIG.weatherDetailMode=mode;CONFIG.showUV=mode==="expanded";CONFIG.showAQI=mode==="expanded";renderWeather();postSettings();renderCtrlWeatherAlerts();};
  extras.grid.append(caction("Standard","Temperature and conditions only",detailMode==="standard"?"on":"",()=>setMode("standard")),caction("Expanded","Adds UV and air quality",detailMode==="expanded"?"on":"",()=>setMode("expanded")));frag.appendChild(extras.group);
  wrap.replaceChildren(frag);
}
