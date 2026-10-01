import assert from "node:assert/strict";
import {readFileSync} from "node:fs";
import {resolve} from "node:path";

const root=resolve(process.argv[2]||".");
const read=rel=>readFileSync(resolve(root,rel),"utf8");
const actions=read("ui/js/app-calendar-actions.js");
const day=read("ui/js/day-popup.js");
const event=read("ui/js/event-popup.js");
const css=read("ui/css/dashboard/app-calendar-actions.css");
const chore=read("internal/household/chores/day.go");
const choreRoutes=read("cmd/dashboard-control-server/chore_wheel.go");
const maintenance=read("internal/household/maintenance/day.go");
const maintenanceHTTP=read("cmd/dashboard-control-server/maintenance_http.go");
const maintenanceMutations=read("internal/household/maintenance/mutations.go");
const routineMutations=read("internal/household/routines/mutations.go");
const routineSchedule=read("internal/household/routines/schedule.go");

assert.match(day,/showActionableAppCalendarGroupPopup/,"day popup must delegate app-owned groups to the actionable renderer");
assert.match(event,/showActionableAppCalendarEvent/,"Agenda/day event cards must route owned events into the same action popup");
assert.match(actions,/showChoresCalendarActionPopup/,"Chores requires a dedicated quick-complete popup");
assert.match(actions,/showMaintenanceCalendarActionPopup/,"Maintenance requires a dedicated quick-complete popup");
assert.match(actions,/showRoutinesCalendarActionPopup/,"Routines requires a dedicated checklist popup");
assert.match(actions,/\/api\/chore-wheel\/day/,"Chores popup must use a narrow day projection");
assert.match(actions,/\/api\/chore-wheel\/assignments\/status/,"Chores popup must use the reversible durable status endpoint");
assert.match(actions,/completed:desired/,"Chore checkbox state must be sent as an explicit desired value");
assert.match(actions,/\/api\/maintenance\/day/,"Maintenance popup must use a narrow day projection");
assert.match(actions,/\/api\/maintenance\/tasks\/complete/,"Maintenance popup must use its narrow completion action");
assert.match(actions,/\/api\/maintenance\/tasks\/undo-complete/,"Maintenance completed rows must use the safe undo endpoint");
assert.doesNotMatch(actions,/const completed=\[\]/,"Maintenance completion must be server-authoritative rather than popup-memory-only");
assert.match(actions,/Done · tap again to reopen/,"completed rows must explain their reversible checkbox behavior");
assert.match(actions,/Complete routine/,"Routines popup needs a one-tap whole-session completion action");
assert.match(actions,/initialExpansion/,"Routines popup must auto-expand the first incomplete person");
assert.match(actions,/session\.actionable!==false&&session\.state!=="skipped"/,"future/skipped routine sessions must be read-only in the popup");
assert.match(css,/appgroup-action-row/,"quick actions need a dedicated non-button row surface");
assert.match(css,/appgroup-action-check input/,"native completion checkbox must receive an explicit touch target");
assert.match(choreRoutes,/\/api\/chore-wheel\/day/,"Chore router must serve the day projection");
assert.match(choreRoutes,/\/api\/chore-wheel\/assignments\/status/,"Chore router must handle reversible status");
assert.match(chore,/SetAssignmentCompleted/,"Chore service must support assigned/completed reversal");
assert.match(chore,/status == "assigned" \|\| status == "completed"/,"completed current chores must remain actionable");
assert.match(maintenanceHTTP,/\/api\/maintenance\/day/,"Maintenance router must serve day projection");
assert.match(maintenanceHTTP,/\/api\/maintenance\/tasks\/undo-complete/,"Maintenance router must expose completion undo");
assert.match(maintenanceMutations,/CanUndoCompletion/,"Maintenance service must verify latest safe completion before undo");
assert.match(maintenanceMutations,/lastCompletionHistoryId/,"Maintenance task state must identify the reversible completion");
assert.match(maintenance,/completedItems/,"Maintenance day response must return durable completed rows");
assert.match(routineMutations,/skipped routine sessions cannot be changed from the calendar/,"Routine service must reject skipped occurrence mutation");
assert.match(routineMutations,/future routine sessions cannot be completed from the calendar/,"Routine service must reject future occurrence mutation");
assert.match(routineSchedule,/copy\["actionable"\]/,"Routine day payload must expose action eligibility");
// Commit-frame content and cancellable reads (1.5.14). The calendar cell already
// named these rows, so they are painted pending instead of leaving an empty list
// while the day projection is in flight — but never as final state, and a
// completion the household asked for must never be cancelled by closing the popup.
assert.match(actions,/function appCalendarActionPendingRows/,"calendar rows already shown must be paintable while the day projection is in flight");
assert.match(actions,/status:"checking",actionable:false/,"preview rows must be explicitly pending and non-actionable, never final state");
assert.match(actions,/if\(status==="checking"\)return "Checking…"/,"preview rows must say what they are");
assert.match(actions,/showChoresCalendarActionPopup\(day,info,events\)/,"chores must receive the calendar rows to preview");
assert.match(actions,/showMaintenanceCalendarActionPopup\(day,info,events\)/,"maintenance must receive the calendar rows to preview");
assert.match(actions,/const pending=appCalendarActionPendingRows\(events,date\)/,"preview rows are painted during the commit frame");
assert.match(actions,/function appCalendarActionAbortable\(token\)/,"in-flight day reads must be cancellable");
assert.match(actions,/popupDefer\(token,ctx=>ctx.onCancel\(\(\)=>controller\.abort\(\)\)\)/,"cancellation must be wired to the popup close path");
assert.match(actions,/\/api\/chore-wheel\/day\?date="\+encodeURIComponent\(date\),null,read\)/,"the chore day read must carry the abort signal");
assert.match(actions,/\/api\/maintenance\/day\?date="\+encodeURIComponent\(date\),null,read\)/,"the maintenance day read must carry the abort signal");
assert.match(actions,/\/api\/routines\/day\?date="\+encodeURIComponent\(date\),null,read\)/,"the routine day read must carry the abort signal");
assert.doesNotMatch(actions,/assignments\/status",\{assignmentId:item\.assignmentId,date,completed:desired\},read\)/,"a completion write must never be aborted by closing the popup");
assert.match(actions,/error\.name==="AbortError"\)return/,"a cancelled read must not report a household-facing failure");
console.log("PASS: app-owned calendar popups use server-authoritative reversible completion, safe guards, and routine checklist actions");
