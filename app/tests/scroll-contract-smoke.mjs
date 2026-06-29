#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import {fileURLToPath} from "node:url";

const app=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const read=p=>fs.readFileSync(path.join(app,p),"utf8");
const index=read("index.html");
const snap=read("ui/js/settings-idle-scroll.js");
const lifecycle=read("ui/js/settings-scroll-lifecycle.js");
const calendar=read("ui/js/calendar-grid.js");
const agenda=read("ui/js/calendar-agenda.js");
const weather=read("ui/js/weather.js");
const popup=read("ui/css/dashboard/popups-alerts-maps.css");
const popupJs=read("ui/js/day-popup.js");
const timelineCss=read("ui/css/dashboard/day-timeline-popup.css");
const controlCss=read("ui/css/control/layout.css");
const contentCss=read("ui/css/control/messages-osk.css")+read("ui/css/control/message-forms.css")+read("ui/css/control/panel-polish.css");
const contentJs=read("ui/js/control-content-osk.js");
const navigation=read("ui/js/control-navigation.js");

function containsAll(source,tokens,label){
  for(const token of tokens)assert.ok(source.includes(token),`${label} missing ${token}`);
}
// The markup advertises only known scroll roots. This avoids a runtime scan or
// a generic listener that could accidentally become a page-wide hot path.
containsAll(index,[
  'id="calscroll" data-scroll-policy="hot-list"',
  'id="agendalist" data-scroll-policy="hot-list"',
  'id="wx14" data-scroll-policy="hot-list"',
  'id="pop" data-scroll-policy="modal"',
  'id="ctrlupdatelog" data-scroll-policy="console"',
  'id="ctrlsystemupdatelog" data-scroll-policy="console"',
  'id="ctrldoctor" data-scroll-policy="console"',
],"known scroll-root markup");
assert.ok(!index.includes('id="ctrllogout"'),"retired Logs console must not remain a scroll root");
for(const id of ["overview","display","calendars","content","control","system"]){
  assert.match(index,new RegExp(`id="ctrlpage-${id}"[^>]*data-scroll-policy="control-page"|data-scroll-policy="control-page"[^>]*id="ctrlpage-${id}"`),`Control page ${id} must declare the page-root policy`);
}

// Input recording and anchor restoration happen around explicit known renders,
// never in a raw scroll callback and never through a mutation observer.
containsAll(lifecycle,["const DASH_SCROLL_ROOT_STATE=new WeakMap()","function scrollRootState","function captureScrollAnchor","function restoreScrollAnchor","inputEpoch","requestAnimationFrame"],"scroll lifecycle");
assert.ok(!lifecycle.includes("MutationObserver"),"scroll lifecycle must not observe every DOM mutation");
assert.ok(!lifecycle.includes('addEventListener("scroll"'),"scroll lifecycle must not attach a scroll listener");
assert.ok(!lifecycle.includes("addEventListener('scroll'"),"scroll lifecycle must not attach a scroll listener");
assert.ok(!lifecycle.includes("addEventListener(\"touchmove\""),"scroll lifecycle must not attach a touchmove blocker");

containsAll(snap,[
  "const DASH_SCROLL_RETURN_PREWARM_MS=400;",
  "function dashboardScrollReturnCreate",
  "function scrollIdleReturnReconcile",
  "root.addEventListener(\"scroll\",state.onScroll,{passive:true})",
  "calendarWeekCullPrewarmAt(top,{before:2,after:2})",
  "calendarWeekCullCommitAt(top)",
],"shared idle return controller");
const calendarScroll=snap.match(/state\.onScroll=\(\)=>\{[\s\S]*?\n  \};/)?.[0]||"";
assert.ok(calendarScroll,"shared controller needs one raw scroll handler");
for(const forbidden of ["offsetTop","getBoundingClientRect","querySelector","appendChild","replaceChildren","innerHTML"]){
  assert.ok(!calendarScroll.includes(forbidden),`calendar raw scroll path must not use ${forbidden}`);
}
containsAll(lifecycle,["scrollIdleReturnReconcile(root)","onComplete"],"anchor restore idle-return reconciliation");
containsAll(calendar,["const homeTop=cw.offsetTop;","setCalendarScrollHomeTop(homeTop)","calendarScrollSnapReconcile()"],"calendar render-time home-offset cache");

containsAll(agenda,["scrollRootState(list,\"hot-list\")","captureScrollAnchor(list","restoreScrollAnchor(list","data-agenda-key","agendaBindDelegatedOpen"],"agenda anchored refresh");
assert.ok(!agenda.includes("row.addEventListener(\"click\""),"Agenda rows must use one delegated opener");
containsAll(weather,["scrollRootState(strip,\"hot-list\")","captureScrollAnchor(strip","restoreScrollAnchor(strip","weatherBindDelegatedOpen","bindTap(strip","data-weather-day"],"forecast anchored refresh");
assert.ok(!weather.includes("cell.addEventListener(\"click\""),"Forecast rows must use one delegated tap handler");

containsAll(popup,["#pop{","overflow-y:auto","overflow-x:hidden","touch-action:pan-y","overscroll-behavior:contain","-webkit-overflow-scrolling:touch"],"generic popup native-scroll policy");
containsAll(controlCss,["#ctrlpanel{","display:flex","flex-direction:column","#ctrlmain{","#ctrlmain[hidden]{display:none;}","#ctrlmain .ctrlpage.show[data-scroll-policy=\"control-page\"]","overflow-y:auto","overflow-x:hidden","touch-action:pan-y","overscroll-behavior:contain","-webkit-overflow-scrolling:touch","will-change:scroll-position"],"Control fixed-shell/page-scroll policy");
const controlTechnicalCss=read("ui/css/control/console-shell-tabs.css");
containsAll(controlTechnicalCss,[".ctrltable-scroll","overflow-x:auto","touch-action:pan-x pan-y"],"Control technical horizontal-scroll policy");
containsAll(contentCss,["#ctrlbuiltins .builtinlist-scroll","max-height:none","overflow:visible","#ctrlpage-content .ctrlsec[open] .ctrlsecbody"],"Messages single-page-root policy");
assert.ok(!contentJs.includes("keepListTop")&&!contentJs.includes("oldList.scrollTop"),"Messages renderer must not restore an inner list scroll position");

// The day Timeline remains a deliberately static exception: one initial
// position calculation and staged initial card construction, never a scroll
// virtualizer or scroll-time DOM/layout work.
containsAll(popupJs,["body.dataset.scrollPolicy=\"day-timeline\"","function dtStageTimelineCards","function dtCancelTimelineStage","body._dtUserScrolled"],"day timeline static-scroll policy");
assert.ok(!popupJs.includes('addEventListener("scroll"'),"day Timeline must not attach a scroll listener");
assert.ok(!popupJs.includes("function dtWindow"),"day Timeline must not revive a scroll-window virtualizer");
assert.ok(!timelineCss.includes("content-visibility"),"day Timeline cards must not use deferred-content placeholders");

// Accordion focus gets one cancellation/input-aware programmatic request.
containsAll(navigation,["let CTRL_SECTION_FOCUS_TOKEN=0","state.inputEpoch","focusCtrlSection","d.scrollIntoView"],"Control focus-scroll guard");
console.log("PASS: native-scroll contracts cover known roots, no scroll-time calendar layout work, anchored Agenda/Forecast refreshes, an authoritative fixed Control shell with one page scroll root, console/table exceptions, and static day Timeline safety");
