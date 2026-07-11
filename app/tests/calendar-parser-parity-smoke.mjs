#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import {fileURLToPath} from "node:url";
const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const parser=fs.readFileSync(path.join(root,"ui/js/ics-parser.js"),"utf8");
const fixtures=JSON.parse(fs.readFileSync(path.join(root,"tests/fixtures/calendar-parity/corpus.json"),"utf8"));
const context=vm.createContext({console,Date,Set,Map,Number,String,Math});
vm.runInContext(parser,context,{filename:"ics-parser.js"});
for(const fixture of fixtures){
  context.fixtureICS=fixture.ics;
  const events=vm.runInContext(`parseICS(fixtureICS,{url:"calendars/parity.ics"})`,context);
  const summary={count:events.length,titles:events.map(e=>e.title||""),durationsMs:events.map(e=>e.end?+e.end-+e.start:0),skipCount:events.reduce((n,e)=>n+((e._skip&&e._skip.size)||0),0)};
  assert.deepEqual(JSON.parse(JSON.stringify(summary)),fixture.expected,fixture.name);
}
console.log("PASS: Go and browser ICS parsers share the same parity corpus");
