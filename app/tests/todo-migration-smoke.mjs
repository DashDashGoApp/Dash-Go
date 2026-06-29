#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import {fileURLToPath} from "node:url";

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const read=rel=>fs.readFileSync(path.join(root,rel),"utf8");
const lists=read("ui/lists-core.js")+"\n"+read("ui/lists-actions.js");
const control=read("ui/js/control-todo.js");
const types=read("internal/todo/todo_types.go");
const sync=read("internal/todo/todo_sync.go");
const store=read("internal/todo/todo_store.go");
const migration=read("internal/todo/todo_migration.go");
const routes=read("cmd/dashboard-control-server/todo_http.go");

assert.match(lists,/function listItemLabel\(id\)[\s\S]*?\$\{listTitle\(id\)\} Item/,"add/edit labels must derive from the active list title");
assert.match(lists,/add\.textContent=`Add \$\{listItemLabel\(LISTS_STATE\.active\)\}`/,"the primary action must read Add {List Title} Item");
assert.match(lists,/promptText\(`Add \$\{listItemLabel\(LISTS_STATE\.active\)\}`/,"the add prompt must use the same list-aware title");
assert.match(lists,/promptText\(`Edit \$\{listItemLabel\(LISTS_STATE\.active\)\}`/,"the edit prompt must use the same list-aware title");
assert.match(lists,/function deleteListItemLabel\(id\)[\s\S]*?`Delete \$\{listItemLabel\(id\)\}`/,"delete wording must derive from the active list title");
assert.match(lists,/edit\.textContent="Edit"/,"row actions must expose a visible Edit action");
assert.match(lists,/del\.textContent="Remove"/,"row deletion action must expose a visible Remove action");
assert.match(lists,/function confirmDeleteTask\(task\)[\s\S]*?`\$\{deleteListItemLabel\(active\)\}\?`[\s\S]*?`Keep \$\{itemLabel\}`[\s\S]*?deleteListItemLabel\(active\)/,"delete confirmation must use list-aware delete and keep labels");
assert.doesNotMatch(lists,/\+ Add task|Delete this task\?|Keep task|Remove Task/,"generic task deletion wording must not return");

assert.match(types,/Origin\s+string\s+`json:"origin,omitempty"`/,"list metadata must carry an explicit origin");
assert.match(store,/func \(a \*Service\) todoListCloudSyncEnabled\(listID string\)/,"Graph work must check list origin as well as account linkage");
assert.match(sync,/func \(a \*Service\) enqueueTodoOp[\s\S]*?todoListCloudSyncEnabled\(op\.ListID\)/,"local lists must never queue Graph writes just because an account is linked");
assert.match(control,/Copy local items to Microsoft[\s\S]*?Archive local \/ fresh Microsoft/,"linked local mappings need explicit copy/fresh migration choices");
assert.match(control,/Copy Microsoft items to local[\s\S]*?Archive Microsoft list \/ fresh local/,"unlinking needs explicit reverse migration choices");
assert.match(control,/90 days/,"migration retention must be visible before action");
assert.match(routes,/"\/api\/todo\/migrate": true/,"migration must remain PIN-managed");
assert.match(routes,/case "\/api\/todo\/migrate"/,"migration endpoint must remain routable");
assert.match(migration,/todoMigrationArchiveRetention = 90 \* 24 \* time\.Hour/,"archive retention must be exactly 90 days");
assert.match(migration,/func \(a \*Service\) startTodoArchiveJanitor\(/,"archive cleanup must run automatically");
assert.match(migration,/Microsoft list itself is left untouched|never deletes a Microsoft list/,"unlink migration must not silently delete the cloud source list");
assert.match(migration,/permanently retires the source from Dash-Go mappings/,"expired snapshots must not cause archived source lists to return");
assert.match(migration,/mapped to both launcher tiles/,"migration must refuse to orphan a second mapping that points at the same source list");
assert.match(migration,/microsoftToLocal && !a\.todoHasMappedMicrosoftList\(\)/,"unlink must wait until every Microsoft-mapped tile has returned local");

console.log("todo migration source smoke passed");
