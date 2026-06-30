// 05-popups-03b-app-calendar-actions.js — small, durable calendar completion
// surfaces for app-owned household work. Complex edits stay in full apps.
function appCalendarActionDate(day){ return localDateKey(startOfDay(day)); }
function appCalendarActionOwner(group){ return String((group&&group.owner)||"").trim().toLowerCase(); }
function appCalendarActionRequest(path,body){
  return fetch(path,{method:body?"POST":"GET",headers:body?{"Content-Type":"application/json",Accept:"application/json"}:undefined,body:body?JSON.stringify(body):undefined,cache:"no-store"}).then(response=>response.json().catch(()=>({})).then(payload=>response.ok?payload:Promise.reject(new Error(payload.error||"Household action failed"))));
}
function appCalendarActionOpen(info,day){
  closeScrim();
  requestAnimationFrame(()=>{
    if(info.owner==="chore-wheel"&&typeof openChoreWheel==="function")openChoreWheel();
    if(info.owner==="maintenance"&&typeof openMaintenance==="function")openMaintenance();
    if(info.owner==="routines"&&typeof openRoutines==="function")openRoutines({date:appCalendarActionDate(day)});
  });
}
function appCalendarActionStatus(item,date){
  const status=String(item&&item.status||"assigned");
  if(status==="completed"){
    if(item&&item.undoAvailable===false)return String(item.correctionMessage||"Completed · review in Maintenance");
    return "Done · tap again to reopen";
  }
  if(status==="skipped")return "Skipped";
  if(item&&item.actionable===false)return `Scheduled for ${date}`;
  return date<appCalendarActionDate(new Date())?"Assigned · overdue":"Assigned today";
}
function appCalendarActionTitle(item){ return item.title||item.choreName||"household item"; }
function appCalendarActionSetAria(check,item){
  check.setAttribute("aria-label",`Mark ${appCalendarActionTitle(item)} ${check.checked?"incomplete":"complete"}`);
}
function appCalendarActionRow(item,date,onToggle){
  const row=el("article","appgroup-action-row"),control=el("label","appgroup-action-check"),check=document.createElement("input"),copy=el("div","appgroup-action-copy"),state=el("span","appgroup-action-state",appCalendarActionStatus(item,date));
  const status=String(item&&item.status||"assigned"),actionable=item&&item.actionable!==false&&(status==="assigned"||status==="completed");
  if(status==="completed")row.classList.add("is-complete");
  check.type="checkbox";check.checked=status==="completed";check.disabled=!actionable;appCalendarActionSetAria(check,item);
  control.appendChild(check);
  copy.append(el("strong","",appCalendarActionTitle(item)),el("small","",item.detail||item.cadence||""));
  row.append(control,copy,state);
  check.addEventListener("change",()=>{
    const desired=check.checked,previous=!desired;
    if(!actionable){check.checked=previous;appCalendarActionSetAria(check,item);return;}
    check.disabled=true;state.textContent="Saving…";row.classList.remove("appgroup-action-error");row.classList.add("is-saving");
    Promise.resolve(onToggle(desired)).catch(error=>{
      check.checked=previous;check.disabled=false;appCalendarActionSetAria(check,item);state.textContent=error.message||"Could not save";row.classList.add("appgroup-action-error");
    }).finally(()=>{row.classList.remove("is-saving");});
  });
  return row;
}
function showChoresCalendarActionPopup(day,info){
  const date=appCalendarActionDate(day);
  popupOpenTransaction({mode:"eventpop",title:info.label,when:FMT.dayLong.format(day),loading:"Opening chores…"},()=>{
    const root=el("section","appgroup-popup appgroup-action-popup"),intro=el("p","appgroup-note","Loading chores…"),host=el("div","appgroup-action-list"),open=el("button","appgroup-open-action",info.action);open.type="button";open.addEventListener("click",event=>{event.stopPropagation();appCalendarActionOpen(info,day);});root.append(intro,host,open);
    function render(payload){
      const items=Array.isArray(payload&&payload.items)?payload.items:[],done=Number(payload&&payload.completed)||0;
      intro.textContent=items.length?`${items.length} chore${items.length===1?"":"s"} · ${done} complete.`:"No chores are assigned for this day.";
      host.replaceChildren();
      for(const item of items){
        item.title=`${item.choreName||"Chore"}${item.personName?" — "+item.personName:""}`;
        item.detail=appCalendarActionStatus(item,date);
        host.appendChild(appCalendarActionRow(item,date,desired=>appCalendarActionRequest("/api/chore-wheel/assignments/status",{assignmentId:item.assignmentId,date,completed:desired}).then(next=>render(next.day||next))));
      }
    }
    appCalendarActionRequest("/api/chore-wheel/day?date="+encodeURIComponent(date)).then(render).catch(error=>{intro.textContent=error.message||"Chores are unavailable.";});
    return root;
  });
}
function showMaintenanceCalendarActionPopup(day,info){
  const date=appCalendarActionDate(day);
  popupOpenTransaction({mode:"eventpop",title:info.label,when:FMT.dayLong.format(day),loading:"Opening maintenance tasks…"},()=>{
    const root=el("section","appgroup-popup appgroup-action-popup"),intro=el("p","appgroup-note","Loading maintenance tasks…"),host=el("div","appgroup-action-list"),open=el("button","appgroup-open-action",info.action);open.type="button";open.addEventListener("click",event=>{event.stopPropagation();appCalendarActionOpen(info,day);});root.append(intro,host,open);
    function render(payload){
      const current=payload||{date,items:[],completedItems:[]},items=Array.isArray(current.items)?current.items:[],completed=Array.isArray(current.completedItems)?current.completedItems:[];
      intro.textContent=items.length||completed.length?`${items.length} due · ${completed.length} completed.`:"No maintenance tasks are due for this day.";
      host.replaceChildren();
      for(const item of items){
        item.status="assigned";item.detail=`Due ${item.dueOn||date} · ${item.cadence||""}`;
        host.appendChild(appCalendarActionRow(item,date,desired=>{
          if(!desired)return Promise.reject(new Error("This task is not completed."));
          return appCalendarActionRequest("/api/maintenance/tasks/complete",{id:item.id,completedOn:appCalendarActionDate(new Date()),dayDate:date}).then(next=>render(next.day||next));
        }));
      }
      if(completed.length){
        host.appendChild(el("div","appgroup-action-section","Completed"));
        for(const item of completed){
          item.status="completed";item.detail=`Completed ${item.completedOn||date} · Next due ${item.nextDueOn||"—"}`;
          host.appendChild(appCalendarActionRow(item,date,desired=>{
            if(desired)return Promise.resolve();
            return appCalendarActionRequest("/api/maintenance/tasks/undo-complete",{id:item.id,completionId:item.completionId,dayDate:date}).then(next=>render(next.day||next));
          }));
        }
      }
    }
    appCalendarActionRequest("/api/maintenance/day?date="+encodeURIComponent(date)).then(render).catch(error=>{intro.textContent=error.message||"Maintenance is unavailable.";});
    return root;
  });
}
function routineCalendarProgress(session){
  const steps=Array.isArray(session&&session.steps)?session.steps:[],done=new Set(Array.isArray(session&&session.completedStepIds)?session.completedStepIds.map(String):[]);
  return {done:steps.filter(step=>done.has(String(step&&step.id))).length,total:steps.length};
}
function showRoutinesCalendarActionPopup(day,info){
  const date=appCalendarActionDate(day),expanded=new Set();let initialExpansion=true;
  popupOpenTransaction({mode:"eventpop",title:info.label,when:FMT.dayLong.format(day),loading:"Opening routine checklists…"},()=>{
    const root=el("section","appgroup-popup appgroup-action-popup"),intro=el("p","appgroup-note","Loading person-centered routine checklists…"),host=el("div","appgroup-list"),open=el("button","appgroup-open-action",info.action);open.type="button";open.addEventListener("click",event=>{event.stopPropagation();appCalendarActionOpen(info,day);});root.append(intro,host,open);
    function mutate(session,payload){return appCalendarActionRequest("/api/routines/occurrence",{...payload,routineId:session.routineId,assignmentId:session.assignmentId,date}).then(next=>render(next.day||next));}
    function render(payload){
      const people=Array.isArray(payload&&payload.people)?payload.people:[];let allDone=0,allSteps=0,totalSessions=0;
      for(const person of people)for(const session of (person.sessions||[])){const progress=routineCalendarProgress(session);allDone+=progress.done;allSteps+=progress.total;totalSessions++;}
      intro.textContent=people.length?`${people.length} ${people.length===1?"person":"people"} · ${totalSessions} routine${totalSessions===1?"":"s"} · ${allDone}/${allSteps} complete.`:"No routines are due.";
      host.replaceChildren();
      let autoExpanded=false;
      for(const person of people){
        const sessions=Array.isArray(person.sessions)?person.sessions:[];let done=0,total=0,complete=0;
        for(const session of sessions){const progress=routineCalendarProgress(session);done+=progress.done;total+=progress.total;if(session.state==="completed")complete++;}
        const summary=complete===sessions.length&&sessions.length?"Complete":`${done}/${total} complete`;
        const card=el("section","routine-calendar-person"),toggle=el("button","appgroup-item");toggle.type="button";toggle.appendChild(el("span","appgroup-item-title",`${person.name||"Household member"} · ${sessions.length} routine${sessions.length===1?"":"s"} · ${summary}`));
        const incomplete=sessions.some(session=>session.state==="active");
        const openPerson=expanded.has(String(person.id))||(initialExpansion&&!autoExpanded&&incomplete);
        if(openPerson&&incomplete)autoExpanded=true;
        const details=el("div","routine-calendar-sessions");details.hidden=!openPerson;
        for(const session of sessions){
          const row=el("div","routine-calendar-session"),progress=routineCalendarProgress(session),actionable=session.actionable!==false&&session.state!=="skipped";
          row.appendChild(el("strong","",`${session.time&&!session.allDay?session.time+" · ":""}${session.routineTitle||"Routine"}`));
          const steps=el("div","routine-calendar-steps"),completed=new Set(Array.isArray(session.completedStepIds)?session.completedStepIds.map(String):[]);
          const status=el("small","",session.state==="completed"?"Complete":session.state==="skipped"?"Skipped":`${progress.done}/${progress.total} complete`);
          for(const step of (session.steps||[])){
            const label=el("label","routine-calendar-step"),check=document.createElement("input");check.type="checkbox";check.checked=completed.has(String(step.id));check.disabled=!actionable;check.setAttribute("aria-label",`Mark ${step.text||"routine step"} ${check.checked?"incomplete":"complete"}`);
            check.addEventListener("change",()=>{const desired=check.checked,previous=!desired;check.disabled=true;status.textContent="Saving…";mutate(session,{op:"step",stepId:step.id,checked:desired}).catch(error=>{check.checked=previous;check.disabled=false;check.setAttribute("aria-label",`Mark ${step.text||"routine step"} ${check.checked?"incomplete":"complete"}`);status.textContent=error.message||"Could not save";});});
            label.append(check,document.createTextNode(step.text||"Step"));steps.appendChild(label);
          }
          row.append(steps,status);
          if(actionable&&session.state==="active"&&progress.done<progress.total){
            const complete=el("button","routine-calendar-complete","Complete routine");complete.type="button";complete.addEventListener("click",()=>{complete.disabled=true;mutate(session,{op:"complete"}).catch(error=>{complete.disabled=false;status.textContent=error.message||"Could not save";});});row.appendChild(complete);
          }
          details.appendChild(row);
        }
        toggle.addEventListener("click",event=>{event.stopPropagation();details.hidden=!details.hidden;if(details.hidden)expanded.delete(String(person.id));else expanded.add(String(person.id));toggle.setAttribute("aria-expanded",String(!details.hidden));});toggle.setAttribute("aria-expanded",String(!details.hidden));card.append(toggle,details);host.appendChild(card);
      }
      initialExpansion=false;
    }
    appCalendarActionRequest("/api/routines/day?date="+encodeURIComponent(date)).then(render).catch(error=>{intro.textContent=error.message||"Routine details are unavailable.";});
    return root;
  });
}
function showActionableAppCalendarGroupPopup(day,group){
  const owner=appCalendarActionOwner(group),info=appCalendarGroupInfo(owner);
  if(!info){showDayPopup(day,(group&&group.events)||[]);return;}
  if(owner==="chore-wheel"){showChoresCalendarActionPopup(day,info);return;}
  if(owner==="maintenance"){showMaintenanceCalendarActionPopup(day,info);return;}
  if(owner==="routines"){showRoutinesCalendarActionPopup(day,info);return;}
  showDayPopup(day,(group&&group.events)||[]);
}
function showActionableAppCalendarEvent(ev){
  const owner=typeof appCalendarOwner==="function"?appCalendarOwner(ev):"";
  if(!owner||!ev||!ev.start){showEventPopup(ev);return;}
  showActionableAppCalendarGroupPopup(startOfDay(ev.start),{owner,events:[ev]});
}
