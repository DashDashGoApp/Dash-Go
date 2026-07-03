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
function ctrlPrivateCalendarState(item,writeback){
  const bits=[ctrlPrivateCalendarProvider(item)];
  const registered=ctrlPrivateCalendarRegistry(item,writeback);
  if(item.writable!==true)bits.push("display-only");
  else if(!registered)bits.push("edit setup needs attention");
  else if(writeback&&writeback.enabled!==true)bits.push("Dashboard edits off");
  else if(registered.enabled===false)bits.push("edits disabled");
  else bits.push("editable");
  if(item.state)bits.push(String(item.state));return bits.join(" · ");
}
function ctrlPrivateCalendarCandidateRow(item){
  const row=el("article","calmanager-row calmanager-private");
  const head=el("div","calmanager-head"),title=el("div","calmanager-title"),dot=el("span","calmanager-dot");dot.style.background=ctrlCalendarChipColor(item.color||item.name);title.append(dot,el("strong","",item.name||"Private calendar"));head.append(title,el("span","calmanager-state","Available"));
  row.append(head,el("div","calmanager-detail",`${ctrlPrivateCalendarProvider(item)} · not syncing yet`));
  const actions=el("div","calmanager-actions");
  actions.appendChild(ctrlCalendarManagerAction("Add display-only","Show this calendar on Dash-Go without allowing event changes from the dashboard.","",async()=>ctrlPrivateCalendarActivate(item,false)));
  actions.appendChild(ctrlCalendarManagerAction("Add & enable edits","Create one exact two-way calendar source and turn on Dashboard edits. Supported normal events can sync through the provider.","primary",async()=>ctrlPrivateCalendarActivate(item,true)));
  row.appendChild(actions);return row;
}
function ctrlPrivateCalendarSelectedRow(item,writeback){
  const row=el("article","calmanager-row calmanager-private");row.dataset.calendarSource=String(item.source||"");
  const head=el("div","calmanager-head"),title=el("div","calmanager-title"),dot=el("span","calmanager-dot");dot.style.background=ctrlCalendarChipColor(item.color||item.name);title.append(dot,el("strong","",item.name||"Private calendar"));
  const registered=ctrlPrivateCalendarRegistry(item,writeback);
  const live=item.writable===true&&registered&&writeback&&writeback.enabled===true&&registered.enabled!==false;
  const statusText=live?"Editable":item.writable!==true?"Display-only":!registered?"Needs attention":"Edits off";
  head.append(title,el("span","calmanager-state "+(live?"on":"off"),statusText));row.append(head,el("div","calmanager-detail",ctrlPrivateCalendarState(item,writeback)));
  if(item.detail)row.appendChild(el("p","calmanager-note",String(item.detail)));
  const actions=el("div","calmanager-actions");
  if(item.writable!==true){
    actions.appendChild(ctrlCalendarManagerAction("Enable Dashboard edits","Allow supported normal events in this exact selected calendar to be created, edited, and deleted from Dash-Go.","primary",async()=>{
      await api("/api/calendars/private/editable","POST",{source:item.source,editable:true});
      await ctrlCalendarManagerRefresh(`${item.name||"Private calendar"} is editable and Dashboard edits are on.`,item.source);
    }));
  }else if(!registered){
    actions.appendChild(ctrlCalendarManagerAction("Repair edit setup","Rebuild this exact calendar’s local writeback registration without changing the remote calendar.","primary",async()=>{
      await api("/api/calendars/private/editable","POST",{source:item.source,editable:true});
      await ctrlCalendarManagerRefresh(`${item.name||"Private calendar"} edit setup repaired.`,item.source);
    }));
  }else if(writeback&&writeback.enabled!==true){
    actions.appendChild(ctrlCalendarManagerAction("Turn on Dashboard edits","This calendar is selected for editing. Turn on the master Dashboard edit setting now.","primary",async()=>{
      await ctrlCalendarWritebackSave(writeback,{enabled:true},item.source);
    }));
  }else{
    actions.appendChild(ctrlCalendarManagerAction("Disable Dashboard edits","Keep this calendar visible and syncing, but prevent Dash-Go from changing its events.","",async()=>{
      await api("/api/calendars/private/editable","POST",{source:item.source,editable:false});
      await ctrlCalendarManagerRefresh(`${item.name||"Private calendar"} is now display-only.`,item.source);
    }));
  }
  actions.appendChild(ctrlCalendarManagerAction("Sync now","Synchronize only this selected private calendar.","",async()=>{
    await api("/api/calendars/private/sync","POST",{source:item.source});
    await ctrlCalendarManagerRefresh(`${item.name||"Private calendar"} sync queued.`,item.source);
  }));
  actions.appendChild(ctrlCalendarManagerConfirmAction("Stop syncing","Stop future sync for this Dash-Go source. The remote calendar is not changed and the local mirror is preserved.","Tap again to stop sync",async()=>{
    await api("/api/calendars/private/deactivate","POST",{source:item.source});
    await ctrlCalendarManagerRefresh(`${item.name||"Private calendar"} stopped syncing. Remote calendar unchanged.`,item.source,true);
  }));
  row.appendChild(actions);return row;
}
async function ctrlPrivateCalendarActivate(item,editable){
  await api("/api/calendars/private/activate","POST",{pair:item.pair,remoteId:item.remoteId,editable});
  await ctrlCalendarManagerRefresh(`${item.name||"Private calendar"} added${editable?" with Dashboard edits enabled":" as display-only"}.`,"",true);
}
