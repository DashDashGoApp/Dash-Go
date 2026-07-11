#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import {fileURLToPath} from "node:url";
const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const read=rel=>fs.readFileSync(path.join(root,rel),"utf8");
const cache=read("ui/js/event-cache.js");
const parser=read("ui/js/ics-parser.js");
const server=read("cmd/dashboard-control-server/http_server.go");
const facade=read("cmd/dashboard-control-server/events_facade.go");
const eventCache=read("internal/calendar/events/cache.go")+read("internal/calendar/events/serialize.go");
const eventSources=read("internal/calendar/events/sources.go");
const eventTypes=read("internal/calendar/events/types.go");
assert.match(cache,/If-None-Match/,'browser must conditionally fetch the event cache');
assert.match(cache,/res\.status===304/,'browser must reuse parsed events on a 304 response');
assert.match(cache,/EVENT_CACHE_WINDOW_START>\+winStart\|\|EVENT_CACHE_WINDOW_END<\+winEnd/,'304 reuse must not serve a parsed cache outside its original coverage');
assert.match(cache,/cache\.version!==11/,'browser must reject pre-beta.2 event caches');
assert.match(cache,/event\.desc,event\.location/,'event signature must hash full description and location content');
assert.doesNotMatch(cache,/\(e\.location\|\|""\)\.length|\(e\.desc\|\|""\)\.length/,'same-length event edits must not be missed');
assert.match(server,/rel == "cache\/events\.cache\.json"/,'server must issue mutable event-cache revisions');
assert.match(server,/http\.StatusNotModified/,'server must return 304 for matching event-cache ETags');
assert.match(facade,/func \(a \*app\) currentEventCacheWindow\(\)/,'cache coverage must derive from active Calendar and Agenda settings');
assert.match(facade,/daysPast, daysFuture := a\.currentEventCacheWindow\(\)/,'ordinary CLI and cron generation must use the settings-aware cache window');
assert.match(facade,/weeksAbove\*7\+14/);
assert.match(facade,/max\(\(weeksBelow\+1\)\*7, agendaDays\)\+7/);
assert.match(eventCache,/acquireEventCacheLock/,'cache generation needs a cross-process lock');
assert.match(eventCache,/refreshMu\.TryLock/,'cache generation needs in-process coalescing');
assert.match(eventSources,/prior\.MtimeNs/,'source digests must be reusable from nanosecond stat identity');
assert.match(eventTypes,/const CacheVersion = 11/);
assert.match(eventTypes,/const FingerprintVersion = 9/);
assert.match(parser,/setDate\(end\.getDate\(\)\+duration\.days\)/,'nominal day durations must preserve local wall time across DST');

const context=vm.createContext({console,Date,Set,Map,Number,String,Math});
vm.runInContext(parser,context,{filename:"ics-parser.js"});
const parsed=vm.runInContext(`parseICS(
  "BEGIN:VCALENDAR\\n"+
  "BEGIN:VEVENT\\nUID:rev\\nSEQUENCE:1\\nDTSTART:20260712T090000Z\\nSUMMARY:Old\\nEND:VEVENT\\n"+
  "BEGIN:VEVENT\\nUID:rev\\nSEQUENCE:2\\nDTSTART:20260712T090000Z\\nDURATION:PT90M\\nSUMMARY:New\\nEND:VEVENT\\n"+
  "BEGIN:VEVENT\\nUID:gone\\nSTATUS:CANCELLED\\nDTSTART:20260712T100000Z\\nSUMMARY:Gone\\nEND:VEVENT\\n"+
  "END:VCALENDAR\\n", {url:"calendars/test.ics"})`,context);
assert.equal(parsed.length,1,'browser fallback must reconcile revisions and remove cancelled standalone events');
assert.equal(parsed[0].title,'New');
assert.equal(+parsed[0].end-+parsed[0].start,90*60*1000,'browser fallback must honor DURATION');
const recurring=vm.runInContext(`parseICS(
  "BEGIN:VCALENDAR\\n"+
  "BEGIN:VEVENT\\nUID:series\\nDTSTART:20260706T090000Z\\nRRULE:FREQ=WEEKLY;COUNT=3\\nSUMMARY:Series\\nEND:VEVENT\\n"+
  "BEGIN:VEVENT\\nUID:series\\nRECURRENCE-ID:20260713T090000Z\\nSTATUS:CANCELLED\\nDTSTART:20260713T090000Z\\nSUMMARY:Cancelled\\nEND:VEVENT\\n"+
  "END:VCALENDAR\\n", {url:"calendars/test.ics"})`,context);
assert.equal(recurring.length,1);
assert.equal(recurring[0]._skip.has(Date.parse('2026-07-13T09:00:00Z')),true,'cancelled overrides must suppress the master occurrence');
console.log("PASS: Calendar beta.2 uses bounded source reuse, serialized generation, conditional browser loads, stable signatures, and revised ICS semantics");
