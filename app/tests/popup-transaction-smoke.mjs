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
const context=vm.createContext({
  console,
  document:{addEventListener(){},getElementById:id=>nodes.get(id)||null,querySelector:sel=>nodes.get(String(sel).replace(/^#/,""))||null},
  requestAnimationFrame:fn=>{frames.push(fn);return frames.length;},
  setTimeout,clearTimeout,
  $:sel=>nodes.get(String(sel).replace(/^#/,""))||null,
  el:(tag,cls,txt)=>{const n=node();n.tagName=tag;n.className=cls||"";if(txt!=null)n.textContent=String(txt);return n;},
  bindTap(){},pauseUiAnimations(){},resumeUiAfterOverlay(){},releaseMessagePopupRotationPause(){},mapFullIsOpen:()=>false,noteMapFullInput(){}
});
vm.runInContext(source+"\nglobalThis.__popup={popupOpenTransaction,popupDefer,closeScrim,popupIsCurrent};",context,{filename:"popup-transaction-source.js"});
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
console.log("PASS: popup shell paints first and stale/deferred popup work cannot commit");
