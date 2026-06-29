// lists-people.js — local household responsibility for To Do and Grocery.
// This module is loaded only with Lists. It never changes Microsoft payloads;
// the server stores dashgoAssignment alongside the local cache record.
(function(){
  const api=window.DashGoLists;if(!api)return;
  const state=api.state;
  const $=api.$;

  function roster(){ return Array.isArray(state.status?.people)?state.status.people:[]; }
  function activePeople(){ return roster().filter(person=>person&&person.state==="active"&&person.id&&person.name); }
  function assignment(task){ return task&&task.dashgoAssignment&&typeof task.dashgoAssignment==="object"?task.dashgoAssignment:null; }
  function assignedID(task){ return String(assignment(task)?.personId||"").trim(); }
  function personFor(id){ return roster().find(person=>String(person?.id||"")===id)||null; }
  function personLabel(task){
    const saved=assignment(task);if(!saved)return "Anyone";
    const person=personFor(saved.personId);
    const name=String(person?.name||saved.personNameSnapshot||"Former household member").trim();
    return person?.state==="active"?name:`Former: ${name}`;
  }
  function matchesTask(task){
    const filter=state.personFilter||"all";
    if(filter==="all")return true;
    if(filter==="unassigned")return !assignedID(task);
    return assignedID(task)===filter;
  }
  function hasAssignmentUI(){ return activePeople().length>0||state.tasks.some(task=>!!assignedID(task)); }
  function usesMicrosoftTaskSync(){
    const list=state.lists.find(item=>item&&item.id===state.active)||{};
    return !!(state.status&&state.status.syncActive&&list.origin==="microsoft");
  }

  function tapButton(label,className,handler){
    const node=document.createElement("button");node.type="button";node.className=className;node.textContent=label;bindTap(node,handler);return node;
  }
  function openPeopleControl(origin){
    if(typeof window.openDashboardPeopleControl!=="function"){api.setStatus("People management is unavailable until Dashboard Control loads.");return;}
    const slot=state.slot;
    window.openDashboardPeopleControl({
      origin:origin||document.activeElement,
      close:()=>api.closeLists?.(),
      reopen:()=>api.openListsImpl?.(slot)
    }).catch(error=>api.setStatus("Could not open People · "+(error&&error.message?error.message:"try again")));
  }
  function managePeopleButton(){
    let button;button=tapButton("Manage people","lists-manage-people",()=>openPeopleControl(button));
    return button;
  }
  function renderFilters(body){
    const section=document.createElement("div");section.className="lists-people-tools";
    if(hasAssignmentUI()){
      const wrap=document.createElement("nav");wrap.className="lists-people-filters";wrap.setAttribute("aria-label",`${api.slotLabel(state.slot)} responsibility filters`);
      const choices=[["all","All"],...activePeople().map(person=>[String(person.id),String(person.name)]),["unassigned","Anyone"]];
      for(const [id,label] of choices){
        const button=tapButton(label,"lists-person-filter"+(state.personFilter===id?" on":""),()=>{state.personFilter=id;api.renderTasks();});
        button.setAttribute("aria-pressed",String(state.personFilter===id));wrap.appendChild(button);
      }
      section.appendChild(wrap);
    }
    section.appendChild(managePeopleButton());
    body.appendChild(section);
  }
  function closePicker(entry,result){
    if(!entry||entry.closed)return;entry.closed=true;document.removeEventListener("keydown",entry.onKey,true);entry.wrap.remove();if(state.prompt===entry)state.prompt=null;$("listsapp")?.classList.remove("compose-open");entry.resolve(result);
  }
  function choosePerson(task){
    if(state.prompt)return Promise.resolve(undefined);
    return new Promise(resolve=>{
      const root=$("listsapp")||document.body,wrap=document.createElement("div"),panel=document.createElement("section"),title=document.createElement("div"),detail=document.createElement("div"),choices=document.createElement("div");
      wrap.className="lists-modal-backdrop";wrap.setAttribute("role","presentation");panel.className="lists-prompt lists-person-picker lists-modal";panel.setAttribute("role","dialog");panel.setAttribute("aria-modal","true");panel.setAttribute("aria-label",`Choose responsibility for ${task.title||"item"}`);
      title.className="lists-prompt-label";title.textContent=`Who is responsible for “${task.title||"this item"}”?`;
      detail.className="lists-prompt-detail";detail.textContent="Responsibility stays on Dash-Go and does not change Microsoft To Do.";
      choices.className="lists-person-choice-grid";
      let entry;const select=id=>closePicker(entry,id);const cancel=()=>closePicker(entry,undefined);
      choices.appendChild(tapButton("Anyone","lists-person-choice"+(!assignedID(task)?" on":""),()=>select("")));
      for(const person of activePeople())choices.appendChild(tapButton(String(person.name),"lists-person-choice"+(assignedID(task)===String(person.id)?" on":""),()=>select(String(person.id))));
      const actions=document.createElement("div");actions.className="lists-prompt-actions";actions.appendChild(tapButton("Cancel","lists-prompt-cancel",cancel));
      const onKey=event=>{if(event.key==="Escape"){event.preventDefault();cancel();}};
      entry={wrap,onKey,closed:false,resolve};state.prompt=entry;$("listsapp")?.classList.add("compose-open");document.addEventListener("keydown",onKey,true);
      wrap.addEventListener("pointerdown",event=>{if(event.target===wrap)event.preventDefault();});
      panel.appendChild(title);
      if(usesMicrosoftTaskSync())panel.appendChild(detail);
      panel.append(choices,actions);wrap.appendChild(panel);root.appendChild(wrap);requestAnimationFrame(()=>choices.querySelector("button")?.focus?.());
    });
  }
  async function setAssignment(task){
    const personId=await choosePerson(task);if(personId===undefined)return;
    if(String(personId)===assignedID(task))return;
    const listID=state.active,epoch=state.openEpoch;
    try{
      state.assignmentBusyTaskID=task.id;api.renderTasks();
      const cache=await api.apiJSON(`/api/todo/lists/${encodeURIComponent(listID)}/tasks/${encodeURIComponent(task.id)}/assignment`,"POST",{personId});
      if(api.listsPanelIsCurrent(listID,epoch)){state.tasks=api.listsMergePendingMutations(cache.tasks||[]);api.setStatus(`${api.listTitle(listID)} · Responsibility saved locally.`);}
    }catch(error){
      if(api.listsPanelIsCurrent(listID,epoch))api.setStatus(`Could not save responsibility · ${error.message}`);
    }finally{
      const current=api.listsPanelIsCurrent(listID,epoch);state.assignmentBusyTaskID="";
      if(current)api.renderTasks();
    }
  }
  const baseTaskRow=api.taskRow;
  function taskRow(task){
    const row=baseTaskRow(task),actions=row.querySelector(".task-actions"),main=row.querySelector(".task-main");
    if(!actions||!main||!hasAssignmentUI())return row;
    const label=personLabel(task),chip=tapButton(label,"task-person"+(assignedID(task)?" assigned":""),()=>setAssignment(task));
    chip.disabled=state.assignmentBusyTaskID===task.id;chip.setAttribute("aria-label",`Change responsibility for ${task.title||"item"}: ${label}`);actions.prepend(chip);
    if(assignedID(task)){
      const meta=main.querySelector(".task-meta"),person=document.createElement("span");person.className="task-person-label";person.textContent=label;meta?.append(" · ",person);
    }
    return row;
  }
  api.taskRow=taskRow;
  window.DashGoListsPeople={matchesTask,renderFilters,personLabel};
})();
