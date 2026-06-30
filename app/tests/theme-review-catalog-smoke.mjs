#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import {fileURLToPath} from "node:url";

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const catalog=fs.readFileSync(path.join(root,"themes.list"),"utf8")
  .split(/\r?\n/).map(line=>line.replace(/\s*#.*/,"").trim()).filter(Boolean);
assert.equal(catalog.length,100,"the beta.3 catalog must contain 100 selectable theme IDs");
assert.equal(new Set(catalog).size,100,"the theme catalog must remain unique");
assert.ok(!catalog.includes("backtoschool")&&!catalog.includes("gameday"),"Back to School and Game Day must not ship");

const context=vm.createContext({console});
for(const file of [
  "config-themes-core.js","config-themes-foundation-seasonal.js","config-themes-color-moods.js",
  "config-themes-occasions-accessibility.js","config-theme-meta.js"
]) vm.runInContext(fs.readFileSync(path.join(root,"ui","js",file),"utf8"),context,{filename:file});
const runtime=vm.runInContext("({THEMES,THEME_META,THEME_GROUP_ORDER,themeGroupColumns})",context);
assert.deepEqual([...Object.keys(runtime.THEME_META)].sort(),[...catalog].sort(),"metadata must cover the shared catalog exactly");
for(const name of catalog) assert.ok(runtime.THEMES[name]!==undefined,`missing palette ${name}`);
assert.ok(!runtime.THEME_GROUP_ORDER.includes("More"),"More must not remain as a picker category");
assert.equal(runtime.THEME_META.desert.group,"Nature & Elements");
assert.equal(runtime.THEME_META.rose.group,"Color");
assert.equal(runtime.THEME_META.noir.group,"Aesthetic");
assert.equal(runtime.THEME_META.hanukkah.group,"Holidays & Observances");
assert.equal(runtime.THEME_META.hanukkah.availability,"jewish-hanukkah");
assert.equal(runtime.THEME_META.kwanzaa.availability,"holiday-kwanzaa");
assert.equal(runtime.themeGroupColumns("Seasons",4),4,"Seasons stay a four-card row");
assert.equal(runtime.themeGroupColumns("Fun",16),4,"Fun stays readable in four columns");
assert.equal(runtime.themeGroupColumns("Aesthetic",12),6,"Aesthetic fits two six-card rows");
assert.equal(runtime.themeGroupColumns("Holidays & Observances",5),5,"five available observance themes fit one row");
assert.equal(runtime.themeGroupColumns("Holidays & Observances",7),4,"seven observance themes avoid a stranded final tile");

const expected={
  sunset:{"--sat":"#ffbf57"},desert:{"--accent":"#cf8550","--today":"#f0c24f"},jade:{"--today":"#8fe6bd"},firefly:{"--today":"#f4c95a"},olive:{"--accent":"#b5772f","--today":"#e8cf52"},cherry:{"--accent":"#e0566f","--today":"#ffd27a"},winter:{"--today":"#8fd3f5"},america:{"--today":"#8fb0ff"},thanksgiving:{"--accent":"#b5612f"},cincodemayo:{"--today":"#ffd23f"},daylight:{"--today":"#b5681a"}
};
for(const [theme,tokens] of Object.entries(expected)) for(const [token,value] of Object.entries(tokens)) assert.equal(runtime.THEMES[theme][token],value,`${theme} review correction ${token}`);

const picker=fs.readFileSync(path.join(root,"ui","js","control-theme.js"),"utf8");
const css=fs.readFileSync(path.join(root,"ui","css","dashboard","control-theme-actions-lite.css"),"utf8");
assert.match(picker,/themeGroupColumns\(group,names\.length\)/,"picker must use category-aware grid choices");
assert.match(css,/--theme-cols-wide/,"picker CSS must support the wide layout");
assert.match(css,/--theme-cols-compact/,"picker CSS must preserve four-column 1024px fitting");
console.log("PASS: 100 curated themes, no More catchall, calendar-gated observances, reviewed palette fixes, and responsive group grids");
