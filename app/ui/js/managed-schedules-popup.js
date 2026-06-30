// One-off correction surface for explicit Dash-Go household schedule events.
// This deliberately does not infer ownership from an event title; only cache
// metadata written by the local schedule generator enables Manage schedule.
function managedScheduleInfo(ev){
  const raw=ev&&ev.managedSchedule;
  if(!raw||typeof raw!=="object")return null;
  const ruleId=String(raw.ruleId||"").trim(),nominalDate=String(raw.nominalDate||"").trim();
  if(!/^[a-z0-9][a-z0-9_-]{0,47}$/.test(ruleId)||!/^\d{4}-\d{2}-\d{2}$/.test(nominalDate))return null;
  return {type:String(raw.type||"schedule"),ruleId,nominalDate,actualDate:String(raw.actualDate||"").trim(),reason:String(raw.reason||"").trim()};
}
function managedScheduleLocalDate(iso){
  const m=String(iso||"").match(/^(\d{4})-(\d{2})-(\d{2})$/);if(!m)return null;
  const value=new Date(+m[1],+m[2]-1,+m[3]);return Number.isNaN(+value)?null:value;
}
function managedScheduleISO(date){return `${date.getFullYear()}-${String(date.getMonth()+1).padStart(2,"0")}-${String(date.getDate()).padStart(2,"0")}`;}
function managedScheduleDateLabel(iso){const d=managedScheduleLocalDate(iso);return d?FMT.dayLong.format(d):String(iso||"");}
function managedScheduleButton(label,cls,fn){
  const b=el("button","managed-schedule-button "+(cls||""),label);b.type="button";bindTap(b,fn);return b;
}
function managedScheduleAction(label,detail,cls,fn){
  const b=managedScheduleButton("",`managed-schedule-action ${cls||""}`,fn);
  b.append(el("span","managed-schedule-action-title",label),el("span","managed-schedule-action-detail",detail));return b;
}
function managedScheduleConfirm(label,armedLabel,fn){
  let timer=0;const b=managedScheduleButton(label,"danger",()=>{
    if(!b.classList.contains("armed")){
      b.classList.add("armed");b.textContent=armedLabel;clearTimeout(timer);
      timer=setTimeout(()=>{b.classList.remove("armed");b.textContent=label;},3200);return;
    }
    clearTimeout(timer);fn();
  });return b;
}
function managedScheduleDateInput(value){
  const input=el("input","managed-schedule-date");
  input.type="text";input.inputMode="numeric";input.placeholder="YYYY-MM-DD";input.value=value||"";input.maxLength=10;
  input.dataset.oskMode="date";input.autocomplete="off";input.setAttribute("aria-label","New date");
  input.addEventListener("focus",()=>{if(typeof showOSKFor==="function")showOSKFor(input);});
  return input;
}
function managedScheduleLocked(){return typeof CTRL_LOCK_STATUS!=="undefined"&&!!(CTRL_LOCK_STATUS&&CTRL_LOCK_STATUS.enabled&&!CTRL_TOKEN);}
function managedScheduleOpenControl(){
  window.DASH_CONTROL_PENDING_SECTION={page:"calendars",lazy:"schedules"};
  closeScrim();
  const open=()=>{
    const pending=typeof lazyOpenCtrl==="function"?lazyOpenCtrl():openCtrl();
    Promise.resolve(pending).then(()=>{
      const route=()=>{
        if(typeof ctrlOpenPendingSection==="function"){
          if(ctrlOpenPendingSection())return;
          if(typeof CTRL_LOCK_STATUS!=="undefined"&&CTRL_LOCK_STATUS&&CTRL_LOCK_STATUS.enabled&&!CTRL_TOKEN)return;
        }
        setTimeout(route,20);
      };
      route();
    }).catch(()=>{});
  };
  setTimeout(open,45);
}
async function managedSchedulePost(body){
  const headers={"Content-Type":"application/json"};
  if(typeof CTRL_TOKEN!=="undefined"&&CTRL_TOKEN)headers["X-Dashboard-Token"]=CTRL_TOKEN;
  const response=await fetch("/api/household-schedules/override",{method:"POST",headers,body:JSON.stringify(body)});
  const payload=await response.json().catch(()=>({}));
  if(!response.ok)throw new Error(payload.error||"Household schedule could not be updated.");
  return payload;
}
async function managedScheduleRefresh(){
  if(typeof loadCalendars==="function")await loadCalendars();
  const open=document.querySelector('#ctrlpage-calendars details.ctrlsec[data-lazy="schedules"]');
  if(open&&open.open&&typeof renderCtrlHouseholdSchedules==="function")await renderCtrlHouseholdSchedules();
}
function managedScheduleShiftGrid(onShift){
  const grid=el("div","managed-schedule-shifts");
  for(const offset of [-7,-3,-2,-1,1,2,3,7]){
    const sign=offset>0?"+":"";
    grid.appendChild(managedScheduleButton(sign+offset+" day"+(Math.abs(offset)===1?"":"s"),"",()=>onShift(offset)));
  }
  return grid;
}
function showManagedSchedulePopup(ev){
  const info=managedScheduleInfo(ev);if(!info)return;
  popupOpenTransaction({mode:"managedschedulepop",title:ev.title||"Household schedule",when:()=>eventPopupWhen(ev),loading:"Opening schedule tools…"},()=>{
    const root=el("div","managed-schedule-popup");
    root.appendChild(el("p","managed-schedule-intro","Change this occurrence only. Its recurring rule and future occurrences stay the same."));
    const detail=el("div","managed-schedule-detail");
    detail.append(el("span","managed-schedule-label","Normally"),el("strong","",managedScheduleDateLabel(info.nominalDate)));
    if(info.actualDate&&info.actualDate!==info.nominalDate)detail.append(el("span","managed-schedule-note",`Currently ${managedScheduleDateLabel(info.actualDate)}${info.reason?` · ${info.reason}`:""}`));
    root.appendChild(detail);
    const message=el("div","managed-schedule-message");root.appendChild(message);
    const actions=el("div","managed-schedule-actions");root.appendChild(actions);
    if(managedScheduleLocked()){
      actions.appendChild(managedScheduleAction("Open Dashboard Control","Unlock Dashboard Control to manage this household schedule.","primary",managedScheduleOpenControl));
      return root;
    }
    const current=managedScheduleLocalDate(info.actualDate)||managedScheduleLocalDate(info.nominalDate);
    const date=managedScheduleDateInput(managedScheduleISO(current));
    const custom=el("label","managed-schedule-custom");custom.append(el("span","","Move to a date"),date);root.appendChild(custom);
    let busy=false;
    const commit=async(action,actualDate)=>{
      if(busy)return;busy=true;root.classList.add("busy");message.textContent="Saving schedule adjustment…";
      try{
        await managedSchedulePost({ruleId:info.ruleId,nominalDate:info.nominalDate,action,actualDate:actualDate||""});
        if(typeof hideOSK==="function")hideOSK();await managedScheduleRefresh();closeScrim();
      }catch(error){
        const text=error&&error.message?error.message:String(error||"");
        message.textContent=/locked|unlock|token/i.test(text)?"Unlock Dashboard Control, then try again.":text;
        if(/locked|unlock|token/i.test(text))actions.appendChild(managedScheduleAction("Open Dashboard Control","Unlock, then return here to change this occurrence.","",managedScheduleOpenControl));
        root.classList.remove("busy");busy=false;
      }
    };
    root.appendChild(el("div","managed-schedule-subhead","Move this occurrence"));
    root.appendChild(managedScheduleShiftGrid(offset=>commit("move",managedScheduleISO(addDays(current,offset)))));
    const customActions=el("div","managed-schedule-custom-actions");
    const move=managedScheduleButton("Save chosen date","primary",()=>{
      if(!/^\d{4}-\d{2}-\d{2}$/.test(date.value)){message.textContent="Use a date in YYYY-MM-DD format.";return;}
      commit("move",date.value);
    });
    const skip=managedScheduleConfirm("Skip this occurrence","Tap again to skip",()=>commit("skip",""));
    const clear=managedScheduleButton("Restore normal date","",()=>commit("clear",""));
    customActions.append(move,skip,clear);root.appendChild(customActions);
    actions.appendChild(managedScheduleAction("Edit recurring schedule","Change future paydays or pickup dates in Dashboard Control.","",managedScheduleOpenControl));
    date._oskSubmit=()=>move.click();date.dataset.oskSubmitLabel="Save";
    return root;
  });
}
