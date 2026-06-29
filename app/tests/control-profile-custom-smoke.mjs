#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import {fileURLToPath} from "node:url";
const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const read=rel=>fs.readFileSync(path.join(root,rel),"utf8");

const profile=read("ui/js/control-profile-editor.js");
const core=read("ui/js/control-ui.js");
const runtime=read("ui/js/settings-runtime.js");
const display=read("ui/js/control-display-weather.js");
const cache=read("ui/js/control-cache.js");
const nav=read("ui/js/control-navigation.js");
const css=read("ui/css/control/display-profile.css");
const server=[read("internal/settings/profile.go"),read("internal/settings/validation.go"),read("cmd/dashboard-control-server/settings.go")].join("\n");
function block(source,name){const at=source.indexOf(`function ${name}(`);assert.notEqual(at,-1,`missing ${name}`);const open=source.indexOf("{",at);let depth=0;for(let i=open;i<source.length;i++){if(source[i]==="{")depth++;else if(source[i]==="}"&&!--depth)return source.slice(at,i+1);}throw new Error(`unterminated ${name}`);}
for(const token of ["function ctrlSaveProfileOwned","function profileChangedSettings","Profile default:","Current:","What a profile resets"])assert.ok(profile.includes(token),`missing Profile comparison contract ${token}`);
for(const removed of ["profileEditorBuildTuning","profileEditorRevealTuning","profileTuningLink","Fine-tune performance","Calendar geometry","Dashboard extras","profilefinetune"])assert.ok(!profile.includes(removed),`retired Profile tuning survived: ${removed}`);
for(const key of ["weeksBelow","weeksAbove","rowHeight","sidebarWidth","showSeconds","showInteractiveMaps","weatherAlerts"])assert.ok(profile.includes(`"${key}"`),`Profile must compare ${key}`);
assert.ok(profile.includes('item.key!=="layoutProfile"')&&profile.includes('filter(key=>key!=="layoutProfile")'),"internal layoutProfile must be filtered from visible comparisons");
assert.ok(server.includes('"changedSettings": changed'),"profile payload must expose structured changed settings");
assert.ok(!/"showInteractiveMaps", "layoutProfile", "weatherAlerts"/.test(read("internal/settings/profile.go")),"layoutProfile must not contribute to Custom state");
for(const [src,label,token] of [[nav,"Calendars","ctrlSaveProfileOwned"],[display,"Display/Weather","ctrlSaveProfileOwned"],[cache,"Event maps","ctrlSaveProfileOwned"]])assert.ok(src.includes(token),`${label} must persist its relocated Profile-owned setting`);
assert.ok(nav.includes("Calendar range")&&nav.includes("Calendar dimensions"),"Calendar owns range and dimensions");
assert.ok(display.includes("Clock seconds")&&display.includes("Background alert monitoring"),"Display owns clock seconds and alert monitoring");
assert.ok(cache.includes("Interactive event maps"),"Event maps owns interactive map behavior");
assert.ok(css.includes("profilechange-values")&&css.includes("settinggrid-calendar-profile"),"comparison and relocated Calendar grids need layout rules");
assert.ok(runtime.includes("CONFIG.showEventMaps=true")&&runtime.includes("CONFIG.pixelShift=2"),"automatic static previews and burn-in protection remain automatic");
assert.ok(!block(core,"applyProfileResult").includes("applySettings("),"Profile mutation must avoid a broad settings reload");
console.log("PASS: beta.100 keeps Performance Profile as preset/reset plus a structured changed-settings comparison.");
