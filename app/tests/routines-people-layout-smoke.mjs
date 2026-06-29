#!/usr/bin/env node
import assert from "node:assert/strict";
import {readFileSync} from "node:fs";
import {resolve} from "node:path";

const root=resolve(process.argv[2]||".");
const read=rel=>readFileSync(resolve(root,rel),"utf8");
const js=read("ui/routines.js");

assert.doesNotMatch(js,/\["people","People"\]/,"Routines must not retain a duplicate People tab");
assert.doesNotMatch(js,/\/api\/routines\/people/,"Routines must not mutate the canonical roster directly");
assert.doesNotMatch(js,/function peopleView\(/,"Routines must not retain a duplicate People editor");
assert.match(js,/openDashboardPeopleControl/,"Routines needs a contextual route to the canonical People editor");
assert.match(js,/Manage people/,"Routines must expose a visible Manage people action");
assert.match(js,/Dashboard Control first/,"empty routine setup must explain where people are managed");
console.log("PASS: Routines consumes the canonical People roster without a duplicate People editor.");
