#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import {fileURLToPath} from "node:url";

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const read=rel=>fs.readFileSync(path.join(root,rel),"utf8");
const exists=rel=>fs.existsSync(path.join(root,rel));
const base=read("ui/css/dashboard/base.css");
const responsive=read("ui/css/dashboard/responsive.css");
const fit=read("ui/js/dashboard-fit.js");
const runtime=read("ui/js/settings-runtime.js");
const configRuntime=read("ui/js/config-runtime.js");
const profileResult=read("ui/js/control-ui.js");
const index=read("index.html");
const nav=read("ui/js/control-navigation.js");
const liteMemory=read("ui/js/control-lite-memory.js");
const presets=read("internal/settings/profile.go");

for(const token of ["--dash-rowheight-preference","--dash-sidebarwidth-preference","--tier-rowheight-min","--tier-rowheight-max","--tier-sidebarwidth-min","--tier-sidebarwidth-max"])assert.ok(base.includes(token),`base geometry token missing: ${token}`);
assert.match(base,/--rowheight:clamp\(var\(--tier-rowheight-min\),var\(--dash-rowheight-preference\),var\(--tier-rowheight-max\)\)/,"row height must clamp preference to responsive bounds");
assert.match(base,/--sidebarwidth:clamp\(var\(--tier-sidebarwidth-min\),var\(--dash-sidebarwidth-preference\),var\(--tier-sidebarwidth-max\)\)/,"sidebar width must clamp preference to responsive bounds");
for(const tier of ["xl","base","compact","dense","min"]){
  const block=(responsive.match(new RegExp(`:root\\[data-fit="${tier}"\\]\\{([\\s\\S]*?)\\n\\}`))||[])[1]||"";
  for(const token of ["--tier-rowheight-min","--tier-rowheight-max","--tier-sidebarwidth-min","--tier-sidebarwidth-max"])assert.ok(block.includes(token),`${tier} bounds missing ${token}`);
}
assert.doesNotMatch(fit,/layoutProfile|999px|--cfg-rowheight|--cfg-sidebarwidth/,"fit controller must not use legacy custom geometry gates or sentinel caps");
assert.match(fit,/root\.style\.setProperty\("--dash-rowheight-preference",row\)/,"fit controller must publish selected row-height preference");
assert.match(fit,/root\.style\.setProperty\("--dash-sidebarwidth-preference",sidebar\)/,"fit controller must publish selected sidebar preference");
for(const source of [runtime,configRuntime,profileResult]){
  assert.match(source,/--dash-rowheight-preference/,"all runtime update paths must update the row preference");
  assert.match(source,/--dash-sidebarwidth-preference/,"all runtime update paths must update the sidebar preference");
  assert.doesNotMatch(source,/--cfg-rowheight|--cfg-sidebarwidth/,"old sentinel preference variables must be gone");
}
assert.doesNotMatch(presets,/"layoutProfile"/,"new profile presets must not write legacy layoutProfile state");

const styleCalls=[];
const rootEl={dataset:{fit:"base"},style:{setProperty:(key,value)=>styleCalls.push([key,value])}};
const app={classList:{contains:()=>false,toggle(){},remove(){}}};
const context={
  CONFIG:{profile:"balanced",layoutProfile:"auto",rowHeight:240,sidebarWidth:440},
  window:{innerWidth:1920,innerHeight:1080,devicePixelRatio:1},
  document:{documentElement:rootEl,getElementById:id=>id==="app"?app:null,addEventListener(){}},
  Math,Number,String,Object,Array,Set,console,setTimeout,clearTimeout,requestAnimationFrame:fn=>fn(),
};
context.window.window=context.window;context.globalThis=context;
vm.createContext(context);
vm.runInContext(`${fit}\ndashboardFitApplyGeometryPreferences();`,context);
assert.deepEqual(styleCalls,[["--dash-rowheight-preference","240px"],["--dash-sidebarwidth-preference","440px"]],"saved geometry must apply even when legacy layoutProfile is auto");

assert.ok(!index.includes('data-lazy="radar"')&&!index.includes('id="ctrlradar"'),"Dashboard Control must remove the redundant Weather radar card");
assert.ok(!nav.includes('case "radar"')&&!liteMemory.includes('"radar"'),"Control lazy/render retention must not retain Radar");
assert.ok(!exists("ui/js/09-control-12d-radar.js")&&!exists("ui/css/control/05e-radar.css"),"Control-only Radar files must be removed");
assert.ok(index.includes('id="radarfull"')&&exists("ui/js/radar-overlay.js")&&exists("ui/js/radar-lite.js"),"Dashboard Radar overlay and Lite renderer must remain available");

console.log("PASS: durable Calendar geometry preferences survive responsive tiers and Dashboard Control removes only its redundant Radar card");
