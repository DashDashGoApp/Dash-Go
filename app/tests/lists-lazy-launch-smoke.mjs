import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import {fileURLToPath} from "node:url";

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const read=rel=>fs.readFileSync(path.join(root,rel),"utf8");

class ClassList{
  constructor(){this.values=new Set();}
  add(...names){names.forEach(name=>this.values.add(name));}
  remove(...names){names.forEach(name=>this.values.delete(name));}
  contains(name){return this.values.has(name);}
  toggle(name,enabled){if(enabled===undefined){this.values.has(name)?this.values.delete(name):this.values.add(name);}else if(enabled)this.values.add(name);else this.values.delete(name);}
}
class ElementFixture{
  constructor(document){this.ownerDocument=document;this.children=[];this.classList=new ClassList();this.attrs={};this.style={};this.hidden=false;this.textContent="";this.parentNode=null;}
  setAttribute(name,value){this.attrs[name]=String(value);}
  getAttribute(name){return this.attrs[name]??null;}
  removeAttribute(name){delete this.attrs[name];}
  appendChild(node){this.children.push(node);node.parentNode=this;return node;}
  append(...nodes){nodes.forEach(node=>this.appendChild(node));}
  replaceChildren(...nodes){this.children=[];nodes.forEach(node=>this.appendChild(node));}
  addEventListener(){}
  removeEventListener(){}
  focus(){}
  remove(){if(this.parentNode)this.parentNode.children=this.parentNode.children.filter(node=>node!==this);}
}
function fixtureDocument(){
  const nodes=new Map();
  const document={
    addEventListener(){},
    createElement(){return new ElementFixture(document);},
    getElementById(id){return nodes.get(id)||null;},
  };
  for(const id of ["listsapp","listsapp-close","listsapp-title","listsapp-status","listsapp-body","cblaunch"]){
    nodes.set(id,new ElementFixture(document));
  }
  return document;
}
function statusPayload(){
  return {
    map:{todo:"local-todo",grocery:"local-grocery"},
    lists:[
      {id:"local-todo",displayName:"To Do",origin:"local"},
      {id:"local-grocery",displayName:"Grocery",origin:"local"},
    ],
    groceryMemory:[],
    syncActive:false,
  };
}
function response(payload){return {ok:true,json:async()=>payload};}

const document=fixtureDocument();
const sandbox={
  Array,Date,Error,Map,Math,Number,Object,Promise,RegExp,Set,String,console,document,encodeURIComponent,
  requestAnimationFrame:callback=>callback(),setTimeout,clearTimeout,
  bindTap(){},hideOSK(){},pauseUiAnimations(){},armOverlayAutoClose(){},overlayIsOpen(){return false;},disarmOverlayAutoClose(){},resumeUiAfterOverlay(){},
  EventSource:class{addEventListener(){}close(){}},
  fetch:async url=>{
    const value=String(url);
    if(value==="/api/todo/status")return response(statusPayload());
    if(/\/api\/todo\/lists\/[^/]+\/tasks$/.test(value))return response({tasks:[]});
    if(/\/api\/todo\/lists\/[^/]+\/sync$/.test(value))return response({inboundSync:{running:false}});
    throw new Error("unexpected Lists test request: "+value);
  },
};
sandbox.window=sandbox;
const context=vm.createContext(sandbox);
for(const rel of ["ui/lists-core.js","ui/lists-actions.js","ui/lists-grocery.js"]){
  vm.runInContext(read(rel),context,{filename:rel});
}

assert.equal(typeof sandbox.openListsImpl,"function","lazy Lists actions must expose the canonical open lifecycle");
for(const [slot,title] of [["todo","To Do"],["grocery","Grocery"]]){
  await sandbox.openListsImpl(slot);
  assert.equal(document.getElementById("listsapp").classList.contains("show"),true,`${title} must open the shared Lists panel`);
  assert.equal(document.getElementById("listsapp-title").textContent,title,`${title} must retain its selected destination after lazy loading`);
}
console.log("PASS: lazy Lists core/actions/grocery modules open To Do and Grocery destinations.");
