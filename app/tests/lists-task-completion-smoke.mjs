import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import {fileURLToPath} from "node:url";

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const read=rel=>fs.readFileSync(path.join(root,rel),"utf8");
const sleep=ms=>new Promise(resolve=>setTimeout(resolve,ms));

class ClassList{
  constructor(){this.values=new Set();}
  add(...names){names.forEach(name=>this.values.add(name));}
  remove(...names){names.forEach(name=>this.values.delete(name));}
  contains(name){return this.values.has(name);}
  toggle(name,enabled){if(enabled===undefined){this.values.has(name)?this.values.delete(name):this.values.add(name);}else if(enabled)this.values.add(name);else this.values.delete(name);}
}
class ElementFixture{
  constructor(document){this.ownerDocument=document;this.children=[];this.classList=new ClassList();this.attrs={};this.style={};this.hidden=false;this.textContent="";this.className="";this.parentNode=null;this.dataset={};}
  setAttribute(name,value){this.attrs[name]=String(value);}
  getAttribute(name){return this.attrs[name]??null;}
  removeAttribute(name){delete this.attrs[name];}
  appendChild(node){this.children.push(node);node.parentNode=this;return node;}
  append(...nodes){nodes.forEach(node=>this.appendChild(node));}
  prepend(...nodes){this.children.unshift(...nodes);nodes.forEach(node=>node.parentNode=this);}
  replaceChildren(...nodes){this.children=[];nodes.forEach(node=>this.appendChild(node));}
  addEventListener(){}
  removeEventListener(){}
  focus(){}
  remove(){if(this.parentNode)this.parentNode.children=this.parentNode.children.filter(node=>node!==this);}
}
function fixtureDocument(){
  const nodes=new Map();
  const document={
    activeElement:null,
    body:null,
    addEventListener(){},
    removeEventListener(){},
    createElement(){return new ElementFixture(document);},
    getElementById(id){return nodes.get(id)||null;},
  };
  document.body=new ElementFixture(document);
  for(const id of ["listsapp","listsapp-close","listsapp-title","listsapp-status","listsapp-body","cblaunch"]){
    nodes.set(id,new ElementFixture(document));
  }
  return document;
}
function findByClass(node,className){
  if(!node)return null;
  if(node.className===className)return node;
  for(const child of node.children||[]){
    const found=findByClass(child,className);
    if(found)return found;
  }
  return null;
}
function response(payload,ok=true){return {ok,status:ok?200:409,json:async()=>payload};}

const document=fixtureDocument();
const batches=[];
let serverTasks=[{id:"milk",title:"Milk",status:"notStarted"}];
const sandbox={
  Array,Date,Error,Map,Math,Number,Object,Promise,RegExp,Set,String,console,document,encodeURIComponent,
  requestAnimationFrame:callback=>callback(),setTimeout,clearTimeout,
  bindTap(node,handler){node.__tap=handler;},hideOSK(){},pauseUiAnimations(){},armOverlayAutoClose(){},overlayIsOpen(){return false;},disarmOverlayAutoClose(){},resumeUiAfterOverlay(){},
  EventSource:class{addEventListener(){}close(){}},
  fetch:async(url,options={})=>{
    const value=String(url),method=String(options.method||"GET").toUpperCase();
    if(value==="/api/todo/status")return response({
      map:{todo:"local-todo",grocery:"local-grocery"},
      lists:[
        {id:"local-todo",displayName:"To Do",origin:"local"},
        {id:"local-grocery",displayName:"Grocery",origin:"local"},
      ],
      groceryMemory:[],syncActive:false,
    });
    if(method==="GET"&&/\/api\/todo\/lists\/[^/]+\/tasks$/.test(value))return response({tasks:serverTasks.map(task=>({...task}))});
    if(method==="POST"&&/\/api\/todo\/lists\/[^/]+\/sync$/.test(value))return response({inboundSync:{running:false}});
    if(method==="POST"&&/\/api\/todo\/lists\/[^/]+\/tasks\/batch$/.test(value)){
      const body=JSON.parse(options.body||"{}");
      batches.push(body);
      if(batches.length===2)return response({error:"storage offline"},false);
      for(const entry of body.patches||[]){
        const task=serverTasks.find(candidate=>candidate.id===entry.id);
        if(task)Object.assign(task,entry.patch||{});
      }
      return response({ok:true,cache:{tasks:serverTasks.map(task=>({...task}))}});
    }
    throw new Error("unexpected Lists completion request: "+method+" "+value);
  },
};
sandbox.window=sandbox;
const context=vm.createContext(sandbox);
for(const rel of ["ui/lists-core.js","ui/lists-actions.js","ui/lists-grocery.js"]){
  vm.runInContext(read(rel),context,{filename:rel});
}

await sandbox.openListsImpl("grocery");
let checkbox=findByClass(document.getElementById("listsapp-body"),"task-check");
assert.equal(typeof checkbox?.__tap,"function","rendered Grocery checkbox must keep the real pointer-release task handler");
checkbox.__tap();
await sleep(120);
assert.deepEqual(batches,[{patches:[{id:"milk",patch:{status:"completed"}}]}],"a Grocery checkbox must send the local-first batch completion mutation");
assert.equal(sandbox.DashGoLists.state.tasks[0].status,"completed","successful completion must replace the optimistic task with the returned cache state");
// Completed tasks are normally hidden in Grocery. Show the completed row so the
// real rendered checkbox can exercise the failed uncheck/rollback path too.
sandbox.DashGoLists.state.showCompleted["local-grocery"]=true;
sandbox.DashGoLists.renderTasks();
checkbox=findByClass(document.getElementById("listsapp-body"),"task-check");
assert.equal(typeof checkbox?.__tap,"function","completed Grocery rows must retain their real checkbox handler");
checkbox.__tap();
await sleep(120);
assert.equal(batches.length,2,"a later toggle must still attempt the batch mutation");
assert.equal(sandbox.DashGoLists.state.tasks[0].status,"completed","a rejected completion change must restore the prior task state");
assert.match(document.getElementById("listsapp-status").textContent,/Could not save/,"a rejected completion must be visible in the Lists status line");
console.log("PASS: real Grocery checkbox batching succeeds, then rolls back visibly on a failed save.");
