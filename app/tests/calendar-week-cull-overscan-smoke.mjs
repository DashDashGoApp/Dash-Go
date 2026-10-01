#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import {fileURLToPath} from "node:url";

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const source=fs.readFileSync(path.join(root,"ui/js/calendar-cull-overscan.js"),"utf8");
const css=fs.readFileSync(path.join(root,"ui/css/dashboard/lite-dashboard.css"),"utf8");

assert.ok(source.split(/\n/).length<=400,"Calendar culling controller must stay a focused split module");
assert.match(source,/const CALENDAR_WEEK_CULL_WARM_MS=250;/,"calendar culling must retain a short post-scroll warm hold");
assert.match(source,/const CALENDAR_WEEK_CULL_BEHIND_OVERSCAN=1;/,"calendar culling must keep one row behind travel");
assert.match(source,/const CALENDAR_WEEK_CULL_AHEAD_OVERSCAN=2;/,"calendar culling must prewarm two rows ahead of travel");
assert.match(source,/function calendarWeekCullPrewarmAt\(top,options\)/,"calendar culling must support a bounded destination prewarm");
assert.match(source,/function calendarWeekCullCommitAt\(top\)/,"calendar culling must settle a direct return using cached geometry");
assert.match(source,/scroll\.addEventListener\("scroll",state\.onScroll,\{passive:true\}\)/,"culling scroll listener must remain passive");
const scrollHandler=source.match(/function calendarWeekCullOnScroll\(\)\{[\s\S]*?\n\}/)?.[0]||"";
assert.ok(scrollHandler,"calendar culling needs one throttled scroll handler");
assert.doesNotMatch(scrollHandler,/getBoundingClientRect|clientHeight|scrollHeight|offsetTop|offsetHeight/,
  "raw scroll handling must use cached geometry instead of forcing layout reads");
assert.match(css,/\.weekrow\[data-cull-near="1"\]\{[\s\S]*?content-visibility:visible;/,
  "warm rows must override automatic content visibility on Lite");

function row(top){return {offsetTop:top,offsetHeight:100,dataset:{}};}
const rows=Array.from({length:12},(_,i)=>row(i*100));
const listeners=new Map();
const timers=new Map();
const raf=[];
let timerId=0;
let scheduleCount=0;
const scroll={
  dataset:{},scrollTop:150,clientHeight:200,
  querySelectorAll:selector=>selector===".weekrow"?rows:[],
  addEventListener:(name,fn,opts)=>listeners.set(name,{fn,opts}),
  removeEventListener:name=>listeners.delete(name)
};
const context={
  console,Math,Number,Array,Set,Object,
  liteVisualProfile:()=>true,
  $:()=>scroll,
  requestAnimationFrame:fn=>{raf.push(fn);return raf.length;},
  cancelAnimationFrame:id=>{raf[id-1]=null;},
  setTimeout:(fn,ms)=>{scheduleCount++;const id=++timerId;timers.set(id,{fn,ms});return id;},
  clearTimeout:id=>timers.delete(id)
};
context.globalThis=context;
vm.createContext(context);
vm.runInContext(source,context);
const flushRaf=()=>{while(raf.length){const fn=raf.shift();if(fn)fn();}};
const near=()=>rows.map((r,i)=>r.dataset.cullNear==="1"?i:null).filter(i=>i!==null);
const activeTimer=ms=>[...timers.entries()].find(([,timer])=>timer.ms===ms);
const runTimer=entry=>{assert.ok(entry,"expected scheduled timer");const [id,timer]=entry;timers.delete(id);timer.fn();};

context.scroll=scroll;
vm.runInContext("calendarSetWeekCullReady(true,scroll)",context);
assert.deepEqual(near(),[0,1,2,3,4],"idle culling must retain one row before and after the visible window");
assert.equal(listeners.get("scroll")?.opts?.passive,true,"scroll listener must be passive");
assert.equal(vm.runInContext("calendarWeekCullPrewarmAt(900,{before:2,after:2})",context),true,"a far home target must be prewarmed from cached row geometry");
assert.deepEqual(near(),[0,1,2,3,4,7,8,9,10,11],"destination prewarm must retain the current and future windows without rendering the gap");
vm.runInContext("calendarWeekCullClearPrewarm()",context);
assert.deepEqual(near(),[0,1,2,3,4],"clearing a prewarm must return to the active window before normal scrolling resumes");

scroll.scrollTop=260;
listeners.get("scroll").fn();
flushRaf();
assert.deepEqual(near(),[1,2,3,4,5,6],"downward travel must retain one row behind and two ahead");
const settle=activeTimer(250);
assert.ok(settle,"culling must defer its idle window after scrolling stops");
runTimer(settle);
assert.deepEqual(near(),[1,2,3,4,5,6],"recently visible/downstream rows must remain warm briefly after scroll settles");
const release=activeTimer(250);
assert.ok(release,"culling must schedule a bounded warm-row release");
runTimer(release);
assert.deepEqual(near(),[1,2,3,4,5],"idle culling must return to a symmetric one-row overscan after the warm hold");

// The settle timer is armed inside the coalesced frame, not once per event: a
// burst of scroll events must cost no timer churn at all.
const beforeBurst=scheduleCount;
scroll.scrollTop=320;
for(let i=0;i<6;i++){scroll.scrollTop+=20;listeners.get("scroll").fn();}
assert.equal(scheduleCount-beforeBurst,0,"a scroll burst must not schedule timers outside the coalesced frame");
flushRaf();
assert.equal(scheduleCount-beforeBurst,1,"the coalesced frame arms the settle timer exactly once per frame");

// S4: a fast flick must not outrun the prewarm — however large the step, every row
// intersecting the viewport after one coalesced frame must be warm. This is the
// question the 1-behind/2-ahead window exists to answer, tested rather than assumed.
scroll.scrollTop=900;
listeners.get("scroll").fn();
flushRaf();
const viewportTop=900,viewportBottom=900+200;
const intersecting=rows.map((r,i)=>i).filter(i=>i*100+100>viewportTop&&i*100<viewportBottom);
assert.ok(intersecting.length>=2,"the fixture must actually place rows in the viewport");
for(const i of intersecting)assert.equal(rows[i].dataset.cullNear,"1",`row ${i} entered the viewport after a fast flick without being warm`);

vm.runInContext("calendarSetWeekCullReady(false,scroll)",context);
assert.deepEqual(near(),[],"disabling culling must clear warm-row markers before calendar fitting/rerendering");
assert.equal(listeners.has("scroll"),false,"disabling culling must remove its scroll listener");

console.log("PASS: Lite Calendar culling uses cached directional 1-behind/2-ahead overscan and releases warm rows after a bounded idle hold");
