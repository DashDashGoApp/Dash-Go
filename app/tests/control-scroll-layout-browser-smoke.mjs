#!/usr/bin/env node
// Computed-layout regression coverage for the Control fixed-shell contract.
// It loads split CSS sources directly so source handoffs stay free of generated
// bundles while still proving a visible #ctrlmain is a real flex-constrained
// scroll owner after the PIN visibility transition.
import assert from "node:assert/strict";
import {execFileSync} from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import {fileURLToPath, pathToFileURL} from "node:url";

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const read=rel=>fs.readFileSync(path.join(root,rel),"utf8");
const css=[
  read("ui/css/control/tokens.css"),
  read("ui/css/dashboard/control-overlay.css"),
  read("ui/css/control/console-shell-tabs.css"),
  read("ui/css/control/layout.css"),
].join("\n");
const lock=read("ui/js/control-location-lock.js");
const helper=lock.match(/function setCtrlMainVisible\(visible\)\{[\s\S]*?\n\}/)?.[0];
assert.ok(helper,"Control scroll browser smoke needs the semantic main-visibility helper");
assert.doesNotMatch(lock,/main\.style\.(?:display|setProperty|removeProperty)/,"browser smoke must exercise semantic Control-main visibility, not an inline display override");

function browserPath(){
  const candidates=[
    process.env.DASHGO_CHROMIUM,
    "/usr/bin/chromium",
    "/usr/bin/chromium-browser",
    "/usr/bin/google-chrome",
  ].filter(Boolean);
  const found=candidates.find(candidate=>fs.existsSync(candidate));
  assert.ok(found,"Chromium is required for the Control computed-layout smoke; set DASHGO_CHROMIUM to its executable path");
  return found;
}
function styleText(text){return text.replaceAll("</style","<\\/style");}
function scriptText(text){return text.replaceAll("</script","<\\/script");}
function fixture(){
  return `<!doctype html>
<html><head><meta charset="utf-8"><style>
html,body{width:100%;height:100%;margin:0;overflow:hidden;}
.ctrlhead{display:flex;align-items:flex-start;justify-content:space-between;}
${styleText(css)}
</style></head><body>
<div id="ctrl" class="show"><div id="ctrlpanel">
  <div class="ctrlhead"><h2>Dashboard Control</h2><button id="ctrlclose">×</button></div>
  <div id="ctrlmsg"></div>
  <div id="ctrlpin"></div>
  <div id="ctrlmain" aria-hidden="false">
    <div class="ctrltabs"><button class="cbtn on">Overview</button><button class="cbtn">System</button></div>
    <div class="ctrlpage show" data-scroll-policy="control-page"><div style="height:2200px">Long Control content</div></div>
  </div>
</div></div>
<script>
function $(selector){return document.querySelector(selector);}
${scriptText(helper)}
(function(){
  try{
    const main=$("#ctrlmain"), page=main.querySelector(".ctrlpage.show"), tabs=main.querySelector(".ctrltabs");
    setCtrlMainVisible(false);
    const locked={hidden:main.hidden,ariaHidden:main.getAttribute("aria-hidden")};
    setCtrlMainVisible(true);
    const tabsTopBefore=Math.round(tabs.getBoundingClientRect().top);
    const computed=getComputedStyle(main);
    const overflowY=getComputedStyle(page).overflowY;
    const scrollHeight=page.scrollHeight, clientHeight=page.clientHeight;
    page.scrollTop=Math.min(240,Math.max(0,scrollHeight-clientHeight));
    const tabsTopAfter=Math.round(tabs.getBoundingClientRect().top);
    const result={
      locked,
      visible:{hidden:main.hidden,ariaHidden:main.getAttribute("aria-hidden")},
      computedMainDisplay:computed.display,
      inlineDisplay:main.style.display,
      overflowY,
      scrollHeight,
      clientHeight,
      scrollTop:page.scrollTop,
      tabsTopBefore,
      tabsTopAfter,
    };
    const report=document.createElement("pre");
    report.id="control-scroll-layout-result";
    report.textContent=JSON.stringify(result);
    document.body.appendChild(report);
  }catch(error){
    const report=document.createElement("pre");
    report.id="control-scroll-layout-error";
    report.textContent=String(error&&error.stack||error);
    document.body.appendChild(report);
  }
})();
</script></body></html>`;
}
function runViewport(executable,width,height,dir){
  const file=path.join(dir,`control-scroll-${width}x${height}.html`);
  fs.writeFileSync(file,fixture());
  const output=execFileSync(executable,[
    "--headless=new","--no-sandbox","--disable-gpu","--disable-dev-shm-usage",
    "--force-device-scale-factor=1",`--window-size=${width},${height}`,"--dump-dom",
    pathToFileURL(file).href,
  ],{encoding:"utf8",timeout:20000,stdio:["ignore","pipe","pipe"]});
  const error=output.match(/<pre id="control-scroll-layout-error">([\s\S]*?)<\/pre>/)?.[1];
  assert.equal(error,undefined,`fixture runtime failed at ${width}×${height}: ${error||"unknown error"}`);
  const body=output.match(/<pre id="control-scroll-layout-result">([\s\S]*?)<\/pre>/)?.[1];
  assert.ok(body,`fixture did not emit a computed-layout result at ${width}×${height}`);
  const result=JSON.parse(body.replaceAll("&quot;",'"').replaceAll("&amp;","&"));
  assert.deepEqual(result.locked,{hidden:true,ariaHidden:"true"},`PIN lock must semantically hide #ctrlmain at ${width}×${height}`);
  assert.deepEqual(result.visible,{hidden:false,ariaHidden:"false"},`PIN unlock must restore #ctrlmain semantic visibility at ${width}×${height}`);
  assert.equal(result.inlineDisplay,"",`PIN transition must not write an inline display value at ${width}×${height}`);
  assert.equal(result.computedMainDisplay,"flex",`visible #ctrlmain must compute to flex at ${width}×${height}`);
  assert.ok(["auto","scroll"].includes(result.overflowY),`active Control page must be vertically scrollable at ${width}×${height}, got ${result.overflowY}`);
  assert.ok(result.scrollHeight>result.clientHeight,`active Control page must have finite overflow at ${width}×${height}`);
  assert.ok(result.scrollTop>0,`active Control page scrollTop must advance at ${width}×${height}`);
  assert.equal(result.tabsTopAfter,result.tabsTopBefore,`tab rail must remain fixed while the page scrolls at ${width}×${height}`);
}

const dir=fs.mkdtempSync(path.join(os.tmpdir(),"dash-go-control-scroll-"));
try{
  const executable=browserPath();
  for(const [width,height] of [[1024,600],[1920,1080]])runViewport(executable,width,height,dir);
  console.log("PASS: Chromium confirms semantic PIN visibility preserves a flex-constrained, tab-stable Control page scroll root at 1024×600 and 1920×1080");
}finally{
  fs.rmSync(dir,{recursive:true,force:true});
}
