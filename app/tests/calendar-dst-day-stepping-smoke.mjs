#!/usr/bin/env node
// The weekday header row and the Agenda day list must step by CALENDAR DAYS.
// Adding i*86400000ms lands at 23:00 on the previous day whenever the span
// crosses a DST changeover, which printed "Sunday Sunday Monday ..." in the
// header and repeated a day in the Agenda. This test runs the real render
// function with the clock frozen inside a fall-back week.
import assert from "node:assert/strict";
import {readFileSync} from "node:fs";
import {resolve} from "node:path";
import vm from "node:vm";

process.env.TZ = "America/Chicago";

const root = resolve(process.argv[2] || ".");
const read = (rel) => readFileSync(resolve(root, rel), "utf8");
const core = read("ui/js/data-core.js");
const helpers = read("ui/js/calendar-span-helpers.js");
const agenda = read("ui/js/calendar-agenda.js");

// The two offending call sites are gone; day stepping goes through addDays().
assert.match(helpers, /const d=addDays\(base,i\);/, "weekday header must step with addDays()");
assert.doesNotMatch(helpers, /new Date\(\+base\+i\*DAY\)/, "weekday header must not step by milliseconds");
assert.match(agenda, /const day=addDays\(today,i\)/, "Agenda day list must step with addDays()");
assert.doesNotMatch(agenda, /new Date\(\+today\+i\*DAY\)/, "Agenda day list must not step by milliseconds");

// Freeze the clock at the US fall-back week: Sunday 2026-11-01 (DST ends that
// morning) is exactly the week that produced two "Sunday" headers.
const FROZEN = new Date(2026, 10, 5, 10, 54, 0).getTime(); // Thu Nov 5 2026, 10:54 local
class FrozenDate extends Date {
  constructor(...args) {
    if (args.length === 0) super(FROZEN);
    else super(...args);
  }
  static now() { return FROZEN; }
}

function makeEl(tag, cls, text) {
  const node = {
    tag, className: "", textContent: text === undefined ? "" : String(text),
    children: [], dataset: {},
    appendChild(child) { this.children.push(child); return child; },
    set innerHTML(v) { this.children.length = 0; this._html = v; },
    get innerHTML() { return this._html || ""; },
  };
  if (cls) node.className = cls;
  return node;
}

const head = makeEl("div");
const sandbox = {
  console, Date: FrozenDate, Intl,
  LOCALE: "en-US",
  CONFIG: {firstDayOfWeek: 0},
  FMT: {weekday: new Intl.DateTimeFormat("en-US", {weekday: "long"})},
  el: makeEl,
  document: {
    getElementById: (id) => (id === "calhead" ? head : null),
    querySelector: (sel) => (sel === "#calhead" ? head : null),
    createElement: (tag) => makeEl(tag),
  },
};
sandbox.globalThis = sandbox;
vm.createContext(sandbox);
vm.runInContext(core, sandbox, {filename: "data-core.js"});
vm.runInContext(helpers, sandbox, {filename: "calendar-span-helpers.js"});

sandbox.__renderCalHead = sandbox.renderCalHead;
sandbox.__addDays = sandbox.addDays;
sandbox.__startOfWeek = sandbox.startOfWeek;

sandbox.__renderCalHead();
const labels = head.children.map((col) => (col.children[0] || {}).textContent);
assert.equal(labels.length, 7, "weekday header must render seven columns");
assert.deepEqual(labels, ["Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"],
  `weekday header must name seven distinct days, got ${JSON.stringify(labels)}`);
assert.equal(new Set(labels).size, 7, "weekday header must never repeat a weekday");

// The same week through the Agenda's stepping path (addDays) must also produce
// seven distinct, consecutive calendar days, not a repeated one.
const today = sandbox.startOfDay(new FrozenDate());
const days = Array.from({length: 7}, (_, i) => sandbox.__addDays(today, i));
const keys = days.map((d) => `${d.getFullYear()}-${d.getMonth() + 1}-${d.getDate()}`);
const expected = Array.from({length: 7}, (_, i) => new Date(2026, 10, 5 + i))
  .map((d) => `${d.getFullYear()}-${d.getMonth() + 1}-${d.getDate()}`);
assert.equal(new Set(keys).size, 7, `agenda day stepping must yield distinct days, got ${JSON.stringify(keys)}`);
assert.deepEqual(keys, expected, "agenda days in the fall-back week must be consecutive dates");

// A spring-forward week (2027-03-14) must behave the same way.
const spring = new Date(2027, 2, 16, 9, 0, 0).getTime();
sandbox.__frozen = spring;
const springBase = sandbox.__startOfWeek(new Date(spring));
const springLabels = Array.from({length: 7}, (_, i) => sandbox.__addDays(springBase, i).getDay());
assert.deepEqual(springLabels, [0, 1, 2, 3, 4, 5, 6], "spring-forward week must also label seven distinct days");

console.log("PASS: weekday header and Agenda step by calendar days across DST changeovers");
