function ctrlCalendarVisibilityRoot(){
  return document.querySelector("#ctrlpage-calendars #ctrlcals");
}
async function renderCtrlCals(){
  const row=ctrlCalendarVisibilityRoot();
  if(!row) return;
  try{
    await cachedApi("/api/calendars",cals=>renderCtrlCalsData(row,cals));
  }catch(e){ ctrlSetError(row,"Calendar controls unavailable",friendlyUnavailable("Calendar controls",e)); }
}
function ctrlCalendarChipColor(raw){
  const named={
    red:"#e35d4f",orange:"#df8a1f",yellow:"#ddb13d",gold:"#ddb13d",
    green:"#76b82a",teal:"#30b59f",cyan:"#3fb7d6",blue:"#4aa3f3",
    personal:"#4aa3f3",work:"#76b82a",family:"#30b59f",
    purple:"#9b7aff",violet:"#9b7aff",pink:"#d75f8f",holiday:"#d75f8f",holidays:"#d75f8f",
    grey:"#8d969d",gray:"#8d969d",trash:"#7f7774",dst:"#8d969d",moon:"#d99520",seasons:"#2fb596"
  };
  const s=String(raw||"").trim();
  if(/^#([0-9a-f]{3}|[0-9a-f]{6})$/i.test(s)) return s;
  const rgb=s.match(/^rgba?\((\d+)\s*,\s*(\d+)\s*,\s*(\d+)/i);
  if(rgb) return `rgb(${rgb[1]},${rgb[2]},${rgb[3]})`;
  return named[s.toLowerCase()] || "#7fd6a8";
}
function ctrlCalendarChipRgb(color){
  const c=String(color||"").trim();
  if(/^#([0-9a-f]{3})$/i.test(c)){
    const m=c.slice(1).split("").map(x=>parseInt(x+x,16));
    return m.join(",");
  }
  if(/^#([0-9a-f]{6})$/i.test(c)){
    const n=parseInt(c.slice(1),16);
    return `${(n>>16)&255},${(n>>8)&255},${n&255}`;
  }
  const rgb=c.match(/^rgba?\((\d+)\s*,\s*(\d+)\s*,\s*(\d+)/i);
  if(rgb) return `${rgb[1]},${rgb[2]},${rgb[3]}`;
  return "127,214,168";
}
function ctrlCalendarEnabled(c){ return !c || c.enabled!==false; }
function ctrlCalendarChip(c,onToggle){
  const color=ctrlCalendarChipColor(c.color||c.name);
  const enabled=ctrlCalendarEnabled(c);
  const b=el("button","calchip "+(enabled?"on":"off"));
  b.type="button";
  b.setAttribute("aria-pressed",enabled?"true":"false");
  b.style.setProperty("--cal-color",color);
  b.style.setProperty("--cal-rgb",ctrlCalendarChipRgb(color));
  b.innerHTML=`<span class="calchip-dot" aria-hidden="true"></span><span class="calchip-label">${escapeHTML(c.name||"Calendar")}</span><span class="calchip-check" aria-hidden="true">${enabled?"✓":""}</span>`;
  bindTap(b,onToggle);
  return b;
}
async function ctrlCalendarRefresh(message){
  delete CTRL_CACHE["/api/calendars"];
  delete CTRL_CACHE["/api/cache/status"];
  await discoverCalendars();
  await loadCalendars();
  await renderCtrlCals();
  const cacheSection=document.querySelector('#ctrlpage-calendars details.ctrlsec[data-lazy="cache"]');
  if(cacheSection&&cacheSection.open)await renderCtrlCache();
  const healthSection=document.querySelector('#ctrlpage-calendars details.ctrlsec[data-lazy="calhealth"]');
  if(healthSection&&healthSection.open)await renderCtrlCalendarHealthPanel();
  if(message)ctrlMsg(message);
}
function ctrlCalendarManagerState(row,open){
  if(!row)return;
  if(open)row.dataset.calendarManagerOpen="1";
  else delete row.dataset.calendarManagerOpen;
}
function ctrlCalendarManagerStatusLabel(item){
  if(item.kind==="app"&&item.outputEnabled===false)return "Calendar output off";
  return item.enabled===false?"Hidden":"Shown";
}
function ctrlCalendarManagerDetail(item){
  const source=String(item.sourceLabel||"Local calendar file");
  const bits=[source];
  if(item.kind==="symlink")bits.push("external target preserved");
  if(item.enabled===false&&item.outputEnabled!==false)bits.push("hidden from dashboard");
  return bits.join(" · ");
}
async function ctrlCalendarManagerPost(path,payload,success){
  try{
    const result=await api(path,"POST",payload||{});
    await ctrlCalendarRefresh(success);
    return result;
  }catch(error){ctrlMsg(error.message||String(error));throw error;}
}
function ctrlCalendarManagerRow(item){
  const card=el("article","calmanager-row calmanager-"+String(item.kind||"unknown"));
  const color=ctrlCalendarChipColor(item.color||item.name);
  const head=el("div","calmanager-head");
  const title=el("div","calmanager-title");
  const dot=el("span","calmanager-dot");dot.style.background=color;
  title.append(dot,el("strong","",item.name||"Calendar"));
  const status=el("span","calmanager-state "+(item.enabled!==false&&item.outputEnabled!==false?"on":"off"),ctrlCalendarManagerStatusLabel(item));
  head.append(title,status);
  card.append(head,el("div","calmanager-detail",ctrlCalendarManagerDetail(item)));
  const actions=el("div","calmanager-actions");
  if(item.kind==="app"){
    if(item.outputEnabled===false){
      actions.appendChild(caction("Enable calendar output","Rebuild this app’s local calendar from existing data.","primary",async()=>{
        await ctrlCalendarManagerPost("/api/calendars/manage/app-output",{owner:item.owner,enabled:true},`${item.name} calendar output enabled.`);
      }));
    }else{
      actions.appendChild(confirmAction("Stop calendar output","Keep app data; remove the generated feed until you enable it again.","Tap again to stop output",async()=>{
        await ctrlCalendarManagerPost("/api/calendars/manage/app-output",{owner:item.owner,enabled:false},`${item.name} calendar output stopped. App data remains local.`);
      }));
      actions.lastChild.classList.add("calmanager-stop");
      actions.appendChild(caction(item.enabled===false?"Show calendar":"Hide calendar",item.enabled===false?"Show generated events on the dashboard.":"Keep output but hide its events on the dashboard.","",async()=>{
        const result=await api("/api/calendars/toggle","POST",{name:item.name,url:item.url});
        await ctrlCalendarRefresh(`${result.name}${result.enabled?" shown":" hidden"}.`);
      }));
    }
  }else if(item.kind==="local"||item.kind==="symlink"){
    actions.appendChild(caction(item.enabled===false?"Show calendar":"Hide calendar",item.enabled===false?"Show this local source on the dashboard.":"Hide this source without deleting it.","",async()=>{
      const result=await api("/api/calendars/toggle","POST",{name:item.name,url:item.url});
      await ctrlCalendarRefresh(`${result.name}${result.enabled?" shown":" hidden"}.`);
    }));
    const isLink=item.kind==="symlink";
    const label=isLink?"Remove calendar link":"Delete local calendar";
    const description=isLink?"Only the Dash-Go symlink is removed. Its external target stays untouched and can be restored for 30 days.":"Move this .ics file to Calendar Trash. Restore is available for 30 days.";
    actions.appendChild(confirmAction(label,description,isLink?"Tap again to remove link":"Tap again to move to trash",async()=>{
      const result=await ctrlCalendarManagerPost("/api/calendars/manage/delete",{url:item.url,name:item.name},`${item.name} moved to Calendar Trash for 30 days.`);
      return result;
    }));
  }else{
    actions.appendChild(caction(item.enabled===false?"Show calendar":"Hide calendar","Unknown source types are visibility-only.","",async()=>{
      const result=await api("/api/calendars/toggle","POST",{name:item.name,url:item.url});
      await ctrlCalendarRefresh(`${result.name}${result.enabled?" shown":" hidden"}.`);
    }));
  }
  card.appendChild(actions);
  return card;
}
function ctrlCalendarTrashRow(item){
  const row=el("article","caltrash-row");
  const copy=el("div","caltrash-copy");
  copy.append(el("strong","",item.name||"Deleted calendar"),el("span","",`${item.isSymlink?"Calendar link":"Local calendar"} · restores until ${String(item.purgeAfter||"").slice(0,10)}`));
  row.append(copy,cbtn("Restore","",async()=>{
    await ctrlCalendarManagerPost("/api/calendars/manage/restore",{id:item.id},`${item.name} restored.`);
  }));
  return row;
}
function renderCtrlCalendarManagerData(wrap,manager){
  wrap.innerHTML="";
  const rows=Array.isArray(manager&&manager.calendars)?manager.calendars:[];
  wrap.appendChild(el("div","calmanager-heading","Manage calendars"));
  wrap.appendChild(el("p","calmanager-note","Hide calendars with the colored chips above. Use these ownership-aware actions to remove local sources safely, control generated app output, or restore a deleted local calendar."));
  const list=el("div","calmanager-list");
  if(!rows.length)list.appendChild(ctrlStateCard("empty","No managed calendars","Add a local .ics calendar or open an app that creates a local calendar feed."));
  else rows.forEach(item=>list.appendChild(ctrlCalendarManagerRow(item)));
  wrap.appendChild(list);
  const trash=Array.isArray(manager&&manager.trash)?manager.trash:[];
  if(trash.length){
    const trashCard=el("section","caltrash");
    trashCard.append(el("div","calmanager-heading","Recently deleted calendars"),el("p","calmanager-note",`Calendar Trash retains local files and links for ${Number(manager.retentionDays)||30} days. Active calendars are never auto-deleted.`));
    trash.forEach(item=>trashCard.appendChild(ctrlCalendarTrashRow(item)));
    wrap.appendChild(trashCard);
  }
}
async function renderCtrlCalendarManager(wrap){
  ctrlSetLoading(wrap,"Loading Calendar Manager…","Reading calendar ownership, local sources, and recently deleted calendars.");
  try{renderCtrlCalendarManagerData(wrap,await api("/api/calendars/manage"));}
  catch(error){ctrlSetError(wrap,"Calendar Manager unavailable",error,[cbtn("Try again","",()=>renderCtrlCalendarManager(wrap))]);}
}
function renderCtrlCalsData(row,cals){
  row.innerHTML="";
  if(!cals.length){
    // Keep Calendar Manager reachable even when every source is hidden, archived,
    // or app output is off; it is the recovery path that can restore them.
    row.appendChild(ctrlStateCard("empty","No active calendars","Manage calendars to restore a deleted local source or enable an app calendar feed.",[cbtn("Refresh calendars","",async()=>{ await discoverCalendars(); await renderCtrlCals(); })]));
  }else{
    const group=el("div","calchipgrid");
    for(const c of cals){
      const b=ctrlCalendarChip(c,async()=>{
        try{
          const result=await api("/api/calendars/toggle","POST",{name:c.name,url:c.url});
          await ctrlCalendarRefresh(result.name+(result.enabled?" enabled":" hidden")+" — calendar updated.");
        }catch(error){ctrlMsg(error.message||String(error));}
      });
      group.appendChild(b);
    }
    row.appendChild(group);
  }
  const actions=el("div","ctrlrow compact calmanager-top-actions");
  const open=row.dataset.calendarManagerOpen==="1";
  actions.appendChild(caction(open?"Close Calendar Manager":"Manage calendars",open?"Return to fast visibility controls.":"Delete local calendars safely, control app output, or restore deleted files.","",async()=>{
    ctrlCalendarManagerState(row,!open);await renderCtrlCals();
  }));
  actions.appendChild(caction("Repair calendar index","Regenerate the manifest, remove stale registrations, and rebuild the event cache.","",async()=>{
    try{
      const result=await api("/api/calendars/manage/repair","POST",{});
      await ctrlCalendarRefresh(`Calendar index repaired: ${result.after||0} active source${Number(result.after)===1?"":"s"}.`);
    }catch(error){ctrlMsg(error.message||String(error));}
  }));
  row.appendChild(actions);
  if(open){
    const manager=el("section","calmanager");
    row.appendChild(manager);
    renderCtrlCalendarManager(manager);
  }
}
