// Calendar writeback controls live only inside the lazily opened Calendar
// Manager. Per-calendar edit state belongs to the selected-calendar row above;
// this card owns only the master safety switch and PIN policy.
function ctrlCalendarWritebackSettings(status){
  status=status||{enabled:false,requirePin:false,calendars:[]};
  const card=el("section","calendar-manager-group calwriteback-settings");
  card.append(el("div","calmanager-heading","Dashboard calendar edits"));
  const rows=Array.isArray(status.calendars)?status.calendars:[];
  card.appendChild(el("p","calmanager-note",rows.length?"The master switch applies only to exact selected private calendars. Use each selected calendar row above to make an individual calendar editable or display-only.":"Discover and select a private Google, iCloud, or CalDAV calendar first. Website subscriptions and generated feeds are always read-only."));
  if(status.last&&status.last.detail)card.appendChild(el("p","calmanager-note",String(status.last.detail)));
  if(!rows.length)return card;
  const toggles=el("div","calmanager-actions");
  toggles.appendChild(ctrlCalendarManagerAction(status.enabled?"Disable Dashboard edits":"Enable Dashboard edits",status.enabled?"Keep private calendars visible but prevent add, edit, skip, and delete actions.":"Allow add, edit, and skip for the selected calendars that are individually enabled.",status.enabled?"":"primary",async()=>{
    await ctrlCalendarWritebackSave(status,{enabled:!status.enabled});
  }));
  toggles.appendChild(ctrlCalendarManagerAction(status.requirePin?"PIN required for edits":"PIN not required for edits",status.requirePin?"Calendar changes require Dashboard Control PIN authorization.":"Create, edit, and skip are available without a PIN; delete always remains PIN-gated.","",async()=>{
    await ctrlCalendarWritebackSave(status,{requirePin:!status.requirePin});
  }));
  card.appendChild(toggles);return card;
}
async function ctrlCalendarWritebackSave(status,patch,source){
  const payload={enabled:status.enabled===true,requirePin:status.requirePin===true,calendars:(status.calendars||[]).map(item=>({source:item.source,enabled:item.enabled!==false}))};
  Object.assign(payload,patch||{});
  await api("/api/calendar/writeback/config","POST",payload);
  await ctrlCalendarManagerRefresh("Calendar edit settings saved.",source||"");
}
