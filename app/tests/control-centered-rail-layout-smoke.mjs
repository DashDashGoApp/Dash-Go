#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import {fileURLToPath} from "node:url";
const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const read=rel=>fs.readFileSync(path.join(root,rel),"utf8");

const profileCss=read("ui/css/control/display-profile.css"),ia=read("ui/css/control/information-architecture.css"),stack=read("ui/css/control/layout.css"),index=read("index.html"),nav=read("ui/js/control-navigation.js"),diagnostics=read("ui/js/control-calendars-logs.js"),allControlCss=fs.readdirSync(path.join(root,"ui/css/control")).filter(name=>name.endsWith(".css")).map(name=>read(path.join("ui/css/control",name))).join("\n");
function block(source,name){const at=source.indexOf(`function ${name}(`);assert.notEqual(at,-1,`missing ${name}`);const open=source.indexOf("{",at);let depth=0;for(let i=open;i<source.length;i++){if(source[i]==="{")depth++;else if(source[i]==="}"&&!--depth)return source.slice(at,i+1);}throw new Error(`unterminated ${name}`);}
assert.match(profileCss,/--ctrl-editor-max:1190px;--ctrl-action-button-max:420px/,"Control must expose one body measure plus an individual action cap");
assert.ok(!allControlCss.includes("--ctrl-rail-medium")&&!allControlCss.includes("--ctrl-rail-compact"),"retired medium/compact rails must not survive");
assert.ok(!allControlCss.includes("margin-inline:0 auto"),"Control rules must never dump rail slack into a one-sided right void");
assert.ok(!profileCss.includes("profilefinetune")&&!profileCss.includes("profileeditors"),"Profile must not retain a nested tuning rail");
assert.match(profileCss,/#ctrl \.ctrlcontentrail,#ctrluisettings\{[^}]*width:min\(100%,var\(--ctrl-editor-max\)\)[^}]*margin-inline:auto/,"Calendar settings must inherit the centered body rail");
assert.match(profileCss,/settinggrid-calendar-general\{[^}]*width:100%[^}]*margin-inline:0/,"Calendar general settings must fill their parent rail");
assert.match(profileCss,/settinggrid-calendar-profile\{[^}]*width:100%[^}]*margin-inline:0/,"Calendar profile-owned settings must fill their parent rail");
assert.match(ia,/grid-2-feature \.actiongroup-grid\{[^}]*grid-template-columns:repeat\(2,minmax\(0,1fr\)\)[^}]*width:100%[^}]*margin-inline:0/,"Display pair grid must fill its rail");
assert.match(ia,/actiongroup-grid>\.cbtn\.actionbtn[^}]*max-width:var\(--ctrl-action-button-max\)[^}]*justify-self:center/,"short action buttons must cap inside their own tracks");
assert.match(stack,/#ctrl \.ctrlcardrow\s*\{\s*display:grid;grid-template-columns:minmax\(0,1fr\);gap:var\(--ctrl-gap\)/,"layout must be the sole owner of the one-column compact-card wrapper");
assert.match(stack,/#ctrlpage-control \.ctrlcardrow-control-secondary\s*\{grid-template-columns:repeat\(2,minmax\(0,1fr\)\);align-items:start;\}/,"desktop keeps Security/Location as a natural-height two-up pair");
assert.match(index,/ctrlcardrow ctrlcardrow-control-secondary[\s\S]*data-lazy="security"[\s\S]*data-lazy="location"/,"Security and Location must retain independent lazy details inside a layout-only pair wrapper");
assert.ok(stack.includes(".ctrlstack>.ctrlcardrow>.ctrlsec"),"nested Control details must retain the final authoritative sizing contract");
const screen=block(nav,"renderCtrlScreenSettings");assert.ok(screen.includes("ctrlcardrow ctrlcardrow-screen"),"Screen controls and sleep schedule must share the desktop two-up wrapper");
const diag=block(diagnostics,"renderCtrlDiagnostics");assert.ok(diag.includes("ctrlcardrow ctrlcardrow-doctor")&&diag.includes("doctorSafeRepairAction"),"Diagnostics short action groups must share the desktop two-up wrapper");
assert.ok(nav.includes('[[true,"On"],[false,"Off"]]'),"Week number On/Off remains direct-touch and visible");
assert.ok(!fs.existsSync(path.join(root,"ui/js/app.bundle.js"))&&!fs.existsSync(path.join(root,"ui/control-layout.css")),"source handoff must not contain generated browser assets");
console.log("PASS: beta.100 uses a centered Control rail with direct Calendar controls and no stale retired tuning rail.");
