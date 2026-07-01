#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import {fileURLToPath} from "node:url";

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const read=rel=>fs.readFileSync(path.join(root,rel),"utf8");
const controlDir=path.join(root,"ui","css","control");
const cssFiles=fs.readdirSync(controlDir).filter(name=>name.endsWith(".css")).sort();
const allCss=cssFiles.map(name=>read(path.join("ui/css/control",name))).join("\n");
const tokens=read("ui/css/control/tokens.css");
const layout=read("ui/css/control/layout.css");
const base=read("ui/css/dashboard/control-overlay.css");
const core=read("ui/js/control-ui.js");
const nav=read("ui/js/control-navigation.js");
const lock=read("ui/js/control-location-lock.js");
const lifecycle=read("ui/js/control-lifecycle.js");
const index=read("index.html");

assert.ok(!fs.existsSync(path.join(controlDir,"00-control-stack-actions.css")),"retired hard-fix layout file must be removed");
for(const token of ["--ctrl-space-1:4px","--ctrl-space-6:24px","--ctrl-gap-compact:var(--ctrl-space-2)","--ctrl-tap-min:48px","--ctrl-summary-min:52px","--ctrl-tab-min:52px","--ctrl-action-min:64px","--ctrl-action-min-lg:76px","--ctrl-metric-min:180px","--ctrl-radius-sm:10px","--ctrl-radius-md:14px","--ctrl-bp-stack:760px","--ctrl-bp-two:1100px","--ctrl-bp-wide:1280px"]){assert.ok(tokens.includes(token),`missing shared token ${token}`);}
assert.match(layout,/#ctrl \.ctrlpage \.actiongroup-grid,[\s\S]*?grid-template-columns:repeat\(auto-fit,minmax\(min\(100%,220px\),1fr\)\)/,"action grids must use one intrinsic default");
assert.match(layout,/@media\s*\(min-width:1280px\)/,"wide Control tier required");
assert.match(layout,/@media\s*\(max-width:1100px\)/,"compact Control tier required");
assert.match(layout,/@media\s*\(max-width:760px\)/,"stack Control tier required");
assert.match(layout,/\.comprow \.cbtn\{padding:var\(--ctrl-space-2\) var\(--ctrl-space-3\);\}/,"compact message actions must still use the tap-floor layer");
assert.match(layout,/:is\(\.cbtn,\.themebtn,\.pinbtn,\.oskkey,[\s\S]*?min-height:var\(--ctrl-tap-min\)/,"all shared interactive controls need the shared touch floor");
assert.match(layout,/\.ctrltabs \.cbtn\.on:after/,"active Control tabs need a persistent orientation indicator");
assert.ok((allCss.match(/!important/g)||[]).length<30,"Control CSS must retain fewer than 30 !important declarations");
// Core Control tiers stay centralized. A small 700px component breakpoint is
// permitted for the dense Household Schedules editor, where two-column touch
// actions otherwise become too narrow before the shared 760px shell tier.
const allowedControlBreakpoints=new Set(["min-width:1280px","max-width:1100px","max-width:760px","max-width:700px"]);
for(const match of allCss.matchAll(/@media\s*\(([^)]*)\)/g))assert.ok(allowedControlBreakpoints.has(match[1].replaceAll(" ","")),`unexpected Control breakpoint: ${match[1]}`);
assert.match(base,/\.comprow \.cbtn\{min-height:var\(--ctrl-tap-min\)/,"legacy compact row button must no longer fall below touch minimum");

assert.match(core,/function ctrlUpdateSettingCard\(key,opts\)/,"Control needs a common in-place setting patch helper");
assert.match(core,/root\.querySelector\("\.settingvalue"\)/,"in-place helper must update the visible setting value");
assert.match(core,/root\.querySelectorAll\("\.settingbuttons \.cbtn"\)/,"in-place helper must update stepper bounds");
assert.match(nav,/ctrlUpdateSettingCard\(key,\{value:`\$\{next\} \$\{unit\}`/,"number steppers must patch in place on success");
assert.match(nav,/startCard\.dataset\.settingKey="firstDayOfWeek"/,"start-day card needs a stable patch key");
assert.match(nav,/weekCard\.dataset\.settingKey="showIsoWeekNumbers"/,"week-number card needs a stable patch key");
assert.match(nav,/nightDim\.dataset\.settingKey="nightDim"/,"night dim action needs a stable patch key");
assert.match(nav,/b\.dataset\.settingChoice=String\(v\)/,"start-day choices need patchable selection markers");
assert.match(nav,/b\.dataset\.settingChoice=String\(on\)/,"week-number choices need patchable selection markers");

for(const page of ["ctrlpage-overview","ctrlpage-display","ctrlpage-calendars","ctrlpage-content","ctrlpage-control","ctrlpage-system"])assert.match(index,new RegExp(`id="${page}"[^>]*data-accordion`),`${page} must opt into the shared accordion model`);
assert.match(nav,/const accordionPage=d\.closest && d\.closest\("\.ctrlpage\[data-accordion\]"\)/,"lazy toggle handler must generalize accordion ownership");
assert.match(nav,/accordionPage\.querySelectorAll\("details\.ctrlsec\[data-lazy\]"\)/,"accordion handler must close only sibling lazy cards");
assert.match(nav,/const CTRL_LAST_OPEN_SECTION=new Map\(\)/,"capable profiles need in-session last-open memory");
assert.match(nav,/ctrlClosePageSectionsForSession\(page,true\)/,"tab hibernation must close cards without erasing the session memory");
assert.match(nav,/restoreCtrlLastOpenSection\(page,requested\)/,"page switching must restore a remembered card when appropriate");
assert.match(nav,/const preserveLast=!!d\._ctrlPreserveLastOpen/,"toggle handling must distinguish hibernation from an explicit close");
assert.match(lifecycle,/CTRL_LAST_OPEN_SECTION\.clear\(\)/,"Control open/close must reset session-only remembered cards");

// CSS owns the Control shell's structural display mode. PIN code exposes only
// semantic visibility, preventing an inline display:block from defeating the
// fixed flex shell and leaving the active page without a finite scroll height.
assert.match(layout,/#ctrlpanel\{[\s\S]*?display:flex[\s\S]*?flex-direction:column[\s\S]*?overflow:hidden/,"authoritative layout must own the fixed Control panel shell");
assert.match(layout,/#ctrlmain\{[\s\S]*?display:flex[\s\S]*?flex-direction:column[\s\S]*?overflow:hidden/,"authoritative layout must keep Control main as a flex column");
assert.match(layout,/#ctrlmain\[hidden\]\{display:none;\}/,"semantic main-region hiding must stay in the authoritative layout layer");
assert.match(layout,/@media\(max-width:760px\)\{[\s\S]*?#ctrlpanel\{[\s\S]*?height:90vh[\s\S]*?min-height:0/,"authoritative layout must retain constrained small-screen Control shell geometry");
assert.match(layout,/#ctrlmain \.ctrlpage\.show\[data-scroll-policy=\"control-page\"\]\{[\s\S]*?overflow-y:auto[\s\S]*?touch-action:pan-y/,"authoritative layout must keep the active Control page as the vertical scroll root");
assert.match(lock,/function setCtrlMainVisible\(visible\)/,"PIN lock needs one semantic main-region visibility helper");
assert.match(lock,/main\.hidden=!visible/,"PIN lock must toggle the main region hidden state");
assert.match(lock,/main\.setAttribute\(\"aria-hidden\",visible\?\"false\":\"true\"\)/,"PIN lock must expose main-region availability to assistive technology");
for(const forbidden of [/main\.style\.display/,/main\.style\.setProperty\(\"display/,/main\.style\.removeProperty\(\"display/])assert.doesNotMatch(lock,forbidden,"PIN lock must not own #ctrlmain display mode");
assert.match(lock,/showPinLock\(\)[\s\S]*?setCtrlMainVisible\(false\)/,"PIN presentation must hide Control main semantically");
assert.match(lock,/setCtrlMainVisible\(true\);[\s\S]*?ctrlMsg\(\"\"\); await renderCtrlAll\(\)/,"PIN unlock must restore semantic Control-main visibility before rerendering");
console.log("PASS: Control shared tokens, universal accordion behavior, and semantic shell visibility keep Control scrolling structural and focus-safe.");
