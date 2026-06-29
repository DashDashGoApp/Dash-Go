(function(){
let LISTS_STATE={slot:"todo",status:null,lists:[],active:"",tasks:[],showCompleted:{},stream:null,prompt:null,groceryMemory:[],groceryManage:false,personFilter:"all",priorFocus:null,loadSeq:0,pendingMutations:{},streamRefreshTimer:null,patchBatchTimer:null,patchBatchEntries:[],patchBatchRuns:new Set(),clearInFlight:false,assignmentBusyTaskID:"",manualSyncTimer:null,manualSyncRefreshPending:false,openEpoch:0};
const $=id=>document.getElementById(id);const LISTS_SLOT_LABELS=Object.freeze({todo:"To Do",grocery:"Grocery"});
const api={state:LISTS_STATE};window.DashGoLists=api;
function normalizedListSlot(value){ return value==="grocery"?"grocery":"todo"; }function slotLabel(slot){ return LISTS_SLOT_LABELS[normalizedListSlot(slot)]; }
function setListsAppTitle(title){
  const node=$("listsapp-title");
  if(node)node.textContent=String(title||"Lists");
}
function listIDForSlot(slot){
  const id=((LISTS_STATE.status||{}).map||{})[normalizedListSlot(slot)];
  return LISTS_STATE.lists.some(item=>item.id===id)?id:"";
}
async function apiJSON(path,method,body){
  const opts={method:method||"GET",headers:{Accept:"application/json"}};
  if(body){opts.headers["Content-Type"]="application/json";opts.body=JSON.stringify(body);}
  const r=await fetch(path,opts),j=await r.json().catch(()=>({}));
  if(!r.ok)throw new Error(j.error||("HTTP "+r.status));
  return j;
}
function listTitle(id){
  const list=LISTS_STATE.lists.find(item=>item.id===id);
  return list?(list.displayName||list.id):id;
}
function listItemLabel(id){
  return `${listTitle(id)} Item`;
}
function listItemLowerLabel(id){
  return `${listTitle(id)} item`;
}
function deleteListItemLabel(id){
  return `Delete ${listItemLabel(id)}`;
}
function keepListItemLabel(id){
  return `Keep ${listItemLabel(id)}`;
}
function activeListOrigin(){
  return (LISTS_STATE.lists.find(item=>item.id===LISTS_STATE.active)||{}).origin||"local";
}
function setStatus(text){
  const node=$("listsapp-status");
  if(node)node.textContent=text||"";
}
async function refreshStatus(){
  LISTS_STATE.status=await apiJSON("/api/todo/status");
  LISTS_STATE.lists=LISTS_STATE.status.lists||[];
  LISTS_STATE.groceryMemory=LISTS_STATE.status.groceryMemory||[];
  return LISTS_STATE.status;
}
function listSyncLabel(cache){
  if(cache&&cache.lastError)return "Offline cache · will retry Microsoft sync";
  if(LISTS_STATE.status&&LISTS_STATE.status.syncActive&&activeListOrigin()==="microsoft")return "Local first · Microsoft sync active";
  return "Saved locally on this dashboard";
}
function manualSyncStatusForActiveList(){
  const status=LISTS_STATE.status||{},byList=status.manualListSync||{};
  return byList[LISTS_STATE.active]||null;
}
function manualSyncSeconds(until,reported){
  const target=Number(until||0);
  if(target>0)return Math.max(0,Math.ceil((target-Date.now())/1000));
  return Math.max(0,Number(reported||0));
}
function manualSyncDuration(seconds){
  const value=Math.max(1,Math.ceil(Number(seconds||0)));
  if(value>=60){const minutes=Math.ceil(value/60);return `${minutes} minute${minutes===1?"":"s"}`;}
  return `${value} second${value===1?"":"s"}`;
}
function stopManualSyncCountdown(){
  if(LISTS_STATE.manualSyncTimer){clearTimeout(LISTS_STATE.manualSyncTimer);LISTS_STATE.manualSyncTimer=null;}
}
function manualSyncPresentation(){
  const linked=!!(LISTS_STATE.status&&LISTS_STATE.status.syncActive&&activeListOrigin()==="microsoft");
  if(!linked)return null;
  const state=manualSyncStatusForActiveList()||{},inbound=(LISTS_STATE.status||{}).inboundSync||{};
  const backoff=manualSyncSeconds(state.backoffUntil||inbound.backoffUntil,state.backoffSeconds||inbound.backoffSeconds);
  const cooldown=manualSyncSeconds(state.cooldownUntil,state.cooldownSeconds);
  const running=!!(state.running||inbound.running),queued=!!(state.queued||inbound.queued);
  if(backoff>0)return {disabled:true,label:"Sync paused",note:`Microsoft asked Dash-Go to wait. Sync now is available in ${manualSyncDuration(backoff)}.`,countdown:true};
  if(running&&cooldown>0)return {disabled:true,label:"Syncing…",note:`Checking Microsoft changes. Sync now is available in ${manualSyncDuration(cooldown)}.`,countdown:true};
  if(running)return {disabled:true,label:"Syncing…",note:"Checking Microsoft changes. Sync now returns when this check finishes.",countdown:false};
  if(queued&&cooldown>0)return {disabled:true,label:"Sync queued",note:`Microsoft sync is queued. Sync now is available in ${manualSyncDuration(cooldown)}.`,countdown:true};
  if(queued)return {disabled:true,label:"Sync queued",note:"Microsoft sync is queued behind the current check.",countdown:false};
  if(cooldown>0)return {disabled:true,label:"Sync cooling down",note:`Sync was requested recently. Sync now is available in ${manualSyncDuration(cooldown)}.`,countdown:true};
  return {disabled:false,label:"Sync now",note:"Check Microsoft now. One request per list every 25 seconds.",countdown:false};
}
function updateManualSyncUI(){
  const button=$("listsapp-sync-now"),note=$("listsapp-sync-note"),view=manualSyncPresentation();
  stopManualSyncCountdown();
  if(!button||!view)return;
  button.disabled=!!view.disabled;
  button.textContent=view.label;
  button.setAttribute("aria-label",view.note);
  if(note)note.textContent=view.note;
  if(view.countdown){
    LISTS_STATE.manualSyncTimer=setTimeout(async()=>{
      LISTS_STATE.manualSyncTimer=null;
      const current=manualSyncPresentation();
      if(current&&current.countdown){updateManualSyncUI();return;}
      if(!LISTS_STATE.manualSyncRefreshPending&&LISTS_STATE.active){
        LISTS_STATE.manualSyncRefreshPending=true;
        try{await refreshStatus();}catch(_){ }finally{LISTS_STATE.manualSyncRefreshPending=false;updateManualSyncUI();}
      }
    },1000);
  }
}
function applyInboundSyncStatus(inboundSync,manualSync){
  if(!LISTS_STATE.status)return;
  if(inboundSync)LISTS_STATE.status.inboundSync=inboundSync;
  if(manualSync&&LISTS_STATE.active)LISTS_STATE.status.manualListSync={...(LISTS_STATE.status.manualListSync||{}),[LISTS_STATE.active]:manualSync};
  updateManualSyncUI();
}
async function loadTasks(listId){
  const seq=++LISTS_STATE.loadSeq;
  const cache=await apiJSON(`/api/todo/lists/${encodeURIComponent(listId)}/tasks`);
  if(seq!==LISTS_STATE.loadSeq||listId!==LISTS_STATE.active)return;
  LISTS_STATE.tasks=listsMergePendingMutations(cache.tasks||[]);
  setListsAppTitle(listTitle(listId));
  renderTasks();
  setStatus(`${listTitle(listId)} · ${listSyncLabel(cache)}`);
}

function listsMutationKey(taskID){ return `${LISTS_STATE.active}:${taskID}`; }
function listsPanelIsCurrent(listID,epoch){
  const root=$("listsapp");
  return LISTS_STATE.active===listID&&LISTS_STATE.openEpoch===epoch&&!!(root&&root.classList.contains("show"));
}
function listsMergePendingMutations(tasks){
  const next=Array.isArray(tasks)?tasks.map(task=>({...task})):[];
  for(const [key,mutation] of Object.entries(LISTS_STATE.pendingMutations)){
    if(!key.startsWith(`${LISTS_STATE.active}:`))continue;
    const index=next.findIndex(task=>task&&task.id===mutation.task.id);
    if(index<0)next.push({...mutation.task,...mutation.patch,_saving:true});
    else next[index]={...next[index],...mutation.patch,_saving:true};
  }
  return next;
}
function listsMarkPendingMutation(task,patch){
  const key=listsMutationKey(task.id);
  if(LISTS_STATE.pendingMutations[key])return null;
  const entry={key,task:{...task},patch:{...patch}};
  LISTS_STATE.pendingMutations[key]=entry;
  return entry;
}
function listsClearPendingEntries(entries){
  for(const entry of entries){
    if(LISTS_STATE.pendingMutations[entry.key]===entry)delete LISTS_STATE.pendingMutations[entry.key];
  }
}
function listsRestoreBatchEntries(entries){
  listsClearPendingEntries(entries);
  for(const entry of entries){
    LISTS_STATE.tasks=LISTS_STATE.tasks.map(item=>item&&item.id===entry.task.id?entry.task:item);
  }
}
function listsSchedulePatchBatch(){
  if(LISTS_STATE.patchBatchTimer)return;
  LISTS_STATE.patchBatchTimer=setTimeout(()=>{
    LISTS_STATE.patchBatchTimer=null;
    listsFlushTaskPatchBatch().catch(()=>{});
  },80);
}
async function listsFlushTaskPatchBatch(){
  if(LISTS_STATE.patchBatchTimer){clearTimeout(LISTS_STATE.patchBatchTimer);LISTS_STATE.patchBatchTimer=null;}
  const entries=LISTS_STATE.patchBatchEntries.splice(0);
  if(!entries.length)return;
  const listID=LISTS_STATE.active,epoch=LISTS_STATE.openEpoch;
  const run=apiJSON(`/api/todo/lists/${encodeURIComponent(listID)}/tasks/batch`,"POST",{patches:entries.map(entry=>({id:entry.task.id,patch:entry.patch}))});
  LISTS_STATE.patchBatchRuns.add(run);
  try{
    const result=await run;
    listsClearPendingEntries(entries);
    if(listsPanelIsCurrent(listID,epoch)){
      LISTS_STATE.tasks=listsMergePendingMutations(((result.cache||{}).tasks)||[]);
      renderTasks();
    }
  }catch(error){
    if(listsPanelIsCurrent(listID,epoch)){
      listsRestoreBatchEntries(entries);
      renderTasks();
      setStatus(`Could not save selected ${listItemLowerLabel(listID)}s · ${error.message}`);
    }else{listsClearPendingEntries(entries);}
  }finally{LISTS_STATE.patchBatchRuns.delete(run);}
}
async function listsSettlePendingMutations(){
  await listsFlushTaskPatchBatch();
  while(LISTS_STATE.patchBatchRuns.size){
    await Promise.allSettled([...LISTS_STATE.patchBatchRuns]);
    if(LISTS_STATE.patchBatchEntries.length)await listsFlushTaskPatchBatch();
  }
}
function queueTaskStatusPatch(task,patch){
  const entry=listsMarkPendingMutation(task,patch);
  if(!entry)return;
  LISTS_STATE.patchBatchEntries.push(entry);
  LISTS_STATE.tasks=listsMergePendingMutations(LISTS_STATE.tasks);
  renderTasks();
  listsSchedulePatchBatch();
}
async function requestListSync(listID){
  const response=await apiJSON(`/api/todo/lists/${encodeURIComponent(listID)}/sync`,"POST",{});
  applyInboundSyncStatus(response&&response.inboundSync);
  if(response&&response.inboundSync&&response.inboundSync.running)setStatus(`${listTitle(listID)} · Checking Microsoft changes…`);
  return response;
}
function scheduleStreamRefresh(){
  if(LISTS_STATE.streamRefreshTimer)clearTimeout(LISTS_STATE.streamRefreshTimer);
  LISTS_STATE.streamRefreshTimer=setTimeout(async()=>{
    LISTS_STATE.streamRefreshTimer=null;
    if(LISTS_STATE.active&&!LISTS_STATE.prompt){
      try{await refreshStatus();await loadTasks(LISTS_STATE.active);updateManualSyncUI();}catch(_){ }
    }
  },140);
}
function renderOpenError(message){
  const body=$("listsapp-body");
  if(!body)return;
  body.replaceChildren(Object.assign(document.createElement("div"),{
    className:"lists-empty",
    textContent:"Lists are unavailable right now. "+String(message||"Try again shortly."),
  }));
}
function groceryContext(){
  return {state:LISTS_STATE,apiJSON,bindTap,setStatus,renderTasks,promptText:api.promptText,confirmListsAction:api.confirmListsAction,setTitle:setListsAppTitle,activeListID:()=>LISTS_STATE.active,listTitle,listItemLowerLabel};
}
function renderTasks(){
  const body=$("listsapp-body");
  if(!body||LISTS_STATE.prompt)return;
  body.replaceChildren();
  if(LISTS_STATE.slot==="grocery"&&LISTS_STATE.groceryManage){
    const manager=window.DashGoGroceryQuickAdd;
    if(manager&&typeof manager.renderManager==="function"){
      manager.renderManager(body,groceryContext());
      return;
    }
    LISTS_STATE.groceryManage=false;
  }
  const toolbar=document.createElement("div");
  toolbar.className="lists-toolbar";
  const add=document.createElement("button");
  add.type="button";
  add.className="lists-add";
  add.textContent=`Add ${listItemLabel(LISTS_STATE.active)}`;
  bindTap(add,()=>api.addTask?.());
  const toggle=document.createElement("button");
  toggle.type="button";
  toggle.className="lists-toggle";
  const show=!!LISTS_STATE.showCompleted[LISTS_STATE.active];
  toggle.textContent=show?"Hide completed":"Show completed";
  bindTap(toggle,()=>{
    LISTS_STATE.showCompleted[LISTS_STATE.active]=!show;
    renderTasks();
  });
  toolbar.append(add,toggle);
  let syncNote=null;
  if(LISTS_STATE.status&&LISTS_STATE.status.syncActive&&activeListOrigin()==="microsoft"){
    const sync=document.createElement("button");
    sync.type="button";
    sync.id="listsapp-sync-now";
    sync.className="lists-sync-now";
    bindTap(sync,()=>api.syncCurrentListNow?.());
    toolbar.appendChild(sync);
    syncNote=document.createElement("div");
    syncNote.id="listsapp-sync-note";
    syncNote.className="lists-sync-note";
    syncNote.setAttribute("aria-live","polite");
  }
  const completed=LISTS_STATE.tasks.filter(task=>task.status==="completed");
  if(LISTS_STATE.slot==="grocery"&&completed.length){
    const clear=document.createElement("button"); clear.type="button"; clear.className="lists-clear-completed";
    clear.disabled=LISTS_STATE.clearInFlight;
    clear.textContent=LISTS_STATE.clearInFlight?"Finishing selected changes…":`Clear completed (${completed.length})`;
    bindTap(clear,()=>{if(!LISTS_STATE.clearInFlight)api.clearCompleted?.();}); toolbar.appendChild(clear);
  }
  if(syncNote)toolbar.appendChild(syncNote);
  body.appendChild(toolbar);
  updateManualSyncUI();
  if(LISTS_STATE.slot==="grocery")window.DashGoGroceryQuickAdd?.renderQuickAdd(body,groceryContext());
  window.DashGoListsPeople?.renderFilters?.(body);
  const tasks=LISTS_STATE.tasks.filter(task=>(window.DashGoListsPeople?.matchesTask?.(task)??true)&&(show||task.status!=="completed"));
  if(!tasks.length){
    body.appendChild(Object.assign(document.createElement("div"),{className:"lists-empty",textContent:`No ${listItemLowerLabel(LISTS_STATE.active)}s to show.`}));
    return;
  }
  const list=document.createElement("div");
  list.className="task-list";
  for(const task of tasks){if(typeof api.taskRow==="function")list.appendChild(api.taskRow(task));}
  body.appendChild(list);
}

Object.assign(api,{ $,activeListOrigin,apiJSON,applyInboundSyncStatus,deleteListItemLabel,groceryContext,keepListItemLabel,listIDForSlot,listItemLabel,listItemLowerLabel,listSyncLabel,listTitle,listsClearPendingEntries,listsFlushTaskPatchBatch,listsMarkPendingMutation,listsMergePendingMutations,listsMutationKey,listsPanelIsCurrent,listsRestoreBatchEntries,listsSchedulePatchBatch,listsSettlePendingMutations,loadTasks,manualSyncDuration,manualSyncPresentation,normalizedListSlot,queueTaskStatusPatch,refreshStatus,renderOpenError,renderTasks,requestListSync,scheduleStreamRefresh,setListsAppTitle,setStatus,slotLabel,stopManualSyncCountdown,updateManualSyncUI});
})();
