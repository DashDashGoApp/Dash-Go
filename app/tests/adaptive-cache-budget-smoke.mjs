#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import {fileURLToPath} from "node:url";

const app=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const read=p=>fs.readFileSync(path.join(app,p),"utf8");
const memory=read("internal/platform/system.go");
const status=read("internal/platform/system.go");
const api=read("ui/js/control-api.js");
const lifecycle=read("ui/js/control-lifecycle.js");
const liteMemory=read("ui/js/control-lite-memory.js");
const statusUI=read("ui/js/control-status-health.js");

for(const token of ["func MemorySnapshotMB", "func ParseMemoryInfoMB", "MemAvailable:", "SwapTotal:", "SwapFree:"]){
  assert.ok(memory.includes(token),`memory parser missing ${token}`);
}
assert.ok(status.includes('"swap_used_mb": swap'),"status payload must report used swap from the same snapshot");
assert.match(statusUI,/\[\"Swap used\",\s*st\.swap_used_mb/,"Device status must surface used swap");
assert.ok(api.includes("function ctrlObserveCachedPayload")&&api.includes('path==="/api/status"'),"existing status cache must update the budget without a second hot path");
assert.ok(lifecycle.includes("ctrlScheduleCacheBudgetProbe"),"Control open must schedule one delayed budget probe only when needed");
for(const token of ["CTRL_CACHE_BUDGET_POLL_MS=90000", "CTRL_RETAINABLE_LITE_PAGES", "CTRL_HEAVY_LAZY_KEYS", "function ctrlCacheBudgetTierFor", "function ctrlHibernatedPageLimit", "function ctrlObserveCacheBudgetStatus"]){
  assert.ok(liteMemory.includes(token),`cache budget contract missing ${token}`);
}
assert.ok(!liteMemory.includes("setInterval("),"adaptive cache budget must not start a memory polling loop");

const pages=new Map();const evicted=[];
function page(name,loaded=[]){
  return {
    id:"ctrlpage-"+name,
    querySelectorAll(selector){
      if(selector==="details.ctrlsec[data-lazy]")return loaded.map(key=>({dataset:{lazy:key,loaded:"1"}}));
      return [];
    },
    querySelector(){return null;}
  };
}
for(const name of ["overview","display","calendars","control"])pages.set("ctrlpage-"+name,page(name));
const context=vm.createContext({
  CTRL_OPEN:true, CTRL_CACHE:{}, setTimeout(){return 1;}, clearTimeout(){},
  ctrlLiteProfile(){return true;}, ctrlEvictPage(node,reason){evicted.push([node.id,reason]);},
  ctrlAbortRequests(){}, $(){return null;}, api:async()=>({mem_avail_mb:212,swap_used_mb:38}),
  document:{documentElement:{dataset:{}},getElementById:id=>pages.get(id)||null,querySelectorAll(){return [];}}
});
vm.runInContext(`${liteMemory}\nglobalThis.__budget={tier:ctrlCacheBudgetTierFor,observe:ctrlObserveCacheBudgetStatus,limit:ctrlHibernatedPageLimit,remember:ctrlRememberHibernatedPage,lru:()=>CTRL_PAGE_LRU.slice(),state:()=>({...CTRL_CACHE_BUDGET})};`,context);
assert.equal(context.__budget.tier({mem_avail_mb:212,swap_used_mb:38},null),"roomy");
assert.equal(context.__budget.tier({mem_avail_mb:150,swap_used_mb:38},38),"normal");
assert.equal(context.__budget.tier({mem_avail_mb:119,swap_used_mb:38},38),"conservative");
assert.equal(context.__budget.tier({mem_avail_mb:212,swap_used_mb:47},38),"conservative","rising swap must override roomy available RAM");
context.__budget.observe({mem_avail_mb:212,swap_used_mb:38});
assert.equal(context.__budget.limit(),2,"roomy Lite systems retain two safe hidden pages");
assert.equal(context.document.documentElement.dataset.ctrlCacheBudget,"roomy");
context.__budget.remember(pages.get("ctrlpage-overview"));
context.__budget.remember(pages.get("ctrlpage-calendars"));
assert.deepEqual([...context.__budget.lru()],["calendars","overview"]);
context.__budget.remember(pages.get("ctrlpage-display"));
assert.deepEqual([...context.__budget.lru()],["display","calendars"],"third lightweight page must evict LRU at roomy limit");
assert.ok(evicted.some(([id])=>id==="ctrlpage-overview"),"roomy cap must evict the oldest retained page");
context.__budget.observe({mem_avail_mb:150,swap_used_mb:38});
assert.equal(context.__budget.limit(),1,"normal Lite systems retain only one hidden page");
assert.equal(context.__budget.lru().length,1,"budget downgrade trims retained DOM immediately");
console.log("PASS: beta.22 adaptive Control cache uses MemAvailable plus swap trend, has no poll loop, retains at most two safe Lite pages, and evicts on pressure");
