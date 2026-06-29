import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import {fileURLToPath} from "node:url";

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const version=fs.readFileSync(path.join(root,"VERSION"),"utf8").trim();
const release=JSON.parse(fs.readFileSync(path.join(root,"release","release.json"),"utf8"));
const index=fs.readFileSync(path.join(root,"index.html"),"utf8");
const defaults=fs.readFileSync(path.join(root,"ui","js","config-defaults.js"),"utf8");
const launcher=fs.readFileSync(path.join(root,"ui","js","app-launcher.js"),"utf8");
const household=fs.readFileSync(path.join(root,"ui","js","household-app-loader.js"),"utf8");
const control=fs.readFileSync(path.join(root,"ui","js","control-lazy-loader.js"),"utf8");

assert.equal(release.version,version,"release.json must match VERSION");
assert.match(index,new RegExp(`ui/dashboard\\.css\\?v=${version}`),"dashboard CSS cache buster must match VERSION");
assert.match(index,new RegExp(`app\\.bundle\\.js\\?v=${version}`),"dashboard bundle cache buster must match VERSION");
assert.match(defaults,new RegExp(`version: "${version}"`),"runtime defaults must match VERSION");
for(const [name,source] of [["launcher",launcher],["household apps",household],["control loader",control]]){
  const fallbacks=[...source.matchAll(/CONFIG\.version\|\|"([^"]+)"/g)].map(match=>match[1]);
  for(const fallback of fallbacks){
    assert.equal(fallback,version,`${name} lazy asset fallback must match VERSION`);
  }
}
console.log("PASS: source version/cache-buster contract");
