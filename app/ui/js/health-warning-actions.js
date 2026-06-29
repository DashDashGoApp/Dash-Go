// 07-health-warning-actions.js — footer health-warning silence controls.
// This module owns presentation silences only; it never changes cache timestamps,
// source selection, diagnostics, or the underlying health state.
let HEALTH_WARNING_PILL_BOUND=false;

const HEALTH_WARNING_SILENCE_DURATIONS=Object.freeze([
  {minutes:15,label:"15 min"},
  {minutes:60,label:"1 hour"},
  {minutes:240,label:"4 hours"},
  {minutes:720,label:"12 hours"},
  {minutes:1440,label:"24 hours"},
]);

function healthWarningKeyLabel(key){
  const labels={messages:"Message freshness reminder",weather:"Weather freshness reminder",calendar:"Calendar freshness reminder",storage:"Storage health notice",clock:"Clock health notice",config:"Configuration health notice",update:"Update health notice",postUpdate:"Post-update health notice",healthGuard:"Health guard notice"};
  return labels[key]||"Dashboard health notice";
}
function healthWarningAgeLabel(ms){
  const minutes=Math.max(0,Math.round((Number(ms)||0)/60000));
  if(minutes<60) return Math.round(minutes)+" minute"+(Math.round(minutes)===1?"":"s");
  const hours=Math.round(minutes/60);
  if(hours<48) return hours+" hour"+(hours===1?"":"s");
  const days=Math.round(hours/24);
  return days+" day"+(days===1?"":"s");
}
function healthWarningDetail(entry){
  const age=healthWarningAgeLabel(entry&&entry.ageMs);
  if(entry&&entry.key==="messages") return "Messages have not refreshed in "+age+". This reminder is shown only because one or more message feed categories are enabled.";
  if(entry&&entry.key==="weather") return "Weather data may be older than expected.";
  if(entry&&entry.key==="calendar") return "Calendar data may be older than expected.";
  if(entry&&entry.tier==="device") return "This device notice can be hidden temporarily while you review it. The diagnostic remains available in Dashboard Control, and critical device failures remain visible.";
  return "This dashboard reminder can be hidden temporarily while you review it.";
}
function healthWarningPopupButton(label,cls,fn){
  const button=document.createElement("button");
  button.type="button"; button.className="healthwarning-btn"+(cls?" "+cls:""); button.textContent=label;
  bindTap(button,fn);
  return button;
}
async function postHealthWarningSilence(key,minutes){
  let res,payload={};
  try{
    res=await fetch("/api/health/warnings/silence",{method:"POST",headers:{"Content-Type":"application/json","Accept":"application/json"},body:JSON.stringify({key,minutes})});
    payload=await res.json().catch(()=>({}));
  }catch(err){ throw new Error(err&&err.message?err.message:"Network request failed"); }
  if(!res.ok) throw new Error(payload.error||("HTTP "+res.status));
  return payload;
}
function setHealthWarningPopupStatus(node,text,kind){
  if(!node) return;
  node.textContent=text||"";
  node.className="healthwarning-status"+(kind?" "+kind:"");
}
function healthWarningSilenceNote(entry){
  if(entry&&entry.tier==="device") return "This hides only this non-critical footer notice until the selected time. It does not clear diagnostics, repair status, or a later critical failure.";
  return "Device, storage, update, clock, and repair concerns remain visible.";
}
function openHealthWarningDurationPopup(entry){
  if(!entry||!entry.silenceable) return;
  popupOpenTransaction({mode:"healthwarningpop",title:"Check soon",when:healthWarningKeyLabel(entry.key),loading:"Opening reminder controls…"},()=>{
    const wrap=el("div","healthwarning-popup");
    const copy=el("div","healthwarning-copy",healthWarningDetail(entry));
    const prompt=el("div","healthwarning-prompt","Silence this notice for:");
    const grid=el("div","healthwarning-duration-grid");
    const status=el("div","healthwarning-status");
    const note=el("div","healthwarning-note",healthWarningSilenceNote(entry));
    const cancel=healthWarningPopupButton("Cancel","ghost",closeScrim);
    const setBusy=busy=>grid.querySelectorAll("button").forEach(button=>{button.disabled=!!busy;});
    for(const duration of HEALTH_WARNING_SILENCE_DURATIONS){
      grid.appendChild(healthWarningPopupButton(duration.label,"duration",async()=>{
        setBusy(true); setHealthWarningPopupStatus(status,"Saving temporary silence…","");
        try{
          const out=await postHealthWarningSilence(entry.key,duration.minutes);
          DEVICE_HEALTH={...(DEVICE_HEALTH||{}),warningSilences:(out&&out.warningSilences)||{}};
          closeScrim(); updateStale();
        }catch(err){
          setBusy(false);
          setHealthWarningPopupStatus(status,"Could not save the temporary silence. The notice is still active.","warn");
        }
      }));
    }
    wrap.append(copy,prompt,grid,status,note,cancel);
    return wrap;
  });
}
function openHealthWarningSelectionPopup(entries){
  popupOpenTransaction({mode:"healthwarningpop",title:"Check soon",when:"Choose a warning",loading:"Opening reminder controls…"},()=>{
    const wrap=el("div","healthwarning-popup");
    wrap.appendChild(el("div","healthwarning-copy","Choose a warning to silence temporarily."));
    const choices=el("div","healthwarning-warning-list");
    for(const entry of entries.filter(item=>item&&item.silenceable)){
      choices.appendChild(healthWarningPopupButton(entry.text,"warning-choice",()=>openHealthWarningDurationPopup(entry)));
    }
    const critical=entries.filter(item=>item&&!item.silenceable&&item.tier==="device");
    if(critical.length){
      wrap.appendChild(el("div","healthwarning-device-note","Critical device notice: "+critical.map(item=>item.text).join(" · ")+". It remains visible."));
    }
    wrap.append(choices,healthWarningPopupButton("Cancel","ghost",closeScrim));
    return wrap;
  });
}
function openHealthWarningInfoPopup(entries){
  popupOpenTransaction({mode:"healthwarningpop",title:"Check soon",when:"Critical device health notice",loading:"Opening status…"},()=>{
    const wrap=el("div","healthwarning-popup");
    wrap.appendChild(el("div","healthwarning-copy",entries.map(entry=>entry.text).join(" · ")||"This critical device notice cannot be temporarily silenced."));
    wrap.appendChild(el("div","healthwarning-note","Critical device, storage, update, clock, and repair concerns remain visible until their underlying condition clears."));
    wrap.appendChild(healthWarningPopupButton("Close","ghost",closeScrim));
    return wrap;
  });
}
function openHealthWarningSilencePopup(){
  const entries=Array.isArray(ACTIVE_STALE_WARNINGS)?ACTIVE_STALE_WARNINGS.slice():[];
  if(!entries.length) return;
  const silenceable=entries.filter(entry=>entry&&entry.silenceable);
  if(silenceable.length===1){ openHealthWarningDurationPopup(silenceable[0]); return; }
  if(silenceable.length>1){ openHealthWarningSelectionPopup(entries); return; }
  openHealthWarningInfoPopup(entries);
}
function bindHealthWarningPill(){
  if(HEALTH_WARNING_PILL_BOUND) return;
  const pill=$("#stale"); if(!pill) return;
  HEALTH_WARNING_PILL_BOUND=true;
  bindTripleTap(pill,openHealthWarningSilencePopup,700,{moveTol:28,onFirstTap:()=>{
    if(!pill.hidden&&Array.isArray(ACTIVE_STALE_WARNINGS)&&ACTIVE_STALE_WARNINGS.length){
      pill.classList.remove("tap-pulse"); void pill.offsetWidth; pill.classList.add("tap-pulse");
      setTimeout(()=>pill.classList.remove("tap-pulse"),520);
    }
  }});
  pill.addEventListener("keydown",event=>{
    if((event.key==="Enter"||event.key===" ")&&!pill.hidden){
      event.preventDefault(); openHealthWarningSilencePopup();
    }
  });
}
