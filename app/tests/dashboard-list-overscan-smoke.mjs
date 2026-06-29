#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import {fileURLToPath} from "node:url";

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const source=fs.readFileSync(path.join(root,"ui/js/calendar-list-overscan.js"),"utf8");
const agenda=fs.readFileSync(path.join(root,"ui/js/calendar-agenda.js"),"utf8");
const weather=fs.readFileSync(path.join(root,"ui/js/weather.js"),"utf8");
const css=fs.readFileSync(path.join(root,"ui/css/dashboard/sidebar-weather-messages.css"),"utf8");

assert.ok(source.split(/\n/).length<=400,"List overscan must stay a focused split module");
assert.match(source,/const DASHBOARD_LIST_OVERSCAN_BEHIND_ROWS=1;/,"lists must retain one row behind travel");
assert.match(source,/const DASHBOARD_LIST_OVERSCAN_AHEAD_ROWS=2;/,"lists must prewarm two rows ahead of travel");
assert.match(source,/root\.addEventListener\("scroll",state\.onScroll,\{passive:true\}\)/,"list scroll listener must remain passive");
const handler=source.match(/function dashboardListOverscanOnScroll\(root\)\{[\s\S]*?\n\}/)?.[0]||"";
assert.ok(handler,"list overscan needs one raw scroll handler");
assert.doesNotMatch(handler,/getBoundingClientRect|clientHeight|scrollHeight|offsetTop|offsetHeight/,"raw list scroll handling must use cached geometry");
assert.match(css,/#agendalist \.agev\[data-list-cull-near="1"\],#wx14 \.wxday\[data-list-cull-near="1"\]\{content-visibility:visible;\}/,"Agenda and Weather warm rows must force visible content");
assert.match(agenda,/dashboardListOverscanClear\(list\)[\s\S]*?dashboardListOverscanAfterRender\(list,"\.agev"\)/,"Agenda must reset and refresh overscan around its explicit render transaction");
assert.match(weather,/dashboardListOverscanClear\(strip\)[\s\S]*?dashboardListOverscanAfterRender\(strip,"\.wxday"\)/,"Weather must reset and refresh overscan around its explicit render transaction");

let list;
function item(index,height=64){return {dataset:{},getBoundingClientRect:()=>({top:index*height-list.scrollTop,height})};}
const items=Array.from({length:7},(_,index)=>item(index));
const listeners=new Map(),timers=new Map(),raf=[];
let timerId=0;
list={scrollTop:96,clientHeight:128,getBoundingClientRect:()=>({top:0}),querySelectorAll:()=>items,
  addEventListener:(name,fn,opts)=>listeners.set(name,{fn,opts}),removeEventListener:name=>listeners.delete(name)};
const context={console,Math,Number,Array,Set,Object,WeakMap,
  requestAnimationFrame:fn=>{raf.push(fn);return raf.length;},cancelAnimationFrame:id=>{raf[id-1]=null;},
  setTimeout:(fn,ms)=>{const id=++timerId;timers.set(id,{fn,ms});return id;},clearTimeout:id=>timers.delete(id)};
context.globalThis=context;vm.createContext(context);vm.runInContext(source,context);
const flushRaf=()=>{while(raf.length){const fn=raf.shift();if(fn)fn();}};
const near=()=>items.map((entry,index)=>entry.dataset.listCullNear==="1"?index:null).filter(index=>index!==null);
const timerAt=ms=>[...timers.entries()].find(([,value])=>value.ms===ms);
const runTimer=entry=>{assert.ok(entry,"expected bounded timer");const [id,value]=entry;timers.delete(id);value.fn();};
vm.runInContext("dashboardListOverscanEnable(list,'.agev')",Object.assign(context,{list}));
assert.deepEqual(near(),[0,1,2,3,4],"idle lists must keep one item before and after the visible slice");
assert.equal(listeners.get("scroll")?.opts?.passive,true,"list listener must be passive");
list.scrollTop=170;listeners.get("scroll").fn();flushRaf();
assert.deepEqual(near(),[1,2,3,4,5,6],"downward list scroll must retain one behind and two ahead");
list.scrollTop=160;listeners.get("scroll").fn();flushRaf();
assert.deepEqual(near(),[0,1,2,3,4,5],"upward list scroll must mirror the two-row prewarm behind travel");
runTimer(timerAt(250));
assert.deepEqual(near(),[0,1,2,3,4,5],"recently active rows remain warm briefly after settling");
runTimer(timerAt(250));
assert.deepEqual(near(),[1,2,3,4,5],"settled list returns to symmetric one-row overscan");
vm.runInContext("dashboardListOverscanClear(list)",context);
assert.deepEqual(near(),[],"clearing a list render transaction removes warm markers");
assert.equal(listeners.has("scroll"),false,"clearing must remove the list scroll listener");
console.log("PASS: Agenda and Weather use all-profile cached directional list overscan with a bounded warm hold");
