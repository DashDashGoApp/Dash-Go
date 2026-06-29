#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import {fileURLToPath} from "node:url";

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const source=fs.readFileSync(path.join(root,"ui/js/tap.js"),"utf8");

class FakeTarget{
  constructor(width,height){this.rect={width,height};this.listeners=new Map();}
  addEventListener(type,fn){const items=this.listeners.get(type)||[];items.push(fn);this.listeners.set(type,items);}
  removeEventListener(type,fn){this.listeners.set(type,(this.listeners.get(type)||[]).filter(item=>item!==fn));}
  getBoundingClientRect(){return this.rect;}
  dispatch(type,event){for(const fn of this.listeners.get(type)||[])fn({target:this,cancelable:true,preventDefault(){},...event});}
}
function makeHarness(width,height){
  let now=0;
  const document={hidden:false,addEventListener(){},removeEventListener(){}};
  const sandbox={
    window:{PointerEvent:function PointerEvent(){}}, document,
    Date:{now:()=>now}, setTimeout(fn){fn();return 1;}, clearTimeout(){},
  };
  vm.createContext(sandbox);
  vm.runInContext(`${source}\nglobalThis.__bindTripleTap=bindTripleTap;`,sandbox);
  const target=new FakeTarget(width,height);
  const tap=(x,y,advance=90)=>{
    now+=advance;
    target.dispatch("pointerdown",{pointerType:"touch",clientX:x,clientY:y});
    now+=8;
    target.dispatch("pointerup",{pointerType:"touch",clientX:x,clientY:y});
  };
  const swipe=(fromX,toX,y,advance=90)=>{
    now+=advance;
    target.dispatch("pointerdown",{pointerType:"touch",clientX:fromX,clientY:y});
    now+=8;
    target.dispatch("pointerup",{pointerType:"touch",clientX:toX,clientY:y});
  };
  return {target,tap,swipe,bind:sandbox.__bindTripleTap};
}

// A Check soon pill can be hundreds of pixels wide. Natural taps distributed
// across that one button must still produce one triple-tap gesture.
{
  const h=makeHarness(320,44); let fired=0;
  h.bind(h.target,()=>{fired++;},700,{moveTol:28});
  h.tap(18,22); h.tap(150,22); h.tap(298,22);
  assert.equal(fired,1,"wide warning pill must accept three taps across its surface");
}

// A delayed second tap starts a new sequence, even on a wide target.
{
  const h=makeHarness(320,44); let fired=0;
  h.bind(h.target,()=>{fired++;},700,{moveTol:28});
  h.tap(18,22); h.tap(150,22,760); h.tap(298,22,90);
  assert.equal(fired,0,"taps outside the gap must not form a triple-tap");
}

// The original tight move tolerance remains responsible for swipe rejection.
{
  const h=makeHarness(320,44); let fired=0;
  h.bind(h.target,()=>{fired++;},700,{moveTol:28});
  h.swipe(12,92,22);
  assert.equal(fired,0,"a moved press/release must not count as a tap");
}

// Existing small controls retain ordinary triple-tap behavior.
{
  const h=makeHarness(84,84); let fired=0;
  h.bind(h.target,()=>{fired++;},700,{});
  h.tap(8,42); h.tap(42,42); h.tap(76,42);
  assert.equal(fired,1,"small moon-style target must retain triple-tap behavior");
}

console.log("PASS: inter-tap clustering follows the bound control footprint while intra-tap swipe rejection stays tight");
