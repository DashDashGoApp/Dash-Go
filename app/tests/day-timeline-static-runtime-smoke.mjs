#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import {fileURLToPath} from "node:url";
const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const source=fs.readFileSync(path.join(root,"ui/js/day-popup.js"),"utf8");
function fakeNode(tag="div",cls="",text=""){
  const node={tagName:tag,className:cls,textContent:text,children:[],dataset:{},style:{setProperty(){}},isConnected:true,parentNode:null,_listeners:new Map(),
    classList:{add(){},remove(){},toggle(){}},
    appendChild(child){
      if(child&&child._fragment){for(const item of [...child.children])this.appendChild(item);child.children.length=0;return child;}
      if(child){child.remove?.();child.parentNode=this;this.children.push(child);}return child;
    },
    append(...items){for(const item of items)this.appendChild(item);},
    remove(){if(!this.parentNode)return;const i=this.parentNode.children.indexOf(this);if(i>=0)this.parentNode.children.splice(i,1);this.parentNode=null;},
    addEventListener(type,fn){this._listeners.set(type,fn);},removeEventListener(type,fn){if(this._listeners.get(type)===fn)this._listeners.delete(type);},
    contains(child){return child===this||this.children.some(x=>x.contains?.(child));},
    querySelector(selector){
      const wanted=selector.startsWith(".")?selector.slice(1):"";
      for(const child of this.children){if(String(child.className||"").split(/\s+/).includes(wanted))return child;const nested=child.querySelector?.(selector);if(nested)return nested;}
      return null;
    }
  };
  Object.defineProperty(node,"childNodes",{get(){return node.children;}});
  return node;
}
const queue=[];
function popupDefer(_token,work){
  const task={cancelled:false,cancel(){this.cancelled=true;}};
  queue.push({task,work});return task;
}
function flushOne(){
  const next=queue.shift();if(!next||next.task.cancelled)return false;
  next.work({isCurrent:()=>!next.task.cancelled,onCancel(){}});return true;
}
function flushAll(){while(flushOne()){} }
const context=vm.createContext({
  console,Map,Math,window:{innerHeight:1080,innerWidth:1920},CONFIG:{profile:"lite"},
  document:{createDocumentFragment(){const f=fakeNode("#fragment");f._fragment=true;return f;}},
  popupDefer,popupNextFrame(fn){fn();},popupIsCurrent:()=>true,
  el:(tag,cls,text)=>fakeNode(tag,cls,text),
  dtEventColor:()=>"#7fd6a8",dtApplyCardColor(){},dtMarkEventCard(){},dtAddEventContents(card){card.appendChild(fakeNode("div","dt-event-title","event"));},dtCalendarMeta:()=>null,
  dtLaneLayout:()=>({left:"0px",width:"100%"}),dtFormatHour:h=>String(h),classify:()=>""
});
vm.runInContext(source+"\nglobalThis.__day={dtBuildTimelineView,dtStageTimelineCards,dtCancelTimelineStage,dtLiteDayPopupProfile};",context,{filename:"day-timeline-source.js"});
const {dtBuildTimelineView,dtStageTimelineCards,dtCancelTimelineStage,dtLiteDayPopupProfile}=context.__day;
assert.equal(dtLiteDayPopupProfile(),true,"Lite profile detection must select the List-first path");
const laid=Array.from({length:49},(_,i)=>({ev:{title:"Fixture"},startMin:480+i*5,endMin:510+i*5,lane:0,laneCount:1,_stackIdx:0}));
const model={timed:laid,allDay:[],timeline:{startHour:8,endHour:24,totalMinutes:960,hourPx:86,laneGap:10,laid,stacks:new Map(),gridHeightPx:1376}};
const view=dtBuildTimelineView(model),stage=view._dtTimelineStage;
assert.ok(stage,"timeline skeleton creates a staged build descriptor");
assert.equal(stage.events.children.length,0,"skeleton must paint before timed-card construction");
dtStageTimelineCards(1,view);
assert.equal(queue.length,1,"first timed-card chunk is deferred from the shell paint");
flushOne();
assert.equal(stage.events.children.length,16,"one frame appends only the bounded timed-card chunk");
assert.equal(stage.events._listeners.size,0,"timed-card container must not receive a scroll handler");
const beforeCancel=stage.events.children.length;
dtCancelTimelineStage(view);flushAll();
assert.equal(stage.events.children.length,beforeCancel,"cancelled popup/view work cannot append further cards");
const staticView=dtBuildTimelineView(model),staticStage=staticView._dtTimelineStage;
dtStageTimelineCards(2,staticView);flushAll();
assert.equal(staticStage.events.children.length,laid.length,"after its one-time fill the timeline retains every card as static DOM");
assert.equal(staticStage.complete,true,"static timeline staging completes deterministically");
assert.equal(staticStage.events._listeners.size,0,"completed static timeline retains no scroll-time listener");
console.log("PASS: Lite timeline stages static cards once, cancels safely, and performs no scroll-time DOM work");
