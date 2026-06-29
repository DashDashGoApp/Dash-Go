#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import {fileURLToPath} from "node:url";
const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const read=rel=>fs.readFileSync(path.join(root,rel),"utf8");

const index=read("index.html"),profile=read("ui/js/control-profile-editor.js"),css=read("ui/css/control/display-profile.css"),nav=read("ui/js/control-navigation.js"),display=read("ui/js/control-display-weather.js"),cache=read("ui/js/control-cache.js");
for(const token of ["function ctrlSaveProfileOwned","profileChangedValue","profilechange-values","Profile default:","Current:"])assert.ok(profile.includes(token),`missing changed-settings structure ${token}`);
for(const retired of ["profileEditorNumber","profileEditorBool","profileEditorAlertsToggle","profileEditorBuildTuning","profileTuningLink","renderCtrlDisplayTuningShortcut","Fine-tune performance"]){assert.ok(!profile.includes(retired),`Profile must not retain tuning surface ${retired}`);}
assert.ok(!index.includes("ctrldisplaytuning"),"Display must not retain the retired tuning shortcut mount");
assert.ok(css.includes("settinggrid-calendar-profile")&&css.includes("repeat(2,minmax(0,1fr))"),"Calendar range/dimension cards need a deliberate desktop two-column grid");
assert.match(css,/@media\(max-width:1100px\)[\s\S]*settinggrid-calendar-profile\{grid-template-columns:1fr/,"Calendar profile cards must stack safely");
assert.ok(css.includes("profilechange")&&!css.includes("profilefinetune")&&!css.includes("profileeditors"),"Profile CSS must keep comparisons and remove retired tuning styling");
assert.ok(nav.includes("Calendar range")&&nav.includes("Calendar dimensions")&&nav.includes("ctrlCalendarProfileNumberCard"),"Calendar owns direct numeric controls");
assert.ok(display.includes("grid-3-provider")&&display.includes("Clock seconds")&&display.includes("grid-1-feature")&&display.includes("Background alert monitoring"),"Display must use count-aware grid groups for relocated controls");
assert.ok(cache.includes("Map behavior")&&cache.includes("Interactive event maps"),"Map behavior must appear above cache maintenance");
console.log("PASS: beta.100 relocates fine-tuning into calendar and display cards with stable responsive grids.");
