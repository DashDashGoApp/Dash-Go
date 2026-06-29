#!/usr/bin/env node
// Beta.13 keeps core as a thin route/transaction adapter while the bounded
// Routines package owns settings, item, and occurrence domain mutations.
import assert from "node:assert/strict";
import {readFileSync} from "node:fs";
import {resolve} from "node:path";

const root=resolve(process.argv[2]||".");
const read=relative=>readFileSync(resolve(root,relative),"utf8");
const lines=source=>source.split("\n").length;
const router=read("cmd/dashboard-control-server/routines_post.go");
const facade=read("cmd/dashboard-control-server/routines_facade.go");
const service=read("internal/household/routines/service.go");
const mutations=read("internal/household/routines/mutations.go");
const people=read("internal/household/routines/people.go");

for(const [name,source] of [["router",router],["facade",facade],["service",service],["mutations",mutations],["people",people]]){
  assert.ok(lines(source)<=400,`Routines ${name} source must remain below the Go navigability limit`);
}
assert.match(router,/case "\/api\/routines\/settings", "\/api\/routines\/items", "\/api\/routines\/occurrence":/,"router must retain the existing settings/item/occurrence endpoints");
assert.match(router,/routinespkg\.ApplySettings/,"Routines service must own settings mutation");
assert.match(router,/routinespkg\.ApplyItem/,"Routines service must own routine-item mutation");
assert.match(router,/routinespkg\.ApplyOccurrence/,"Routines service must own checklist/session mutation");
assert.match(router,/a\.householdService\(\)\.WithLock[\s\S]*a\.routinesService\(\)\.WithLock/,"legacy People action must retain People → Routines lock order");
assert.match(router,/result\.StateOnly[\s\S]*saveRoutinesStateOnly/,"ordinary checklist ticks must retain state-only save behavior");
assert.match(service,/routines\.json/,"Routines package must own its durable document path");
assert.match(service,/s\.mu\.Lock\(\)/,"Routines package must own its mutation lock");
assert.match(mutations,/case "steps":/,"Routines mutation package must retain batched checklist behavior");
assert.match(mutations,/future routine sessions cannot be completed from the calendar/,"Routines package must retain future-session guard");
assert.match(people,/func ReconcilePeople/,"Routines package must own canonical People reconciliation");
assert.match(facade,/internal\/household\/routines/,"core must use the bounded Routines child service");
console.log("PASS: Routines routes are thin adapters and domain mutations, persistence, and People reconciliation are bounded in internal/household/routines");
