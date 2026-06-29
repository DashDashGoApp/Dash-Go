#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import {fileURLToPath} from "node:url";

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const source=fs.readFileSync(path.join(root,"ui/js/shared-osk.js"),"utf8");

assert.match(source,/function oskRoot\(\)\{[\s\S]*?document\.querySelectorAll\("#osk"\)/,"OSK must resolve its body-mounted root with an ID selector");
assert.doesNotMatch(source,/\$\("osk"\)/,"OSK must not look for a nonexistent <osk> tag");
assert.doesNotMatch(source,/\$\("ctrlpage-content"\)/,"OSK fallback page lookup must use the Control content ID selector");
assert.match(source,/function hideOSK\(\)\{[\s\S]*?const osk=oskRoot\(\);[\s\S]*?osk\.classList\.remove\("show"\)/,"hideOSK must remove the visible state from the canonical keyboard root");
assert.match(source,/function buildOSK\(\)\{\s*let osk=oskRoot\(\);/,"OSK construction must reuse the canonical root instead of creating duplicate IDs");
assert.match(source,/document\.documentElement\.style\.removeProperty\("--osk-h"\)/,"hiding the OSK must release its reserved scroll-space measurement");
assert.match(source,/if\(ch==="hide"\) return hideOSK\(\);/,"the visible hide key must retain a direct hide action");

const removed=[];
const makeClassList=(initial=[])=>{
  const set=new Set(initial);
  return {set,remove(...names){for(const name of names)set.delete(name);},contains(name){return set.has(name);}};
};
const visible={classList:makeClassList(["show"]),parentNode:{removeChild(node){removed.push(node);}}};
const legacyDuplicate={classList:makeClassList(["show"]),parentNode:{removeChild(node){removed.push(node);}}};
const target={classList:makeClassList(["oskfocus"])};
const openPage={classList:makeClassList(["osk-open"])};
const cleared=[];
const requested=[];
const document={
  querySelectorAll(selector){
    requested.push(selector);
    if(selector==="#osk") return [visible,legacyDuplicate];
    if(selector===".ctrlpage.osk-open,#listsapp.osk-open,#chorewheel.osk-open,#familyboard.osk-open,#maintenance.osk-open,#routines.osk-open") return [openPage];
    return [];
  },
  documentElement:{style:{removeProperty(name){cleared.push(name);}}},
};
const context={document,__target:target};
vm.createContext(context);
vm.runInContext(source,context,{filename:"shared-osk.js"});
vm.runInContext("_oskTarget=__target; hideOSK();",context);

assert.deepEqual(requested,["#osk",".ctrlpage.osk-open,#listsapp.osk-open,#chorewheel.osk-open,#familyboard.osk-open,#maintenance.osk-open,#routines.osk-open"],"hide must resolve the keyboard by #osk before releasing all active overlay surfaces");
assert.equal(visible.classList.contains("show"),false,"hide must remove the visible keyboard class");
assert.equal(target.classList.contains("oskfocus"),false,"hide must clear the focused field affordance");
assert.equal(openPage.classList.contains("osk-open"),false,"hide must release Control, Lists, Chore Wheel, Family Board, Maintenance, and Routines scroll-room state");
assert.deepEqual(cleared,["--osk-h"],"hide must clear the keyboard-height custom property");
assert.deepEqual(removed,[legacyDuplicate],"a legacy duplicate keyboard root must be removed rather than left visible");

console.log("osk-hide smoke passed");
