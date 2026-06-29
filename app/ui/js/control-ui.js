function ctrlMsg(t){ const m=$("#ctrlmsg"); if(m) m.textContent=t||""; }

function ctrlStateCard(kind,title,detail,actions){
  const card=el("div","ctrlstate "+(kind||"info"));
  card.innerHTML=`<div class="ctrlstate-title">${escapeHTML(title||"")}</div>${detail?`<div class="ctrlstate-detail">${escapeHTML(detail)}</div>`:""}`;
  if(actions){ const row=el("div","ctrlrow compact ctrlstate-actions"); for(const a of actions) row.appendChild(a); card.appendChild(row); }
  return card;
}

// Shared compact metric tile for Control renderers. Keeping the factory here
// prevents lazy sections from depending on page-local helpers and guarantees
// all labels/values are escaped before they enter the Control DOM.
function ctrlStat(label,value,state){
  const tile=el("div","stat "+(state||"unknown"));
  tile.innerHTML=`<div class="k">${escapeHTML(String(label??""))}</div><div class="v">${escapeHTML(String(value??"—"))}</div>`;
  return tile;
}
function ctrlRefreshAfterPaint(fn){
  const later=typeof requestAnimationFrame==="function"?requestAnimationFrame:setTimeout;
  later(()=>setTimeout(fn,0));
}
function ctrlWarmContentPresent(wrap){
  if(!wrap||!wrap.children||!wrap.children.length)return false;
  return !Array.from(wrap.children).every(node=>node.classList&&typeof node.classList.contains==="function"&&node.classList.contains("ctrlstate")&&node.classList.contains("loading"));
}
function ctrlStopWarmRefresh(wrap,state){
  if(!wrap||wrap._ctrlWarmRefresh!==state)return;
  if(state.observer)try{state.observer.disconnect();}catch(_){}
  clearTimeout(state.timeout);
  wrap._ctrlWarmRefresh=null;
  wrap.classList.remove("ctrlrefreshing");
  if(state.minHeight===null)wrap.style.removeProperty("min-height");else wrap.style.minHeight=state.minHeight;
  if(state.ariaBusy===null)wrap.removeAttribute("aria-busy");else wrap.setAttribute("aria-busy",state.ariaBusy);
  if(state.label===undefined)delete wrap.dataset.ctrlRefreshLabel;else wrap.dataset.ctrlRefreshLabel=state.label;
}
function ctrlQueueWarmRefreshEnd(wrap,state){
  if(!wrap||wrap._ctrlWarmRefresh!==state)return;
  const token=++state.finishToken;
  ctrlRefreshAfterPaint(()=>ctrlRefreshAfterPaint(()=>{
    if(wrap._ctrlWarmRefresh===state&&state.finishToken===token)ctrlStopWarmRefresh(wrap,state);
  }));
}
function ctrlBeginWarmRefresh(wrap,label){
  if(!wrap||!wrap.style||!wrap.dataset||!wrap.classList||typeof wrap.classList.add!=="function"||typeof wrap.setAttribute!=="function")return null;
  const existing=wrap._ctrlWarmRefresh;
  if(existing){existing.finishToken++;return existing;}
  const rect=wrap.getBoundingClientRect?wrap.getBoundingClientRect():null;
  const state={
    minHeight:wrap.style.minHeight||null,
    ariaBusy:wrap.getAttribute("aria-busy"),
    label:Object.prototype.hasOwnProperty.call(wrap.dataset,"ctrlRefreshLabel")?wrap.dataset.ctrlRefreshLabel:undefined,
    observer:null,timeout:0,finishToken:0,
  };
  wrap._ctrlWarmRefresh=state;
  if(rect&&Number(rect.height)>0)wrap.style.minHeight=Math.ceil(Number(rect.height))+"px";
  wrap.classList.add("ctrlrefreshing");
  wrap.dataset.ctrlRefreshLabel=label||"Refreshing…";
  wrap.setAttribute("aria-busy","true");
  if(typeof MutationObserver!=="undefined"){
    state.observer=new MutationObserver(records=>{
      if(records.some(record=>record.type==="childList"&&(record.addedNodes.length||record.removedNodes.length)))ctrlQueueWarmRefreshEnd(wrap,state);
    });
    state.observer.observe(wrap,{childList:true});
  }
  // A cancelled page request normally clears its section during hibernation.
  // This bounded fallback only prevents a stale busy affordance if an older
  // renderer returns without committing any DOM at all.
  state.timeout=setTimeout(()=>ctrlStopWarmRefresh(wrap,state),15000);
  return state;
}
function ctrlSetLoading(wrap,title,detail){
  if(!wrap)return false;
  if(ctrlWarmContentPresent(wrap)){
    ctrlBeginWarmRefresh(wrap,"Refreshing…");
    return true;
  }
  const stale=wrap._ctrlWarmRefresh;
  if(stale)ctrlStopWarmRefresh(wrap,stale);
  wrap.replaceChildren(ctrlStateCard("loading",title||"Loading…",detail||"Checking the dashboard now."));
  return false;
}
function ctrlSetEmpty(wrap,title,detail,actions){
  if(!wrap) return;
  wrap.innerHTML="";
  wrap.appendChild(ctrlStateCard("empty",title||"Nothing here yet",detail||"There is nothing to show right now.",actions));
}
function ctrlSetError(wrap,title,err,actions){
  if(!wrap) return;
  const msg=err && err.message ? err.message : String(err||"Unknown error");
  wrap.innerHTML="";
  wrap.appendChild(ctrlStateCard("bad",title||"This section could not load",msg,actions));
}
function friendlyUnavailable(noun,err){
  const msg=err && err.message ? err.message : String(err||"");
  return noun+" is not available right now"+(msg?": "+msg:"")+".";
}
function ctrlUpdateSettingCard(key,opts){
  // Patch a small mounted setting/action surface without replacing its parent
  // card. This keeps pointer/keyboard focus on the control that just changed
  // and avoids a full Control-page layout pass for ordinary steppers/toggles.
  const name=String(key||"").replace(/["\\]/g,"\\$&");
  const root=document.querySelector(`.settingcard[data-setting-key="${name}"],.cbtn[data-setting-key="${name}"]`);
  if(!root)return false;
  const next=opts||{};
  if(next.value!=null){const value=root.querySelector(".settingvalue");if(value)value.textContent=String(next.value);}
  if(next.title!=null){const title=root.querySelector(".bt");if(title)title.textContent=String(next.title);}
  if(next.detail!=null){const detail=root.querySelector(".bd");if(detail)detail.textContent=String(next.detail);}
  if(next.pressed!=null){root.classList.toggle("on",!!next.pressed);root.setAttribute("aria-pressed",String(!!next.pressed));}
  if(next.selectedChoice!=null){
    const selected=String(next.selectedChoice);
    root.querySelectorAll("[data-setting-choice]").forEach(button=>{
      const active=button.dataset.settingChoice===selected;
      button.classList.toggle("on",active);
      button.setAttribute("aria-pressed",String(active));
    });
  }
  const buttons=root.querySelectorAll(".settingbuttons .cbtn");
  if(buttons.length>=2){
    if(next.minusDisabled!=null)buttons[0].disabled=!!next.minusDisabled;
    if(next.plusDisabled!=null)buttons[1].disabled=!!next.plusDisabled;
  }
  return true;
}
function cbtn(label,cls,fn){
  const b=el("button","cbtn"+(cls?" "+cls:""),label);
  b.type="button";
  bindTap(b,fn);
  return b;
}
function caction(label,desc,cls,fn){
  const b=cbtn("", "actionbtn"+(cls?" "+cls:""), fn);
  b.innerHTML=`<span class="bt">${escapeHTML(label)}</span>${desc?`<span class="bd">${escapeHTML(desc)}</span>`:""}`;
  return b;
}
function actionGroup(title,detail,cls){
  const group=el("div","actiongroup"+(cls?" "+cls:""));
  group.innerHTML=`<div class="actiongroup-head"><div class="actiongroup-title">${escapeHTML(title)}</div>${detail?`<div class="actiongroup-detail">${escapeHTML(detail)}</div>`:""}</div>`;
  const grid=el("div","actiongroup-grid");
  group.appendChild(grid);
  return {group,grid};
}
function confirmAction(label,desc,armedLabel,fn){
  const b=confirmBtn(label,armedLabel,fn);
  b.classList.add("actionbtn");
  b.classList.add("requiresconfirm");
  b.innerHTML=`<span class="bt">${escapeHTML(label)}</span>${desc?`<span class="bd">${escapeHTML(desc)}</span>`:""}`;
  b.dataset.normalHtml=b.innerHTML;
  return b;
}
function doctorSafeRepairAction(label,desc,armedLabel,fn){
  const b=confirmAction(label,desc,armedLabel,fn);
  // Safe Doctor repairs still require a second tap, but they are reversible
  // maintenance work rather than a destructive or power action.
  b.classList.remove("danger","requiresconfirm");
  b.classList.add("doctor-safe-repair");
  return b;
}
function fmtBytes(n){
  n=Number(n||0);
  if(n>=1024*1024*1024) return (n/1024/1024/1024).toFixed(1)+" GB";
  if(n>=1024*1024) return (n/1024/1024).toFixed(1)+" MB";
  if(n>=1024) return Math.round(n/1024)+" KB";
  return n+" B";
}
function profileLabel(name){
  const map={lite:"Lite",balanced:"Balanced",enhanced:"Enhanced",custom:"Custom",maximum:"Enhanced",zero2:"Lite",low:"Lite","low-power":"Lite"};
  const k=String(name||"balanced").toLowerCase();
  return map[k] || (k?k.charAt(0).toUpperCase()+k.slice(1):"Balanced");
}
function profileDetailLine(p){
  if(!p) return "Performance and visibility preset.";
  return p.detail || p.profileDetail || "Performance and visibility preset.";
}
function profileRuntimeEqual(left,right){
  if(left===right)return true;
  if(typeof left!=="object"||typeof right!=="object"||!left||!right)return String(left)===String(right);
  const leftKeys=Object.keys(left).sort(),rightKeys=Object.keys(right).sort();
  if(leftKeys.length!==rightKeys.length)return false;
  return leftKeys.every((key,index)=>key===rightKeys[index]&&profileRuntimeEqual(left[key],right[key]));
}
function applyProfileResult(r){
  if(!r)return new Set();
  // Profile edits are persisted by /api/profile. Apply only the deliberately
  // user-tunable values; all retired knobs stay fixed automatic defaults.
  const vals=r.config||r.values||{},settings=r.settings||{},changed=new Set();
  const base=String(r.base||r.current||CONFIG.profile||"balanced").toLowerCase();
  const before={profile:CONFIG.profile,showSeconds:CONFIG.showSeconds,showInteractiveMaps:CONFIG.showInteractiveMaps,weeksAbove:CONFIG.weeksAbove,weeksBelow:CONFIG.weeksBelow,rowHeight:CONFIG.rowHeight,sidebarWidth:CONFIG.sidebarWidth,weatherAlerts:CONFIG.weatherAlerts};
  if(typeof SETTINGS!=="undefined"){SETTINGS={...SETTINGS,...settings,pixelShiftEnabled:true};if(typeof syncDashboardRuntimeSettings==="function")syncDashboardRuntimeSettings();}
  if(["lite","balanced","enhanced"].includes(base))CONFIG.profile=base;
  for(const key of ["showSeconds","showInteractiveMaps"])if(key in vals)CONFIG[key]=!!vals[key];
  for(const key of ["weeksAbove","weeksBelow","rowHeight","sidebarWidth"])if(Number.isFinite(+vals[key]))CONFIG[key]=+vals[key];
  if(vals.weatherAlerts&&typeof vals.weatherAlerts==="object")CONFIG.weatherAlerts={...(CONFIG.weatherAlerts||{}),...vals.weatherAlerts,refreshMinutes:5};
  CONFIG.showEventMaps=true;CONFIG.pixelShift=2;
  for(const [key,value] of Object.entries(before))if(!profileRuntimeEqual(value,CONFIG[key]))changed.add(key);
  if(changed.has("profile")&&typeof applyVisualSettings==="function")applyVisualSettings();
  const root=document.documentElement;
  const geometryChanged=changed.has("rowHeight")||changed.has("sidebarWidth");
  // Preview dimensions immediately behind Control so a household can tune the
  // visible Calendar/sidebar outline. The costly row rebuild and Today-anchor
  // rewrite are deliberately committed once when Control closes.
  const geometryPreview=geometryChanged&&typeof calendarGeometryBeginPreview==="function"?calendarGeometryBeginPreview():false;
  if(changed.has("rowHeight"))root.style.setProperty("--dash-rowheight-preference",CONFIG.rowHeight+"px");
  if(changed.has("sidebarWidth"))root.style.setProperty("--dash-sidebarwidth-preference",CONFIG.sidebarWidth+"px");
  if(changed.has("showSeconds")){buildFormatters();_clockEls=null;tickClock();if(typeof armClockTimer==="function")armClockTimer();}
  if(changed.has("weatherAlerts")){if(typeof loadAlerts==="function")loadAlerts();if(typeof applyNightDim==="function")applyNightDim();}
  if((changed.has("weatherAlerts")||changed.has("profile"))&&typeof armDashboardRefreshSchedules==="function")armDashboardRefreshSchedules();
  const calendarRange=changed.has("weeksAbove")||changed.has("weeksBelow");
  const calendarPaint=calendarRange||geometryChanged;
  if(calendarRange&&typeof loadCalendars==="function")loadCalendars();
  else if(calendarPaint&&!geometryPreview&&typeof renderCalendar==="function")renderCalendar();
  if(calendarRange&&typeof renderAgenda==="function")renderAgenda();
  // A direct geometry step already publishes CSS preview variables. Do not let
  // the fit controller turn each +/− press into a second Calendar transaction.
  if(typeof dashboardFitSchedule==="function"&&(!geometryPreview||changed.has("profile")||calendarRange))dashboardFitSchedule("profile",0);
  return changed;
}
