#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import {fileURLToPath} from "node:url";
const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const read=rel=>fs.readFileSync(path.join(root,rel),"utf8");

const index=read("index.html"),base=read("ui/css/dashboard/base.css"),ctrl=read("ui/css/dashboard/control-overlay.css"),popup=read("ui/css/dashboard/popups-alerts-maps.css"),radarCss=read("ui/css/dashboard/touch-radar.css"),display=read("ui/js/control-display-weather.js"),cache=read("ui/js/control-cache.js"),profile=read("ui/js/control-profile-editor.js"),ia=read("ui/css/control/information-architecture.css"),nav=read("ui/js/control-navigation.js");
assert.ok(!index.includes("At a glance")&&!index.includes("ctrloverview"),"cache-only At a glance must remain removed");
for(const token of ["Dashboard display","Weather &amp; alerts","Event maps &amp; cache","Personal messages"])assert.ok(index.includes(token),`missing Control surface ${token}`);
const order=["Theme","Visual style","Dashboard display","Weather &amp; alerts","Event maps &amp; cache","Screen controls &amp; sleep schedule"].map(x=>index.indexOf(x));assert.ok(order.every((n,i)=>n>=0&&(i===0||n>order[i-1])),"Display cards must follow appearance → dashboard → weather → maps → screen order");
const zi=s=>Number((s.match(/z-index:(\d+)/)||[])[1]);assert.ok(zi(ctrl)>zi(base),"Control must paint above Night Dim");assert.ok(zi(popup)>zi(base)&&zi(radarCss)>zi(base)&&/\#mapfull\{[^}]*z-index:130/.test(popup),"interactive overlays must paint above Night Dim");
assert.ok(!index.includes("ctrldisplaytuning")&&!profile.includes("Fine-tune performance"),"Display must not retain a misplaced retired tuning route");
assert.ok(!index.includes('data-lazy="radar"')&&!index.includes('id="ctrlradar"'),"Dashboard Control must not retain the redundant Weather radar card");
assert.ok(!fs.existsSync(path.join(root,"ui/js/09-control-12d-radar.js"))&&!fs.existsSync(path.join(root,"ui/css/control/05e-radar.css")),"Control-only Radar renderer and CSS must be removed");
assert.ok(index.includes('id="radarfull"')&&fs.existsSync(path.join(root,"ui/js/radar-overlay.js")),"Dashboard radar overlay must remain available outside Dashboard Control");
assert.ok(nav.includes("Calendar range")&&nav.includes("Calendar dimensions"),"Calendar owns range and geometry");
assert.ok(display.includes("Clock seconds")&&display.includes("Background alert monitoring"),"Display owns clock seconds and alert monitoring");
assert.ok(cache.includes("Interactive event maps"),"Maps owns interactive map behavior");
assert.ok(!nav.includes('case "messagebehavior"'),"automatic messages must not retain a dead behavior route");
for(const token of ["grid-3-provider","mapmaintenance"])assert.ok(ia.includes(token),`grid contract missing ${token}`);
assert.ok(!ia.includes("grid-4-dashboard")&&!ia.includes("grid-4-screen"),"retired fixed-count grid aliases must stay removed");
assert.ok(nav.includes("inline-time-editor"),"sleep time editing must remain inline");
assert.ok(!profile.includes("Radar budget")&&!profile.includes("radarHistoryMode")&&!profile.includes("radarRenderMode"),"Radar budget should be retired from Dashboard Control");
console.log("PASS: beta.100 keeps fine controls in their natural Control cards without an information-only Messages card.");
