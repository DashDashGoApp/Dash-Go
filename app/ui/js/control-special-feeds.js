function applyBirthdayRuntime(items){
  if(!window.DASHBOARD_LOCAL) window.DASHBOARD_LOCAL={};
  window.DASHBOARD_LOCAL.birthdays=Array.isArray(items)?items:[];
  CONFIG.compliments=(CONFIG.compliments||[]).filter(c=>!c._bday);
  for(const b of window.DASHBOARD_LOCAL.birthdays){
    if(b && b.name && b.date) CONFIG.compliments.push({text:`Happy Birthday ${b.name}!`,date:b.date,weight:30,_bday:true});
  }
}
function dateRowMeta(item){ return [item.date||"", item.name||item.label||""].filter(Boolean).join(" · "); }
function specialDateRow(item,kind,handlers){
  const row=el("div","comprow specialdaterow");
  row.appendChild(el("span","ct",kind==="birthday"?(item.name||""):(item.label||"")));
  row.appendChild(el("span","cm",item.date||""));
  row.appendChild(cbtn("Edit","",handlers.edit));
  const del=cbtn("Del","danger",async()=>{
    if(!del.classList.contains("armed")){ del.classList.add("armed"); del.textContent="Sure?"; setTimeout(()=>{del.classList.remove("armed");del.textContent="Del";},3000); return; }
    await handlers.del();
  });
  row.appendChild(del);
  return row;
}
async function renderCtrlBirthdays(){
  const wrap=$("#ctrlbirthdays"); if(!wrap) return;
  ctrlSetLoading(wrap,"Loading birthdays…","Reading installer birthday entries from config.local.js.");
  let data;
  try{ data=await api("/api/birthdays"); }
  catch(e){ ctrlSetError(wrap,"Birthdays unavailable",friendlyUnavailable("Birthdays",e)); return; }
  wrap.innerHTML=""; hideOSK();
  let items=Array.isArray(data.items)?data.items:[];
  wrap.appendChild(ctrlStateCard("info","Birthday messages","Birthdays are recorded by the installer, shown on the calendar, and create high-priority birthday messages on their date."));
  const actions=el("div","ctrlrow compact msgactiongrid singleaction"); actions.appendChild(cbtn("Add birthday","on",()=>editor(null))); wrap.appendChild(actions);
  const list=el("div","specialdatelist"); wrap.appendChild(list);
  function draw(){
    list.innerHTML="";
    if(!items.length){ list.appendChild(ctrlStateCard("empty","No birthdays yet","Add birthdays here or through the installer setup questions.")); return; }
    for(const item of items) list.appendChild(specialDateRow(item,"birthday",{edit:()=>editor(item),del:async()=>{
      try{ const res=await api("/api/birthdays/delete","POST",{id:item.id}); items=res.items||items.filter(x=>x.id!==item.id); applyBirthdayRuntime(items); await loadCompliments(); renderCalendar(); renderAgenda(); ctrlMsg("Birthday removed."); await stableMessageAction(()=>renderCtrlBirthdays()); }
      catch(e){ ctrlMsg(e.message); }
    }}));
  }
  function editor(item){
    list.innerHTML=""; actions.style.display="none"; hideOSK();
    const form=el("div","compeditor");
    const name=oskInput("name",item?item.name:"");
    const date=oskInput("MM-DD",item?item.date:"",{mode:"mmdd"});
    form.appendChild(msgField("Name",name));
    form.appendChild(msgField("Birthday",date,"Use MM-DD. Dashes are inserted automatically."));
    const row=el("div","crow");
    const saveBirthday=cbtn(item?"Save birthday":"Add birthday","on",async()=>{
      const body={name:name.value.trim(),date:date.value.trim()};
      if(!body.name || !/^\d{2}-\d{2}$/.test(body.date)){ ctrlMsg("Use a name and MM-DD date."); showOSKFor(!body.name?name:date); return; }
      try{
        let res;
        if(item){ body.id=item.id; res=await api("/api/birthdays/update","POST",body); }
        else res=await api("/api/birthdays/add","POST",body);
        items=res.items||items; applyBirthdayRuntime(items); await loadCompliments(); renderCalendar(); renderAgenda(); ctrlMsg(item?"Birthday updated.":"Birthday added."); await stableMessageAction(()=>renderCtrlBirthdays());
      }catch(e){ ctrlMsg(e.message); }
    });
    oskSetSubmit(name,item?"Save":"Add",()=>saveBirthday.click());
    oskSetSubmit(date,item?"Save":"Add",()=>saveBirthday.click());
    row.appendChild(saveBirthday);
    row.appendChild(cbtn("Cancel","",()=>renderCtrlBirthdays())); form.appendChild(row); list.appendChild(form);
  }
  draw();
}
async function renderCtrlCelebrations(){
  const wrap=$("#ctrlcelebrations"); if(!wrap) return;
  ctrlSetLoading(wrap,"Loading celebrations…","Reading generated-calendar celebrations from ~/.dashboard-celebrations.");
  let data;
  try{ data=await api("/api/celebrations"); }
  catch(e){ ctrlSetError(wrap,"Celebrations unavailable",friendlyUnavailable("Celebrations",e)); return; }
  wrap.innerHTML=""; hideOSK();
  let items=Array.isArray(data.items)?data.items:[];
  wrap.appendChild(ctrlStateCard("info","Celebrations","Celebrations and special dates are used by the generated Celebrations calendar. Changes refresh generated calendars and the event cache."));
  const actions=el("div","ctrlrow compact msgactiongrid singleaction"); actions.appendChild(cbtn("Add celebration","on",()=>editor(null))); wrap.appendChild(actions);
  const list=el("div","specialdatelist"); wrap.appendChild(list);
  function draw(){
    list.innerHTML="";
    if(!items.length){ list.appendChild(ctrlStateCard("empty","No celebrations yet","Add anniversaries, special dates, or one-time reminders here.")); return; }
    for(const item of items) list.appendChild(specialDateRow(item,"celebration",{edit:()=>editor(item),del:async()=>{
      try{ const res=await api("/api/celebrations/delete","POST",{id:item.id}); items=res.items||items.filter(x=>x.id!==item.id); await loadCalendars(); ctrlMsg("Celebration removed."); await stableMessageAction(()=>renderCtrlCelebrations()); }
      catch(e){ ctrlMsg(e.message); }
    }}));
  }
  function editor(item){
    list.innerHTML=""; actions.style.display="none"; hideOSK();
    const form=el("div","compeditor");
    const label=oskInput("celebration label",item?item.label:"");
    const date=oskInput("MM-DD or YYYY-MM-DD",item?item.date:"",{mode:(item&&/^\d{4}-/.test(item.date))?"date":"mmdd"});
    form.appendChild(msgField("Label",label));
    form.appendChild(msgField("Date",date,"Use MM-DD for yearly dates or YYYY-MM-DD for a one-time date."));
    const row=el("div","crow");
    const saveCelebration=cbtn(item?"Save celebration":"Add celebration","on",async()=>{
      const body={label:label.value.trim(),date:date.value.trim()};
      if(!body.label || !/^(\d{2}-\d{2}|\d{4}-\d{2}-\d{2})$/.test(body.date)){ ctrlMsg("Use a label and MM-DD or YYYY-MM-DD date."); showOSKFor(!body.label?label:date); return; }
      try{
        let res;
        if(item){ body.id=item.id; res=await api("/api/celebrations/update","POST",body); }
        else res=await api("/api/celebrations/add","POST",body);
        items=res.items||items; await loadCalendars(); ctrlMsg(item?"Celebration updated.":"Celebration added."); await stableMessageAction(()=>renderCtrlCelebrations());
      }catch(e){ ctrlMsg(e.message); }
    });
    oskSetSubmit(label,item?"Save":"Add",()=>saveCelebration.click());
    oskSetSubmit(date,item?"Save":"Add",()=>saveCelebration.click());
    row.appendChild(saveCelebration);
    row.appendChild(cbtn("Cancel","",()=>renderCtrlCelebrations())); form.appendChild(row); list.appendChild(form);
  }
  draw();
}

async function renderCtrlSources(){
  const wrap=$("#ctrlsources"); if(!wrap) return;
  ctrlSetLoading(wrap,"Loading jokes, quotes, and facts sources…","Reading selectable message categories.");
  let data;
  try{ data=await api("/api/message-sources"); }
  catch(e){ ctrlSetError(wrap,"Feed providers & API keys unavailable",friendlyUnavailable("Feed providers & API keys",e)); return; }
  wrap.innerHTML=""; hideOSK();
  const defs=data.defs||[];
  const cache=data.cache||{};
  const sourceStatus=Array.isArray(cache.sourceStatus)?cache.sourceStatus:[];
  const statusById={}; sourceStatus.forEach(s=>{ if(s&&s.id) statusById[s.id]=s; });
  let enabled=new Set((data.prefs&&data.prefs.enabled)||[]);
  wrap.appendChild(ctrlStateCard("info","Feed providers & API keys","Choose message feed categories and optional online providers. Local text remains available when online sources are unavailable."));
  const actions=el("div","ctrlrow compact msgactiongrid");
  actions.appendChild(cbtn("Refresh now","on",async()=>{try{ await api("/api/message-sources/refresh","POST",{manual:true}); await loadCompliments(); ctrlMsg("Message feeds refreshed."); await stableMessageAction(()=>renderCtrlSources()); }catch(e){ ctrlMsg("Feed refresh failed: "+e.message); }}));
  actions.appendChild(cbtn("Save selection","",async()=>saveSources(true)));
  wrap.appendChild(actions);
  const grid=el("div","sourcegrid compactsourcegrid"); wrap.appendChild(grid);
  const statusBox=el("div","sourcehealth compact"); wrap.appendChild(statusBox);
  function sourceLine(d,st){
    if(!st) return "Not refreshed yet.";
    if(st.ok) return "Last: "+(st.servedByLabel||"online");
    return "Using local text";
  }
  function sourceDetailLine(d,st){
    const parts=[];
    const providers=Array.isArray(d.providerLabels)?d.providerLabels:[];
    const keyed=Array.isArray(d.keyEnv)?d.keyEnv:[];
    const errs=st&&Array.isArray(st.errors)?st.errors.length:0;
    const skipped=st&&Array.isArray(st.skipped)?st.skipped.length:0;
    parts.push((d.nsfw?"NSFW · ":"")+(providers.length?providers.length+" online fallback"+(providers.length===1?"":"s"):"local only"));
    if(keyed.length) parts.push("optional API key supported");
    if(errs) parts.push(errs+" provider issue"+(errs===1?"":"s"));
    if(skipped) parts.push(skipped+" keyed provider"+(skipped===1?"":"s")+" skipped");
    return parts.join(" · ");
  }
  function drawStatus(){
    statusBox.innerHTML="";
    if(!defs.length) return;
    const active=defs.filter(d=>enabled.has(d.id));
    const fallback=active.filter(d=>{ const st=statusById[d.id]; return st && !st.ok; }).length;
    const skipped=sourceStatus.reduce((n,st)=>n+(Array.isArray(st&&st.skipped)?st.skipped.length:0),0);
    const summary=fallback?fallback+" categor"+(fallback===1?"y is":"ies are")+" using local text.":"Enabled categories are ready.";
    statusBox.appendChild(ctrlStateCard(fallback?"warn":"ok","Source health",summary+(skipped?" Optional keyed providers skipped: "+skipped+".":"")));
    const details=document.createElement("details");
    details.className="sourceproviderdetails";
    details.innerHTML='<summary>Provider details</summary>';
    const list=el("div","sourceproviderlist");
    for(const d of defs){
      const st=statusById[d.id]||null;
      const row=el("div","sourceproviderrow");
      row.appendChild(el("span","ct",d.label));
      row.appendChild(el("span","cm",sourceDetailLine(d,st)));
      list.appendChild(row);
    }
    details.appendChild(list);
    statusBox.appendChild(details);
  }
  function drawSources(){
    const _top=(messagesPageEl()||{}).scrollTop||0;
    grid.innerHTML="";
    if(!defs.length){ grid.appendChild(ctrlStateCard("empty","No source definitions found","Message source definitions did not load.")); return; }
    for(const d of defs){
      const on=enabled.has(d.id);
      const st=statusById[d.id]||null;
      const desc=sourceLine(d,st);
      const btn=caction(d.label,desc,on?"on":"",()=>{ enabled.has(d.id)?enabled.delete(d.id):enabled.add(d.id); drawSources(); drawStatus(); });
      if(d.nsfw) btn.classList.add("nsfwsource");
      if(st && !st.ok) btn.classList.add("warnsource");
      grid.appendChild(btn);
    }
    drawStatus(); restoreMessagesScroll(_top);
  }
  async function saveSources(refresh){
    try{
      const res=await api("/api/message-sources","POST",{enabled:[...enabled],refresh:!!refresh,manual:!!refresh});
      CTRL_CACHE["/api/message-sources"]=res.status;
      const confirmed=Array.isArray(res&&res.status&&res.status.prefs&&res.status.prefs.enabled)?res.status.prefs.enabled:[];
      if(typeof setMessageFeedFreshnessEnabled==="function") setMessageFeedFreshnessEnabled(confirmed);
      await loadCompliments();
      if(typeof updateStale==="function") updateStale();
      ctrlMsg("Feed providers saved"+(refresh?" and refreshed.":"."));
      await stableMessageAction(()=>renderCtrlSources());
    }
    catch(e){ ctrlMsg("Could not save message sources: "+e.message); }
  }
  drawSources();
}

async function renderCtrlFeeds(){
  const wrap=$("#ctrlfeeds"); if(!wrap) return;
  ctrlSetLoading(wrap,"Loading pulled messages…","Reading cached quotes, jokes, and facts.");
  let data;
  try{ data=await api("/api/message-sources"); }
  catch(e){ ctrlSetError(wrap,"Pulled messages unavailable",friendlyUnavailable("Pulled messages",e)); return; }
  wrap.innerHTML=""; hideOSK();
  const cache=data.cache||{};
  wrap.appendChild(ctrlStateCard("info","Pulled items","Cached jokes, quotes, and facts rotate with your messages."));
  const actions=el("div","ctrlrow compact msgactiongrid singleaction");
  actions.appendChild(cbtn("Refresh now","on",async()=>{try{ await api("/api/message-sources/refresh","POST",{manual:true}); await loadCompliments(); ctrlMsg("Message feeds refreshed."); await stableMessageAction(()=>renderCtrlFeeds()); }catch(e){ ctrlMsg("Feed refresh failed: "+e.message); }}));
  wrap.appendChild(actions);
  const itemList=el("div","feeditemlist"); wrap.appendChild(itemList);
  function drawItems(){
    itemList.innerHTML="";
    const items=Array.isArray(cache.items)?cache.items:[];
    if(!items.length){ itemList.appendChild(ctrlStateCard("empty","No pulled items yet","Enable a source in Feed providers & API keys and tap Refresh now.")); return; }
    for(const it of items){
      const row=el("div","comprow feeditemrow");
      row.appendChild(el("span","ct",it.text||""));
      row.appendChild(el("span","cm",[it.source||"source",it.edited?"edited":"",msgWeightLabel(it.weight||1)].filter(Boolean).join(" · ")));
      row.appendChild(cbtn("Edit","",()=>editFeedItem(it)));
      const del=cbtn("Del","danger",async()=>{
        if(!del.classList.contains("armed")){ del.classList.add("armed"); del.textContent="Sure?"; setTimeout(()=>{del.classList.remove("armed");del.textContent="Del";},3000); return; }
        try{ await api("/api/message-sources/item/delete","POST",{id:it.id}); await loadCompliments(); ctrlMsg("Pulled item deleted. It will stay removed after refresh."); await stableMessageAction(()=>renderCtrlFeeds()); }
        catch(e){ ctrlMsg(e.message); }
      });
      row.appendChild(del); itemList.appendChild(row);
    }
  }
  function editFeedItem(it){
    itemList.innerHTML=""; hideOSK();
    const form=el("div","compeditor");
    const text=oskInput("feed item text",it.text||"");
    form.appendChild(msgField("Pulled item",text,"Edits persist across feed refreshes."));
    const weightPick=msgWeightPicker(it.weight||1,{title:"Pulled item priority"}); form.appendChild(weightPick.root);
    const row=el("div","crow");
    const savePulled=cbtn("Save pulled item","on",async()=>{
      try{ await api("/api/message-sources/item/update","POST",{id:it.id,text:text.value,weight:weightPick.get()}); await loadCompliments(); ctrlMsg("Pulled item updated."); await stableMessageAction(()=>renderCtrlFeeds()); }
      catch(e){ ctrlMsg(e.message); }
    });
    oskSetSubmit(text,"Save",()=>savePulled.click());
    row.appendChild(savePulled);
    row.appendChild(cbtn("Cancel","",()=>renderCtrlFeeds())); form.appendChild(row); itemList.appendChild(form);
  }
  drawItems();
}


// After the screen has been off (DPMS) or simply untouched for a while, the
// FIRST touch is meant to wake/orient — not to activate whatever happens to
// be under the finger (the wake tap was opening day popups). Swallow that
// first interaction when the input gap is large; normal use is unaffected.
let _lastInputTs=Date.now(), _swallowUntil=0;
document.addEventListener("touchstart",function(e){
  const now=Date.now();
  if(now-_lastInputTs>90000) _swallowUntil=now+800;
  _lastInputTs=now;
},{capture:true,passive:true});
for(const evt of ["touchend","click"]){
  document.addEventListener(evt,function(e){
    if(Date.now()<_swallowUntil){ e.stopPropagation(); e.preventDefault(); }
  },{capture:true});
}
