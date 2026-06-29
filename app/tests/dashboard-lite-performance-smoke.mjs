#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import {fileURLToPath} from "node:url";

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const read=rel=>fs.readFileSync(path.join(root,rel),"utf8");
const fit=read("ui/js/dashboard-fit.js");
const agenda=read("ui/js/calendar-agenda.js");
const weather=read("ui/js/weather.js");
const calendar=read("ui/css/dashboard/calendar.css");
const responsive=read("ui/css/dashboard/responsive.css");
const liteCss=read("ui/css/dashboard/lite-dashboard.css");
const grid=read("ui/js/calendar-grid.js");
const cull=read("ui/js/calendar-cull-overscan.js");
const spanBars=read("ui/js/calendar-span-bars.js");
const sidebar=read("ui/css/dashboard/sidebar-weather-messages.css");
const compliments=read("ui/js/messages-fit.js");
const liteCompliments=read("ui/js/messages-lite-fit.js");

assert.match(fit,/function dashboardFitLiteProfile\(\)/,"fit controller must have a shared Lite/Zero profile gate");
assert.match(fit,/if\(dashboardFitLiteProfile\(\)\)\{[\s\S]*?dashboardFitBlankState\(tier\)[\s\S]*?return;/,"Lite controller path must exit before measured dock work");
assert.match(fit,/if\(dashboardFitLiteProfile\(\)\|\|DASHBOARD_FIT\.renderQueued\)return;/,"Lite must never queue geometry-triggered Calendar or Agenda renders");
assert.match(fit,/dashboardFitPrimeBootSignature\(\);\s*dashboardFitController\("boot"\);/,"boot must seed the head-tier signature before controller work");
assert.doesNotMatch(agenda,/dashboardFitSchedule/,"Agenda paint must not schedule fit work after every render");
assert.doesNotMatch(weather,/dashboardFitSchedule/,"Weather paint/error paths must not schedule fit work after every render");
assert.ok(grid.split(/\n/).length<=400,"Calendar grid renderer must stay below the split-source navigability cap");
assert.match(spanBars,/function fillSpanBar\(bar,it\)/,"Calendar span-bar display helpers must live in their focused split module");
assert.match(grid,/fillSpanBar\(bar,it\);/,"Calendar grid must use the split span-bar display helper");
assert.match(calendar,/\.spanbar \.spantitle\{[\s\S]*?overflow-wrap:break-word;/,"span titles must retain safe long-token wrapping");
assert.match(calendar,/\.ev \.etitle\{[\s\S]*?overflow-wrap:break-word;/,"event titles must retain safe long-token wrapping");
assert.match(responsive,/\.ev,\.spanbar\{overflow-wrap:break-word;\}/,"calendar shells must use the lower-cost break-word mode");
assert.doesNotMatch(responsive,/\.ev,\.spanbar\{overflow-wrap:anywhere;\}/,"calendar shells must not retain anywhere on every render");
assert.match(compliments,/const lite=complimentLiteProfile\(\),metrics=lite\?complimentLiteMetricsForFit\(el\):complimentBoxMetrics\(el\)/,"Lite compliment rotation must use cached geometry rather than a DOM measurement path");
assert.match(liteCompliments,/function complimentLiteReadGeometry\(el\)/,"Lite must capture a real message-box snapshot after layout changes");
assert.match(liteCompliments,/function complimentLiteLineCount\(text,size,metrics,limit\)/,"Lite must use Canvas word-aware fitting");
assert.match(liteCompliments,/function complimentLiteReadingFloors\(metrics\)/,"Lite must retain tier-and-line reading floors");
assert.match(liteCompliments,/function complimentLiteScheduleGeometryCapture\(el,reason\)/,"Lite must coalesce geometry captures outside normal rotation");
assert.match(liteCompliments,/for\(const node of \[parent,sun,stale\]\)/,"Lite must observe footer geometry sources, not rotating text");
assert.doesNotMatch(liteCompliments,/observer\.observe\(el\)/,"Lite must not observe #comptext on every message swap");
assert.doesNotMatch(liteCompliments,/function complimentLiteTextBucket/,"Lite must no longer use broad shape-bucket cache keys");
const liteFitLogic=(liteCompliments.match(/function complimentLiteFit\(text,metrics\)\{[\s\S]*?\n\}/)?.[0]||"").replace(/\/\/.*$/gm,"");
assert.ok(liteFitLogic,"Lite message fit function must be present");
assert.doesNotMatch(liteFitLogic,/getComputedStyle|clientWidth|clientHeight|getBoundingClientRect|scrollWidth|scrollHeight/,"Lite message rotation must not force layout reads");
assert.match(liteCss,/@supports \(content-visibility:auto\)\{[\s\S]*?html\.profile-lite #calscroll\[data-week-cull-ready="1"\] \.weekrow\{[\s\S]*?content-visibility:auto;[\s\S]*?contain-intrinsic-size:auto var\(--rowheight\);/,
  "Lite week culling must be feature-guarded, profile-scoped, and retain the real row height");
assert.match(liteCss,/\.weekrow\[data-cull-near="1"\]\{[\s\S]*?content-visibility:visible;/,
  "Lite culling must keep its measured warm window paint-ready");
assert.doesNotMatch(calendar,/\.weekrow\{[\s\S]*?content-visibility:auto;/,
  "base Calendar rows must remain fully rendered until the Lite-ready marker opts them in");
assert.match(cull,/function calendarSetWeekCullReady\(ready,scroll\)\{[\s\S]*?if\(ready\)\{[\s\S]*?target\.dataset\.weekCullReady="1";[\s\S]*?calendarWeekCullEnable\(target\);[\s\S]*?else\{[\s\S]*?delete target\.dataset\.weekCullReady;[\s\S]*?calendarWeekCullDisable\(target\);/,
  "Calendar needs one explicit culling readiness marker with deterministic cleanup");
assert.match(cull,/const CALENDAR_WEEK_CULL_BEHIND_OVERSCAN=1;[\s\S]*?const CALENDAR_WEEK_CULL_AHEAD_OVERSCAN=2;[\s\S]*?const CALENDAR_WEEK_CULL_WARM_MS=250;/,
  "Lite culling must preserve bounded directional overscan and a short warm hold");
assert.match(grid,/function finishCalendarDayEvents\(\)\{[\s\S]*?lists\.forEach\(evlist=>fitDayEventList\(evlist,fitGap\)\);[\s\S]*?calendarSetWeekCullReady\(true\);/,
  "Lite culling may start only after Calendar event fitting completes");
assert.match(grid,/function requestCalendarLayoutFit\(reason,opts\)\{[\s\S]*?_calendarFitSig=sig;[\s\S]*?calendarSetWeekCullReady\(false,scroll\);[\s\S]*?requestAnimationFrame\(\(\)=>runCalendarFitPipeline/,
  "Calendar must clear Lite culling before every geometry measurement pass");
assert.match(grid,/function renderCalendar\(opts\)\{[\s\S]*?_calendarRenderSig=renderSig;[\s\S]*?calendarSetWeekCullReady\(false,scroll\);[\s\S]*?renderCalHead\(\);/,
  "Calendar must clear Lite culling before replacing week rows");
assert.match(liteCss,/html\.profile-lite \.listsdock-ticker\.is-moving \.listsdock-ticker-track\{[\s\S]*?animation:none;[\s\S]*?will-change:auto;/,
  "Lite Lists ticker must be static and release its permanent compositor hint");
assert.match(sidebar,/#comptext\{[\s\S]*?transition:opacity var\(--compfade\) ease-in-out;[\s\S]*?will-change:opacity;[\s\S]*?backface-visibility:hidden;/,
  "base message behavior must retain its normal fade and capable-profile layer hints");
assert.match(liteCss,/html\.profile-lite #comptext\{[\s\S]*?will-change:auto;[\s\S]*?backface-visibility:visible;/,
  "Lite messages must drop only the idle compositor hints while keeping the fade");
assert.doesNotMatch(liteCss,/html\.profile-lite #comptext\{[\s\S]*?transition:none;/,
  "Lite message swaps must retain the existing visual fade");
assert.match(liteCss,/html\.profile-lite #compliment \.cb-launch,\s*html\.profile-lite #snapback\{[\s\S]*?box-shadow:none;/,
  "Lite paint diet must remove only the selected decorative static shadows");

function classList(initial=[]){
  const values=new Set(initial);
  return {
    contains:name=>values.has(name),
    toggle:(name,on)=>{if(on)values.add(name);else values.delete(name);},
    remove:(...names)=>names.forEach(name=>values.delete(name)),
    toString:()=>[...values].sort().join(" "),
  };
}
function fitContext(profile){
  const app={classList:classList()};
  const rootEl={dataset:{fit:"base"},style:{setProperty(){}}};
  const document={
    documentElement:rootEl,
    getElementById:id=>id==="app"?app:null,
    addEventListener(){},
  };
  const window={innerWidth:1920,innerHeight:1080,devicePixelRatio:1};
  window.window=window;
  const context={
    CONFIG:{profile,layoutProfile:"balanced"},window,document,Math,Number,String,Object,Array,Set,console,
    setTimeout,clearTimeout,requestAnimationFrame:fn=>fn(),
  };
  context.globalThis=context;
  return {context,app,rootEl};
}

{
  const {context,app,rootEl}=fitContext("lite");
  vm.createContext(context);
  vm.runInContext(`${fit}
globalThis.__measureCalls=0; dashboardFitMeasure=()=>{globalThis.__measureCalls+=1; throw new Error("Lite must not measure")}; dashboardFitPrimeBootSignature(); dashboardFitController("test"); dashboardFitSchedule("resize",0); globalThis.__state={measureCalls:globalThis.__measureCalls,fit:document.documentElement.dataset.fit,classes:document.getElementById("app").classList.toString(),limit:dashboardFitWeatherDayLimit()};`,context);
  assert.equal(context.__state.measureCalls,0,"Lite controller/schedule must not read measured geometry");
  assert.equal(context.__state.fit,"base","Lite controller must retain CSS-pixel tier tokens");
  assert.equal(context.__state.classes,"","Lite controller must not enter dock or stack mode");
  assert.equal(context.__state.limit,0,"Lite must not render a reduced inline forecast from dock state");
  assert.equal(app.classList.toString(),"", "Lite app shell must remain undocked");
  assert.equal(rootEl.dataset.fit,"base","Lite controller must retain the selected viewport tier");
}
{
  const {context}=fitContext("balanced");
  vm.createContext(context);
  vm.runInContext(`${fit}
globalThis.__queueCalls=0; dashboardFitQueueGeometryRender=()=>{globalThis.__queueCalls+=1}; dashboardFitPrimeBootSignature(); dashboardFitController("boot");`,context);
  assert.equal(context.__queueCalls,0,"head-selected ordinary first paint must not enqueue a duplicate Calendar/Agenda render");
}

console.log("PASS: Lite dashboard fit skips layout measurement/rerenders, week culling starts only after Calendar geometry fit, idle dashboard layers stay bounded, and compliments use cached geometry plus Canvas fitting");
