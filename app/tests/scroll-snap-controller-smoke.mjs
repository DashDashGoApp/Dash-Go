#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import {fileURLToPath} from "node:url";

const app=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const source=fs.readFileSync(path.join(app,"ui/js/settings-idle-scroll.js"),"utf8");
assert.ok(source.split(/\n/).length<=400,"idle return controller must remain a focused source module");
for(const token of [
  "const DASH_SCROLL_RETURN_PREWARM_MS=400;",
  "const DASH_SCROLL_RETURN_VERIFY_MS=700;",
  "function dashboardScrollReturnCreate",
  "function scrollIdleReturnReconcile",
  "calendarWeekCullPrewarmAt(top,{before:2,after:2})",
  "root.scrollTop=top;",
  "Native smooth scrolling was interrupted or never started",
])assert.ok(source.includes(token),`idle return controller missing ${token}`);

function root(initial=0){
  const listeners=new Map();
  return {
    scrollTop:initial,
    listeners,
    addEventListener(name,fn,opts){listeners.set(name,{fn,opts});},
    removeEventListener(name){listeners.delete(name);},
    emit(name){listeners.get(name)?.fn();},
    // Simulate a WebKit smooth request that never moves; verification must
    // perform the bounded direct numeric fallback.
    scrollTo(){},
  };
}
const calendar=root(200),agenda=root(0),weather=root(0);
const button={classList:{toggle:()=>{}},addEventListener:()=>{}};
const timers=new Map(),raf=[];
const cull=[];
let timerId=0;
const context={
  CONFIG:{snapBackSeconds:35},
  $:selector=>selector==="#calscroll"?calendar:selector==="#snapback"?button:selector==="#agendalist"?agenda:selector==="#wx14"?weather:null,
  calendarWeekCullViewportHeight:()=>200,
  calendarWeekCullPrewarmAt:(top,options)=>{cull.push(["prewarm",top,options.before,options.after]);return true;},
  calendarWeekCullClearPrewarm:()=>cull.push(["clear"]),
  calendarWeekCullCommitAt:top=>cull.push(["commit",top]),
  Math,Number,Date,WeakMap,Map,Set,Object,console,
  setTimeout:(fn,ms)=>{const id=++timerId;timers.set(id,{fn,ms});return id;},
  clearTimeout:id=>timers.delete(id),
  requestAnimationFrame:fn=>{raf.push(fn);return raf.length;},
};
context.globalThis=context;
vm.createContext(context);
vm.runInContext(source,context);
const timerAt=ms=>[...timers.entries()].find(([,entry])=>entry.ms===ms);
const runTimer=entry=>{assert.ok(entry,`expected timer ${entry}`);const [id,value]=entry;timers.delete(id);value.fn();};
const flushRaf=()=>{while(raf.length){const fn=raf.shift();if(fn)fn();}};

vm.runInContext("setCalendarScrollHomeTop(1000);calendarScrollSnapReconcile();",context);
assert.ok(timerAt(34600),"far Calendar idle return must schedule a 400 ms destination prewarm");
assert.ok(timerAt(35000),"far Calendar idle return must retain the normal 35 second deadline");
runTimer(timerAt(34600));
assert.deepEqual(cull,[['prewarm',1000,2,2]],"prewarm must happen before the idle return starts");
runTimer(timerAt(35000));
flushRaf();
assert.equal(calendar.scrollTop,1000,"far idle return must commit the already-warm numeric home target");
assert.deepEqual(cull,[['prewarm',1000,2,2],['commit',1000],['clear']],"far return must commit then release the temporary destination window");

calendar.scrollTop=900;
vm.runInContext("calendarScrollSnapReconcile();",context);
runTimer(timerAt(35000));
assert.ok(timerAt(700),"near smooth return must retain a bounded verification fallback");
runTimer(timerAt(700));
flushRaf();
assert.equal(calendar.scrollTop,1000,"failed smooth return must fall back exactly once to numeric home");

calendar.scrollTop=200;
vm.runInContext("calendarScrollSnapReconcile();",context);
assert.ok(timers.size>0,"away Calendar must arm before user input");
calendar.emit("pointerdown");
assert.equal(timers.size,0,"real input must cancel pending prewarm/return work immediately");
assert.equal(calendar.listeners.get("scroll")?.opts?.passive,true,"raw return scroll listener must remain passive");

agenda.scrollTop=120;
vm.runInContext("scrollIdleReturnReconcile(agenda);",Object.assign(context,{agenda}));
assert.ok(timerAt(35000),"anchor restoration can explicitly rearm Agenda without relying on a browser scroll event");
console.log("PASS: shared idle return prewarms far Calendar homes, verifies smooth returns, respects user input, and re-arms known list roots explicitly");
