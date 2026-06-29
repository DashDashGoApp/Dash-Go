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
assert.match(fit,/preferredFloor=Math\.max\(COMP_FIT\.hardFloor,Math\.min\(Math\.max\(COMP_FIT\.hardFloor,start-1\),floor\)\)/,"scaled floor must be honored before the absolute fallback");
assert.doesNotMatch(fit,/Math\.min\(floor,COMP_FIT\.hardFloor\)/,"the former dead scaled-floor override must not return");
assert.match(fit,/const lite=complimentLiteProfile\(\),metrics=lite\?complimentLiteMetricsForFit\(el\):complimentBoxMetrics\(el\)/,"Lite must use its cached geometry path rather than read the DOM on each rotation");
assert.match(lite,/function complimentLiteReadGeometry\(el\)/,"Lite must capture actual rendered geometry after layout changes");
assert.match(lite,/fontFamily:elStyle\.fontFamily[\s\S]*?fontWeight:elStyle\.fontWeight/,"Lite snapshot must retain effective font family and weight for Canvas measurement");
assert.match(lite,/function complimentLiteLineCount\(text,size,metrics,limit\)/,"Lite must use word-aware Canvas line measurement");
assert.match(lite,/function complimentLiteMetricsForLines\(metrics,lines\)/,"Lite must derive selected multi-line capacity from cached geometry");
assert.match(lite,/--comptext-base-pad-y/,"Lite geometry snapshots must remain anchored to the normal message inset");
assert.match(layout,/function complimentLayoutCandidates\(text\)/,"message layout must enumerate bounded display candidates");
assert.match(layout,/question-answer/,"question-and-answer punctuation must receive a semantic candidate");
assert.match(layout,/function complimentLayoutChoose\(text,assess\)/,"message layout must compare fitted candidates rather than force a line count");
assert.match(fit,/__dashComplimentRawText/,"display-only line planning must preserve raw rotation text outside the rendered composition");
assert.match(lite,/function complimentLiteReadingFloors\(metrics\)/,"Lite must define tier-and-line reading floors");
assert.match(fit,/function complimentVerticalFitReserve\(size,lines,lite\)/,"message fitting must reserve a small vertical WebKit safety margin");
assert.match(lite,/metrics\.contentHeight-complimentVerticalFitReserve/,"Lite Canvas fitting must apply the vertical safety reserve");
assert.match(lite,/maxLines:3/,"Lite exceptional content must clamp at three lines");
assert.doesNotMatch(lite,/function complimentLiteTextBucket/,"Lite may not reuse broad text-shape cache buckets");
const liteFitSource=lite.match(/function complimentLiteFit\(text,metrics\)\{[\s\S]*?\n\}/)?.[0]||"";
assert.ok(liteFitSource,"Lite fit function must be present");
assert.doesNotMatch(liteFitSource,/getComputedStyle|clientWidth|clientHeight|getBoundingClientRect|scrollWidth|scrollHeight/,"ordinary Lite fitting must not read layout during rotation");
assert.match(lite,/for\(const node of \[parent,sun,stale\]\)/,"Lite observers must watch layout siblings rather than the rotating text element itself");
assert.doesNotMatch(lite,/observer\.observe\(el\)/,"Lite must not observe #comptext and turn every message swap into a geometry callback");

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
  return {outerWidth:width,outerHeight:height,contentWidth:width,contentHeight:height,elPadV:0,elPadH:0,fontFamily:"Arial",fontWeight:"800",fontStyle:"normal",letterSpacing:0,tier,revision:7,cacheKey:`${tier}:${width}x${height}@7`};
}
const context=makeContext(1);
vm.createContext(context);
vm.runInContext(`${fit}\n${lite}\n${layout}\nglobalThis.__fit={liteFit:complimentLiteFit,key:complimentFitKey,candidates:complimentLayoutCandidates,choose:complimentLayoutChoose,metricsForLines:complimentLiteMetricsForLines,reserve:complimentVerticalFitReserve};`,context);
const prose="The family can see today’s plans, meals, chores, and reminders at a glance without needing to open another screen.";
const cases=[
  ["min",420,60,16],
  ["compact",620,88,20],
  ["base",900,100,24],
  ["base",1400,100,24],
];
for(const [tier,width,height,floor] of cases){
  const result=context.__fit.liteFit(prose,metrics(tier,width,height));
  assert.ok(result.size>=floor,`${tier} ordinary prose must retain its tier reading floor`);
  assert.ok(result.lines>=1&&result.lines<=3,`${tier} ordinary prose must use at most three lines`);
  assert.equal(result.maxLines,3,`${tier} Lite result must clamp exceptional content at three lines`);
}

const joke="Where do fish keep their money? In the riverbank";
const jokeBefore=joke;
const plannerMetrics={...metrics("base",800,100),contentHeight:74,elPadV:26};
const jokeCandidates=context.__fit.candidates(joke);
const jokeBaseline=context.__fit.liteFit(jokeCandidates[0].displayText,plannerMetrics);
const jokeChoice=context.__fit.choose(joke,(displayText,candidate)=>context.__fit.liteFit(displayText,context.__fit.metricsForLines(plannerMetrics,candidate.lines)));
assert.equal(joke,jokeBefore,"raw message content must remain unchanged when a display plan is chosen");
assert.equal(jokeChoice.candidate.kind,"question-answer","a balanced question-and-answer joke should use its semantic split");
assert.equal(jokeChoice.candidate.displayText,"Where do fish keep their money?\nIn the riverbank","question punctuation must become the display-only break");
assert.equal(jokeChoice.fit.lines,2,"the question-and-answer composition must reserve two deliberate lines");
assert.ok(jokeChoice.fit.size>jokeBaseline.size,"the selected question-and-answer composition must be larger than the cramped natural composition");
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
assert.equal(typical.lines,2,"ordinary 87+ character prose should use a readable two-line fit when the real width allows it");
assert.ok(typical.size>=34,"ordinary two-line base prose must not fall back to tiny text");
const exceptional=context.__fit.liteFit("unbroken".repeat(260),metrics("base",620,100));
assert.equal(exceptional.maxLines,3,"exceptional text must receive the three-line clamp");
assert.equal(exceptional.fits,false,"exceptional text should clamp rather than silently shrinking below the reading floor");

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
console.log("PASS: responsive message band, bounded semantic layout planning, exact-cache Lite geometry, Canvas fitting, typography scaling, tier reading floors, and WebKit-safe vertical headroom");
