#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import {fileURLToPath} from "node:url";

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const read=p=>fs.readFileSync(path.join(root,p),"utf8");
const ia=read("ui/css/control/information-architecture.css");
const nav=read("ui/js/control-navigation.js");

const desktop=ia.match(/@media\(min-width:1280px\)\{([\s\S]*?)\n\}/);
assert.ok(desktop,"missing desktop Control-pair media block");
const rules=desktop[1];
assert.match(rules,/#ctrlscreensettings \.ctrlcardrow-screen,#ctrldiag \.ctrlcardrow-doctor\{grid-template-columns:repeat\(2,minmax\(0,1fr\)\);align-items:stretch;\}/,"Screen/Sleep must retain the shared desktop equal-height peer wrapper");
assert.match(rules,/#ctrlscreensettings \.ctrlcardrow-screen>\.actiongroup,#ctrldiag \.ctrlcardrow-doctor>\.actiongroup\{display:flex;flex-direction:column;min-height:0;\}/,"each Screen/Sleep panel must accept the shared desktop height");
assert.match(rules,/#ctrlscreensettings \.ctrlcardrow-screen>\.actiongroup>\.actiongroup-grid,#ctrldiag \.ctrlcardrow-doctor>\.actiongroup>\.actiongroup-grid\{flex:1 1 auto;min-height:0;\}/,"each Screen/Sleep action grid must fill its matched outer panel");
assert.ok(!/#ctrlscreensettings \.ctrlcardrow-screen\{[^}]*align-items:start;/.test(rules),"Screen/Sleep must not retain the start-aligned unequal-height override");
assert.match(nav,/const cardRow=el\("div","ctrlcardrow ctrlcardrow-screen"\)/,"Screen controls and Sleep schedule must retain their shared two-up wrapper");
assert.ok(nav.includes('cardRow.appendChild(power.group)')&&nav.includes('cardRow.appendChild(schedule.group)'),"both peer action groups must remain inside the equal-height wrapper");
console.log("PASS: beta.28 keeps desktop Screen controls and Sleep schedule as equal-height peer panels without changing the narrow one-column flow");
