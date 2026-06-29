// 11b-family-board-footer.js — bounded dashboard alert for urgent family notes.
const FAMILY_BOARD_FOOTER={timer:null,summary:null,booted:false,resizeTimer:null};
function familyBoardFooterRoot(){return document.getElementById("familyboardfooter");}
function familyBoardFooterLane(){return document.getElementById("compliment");}
function familyBoardFooterClearTimer(){if(FAMILY_BOARD_FOOTER.timer){clearTimeout(FAMILY_BOARD_FOOTER.timer);FAMILY_BOARD_FOOTER.timer=null;}}
function familyBoardFooterMode(summary){
  const intended=String(summary&&summary.displayMode||"none");
  if(!summary||!summary.showUrgentAlertsOnDashboard||!Number(summary.urgentCount)||!(["alert","message"].includes(intended)))return "none";
  if(intended!=="message")return "alert";
  const lane=familyBoardFooterLane(),stale=document.getElementById("stale");
  const width=lane?lane.getBoundingClientRect().width:0;
  // At compact dashboard widths the message lane shares space with Clock/App
  // controls and an operational health pill. Degrade the household card to a
  // touch-safe ! before either urgency surface becomes unreadable.
  const healthVisible=!!stale?.classList.contains("show");
  return width>0&&width<(healthVisible?760:500)?"alert":"message";
}
function familyBoardFooterSetLane(mode){
  const lane=familyBoardFooterLane();if(!lane)return;
  lane.classList.toggle("has-family-board-alert",mode!=="none");
  lane.classList.toggle("has-family-board-message",mode==="message");
}
function familyBoardFooterRender(summary){
  FAMILY_BOARD_FOOTER.summary=summary||{};const root=familyBoardFooterRoot();if(!root)return;
  const mode=familyBoardFooterMode(FAMILY_BOARD_FOOTER.summary),note=summary&&summary.note;
  root.hidden=mode==="none";root.className=`family-board-footer mode-${mode}`;root.dataset.mode=mode;familyBoardFooterSetLane(mode);
  if(mode==="none"){root.replaceChildren();root.setAttribute("aria-label","Open Family Message Board");familyBoardFooterClearTimer();return;}
  const urgentCount=Math.max(1,Number(summary.urgentCount)||1),countLabel=`${urgentCount} urgent ${urgentCount===1?"note":"notes"}`;
  if(mode==="alert"){
    const mark=document.createElement("span");mark.className="family-board-footer-mark";mark.setAttribute("aria-hidden","true");mark.textContent="!";
    root.replaceChildren(mark);root.setAttribute("aria-label",`Open Family Message Board: ${countLabel}.`);
  }else{
    const label=document.createElement("span");label.className="family-board-footer-label";label.textContent="! Urgent";
    const text=document.createElement("span");text.className="family-board-footer-text";text.textContent=String(note&&note.text||"");
    const extra=document.createElement("span");extra.className="family-board-footer-extra";extra.textContent=urgentCount>1?`+${urgentCount-1} urgent`:"";
    root.replaceChildren(label,extra,text);root.setAttribute("aria-label",`Open Family Message Board: ${countLabel}. ${String(note&&note.text||"")}`);
  }
  familyBoardFooterArmExpiry(summary.nextUrgentExpiryAt);
}
function familyBoardFooterArmExpiry(stamp){
  familyBoardFooterClearTimer();const at=Date.parse(String(stamp||""));if(!Number.isFinite(at)||at<=Date.now())return;
  FAMILY_BOARD_FOOTER.timer=setTimeout(()=>familyBoardFooterRefresh(),Math.min(Math.max(1000,at-Date.now()+500),36*60*60*1000));
}
async function familyBoardFooterRefresh(summary){
  try{const next=summary||await fetch("/api/family-board/summary",{cache:"no-store"}).then(r=>r.ok?r.json():null);familyBoardFooterRender(next||{});}catch(_){familyBoardFooterRender({});}
}
function familyBoardFooterReflow(){familyBoardFooterRender(FAMILY_BOARD_FOOTER.summary||{});}
function familyBoardFooterBoot(){
  if(FAMILY_BOARD_FOOTER.booted)return;FAMILY_BOARD_FOOTER.booted=true;const root=familyBoardFooterRoot();if(!root)return;
  bindTap(root,()=>{if(typeof window.openFamilyBoard==="function")window.openFamilyBoard().catch(()=>{});});
  window.addEventListener("resize",()=>{clearTimeout(FAMILY_BOARD_FOOTER.resizeTimer);FAMILY_BOARD_FOOTER.resizeTimer=setTimeout(familyBoardFooterReflow,180);},{passive:true});
  // One bounded local summary fetch gives the persisted board preference a
  // truthful bootstrap without loading the full app or a recurring feed.
  setTimeout(()=>familyBoardFooterRefresh(),1200);
}
window.familyBoardFooterRefresh=familyBoardFooterRefresh;
window.familyBoardFooterReflow=familyBoardFooterReflow;
window.familyBoardFooterBoot=familyBoardFooterBoot;
