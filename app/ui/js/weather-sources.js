// 06-weather-sources.js — provider adapters and multi-source blending.
let WEATHER_LAST_SOURCE_STATUS=[];
const WEATHER_KEY_REQUIRED=new Set(["weatherapi","openweather","googleweather","tomorrow","visualcrossing","weatherbit","pirateweather","accuweather","xweather"]);
const WEATHER_SOURCE_META={
  openmeteo:{label:"Open-Meteo",tier:"free · no key · non-commercial",maxDays:16,refreshMin:15},
  nws:{label:"NWS / NOAA",tier:"free · no key · US-only · NOAA/NWS",maxDays:7,refreshMin:15},
  weatherapi:{label:"WeatherAPI.com",tier:"free key · 100K/month · 3-day free forecast",maxDays:3,refreshMin:30},
  openweather:{label:"OpenWeather",tier:"free allowance · 1,000/day then billable · 8-day",maxDays:8,refreshMin:30},
  googleweather:{label:"Google Weather",tier:"PAID · Google Maps Platform billing · no normal free tier · 10-day",maxDays:10,refreshMin:30},
  tomorrow:{label:"Tomorrow.io",tier:"free key · 500/day, 25/hour · core forecast ~5 days",maxDays:5,refreshMin:30},
  visualcrossing:{label:"Visual Crossing",tier:"free key · 1,000 records/day · attribution · 15-day",maxDays:15,refreshMin:30},
  weatherbit:{label:"Weatherbit",tier:"free key · 50/day · NON-COMMERCIAL · 7-day",maxDays:7,refreshMin:90},
  pirateweather:{label:"Pirate Weather",tier:"free key · 10,000/month · 8-day",maxDays:8,refreshMin:30},
  accuweather:{label:"AccuWeather",tier:"14-DAY TRIAL then paid · 500/day during trial · 5-day",maxDays:5,refreshMin:30},
  xweather:{label:"Xweather",tier:"free/trial/metered · conservative 9K/month cap · US/CA · 15-day",maxDays:15,refreshMin:30},
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
  return Math.max(weatherConfiguredRefreshMinimum(),weatherRefreshProfileDefaultMinutes());
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
function weatherKey(id){
  const k=(CONFIG.weatherProviderKeys||{})[id];
  return k || (id==="openmeteo"||id==="openmeteo-custom"?CONFIG.apiKey:"") || "";
}
function weatherUnits(){ return CONFIG.tempUnit==="celsius"?"metric":"imperial"; }
function weatherApiLocationQuery(lat,lon){
  const clean=(v)=>{
    const n=Number(v);
    return Number.isFinite(n)?String(Math.round(n*1000000)/1000000):String(v||"").trim();
  };
  return clean(lat)+","+clean(lon);
}
function weatherApiForecastUrl(key,days){
  const base="https://api.weatherapi.com/v1/forecast.json";
  const q=weatherApiLocationQuery(CONFIG.lat,CONFIG.lon);
  const p=new URLSearchParams({key:key,days:String(days),aqi:"no",alerts:"no"});
  return base+"?"+p.toString()+"&q="+q;
}
function cToF(v){ return v==null?null:v*9/5+32; }
function fToC(v){ return v==null?null:(v-32)*5/9; }
function mphToKmh(v){ return v==null?null:v*1.609344; }
function kmhToMph(v){ return v==null?null:v/1.609344; }
function toTemp(v,unit){ return v==null?null:(CONFIG.tempUnit==="celsius"?(unit==="f"?fToC(v):v):(unit==="c"?cToF(v):v)); }
function toWind(v,unit){
  if(v==null || !Number.isFinite(+v)) return null;
  v=+v; unit=unit||CONFIG.windUnit;
  if(unit==="ms"){
    if(CONFIG.windUnit==="kmh") return v*3.6;
    if(CONFIG.windUnit==="mph") return v/0.44704;
    return v;
  }
  if(CONFIG.windUnit==="kmh") return unit==="mph"?mphToKmh(v):v;
  if(CONFIG.windUnit==="mph") return unit==="kmh"?kmhToMph(v):v;
  return unit==="mph"?v*0.44704:(unit==="kmh"?v/3.6:v);
}
function firstNumber(text){ const m=String(text||"").match(/[\d.]+/); return m?parseFloat(m[0]):null; }
function textCode(text){
  text=String(text||"").toLowerCase();
  if(/thunder|storm/.test(text)) return 95;
  if(/snow|sleet|ice|flurr/.test(text)) return 71;
  if(/rain|shower|drizzle/.test(text)) return 61;
  if(/fog|mist|haze/.test(text)) return 45;
  if(/overcast|cloudy/.test(text)) return 3;
  if(/partly|mostly sunny|few/.test(text)) return 2;
  if(/clear|sunny/.test(text)) return 0;
  return 3;
}
function owCode(id){
  id=Number(id||0);
  if(id>=200&&id<300) return 95;
  if(id>=300&&id<600) return id>=500?61:51;
  if(id>=600&&id<700) return 71;
  if(id>=700&&id<800) return 45;
  if(id===800) return 0;
  if(id===801) return 2;
  return 3;
}

async function fetchServerWeatherSources(){
  try{
    const res=await fetch("/api/weather?t="+Date.now(),{cache:"no-store"});
    if(!res.ok) throw new Error("HTTP "+res.status);
    const payload=await res.json();
    if(payload && Array.isArray(payload.status)) WEATHER_LAST_SOURCE_STATUS=payload.status;
    if(payload && Array.isArray(payload.selected) && payload.selected.length) CONFIG.weatherProviders=payload.selected;
    if(payload && payload.keysInServedConfig===true) console.warn("Weather keys are still present in served config.local.js; rerun installer option 6 to move them to ~/.dashboard-weather.env");
    if(payload && Array.isArray(payload.sources)) return payload.sources;
  }catch(e){ console.warn("server weather proxy unavailable; falling back to browser providers",e); }
  return null;
}

async function fetchWeatherSources(){
  const viaServer=await fetchServerWeatherSources();
  if(viaServer) return viaServer;
  const requestedIds=weatherProviderList();
  const status=[];
  for(const id of requestedIds){
    if(WEATHER_KEY_REQUIRED.has(id) && !weatherKey(id)){
      const meta=WEATHER_SOURCE_META[id]||{label:id,tier:""};
      status.push({id,label:meta.label||id,tier:meta.tier||"",ok:false,disabled:true,error:"Missing API key; source ignored until a key is saved"});
    }
  }
  const ids=requestedIds.filter(id=>!WEATHER_KEY_REQUIRED.has(id) || weatherKey(id));
  const tries=ids.map(async id=>{
    const meta=WEATHER_SOURCE_META[id]||{label:id,tier:""};
    try{
      let data=null;
      if(id==="openmeteo"||id==="openmeteo-custom") data=await fetchOpenMeteo(id);
      else if(id==="weatherapi") data=await fetchWeatherApi(id);
      else if(id==="openweather") data=await fetchOpenWeather(id);
      else if(id==="tomorrow") data=await fetchTomorrow(id);
      if(data){ status.push({id,label:meta.label||id,tier:meta.tier||"",ok:true}); return data; }
      throw new Error("unknown provider");
    }catch(e){
      const msg=e&&e.message?e.message:String(e||"failed");
      console.warn("weather source failed",id,e);
      status.push({id,label:meta.label||id,tier:meta.tier||"",ok:false,error:msg});
    }
    return null;
  });
  const out=(await Promise.all(tries)).filter(Boolean);
  if(!out.length && !ids.includes("openmeteo")){
    try{
      const data=await fetchOpenMeteo("openmeteo");
      out.push(data);
      status.push({id:"openmeteo",label:"Open-Meteo",tier:"fallback · free",ok:true,fallback:true});
    }catch(e){
      status.push({id:"openmeteo",label:"Open-Meteo",tier:"fallback · free",ok:false,error:e&&e.message?e.message:String(e)});
    }
  }
  const order=requestedIds.concat(ids).filter((x,i,a)=>a.indexOf(x)===i);
  WEATHER_LAST_SOURCE_STATUS=status.sort((a,b)=>order.indexOf(a.id)-order.indexOf(b.id));
  return out;
}
async function fetchOpenMeteo(id){
  const p=new URLSearchParams({
    latitude:CONFIG.lat, longitude:CONFIG.lon,
    temperature_unit:CONFIG.tempUnit, wind_speed_unit:CONFIG.windUnit, precipitation_unit:"mm",
    timezone:"auto", forecast_days:String(Math.min(16,Math.max(1,Number(CONFIG.weatherForecastMaxDays)||16))),
    current:"temperature_2m,apparent_temperature,weather_code,wind_speed_10m,relative_humidity_2m",
    daily:["weather_code","temperature_2m_max","temperature_2m_min","apparent_temperature_max","precipitation_sum","precipitation_probability_max","wind_speed_10m_max","uv_index_max","sunrise","sunset"].join(","),
    hourly:"temperature_2m,weather_code,precipitation_probability",
  });
  const key=weatherKey(id); if(key) p.set("apikey",key);
  const res=await fetch((CONFIG.wxApi||"https://api.open-meteo.com")+"/v1/forecast?"+p.toString(),{cache:"no-store"});
  if(!res.ok) throw new Error("HTTP "+res.status);
  return sourceOk(id,await res.json());
}
async function fetchWeatherApi(id){
  const key=weatherKey(id); if(!key) throw new Error("missing WeatherAPI.com key");
  const days=weatherProviderDays(id,14);
  const url=weatherApiForecastUrl(key,days);
  const res=await fetch(url,{cache:"no-store"}); if(!res.ok) throw new Error("HTTP "+res.status);
  const j=await res.json(), d=emptyDaily();
  for(const x of ((j.forecast&&j.forecast.forecastday)||[])){
    const day=x.day||{}; d.time.push(x.date); d.weather_code.push(textCode(day.condition&&day.condition.text));
    d.temperature_2m_max.push(toTemp(CONFIG.tempUnit==="celsius"?day.maxtemp_c:day.maxtemp_f,CONFIG.tempUnit==="celsius"?"c":"f"));
    d.temperature_2m_min.push(toTemp(CONFIG.tempUnit==="celsius"?day.mintemp_c:day.mintemp_f,CONFIG.tempUnit==="celsius"?"c":"f"));
    d.apparent_temperature_max.push(null); d.precipitation_sum.push(precipitationMM(day.totalprecip_mm,"mm")); d.precipitation_probability_max.push(day.daily_chance_of_rain); d.wind_speed_10m_max.push(toWind(day.maxwind_mph,"mph")); d.uv_index_max.push(day.uv); d.sunrise.push(null); d.sunset.push(null);
  }
  const c=j.current||{};
  return sourceOk(id,{current:{temperature_2m:toTemp(CONFIG.tempUnit==="celsius"?c.temp_c:c.temp_f,CONFIG.tempUnit==="celsius"?"c":"f"),apparent_temperature:toTemp(CONFIG.tempUnit==="celsius"?c.feelslike_c:c.feelslike_f,CONFIG.tempUnit==="celsius"?"c":"f"),weather_code:textCode(c.condition&&c.condition.text),wind_speed_10m:toWind(c.wind_mph,"mph"),relative_humidity_2m:c.humidity},daily:d,hourly:null});
}
async function fetchOpenWeather(id){
  const key=weatherKey(id); if(!key) throw new Error("missing OpenWeather key");
  const url="https://api.openweathermap.org/data/3.0/onecall?"+new URLSearchParams({lat:CONFIG.lat,lon:CONFIG.lon,appid:key,units:weatherUnits(),exclude:"minutely,alerts"});
  const res=await fetch(url,{cache:"no-store"}); if(!res.ok) throw new Error("HTTP "+res.status);
  const j=await res.json(), d=emptyDaily();
  for(const x of (j.daily||[]).slice(0,weatherProviderDays(id,8))){
    const day=new Date((x.dt||0)*1000).toISOString().slice(0,10); d.time.push(day); d.weather_code.push(owCode(x.weather&&x.weather[0]&&x.weather[0].id));
    d.temperature_2m_max.push(x.temp&&x.temp.max); d.temperature_2m_min.push(x.temp&&x.temp.min); d.apparent_temperature_max.push(x.feels_like&&x.feels_like.day); d.precipitation_sum.push(precipitationSumMM(x.rain,x.snow)); d.precipitation_probability_max.push(x.pop!=null?Math.round(x.pop*100):null); d.wind_speed_10m_max.push(toWind(x.wind_speed,CONFIG.tempUnit==="celsius"?"ms":"mph")); d.uv_index_max.push(x.uvi); d.sunrise.push(x.sunrise?new Date(x.sunrise*1000).toISOString():null); d.sunset.push(x.sunset?new Date(x.sunset*1000).toISOString():null);
  }
  const c=j.current||{};
  return sourceOk(id,{current:{temperature_2m:c.temp,apparent_temperature:c.feels_like,weather_code:owCode(c.weather&&c.weather[0]&&c.weather[0].id),wind_speed_10m:toWind(c.wind_speed,CONFIG.tempUnit==="celsius"?"ms":"mph"),relative_humidity_2m:c.humidity},daily:d,hourly:null});
}
async function fetchTomorrow(id){
  const key=weatherKey(id); if(!key) throw new Error("missing Tomorrow.io key");
  const units=weatherUnits()==="metric"?"metric":"imperial";
  const url="https://api.tomorrow.io/v4/weather/forecast?"+new URLSearchParams({location:CONFIG.lat+","+CONFIG.lon,apikey:key,units,timesteps:"1d,1h"});
  const res=await fetch(url,{cache:"no-store"}); if(!res.ok) throw new Error("HTTP "+res.status);
  const j=await res.json(), d=emptyDaily(), daily=((j.timelines&&j.timelines.daily)||[]);
  for(const x of daily.slice(0,weatherProviderDays(id,5))){
    const v=x.values||{}; d.time.push(String(x.time||"").slice(0,10)); d.weather_code.push(textCode(v.weatherCodeFullDay||v.weatherCodeMax)); d.temperature_2m_max.push(v.temperatureMax); d.temperature_2m_min.push(v.temperatureMin); d.apparent_temperature_max.push(v.temperatureApparentMax); d.precipitation_sum.push(precipitationMM(v.precipitationAccumulationSum,units==="imperial"?"in":"mm")); d.precipitation_probability_max.push(v.precipitationProbabilityMax); d.wind_speed_10m_max.push(v.windSpeedMax); d.uv_index_max.push(v.uvIndexMax); d.sunrise.push(v.sunriseTime||null); d.sunset.push(v.sunsetTime||null);
  }
  const cur=((j.timelines&&j.timelines.hourly&&j.timelines.hourly[0]&&j.timelines.hourly[0].values)||{});
  return sourceOk(id,{current:{temperature_2m:cur.temperature,apparent_temperature:cur.temperatureApparent,weather_code:textCode(cur.weatherCode||cur.weatherCodeFull),wind_speed_10m:cur.windSpeed,relative_humidity_2m:cur.humidity},daily:d,hourly:null});
}
