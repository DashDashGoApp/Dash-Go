#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import {fileURLToPath} from "node:url";
const app=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const read=rel=>fs.readFileSync(path.join(app,rel),"utf8");
const fit=read("ui/js/messages-fit.js");
const rotation=read("ui/js/messages-rotation.js");
const schedule=read("ui/js/messages-schedule.js");
const core=read("ui/js/messages-core.js");
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
// W7: every #comptext visibility change must funnel through one helper so
// pause, force, and instant swaps behave identically in all fade modes.
assert.match(fit,/function compSetVisible\(el,visible\)/,"visibility changes must share one compSetVisible helper");
assert.match(fit,/function liteMessageFadeMode\(/,"the hidden Lite fade mode selector must exist and default to opacity");
for(const [name,src] of [["messages-core",core],["messages-rotation",rotation],["messages-schedule",schedule]]){
  assert.doesNotMatch(src,/style\.opacity\s*=/,`${name} must not set #comptext opacity directly`);
  assert.ok(src.includes("compSetVisible("),`${name} visibility changes must go through compSetVisible`);
}
// W2: dwell still scales with word count — a 40-word temporary message must
// hold the lane at least 20 seconds (and never past the 45 s ceiling).
const delayContext=vm.createContext({
  CONFIG:{complimentSeconds:18,complimentWordThreshold:10,complimentExtraWordStep:8,complimentExtraSecondsPerStep:2,complimentMaxSeconds:45,showSeconds:false}
});
vm.runInContext(`${block(schedule,"messageWordCount")}\n${block(schedule,"complimentDelayMs")}\nglobalThis.__delay=complimentDelayMs;globalThis.__words=messageWordCount;`,delayContext);
assert.equal(delayContext.__words("one two three four five six seven eight nine ten"),10,"word counting must match the dwell threshold");
const longDwell=delayContext.__delay(Array.from({length:40},(_,i)=>`word${i}`).join(" "));
assert.ok(longDwell>=20000,"a 40-word temporary message must dwell at least 20 seconds");
assert.ok(longDwell<=45000,"dwell must never exceed the configured ceiling");
const context=vm.createContext({CONFIG:{complimentFadeMs:750}});
vm.runInContext(`${block(fit,"complimentFadeValueMs")}\n${block(fit,"complimentFadeDelayMs")}\nglobalThis.__fade=complimentFadeDelayMs;`,context);
assert.equal(context.__fade(),750,"automatic default fade must be 750 ms");
console.log("PASS: beta.64 message fade remains smooth and bounded without a user-facing card, with a single visibility funnel and word-scaled dwell");
