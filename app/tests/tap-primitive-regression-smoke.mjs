#!/usr/bin/env node
// Beta.16 guard for shared pointer-release activation. A real primary
// pointerup must remain authoritative on Surf/WebKitGTK, but disabled controls,
// keyboard/programmatic clicks, and cancelled multi-tap sequences need explicit
// state-machine boundaries.
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import {fileURLToPath} from "node:url";

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const tap=fs.readFileSync(path.join(root,"ui/js/tap.js"),"utf8");

assert.match(tap,/const isTapDisabled=node=>/,"shared primitive must centralize disabled/ARIA-disabled eligibility");
assert.match(tap,/node\.getAttribute\?\.\("aria-disabled"\)===\"true\"/,"ARIA-disabled controls must not activate through pointer release");
assert.match(tap,/const duplicate=Date\.now\(\)<suppressClickUntil;/,"every click in a completed pointer-release window must be suppressed");
assert.doesNotMatch(tap,/keyboardLike=Number\(e\.detail\)===0/,"detail zero is unsafe as a touch/keyboard discriminator on Surf/WebKitGTK");
assert.match(tap,/lostpointercapture/,"lost pointer capture must cancel partial tap state");
assert.match(tap,/const cancelGesture=\(\)=>\{[\s\S]*?reset\(\);[\s\S]*?suppressClickUntil=0;/,"cancellation must clear partial taps and duplicate suppression");

class Target{
  constructor(){
    this.listeners=new Map();
    this.rect={width:140,height:52};
    this.disabled=false;
    this.attrs=new Map();
  }
  addEventListener(type,fn){
    const list=this.listeners.get(type)||[];
    list.push(fn);
    this.listeners.set(type,list);
  }
  removeEventListener(type,fn){
    this.listeners.set(type,(this.listeners.get(type)||[]).filter(item=>item!==fn));
  }
  getBoundingClientRect(){return this.rect;}
  getAttribute(name){return this.attrs.has(name)?this.attrs.get(name):null;}
  setAttribute(name,value){this.attrs.set(name,String(value));}
  matches(selector){return selector===":disabled" && this.disabled;}
  dispatch(type,extra={}){
    const event={
      target:this,currentTarget:this,cancelable:true,button:0,isPrimary:true,
      clientX:70,clientY:26,detail:1,
      defaultPrevented:false,
      preventDefault(){this.defaultPrevented=true;},
      stopPropagation(){},
      ...extra,
    };
    for(const fn of this.listeners.get(type)||[]) fn(event);
    return event;
  }
}

let now=0;
const documentListeners=new Map();
const document={
  hidden:false,
  addEventListener(type,fn){const list=documentListeners.get(type)||[];list.push(fn);documentListeners.set(type,list);},
  removeEventListener(type,fn){documentListeners.set(type,(documentListeners.get(type)||[]).filter(item=>item!==fn));},
};
const context={
  window:{PointerEvent:function PointerEvent(){}},
  document,
  Date:{now:()=>now},
  Math,Number,Set,console,
  setTimeout(fn){fn();return 1;},clearTimeout(){},
};
vm.createContext(context);
vm.runInContext(tap,context,{filename:"tap.js"});

const pointerTap=(target,{type="mouse"}={})=>{
  now+=10;
  target.dispatch("pointerdown",{pointerType:type});
  now+=12;
  target.dispatch("pointerup",{pointerType:type});
};

// Disabled and ARIA-disabled controls must reject direct pointer-release work,
// then become eligible immediately when their actual disabled contract clears.
let disabledCalls=0;
const disabledButton=new Target();
context.bindTap(disabledButton,()=>{disabledCalls++;});
disabledButton.disabled=true;
pointerTap(disabledButton);
assert.equal(disabledCalls,0,"a disabled button must not run a pointer-release callback");
disabledButton.disabled=false;
disabledButton.setAttribute("aria-disabled","true");
pointerTap(disabledButton);
assert.equal(disabledCalls,0,"an ARIA-disabled button must not run a pointer-release callback");
disabledButton.attrs.delete("aria-disabled");
pointerTap(disabledButton);
assert.equal(disabledCalls,1,"an enabled button must resume normal pointer-release activation");

// A delegated backdrop binding must ignore child fields before pointer-release
// default prevention. Otherwise a touch release on a native input can be
// consumed before the browser focuses it and the OSK never has a target.
let backdropCalls=0;
const backdrop=new Target();
const childInput={tagName:"INPUT"};
context.bindTap(backdrop,()=>{backdropCalls++;},{ignore:event=>event.target!==backdrop});
now+=10;
backdrop.dispatch("pointerdown",{pointerType:"touch",target:childInput});
now+=12;
const childRelease=backdrop.dispatch("pointerup",{pointerType:"touch",target:childInput});
assert.equal(backdropCalls,0,"a delegated backdrop must not activate for an input child");
assert.equal(childRelease.defaultPrevented,false,"an ignored input release must keep its native focus default");

// Synthetic clicks remain suppressed even when Surf reports detail 0. A
// keyboard/programmatic click remains available as soon as no pointer window is pending.
let clickCalls=0;
const action=new Target();
context.bindTap(action,()=>{clickCalls++;});
pointerTap(action);
assert.equal(clickCalls,1,"pointerup must commit the primary activation");
// A post-release lostpointercapture is a normal cleanup signal on some touch
// stacks. It must not erase the duplicate guard for an already committed tap.
action.dispatch("lostpointercapture",{pointerType:"mouse"});
action.dispatch("click",{detail:1,pointerType:"mouse"});
assert.equal(clickCalls,1,"post-release lostpointercapture must not reopen the pointer-like duplicate path");
action.dispatch("click",{detail:0});
assert.equal(clickCalls,1,"detail-zero touch follow-up must remain inside the duplicate window");
now+=701;
action.dispatch("click",{detail:0});
assert.equal(clickCalls,2,"keyboard/programmatic click must remain available after the pointer window");

// A cancelled continuation cannot carry an old partial triple sequence into a
// later gesture. Test both normal cancellation and a lost pointer capture.
const tripleSequence=(cancelType)=>{
  let triples=0;
  const target=new Target();
  context.bindTripleTap(target,()=>{triples++;},650);
  pointerTap(target);                    // count = 1
  now+=10;
  target.dispatch("pointerdown",{pointerType:"mouse"});
  now+=10;
  target.dispatch(cancelType,{pointerType:"mouse"}); // must reset count
  pointerTap(target);
  pointerTap(target);
  assert.equal(triples,0,`${cancelType} must prevent old partial taps from completing a triple action`);
  pointerTap(target);
  assert.equal(triples,1,`three fresh taps after ${cancelType} must invoke once`);
};
tripleSequence("pointercancel");
tripleSequence("lostpointercapture");

assert.equal((documentListeners.get("visibilitychange")||[]).length,1,"all bindTap/bindTripleTap controls must share one document visibility listener");
assert.equal((tap.match(/document\.addEventListener\("visibilitychange"/g)||[]).length,1,"source must not create one visibility listener per bound control");
console.log("PASS: pointer-release primitive respects disabled controls, child input focus defaults, detail-zero touch duplicates, cancelled multi-tap state, and shared listener cleanup");
