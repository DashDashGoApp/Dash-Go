#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import {fileURLToPath} from "node:url";

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const weather=fs.readFileSync(path.join(root,"ui","js","weather.js"),"utf8")+fs.readFileSync(path.join(root,"ui","js","weather-now.js"),"utf8");
const css=fs.readFileSync(path.join(root,"ui","css","dashboard","sidebar-weather-messages.css"),"utf8");

assert.match(weather,/metrics=el\("span","sub wx-current-metrics"\)/,
  "current weather details must use a dedicated wrapping metric row");
assert.match(weather,/el\("span","wx-metric-token wx-feels-token",Number\.isFinite\(c\.apparent_temperature\)\?"Feels "\+Math\.round\(c\.apparent_temperature\)\+"°":"Feels —"\)/,
  "feels-like value must remain an atomic metric token created as text");
assert.ok(!/wx-feels-token","Feels "\+Math\.round/.test(weather),
  "the feels-like token must be guarded so a provider that omits apparent temperature (NWS) cannot render a false 0°");
assert.match(weather,/wind=el\("span","wx-metric-token wx-wind-token"\)/,
  "wind value and configured unit must remain one atomic token");
assert.match(weather,/document\.createTextNode\(Math\.round\(c\.wind_speed_10m\)\+" "\+CONFIG\.windUnit\)/,
  "wind value and configured unit must be emitted through a text node");
assert.match(weather,/sep\.setAttribute\("aria-hidden","true"\)/,
  "wind separator must retain its accessibility contract");
assert.match(css,/#wxnow \.meta \.sub\.wx-current-metrics\{display:flex;flex-wrap:wrap;align-items:baseline;/,
  "current weather metrics must be allowed to wrap between complete tokens");
assert.match(css,/#wxnow \.wx-metric-token\{display:inline-flex;flex:0 0 auto;align-items:baseline;white-space:nowrap;\}/,
  "weather metric tokens must be unbreakable internally");
assert.match(css,/#wxnow \.wx-wind-token\{gap:\.32em;\}/,
  "wind separator spacing must be owned by the wind token rather than a breakable text space");
assert.ok(!/Feels \$\{Math\.round\(c\.apparent_temperature\)\}° · \$\{Math\.round\(c\.wind_speed_10m\)\} \$\{CONFIG\.windUnit\}/.test(weather),
  "the legacy raw inline wind string must not return");
console.log("PASS: sidebar current-weather metrics wrap only between complete value/unit tokens");
