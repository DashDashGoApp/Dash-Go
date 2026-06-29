#!/usr/bin/env node
import assert from "node:assert/strict";
import {readFileSync} from "node:fs";
import {resolve} from "node:path";

const root=resolve(process.argv[2]||".");
const read=rel=>readFileSync(resolve(root,rel),"utf8");
const people=read("ui/js/control-people.js");
const keypad=read("ui/js/control-people-pin-keypad.js");
const css=read("ui/css/control/people-pin-keypad.css");
const lifecycle=read("ui/js/control-lifecycle.js");
const form=people.match(/function peopleInboxPINForm\(person\)\{[\s\S]*?\n\}\n\nasync function peopleNotificationMutate/);
assert.ok(form,"personal inbox PIN editor must remain a focused form");
assert.match(form[0],/peopleOpenInboxPINKeypad/,"personal inbox PIN fields must open the compact keypad");
assert.doesNotMatch(form[0],/oskInput|buildOSK|peoplePINInput/,"personal inbox PIN fields must not invoke the full text OSK");
assert.match(form[0],/Tap each field to open the numeric keypad/,"personal inbox PIN editor must explain the numeric path");
assert.match(keypad,/function peopleOpenInboxPINKeypad/,"personal inbox keypad needs one canonical open lifecycle");
assert.match(keypad,/function closePeopleInboxPINKeypad/,"personal inbox keypad needs one canonical close lifecycle");
assert.match(keypad,/\["1","2","3","4","5","6","7","8","9"\]/,"keypad must render numeric keys");
assert.match(keypad,/cbtn\("OK","on",submit\)/,"keypad must offer an OK action");
assert.match(keypad,/event\.key==="Enter"/,"physical Enter must submit keypad input");
assert.match(keypad,/event\.key==="Backspace"/,"physical Backspace must edit keypad input");
assert.match(keypad,/event\.key==="Escape"/,"physical Escape must cancel keypad input");
assert.match(keypad,/\^\\d\{4,8\}\$/,"keypad must enforce the existing 4–8 digit PIN boundary");
assert.doesNotMatch(keypad,/\b(?:window\.)?(?:alert|confirm|prompt)\s*\(/,"keypad must keep the themed in-overlay path");
assert.match(lifecycle,/closePeopleInboxPINKeypad\(\{restoreFocus:false\}\)/,"closing Dashboard Control must remove the personal inbox keypad");
assert.match(css,/#people-inbox-pin-keypad\{[\s\S]*z-index:10050/,"keypad must render above Control and the shared OSK");
assert.match(css,/\.people-pin-keypad-grid\{display:grid;grid-template-columns:repeat\(3/,"keypad must keep a clear three-column touch grid");
console.log("PASS: personal inbox PIN uses a compact numeric keypad without the full OSK");
