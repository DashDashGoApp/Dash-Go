#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import {fileURLToPath} from "node:url";

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const read=rel=>fs.readFileSync(path.join(root,rel),"utf8");
const base=read("ui/css/dashboard/base.css");
const responsive=read("ui/css/dashboard/responsive.css");
const calendar=read("ui/css/dashboard/calendar.css");
const sidebar=read("ui/css/dashboard/sidebar-weather-messages.css");
const listsDock=read("ui/css/dashboard/lists-dock.css");
const index=read("index.html");
const fit=read("ui/js/dashboard-fit.js");
const boot=read("ui/js/boot.js");
const weather=read("ui/js/weather.js");
const agenda=read("ui/js/calendar-agenda.js");
const grid=read("ui/js/calendar-grid.js");
const overlays=read("ui/js/popup-overlays.js");
const runtime=read("ui/js/settings-runtime.js");

for(const tier of ["xl","base","compact","dense","min"]){
  assert.match(responsive,new RegExp(`:root\\[data-fit="${tier}"\\]`),`responsive CSS must define ${tier} tier`);
}
assert.match(base,/--dash-rowheight-preference:210px/,"base must expose the durable row-height preference");
assert.match(base,/--dash-sidebarwidth-preference:380px/,"base must expose the durable sidebar-width preference");
assert.match(base,/--rowheight:clamp\(var\(--tier-rowheight-min\),var\(--dash-rowheight-preference\),var\(--tier-rowheight-max\)\)/,"row height must clamp the saved preference to tier safety bounds");
assert.match(base,/--sidebarwidth:clamp\(var\(--tier-sidebarwidth-min\),var\(--dash-sidebarwidth-preference\),var\(--tier-sidebarwidth-max\)\)/,"sidebar width must clamp the saved preference to tier safety bounds");
assert.doesNotMatch(base,/--cfg-rowheight|--cfg-sidebarwidth|--fit-rowheight|--fit-sidebarwidth/,"responsive geometry must not retain sentinel/custom-cap variables");
for(const tier of ["xl","base","compact","dense","min"]){
  const block=(responsive.match(new RegExp(String.raw`:root\[data-fit="${tier}"\]\{([\s\S]*?)\n\}`))||[])[1]||"";
  for(const token of ["--tier-rowheight-min","--tier-rowheight-max","--tier-sidebarwidth-min","--tier-sidebarwidth-max"])assert.ok(block.includes(token),`${tier} must provide all geometry safety bounds`);
}
assert.match(base,/grid-template-columns:minmax\(0,1fr\) var\(--sidebarwidth\)/,"main grid must retain shrinkable calendar width");
assert.match(base,/grid-template-rows:minmax\(0,1fr\) var\(--compliment-height\)/,"main grid must reserve the tiered message band");
assert.match(fit,/DASHBOARD_FIT_MIN_CALENDAR_COLUMN/,"controller source must use a measured calendar-column guard");
assert.match(responsive,/fit-dock-weather|fit-dock-sidebar|fit-stack/,"responsive CSS must own dock and stack classes");
assert.match(responsive,/#app\.fit-stack\{[\s\S]*grid-template-areas:"clock" "calendar" "compliment"/,"stacked layout must make Calendar primary with a slim clock strip");
assert.match(responsive,/#app\.fit-dock-weather:not\(.fit-dock-sidebar\):not\(.fit-stack\) #wx14\{display:none;/,"step one must dock only the long forecast list");
assert.match(responsive,/#app\.fit-dock-sidebar:not\(.fit-stack\) #weather\{display:none;/,"step two must dock full Weather");
assert.match(responsive,/#fitdocksheet\{[\s\S]*position:fixed/,"dock sheet must be a real bounded overlay");
assert.match(calendar,/top:var\(--cell-head\)/,"calendar event list must consume the responsive header token");
assert.match(calendar,/overflow-wrap:break-word/,"calendar title surfaces must break long unbroken tokens without the heavier anywhere min-content path");
assert.match(responsive,/\.ev,\.spanbar\{overflow-wrap:break-word;\}/,"responsive calendar event shells must use break-word on constrained kiosks");
assert.doesNotMatch(responsive,/\.ev,\.spanbar\{overflow-wrap:anywhere;\}/,"calendar event shells must not use the heavier anywhere break mode on every render");
assert.match(sidebar,/padding:var\(--dash-clock-pad\)/,"clock padding must be tiered");
assert.match(sidebar,/font-size:var\(--dash-weather-icon-size\)/,"weather icon size must be tiered");
assert.doesNotMatch(sidebar,/@media \(max-width:1100px\)|@media \(min-width:1800px\)/,"legacy standalone weather icon breakpoints must be retired");
assert.match(listsDock,/var\(--compliment-height\)/,"Lists dock must preserve the responsive message row allocation");
assert.match(listsDock,/#app\.fit-stack\.lists-dock-visible/,"stacked fit mode must take precedence over the optional Lists dock");
for(const id of ["fitdocktabs","fitdock-agenda-tab","fitdock-weather-tab","fitdock-weather-toggle","fitdock-weather-compact","fitdocksheet","fitdockclose","fitdockbody"]){
  assert.ok(index.includes(`id="${id}"`),`responsive dock shell missing ${id}`);
}
const firstStylesheet=index.indexOf('<link rel="stylesheet" href="ui/dashboard.css');
const earlyTier=index.indexOf('document.documentElement.dataset.fit=window.dashboardFitTierFromViewport');
assert.ok(earlyTier>=0&&earlyTier<firstStylesheet,"initial fit tier must be applied before dashboard CSS parses");
assert.match(index,/window\.dashboardFitTierFromViewport=function\(width,height\)/,"head bootstrap must expose the shared early tier selector");
assert.ok(index.indexOf('id="fitdocktabs"')>index.indexOf('id="clock"'),"stack dock tabs must live inside the clock strip rather than Calendar");
assert.ok(index.indexOf('id="fitdocktabs"')<index.indexOf('id="agendalist"'),"stack dock tabs must remain in the clock header before Agenda content");
assert.ok(responsive.includes("#fitdocktabs{\n  position:static"),"stack dock tabs must no longer float over Calendar cells");
assert.match(responsive,/#app\.fit-stack #fitdocktabs\{display:flex;flex:0 0 auto;margin-left:auto;\}/,"stack dock tabs must align within the clock strip");
assert.match(fit,/function dashboardFitTier\(view\)/,"fit controller must expose tier logic");
assert.match(fit,/function dashboardFitController\(reason\)/,"fit controller must own measured state application");
assert.match(fit,/function dashboardFitLiteProfile\(\)/,"fit controller must identify Lite/Zero before measured work");
assert.match(fit,/function dashboardFitApplyGeometryPreferences\(\)/,"fit controller must apply durable geometry preferences");
assert.doesNotMatch(fit,/layoutProfile|999px|--cfg-rowheight|--cfg-sidebarwidth/,"fit controller must not gate geometry behind legacy custom state or sentinel caps");
assert.match(fit,/function dashboardFitPrimeBootSignature\(\)/,"fit controller must prime the first-paint state before boot measurement");
assert.match(fit,/function dashboardFitWeatherDayLimit\(\)/,"fit controller must provide bounded inline forecast behavior");
assert.match(fit,/function openDashboardFitSheet\(kind,trigger\)/,"dock tabs must open a shared lifecycle sheet");
assert.match(fit,/window\.innerWidth/,"tiering must read CSS-pixel viewport width");
assert.match(fit,/window\.devicePixelRatio/,"fit controller must retain DPR diagnostic state");
assert.match(boot,/dashboardFitBoot\(\)/,"boot must initialize responsive dock controls");
assert.match(boot,/dashboardFitSchedule\("resize",0\)/,"resize settle must reuse one responsive fit pass");
assert.match(weather,/dashboardFitWeatherDayLimit\(\)/,"weather renderer must honor a bounded inline forecast preview");
assert.doesNotMatch(weather,/dashboardFitSchedule/,"weather rendering must not schedule measured fit work on every refresh");
assert.doesNotMatch(agenda,/dashboardFitSchedule/,"agenda rendering must not schedule measured fit work on every refresh");
assert.match(grid,/getPropertyValue\("--cell-head"\)/,"span bars must read the responsive cell header token");
assert.match(grid,/spanLaneStep/,"span bars must use the responsive lane step");
assert.match(overlays,/fitDockSheetIsOpen/,"shared overlay lifecycle must include dock sheets");
assert.match(runtime,/dashboardFitSchedule\("settings",0\)/,"settings refresh must re-evaluate responsive caps");

const context={
  window:{innerWidth:1920,innerHeight:1080,devicePixelRatio:1},
  document:{documentElement:{},getElementById(){return null;}},
  Math,Number,String,Object,Array,Set,console,setTimeout,clearTimeout,requestAnimationFrame:fn=>fn(),
};
context.globalThis=context;
vm.createContext(context);
vm.runInContext(`${fit}\nglobalThis.__tier=dashboardFitTier; globalThis.__state=dashboardFitState;`,context);
const expected=[
  [[800,480],"min"],[[1024,600],"compact"],[[1280,720],"base"],[[1366,768],"base"],[[1920,1080],"base"],[[2560,1440],"xl"],[[3840,2160],"xl"],
];
for(const [[width,height],tier] of expected)assert.equal(context.__tier({width,height,dpr:1}),tier,`${width}×${height} must select ${tier}`);
assert.equal(context.__tier({width:1920,height:1080,dpr:2}),"base","4K-at-200%-style CSS viewport must remain base, not become xl");
assert.equal(context.__tier({width:1023,height:600,dpr:1}),"dense","1023×600 must remain below the compact boundary");
assert.equal(context.__tier({width:1024,height:599,dpr:1}),"dense","1024×599 must remain below the compact boundary");
assert.equal(context.__tier({width:1280,height:719,dpr:1}),"compact","1280×719 must remain below the base boundary");
assert.equal(context.__tier({width:2400,height:720,dpr:1}),"xl","2400px wide must enter the XL tier");

const firstScript=[...index.matchAll(/<script>([\s\S]*?)<\/script>/g)][0]?.[1]||"";
const earlyContext={window:{innerWidth:800,innerHeight:480},document:{documentElement:{dataset:{}}},Math,Number};
earlyContext.window.window=earlyContext.window;
vm.createContext(earlyContext);
vm.runInContext(firstScript,earlyContext);
assert.equal(earlyContext.document.documentElement.dataset.fit,"min","head bootstrap must set the tier before first stylesheet paint");
for(const [[width,height],tier] of expected){
  assert.equal(earlyContext.window.dashboardFitTierFromViewport(width,height),tier,`early tier selector must match controller at ${width}×${height}`);
}

const comfortable={
  columnWidth:140,weather:{},wxnow:{},weatherHeight:300,currentWeatherHeight:100,
  agenda:{},clock:{},agendaHeight:260,clockHeight:70,sidebar:{},sidebarHeight:520,expectedSidebarOverflow:false,
};
const minStack=context.__state(context.__tier({width:800,height:480,dpr:1}),{...comfortable,columnWidth:75});
assert.deepEqual({dockWeather:minStack.dockWeather,dockSidebar:minStack.dockSidebar,stack:minStack.stack},{dockWeather:true,dockSidebar:true,stack:true},"800×480 must progress to calendar-first stack when measured columns are too narrow");
for(const [width,height] of [[1024,600],[1280,720],[1920,1080]]){
  const state=context.__state(context.__tier({width,height,dpr:1}),comfortable);
  assert.deepEqual({dockWeather:state.dockWeather,dockSidebar:state.dockSidebar,stack:state.stack},{dockWeather:false,dockSidebar:false,stack:false},`${width}×${height} comfortable geometry must not dock`);
}
console.log("PASS: responsive dashboard tiers, first-paint selection, measured weather/sidebar docking, stack shell, and fluid geometry contracts");
