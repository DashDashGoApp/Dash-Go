#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import {fileURLToPath} from "node:url";
const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const runtime=fs.readFileSync(path.join(root,"ui/js/config-runtime.js"),"utf8");
const health=fs.readFileSync(path.join(root,"ui/js/display-health.js"),"utf8");
function node(){const classes=new Set();return {style:{filter:"brightness(.4)",setProperty(k,v){this[k]=v;},removeProperty(k){delete this[k];}},classList:{toggle(k,on){on?classes.add(k):classes.delete(k);},contains:k=>classes.has(k)},setAttribute(){},appendChild(){}};}
const style=node(),dim=node(),body=node(),rootNode=node();let renderCalls=0,decorCalls=0;
const context=vm.createContext({console,THEMES:{basic:{"--bg":"#111"},paper:{"--bg":"#eee","--fg":"#111"}},CONFIG:{nightDim:{start:0,end:24,level:.6}},window:{DASHGO_RUNTIME_SETTINGS:{nightDimEnabled:true}},ALERTS:[],document:{head:{appendChild(){}},body,documentElement:rootNode,getElementById:id=>id==="dashboard-theme-vars"?style:id==="nightdim"?dim:null,createElement:()=>style},setTimeout(){},fetch:async()=>({ok:false}),seasonalDecorSignature:()=>"off",applySeasonalDecor:()=>decorCalls++,renderCalendar:()=>renderCalls++,Date:class extends Date{static now(){return 0;}}});
vm.runInContext(runtime+"\n"+health+"\nglobalThis.out={applyTheme,applyNightDim,nightDimOpacity};",context);
context.out.applyTheme("paper");assert.equal(renderCalls,0,"plain theme swap does not rebuild calendar");assert.equal(decorCalls,0,"theme variables do not patch decor synchronously");
assert.ok(!runtime.slice(runtime.indexOf("function applyTheme"),runtime.indexOf("// CACHE-PROOF")).includes("seasonalDecorSignature"),"early theme path does not read later SETTINGS");
context.out.applyNightDim();assert.equal(body.style.filter,undefined,"night dim clears legacy body filter");assert.equal(dim.classList.contains("show"),true,"night dim uses overlay");vm.runInContext("ALERTS=[{severity:\"extreme\"}]",context);context.out.applyNightDim();assert.equal(dim.classList.contains("show"),false,"extreme alert disables dim immediately");
console.log("PASS: atomic theme vars and pointer-overlay night dim stay independent of calendar rendering");
