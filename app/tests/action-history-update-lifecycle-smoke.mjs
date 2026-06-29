#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import {fileURLToPath} from "node:url";

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const read=rel=>fs.readFileSync(path.join(root,rel),"utf8");
const ui=read("ui/js/control-cache.js");
const history=read("cmd/dashboard-control-server/action_history.go");
const route=read("cmd/dashboard-control-server/http_routes_post.go");
const runner=read("bin/dashboard-update-runner.sh");

assert.match(ui,/state==="rolledback"\) return "Rolled back"/,"Recent Actions must name a verified rollback");
assert.match(ui,/state==="unknown"\) return "Outcome unknown"/,"legacy unfinalized updates must not look live forever");
assert.match(history,/"updateJobId"/,"new update rows must carry a durable job identity in metadata");
assert.match(history,/func \(a \*app\) recordUpdateAction\(/,"Dashboard Control must create one linked update row before runner launch");
assert.match(history,/func finalizeUpdateActionHistoryFile\(/,"the updater must be able to finalize the existing row");
assert.match(history,/func reconcileUpdateActionHistoryFile\(/,"server startup/history reads must reconcile an interrupted finalizer");
assert.doesNotMatch(history,/legacyUpdateOutcomeUnknownMsg/,"current reconciliation must use durable update job IDs rather than text-matching unsupported legacy rows");
assert.match(route,/errors\.Is\(err, errDashboardUpdateRunning\)[\s\S]*http\.StatusConflict[\s\S]*else if !updateActionAlreadyRecorded\(err\)/,"duplicate active update requests must not append another action row");
assert.doesNotMatch(route,/a\.recordAction\("update", "Update dashboard", "running"/,"successful update launch must use the job-linked record instead of a legacy running row");
assert.match(runner,/ACTION_HISTORY=.*action-history\.json/,"runner needs the durable action history path");
assert.match(runner,/--finalize-update-action --history "\$ACTION_HISTORY" --job "\$JOB"/,"runner must finalize the matching linked row after terminal job persistence");
const duplicateRunnerBlock=runner.slice(runner.indexOf("if ! flock -n 9; then"),runner.indexOf("if [ ! -x \"$INSTALLER\" ]; then"));
assert.doesNotMatch(duplicateRunnerBlock,/job --state failed/,"rejected duplicate runners must not corrupt the live job/action row");

console.log("Action-history update lifecycle smoke: linked terminal updates and durable-ID reconciliation contracts ok");
