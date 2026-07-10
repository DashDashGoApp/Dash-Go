#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import {fileURLToPath} from "node:url";

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const source=fs.readFileSync(path.join(root,"ui","js","weather-icons.js"),"utf8");
assert.match(source,/const WEATHER_ICON_SVG_SETS=Object\.freeze\(\{/,
  "weather icons must be the reviewed static inline SVG catalog");
assert.match(source,/bold:Object\.freeze\(\{[\s\S]*?sun:"<svg[^\n]+#ffd768[\s\S]*?edgeWidth|bold:Object\.freeze\(\{[\s\S]*?#ffd768/,
  "Bold must retain the reviewed brighter SVG treatment");
assert.match(source,/contrast:Object\.freeze\(\{[\s\S]*?#ffe66b[\s\S]*?#182535/,
  "High Contrast must retain near-white forms, vivid accents, and dark keylines");
assert.match(source,/const WX_METRIC_ICON_SVGS=Object\.freeze\(\{/,
  "weather metric icons must be a reviewed static inline SVG catalog");
const context=vm.createContext({WEATHER_ICON_STYLES:{soft:{},bold:{},outline:{},contrast:{},playful:{}},Math,Array,Object,String,Number});
vm.runInContext(`${source}\nglobalThis.__icons={soft:buildWeatherIconSet('soft'),bold:buildWeatherIconSet('bold'),contrast:buildWeatherIconSet('contrast')};\nglobalThis.__metrics=WX_METRIC_ICON_SVGS;`,context,{filename:"weather-icons.js"});
const keys=["sun","partly","cloud","overcast","fog","drizzle","rain","snow","storm"];
for(const style of ["soft","bold","contrast"]){
  assert.deepEqual(Object.keys(context.__icons[style]),keys,`${style} must expose the nine weather icon keys`);
}
for(const style of ["soft","bold","contrast"]){
  for(const key of keys){
    const svg=context.__icons[style][key];
    assert.match(svg,/^<svg /,`${style} ${key} must remain inline SVG`);
    assert.match(svg,/class="wxsvg wxsvg-[^"]+"/,`${style} ${key} must preserve weather SVG classes`);
    assert.doesNotMatch(svg,/<(?:filter|mask|image|animate)\b|https?:/i,`${style} ${key} must stay static and local`);
  }
}
for(const key of ["high","low","feels","precipChance","precipTotal","wind","uv","sunrise","sunset","other"]){
  assert.match(context.__metrics[key],/^<svg /,`${key} metric must remain inline SVG`);
  assert.match(context.__metrics[key],/class="wxstaticon"/,`${key} metric must preserve summary icon class`);
  assert.doesNotMatch(context.__metrics[key],/<(?:filter|mask|image|animate)\b|https?:/i,`${key} metric must stay static and local`);
}
assert.notEqual(context.__icons.soft.sun,context.__icons.bold.sun,"Bold sun must visibly differ from Soft");
assert.notEqual(context.__icons.soft.cloud,context.__icons.contrast.cloud,"High Contrast cloud must visibly differ from Soft");
assert.doesNotMatch(source,/-?\d+\.\d{6,}/,"generated weather SVG coordinates must not retain floating-point artifact tails");
console.log("PASS: reviewed static weather and metric SVG catalogs stay local, distinct, and cached");
