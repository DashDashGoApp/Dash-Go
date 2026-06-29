#!/usr/bin/env node
// Source-level contract for the two local household apps. This intentionally
// checks their lifecycle and bounded/local-first shape without depending on a
// browser implementation or provider service.
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import {fileURLToPath} from "node:url";

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const read=rel=>fs.readFileSync(path.join(root,rel),"utf8");
const index=read("index.html");
const launcher=read("ui/js/app-launcher.js");
const householdApps=read("ui/js/household-app-loader.js");
const overlays=read("ui/js/popup-overlays.js");
const osk=read("ui/js/shared-osk.js");
const footer=read("ui/js/family-board-footer.js");
const footerCss=read("ui/css/dashboard/family-board-footer.css");
const board=read("ui/family-board.js");
const boardCss=read("ui/family-board.css");
const maintenance=read("ui/maintenance.js");
const maintenanceCss=read("ui/maintenance.css");
const familyStore=read("internal/household/family/store.go");
const familyService=read("internal/household/family/service.go");
const familySummary=read("internal/household/family/summary.go");
const familyMutations=read("internal/household/family/mutations.go");
const familyFacade=read("cmd/dashboard-control-server/family_board_facade.go");
const boardHTTP=read("cmd/dashboard-control-server/family_board_http.go");
const maintenanceService=read("internal/household/maintenance/service.go");
const maintenanceModel=read("internal/household/maintenance/model.go");
const maintenanceMutations=read("internal/household/maintenance/mutations.go");
const maintenanceCalendar=read("internal/household/maintenance/calendar.go");
const maintenanceCoreCalendar=read("cmd/dashboard-control-server/maintenance_calendar.go");
const maintenanceHTTP=read("cmd/dashboard-control-server/maintenance_http.go");
const maintenanceFacade=read("cmd/dashboard-control-server/maintenance_facade.go");

for(const [id,title,close,body] of [["familyboard","familyboard-title","familyboard-close","familyboard-body"],["maintenance","maintenance-title","maintenance-close","maintenance-body"]]){
  assert.match(index,new RegExp(`id="${id}"[^>]*aria-modal="true"[^>]*role="dialog"`),`${id} needs a static accessible app shell`);
  for(const child of [title,close,body])assert.ok(index.includes(`id="${child}"`),`${id} missing ${child}`);
}
assert.ok(index.includes('id="familyboardfooter"'),"Family Board needs a dormant in-flow dashboard footer root");
assert.match(launcher,/id:"family-board"[\s\S]*?available:\(\)=>true[\s\S]*?launch:\(\)=>openFamilyBoard\(\)/,"Family Board must be a code-owned local launcher tile");
assert.match(launcher,/id:"maintenance"[\s\S]*?available:\(\)=>true[\s\S]*?launch:\(\)=>openMaintenance\(\)/,"Maintenance must be a code-owned local launcher tile");
assert.match(householdApps,/ui\/family-board\.css\?v=[\s\S]*?ui\/family-board-core\.js\?v=[\s\S]*?ui\/family-board\.js\?v=/,"Family Board assets must remain local and lazy");
assert.match(householdApps,/ui\/maintenance\.css\?v=[\s\S]*?ui\/maintenance-core\.js\?v=[\s\S]*?ui\/maintenance\.js\?v=/,"Maintenance assets must remain local and lazy");
const maintenanceIconBody=householdApps.match(/function appIconMaintenance\(\)\{([\s\S]*?)\n\}/)?.[1]||"";
assert.match(maintenanceIconBody,/appSvgElement\("rect",\{x:"11",y:"16",width:"38",height:"34",rx:"4"\}\)/,"Maintenance must use the selected calendar frame");
assert.match(maintenanceIconBody,/M11 26h38/,"Maintenance must keep the calendar header divider");
assert.match(maintenanceIconBody,/M21 12v8/,"Maintenance must keep the left calendar binding post");
assert.match(maintenanceIconBody,/M39 12v8/,"Maintenance must keep the right calendar binding post");
assert.match(maintenanceIconBody,/transform:"translate\(15 16\) scale\(0\.55\)"/,"Maintenance must place the selected inset cog with the provided transform");
assert.match(maintenanceIconBody,/M43\.10 35\.39 L43\.50 30\.20 L48\.50 30\.20/,"Maintenance must use the selected cog path geometry");
assert.match(maintenanceIconBody,/appSvgElement\("circle",\{cx:"46",cy:"46",r:"4"\}\)/,"Maintenance must keep the cog center circle");
assert.ok(!/fill:"currentColor"|stroke:"none"/.test(maintenanceIconBody),"Maintenance must remain a single-color line icon without filled icon parts");
assert.ok(!/https?:\/\//.test((launcher+householdApps).replace("http://www.w3.org/2000/svg","")),"app launcher must not add remote app dependencies");

for(const [name,source,closeName,rootId] of [["Family Board",board,"closeFamilyBoard","familyboard"],["Maintenance",maintenance,"closeMaintenance","maintenance"]]){
  assert.ok(!/\b(?:window\.)?(?:alert|confirm|prompt)\s*\(/.test(source),`${name} must use in-overlay confirmation rather than native dialogs`);
  assert.match(source,new RegExp(`bindTap\\(close,${closeName}\\)`),`${name} close button must use shared tap primitive`);
  assert.match(source,new RegExp(`bindTap\\(shell,${closeName},\\{ignore:event=>event\\.target!==shell\\}\\)`),`${name} backdrop must not intercept child field focus`);
  assert.match(source,/showOSKFor\(input\)/,`${name} fields must invoke the OSK`);
  assert.match(source,/hideOSK\(\)/,`${name} close/cancel must release OSK space`);
  assert.match(source,new RegExp(`document\\.getElementById\\("${rootId}"\\)`),`${name} must use an explicit stable root lookup`);
}
for(const [name,css,rootId] of [["Family Board",boardCss,"familyboard"],["Maintenance",maintenanceCss,"maintenance"]]){
  assert.match(css,new RegExp(`#${rootId}\\{position:fixed;inset:0`),`${name} needs a bounded overlay backdrop`);
  assert.match(css,/height:min\([^;]+94vh\)/,`${name} panel must keep stable viewport-bounded height`);
  assert.match(css,/grid-template-rows:auto auto minmax\(0,1fr\)/,`${name} must reserve body-only scrolling`);
  assert.match(css,/background:linear-gradient\(var\(--panel\)/,`${name} panel must be opaque and theme-aware`);
  assert.match(css,/var\(--fg\).*var\(--card\).*var\(--line\)/s,`${name} must use dashboard theme variables`);
}
assert.match(overlays,/document\.getElementById\("familyboard"\)/,"shared overlay lifecycle must include Family Board");
assert.match(overlays,/document\.getElementById\("maintenance"\)/,"shared overlay lifecycle must include Maintenance");
assert.match(osk,/#familyboard\.osk-open,#maintenance\.osk-open/,"OSK cleanup must include both app hosts");
assert.match(osk,/#chorewheel,#familyboard,#maintenance/,"OSK internal-click guard must recognize both app hosts");
assert.match(footer,/familyBoardFooterBoot\(/,"Family Board footer needs one bounded bootstrap");
assert.ok(!footer.includes("setInterval"),"dashboard footer may not add a repeating board poll");
assert.ok(!/position\s*:\s*fixed/.test(footerCss),"Family Board footer must remain in normal dashboard flow");
assert.match(footerCss,/:not\(\[hidden\]\)\{display:inline-flex\}/,"footer visibility must be explicit and dormant by default");

assert.match(familyService,/\.dashboard-family-board\.json/,"Family Board state must stay in a distinct owner-only local file");
assert.match(familyService,/fileio\.WriteAtomic\(s\.storePath, data, 0600\)/,"Family Board storage must be owner-only");
assert.match(familyService,/ArchiveDays\s*=\s*90/,"Family Board archive retention must remain bounded at 90 days");
assert.match(familyService,/func \(s \*Service\) Read\(\)/,"Family Board reads must persist expiry normalization once");
assert.match(familyStore,/func Equal\(left, right map\[string\]any/,"Family Board reads must compare canonical JSON before deciding to write");
assert.match(familyStore,/bytes\.Equal\(leftJSON, rightJSON\)/,"Family Board reads must avoid numeric decode differences causing write-on-read churn");
assert.match(familyFacade,/internal\/household\/family/,"core must use the bounded Family Board child service");
assert.match(boardHTTP,/\/api\/family-board\/notes\/add/,"Family Board needs action endpoints rather than document overwrite");
assert.match(boardHTTP,/familyBoardReadPayload\(\)/,"Family Board GET and summary must use the durable normalized read path");
assert.match(familyMutations,/family note text is required/,"Family Board must validate note text server-side");
assert.match(maintenanceService,/maintenance-tracker\.json/,"Maintenance state must stay in a distinct local config file");
assert.match(maintenanceModel,/func NextDue\(completed, unit string, every int\)/,"Maintenance next due must derive from actual local completion date");
assert.match(maintenanceModel,/HistoryLimit\s*=\s*500/,"Maintenance history must remain bounded");
assert.match(maintenanceCalendar,/AppOwner: "maintenance"/,"Maintenance calendar projection must declare canonical owner metadata");
assert.match(maintenanceCalendar,/maintenance-.*-.*day/,"Maintenance calendar IDs must depend on task and due date");
assert.match(maintenanceCoreCalendar,/calendarService\(\)\.CommitOwnedFeed/,"Maintenance mutations must regenerate the bounded next-due feed through Calendar");
assert.match(maintenanceFacade,/internal\/household\/maintenance/,"core must use the bounded Maintenance child service");
assert.match(maintenanceHTTP,/\/api\/maintenance\/tasks\/complete/,"Maintenance needs a discrete completion endpoint");
assert.match(maintenanceHTTP,/\/api\/maintenance\/tasks\/delete/,"Maintenance deletion must remain a separate action endpoint");
assert.match(maintenance,/A bounded local history record will remain/,"Maintenance delete wording must state the retained bounded history");
assert.ok(!maintenance.includes("Delete permanently"),"Maintenance must not promise an irreversible history purge it does not perform");
assert.match(maintenanceMutations,/if DueChanged\(old, task\)/,"ordinary Maintenance edits must only be historical reschedules when due date changes");

console.log("PASS: Family Message Board and Maintenance Tracker are lazy, local-first, opaque, OSK-capable, bounded household apps");
