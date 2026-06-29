// 00b-shared-osk.js — always-loaded kiosk OSK runtime for Dashboard Control and every app.
let _oskTarget=null;
let _oskLayer="letters";
let _oskShift=false;
let _oskCapsLock=false;
let _oskLastShiftTap=0;
let _oskSubmitting=false;
function _oskMode(){ return (_oskTarget && _oskTarget.dataset && _oskTarget.dataset.oskMode) || "text"; }
function _formatDateDigits(v,pattern){
  const digits=String(v||"").replace(/\D/g,"").slice(0, pattern==="mmdd"?4:8);
  if(pattern==="mmdd") return digits.length<=2 ? digits : digits.slice(0,2)+"-"+digits.slice(2);
  if(digits.length<=4) return digits;
  if(digits.length<=6) return digits.slice(0,4)+"-"+digits.slice(4);
  return digits.slice(0,4)+"-"+digits.slice(4,6)+"-"+digits.slice(6);
}
function _formatTimeDigits(v){
  const digits=String(v||"").replace(/\D/g,"").slice(0,4);
  return digits.length<=2 ? digits : digits.slice(0,2)+":"+digits.slice(2);
}
function _oskSetValue(v){
  if(!_oskTarget) return;
  const mode=_oskMode();
  if(mode==="date") v=_formatDateDigits(v,"ymd");
  else if(mode==="mmdd") v=_formatDateDigits(v,"mmdd");
  else if(mode==="time") v=_formatTimeDigits(v);
  else if(mode==="numbers") v=String(v||"").replace(/\D/g,"");
  _oskTarget.value=v;
  if(_oskTarget._oninput) _oskTarget._oninput();
  _oskTarget.dispatchEvent(new Event("input",{bubbles:true}));
}
function oskType(ch){
  if(!_oskTarget) return;
  const mode=_oskMode();
  if(ch==="\b") return _oskSetValue(_oskTarget.value.slice(0,-1));
  if(mode==="date" || mode==="mmdd" || mode==="time" || mode==="numbers"){
    if(!/\d/.test(ch)) return;
    return _oskSetValue((_oskTarget.value||"")+ch);
  }
  _oskSetValue((_oskTarget.value||"")+ch);
}
function oskRoot(){
  // `$()` is a CSS-selector helper. This keyboard is body-mounted by ID, so a
  // bare "osk" lookup would search for an <osk> element and silently miss it.
  // Clean up legacy duplicate roots created by that old selector bug before
  // returning the one canonical keyboard surface.
  const roots=[...document.querySelectorAll("#osk")];
  const root=roots.shift()||null;
  for(const stale of roots){
    if(stale&&stale.parentNode) stale.parentNode.removeChild(stale);
  }
  return root;
}
function hideOSK(){
  const osk=oskRoot();
  if(osk) osk.classList.remove("show");
  if(_oskTarget) _oskTarget.classList.remove("oskfocus");
  document.querySelectorAll(".ctrlpage.osk-open,#listsapp.osk-open,#chorewheel.osk-open,#familyboard.osk-open,#maintenance.osk-open,#routines.osk-open").forEach(page=>page.classList.remove("osk-open"));
  document.documentElement.style.removeProperty("--osk-h");
  _oskTarget=null;
}
function oskSetSubmit(input,label,handler){
  if(!input) return input;
  input._oskSubmit=typeof handler==="function"?handler:null;
  input.dataset.oskSubmitLabel=String(label||"Enter").trim()||"Enter";
  return input;
}
function oskSubmitLabel(){
  return (_oskTarget&&_oskTarget.dataset&&_oskTarget.dataset.oskSubmitLabel)||"Enter";
}
async function oskSubmit(){
  const input=_oskTarget;
  if(!input||_oskSubmitting) return;
  const submit=input._oskSubmit;
  // Close before validation/action work. A handler that needs correction can
  // deliberately focus the field again without fighting the current surface.
  hideOSK();
  if(typeof submit!=="function") return;
  _oskSubmitting=true;
  try{ await submit(); }
  finally{ _oskSubmitting=false; }
}
function buildOSK(){
  let osk=oskRoot();
  if(!osk){
    osk=el("div");
    osk.id="osk";
    document.body.appendChild(osk);
    document.addEventListener("click",event=>{
      if(!osk.classList.contains("show")) return;
      const target=event.target;
      if(target.closest && (target.closest("#osk") || target.closest(".lists-prompt") || target.classList.contains("oskfield"))) return;
      // Chore Wheel creates its touch-first form after the trigger release.
      // Surf may dispatch that trigger's synthetic click after the OSK opens;
      // treat all app-internal clicks as internal while a Chore Wheel field owns
      // the keyboard. Explicit save/cancel/close paths still call hideOSK().
      const appHost=_oskTarget&&_oskTarget.closest&&_oskTarget.closest("#chorewheel,#familyboard,#maintenance,#routines");
      if(appHost && target.closest && target.closest("#chorewheel,#familyboard,#maintenance,#routines")) return;
      hideOSK();
    },true);
    document.addEventListener("toggle",event=>{
      if(event.target && event.target.matches && event.target.matches("#ctrlpage-content details:not([open])")) hideOSK();
    },true);
  }
  refreshOSKKeys(osk);
  return osk;
}
function oskRowsForMode(){
  const mode=_oskMode();
  if(mode==="date" || mode==="mmdd" || mode==="time" || mode==="numbers") return [["1","2","3"],["4","5","6"],["7","8","9"],["⌫","0","hide","submit"]];
  const letterRows=[
    ["1","2","3","4","5","6","7","8","9","0"],
    ["q","w","e","r","t","y","u","i","o","p"],
    ["a","s","d","f","g","h","j","k","l","'"],
    ["z","x","c","v","b","n","m",",",".",":"],
  ];
  const symbolRows=[
    ["!","@","#","$","%","&","*","(",")","/"],
    ["-","_","=","+","\\","?",";",":","'","\""],
    [",",".","<",">","[","]","{","}","|","`"],
  ];
  return _oskLayer==="symbols"?symbolRows:letterRows;
}
function refreshOSKKeys(osk){
  if(!osk) return;
  osk.innerHTML="";
  const mode=_oskMode();
  const mkKey=(label,cls,fn)=>{
    const key=el("button","oskkey"+(cls?" "+cls:""),label);
    key.addEventListener("pointerdown",event=>event.preventDefault());
    key.addEventListener("click",event=>{event.preventDefault();event.stopPropagation();fn();});
    return key;
  };
  function typeChar(ch){
    if(ch==="hide") return hideOSK();
    if(ch==="submit") return oskSubmit();
    if(ch==="⌫") return oskType("\b");
    const out=(_oskLayer==="letters" && /^[a-z]$/.test(ch) && (_oskShift||_oskCapsLock))?ch.toUpperCase():ch;
    oskType(out);
    if(_oskShift && !_oskCapsLock){_oskShift=false;refreshOSKKeys(osk);measureOSK(osk);}
  }
  for(const row of oskRowsForMode()){
    const rowEl=el("div","oskrow"+(mode==="date"||mode==="mmdd"||mode==="time"?" osknumrow":""));
    for(const ch of row){
      const control=ch==="⌫"||ch==="hide"||ch==="submit";
      const label=ch==="submit"?oskSubmitLabel():((_oskLayer==="letters" && /^[a-z]$/.test(ch) && (_oskShift||_oskCapsLock))?ch.toUpperCase():ch);
      rowEl.appendChild(mkKey(label,control?(ch==="submit"?"wide osksubmit":"wide"):"",()=>typeChar(ch)));
    }
    osk.appendChild(rowEl);
  }
  if(mode==="date" || mode==="mmdd" || mode==="time" || mode==="numbers"){
    osk.appendChild(el("div","oskhint",mode==="time"?"Numbers only — colon is inserted automatically.":(mode==="numbers"?"Numbers only.":"Numbers only — dashes are inserted automatically.")));
    measureOSK(osk);
    return;
  }
  const last=el("div","oskrow osk-actions");
  last.appendChild(mkKey(_oskLayer==="symbols"?"ABC":"?123","wide",()=>{
    _oskLayer=_oskLayer==="symbols"?"letters":"symbols";
    refreshOSKKeys(osk);
    measureOSK(osk);
  }));
  const shiftKey=mkKey(_oskCapsLock?"CAPS":"SHIFT","wide",()=>{
    const now=Date.now();
    if(_oskShift && now-_oskLastShiftTap<700){_oskCapsLock=!_oskCapsLock;_oskShift=_oskCapsLock;}
    else if(_oskCapsLock){_oskCapsLock=false;_oskShift=false;}
    else {_oskShift=!_oskShift;}
    _oskLastShiftTap=now;
    refreshOSKKeys(osk);
    measureOSK(osk);
  });
  if(_oskShift||_oskCapsLock) shiftKey.classList.add("on");
  last.appendChild(shiftKey);
  last.appendChild(mkKey("space","space",()=>typeChar(" ")));
  last.appendChild(mkKey("⌫","wide",()=>oskType("\b")));
  last.appendChild(mkKey("hide","wide",()=>hideOSK()));
  last.appendChild(mkKey(oskSubmitLabel(),"wide osksubmit",()=>oskSubmit()));
  osk.appendChild(last);
  measureOSK(osk);
}
function measureOSK(osk){
  if(!osk) return;
  requestAnimationFrame(()=>{
    const height=Math.ceil(osk.getBoundingClientRect().height||300);
    document.documentElement.style.setProperty("--osk-h",height+"px");
  });
}
function showOSKFor(input){
  if(_oskTarget && _oskTarget!==input) _oskTarget.classList.remove("oskfocus");
  _oskTarget=input;
  input.classList.add("oskfocus");
  if(_oskMode()==="date" || _oskMode()==="mmdd" || _oskMode()==="time" || _oskMode()==="numbers"){
    _oskLayer="numbers";
    _oskShift=false;
    _oskCapsLock=false;
  }else{
    // Text entry begins like a normal sentence: the first letter is capitalized.
    _oskLayer="letters";
    _oskShift=true;
    _oskCapsLock=false;
    _oskLastShiftTap=0;
  }
  const osk=buildOSK();
  osk.classList.add("show");
  const page=input.closest("#listsapp,#chorewheel,#familyboard,#maintenance,#routines,.ctrlpage")||$("#ctrlpage-content");
  if(page) page.classList.add("osk-open");
  measureOSK(osk);
  setTimeout(()=>input.scrollIntoView({block:"center",behavior:"smooth"}),40);
}
