#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import {fileURLToPath} from "node:url";

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const load=rel=>fs.readFileSync(path.join(root,rel),"utf8");
const shared=load("ui/js/shared-osk.js");
const map=load("ui/js/map-interactive.js");
const people=load("ui/js/control-people.js");
const consistency=load("ui/css/control/consistency.css");
const tokens=load("ui/css/control/tokens.css");
const forms=load("ui/css/control/message-forms.css");
const oskCss=load("ui/css/control/messages-osk.css");
const liteCss=load("ui/css/control/lite-performance.css");
const mapCss=load("ui/css/dashboard/popups-alerts-maps.css");
const radarCss=load("ui/css/dashboard/touch-radar.css");
const overlayCss=load("ui/css/dashboard/control-overlay.css");
const infoCss=load("ui/css/control/information-architecture.css");
const profileCss=load("ui/css/control/display-profile.css");
const layoutCss=load("ui/css/control/layout.css");
const weatherCss=load("ui/css/control/theme-weather.css");
const todoCss=load("ui/css/control/todo.css");
const statusCss=load("ui/css/dashboard/control-status-maintenance.css");
const actionsLiteCss=load("ui/css/dashboard/control-theme-actions-lite.css");
const familyCss=load("ui/family-board.css");

assert.match(shared,/function oskSetSubmit\(input,label,handler\)/,"shared OSK must expose one input-owned affirmative-action binding");
assert.match(shared,/async function oskSubmit\(\)/,"shared OSK must own one guarded affirmative action path");
assert.match(shared,/if\(ch==="submit"\) return oskSubmit\(\);/,"numeric Enter must use the shared submit path");
assert.match(shared,/last\.appendChild\(mkKey\(oskSubmitLabel\(\),"wide osksubmit",\(\)=>oskSubmit\(\)\)\)/,"text OSK must expose a labelled affirmative key");
assert.match(shared,/if\(_oskMode\(\)===\"date\"[\s\S]*?_oskShift=false;[\s\S]*?\}else\{[\s\S]*?_oskLayer=\"letters\";[\s\S]*?_oskShift=true;/,"opening a text field must begin Shift-active while numeric fields do not");
assert.match(map,/_mapOskLayer=\"letters\";[\s\S]*?_mapOskShift=true;[\s\S]*?buildMapKeyboard\(\);/,"interactive-map text keyboard must also begin Shift-active");

// Enter must close before invoking the input-owned affirmative action. That lets
// a validation handler deliberately re-open the keyboard for correction.
const target={dataset:{},classList:{add(){},remove(){}},value:""};
const sandbox={
  __target:target,
  __calls:[],
  document:{querySelectorAll:()=>[],documentElement:{style:{removeProperty(){}}}},
  requestAnimationFrame:fn=>fn(),
  setTimeout:()=>0,
  Event:class Event{constructor(){}},
};
vm.createContext(sandbox);
vm.runInContext(shared,sandbox);
await vm.runInContext(`(async()=>{
  _oskTarget=__target;
  oskSetSubmit(__target,"Save",async()=>{__calls.push("submit");});
  hideOSK=()=>{__calls.push("hide");_oskTarget=null;};
  await oskSubmit();
})()`,sandbox);
assert.deepEqual(sandbox.__calls,["hide","submit"],"OSK Enter must hide first and invoke the bound action once");

for(const rel of [
  "ui/js/control-content-osk.js","ui/js/control-location-lock.js","ui/js/control-message-schedules.js",
  "ui/js/control-navigation.js","ui/js/control-people.js","ui/js/control-special-feeds.js","ui/js/control-todo.js",
]) assert.match(load(rel),/oskSetSubmit\(/,`${rel} must bind the OSK affirmative key to its existing action`);
assert.doesNotMatch(people,/input\.className="people-text-input"/,"People name fields must retain the shared oskfield class instead of overriding it");

assert.match(consistency,/#ctrl \.oskfield,[\s\S]*?min-height:52px;[\s\S]*?background:var\(--panel\);[\s\S]*?border-radius:var\(--ctrl-radius-sm\);/,"OSK inputs require one complete Control surface independent of page position");
assert.match(consistency,/#ctrl \.oskfield\.oskfocus,[\s\S]*?outline:2px solid var\(--accent\)/,"shared OSK input focus needs a visible tokenized ring");
assert.equal((forms.match(/#ctrlpage-content input\{/g)||[]).length,1,"message forms must retain only the non-conflicting layout-only generic input rule");
assert.doesNotMatch(forms,/background:#202028/,"message-form inputs must not force a dark literal background");
assert.doesNotMatch(overlayCss,/#ctrlcomp input\{[^}]*(background:|border-radius:|padding:)/,"dashboard-shell input fallback must stay layout-only");
assert.doesNotMatch(overlayCss,/#ctrlpanel select\{[\s\S]*?background:#202028/,"Control selects must not force a dark literal background");
assert.match(mapCss,/#mapsearch\{[\s\S]*?min-height:52px;[\s\S]*?border-radius:var\(--ctrl-radius-sm\);[\s\S]*?font:700 17px\/1\.2 var\(--sans\)/,"map search must share the Control text-input geometry");
assert.match(todoCss,/#ctrl \.oskfield\.todo-client-id\{min-height:56px;/,"To Do input variants must preserve their larger layout without replacing the shared visual treatment");
assert.match(radarCss,/#radarscrub::-webkit-slider-runnable-track[\s\S]*?var\(--panel\)/,"radar timeline must style its WebKit track from theme tokens");
assert.match(radarCss,/#radarscrub::-webkit-slider-thumb[\s\S]*?var\(--accent\)/,"radar timeline must style its WebKit thumb from theme tokens");
for(const [name,css] of [["OSK",oskCss],["Lite",liteCss],["dashboard status shell",statusCss],["dashboard Lite shell",actionsLiteCss]]) assert.doesNotMatch(css,/#15151a/,`${name} surface must not force a dark literal background on light themes`);
for(const [name,css] of [["overlay",overlayCss],["danger architecture",infoCss],["profile badge",profileCss],["danger drawer",layoutCss],["light overrides",weatherCss],["dashboard status shell",statusCss],["dashboard action shell",actionsLiteCss]]) assert.doesNotMatch(css,/#8a2a2a|#8a3a3a|#ffd2bd|#ffcf8b|#513019|#6b4d39/,`${name} danger/warning surface must not retain the reviewed hard-coded color`);
assert.match(tokens,/--ctrl-danger-fg:var\(--sun\);[\s\S]*?--ctrl-danger-armed-bg:[\s\S]*?--ctrl-safe-repair-armed-bg:/,"danger and safe-repair states must be defined through one token family");
assert.match(familyCss,/#familyboard\.osk-open \.fb-body\{scrollbar-width:none;\}/,"Family Board must hide only the native scrollbar while the shared OSK is open");
assert.match(familyCss,/#familyboard\.osk-open \.fb-body::\-webkit-scrollbar\{display:none;width:0;height:0;\}/,"WebKitGTK must not paint the Family Board scrollbar over the fixed OSK");
console.log("PASS: OSK Shift/Enter bindings, shared input treatment, themed radar range, and tokenized light-theme surfaces");
