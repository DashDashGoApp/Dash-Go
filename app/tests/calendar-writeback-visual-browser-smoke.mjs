#!/usr/bin/env node
// Computed-layout coverage for the Dash-Go-owned event-calendar picker and
// grouped quick-time controls. It loads split CSS directly so source handoffs
// remain free of generated bundles.
import assert from "node:assert/strict";
import {execFileSync} from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import {fileURLToPath,pathToFileURL} from "node:url";

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const read=rel=>fs.readFileSync(path.join(root,rel),"utf8");
const css=[
  read("ui/css/dashboard/base.css"),
  read("ui/css/dashboard/popups-alerts-maps.css"),
  read("ui/css/dashboard/calendar-writeback.css"),
].join("\n");
function chromium(){
  const candidates=[process.env.DASHGO_CHROMIUM,"/usr/bin/chromium","/usr/bin/chromium-browser","/usr/bin/google-chrome"].filter(Boolean);
  const found=candidates.find(candidate=>fs.existsSync(candidate));
  assert.ok(found,"Chromium is required for the calendar writeback computed-layout smoke; set DASHGO_CHROMIUM to its executable path");
  return found;
}
function styleText(text){return text.replaceAll("</style","<\\/style");}
function fixture(){
  return `<!doctype html><html><head><meta charset="utf-8"><style>
html,body{width:100%;height:100%;margin:0;overflow:hidden;}
${styleText(css)}
</style></head><body><div id="scrim" class="show"><section id="pop" class="calendareventform"><div id="popbody"><form class="calendar-writeback-form">
<label class="calendar-writeback-field"><span class="calendar-writeback-label">Calendar</span>
  <div id="picker" class="calendar-writeback-calendar-picker"><button id="trigger" type="button" class="calendar-writeback-calendar-trigger" aria-controls="options" aria-expanded="false"><span class="calendar-writeback-calendar-copy"><span class="calendar-writeback-calendar-name">Family</span><span class="calendar-writeback-calendar-meta">Google calendar · Choose calendar</span></span><span class="calendar-writeback-calendar-chevron">⌄</span></button><div id="options" class="calendar-writeback-calendar-options" role="group" aria-label="Calendar choices" hidden><button type="button" class="calendar-writeback-calendar-option is-selected" aria-pressed="true"><span class="calendar-writeback-calendar-copy"><span class="calendar-writeback-calendar-name">Family</span><span class="calendar-writeback-calendar-meta">Google calendar · Selected</span></span></button><button type="button" class="calendar-writeback-calendar-option" aria-pressed="false"><span class="calendar-writeback-calendar-copy"><span class="calendar-writeback-calendar-name">Personal</span><span class="calendar-writeback-calendar-meta">iCloud / CalDAV</span></span></button></div></div>
</label><div class="calendar-writeback-quick"><span class="calendar-writeback-quick-label">Start</span><div class="calendar-writeback-quick-options"><button type="button" class="calendar-writeback-quick-chip">−1 hr</button><button type="button" class="calendar-writeback-quick-chip">−15 min</button><button type="button" class="calendar-writeback-quick-chip">+15 min</button><button type="button" class="calendar-writeback-quick-chip">+1 hr</button></div></div></form></div></section></div><script>
const picker=document.getElementById("picker"),trigger=document.getElementById("trigger"),options=document.getElementById("options");
const initial={
  nativeSelects:document.querySelectorAll("select").length,
  triggerHeight:Math.round(trigger.getBoundingClientRect().height),
  optionsDisplay:getComputedStyle(options).display,
  quickDisplay:getComputedStyle(document.querySelector(".calendar-writeback-quick")).display,
  quickOverflow:getComputedStyle(document.querySelector(".calendar-writeback-quick")).overflowY,
};
options.hidden=false;picker.classList.add("is-open");trigger.setAttribute("aria-expanded","true");
const opened={optionsDisplay:getComputedStyle(options).display,optionsOverflow:getComputedStyle(options).overflowY,pickerOverflow:getComputedStyle(picker).overflowY,expanded:trigger.getAttribute("aria-expanded")};
const report=document.createElement("pre");report.id="calendar-writeback-visual-result";report.textContent=JSON.stringify({initial,opened});document.body.appendChild(report);
</script></body></html>`;
}
function run(executable,width,height,dir){
  const file=path.join(dir,`calendar-writeback-${width}x${height}.html`);fs.writeFileSync(file,fixture());
  const output=execFileSync(executable,["--headless=new","--no-sandbox","--disable-gpu","--disable-dev-shm-usage","--force-device-scale-factor=1",`--window-size=${width},${height}`,"--dump-dom",pathToFileURL(file).href],{encoding:"utf8",timeout:20000,stdio:["ignore","pipe","pipe"]});
  const body=output.match(/<pre id="calendar-writeback-visual-result">([\s\S]*?)<\/pre>/)?.[1];assert.ok(body,`fixture did not emit a layout result at ${width}×${height}`);
  const result=JSON.parse(body.replaceAll("&quot;",'"').replaceAll("&amp;","&"));
  assert.equal(result.initial.nativeSelects,0,`calendar picker must not use a browser select at ${width}×${height}`);
  assert.ok(result.initial.triggerHeight>=52,`calendar trigger must keep a 52px touch target at ${width}×${height}`);
  assert.equal(result.initial.optionsDisplay,"none",`closed options must not occupy layout at ${width}×${height}`);
  assert.equal(result.initial.quickDisplay,"grid",`quick time controls must remain one grouped grid at ${width}×${height}`);
  assert.equal(result.initial.quickOverflow,"visible",`quick time controls may not become a nested scroll port at ${width}×${height}`);
  assert.equal(result.opened.optionsDisplay,"grid",`opened options must render as a local themed grid at ${width}×${height}`);
  assert.equal(result.opened.optionsOverflow,"visible",`opened options may not become a nested scroll port at ${width}×${height}`);
  assert.equal(result.opened.pickerOverflow,"visible",`calendar picker may not create a nested scroll port at ${width}×${height}`);
  assert.equal(result.opened.expanded,"true",`calendar trigger must expose expanded state at ${width}×${height}`);
}
const dir=fs.mkdtempSync(path.join(os.tmpdir(),"dash-go-calendar-writeback-"));
try{
  const executable=chromium();
  for(const [width,height] of [[1024,600],[1920,1080]])run(executable,width,height,dir);
  console.log("PASS: Chromium confirms the themed calendar picker and quick-time controls keep touch geometry and one popup scroll surface at 1024×600 and 1920×1080");
}finally{fs.rmSync(dir,{recursive:true,force:true});}
