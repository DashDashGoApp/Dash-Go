import assert from "node:assert/strict";
import fs from "node:fs";
import vm from "node:vm";
const root=new URL("..",import.meta.url).pathname;
const launcher=fs.readFileSync(root+"ui/js/app-launcher.js","utf8");
const defaults=fs.readFileSync(root+"ui/js/config-defaults.js","utf8");
const runtime=fs.readFileSync(root+"ui/js/settings-runtime.js","utf8");
const core=fs.readFileSync(root+"ui/js/control-ui.js","utf8");
const profile=fs.readFileSync(root+"ui/js/control-profile-editor.js","utf8");
const radar=fs.readFileSync(root+"ui/js/radar-overlay.js","utf8");
const boot=fs.readFileSync(root+"ui/js/boot.js","utf8");
const todoStore=fs.readFileSync(root+"internal/todo/todo_store.go","utf8");
const todoHTTP=fs.readFileSync(root+"cmd/dashboard-control-server/todo_http.go","utf8");
assert.match(launcher,/const DASHBOARD_APPS=Object\.freeze\(/,"launcher must retain a static local registry");
assert.match(launcher,/function availableDashboardApps\(\)\{ return DASHBOARD_APPS\.slice\(\)\.sort/,"all launcher apps must be permanent registry entries");
assert.match(launcher,/trigger\.hidden=false;/,"Apps trigger must remain visible");
assert.match(launcher,/footer\.classList\.add\("has-app-launcher"\)/,"footer layout must always reserve the Apps trigger");
assert.equal((launcher.match(/id:"(chalkboard|radar|todo|grocery|family-board|chore-wheel|maintenance|routines|dashboard-control)"/g)||[]).length,9,"launcher must contain exactly nine permanent destinations");
for(const id of ["chalkboard","radar","todo","grocery","family-board","chore-wheel","maintenance","routines","dashboard-control"]){assert.match(launcher,new RegExp(`id:"${id}"`),`${id} must remain in launcher registry`);}
assert.ok(!launcher.includes("chalkboardEnabled"),"launcher must not hide Chalkboard");
assert.ok(!launcher.includes("todoSlotAvailable"),"launcher must not hide List slots");
assert.ok(!launcher.includes("warmListsAssets"),"launcher must not preload Lists before the family opens it");
assert.ok(!launcher.includes("APP_LAUNCHER_HANDOFF"),"launcher must not restore the removed loading handoff");
assert.ok(!launcher.includes("app-launcher-handoff"),"launcher must not load an app-shaped transition screen");
assert.ok(!launcher.includes("setAppLauncherBusy"),"launcher must return to direct app opening without a busy/loading shell");
assert.ok(!launcher.includes("radarEnabled"),"launcher must not hide Radar");
assert.ok(!defaults.includes("showChalkboard"),"runtime defaults must retire Chalkboard visibility");
assert.ok(!defaults.includes("radarEnabled"),"runtime defaults must retire Radar visibility");
assert.ok(!runtime.includes("showChalkboard"),"normal settings saves must not recreate Chalkboard visibility");
assert.ok(!runtime.includes("radarEnabled"),"runtime settings must ignore legacy radar visibility");
assert.ok(!core.includes("showChalkboard"),"profile result must not carry Chalkboard visibility");
assert.ok(!core.includes("radarEnabled"),"profile result must not carry Radar visibility");
assert.ok(!profile.includes("showChalkboard"),"Profile editor must not expose Chalkboard visibility");
assert.match(profile,/What a profile resets","Profiles reset Calendar layout, Dashboard display, Weather & alerts, and Event maps controls/,"Profile must describe reset scope without exposing app visibility toggles");
assert.ok(!fs.existsSync(root+"ui/js/09-control-12d-radar.js"),"Dashboard Control must not retain a separate Radar renderer");
assert.ok(!fs.existsSync(root+"ui/css/control/05e-radar.css"),"Dashboard Control must not retain Radar-only CSS");
assert.ok(!radar.includes("radarEnabled"),"Radar overlay must not be gated by a retired setting");
assert.ok(!boot.includes("radarEnabled"),"weather triple-tap must not be gated by a retired setting");
assert.ok(!todoStore.includes("todoAppsEnabled"),"server must not retain a To Do launcher toggle");
assert.ok(!todoStore.includes('"enabled": a.todo'),"To Do status must not expose app visibility");
assert.ok(!todoHTTP.includes('"/api/todo/apps"'),"server must not expose a To Do app visibility endpoint");
const radarIconBody=launcher.match(/function appIconRadar\(\)\{([\s\S]*?)\n\}/)?.[1]||"";
assert.match(radarIconBody,/viewBox","0 0 72 72"/,"radar icon must reserve visual clearance inside a spacious viewBox");
assert.match(radarIconBody,/M18 58A28 28 0 1 1 54 58/,"radar icon must retain an inset, open outer sweep instead of a clipped edge sweep");
assert.match(radarIconBody,/M36 24c-6\.1 0-11 4\.8-11 10\.9/,"radar icon must use a centered map-pin silhouette");
assert.match(radarIconBody,/cx:"36",cy:"34\.5",r:"3\.5",fill:"currentColor"/,"radar icon must use a compact filled target dot");
assert.ok(!radarIconBody.includes('r:"7",fill:"none"'),"radar icon must not recreate the eye-like outlined center");

const todoIconBody=launcher.match(/function appIconCheckGrid\(\)\{([\s\S]*?)\n\}/)?.[1]||"";
assert.match(todoIconBody,/appSvgElement\("rect",\{x:"11",y:"10",width:"42",height:"44",rx:"8"\}\)/,"To Do must use the selected Check Grid outer task card");
assert.match(todoIconBody,/M19\.4 21\.5l1\.7 1\.8 3\.2-3\.6/,"To Do must retain Check Grid completed task marks");
assert.match(todoIconBody,/appSvgElement\("rect",\{x:"18",y:"42",width:"7",height:"7",rx:"1\.6"\}\)/,"To Do must retain Check Grid's visible open task row");
assert.match(launcher,/id:"todo"[\s\S]*?icon:appIconCheckGrid/,"To Do tile must select the Check Grid icon");
const routinesIconBody=launcher.match(/function appIconRepeatCheck\(\)\{([\s\S]*?)\n\}/)?.[1]||"";
assert.match(routinesIconBody,/M20 21a17 17 0 0 1 26 2/,"Routines must retain the selected Repeat Check upper loop");
assert.match(routinesIconBody,/M44 43a17 17 0 0 1-26-2/,"Routines must retain the selected Repeat Check lower loop");
assert.match(routinesIconBody,/cx:"32",cy:"32",r:"10"/,"Routines must retain Repeat Check's centered completion ring");
assert.match(routinesIconBody,/M27\.5 32l3\.1 3\.2 6\.4-7/,"Routines must retain Repeat Check's centered tick");
assert.match(launcher,/id:"routines"[\s\S]*?icon:appIconRepeatCheck/,"Routines tile must select the Repeat Check icon");

const elements=new Map();
function element(){return {hidden:true,style:{display:""},attrs:{},classList:{add(){},remove(){},contains(){return false;}},setAttribute(k,v){this.attrs[k]=v;}};}
elements.set("cblaunch",element());elements.set("compliment",element());
const sandbox={
  CONFIG:{version:"1.4.3-beta.46"},
  document:{getElementById:id=>elements.get(id)||null,addEventListener(){},createElementNS(){return {setAttribute(){},append(){}};}},
  Object,Date,Promise,fetch:async()=>({json:async()=>({})}),console,
  openRadar(){},openChalkboardImpl(){},openListsImpl(){},openListsForAdd(){},openFamilyBoard(){},openChoreWheel(){},openMaintenance(){},openRoutines(){},lazyOpenCtrl(){return Promise.resolve();},
  appIconFamilyBoard(){return {};},appIconChoreWheel(){return {};},appIconMaintenance(){return {};},
};
sandbox.window=sandbox;
vm.createContext(sandbox);
vm.runInContext(launcher+"\nthis.__apps=availableDashboardApps; this.__update=updateAppLauncherTrigger;",sandbox);
assert.deepEqual(Array.from(sandbox.__apps(),app=>app.id),["chalkboard","radar","todo","grocery","family-board","chore-wheel","maintenance","routines","dashboard-control"],"all seven apps must have stable launcher order");
sandbox.__update();
assert.equal(elements.get("cblaunch").hidden,false,"Apps trigger must remain visible");
assert.equal(elements.get("cblaunch").style.display,"inline-flex","Apps trigger must use touch layout");
const launcherCss=fs.readFileSync(root+"ui/css/dashboard/app-launcher.css","utf8");
assert.match(launcherCss,/grid-template-columns:repeat\(3,minmax\(0,1fr\)\)/,"launcher must use a fixed 3-column grid");
assert.match(launcherCss,/grid-template-rows:repeat\(3,minmax\(0,1fr\)\)/,"launcher must use a fixed 3-row grid");
assert.ok(!/overflow:auto/.test(launcherCss),"launcher must not scroll to reveal permanent apps");
assert.match(launcher,/openDashboardControlFromLauncher/,"launcher must hand off Dashboard Control without a duplicate settings app");
assert.match(launcher,/openRoutines\(\)/,"launcher must lazily open Routines");
console.log("PASS: permanent 3x3 App Launcher visibility contract");
