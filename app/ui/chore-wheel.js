// chore-wheel.js — local-only, lazy Chore Wheel overlay. Depends on chore-wheel-core.js.
(function(){
"use strict";
const core=window.ChoreWheelCore;
if(!core){console.warn("Chore Wheel core did not load");return;}
const $=id=>document.getElementById(id);
const WHEEL_SPIN_MS=2450,WHEEL_SPIN_TURNS=6;
const state={data:core.normalize({}),view:"today",busy:false,openToken:0,spinTimer:0,spin:null,spinSerial:0,confirm:null,form:null,formFocusSerial:0,pendingFormFocus:0,priorFocus:null,status:"",renderIndex:null};
const root=()=>$("chorewheel");
const body=()=>$("chorewheel-body");
const tabs=()=>$("chorewheel-tabs");
const todayKey=()=>core.localDateKey(new Date());
const make=(tag,className,text)=>{const node=document.createElement(tag);if(className)node.className=className;if(text!==undefined&&text!==null)node.textContent=String(text);return node;};
const append=(parent,...children)=>{for(const child of children.flat()){if(child)parent.appendChild(child);}return parent;};

async function api(path,method,bodyValue){
  const options={method:method||"GET",headers:{Accept:"application/json"}};
  if(bodyValue){options.headers["Content-Type"]="application/json";options.body=JSON.stringify(bodyValue);}
  const response=await fetch(path,options);
  const payload=await response.json().catch(()=>({}));
  if(!response.ok){const error=new Error(payload.error||"Chore Wheel is unavailable.");error.status=response.status;throw error;}
  return payload;
}
async function reloadAfterChoreConflict(){
  state.data=core.normalize(await api("/api/chore-wheel"));
  state.status="Chores changed elsewhere. Loaded the latest plan.";
}
function isOpen(){return !!(root()&&root().classList.contains("show"));}
function setStatus(text){state.status=String(text||"");const node=$("chorewheel-status");if(node)node.textContent=state.status;}
function setBusy(on){
  state.busy=!!on;const shell=root();if(!shell)return;
  shell.classList.toggle("busy",state.busy);shell.setAttribute("aria-busy",state.busy?"true":"false");
  shell.querySelectorAll("button,select").forEach(node=>{node.disabled=state.busy||node.dataset.cwLocked==="1";});
}
function refreshRenderIndex(){state.renderIndex=core.choreIndexes(state.data,todayKey());return state.renderIndex;}
function renderIndex(){return state.renderIndex||refreshRenderIndex();}
function activeAssignment(item){return (item.status||"assigned")==="assigned";}
function lookupChore(id){return renderIndex().choreByID.get(id)||null;}
function lookupPerson(id){return renderIndex().peopleByID.get(id)||null;}
function dueToday(){return core.dueChores(state.data,todayKey());}
function assignmentAt(choreId,key){return core.assignmentFor(state.data,choreId,key,renderIndex());}
function eligibleFor(chore){return core.eligiblePeople(state.data,chore,renderIndex());}
function unassignedDue(){return dueToday().filter(chore=>!assignmentAt(chore.id,todayKey()));}
function nextDue(){return unassignedDue()[0]||null;}
function calendarOutputEnabled(){return state.data?.settings?.calendarOutputEnabled!==false;}
function actionButton(label,action,data,options){
  const node=make("button","",label);node.type="button";node.dataset.cwAction=action;
  for(const [key,value] of Object.entries(data||{})){if(value!==undefined&&value!==null)node.dataset[key]=String(value);}
  const config=options||{};
  if(config.locked)node.dataset.cwLocked="1";
  if(config.ariaPressed!==undefined)node.setAttribute("aria-pressed",String(config.ariaPressed));
  if(config.describedBy)node.setAttribute("aria-describedby",config.describedBy);
  node.disabled=!!config.disabled||state.busy||node.dataset.cwLocked==="1";
  bindTap(node,()=>onAction(node));
  return node;
}
function statusChip(item){
  const status=item.status||"assigned";
  return make("span",`cw-status cw-status-${status}`,status==="completed"?"Done":status==="skipped"?"Skipped":"Assigned");
}
function calendarOutputNotice(){
  if(calendarOutputEnabled())return null;
  const card=make("section","cw-card cw-calendar-output-off");
  const actions=make("div","cw-primary-actions");
  actions.appendChild(actionButton("Enable Chores calendar","enable-calendar-output"));
  card.append(make("div","cw-card-title","Calendar output off"),make("p","cw-note","Chore assignments remain saved locally. Enable output to rebuild the Chores calendar from your current plan."),actions);
  return card;
}
function wheelBackground(candidates){
  const colors=["#7fc4c4","#d9c074","#8bb4d4","#d99a9a","#9a8fb0","#cda76a"];
  if(!candidates.length)return "var(--card)";
  const width=100/candidates.length;
  return `conic-gradient(${candidates.map((person,index)=>`${colors[index%colors.length]} ${(index*width).toFixed(2)}% ${((index+1)*width).toFixed(2)}%`).join(",")})`;
}
function activeSpinFor(chore){const spin=state.spin;return spin&&chore&&spin.choreId===chore.id?spin:null;}
function spinCandidates(spin){return spin?(spin.candidateIds||[]).map(lookupPerson).filter(Boolean):[];}
function spinInProgress(){return !!(state.spin&&state.spin.phase!=="complete");}
function spinTargetDegrees(count,index){return 360*WHEEL_SPIN_TURNS-(360/count)*(index+.5);}
function renderWheel(chore,candidates){
  const spin=activeSpinFor(chore),phase=spin?spin.phase:"idle",winner=spin?lookupPerson(spin.winnerId):null;
  const label=phase==="saving"?"Preparing wheel…":(phase==="arming"||phase==="spinning"?"Spinning…":winner?`${chore?chore.name:"Chore"}\n${winner.name}`:chore?chore.name:"No due chore");
  const segmentClass=`cw-wheel-segments${phase==="arming"?" cw-spin-stage":phase==="spinning"?" cw-spinning":phase==="complete"?" cw-spin-complete":""}`;
  const wheelClass=`cw-wheel${phase==="arming"||phase==="spinning"?" cw-wheel-spinning":""}${phase==="complete"?" cw-wheel-complete":""}`;
  const card=make("section","cw-card cw-wheel-card"),frame=make("div","cw-wheel-frame"),wheel=make("div",wheelClass),segments=make("div",segmentClass),wheelLabel=make("div","cw-wheel-label",label),legend=make("div","cw-candidates");
  wheel.id="cwwheel";wheel.dataset.cwSpinPhase=phase;segments.id="cwwheelsegments";segments.style.background=wheelBackground(candidates);segments.style.transform=`rotate(${phase==="complete"?Number(spin.targetDegrees)||0:0}deg)`;
  wheel.append(segments,wheelLabel);frame.appendChild(wheel);
  if(candidates.length)candidates.forEach(person=>legend.appendChild(make("span","",person.name)));else legend.textContent="No eligible people.";
  card.append(make("div","cw-card-title","Next assignment"),frame,legend,make("p","cw-note",chore?`${chore.name} · ${core.cadenceText(chore.cadence)}`:"Add people and chores to begin."));
  return card;
}
function assignmentControls(item,pending){
  const actions=make("div","cw-row-actions");
  if(pending){actions.appendChild(make("span","cw-note cw-spin-pending","Wheel spinning…"));return actions;}
  if(!activeAssignment(item)){actions.appendChild(actionButton("Remove","remove-assignment",{id:item.id}));return actions;}
  actions.append(actionButton("Done","complete-assignment",{id:item.id}),actionButton("Skip","skip-assignment",{id:item.id}),actionButton("Reassign","reassign-assignment",{id:item.id}),actionButton("More","remove-assignment",{id:item.id}));
  return actions;
}
function assignmentRow(chore,item,actionLocked){
  const row=make("article","cw-assignment"),copy=make("div");
  copy.appendChild(make("strong","",chore.name));
  if(item){
    const pending=!!activeSpinFor(chore)&&spinInProgress();
    const detail=make("span","",pending?"Wheel spinning…":item.personName||"Unassigned");
    if(!pending)detail.appendChild(document.createTextNode(" ")),detail.appendChild(statusChip(item));
    copy.appendChild(detail);row.append(copy,assignmentControls(item,pending));return row;
  }
  const people=eligibleFor(chore);
  if(!people.length){row.classList.add("cw-warning");copy.append(make("strong","",chore.name),make("span","","No eligible person"));row.append(copy,actionButton("Edit chore","manage-chore",{id:chore.id}));return row;}
  copy.append(make("strong","",chore.name),make("span","",`Unassigned · ${core.cadenceText(chore.cadence)}`));
  const actions=make("div","cw-row-actions"),select=make("select");select.dataset.cwManual=chore.id;select.disabled=state.busy||actionLocked;
  select.appendChild(Object.assign(make("option","","Assign manually"),{value:""}));
  people.forEach(person=>{const option=make("option","",person.name);option.value=person.id;select.appendChild(option);});
  actions.append(select,actionButton("Assign","manual-assignment",{id:chore.id},{disabled:actionLocked,locked:actionLocked}));row.append(copy,actions);return row;
}
function upcomingRows(days){
  const fragment=document.createDocumentFragment();let count=0;
  for(let offset=1;offset<=days;offset++){
    const key=core.addDays(todayKey(),offset),chores=core.dueChores(state.data,key);
    if(!chores.length)continue;
    const row=make("div","cw-upcoming-row"),line=chores.map(chore=>{const item=assignmentAt(chore.id,key);return `${chore.name} — ${item?item.personName||"Unassigned":"Unassigned"}`;}).join(" · ");
    row.append(make("strong","",key),make("span","",line));fragment.appendChild(row);count++;
  }
  if(!count)fragment.appendChild(make("div","cw-empty","No upcoming chores in the next few days."));
  return fragment;
}
function renderToday(){
  const fragment=document.createDocumentFragment(),due=dueToday(),next=nextDue(),spin=state.spin,displayChore=spin?(lookupChore(spin.choreId)||next):next,candidates=spin?spinCandidates(spin):(displayChore?eligibleFor(displayChore):[]),actionLocked=spinInProgress();
  const grid=make("div","cw-today-grid"),left=make("div"),primary=make("div","cw-primary-actions"),todayCard=make("section","cw-card");
  primary.append(actionButton("Spin next due chore","spin-next",{}, {disabled:!next||!eligibleFor(next).length||actionLocked,locked:!next||!eligibleFor(next).length||actionLocked}),actionButton("Assign all remaining today","assign-remaining",{}, {disabled:!next||actionLocked,locked:!next||actionLocked}));
  left.append(renderWheel(displayChore,candidates),primary,make("p","cw-note","Assigning remaining chores does not animate and keeps existing choices."));
  todayCard.appendChild(make("div","cw-card-title","Today’s chores"));
  if(!due.length)todayCard.appendChild(make("div","cw-empty","No chores are due today."));else due.forEach(chore=>todayCard.appendChild(assignmentRow(chore,assignmentAt(chore.id,todayKey()),actionLocked)));
  grid.append(left,todayCard);fragment.appendChild(grid);
  const upcoming=make("section","cw-card cw-upcoming"),upcomingActions=make("div","cw-upcoming-actions");upcomingActions.appendChild(actionButton("View plan","view-plan",{}, {disabled:actionLocked,locked:actionLocked}));upcoming.append(make("div","cw-card-title","Upcoming"),upcomingRows(4),upcomingActions);fragment.appendChild(upcoming);
  return fragment;
}
function renderPlan(){
  const horizon=Math.max(1,Math.min(30,Number(state.data.settings.horizonDays)||14)),fragment=document.createDocumentFragment(),controls=make("section","cw-card"),horizonButtons=make("div","cw-horizon"),primary=make("div","cw-primary-actions"),list=make("section","cw-card");
  controls.append(make("div","cw-card-title","Plan ahead"),make("p","cw-note","Existing assignments, completions, skips, and manual choices are kept."));
  const selection=make("p","cw-plan-selection");selection.append(make("strong","",`Planning horizon: ${horizon} days.`),document.createTextNode(" Choose how far ahead to view and create missing assignments."));controls.appendChild(selection);
  for(const days of [7,14,30])horizonButtons.appendChild(actionButton(`${days} days`,`set-horizon`,{days},{ariaPressed:days===horizon}));
  primary.appendChild(actionButton("Generate missing assignments","generate-plan"));controls.append(horizonButtons,primary);fragment.appendChild(controls);
  list.appendChild(make("div","cw-card-title",`Next ${horizon} days`));let count=0;
  for(let offset=0;offset<horizon;offset++){
    const key=core.addDays(todayKey(),offset);
    for(const chore of core.dueChores(state.data,key)){
      const item=assignmentAt(chore.id,key),row=make("div","cw-plan-row");row.append(make("strong","",key),make("span","",chore.name),make("span","",item?item.personName||"Unassigned":"Unassigned"));if(item)row.appendChild(statusChip(item));list.appendChild(row);count++;
    }
  }
  if(!count)list.appendChild(make("div","cw-empty","No chores scheduled in this range."));fragment.appendChild(list);return fragment;
}
function formLabel(labelText,input){const label=make("label","",labelText);label.appendChild(input);return label;}
function selectOption(value,label,selected){const option=make("option","",label);option.value=value;option.selected=!!selected;return option;}
function choreForm(){
  const editing=state.form&&state.form.kind==="chore"?lookupChore(state.form.id):null,cadence=editing?editing.cadence:{type:"daily",day:0,every:1},selected=new Set(editing?editing.eligible:state.data.people.map(person=>person.id));
  const form=make("section","cw-form"),name=make("input","oskfield"),cadenceSelect=make("select"),weekday=make("select"),every=make("input","oskfield"),effort=make("select"),fieldset=make("fieldset"),actions=make("div","cw-row-actions");
  name.id="cw-chore-name";name.dataset.oskMode="text";name.value=editing?editing.name:"";name.maxLength=96;form.appendChild(formLabel("Chore name",name));
  cadenceSelect.id="cw-cadence";[["daily","Daily"],["weekdays","Weekdays"],["weekly","Weekly"],["days","Every N days"]].forEach(([value,label])=>cadenceSelect.appendChild(selectOption(value,label,cadence.type===value)));form.appendChild(formLabel("Cadence",cadenceSelect));
  weekday.id="cw-weekday";core.DAY_NAMES.forEach((label,index)=>weekday.appendChild(selectOption(String(index),label,Number(cadence.day)===index)));const weekdayLabel=formLabel("Weekly day",weekday);weekdayLabel.classList.add("cw-weekday-field");form.appendChild(weekdayLabel);
  every.id="cw-every";every.dataset.oskMode="numbers";every.inputMode="numeric";every.value=String(Math.max(1,Number(cadence.every)||1));const everyLabel=formLabel("Every N days",every);everyLabel.classList.add("cw-every-field");form.appendChild(everyLabel);
  effort.id="cw-effort";[["1","Quick"],["2","Normal"],["3","Big"]].forEach(([value,label])=>effort.appendChild(selectOption(value,label,Number(editing?.effort||2)===Number(value))));form.appendChild(formLabel("Effort",effort));
  fieldset.appendChild(make("legend","","Eligible people"));
  if(!state.data.people.length)fieldset.appendChild(make("p","cw-note","Manage People in Dashboard Control before adding an eligible person."));
  state.data.people.forEach(person=>{const label=make("label","cw-check"),check=make("input");check.type="checkbox";check.dataset.cwEligible="1";check.value=person.id;check.checked=selected.has(person.id);label.append(check,document.createTextNode(person.name));fieldset.appendChild(label);});
  actions.append(actionButton("Manage people","manage-people"),actionButton(editing?"Save chore":"Add chore","save-chore"),actionButton("Cancel","cancel-form"));form.append(fieldset,actions);return form;
}
function renderManage(){
  const fragment=document.createDocumentFragment(),peopleCard=make("section","cw-card"),choresCard=make("section","cw-card"),form=state.form;
  peopleCard.append(make("div","cw-card-title","People"),make("p","cw-note","People are shared with Routines, To Do, Grocery, and Maintenance. Manage the household roster in Dashboard Control."));
  if(!state.data.people.length)peopleCard.appendChild(make("div","cw-empty","No household people are configured yet."));
  else state.data.people.forEach(person=>{const row=make("div","cw-manage-row"),copy=make("span");copy.append(make("span","",person.name),make("small","","Shared household person"));row.appendChild(copy);peopleCard.appendChild(row);});
  const peopleActions=make("div","cw-primary-actions");peopleActions.appendChild(actionButton("Manage people","manage-people"));peopleCard.appendChild(peopleActions);
  choresCard.appendChild(make("div","cw-card-title","Chores"));
  if(form&&form.kind==="chore")choresCard.appendChild(choreForm());
  else if(!state.data.people.length){const notice=make("p","cw-prereq","Add people in Dashboard Control. Each chore needs at least one eligible person.");notice.id="cw-chore-prereq";const actions=make("div","cw-primary-actions");actions.append(actionButton("Manage people","manage-people"),actionButton("Add chore","add-chore-form",{}, {disabled:true,locked:true,describedBy:"cw-chore-prereq"}));choresCard.append(notice,actions);}
  else {if(!state.data.chores.length)choresCard.appendChild(make("div","cw-empty","Add chores with their own cadence."));else state.data.chores.forEach(chore=>{const row=make("div","cw-manage-row"),copy=make("span"),actions=make("div");copy.append(make("strong","",chore.name),make("small","",`${core.cadenceText(chore.cadence)} · ${chore.effort===3?"Big":chore.effort===1?"Quick":"Normal"} · ${eligibleFor(chore).length||0} eligible`));actions.append(actionButton("Edit","edit-chore",{id:chore.id}),actionButton("Remove","remove-chore",{id:chore.id}));row.append(copy,actions);choresCard.appendChild(row);});const actions=make("div","cw-primary-actions");actions.appendChild(actionButton("Add chore","add-chore-form"));choresCard.appendChild(actions);}
  fragment.append(peopleCard,choresCard);return fragment;
}
function renderHistory(){
  const card=make("section","cw-card");card.appendChild(make("div","cw-card-title","History"));const rows=core.recentAssignments(state.data,60);
  if(!rows.length)card.appendChild(make("div","cw-empty","No assignments yet."));else rows.forEach(item=>{const row=make("article","cw-assignment"),copy=make("div");copy.append(make("strong","",`${item.date} · ${item.choreName||"Chore"} — ${item.personName||"Unassigned"}`),statusChip(item));row.append(copy,assignmentControls(item,false));card.appendChild(row);});return card;
}
function confirmPanel(){
  const prompt=state.confirm;if(!prompt)return null;const layer=make("section","cw-confirm"),card=make("div","cw-card"),actions=make("div","cw-row-actions");layer.setAttribute("role","alertdialog");layer.setAttribute("aria-modal","true");card.append(make("strong","",prompt.title),make("p","",prompt.detail));actions.appendChild(actionButton("Cancel","cancel-confirm"));
  if(Array.isArray(prompt.choices)&&prompt.choices.length)prompt.choices.forEach(choice=>actions.appendChild(actionButton(choice.label,"accept-confirm-choice",{choice:choice.id})));
  else actions.appendChild(actionButton(prompt.confirm,"accept-confirm"));card.appendChild(actions);layer.appendChild(card);return layer;
}
function loadingView(){const loading=make("div","cw-loading","Loading Chore Wheel…");loading.setAttribute("role","status");return loading;}
function errorView(message){const card=make("section","cw-card cw-error"),actions=make("div","cw-row-actions");card.append(make("strong","","Chore Wheel is unavailable."),make("p","",message));actions.append(actionButton("Retry","retry"),actionButton("Close","close"));card.appendChild(actions);return card;}
function render(){
  if(!isOpen())return;refreshRenderIndex();const tabHost=tabs();tabHost.replaceChildren();for(const view of ["today","plan","manage","history"]){tabHost.appendChild(actionButton(view[0].toUpperCase()+view.slice(1),"view",{view},{ariaPressed:view===state.view}));}
  const content=body();content.replaceChildren();const notice=calendarOutputNotice();if(notice)content.appendChild(notice);content.appendChild(state.view==="plan"?renderPlan():state.view==="manage"?renderManage():state.view==="history"?renderHistory():renderToday());const confirm=confirmPanel();if(confirm)content.appendChild(confirm);setStatus(state.status||"Local, fair chore rotations · calendar-backed");connectInputs();
}
function revealOskForInput(input){
  if(!input||input.disabled||input.readOnly||input.dataset.cwOskPending==="1")return;input.dataset.cwOskPending="1";
  requestAnimationFrame(()=>{delete input.dataset.cwOskPending;const shell=root();if(!shell||!isOpen()||!shell.contains(input)||input.disabled||input.readOnly)return;if(document.activeElement!==input){try{input.focus({preventScroll:true});}catch(_){input.focus();}}if(typeof showOSKFor==="function")showOSKFor(input);});
}
function queueFormFocus(){state.pendingFormFocus=++state.formFocusSerial;}
function focusPendingFormField(){
  const request=state.pendingFormFocus,form=state.form;if(!request||!form)return;state.pendingFormFocus=0;const inputId=form.kind==="person"?"cw-person-name":"cw-chore-name";
  const activate=()=>{if(state.formFocusSerial!==request||!state.form||!isOpen())return;const input=$(inputId);if(input)revealOskForInput(input);};requestAnimationFrame(()=>{activate();setTimeout(activate,110);});
}
function connectInputs(){
  root().querySelectorAll(".oskfield").forEach(input=>{input.addEventListener("focus",()=>revealOskForInput(input));input.addEventListener("pointerup",event=>{if(event.isPrimary===false||(event.button!=null&&event.button!==0))return;revealOskForInput(input);});});
  const cadence=$("cw-cadence");if(cadence){cadence.addEventListener("change",syncCadenceFields);syncCadenceFields();}focusPendingFormField();
}
function syncCadenceFields(){const type=$("cw-cadence")?.value;root().querySelectorAll(".cw-weekday-field").forEach(node=>node.hidden=type!=="weekly");root().querySelectorAll(".cw-every-field").forEach(node=>node.hidden=type!=="days");}
async function saveChange(label,mutate){
  if(state.busy)return false;const before=JSON.parse(JSON.stringify(state.data));
  try{mutate(state.data);state.data=core.normalize(state.data);setBusy(true);setStatus(label);state.data=core.normalize(await api("/api/chore-wheel","POST",state.data));state.status=label;return true;}
  catch(error){if(error&&error.status===409){try{await reloadAfterChoreConflict();}catch(reloadError){state.data=before;state.status=reloadError.message||"Could not reload Chore Wheel.";}}else{state.data=before;state.status=error.message||"Could not save Chore Wheel.";}return false;}
  finally{setBusy(false);render();}
}
async function setCalendarOutput(enabled){
  if(state.busy)return;setBusy(true);setStatus(enabled?"Enabling Chores calendar…":"Stopping Chores calendar output…");
  try{const result=await api("/api/calendars/manage/app-output","POST",{owner:"chore-wheel",enabled:!!enabled});state.data=core.normalize(result.state||state.data);state.status=enabled?"Chores calendar output enabled.":"Chores calendar output stopped. Assignments remain local.";}
  catch(error){state.status=error.message||"Could not change Chores calendar output.";}finally{setBusy(false);render();}
}
async function setPlanningHorizon(days){
  if(state.busy)return;const target=Math.max(1,Math.min(30,Number(days)||14));if(Number(state.data.settings.horizonDays)===target){state.status=`Planning horizon: ${target} days.`;render();return;}const before=JSON.parse(JSON.stringify(state.data));state.data.settings.horizonDays=target;state.data=core.normalize(state.data);state.status=`Planning horizon: ${target} days.`;render();setBusy(true);
  try{state.data=core.normalize(await api("/api/chore-wheel","POST",state.data));}catch(error){if(error&&error.status===409){try{await reloadAfterChoreConflict();}catch(reloadError){state.data=before;state.status=reloadError.message||"Could not reload Chore Wheel.";}}else{state.data=before;state.status=error.message||"Could not save planning horizon.";}}finally{setBusy(false);render();}
}
function idFor(prefix){return `${prefix}-${Date.now()}-${Math.random().toString(36).slice(2,7)}`;}
function openForm(kind,id){if(typeof hideOSK==="function")hideOSK();state.form={kind,id:id||""};queueFormFocus();render();}
function setConfirm(title,detail,confirm,run){state.confirm={title,detail,confirm,run};render();}
async function spinNext(){
  if(spinInProgress())return;const chore=nextDue(),key=todayKey(),candidates=chore?eligibleFor(chore):[],winner=chore?core.chooseFairCandidate(state.data,chore,key,""):null;
  if(!chore||!winner){state.status="Choose eligible people before spinning.";render();return;}const index=candidates.findIndex(person=>person.id===winner.id);if(index<0){state.status="Could not place the selected person on the wheel.";render();return;}
  const spin={id:++state.spinSerial,choreId:chore.id,winnerId:winner.id,candidateIds:candidates.map(person=>person.id),targetDegrees:spinTargetDegrees(candidates.length,index),phase:"saving"};state.spin=spin;state.status=`Preparing ${chore.name}…`;render();
  if(await saveChange("Locking assignment…",data=>data.assignments.push(core.makeAssignment(chore,winner,key,"spin"))))armWheelSpin(spin);else if(state.spin===spin){state.spin=null;render();}
}
async function assignRemaining(){
  const key=todayKey();let count=0;const changed=await saveChange("Assigning remaining chores…",data=>{const plan=core.choreIndexes(data,key);core.dueChores(data,key).filter(chore=>!core.assignmentFor(data,chore.id,key,plan)).forEach(chore=>{const winner=core.chooseFairCandidate(data,chore,key,"",plan);if(winner){data.assignments.push(core.makeAssignment(chore,winner,key,"batch"));core.noteFairAssignment(plan,chore,winner,key);count++;}});});
  if(changed){state.status=count?`Assigned ${count} chore${count===1?"":"s"}.`:"No remaining chores could be assigned.";render();}
}
function finishWheelSpin(spin){if(state.spin!==spin||!isOpen())return;clearTimeout(state.spinTimer);state.spinTimer=0;spin.phase="complete";const chore=lookupChore(spin.choreId),winner=lookupPerson(spin.winnerId);state.status=chore&&winner?`Assigned ${winner.name} to ${chore.name}.`:"Assignment saved.";render();}
function armWheelSpin(spin){
  if(state.spin!==spin||!isOpen())return;if(window.matchMedia&&window.matchMedia("(prefers-reduced-motion: reduce)").matches){finishWheelSpin(spin);return;}spin.phase="arming";state.status="Spinning the wheel…";render();
  requestAnimationFrame(()=>requestAnimationFrame(()=>{if(state.spin!==spin||!isOpen())return;const wheel=$("cwwheelsegments");if(!wheel){finishWheelSpin(spin);return;}wheel.classList.remove("cw-spin-stage");wheel.classList.add("cw-spinning");spin.phase="spinning";wheel.style.transform=`rotate(${spin.targetDegrees}deg)`;const finish=()=>finishWheelSpin(spin);wheel.addEventListener("transitionend",event=>{if(event.propertyName==="transform")finish();},{once:true});clearTimeout(state.spinTimer);state.spinTimer=setTimeout(finish,WHEEL_SPIN_MS+300);}));
}
async function generatePlan(){
  const horizon=Math.max(1,Math.min(30,Number(state.data.settings.horizonDays)||14));let created=0,kept=0;const changed=await saveChange("Generating missing assignments…",data=>{for(let offset=0;offset<horizon;offset++){const key=core.addDays(todayKey(),offset),plan=core.choreIndexes(data,key);core.dueChores(data,key).forEach(chore=>{if(core.assignmentFor(data,chore.id,key,plan)){kept++;return;}const winner=core.chooseFairCandidate(data,chore,key,"",plan);if(winner){data.assignments.push(core.makeAssignment(chore,winner,key,"plan"));core.noteFairAssignment(plan,chore,winner,key);created++;}});}});
  if(changed){state.status=`Created ${created} assignment${created===1?"":"s"}; ${kept} existing choice${kept===1?"":"s"} kept.`;render();}
}
function manualAssignment(choreId){const personId=[...root().querySelectorAll("[data-cw-manual]")].find(node=>node.dataset.cwManual===choreId)?.value||"",chore=lookupChore(choreId),person=lookupPerson(personId);if(!chore||!person)return;saveChange(`Assigned ${person.name} to ${chore.name}.`,data=>data.assignments.push(core.makeAssignment(chore,person,todayKey(),"manual")));}
function assignmentMutation(id,kind){
  const item=state.data.assignments.find(row=>row.id===id);if(!item)return;
  if(kind==="remove")return setConfirm(`Remove “${item.choreName||"Chore"} — ${item.personName||"Unassigned"}”?`,"This removes this calendar assignment. The chore remains available for scheduling.","Remove assignment",()=>saveChange("Assignment removed.",data=>{data.assignments=data.assignments.filter(row=>row.id!==id);}));
  if(kind==="reassign"){const chore=lookupChore(item.choreId),winner=chore&&core.chooseFairCandidate(state.data,chore,item.date,item.personId);if(!winner){state.status="No alternate eligible person is available.";render();return;}return saveChange(`Reassigned ${chore.name} to ${winner.name}.`,data=>{const row=data.assignments.find(value=>value.id===id);row.personId=winner.id;row.personName=winner.name;row.status="assigned";});}
  saveChange(kind==="complete"?"Chore marked done.":"Chore skipped.",data=>{const row=data.assignments.find(value=>value.id===id);row.status=kind==="complete"?"completed":"skipped";});
}
function openPeopleControl(origin){
  if(typeof window.openDashboardPeopleControl!=="function"){state.status="People management is unavailable until Dashboard Control loads.";render();return;}const view=state.view;window.openDashboardPeopleControl({origin:origin||document.activeElement,close:closeChoreWheel,reopen:()=>openChoreWheel().then(()=>{state.view=view;render();})}).catch(error=>{state.status="Could not open People · "+(error&&error.message?error.message:"try again");render();});
}
function onAction(buttonNode){
  const action=buttonNode.dataset.cwAction;if(state.busy||spinInProgress())return;
  if(action==="spin-next")return spinNext();if(action==="assign-remaining")return assignRemaining();if(action==="view-plan"){state.view="plan";render();return;}
  if(action==="view"){state.view=buttonNode.dataset.view;state.form=null;state.confirm=null;if(state.view!=="manage"&&typeof hideOSK==="function")hideOSK();render();return;}
  if(action==="enable-calendar-output")return setCalendarOutput(true);if(action==="set-horizon")return setPlanningHorizon(buttonNode.dataset.days);if(action==="generate-plan")return generatePlan();if(action==="manage-people")return openPeopleControl(buttonNode);if(action==="add-chore-form")return openForm("chore");
  if(action==="edit-chore"||action==="manage-chore"){state.view="manage";return openForm("chore",buttonNode.dataset.id);}if(action==="cancel-form"){state.form=null;state.pendingFormFocus=0;state.formFocusSerial++;if(typeof hideOSK==="function")hideOSK();render();return;}
  if(action==="save-chore")return saveChore();if(action==="manual-assignment")return manualAssignment(buttonNode.dataset.id);if(action==="complete-assignment")return assignmentMutation(buttonNode.dataset.id,"complete");if(action==="skip-assignment")return assignmentMutation(buttonNode.dataset.id,"skip");if(action==="reassign-assignment")return assignmentMutation(buttonNode.dataset.id,"reassign");if(action==="remove-assignment")return assignmentMutation(buttonNode.dataset.id,"remove");if(action==="remove-chore")return removeChore(buttonNode.dataset.id);
  if(action==="retry"){closeChoreWheel();return openChoreWheel();}if(action==="close")return closeChoreWheel();if(action==="cancel-confirm"){state.confirm=null;render();return;}if(action==="accept-confirm"){const run=state.confirm&&state.confirm.run;state.confirm=null;return run&&run();}if(action==="accept-confirm-choice"){const choice=(state.confirm&&state.confirm.choices||[]).find(item=>item.id===buttonNode.dataset.choice);state.confirm=null;return choice&&choice.run&&choice.run();}
}
function saveChore(){
  const name=$("cw-chore-name")?.value.trim()||"";if(!name){state.status="Enter a chore name.";render();return;}const type=$("cw-cadence")?.value||"daily",editing=state.form&&state.form.id,eligible=[...root().querySelectorAll("[data-cw-eligible]:checked")].map(input=>input.value),existing=editing&&lookupChore(editing),cadence={type,day:Number($("cw-weekday")?.value)||0,every:Math.max(1,Number($("cw-every")?.value)||1),anchorDate:existing?.cadence?.anchorDate||todayKey()};
  saveChange(editing?"Chore updated.":"Chore added.",data=>{const row={id:editing||idFor("c"),name,createdAt:existing?.createdAt||new Date().toISOString(),cadence,effort:Number($("cw-effort")?.value)||2,eligible};if(editing){const index=data.chores.findIndex(item=>item.id===editing);data.chores[index]=row;data.assignments.filter(item=>item.choreId===editing).forEach(item=>item.choreName=name);}else data.chores.push(row);}).then(ok=>{if(ok){state.form=null;state.pendingFormFocus=0;state.formFocusSerial++;if(typeof hideOSK==="function")hideOSK();render();}});
}
function removeChore(id){const chore=lookupChore(id);if(!chore)return;const future=state.data.assignments.filter(row=>row.choreId===id&&row.date>todayKey()&&activeAssignment(row)).length;setConfirm(`Remove “${chore.name}”?`,future?`${future} future assigned ${future===1?"instance will":"instances will"} be removed from the Chores calendar. Past, completed, and skipped history remains in Chore Wheel.`:"Future scheduling stops. Past, completed, and skipped history remains in Chore Wheel.","Remove chore",()=>saveChange("Chore removed. Future calendar assignments cleared.",data=>core.removeChoreAssignments(data,id,todayKey())));}
async function openChoreWheel(){
  const shell=root();if(!shell||isOpen())return;state.priorFocus=document.activeElement;state.openToken++;state.spinSerial++;clearTimeout(state.spinTimer);state.spinTimer=0;state.spin=null;const token=state.openToken;state.view="today";state.confirm=null;state.form=null;state.status="Loading Chore Wheel…";shell.hidden=false;shell.classList.add("show");shell.setAttribute("aria-hidden","false");setBusy(true);body().replaceChildren(loadingView());
  if(typeof pauseUiAnimations==="function")pauseUiAnimations();if(typeof armOverlayAutoClose==="function")armOverlayAutoClose();
  try{const payload=await api("/api/chore-wheel");if(token!==state.openToken||!isOpen())return;state.data=core.normalize(payload);state.status="Local, fair chore rotations · calendar-backed";}
  catch(error){if(token!==state.openToken||!isOpen())return;state.data=core.normalize({});state.status=error.message||"Could not load Chore Wheel.";body().replaceChildren(errorView(state.status));}
  finally{if(token===state.openToken&&isOpen()){setBusy(false);if(!body().querySelector(".cw-error")){render();requestAnimationFrame(()=>$("chorewheel-close")?.focus());}}}
}
function closeChoreWheel(){
  const shell=root();if(!shell||!isOpen()||(typeof appLauncherHandoffActive==="function"&&appLauncherHandoffActive()))return;state.openToken++;state.spinSerial++;clearTimeout(state.spinTimer);state.spinTimer=0;state.spin=null;state.confirm=null;state.form=null;state.pendingFormFocus=0;state.formFocusSerial++;setBusy(false);if(typeof hideOSK==="function")hideOSK();shell.classList.remove("show","busy","osk-open");shell.hidden=true;shell.setAttribute("aria-hidden","true");if(typeof completeAppLauncherHandoff==="function")completeAppLauncherHandoff();if(typeof disarmOverlayAutoClose==="function")disarmOverlayAutoClose();if(typeof resumeUiAfterOverlay==="function"&&!(typeof overlayIsOpen==="function"&&overlayIsOpen()))resumeUiAfterOverlay();const trigger=$("cblaunch");(trigger&&!trigger.hidden?trigger:state.priorFocus)?.focus?.();
}
function bindShell(){
  const shell=root(),close=$("chorewheel-close");if(!shell)return;if(close)bindTap(close,closeChoreWheel);bindTap(shell,closeChoreWheel,{ignore:event=>event.target!==shell});document.addEventListener("keydown",event=>{if(event.key!=="Escape"||!isOpen())return;event.preventDefault();if(state.confirm){state.confirm=null;render();}else closeChoreWheel();});
}
window.openChoreWheelImpl=openChoreWheel;window.closeChoreWheel=closeChoreWheel;window.choreWheelIsOpen=isOpen;bindShell();
})();
