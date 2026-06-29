#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import {fileURLToPath} from "node:url";

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const read=p=>fs.readFileSync(path.join(root,p),"utf8");
const runtime=read("ui/js/config-runtime.js");
const seasonal=read("ui/js/calendar-seasonal-decor.js");
const settings=read("ui/js/settings-runtime.js");

function functionBody(source,name){
  const start=source.indexOf(`function ${name}(`);
  assert.notEqual(start,-1,`missing function ${name}`);
  const open=source.indexOf("{",start),endMarker="\n}\n";
  let depth=0;
  for(let i=open;i<source.length;i++){
    if(source[i]==="{")depth++;
    else if(source[i]==="}"&&!--depth)return source.slice(start,i+1);
  }
  throw new Error(`unterminated function ${name}: ${endMarker}`);
}

const themeBody=functionBody(runtime,"applyTheme");
assert.ok(runtime.includes("function dashboardRuntimeSettings"),"early runtime settings bridge is present");
assert.ok(settings.includes("function syncDashboardRuntimeSettings"),"settings bridge stays synchronized");
assert.ok(seasonal.includes("function scheduleSeasonalDecorReconcile"),"decor reconciliation is deferred");
assert.ok(!themeBody.includes("seasonalDecorSignature"),"early applyTheme never reads seasonal settings directly");
assert.ok(!themeBody.includes("applySeasonalDecor"),"early applyTheme never patches decor synchronously");

function classList(){return {toggle(){},add(){},remove(){}};}
function style(){return {textContent:"",setProperty(k,v){this[k]=v;},removeProperty(k){delete this[k];}};}
const themeStyle=style(),rootNode={style:style(),classList:classList(),setAttribute(k,v){this[k]=v;},getAttribute(k){return this[k]||"";}};
const frames=[];
const context=vm.createContext({
  console,
  THEMES:{basic:{"--bg":"#111"},paper:{"--bg":"#eee","--fg":"#111"}},
  CONFIG:{profile:"lite",seasonalDecor:"off",fontPreset:"default",weatherIconStyle:"soft"},
  window:{DASHBOARD_LOCAL:{theme:"paper"}},
  location:{search:""},
  URLSearchParams,
  document:{
    head:{appendChild(){}},
    body:{style:style()},
    documentElement:rootNode,
    getElementById:id=>id==="dashboard-theme-vars"?themeStyle:null,
    createElement:()=>themeStyle,
    querySelector:()=>null
  },
  requestAnimationFrame(fn){frames.push(fn);return frames.length;},
  cancelAnimationFrame(){},
  setTimeout,clearTimeout,
  fetch:async()=>({ok:false})
});

// The production bundle is one ordered classic script. The early config-local
// call executes while the later lexical SETTINGS declaration is still in TDZ.
// This must complete without touching that binding; deferred decor work runs
// only after settings initializes and mirrors itself to window.
assert.doesNotThrow(()=>vm.runInContext(`${runtime}\n${seasonal}\n${settings}\nglobalThis.__themeBoot={theme:CURRENT_THEME,settings:window.DASHGO_RUNTIME_SETTINGS};`,context),"bundle startup must survive early theme application");
while(frames.length)frames.shift()();
assert.equal(context.__themeBoot.theme,"paper","config-local theme applies during startup");
assert.equal(context.__themeBoot.settings.seasonalDecor,"off","settings bridge is available after initialization");
assert.equal(themeStyle.textContent.includes("--bg:#eee"),true,"theme variables commit atomically before boot");

console.log("PASS: theme bootstrap avoids SETTINGS TDZ and defers seasonal decor safely");
