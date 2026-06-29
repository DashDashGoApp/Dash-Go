// Compact numeric keypad for the optional Family Board personal inbox PIN.
// This is deliberately separate from the shared text OSK: PIN fields never
// need letters, punctuation, or a browser/natural keyboard surface.
let PEOPLE_INBOX_PIN_KEYPAD=null;

function peopleInboxPINDigits(value){
  return String(value||"").replace(/\D/g,"").slice(0,8);
}
function peopleInboxPINKeypadRoot(){
  let root=document.getElementById("people-inbox-pin-keypad");
  if(root)return root;
  root=el("div","people-pin-keypad");
  root.id="people-inbox-pin-keypad";
  root.setAttribute("aria-hidden","true");
  document.body.appendChild(root);
  return root;
}
function closePeopleInboxPINKeypad(opts){
  const state=PEOPLE_INBOX_PIN_KEYPAD;
  if(!state)return;
  PEOPLE_INBOX_PIN_KEYPAD=null;
  document.removeEventListener("keydown",state.onKeyDown,true);
  state.root.classList.remove("show");
  state.root.setAttribute("aria-hidden","true");
  state.root.replaceChildren();
  if(!opts||opts.restoreFocus!==false){
    if(window.DashGoAppDialog)window.DashGoAppDialog.restoreFocus(state.origin);
    else state.origin?.focus?.();
  }
}
function peopleOpenInboxPINKeypad(opts){
  if(!opts||typeof opts.onCommit!=="function")return;
  closePeopleInboxPINKeypad({restoreFocus:false});
  if(typeof hideOSK==="function")hideOSK();
  const root=peopleInboxPINKeypadRoot();
  const state={
    root,
    origin:document.activeElement,
    value:peopleInboxPINDigits(opts.value),
    onCommit:opts.onCommit,
    onKeyDown:null
  };
  PEOPLE_INBOX_PIN_KEYPAD=state;
  root.replaceChildren();
  root.classList.add("show");
  root.setAttribute("aria-hidden","false");

  const panel=el("section","people-pin-keypad-panel");
  panel.setAttribute("role","dialog");
  panel.setAttribute("aria-modal","true");
  panel.setAttribute("aria-label",opts.title||"Personal inbox PIN keypad");
  const heading=el("div","people-pin-keypad-title",opts.title||"Personal inbox PIN");
  const detail=el("div","people-pin-keypad-detail",opts.detail||"Use 4–8 digits.");
  const display=el("div","people-pin-keypad-display","");
  display.setAttribute("aria-live","polite");
  display.setAttribute("aria-label","PIN entry");
  const notice=el("div","people-pin-keypad-notice","");
  const grid=el("div","people-pin-keypad-grid");
  const actions=el("div","people-pin-keypad-actions");

  function draw(){
    const n=state.value.length;
    display.textContent=n?"•".repeat(n):"—";
    display.setAttribute("aria-label",n+" digit"+(n===1?"":"s")+" entered");
  }
  function setNotice(text){notice.textContent=text||"";}
  function appendDigit(digit){
    if(state.value.length>=8){setNotice("A personal inbox PIN can have at most 8 digits.");return;}
    state.value+=digit;
    setNotice("");
    draw();
  }
  function erase(){
    state.value=state.value.slice(0,-1);
    setNotice("");
    draw();
  }
  function clear(){
    state.value="";
    setNotice("");
    draw();
  }
  function submit(){
    if(!/^\d{4,8}$/.test(state.value)){setNotice("Enter 4–8 digits, or tap Cancel.");return;}
    const value=state.value,onCommit=state.onCommit;
    closePeopleInboxPINKeypad({restoreFocus:false});
    onCommit(value);
  }
  for(const digit of ["1","2","3","4","5","6","7","8","9"]){
    grid.appendChild(cbtn(digit,"people-pin-keypad-key",()=>appendDigit(digit)));
  }
  grid.append(
    cbtn("Clear","people-pin-keypad-key people-pin-keypad-small",clear),
    cbtn("0","people-pin-keypad-key",()=>appendDigit("0")),
    cbtn("⌫","people-pin-keypad-key people-pin-keypad-small",erase)
  );
  actions.append(
    cbtn("Cancel","",()=>closePeopleInboxPINKeypad()),
    cbtn("OK","on",submit)
  );
  panel.append(heading,detail,display,notice,grid,actions);
  root.appendChild(panel);
  state.onKeyDown=event=>{
    if(PEOPLE_INBOX_PIN_KEYPAD!==state)return;
    if(/^\d$/.test(event.key)){event.preventDefault();appendDigit(event.key);return;}
    if(event.key==="Backspace"){event.preventDefault();erase();return;}
    if(event.key==="Enter"){event.preventDefault();submit();return;}
    if(event.key==="Escape"){event.preventDefault();closePeopleInboxPINKeypad();}
  };
  document.addEventListener("keydown",state.onKeyDown,true);
  draw();
  if(window.DashGoAppDialog)window.DashGoAppDialog.focusInitial(root,".people-pin-keypad-key");
}
