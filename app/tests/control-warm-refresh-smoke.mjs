#!/usr/bin/env node
// Beta.92 regression guard: a warm Dashboard Control refresh keeps live card
// content and its measured height stable until the replacement DOM is ready.
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import {fileURLToPath} from "node:url";

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const read=rel=>fs.readFileSync(path.join(root,rel),"utf8");
const core=read("ui/js/control-ui.js");
const nav=read("ui/js/control-navigation.js");
const tap=read("ui/js/tap.js");
const comp=read("ui/js/control-content-osk.js");
const css=read("ui/css/control/panel-polish.css");

assert.match(core,/function ctrlBeginWarmRefresh\(/,"Control needs a warm-refresh lifecycle helper");
assert.match(core,/new MutationObserver/,"warm refresh must detect the renderer's final child swap");
assert.match(core,/wrap\.style\.minHeight=Math\.ceil\(Number\(rect\.height\)\)\+"px"/,
  "warm refresh must reserve the previous live-card height");
assert.match(core,/wrap\.setAttribute\("aria-busy","true"\)/,"warm refresh must expose busy state without replacing content");
assert.match(core,/if\(ctrlWarmContentPresent\(wrap\)\)\{[\s\S]*?ctrlBeginWarmRefresh\(wrap,"Refreshing…"\);[\s\S]*?return true;/,
  "ctrlSetLoading must preserve non-empty live cards");
assert.match(css,/\[data-ctrl-refresh-label\]\.ctrlrefreshing::after/,
  "busy affordance must be paint-only rather than layout content");
assert.match(css,/position:absolute/,
  "busy affordance must not expand a refreshed card");
assert.match(nav,/function switchCtrlPage\(name,opts\)/,"page switcher must accept a pending target section");
assert.match(nav,/collapseCtrlPageSections\(page\);[\s\S]*?if\(requested\)requested\.open=true;[\s\S]*?setCtrlPageVisual\(name,page\);/,
  "contextual target section must open before its page is shown");
assert.match(nav,/function queueCtrlSectionLoad\(/,"lazy sections need one serialized load queue");
assert.match(nav,/if\(d\._lazyPromise\)return d\._lazyPromise;/,"simultaneous route/toggle loads must deduplicate");
assert.doesNotMatch(nav,/setTimeout\(open,ctrlLiteProfile\(\)\?70:0\)/,
  "contextual route opening must not retain the Lite all-collapsed delay");
assert.doesNotMatch(comp,/ctrlAfterNextPaint\(\)/,
  "Personal messages must not deliberately paint a compact loader during warm refresh");
assert.match(comp,/btns\.appendChild\(cbtn\("Cancel","",\(\)=>\{top\.style\.display="";drawList\(\);\}\)\)/,
  "Personal-message Cancel must restore the resident list without a network redraw");
assert.match(tap,/const duplicate=Date\.now\(\)<suppressClickUntil;/,
  "detail-zero touch clicks must use the accepted pointer-release duplicate window");

let observer=null;
class FakeMutationObserver{
  constructor(fn){this.fn=fn;observer=this;}
  observe(){}
  disconnect(){this.disconnected=true;}
}
const classes=new Set();
const attrs=new Map();
const style={minHeight:"",removeProperty(name){if(name==="min-height")this.minHeight="";}};
const wrap={
  children:[{classList:{contains:()=>false}}],style,dataset:{},
  classList:{add(name){classes.add(name);},remove(name){classes.delete(name);}},
  getBoundingClientRect(){return {height:286};},
  getAttribute(name){return attrs.has(name)?attrs.get(name):null;},
  setAttribute(name,value){attrs.set(name,String(value));},
  removeAttribute(name){attrs.delete(name);},
  replaceChildren(){throw new Error("warm refresh must not replace live content before data is ready");},
};
let nextTimer=0;
const timers=new Map();
const context={
  Array,Object,Math,Number,MutationObserver:FakeMutationObserver,
  requestAnimationFrame(fn){fn();return 1;},
  setTimeout(fn,ms){const id=++nextTimer;if(ms===15000){timers.set(id,fn);return id;}fn();return id;},
  clearTimeout(id){timers.delete(id);},
  "$":()=>null,el(){throw new Error("cold path should not run");},escapeHTML:value=>String(value),
};
vm.createContext(context);
vm.runInContext(core+"\nglobalThis.__warm={ctrlSetLoading};",context,{filename:"control-core-ui.js"});
assert.equal(context.__warm.ctrlSetLoading(wrap,"Checking",""),true,"existing card must enter warm refresh mode");
assert.equal(wrap.style.minHeight,"286px","warm refresh must reserve prior height");
assert.equal(wrap.getAttribute("aria-busy"),"true","warm refresh must be accessible busy state");
assert.ok(classes.has("ctrlrefreshing"),"warm refresh indicator class must be active");
assert.ok(observer,"warm refresh must observe the final child commit");
observer.fn([{type:"childList",addedNodes:[{}],removedNodes:[{}]}]);
assert.equal(wrap.style.minHeight,"","height reservation must clear only after the committed replacement paint");
assert.equal(wrap.getAttribute("aria-busy"),null,"busy state must clear after the committed replacement paint");
assert.ok(!classes.has("ctrlrefreshing"),"busy indicator must clear after commit");

console.log("PASS: Control warm refresh preserves live content, route opening, and touch-card stability");
