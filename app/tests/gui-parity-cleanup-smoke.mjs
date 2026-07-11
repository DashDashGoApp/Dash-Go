#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import {fileURLToPath} from "node:url";

const appRoot=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const read=relative=>fs.readFileSync(path.join(appRoot,relative),"utf8");

const configRuntime=read("ui/js/config-runtime.js");
const configLocal=read("ui/js/config-runtime.js");
const weather=read("ui/js/weather-sources.js");
const dataSources=read("ui/js/data-sources.js");
const dataRefresh=read("ui/js/data-refresh.js");
const messageSchedule=read("ui/js/messages-schedule.js");
const settingsRuntime=read("ui/js/settings-runtime.js");
const runtimeAssets=read("cmd/dashboard-control-server/runtime_assets.go");
const launcher=read("ui/js/app-launcher.js");
const household=read("ui/js/household-app-loader.js");
const tap=read("ui/js/tap.js");
const sharedOSK=read("ui/js/shared-osk.js");
const mapKeyboard=read("ui/js/map-keyboard-controls.js");
const controlApi=read("ui/js/control-api.js");
const controlCore=read("ui/js/control-core.js");
const controlMemory=read("ui/js/control-lite-memory.js");
const controlCSS=read("ui/css/control/console-shell-tabs.css");
const httpServer=read("cmd/dashboard-control-server/http_server.go");

// Local mutable resources already use request cache:no-store and server-side
// Cache-Control:no-store. Stable URLs must not acquire unique timestamp keys.
for(const [name,source,pattern] of [
  ["config.local",configLocal,/fetch\(path\+"\?t="\+Date\.now\(\)/],
  ["weather",weather,/\/api\/weather\?t=/],
  ["calendar manifest",dataSources,/calendars\/calendars\.json\?t=/],
  ["ICS source",dataRefresh,/cal\.url\+"\?t="/],
  ["message schedule",messageSchedule,/path\+"\?t="/],
  ["settings",settingsRuntime,/config\/settings\.json\?t=/],
]) assert.doesNotMatch(source,pattern,`${name} must keep a stable no-store URL`);
assert.match(configLocal,/fetch\(path,\{cache:"no-store"\}\)/,"config.local body request remains no-store");
assert.match(dataRefresh,/fetch\(cal\.url,\{cache:"no-store"\}\)/,"ICS requests remain no-store");

// CSS generation must use the same deterministic source-owned minifier model
// as JavaScript while retaining split CSS as the editable source.
assert.match(runtimeAssets,/minifycss "github\.com\/tdewolff\/minify\/v2\/css"/,"runtime asset generator imports the CSS minifier");
assert.match(runtimeAssets,/minifier := minifycss\.Minifier\{\}/,"CSS generation uses deterministic minification");
assert.match(runtimeAssets,/minify generated %s/,"CSS minification errors remain fail-closed");
assert.doesNotMatch(runtimeAssets,/fmt\.Fprintf\(&b, "\\n\/\* ---- %s ---- \*\//,"generated CSS must not retain split-source comment separators");

// One loader owns retry, ready, failed, and tag cleanup for all lazy apps.
assert.match(launcher,/function loadDashboardLazyAsset\(kind,src,dataName,errorMessage\)/,"one shared lazy asset loader is required");
assert.match(launcher,/prior&&prior\.dataset\.failed==="1"\)prior\.remove\(\)/,"failed lazy tags are discarded before retry");
assert.match(launcher,/node\.dataset\.failed="1";node\.remove\(\);reject\(/,"failed lazy tags are removed on error");
assert.doesNotMatch(household,/function appendLazy(?:Script|Style)\(/,"household apps must not duplicate lazy asset implementations");
for(const asset of ["chore-wheel.css","family-board-core.js","maintenance.js","routines.js"]){
  assert.ok(household.includes(`loadDashboardLazyAsset(`)&&household.includes(asset),`${asset} must use the shared lazy loader`);
}

// Lite aliases are declared once and every subsystem delegates to that helper.
assert.equal((configRuntime.match(/"lite","zero2","low","low-power"/g)||[]).length,1,"Lite aliases must have one canonical declaration");
for(const relative of ["boot.js","control-core.js","control-lazy-loader.js","dashboard-fit.js","day-popup.js","event-cache.js","messages-fit.js","radar-sources.js","settings-runtime.js"]){
  const source=read(`ui/js/${relative}`);
  assert.doesNotMatch(source,/\["lite","zero2","low","low-power"\]/,`${relative} must not redeclare Lite aliases`);
  assert.match(source,/dashboardLiteProfile\(/,`${relative} must delegate Lite classification`);
}

// Destructive confirmations share one timer owner while retaining per-call
// durations and render callbacks.
assert.match(tap,/const DASH_CONFIRM_TIMERS=new WeakMap\(\)/,"confirmation timers must be weakly keyed by their button");
assert.match(tap,/function dashConfirmTap\(button,options\)/,"shared two-tap confirmation helper is required");
assert.match(tap,/if\(!button\.isConnected\)return;/,"detached buttons must not be mutated by expiry timers");
for(const relative of ["control-calendars.js","control-content-osk.js","control-special-feeds.js","control-terminal-profile.js","managed-schedules-popup.js"]){
  assert.match(read(`ui/js/${relative}`),/dashConfirmTap\(/,`${relative} must use the shared confirmation lifecycle`);
}

// Both keyboards use the same immutable rows but retain separate state and
// controls around those rows.
assert.match(sharedOSK,/const DASH_OSK_LETTER_ROWS=Object\.freeze\(/,"shared letter rows are required");
assert.match(sharedOSK,/const DASH_OSK_SYMBOL_ROWS=Object\.freeze\(/,"shared symbol rows are required");
assert.match(sharedOSK,/return _oskLayer==="symbols"\?DASH_OSK_SYMBOL_ROWS:DASH_OSK_LETTER_ROWS;/,"main OSK uses shared rows");
assert.match(mapKeyboard,/return _mapOskLayer==="symbols"\?DASH_OSK_SYMBOL_ROWS:DASH_OSK_LETTER_ROWS;/,"map OSK uses shared rows");
assert.doesNotMatch(mapKeyboard,/const letterRows=|const symbolRows=/,"map keyboard must not duplicate row data");

// Control cache equality must ignore object member order while preserving array
// order, primitive type distinctions, and nested values.
assert.match(controlApi,/function jsonValueEqual\(left,right\)/,"Control needs semantic JSON equality");
assert.doesNotMatch(controlApi,/JSON\.stringify\(had\)===JSON\.stringify\(fresh\)/,"Control must not compare cached JSON by serialization order");
const equalitySource=controlApi.match(/function jsonValueEqual\(left,right\)\{[\s\S]*?\n\}/)?.[0];
assert.ok(equalitySource,"semantic equality helper source is extractable");
const equalityContext=vm.createContext({Object,Array});
vm.runInContext(`${equalitySource}\nglobalThis.eq=jsonValueEqual;`,equalityContext);
const eq=equalityContext.eq;
assert.equal(eq({a:1,b:{c:[2,3]}},{b:{c:[2,3]},a:1}),true,"object member order is not semantic");
assert.equal(eq([1,2],[2,1]),false,"array order remains semantic");
assert.equal(eq({a:1},{a:"1"}),false,"primitive types remain semantic");
assert.equal(eq({a:null},{a:{}}),false,"null remains distinct from an object");

// Major Control shells use semantic hidden state. CSS owns display geometry.
assert.doesNotMatch(launcher,/trigger\.style\.display/,"App Launcher trigger display is CSS-owned");
assert.doesNotMatch(controlCore,/\.style\.display=/,"Control output console visibility uses hidden state");
assert.match(controlCore,/c\.wrap\.hidden=false;[\s\S]*?c\.pre\.hidden=false;/,"showing output clears hidden state");
assert.match(controlCore,/pre\.hidden=true;[\s\S]*?wrap\.hidden=true;/,"hiding output restores hidden state");
assert.match(controlMemory,/\.ctrloutputconsole:not\(\[hidden\]\)/,"Lite retention detects visible semantic consoles");
assert.match(controlCSS,/\.ctrloutputconsole\[hidden\],[\s\S]*?pre\[hidden\]\{display:none!important;\}/,"Control CSS owns hidden console display");
for(const relative of ["control-content-osk.js","control-special-feeds.js","control-message-schedules.js"]){
  assert.doesNotMatch(read(`ui/js/${relative}`),/(?:top|actions)\.style\.display/,`${relative} editor shells must use hidden state`);
}

// Static aliases are fixed compile-time routes, not a map allocated on every
// browser asset request.
assert.doesNotMatch(httpServer,/aliases := map\[string\]string/,"static request path must not allocate an alias map");
assert.match(httpServer,/switch requestPath \{[\s\S]*?case "\/dashboard\.css":[\s\S]*?case "\/control-layout\.css":[\s\S]*?case "\/dashboard\.js":/,"the three legacy static aliases remain exact");

console.log("PASS: beta.1 GUI parity cleanup keeps URLs stable, assets deterministic, shared lifecycles centralized, semantic state explicit, and visible behavior unchanged");
