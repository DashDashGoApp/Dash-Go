#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import {fileURLToPath} from "node:url";

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const source=fs.readFileSync(path.join(root,"ui/js/control-api.js"),"utf8");
let activePage="status",aborts=0,resolveFetch;
const context=vm.createContext({
  console,AbortController,
  CTRL_CACHE:{},CTRL_OPEN:true,CTRL_PAGE_RENDER_SEQ:1,CTRL_TOKEN:"",CTRL_LOCK_STATUS:null,
  SAFE_SESSION:{remove(){}},ctrlLiteProfile:()=>false,ctrlActivePageName:()=>activePage,showPinLock(){},
  fetch:(_url,opts)=>new Promise((resolve,reject)=>{
    opts.signal.addEventListener("abort",()=>{aborts++;const e=new Error("aborted");e.name="AbortError";reject(e);},{once:true});
    resolveFetch=()=>resolve({ok:true,json:async()=>({ok:true})});
  }),
  XMLHttpRequest:class {}
});
vm.runInContext(source+"\nglobalThis.__controlApi={api,ctrlAbortPageRequests,ctrlPageRequestScope,ctrlPageScopeCurrent};",context,{filename:"control-api-source.js"});
const api=context.__controlApi;
const cancelled=api.api("/api/status");
context.CTRL_PAGE_RENDER_SEQ=2;activePage="calendar";
api.ctrlAbortPageRequests("status");
await assert.rejects(cancelled,/request cancelled/);
assert.equal(aborts,1,"tab-scoped abort cancels the active GET request");
context.CTRL_PAGE_RENDER_SEQ=3;activePage="status";
const stale=api.api("/api/status");
context.CTRL_PAGE_RENDER_SEQ=4;activePage="calendar";resolveFetch();
await assert.rejects(stale,/request cancelled/);
assert.equal(api.ctrlPageScopeCurrent({page:"status",seq:3}),false,"stale page scopes cannot mutate a newer tab");
console.log("PASS: Control GET work aborts by page and stale scopes are rejected");
