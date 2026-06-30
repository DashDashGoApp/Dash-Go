#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import {fileURLToPath} from "node:url";

const app=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const read=rel=>fs.readFileSync(path.join(app,rel),"utf8");
const patch=JSON.parse(read("tests/fixtures/default-message-catalog-patch.json"));
const RealDate=Date;
const fixedNow=new RealDate(2026,11,25,12,0,0,0);
function FixedDate(...args){ return new.target ? new RealDate(...(args.length?args:[fixedNow.getTime()])) : RealDate(...args); }
FixedDate.prototype=RealDate.prototype;
FixedDate.now=()=>fixedNow.getTime();
FixedDate.parse=RealDate.parse;
FixedDate.UTC=RealDate.UTC;
const context={
  Math, Number, String, Array, Object, Map, Set, console,
  Date:FixedDate, setTimeout, clearTimeout, __events:[]
};
context.globalThis=context;
vm.createContext(context);
vm.runInContext("var EVENTS=[]; var COMP_LIST=null;",context);
for(const rel of [
  "ui/js/config-defaults.js",
  "ui/js/config-default-messages.js",
  "ui/js/messages-holidays.js",
  "ui/js/messages-core.js"
]) vm.runInContext(read(rel),context,{filename:rel});

const catalog=vm.runInContext("CONFIG.compliments",context);
const runtime=vm.runInContext("({eligibleCompliments,holidayContextsForEvents})",context);
const norm=value=>String(value||"").trim().replace(/\s+/g," ").toLowerCase();
const ordinary=catalog.filter(entry=>!entry.holiday);
const holiday=catalog.filter(entry=>entry.holiday);
assert.equal(ordinary.length,202,"the JSON catalog patch must produce 202 ordinary/default messages");
assert.equal(holiday.length,46,"the curated holiday catalog must remain deliberately bounded");
assert.equal(catalog.length,248,"full default catalog count must stay explicit");
assert.equal(new Set(catalog.map(entry=>norm(entry.text))).size,catalog.length,"every default message needs an independent toggle key");
for(const removed of patch.remove) assert.ok(!ordinary.some(entry=>norm(entry.text)===norm(removed)),`removed default remained: ${removed}`);
for(const revision of patch.revise){
  const entry=ordinary.find(candidate=>norm(candidate.text)===norm(revision.to));
  assert.ok(entry,`revised default missing: ${revision.to}`);
  assert.ok((entry.legacyKeys||[]).some(key=>norm(key)===norm(revision.from)),`revision must preserve hide/edit state: ${revision.from}`);
}
for(const addition of patch.add) assert.ok(ordinary.some(entry=>norm(entry.text)===norm(addition.text)),`JSON addition missing: ${addition.text}`);
assert.ok(!holiday.some(entry=>entry.text==="Happy %holiday%!"),"broad cheerful legacy holiday template must not bypass curated rules");
assert.ok(holiday.some(entry=>entry.text.includes("Yom Kippur")&&entry.holidayTone==="solemn"),"solemn observance needs respectful wording");
assert.ok(!holiday.some(entry=>entry.holidayTone==="direct"&&(entry.holidayNames||[]).map(norm).includes("yom kippur")),"Yom Kippur must not receive an upbeat direct greeting");

const event=(title,url)=>({title,start:fixedNow.getTime(),cal:{url,tag:"holiday"}});
const poolFor=events=>{
  context.__events=events;
  vm.runInContext("EVENTS=globalThis.__events; COMP_LIST=null;",context);
  return runtime.eligibleCompliments();
};
const textPool=events=>poolFor(events).map(item=>item.text);
const share=items=>{
  const total=items.reduce((sum,item)=>sum+item.weight,0);
  return items.filter(item=>item.holiday).reduce((sum,item)=>sum+item.weight,0)/total;
};

const hanukkahEvent=event("Hanukkah","calendars/jewish-holidays.violet.holiday.ics");
const hanukkah=textPool([hanukkahEvent]);
assert.ok(hanukkah.includes("Happy Hanukkah — may your home be filled with light."),"selected Jewish calendar must unlock the curated Hanukkah greeting");
assert.ok(hanukkah.includes("Wishing a meaningful Hanukkah to those observing."),"selected Jewish calendar must unlock respectful neutral wording");
assert.ok(!hanukkah.some(text=>text.startsWith("Eid Mubarak")),"unselected Islamic greeting must never leak into a Jewish-only day");
assert.ok(Math.abs(share(poolFor([hanukkahEvent]))-.60)<1e-9,"major celebration holiday pool must target 60%");

const genericEvent=event("Hanukkah","calendars/family-holiday.ics");
const generic=textPool([genericEvent]);
assert.ok(generic.includes("Today is Hanukkah."),"a manually tagged holiday calendar still receives neutral acknowledgement");
assert.ok(!generic.includes("Happy Hanukkah — may your home be filled with light."),"direct Jewish greeting requires the installer-generated Jewish source layer");
assert.ok(Math.abs(share(poolFor([genericEvent]))-.40)<1e-9,"ordinary holiday pool must target 40%");

const duplicate=runtime.holidayContextsForEvents([
  event("Christmas Day","calendars/holidays.blue.holiday.ics"),
  event("Christmas Day","calendars/christian-holidays.gold.holiday.ics")
],fixedNow);
assert.equal(duplicate.length,1,"the same celebration from two subscribed calendars must not become a false overlap");
assert.deepEqual(Array.from(duplicate[0].layers).sort(),["christian","civil"],"same-day duplicate must retain its contributing layers");
const christmas=textPool([
  event("Christmas Day","calendars/holidays.blue.holiday.ics"),
  event("Christmas Day","calendars/christian-holidays.gold.holiday.ics")
]);
assert.ok(christmas.includes("Merry Christmas to everyone celebrating."),"curated greeting must work from an enabled exact matching source");

const overlap=textPool([
  event("New Year's Day","calendars/holidays.blue.holiday.ics"),
  event("Hanukkah","calendars/jewish-holidays.violet.holiday.ics")
]);
assert.ok(overlap.includes("Wishing a meaningful day to everyone observing today."),"distinct simultaneous occasions need inclusive wording");
assert.ok(!overlap.includes("Happy New Year — here’s to a good year ahead."),"overlap must suppress one-occasion direct greetings");
assert.ok(!overlap.includes("Happy Hanukkah — may your home be filled with light."),"overlap must suppress competing direct religious greetings");
assert.ok(!overlap.some(text=>text.includes("Happy Hanukkah")||text.includes("Happy New Year")),"overlap must remain inclusive rather than choosing one celebration");

const yomKippur=textPool([event("Yom Kippur","calendars/jewish-holidays.violet.holiday.ics")]);
assert.ok(yomKippur.includes("Wishing an easy and meaningful Yom Kippur to those observing."),"solemn holiday must retain its specific respectful wording");

const holidaySource=read("ui/js/messages-holidays.js");
const coreSource=read("ui/js/messages-core.js");
assert.match(holidaySource,/holidayContextsForEvents/,"holiday contexts must be event-derived");
assert.match(holidaySource,/HOLIDAY_LAYER_SOURCES/,"installer layer filenames must be mapped explicitly");
assert.match(coreSource,/applyHolidayMessageShare/,"holiday share policy must remain centralized");
assert.doesNotMatch(holidaySource,/getMonth\(\).*holiday|holiday.*getMonth\(/i,"holiday eligibility must not fall back to a date guess");

console.log("PASS: JSON default catalog migration, exact installer-layer holiday eligibility, neutral/direct/solemn wording, inclusive overlaps, and 40/60 holiday rotation shares");
