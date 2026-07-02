// Calendar writeback controls live only inside the lazily opened Calendar
// Manager. They configure pre-registered private vdir collections; no public
// ICS URL can be entered here or made writable.
function ctrlCalendarWritebackSettings(status){
  status=status||{enabled:false,requirePin:false,calendars:[]};
  const card=el("section","calmanager calwriteback-settings");
  card.append(el("div","calmanager-heading","Dashboard calendar edits"));
  const rows=Array.isArray(status.calendars)?status.calendars:[];
  card.appendChild(el("p","calmanager-note",rows.length
    ?"Only explicitly registered private CalDAV collections can be edited here. Website subscriptions and generated feeds stay read-only."
    :"Set up a private CalDAV/iCloud collection first. Website subscriptions and generated feeds are always read-only."));
  if(status.last&&status.last.detail)card.appendChild(el("p","calmanager-note",String(status.last.detail)));
  if(!rows.length)return card;
  const toggles=el("div","calmanager-actions");
  toggles.appendChild(caction(status.enabled?"Disable Dashboard edits":"Enable Dashboard edits",status.enabled?"Keep private calendars visible but prevent add, edit, skip, and delete actions.":"Allow add, edit, and skip only for enabled private collections.",status.enabled?"":"primary",async()=>{
    await ctrlCalendarWritebackSave(status,{enabled:!status.enabled});
  }));
  toggles.appendChild(caction(status.requirePin?"PIN required for edits":"PIN not required for edits",status.requirePin?"Calendar changes require Dashboard Control PIN authorization.":"Create, edit, and skip are available without a PIN; delete always remains PIN-gated.","",async()=>{
    await ctrlCalendarWritebackSave(status,{requirePin:!status.requirePin});
  }));
  card.appendChild(toggles);
  const list=el("div","calmanager-list");
  for(const cal of rows){
    const row=el("article","calmanager-row calmanager-writeback");
    row.append(el("div","calmanager-title",cal.name||cal.source),el("div","calmanager-detail","Private CalDAV collection · remote calendar is never deleted from Dashboard Control"));
    const actions=el("div","calmanager-actions");
    actions.appendChild(caction(cal.enabled===false?"Enable this calendar":"Disable this calendar",cal.enabled===false?"Allow it when Dashboard edits are enabled.":"Keep it visible but make it read-only in Dash-Go.",cal.enabled===false?"primary":"",async()=>{
      const calendars=rows.map(item=>({source:item.source,enabled:item.source===cal.source?!item.enabled:item.enabled!==false}));
      await ctrlCalendarWritebackSave(status,{calendars});
    }));
    row.appendChild(actions);list.appendChild(row);
  }
  card.appendChild(list);return card;
}
async function ctrlCalendarWritebackSave(status,patch){
  const payload={enabled:status.enabled===true,requirePin:status.requirePin===true,calendars:(status.calendars||[]).map(item=>({source:item.source,enabled:item.enabled!==false}))};
  Object.assign(payload,patch||{});
  try{
    await api("/api/calendar/writeback/config","POST",payload);
    ctrlMsg("Calendar edit settings saved.");
    const section=document.querySelector('#ctrlpage-calendars details.ctrlsec[data-lazy="calendars"]');
    if(section&&section.open){const body=section.querySelector(".ctrlbody");if(body)renderCtrlCalendarManager(body);}
  }catch(error){ctrlMsg(error.message||String(error));}
}
