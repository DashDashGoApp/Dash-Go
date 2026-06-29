let CTRL_PEOPLE_RENDER_SEQ=0;
let CTRL_PEOPLE_STATE={payload:null,confirm:null,editing:"",pinEditing:"",busy:false};

function peopleControlRoot(){
  return document.querySelector("#ctrlpage-control [data-people-control-root]");
}
function peopleControlCurrent(root,seq){
  return root===peopleControlRoot() && seq===CTRL_PEOPLE_RENDER_SEQ;
}
function peopleImpactParts(impact){
  const labels=[
    ["routines","routine"],
    ["chores","chore"],
    ["maintenance","maintenance task"],
    ["todo","To Do item"],
    ["grocery","Grocery item"],
    ["messages","private message"]
  ];
  return labels.map(([key,label])=>{
    const count=Number(impact&&impact[key]||0);
    return count?`${count} ${label}${count===1?"":"s"}`:"";
  }).filter(Boolean);
}
function peopleImpactText(impact){
  const parts=peopleImpactParts(impact);
  return parts.length?parts.join(" · "):"No current assignments";
}
function peoplePersonName(person){
  return String(person&&person.name||"Former household member").trim()||"Former household member";
}
function peopleActiveRows(payload){
  return (payload&&Array.isArray(payload.people)?payload.people:[]).filter(person=>person&&person.state==="active");
}
function peopleControlBusy(){ return !!CTRL_PEOPLE_STATE.busy; }
function peopleButton(label,cls,handler){
  const button=cbtn(label,cls,handler);
  button.disabled=peopleControlBusy();
  return button;
}
async function peopleControlMutate(body){
  if(peopleControlBusy())return;
  CTRL_PEOPLE_STATE.busy=true;
  try{
    const payload=await api("/api/household/people","POST",body);
    CTRL_PEOPLE_STATE.payload=payload;
    CTRL_PEOPLE_STATE.confirm=null;
    CTRL_PEOPLE_STATE.editing="";
    CTRL_PEOPLE_STATE.pinEditing="";
    ctrlMsg("People updated.");
  }catch(error){
    ctrlMsg(error&&error.message?error.message:"People could not be updated.");
  }finally{
    CTRL_PEOPLE_STATE.busy=false;
    renderCtrlPeople();
  }
}
async function peopleInboxPinMutate(path,body,success){
  if(peopleControlBusy())return;
  CTRL_PEOPLE_STATE.busy=true;
  try{
    const payload=await api(path,"POST",body);
    CTRL_PEOPLE_STATE.payload=payload;
    CTRL_PEOPLE_STATE.confirm=null;
    CTRL_PEOPLE_STATE.pinEditing="";
    ctrlMsg(success);
  }catch(error){
    ctrlMsg(error&&error.message?error.message:"Personal inbox PIN could not be updated.");
  }finally{
    CTRL_PEOPLE_STATE.busy=false;
    renderCtrlPeople();
  }
}
function peopleInboxPINField(label,value,onEdit){
  const field=el("div","people-field");
  const entry=el("button","people-text-input people-pin-input people-pin-entry");
  entry.type="button";
  entry.disabled=peopleControlBusy();
  entry.setAttribute("aria-label",label);
  const digits=String(value||"").replace(/\D/g,"").slice(0,8);
  entry.append(
    el("span","people-pin-entry-value",digits?"•".repeat(digits.length):"Tap to enter"),
    el("span","people-pin-entry-action",digits?"Edit":"Open keypad")
  );
  bindTap(entry,onEdit);
  field.append(el("span","people-field-label",label),entry);
  return field;
}
function peopleInboxPINForm(person){
  const form=el("section","people-editor people-pin-editor");
  const values={pin:"",confirm:""};
  const fields=el("div","people-pin-fields");
  const actions=el("div","ctrlrow compact");
  function openPINField(key){
    const label=key==="pin"?"New personal inbox PIN":"Repeat personal inbox PIN";
    peopleOpenInboxPINKeypad({
      title:label,
      detail:key==="pin"?"Choose 4–8 digits for this shared-display inbox.":"Enter the same 4–8 digits again.",
      value:values[key],
      onCommit:value=>{values[key]=value;drawFields();}
    });
  }
  function drawFields(){
    fields.replaceChildren(
      peopleInboxPINField("New personal inbox PIN",values.pin,()=>openPINField("pin")),
      peopleInboxPINField("Repeat personal inbox PIN",values.confirm,()=>openPINField("confirm"))
    );
  }
  actions.append(
    peopleButton(person.inboxPinConfigured?"Change inbox PIN":"Set inbox PIN","on",async()=>{
      if(!/^\d{4,8}$/.test(values.pin)){ctrlMsg("Personal inbox PINs use 4 to 8 digits.");openPINField("pin");return;}
      if(values.pin!==values.confirm){ctrlMsg("The personal inbox PIN entries do not match.");openPINField("confirm");return;}
      await peopleInboxPinMutate("/api/household/people/inbox-pin/set",{personId:person.id,pin:values.pin},"Personal inbox PIN saved.");
    }),
    peopleButton("Cancel","",()=>{CTRL_PEOPLE_STATE.pinEditing="";renderCtrlPeople();})
  );
  form.append(
    el("div","people-editor-title",person.inboxPinConfigured?"Change personal inbox PIN":"Set personal inbox PIN"),
    el("div","people-editor-note","This PIN protects this inbox on the shared display. Tap each field to open the numeric keypad. You can remove or replace a forgotten PIN here without deleting messages."),
    fields,actions
  );
  drawFields();
  return form;
}

async function peopleNotificationMutate(person,next){
  if(peopleControlBusy())return;
  CTRL_PEOPLE_STATE.busy=true;
  try{
    const payload=await api("/api/household/people/notifications","POST",{
      personId:person.id,
      urgentHousehold:!!next.urgentHousehold,
      privateMessages:!!next.privateMessages,
      privatePreviews:!!next.privatePreviews
    });
    CTRL_PEOPLE_STATE.payload=payload;
    ctrlMsg("External notification preferences updated.");
  }catch(error){
    ctrlMsg(error&&error.message?error.message:"External notification preferences could not be updated.");
  }finally{
    CTRL_PEOPLE_STATE.busy=false;
    renderCtrlPeople();
  }
}
function peopleNotificationButton(label,on,handler){
  return peopleButton(label+(on?" · On":" · Off"),on?"on":"",handler);
}
const PEOPLE_APPRISE_ROUTE_SUPPORT_NOTE="Route setup supports apprise(s) through a full Apprise API server, gotify(s), ifttt, json(s), ntfy(s), form(s), and xml(s). Discord, email, Telegram, Slack, and Pushover require the Apprise API route.";
function peopleNotificationCard(person){
  const state=person&&person.notifications||{};
  const card=el("section","people-notifications");
  card.append(el("div","people-editor-title","External notifications"));
  const ready=!!state.routeConfigured&&!!state.deliveryEnabled;
  if(!state.routeConfigured){
    card.append(
      el("div","people-editor-note","No private delivery route is configured. Local inboxes still work normally. Set a route through Installer > Notifications (Apprise-Go)."),
      el("div","people-editor-note",PEOPLE_APPRISE_ROUTE_SUPPORT_NOTE)
    );
    return card;
  }
  if(!state.deliveryEnabled){
    card.append(el("div","people-editor-note","A private route is configured, but external Dash-Go delivery is disabled in Installer > Notifications (Apprise-Go)."));
    return card;
  }
  const current={
    urgentHousehold:!!state.urgentHousehold,
    privateMessages:!!state.privateMessages,
    privatePreviews:!!state.privatePreviews
  };
  const actions=el("div","ctrlrow compact people-notification-actions");
  const urgent=peopleNotificationButton("Urgent household alerts",current.urgentHousehold,()=>peopleNotificationMutate(person,{...current,urgentHousehold:!current.urgentHousehold}));
  const privateMessages=peopleNotificationButton("Private messages",current.privateMessages,()=>peopleNotificationMutate(person,{...current,privateMessages:!current.privateMessages,privatePreviews:current.privateMessages?current.privatePreviews:false}));
  const previews=peopleNotificationButton("Private message previews",current.privatePreviews,()=>peopleNotificationMutate(person,{...current,privatePreviews:!current.privatePreviews}));
  previews.disabled=!current.privateMessages||peopleControlBusy();
  actions.append(urgent,privateMessages,previews);
  card.append(
    el("div","people-editor-note","Private messages notify only this person. Previews may appear on outside provider history or lock screens."),
    actions
  );
  if(state.lastState){
    card.append(el("div","people-notification-status","Last delivery: "+String(state.lastState).replace(/-/g," ")+(state.lastAt?" · "+new Date(Number(state.lastAt)*1000).toLocaleString():"")));
  }else if(ready){
    card.append(el("div","people-notification-status","Delivery route ready. No message has been sent yet."));
  }
  return card;
}

function peopleNameInput(placeholder,value,ariaLabel){
  const input=oskInput(placeholder,value);
  input.maxLength=64;
  input.autocomplete="off";
  input.spellcheck=false;
  input.setAttribute("aria-label",ariaLabel);
  return input;
}
function peopleNameField(label,input){
  const field=el("label","people-field");
  field.append(el("span","people-field-label",label),input);
  return field;
}
function peopleAddForm(){
  const form=el("section","people-editor people-add-editor");
  const input=peopleNameInput("Person name","","Add household person");
  const controls=el("div","people-add-controls");
  controls.appendChild(peopleNameField("Name",input));
  const actions=el("div","ctrlrow compact people-add-actions");
  const addPerson=peopleButton("Add person","on",async()=>{
    const name=String(input.value||"").trim();
    if(!name){ctrlMsg("Enter a household member name.");showOSKFor(input);return;}
    await peopleControlMutate({op:"add",name});
  });
  oskSetSubmit(input,"Add",()=>addPerson.click());
  actions.append(addPerson);
  controls.appendChild(actions);
  form.append(
    el("div","people-editor-title","Add household member"),
    el("div","people-editor-note","Add people once, then assign household work wherever it belongs."),
    controls
  );
  buildOSK(form);
  return form;
}
function peopleRenameForm(person){
  const form=el("section","people-editor people-rename-editor");
  const input=peopleNameInput("Person name",peoplePersonName(person),"Rename "+peoplePersonName(person));
  const actions=el("div","ctrlrow compact people-rename-actions");
  const saveName=peopleButton("Save name","on",async()=>{
    const name=String(input.value||"").trim();
    if(!name){ctrlMsg("Enter a household member name.");showOSKFor(input);return;}
    await peopleControlMutate({op:"rename",id:person.id,name});
  });
  oskSetSubmit(input,"Save",()=>saveName.click());
  actions.append(saveName,peopleButton("Cancel","",()=>{CTRL_PEOPLE_STATE.editing="";renderCtrlPeople();}));
  form.append(
    el("div","people-editor-title","Rename household member"),
    peopleNameField("Name",input),
    actions
  );
  buildOSK(form);
  return form;
}
function peopleConfirmCard(person){
  const kind=CTRL_PEOPLE_STATE.confirm&&CTRL_PEOPLE_STATE.confirm.kind;
  if(!kind)return null;
  const card=el("section","people-confirm people-confirm-"+kind);
  const impact=peopleImpactText(person.impact);
  if(kind==="remove-pin"){
    card.append(
      el("div","settinglabel","Remove "+peoplePersonName(person)+"’s personal inbox PIN?"),
      el("div","settingdesc","Messages stay in this inbox. It will be open until a new personal PIN is set."),
      el("div","people-impact","This also ends any active private inbox session.")
    );
    const actions=el("div","ctrlrow compact");actions.append(
      peopleButton("Remove inbox PIN","danger",()=>peopleInboxPinMutate("/api/household/people/inbox-pin/remove",{personId:person.id},"Personal inbox PIN removed.")),
      peopleButton("Keep PIN","",()=>{CTRL_PEOPLE_STATE.confirm=null;renderCtrlPeople();})
    );card.appendChild(actions);return card;
  }
  if(kind==="archive"){
    card.append(
      el("div","settinglabel","Archive "+peoplePersonName(person)+"?"),
      el("div","settingdesc","New assignments will not offer this person. Existing To Do, Grocery, and Maintenance ownership remains visible as Former: "+peoplePersonName(person)+". Future routine and chore participation stops."),
      el("div","people-impact","Current use: "+impact)
    );
    const actions=el("div","ctrlrow compact");
    actions.append(
      peopleButton("Archive person","danger",()=>peopleControlMutate({op:"archive",id:person.id})),
      peopleButton("Cancel","",()=>{CTRL_PEOPLE_STATE.confirm=null;renderCtrlPeople();})
    );
    card.appendChild(actions);
    return card;
  }
  const active=peopleActiveRows(CTRL_PEOPLE_STATE.payload).filter(candidate=>candidate.id!==person.id);
  const select=document.createElement("select");
  select.className="people-reassign-select";
  select.setAttribute("aria-label","Future assignment resolution");
  select.appendChild(new Option("Make future work unassigned",""));
  active.forEach(candidate=>select.appendChild(new Option("Reassign future work to "+peoplePersonName(candidate),candidate.id)));
  const privateMessages=Number(person.impact&&person.impact.messages||0);
  card.append(el("div","settinglabel","Remove "+peoplePersonName(person)+"?"));
  if(privateMessages>0){
    card.append(
      el("div","settingdesc","This person has private Family Message Board history. Archive them instead so their inbox remains private and can be restored later."),
      el("div","people-impact","Private history: "+privateMessages+" message"+(privateMessages===1?"":"s"))
    );
    const actions=el("div","ctrlrow compact");actions.append(peopleButton("Archive person","danger",()=>peopleControlMutate({op:"archive",id:person.id})),peopleButton("Cancel","",()=>{CTRL_PEOPLE_STATE.confirm=null;renderCtrlPeople();}));card.appendChild(actions);return card;
  }
  card.append(
    el("div","settingdesc","This permanently removes the household person. Completed and skipped history stays unchanged. Choose whether future/open work becomes unassigned or is reassigned."),
    el("div","people-impact","Affected current work: "+impact),select
  );
  const actions=el("div","ctrlrow compact");actions.append(peopleButton("Remove person","danger",()=>peopleControlMutate({op:"delete",id:person.id,resolution:select.value?"reassign":"unassign",reassignTo:select.value})),peopleButton("Cancel","",()=>{CTRL_PEOPLE_STATE.confirm=null;renderCtrlPeople();}));card.appendChild(actions);return card;
}
function peopleRow(person){
  const row=el("section","people-row");
  const title=el("div","people-row-title");
  title.appendChild(el("strong","",peoplePersonName(person)));
  title.appendChild(el("span","people-state "+(person.state==="active"?"active":"archived"),person.state==="active"?"Active":"Archived"));
  row.appendChild(title);
  row.appendChild(el("div","settingdesc",person.state==="active"?"Available for new household assignments and a private Message Board inbox.":"Not available for new work; existing responsibility remains visible."));
  row.appendChild(el("div","people-impact",peopleImpactText(person.impact)));
  if(person.state==="active")row.appendChild(el("div","people-inbox-state",person.inboxPinConfigured?"Personal inbox PIN: Set":"Personal inbox PIN: Not set"));
  if(person.state==="active")row.appendChild(peopleNotificationCard(person));
  if(CTRL_PEOPLE_STATE.editing===person.id){
    row.appendChild(peopleRenameForm(person));
    return row;
  }
  if(CTRL_PEOPLE_STATE.pinEditing===person.id){
    row.appendChild(peopleInboxPINForm(person));
    return row;
  }
  const confirmation=CTRL_PEOPLE_STATE.confirm;
  if(confirmation&&confirmation.id===person.id){
    row.appendChild(peopleConfirmCard(person));
    return row;
  }
  const actions=el("div","ctrlrow compact people-row-actions");
  actions.appendChild(peopleButton("Rename","",()=>{CTRL_PEOPLE_STATE.editing=person.id;CTRL_PEOPLE_STATE.pinEditing="";CTRL_PEOPLE_STATE.confirm=null;renderCtrlPeople();}));
  if(person.state==="active"){
    actions.appendChild(peopleButton(person.inboxPinConfigured?"Change inbox PIN":"Set inbox PIN","",()=>{CTRL_PEOPLE_STATE.pinEditing=person.id;CTRL_PEOPLE_STATE.editing="";CTRL_PEOPLE_STATE.confirm=null;renderCtrlPeople();}));
    if(person.inboxPinConfigured)actions.appendChild(peopleButton("Remove inbox PIN","",()=>{CTRL_PEOPLE_STATE.confirm={kind:"remove-pin",id:person.id};CTRL_PEOPLE_STATE.editing="";CTRL_PEOPLE_STATE.pinEditing="";renderCtrlPeople();}));
  }
  if(person.state==="active"){
    actions.appendChild(peopleButton("Archive","",()=>{CTRL_PEOPLE_STATE.confirm={kind:"archive",id:person.id};CTRL_PEOPLE_STATE.editing="";renderCtrlPeople();}));
    actions.appendChild(peopleButton("Remove","danger",()=>{CTRL_PEOPLE_STATE.confirm={kind:"delete",id:person.id};CTRL_PEOPLE_STATE.editing="";renderCtrlPeople();}));
  }else{
    actions.appendChild(peopleButton("Restore","on",()=>peopleControlMutate({op:"restore",id:person.id})));
    actions.appendChild(peopleButton("Remove","danger",()=>{CTRL_PEOPLE_STATE.confirm={kind:"delete",id:person.id};CTRL_PEOPLE_STATE.editing="";renderCtrlPeople();}));
  }
  row.appendChild(actions);
  return row;
}
function renderPeopleControlPayload(root,payload){
  root.replaceChildren();
  root.appendChild(ctrlStateCard("info","Shared household roster",payload.note||"People are shared by household apps. Manage them here once."));
  root.appendChild(peopleAddForm());
  const active=(payload.people||[]).filter(person=>person&&person.state==="active");
  const archived=(payload.people||[]).filter(person=>person&&person.state!=="active");
  const activeGroup=el("section","people-group");
  activeGroup.appendChild(el("div","settinglabel","Active people"));
  if(active.length)active.forEach(person=>activeGroup.appendChild(peopleRow(person)));
  else activeGroup.appendChild(el("div","settingdesc","Add the people who share household work. Grocery, To Do, Maintenance, Routines, Chore Wheel, and Family Message Board remain usable without assignments."));
  root.appendChild(activeGroup);
  if(archived.length){
    const archivedGroup=el("section","people-group people-archived-group");
    archivedGroup.appendChild(el("div","settinglabel","Archived people"));
    archived.forEach(person=>archivedGroup.appendChild(peopleRow(person)));
    root.appendChild(archivedGroup);
  }
}
async function renderCtrlPeople(){
  const root=peopleControlRoot();
  if(!root)throw new Error("People Control root is missing");
  const seq=++CTRL_PEOPLE_RENDER_SEQ;
  if(!CTRL_PEOPLE_STATE.payload)root.replaceChildren(ctrlStateCard("loading","Loading People","Reading the local household roster."));
  try{
    const payload=await api("/api/household/people","GET");
    if(!peopleControlCurrent(root,seq))return;
    CTRL_PEOPLE_STATE.payload=payload;
    const ids=new Set((payload.people||[]).map(person=>person&&person.id));
    if(CTRL_PEOPLE_STATE.editing&&!ids.has(CTRL_PEOPLE_STATE.editing))CTRL_PEOPLE_STATE.editing="";
    if(CTRL_PEOPLE_STATE.pinEditing&&!ids.has(CTRL_PEOPLE_STATE.pinEditing))CTRL_PEOPLE_STATE.pinEditing="";
    if(CTRL_PEOPLE_STATE.confirm&&!ids.has(CTRL_PEOPLE_STATE.confirm.id))CTRL_PEOPLE_STATE.confirm=null;
    renderPeopleControlPayload(root,payload);
  }catch(error){
    if(!peopleControlCurrent(root,seq))return;
    root.replaceChildren(ctrlStateCard("warn","People unavailable",error&&error.message?error.message:"The local household roster could not be read.",[cbtn("Try again","",()=>renderCtrlPeople())]));
  }
}
