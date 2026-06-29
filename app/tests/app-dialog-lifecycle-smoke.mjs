#!/usr/bin/env node
// Beta.36 app-dialog contract: every launcher app must have one accessible,
// touch-safe dialog lifecycle with deterministic focus and no browser-native
// destructive prompts.
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import {fileURLToPath} from "node:url";

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const read=rel=>fs.readFileSync(path.join(root,rel),"utf8");
const index=read("index.html");
const helper=read("ui/js/app-dialog.js");
const launcher=read("ui/js/app-launcher.js");
const household=read("ui/js/household-app-loader.js");
const chalk=read("ui/chalkboard.js");
const chalkCss=read("ui/chalkboard.css");
const lists=read("ui/lists-core.js")+"\n"+read("ui/lists-actions.js");
const radar=read("ui/js/radar-overlay.js");
const routines=read("ui/routines.js");
const routinesCss=read("ui/routines.css");

assert.match(helper,/window\.DashGoAppDialog=Object\.freeze\(\{focusInitial,restoreFocus\}\)/,"shared dialog focus helper must remain explicit");
assert.match(index,/id="radarfull"[^>]*aria-modal="true"[^>]*role="dialog"[^>]*aria-labelledby="radartitle"/,"radar needs the same named dialog contract as household apps");
assert.match(chalk,/setAttribute\("aria-modal","true"\)/,"Chalkboard must declare a modal dialog");
assert.match(chalk,/setAttribute\("aria-labelledby","chalkboard-title"\)/,"Chalkboard must name its dialog title");
assert.match(chalk,/title\.id="chalkboard-title"/,"Chalkboard title needs a stable accessible ID");
assert.match(chalk,/DashGoAppDialog\.focusInitial\(r,"#chalkboard-close"\)/,"Chalkboard must move initial focus to its close control");
assert.match(chalk,/DashGoAppDialog\.restoreFocus\(S\.priorFocus,trigger\)/,"Chalkboard must restore launcher/prior focus when it closes");
assert.match(chalk,/event\.key!=="Escape"\|\|!S\.open/,"Chalkboard must close or cancel on Escape");
assert.match(chalk,/showClearAllConfirm/,"Chalkboard Clear all must use an in-overlay confirmation path");
assert.ok(!/\b(?:window\.)?(?:alert|confirm|prompt)\s*\(/.test(chalk),"Chalkboard must not use native browser dialogs");
assert.match(chalk,/bind\(r,closeBoard,\{ignore:event=>event\.target!==r\}\)/,"Chalkboard backdrop must use shared pointer-release behavior without stealing child focus");
assert.match(chalk,/opts&&typeof opts\.ignore==="function"/,"Chalkboard click fallback must respect descendant-focus protection");
assert.ok(!chalk.includes("armOverlayAutoClose"),"Chalkboard retains its own bounded inactivity timer rather than joining the incompatible shared overlay timer");
assert.match(lists,/LISTS_STATE\.priorFocus=document\.activeElement/,"Lists must remember its origin before opening");
assert.match(lists,/DashGoAppDialog\.focusInitial\(root,"#listsapp-close"\)/,"Lists must focus its close action on open");
assert.match(lists,/DashGoAppDialog\.restoreFocus\(LISTS_STATE\.priorFocus,trigger\)/,"Lists must restore focus on close");
assert.match(lists,/bindTap\(root,closeLists,\{ignore:event=>event\.target!==root\}\)/,"Lists backdrop must use shared pointer-release behavior");
assert.match(radar,/RADAR_STATE\.priorFocus=document\.activeElement/,"Radar must remember its origin before opening");
assert.match(radar,/DashGoAppDialog\.focusInitial\(root,"#radarclose"\)/,"Radar must focus its close action on open");
assert.match(radar,/DashGoAppDialog\.restoreFocus\(RADAR_STATE\.priorFocus,trigger\)/,"Radar must restore focus on close");
assert.match(radar,/event\.key!=="Escape"\|\|!radarIsOpen\(\)/,"Radar must close on Escape");
assert.match(radar,/bindTap\(root,closeRadar,\{ignore:event=>event\.target!==root\}\)/,"Radar backdrop must use shared pointer-release behavior");
assert.match(household,/window\.openChoreWheel=openChoreWheel;/,"Chore Wheel launcher export must be explicit like Family Board and Maintenance");
assert.ok(!/#(?:[\da-f]{3}|[\da-f]{6,8})\b/i.test(chalkCss),"Chalkboard chrome CSS must not hard-code hex colors");
assert.ok(!/rgba?\(/i.test(chalkCss),"Chalkboard chrome CSS must use shared theme tokens instead of fixed alpha colors");
for(const token of ["var(--fg)","var(--accent)","var(--panel)","var(--line)","var(--dim)","var(--card)","var(--bg)"]){
  assert.ok(chalkCss.includes(token),"Chalkboard CSS missing theme token "+token);
}
assert.match(chalkCss,/:focus-visible\{outline:2px solid var\(--accent\)/,"Chalkboard chrome must retain visible keyboard focus");
assert.match(launcher,/id:"chalkboard"[\s\S]*?launch:\(\)=>openChalkboard\(\)/,"launcher must retain Chalkboard entry");
assert.match(index,/id="routines"[^>]*aria-modal="true"[^>]*role="dialog"[^>]*aria-labelledby="routines-title"/,"Routines needs the shared dialog contract");
assert.match(routines,/DashGoAppDialog\.focusInitial\(shell,"#routines-close"\)/,"Routines must focus its close action on open");
assert.match(routines,/DashGoAppDialog\.restoreFocus\(state\.priorFocus,trigger\)/,"Routines must restore focus on close");
assert.match(routines,/event\.key!=="Escape"\|\|!isOpen\(\)/,"Routines must close/cancel on Escape");
assert.match(routines,/bindTap\(shell,closeRoutines,\{ignore:event=>event\.target!==shell\}\)/,"Routines backdrop must be pointer-safe");
for(const token of ["var(--fg)","var(--accent)","var(--panel)","var(--line)","var(--dim)","var(--card)","var(--bg)"]){assert.ok(routinesCss.includes(token),"Routines CSS missing theme token "+token);}
assert.ok(!/\b(?:window\.)?(?:alert|confirm|prompt)\s*\(/.test(routines),"Routines must not use native browser dialogs");
console.log("PASS: launcher apps share accessible dialog lifecycle, focus restoration, pointer-safe backdrops, and a fully themed Chalkboard shell");
