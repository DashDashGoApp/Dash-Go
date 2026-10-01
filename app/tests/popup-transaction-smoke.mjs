#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import {fileURLToPath} from "node:url";

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const source=fs.readFileSync(path.join(root,"ui/js/popup-overlays.js"),"utf8");
function classes(){const set=new Set();return {add:(...v)=>v.forEach(x=>set.add(x)),remove:(...v)=>v.forEach(x=>set.delete(x)),contains:v=>set.has(v),toggle:(v,on)=>{if(on===undefined)on=!set.has(v);on?set.add(v):set.delete(v);return on;}};}
function node(id=""){return {id,className:"",textContent:"",children:[],classList:classes(),style:{},attributes:{},setAttribute(k,v){this.attributes[k]=String(v);},replaceChildren(...kids){this.children=kids;this.textContent="";},appendChild(kid){this.children.push(kid);return kid;},addEventListener(){}};}
const nodes=new Map([["scrim",node("scrim")],["ctrl",node("ctrl")],["pop",node("pop")],["poptitle",node("poptitle")],["popwhen",node("popwhen")],["popbody",node("popbody")],["popclose",node("popclose")]]);
const frames=[];
let clock=0;
const context=vm.createContext({
  console,
  dashboardLiteProfile:()=>true,
  performance:{now:()=>clock,mark(){}},
  document:{addEventListener(){},getElementById:id=>nodes.get(id)||null,querySelector:sel=>nodes.get(String(sel).replace(/^#/,""))||null},
  requestAnimationFrame:fn=>{frames.push(fn);return frames.length;},
  setTimeout,clearTimeout,
  $:sel=>nodes.get(String(sel).replace(/^#/,""))||null,
  el:(tag,cls,txt)=>{const n=node();n.tagName=tag;n.className=cls||"";if(txt!=null)n.textContent=String(txt);return n;},
  bindTap(){},pauseUiAnimations(){},resumeUiAfterOverlay(){},releaseMessagePopupRotationPause(){},mapFullIsOpen:()=>false,noteMapFullInput(){}
});
vm.runInContext(source+"\nglobalThis.__popup={popupOpenTransaction,popupDefer,closeScrim,popupIsCurrent,popupRegisterLiveRefresh,popupNotifyDataCommit,popupTimingEnabled,popupTimingMark,popupTimingRecent,popupTimingOverBudget};",context,{filename:"popup-transaction-source.js"});
function flush(){while(frames.length)frames.shift()();}
const popup=context.__popup;
let oldBuilds=0,newBuilds=0;
const oldToken=popup.popupOpenTransaction({mode:"eventpop",title:"Old",loading:"Old loading"},()=>{oldBuilds++;return node("old");});
assert.equal(nodes.get("scrim").classList.contains("show"),true,"popup shell opens before body work");
assert.equal(oldBuilds,0,"body builder waits one frame");
assert.equal(nodes.get("popbody").children[0].className,"popup-skeleton","loading skeleton is visible immediately");
popup.popupOpenTransaction({mode:"eventpop",title:"New",loading:"New loading"},()=>{newBuilds++;const n=node("new");n.textContent="new body";return n;});
flush();
assert.equal(oldBuilds,0,"superseded popup never builds into a newer popup");
assert.equal(newBuilds,1,"current popup builds after shell paint");
assert.equal(nodes.get("popbody").children[0].id,"new","only the current popup body commits");
let cancelled=0;
popup.popupDefer(oldToken+1,task=>{task.onCancel(()=>cancelled++);task.onCancel(()=>cancelled++);});
flush();
popup.closeScrim();
assert.equal(cancelled,2,"popup close runs every deferred cancellation hook");
assert.equal(nodes.get("scrim").classList.contains("show"),false,"close invalidates and hides the popup");

// --- live refresh registry --------------------------------------------------
// An open popup registers one refresher for its token; a calendar commit
// (popupNotifyDataCommit) reaches it, and the registration dies with the token so
// a superseded or closed popup can never be refreshed.
let refreshes=0;
let dayBuilds=0;
const dayToken=popup.popupOpenTransaction({mode:"daytimelinepop",title:"Day",loading:"Day"},()=>{dayBuilds++;return node("day");});
flush();
assert.equal(dayBuilds,1,"day popup body still builds after the shell");
assert.equal(popup.popupRegisterLiveRefresh(dayToken,()=>{refreshes++;return true;}),true,"an open popup can register a live refresher");
assert.equal(popup.popupNotifyDataCommit(),true,"a data commit reaches the open popup");
assert.equal(refreshes,1,"the refresher ran exactly once");
popup.popupRegisterLiveRefresh(dayToken,()=>false);
assert.equal(popup.popupNotifyDataCommit(),false,"a refresh with nothing to do reports no work");

// A newer popup supersedes the registration (the same invalidation that cancels
// deferred work clears it).
popup.popupRegisterLiveRefresh(dayToken,()=>{refreshes++;return true;});
popup.popupOpenTransaction({mode:"eventpop",title:"Newer",loading:"Newer"},()=>node("newer"));
flush();
assert.equal(popup.popupNotifyDataCommit(),false,"a superseded popup no longer receives commits");
assert.equal(refreshes,1,"a superseded refresher must not run");

// Closing the popup clears it as well.
const liveToken=popup.popupOpenTransaction({mode:"daytimelinepop",title:"Live",loading:"Live"},()=>node("live"));
flush();
popup.popupRegisterLiveRefresh(liveToken,()=>{refreshes++;return true;});
popup.closeScrim();
assert.equal(popup.popupNotifyDataCommit(),false,"a closed popup no longer receives commits");
assert.equal(refreshes,1,"a refresher must not run after the popup closed");

// A refresher that throws must not take the open popup down with it.
const safeToken=popup.popupOpenTransaction({mode:"daytimelinepop",title:"Safe",loading:"Safe"},()=>node("safe"));
flush();
const realWarn=console.warn;
console.warn=()=>{};                   // the containment warning is expected here
popup.popupRegisterLiveRefresh(safeToken,()=>{throw new Error("boom");});
assert.equal(popup.popupNotifyDataCommit(),false,"a failing refresher is contained");
console.warn=realWarn;
assert.equal(nodes.get("scrim").classList.contains("show"),true,"the popup survives a failing refresh");

// --- latency marks ----------------------------------------------------------
// The marks exist so the shell/content budget can be proven on the device rather
// than guessed, and the ring is bounded because the kiosk never reloads.
assert.equal(popup.popupTimingEnabled(),true,"Lite with a clock enables popup timing");
clock=0;
popup.popupOpenTransaction({mode:"eventpop",title:"Fast",loading:"Fast"},()=>node("fast"));
clock=40;
flush();
let recent=popup.popupTimingRecent();
assert.equal(recent[recent.length-2].hop,"shell","the shell milestone is recorded when the scrim paints");
assert.equal(recent[recent.length-2].ms,0,"the shell is recorded at the open, before any body work");
assert.equal(recent[recent.length-1].hop,"content","the content milestone is recorded after the body commits");
assert.equal(recent[recent.length-1].ms,40,"content is measured from the open");
assert.equal(popup.popupTimingOverBudget().length,0,"an open inside its budget reports nothing over budget");

clock=1000;
popup.popupOpenTransaction({mode:"eventpop",title:"Slow",loading:"Slow"},()=>node("slow"));
clock=1400;
flush();
const over=popup.popupTimingOverBudget();
assert.ok(over.some(entry=>entry.hop==="content"),"a body build past 250ms is reported over budget");
assert.ok(over.every(entry=>entry.ms>entry.budget),"only genuinely over-budget milestones are reported");
for(let i=0;i<12;i++)popup.popupTimingMark("settled");
assert.ok(popup.popupTimingRecent().length<=8,"the mark ring stays bounded on a kiosk that never reloads");

// Close the harness. An open overlay arms the 3-minute idle auto-close timer,
// which would otherwise keep the test process alive long after it has passed.
popup.closeScrim();
console.log("PASS: popup shell paints first and stale/deferred popup work cannot commit");
