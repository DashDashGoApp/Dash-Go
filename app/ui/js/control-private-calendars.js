// Private-calendar discovery is a separate, lazy Calendar Manager action.
// Discovery is inventory-only until a user explicitly adds a selected source.
function ctrlPrivateCalendarSettings(state,writeback){
  state=state||{available:false,discovering:false,candidates:[],selected:[],notices:[]};
  writeback=writeback||{enabled:false,requirePin:false,calendars:[]};
  const card=el("section","calendar-manager-group calprivate-settings");
  card.append(el("div","calmanager-heading","Private calendar connections"));
  card.append(el("p","calmanager-note","Discover checks connected Google, iCloud, and CalDAV accounts for calendars. Nothing discovered here syncs or changes until you add it."));
  const actions=el("div","calmanager-actions");
  if(state.available){
    actions.appendChild(ctrlCalendarManagerAction(state.discovering?"Discovering calendars…":"Discover available calendars",state.discovering?"Reading connected accounts without changing active calendars.":"Refresh the review-only inventory. Existing selections stay unchanged.","primary",async()=>{
      await api("/api/calendars/private/discover","POST",{});
      await ctrlCalendarManagerRefresh("Private calendar discovery finished. Nothing was activated automatically.");
    }));
  }else actions.appendChild(el("span","calmanager-note","Set up a private Google, iCloud, or CalDAV connection first."));
  card.appendChild(actions);
  (Array.isArray(state.notices)?state.notices:[]).forEach(note=>card.appendChild(el("p","calmanager-note",String(note))));
  if(state.legacyBroadMirror===true&&Array.isArray(state.selected)&&state.selected.length)card.appendChild(el("p","calmanager-note","A legacy all-calendars mirror is still preserved. It may overlap these exact selected calendars; review it here and hide it only after you confirm the selected sources look correct."));
  const selected=Array.isArray(state.selected)?state.selected:[];
  if(selected.length){
    card.appendChild(el("div","calmanager-heading","Selected calendars"));
    const list=el("div","calmanager-list");selected.forEach(item=>list.appendChild(ctrlPrivateCalendarSelectedRow(item,writeback)));card.appendChild(list);
  }
  const candidates=(Array.isArray(state.candidates)?state.candidates:[]).filter(item=>item&&item.selected!==true);
  if(candidates.length){
    card.appendChild(el("div","calmanager-heading","Available to add"));
    const list=el("div","calmanager-list");candidates.forEach(item=>list.appendChild(ctrlPrivateCalendarCandidateRow(item)));card.appendChild(list);
  }else if(!selected.length&&!state.discovering)card.appendChild(ctrlStateCard("empty","No discovered private calendars","Use Discover available calendars to review connected account collections. Discovery never enables sync by itself."));
  return card;
}
function ctrlPrivateCalendarProvider(item){return String(item.providerLabel||((item.provider==="google")?"Google":"iCloud / CalDAV"));}
function ctrlPrivateCalendarRegistry(item,writeback){return (Array.isArray(writeback&&writeback.calendars)?writeback.calendars:[]).find(row=>row&&row.source===item.source)||null;}
function ctrlPrivateCalendarSync(item){return String(item&&item.sync||"");}
function ctrlPrivateCalendarConflict(item){return ctrlPrivateCalendarSync(item)==="conflict"||String(item&&item.state||"")==="conflict";}
function ctrlPrivateCalendarNeedsRepair(item){return ctrlPrivateCalendarSync(item)==="attention-undiscovered";}
function ctrlPrivateCalendarAuthHelp(item){
  const sync=ctrlPrivateCalendarSync(item);
  if(sync!=="attention-auth"&&sync!=="skipped")return "";
  return item&&item.provider==="google"
    ?"Google authorization is required. Reconnect this account through private-calendar setup from SSH, then return here and use Sync now."
    :"The calendar server rejected authentication. Update the account or app password through private-calendar setup from SSH, then return here and use Sync now.";
}
function ctrlPrivateCalendarPresentation(item,writeback){
  const provider=ctrlPrivateCalendarProvider(item),registered=ctrlPrivateCalendarRegistry(item,writeback),sync=ctrlPrivateCalendarSync(item);
  if(ctrlPrivateCalendarConflict(item))return {kind:"conflict",label:"Conflict",detail:`${provider} · Sync conflict — choose a version`};
  if(sync==="attention-undiscovered")return {kind:"attention",label:"Needs attention",detail:`${provider} · Connection repair required`};
  if(sync==="attention-auth"||sync==="skipped")return {kind:"attention",label:"Authorization required",detail:`${provider} · Authorization required`};
  if(sync==="attention-empty")return {kind:"attention",label:"Needs attention",detail:`${provider} · Local sync safety check required`};
  if(sync==="failed")return {kind:"attention",label:"Needs attention",detail:`${provider} · Last sync failed`};
  if(item.writable!==true)return {kind:"readonly",label:"View-only",detail:`${provider} · Dash-Go cannot change it`};
  if(!registered)return {kind:"attention",label:"Needs attention",detail:`${provider} · Writeback registration required`};
  if(writeback&&writeback.enabled!==true)return {kind:"readonly",label:"Edits off",detail:`${provider} · Dashboard edits off`};
  if(registered.enabled===false)return {kind:"readonly",label:"Edits off",detail:`${provider} · Dashboard edits disabled`};
  return {kind:"healthy",label:"Two-way",detail:`${provider} · Dash-Go can send approved changes`};
}
function ctrlPrivateCalendarConflictActions(item,row){
  const reveal=ctrlCalendarManagerAction("Resolve conflict","Review which version should win for every unresolved conflict in this one calendar. A Dashboard Control PIN is required.","warning",async()=>{
    reveal.hidden=true;
    const panel=el("section","calmanager-conflict-options");
    panel.append(el("div","calmanager-conflict-heading","Resolve calendar conflict"),el("p","calmanager-note",`Normal sync is paused to protect both versions. A choice applies to every unresolved conflict in ${item.name||"this calendar"}.`));
    if(item.deleteAllowed!==true){
      panel.appendChild(el("p","calmanager-note","Configure and unlock a Dashboard Control PIN before choosing a version. Both versions remain unchanged until then."));
    }else{
      const choices=el("div","calmanager-actions");
      choices.appendChild(ctrlCalendarManagerConfirmAction("Use phone / remote version","Replace the local conflicting items with the remote calendar’s version. An owner-only local snapshot is kept.","Tap again: use remote version",async()=>{
        await api("/api/calendars/private/resolve","POST",{source:item.source,winner:"remote"});
        await ctrlCalendarManagerRefresh(`${item.name||"Private calendar"} conflict resolved using the remote version.`,item.source,true);
      }));
      choices.appendChild(ctrlCalendarManagerConfirmAction("Use this dashboard’s version","Push this dashboard’s local conflicting items to the remote calendar. An owner-only local snapshot is kept.","Tap again: use Dashboard version",async()=>{
        await api("/api/calendars/private/resolve","POST",{source:item.source,winner:"dashboard"});
        await ctrlCalendarManagerRefresh(`${item.name||"Private calendar"} conflict resolved using the Dashboard version.`,item.source,true);
      }));
      panel.appendChild(choices);
    }
    const exit=el("div","calmanager-conflict-exit");
    exit.appendChild(ctrlCalendarManagerAction("Keep both unchanged","Close these choices. You can decide later; normal sync remains paused.","",async()=>{panel.remove();reveal.hidden=false;}));
    panel.appendChild(exit);row.insertBefore(panel,reveal.nextSibling);
  });
  return reveal;
}
function ctrlPrivateCalendarCandidateRow(item){
  const row=el("article","calmanager-row calmanager-private");
  const head=el("div","calmanager-head"),title=el("div","calmanager-title"),dot=el("span","calmanager-dot");dot.style.background=ctrlCalendarChipColor(item.color||item.name);title.append(dot,el("strong","",item.name||"Private calendar"));head.append(title,el("span","calmanager-state available","Available"));
  row.append(head,el("div","calmanager-detail",`${ctrlPrivateCalendarProvider(item)} · Not syncing yet`));
  const actions=el("div","calmanager-actions");
  actions.appendChild(ctrlCalendarManagerAction("Add view-only","Show this calendar on Dash-Go. Dash-Go cannot change the provider calendar.","",async()=>ctrlPrivateCalendarActivate(item,false)));
  actions.appendChild(ctrlCalendarManagerAction("Add with two-way sync","Create one exact signed-in calendar source. Supported Dash-Go changes can sync through this provider.","primary",async()=>ctrlPrivateCalendarActivate(item,true)));
  row.appendChild(actions);return row;
}
function ctrlPrivateCalendarSelectedRow(item,writeback){
  const row=el("article","calmanager-row calmanager-private");row.dataset.calendarSource=String(item.source||"");
  const head=el("div","calmanager-head"),title=el("div","calmanager-title"),dot=el("span","calmanager-dot");dot.style.background=ctrlCalendarChipColor(item.color||item.name);title.append(dot,el("strong","",item.name||"Private calendar"));
  const registered=ctrlPrivateCalendarRegistry(item,writeback),conflict=ctrlPrivateCalendarConflict(item),repair=ctrlPrivateCalendarNeedsRepair(item),presentation=ctrlPrivateCalendarPresentation(item,writeback);
  head.append(title,el("span",`calmanager-state ${presentation.kind}`,presentation.label));row.append(head,el("div","calmanager-detail",presentation.detail));
  const authHelp=ctrlPrivateCalendarAuthHelp(item);if(authHelp)row.appendChild(el("p","calmanager-note",authHelp));
  const actions=el("div","calmanager-actions");
  if(conflict){
    actions.appendChild(ctrlPrivateCalendarConflictActions(item,row));
  }else if(repair){
    actions.appendChild(ctrlCalendarManagerAction("Repair connection","Run one deliberate discovery and sync for this exact selected calendar. It does not change calendar selections or remote events.","primary",async()=>{
      await api("/api/calendars/private/repair","POST",{source:item.source});
      await ctrlCalendarManagerRefresh(`${item.name||"Private calendar"} connection repaired.`,item.source,true);
    }));
  }else if(item.writable!==true){
    actions.appendChild(ctrlCalendarManagerConfirmAction("Switch to two-way sync","Dash-Go will first verify this exact provider calendar, then allow supported calendar changes to sync back. Your sign-in and selected calendar stay the same.","Tap again: enable two-way sync",async()=>{
      await api("/api/calendars/private/editable","POST",{source:item.source,editable:true});
      await ctrlCalendarManagerRefresh(`${item.name||"Private calendar"} now has verified two-way sync.`,item.source);
    }));
  }else if(!registered){
    actions.appendChild(ctrlCalendarManagerConfirmAction("Repair two-way sync","Rebuild this exact calendar’s writeback registration and verify a targeted provider sync before edits are available.","Tap again: repair two-way sync",async()=>{
      await api("/api/calendars/private/editable","POST",{source:item.source,editable:true});
      await ctrlCalendarManagerRefresh(`${item.name||"Private calendar"} two-way sync setup repaired.`,item.source);
    }));
  }else if(writeback&&writeback.enabled!==true){
    actions.appendChild(ctrlCalendarManagerAction("Turn on Dashboard edits","This calendar is set to two-way sync. Turn on the master Dashboard edit setting now.","primary",async()=>{
      await ctrlCalendarWritebackSave(writeback,{enabled:true},item.source);
    }));
  }else{
    const lockDescription=item.provider==="google"
      ?"Stop queued Dash-Go writes now, save an owner-only local snapshot, and keep this secure Google connection view-only. To replace it with a Google calendar link later, add and verify the link with installer option 9 before stopping this connection."
      :"Stop queued Dash-Go writes now, save an owner-only local snapshot, and keep this provider calendar visible. Future provider changes still appear here.";
    actions.appendChild(ctrlCalendarManagerConfirmAction("Switch to view-only",lockDescription,"Tap again: switch to view-only",async()=>{
      await api("/api/calendars/private/editable","POST",{source:item.source,editable:false});
      await ctrlCalendarManagerRefresh(`${item.name||"Private calendar"} is now view-only.`,item.source);
    }));
  }
  if(!conflict&&!repair){
    actions.appendChild(ctrlCalendarManagerAction("Sync now","Synchronize only this selected private calendar.","",async()=>{
      await api("/api/calendars/private/sync","POST",{source:item.source});
      await ctrlCalendarManagerRefresh(`${item.name||"Private calendar"} sync queued.`,item.source);
    }));
  }
  actions.appendChild(ctrlCalendarManagerConfirmAction("Stop syncing","Stop future sync for this Dash-Go source. The remote calendar is not changed and the local mirror is preserved.","Tap again to stop sync",async()=>{
    await api("/api/calendars/private/deactivate","POST",{source:item.source});
    await ctrlCalendarManagerRefresh(`${item.name||"Private calendar"} stopped syncing. Remote calendar unchanged.`,item.source,true);
  }));
  row.appendChild(actions);return row;
}
async function ctrlPrivateCalendarActivate(item,editable){
  await api("/api/calendars/private/activate","POST",{pair:item.pair,remoteId:item.remoteId,editable});
  await ctrlCalendarManagerRefresh(`${item.name||"Private calendar"} added${editable?" with two-way sync enabled":" as view-only"}.`,"",true);
}
