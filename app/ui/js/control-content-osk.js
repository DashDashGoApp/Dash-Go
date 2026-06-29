const MSG_WEIGHT_PRESETS=[
  {label:"Normal",value:1,detail:"Rotates like any other message."},
  {label:"Often",value:5,detail:"Appears noticeably more."},
  {label:"Priority",value:25,detail:"Good for reminders."},
  {label:"Spotlight",value:100,detail:"Dominates without being exclusive."},
  {label:"Dominant",value:500,detail:"Best for temporary or urgent notes."},
];
function msgWeightLabel(v){
  const n=+v||1, best=MSG_WEIGHT_PRESETS.reduce((a,b)=>Math.abs(b.value-n)<Math.abs(a.value-n)?b:a,MSG_WEIGHT_PRESETS[0]);
  return best.value===n ? best.label : "Custom "+n;
}
function msgWeightPicker(initial,opts){
  opts=opts||{};
  let value=+initial||+opts.defaultValue||1;
  const root=el("div","msgweight");
  const title=el("div","msgweight-title",opts.title||"How often should this show?");
  const detail=el("div","msgweight-detail",opts.detail||"Pick a priority instead of typing a raw weight number.");
  const grid=el("div","msgweight-grid");
  root.append(title,detail,grid);
  function draw(){
    grid.innerHTML="";
    for(const o of MSG_WEIGHT_PRESETS){
      const b=caction(o.label,o.detail,o.value===value?"on":"",()=>{value=o.value;draw();});
      b.classList.add("weightoption");
      grid.appendChild(b);
    }
  }
  draw();
  return {root,get:()=>value,set:v=>{value=+v||1;draw();}};
}
function msgPresetPicker(options,initial,opts){
  opts=opts||{};
  let value=initial || (options[0]&&options[0][0]);
  const root=el("div","msgweight msgpreset");
  root.appendChild(el("div","msgweight-title",opts.title||"Choose one"));
  if(opts.detail) root.appendChild(el("div","msgweight-detail",opts.detail));
  const grid=el("div","msgweight-grid"); root.appendChild(grid);
  function draw(){
    grid.innerHTML="";
    for(const [val,label,detail] of options){
      const b=caction(label,detail||"",String(val)===String(value)?"on":"",()=>{value=val;draw();});
      grid.appendChild(b);
    }
  }
  draw();
  return {root,get:()=>value,set:v=>{value=v;draw();}};
}
function msgField(label,input,detail){
  const box=el("label","msgfield");
  box.appendChild(el("div","msgfield-label",label));
  box.appendChild(input);
  if(detail) box.appendChild(el("div","msgfield-detail",detail));
  return box;
}
function localDateISO(d){
  d=d||new Date();
  return d.getFullYear()+"-"+String(d.getMonth()+1).padStart(2,"0")+"-"+String(d.getDate()).padStart(2,"0");
}
function localTimeHM(d){
  d=d||new Date();
  return String(d.getHours()).padStart(2,"0")+":"+String(d.getMinutes()).padStart(2,"0");
}
function dayFromISO(s){
  const m=String(s||"").match(/^(\d{4})-(\d{2})-(\d{2})$/);
  if(!m) return new Date().getDay();
  return new Date(+m[1],+m[2]-1,+m[3]).getDay();
}
const MSG_DAYS=["Sun","Mon","Tue","Wed","Thu","Fri","Sat"];
function schedSummary(m){
  const rec={once:"One time",daily:"Daily",weekly:"Weekly",biweekly:"Bi-weekly",xweeks:"Every "+(m.intervalWeeks||3)+" weeks",monthly:"Monthly",xmonths:"Every "+(m.intervalMonths||2)+" months",yearly:"Yearly"}[m.recurrence||"once"]||"Scheduled";
  const days=(m.days||[]).map(d=>MSG_DAYS[+d]).filter(Boolean).join(", ");
  const dates=m.recurrence==="once" ? ((m.startDate||"")+" → "+(m.endDate||m.startDate||"")) : ((m.startDate||"")+(m.endDate?" until "+m.endDate:""));
  return [rec,days,dates,(m.startTime||"")+"–"+(m.endTime||"")].filter(Boolean).join(" · ");
}

function messagesPageEl(){ return document.querySelector("#ctrlpage-content"); }
function restoreMessagesScroll(top){
  const page=messagesPageEl(); if(!page) return;
  requestAnimationFrame(()=>{
    const max=Math.max(0,page.scrollHeight-page.clientHeight);
    page.scrollTop=Math.max(0,Math.min(top==null?page.scrollTop:top,max));
  });
}
function stableMessageAction(fn){
  const page=messagesPageEl();
  const top=page?page.scrollTop:0;
  return Promise.resolve().then(fn).finally(()=>restoreMessagesScroll(top));
}

function oskInput(placeholder,value,opts){
  opts=opts||{};
  const i=document.createElement("input");
  i.placeholder=placeholder; i.value=value||""; i.readOnly=true; i.className="oskfield";
  const ph=String(placeholder||"").toLowerCase();
  const mode=opts.mode || (ph.includes("yyyy-mm-dd")?"date":(ph.includes("mm-dd")?"mmdd":(ph.includes("hh:mm")?"time":"text")));
  i.dataset.oskMode=mode;
  i.addEventListener("focus",()=>showOSKFor(i));
  i.addEventListener("click",e=>{e.stopPropagation();showOSKFor(i);});
  return i;
}
function compPayloadMessages(r){ return (r&&Array.isArray(r.messages))?r.messages:[]; }
function defaultBuiltins(){ return (CONFIG.compliments||[]).filter(c=>c && c.text && !c._bday); }
function normMsgText(v){ return String(v||"").trim().replace(/\s+/g," ").toLowerCase(); }
function defaultKey(c){ return normMsgText(c && c.text); }
function defaultKeys(){ return defaultBuiltins().map(defaultKey).filter(Boolean); }
function defaultTextSet(){ return new Set(defaultKeys()); }
let _defaultsReconciled=false;
async function reconcileDefaultMessages(){
  if(_defaultsReconciled) return;
  _defaultsReconciled=true;
  try{
    const payload=await api("/api/compliments/reconcile-defaults","POST",{messages:defaultBuiltins()});
    CTRL_CACHE["/api/compliments"]=payload;
  }catch(_){ }
}
function _customBody(c){ return {...c, origin:"custom"}; }

async function renderCtrlComp(){
  const wrap=$("#ctrlcomp"); if(!wrap) return;
  ctrlSetLoading(wrap,"Loading message editor…","Preparing editable messages.");
  if(!CTRL_OPEN || !wrap.closest(".ctrlpage.show")) return;
  await reconcileDefaultMessages();
  let payload, items=[];
  try{
    payload=await api("/api/compliments"); CTRL_CACHE["/api/compliments"]=payload;
    const defaults=defaultTextSet();
    items=compPayloadMessages(payload).filter(m=>m.origin==="custom" && !defaults.has(normMsgText(m.text)));
  }catch(e){ ctrlSetError(wrap,"Messages unavailable",friendlyUnavailable("Message editing",e)); return; }
  wrap.replaceChildren(); hideOSK();
  const top=el("div","crow msgsearchbar");
  const search=oskInput("search…","");
  const addBtn=cbtn("Add new","",()=>editor(null));
  oskSetSubmit(search,"Done",()=>drawList());
  top.append(search,addBtn); wrap.appendChild(top);
  const list=el("div"); list.id="complist"; wrap.appendChild(list);
  const note=el("div",null,"Personal messages are custom notes only. Birthday messages stay in the installer birthday list; defaults are managed in Default messages.");
  note.style.cssText="font-size:13px;color:var(--dimmer);margin-top:6px;";
  wrap.appendChild(note);
  function drawList(){
    const q=search.value.toLowerCase(); list.innerHTML="";
    const shown=items.filter(c=>!q || (c.text||"").toLowerCase().includes(q));
    if(!shown.length){ list.appendChild(ctrlStateCard("empty",q?"No matching personal messages":"No personal messages yet",q?"Try a different search.":"Add your own with Add new.")); return; }
    for(const c of shown) list.appendChild(messageRow(c,{edit:()=>editor(c),del:()=>deleteComp(c,drawList,items,v=>items=v)}));
  }
  async function deleteComp(c,redraw,getItems,setItems){
    try{ delete CTRL_CACHE["/api/compliments"]; await api("/api/compliments/delete","POST",{id:c.id}); setItems(getItems.filter(x=>x.id!==c.id)); redraw(); await loadCompliments(); ctrlMsg("Message removed."); }
    catch(e){ ctrlMsg(e.message); }
  }
  function editor(c){
    list.innerHTML=""; top.style.display="none"; hideOSK();
    const form=el("div","compeditor");
    const f1=oskInput("message text",c?c.text:"");
    const f2=oskInput("date MM-DD (optional — shows only that day)",c&&c.date?c.date:"");
    form.appendChild(msgField("Message",f1));
    form.appendChild(msgField("Optional date",f2,"Use MM-DD when a message should only show on one date each year."));
    const weightPick=msgWeightPicker(c&&c.weight?c.weight:1,{title:"Message priority"}); form.appendChild(weightPick.root);
    const btns=el("div","crow");
    const saveMessage=cbtn(c?"Save":"Add","on",async()=>{
      const body=_customBody({ text:f1.value.trim() });
      if(f2.value.trim()) body.date=f2.value.trim();
      body.weight=weightPick.get();
      if(!body.text){ ctrlMsg("Message text can't be empty."); showOSKFor(f1); return; }
      if(body.date && !/^\d{2}-\d{2}$/.test(body.date)){ ctrlMsg("Date must be MM-DD, e.g. 10-04."); showOSKFor(f2); return; }
      try{
        if(c){ body.id=c.id; delete CTRL_CACHE["/api/compliments"]; await api("/api/compliments/update","POST",body); }
        else { delete CTRL_CACHE["/api/compliments"]; await api("/api/compliments/add","POST",body); }
        await loadCompliments(); ctrlMsg(c?"Message updated.":"Message added."); await stableMessageAction(()=>renderCtrlComp());
      }catch(e){ ctrlMsg(e.message); }
    });
    oskSetSubmit(f1,c?"Save":"Add",()=>saveMessage.click());
    oskSetSubmit(f2,c?"Save":"Add",()=>saveMessage.click());
    btns.appendChild(saveMessage);
    btns.appendChild(cbtn("Cancel","",()=>{top.style.display="";drawList();})); form.appendChild(btns); list.appendChild(form);
  }
  search._oninput=drawList; drawList();
}
function messageRow(c,handlers){
  const row=el("div","comprow");
  const meta=[c.date?("on "+c.date):"", (c.weight&&c.weight!==1)?msgWeightLabel(c.weight):"", c.when?c.when.join("/"):"", c.holiday?"holiday":"", c.share?"share "+Math.round(c.share*100)+"%":""].filter(Boolean).join(" · ");
  row.appendChild(el("span","ct",c.text||""));
  if(meta) row.appendChild(el("span","cm",meta));
  if(handlers.edit) row.appendChild(cbtn("Edit","",handlers.edit));
  if(handlers.del){
    const del=cbtn("Del","danger",async()=>{
      if(!del.classList.contains("armed")){ del.classList.add("armed"); del.textContent="Sure?"; setTimeout(()=>{del.classList.remove("armed");del.textContent="Del";},3000); return; }
      await handlers.del();
    });
    row.appendChild(del);
  }
  return row;
}
function fmtExpiry(ms){
  const n=+ms||0; if(!n) return "unknown";
  try{return new Date(n).toLocaleString([], {month:"short",day:"numeric",hour:"numeric",minute:"2-digit"});}
  catch(_){return String(n);}
}

async function renderCtrlBuiltins(){
  const wrap=$("#ctrlbuiltins"); if(!wrap) return;
  const oldDrawer=wrap.querySelector(".builtinlistdrawer");
  const keepListOpen=!!(oldDrawer&&oldDrawer.open);
  // The Messages page owns vertical position. Default messages intentionally
  // have no inner scrollTop to restore; stableMessageAction/anchor logic keeps
  // the page stable around an explicit list rebuild.
  ctrlSetLoading(wrap,"Loading default messages…","Reading built-in message toggles.");
  const built=defaultBuiltins();
  let payload;
  try{ payload=await api("/api/compliments"); CTRL_CACHE["/api/compliments"]=payload; }
  catch(e){ ctrlSetError(wrap,"Default messages unavailable",friendlyUnavailable("Default messages",e)); return; }
  wrap.innerHTML=""; hideOSK();
  const removed=new Set((payload&&Array.isArray(payload.removedDefaults))?payload.removedDefaults:[]);
  const cleared=payload&&payload.defaultsCleared===true;
  const keys=defaultKeys();
  const hidden=cleared?built.length:built.filter(d=>removed.has(defaultKey(d))).length;
  wrap.appendChild(ctrlStateCard("info","Default messages",built.length+" built-in · "+hidden+" hidden. Built-ins are set-and-forget; open the list only when you need to hide or restore individual defaults."));
  const actions=el("div","ctrlrow compact msgactiongrid");
  const addAll=cbtn("Restore all","on",async()=>{
    try{ delete CTRL_CACHE["/api/compliments"]; await api("/api/compliments/defaults/add-all","POST",{}); await loadCompliments(); ctrlMsg("All default messages restored."); await stableMessageAction(()=>renderCtrlBuiltins()); }
    catch(e){ ctrlMsg("Could not restore defaults: "+e.message); }
  });
  const removeAll=cbtn("Hide all","danger",async()=>{
    if(!removeAll.classList.contains("armed")){ removeAll.classList.add("armed"); removeAll.textContent="Sure?"; setTimeout(()=>{removeAll.classList.remove("armed");removeAll.textContent="Hide all";},3000); return; }
    try{ delete CTRL_CACHE["/api/compliments"]; await api("/api/compliments/defaults/remove-all","POST",{keys}); await loadCompliments(); ctrlMsg("All default messages hidden."); await stableMessageAction(()=>renderCtrlBuiltins()); }
    catch(e){ ctrlMsg("Could not hide defaults: "+e.message); }
  });
  actions.append(addAll,removeAll); wrap.appendChild(actions);
  const drawer=document.createElement("details");
  drawer.className="builtinlistdrawer";
  drawer.innerHTML='<summary>Show default message list</summary>';
  if(keepListOpen) drawer.open=true;
  const summary=drawer.querySelector("summary");
  const syncSummary=()=>{ if(summary) summary.textContent=drawer.open?"Hide default message list":"Show default message list"; };
  drawer.addEventListener("toggle",syncSummary); syncSummary();
  const list=el("div","builtinlist builtinlist-scroll"); drawer.appendChild(list); wrap.appendChild(drawer);
  if(!built.length){ list.appendChild(ctrlStateCard("empty","No built-in defaults found","The dashboard default-message list did not load.")); return; }
  for(const d of built){
    const key=defaultKey(d);
    const isRemoved=cleared || removed.has(key);
    const row=el("div","comprow builtinrow"+(isRemoved?" removed":""));
    row.appendChild(el("span","ct",d.text||""));
    const meta=[d.date?("on "+d.date):"", d.weight&&d.weight!==1?msgWeightLabel(d.weight):""].filter(Boolean).join(" · ");
    row.appendChild(el("span","cm",meta));
    const tog=cbtn(isRemoved?"Removed":"Added",isRemoved?"":"on",async()=>{
      try{
        delete CTRL_CACHE["/api/compliments"];
        await api("/api/compliments/defaults/toggle","POST",{key,removed:!isRemoved,allKeys:keys});
        await loadCompliments();
        await stableMessageAction(()=>renderCtrlBuiltins());
      }catch(e){ ctrlMsg("Could not update default: "+e.message); }
    });
    tog.classList.add(isRemoved?"removedtoggle":"addedtoggle");
    row.appendChild(tog); list.appendChild(row);
  }
}
