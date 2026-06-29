function messageActionFor(item){
  if(!item) return null;
  if(item.kind==="default" && item.defaultKey) return {label:"Hide this message", path:"/api/compliments/defaults/toggle", body:{key:item.defaultKey,removed:true}, cache:"/api/compliments"};
  if(item.kind==="feed" && item.id) return {label:"Hide this pulled item", path:"/api/message-sources/item/delete", body:{id:item.id}, cache:"/api/message-sources"};
  if(item.kind==="custom" && item.id) return {label:"Remove this message", path:"/api/compliments/delete", body:{id:item.id}, cache:"/api/compliments"};
  if(item.kind==="temporary" && item.id) return {label:"Remove temporary message", path:"/api/temporary-messages/delete", body:{id:item.id}, cache:"/api/temporary-messages"};
  if(item.kind==="scheduled" && item.id) return {label:"Remove scheduled message", path:"/api/scheduled-messages/delete", body:{id:item.id}, cache:"/api/scheduled-messages"};
  return null;
}
function messageKindLabel(item){
  if(!item) return "message";
  if(item.kind==="feed") return (item.source||"feed")+" message";
  if(item.kind==="default") return "default message";
  if(item.kind==="custom") return "personal message";
  if(item.kind==="temporary") return "temporary message";
  if(item.kind==="scheduled") return "scheduled message";
  if(item.kind==="birthday") return "birthday message";
  return "message";
}
async function messagePost(path,body,tokenOverride){
  const headers={"Content-Type":"application/json","Accept":"application/json"};
  if(tokenOverride) headers["X-Dashboard-Token"]=tokenOverride;
  else{ try{ if(typeof CTRL_TOKEN!=="undefined" && CTRL_TOKEN) headers["X-Dashboard-Token"]=CTRL_TOKEN; }catch(_){} }
  let res, j={};
  try{
    res=await fetch(path,{method:"POST",headers,body:JSON.stringify(body||{})});
    j=await res.json().catch(()=>({}));
  }catch(e){
    const err=new Error(e&&e.message?e.message:String(e||"Network request failed"));
    err.status=0; throw err;
  }
  if(!res.ok){
    const err=new Error(j.error||("HTTP "+res.status));
    err.status=res.status; err.payload=j; throw err;
  }
  return j;
}
function messageIsLockedError(e){
  const msg=String(e&&e.message?e.message:e||"").toLowerCase();
  return (e&&e.status===401 && msg.includes("locked")) || msg==="locked" || msg.includes("locked");
}
function messageSetPopupBasics(titleText, whenText){
  setPopupMode("messagepop");
  const title=$("#poptitle"), when=$("#popwhen"), body=$("#popbody");
  if(title) title.textContent=titleText||"Message options";
  if(when) when.textContent=whenText||"";
  if(body) body.innerHTML="";
  return body;
}
async function messageOneShotUnlock(action,item){
  return new Promise((resolve,reject)=>{
    const body=messageSetPopupBasics("Enter dashboard passcode","One-time message action");
    if(!body){ reject(new Error("message popup unavailable")); return; }
    let val="", lockoutUntil=0, lockTimer=null;
    const quote=document.createElement("div"); quote.className="msgpopquote"; quote.textContent='“'+(item&&item.text?item.text:"message")+'”';
    const note=document.createElement("div"); note.className="msgpopnote"; note.textContent="Enter the Dashboard Control PIN to allow this one message action. Dashboard Control will remain locked.";
    const dots=document.createElement("div"); dots.className="msgpindots";
    const status=document.createElement("div"); status.className="msgpopstatus";
    const grid=document.createElement("div"); grid.className="msgpingrid";
    function draw(){ dots.textContent=val?"•".repeat(val.length):"—"; }
    function remaining(){ return Math.max(0,Math.ceil((lockoutUntil-Date.now())/1000)); }
    function setStatus(msg,kind){ status.textContent=msg||""; status.className="msgpopstatus"+(kind?" "+kind:""); }
    function setButtons(disabled){ grid.querySelectorAll("button").forEach(b=>{ if((b.textContent||"").trim()!=="Clear") b.disabled=!!disabled; }); }
    function clearTimer(){ if(lockTimer){ clearInterval(lockTimer); lockTimer=null; } }
    function setLockout(seconds){
      const sec=Math.max(1,parseInt(seconds||0,10)||1); lockoutUntil=Date.now()+sec*1000; val=""; draw(); clearTimer();
      const tick=()=>{
        const rem=remaining();
        if(rem<=0){ clearTimer(); lockoutUntil=0; setButtons(false); setStatus("Lockout ended. Enter the passcode again.","ok"); return; }
        setButtons(true); setStatus("Too many wrong passcode attempts. Try again in "+rem+" second"+(rem===1?"":"s")+".","warn");
      };
      tick(); lockTimer=setInterval(tick,1000);
    }
    async function submit(){
      if(remaining()>0){ setLockout(remaining()); return; }
      if(val.length<4){ setStatus("Enter 4–8 digits.","warn"); return; }
      setStatus("Checking…","");
      try{
        const r=await messagePost("/api/lock/message-action",{pin:val,path:action.path});
        clearTimer(); resolve(r.token||"");
      }catch(e){
        const msg=String(e&&e.message?e.message:e||""); val=""; draw();
        let sec=0; const m=msg.match(/(?:try again in|in)\s+(\d+)\s*s/i); if(m) sec=parseInt(m[1],10);
        if(/too many|lockout|429/i.test(msg)) setLockout(sec||60);
        else setStatus("Wrong passcode. Try again.","warn");
      }
    }
    function addButton(label,cls,fn){ const b=messagePopupButton(label,cls||"",fn); grid.appendChild(b); return b; }
    ["1","2","3","4","5","6","7","8","9"].forEach(n=>addButton(n,"pin",()=>{ if(remaining()>0){ setLockout(remaining()); return; } if(val.length<8){ val+=n; draw(); setStatus("",""); } }));
    addButton("Clear","pin small",()=>{ val=""; draw(); if(!remaining()) setStatus("",""); });
    addButton("0","pin",()=>{ if(remaining()>0){ setLockout(remaining()); return; } if(val.length<8){ val+="0"; draw(); setStatus("",""); } });
    addButton("OK","pin small on",submit);
    const cancel=messagePopupButton("Cancel","ghost",()=>{ clearTimer(); reject(new Error("cancelled")); closeScrim(); });
    body.append(quote,note,dots,grid,status,cancel); draw();
  });
}
async function runMessageAction(action,item,status){
  status.textContent="Saving…";
  try{
    await messagePost(action.path,action.body);
  }catch(e){
    if(!messageIsLockedError(e)) throw e;
    status.textContent="PIN required…";
    const token=await messageOneShotUnlock(action,item);
    status=document.querySelector("#popbody .msgpopstatus") || status;
    status.textContent="Saving…";
    await messagePost(action.path,action.body,token);
  }
  try{ if(typeof CTRL_CACHE!=="undefined" && action.cache) delete CTRL_CACHE[action.cache]; }catch(_){}
  await loadCompliments();
  closeScrim();
  forceNextCompliment();
}
function messagePopupButton(label,cls,fn){
  const b=document.createElement("button");
  b.className="msgpopbtn "+(cls||""); b.textContent=label;
  b.addEventListener("click",fn);
  return b;
}
let _messagePopupPauseToken=null;
let _messagePopupShell=null;
function holdMessageRotationForPopup(){
  releaseMessagePopupRotationPause();
  _messagePopupPauseToken=acquireMessageRotationPause("message-options",120000);
  return _messagePopupPauseToken;
}
function releaseMessagePopupRotationPause(){
  if(!_messagePopupPauseToken) return;
  const token=_messagePopupPauseToken; _messagePopupPauseToken=null;
  releaseMessageRotationPause(token,true,"message-popup-close");
}
function ensureMessagePopupShell(){
  const body=$("#popbody"); if(!body) return null;
  const shell=_messagePopupShell;
  if(shell && shell.body===body && body.contains(shell.quote) && body.contains(shell.actions)) return shell;
  body.innerHTML="";
  const quote=document.createElement("div"); quote.className="msgpopquote";
  const note=document.createElement("div"); note.className="msgpopnote";
  const actions=document.createElement("div"); actions.className="msgpopactions";
  const status=document.createElement("div"); status.className="msgpopstatus";
  const next={body,quote,note,actions,status,item:null,remove:null};
  actions.appendChild(messagePopupButton("Skip once","primary",()=>{ closeScrim(); forceNextCompliment(); }));
  actions.appendChild(messagePopupButton("Show fewer like this","",()=>{ if(next.item) showFewerMessagesLike(next.item); closeScrim(); forceNextCompliment(); }));
  const remove=messagePopupButton("","warn",async()=>{
    const item=next.item, action=messageActionFor(item); if(!item||!action) return;
    try{ await runMessageAction(action,item,status); }
    catch(e){ if(String(e&&e.message?e.message:e)!=="cancelled") status.textContent="Could not update message: "+(e&&e.message?e.message:String(e)); }
  });
  remove.hidden=true; next.remove=remove; actions.appendChild(remove);
  actions.appendChild(messagePopupButton("Cancel","ghost",()=>closeScrim()));
  body.append(quote,note,actions,status); _messagePopupShell=next;
  return next;
}
function clearMessagePopupShellForReveal(){
  const shell=_messagePopupShell;
  if(!shell || !shell.body || !shell.body.contains(shell.quote)){
    const body=$("#popbody"); if(body) body.textContent="";
    return;
  }
  shell.quote.textContent=""; shell.note.textContent=""; shell.status.textContent=""; shell.actions.hidden=true;
}
function fillMessagePopupShell(shell,item){
  if(!shell||!item) return;
  shell.item={...item};
  shell.quote.textContent='“'+item.text+'”';
  shell.note.textContent="Choose what to do with this rotating message. Hide/remove only affects this item; Show fewer adjusts similar feed/category picks.";
  const action=messageActionFor(item);
  shell.remove.hidden=!action; shell.remove.textContent=action?action.label:"";
  shell.status.textContent=""; shell.actions.hidden=false;
}
function openMessageOptionsPopup(itemOverride){
  const item=itemOverride || _lastCompItem;
  if(!item || !item.text) return;
  const title=$("#poptitle"), when=$("#popwhen"), body=$("#popbody");
  if(!body) return;
  setPopupMode("messagepop");
  if(title) title.textContent="Message options";
  if(when) when.textContent=messageKindLabel(item);
  clearMessagePopupShellForReveal();
  holdMessageRotationForPopup();
  // Acknowledge the gesture before the DOM-heavy body work. This is especially
  // noticeable on the Pi Zero 2 W and keeps animation timers out of the way.
  openScrim();
  requestAnimationFrame(()=>{
    const scrim=$("#scrim"), pop=$("#pop");
    if(!scrim || !scrim.classList.contains("show") || !pop || !pop.classList.contains("messagepop")) return;
    const shell=ensureMessagePopupShell(); fillMessagePopupShell(shell,item);
  });
}
function setupMessageLongPress(){
  // A large triple-tap surface remains deliberately unobtrusive, but it now
  // shares the same pointer/touch/mouse semantics as the moon and Control.
  const target=document.getElementById("compliment") || document.getElementById("comptext");
  if(!target || target.dataset.messageActionsReady==="1") return;
  target.dataset.messageActionsReady="1";
  const shouldIgnoreStart=e=>{
    const n=e&&e.target;
    return !!(n&&n.closest&&n.closest("#sun,#stale,.cb-launch,button,a,input,textarea,select,[data-no-message-action],[data-no-message-longpress]"));
  };
  const tapDebug=(msg,obj)=>{ try{ if(localStorage.getItem("dashboard:messageTapDebug")==="1") console.log("message-triple-tap",msg,obj||""); }catch(_){} };
  let tapPause=null, tapPauseTimer=null;
  const releaseTapPause=(reschedule,why)=>{
    if(tapPauseTimer){ clearTimeout(tapPauseTimer); tapPauseTimer=null; }
    if(!tapPause) return;
    const token=tapPause; tapPause=null;
    releaseMessageRotationPause(token,reschedule,why);
  };
  target.addEventListener("contextmenu",e=>e.preventDefault());
  target.addEventListener("selectstart",e=>e.preventDefault());
  target.addEventListener("dragstart",e=>e.preventDefault());
  attachTaps(target,{
    maxTaps:3,gap:800,moveTol:32,ignore:shouldIgnoreStart,
    onAnyTap:(n,meta)=>{
      tapDebug("tap",{count:n,source:meta&&meta.source});
      if(n!==1) return;
      releaseTapPause(false,"tap-restart");
      tapPause=acquireMessageRotationPause("message-tap",2500);
      tapPauseTimer=setTimeout(()=>releaseTapPause(true,"tap-abandoned"),2500);
    },
    onTaps:()=>{
      const item=(_lastCompItem&&_lastCompItem.text)?{..._lastCompItem}:null;
      if(!item){ releaseTapPause(true,"no-item"); return; }
      tapDebug("open",{kind:item.kind,source:item.source});
      openMessageOptionsPopup(item); // acquires its own pause before handoff
      releaseTapPause(false,"handed-to-popup");
    }
  });
}
