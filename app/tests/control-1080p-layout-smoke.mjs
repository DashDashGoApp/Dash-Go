#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import {fileURLToPath} from "node:url";
const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const read=rel=>fs.readFileSync(path.join(root,rel),"utf8");

const index=read("index.html"),shell=read("ui/css/control/layout.css"),profileCss=read("ui/css/control/display-profile.css"),ia=read("ui/css/control/information-architecture.css"),securityCss=read("ui/css/control/display-location.css"),statusCss=read("ui/css/dashboard/control-status-maintenance.css"),profile=read("ui/js/control-profile-editor.js"),nav=read("ui/js/control-navigation.js"),cache=read("ui/js/control-cache.js"),calLogs=read("ui/js/control-calendars-logs.js"),display=read("ui/js/control-display-weather.js"),lock=read("ui/js/control-location-lock.js"),updates=read("ui/js/control-updates.js");
assert.match(shell,/@media\(min-width:1280px\)[\s\S]*width:min\(1480px,calc\(100vw - 96px\)\)/,"wide Control tier must retain the bounded 1480px shell");
assert.ok(!fs.existsSync(path.join(root,"ui/css/control/tabs-shell.css")),"retired tab-shell layer must be removed so layout owns the rail");
assert.match(profileCss,/--ctrl-editor-max:1190px;--ctrl-action-button-max:420px/,"wide Control must use one centered body rail and one individual action cap");
assert.ok(!profileCss.includes("--ctrl-rail-medium")&&!profileCss.includes("--ctrl-rail-compact"),"retired left-pinnable subrails must not return");
assert.ok(!profileCss.includes("profilefinetune")&&!profileCss.includes("profileeditors"),"Profile must not retain a nested retired tuning layout");
assert.match(profileCss,/settinggrid-calendar-profile\{[^}]*grid-template-columns:repeat\(2,minmax\(0,1fr\)\)/,"Calendar range/dimensions need a deliberate desktop pair grid");
assert.match(profileCss,/@media\(max-width:1100px\)[\s\S]*settinggrid-calendar-profile\{grid-template-columns:1fr/,"Calendar pair grid must stack at the shared compact breakpoint");
assert.match(ia,/#ctrlpage-display \.grid-2-feature \.actiongroup-grid\{[^}]*grid-template-columns:repeat\(2,minmax\(0,1fr\)\)[^}]*width:100%[^}]*margin-inline:0/,"wide Display pairs must fill the centered body rail");
assert.match(ia,/actiongroup-grid>\.cbtn\.actionbtn[^}]*max-width:var\(--ctrl-action-button-max\)[^}]*justify-self:center/,"short Display actions must cap inside their tracks rather than left-pin a narrow grid");
assert.ok(display.includes("Clock seconds")&&display.includes("Background alert monitoring")&&cache.includes("Interactive event maps"),"relocated controls must remain visible at 1080p");
assert.ok(!index.includes("ctrldisplaytuning")&&!profile.includes("renderCtrlDisplayTuningShortcut"),"Display must not retain a tuning shortcut");
assert.match(index,/data-lazy="calhealth"[^>]*><summary>Calendar source health/,"Calendar health needs its own lazy route");
assert.match(nav,/case "calhealth": await renderCtrlCalendarHealthPanel\(\);/,"calendar health lazy route missing");
assert.ok(calLogs.includes("async function renderCtrlCalendarHealthPanel")&&calLogs.includes('cachedApi("/api/cache/status"'),"Calendar health must render directly from its own cached endpoint path");
assert.ok(lock.includes('locktiming-grid locktiming-grid-"+kind')&&lock.includes('lockTimingCluster("Short unlocks","short"')&&lock.includes('lockTimingCluster("Longer unlocks","long"'),"Security timing must render semantic short and longer direct-touch rows");
assert.ok(updates.includes("function ctrlUpdateApplyProgress")&&updates.includes("Check for updates"),"Update card must retain stable progress handling");
assert.match(statusCss,/\.maintenancegrid \.stat\.quiet\{box-shadow:none/,"quiet System inventory needs a neutral visual lane");
console.log("PASS: beta.100 keeps the centered 1080p Control shell while individual choices move to their feature cards.");
