#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import {fileURLToPath} from "node:url";

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const jsRoot=path.join(root,"ui","js");
const artFiles=["calendar-decor-art-seasons.js","calendar-decor-art-seasonal.js","calendar-decor-art-observances.js"];
const runtimeFile="calendar-seasonal-decor.js";
const manifest=JSON.parse(fs.readFileSync(path.join(jsRoot,"bundle.manifest.json"),"utf8"));
const ordered=manifest.bundles.app;
for(const name of [...artFiles,runtimeFile])assert.ok(ordered.includes(name),`bundle manifest is missing ${name}`);
assert.ok(Math.max(...artFiles.map(name=>ordered.indexOf(name)))<ordered.indexOf(runtimeFile),"calendar décor art must load before its placement runtime");

const context=vm.createContext({
  CONFIG:{seasonalDecor:"standard"},
  CURRENT_THEME:"christmas",
  Date,Math,Array,Object,String,Set,Number,
  document:{documentElement:{getAttribute:()=>""},querySelector:()=>null},
  dashboardRuntimeSettings:()=>({seasonalDecor:"standard"})
});
for(const name of [...artFiles,runtimeFile])vm.runInContext(fs.readFileSync(path.join(jsRoot,name),"utf8"),context,{filename:name});
vm.runInContext("globalThis.__decor={art:SEASONAL_DECOR_SVGS,map:THEME_DECOR_MAP,counts:SEASONAL_DECOR_COUNTS,signature:seasonalDecorSignature,rotation:seasonalDecorRotationOffset};",context);
const {art,map,counts}=context.__decor;
const expected=[
  "spring","summer","autumn","winter","halloween","christmas","thanksgiving","valentine","stpatricks","easter","newyear","america","mardigras","lunar","earthday","pride","oktoberfest","cincodemayo","juneteenth","muertos",
  "memorialday","laborday","veterans","mothersday","fathersday","hanukkah","kwanzaa"
].sort();
assert.deepEqual([...Object.keys(art)].sort(),expected,"all 27 seasonal and observance décor themes must be covered");
assert.deepEqual([...Object.keys(map)].sort(),expected,"every supported calendar décor theme must resolve through the theme map");
assert.deepEqual({...counts},{subtle:5,standard:10},"every profile must use five subtle or ten standard static decals");
for(const key of expected){
  const decals=art[key];
  assert.equal(decals.length,5,`${key} must keep the reviewed five-icon set`);
  assert.equal(new Set(decals).size,5,`${key} needs five distinct SVG strings`);
  for(const svg of decals){
    assert.match(svg,/^<svg viewBox="0 0 48 48">/,`${key} must use the shared static 48px SVG viewport`);
    assert.doesNotMatch(svg,/<(?:animate|filter|mask|image|foreignObject)\b|url\(|https?:|href=/i,`${key} decals must stay local, static, and filter-free`);
  }
}
assert.equal(context.__decor.signature(),"standard:christmas","Lite must not silently downgrade the selected standard décor density");
const month=new Date(2026,9,1);
const start=context.__decor.rotation("christmas",5,month);
assert.equal(start,context.__decor.rotation("christmas",5,month),"same theme/month decal rotation must be deterministic");
assert.deepEqual([...Array(10)].map((_,i)=>art.christmas[(start+i)%5]).filter((value,index,all)=>all.indexOf(value)===index).length,5,"standard density must cycle through all five reviewed icons");

const placement=fs.readFileSync(path.join(jsRoot,runtimeFile),"utf8");
const css=fs.readFileSync(path.join(root,"ui","css","dashboard","control-visual-style.css"),"utf8");
assert.doesNotMatch(placement,/seasonalDecorIsLite|effectiveMode/,"placement must not turn Lite Standard into Subtle");
assert.match(placement,/const count=Math\.min\(SEASONAL_DECOR_COUNTS\[mode\]\|\|0,cells\.length\);/,"placement must use the shared selected density on every profile");
assert.match(placement,/function seasonalDecorRotationOffset\(kind,length,now\)/,"placement must rotate the five-icon set deterministically by month");
assert.doesNotMatch(css,/filter:drop-shadow/,"calendar decals must not use a paint-expensive drop shadow");
assert.match(css,/html\[data-profile="lite"\] \.seasonal-decor,[\s\S]*?width:34px;height:34px;opacity:\.30;/,"Lite keeps static compact decals rather than sparse counts");
console.log("PASS: reviewed five-icon seasonal and observance décor catalog, all-profile density, deterministic rotation, content-safe static SVG policy, and Lite-safe paint treatment");
