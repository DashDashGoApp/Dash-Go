// Performance Profile stays focused on selecting/resetting a device tier. The
// individual profile-owned settings live with the feature they affect.
let CTRL_PROFILE_EDITOR=null;
const CTRL_PROFILE_SAVE_QUEUES=new Map();

function profileBaseName(data){
  const base=String((data&&data.base)||CONFIG.profile||"balanced").toLowerCase();
  return ["lite","balanced","enhanced"].includes(base)?base:"balanced";
}
function ctrlResetProfileEditor(wrap){
  const state=CTRL_PROFILE_EDITOR;
  if(state&&(!wrap||state.wrap===wrap)){state.disposed=true;CTRL_PROFILE_EDITOR=null;}
  if(wrap&&wrap.dataset)delete wrap.dataset.profileReady;
}
function profileChangedLabel(key){
  const labels={showSeconds:"Clock seconds",weeksAbove:"Calendar back",weeksBelow:"Calendar ahead",rowHeight:"Calendar row height",sidebarWidth:"Sidebar width",showInteractiveMaps:"Interactive event maps",weatherAlerts:"Alert monitoring"};
  return labels[key]||String(key||"").replace(/([A-Z])/g," $1").replace(/^./,c=>c.toUpperCase());
}
function profileChangedValue(key,value){
  if(key==="showSeconds"||key==="showInteractiveMaps")return value?"On":"Off";
  if(key==="weatherAlerts")return value&&value.enabled===false?"Off":"On";
  if(key==="weeksAbove"||key==="weeksBelow"){
    const weeks=Number(value)||0;return `${weeks} week${weeks===1?"":"s"}`;
  }
  if(key==="rowHeight"||key==="sidebarWidth")return `${Math.round(Number(value)||0)} px`;
  return String(value??"—");
}
function profileChangedSettings(data){
  if(Array.isArray(data&&data.changedSettings))return data.changedSettings.filter(item=>item&&item.key!=="layoutProfile");
  const base=profileBaseName(data),preset=(Array.isArray(data&&data.options)?data.options:[]).find(item=>String(item&&item.name||"").toLowerCase()===base)||{},defaults=preset.values||{},values=(data&&data.values)||{};
  return (Array.isArray(data&&data.diverged)?data.diverged:[]).filter(key=>key!=="layoutProfile").map(key=>({key,default:defaults[key],current:values[key]}));
}
function profileRefreshMountedOwners(changed,exceptLazy){
  const routes={showSeconds:"dashboarddisplay",showInteractiveMaps:"mapcache",weatherAlerts:"weatheralerts",weeksAbove:"calendarlayout",weeksBelow:"calendarlayout",rowHeight:"calendarlayout",sidebarWidth:"calendarlayout"},affected=new Set();
  for(const key of (changed instanceof Set?changed:new Set()))if(routes[key]&&routes[key]!==exceptLazy)affected.add(routes[key]);
  for(const lazy of affected){
    const open=document.querySelector(`#ctrl details.ctrlsec[data-lazy="${lazy}"][open]`);if(!open)continue;
    if(lazy==="dashboarddisplay"&&typeof renderCtrlDashboardDisplay==="function")renderCtrlDashboardDisplay();
    else if(lazy==="weatheralerts"&&typeof renderCtrlWeatherAlerts==="function")renderCtrlWeatherAlerts();
    else if(lazy==="mapcache"&&typeof renderCtrlMapCache==="function")renderCtrlMapCache();
    else if(lazy==="calendarlayout"&&typeof renderCtrlUiSettings==="function")renderCtrlUiSettings();
  }
}
function profileEditorUpdate(state){
  const data=state.data||{},base=profileBaseName(data),changes=profileChangedSettings(data),custom=changes.length>0;
  state.title.textContent=custom?"Custom":profileLabel(base);state.badge.textContent=custom?"custom":base;state.badge.classList.toggle("custom",custom);
  state.detail.textContent=custom?`Based on ${profileLabel(base)} · ${changes.length} setting${changes.length===1?"":"s"} changed.`:`Using ${profileLabel(base)} defaults.`;
  state.changed.hidden=!custom;state.changedSummary.textContent=custom?`View ${changes.length} changed setting${changes.length===1?"":"s"}`:"";state.changedList.replaceChildren();
  for(const change of changes){
    const row=el("div","profilechange"),title=el("div","profilechange-title",profileChangedLabel(change.key)),values=el("div","profilechange-values");
    values.append(el("div","profilechange-value",`Profile default: ${profileChangedValue(change.key,change.default)}`),el("div","profilechange-value current",`Current: ${profileChangedValue(change.key,change.current)}`));row.append(title,values);state.changedList.appendChild(row);
  }
  for(const item of state.presetButtons){const active=item.name===base;item.title.textContent=active?(custom?`Restore ${item.label} defaults`:`${item.label} active`):`Apply ${item.label} defaults`;item.detail.textContent=active?(custom?"Reset Calendar, Display, Weather & alerts, and Event maps settings.":"This profile is already intact."):item.description;item.button.classList.toggle("on",active);item.button.disabled=!!state.presetBusy||(!custom&&active);}
}
function profileSyncEditor(result){
  const state=CTRL_PROFILE_EDITOR;if(!state||state.disposed||!result)return;
  state.data=result;profileEditorUpdate(state);
}
function ctrlApplyProfilePayload(result,exceptLazy){
  if(!result)return new Set();
  const changed=applyProfileResult(result);profileSyncEditor(result);profileRefreshMountedOwners(changed,exceptLazy);return changed;
}
function ctrlSaveProfileOwned(key,value,label,exceptLazy){
  const previous=CTRL_PROFILE_SAVE_QUEUES.get(key)||Promise.resolve();
  const save=previous.catch(()=>{}).then(async()=>{
    const result=await api("/api/profile","POST",{set:{[key]:value}});ctrlApplyProfilePayload(result,exceptLazy);ctrlMsg(`${label||profileChangedLabel(key)} updated.`);return result;
  });
  const settled=save.finally(()=>{if(CTRL_PROFILE_SAVE_QUEUES.get(key)===settled)CTRL_PROFILE_SAVE_QUEUES.delete(key);});
  CTRL_PROFILE_SAVE_QUEUES.set(key,settled);return settled;
}
function profileEditorBuild(state,wrap){
  state.wrap=wrap;state.presetButtons=[];const frag=document.createDocumentFragment(),top=el("div","profiletop"),copy=el("div");copy.append(el("div","profileeyebrow","Performance profile"),state.title=el("div","profiletitle"),state.detail=el("div","profiledetail"));state.badge=el("div","profilebadge");top.append(copy,state.badge);frag.append(top,el("div","profilegroup-title","Choose a profile"));const grid=el("div","profilegrid");
  for(const opt of (state.data.options||[])){const name=String(opt.name||"").toLowerCase(),label=opt.label||profileLabel(name),button=confirmAction("","","Tap again to apply",async()=>profileEditorApplyPreset(state,name,label));button.classList.add("profileoption");button.dataset.settingKey="profile";const title=el("span","bt"),detail=el("span","bd");button.replaceChildren(title,detail);grid.append(button);state.presetButtons.push({name,label,description:opt.detail||"Apply this performance preset.",button,title,detail});}
  frag.append(grid);const note=ctrlStateCard("info","What a profile resets","Profiles reset Calendar layout, Dashboard display, Weather & alerts, and Event maps controls. Your location, theme, calendars, messages, weather keys, display schedule, and PIN remain unchanged.");note.classList.add("profile-reset-note");frag.append(note);
  const changed=document.createElement("details");changed.className="profilechanges";state.changed=changed;state.changedSummary=el("summary","","");state.changedList=el("div","profilechanges-list");changed.append(state.changedSummary,state.changedList);frag.append(changed);wrap.replaceChildren(frag);wrap.dataset.profileReady="1";profileEditorUpdate(state);
}
async function profileEditorApplyPreset(state,name,label){
  if(state.presetBusy)return;state.presetBusy=true;profileEditorUpdate(state);
  try{const result=await api("/api/profile","POST",{profile:name,applyDefaults:true});ctrlApplyProfilePayload(result,"profile");ctrlMsg(`${label} defaults applied. Personal settings were preserved.`);}
  catch(err){ctrlMsg(`Profile update failed: ${err&&err.message?err.message:String(err)}`);}
  finally{state.presetBusy=false;profileEditorUpdate(state);}
}
async function renderCtrlProfile(force){
  const wrap=$("#ctrlprofile");if(!wrap)return;if(!force&&CTRL_PROFILE_EDITOR&&CTRL_PROFILE_EDITOR.wrap===wrap&&wrap.dataset.profileReady==="1"&&CTRL_PROFILE_EDITOR.data)return;
  const state={wrap,data:null,disposed:false,presetButtons:[],presetBusy:false,title:null,detail:null,badge:null,changed:null,changedSummary:null,changedList:null};CTRL_PROFILE_EDITOR=state;ctrlSetLoading(wrap,"Checking performance profile…","Reading the current preset and changed settings.");
  try{const data=await api("/api/profile");if(CTRL_PROFILE_EDITOR!==state)return;state.data=data;profileEditorBuild(state,wrap);}
  catch(err){if(CTRL_PROFILE_EDITOR!==state)return;wrap.innerHTML="";ctrlSetError(wrap,"Performance profile unavailable",err,[cbtn("Try again","",()=>renderCtrlProfile(true))]);}
}
