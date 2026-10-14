import assert from "node:assert/strict";
import fs from "node:fs";
const fit=fs.readFileSync("ui/js/messages-fit.js","utf8");
const lite=fs.readFileSync("ui/js/messages-lite-fit.js","utf8");
const ctrl=fs.readFileSync("ui/js/control-dashboard-typography.js","utf8");
// The fit diagnostics stay on the session counters: they record clipped fits,
// corrected fits, and dropped unreadable messages (counts only, never message
// text) so the owner can see whether the readable-size contract is doing work.
assert.match(fit,/const COMP_FIT_DIAGNOSTICS=\{clipped:0,corrected:0,lastClippedAt:0,dropped:0,lastDroppedAt:0\}/,"fit diagnostics must track clipped, corrected, and dropped-unreadable sessions");
assert.match(fit,/function complimentFitDiagnostics\(\)\{\s*return \{\.\.\.COMP_FIT_DIAGNOSTICS\};\s*\}/,"diagnostics must expose the full session counter object (clipped, corrected, dropped)");
assert.match(fit,/function noteComplimentDropped\(\)/,"a dropped unreadable message must be recorded");
assert.match(fit,/COMP_FIT_DIAGNOSTICS\.dropped\+=1/,"a drop must increment the dropped counter");
assert.match(fit,/noteComplimentFitClipped\(fit\);/,"a clipped fit must still be recorded");
assert.match(fit,/COMP_FIT_DIAGNOSTICS\.corrected\+=1/,"a corrected rendered fit must be recorded");
assert.match(lite,/Keep observing parent\/sun\/stale instead/,"Lite keeps observing the footer geometry, not rotating text");
assert.match(ctrl,/complimentFitDiagnostics/,"Control must surface the fit diagnostics in the editor note");
assert.match(ctrl,/final safe clip this session/,"Control must report clipped fits");
assert.match(ctrl,/was dropped this session/,"Control must report dropped unreadable messages");
console.log("PASS: message fit diagnostics");
