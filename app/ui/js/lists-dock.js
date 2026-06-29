// 11a-dashboard-lists-dock.js — optional, local-first dashboard Lists ticker.
// The persisted preference arms a bounded cache-only summary + stream. The
// actual grid row appears only while a selected list has an open item.
const DASH_LIST_DOCK={
  enabled:false,
  visible:false,
  expanded:false,
  status:null,
  summary:null,
  slot:"",
  listID:"",
  stream:null,
  loadSeq:0,
  pendingTaskKey:"",
  taskError:"",
  reflowTimer:null,
  tickerTimer:null,
  resizeTimer:null,
  resizeHandler:null,
};
const DASH_LIST_DOCK_MAX_ITEMS=4;
const DASH_LIST_DOCK_TICKER_MAX_ITEMS=12;

function dashboardListsDockShell(){ return document.getElementById("listsdock"); }
function dashboardListsDockApp(){ return document.getElementById("app"); }
function dashboardListsDockSettingEnabled(){
  const todo=(typeof SETTINGS!=="undefined"&&SETTINGS&&SETTINGS.todo)||{};
  return todo.dashboardDock===true;
}
function dashboardListsDockSyncRuntimeSetting(enabled,status){
  if(typeof SETTINGS==="undefined"||!SETTINGS)return;
  const todo={...(SETTINGS.todo||{}),dashboardDock:!!enabled};
  if(status&&status.dashboardDockSlots&&typeof status.dashboardDockSlots==="object")todo.dashboardDockSlots={...status.dashboardDockSlots};
  SETTINGS.todo=todo;
  if(typeof syncDashboardRuntimeSettings==="function")syncDashboardRuntimeSettings();
}
function dashboardListsDockSummarySlots(){
  const slots=DASH_LIST_DOCK.summary&&DASH_LIST_DOCK.summary.slots;
  return Array.isArray(slots)?slots.filter(item=>item&&item.slot&&item.listId):[];
}
function dashboardListsDockHasOpenItems(){
  const total=Number(DASH_LIST_DOCK.summary&&DASH_LIST_DOCK.summary.totalOpenCount);
  if(Number.isFinite(total))return total>0;
  return dashboardListsDockSummarySlots().some(item=>Number(item.openCount)>0);
}
function dashboardListsDockListTitle(listID){
  const list=((DASH_LIST_DOCK.status&&DASH_LIST_DOCK.status.lists)||[]).find(item=>item&&item.id===listID);
  return String((list&&(list.displayName||list.id))||listID||"Lists");
}
function dashboardListsDockItemLabel(){ return `${dashboardListsDockListTitle(DASH_LIST_DOCK.listID)} Item`; }
function dashboardListsDockActiveSlot(){ return dashboardListsDockSummarySlots().find(item=>item.slot===DASH_LIST_DOCK.slot)||null; }
function dashboardListsDockIsCurrent(seq){ return DASH_LIST_DOCK.enabled&&seq===DASH_LIST_DOCK.loadSeq; }
function dashboardListsDockText(value){
  const text=String(value||"").replace(/\s+/g," ").trim()||"Untitled item";
  return text.length>96?text.slice(0,95)+"…":text;
}

function dashboardListsDockScheduleReflow(force){
  if(!DASH_LIST_DOCK.enabled&&!force)return;
  if(DASH_LIST_DOCK.reflowTimer){clearTimeout(DASH_LIST_DOCK.reflowTimer);DASH_LIST_DOCK.reflowTimer=null;}
  requestAnimationFrame(()=>{
    if(!DASH_LIST_DOCK.enabled&&!force)return;
    if(typeof renderCalendar==="function")renderCalendar();
    if(typeof renderAgenda==="function")renderAgenda();
  });
  // One settled reflow covers a real grid-row geometry change; never attach a
  // ResizeObserver or a continuous layout loop to Calendar/Agenda.
  DASH_LIST_DOCK.reflowTimer=setTimeout(()=>{
    DASH_LIST_DOCK.reflowTimer=null;
    if(!DASH_LIST_DOCK.enabled&&!force)return;
    if(typeof renderCalendar==="function")renderCalendar();
    if(typeof renderAgenda==="function")renderAgenda();
  },180);
}
function dashboardListsDockCloseStream(){
  if(DASH_LIST_DOCK.stream){try{DASH_LIST_DOCK.stream.close();}catch(_){}DASH_LIST_DOCK.stream=null;}
}
function dashboardListsDockUnbindResize(){
  if(DASH_LIST_DOCK.resizeTimer){clearTimeout(DASH_LIST_DOCK.resizeTimer);DASH_LIST_DOCK.resizeTimer=null;}
  if(DASH_LIST_DOCK.resizeHandler){window.removeEventListener("resize",DASH_LIST_DOCK.resizeHandler);DASH_LIST_DOCK.resizeHandler=null;}
}
function dashboardListsDockBindResize(){
  if(DASH_LIST_DOCK.resizeHandler)return;
  DASH_LIST_DOCK.resizeHandler=()=>{
    if(!DASH_LIST_DOCK.visible)return;
    if(DASH_LIST_DOCK.resizeTimer)clearTimeout(DASH_LIST_DOCK.resizeTimer);
    DASH_LIST_DOCK.resizeTimer=setTimeout(()=>{DASH_LIST_DOCK.resizeTimer=null;dashboardListsDockMeasureTicker();},140);
  };
  window.addEventListener("resize",DASH_LIST_DOCK.resizeHandler,{passive:true});
}
function dashboardListsDockSetVisible(visible){
  visible=!!visible;
  const changed=visible!==DASH_LIST_DOCK.visible;
  if(!visible)DASH_LIST_DOCK.expanded=false;
  DASH_LIST_DOCK.visible=visible;
  const shell=dashboardListsDockShell(),app=dashboardListsDockApp();
  if(app){
    app.classList.toggle("lists-dock-visible",visible);
    app.classList.toggle("lists-dock-expanded",visible&&DASH_LIST_DOCK.expanded);
  }
  if(shell){
    shell.hidden=!visible;
    shell.setAttribute("aria-hidden",String(!visible));
    if(!visible){
      shell.querySelector("#listsdock-head")?.replaceChildren();
      shell.querySelector("#listsdock-body")?.replaceChildren();
    }
  }
  if(changed)dashboardListsDockScheduleReflow();
}
function dashboardListsDockDisable(){
  const wasVisible=DASH_LIST_DOCK.visible;
  DASH_LIST_DOCK.enabled=false;
  DASH_LIST_DOCK.visible=false;
  DASH_LIST_DOCK.expanded=false;
  DASH_LIST_DOCK.summary=null;
  DASH_LIST_DOCK.loadSeq++;
  DASH_LIST_DOCK.pendingTaskKey="";
  DASH_LIST_DOCK.taskError="";
  dashboardListsDockCloseStream();
  dashboardListsDockUnbindResize();
  if(DASH_LIST_DOCK.reflowTimer){clearTimeout(DASH_LIST_DOCK.reflowTimer);DASH_LIST_DOCK.reflowTimer=null;}
  const shell=dashboardListsDockShell(),app=dashboardListsDockApp();
  if(app)app.classList.remove("lists-dock-visible","lists-dock-expanded");
  if(shell){shell.hidden=true;shell.setAttribute("aria-hidden","true");shell.querySelector("#listsdock-head")?.replaceChildren();shell.querySelector("#listsdock-body")?.replaceChildren();}
  if(wasVisible)dashboardListsDockScheduleReflow(true);
}
function dashboardListsDockReconcileActiveSlot(){
  const slots=dashboardListsDockSummarySlots();
  if(!slots.length){DASH_LIST_DOCK.slot="";DASH_LIST_DOCK.listID="";return;}
  let chosen=slots.find(item=>item.slot===DASH_LIST_DOCK.slot);
  if(!chosen||(!Number(chosen.openCount)&&slots.some(item=>Number(item.openCount)>0)))chosen=slots.find(item=>Number(item.openCount)>0)||slots[0];
  DASH_LIST_DOCK.slot=chosen.slot;
  DASH_LIST_DOCK.listID=chosen.listId;
}
async function dashboardListsDockFetchStatus(){
  const response=await fetch("/api/todo/status",{cache:"no-store"});
  const status=await response.json().catch(()=>({}));
  if(!response.ok)throw new Error(status.error||"Lists status is unavailable");
  return status&&typeof status==="object"?status:{};
}
async function dashboardListsDockFetchSummary(){
  const response=await fetch("/api/todo/dock",{cache:"no-store"});
  const summary=await response.json().catch(()=>({}));
  if(!response.ok)throw new Error(summary.error||"Lists summary is unavailable");
  return summary&&typeof summary==="object"?summary:{};
}
function dashboardListsDockAcceptStatus(status){
  DASH_LIST_DOCK.status=status||{};
  try{if(typeof TODO_STATUS!=="undefined")TODO_STATUS=DASH_LIST_DOCK.status;}catch(_){}
}
function dashboardListsDockInboundSyncError(){
  const sync=DASH_LIST_DOCK.status&&DASH_LIST_DOCK.status.inboundSync;
  const error=String(sync&&sync.lastError||"").trim();
  return error&&error!=="<nil>"?error:"";
}
function dashboardListsDockOpenSyncSettings(){
  const open=window.openDashboardControl;
  if(typeof open!=="function")return;
  open().catch(error=>console.warn("Dashboard Control launch failed",error));
}
function dashboardListsDockAcceptSummary(summary){
  DASH_LIST_DOCK.summary=summary||{};
  dashboardListsDockReconcileActiveSlot();
}
function dashboardListsDockOpenStream(){
  if(!DASH_LIST_DOCK.enabled||DASH_LIST_DOCK.stream||typeof EventSource==="undefined")return;
  try{
    const stream=new EventSource("/api/todo/stream");
    const refresh=()=>{if(DASH_LIST_DOCK.enabled)dashboardListsDockLoadSummary().catch(()=>{});};
    const syncState=()=>{
      if(!DASH_LIST_DOCK.enabled)return;
      // A local status read carries the final inbound error/backoff state. It is
      // event-driven rather than a dashboard poll and never wakes Microsoft Graph.
      dashboardListsDockFetchStatus().then(status=>{
        if(!DASH_LIST_DOCK.enabled)return;
        dashboardListsDockAcceptStatus(status);
        dashboardListsDockRender();
      }).catch(()=>{});
      refresh();
    };
    // The stream uses named `todo` and `sync.state` frames, not unnamed
    // EventSource `message` frames. This keeps the ticker current after a
    // server-owned Microsoft delta pull without browser polling.
    stream.addEventListener("todo",refresh);
    stream.addEventListener("sync.state",syncState);
    stream.onerror=()=>{};
    DASH_LIST_DOCK.stream=stream;
  }catch(_){}
}
async function dashboardListsDockLoadSummary(){
  if(!DASH_LIST_DOCK.enabled)return;
  const seq=++DASH_LIST_DOCK.loadSeq;
  try{
    const summary=await dashboardListsDockFetchSummary();
    if(!dashboardListsDockIsCurrent(seq))return;
    dashboardListsDockAcceptSummary(summary);
  }catch(error){
    if(!dashboardListsDockIsCurrent(seq))return;
    DASH_LIST_DOCK.summary={slots:[],totalOpenCount:0};
    console.warn("Dashboard Lists summary unavailable",error);
  }
  dashboardListsDockRender();
}
function dashboardListsDockButton(label,className,handler){
  const button=document.createElement("button");
  button.type="button";
  button.className=className;
  button.textContent=label;
  bindTap(button,handler);
  return button;
}
function dashboardListsDockOpenFullList(add){
  const slot=DASH_LIST_DOCK.slot;
  if(!slot)return;
  const open=add?window.openListsAppWithAdd:window.openListsApp;
  if(typeof open!=="function")return;
  open(slot).catch(error=>console.warn("Lists dock launch failed",error));
}
function dashboardListsDockSetExpanded(expanded){
  if(!DASH_LIST_DOCK.enabled||!DASH_LIST_DOCK.visible)return;
  DASH_LIST_DOCK.expanded=!!expanded;
  const app=dashboardListsDockApp();
  if(app)app.classList.toggle("lists-dock-expanded",DASH_LIST_DOCK.expanded);
  dashboardListsDockRender();
  dashboardListsDockScheduleReflow();
}
function dashboardListsDockSetSlot(slot){
  if(!DASH_LIST_DOCK.enabled||slot===DASH_LIST_DOCK.slot)return;
  const candidate=dashboardListsDockSummarySlots().find(item=>item.slot===slot);
  if(!candidate)return;
  DASH_LIST_DOCK.slot=candidate.slot;
  DASH_LIST_DOCK.listID=candidate.listId;
  DASH_LIST_DOCK.taskError="";
  dashboardListsDockRender();
}
async function dashboardListsDockPatchTask(task){
  const active=dashboardListsDockActiveSlot();
  if(!DASH_LIST_DOCK.enabled||!active||!task||!task.id)return;
  const key=`${active.listId}:${task.id}`;
  if(DASH_LIST_DOCK.pendingTaskKey)return;
  const done=task.status==="completed";
  DASH_LIST_DOCK.taskError="";
  DASH_LIST_DOCK.pendingTaskKey=key;
  dashboardListsDockRender();
  try{
    const response=await fetch(`/api/todo/lists/${encodeURIComponent(active.listId)}/tasks/${encodeURIComponent(task.id)}`,{
      method:"POST",headers:{"Content-Type":"application/json",Accept:"application/json"},body:JSON.stringify({status:done?"notStarted":"completed"}),
    });
    const cache=await response.json().catch(()=>({}));
    if(!response.ok)throw new Error(cache.error||"Could not update item");
    await dashboardListsDockLoadSummary();
  }catch(error){
    DASH_LIST_DOCK.taskError=`Could not update ${dashboardListsDockText(task.title)}. Try again.`;
    console.warn("Lists dock task update failed",error);
  }finally{
    DASH_LIST_DOCK.pendingTaskKey="";
    if(DASH_LIST_DOCK.enabled)dashboardListsDockRender();
  }
}
function dashboardListsDockTickerItems(){
  const items=[];
  for(const slot of dashboardListsDockSummarySlots()){
    for(const task of Array.isArray(slot.items)?slot.items:[]){
      if(task&&task.status!=="completed")items.push({title:dashboardListsDockText(slot.title),task:dashboardListsDockText(task.title)});
      if(items.length>=DASH_LIST_DOCK_TICKER_MAX_ITEMS)return items;
    }
  }
  return items;
}
function dashboardListsDockBuildTicker(){
  const items=dashboardListsDockTickerItems();
  const text=items.map(item=>`${item.title} · ${item.task}`).join("  •  ");
  const accessible=items.slice(0,4).map(item=>`${item.title}: ${item.task}`).join("; ");
  const button=document.createElement("button");
  button.type="button";
  button.className="listsdock-ticker";
  button.setAttribute("aria-label",`Open Lists items: ${accessible||"Open items"}`);
  const viewport=document.createElement("span");viewport.className="listsdock-ticker-viewport";
  const track=document.createElement("span");track.className="listsdock-ticker-track";
  for(let copy=0;copy<2;copy++){
    const item=document.createElement("span");item.className="listsdock-ticker-copy";item.textContent=text;item.setAttribute("aria-hidden","true");track.appendChild(item);
  }
  viewport.appendChild(track);button.appendChild(viewport);
  bindTap(button,()=>dashboardListsDockSetExpanded(true));
  return button;
}
function dashboardListsDockMeasureTicker(){
  if(!DASH_LIST_DOCK.visible)return;
  const ticker=dashboardListsDockShell()?.querySelector(".listsdock-ticker");
  const viewport=ticker?.querySelector(".listsdock-ticker-viewport");
  const copy=ticker?.querySelector(".listsdock-ticker-copy");
  if(!ticker||!viewport||!copy)return;
  ticker.classList.remove("is-moving");
  if(window.matchMedia&&window.matchMedia("(prefers-reduced-motion: reduce)").matches)return;
  const needed=Math.ceil(copy.scrollWidth)+48;
  if(needed<=viewport.clientWidth)return;
  ticker.style.setProperty("--listsdock-ticker-seconds",`${Math.max(28,Math.min(90,Math.ceil(needed/26)))}s`);
  ticker.classList.add("is-moving");
}
function dashboardListsDockScheduleTickerMeasure(){
  if(DASH_LIST_DOCK.tickerTimer)cancelAnimationFrame(DASH_LIST_DOCK.tickerTimer);
  DASH_LIST_DOCK.tickerTimer=requestAnimationFrame(()=>{DASH_LIST_DOCK.tickerTimer=null;dashboardListsDockMeasureTicker();});
}
function dashboardListsDockRender(){
  if(!DASH_LIST_DOCK.enabled)return;
  dashboardListsDockSetVisible(dashboardListsDockHasOpenItems());
  if(!DASH_LIST_DOCK.visible)return;
  const shell=dashboardListsDockShell();
  const head=document.getElementById("listsdock-head"),body=document.getElementById("listsdock-body");
  if(!shell||!head||!body)return;
  const count=Number(DASH_LIST_DOCK.summary&&DASH_LIST_DOCK.summary.totalOpenCount)||0;
  head.replaceChildren();
  const summary=document.createElement("div");summary.className="listsdock-summary";
  summary.append(Object.assign(document.createElement("div"),{className:"listsdock-title",textContent:"Lists"}),Object.assign(document.createElement("div"),{className:"listsdock-count",textContent:`${count} open`}));
  const syncError=dashboardListsDockInboundSyncError();
  if(syncError){
    const warning=dashboardListsDockButton("Sync issue","listsdock-sync-warning",dashboardListsDockOpenSyncSettings);
    warning.title=syncError;
    warning.setAttribute("aria-label","Microsoft To Do sync needs attention: "+syncError+". Open Dashboard Control.");
    summary.appendChild(warning);
  }
  const ticker=dashboardListsDockBuildTicker();
  const toggle=dashboardListsDockButton(DASH_LIST_DOCK.expanded?"Collapse":"View items","listsdock-action primary",()=>dashboardListsDockSetExpanded(!DASH_LIST_DOCK.expanded));
  toggle.setAttribute("aria-expanded",String(DASH_LIST_DOCK.expanded));
  head.append(summary,ticker,toggle);
  if(!DASH_LIST_DOCK.expanded){body.replaceChildren();dashboardListsDockScheduleTickerMeasure();return;}

  body.replaceChildren();
  const slots=dashboardListsDockSummarySlots();
  const tabs=document.createElement("nav");tabs.className="listsdock-tabs";tabs.setAttribute("aria-label","Lists shown in dashboard dock");
  for(const item of slots){
    const tab=dashboardListsDockButton(item.title,"listsdock-tab"+(item.slot===DASH_LIST_DOCK.slot?" on":""),()=>dashboardListsDockSetSlot(item.slot));
    tab.setAttribute("aria-pressed",String(item.slot===DASH_LIST_DOCK.slot));
    tabs.appendChild(tab);
  }
  body.appendChild(tabs);
  const active=dashboardListsDockActiveSlot();
  if(!active){body.appendChild(Object.assign(document.createElement("div"),{className:"listsdock-empty",textContent:"No selected list is available."}));return;}
  const toolbar=document.createElement("div");toolbar.className="listsdock-toolbar";
  toolbar.append(
    dashboardListsDockButton(`Add ${dashboardListsDockItemLabel()}`,"listsdock-action primary listsdock-add",()=>dashboardListsDockOpenFullList(true)),
    dashboardListsDockButton("Open full list","listsdock-action",()=>dashboardListsDockOpenFullList(false)),
  );
  body.appendChild(toolbar);
  const openTasks=(Array.isArray(active.items)?active.items:[]).filter(task=>task&&task.status!=="completed").slice(0,DASH_LIST_DOCK_MAX_ITEMS);
  if(!openTasks.length){body.appendChild(Object.assign(document.createElement("div"),{className:"listsdock-empty",textContent:`No open ${String(active.title).toLowerCase()} items.`}));return;}
  const list=document.createElement("div");list.className="listsdock-task-list";
  for(const task of openTasks){
    const row=document.createElement("article");row.className="listsdock-task";
    const key=`${active.listId}:${task.id}`;
    const check=dashboardListsDockButton("","listsdock-check",()=>dashboardListsDockPatchTask(task));
    check.setAttribute("aria-label",`Complete: ${task.title||"item"}`);
    check.disabled=DASH_LIST_DOCK.pendingTaskKey===key;
    const title=task.title||"Untitled item",main=dashboardListsDockButton("","listsdock-task-title",()=>dashboardListsDockOpenFullList(false));
    main.title=title;main.setAttribute("aria-label",task.assignee?`${title} · Responsible: ${task.assignee}`:title);const label=document.createElement("span");label.className="listsdock-task-title-label";label.textContent=title;main.appendChild(label);
    if(task.assignee){const person=document.createElement("small");person.className="listsdock-task-person";person.textContent=task.assignee;main.appendChild(person);}row.append(check,main);list.appendChild(row);
  }
  body.appendChild(list);
  if(DASH_LIST_DOCK.taskError){
    const feedback=document.createElement("div");
    feedback.className="listsdock-task-status";
    feedback.setAttribute("role","status");
    feedback.textContent=DASH_LIST_DOCK.taskError;
    body.appendChild(feedback);
  }
  dashboardListsDockScheduleTickerMeasure();
}
async function dashboardListsDockEnable(status){
  if(!DASH_LIST_DOCK.enabled){
    DASH_LIST_DOCK.enabled=true;
    DASH_LIST_DOCK.expanded=false;
    dashboardListsDockBindResize();
  }
  try{
    dashboardListsDockAcceptStatus(status||await dashboardListsDockFetchStatus());
    dashboardListsDockOpenStream();
    await dashboardListsDockLoadSummary();
  }catch(error){
    DASH_LIST_DOCK.summary={slots:[],totalOpenCount:0};
    dashboardListsDockRender();
    console.warn("Dashboard Lists dock unavailable",error);
  }
}
function dashboardListsDockSetEnabled(enabled,status){
  dashboardListsDockSyncRuntimeSetting(enabled,status);
  if(!enabled){dashboardListsDockDisable();return;}
  dashboardListsDockEnable(status);
}
function dashboardListsDockSettingsChanged(){
  const enabled=dashboardListsDockSettingEnabled();
  if(!enabled){if(DASH_LIST_DOCK.enabled)dashboardListsDockDisable();return;}
  if(!DASH_LIST_DOCK.enabled)dashboardListsDockSetEnabled(true);
}
window.dashboardListsDockSetEnabled=dashboardListsDockSetEnabled;
window.dashboardListsDockSettingsChanged=dashboardListsDockSettingsChanged;
