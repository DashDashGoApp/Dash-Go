// 06-weather-sources.js — provider adapters and multi-source blending.
let WEATHER_LAST_SOURCE_STATUS=[];
const WEATHER_KEY_REQUIRED=new Set(["weatherapi","openweather","googleweather","tomorrow","visualcrossing","weatherbit","pirateweather","accuweather","xweather"]);
const WEATHER_SOURCE_META={
  openmeteo:{label:"Open-Meteo",tier:"free · no key · non-commercial · hourly + 16-day",maxDays:16,refreshMin:15},
  nws:{label:"NWS / NOAA",tier:"free · no key · US-only · NOAA/NWS · hourly + 7-day",maxDays:7,refreshMin:15},
  weatherapi:{label:"WeatherAPI.com",tier:"free key · 100K/month · 3-day free forecast · hourly on free",maxDays:3,refreshMin:30},
  openweather:{label:"OpenWeather",tier:"free allowance · 1,000/day then billable · 8-day + 48h hourly",maxDays:8,refreshMin:30},
  googleweather:{label:"Google Weather",tier:"PAID · Google Maps Platform billing · no normal free tier · 10-day + 24h hourly per call",maxDays:10,refreshMin:30},
  tomorrow:{label:"Tomorrow.io",tier:"free key · 500/day, 25/hour · core forecast ~5 days · hourly",maxDays:5,refreshMin:30},
  visualcrossing:{label:"Visual Crossing",tier:"free key · 1,000 records/day · attribution · 15-day · hourly costs one forecast record",maxDays:15,refreshMin:30},
  weatherbit:{label:"Weatherbit",tier:"free key · 50/day · NON-COMMERCIAL · 7-day · hourly needs a paid plan",maxDays:7,refreshMin:90},
  pirateweather:{label:"Pirate Weather",tier:"free key · 10,000/month · 8-day · hourly",maxDays:8,refreshMin:30},
  accuweather:{label:"AccuWeather",tier:"14-DAY TRIAL then paid · 500/day during trial · 5-day + 12h hourly",maxDays:5,refreshMin:30},
  xweather:{label:"Xweather",tier:"free/trial/metered · conservative 9K/month cap · US/CA · 15-day · hourly costs one extra request",maxDays:15,refreshMin:30},
  "openmeteo-custom":{label:"Custom Open-Meteo",tier:"custom Open-Meteo compatible endpoint/key",maxDays:16,refreshMin:30},
};
function weatherProviderRefreshMinimum(id){
  return Math.max(15,Number((WEATHER_SOURCE_META[String(id||"").trim().toLowerCase()]||{}).refreshMin)||30);
}
function weatherConfiguredRefreshMinimum(){
  let minimum=15;
  for(const id of weatherProviderList())minimum=Math.max(minimum,weatherProviderRefreshMinimum(id));
  return minimum;
}
function weatherRefreshProfileDefaultMinutes(){
  return String(CONFIG.profile||"balanced").toLowerCase()==="lite"?45:30;
}
function effectiveWeatherRefreshMinutes(){
  // The aggregate weather check follows the profile cadence. Server-side
  // provider caches independently honor each source's quota-safe minimum.
  return Math.max(15,weatherRefreshProfileDefaultMinutes());
}
function weatherProviderDays(id,defaultMax){
  const meta=WEATHER_SOURCE_META[id]||{};
  const max=Number(meta.maxDays||defaultMax||CONFIG.weatherForecastMaxDays||16);
  return Math.max(1,Math.min(max,Math.max(1,Number(CONFIG.weatherForecastMaxDays)||16)));
}
const WEATHER_DISABLED_STORE="dashGo.weatherDisabledSources";
let WEATHER_DISABLED_SOURCE_IDS=new Set();
function loadWeatherDisabledSources(){
  try{
    const raw=localStorage.getItem(WEATHER_DISABLED_STORE);
    const arr=raw?JSON.parse(raw):[];
    WEATHER_DISABLED_SOURCE_IDS=new Set(Array.isArray(arr)?arr.map(x=>String(x||"").trim().toLowerCase()).filter(Boolean):[]);
  }catch(e){ WEATHER_DISABLED_SOURCE_IDS=new Set(); }
}
function saveWeatherDisabledSources(){
  try{ localStorage.setItem(WEATHER_DISABLED_STORE,JSON.stringify([...WEATHER_DISABLED_SOURCE_IDS].sort())); }catch(e){}
}
loadWeatherDisabledSources();
function weatherSourceDisabled(id){ return WEATHER_DISABLED_SOURCE_IDS.has(String(id||"").trim().toLowerCase()); }
function weatherSourceIdsFromSources(sources){ return (sources||[]).map(s=>String(s&&s._source||"").trim().toLowerCase()).filter(Boolean); }
function canDisableWeatherSource(id,sources){
  id=String(id||"").trim().toLowerCase();
  const ids=weatherSourceIdsFromSources(sources);
  const enabled=ids.filter(x=>x!==id && !weatherSourceDisabled(x));
  return enabled.length>0;
}
function setWeatherSourceDisabled(id,disabled,sources){
  id=String(id||"").trim().toLowerCase();
  if(!id) return false;
  if(disabled){
    if(!canDisableWeatherSource(id,sources || (WX&&WX._sources))) return false;
    WEATHER_DISABLED_SOURCE_IDS.add(id);
  }else{ WEATHER_DISABLED_SOURCE_IDS.delete(id); }
  saveWeatherDisabledSources();
  return true;
}
function toggleWeatherSourceDisabled(id,sources){
  id=String(id||"").trim().toLowerCase();
  return setWeatherSourceDisabled(id,!weatherSourceDisabled(id),sources);
}
function weatherProviderList(){
  let list=Array.isArray(CONFIG.weatherProviders)?CONFIG.weatherProviders:[];
  if(!list.length) list=[CONFIG.weatherProvider||"openmeteo"];
  const seen=new Set();
  const replacements={metno:"weatherbit",meteosource:"weatherbit"};
  const supported=new Set(["openmeteo","nws","weatherapi","openweather","googleweather","tomorrow","visualcrossing","pirateweather","accuweather","weatherbit","xweather","openmeteo-custom"]);
  return list.map(x=>replacements[String(x||"").trim().toLowerCase()]??String(x||"").trim().toLowerCase()).filter(x=>x && supported.has(x) && !seen.has(x)&&seen.add(x));
}
let WEATHER_LAST_AGGREGATE=null;
async function fetchServerWeatherSources(){
  const res=await fetch("/api/weather",{cache:"no-store"});
  if(!res.ok) throw new Error("HTTP "+res.status);
  const payload=await res.json();
  WEATHER_LAST_AGGREGATE=payload||null;
  if(payload && Array.isArray(payload.status)) WEATHER_LAST_SOURCE_STATUS=payload.status;
  if(payload && Array.isArray(payload.selected) && payload.selected.length) CONFIG.weatherProviders=payload.selected;
  if(payload && payload.keysInServedConfig===true) console.warn("Weather keys are still present in served config.local.js; rerun installer option 6 to move them to ~/.dashboard-weather.env");
  return payload && Array.isArray(payload.sources) ? payload.sources : [];
}
async function fetchWeatherSources(){
  // Provider URLs, keys, redirects, response limits, and last-good caches are
  // owned by the loopback dashboard control server. The browser deliberately
  // has no direct provider fallback; its degraded path is the same-origin
  // aggregate returned by /api/weather.
  const sources=await fetchServerWeatherSources();
  if(!sources.length) throw new Error((WEATHER_LAST_AGGREGATE&&WEATHER_LAST_AGGREGATE.error)||"no selected weather source answered");
  return sources;
}
