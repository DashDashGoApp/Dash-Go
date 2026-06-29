#!/usr/bin/env node
// Regression guard for Dashboard Control card headers. Native <summary>
// controls toggle their parent <details> as the default click action. The
// shared tap helper also commits a kiosk pointer stream on pointerup, so the
// immediately following synthetic click must be prevented—not merely ignored—
// or every card opens then instantly closes.
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import {fileURLToPath} from "node:url";

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const read=rel=>fs.readFileSync(path.join(root,rel),"utf8");
const tap=read("ui/js/tap.js");
const navigation=read("ui/js/control-navigation.js");

assert.match(navigation,/function bindCtrlSummaryTaps\(\)/,
  "Control must bind card summaries through the shared tap primitive");
assert.match(navigation,/bindTap\(s,[\s\S]*?\{preventDefault:true\}\)/,
  "Control summary bindings must own native details toggling explicitly");
assert.match(tap,/const duplicate=Date\.now\(\)<suppressClickUntil;/,
  "shared tap helper must suppress every follow-up click after an accepted pointer release");
assert.doesNotMatch(tap,/keyboardLike=Number\(e\.detail\)===0/,
  "detail zero is not a reliable Surf/WebKitGTK keyboard distinction after touch release");
assert.match(tap,/if\(opts\.preventDefault\s*&&\s*e\.cancelable\)\s*e\.preventDefault\(\);/,
  "preventDefault bindings must cancel native click default even when duplicate activation is suppressed");

class Target{
  constructor(){this.listeners=new Map();this.rect={width:480,height:62};this.tagName="SUMMARY";this.parentElement={tagName:"DETAILS",open:false};}
  addEventListener(type,fn){const list=this.listeners.get(type)||[];list.push(fn);this.listeners.set(type,list);}
  removeEventListener(type,fn){this.listeners.set(type,(this.listeners.get(type)||[]).filter(item=>item!==fn));}
  getBoundingClientRect(){return this.rect;}
  dispatch(type,extra={}){
    const event={target:this,currentTarget:this,cancelable:true,button:0,isPrimary:true,clientX:240,clientY:31,
      defaultPrevented:false,preventDefault(){this.defaultPrevented=true;},stopPropagation(){},...extra};
    for(const fn of this.listeners.get(type)||[])fn(event);
    return event;
  }
}

let now=0;
const document={hidden:false,addEventListener(){},removeEventListener(){}};
const context={window:{PointerEvent:function PointerEvent(){}},document,Date:{now:()=>now},setTimeout(fn){fn();return 1;},clearTimeout(){}};
vm.createContext(context);
vm.runInContext(tap,context,{filename:"tap.js"});

const summary=new Target();
context.bindTap(summary,()=>{summary.parentElement.open=!summary.parentElement.open;},{preventDefault:true});

// Surf/WebKitGTK may deliver a primary mouse-compatible pointer stream followed
// by a click. Pointerup opens the card; the synthetic click must not run the
// native <summary> default action and immediately close it again.
now+=10;
summary.dispatch("pointerdown",{pointerType:"mouse"});
now+=12;
const release=summary.dispatch("pointerup",{pointerType:"mouse"});
assert.equal(release.defaultPrevented,true,"summary pointer release must suppress its native toggle path");
assert.equal(summary.parentElement.open,true,"pointer release must open the selected Control card");
const syntheticClick=summary.dispatch("click",{pointerType:"mouse",detail:1});
if(!syntheticClick.defaultPrevented) summary.parentElement.open=!summary.parentElement.open;
assert.equal(syntheticClick.defaultPrevented,true,"suppressed synthetic click must still cancel native summary default");
assert.equal(summary.parentElement.open,true,"synthetic click must not re-close the card that pointerup opened");

// Surf may report the pointer follow-up click with detail 0. It must be
// suppressed exactly like a detail-1 synthetic click, then a true later
// keyboard/programmatic click remains usable after the duplicate window.
now+=1;
const detailZeroClick=summary.dispatch("click",{detail:0});
if(!detailZeroClick.defaultPrevented) summary.parentElement.open=!summary.parentElement.open;
assert.equal(detailZeroClick.defaultPrevented,true,"detail-zero follow-up must cancel native summary default");
assert.equal(summary.parentElement.open,true,"detail-zero follow-up must not re-close the card");
now+=701;
const keyboardClick=summary.dispatch("click",{detail:0});
if(!keyboardClick.defaultPrevented) summary.parentElement.open=!summary.parentElement.open;
assert.equal(keyboardClick.defaultPrevented,true,"later keyboard click must still own native summary behavior");
assert.equal(summary.parentElement.open,false,"keyboard click outside the pointer window must toggle once");

console.log("PASS: Control card summaries suppress detail-zero touch duplicates and retain later keyboard activation");
