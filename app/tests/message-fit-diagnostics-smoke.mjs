#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
const root=process.argv[2]||path.resolve(path.dirname(new URL(import.meta.url).pathname),"..");
const fit=fs.readFileSync(path.join(root,"ui/js/messages-fit.js"),"utf8");
const lite=fs.readFileSync(path.join(root,"ui/js/messages-lite-fit.js"),"utf8");
const ctrl=fs.readFileSync(path.join(root,"ui/js/control-dashboard-typography.js"),"utf8");
assert.match(fit,/const COMP_FIT_DIAGNOSTICS=/);
assert.match(fit,/function complimentFitDiagnostics\(\)/);
assert.match(fit,/noteComplimentFitClipped\(fit\)/);
assert.match(fit,/COMP_FIT_DIAGNOSTICS\.corrected\+=1/);
assert.match(lite,/Keep observing parent\/sun\/stale instead/);
assert.match(ctrl,/complimentFitDiagnostics/);
assert.match(ctrl,/final safe clip this session/);
console.log("PASS: final message-fit clips are session-visible without rotation-time layout work");
