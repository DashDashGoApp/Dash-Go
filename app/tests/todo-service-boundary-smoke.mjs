#!/usr/bin/env node
// Beta.14 isolates the whole local-first Microsoft To Do/Grocery lifecycle
// behind a narrow service boundary. Core keeps route/SSE adaptation only.
import assert from "node:assert/strict";
import {readFileSync} from "node:fs";
import {resolve} from "node:path";

const root=resolve(process.argv[2]||".");
const read=relative=>readFileSync(resolve(root,relative),"utf8");
const lines=source=>source.split("\n").length;
const main=read("cmd/dashboard-control-server/main.go");
const facade=read("cmd/dashboard-control-server/todo_facade.go");
const service=read("internal/todo/service.go");
const auth=read("internal/todo/todo_auth.go");
const inboundFile=read("internal/todo/todo_inbound.go");
const coordinator=read("internal/todo/todo_inbound_coordinator.go");
const inbound=inboundFile+"\n"+coordinator;
const store=read("internal/todo/todo_store.go");
const sync=read("internal/todo/todo_sync.go");
const migration=read("internal/todo/todo_migration.go");
const grocery=read("internal/todo/todo_grocery_memory.go");
const http=read("cmd/dashboard-control-server/todo_http.go");
const tasks=read("cmd/dashboard-control-server/todo_http_tasks.go");

for(const [name,source] of [["service",service],["auth",auth],["inbound",inboundFile],["inbound coordinator",coordinator],["store",store],["sync",sync],["migration",migration],["grocery",grocery],["facade",facade]]){
  assert.ok(lines(source)<=400,`${name} must remain below the Go navigability limit`);
}
assert.match(main,/todopkg "github\.com\/DashDashGoApp\/Dash-Go\/app\/internal\/todo"/,"core must construct the bounded To Do service");
assert.match(main,/todoInitMu[\s\S]*todo\s+\*todopkg\.Service[\s\S]*todoStreamMu[\s\S]*todoStreams/,"core may retain only lazy service and SSE adapter state");
assert.doesNotMatch(main,/todoCloudMu|todoInboundMu|todoListLocksMu|todoMigrationMu|todoArchiveMu|todoDraining|todoManualSyncUntil|todoAuthCancel|todoAuthState/,"core must not retain service-local To Do synchronization or auth state");
assert.match(facade,/todopkg\.New\(todopkg\.ServiceConfig/,"facade must construct internal/todo from narrow callbacks");
assert.match(facade,/LoadSettings:\s+a\.loadSettings[\s\S]*PeoplePayload:\s+a\.householdPeoplePayload[\s\S]*Emit:\s+a\.todoEmit/,"service configuration must pass narrow settings, People, and event seams");
assert.doesNotMatch(service,/github\.com\/DashDashGoApp\/Dash-Go\/app\/cmd|package main/,"internal/todo may not import core");
assert.match(service,/todoMu\s+sync\.Mutex[\s\S]*todoCloudMu\s+sync\.Mutex[\s\S]*todoInboundMu\s+sync\.Mutex[\s\S]*todoListLocksMu\s+sync\.Mutex[\s\S]*todoMigrationMu\s+sync\.Mutex/,"service must own durable/sync/auth/migration coordination locks");
assert.match(auth,/func \(a \*Service\) startTodoAuth/,"device authorization must be service-owned");
assert.match(inbound,/func \(a \*Service\) startTodoInboundScheduler/,"inbound scheduler must be service-owned");
assert.match(store,/func \(a \*Service\) readTodoListsIndex/,"local list/cache persistence must be service-owned");
assert.match(sync,/func \(a \*Service\) enqueueTodoOp/,"local-first queueing must be service-owned");
assert.match(migration,/func \(a \*Service\) migrateTodoSlot/,"migration/archive handling must be service-owned");
assert.match(grocery,/func \(a \*Service\) addTodoGroceryMemoryItem/,"Grocery Memory mutations must be service-owned");
assert.match(http,/case "\/api\/todo\/auth\/start"[\s\S]*?a\.startTodoAuth/,"HTTP adapter must preserve auth route behavior through the facade");
assert.match(tasks,/a\.upsertTodoTask|a\.patchTodoTask|a\.deleteTodoTask/,"task HTTP adapter must retain current facade contracts");
console.log("PASS: beta.14 keeps Microsoft To Do, queueing/recovery, and Grocery Memory inside internal/todo while core remains a route/SSE adapter");
