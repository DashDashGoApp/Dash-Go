#!/usr/bin/env node
import assert from "node:assert/strict";
import {readFileSync, existsSync} from "node:fs";
import {resolve} from "node:path";

const root=resolve(process.argv[2]||".");
const read=rel=>readFileSync(resolve(root,rel),"utf8");
const index=read("index.html");
const base=read("ui/css/dashboard/base.css");
const runtime=read("cmd/dashboard-control-server/runtime_assets.go");
const osk=read("ui/js/shared-osk.js");
const routines=read("ui/routines.js");
const schedule=read("internal/household/routines/schedule.go");
const calendar=read("internal/household/routines/calendar.go");
const mutations=read("internal/household/routines/mutations.go");
const service=read("internal/household/routines/service.go");
const people=read("cmd/dashboard-control-server/household_people.go")+"\n"+read("internal/household/service.go")+"\n"+read("internal/household/people.go");
const popup=read("ui/js/day-popup.js");
const actions=read("ui/js/app-calendar-actions.js");
const lists=read("ui/lists-core.js")+"\n"+read("ui/lists-actions.js");
const listsCSS=read("ui/lists.css");
const maintenance=read("ui/maintenance.js");
const maintenanceCSS=read("ui/maintenance.css");
const board=read("ui/family-board.js");
const boardCore=read("ui/family-board-core.js");
const routinesCSS=read("ui/routines.css");

assert.ok(existsSync(resolve(root,"ui/js/shared-osk.js")),"shared OSK source must exist outside the lazy Control bundle");
assert.match(runtime,/runtimeAssetManifestFiles\(root, runtimeAssetJSManifestRel, "ui\/js", "\.js", "app", "control"\)/,"runtime asset generation must resolve ordinary shared UI files from the ordered manifest");
assert.match(osk,/function showOSKFor\(/,"shared OSK must expose normal field opening");
assert.match(osk,/function hideOSK\(/,"shared OSK must expose normal teardown");
for(const id of ["listsapp","chorewheel","familyboard","maintenance","routines"]){
  assert.match(index,new RegExp(`id="${id}"[^>]*\\bhidden\\b`),`${id} static shell must be hidden before lazy app CSS loads`);
  assert.match(base,new RegExp(`#${id}\\[aria-hidden="true"\\]`),`${id} needs boot CSS guard before lazy CSS loads`);
}
assert.match(lists,/classList\.add\("compose-open"\)/,"Lists composer must suppress duplicate add affordances");
assert.match(lists,/classList\.remove\("compose-open"\)/,"Lists composer state must clear on complete/cancel/close");
assert.match(listsCSS,/height:min\(820px,94vh\)/,"Lists needs stable bounded shell geometry");
assert.match(maintenance,/DashGoAppDialog\.focusInitial\(shell,"#maintenance-close"\)/,"Maintenance must receive initial dialog focus");
assert.match(maintenanceCSS,/\.mt-form \.mt-actions\{position:sticky/,"Maintenance compact form actions must stay reachable");
assert.match(board,/DashGoAppDialog\.focusInitial\(shell,"#familyboard-close"\)/,"Family Board must receive initial dialog focus");
assert.match(board,/Showing 50 of \$\{all\.length\} active notes/,"Family Board active cap must be visible to users");
assert.match(boardCore,/priority==="urgent"/,"Family Board sorting must make urgency explicit");
assert.match(routines,/state\.view==="history"/,"Routines must provide bounded history navigation");
assert.match(routines,/Apply first selected schedule to all/,"Routines must support explicit per-person schedule copying rather than implicit overwrite");
assert.match(routines,/humanDate\(/,"Routines must display a human-readable local date");
assert.match(routinesCSS,/\.rt-form \.rt-actions\{position:sticky/,"Routines compact form actions must stay reachable");
assert.match(people,/household-people\.json/,"person-centered apps must share a canonical local roster");
assert.match(schedule,/func civilDayIndex/,"Routines day cadence must use civil-day arithmetic");
assert.match(schedule,/weeks%every/,"Weekday cadence must honor the Every value");
assert.match(schedule,/func scheduledDay/,"monthly/yearly short-month policy must be explicit");
assert.match(schedule,/!PersonActive/,"paused people must not receive future routine sessions");
assert.match(mutations,/if len\(completed\) == len\(valid\) && len\(valid\) > 0/,"final checked routine step must become completed");
assert.match(mutations,/StateOnly: !rebuild/,"ordinary checklist ticks must avoid calendar rebuilds");
assert.match(calendar,/type bucket struct/,"Routines ICS must aggregate session events");
assert.match(service,/routines\.json/,"Routines state must remain in its distinct local config file");
assert.match(calendar,/Routines — %s · %d/,"aggregated Routines sessions need person/count titles");
assert.match(actions,/render\(next\.day\|\|next\)/,"calendar checklist mutation must refresh popup progress from returned state");
console.log("PASS: beta.40 shared app foundation, Routines correctness, bounded calendar projection, and reviewed form polish sources are present");
