#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import {fileURLToPath} from "node:url";

const app=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const read=p=>fs.readFileSync(path.join(app,p),"utf8");
const index=read("index.html");
const core=read("ui/js/control-core.js");
const nav=read("ui/js/control-navigation.js");
const calendarDiagnostics=read("ui/js/control-calendars-logs.js");
const consoleCss=read("ui/css/control/console-shell-tabs.css");
const baseCss=read("ui/css/dashboard/control-overlay.css");
const getRoutes=read("cmd/dashboard-control-server/http_routes_get.go");
const systemUpdate=read("ui/js/control-system-actions.js");

// The retired generic Logs card must leave no dormant DOM, lazy route, renderer,
// cleanup target, or Logs-only CSS behind in Dashboard Control.
for(const forbidden of ["ctrlsec-logs","data-lazy=\"logs\"","ctrllogs","ctrllogout"]){
  assert.ok(!index.includes(forbidden),`retired Logs markup survived: ${forbidden}`);
  assert.ok(!core.includes(forbidden),`retired Logs core reference survived: ${forbidden}`);
}
assert.ok(!nav.includes('case "logs"')&&!nav.includes("renderCtrlLogs"),"retired Logs route survived");
assert.ok(!calendarDiagnostics.includes("function renderCtrlLogs"),"retired Logs renderer survived");
assert.ok(!consoleCss.includes("ctrllogout")&&!consoleCss.includes("ctrlsec-logs"),"Logs-only console CSS survived");
assert.ok(!baseCss.includes("ctrllogout"),"Logs-only base CSS survived");

// Diagnostics remains the intentional support surface and names the private
// collection location accurately. The API remains only for the System update
// card's targeted update-log action; removing the generic Logs card must not
// break that existing action.
assert.match(index,/data-lazy="diagnostics"[\s\S]*id="ctrldiag"/,"Diagnostics card must remain available");
assert.match(calendarDiagnostics,/Export diagnostics/,"Diagnostics export must remain available");
assert.match(calendarDiagnostics,/~\/\.dashboard-diagnostics\//,"Diagnostics export must name the private collection directory");
assert.match(getRoutes,/case "\/api\/logs"/,"targeted log API must remain for System update");
assert.match(systemUpdate,/\/api\/logs\?name=system-update/,"System update log action must remain targeted and usable");
console.log("PASS: Dashboard Control retires generic Logs while retaining Diagnostics export and the targeted System update log");
