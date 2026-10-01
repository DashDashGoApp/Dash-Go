#!/usr/bin/env node
// Behavioural proof of the "did this popup's data actually change?" decisions
// behind the live refresh. Getting these wrong is invisible in the worst way: a
// false negative leaves a stale popup on screen (the bug being fixed) and a
// false positive rebuilds a whole day of DOM on every commit.
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import {fileURLToPath} from "node:url";

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const daySource=fs.readFileSync(path.join(root,"ui/js/day-popup.js"),"utf8");
const eventSource=fs.readFileSync(path.join(root,"ui/js/event-popup.js"),"utf8");

function node(tag,cls,txt){
  const n={tagName:tag,className:cls||"",children:[],style:{},dataset:{},attributes:{},
    get childNodes(){return this.children;},
    setAttribute(k,v){this.attributes[k]=String(v);},
    appendChild(c){this.children.push(c);return c;},
    append(...cs){this.children.push(...cs);},
    addEventListener(){}};
  if(txt!=null)n.textContent=String(txt);
  return n;
}
const context=vm.createContext({
  console,
  el:node,
  CONFIG:{showInteractiveMaps:false},
  EVENTS:[],
  document:{
    createTextNode:t=>({textContent:String(t)}),
    createDocumentFragment:()=>({children:[],appendChild(c){this.children.push(c);return c;}})
  }
});
context.globalThis=context;
vm.runInContext(daySource,context,{filename:"day-popup-source.js"});
vm.runInContext(eventSource,context,{filename:"event-popup-source.js"});
const run=code=>vm.runInContext(code,context);

function mkEvent(over){
  return Object.assign({
    id:"a",uid:"u1",start:new Date(2026,9,1,9,0),end:new Date(2026,9,1,10,0),
    allDay:false,title:"Dentist",location:"Main St",desc:"",cal:{name:"Family"}
  },over||{});
}

// --- day content identity ---------------------------------------------------
context.__evs=[mkEvent()];
const dayKey=run("dtEventSetKey(__evs)");
context.__evs=[mkEvent()];                    // a commit rebuilds fresh objects
assert.equal(run("dtEventSetKey(__evs)"),dayKey,"identical content rebuilt as new objects must share a key");
context.__evs=[mkEvent({title:"Dentist (moved)"})];
assert.notEqual(run("dtEventSetKey(__evs)"),dayKey,"a changed title must change the key");
context.__evs=[mkEvent({start:new Date(2026,9,1,11,0)})];
assert.notEqual(run("dtEventSetKey(__evs)"),dayKey,"a changed start must change the key");
context.__evs=[mkEvent({location:"Oak Ave"})];
assert.notEqual(run("dtEventSetKey(__evs)"),dayKey,"a changed location must change the key");
context.__evs=[mkEvent({allDay:true})];
assert.notEqual(run("dtEventSetKey(__evs)"),dayKey,"an all-day change must change the key");
// Order is part of the key deliberately: the popup renders in the order it is
// given, so an order-only change should still rebuild rather than mislabel cards.
context.__evs=[mkEvent(),mkEvent({id:"b",uid:"u2",title:"School run"})];
const twoKey=run("dtEventSetKey(__evs)");
context.__evs=[mkEvent({id:"b",uid:"u2",title:"School run"}),mkEvent()];
assert.notEqual(run("dtEventSetKey(__evs)"),twoKey,"reordering events must change the key");
context.__evs=[];
assert.equal(run("dtEventSetKey(__evs)"),"","an empty day has an empty key");

// --- event popup identity and change detection ------------------------------
context.__ev=mkEvent();
assert.equal(run("eventPopupDataKey(__ev)"),run("eventPopupDataKey(__ev)"),"event key is stable");
context.__ev2=mkEvent({desc:"Bring the form"});
assert.notEqual(run("eventPopupDataKey(__ev2)"),run("eventPopupDataKey(__ev)"),"a changed description must change the event key");
context.__ev3=mkEvent({cal:{name:"Work"}});
assert.notEqual(run("eventPopupDataKey(__ev3)"),run("eventPopupDataKey(__ev)"),"a changed calendar name must change the event key");

context.__a=mkEvent();
context.__b=mkEvent({title:"Renamed"});       // same id, different content
assert.equal(run("eventPopupSameEvent(__a,__b)"),true,"the same provider id identifies the same event across a rebuild");
context.__c=mkEvent({id:undefined,uid:"u1"});
context.__d=mkEvent({id:undefined,uid:"u1",title:"Renamed"});
assert.equal(run("eventPopupSameEvent(__c,__d)"),true,"a shared uid identifies the same event");
context.__e=mkEvent({id:undefined,uid:undefined,title:"Yoga"});
context.__f=mkEvent({id:undefined,uid:undefined,title:"Yoga"});
assert.equal(run("eventPopupSameEvent(__e,__f)"),true,"without identifiers, start plus title is the fallback identity");
context.__g=mkEvent({id:"z",uid:"u9",title:"Different"});
assert.equal(run("eventPopupSameEvent(__a,__g)"),false,"different ids are different events");
assert.equal(run("eventPopupSameEvent(__a,null)"),false,"a missing event never matches");

// --- finding the live replacement -------------------------------------------
context.EVENTS=[mkEvent({title:"Renamed"}),mkEvent({id:"b",uid:"u2",title:"School run"})];
context.__lookup=mkEvent();
assert.equal(run("eventPopupFindLive(__lookup).title"),"Renamed","the refreshed event is found by identity, not reference");
context.EVENTS=[];
assert.equal(run("eventPopupFindLive(__lookup)"),null,"a deleted event has no live replacement");
context.EVENTS=[mkEvent({id:"b",uid:"u2"})];
assert.equal(run("eventPopupFindLive(__lookup)"),null,"an unrelated calendar does not resolve to some other event");

// --- detail nodes the refresh swaps ----------------------------------------
context.__bare=mkEvent({location:"",desc:"",cal:null});
const bare=run("eventPopupDetails(__bare)");
assert.equal(bare.meta,null,"no location and no calendar produces no meta block");
assert.equal(bare.desc,null,"no description produces no description node");
context.__full=mkEvent({desc:"Bring the referral form"});
const full=run("eventPopupDetails(__full)");
assert.ok(full.meta&&full.meta.className==="eventpopupmeta","a located event produces the meta block");
assert.equal(full.meta.childNodes.length,2,"location and calendar are separate meta items, not merged");
assert.equal(full.meta.childNodes[0].className,"eventmetaitem eventlocation","the location item keeps its class contract");
assert.equal(full.meta.childNodes[1].className,"eventmetaitem eventcalendar","the calendar item keeps its class contract");
assert.ok(full.desc&&full.desc.className==="eventpopupdesc","the description node is classed so a refresh can swap it");
assert.equal(full.desc.textContent,"Bring the referral form","the description text is carried through");

console.log("PASS: popup live-refresh change detection distinguishes real data changes from rebuilt objects");
