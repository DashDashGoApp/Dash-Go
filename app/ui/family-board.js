// family-board.js — lazy static-overlay Family Message Board.
(function(){
  const core=window.familyBoardCore;
  const state={data:null,inboxes:[],view:"board",form:null,confirm:null,priorFocus:null,busy:false,focusSerial:0,showAllActive:false,inbox:null,inboxPrompt:null,inboxDraft:null,inboxConfirm:null};
  const root=()=>document.getElementById("familyboard");
  const body=()=>document.getElementById("familyboard-body");
  const status=()=>document.getElementById("familyboard-status");
  const tabs=()=>document.getElementById("familyboard-tabs");
  const isOpen=()=>!!root()?.classList.contains("show");
  const noteList=()=>Array.isArray(state.data?.notes)?state.data.notes:[];
  const settings=()=>state.data?.settings||{};
  const alertsEnabled=()=>!!settings().showUrgentAlertsOnDashboard;
  function setStatus(text){const node=status();if(node)node.textContent=text||"";}
  function button(label,cls,handler){const node=el("button",cls||"fb-action",label);node.type="button";bindTap(node,handler);return node;}
  function field(label,value,mode,placeholder){
    const wrap=el("label","fb-field"),caption=el("span","",label),input=el("input");
    input.value=value==null?"":String(value);input.placeholder=placeholder||"";input.autocomplete="off";input.dataset.oskMode=mode||"text";
    input.addEventListener("focus",()=>showOSKFor(input));
    input.addEventListener("pointerup",()=>{input.focus();showOSKFor(input);},{passive:true});
    wrap.append(caption,input);return {wrap,input};
  }
  function choice(label,options,value,onChange){
    const wrap=el("div","fb-choice"),title=el("span","fb-choice-label",label);
    wrap.appendChild(title);for(const [key,name] of options){const b=button(name,"fb-choice-button"+(key===value?" on":""),()=>{onChange(key);if(state.form)state.form.focusNote=false;render();});b.setAttribute("aria-pressed",String(key===value));wrap.appendChild(b);}return wrap;
  }
  function emptyExpiration(){return {kind:"none",date:"",amount:"30",unit:"minutes"};}
  function draftFor(note,restore){
    const expiresAt=String(note?.expiresAt||"");
    return {text:String(note?.text||""),priority:note?.priority==="urgent"?"urgent":"normal",pinned:!!note?.pinned,expiration:expiresAt&&!restore?{kind:"keep",date:"",amount:"30",unit:"minutes",expiresAt}:emptyExpiration()};
  }
  function expirationPayload(expiration){
    const exp=expiration||emptyExpiration(),kind=String(exp.kind||"none");
    if(kind==="date")return {kind,date:String(exp.date||"")};
    if(kind==="duration")return {kind,amount:String(exp.amount||""),unit:String(exp.unit||"")};
    return {kind:kind==="keep"?"keep":"none"};
  }
  async function request(path,payload,token){
    const headers={Accept:"application/json"};if(payload)headers["Content-Type"]="application/json";if(token)headers["X-DashGo-Inbox-Token"]=token;
    const response=await fetch(path,{method:payload?"POST":"GET",headers,body:payload?JSON.stringify(payload):undefined,cache:"no-store"});
    const data=await response.json().catch(()=>({}));if(!response.ok)throw new Error(data.error||"Family Message Board is unavailable");return data;
  }
  function accept(response){
    state.data=response.state||response;
    state.inboxes=Array.isArray(response.inboxes)?response.inboxes:state.inboxes;
    const active=core.activeOrder(noteList()),count=active.length;
    setStatus(`${count} active household message${count===1?"":"s"}.`);
    if(typeof window.familyBoardFooterRefresh==="function")window.familyBoardFooterRefresh(response.summary);
  }
  function inboxToken(){return String(state.inbox?.token||"");}
  function inboxPerson(){return state.inbox?.person||null;}
  function releaseInbox(){const token=inboxToken();state.inbox=null;state.inboxDraft=null;state.inboxPrompt=null;state.inboxConfirm=null;if(token)request("/api/family-board/inboxes/lock",{},token).catch(()=>{});}
  async function inboxLoad(){const person=inboxPerson();if(!person)return;setStatus("Opening private inbox…");const data=await request("/api/family-board/inboxes/"+encodeURIComponent(person.id),null,inboxToken());state.inbox={...state.inbox,...data};setStatus(person.name+"’s private inbox is open.");render();}
  async function inboxUnlock(person,pin){if(state.busy)return;state.busy=true;setStatus("Unlocking inbox…");try{const data=await request("/api/family-board/inboxes/unlock",{personId:person.id,pin:pin||""});state.inbox={person:data.person,token:data.inboxToken,inbox:[],archive:[],sent:[],mode:"inbox",ttl:data.ttl||120};state.inboxPrompt=null;await inboxLoad();}catch(error){setStatus(error.message||"Could not unlock this inbox.");render();}finally{state.busy=false;}}
  async function inboxMutate(path,payload,success){if(state.busy||!inboxToken())return;state.busy=true;setStatus("Saving private message…");try{const data=await request(path,payload,inboxToken());state.inbox={...state.inbox,...data};state.inboxDraft=null;state.inboxConfirm=null;if(success)setStatus(success);render();}catch(error){setStatus(error.message||"Could not save private message.");render();}finally{state.busy=false;}}
  async function load(){setStatus("Loading family notes…");const result=await request("/api/family-board");accept(result);render();}
  async function mutate(path,payload,success){
    if(state.busy)return;state.busy=true;root()?.classList.add("busy");setStatus("Saving…");
    try{accept(await request(path,payload));state.form=null;state.confirm=null;if(success)setStatus(success);render();}
    catch(error){setStatus(error.message||"Could not save family note.");render();}
    finally{state.busy=false;root()?.classList.remove("busy");}
  }
  function queueFocus(input){const serial=++state.focusSerial;requestAnimationFrame(()=>{if(serial!==state.focusSerial||!isOpen()||!input.isConnected)return;input.focus();showOSKFor(input);});}
  function startForm(mode,note,restore){state.form={mode,id:note?.id||"",draft:draftFor(note,restore),focusNote:true};render();}
  function renderExpiration(draft,mode){
    const exp=draft.expiration||(draft.expiration=emptyExpiration());
    const choices=mode==="edit"&&exp.expiresAt?[["keep","Keep current"],["none","No expiration"],["date","On a date"],["duration","After a duration"]]:[["none","No expiration"],["date","On a date"],["duration","After a duration"]];
    const wrap=el("section","fb-expiration");wrap.appendChild(choice("Expiration",choices,exp.kind,key=>{if(key!=="keep"&&key!==exp.kind){const previous=exp.expiresAt||"";draft.expiration={...emptyExpiration(),kind:key,expiresAt:previous};}}));
    if(exp.kind==="keep"){
      wrap.appendChild(el("p","fb-note",core.expirationLabel(exp.expiresAt)));
    }else if(exp.kind==="date"){
      const date=field("Keep active through date",exp.date,"date","YYYY-MM-DD");date.input.inputMode="numeric";date.input.addEventListener("input",()=>{exp.date=date.input.value;});wrap.append(date.wrap,el("p","fb-note","The note expires at the next local midnight after this date."));
    }else if(exp.kind==="duration"){
      const amount=field("Expires after",exp.amount,"number","30");amount.input.inputMode="numeric";amount.input.addEventListener("input",()=>{exp.amount=amount.input.value;});
      const units=choice("Unit",[["minutes","Minutes"],["hours","Hours"]],exp.unit,key=>{exp.unit=key;});
      const quick=el("div","fb-choice fb-expiration-presets"),caption=el("span","fb-choice-label","Quick choices");quick.appendChild(caption);for(const [amountValue,unit,label] of [["15","minutes","15 min"],["30","minutes","30 min"],["1","hours","1 hour"],["2","hours","2 hours"],["4","hours","4 hours"]])quick.appendChild(button(label,"fb-choice-button",()=>{draft.expiration={...exp,kind:"duration",amount:amountValue,unit};state.form.focusNote=false;render();}));
      wrap.append(amount.wrap,units,quick,el("p","fb-note","Minutes: 1–1,440. Hours: 1–168. The server starts the duration when you save."));
    }else{
      wrap.appendChild(el("p","fb-note","Keeps until archived."));
    }
    return wrap;
  }
  function renderForm(){
    const draft=state.form.draft;const card=el("section","fb-card fb-form");card.appendChild(el("div","fb-card-title",state.form.mode==="add"?"Add household message":"Edit Family Note"));
    const note=field("Household message",draft.text,"text","Example: Pick up Sam at 4.");note.input.maxLength=320;card.appendChild(note.wrap);
    const meter=el("p","fb-note",`${String(draft.text||"").length}/320 characters`);card.appendChild(meter);note.input.addEventListener("input",()=>{draft.text=note.input.value;meter.textContent=`${draft.text.length}/320 characters`;});
    card.appendChild(choice("Priority",[["normal","Normal"],["urgent","Urgent"]],draft.priority,key=>{draft.priority=key;}));
    const pin=el("label","fb-check"),check=document.createElement("input");check.type="checkbox";check.checked=!!draft.pinned;check.addEventListener("change",()=>{draft.pinned=check.checked;});pin.append(check,document.createTextNode(" Pin as the dashboard message when urgent"));card.appendChild(pin);
    if(draft.priority!=="urgent")card.appendChild(el("p","fb-note","Pinned normal notes stay inside Family Message Board. Only urgent notes use the dashboard alert."));
    if(!alertsEnabled()){const context=el("p","fb-note","Urgent Family Board alerts are currently off on the dashboard.");const enable=button("Turn urgent alerts on","fb-action",()=>mutate("/api/family-board/settings",{showUrgentAlertsOnDashboard:true},"Urgent dashboard alerts on."));card.append(context,enable);}
    card.appendChild(renderExpiration(draft,state.form.mode));
    const actions=el("div","fb-actions");const endpoint=state.form.mode==="add"?"/api/family-board/notes/add":(state.form.mode==="restore"?"/api/family-board/notes/restore":"/api/family-board/notes/update");actions.append(button("Save Family Note","fb-action primary",()=>mutate(endpoint,{text:draft.text,priority:draft.priority,pinned:!!draft.pinned,expiration:expirationPayload(draft.expiration),id:state.form.id},"Household message saved.")),button("Cancel","fb-action",()=>{state.form=null;state.focusSerial++;hideOSK();render();}));card.appendChild(actions);if(state.form.focusNote!==false)queueFocus(note.input);return card;
  }
  function noteRow(note,archived){
    const row=el("article","fb-note-row"),main=el("div","fb-note-main"),badges=el("div","fb-badges");
    if(note.priority==="urgent")badges.appendChild(el("span","fb-badge urgent","Urgent"));if(note.pinned)badges.appendChild(el("span","fb-badge","Pinned"));
    if(badges.childNodes.length)main.appendChild(badges);main.append(el("strong","",note.text),el("small","",archived?core.archiveLabel(note):core.expirationLabel(note.expiresAt)));
    const actions=el("div","fb-row-actions");
    if(!archived&&note.priority==="urgent"&&!note.householdAcknowledgedAt)actions.append(button("Acknowledge alert","fb-action",()=>mutate("/api/family-board/notes/acknowledge",{id:note.id},"Household alert acknowledged.")));
    if(archived){actions.append(button("Restore","fb-action",()=>startForm("restore",note,true)),button("Delete permanently","fb-action danger",()=>{state.confirm={id:note.id,action:"delete",text:"Delete this archived family note permanently?"};render();}));}
    else {actions.append(button("Edit","fb-action",()=>startForm("edit",note,false)),button("Archive","fb-action",()=>mutate("/api/family-board/notes/archive",{id:note.id},"Household message archived.")),button("Delete","fb-action danger",()=>{state.confirm={id:note.id,action:"delete",text:"Delete this family note permanently?"};render();}));}
    row.append(main,actions);return row;
  }
  function renderActive(){
    const wrap=el("div","fb-view");wrap.append(button("Add household message","fb-action primary fb-add",()=>startForm("add",null,false)));
    const all=core.activeOrder(noteList()),notes=state.showAllActive?all:all.slice(0,50);
    if(!notes.length)wrap.appendChild(el("div","fb-empty","No active household messages. Add a short message everyone should see."));else notes.forEach(note=>wrap.appendChild(noteRow(note,false)));
    if(all.length>50&&!state.showAllActive){wrap.append(el("p","fb-note",`Showing 50 of ${all.length} active notes.`),button("Show all active notes","fb-action",()=>{state.showAllActive=true;render();}));}
    return wrap;
  }
  function renderArchive(){const wrap=el("div","fb-view"),notes=core.archiveOrder(noteList());if(!notes.length)wrap.appendChild(el("div","fb-empty","No archived household messages. Archived or expired messages remain here for 90 days."));else notes.forEach(note=>wrap.appendChild(noteRow(note,true)));return wrap;}
  function renderSettings(){const wrap=el("section","fb-card fb-settings"),enabled=alertsEnabled();wrap.append(el("div","fb-card-title","Dashboard urgent alerts"),el("p","fb-note","Pinned urgent notes show their message. Other urgent notes show a compact alert button. Normal notes stay inside Family Message Board."));const toggle=button(enabled?"Urgent Family Board alerts: On":"Urgent Family Board alerts: Off","fb-action primary",()=>mutate("/api/family-board/settings",{showUrgentAlertsOnDashboard:!enabled},enabled?"Urgent dashboard alerts off.":"Urgent dashboard alerts on."));toggle.setAttribute("aria-pressed",String(enabled));wrap.appendChild(toggle);return wrap;}
  function inboxDirectory(){return Array.isArray(state.inboxes)?state.inboxes:[];}
  function inboxCard(person){
    const card=el("section","fb-card fb-inbox-card");const line=el("div","fb-note-main");line.append(el("strong","",person.name),el("small","",person.protected?"Private inbox · PIN required":"Private inbox"));
    const action=button(person.protected?"Unlock inbox":"Open inbox","fb-action primary",()=>{if(person.protected){state.inboxPrompt={person,pin:""};render();}else inboxUnlock(person,"");});card.append(line,action);return card;
  }
  function renderInboxPrompt(){
    const person=state.inboxPrompt?.person;if(!person)return null;const card=el("section","fb-card fb-inbox-pin");card.append(el("div","fb-card-title","Unlock "+person.name+"’s inbox"),el("p","fb-note","This unlocks only this person’s private inbox for a short session."));
    const pin=field("Personal inbox PIN",state.inboxPrompt.pin,"number","PIN");pin.input.type="password";pin.input.inputMode="numeric";pin.input.maxLength=8;pin.input.addEventListener("input",()=>{state.inboxPrompt.pin=pin.input.value.replace(/\D/g,"");});
    const actions=el("div","fb-actions");actions.append(button("Unlock","fb-action primary",()=>inboxUnlock(person,state.inboxPrompt.pin)),button("Cancel","fb-action",()=>{state.inboxPrompt=null;hideOSK();render();}));card.append(pin.wrap,actions);queueFocus(pin.input);return card;
  }
  function privateMessageWhen(value){
    const stamp=Number(value||0),date=stamp>0?new Date(stamp):null;
    if(!date||Number.isNaN(date.getTime()))return "Just now";
    return date.toLocaleString(undefined,{month:"short",day:"numeric",hour:"numeric",minute:"2-digit"});
  }
  function privateRow(note,direction){
    const row=el("article","fb-note-row fb-private-row"),main=el("div","fb-note-main"),badges=el("div","fb-badges"),archived=direction==="archive";
    if(note.priority==="urgent")badges.append(el("span","fb-badge urgent","Urgent"));if(note.withdrawn)badges.append(el("span","fb-badge","Withdrawn"));if(archived)badges.append(el("span","fb-badge","Archived"));if(badges.childNodes.length)main.appendChild(badges);
    const peer=direction==="sent"?String(note.recipientNameSnapshot||"Recipient"):String(note.senderNameSnapshot||"Sender");
    const archiveDetail=archived&&note.recipientArchivedAt?" · Archived "+privateMessageWhen(note.recipientArchivedAt):"";
    main.append(el("strong","",note.text),el("small","",(direction==="sent"?"To ":"From ")+peer+" · "+privateMessageWhen(note.createdAt)+archiveDetail));
    const actions=el("div","fb-row-actions");
    if(direction==="sent"&&!note.withdrawn&&!note.recipientReadAt)actions.append(button("Withdraw","fb-action danger",()=>inboxMutate("/api/family-board/messages/"+encodeURIComponent(note.id)+"/withdraw",{},"Private message withdrawn.")));
    if(direction==="inbox")actions.append(button("Archive","fb-action",()=>inboxMutate("/api/family-board/messages/"+encodeURIComponent(note.id)+"/archive",{},"Private message moved to this inbox archive.")));
    if(archived){
      actions.append(button("Restore","fb-action",()=>inboxMutate("/api/family-board/messages/"+encodeURIComponent(note.id)+"/restore",{},"Private message restored to this inbox.")));
      actions.append(button("Delete","fb-action danger",()=>{state.inboxConfirm={id:note.id,text:"Delete this archived private message from "+String(inboxPerson()?.name||"this")+"’s inbox? The sender’s Sent history stays unchanged."};render();}));
    }
    row.append(main,actions);return row;
  }
  function renderDirectComposer(){
    const person=inboxPerson(),draft=state.inboxDraft||(state.inboxDraft={recipientPersonId:"",text:"",priority:"normal",focus:true});const card=el("section","fb-card fb-form");card.append(el("div","fb-card-title","New private message"),el("p","fb-note","From "+person.name+". Private messages never create a dashboard alert, including urgent messages."));
    const targets=inboxDirectory().filter(candidate=>candidate.id!==person.id);card.append(choice("To",targets.map(candidate=>[candidate.id,candidate.name]),draft.recipientPersonId,key=>{draft.recipientPersonId=key;}));
    const text=field("Message",draft.text,"text","Write a short private message.");text.input.maxLength=320;text.input.addEventListener("input",()=>{draft.text=text.input.value;});card.append(text.wrap,choice("Priority",[["normal","Normal"],["urgent","Urgent"]],draft.priority,key=>{draft.priority=key;}));
    const actions=el("div","fb-actions");actions.append(button("Send private message","fb-action primary",()=>inboxMutate("/api/family-board/messages",{recipientPersonId:draft.recipientPersonId,text:draft.text,priority:draft.priority},"Private message sent.")),button("Cancel","fb-action",()=>{state.inboxDraft=null;hideOSK();render();}));card.append(actions);if(draft.focus!==false)queueFocus(text.input);return card;
  }
  function renderInboxConfirm(){
    const pending=state.inboxConfirm;if(!pending)return null;const layer=el("div","fb-confirm"),card=el("section","fb-card");
    card.append(el("strong","",pending.text),el("p","fb-note","This removes the message from this recipient inbox only and cannot be undone."));
    const actions=el("div","fb-actions");actions.append(button("Delete from inbox","fb-action danger",()=>inboxMutate("/api/family-board/messages/"+encodeURIComponent(pending.id)+"/delete",{},"Archived private message deleted from this inbox.")),button("Keep archived message","fb-action",()=>{state.inboxConfirm=null;render();}));card.appendChild(actions);layer.appendChild(card);return layer;
  }
  function renderInbox(){
    const wrap=el("div","fb-view");if(state.inboxPrompt){wrap.appendChild(renderInboxPrompt());return wrap;}const person=inboxPerson();if(!person){wrap.append(el("p","fb-note","Choose an inbox. Private message details and unread state stay hidden until that person opens their inbox."));if(!inboxDirectory().length)wrap.appendChild(el("div","fb-empty","Add active household people in Dashboard Control before using private inboxes."));else inboxDirectory().forEach(entry=>wrap.appendChild(inboxCard(entry)));return wrap;}
    const pane=state.inbox.mode||"inbox",archiveCount=Array.isArray(state.inbox.archive)?state.inbox.archive.length:0,switcher=el("div","fb-choice");
    for(const [id,label] of [["inbox","Inbox"],["archive","Archive"+(archiveCount?" ("+archiveCount+")":"")],["sent","Sent"]]){const tab=button(label,"fb-choice-button"+(pane===id?" on":""),()=>{state.inbox.mode=id;state.inboxDraft=null;state.inboxConfirm=null;render();});tab.setAttribute("aria-pressed",String(pane===id));switcher.appendChild(tab);}switcher.appendChild(button("Lock inbox","fb-action",()=>{releaseInbox();setStatus("Private inbox locked.");render();}));
    const header=el("section","fb-card");header.append(el("div","fb-card-title",person.name+"’s private inbox"),switcher,el("p","fb-note","Archive belongs only to this inbox. Deleting an archived private message does not erase the sender’s Sent history."));wrap.appendChild(header);
    if(state.inboxDraft){wrap.appendChild(renderDirectComposer());return wrap;}
    if(pane==="inbox")wrap.append(button("New private message","fb-action primary fb-add",()=>{state.inboxDraft={recipientPersonId:"",text:"",priority:"normal",focus:true};render();}));
    const rows=Array.isArray(state.inbox[pane])?state.inbox[pane]:[];
    const empty=pane==="sent"?"No private messages sent from this inbox.":(pane==="archive"?"No archived private messages. Archive a received message to keep it out of this inbox without deleting it.":"No private messages in this inbox.");
    if(!rows.length)wrap.appendChild(el("div","fb-empty",empty));else rows.forEach(note=>wrap.appendChild(privateRow(note,pane)));
    if(state.inboxConfirm)wrap.appendChild(renderInboxConfirm());return wrap;
  }
  function renderConfirm(){const layer=el("div","fb-confirm"),card=el("section","fb-card");card.append(el("strong","",state.confirm.text),el("p","fb-note","This cannot be undone."));const actions=el("div","fb-actions");actions.append(button("Delete Family Note","fb-action danger",()=>mutate("/api/family-board/notes/delete",{id:state.confirm.id},"Household message deleted.")),button("Keep Family Note","fb-action",()=>{state.confirm=null;render();}));card.appendChild(actions);layer.appendChild(card);return layer;}
  function render(){if(!isOpen()||!state.data)return;const nav=tabs(),content=body();nav.replaceChildren();for(const [id,label] of [["board","Household Board"],["inboxes","Inboxes"],["archive","Archive"],["settings","Settings"]]){const tab=button(label,"fb-tab",()=>{state.view=id;state.form=null;state.confirm=null;state.showAllActive=false;state.inboxDraft=null;state.inboxPrompt=null;state.inboxConfirm=null;hideOSK();render();});tab.setAttribute("aria-pressed",String(state.view===id));nav.appendChild(tab);}let next=state.form?renderForm():(state.view==="inboxes"?renderInbox():state.view==="archive"?renderArchive():state.view==="settings"?renderSettings():renderActive());content.replaceChildren(next);if(state.confirm)content.appendChild(renderConfirm());}
  async function openFamilyBoard(){
    if(isOpen())return;const shell=root();if(!shell)return;state.priorFocus=document.activeElement;state.view="board";state.form=null;state.confirm=null;state.showAllActive=false;state.inbox=null;state.inboxPrompt=null;state.inboxDraft=null;state.inboxConfirm=null;shell.hidden=false;shell.classList.add("show");shell.setAttribute("aria-hidden","false");if(typeof completeAppLauncherHandoff==="function")completeAppLauncherHandoff();if(typeof armOverlayAutoClose==="function")armOverlayAutoClose();if(window.DashGoAppDialog)window.DashGoAppDialog.focusInitial(shell,"#familyboard-close");else requestAnimationFrame(()=>document.getElementById("familyboard-close")?.focus?.());try{await load();}catch(error){setStatus(error.message||"Family Message Board is unavailable");state.data={notes:[],settings:{showUrgentAlertsOnDashboard:false}};render();}
  }
  function closeFamilyBoard(){const shell=root();if(!shell)return;state.focusSerial++;hideOSK();state.form=null;state.confirm=null;releaseInbox();shell.classList.remove("show");shell.hidden=true;shell.setAttribute("aria-hidden","true");if(typeof disarmOverlayAutoClose==="function")disarmOverlayAutoClose();if(typeof resumeUiAfterOverlay==="function"&&!(typeof overlayIsOpen==="function"&&overlayIsOpen()))resumeUiAfterOverlay();const trigger=document.getElementById("cblaunch");if(window.DashGoAppDialog)window.DashGoAppDialog.restoreFocus(state.priorFocus,trigger);else (trigger&&!trigger.hidden?trigger:state.priorFocus)?.focus?.();}
  function bindShell(){const shell=root(),close=document.getElementById("familyboard-close");if(!shell)return;if(close)bindTap(close,closeFamilyBoard);bindTap(shell,closeFamilyBoard,{ignore:event=>event.target!==shell});document.addEventListener("keydown",event=>{if(event.key!=="Escape"||!isOpen())return;event.preventDefault();if(state.confirm){state.confirm=null;render();}else if(state.form){state.form=null;state.focusSerial++;hideOSK();render();}else if(state.inboxPrompt||state.inboxDraft||state.inboxConfirm){state.inboxPrompt=null;state.inboxDraft=null;state.inboxConfirm=null;state.focusSerial++;hideOSK();render();}else closeFamilyBoard();});}
  window.openFamilyBoardImpl=openFamilyBoard;window.closeFamilyBoard=closeFamilyBoard;window.familyBoardIsOpen=isOpen;bindShell();
})();
