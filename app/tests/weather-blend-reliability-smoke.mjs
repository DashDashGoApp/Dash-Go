#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import {fileURLToPath} from "node:url";
const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const source=fs.readFileSync(path.join(root,"ui/js/weather-blend.js"),"utf8");
const context=vm.createContext({
  console,
  CONFIG:{tempUnit:"fahrenheit",windUnit:"mph",weatherForecastMaxDays:10},
  WEATHER_SOURCE_META:{},
  WEATHER_DISABLED_SOURCE_IDS:new Set(),
  WEATHER_LAST_SOURCE_STATUS:[],
  weatherProviderList:()=>["a","b","c"],
  weatherSourceDisabled:()=>false
});
vm.runInContext(source,context,{filename:"weather-blend.js"});
const run=expr=>vm.runInContext(expr,context);
assert.deepEqual(Array.from(run("finiteNums([null,undefined,'',false,0,'2'])")),[0,2],"missing and boolean values must not become numeric votes");
assert.equal(run("blendPrecipitationTotal([0.25,0.3,25.7]).value"),0.3,"three-source precipitation must use the consensus median");
assert.equal(run("blendPrecipProbability([10,10,90]).value"),10,"three-source precipitation probability must use the median");
assert.equal(run("blendWeatherCode([null,'',undefined],50,10)"),null,"missing conditions must remain unknown rather than clear");
assert.equal(run("blendWeatherCode([0,61],70,8)"),61,"a clear/rain tie with supporting precipitation must select rain");
assert.equal(run("blendWeatherCode([0,61],5,0)"),0,"a clear/rain tie without precipitation support may select clear");
const staleBlend=run(`blendWeatherSources([
  {_source:'fresh',current:{temperature_2m:70,weather_code:0},daily:{time:['2026-07-11'],weather_code:[0],temperature_2m_max:[80],temperature_2m_min:[60],apparent_temperature_max:[80],precipitation_sum:[1],precipitation_probability_max:[10],wind_speed_10m_max:[5],uv_index_max:[4],sunrise:['2026-07-11T11:00:00Z'],sunset:['2026-07-12T01:00:00Z']}},
  {_source:'stale',_stale:true,current:{temperature_2m:120,weather_code:95},daily:{time:['2026-07-11'],weather_code:[95],temperature_2m_max:[120],temperature_2m_min:[90],apparent_temperature_max:[120],precipitation_sum:[100],precipitation_probability_max:[100],wind_speed_10m_max:[100],uv_index_max:[15],sunrise:['2026-07-11T08:00:00Z'],sunset:['2026-07-12T04:00:00Z']}}
])`);
assert.equal(staleBlend.current.temperature_2m,70,"fresh current conditions must exclude stale votes");
assert.equal(staleBlend.daily.precipitation_sum[0],1,"fresh daily data must exclude stale totals");
assert.equal(staleBlend._blend.daily['2026-07-11'].staleExcluded,1,"blend metadata must report excluded stale providers");
run("WEATHER_DISABLED_SOURCE_IDS=new Set(['a','b'])");
run("weatherProviderList=()=>['a','b']");
const excluded=run(`blendWeatherSources([
  {_source:'a',current:{temperature_2m:70},daily:{time:[]}},
  {_source:'b',current:{temperature_2m:90},daily:{time:[]}}
])`);
assert.equal(excluded.current.temperature_2m,70,"all-excluded recovery must use one deterministic source, not silently blend all excluded providers");
assert.equal(excluded._blend.allExcludedFallback,true);
console.log("PASS: weather blending rejects missing votes, resists small-source outliers, prefers fresh data, and handles exclusions explicitly");
