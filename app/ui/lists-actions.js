(function(){
const api=window.DashGoLists;if(!api)return;
const LISTS_STATE=api.state;
const {$,activeListOrigin,apiJSON,applyInboundSyncStatus,deleteListItemLabel,keepListItemLabel,listIDForSlot,listItemLabel,listItemLowerLabel,listTitle,listsFlushTaskPatchBatch,listsMarkPendingMutation,listsMergePendingMutations,listsPanelIsCurrent,listsRestoreBatchEntries,listsSettlePendingMutations,loadTasks,manualSyncDuration,normalizedListSlot,queueTaskStatusPatch,refreshStatus,renderOpenError,renderTasks,requestListSync,scheduleStreamRefresh,setListsAppTitle,setStatus,slotLabel,stopManualSyncCountdown,updateManualSyncUI}=api;

function taskState(task){
  if(task&&task._saving)return "Saving locally…";
  if(task&&task._syncFailed)return "Microsoft sync needs attention";
  if(task&&task._pending)return "Microsoft sync pending";
  if(LISTS_STATE.status&&LISTS_STATE.status.syncActive&&activeListOrigin()==="microsoft")return "Synced locally first";
  return "Saved locally";
}
function taskRow(task){
  const row=document.createElement("article");
  row.className="task-row"+(task&&task._saving?" saving":"");
  const done=task.status==="completed",saving=!!(task&&task._saving);
  const check=document.createElement("button");
  check.type="button";
  check.className="task-check";
  check.textContent=done?"✓":"";
  check.disabled=saving;
  check.setAttribute("aria-label",(saving?"Saving: ":(done?"Mark incomplete: ":"Complete: "))+(task.title||"task"));
  bindTap(check,()=>{if(!saving)patchTask(task,{status:done?"notStarted":"completed"});});
  const main=document.createElement("button");
  main.type="button";
  main.className="task-main";
  main.disabled=saving;
  const title=document.createElement("div");
  title.className="task-title"+(done?" completed":"");
  title.textContent=task.title||"Untitled task";
  const meta=document.createElement("div");
  meta.className="task-meta";
  meta.textContent=taskState(task);
  main.append(title,meta);
  bindTap(main,()=>{if(!saving)editTask(task);});
  const actions=document.createElement("div");
  actions.className="task-actions";
  const edit=document.createElement("button");
  edit.type="button";
  edit.textContent="Edit";
  edit.disabled=saving;
  edit.setAttribute("aria-label",`Edit ${listItemLabel(LISTS_STATE.active)}: ${task.title||"Untitled item"}`);
  bindTap(edit,()=>{if(!saving)editTask(task);});
  const del=document.createElement("button");
  del.type="button";
  del.textContent="Remove";
  del.disabled=saving;
  del.setAttribute("aria-label",`${deleteListItemLabel(LISTS_STATE.active)}: ${task.title||"Untitled item"}`);
  bindTap(del,()=>{if(!saving)deleteTask(task);});
  actions.append(edit,del);
  row.append(check,main,actions);
  return row;
}
function listPromptHost(){ return $("listsapp-body")||document.body; }
function makeListPromptButton(label,cls,fn){
  const button=document.createElement("button");
  button.type="button";
  button.className=cls;
  button.textContent=label;
  button.addEventListener("pointerdown",event=>event.preventDefault());
  bindTap(button,fn);
  return button;
}
function clearListsPrompt(entry){
  if(!entry||entry.closed)return;
  entry.closed=true;
  document.removeEventListener("keydown",entry.onKey,true);
  if(typeof hideOSK==="function")hideOSK();
  entry.wrap.remove();
  if(LISTS_STATE.prompt===entry)LISTS_STATE.prompt=null;
  $("listsapp")?.classList.remove("compose-open");
}
function promptText(label,value){
  const initial=String(value||"");
  if(LISTS_STATE.prompt)return Promise.resolve(initial);
  return new Promise(resolve=>{
    const host=listPromptHost();
    const wrap=document.createElement("section");
    wrap.className="lists-prompt";
    wrap.setAttribute("role","group");
    wrap.setAttribute("aria-label",label);
    const title=document.createElement("label");
    title.className="lists-prompt-label";
    title.textContent=label;
    const input=typeof oskInput==="function"?oskInput(label,initial,{}):document.createElement("input");
    input.classList.add("lists-prompt-input");
    input.setAttribute("aria-label",label);
    if(typeof oskInput!=="function"){input.type="text";input.value=initial;}
    const actions=document.createElement("div");
    actions.className="lists-prompt-actions";
    let entry;
    const finish=valueToReturn=>{
      if(!entry||entry.closed)return;
      clearListsPrompt(entry);
      resolve(valueToReturn);
    };
    const save=makeListPromptButton("Save","lists-prompt-save",()=>finish(input.value||""));
    const cancel=makeListPromptButton("Cancel","lists-prompt-cancel",()=>finish(initial));
    const onKey=event=>{
      if(event.key==="Enter"){event.preventDefault();finish(input.value||"");}
      if(event.key==="Escape"){event.preventDefault();finish(initial);}
    };
    entry={wrap,input,onKey,closed:false,cancel:()=>finish(initial)};
    input.addEventListener("blur",()=>setTimeout(()=>{
      if(entry.closed||wrap.contains(document.activeElement))return;
      finish(input.value||"");
    },0));
    actions.append(cancel,save);
    wrap.append(title,input,actions);
    host.prepend(wrap);
    LISTS_STATE.prompt=entry;
    $("listsapp")?.classList.add("compose-open");
    document.addEventListener("keydown",onKey,true);
    requestAnimationFrame(()=>{
      input.focus?.();
      if(typeof showOSKFor==="function")showOSKFor(input);
    });
  });
}
function confirmListsAction(titleText,detailText,cancelLabel,confirmLabel){
  if(LISTS_STATE.prompt)return Promise.resolve(false);
  return new Promise(resolve=>{
    const root=$("listsapp")||document.body;
    const wrap=document.createElement("div");
    wrap.className="lists-modal-backdrop";
    wrap.setAttribute("role","presentation");
    const panel=document.createElement("section");
    panel.className="lists-prompt lists-confirm lists-modal";
    panel.setAttribute("role","alertdialog");
    panel.setAttribute("aria-modal","true");
    panel.setAttribute("aria-label",titleText);
    const title=document.createElement("div");title.className="lists-prompt-label";title.textContent=titleText;
    const detail=document.createElement("div");detail.className="lists-prompt-detail";detail.textContent=detailText;
    const actions=document.createElement("div");actions.className="lists-prompt-actions";
    let entry;
    const finish=result=>{if(!entry||entry.closed)return;clearListsPrompt(entry);resolve(result);};
    const cancel=makeListPromptButton(cancelLabel,"lists-prompt-cancel",()=>finish(false));
    const apply=makeListPromptButton(confirmLabel,"lists-prompt-delete",()=>finish(true));
    const onKey=event=>{if(event.key==="Escape"){event.preventDefault();finish(false);}if(event.key==="Enter"){event.preventDefault();finish(true);}};
    entry={wrap,onKey,closed:false,cancel:()=>finish(false)};
    wrap.addEventListener("pointerdown",event=>{if(event.target===wrap)event.preventDefault();});
    actions.append(cancel,apply);panel.append(title,detail,actions);wrap.appendChild(panel);root.appendChild(wrap);
    LISTS_STATE.prompt=entry;$("listsapp")?.classList.add("compose-open");document.addEventListener("keydown",onKey,true);
    requestAnimationFrame(()=>apply.focus?.());
  });
}
function confirmDeleteTask(task){
  const active=LISTS_STATE.active;
  const itemLabel=listItemLabel(active);
  return confirmListsAction(
    `${deleteListItemLabel(active)}?`,
    `“${task.title||"Untitled item"}” will be removed from ${listTitle(active)}.`,
    `Keep ${itemLabel}`,
    deleteListItemLabel(active),
  );
}
function confirmClearCompleted(count){
  const itemLabel=listItemLowerLabel(LISTS_STATE.active);
  return confirmListsAction(
    `Clear ${count} completed ${itemLabel}${count===1?"":"s"}?`,
    "This removes only the completed items shown when you opened this confirmation. Items checked afterward stay on this list.",
    "Keep items",
    `Clear ${count} item${count===1?"":"s"}`,
  );
}
async function clearCompleted(){
  if(LISTS_STATE.clearInFlight)return;
  LISTS_STATE.clearInFlight=true;
  renderTasks();
  try{
    if(Object.keys(LISTS_STATE.pendingMutations).length){
      setStatus("Finishing selected changes before clearing completed items…");
      await listsSettlePendingMutations();
    }
    const snapshot=LISTS_STATE.tasks.filter(task=>task.status==="completed").map(task=>task.id).filter(Boolean);
    const count=snapshot.length;
    if(!count||!await confirmClearCompleted(count))return;
    const result=await apiJSON("/api/todo/lists/clear-completed","POST",{listId:LISTS_STATE.active,taskIds:snapshot});
    LISTS_STATE.tasks=listsMergePendingMutations((result.cache||{}).tasks||[]);
    LISTS_STATE.groceryMemory=result.groceryMemory||LISTS_STATE.groceryMemory;
    const cleared=Number(result.cleared||0);
    setStatus(cleared?`Cleared ${cleared} completed ${listItemLowerLabel(LISTS_STATE.active)}${cleared===1?"":"s"}.`:"No matching completed items remained to clear.");
  }catch(error){setStatus(`Could not clear completed items · ${error.message}`);}
  finally{LISTS_STATE.clearInFlight=false;renderTasks();}
}
async function addTask(){
  const title=(await promptText(`Add ${listItemLabel(LISTS_STATE.active)}`,"")).trim();
  if(!title)return;
  try{
    const cache=await apiJSON(`/api/todo/lists/${encodeURIComponent(LISTS_STATE.active)}/tasks`,"POST",{title});
    LISTS_STATE.tasks=cache.tasks||[];
    renderTasks();
  }catch(error){setStatus(`Could not add ${listItemLowerLabel(LISTS_STATE.active)} · ${error.message}`);}
}
async function editTask(task){
  const title=(await promptText(`Edit ${listItemLabel(LISTS_STATE.active)}`,task.title||"")).trim();
  if(!title||title===task.title)return;
  await patchTask(task,{title});
}
async function patchTask(task,patch){
  if(!task||!task.id)return;
  // Rapid checkbox touches become one local cache transaction. Title/other
  // edits remain immediate single-task saves so text editing stays predictable.
  if(Object.prototype.hasOwnProperty.call(patch,"status")){
    queueTaskStatusPatch(task,patch);
    return;
  }
  const entry=listsMarkPendingMutation(task,patch);
  if(!entry)return;
  const listID=LISTS_STATE.active,epoch=LISTS_STATE.openEpoch;
  LISTS_STATE.tasks=listsMergePendingMutations(LISTS_STATE.tasks);
  renderTasks();
  try{
    const cache=await apiJSON(`/api/todo/lists/${encodeURIComponent(listID)}/tasks/${encodeURIComponent(task.id)}`,"POST",patch);
    delete LISTS_STATE.pendingMutations[entry.key];
    if(listsPanelIsCurrent(listID,epoch)){
      LISTS_STATE.tasks=listsMergePendingMutations(cache.tasks||[]);
      renderTasks();
    }
  }catch(error){
    delete LISTS_STATE.pendingMutations[entry.key];
    if(listsPanelIsCurrent(listID,epoch)){
      LISTS_STATE.tasks=LISTS_STATE.tasks.map(item=>item&&item.id===task.id?entry.task:item);
      renderTasks();
      setStatus(`Could not save ${listItemLowerLabel(listID)} · ${error.message}`);
    }
  }
}
async function deleteTask(task){
  if(!await confirmDeleteTask(task))return;
  try{
    const cache=await apiJSON(`/api/todo/lists/${encodeURIComponent(LISTS_STATE.active)}/tasks/${encodeURIComponent(task.id)}/delete`,"POST",{});
    LISTS_STATE.tasks=cache.tasks||[];
    renderTasks();
  }catch(error){setStatus(`Could not delete ${listItemLowerLabel(LISTS_STATE.active)} · ${error.message}`);}
}
function manualSyncResponseText(response){
  const sync=response&&response.manualSync||{},until=Number(sync.cooldownUntil||sync.backoffUntil||0);
  const seconds=until>0?Math.max(0,Math.ceil((until-Date.now())/1000)):Math.max(0,Number(sync.cooldownSeconds||sync.backoffSeconds||0));
  if(sync.running)return seconds?`Checking Microsoft changes · Sync now available in ${manualSyncDuration(seconds)}.`:"Checking Microsoft changes.";
  if(sync.queued)return seconds?`Microsoft sync is queued · Sync now available in ${manualSyncDuration(seconds)}.`:"Microsoft sync is queued.";
  if(seconds)return `${sync.reason||response.reason||"Sync requested recently."} Sync now is available in ${manualSyncDuration(seconds)}.`;
  return sync.reason||response.reason||"Microsoft sync is ready.";
}
async function syncCurrentListNow(){
  const listID=LISTS_STATE.active;
  if(!listID||!LISTS_STATE.status||!LISTS_STATE.status.syncActive||activeListOrigin()!=="microsoft")return;
  try{
    const response=await apiJSON(`/api/todo/lists/${encodeURIComponent(listID)}/sync-now`,"POST",{});
    applyInboundSyncStatus(response.inboundSync,response.manualSync);
    setStatus(`${listTitle(listID)} · ${manualSyncResponseText(response)}`);
    updateManualSyncUI();
  }catch(error){
    setStatus(`${listTitle(listID)} · Could not check Microsoft changes · ${error.message}`);
  }
}
function openStream(){
  try{
    if(LISTS_STATE.stream)LISTS_STATE.stream.close();
    const es=new EventSource("/api/todo/stream");
    // Stream events only refresh the local cache. The cache GET endpoint has no
    // Graph side effect, so an SSE completion cannot create a pull loop.
    const refresh=()=>{if(LISTS_STATE.active&&!LISTS_STATE.prompt)scheduleStreamRefresh();};
    es.addEventListener("todo",refresh);
    es.addEventListener("sync.state",refresh);
    LISTS_STATE.stream=es;
  }catch(_){}
}
function closeLists(){
  if(typeof appLauncherHandoffActive==="function"&&appLauncherHandoffActive())return;
  LISTS_STATE.openEpoch++;
  LISTS_STATE.loadSeq++;
  listsFlushTaskPatchBatch().catch(()=>{});
  if(LISTS_STATE.prompt)LISTS_STATE.prompt.cancel();
  LISTS_STATE.groceryManage=false;
  if(typeof completeAppLauncherHandoff==="function")completeAppLauncherHandoff();
  const root=$("listsapp");
  if(root){root.classList.remove("show","osk-open","compose-open");root.hidden=true;root.setAttribute("aria-hidden","true");}
  if(LISTS_STATE.stream){LISTS_STATE.stream.close();LISTS_STATE.stream=null;}
  if(LISTS_STATE.streamRefreshTimer){clearTimeout(LISTS_STATE.streamRefreshTimer);LISTS_STATE.streamRefreshTimer=null;}
  stopManualSyncCountdown();
  LISTS_STATE.manualSyncRefreshPending=false;
  LISTS_STATE.pendingMutations={};
  LISTS_STATE.clearInFlight=false;
  if(typeof overlayIsOpen==="function" && overlayIsOpen()){
    if(typeof armOverlayAutoClose==="function")armOverlayAutoClose();
  }else{
    if(typeof disarmOverlayAutoClose==="function")disarmOverlayAutoClose();
    if(typeof resumeUiAfterOverlay==="function")resumeUiAfterOverlay();
  }
  const trigger=$("cblaunch");if(window.DashGoAppDialog)window.DashGoAppDialog.restoreFocus(LISTS_STATE.priorFocus,trigger);else (trigger&&!trigger.hidden?trigger:LISTS_STATE.priorFocus)?.focus?.();
}
async function openListsImpl(slot){
  const root=$("listsapp");
  if(!root)return;
  LISTS_STATE.openEpoch++;
  LISTS_STATE.slot=normalizedListSlot(slot);LISTS_STATE.priorFocus=document.activeElement;
  LISTS_STATE.groceryManage=false;
  LISTS_STATE.personFilter="all";
  LISTS_STATE.clearInFlight=false;
  LISTS_STATE.pendingMutations={};
  LISTS_STATE.active="";
  LISTS_STATE.tasks=[];
  setListsAppTitle(slotLabel(LISTS_STATE.slot));
  root.hidden=false;root.classList.add("show");
  root.setAttribute("aria-hidden","false");
  if(window.DashGoAppDialog)window.DashGoAppDialog.focusInitial(root,"#listsapp-close");else requestAnimationFrame(()=>$("listsapp-close")?.focus?.());
  if(typeof pauseUiAnimations==="function")pauseUiAnimations();
  if(typeof armOverlayAutoClose==="function")armOverlayAutoClose();
  setStatus("Loading lists…");
  try{
    await refreshStatus();
    LISTS_STATE.active=listIDForSlot(LISTS_STATE.slot);
    if(!LISTS_STATE.active){
      setListsAppTitle(slotLabel(LISTS_STATE.slot));
      renderOpenError(`No ${slotLabel(LISTS_STATE.slot)} list is available.`);
      setStatus(`${slotLabel(LISTS_STATE.slot)} is unavailable right now.`);
      return;
    }
    openStream();
    await loadTasks(LISTS_STATE.active);
    requestListSync(LISTS_STATE.active).catch(error=>setStatus(`${listTitle(LISTS_STATE.active)} · Local cache shown; Microsoft check will retry.`));
  }catch(error){
    LISTS_STATE.status=null;
    LISTS_STATE.lists=[];
    LISTS_STATE.active="";
    LISTS_STATE.tasks=[];
    setListsAppTitle(slotLabel(LISTS_STATE.slot));
    renderOpenError(error.message);
    setStatus("Lists unavailable · "+error.message);
  }
}
function bindShell(){
  const close=$("listsapp-close"),root=$("listsapp");
  if(close)bindTap(close,closeLists);
  if(root)bindTap(root,closeLists,{ignore:event=>event.target!==root});
  document.addEventListener("keydown",event=>{
    if(event.key==="Escape"&&root&&root.classList.contains("show")&&!LISTS_STATE.prompt){
      event.preventDefault();
      closeLists();
    }
  });
}
async function openListsForAdd(slot){
  await openListsImpl(slot);
  if(LISTS_STATE.active) await addTask();
}
Object.assign(api,{addTask,clearCompleted,closeLists,confirmListsAction,deleteTask,editTask,openListsImpl,patchTask,promptText,syncCurrentListNow,taskRow});
window.openListsImpl=openListsImpl;
window.openListsForAdd=openListsForAdd;
window.closeListsApp=closeLists;
window.listsAppIsOpen=()=>!!($("listsapp")&&$("listsapp").classList.contains("show"));
bindShell();

})();
