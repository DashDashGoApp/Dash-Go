#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import {fileURLToPath} from "node:url";

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const read=rel=>fs.readFileSync(path.join(root,rel),"utf8");
const index=read("index.html");
const control=read("ui/js/control-todo.js");
const navigation=read("ui/js/control-navigation.js");
const todoCSS=read("ui/css/control/todo.css");

assert.match(index,/<div id="ctrltodo" data-todo-control-root aria-live="polite"><\/div>/,"Lists Control must retain one stable semantic content root");
assert.match(control,/function todoControlRoot\(\)\{\s*return document\.querySelector\("#ctrlpage-control \[data-todo-control-root\]"\);/,"Lists renderer must scope its semantic root to the Control page");
assert.doesNotMatch(control,/\$\("ctrltodo"\)/,"Lists renderer must not query a nonexistent <ctrltodo> element");
assert.match(control,/wrap\.replaceChildren\(ctrlStateCard\("loading","Loading Lists"/,"Lists renderer must paint a bounded loading state before requesting status");
assert.match(control,/Lists status unavailable/,"Lists renderer must replace request failures with a visible error state");
assert.match(control,/const seq=\+\+CTRL_TODO_RENDER_SEQ;/,"Lists renderer must reject stale asynchronous status responses");
assert.match(navigation,/case "todo": await renderCtrlTodo\(\); break;/,"Control lazy dispatcher must invoke the Lists renderer");
assert.match(todoCSS,/\.todo-map-select\{[^}]*appearance:none[^}]*-webkit-appearance:none[^}]*background-color:var\(--card\)!important[^}]*color:var\(--fg\)!important[^}]*-webkit-text-fill-color:var\(--fg\)/,"List destination select must use native-safe theme tokens in WebKit");
assert.match(todoCSS,/\.todo-map-select option,\.todo-map-select optgroup\{background-color:var\(--bg\);color:var\(--fg\);-webkit-text-fill-color:var\(--fg\)\}/,"List destination options must inherit theme foreground/background tokens");

const selectors=[];
const calls=[];
const wrap={
  replaceChildren(...nodes){calls.push({kind:"replace",nodes});},
  appendChild(node){calls.push({kind:"append",node});},
};
const context={
  document:{querySelector(selector){selectors.push(selector); return selector==="#ctrlpage-control [data-todo-control-root]"?wrap:null;}},
  ctrlStateCard:(kind,title,detail,actions)=>({kind,title,detail,actions}),
  cbtn:(label,cls,handler)=>({label,cls,handler}),
  api:async()=>{throw new Error("loopback status unavailable");},
};
vm.createContext(context);
vm.runInContext(control,context,{filename:"control-todo.js"});
await vm.runInContext("renderCtrlTodo()",context);

assert.deepEqual(selectors,["#ctrlpage-control [data-todo-control-root]","#ctrlpage-control [data-todo-control-root]"],"renderer must consistently resolve the semantic Lists root");
assert.equal(calls.length,2,"failed status fetch must leave a visible loading-to-error transition, never an empty card");
assert.equal(calls[0].kind,"replace");
assert.equal(calls[0].nodes[0].title,"Loading Lists");
assert.equal(calls[1].kind,"replace");
assert.equal(calls[1].nodes[0].title,"Lists status unavailable");
assert.ok(Array.isArray(calls[1].nodes[0].actions)&&calls[1].nodes[0].actions.length===1,"error state must provide a retry action");

console.log("todo-control-card smoke passed");
