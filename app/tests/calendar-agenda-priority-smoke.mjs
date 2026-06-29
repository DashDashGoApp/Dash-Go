import assert from "node:assert/strict";
import {readFileSync} from "node:fs";
import {resolve} from "node:path";
import vm from "node:vm";

const root=resolve(process.argv[2]||".");
const read=rel=>readFileSync(resolve(root,rel),"utf8");
const helpers=read("ui/js/calendar-span-helpers.js");
const agenda=read("ui/js/calendar-agenda.js");

assert.match(helpers,/calendarRank:10/,"Chores need explicit month-cell rank");
assert.match(helpers,/calendarRank:20/,"Maintenance needs explicit month-cell rank");
assert.match(helpers,/calendarRank:30/,"Routines needs explicit month-cell rank");
assert.match(helpers,/agendaRank:10/,"Chores need explicit Agenda rank");
assert.match(helpers,/agendaRank:20/,"Maintenance needs explicit Agenda rank");
assert.match(helpers,/agendaRank:30/,"Routines need explicit Agenda rank");
assert.match(helpers,/a\.rank-b\.rank \|\| a\.owner\.localeCompare\(b\.owner\)/,"month app groups must use owner rank rather than visible labels");
assert.match(agenda,/function agendaEventComparator\(/,"Agenda needs an explicit presentation comparator");
assert.match(agenda,/function agendaOrderedEvents\(/,"Agenda must order a copied presentation list");
assert.match(agenda,/agendaOrderedEvents\(eventsOnDay\(day\)\)/,"Agenda rendering must use the explicit presentation order");
assert.match(agenda,/\[\.\.\.source\]\.sort\(agendaEventComparator\)/,"Agenda ordering must not mutate DAY_INDEX arrays");

const sandbox={console,Date};
sandbox.globalThis=sandbox;
vm.createContext(sandbox);
vm.runInContext(`${helpers}
globalThis.__calendarPriority={calendarCellDisplayRows,appCalendarOwner,appCalendarGroupInfo};`,sandbox,{filename:"calendar-span-helpers.js"});
vm.runInContext(`${agenda}
globalThis.__agendaPriority={agendaOwnerRank,agendaEventComparator,agendaOrderedEvents};`,sandbox,{filename:"calendar-agenda.js"});

const app=(owner,title,start,allDay=false,id=owner)=>({id,title,start:new Date(start),allDay,appOwner:owner,cal:{owner,color:"#456"}});
const normal=(title,start,allDay=false,id=title)=>({id,title,start:new Date(start),allDay,cal:{}});
const {calendarCellDisplayRows}=sandbox.__calendarPriority;
const {agendaOrderedEvents}=sandbox.__agendaPriority;

const cellRows=calendarCellDisplayRows([
  app("routines","Z localized label", "2026-06-24T19:00:00",false),
  normal("Dentist","2026-06-24T09:00:00"),
  app("maintenance","A localized label","2026-06-24T00:00:00",true),
  app("chore-wheel","M localized label","2026-06-24T00:00:00",true)
]);
assert.deepEqual(Array.from(cellRows,row=>row.kind==="event"?row.event.title:row.owner),[
  "Dentist","chore-wheel","maintenance","routines"
],"month cells must keep ordinary events first and rank app groups Chores -> Maintenance -> Routines");

const original=[
  normal("School out","2026-06-24T00:00:00",true,"holiday"),
  normal("Dentist","2026-06-24T09:00:00",false,"dentist"),
  app("routines","Night routine","2026-06-24T20:00:00",false,"routine-night"),
  app("maintenance","HVAC filter","2026-06-24T00:00:00",true,"maint"),
  app("chore-wheel","Kitty litter","2026-06-24T00:00:00",true,"chore"),
  app("routines","Morning routine","2026-06-24T07:15:00",false,"routine-morning"),
  normal("All day normal","2026-06-24T00:00:00",true,"all-day-normal")
];
const before=original.map(item=>item.id);
const ordered=agendaOrderedEvents(original);
assert.deepEqual(Array.from(ordered,item=>item.id),[
  "chore","maint","routine-morning","routine-night","all-day-normal","holiday","dentist"
],"Agenda must place household app items first, preserve their owner rank, then retain ordinary all-day-before-timed chronology");
assert.deepEqual(original.map(item=>item.id),before,"Agenda sorting must not mutate the shared DAY_INDEX event array");
assert.equal(ordered[0].allDay,true,"Agenda owner ordering must retain all-day display semantics");

console.log("PASS: month cells keep app groups last while Agenda deterministically prioritizes Chores, Maintenance, and Routines without mutating source events");
