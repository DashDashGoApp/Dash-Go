#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import {fileURLToPath} from "node:url";
const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");const read=rel=>fs.readFileSync(path.join(root,rel),"utf8");
const profile=read("ui/js/control-profile-editor.js");
const defaults=read("ui/js/config-defaults.js");
const runtime=read("ui/js/config-runtime.js");
const css=read("ui/css/control/display-profile.css");
const index=read("index.html");
const nav=read("ui/js/control-navigation.js");
for(const retired of ["profileEditorMessageTiming","profileEditorTimingRow","message-timing-panel","complimentFadeLabel","complimentFadeValueMs","complimentSeconds","complimentFadeMs"]){assert.ok(!profile.includes(retired),`Profile must not retain ${retired}`);}
assert.ok(!fs.existsSync(path.join(root,"ui/js/09-control-12aa-message-behavior.js")),"redundant Message behavior module must be removed");
assert.ok(!index.includes('data-lazy="messagebehavior"')&&!index.includes("ctrlmessagebehavior")&&!index.includes("<summary>Message behavior"),"Messages must not retain an information-only behavior card");
assert.ok(!nav.includes('case "messagebehavior"'),"Messages must not retain a dead lazy route");
assert.ok(defaults.includes("complimentSeconds: 18")&&defaults.includes("complimentFadeMs: 750"),"default message timing must stay smooth and readable");
assert.ok(runtime.includes("CONFIG.complimentSeconds=18")&&runtime.includes("CONFIG.complimentFadeMs=750"),"old config.local timing must not revive retired controls");
assert.ok(!css.includes("message-timing-panel")&&!css.includes("message-timing-row")&&!css.includes("ctrlmessagebehavior"),"retired Message timing and behavior-card CSS must be removed");
console.log("PASS: beta.64 keeps automatic message timing without a redundant Dashboard Control card");
