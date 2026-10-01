#!/usr/bin/env node
// The Lite day popup defaults to the List view, so List needs the same bounded
// staging contract the Timeline already had: the first chunk commits with the
// shell (so rows and the initial position exist immediately), the remainder is
// appended in bounded frames, cancellation stops it, and scrolling never stages.
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import {fileURLToPath} from "node:url";

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
// The bundle concatenates the day popup before the event popup, and the shared
// chunk size is declared in the former while the list staging uses it in the
// latter — so both are loaded, in that order, exactly as the bundle arranges.
const daySource=fs.readFileSync(path.join(root,"ui/js/day-popup.js"),"utf8");
const source=fs.readFileSync(path.join(root,"ui/js/event-popup.js"),"utf8");

function fakeNode(tag="div",cls="",text=""){
  const node={tagName:tag,className:cls,textContent:text,children:[],dataset:{},style:{setProperty(){}},isConnected:true,parentNode:null,_listeners:new Map(),
    classList:{add(){},remove(){},toggle(){}},
    appendChild(child){
      if(child&&child._fragment){for(const item of [...child.children])this.appendChild(item);child.children.length=0;return child;}
      if(child){child.remove?.();child.parentNode=this;this.children.push(child);}return child;
    },
    append(...items){for(const item of items)this.appendChild(item);},
    remove(){if(!this.parentNode)return;const i=this.parentNode.children.indexOf(this);if(i>=0)this.parentNode.children.splice(i,1);this.parentNode=null;},
    addEventListener(type,fn){this._listeners.set(type,fn);},
    removeEventListener(type,fn){if(this._listeners.get(type)===fn)this._listeners.delete(type);},
    contains(child){return child===this||this.children.some(x=>x.contains?.(child));}
  };
  Object.defineProperty(node,"childNodes",{get(){return node.children;}});
  return node;
}
const queue=[];
function popupDefer(_token,work){
  const task={cancelled:false,cancel(){this.cancelled=true;}};
  queue.push({task,work});return task;
}
function flushOne(){const next=queue.shift();if(!next||next.task.cancelled)return false;next.work({isCurrent:()=>!next.task.cancelled,onCancel(){}});return true;}
function flushAll(){while(flushOne()){}}

const context=vm.createContext({
  console,Map,Math,Date,
  window:{innerWidth:1920,innerHeight:1080},
  CONFIG:{showInteractiveMaps:false},
  FMT:{time:{format:()=>"9:00"},popDay:{format:()=>"Oct 1"},dayLong:{format:()=>"Thursday"}},
  startOfDay:day=>new Date(day.getFullYear(),day.getMonth(),day.getDate()),
  addDays:(day,count)=>new Date(+day+count*86400000),
  document:{createDocumentFragment(){const f=fakeNode("#fragment");f._fragment=true;return f;},documentElement:{getAttribute:()=>""}},
  el:(tag,cls,text)=>fakeNode(tag,cls,text),
  classify:()=>"",dtApplyCardColor(){},dtMarkEventCard(){},dtCalendarMeta:()=>null,
  popupDefer,popupIsCurrent:()=>true,popupNextFrame:fn=>fn()
});
vm.runInContext(daySource,context,{filename:"day-popup-source.js"});
vm.runInContext(source+"\nglobalThis.__list={dtBuildListView,dtStageListCards,dtCancelListStage,DT_CARD_STAGE_CHUNK};",context,{filename:"event-popup-source.js"});
const {dtBuildListView,dtStageListCards,dtCancelListStage,DT_CARD_STAGE_CHUNK}=context.__list;

const day=new Date(2026,9,1);
const rows=count=>Array.from({length:count},(_,i)=>({id:"e"+i,uid:"u"+i,title:"Fixture "+i,start:new Date(2026,9,1,9,0,i),end:new Date(2026,9,1,9,30,i),allDay:false,cal:{name:"Family"}}));
const model=evs=>({day,evs,timed:evs,allDay:[]});

assert.equal(DT_CARD_STAGE_CHUNK,16,"the bounded chunk size is shared with the timeline");

// --- staging ---------------------------------------------------------------
const evs=rows(40);
const view=dtBuildListView(day,evs,model(evs)),stage=view._dtListStage;
assert.ok(stage,"the list view creates a staged build descriptor");
assert.equal(view.children.length,16,"the first bounded chunk commits with the view, so rows and the initial position exist in the commit frame");
assert.equal(stage.complete,false,"a 40-event day is not finished in the commit frame");
dtStageListCards(1,view);
assert.equal(queue.length,1,"the remainder is deferred out of the commit frame");
flushOne();
assert.equal(view.children.length,32,"one frame appends exactly one bounded chunk");
flushAll();
assert.equal(view.children.length,evs.length,"staging completes with every row present as static DOM");
assert.equal(stage.complete,true,"list staging completes deterministically");
assert.equal(view._listeners.size,0,"the staged list must not install a scroll listener");

// --- a day that fits one chunk finishes immediately -------------------------
const small=rows(3);
const smallView=dtBuildListView(day,small,model(small));
assert.equal(smallView.children.length,3,"a short day builds completely in the commit frame");
assert.equal(smallView._dtListStage.complete,true,"a short day needs no follow-up frames");
dtStageListCards(2,smallView);
assert.equal(queue.length,0,"a completed stage schedules no further work");

// --- cancellation ----------------------------------------------------------
const many=rows(80);
const cancelView=dtBuildListView(day,many,model(many));
dtStageListCards(3,cancelView);
flushOne();
const pausedAt=cancelView.children.length;
dtCancelListStage(cancelView);
flushAll();
assert.equal(cancelView.children.length,pausedAt,"a cancelled list stage cannot append further rows");
assert.equal(pausedAt,32,"cancellation happens after the chunk boundary, not mid-chunk");

// --- ordering --------------------------------------------------------------
const mixed=[{id:"b",title:"Timed",start:new Date(2026,9,1,14,0),allDay:false},{id:"a",title:"Holiday",start:new Date(2026,9,1,0,0),allDay:true}];
const ordered=dtBuildListView(day,mixed,model(mixed));
assert.equal(ordered.children[0].className.includes("dt-list-card"),true,"rows are list cards");
assert.equal(ordered._dtListStage.rows[0].allDay,true,"all-day rows sort ahead of timed rows");

console.log("PASS: the Lite List view stages bounded chunks, completes deterministically, and cancels safely");
