let CTRL_TODO_RENDER_SEQ=0;

function todoControlRoot(){
  return document.querySelector("#ctrlpage-control [data-todo-control-root]");
}

function todoControlRenderCurrent(root,seq){
  return root===todoControlRoot() && seq===CTRL_TODO_RENDER_SEQ;
}

function todoListOriginLabel(list){
  return list&&list.origin==="microsoft"?"Microsoft":"Local";
}
function todoMappedSlots(st,origin){
  const byID=new Map((st.lists||[]).map(list=>[list.id,list]));
  return ["todo","grocery"].map(slot=>{
    const list=byID.get((st.map||{})[slot]);
    return list?{slot,list}:null;
  }).filter(item=>item&&todoListOriginLabel(item.list).toLowerCase()===origin);
}
function todoDashboardDockSlots(st){
  const raw=(st&&st.dashboardDockSlots&&typeof st.dashboardDockSlots==="object")?st.dashboardDockSlots:{};
  const slots={todo:raw.todo!==false,grocery:raw.grocery!==false};
  if(!slots.todo&&!slots.grocery)return {todo:true,grocery:true};
  return slots;
}
function todoDashboardDockSlotLabel(slot){ return slot==="todo"?"To Do":"Grocery"; }

const TODO_INBOUND_SYNC_SECONDS=25;
function todoInboundSyncState(st){
  const raw=(st&&st.inboundSync&&typeof st.inboundSync==="object")?st.inboundSync:{};
  return {...raw,configuredSeconds:TODO_INBOUND_SYNC_SECONDS,mode:"automatic"};
}
function todoInboundSyncDetail(sync){
  if(sync.lastError)return "Microsoft task sync needs attention: "+sync.lastError;
  if(sync.backoffSeconds>0)return `Microsoft asked Dash-Go to wait about ${sync.backoffSeconds} seconds before the next pull.`;
  if(sync.running&&sync.queued)return "Microsoft task sync is running. One follow-up check is queued; additional requests are being combined, not stacked.";
  if(sync.running)return "Microsoft task sync is running now.";
  if(sync.queued)return "Microsoft task sync has one follow-up check queued.";
  return "Automatic sync checks mapped household Microsoft lists about every 25 seconds while idle. Opening To Do or Grocery requests an immediate background check; Dash-Go list changes save locally, then send and reconcile afterward. Use Sync Microsoft To Do now after a phone change when you want an immediate check.";
}
function todoInboundSyncStatusCard(sync){
  const box=el("div","todo-map-card"),title=el("div","settinglabel","Automatic Microsoft sync · every 25 seconds");
  box.append(title,el("div","settingdesc",todoInboundSyncDetail(sync)));
  return box;
}

function todoMigrationSelect(label,items){
  const box=el("div","todo-map-card"),title=el("div","settinglabel",label),sel=document.createElement("select");
  sel.className="todo-map-select";
  sel.setAttribute("aria-label",label);
  items.forEach(({slot,list})=>sel.appendChild(new Option((slot==="todo"?"To Do":"Grocery")+" · "+(list.displayName||list.id),slot)));
  box.append(title,sel);
  return {box,select:sel};
}
async function runTodoMigration(action,slot){
  try{
    const result=await api("/api/todo/migrate","POST",{action,slot});
    const destination=(result.destination&&result.destination.displayName)||"new list";
    ctrlMsg("List migration complete · "+destination+" is now mapped. The prior list archive expires automatically in 90 days.");
    if(typeof refreshTodoStatus==="function") await refreshTodoStatus(true);
      await renderCtrlTodo();
  }catch(error){
    ctrlMsg(error&&error.message?error.message:"List migration could not be completed.");
  }
}
async function resolveTodoBlockedWrites(listID,action){
  try{
    const response=await api("/api/todo/lists/"+encodeURIComponent(listID)+"/sync-failures","POST",{action});
    const count=Number(response.resolved||0);
    const label=count===1?"1 Dash-Go change":""+count+" Dash-Go changes";
    if(action==="retry")ctrlMsg(label+" will retry Microsoft sync.");
    else if(action==="keep-local")ctrlMsg(label+" will remain local and stop mirroring to Microsoft.");
    else ctrlMsg(label+" were discarded and the Microsoft list will be refreshed.");
    if(typeof refreshTodoStatus==="function")await refreshTodoStatus(true);
    await renderCtrlTodo();
  }catch(error){
    ctrlMsg(error&&error.message?error.message:"Could not resolve the blocked Microsoft change.");
  }
}
function todoBlockedWriteGroups(st){
  const blocked=Array.isArray(st&&st.blockedWrites)?st.blockedWrites:[];
  return blocked.filter(item=>item&&item.listId&&Number(item.count)>0).map(item=>{
    const count=Number(item.count);
    const title=item.title||"Microsoft list";
    const group=actionGroup("Microsoft sync decision · "+title,`${count} Dash-Go change${count===1?"":"s"} could not be sent after bounded retries. Phone-to-dashboard pulls continue. Choose a recovery path; none of these actions exposes task text or Microsoft credentials.`,"displaygroup ctrl-todo-sync-recovery");
    group.grid.append(
      caction("Retry failed Dash-Go changes","Return the saved local changes to the bounded Microsoft queue.","on",()=>resolveTodoBlockedWrites(item.listId,"retry")),
      confirmAction("Keep local only","Retain the saved local change on this dashboard and stop mirroring it.","Tap again to keep local",()=>resolveTodoBlockedWrites(item.listId,"keep-local")),
      confirmAction("Discard failed Dash-Go changes","Remove the failed local operation and refresh this list from Microsoft.","Tap again to discard",()=>resolveTodoBlockedWrites(item.listId,"discard"))
    );
    return group.group;
  });
}

function todoMigrationGroups(st){
  if(!st.syncActive)return [];
  const groups=[];
  const localSlots=todoMappedSlots(st,"local");
  if(localSlots.length){
    const migration=actionGroup("Move a local list to Microsoft","Choose one currently local App destination. Dash-Go creates a new Microsoft list with the same title, maps the tile only after the migration is ready, and keeps a local read-only archive for 90 days. The new active Microsoft list is never auto-deleted.","displaygroup ctrl-todo-migration");
    const picker=todoMigrationSelect("Local App destination",localSlots);
    migration.grid.append(
      picker.box,
      confirmAction("Copy local items to Microsoft","Tap again to create a Microsoft copy",()=>runTodoMigration("local-to-microsoft-copy",picker.select.value)),
      confirmAction("Archive local / fresh Microsoft","Tap again to start a fresh Microsoft list",()=>runTodoMigration("local-to-microsoft-fresh",picker.select.value))
    );
    groups.push(migration.group);
  }
  const microsoftSlots=todoMappedSlots(st,"microsoft");
  if(microsoftSlots.length){
    const migration=actionGroup("Disconnect Microsoft and return local","Choose one Microsoft-mapped App destination. Dash-Go refreshes its cache before switching and creates a local destination. Microsoft is unlinked only after every Microsoft-mapped App destination has returned local. A 90-day local archive snapshot is retained; the Microsoft account list itself is left untouched.","displaygroup ctrl-todo-migration");
    const picker=todoMigrationSelect("Microsoft App destination",microsoftSlots);
    migration.grid.append(
      picker.box,
      confirmAction("Copy Microsoft items to local","Tap again to copy and unlink",()=>runTodoMigration("microsoft-to-local-copy",picker.select.value)),
      confirmAction("Archive Microsoft list / fresh local","Tap again to start a fresh local list",()=>runTodoMigration("microsoft-to-local-fresh",picker.select.value))
    );
    groups.push(migration.group);
  }
  return groups;
}

async function renderCtrlTodo(){
  const wrap=todoControlRoot();
  if(!wrap) throw new Error("Lists Control root is missing");
  const seq=++CTRL_TODO_RENDER_SEQ;
  wrap.replaceChildren(ctrlStateCard("loading","Loading Lists","Checking local Lists state. Microsoft To Do is optional."));
  let st={source:"local",state:"local",syncMode:"local",lists:[],map:{todo:"local-todo",grocery:"local-grocery"}};
  try{
    st=await api("/api/todo/status","GET");
  }catch(error){
    if(!todoControlRenderCurrent(wrap,seq))return;
    wrap.replaceChildren(ctrlStateCard("warn","Lists status unavailable",error&&error.message?error.message:"The local Lists service did not respond.",[cbtn("Try again","",()=>renderCtrlTodo())]));
    return;
  }
  if(!todoControlRenderCurrent(wrap,seq))return;
  wrap.replaceChildren();
  const syncActive=!!st.syncActive;
  const stateLabel=syncActive?"Microsoft sync linked":(st.syncMode==="microsoft"?"Microsoft sync needs linking":"Local lists only");
  const stateDetail=syncActive
    ?"Microsoft is linked. Existing local mappings remain local until you deliberately move a list below; only Microsoft-origin lists mirror to Graph."
    :"Tasks stay on this dashboard. No Microsoft account or network connection is required.";
  wrap.appendChild(ctrlStateCard(syncActive?"good":"info",stateLabel,stateDetail));

  const dashboardDock=actionGroup("Bottom Lists dock","Optional dashboard ticker. When armed, it appears only while one selected list has an open item. It uses a bounded in-flow bottom row, so the calendar and sidebar shrink in place instead of being pushed off-screen.","displaygroup ctrl-todo-dashboard-dock");
  dashboardDock.grid.append(
    caction("Show bottom Lists dock","Arm the compact dashboard ticker. It appears automatically when a selected list has an open item.",st.dashboardDock===true?"on":"",async()=>{
      await setTodoDashboardDockVisible(true);
    }),
    caction("Hide bottom Lists dock","Remove the ticker and restore the dashboard’s full available height.",st.dashboardDock!==true?"on":"",async()=>{
      await setTodoDashboardDockVisible(false);
    })
  );
  wrap.appendChild(dashboardDock.group);

  const dockSlots=todoDashboardDockSlots(st);
  const visibleDockSlots=actionGroup("Lists shown in Bottom Lists dock","Choose which permanent list destinations contribute to the dashboard ticker. This does not change the Apps launcher, list destinations, or any tasks. Choose at least one list.","displaygroup ctrl-todo-dashboard-dock-slots");
  for(const slot of ["todo","grocery"]){
    const selected=!!dockSlots[slot];
    visibleDockSlots.grid.append(caction(
      todoDashboardDockSlotLabel(slot),
      selected?"Shown in the Bottom Lists ticker.":"Hidden from the Bottom Lists ticker.",
      selected?"on":"",
      async()=>{
        if(selected&&Object.values(dockSlots).filter(Boolean).length<=1){
          ctrlMsg("Choose at least one list for the Bottom Lists dock.");
          return;
        }
        await setTodoDashboardDockSlots({...dockSlots,[slot]:!selected});
      }
    ));
  }
  wrap.appendChild(visibleDockSlots.group);

  const source=actionGroup("List source","Use local lists as the default household source of truth. Microsoft To Do remains an opt-in mirror and can be paused without deleting local tasks.","displaygroup ctrl-todo-source");
  source.grid.append(
    caction("Use local lists","Keep To Do and Grocery local only.",st.syncMode!=="microsoft"?"on":"",async()=>{
      await api("/api/todo/source","POST",{syncMode:"local"});
      ctrlMsg("Local Lists are active. Existing Microsoft tokens were left untouched.");
      await renderCtrlTodo();
    }),
    caction("Use Microsoft sync","Prepare optional Microsoft To Do mirroring.",st.syncMode==="microsoft"?"on":"",async()=>{
      await api("/api/todo/source","POST",{syncMode:"microsoft"});
      ctrlMsg("Microsoft sync is prepared. Enter a client ID and link the account below.");
      await renderCtrlTodo();
    })
  );
  wrap.appendChild(source.group);

  const link=actionGroup("Optional Microsoft To Do","A private public-client application ID is required. The dashboard stores refresh tokens only in its owner-only home-directory file; no token is sent to the browser.","displaygroup ctrl-todo-account");
  const client=oskInput("Microsoft client ID",String(st.clientId||""),{});
  client.className+=" todo-client-id";
  link.grid.appendChild(client);
  const linkAccount=caction("Link Microsoft account","Start device-code sign-in after you enter the private client ID.","on",async()=>{
    try{
      const result=await api("/api/todo/auth/start","POST",{clientId:client.value.trim()});
      ctrlMsg("On your phone or computer, visit "+result.verificationUri+" and enter code "+result.userCode+".");
      await renderCtrlTodo();
    }catch(error){ctrlMsg(error.message);}
  });
  oskSetSubmit(client,"Link",()=>linkAccount.click());
  link.grid.append(
    linkAccount,
    caction("Cancel link","Stop pending device-code sign-in.","",async()=>{
      await api("/api/todo/auth/cancel","POST",{});
      await renderCtrlTodo();
    }),
    ...(!todoMappedSlots(st,"microsoft").length?[confirmAction("Unlink Microsoft","All current App destinations are local. Remove the saved Microsoft token.","Tap again to unlink",async()=>{
      await api("/api/todo/unlink","POST",{});
      await renderCtrlTodo();
    })]:[])
  );
  wrap.appendChild(link.group);
  if(st.auth&&st.auth.userCode)wrap.appendChild(ctrlStateCard("warn","Microsoft device code",`${st.auth.userCode} · ${st.auth.verificationUri}`));

  if(syncActive){
    const syncState=todoInboundSyncState(st);
    const sync=actionGroup("Microsoft task sync","Dash-Go uses one bounded cloud coordinator for phone changes, open-list checks, local writes, and Sync now. The periodic safety check is fixed at every 25 seconds while the coordinator, bounded candidates, and Graph backoff keep the work controlled.","displaygroup ctrl-todo-sync");
    sync.grid.append(
      todoInboundSyncStatusCard(syncState),
      caction("Sync Microsoft To Do now","Push any queued Dash-Go changes, then pull phone and Microsoft changes for mapped and already-tracked household lists.","on",async()=>{
        await runTodoInboundSync();
      })
    );
    wrap.appendChild(sync.group);
    todoBlockedWriteGroups(st).forEach(group=>wrap.appendChild(group));
  }

  const maps=actionGroup("List destinations","To Do and Grocery are permanent Apps destinations. They start mapped to built-in local lists; each option identifies its Local or Microsoft origin. Changing a destination does not move existing items; use the deliberate migration actions below when you want a copy or fresh start.","displaygroup ctrl-todo-map");
  maps.grid.append(todoMapSelect("todo","To Do destination",st),todoMapSelect("grocery","Grocery destination",st));
  wrap.appendChild(maps.group);
  todoMigrationGroups(st).forEach(group=>wrap.appendChild(group));
  if((st.archives||[]).length){
    const count=st.archives.length;
    wrap.appendChild(ctrlStateCard("info",count+" migration archive"+(count===1?"":"s"),"Previous-list snapshots are read-only, local to this dashboard, and purge automatically 90 days after migration. Active destination lists are not auto-deleted."));
  }

  const lists=actionGroup("Lists management",syncActive?"Create a Microsoft list, then map an icon to it. Local lists remain the write-first source.":"Create another local list for this dashboard. Microsoft sync is optional.","displaygroup ctrl-todo-lists");
  const newList=oskInput("New list name","",{});
  newList.className+=" todo-client-id todo-new-list";
  if(syncActive)lists.grid.append(caction("Refresh available Microsoft lists","Refresh list names for List destinations. This does not pull task items.","",async()=>{
    try{
      await api("/api/todo/lists/refresh","POST",{});
      ctrlMsg("Available Microsoft lists refreshed.");
      await renderCtrlTodo();
    }catch(error){ctrlMsg(error&&error.message?error.message:"Microsoft list names could not be refreshed.");}
  }));
  const createList=caction("Create list",syncActive?"Create a list in Microsoft To Do.":"Create a local list on this dashboard.","",async()=>{
    const name=newList.value.trim();
    if(!name){
      ctrlMsg("Enter a list name first.");
      showOSKFor(newList);
      return;
    }
    try{
      await api("/api/todo/lists","POST",{displayName:name});
      await renderCtrlTodo();
    }catch(error){ctrlMsg(error.message);}
  });
  oskSetSubmit(newList,"Create",()=>createList.click());
  lists.grid.append(newList,createList);
  wrap.appendChild(lists.group);
  buildOSK(wrap);
}
async function setTodoDashboardDockVisible(enabled){
  try{
    const next=await api("/api/todo/dock","POST",{enabled:!!enabled});
    if(typeof dashboardListsDockSetEnabled==="function")dashboardListsDockSetEnabled(!!next.dashboardDock,next);
    if(typeof refreshTodoStatus==="function")await refreshTodoStatus(true);
    ctrlMsg(next.dashboardDock?"Bottom Lists dock is armed. It appears when a selected list has an open item.":"Bottom Lists dock is hidden. The dashboard returned to its full height.");
    await renderCtrlTodo();
  }catch(error){
    ctrlMsg(error&&error.message?error.message:"Could not update the bottom Lists dock.");
  }
}
async function setTodoDashboardDockSlots(slots){
  try{
    const next=await api("/api/todo/dock/slots","POST",{slots});
    if(typeof dashboardListsDockSetEnabled==="function")dashboardListsDockSetEnabled(!!next.dashboardDock,next);
    if(typeof refreshTodoStatus==="function")await refreshTodoStatus(true);
    const names=["todo","grocery"].filter(slot=>next.dashboardDockSlots&&next.dashboardDockSlots[slot]).map(todoDashboardDockSlotLabel);
    ctrlMsg("Bottom Lists dock now follows "+names.join(" and ")+".");
    await renderCtrlTodo();
  }catch(error){
    ctrlMsg(error&&error.message?error.message:"Could not update the lists shown in the Bottom Lists dock.");
  }
}

async function runTodoInboundSync(){
  try{
    const response=await api("/api/todo/sync","POST",{});
    if(response.result&&response.result.alreadyRunning){
      ctrlMsg(response.result.queued?"Microsoft To Do sync is running; one follow-up check is queued.":"Microsoft To Do sync is already running.");
    }else if(response.result&&response.result.skipped){
      ctrlMsg(response.result.reason||"No Microsoft list is ready to sync.");
    }else{
      ctrlMsg(response.summary||"Microsoft To Do synced.");
    }
    if(typeof refreshTodoStatus==="function")await refreshTodoStatus(true);
    await renderCtrlTodo();
  }catch(error){
    ctrlMsg(error&&error.message?error.message:"Microsoft To Do could not be synced.");
  }
}

function todoMapSelect(slot,label,st){
  const box=el("div","todo-map-card"),title=el("div","settinglabel",label),sel=document.createElement("select");
  sel.className="todo-map-select";
  sel.setAttribute("aria-label",label);
  (st.lists||[]).forEach(list=>sel.appendChild(new Option((list.displayName||list.id)+" · "+todoListOriginLabel(list),list.id)));
  const fallback=slot==="todo"?"local-todo":"local-grocery";
  sel.value=(st.map&&st.map[slot])||fallback;
  const save=cbtn("Save mapping","on",async()=>{
    const body={};
    body[slot]=sel.value;
    await api("/api/todo/map","POST",body);
    ctrlMsg(label+" updated.");
    if(typeof refreshTodoStatus==="function")await refreshTodoStatus(true);
  });
  box.append(title,sel,save);
  return box;
}
