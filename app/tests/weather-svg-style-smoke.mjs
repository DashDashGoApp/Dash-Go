#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import {fileURLToPath} from "node:url";

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const source=fs.readFileSync(path.join(root,"ui","js","weather-icons.js"),"utf8");
assert.match(source,/bold:\{sun:'#ffd768',[\s\S]*?ray:3\.0,drop:2\.7,disk:5\.8,edge:'rgba\(26,38,54,\.46\)',edgeWidth:\.78,flake:1\.45/,"Bold must use the reviewed heavier, brighter SVG treatment");
assert.match(source,/contrast:\{sun:'#ffe66b',cloud:'#f8fbff',cloudDk:'#dcecff',rain:'#24bfff',snow:'#ffffff',bolt:'#fff14a',[\s\S]*?edge:'#182535',edgeWidth:\.78/,"High Contrast must use near-white forms, vivid accents, and a thin dark keyline");
assert.match(source,/const iconEdge=p\.edge\|\|'rgba\(28,36,44,\.26\)'/,"weather icon painter must allow style-specific keylines");
assert.match(source,/r="\$\{p\.outline\?4\.8:\(p\.disk\|\|5\)\}"/,"Bold must be allowed the reviewed larger sun disk");
const context=vm.createContext({WEATHER_ICON_STYLES:{soft:{},bold:{},outline:{},contrast:{},playful:{}},Math,Array,Object,String,Number});
vm.runInContext(`${source}\nglobalThis.__icons={soft:buildWeatherIconSet('soft'),bold:buildWeatherIconSet('bold'),contrast:buildWeatherIconSet('contrast')};`,context,{filename:"weather-icons.js"});
for(const key of ["sun","partly","cloud","overcast","fog","drizzle","rain","snow","storm"]){
  assert.match(context.__icons.bold[key],/^<svg /,`Bold ${key} must remain inline SVG`);
  assert.match(context.__icons.contrast[key],/^<svg /,`High Contrast ${key} must remain inline SVG`);
  assert.doesNotMatch(context.__icons.bold[key],/<(?:filter|mask|image|animate)\b|https?:/i,`Bold ${key} must stay static and local`);
}
assert.notEqual(context.__icons.soft.sun,context.__icons.bold.sun,"Bold sun must visibly differ from Soft");
assert.notEqual(context.__icons.soft.cloud,context.__icons.contrast.cloud,"High Contrast cloud must visibly differ from Soft");
console.log("PASS: reviewed Bold and High Contrast weather SVG distinctions stay local, static, and cached");
