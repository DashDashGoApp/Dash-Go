import assert from "node:assert/strict";
import {readFileSync} from "node:fs";
import {resolve} from "node:path";
const root=resolve(process.argv[2]||".");
const read=rel=>readFileSync(resolve(root,rel),"utf8");
const manifest=read("ui/js/bundle.manifest.json");
const ui=read("ui/js/control-private-calendars.js");
const routes=read("cmd/dashboard-control-server/http_routes_post_calendar.go");
const controls=read("cmd/dashboard-control-server/private_calendar_controls.go");
const mutations=read("cmd/dashboard-control-server/private_calendar_selection_controls.go");
const server=controls+"\n"+mutations;
const discover=read("bin/private-calendar-discovery.sh");
const select=read("bin/private-calendar-selection.sh");

assert.match(manifest,/"control-calendar-writeback\.js"[\s\S]*?"control-private-calendars\.js"/,'private-calendar controls must load after calendar writeback controls');
assert.match(ui,/Discover available calendars/,'Calendar Manager must expose a user-led discovery action');
assert.match(ui,/Nothing discovered here syncs or changes until you add it/,'discovery UI must state its non-destructive boundary');
assert.match(ui,/legacy all-calendars mirror/,'selection UI must warn before a preserved broad mirror can duplicate exact sources');
for(const endpoint of ["/api/calendars/private/discover","/api/calendars/private/activate","/api/calendars/private/editable","/api/calendars/private/deactivate","/api/calendars/private/sync"]){
  assert.match(ui,new RegExp(endpoint.replace(/[/.]/g,"\\$&")),'private calendar UI must use '+endpoint);
  assert.match(routes,new RegExp(endpoint.replace(/[/.]/g,"\\$&")),'server must route '+endpoint);
}
assert.match(server,/private-calendar-discovery\.sh/,'server discovery must invoke the isolated helper');
assert.match(server,/--set-editable/,'server must support changing a selected calendar between display-only and editable');
assert.match(server,/queueCalendarWritebackSync\(selection\.Source, selection\.Pair\)/,'manual sync must target one selected pair');
assert.match(discover,/mktemp -d/,'discovery must allocate a disposable workspace');
assert.match(discover,/trap .*rm -rf/,'discovery workspace must be cleaned');
assert.match(discover,/discover "\$stage_pair"/,'discovery helper must perform vdirsyncer discovery');
assert.doesNotMatch(discover,/sync-vdir\.sh/,'discovery helper must not invoke regular synchronization');
assert.doesNotMatch(discover,/>[[:space:]]*"\$VDIR_PAIRS"/,'discovery helper must not rewrite active pair state');
assert.match(select,/--set-editable/,'selection helper must support later edit-permission changes');
assert.match(select,/local_id="collection_/,'selection helper must generate a safe local vdir key');
assert.doesNotMatch(ui,/(?:token_file|client_secret|password\.fetch|google-tokens)/,'browser control source must not contain provider secret details');
assert.doesNotMatch(ui,/window\.(?:alert|confirm|prompt)\s*\(/,'private calendar controls may not use browser dialogs');
console.log("private calendar controls smoke: discovery, explicit selection, targeted management, and secret boundaries hold");
