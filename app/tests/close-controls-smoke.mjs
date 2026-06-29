#!/usr/bin/env node
// Regression guard for kiosk close controls. Surf/WebKitGTK can emit a
// mouse-compatible PointerEvent for a touchscreen release without delivering a
// dependable synthetic click afterward. Every close control shares bindTap, so
// test the primitive through the real popup close binding and keep all shell
// close controls statically connected to it.
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import {fileURLToPath} from "node:url";

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const read=rel=>fs.readFileSync(path.join(root,rel),"utf8");
const tap=read("ui/js/tap.js");
const popup=read("ui/js/popup-overlays.js");
const boot=read("ui/js/boot.js");
const map=read("ui/js/map-interactive.js");
const radar=read("ui/js/radar-overlay.js");
const launcher=read("ui/js/app-launcher.js");
const lists=read("ui/lists-core.js")+"\n"+read("ui/lists-actions.js");
const chore=read("ui/chore-wheel.js");

assert.match(tap,/Commit every valid primary PointerEvent on release/,
  "tap primitive must activate mouse-compatible PointerEvents on pointerup");
assert.match(tap,/suppressClickUntil=Date\.now\(\)\+700/,
  "tap primitive must suppress only the immediate synthetic duplicate");
assert.match(tap,/pointercancel/,
  "tap primitive must reset a cancelled pointer stream");
assert.match(tap,/lostpointercapture/,
  "tap primitive must also reset a lost pointer capture stream");
assert.match(tap,/const duplicate=Date\.now\(\)<suppressClickUntil;/,
  "all click follow-ups inside the accepted pointer window must be suppressed");
assert.doesNotMatch(tap,/keyboardLike=Number\(e\.detail\)===0/,
  "detail zero is unsafe as a touch/keyboard discriminator after Surf pointer release");
assert.match(boot,/bindTap\(\$\("#ctrlclose"\),closeCtrl\)/,
  "Dashboard Control close must use the hardened shared primitive");
assert.match(popup,/bindTap\(\$\("#popclose"\),closeScrim\)/,
  "popup close must use the hardened shared primitive");
assert.match(map,/bindTap\(\$\("#mapclose"\),closeInteractiveMap\)/,
  "map close must use the hardened shared primitive");
assert.match(radar,/bindTap\(document\.getElementById\("radarclose"\),closeRadar\)/,
  "radar close must use the hardened shared primitive");
assert.match(launcher,/if\(close\) bindTap\(close,\(\)=>closeAppLauncher\(\)\)/,
  "Apps close must use the hardened shared primitive");
assert.match(lists,/if\(close\)bindTap\(close,closeLists\)/,
  "Lists close must use the hardened shared primitive");
assert.match(chore,/if\(close\)bindTap\(close,closeChoreWheel\)/,
  "Chore Wheel close must use the hardened shared primitive");
assert.match(chore,/bindTap\(shell,closeChoreWheel,\{ignore:event=>event\.target!==shell\}\)/,
  "Chore Wheel backdrop must use the shared primitive without intercepting child input focus");

class ClassList{
  constructor(values=[]){this.values=new Set(values);}
  add(...values){values.forEach(value=>this.values.add(value));}
  remove(...values){values.forEach(value=>this.values.delete(value));}
  contains(value){return this.values.has(value);}
}
class Target{
  constructor(id){this.id=id;this.classList=new ClassList();this.listeners=new Map();this.rect={width:52,height:52};}
  addEventListener(type,fn){const list=this.listeners.get(type)||[];list.push(fn);this.listeners.set(type,list);}
  removeEventListener(type,fn){this.listeners.set(type,(this.listeners.get(type)||[]).filter(item=>item!==fn));}
  getBoundingClientRect(){return this.rect;}
  dispatch(type,event={}){
    for(const fn of this.listeners.get(type)||[]){
      fn({target:this,currentTarget:this,cancelable:true,button:0,isPrimary:true,clientX:26,clientY:26,
        preventDefault(){},stopPropagation(){},...event});
    }
  }
}

let now=0,resumeCalls=0;
const nodes={scrim:new Target("scrim"),ctrl:new Target("ctrl"),pop:new Target("pop"),popclose:new Target("popclose"),applauncher:new Target("applauncher")};
nodes.scrim.classList.add("show");
const document={
  hidden:false,
  addEventListener(){},removeEventListener(){},
  getElementById(id){return nodes[id]||null;},
  querySelector(selector){return selector.startsWith("#")?nodes[selector.slice(1)]||null:null;},
};
const context={
  window:{PointerEvent:function PointerEvent(){}},document,
  Date:{now:()=>now},Set,console,
  setTimeout(fn){fn();return 1;},clearTimeout(){},requestAnimationFrame(fn){fn();return 1;},
  mapFullIsOpen:()=>false,noteMapFullInput(){},pauseUiAnimations(){},
  resumeUiAfterOverlay(){resumeCalls++;},
  releaseMessagePopupRotationPause(){},
  $:selector=>document.querySelector(selector),el(){return new Target("generated");},
};
vm.createContext(context);
vm.runInContext(tap,context,{filename:"tap.js"});
vm.runInContext(popup,context,{filename:"popup-overlays.js"});

// This is the missed device path: primary mouse-compatible PointerEvents, no
// click event. It must still dismiss the popup on pointer release.
now+=10;
nodes.popclose.dispatch("pointerdown",{pointerType:"mouse"});
now+=12;
nodes.popclose.dispatch("pointerup",{pointerType:"mouse"});
assert.equal(nodes.scrim.classList.contains("show"),false,
  "mouse-compatible pointerup must dismiss without a follow-up click");
assert.equal(resumeCalls,1,"pointer release must run close lifecycle once");

// A browser may still emit a synthetic click. It must not perform a duplicate
// close/resume cycle after the pointer-release action.
nodes.scrim.classList.add("show");
now+=20;
nodes.popclose.dispatch("click",{pointerType:"mouse"});
assert.equal(nodes.scrim.classList.contains("show"),true,
  "immediate synthetic click must be suppressed after pointer release");
assert.equal(resumeCalls,1,"synthetic duplicate must not repeat close lifecycle");

// Surf can label the synthetic touch click as detail 0. It must still be
// suppressed while the accepted pointer-release window is active.
now+=1;
nodes.popclose.dispatch("click",{detail:0});
assert.equal(nodes.scrim.classList.contains("show"),true,
  "detail-zero touch follow-up must not repeat close activation");
assert.equal(resumeCalls,1,"detail-zero follow-up must not repeat close lifecycle");
now+=701;
nodes.popclose.dispatch("click",{detail:0});
assert.equal(nodes.scrim.classList.contains("show"),false,
  "keyboard/programmatic click must remain available after the pointer window");
assert.equal(resumeCalls,2,"later keyboard click must run close lifecycle once");

console.log("PASS: shared close controls survive mouse-compatible kiosk pointer streams and suppress detail-zero synthetic duplicates");
