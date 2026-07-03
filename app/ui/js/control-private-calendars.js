// Private-calendar discovery is deliberately a separate, lazy Calendar Manager
// action. Results are inventory only until the user explicitly adds one source.
function ctrlPrivateCalendarSettings(state){
  state=state||{available:false,discovering:false,candidates:[],selected:[],notices:[]};
  const card=el("section","calmanager calprivate-settings");
  card.append(el("div","calmanager-heading","Private calendar connections"));
  card.append(el("p","calmanager-note","Discover checks connected Google, iCloud, and CalDAV accounts for calendars. Nothing discovered here syncs or changes until you add it."));
  const actions=el("div","calmanager-actions");
  if(state.available){
    actions.appendChild(caction(state.discovering?"Discovering calendars…":"Discover available calendars",state.discovering?"Reading connected accounts without changing active calendars.":"Refresh the review-only inventory. Existing selections stay unchanged.","primary",async()=>{
      try{
        await api("/api/calendars/private/discover","POST",{});
        await ctrlCalendarRefresh("Private calendar discovery finished. Nothing was activated automatically.");
      }catch(error){ctrlMsg(error.message||String(error));}
    }));
  }else actions.appendChild(el("span","calmanager-note","Set up a private Google, iCloud, or CalDAV connection first."));
  card.appendChild(actions);
  const notices=Array.isArray(state.notices)?state.notices:[];
  notices.forEach(note=>card.appendChild(el("p","calmanager-note",String(note))));
  if(state.legacyBroadMirror===true&&Array.isArray(state.selected)&&state.selected.length){
    card.appendChild(el("p","calmanager-note","A legacy all-calendars mirror is still preserved. It may overlap these exact selected calendars; review it in Calendar Manager and hide it only after you confirm the selected sources look correct."));
  }
  const selected=Array.isArray(state.selected)?state.selected:[];
  if(selected.length){
    card.appendChild(el("div","calmanager-heading","Selected calendars"));
    const list=el("div","calmanager-list");
    selected.forEach(item=>list.appendChild(ctrlPrivateCalendarSelectedRow(item)));
    card.appendChild(list);
  }
  const candidates=Array.isArray(state.candidates)?state.candidates:[];
  const unselected=candidates.filter(item=>item&&item.selected!==true);
  if(unselected.length){
    card.appendChild(el("div","calmanager-heading","Available to add"));
    const list=el("div","calmanager-list");
    unselected.forEach(item=>list.appendChild(ctrlPrivateCalendarCandidateRow(item)));
    card.appendChild(list);
  }else if(!selected.length&&!state.discovering){
    card.appendChild(ctrlStateCard("empty","No discovered private calendars","Use Discover available calendars to review connected account collections. Discovery never enables sync by itself."));
  }
  return card;
}
function ctrlPrivateCalendarProvider(item){return String(item.providerLabel||((item.provider==="google")?"Google":"iCloud / CalDAV"));}
function ctrlPrivateCalendarState(item){
  const bits=[ctrlPrivateCalendarProvider(item)];
  if(item.writable===true)bits.push("editable");else bits.push("display-only");
  if(item.state)bits.push(String(item.state));
  return bits.join(" · ");
}
function ctrlPrivateCalendarCandidateRow(item){
  const row=el("article","calmanager-row calmanager-private");
  const head=el("div","calmanager-head");
  const title=el("div","calmanager-title");
  const dot=el("span","calmanager-dot");dot.style.background=ctrlCalendarChipColor(item.color||item.name);
  title.append(dot,el("strong","",item.name||"Private calendar"));
  head.append(title,el("span","calmanager-state","Available"));
  row.append(head,el("div","calmanager-detail",`${ctrlPrivateCalendarProvider(item)} · not syncing yet`));
  const actions=el("div","calmanager-actions");
  actions.appendChild(caction("Add display-only","Show this calendar on Dash-Go without allowing event changes from the dashboard.","",async()=>{
    await ctrlPrivateCalendarActivate(item,false);
  }));
  actions.appendChild(caction("Add & enable edits","Create one exact two-way calendar source. Normal event adds, edits, and deletes can sync through the provider.","primary",async()=>{
    await ctrlPrivateCalendarActivate(item,true);
  }));
  row.appendChild(actions);return row;
}
function ctrlPrivateCalendarSelectedRow(item){
  const row=el("article","calmanager-row calmanager-private");
  const head=el("div","calmanager-head");
  const title=el("div","calmanager-title");
  const dot=el("span","calmanager-dot");dot.style.background=ctrlCalendarChipColor(item.color||item.name);
  title.append(dot,el("strong","",item.name||"Private calendar"));
  const statusText=item.state?String(item.state):(item.writable===true?"Editable":"Display-only");
  head.append(title,el("span","calmanager-state",statusText));
  row.append(head,el("div","calmanager-detail",ctrlPrivateCalendarState(item)));
  if(item.detail)row.appendChild(el("p","calmanager-note",String(item.detail)));
  const actions=el("div","calmanager-actions");
  actions.appendChild(caction(item.writable===true?"Disable Dashboard edits":"Enable Dashboard edits",item.writable===true?"Keep this calendar visible and syncing, but prevent Dash-Go from changing its events.":"Allow supported normal events in this exact selected calendar to be created, edited, and deleted from Dash-Go when Dashboard edits are enabled.",item.writable===true?"":"primary",async()=>{
    try{await api("/api/calendars/private/editable","POST",{source:item.source,editable:item.writable!==true});await ctrlCalendarRefresh(`${item.name||"Private calendar"} is now ${item.writable===true?"display-only":"editable"}.`);}catch(error){ctrlMsg(error.message||String(error));}
  }));
  actions.appendChild(caction("Sync now","Synchronize only this selected private calendar.","",async()=>{
    try{await api("/api/calendars/private/sync","POST",{source:item.source});await ctrlCalendarRefresh(`${item.name||"Private calendar"} sync queued.`);}catch(error){ctrlMsg(error.message||String(error));}
  }));
  actions.appendChild(confirmAction("Stop syncing","Stop future sync for this Dash-Go source. The remote calendar is not changed and the local mirror is preserved.","Tap again to stop sync",async()=>{
    try{await api("/api/calendars/private/deactivate","POST",{source:item.source});await ctrlCalendarRefresh(`${item.name||"Private calendar"} stopped syncing. Remote calendar unchanged.`);}catch(error){ctrlMsg(error.message||String(error));throw error;}
  }));
  row.appendChild(actions);return row;
}
async function ctrlPrivateCalendarActivate(item,editable){
  try{
    await api("/api/calendars/private/activate","POST",{pair:item.pair,remoteId:item.remoteId,editable});
    await ctrlCalendarRefresh(`${item.name||"Private calendar"} added${editable?" with Dashboard edits enabled":" as display-only"}.`);
  }catch(error){ctrlMsg(error.message||String(error));}
}
