#!/usr/bin/env node
// Behavioural proof for refresh-time scroll restoration.
//
// A background refresh rebuilds the agenda by clearing the list first, which
// clamps scrollTop to 0. If a gesture (tap, wheel, touch) arrives during that
// rebuild the element anchor is unusable — but the restore must still put the
// viewer back numerically, otherwise the list snaps to the top under their
// finger. This exercises the real module in a VM rather than matching source
// text, so a regression fails here instead of reaching the kiosk.
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import {fileURLToPath} from "node:url";

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const lifecyclePath=path.join(root,"ui/js/settings-scroll-lifecycle.js");
const source=process.env.DASHGO_LIFECYCLE_SOURCE
  ? fs.readFileSync(process.env.DASHGO_LIFECYCLE_SOURCE,"utf8")
  : fs.readFileSync(lifecyclePath,"utf8");
assert.ok(source.split(/\n/).length<=400,"scroll lifecycle must stay a focused split module");

function makeContext(){
  const listeners=new Map();
  const raf=[];
  const context={console,Math,Number,Array,Object,Date,WeakMap,
    requestAnimationFrame:fn=>{raf.push(fn);return raf.length;}};
  context.globalThis=context;
  vm.createContext(context);
  vm.runInContext(source,context);
  const run=code=>vm.runInContext(code,context);
  const flush=()=>{while(raf.length){const fn=raf.shift();if(fn)fn();}};
  const newRoot=()=>({
    isConnected:true,scrollTop:0,scrollHeight:2000,clientHeight:400,dataset:{},
    querySelectorAll:()=>[],
    getBoundingClientRect:()=>({top:0}),
    addEventListener:(name,fn)=>listeners.set(name,fn),
    removeEventListener:name=>listeners.delete(name)
  });
  const fire=name=>{const fn=listeners.get(name);assert.ok(fn,`expected a passive ${name} epoch listener`);fn();};
  return {context,run,flush,newRoot,fire,listeners};
}

// --- 1. No gesture: the viewer is put back exactly where they were ----------
{
  const {context,run,flush,newRoot}=makeContext();
  const r=newRoot();
  context.__r=r;
  r.scrollTop=600;
  run("globalThis.__snap=captureScrollAnchor(__r,'.row','rowKey')");
  r.scrollTop=0;                       // the renderer cleared the list
  run("restoreScrollAnchor(__r,__snap,'.row','rowKey')");
  flush();
  assert.equal(r.scrollTop,600,"an uninterrupted refresh must restore the reading position");
}

// --- 2. Gesture during the rebuild: still restored numerically --------------
// This is the regression: the previous build returned early here, leaving the
// list clamped at 0 (a visible snap to the top).
{
  const {context,run,flush,newRoot,fire}=makeContext();
  const r=newRoot();
  context.__r=r;
  r.scrollTop=600;
  run("globalThis.__snap=captureScrollAnchor(__r,'.row','rowKey')");
  fire("pointerdown");                  // finger lands while the refresh runs
  r.scrollTop=0;                        // the renderer cleared the list
  run("restoreScrollAnchor(__r,__snap,'.row','rowKey')");
  flush();
  assert.equal(r.scrollTop,600,"a refresh that lands mid-gesture must not snap the list to the top");
}

// --- 3. Gesture and the viewer moved the list themselves: hands off ---------
{
  const {context,run,flush,newRoot,fire}=makeContext();
  const r=newRoot();
  context.__r=r;
  r.scrollTop=600;
  run("globalThis.__snap=captureScrollAnchor(__r,'.row','rowKey')");
  fire("touchstart");
  fire("wheel");
  r.scrollTop=250;                      // the viewer scrolled during the rebuild
  run("restoreScrollAnchor(__r,__snap,'.row','rowKey')");
  flush();
  assert.equal(r.scrollTop,250,"an in-flight gesture owns the position; the restore must not yank it back");
}

// --- 4. A detached root is still abandoned ---------------------------------
{
  const {context,run,flush,newRoot}=makeContext();
  const r=newRoot();
  context.__r=r;
  r.scrollTop=600;
  run("globalThis.__snap=captureScrollAnchor(__r,'.row','rowKey')");
  r.isConnected=false;
  run("restoreScrollAnchor(__r,__snap,'.row','rowKey')");
  flush();
  assert.equal(r.scrollTop,600,"a detached root must be left alone");
}

console.log("PASS: refresh-time scroll restore survives an interleaved gesture without snapping to the top");
