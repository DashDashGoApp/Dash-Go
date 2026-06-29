#!/usr/bin/env node
// Long-running kiosk regression coverage for beta.1 rendering/input work.
// This stays source-level because it proves that all lazy modules retain the
// same cleanup/retry contracts even before the browser bundles are generated.
import assert from "node:assert/strict";
import {readFileSync} from "node:fs";
import {resolve} from "node:path";

const root=resolve(process.argv[2]||".");
const read=relative=>readFileSync(resolve(root,relative),"utf8");
const tap=read("ui/js/tap.js");
const household=read("ui/js/household-app-loader.js");
const launcher=read("ui/js/app-launcher.js");
const chores=read("ui/chore-wheel.js");
const choreCore=read("ui/chore-wheel-core.js");
const lists=read("ui/lists-core.js")+"\n"+read("ui/lists-actions.js");
const routines=read("ui/routines.js");
const routineMutations=read("internal/household/routines/mutations.go");
const routinePost=read("cmd/dashboard-control-server/routines_post.go");
const familyBoard=read("ui/family-board.js");

// One shared visibility handler prevents a long-running dashboard from keeping
// one document listener per card/button rebuilt by app renderers.
assert.match(tap,/const DASHGO_TAP_BINDINGS=new Set\(\)/,"tap primitive must track live bindings centrally");
assert.match(tap,/function pruneDashGoTapBindings\(\)/,"tap primitive must prune detached controls");
assert.match(tap,/new MutationObserver\(\(\)=>pruneDashGoTapBindings\(\)\)/,"tap primitive must clean detached render trees");
assert.equal((tap.match(/document\.addEventListener\("visibilitychange"/g)||[]).length,1,"tap primitive must install one shared visibility listener");
assert.match(tap,/DASHGO_TAP_BINDINGS\.delete\(binding\)/,"tap disposer must remove its central binding record");

// Failed lazy tags must be removed so a later open gets a new network request
// rather than a listener attached to an already-failed element.
for(const [name,source] of [["household app loader",household],["launcher loader",launcher]]){
  assert.match(source,/dataset\.failed="1";(?:script|link)\.remove\(\);reject\(/,"${name} must remove failed lazy elements before rejecting");
  assert.match(source,/prior&&prior\.dataset\.failed==="1"\)prior\.remove\(\)/,"${name} must discard a previously failed lazy element on retry");
}
assert.match(launcher,/function appendLauncherScript\(/,"chalkboard must use the same retry-safe script helper as other apps");
assert.match(launcher,/function appendListsScript\(/,"Lists must retain retry-safe lazy scripts");

// Dynamic household strings must enter through DOM text nodes rather than
// concatenated HTML. This removes the remaining hand-escaped XSS surface.
assert.doesNotMatch(chores,/\binnerHTML\b|function esc\b/,"Chore Wheel must use DOM builders for user-entered content");
assert.doesNotMatch(lists,/\binnerHTML\b|function esc\b/,"Lists must use textContent/DOM builders for user-entered content");
assert.match(chores,/const make=\(tag,className,text\)=>/,"Chore Wheel needs its DOM text helper");
assert.match(lists,/title\.textContent=task\.title\|\|"Untitled task"/,"Lists task titles must use textContent");

// Chore Wheel now prepares lookup/fairness maps once and sorts already
// decorated candidates rather than repeatedly scanning assignments inside the
// comparator during a plan horizon.
assert.match(choreCore,/function choreIndexes\(data,key\)/,"Chore Wheel must build reusable render/fairness indexes");
assert.match(choreCore,/assignmentByChoreDate\.set\(/,"Chore Wheel index must include assignment lookup keys");
assert.match(choreCore,/return pool\.map\(person=>\(\{/,"fair choice must decorate each candidate once");
assert.match(choreCore,/function noteFairAssignment\(/,"planning must update the working index after a decision");
assert.match(chores,/function refreshRenderIndex\(\)\{state\.renderIndex=core\.choreIndexes/,"Chore Wheel render must reuse a single indexed snapshot");
assert.match(chores,/core\.noteFairAssignment\(plan,chore,winner,key\)/,"batch planners must advance the index instead of rescanning");

// Routine checkboxes are optimistic and batched. The visible session row is
// patched in place; arbitrary checklist tapping does not globally disable the
// panel or require a second GET for the recomputed day.
assert.match(routines,/stepBatch:\{timer:0,flight:null,changes:new Map\(\)\}/,"Routines needs bounded pending step state");
assert.match(routines,/function queueRoutineStep\(/,"Routines must queue checkbox changes");
assert.match(routines,/setTimeout\(\(\)=>flushRoutineSteps\(\),80\)/,"Routines must coalesce quick checkbox taps");
assert.match(routines,/op:"steps"/,"Routines must send one batched occurrence operation");
assert.match(routines,/current\.replaceWith\(sessionCard\(session\)\)/,"Routines must patch a session card instead of rebuilding the full day for each checkbox");
assert.match(routines,/function captureFormFocus\(\)/,"Routines must capture focused form fields before structural rerender");
assert.match(routines,/function restoreFormFocus\(snapshot\)/,"Routines must restore focus/caret after structural rerender");
assert.match(routines,/dayDate:state\.date/,"Routine mutations must request the recomputed day in their response");
assert.doesNotMatch(routines,/if\(!response\.day\)await refreshDay\(\)/,"Routine mutations must not add an avoidable second day GET");
assert.match(routineMutations,/case "steps":/,"Routines service must accept a bounded batched routine step operation");
assert.match(routineMutations,/at least one checklist step is required/,"batched routine operations must validate empty requests");
assert.match(routinePost,/response\["day"\] = routinesDayResponse\(result\.Payload, result\.DayDate\)/,"route adapter must return the recomputed day inline");

// Family Board only needs one sorted active-note list to report its count.
assert.match(familyBoard,/const active=core\.activeOrder\(noteList\(\)\),count=active\.length;/,"Family Board must sort active notes once when accepting state");

console.log("PASS: app rendering work keeps tap cleanup bounded, lazy retries recoverable, user strings DOM-safe, Chore Wheel indexed, and Routines checkbox updates focused and batched");
