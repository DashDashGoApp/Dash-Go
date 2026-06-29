#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import {fileURLToPath} from "node:url";

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const weather=fs.readFileSync(path.join(root,"ui/js/weather.js"),"utf8");
const close=(actual,expected,message)=>assert.ok(Math.abs(actual-expected)<1e-9,`${message}: expected ${expected}, got ${actual}`);
const normalized=phase=>((phase%1)+1)%1;
const idFor=phase=>`mp${Math.round(normalized(phase)*10000)}`;
const clipFor=(svg,id)=>svg.match(new RegExp(`<clipPath id="lit${id}">([\\s\\S]*?)<\\/clipPath>`))?.[1]||"";

assert.match(weather,/function moonSVG\(\)\{\s*return moonPhaseSVG\(moonPhaseAt\(Date\.now\(\)\)\);/,
  "moon entry point must calculate the phase from the live current instant");
assert.match(weather,/function moonPhaseAt\(nowMs\)/,"moon math must remain an explicit local helper");
assert.match(weather,/function moonPhaseSVG\(phase\)/,"moon phases must render through one reusable SVG builder");
assert.match(weather,/const size=32, r=size\*0\.44, cx=size\/2, cy=size\/2/,
  "moon SVG must retain its compact 32px viewport geometry");
assert.match(weather,/feTurbulence/,"moon SVG must retain its inline surface texture");
assert.match(weather,/const detailRelief=/,"moon SVG must vary crater relief through the lunar cycle");
assert.match(weather,/function terminatorPath\(\)/,"moon SVG must draw a phase-correct terminator");
assert.match(weather,/<mask id="night\$\{uid\}"/,
  "moon SVG must derive the dark side from the exact inverse of the illuminated mask");
assert.doesNotMatch(weather,/function shadClip\(/,"moon SVG must not maintain an independently overpainted shadow clip");
assert.doesNotMatch(weather,/id="shd\$\{uid\}"/,"moon SVG must not emit a shadow clip that can cover lit terrain");
assert.ok(!weather.includes("<img "),"moon rendering must remain a self-contained inline SVG, not an external image request");

const context=vm.createContext({Math,Date,Number});
vm.runInContext(`${weather}\nglobalThis.__moonPhaseSVG=moonPhaseSVG;globalThis.__moonPhaseAt=moonPhaseAt;`,context,{filename:"weather.js"});
const synodicSeconds=2551443,referenceSeconds=592500;
close(context.__moonPhaseAt(referenceSeconds*1000),0,"reference instant must be New Moon");
close(context.__moonPhaseAt((referenceSeconds+synodicSeconds*0.25)*1000),0.25,"quarter-cycle instant must be First Quarter");
close(context.__moonPhaseAt((referenceSeconds+synodicSeconds*0.5)*1000),0.5,"half-cycle instant must be Full Moon");

for(const phase of [0,0.03,0.125,0.25,0.3992587724,0.4669851531,0.5,0.625,0.75,0.875,0.97,0.999,1]){
  const p=normalized(phase),svg=context.__moonPhaseSVG(phase),uid=idFor(phase);
  const illumination=(1-Math.cos(2*Math.PI*p))/2;
  assert.match(svg,/^<svg class="moonphase-svg" viewBox="0 0 32 32" width="1em" height="1em"/,
    `phase ${phase} must provide a compact inline moon SVG`);
  for(const id of [`disc${uid}`,`lit${uid}`,`night${uid}`,`sky${uid}`,`surf${uid}`,`earth${uid}`,`ray${uid}`,`noise${uid}`,`spec${uid}`,`tglow${uid}`]){
    assert.ok(svg.includes(`id="${id}"`),`phase ${phase} is missing its locally-scoped ${id} definition`);
  }
  assert.ok(svg.includes(`clip-path="url(#lit${uid})"`),`phase ${phase} must clip illuminated terrain to its local phase`);
  assert.ok(svg.includes(`mask="url(#night${uid})"`),`phase ${phase} must derive earthshine from the inverse illumination mask`);
  assert.ok(!svg.includes(`shd${uid}`),`phase ${phase} must not reintroduce an overlapping shadow clip`);
  assert.ok(!/NaN|undefined|null/.test(svg),`phase ${phase} SVG must not contain invalid geometry`);
  assert.ok(illumination>=0&&illumination<=1,`phase ${phase} illumination must remain normalized`);
  const lit=clipFor(svg,uid);
  if(p===0){
    assert.match(lit,/r="0\.01"/,
      `phase ${phase} must remain New Moon rather than flip to a false Full Moon`);
    assert.ok(!svg.includes('stroke="#c9d7ee"'),`phase ${phase} should not draw a terminator at exact New Moon`);
  }else if(Math.abs(p-0.5)<1e-9){
    assert.match(lit,/r="14\.08"/,
      "Full Moon must retain the complete realistic terrain disc");
    assert.ok(!svg.includes('stroke="#c9d7ee"'),"Full Moon should not draw a false terminator");
  }else{
    assert.match(lit,/<path d="M16,1\.92 A14\.08,14\.08 0 0,[01] 16,30\.08 A/,
      `intermediate phase ${phase} must keep a curved continuous phase boundary`);
    assert.ok(svg.includes('stroke="#c9d7ee"'),`intermediate phase ${phase} needs a subtle terminator treatment`);
  }
  if(p>0&&p<0.5)assert.match(lit,/A14\.08,14\.08 0 0,1 16,30\.08/,
    `waxing phase ${phase} must illuminate the right-hand side`);
  if(p>0.5&&p<1)assert.match(lit,/A14\.08,14\.08 0 0,0 16,30\.08/,
    `waning phase ${phase} must illuminate the left-hand side`);
}

for(const [phase,minimum,maximum] of [[0.3992587724,0.88,0.92],[0.4669851531,0.97,0.99],[0.625,0.84,0.86],[0.875,0.14,0.16]]){
  const p=normalized(phase),illumination=(1-Math.cos(2*Math.PI*p))/2;
  assert.ok(illumination>minimum&&illumination<maximum,
    `phase ${phase} must remain in its expected continuous illumination band`);
  const svg=context.__moonPhaseSVG(phase),lit=clipFor(svg,idFor(phase));
  assert.ok(!/r="14\.08"/.test(lit),`phase ${phase} must not collapse into a false Full Moon`);
}

console.log("PASS: live-time realistic moon SVG keeps terrain, phase-aware relief, exact inverse earthshine, continuous waxing/full/waning geometry, and no dark-side overpaint regression");
