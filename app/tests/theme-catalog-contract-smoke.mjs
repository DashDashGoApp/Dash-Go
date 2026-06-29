#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import {fileURLToPath} from "node:url";
const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const catalog=fs.readFileSync(path.join(root,"themes.list"),"utf8").split(/\r?\n/).map(line=>line.replace(/\s*#.*/,"").trim()).filter(Boolean);
assert.ok(catalog.length>0,"shared catalog must not be empty");
assert.equal(catalog[0],"basic","basic remains first picker theme");
assert.equal(new Set(catalog).size,catalog.length,"shared catalog may not repeat names");
for(const name of catalog)assert.match(name,/^[a-z][a-z0-9]*$/,"catalog names must be shell-safe identifiers");
const meta=fs.readFileSync(path.join(root,"ui","js","config-theme-meta.js"),"utf8");
const browser=[...meta.matchAll(/^\s{2}([a-z][a-z0-9]*):\{label:/gm)].map(match=>match[1]);
assert.deepEqual([...catalog].sort(),[...browser].sort(),"shared shell catalog must match browser theme names exactly");
for(const rel of [["installer",path.join(root,"..","installer","install.sh")],["set-theme",path.join(root,"bin","set-theme.sh")]]){
 const [label,file]=rel, source=fs.readFileSync(file,"utf8");
 assert.match(source,/themes\.list/,`${label} must read themes.list`);
 assert.doesNotMatch(source,/THEMES_LIST=/,`${label} must not retain a private copied list`);
}
console.log("PASS: shared theme catalog matches browser metadata and drives shell pickers");
