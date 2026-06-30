// Dashboard Control editor for Dash-Go-owned Paydays, Trash Pickup, and
// Recycling Pickup. It intentionally edits only the local generated feeds.
let HOUSEHOLD_SCHEDULES_STATE=null;
function hscRoot(){return document.querySelector("#ctrlpage-calendars #ctrlschedules");}
function hscClone(value){return JSON.parse(JSON.stringify(value||{}));}
function hscDate(){return typeof localDateISO==="function"?localDateISO():new Date().toISOString().slice(0,10);}
function hscRulePreview(data,id){return ((data.preview||{})[id]||[]).map(row=>{
  const actual=row.date||"",nominal=row.nominalDate||actual;
  const text=actual&&nominal&&actual!==nominal?`${actual} · normally ${nominal}${row.reason?` · ${row.reason}`:""}`:actual;
  return text;
}).filter(Boolean);}
function hscHolidayLabel(data,id){const found=(data.holidayLayers||[]).find(row=>row.id===id);return found?found.label:id;}
function hscScheduleSummary(rule,type){
  if(type==="payday"){
    if(rule.kind==="monthly-dates")return `Monthly · ${(rule.days||[]).join(" and ")}`;
    if(rule.kind==="nth-weekday"){const nth={1:"First",2:"Second",3:"Third",4:"Fourth","-1":"Last"}[rule.nth]||"";return `${nth} ${String(rule.weekday||"").replace(/^./,c=>c.toUpperCase())} each month`;}
    return `Every ${rule.everyWeeks||1} week${(+rule.everyWeeks||1)===1?"":"s"} from ${rule.start||"a known payday"}`;
  }
  return `Every ${rule.everyWeeks||1} week${(+rule.everyWeeks||1)===1?"":"s"} · ${String(rule.weekday||"").replace(/^./,c=>c.toUpperCase())}`;
}
function hscAdjustmentSummary(adj){
  if(!adj||adj.mode==="none")return "No automatic adjustment";
  const mode={"previous-business-day":"Previous business day","next-business-day":"Next business day","shift-forward":"Holiday shift forward","shift-backward":"Holiday shift back"}[adj.mode]||"Adjustment";
  const sources=(adj.holidayLayers||[]).length?" selected holidays":"";
  return mode+(adj.weekends?" for weekends":"")+sources;
}
function hscButton(label,cls,fn){return cbtn(label,cls||"",fn);}
function hscAction(label,detail,cls,fn){return caction(label,detail,cls||"",fn);}
async function hscFetch(){return api("/api/household-schedules");}
async function hscSave(next,message){
  const payload=await api("/api/household-schedules","POST",{schedules:next});
  HOUSEHOLD_SCHEDULES_STATE=payload;
  await loadCalendars();
  if(message)ctrlMsg(message);
  return payload;
}
function hscRuleCard(data,rule,type){
  const card=el("article","hsc-rule-card"+(rule.enabled===false?" off":""));
  const head=el("div","hsc-rule-head");
  const copy=el("div","hsc-rule-copy");copy.append(el("strong","",rule.label||"Household schedule"),el("span","",hscScheduleSummary(rule,type)),el("span","hsc-rule-adjust",hscAdjustmentSummary(rule.adjustment)));
  const state=el("span","hsc-rule-state "+(rule.enabled===false?"off":"on"),rule.enabled===false?"Paused":"Active");head.append(copy,state);card.appendChild(head);
  const upcoming=hscRulePreview(data,rule.id);if(upcoming.length)card.appendChild(el("div","hsc-preview","Next: "+upcoming.join(" · ")));
  const actions=el("div","hsc-rule-actions");
  actions.append(hscButton("Edit","",()=>hscEditRule(data,rule,type)),hscButton(rule.enabled===false?"Resume":"Pause",rule.enabled===false?"on":"",async()=>{
    const next=hscClone(data.schedules);const rows=type==="payday"?next.paydays:next.pickups;const target=rows.find(row=>row.id===rule.id);if(!target)return;target.enabled=target.enabled===false;await hscSave(next,`${target.label} ${target.enabled?"resumed":"paused"}.`);await renderCtrlHouseholdSchedules();
  }));
  if(type==="payday"){
    const remove=confirmAction("Remove","Remove this payday rule and its future generated events.","Tap again to remove",async()=>{
      const next=hscClone(data.schedules);next.paydays=next.paydays.filter(row=>row.id!==rule.id);next.overrides=next.overrides.filter(row=>row.ruleId!==rule.id);await hscSave(next,`${rule.label} removed.`);await renderCtrlHouseholdSchedules();
    });remove.classList.add("hsc-remove");actions.appendChild(remove);
  }
  card.appendChild(actions);return card;
}
function hscNewPayday(data){
  const used=new Set((data.schedules.paydays||[]).map(row=>row.id));let n=1,id="payday";
  while(used.has(id)){n++;id=`payday-${n}`;}
  return {id,label:n===1?"Payday":`Payday ${n}`,enabled:true,kind:"every-weeks",start:hscDate(),everyWeeks:2,days:[],nth:0,weekday:"",adjustment:{mode:"none",weekends:true,holidayLayers:[]}};
}
function hscNewPickup(id){return {id,label:id==="trash"?"Trash pickup":"Recycling pickup",enabled:true,weekday:"monday",everyWeeks:id==="trash"?1:2,start:"",adjustment:{mode:"none",days:1,weekends:false,holidayLayers:[]}};}
function hscChoice(root,options,get,set){
  const grid=el("div","hsc-choice-grid");root.appendChild(grid);
  const draw=()=>{grid.replaceChildren();for(const [value,label] of options){const b=hscButton(label,String(get())===String(value)?"on":"",()=>{set(value);draw();});b.setAttribute("aria-pressed",String(String(get())===String(value)));grid.appendChild(b);}};draw();return draw;
}
function hscField(label,input,detail){const field=el("label","hsc-field");field.append(el("span","hsc-field-label",label),input);if(detail)field.appendChild(el("span","hsc-field-detail",detail));return field;}
function hscMonthDays(rule){
  const root=el("div","hsc-day-picker");root.appendChild(el("div","hsc-field-label","Monthly date(s)"));const grid=el("div","hsc-month-day-grid");root.appendChild(grid);
  const draw=()=>{const selected=new Set(rule.days||[]);grid.replaceChildren();for(let day=1;day<=31;day++){const b=hscButton(String(day),selected.has(day)?"on":"",()=>{selected.has(day)?selected.delete(day):selected.add(day);rule.days=[...selected].sort((a,b)=>a-b);draw();});b.setAttribute("aria-pressed",String(selected.has(day)));grid.appendChild(b);}};draw();return root;
}
function hscHolidayLayers(data,adjustment){
  const root=el("div","hsc-holidays");root.appendChild(el("div","hsc-field-label","Holiday calendars to treat as non-business days"));
  if(!(data.holidayLayers||[]).length){root.appendChild(el("div","hsc-field-detail","No installed holiday calendars are available. You can still adjust for weekends."));return root;}
  const grid=el("div","hsc-choice-grid");root.appendChild(grid);const draw=()=>{const selected=new Set(adjustment.holidayLayers||[]);grid.replaceChildren();for(const layer of data.holidayLayers||[]){const b=hscButton(layer.label,selected.has(layer.id)?"on":"",()=>{selected.has(layer.id)?selected.delete(layer.id):selected.add(layer.id);adjustment.holidayLayers=[...selected];draw();});b.setAttribute("aria-pressed",String(selected.has(layer.id)));grid.appendChild(b);}};draw();return root;
}
function hscAdjustmentEditor(data,rule,type){
  const box=el("section","hsc-adjustment");box.appendChild(el("h4","","Automatic adjustment"));
  rule.adjustment=rule.adjustment||{mode:"none",weekends:type==="payday",holidayLayers:[]};
  const modes=type==="payday"?[["none","No adjustment"],["previous-business-day","Previous business day"],["next-business-day","Next business day"]]:[["none","No adjustment"],["shift-forward","Shift forward"],["shift-backward","Shift backward"],["previous-business-day","Previous business day"],["next-business-day","Next business day"]];
  const draw=()=>{box.replaceChildren(el("h4","","Automatic adjustment"));hscChoice(box,modes,()=>rule.adjustment.mode,value=>{rule.adjustment.mode=value;if((value==="previous-business-day"||value==="next-business-day")&&type==="payday")rule.adjustment.weekends=true;draw();});
    const business=rule.adjustment.mode==="previous-business-day"||rule.adjustment.mode==="next-business-day";
    if(business){const weekend=hscButton(rule.adjustment.weekends?"Weekends: adjust":"Weekends: ignore",rule.adjustment.weekends?"on":"",()=>{rule.adjustment.weekends=!rule.adjustment.weekends;draw();});box.appendChild(weekend);}
    if(rule.adjustment.mode==="shift-forward"||rule.adjustment.mode==="shift-backward"){const days=oskInput("days",String(rule.adjustment.days||1),{mode:"number"});days.maxLength=1;days._oskSubmit=()=>{rule.adjustment.days=Math.max(1,Math.min(7,+days.value||1));draw();};box.appendChild(hscField("Shift by 1–7 days",days,"Use this only for the selected holiday calendars."));}
    if(rule.adjustment.mode!=="none")box.appendChild(hscHolidayLayers(data,rule.adjustment));
  };draw();return box;
}
function hscEditRule(data,source,type){
  const root=hscRoot();if(!root)return;hideOSK();const rule=hscClone(source);root.replaceChildren();
  const form=el("div","hsc-editor");form.appendChild(el("h3","",source?`Edit ${rule.label}`:"Add payday"));
  const label=oskInput("label",rule.label,{mode:"text"});form.appendChild(hscField("Name",label,"This name appears on the calendar."));
  if(type==="payday"){
    const typeBox=el("section","hsc-editor-section");typeBox.appendChild(el("h4","","Schedule"));
    const draw=()=>{typeBox.replaceChildren(el("h4","","Schedule"));hscChoice(typeBox,[["every-weeks","Every N weeks"],["monthly-dates","Monthly date(s)"],["nth-weekday","Nth weekday"]],()=>rule.kind,value=>{rule.kind=value;if(value==="every-weeks"){rule.start=rule.start||hscDate();rule.everyWeeks=rule.everyWeeks||2;}if(value==="monthly-dates")rule.days=Array.isArray(rule.days)?rule.days:[];if(value==="nth-weekday"){rule.nth=rule.nth||1;rule.weekday=rule.weekday||"friday";}draw();});
      if(rule.kind==="every-weeks"){const start=oskInput("YYYY-MM-DD",rule.start||hscDate(),{mode:"date"}),every=oskInput("weeks",String(rule.everyWeeks||2),{mode:"number"});const sync=()=>{rule.start=start.value.trim();rule.everyWeeks=Math.max(1,Math.min(52,+every.value||1));};start.addEventListener("input",sync);every.addEventListener("input",sync);start._oskSubmit=sync;every._oskSubmit=sync;typeBox.append(hscField("Known payday",start),hscField("Every how many weeks",every));}
      else if(rule.kind==="monthly-dates")typeBox.appendChild(hscMonthDays(rule));
      else {const nthOptions=[[1,"First"],[2,"Second"],[3,"Third"],[4,"Fourth"],[-1,"Last"]],weekOptions=[["monday","Monday"],["tuesday","Tuesday"],["wednesday","Wednesday"],["thursday","Thursday"],["friday","Friday"],["saturday","Saturday"],["sunday","Sunday"]];typeBox.append(el("div","hsc-field-label","Which weekday?"));hscChoice(typeBox,nthOptions,()=>rule.nth||1,value=>rule.nth=+value);hscChoice(typeBox,weekOptions,()=>rule.weekday||"friday",value=>rule.weekday=value);}
    };draw();form.appendChild(typeBox);
  }else{
    const base=el("section","hsc-editor-section");base.appendChild(el("h4","","Recurring pickup"));const weekday=rule.weekday||"monday",week=rule.everyWeeks||1;rule.weekday=weekday;rule.everyWeeks=week;const dayOptions=[["monday","Mon"],["tuesday","Tue"],["wednesday","Wed"],["thursday","Thu"],["friday","Fri"],["saturday","Sat"],["sunday","Sun"]];hscChoice(base,dayOptions,()=>rule.weekday||weekday,value=>rule.weekday=value);const every=oskInput("weeks",String(week),{mode:"number"});const syncEvery=()=>{rule.everyWeeks=Math.max(1,Math.min(52,+every.value||1));};every.addEventListener("input",syncEvery);every._oskSubmit=syncEvery;base.appendChild(hscField("Every how many weeks",every,"Use 1 for weekly, 2 for every other week."));form.appendChild(base);
  }
  form.appendChild(hscAdjustmentEditor(data,rule,type));
  const actions=el("div","hsc-editor-actions");const save=hscButton("Save schedule","on",async()=>{
    rule.label=label.value.trim();if(!rule.label){ctrlMsg("Give this schedule a name.");showOSKFor(label);return;}
    if(type==="payday"&&rule.kind==="every-weeks"&&(!/^\d{4}-\d{2}-\d{2}$/.test(rule.start||""))){ctrlMsg("Choose a known payday date.");return;}
    if(type==="payday"&&rule.kind==="monthly-dates"&&!(rule.days||[]).length){ctrlMsg("Choose at least one monthly date.");return;}
    const next=hscClone(data.schedules);const rows=type==="payday"?next.paydays:next.pickups;const index=rows.findIndex(row=>row.id===rule.id);if(index<0)rows.push(rule);else rows[index]=rule;await hscSave(next,"Household schedule saved.");await renderCtrlHouseholdSchedules();
  });
  actions.append(save,hscButton("Cancel","",()=>renderCtrlHouseholdSchedules()));form.appendChild(actions);root.appendChild(form);oskSetSubmit(label,"Save",()=>save.click());
}
async function hscClearOverride(data,row){
  await api("/api/household-schedules/override","POST",{ruleId:row.ruleId,nominalDate:row.nominalDate,action:"clear"});await loadCalendars();ctrlMsg("Occurrence restored to its normal schedule.");await renderCtrlHouseholdSchedules();
}
async function renderCtrlHouseholdSchedules(){
  const root=hscRoot();if(!root)return;ctrlSetLoading(root,"Loading household schedules…","Reading local Paydays and pickup rules.");
  let data;try{data=await hscFetch();HOUSEHOLD_SCHEDULES_STATE=data;}catch(error){ctrlSetError(root,"Household schedules unavailable",friendlyUnavailable("Household schedules",error));return;}
  root.replaceChildren();
  root.appendChild(ctrlStateCard("info","Household Schedules","Paydays, Trash Pickup, and Recycling Pickup stay local. Use an event’s Manage schedule action for a one-time move or skip."));
  if(data.migrated)root.appendChild(ctrlStateCard("info","Existing schedule ready","Your installer-created payday and pickup settings are ready to edit here. Saving keeps them in the new local format."));
  const paydays=data.schedules.paydays||[];const paydaySection=el("section","hsc-section");paydaySection.append(el("h3","","Paydays"),el("p","","Add as many named paydays as your household needs. A rule may use multiple monthly dates, a week interval, or an nth weekday."));
  for(const rule of paydays)paydaySection.appendChild(hscRuleCard(data,rule,"payday"));paydaySection.appendChild(hscAction("Add payday rule","Create another local paycheck schedule.","primary",()=>hscEditRule(data,hscNewPayday(data),"payday")));root.appendChild(paydaySection);
  const pickupSection=el("section","hsc-section");pickupSection.append(el("h3","","Pickup days"),el("p","","Pause, correct, or edit only Dash-Go’s local Trash and Recycling calendars. Imported calendars are never changed here."));
  for(const id of ["trash","recycling"]){const rule=(data.schedules.pickups||[]).find(row=>row.id===id);if(rule)pickupSection.appendChild(hscRuleCard(data,rule,"pickup"));else pickupSection.appendChild(hscAction(id==="trash"?"Set up Trash Pickup":"Set up Recycling Pickup","Add this local household calendar when you are ready.","",()=>hscEditRule(data,hscNewPickup(id),"pickup")));}root.appendChild(pickupSection);
  const overrides=data.schedules.overrides||[];if(overrides.length){const adjusted=el("section","hsc-section hsc-adjusted");adjusted.append(el("h3","","One-time adjustments"),el("p","","These move or skip a single occurrence. Restoring one returns it to its recurring rule."));for(const row of overrides){const name=[...paydays,...(data.schedules.pickups||[])].find(rule=>rule.id===row.ruleId)?.label||row.ruleId;const card=el("div","hsc-override-row");card.append(el("div","",`${name} · normally ${row.nominalDate}${row.action==="skip"?" · skipped":` · moved to ${row.actualDate}`}`),hscButton("Restore","",()=>hscClearOverride(data,row)));adjusted.appendChild(card);}root.appendChild(adjusted);}
}
