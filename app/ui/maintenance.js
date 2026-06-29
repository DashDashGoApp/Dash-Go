// maintenance.js — lazy local Maintenance Tracker overlay.
(function(){
  const core=window.maintenanceCore;
  const state={data:null,summary:null,view:"due",form:null,confirm:null,busy:false,personFilter:"all",priorFocus:null,focusSerial:0};
  const root=()=>document.getElementById("maintenance");
  const body=()=>document.getElementById("maintenance-body");
  const tabs=()=>document.getElementById("maintenance-tabs");
  const status=()=>document.getElementById("maintenance-status");
  const isOpen=()=>!!root()?.classList.contains("show");
  const tasks=()=>Array.isArray(state.data?.tasks)?state.data.tasks:[];
  const settings=()=>state.data?.settings||{};
  const people=()=>Array.isArray(state.data?.people)?state.data.people:[];
  const activePeople=()=>people().filter(person=>person&&person.state==="active"&&person.id&&person.name);
  const calendarOutputEnabled=()=>settings().calendarOutputEnabled!==false;
  const taskPersonID=task=>String(task?.responsiblePersonId||"").trim();
  const personFor=id=>people().find(person=>String(person?.id||"")===String(id||""))||null;

  function setStatus(text){const node=status();if(node)node.textContent=text||"";}
  function button(label,cls,fn){const node=el("button",cls||"mt-action",label);node.type="button";bindTap(node,fn);return node;}
  function inputField(label,value,mode,placeholder,multiline){
    const wrap=el("label","mt-field"),caption=el("span","",label),input=document.createElement(multiline?"textarea":"input");
    input.value=value||"";input.placeholder=placeholder||"";input.autocomplete="off";input.dataset.oskMode=mode||"text";
    input.addEventListener("focus",()=>showOSKFor(input));
    input.addEventListener("pointerup",()=>{input.focus();showOSKFor(input);},{passive:true});
    wrap.append(caption,input);return{wrap,input};
  }
  function selectField(label,value,entries,onChange){
    const wrap=el("label","mt-field"),caption=el("span","",label),select=document.createElement("select");
    for(const [key,name] of entries){const option=document.createElement("option");option.value=key;option.textContent=name;option.selected=key===value;select.appendChild(option);}
    select.addEventListener("change",()=>onChange(select.value));wrap.append(caption,select);return{wrap,select};
  }
  async function request(path,payload){
    const response=await fetch(path,{method:payload?"POST":"GET",headers:payload?{"Content-Type":"application/json",Accept:"application/json"}:undefined,body:payload?JSON.stringify(payload):undefined,cache:"no-store"});
    const data=await response.json().catch(()=>({}));if(!response.ok)throw new Error(data.error||"Maintenance is unavailable");return data;
  }
  function accept(response){
    const payload=response.state||response;
    state.data={...payload,people:Array.isArray(response.people)?response.people:Array.isArray(payload.people)?payload.people:[]};
    state.summary=response.summary||{};
    const counts=state.summary.counts||{};setStatus(`${counts.overdue||0} overdue · ${(counts.today||0)+(counts.soon||0)} due soon`);
  }
  async function load(){setStatus("Loading maintenance tasks…");accept(await request("/api/maintenance"));render();}
  async function mutate(path,payload,success){
    if(state.busy)return;state.busy=true;root()?.classList.add("busy");setStatus("Saving…");
    try{accept(await request(path,payload));state.form=null;state.confirm=null;if(success)setStatus(success);render();}
    catch(error){setStatus(error.message||"Could not save maintenance task.");render();}
    finally{state.busy=false;root()?.classList.remove("busy");}
  }
  async function enableCalendarOutput(){
    if(state.busy||calendarOutputEnabled())return;state.busy=true;root()?.classList.add("busy");setStatus("Enabling Maintenance calendar…");
    try{accept(await request("/api/calendars/manage/app-output",{owner:"maintenance",enabled:true}));setStatus("Maintenance calendar output enabled.");render();}
    catch(error){setStatus(error.message||"Could not enable Maintenance calendar output.");render();}
    finally{state.busy=false;root()?.classList.remove("busy");}
  }
  function queueFocus(field){const serial=++state.focusSerial;requestAnimationFrame(()=>{if(serial!==state.focusSerial||!isOpen()||!field.isConnected)return;field.focus();showOSKFor(field);});}
  function taskPersonLabel(task){
    const id=taskPersonID(task);if(!id)return "";
    const person=personFor(id),snapshot=String(task?.responsiblePersonNameSnapshot||"").trim();
    const name=String(person?.name||snapshot||"Former household member").trim();
    return person?.state==="active"?name:`Former: ${name}`;
  }
  function assignmentPeople(){
    const out=[];const seen=new Set();
    for(const person of people()){
      const id=String(person?.id||"").trim();if(!id||seen.has(id))continue;
      if(person.state==="active"||tasks().some(task=>taskPersonID(task)===id)){seen.add(id);out.push(person);}
    }
    return out;
  }
  function matchesPerson(task){
    const filter=state.personFilter||"all";
    if(filter==="all")return true;
    if(filter==="unassigned")return !taskPersonID(task);
    return taskPersonID(task)===filter;
  }
  function hasPersonUI(){return activePeople().length>0||tasks().some(task=>!!taskPersonID(task));}
  function openPeopleControl(origin){
    if(typeof window.openDashboardPeopleControl!=="function"){setStatus("People management is unavailable until Dashboard Control loads.");return;}
    window.openDashboardPeopleControl({
      origin:origin||document.activeElement,
      close:closeMaintenance,
      reopen:openMaintenance
    }).catch(error=>setStatus("Could not open People · "+(error&&error.message?error.message:"try again")));
  }
  function managePeopleButton(className){
    let control;control=button("Manage people",className||"mt-action",()=>openPeopleControl(control));return control;
  }
  function personFilters(){
    if(!hasPersonUI())return null;
    const wrap=el("nav","mt-people-filters");wrap.setAttribute("aria-label","Maintenance responsibility filters");
    const choices=[["all","All"],...assignmentPeople().map(person=>[String(person.id),person.state==="active"?String(person.name):`Former: ${String(person.name||"household member")}`]),["unassigned","Anyone"]];
    for(const [id,label] of choices){
      const control=button(label,"mt-person-filter"+(state.personFilter===id?" on":""),()=>{state.personFilter=id;render();});
      control.setAttribute("aria-pressed",String(state.personFilter===id));wrap.appendChild(control);
    }
    return wrap;
  }
  function personEntries(draft){
    const entries=[["","Anyone"],...activePeople().map(person=>[String(person.id),String(person.name)])];
    const current=String(draft.responsiblePersonId||"").trim();
    if(current&&!entries.some(([id])=>id===current))entries.push([current,taskPersonLabel(draft)||`Former: ${String(draft.responsiblePersonNameSnapshot||"household member")}`]);
    return entries;
  }
  function taskForm(){
    const draft=state.form.draft,edit=state.form.kind==="edit",card=el("section","mt-card mt-form");
    card.appendChild(el("div","mt-card-title",edit?"Edit Maintenance Task":"Add Maintenance Task"));
    const title=inputField("Task name",draft.title,"text","Example: Replace HVAC filter");
    const note=inputField("Optional note",draft.note,"text","Details such as filter size",true);
    title.input.maxLength=120;note.input.maxLength=280;card.append(title.wrap,note.wrap);
    title.input.addEventListener("input",()=>draft.title=title.input.value);note.input.addEventListener("input",()=>draft.note=note.input.value);
    const cadence=selectField("How often",draft.cadence.unit,[["days","Days"],["weeks","Weeks"],["months","Months"],["years","Years"]],value=>draft.cadence.unit=value);
    const every=inputField("Every",draft.cadence.every,"numbers","1");every.input.inputMode="numeric";every.input.addEventListener("input",()=>draft.cadence.every=every.input.value);
    const due=inputField("Next due date",draft.nextDueOn,"date","YYYY-MM-DD");due.input.inputMode="numeric";due.input.addEventListener("input",()=>draft.nextDueOn=due.input.value);
    const responsible=selectField("Responsible",String(draft.responsiblePersonId||""),personEntries(draft),value=>{
      draft.responsiblePersonId=value;
      const person=personFor(value);draft.responsiblePersonNameSnapshot=person?.name||draft.responsiblePersonNameSnapshot||"";
    });
    card.append(cadence.wrap,every.wrap,due.wrap,responsible.wrap,managePeopleButton("mt-action mt-manage-people"));
    const schedulePreview=el("p","mt-note",`Next: ${draft.nextDueOn||"Choose a date"} · Then ${core.cadenceLabel({cadence:draft.cadence})}`);
    const refreshSchedule=()=>schedulePreview.textContent=`Next: ${draft.nextDueOn||"Choose a date"} · Then ${core.cadenceLabel({cadence:draft.cadence})}`;
    due.input.addEventListener("input",refreshSchedule);every.input.addEventListener("input",refreshSchedule);card.appendChild(schedulePreview);
    const calendar=el("label","mt-check"),check=document.createElement("input");check.type="checkbox";check.checked=!!draft.calendarEnabled;check.addEventListener("change",()=>draft.calendarEnabled=check.checked);calendar.append(check,document.createTextNode(" Show on Maintenance calendar"));card.appendChild(calendar);
    const actions=el("div","mt-actions");
    actions.append(button(edit?"Save Maintenance Task":"Add Maintenance Task","mt-action primary",()=>mutate(edit?"/api/maintenance/tasks/update":"/api/maintenance/tasks/add",{...draft,id:state.form.id},"Maintenance task saved.")),button("Cancel","mt-action",()=>{state.form=null;state.focusSerial++;hideOSK();render();}));
    card.appendChild(actions);queueFocus(title.input);return card;
  }
  function dateForm(){
    const task=state.form.task,complete=state.form.kind==="complete",restore=state.form.kind==="restore";
    const label=complete?"Complete "+task.title:(restore?"Restore "+task.title:"Reschedule "+task.title),card=el("section","mt-card mt-form");card.appendChild(el("div","mt-card-title",label));
    const key=complete?"completedOn":"nextDueOn",value=complete?core.today():task.nextDueOn,field=inputField(complete?"Completed on":"Next due date",value,"date","YYYY-MM-DD");field.input.inputMode="numeric";card.appendChild(field.wrap);
    const actions=el("div","mt-actions"),endpoint=complete?"/api/maintenance/tasks/complete":restore?"/api/maintenance/tasks/restore":"/api/maintenance/tasks/reschedule";
    actions.append(button(complete?"Mark complete":restore?"Restore task":"Save date","mt-action primary",()=>mutate(endpoint,{id:task.id,[key]:field.input.value},complete?"Maintenance task completed.":"Maintenance task updated.")),button("Cancel","mt-action",()=>{state.form=null;state.focusSerial++;hideOSK();render();}));card.appendChild(actions);queueFocus(field.input);return card;
  }
  function form(){return["add","edit"].includes(state.form.kind)?taskForm():dateForm();}
  function editDraft(task){return{title:task.title,note:task.note||"",cadence:{...(task.cadence||{})},nextDueOn:task.nextDueOn,calendarEnabled:!!task.calendarEnabled,responsiblePersonId:taskPersonID(task),responsiblePersonNameSnapshot:String(task.responsiblePersonNameSnapshot||"")};}
  function taskRow(task,archived){
    const row=el("article","mt-task"),main=el("div","mt-task-main");
    main.append(el("strong","",task.title),el("small","",archived?`Archived ${String(task.archivedAt||"").slice(0,10)}`:`${core.dueLabel(task,state.summary)} · ${core.cadenceLabel(task)}`));
    if(!archived&&task.lastCompletedOn)main.appendChild(el("small","",`Last completed ${core.dateLabel(task.lastCompletedOn)}`));
    const owner=taskPersonLabel(task);if(owner)main.appendChild(el("small","mt-person-line",`Responsible: ${owner}`));
    if(task.note)main.appendChild(el("p","mt-note",task.note));
    const actions=el("div","mt-row-actions");
    if(archived){actions.append(button("Restore","mt-action",()=>{state.form={kind:"restore",task};render();}),button("Delete task","mt-action danger",()=>{state.confirm={task,kind:"delete"};render();}));}
    else{actions.append(button("Complete","mt-action primary",()=>{state.form={kind:"complete",task};render();}),button("Reschedule","mt-action",()=>{state.form={kind:"reschedule",task};render();}),button("Edit","mt-action",()=>{state.form={kind:"edit",id:task.id,draft:editDraft(task)};render();}),button("Archive","mt-action",()=>mutate("/api/maintenance/tasks/archive",{id:task.id},"Maintenance task archived.")));}
    row.append(main,actions);return row;
  }
  function emptyMessage(active){return state.personFilter!=="all"?"No maintenance tasks match this person.":active?"No active maintenance tasks. Add the first recurring household task.":"No maintenance tasks yet.";}
  function dueView(){
    const wrap=el("div","mt-view"),counts=state.summary?.counts||{};
    const add=button("Add Maintenance Task","mt-action primary mt-add",()=>{state.form={kind:"add",draft:{title:"",note:"",cadence:{unit:"months",every:3},nextDueOn:core.today(),calendarEnabled:!!settings().defaultCalendarEnabled,responsiblePersonId:"",responsiblePersonNameSnapshot:""}};render();});
    const topActions=el("div","mt-actions mt-overview-actions");topActions.append(add,managePeopleButton("mt-action"));
    wrap.append(el("p","mt-overview",`${counts.overdue||0} overdue · ${(counts.today||0)+(counts.soon||0)} due in the next ${state.summary?.dueSoonDays||30} days`),topActions);
    const filters=personFilters();if(filters)wrap.appendChild(filters);
    const groups=[["overdue","Overdue"],["today","Due today"],["soon","Due soon"],["later","Later"]];let count=0;
    for(const [kind,label] of groups){const rows=core.active(tasks()).filter(matchesPerson).filter(task=>core.status(task,state.summary)===kind);if(!rows.length)continue;count+=rows.length;const card=el("section","mt-card");card.appendChild(el("div","mt-card-title",label));rows.forEach(task=>card.appendChild(taskRow(task,false)));wrap.appendChild(card);}
    if(!count)wrap.appendChild(el("div","mt-empty",emptyMessage(true)));return wrap;
  }
  function allView(){
    const wrap=el("div","mt-view"),active=core.active(tasks()).filter(matchesPerson),archived=tasks().filter(task=>task&&task.state==="archived"&&matchesPerson(task));
    const filters=personFilters();if(filters)wrap.appendChild(filters);
    if(!active.length&&!archived.length)wrap.appendChild(el("div","mt-empty",emptyMessage(false)));
    if(active.length){const card=el("section","mt-card");card.appendChild(el("div","mt-card-title","Active"));active.forEach(task=>card.appendChild(taskRow(task,false)));wrap.appendChild(card);}
    if(archived.length){const card=el("section","mt-card");card.appendChild(el("div","mt-card-title","Archived"));archived.forEach(task=>card.appendChild(taskRow(task,true)));wrap.appendChild(card);}return wrap;
  }
  function historyView(){
    const wrap=el("section","mt-card"),rows=Array.isArray(state.data?.history)?state.data.history:[],names=new Map(tasks().map(task=>[task.id,task.title]));wrap.appendChild(el("div","mt-card-title","History"));
    if(!rows.length)wrap.appendChild(el("p","mt-note","No maintenance history yet."));
    else rows.slice(0,80).forEach(row=>{const title=names.get(row.taskId)||"Removed maintenance task",owner=String(row.responsiblePersonNameSnapshot||"").trim();wrap.appendChild(el("div","mt-history-row",`${row.occurredOn} · ${title} · ${row.action}${owner?` · ${owner}`:""}`));});return wrap;
  }
  function settingsView(){
    const wrap=el("section","mt-card"),current=Number(settings().dueSoonDays)||30;wrap.append(el("div","mt-card-title","Settings"),el("p","mt-note","Choose the ordinary due-soon window for this app."));
    const options=el("div","mt-settings-options");for(const days of [14,30,45,60]){const control=button(`${days} days`,`mt-action${days===current?" primary":""}`,()=>mutate("/api/maintenance/settings",{dueSoonDays:days,defaultCalendarEnabled:!!settings().defaultCalendarEnabled},"Maintenance settings saved."));control.setAttribute("aria-pressed",String(days===current));options.appendChild(control);}wrap.appendChild(options);
    wrap.appendChild(button(settings().defaultCalendarEnabled?"New tasks show on calendar: On":"New tasks show on calendar: Off","mt-action",()=>mutate("/api/maintenance/settings",{dueSoonDays:current,defaultCalendarEnabled:!settings().defaultCalendarEnabled},"Maintenance settings saved.")));
    wrap.appendChild(managePeopleButton("mt-action"));
    if(!calendarOutputEnabled()){wrap.append(el("p","mt-note mt-calendar-output-off","Calendar output is off. Tasks and history remain local until you enable the Maintenance calendar again."));wrap.appendChild(button("Enable Maintenance calendar","mt-action primary",enableCalendarOutput));}return wrap;
  }
  function confirmation(){const layer=el("div","mt-confirm"),card=el("section","mt-card"),task=state.confirm.task;card.append(el("strong","",`Delete ${task.title}?`),el("p","mt-note","This removes the task and its generated Maintenance calendar event. A bounded local history record will remain."));const actions=el("div","mt-actions");actions.append(button("Delete Maintenance Task","mt-action danger",()=>mutate("/api/maintenance/tasks/delete",{id:task.id},"Maintenance task deleted.")),button("Keep Maintenance Task","mt-action",()=>{state.confirm=null;render();}));card.appendChild(actions);layer.appendChild(card);return layer;}
  function render(){
    if(!isOpen()||!state.data)return;const nav=tabs(),content=body();nav.replaceChildren();
    for(const [id,label] of [["due","Due"],["all","All tasks"],["history","History"],["settings","Settings"]]){const tab=button(label,"mt-tab",()=>{state.view=id;state.form=null;state.confirm=null;hideOSK();render();});tab.setAttribute("aria-pressed",String(state.view===id));nav.appendChild(tab);}
    content.replaceChildren(state.form?form():(state.view==="all"?allView():state.view==="history"?historyView():state.view==="settings"?settingsView():dueView()));if(state.confirm)content.appendChild(confirmation());
  }
  async function openMaintenance(){
    if(isOpen())return;const shell=root();if(!shell)return;state.priorFocus=document.activeElement;state.view="due";state.personFilter="all";state.form=null;state.confirm=null;shell.hidden=false;shell.classList.add("show");shell.setAttribute("aria-hidden","false");
    if(typeof completeAppLauncherHandoff==="function")completeAppLauncherHandoff();if(typeof armOverlayAutoClose==="function")armOverlayAutoClose();if(window.DashGoAppDialog)window.DashGoAppDialog.focusInitial(shell,"#maintenance-close");else requestAnimationFrame(()=>document.getElementById("maintenance-close")?.focus?.());
    try{await load();}catch(error){setStatus(error.message||"Maintenance is unavailable");state.data={tasks:[],history:[],people:[],settings:{defaultCalendarEnabled:true,calendarOutputEnabled:true,dueSoonDays:30}};state.summary={counts:{},today:core.today(),dueSoonDays:30};render();}
  }
  function closeMaintenance(){const shell=root();if(!shell)return;state.focusSerial++;hideOSK();state.form=null;state.confirm=null;shell.classList.remove("show");shell.hidden=true;shell.setAttribute("aria-hidden","true");if(typeof disarmOverlayAutoClose==="function")disarmOverlayAutoClose();if(typeof resumeUiAfterOverlay==="function"&&!(typeof overlayIsOpen==="function"&&overlayIsOpen()))resumeUiAfterOverlay();const trigger=document.getElementById("cblaunch");if(window.DashGoAppDialog)window.DashGoAppDialog.restoreFocus(state.priorFocus,trigger);else (trigger&&!trigger.hidden?trigger:state.priorFocus)?.focus?.();}
  function bindShell(){const shell=root(),close=document.getElementById("maintenance-close");if(!shell)return;if(close)bindTap(close,closeMaintenance);bindTap(shell,closeMaintenance,{ignore:event=>event.target!==shell});document.addEventListener("keydown",event=>{if(event.key!=="Escape"||!isOpen())return;event.preventDefault();if(state.confirm){state.confirm=null;render();}else if(state.form){state.form=null;state.focusSerial++;hideOSK();render();}else closeMaintenance();});}
  window.openMaintenanceImpl=openMaintenance;window.closeMaintenance=closeMaintenance;window.maintenanceIsOpen=isOpen;bindShell();
})();
