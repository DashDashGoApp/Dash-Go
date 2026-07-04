#!/usr/bin/env node
import assert from "node:assert/strict";
import {readFileSync} from "node:fs";
import {resolve} from "node:path";

const root=resolve(process.argv[2]||".");
const read=rel=>readFileSync(resolve(root,rel),"utf8");
const index=read("index.html");
const calendars=read("ui/js/control-calendars.js");
const quick=read("ui/js/control-system-actions.js");
const privateUi=read("ui/js/control-private-calendars.js");
const css=read("ui/css/control/calendar-manager-visibility.css");
const manifest=JSON.parse(read("ui/css/bundle.manifest.json"));

assert.match(index,/data-lazy="cals" data-default-open="true"><summary>Manage calendars<\/summary>/,"Manage Calendars must be the first open Calendar-tab action");
assert.doesNotMatch(index,/<summary>Calendar visibility<\/summary>/i,"Calendar Visibility must not remain a standalone Calendar-tab card");
assert.match(calendars,/async function renderCtrlCals\(\)\{[\s\S]*?api\("\/api\/calendars\/manage"\)/,"Manage Calendars must load one ownership/access/health model");
assert.match(calendars,/function ctrlOpenCalendarVisibilityShortcut\(\)/,"Quick visibility needs a shared popup controller");
assert.match(calendars,/Quick show\/hide only — access mode and sync settings stay unchanged\./,"Quick visibility must state its limited scope");
assert.match(calendars,/ctrlCalendarManagerBadges\(item\)/,"calendar rows must surface shown/source/access/health badges");
assert.match(privateUi,/item\.enabled===false\?"Show calendar":"Hide calendar"/,"selected private calendars must expose direct visibility without a separate card");
assert.match(quick,/Calendar visibility","Show or hide calendars quickly\. Sync access and provider settings stay unchanged\./,"Quick Actions needs the compact Calendar visibility shortcut");
assert.ok(manifest.bundles.control.includes("control/calendar-manager-visibility.css"),"Calendar Manager visibility CSS must ship in the control bundle");
assert.match(css,/\.calendar-visibility-popup/,"quick visibility popup needs a bounded visual shell");
assert.match(css,/\.calmanager-badges/,"manager rows need readable status badges");
assert.doesNotMatch(calendars,/function ctrlCalendarManagerState\(/,"retired standalone Calendar Visibility state helper must be removed");
console.log("Calendar Manager smoke: primary manager, shared visibility, and source/access/health rows are wired.");
