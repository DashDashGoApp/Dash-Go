import assert from "node:assert/strict";
import {readFileSync} from "node:fs";
import {resolve} from "node:path";
import vm from "node:vm";

const root=resolve(process.argv[2]||".");
const form=readFileSync(resolve(root,"ui/js/calendar-event-form.js"),"utf8");

// Prevent a future refactor from returning to el(..., DOMNode), where el's
// third argument is text-only and silently renders [object HTMLSpanElement].
assert.match(form,/function calendarWritebackFixedCalendarRow\(choice,detail\)\{/,'fixed calendars need one dedicated nested-DOM helper');
assert.match(form,/const row=el\("div","calendar-writeback-calendar-fixed"\);\s*row\.appendChild\(calendarWritebackCalendarCopy\(choice,detail\)\);/s,'fixed calendar helper must append the calendar copy as a nested element');
assert.doesNotMatch(form,/el\("div","calendar-writeback-calendar-fixed",\s*calendarWritebackCalendarCopy\(/,'fixed calendar rows may not pass a DOM node to el text content');
assert.match(form,/calendarWritebackFixedCalendarRow\(\{name:"Calendar unavailable"/,'unavailable fixed state must use the nested helper');
assert.match(form,/calendarWritebackFixedCalendarRow\(selected,detail\)/,'existing and one-calendar fixed state must use the nested helper');

class Node {
  constructor(tag,className,text){this.tag=tag;this.className=className;this._text=text===undefined?"":String(text);this.children=[];}
  appendChild(node){this.children.push(node);return node;}
  get textContent(){return this._text+this.children.map(child=>child.textContent).join("");}
}
const context={
  el:(tag,className,text)=>new Node(tag,className,text),
  console,
  Map,
  Date,
  String,
  Number,
  Array,
  Object,
  setTimeout:()=>0,
  clearTimeout:()=>{},
};
vm.createContext(context);
vm.runInContext(form,context,{filename:"calendar-event-form.js"});
const fixed=context.calendarWritebackFixedCalendarRow;
assert.equal(typeof fixed,"function",'fixed calendar helper should be executable');

for(const [choice,detail] of [
  [{name:"Payday",provider:"Private calendar",ordinal:0},"This edit stays here"],
  [{name:"Household",provider:"Google calendar",ordinal:0},"Selected writable calendar"],
  [{name:"Calendar unavailable",provider:"Dashboard edits are no longer enabled",ordinal:0},"Return to the event and refresh Calendar Manager."],
]){
  const row=fixed(choice,detail);
  assert.equal(row.tag,"div");
  assert.equal(row.className,"calendar-writeback-calendar-fixed");
  assert.equal(row.children.length,1,'fixed row must contain one nested calendar-copy element');
  const copy=row.children[0];
  assert.equal(copy.tag,"span");
  assert.equal(copy.className,"calendar-writeback-calendar-copy");
  assert.equal(copy.children[0].className,"calendar-writeback-calendar-name");
  assert.equal(copy.children[0].textContent,choice.name);
  assert.equal(copy.children[1].className,"calendar-writeback-calendar-meta");
  assert.match(copy.children[1].textContent,new RegExp(choice.provider.replace(/[.*+?^${}()|[\]\\]/g,"\\$&")));
  assert.doesNotMatch(row.textContent,/\[object HTML(?:Span)?Element\]|<span|&lt;span/i,'fixed calendar copy must remain nested DOM, never stringified or escaped markup');
}

console.log("calendar fixed-calendar smoke: local fixed labels remain nested, provider-aware DOM in existing, one-choice, and unavailable states");
