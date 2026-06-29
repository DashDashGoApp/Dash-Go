#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import {fileURLToPath} from "node:url";
const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const read=rel=>fs.readFileSync(path.join(root,rel),"utf8");

const index=read("index.html"),profile=read("ui/js/control-profile-editor.js"),display=read("ui/js/control-display-weather.js"),cache=read("ui/js/control-cache.js"),nav=read("ui/js/control-navigation.js"),runtime=read("ui/js/settings-runtime.js");
for(const [src,token,label] of [
  [profile,"changedSettings","structured Profile divergence payload"],[profile,"ctrlSaveProfileOwned","shared Profile persistence"],[profile,"Profile default:","profile default value"],[profile,"Current:","current value"],
  [display,"Clock seconds","Dashboard display clock toggle"],[display,"Background alert monitoring","Weather alert toggle"],[cache,"Interactive event maps","map behavior toggle"],[nav,"Calendar dimensions","Calendar geometry controls"],
[runtime,"CONFIG.showEventMaps=true","automatic static maps"],[runtime,"CONFIG.pixelShift=2","automatic burn-in shift"]
])assert.ok(src.includes(token),`missing ${label}`);
for(const retired of ["ctrldisplaytuning","profileTuningLink","renderCtrlDisplayTuningShortcut","profilefinetune","profileEditorBuildTuning"]){assert.ok(!index.includes(retired)&&!profile.includes(retired)&&!nav.includes(retired),`retired tuning route survives: ${retired}`);}
assert.ok(!nav.includes('case "messagebehavior"'),"Messages must not retain an information-only lazy route");
assert.ok(!index.includes('data-lazy="radar"')&&!fs.existsSync(path.join(root,"ui/js/09-control-12d-radar.js")),"Dashboard Control must not retain the redundant Radar destination");
assert.ok(index.includes('id="radarfull"')&&fs.existsSync(path.join(root,"ui/js/radar-overlay.js")),"Radar remains available through the Dashboard overlay");
console.log("PASS: beta.100 keeps one shared Profile persistence path while controls live with their features.");
