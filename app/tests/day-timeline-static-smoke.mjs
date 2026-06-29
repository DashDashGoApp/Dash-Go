#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import {fileURLToPath} from "node:url";
const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const read=p=>fs.readFileSync(path.join(root,p),"utf8");
const day=read("ui/js/day-popup.js");
const event=read("ui/js/event-popup.js");
const css=read("ui/css/dashboard/day-timeline-popup.css");
const kiosk=read("kiosk.sh");
for(const [source,token,label] of [
  [day,"const DT_CARD_STAGE_CHUNK=16","bounded staged card batch"],
  [day,"function dtStageTimelineCards","one-time timeline staging"],
  [day,"function dtCancelTimelineStage","staged-build cancellation"],
  [day,"wrap._dtTimelineStage","cached static timeline stage"],
  [day,"dtLiteDayPopupProfile","Lite list-first selector"],
  [day,"initial=(dtLiteDayPopupProfile()&&model.timed.length)?\"list\"","Lite defaults to List when timed events exist"],
  [day,"body._dtUserScrolled","user scroll precedence"],
  [day,"if(previous===\"timeline\"&&kind!==\"timeline\")dtCancelTimelineStage","view switch cancels unfinished build"],
  [event,"function dtBeginDayCardPaintContext","one-time popup paint context"],
  [event,"function dtPanelBaseRGB","opaque panel-color baseline"],
  [event,"function dtOpaque","opaque event color blend"],
  [event,"paint&&paint.lite","Lite opaque card background path"],
  [css,"html.profile-lite #pop.daytimelinepop .dt-grid","Lite opaque grid rule"],
  [css,"box-shadow:none;border-radius:3px;text-rendering:optimizeSpeed","Lite flat-card paint rule"],
  [css,"html.profile-lite #pop.daytimelinepop .dt-card-dot{box-shadow:none;}","Lite dot-shadow removal"],
  [kiosk,"export WEBKIT_DISABLE_COMPOSITING_MODE=1","Lite software-compositing safety baseline"]
])assert.ok(source.includes(token),`missing ${label}`);
assert.ok(!day.includes("function dtWindow"),"Lite timeline must not retain a scroll-driven windower");
assert.ok(!day.includes('addEventListener("scroll"'),"Lite timeline must not install a scroll handler");
assert.ok(!day.includes("getBoundingClientRect"),"initial timeline positioning must not force rectangle measurement");
assert.ok(!css.includes("content-visibility")&&!css.includes("contain-intrinsic-size"),"day popup cards must never defer painting to scroll time");
assert.ok(!css.includes("dt-event-stripe"),"timeline must not restore stripe paint nodes");
assert.ok(!css.includes("0 6px 18px")&&!css.includes("0 4px 14px"),"timeline must not restore broad blurred shadows");
assert.ok(!kiosk.includes("DASHGO_LITE_COMPOSITING"),"beta.11 must not ship unvalidated Lite compositing re-enable behavior");
console.log("PASS: Lite day timeline uses a one-time static staged build, no scroll-time DOM churn, opaque paint, and List-first default");
