#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import {fileURLToPath} from "node:url";

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const read=rel=>fs.readFileSync(path.join(root,rel),"utf8");
const css=read("ui/css/dashboard/sidebar-weather-messages.css");
const responsive=read("ui/css/dashboard/responsive.css");
const fit=read("ui/js/messages-fit.js");
const lite=read("ui/js/messages-lite-fit.js");
const layout=read("ui/js/messages-layout.js");
const schedule=read("ui/js/messages-schedule.js");
const core=read("ui/js/messages-core.js");

assert.match(css,/height:var\(--compliment-height\)/,"compliment band must consume the shared responsive height token");
assert.match(css,/flex:0 0 var\(--compliment-height\)/,"compliment flex allocation must use the same responsive height token");
assert.match(responsive,/--compliment-height:clamp\(124px,12vh,200px\)/,"xl tier must grow the message band on large displays");
assert.match(responsive,/:root\[data-fit="min"\]\{[\s\S]*?--compliment-height:84px/,"small tier must return space to the calendar intentionally");
assert.doesNotMatch(css,/height:124px;\s*\n\s*min-height:124px;\s*\n\s*max-height:124px/,"compliment band must not retain a fixed letterbox height");
assert.match(css,/overflow-wrap:anywhere/,"message text must break long unbroken tokens instead of clipping horizontally");
assert.match(css,/word-break:break-word/,"message text needs a WebKit-safe word-break fallback");
assert.match(css,/white-space:pre-line/,"planned display-only line breaks must render as literal text lines");
assert.match(css,/--comptext-planned-pad-y:6px/,"selected multi-line layouts must reclaim only their unused vertical inset");
assert.match(fit,/function complimentBoxMetrics\(el\)/,"capable profiles must retain measured box fitting");
assert.match(fit,/function complimentVisualCap\(metrics\)/,"capable fitting must derive a seed from box measurements");
// W1: one readable-size table drives both fit paths. The plan's numbers are
// the contract: preferred [1,2,3]-line floors, the drop threshold a
// non-priority message may never be rendered below, and the priority floor
// for a fourth line. The absolute hard floor is the 16px last resort only.
assert.match(fit,/const COMP_READABLE=/,"readable size contract must live in one named table");
assert.match(fit,/base:\s*\{floors:\[48,36,28\],drop:28,priority:24\}/,"base tier must carry the plan's readable table verbatim");
assert.match(fit,/min:\s*\{floors:\[30,22,18\],drop:18,priority:16\}/,"min tier must carry the plan's readable table verbatim");
assert.match(fit,/hardFloor:16/,"the absolute hard floor must be 16px and belong to priority text only");
assert.match(fit,/function complimentReadableTier\(metrics\)/,"both fit paths must resolve one shared tier table");
assert.match(fit,/function complimentReadableThreshold\(metrics,kind\)/,"both fit paths must share one readable-threshold source");
assert.doesNotMatch(fit,/Math\.min\(floor,COMP_FIT\.hardFloor\)/,"the former dead scaled-floor override must not return");
assert.match(fit,/const lite=complimentLiteProfile\(\),metrics=lite\?complimentLiteMetricsForFit\(el\):complimentBoxMetrics\(el\)/,"Lite must use its cached geometry path rather than read the DOM on each rotation");
assert.match(lite,/function complimentLiteReadGeometry\(el\)/,"Lite must capture actual rendered geometry after layout changes");
assert.match(lite,/fontFamily:elStyle\.fontFamily[\s\S]*?fontWeight:elStyle\.fontWeight/,"Lite snapshot must retain effective font family and weight for Canvas measurement");
assert.match(lite,/function complimentLiteLineCount\(text,size,metrics,limit\)/,"Lite must use word-aware Canvas line measurement");
assert.match(lite,/function complimentLiteMetricsForLines\(metrics,lines\)/,"Lite must derive selected multi-line capacity from cached geometry");
assert.match(lite,/fontsReady/,"Lite geometry must distinguish settled fonts from the emergency fallback");
assert.match(lite,/raw\*\(ready\?1\.03:1\.045\)/,"Lite Canvas width must include a conservative WebKit wrap allowance");
assert.match(lite,/--comptext-base-pad-y/,"Lite geometry snapshots must remain anchored to the normal message inset");
assert.match(layout,/function complimentLayoutCandidates\(text\)/,"message layout must enumerate bounded display candidates");
assert.match(layout,/question-answer/,"question-and-answer punctuation must receive a semantic candidate");
assert.match(layout,/function complimentLayoutChoose\(text,assess\)/,"message layout must compare fitted candidates rather than force a line count");
assert.match(fit,/__dashComplimentRawText/,"display-only line planning must preserve raw rotation text outside the rendered composition");
assert.match(lite,/function complimentLiteReadingFloors\(metrics\)/,"Lite must define tier-and-line reading floors from the shared table");
assert.match(lite,/COMP_READABLE/,"Lite reading floors must come from the one shared readable table");
assert.match(fit,/function complimentVerticalFitReserve\(size,lines,lite\)/,"message fitting must reserve a small vertical WebKit safety margin");
assert.match(fit,/function complimentFitLineHeight\(lines\)\{return Number\(lines\)<=1\?1\.06:1\.13;\}/,"fitting must budget more height than the applied WebKit line-height");
assert.match(lite,/metrics\.contentHeight-complimentVerticalFitReserve/,"Lite Canvas fitting must apply the vertical safety reserve");
assert.match(lite,/complimentFitLineHeight\(lines\)/,"Lite height fitting must use the conservative line-box budget");
assert.match(lite,/maxLines:3/,"non-priority Lite fits must clamp at three lines");
assert.match(fit,/maxLines:4/,"priority Lite fits may use a bounded fourth line");
assert.match(fit,/el\.style\.webkitLineClamp=String\(lines\)/,"the applied WebKit clamp must equal the selected fit line count");
assert.match(fit,/function complimentScheduleRenderedVerification\(el,key,fit,metrics,lite,onVerified\)/,"new fit cache entries must receive one bounded rendered verification with a fade-in callback");
assert.match(fit,/function complimentRenderedOverflow\(el,fit\)/,"the rendered verification must detect actual line/box overflow");
assert.match(fit,/el\.style\.webkitLineClamp="unset"/,"rendered verification must inspect unclamped WebKit wrapping before restoring the safe clamp");
assert.doesNotMatch(lite,/function complimentLiteTextBucket/,"Lite may not reuse broad text-shape cache buckets");
// W1: the applied size may never fall below the tier drop threshold for
// non-priority text; priority text is guarded by the 16px hard floor.
assert.match(fit,/const minSize=fit&&fit\.priority\?COMP_FIT\.hardFloor:Math\.max\(COMP_FIT\.hardFloor,complimentReadableThreshold\(metrics,"drop"\)\)/,"no code path may assign a non-priority font size below the tier drop threshold");
assert.match(fit,/readable:priority\?true:best>=drop/,"the full measured fit must report the readable contract");
const liteFitSource=lite.match(/function complimentLiteFit\(text,metrics,minimumLines,opts\)\{[\s\S]*?\n\}/)?.[0]||"";
assert.ok(liteFitSource,"Lite fit function must be present");
assert.doesNotMatch(liteFitSource,/getComputedStyle|clientWidth|clientHeight|getBoundingClientRect|scrollWidth|scrollHeight/,"ordinary Lite fitting must not read layout during rotation");
assert.match(liteFitSource,/readable:false/,"unreadable non-priority content must be reported, not silently shrunk");
assert.match(lite,/observer\.observe\(node\)/,"Lite observers must watch layout siblings rather than the rotating text element itself");
assert.doesNotMatch(lite,/observer\.observe\(el\)/,"Lite must not observe #comptext and turn every message swap into a geometry callback");
// W1 wiring: the drop loop lives next to the pick; dropped picks are
// remembered per geometry revision; the old text is kept when nothing
// remaining is readable.
assert.match(schedule,/function pickReadableCompliment\(\)/,"the readable drop loop must live next to the pick");
assert.doesNotMatch(schedule,/function pickCompliment\(\)/,"the old unconditional pick must be replaced by the readable drop loop");
assert.match(schedule,/for\(let attempt=0;attempt<4;attempt\+\+\)/,"the drop loop must be bounded and must never loop over an unchanged pool");
assert.match(schedule,/a\.text\.length-b\.text\.length/,"after bounded re-picks the shortest eligible non-priority message is the last chance");
assert.match(schedule,/return null;/,"when nothing remaining is readable the current message is kept and the rotation rescheduled");
assert.match(schedule,/bumpCompListRevision\(\);/,`a refreshed pool must bump the eligibility memo revision`);
assert.match(schedule,/compUnreadableClear\(\)/,"a refreshed pool must re-admit dropped messages");
assert.match(lite,/compUnreadableClear\(\)/,"a committed geometry change must re-admit dropped messages");
assert.match(core,/function adjustedMessageWeights\(pool\)/,"weights must live in a parallel array instead of per-pick copies");
assert.match(core,/weights\[i\]=weight;/,"the memoized eligible pool must not be copied into per-pick objects");
assert.doesNotMatch(core,/return \{\.\.\.c,weight\}/,"the former per-pick object copies must be gone");
assert.match(core,/_ELIGIBLE_COMPLIMENT_MEMO/,"eligibility must be memoized across picks within the same window");
assert.ok(core.includes('COMP_LIST_REVISION+"|"'),"the memo must invalidate with the pool content revision");

function makeContext(multiplier=1){
  const measure=(text,font)=>{
    const size=Number(String(font).match(/(\d+(?:\.\d+)?)px/)?.[1]||16);
    let units=0;
    for(const ch of Array.from(String(text||""))){
      units+=/[ilI1.,'`]/.test(ch)?.29:/[MW@#%]/.test(ch)?.85:/\s/.test(ch)?.28:.52;
    }
    return {width:units*size};
  };
  const context={
    CONFIG:{profile:"lite",fontPreset:"default"},SETTINGS:{},
    window:{innerWidth:1920,innerHeight:1080,devicePixelRatio:1,addEventListener(){}},
    document:{
      documentElement:{dataset:{fit:"base"},getAttribute(){return "default";}},
      fonts:{status:"loaded",ready:Promise.resolve()},
      createElement(tag){return tag==="canvas"?{getContext(){return {font:"",measureText(text){return measure(text,this.font);}};}}:{};},
      getElementById(){return null;}
    },
    messageTypographySizeMultiplier:()=>multiplier,
    Math,Number,String,Map,Array,Object,Set,console,setTimeout,clearTimeout
  };
  context.globalThis=context;
  return context;
}
function metrics(tier,width,height){
  return {outerWidth:width,outerHeight:height,contentWidth:width,contentHeight:height,elPadV:0,elPadH:0,fontFamily:"Arial",fontWeight:"800",fontStyle:"normal",letterSpacing:0,fontsReady:true,tier,revision:7,cacheKey:`${tier}:${width}x${height}@7`};
}
const context=makeContext(1);
vm.createContext(context);
vm.runInContext(`${fit}\n${lite}\n${layout}\nglobalThis.__fit={liteFit:complimentLiteFit,key:complimentFitKey,candidates:complimentLayoutCandidates,choose:complimentLayoutChoose,metricsForLines:complimentLiteMetricsForLines,reserve:complimentVerticalFitReserve,liteWidth:complimentLiteWidth};`,context);
const rawWidth=40*20*.52;
const conservativeWidth=context.__fit.liteWidth({measureText(){return {width:rawWidth};}},"a".repeat(40),20,{letterSpacing:0,fontsReady:true});
const fallbackWidth=context.__fit.liteWidth({measureText(){return {width:rawWidth};}},"a".repeat(40),20,{letterSpacing:0,fontsReady:false});
assert.ok(conservativeWidth>rawWidth*1.02,"settled-font Lite Canvas measurement must include a 2–4% wrap guard");
assert.ok(fallbackWidth>conservativeWidth,"pre-font emergency Lite measurement must be still more conservative");
// Readable contract per tier: ordinary prose keeps at least its tier drop
// threshold, and non-priority fits never exceed three lines.
const prose="The family can see today’s plans, meals, and reminders at a glance today.";
const shortNote="A calm family note that stays on one comfortable line.";
const cases=[
  ["min",420,60,18,prose.slice(0,60)],
  ["compact",620,88,22,prose.slice(0,90)],
  ["base",900,100,28,prose],
  ["base",1400,100,28,shortNote],
];
for(const [tier,width,height,drop,text] of cases){
  const result=context.__fit.liteFit(text,metrics(tier,width,height));
  assert.equal(result.readable,true,`${tier} ordinary prose must reach its tier drop threshold of ${drop}px`);
  assert.ok(result.size>=drop,`${tier} ordinary prose must render at or above its ${drop}px drop threshold`);
  assert.ok(result.lines>=1&&result.lines<=3,`${tier} non-priority prose must use at most three lines`);
  assert.equal(result.maxLines,3,`${tier} non-priority Lite result must clamp at three lines`);
}
// W1 acceptance: a 300-char non-priority message at the min tier is
// unreadable in this fixture and must come back DROPPED — it is never
// rendered below the 18px drop threshold.
const long300="word ".repeat(60).replace(/ $/,"")+"!";
assert.equal(long300.length,300,"drop fixture must be a 300-character message");
const dropped=context.__fit.liteFit(long300,metrics("min",420,60));
assert.equal(dropped.readable,false,"300-char non-priority text at min tier must be reported unreadable so the drop loop replaces it");
assert.equal(dropped.fits,false,"unreadable non-priority text must not silently shrink into a clamp");
assert.equal(dropped.size,0,"a dropped pick carries no renderable size");
const droppedBase=context.__fit.liteFit(prose.repeat(2),metrics("base",900,100));
assert.equal(droppedBase.readable,false,"long non-priority prose that cannot reach its drop threshold must be dropped, not shrunk");

// W1 acceptance: a 260-char temporary (priority) message is never dropped —
// it renders at at least 16px in at most four lines.
const temp260="temp ".repeat(52);
assert.equal(temp260.length,260,"priority fixture must be a 260-character message");
for(const [tier,width,height] of [["base",900,100],["min",420,60]]){
  const temp=context.__fit.liteFit(temp260,metrics(tier,width,height),1,{priority:true});
  assert.equal(temp.readable,true,`${tier} priority content must never be unreadable`);
  assert.ok(temp.size>=16,`${tier} 260-char temporary message must render at at least 16px (got ${temp.size}px)`);
  assert.ok(temp.lines>=1&&temp.lines<=4,`${tier} priority content may use at most four lines`);
  assert.equal(temp.maxLines,4,`${tier} priority content may use the bounded fourth line`);
}

const joke="Is the porch light left on tonight? Yes, it stays on until the rain stops.";
const jokeBefore=joke;
const plannerMetrics={...metrics("base",800,100),contentHeight:74,elPadV:26};
const jokeCandidates=context.__fit.candidates(joke);
const jokeBaseline=context.__fit.liteFit(jokeCandidates[0].displayText,plannerMetrics);
const jokeChoice=context.__fit.choose(joke,(displayText,candidate)=>context.__fit.liteFit(displayText,context.__fit.metricsForLines(plannerMetrics,candidate.lines)));
assert.equal(joke,jokeBefore,"raw message content must remain unchanged when a display plan is chosen");
assert.equal(jokeChoice.candidate.kind,"question-answer","a balanced question-and-answer joke should use its semantic split");
assert.equal(jokeChoice.candidate.displayText,"Is the porch light left on tonight?\nYes, it stays on until the rain stops.","question punctuation must become the display-only break");
assert.equal(jokeChoice.fit.lines,2,"the question-and-answer composition must reserve two deliberate lines");
assert.ok(jokeChoice.fit.size>=jokeBaseline.size,"the selected question-and-answer composition must not lose readable size to the conservative wrap budget");
const shortPunctuation="Today is bright, and you are ready.";
const shortChoice=context.__fit.choose(shortPunctuation,(displayText,candidate)=>context.__fit.liteFit(displayText,context.__fit.metricsForLines(plannerMetrics,candidate.lines)));
assert.equal(shortChoice.candidate.kind,"single","a short comma phrase must not be split merely because punctuation exists");
const sentenceCandidates=context.__fit.candidates("The door is open. The family is ready.");
assert.ok(sentenceCandidates.some(candidate=>candidate.kind==="sentence"),"sentence-ending punctuation should be considered as a semantic break");

const reserveOne=context.__fit.reserve(48,1,true);
const reserveTwo=context.__fit.reserve(48,2,true);
const reserveThree=context.__fit.reserve(48,3,true);
assert.ok(reserveOne>=5&&reserveOne<reserveTwo&&reserveTwo<reserveThree,"Lite vertical reserve must grow from one to three lines");
const clippedPhrase="Kindred — similar in\ncharacter or nature.";
const clippedMetrics=context.__fit.metricsForLines({...metrics("base",620,100),contentHeight:74,elPadV:26},2);
const clippedFit=context.__fit.liteFit(clippedPhrase,clippedMetrics);
const clippedReserve=context.__fit.reserve(clippedFit.size,clippedFit.lines,true);
assert.ok(clippedFit.size*1.08*clippedFit.lines+clippedReserve<=clippedMetrics.contentHeight,"two-line descender phrase must retain vertical headroom");

const baseMetrics=metrics("base",1400,100);
const typical=context.__fit.liteFit("A thoughtful family message that is long enough to wrap naturally but should remain easy to read.",baseMetrics);
assert.equal(typical.lines,2,"ordinary long prose should use a readable two-line fit when the real width allows it");
assert.ok(typical.size>=28,"ordinary prose must never fall below the base tier drop threshold");
assert.equal(typical.readable,true,"ordinary long prose must stay readable in this fixture");

const largeContext=makeContext(1.16);
vm.createContext(largeContext);
vm.runInContext(`${fit}\n${lite}\n${layout}\nglobalThis.__fit=complimentLiteFit;`,largeContext);
const headline="A bright day starts here.";
const defaultHeadline=context.__fit.liteFit(headline,baseMetrics);
const largeHeadline=largeContext.__fit(headline,baseMetrics);
assert.ok(largeHeadline.size>defaultHeadline.size,"Large / Extra large typography must affect Lite message sizing");
const keyA=context.__fit.key("A calm family note",40,24,true,baseMetrics);
const keyB=context.__fit.key("A bright family note",40,24,true,baseMetrics);
assert.notEqual(keyA,keyB,"same-length Lite messages must not share an imprecise shape-bucket cache key");

const correctionContext={
  CONFIG:{profile:"lite",fontPreset:"default"},SETTINGS:{},
  window:{devicePixelRatio:1},
  document:{
    documentElement:{getAttribute(){return "default";}},
    fonts:{status:"loaded"},
    createRange(){throw new Error("use synthetic metrics");},
  },
  getComputedStyle(el){
    const size=parseFloat(el.style.fontSize)||20,ratio=parseFloat(el.style.lineHeight)||1.08;
    return {fontSize:size+"px",lineHeight:(size*ratio)+"px"};
  },
  messageTypographySizeMultiplier(){return 1;},
  complimentLiteFit(_text,_metrics,minimumLines){
    return {size:20,lines:Math.max(2,minimumLines||1),maxLines:3,fits:true,preferredFloor:18,targetLines:Math.max(2,minimumLines||1)};
  },
  complimentLiteMetricsForLines(metrics){return metrics;},
  Math,Number,String,Map,Array,Object,Set,console,setTimeout,clearTimeout,
  requestAnimationFrame(fn){fn();}
};
correctionContext.globalThis=correctionContext;
vm.createContext(correctionContext);
vm.runInContext(`${fit}
globalThis.__guard={apply:complimentApplyFitAndVerify,cache:COMP_FIT_CACHE};`,correctionContext);
const guardElement={
  style:{},dataset:{},textContent:"Kindred — similar in character or related.",
  clientWidth:620,clientHeight:60,scrollWidth:620,
  get scrollHeight(){
    const size=parseFloat(this.style.fontSize)||20;
    return this.style.webkitLineClamp==="unset"?(size>=30?61:40):(size>=30?60:40);
  }
};
const initialFit={size:30,lines:1,maxLines:3,fits:true,displayText:guardElement.textContent,layout:"single"};
correctionContext.__guard.apply(guardElement,"synthetic-overflow",initialFit,{contentWidth:620,contentHeight:60},true);
assert.equal(guardElement.style.webkitLineClamp,"2","rendered correction must reapply the corrected fitted-line clamp");
assert.equal(guardElement.dataset.fitCorrected,"1","rendered correction must record exactly one bounded correction");
assert.equal(correctionContext.__guard.cache.get("synthetic-overflow").lines,2,"corrected Lite fit must replace the original cache entry");
// The corrected fake fit asks for 20px, but the applied size may never drop
// below the base tier drop threshold: the guard must hold the line at 28px.
assert.ok(parseFloat(guardElement.style.fontSize)>=28,"no applied font size may fall below the tier drop threshold for non-priority text");

console.log("PASS: responsive message band, bounded semantic layout planning, exact-cache Lite geometry, conservative Canvas fitting, fitted-line clamping, one-frame correction, typography scaling, readable-size drop contract, and WebKit-safe vertical headroom");
