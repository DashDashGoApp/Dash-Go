#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import {fileURLToPath} from "node:url";
const app=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");const read=rel=>fs.readFileSync(path.join(app,rel),"utf8");
const fit=read("ui/js/messages-fit.js");
const rotation=read("ui/js/messages-rotation.js");
const profile=read("ui/js/control-profile-editor.js");
const css=read("ui/css/dashboard/sidebar-weather-messages.css");
const base=read("ui/css/dashboard/base.css");
function block(source,name){const at=source.indexOf(`function ${name}(`);assert.notEqual(at,-1,`missing ${name}`);const open=source.indexOf("{",at);let depth=0;for(let i=open;i<source.length;i++){if(source[i]==="{")depth++;else if(source[i]==="}"&&!--depth)return source.slice(at,i+1);}throw new Error(`unterminated ${name}`);}
for(const name of ["complimentFadeValueMs","complimentFadeDelayMs"])assert.ok(fit.includes(`function ${name}(`),`missing ${name}`);
assert.ok(block(rotation,"rotateCompliment").includes("},fadeMs);"),"swap must wait for its smooth fade-out duration");
assert.ok(css.includes("transition:opacity var(--compfade) ease-in-out"),"message fade must use one compositor opacity transition");
assert.ok(base.includes("--compfade:750ms"),"CSS fallback must use the automatic smooth fade");
assert.ok(!profile.includes("complimentFadeMs")&&!profile.includes("Message timing"),"Profile must not expose fade tuning");
assert.ok(!fs.existsSync(path.join(app,"ui/js/09-control-12aa-message-behavior.js")),"fixed fade must not require an information-only Messages card");
assert.ok(!read("index.html").includes("ctrlmessagebehavior")&&!read("ui/js/control-navigation.js").includes('case "messagebehavior"'),"fixed fade must not retain a dead Messages route");
const context=vm.createContext({CONFIG:{complimentFadeMs:750}});
vm.runInContext(`${block(fit,"complimentFadeValueMs")}\n${block(fit,"complimentFadeDelayMs")}\nglobalThis.__fade=complimentFadeDelayMs;`,context);
assert.equal(context.__fade(),750,"automatic default fade must be 750 ms");
console.log("PASS: beta.64 message fade remains smooth and bounded without a user-facing card");
