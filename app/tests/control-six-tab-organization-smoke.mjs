#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import {fileURLToPath} from "node:url";
const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const read=rel=>fs.readFileSync(path.join(root,rel),"utf8");

const index=read("index.html"),nav=read("ui/js/control-navigation.js"),actions=read("ui/js/control-system-actions.js"),location=read("ui/js/control-location-lock.js"),profile=read("ui/js/control-profile-editor.js"),css=read("ui/css/control/information-architecture.css");
const ids=["overview","display","calendars","content","control","system"];
for(const id of ids)assert.match(index,new RegExp(`data-ctrlpage="${id}"`),`missing ${id} tab`);
assert.match(index,/class="ctrlpage show" id="ctrlpage-overview"/,"Overview must be default");assert.match(index,/id="ctrlpage-content"/,"Messages page id must remain content");assert.ok(!index.includes("ctrloverview"),"cache-only At a glance must be removed");
function pageSlice(id){const start=index.indexOf(`id="ctrlpage-${id}"`);assert.notEqual(start,-1);const next=ids.map(x=>index.indexOf(`id="ctrlpage-${x}"`,start+1)).filter(x=>x>start).sort((a,b)=>a-b)[0]||index.indexOf('<pre id="ctrldoctor"',start);return index.slice(start,next);}
const overview=pageSlice("overview"),display=pageSlice("display"),calendar=pageSlice("calendars"),content=pageSlice("content"),control=pageSlice("control"),system=pageSlice("system");
for(const token of ["ctrlstatus","ctrlhealth","ctrlactions"])assert.ok(overview.includes(token),`Overview missing ${token}`);
for(const token of ["ctrltheme","ctrlvisualstyle","ctrldashboarddisplay","ctrlscreensettings","ctrlweatheralerts","ctrlmapcache"])assert.ok(display.includes(token),`Display missing ${token}`);
for(const token of ["ctrluisettings","ctrlmoon","ctrlcalhealth","ctrlcache","ctrlcals"])assert.ok(calendar.includes(token),`Calendars missing ${token}`);
for(const token of ["ctrlcomp","ctrltempmsg","ctrlschedmsg","ctrlbirthdays","ctrlcelebrations","ctrlbuiltins","ctrlfeeds","ctrlsources"])assert.ok(content.includes(token),`Messages missing ${token}`);
for(const token of ["ctrlprofile","ctrlsecurity","ctrlloc"])assert.ok(control.includes(token),`Control missing ${token}`);
for(const token of ["ctrlupdate","ctrlsystemupdate","ctrlterminal","ctrlhistory","ctrldiag","ctrlpoweractions"])assert.ok(system.includes(token),`System missing ${token}`);
for(const token of ['case "dashboarddisplay"','case "weatheralerts"','case "screen"','case "calendarlayout"','case "moon"','case "calhealth"'])assert.ok(nav.includes(token),`missing lazy key ${token}`);
assert.ok(!display.includes('data-lazy="radar"')&&!nav.includes('case "radar"'),"Dashboard Control must not retain a Weather radar lazy destination");
assert.ok(index.includes('id="radarfull"'),"Weather radar overlay must remain available from Dashboard interactions");
assert.ok(!nav.includes('case "messagebehavior"')&&!nav.includes('case "logs"'),"retired Message behavior and Logs routes must remain absent");
assert.match(nav,/if\(name==="overview"\) renderCtrlQuickActions\(\)/);assert.match(nav,/else if\(name==="system"\) renderCtrlPowerActions\(\)/);assert.match(actions,/function renderCtrlQuickActions\(/);assert.match(actions,/function renderCtrlPowerActions\(/);assert.match(location,/function renderCtrlMoon\(/);
for(const token of ["function ctrlSaveProfileOwned","Profile default:","Current:","What a profile resets"])assert.ok(profile.includes(token),`Profile must retain ${token}`);
for(const retired of ["Fine-tune performance","profileEditorQueuePatch","profileEditorBuildTuning","profileTuningLink"])assert.ok(!profile.includes(retired),`retired Profile surface survived: ${retired}`);
assert.ok(profile.includes('item.key!=="layoutProfile"'),"internal layoutProfile must be hidden from the visible Profile comparison");
assert.ok(!css.includes("grid-4-dashboard")&&!css.includes("grid-4-screen")&&css.includes("grid-3-provider")&&css.includes("mapmaintenance"),"retired grid aliases must be removed while live count-aware grids remain");
assert.ok(!profile.includes("Radar budget")&&!profile.includes("radarHistoryMode")&&!profile.includes("radarRenderMode"),"Radar budget should be retired from Dashboard Control");
assert.match(index,/data-ctrlpage="control">Settings</,"household and preference tab must use the clearer Settings name");
console.log("PASS: Control tabs retain focused tools with Settings reserved for household preferences and Profile for presets.");
