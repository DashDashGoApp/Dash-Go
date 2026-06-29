#!/usr/bin/env node
import assert from "node:assert/strict";
import {readFileSync} from "node:fs";
import {resolve} from "node:path";
import vm from "node:vm";

const root=resolve(process.argv[2]||".");
const source=readFileSync(resolve(root,"ui/js/control-calendars.js"),"utf8");

assert.match(source,/function ctrlCalendarVisibilityRoot\(\)\{\s*return document\.querySelector\("#ctrlpage-calendars #ctrlcals"\);\s*\}/,"Calendar Visibility must resolve its real semantic ID root");
assert.match(source,/async function renderCtrlCals\(\)\{\s*const row=ctrlCalendarVisibilityRoot\(\);\s*if\(!row\) return;/,"Calendar Visibility must guard an unavailable root instead of throwing after the accordion opens");
assert.doesNotMatch(source,/\$\("ctrlcals"\)/,"Calendar Visibility must not query a nonexistent <ctrlcals> tag through the CSS-selector helper");

const selectors=[];
const row={
  innerHTML:"",
  dataset:{},
  children:[],
  appendChild(node){this.children.push(node); return node;},
};
const context={
  document:{querySelector(selector){selectors.push(selector); return selector==="#ctrlpage-calendars #ctrlcals"?row:null;}},
  cachedApi:async(path,onData)=>{assert.equal(path,"/api/calendars"); onData([],false);},
  ctrlStateCard:(kind,title,detail,actions)=>({kind,title,detail,actions}),
  el:(tag,className,text)=>({tag,className,text,children:[],appendChild(node){this.children.push(node); return node;},append(...nodes){this.children.push(...nodes);}}),
  cbtn:(label,cls,handler)=>({label,cls,handler}),
  caction:(label,desc,cls,handler)=>({label,desc,cls,handler}),
  ctrlSetError:()=>{throw new Error("empty calendar payload must not become an error state");},
  friendlyUnavailable:(_,error)=>String(error||""),
};
vm.createContext(context);
vm.runInContext(source,context,{filename:"control-calendars.js"});
await vm.runInContext("renderCtrlCals()",context);
assert.deepEqual(selectors,["#ctrlpage-calendars #ctrlcals"],"Calendar Visibility must use the semantic root on its first lazy render");
assert.equal(row.innerHTML,"","Calendar Visibility must clear prior stale content before rendering fresh calendar controls");
assert.equal(row.children.length,2,"empty calendar state must still render recovery content and Calendar Manager actions");
assert.equal(row.children[0].kind,"empty","empty Calendar Visibility must remain visible rather than leaving a blank card");

let requestCount=0;
const missingRootContext={
  document:{querySelector(){return null;}},
  cachedApi:async()=>{requestCount++;},
};
vm.createContext(missingRootContext);
vm.runInContext(source,missingRootContext,{filename:"control-calendars.js"});
await vm.runInContext("renderCtrlCals()",missingRootContext);
assert.equal(requestCount,0,"missing Calendar Visibility markup must exit safely before requesting calendar data");

console.log("Calendar Visibility Control smoke: ok");
