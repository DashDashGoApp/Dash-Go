#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import {fileURLToPath} from "node:url";

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const read=rel=>fs.readFileSync(path.join(root,rel),"utf8");
const geometry=read("ui/js/settings-calendar-geometry.js");
const core=read("ui/js/control-ui.js");
const close=read("ui/js/control-lifecycle.js");
const grid=read("ui/js/calendar-grid.js");
const finish=read("ui/js/calendar-layout-finish.js");
const snap=read("ui/js/settings-idle-scroll.js");
const defaults=read("ui/js/config-defaults.js");
const navigation=read("ui/js/control-navigation.js");

assert.match(geometry,/function calendarGeometryBeginPreview\(\)/,"geometry changes need a single preview transaction");
assert.match(geometry,/calendarGeometryCaptureAnchor\(scroll\)/,"preview must capture a semantic Calendar anchor before CSS geometry changes");
assert.match(geometry,/renderCalendar\(\{force:true,deferHome:true\}\)/,"Control close must force one final Calendar render");
assert.match(geometry,/calendarAfterLayoutFit\(renderSerial,\(\)=>calendarGeometryFinishCommit\(serial\)\)/,"Today-anchor restoration must wait for the existing event/span fitting pipeline");
assert.match(geometry,/setCalendarScrollHomeTop\(home\)/,"final commit must replace the cached Today home offset");
assert.match(geometry,/rowHeight\*calendarGeometryClampRatio\(anchor\.ratio\)/,"away-from-home restoration must preserve proportional position inside the visible week");
assert.match(core,/const geometryPreview=geometryChanged&&typeof calendarGeometryBeginPreview/,"profile application must begin the preview before publishing geometry values");
assert.match(core,/calendarPaint&&!geometryPreview&&typeof renderCalendar/,"geometry steps must not rebuild Calendar while Control remains open");
assert.match(close,/calendarGeometryCommitAfterControlClose\(\)/,"all canonical Dashboard Control closes must flush the pending geometry transaction");
assert.match(grid,/calendarLayoutFitDidComplete\(_calendarRenderSerial\)/,"Calendar must signal completion after its existing span/event fit pipeline");
assert.match(finish,/const CALENDAR_LAYOUT_FIT_WAITERS=\[\]/,"fit completion callbacks must be bounded and explicit");
assert.match(snap,/function calendarScrollSnapSuspend\(next\)/,"snap-back must pause while geometry is previewed");
assert.match(snap,/function calendarScrollSnapReconcile\(\)/,"snap-back must be rearmed after the correct home top is committed");
assert.match(defaults,/snapBackSeconds:\s*35/,"default snap-back delay must be reduced from 45 to 35 seconds");
assert.match(navigation,/Previews the grid outline immediately/,"row-height card must explain the immediate preview");
assert.match(navigation,/Previews the Calendar\/sidebar outline immediately/,"sidebar card must explain the immediate preview");

const rows=[
  {id:"",offsetTop:0,offsetHeight:200},
  {id:"currentweek",offsetTop:200,offsetHeight:200},
  {id:"",offsetTop:400,offsetHeight:200},
];
const ctrl={classList:{contains:name=>name==="show"}};
const scroll={scrollTop:450,querySelectorAll:selector=>selector===".weekrow"?rows:[]};
const current=rows[1];
let home=200,renderCount=0,fitCulling=[],snapStates=[];
const context={
  CALENDAR_SCROLL_HOME:{ready:true},
  $:selector=>selector==="#calscroll"?scroll:selector==="#currentweek"?current:selector==="#ctrl"?ctrl:null,
  calendarScrollHomeTop:()=>home,
  setCalendarScrollHomeTop:value=>{home=value;},
  calendarSetWeekCullReady:value=>fitCulling.push(value),
  calendarScrollSnapSuspend:value=>snapStates.push(value),
  calendarScrollSnapReconcile:()=>{},
  requestAnimationFrame:fn=>fn(),
  renderCalendar:()=>{
    renderCount++;
    rows[0].offsetTop=0;rows[0].offsetHeight=300;
    rows[1].offsetTop=300;rows[1].offsetHeight=300;
    rows[2].offsetTop=600;rows[2].offsetHeight=300;
  },
  calendarRenderSerial:()=>1,
  calendarAfterLayoutFit:(_serial,callback)=>callback(),
  Math,Number,String,Array,Object,console,
};
context.globalThis=context;
vm.createContext(context);
vm.runInContext(geometry,context);
assert.equal(vm.runInContext("calendarGeometryBeginPreview()",context),true,"open Control must start a preview");
assert.equal(vm.runInContext("calendarGeometryCommitAfterControlClose()",context),true,"closing Control must schedule exactly one commit");
assert.equal(renderCount,1,"one final Calendar render must occur after close");
assert.equal(home,300,"final current-week home top must use the newly rendered row geometry");
assert.equal(scroll.scrollTop,675,"away-from-home view must restore the same week and proportional in-row position");
assert.deepEqual(snapStates,[true,false],"snap-back must suspend for preview and resume only after the committed home top");
assert.ok(fitCulling.includes(false),"preview must temporarily disable off-screen culling before final fit work");
assert.equal(vm.runInContext("calendarGeometryCommitPending()",context),false,"transaction must clear after its final fit/anchor commit");

console.log("PASS: Calendar geometry previews immediately, commits one close-time render/fit/home-anchor transaction, preserves semantic scroll position, and shortens snap-back delay");
