function ctrlCalendarVisibilityRoot(){
  return document.querySelector("#ctrlpage-calendars #ctrlcals");
}
function ctrlCalendarManagerScrollRoot(){return document.querySelector("#ctrlpage-calendars");}
function ctrlCalendarManagerSourceRow(root,source){
  const key=String(source||"");
  if(!root||!key)return null;
  return Array.from(root.querySelectorAll("[data-calendar-source]")).find(node=>node.dataset.calendarSource===key)||null;
}
function ctrlCalendarManagerAnchor(source){
  const root=ctrlCalendarManagerScrollRoot();
  if(!root)return null;
  const active=document.activeElement&&document.activeElement.closest?document.activeElement.closest("[data-calendar-source]"):null;
  const key=String(source||active&&active.dataset.calendarSource||"");
  const target=ctrlCalendarManagerSourceRow(root,key);
  return {root,scrollTop:root.scrollTop,key,offset:target?target.getBoundingClientRect().top-root.getBoundingClientRect().top:null};
}
function ctrlCalendarRestoreManagerAnchor(anchor){
  if(!anchor||!anchor.root||!anchor.root.isConnected)return;
  const root=anchor.root;
  if(anchor.key){
    const target=ctrlCalendarManagerSourceRow(root,anchor.key);
    if(target&&anchor.offset!==null){
      root.scrollTop+=target.getBoundingClientRect().top-root.getBoundingClientRect().top-anchor.offset;
      return;
    }
  }
  root.scrollTop=anchor.scrollTop;
}
function ctrlCalendarManagerAction(label,desc,cls,fn){
  const b=el("button","cbtn actionbtn"+(cls?" "+cls:""));b.type="button";
  b.innerHTML=`<span class="bt">${escapeHTML(label)}</span>${desc?`<span class="bd">${escapeHTML(desc)}</span>`:""}`;
  bindTap(b,async()=>{ctrlSetActionFeedbackTarget(b);if(typeof fn==="function")return await fn();},{scrollRoot:ctrlCalendarManagerScrollRoot});
  return b;
}
function ctrlCalendarManagerConfirmAction(label,desc,armedLabel,fn){
  const b=ctrlCalendarManagerAction(label,desc,"danger requiresconfirm",async()=>{
    const normal=b.dataset.normalHtml||b.innerHTML;
    b.dataset.normalHtml=normal;
    const confirmed=dashConfirmTap(b,{duration:5000,
      renderArmed:()=>{b.innerHTML=`<span class="bt">${escapeHTML(armedLabel)}</span><span class="bd">Tap once more to confirm.</span>`;},
      renderNormal:()=>{b.innerHTML=b.dataset.normalHtml||normal;}
    });
    if(!confirmed)return;
    return await fn(b);
  });
  return b;
}
async function ctrlCalendarManagerRefresh(message,source,syncDashboard){
  const anchor=ctrlCalendarManagerAnchor(source);
  try{
    if(syncDashboard){
      await ctrlCalendarRefresh(message,anchor);
      return;
    }
    const manager=ctrlCalendarVisibilityRoot()?.querySelector(".calendar-manager-shell");
    if(manager){
      ctrlBeginWarmRefresh(manager,"Refreshing calendar settings…");
      renderCtrlCalendarManagerData(manager,await api("/api/calendars/manage"));
      ctrlCalendarRestoreManagerAnchor(anchor);
    }
    if(message)ctrlMsg(message);
  }catch(error){ctrlMsg(error.message||String(error));throw error;}
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
async function ctrlCalendarRefresh(message,anchor){
  const savedAnchor=anchor||ctrlCalendarManagerAnchor();
  delete CTRL_CACHE["/api/calendars"];
  delete CTRL_CACHE["/api/cache/status"];
  await discoverCalendars();
  await loadCalendars();
  await renderCtrlCals();
  const cacheSection=document.querySelector('#ctrlpage-calendars details.ctrlsec[data-lazy="cache"]');
  if(cacheSection&&cacheSection.open)await renderCtrlCache();
  const healthSection=document.querySelector('#ctrlpage-calendars details.ctrlsec[data-lazy="calhealth"]');
  if(healthSection&&healthSection.open)await renderCtrlCalendarHealthPanel();
  ctrlCalendarRestoreManagerAnchor(savedAnchor);
  if(message)ctrlMsg(message);
}
function ctrlCalendarManagerStatusLabel(item){
  if(item.kind==="app"&&item.outputEnabled===false)return "Output off";
  return item.enabled===false?"Hidden":"Shown";
}
function ctrlCalendarManagerSourceLabel(item){
  if(item.privateSelected===true){
    return item.provider==="google"?"Google":"iCloud / CalDAV";
  }
  if(item.kind==="symlink")return "Calendar link";
  if(item.kind==="app")return "Dash-Go app";
  return String(item.sourceLabel||"Local calendar").replace(/ · .*/,"");
}
function ctrlCalendarManagerAccessLabel(item){
  if(item.privateSelected===true)return item.privateWritable===true?"Two-way":"View-only";
  return "Local";
}
function ctrlCalendarManagerHealthLabel(item){
  const state=String(item.privateSync||item.privateState||"");
  if(state==="conflict")return "Conflict";
  if(state==="attention-auth"||state==="attention-undiscovered"||state==="attention-empty"||state==="failed")return "Needs attention";
  return "Healthy";
}
function ctrlCalendarManagerDetail(item){
  const bits=[ctrlCalendarManagerSourceLabel(item),ctrlCalendarManagerAccessLabel(item)];
  if(item.kind==="symlink")bits.push("external target preserved");
  if(item.enabled===false&&item.outputEnabled!==false)bits.push("hidden from dashboard");
  return bits.join(" · ");
}
function ctrlCalendarManagerBadges(item){
  const badges=el("div","calmanager-badges");
  const add=(label,cls)=>badges.appendChild(el("span","calmanager-badge "+cls,label));
  add(ctrlCalendarManagerStatusLabel(item),item.enabled!==false&&item.outputEnabled!==false?"shown":"hidden");
  add(ctrlCalendarManagerSourceLabel(item),"source");
  add(ctrlCalendarManagerAccessLabel(item),item.privateWritable===true?"twoway":"readonly");
  add(ctrlCalendarManagerHealthLabel(item),ctrlCalendarManagerHealthLabel(item)==="Healthy"?"healthy":"attention");
  return badges;
}
function ctrlCalendarManagerEnrichRows(manager){
  const selected=Array.isArray(manager&&manager.privateCalendars&&manager.privateCalendars.selected)?manager.privateCalendars.selected:[];
  const bySource=new Map(selected.map(row=>[String(row&&row.source||""),row]));
  return (Array.isArray(manager&&manager.calendars)?manager.calendars:[]).map(row=>{
    const copy=Object.assign({},row),privateRow=bySource.get(String(row&&row.url||""));
    if(privateRow){
      copy.privateSelected=true;
      copy.privateWritable=privateRow.writable===true;
      copy.privateSync=privateRow.sync;
      copy.privateState=privateRow.state;
      copy.provider=privateRow.provider;
      copy.sourceLabel=privateRow.providerLabel||copy.sourceLabel;
    }
    return copy;
  });
}
function ctrlCalendarVisibilityBySource(rows){
  const out=new Map();
  for(const row of Array.isArray(rows)?rows:[])out.set(String(row&&row.url||""),row&&row.enabled!==false);
  return out;
}
function ctrlCalendarManagerSummary(manager){
  const rows=ctrlCalendarManagerEnrichRows(manager);
  const shown=rows.filter(item=>item.enabled!==false&&item.outputEnabled!==false).length;
  const twoWay=rows.filter(item=>item.privateSelected===true&&item.privateWritable===true).length;
  const attention=rows.filter(item=>ctrlCalendarManagerHealthLabel(item)!=="Healthy").length;
  const bits=[`${rows.length} calendar${rows.length===1?"":"s"}`,`${shown} shown on dashboard`,`${twoWay} two-way synced`];
  if(attention)bits.push(`${attention} needs attention`);
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
  if(item&&item.url)card.dataset.calendarSource=String(item.url);
  const color=ctrlCalendarChipColor(item.color||item.name);
  const head=el("div","calmanager-head");
  const title=el("div","calmanager-title");
  const dot=el("span","calmanager-dot");dot.style.background=color;
  title.append(dot,el("strong","",item.name||"Calendar"));
  head.append(title);
  card.append(head,ctrlCalendarManagerBadges(item),el("div","calmanager-detail",ctrlCalendarManagerDetail(item)));
  const actions=el("div","calmanager-actions");
  if(item.kind==="app"){
    if(item.outputEnabled===false){
      actions.appendChild(ctrlCalendarManagerAction("Enable calendar output","Rebuild this app’s local calendar from existing data.","primary",async()=>{
        await ctrlCalendarManagerPost("/api/calendars/manage/app-output",{owner:item.owner,enabled:true},`${item.name} calendar output enabled.`);
      }));
    }else{
      actions.appendChild(ctrlCalendarManagerConfirmAction("Stop calendar output","Keep app data; remove the generated feed until you enable it again.","Tap again to stop output",async()=>{
        await ctrlCalendarManagerPost("/api/calendars/manage/app-output",{owner:item.owner,enabled:false},`${item.name} calendar output stopped. App data remains local.`);
      }));
      actions.lastChild.classList.add("calmanager-stop");
      actions.appendChild(ctrlCalendarManagerAction(item.enabled===false?"Show calendar":"Hide calendar",item.enabled===false?"Show generated events on the dashboard.":"Keep output but hide its events on the dashboard.","",async()=>{
        const result=await api("/api/calendars/toggle","POST",{name:item.name,url:item.url});
        await ctrlCalendarRefresh(`${result.name}${result.enabled?" shown":" hidden"}.`);
      }));
    }
  }else if(item.kind==="writeback"){
    actions.appendChild(ctrlCalendarManagerAction(item.enabled===false?"Show calendar":"Hide calendar",item.enabled===false?"Show this private calendar mirror on the dashboard.":"Hide this mirror without altering the remote calendar.","",async()=>{
      const result=await api("/api/calendars/toggle","POST",{name:item.name,url:item.url});
      await ctrlCalendarRefresh(`${result.name}${result.enabled?" shown":" hidden"}.`);
    }));
  }else if(item.kind==="local"||item.kind==="symlink"){
    actions.appendChild(ctrlCalendarManagerAction(item.enabled===false?"Show calendar":"Hide calendar",item.enabled===false?"Show this local source on the dashboard.":"Hide this source without deleting it.","",async()=>{
      const result=await api("/api/calendars/toggle","POST",{name:item.name,url:item.url});
      await ctrlCalendarRefresh(`${result.name}${result.enabled?" shown":" hidden"}.`);
    }));
    const isLink=item.kind==="symlink";
    const label=isLink?"Remove calendar link":"Delete local calendar";
    const description=isLink?"Only the Dash-Go symlink is removed. Its external target stays untouched and can be restored for 30 days.":"Move this .ics file to Calendar Trash. Restore is available for 30 days.";
    actions.appendChild(ctrlCalendarManagerConfirmAction(label,description,isLink?"Tap again to remove link":"Tap again to move to trash",async()=>{
      const result=await ctrlCalendarManagerPost("/api/calendars/manage/delete",{url:item.url,name:item.name},`${item.name} moved to Calendar Trash for 30 days.`);
      return result;
    }));
  }else{
    actions.appendChild(ctrlCalendarManagerAction(item.enabled===false?"Show calendar":"Hide calendar","Unknown source types are visibility-only.","",async()=>{
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
  const rows=ctrlCalendarManagerEnrichRows(manager);
  wrap.appendChild(el("div","calmanager-heading","Manage calendars"));
  wrap.appendChild(el("p","calmanager-summary",ctrlCalendarManagerSummary(manager)));
  wrap.appendChild(el("p","calmanager-note","Each calendar keeps its own shown/hidden setting, source, health, and access mode. Shown/hidden affects the dashboard only; view-only/two-way controls whether Dash-Go may send changes back."));
  const localRows=rows.filter(item=>item.privateSelected!==true&&item.kind!=="writeback");
  const list=el("div","calmanager-list");
  if(localRows.length)localRows.forEach(item=>list.appendChild(ctrlCalendarManagerRow(item)));
  else list.appendChild(ctrlStateCard("empty","No local calendars","Add a read-only calendar link, connect a personal calendar, or open an app that creates a local calendar feed."));
  wrap.appendChild(list);
  if(typeof ctrlPrivateCalendarSettings==="function")wrap.appendChild(ctrlPrivateCalendarSettings(manager&&manager.privateCalendars,manager&&manager.writeback,ctrlCalendarVisibilityBySource(rows)));
  if(typeof ctrlCalendarWritebackSettings==="function")wrap.appendChild(ctrlCalendarWritebackSettings(manager&&manager.writeback));
  const trash=Array.isArray(manager&&manager.trash)?manager.trash:[];
  if(trash.length){
    const trashCard=el("section","calendar-manager-group caltrash");
    trashCard.append(el("div","calmanager-heading","Recently deleted calendars"),el("p","calmanager-note",`Calendar Trash retains local files and links for ${Number(manager.retentionDays)||30} days. Active calendars are never auto-deleted.`));
    trash.forEach(item=>trashCard.appendChild(ctrlCalendarTrashRow(item)));
    wrap.appendChild(trashCard);
  }
}
async function renderCtrlCalendarManager(wrap){
  ctrlSetLoading(wrap,"Loading Calendar Manager…","Reading calendar ownership, visibility, sync access, and recently deleted calendars.");
  try{renderCtrlCalendarManagerData(wrap,await api("/api/calendars/manage"));}
  catch(error){ctrlSetError(wrap,"Calendar Manager unavailable",error,[cbtn("Try again","",()=>renderCtrlCalendarManager(wrap))]);}
}
function ctrlCalendarVisibilityPopupRows(root,cals){
  root.replaceChildren();
  const list=el("div","calendar-visibility-popup-list");
  if(!cals.length){list.appendChild(ctrlStateCard("empty","No active calendars","Manage calendars to add or restore a calendar source."));root.appendChild(list);return;}
  for(const calendar of cals){
    const row=el("div","calendar-visibility-popup-row");
    row.appendChild(ctrlCalendarChip(calendar,async()=>{
      try{
        const result=await api("/api/calendars/toggle","POST",{name:calendar.name,url:calendar.url});
        await ctrlCalendarRefresh(`${result.name}${result.enabled?" shown":" hidden"}.`);
        const latest=await api("/api/calendars");
        ctrlCalendarVisibilityPopupRows(root,Array.isArray(latest)?latest:[]);
      }catch(error){ctrlMsg(error.message||String(error));}
    }));
    row.appendChild(el("span","calendar-visibility-popup-state",ctrlCalendarEnabled(calendar)?"Shown":"Hidden"));
    list.appendChild(row);
  }
  root.appendChild(list);
}
async function ctrlCalendarSetAllVisibility(cals,enabled,root){
  const targets=(Array.isArray(cals)?cals:[]).filter(calendar=>ctrlCalendarEnabled(calendar)!==enabled);
  for(const calendar of targets)await api("/api/calendars/toggle","POST",{name:calendar.name,url:calendar.url});
  await ctrlCalendarRefresh(enabled?"All calendars shown on the dashboard.":"All calendars hidden from the dashboard.");
  const latest=await api("/api/calendars");
  ctrlCalendarVisibilityPopupRows(root,Array.isArray(latest)?latest:[]);
}
function ctrlOpenCalendarVisibilityShortcut(){
  const prior=document.activeElement;
  popupOpenTransaction({title:"Calendar visibility",when:"Quick show/hide only — access mode and sync settings stay unchanged.",mode:"calendarconfirm",loading:"Loading calendar visibility…",afterCommit:()=>{
    const close=document.querySelector("#popclose");if(close&&close.isConnected)close.focus();
  }},()=>{
    const shell=el("section","calendar-visibility-popup");
    const actions=el("div","calendar-visibility-popup-actions");
    const content=el("div","calendar-visibility-popup-content");
    actions.appendChild(caction("Show all","Show every configured calendar on the dashboard.","",async()=>{
      const cals=await api("/api/calendars");await ctrlCalendarSetAllVisibility(cals,true,content);
    }));
    actions.appendChild(caction("Hide all","Hide every calendar without stopping its sync.","",async()=>{
      const cals=await api("/api/calendars");await ctrlCalendarSetAllVisibility(cals,false,content);
    }));
    shell.append(actions,content);
    api("/api/calendars").then(cals=>ctrlCalendarVisibilityPopupRows(content,Array.isArray(cals)?cals:[])).catch(error=>ctrlSetError(content,"Calendar visibility unavailable",error));
    return shell;
  });
  const scrim=document.querySelector("#scrim");
  if(scrim)scrim.dataset.restoreCalendarVisibilityFocus=prior&&prior.id?prior.id:"";
}
function renderCtrlCalsData(row,manager){
  row.innerHTML="";
  const actions=el("div","ctrlrow compact calmanager-top-actions");
  actions.appendChild(caction("Quick visibility","Show or hide calendars quickly. This does not change read-only or two-way sync.","",async()=>ctrlOpenCalendarVisibilityShortcut()));
  actions.appendChild(caction("Repair calendar index","Regenerate the manifest, remove stale registrations, and rebuild the event cache.","",async()=>{
    try{
      const result=await api("/api/calendars/manage/repair","POST",{});
      await ctrlCalendarRefresh(`Calendar index repaired: ${result.after||0} active source${Number(result.after)===1?"":"s"}.`);
    }catch(error){ctrlMsg(error.message||String(error));}
  }));
  row.appendChild(actions);
  const managerShell=el("section","calendar-manager-shell");
  row.appendChild(managerShell);
  renderCtrlCalendarManagerData(managerShell,manager);
}
async function renderCtrlCals(){
  const row=ctrlCalendarVisibilityRoot();
  if(!row)return;
  ctrlSetLoading(row,"Loading Calendar Manager…","Reading calendar ownership, visibility, sync access, and health.");
  try{renderCtrlCalsData(row,await api("/api/calendars/manage"));}
  catch(error){ctrlSetError(row,"Calendar Manager unavailable",error,[cbtn("Try again","",()=>renderCtrlCals())]);}
}
