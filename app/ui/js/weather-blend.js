// 06-weather-blend.js — canonical browser weather normalization and blending.
// Provider adapters remain in weather-sources.js; this file owns the one blend
// implementation consumed by the dashboard and its per-source review surface.
function finiteNums(vals){ return (vals||[]).map(v=>Number(v)).filter(v=>Number.isFinite(v)); }
function avg(vals){ const nums=finiteNums(vals); return nums.length?nums.reduce((a,b)=>a+b,0)/nums.length:null; }
function meanNums(nums){ return nums.length?nums.reduce((a,b)=>a+b,0)/nums.length:null; }
function medianNums(nums){ if(!nums.length) return null; const a=[...nums].sort((x,y)=>x-y), m=Math.floor(a.length/2); return a.length%2?a[m]:(a[m-1]+a[m])/2; }
function quantileNums(nums,q){ if(!nums.length) return null; const a=[...nums].sort((x,y)=>x-y), pos=(a.length-1)*q, lo=Math.floor(pos), hi=Math.ceil(pos); return lo===hi?a[lo]:a[lo]+(a[hi]-a[lo])*(pos-lo); }
function firstGood(vals){ return vals.find(v=>v!==undefined&&v!==null&&v!==""); }
function emptyDaily(){ return {time:[],weather_code:[],temperature_2m_max:[],temperature_2m_min:[],apparent_temperature_max:[],precipitation_sum:[],precipitation_probability_max:[],wind_speed_10m_max:[],uv_index_max:[],sunrise:[],sunset:[]}; }
// Daily precipitation is canonical millimetres of liquid water throughout
// Dash-Go. Convert at each provider boundary before robust blending so inches
// and millimetres can never be averaged together.
const WEATHER_PRECIP_MM_MAX=1500;
function precipitationMM(value,unit){
  const n=Number(value); if(!Number.isFinite(n)||n<0) return null;
  switch(String(unit||"").trim().toLowerCase()){
    case "mm":case "millimeter":case "millimeters":case "millimetre":case "millimetres": return n;
    case "cm":case "centimeter":case "centimeters":case "centimetre":case "centimetres": return n*10;
    case "in":case "inch":case "inches": return n*25.4;
    default:return null;
  }
}
function precipitationSumMM(...values){
  let total=0,found=false;
  for(const value of values){ const n=Number(value); if(!Number.isFinite(n)||n<0) continue; total+=n;found=true; }
  return found?total:0;
}
function sourceOk(id,data){ data._source=id; data._sourceLabel=(WEATHER_SOURCE_META[id]||{}).label||id; return data; }
function clampWeatherValue(v,key){
  if(v===null || v===undefined || v==="") return null;
  v=Number(v); if(!Number.isFinite(v)) return null;
  if(/temperature/.test(key)){
    const min=CONFIG.tempUnit==="celsius"?-62:-80, max=CONFIG.tempUnit==="celsius"?55:131;
    return v>=min&&v<=max?v:null;
  }
  if(key==="relative_humidity_2m"||key==="precipitation_probability_max"||key==="precipitation_probability") return v>=0&&v<=100?v:null;
  if(key==="wind_speed_10m"||key==="wind_speed_10m_max") return v>=0&&v<=180?v:null;
  if(key==="uv_index_max") return v>=0&&v<=20?v:null;
  if(key==="us_aqi") return v>=0&&v<=500?v:null;
  if(key==="precipitation_sum") return v>=0&&v<=WEATHER_PRECIP_MM_MAX?v:null;
  return v;
}
function cleanUv(v){ return clampWeatherValue(v,"uv_index_max"); }
function cleanAqi(v){ return clampWeatherValue(v,"us_aqi"); }
function cleanWeatherArray(arr,key){ return Array.isArray(arr)?arr.map(v=>clampWeatherValue(v,key)):arr; }
function cloneWeatherSource(src){
  const out={...src,current:{...(src&&src.current||{})},daily:{...(src&&src.daily||{})},hourly:src&&src.hourly?{...src.hourly}:src&&src.hourly};
  const d=out.daily||{};
  for(const key of ["temperature_2m_max","temperature_2m_min","apparent_temperature_max","precipitation_sum","precipitation_probability_max","wind_speed_10m_max","uv_index_max"]){
    if(Array.isArray(d[key])) d[key]=cleanWeatherArray(d[key],key);
  }
  const c=out.current||{};
  for(const key of ["temperature_2m","apparent_temperature","wind_speed_10m","relative_humidity_2m"]){ c[key]=clampWeatherValue(c[key],key); }
  if(out.hourly){
    const h=out.hourly;
    if(Array.isArray(h.temperature_2m)) h.temperature_2m=cleanWeatherArray(h.temperature_2m,"temperature_2m");
    if(Array.isArray(h.precipitation_probability)) h.precipitation_probability=cleanWeatherArray(h.precipitation_probability,"precipitation_probability");
  }
  return out;
}
function weatherThreshold(key,med){
  if(/temperature/.test(key)) return CONFIG.tempUnit==="celsius"?8.5:15;
  if(key==="wind_speed_10m"||key==="wind_speed_10m_max") return Math.max(CONFIG.windUnit==="kmh"?16:10, Math.abs(med||0)*0.65);
  if(key==="relative_humidity_2m") return 25;
  if(key==="uv_index_max") return 3;
  if(key==="precipitation_sum") return Math.max(1, Math.abs(med||0)*2+5);
  return 999999;
}
function robustNumeric(vals,key){
  const nums=(vals||[]).map(v=>clampWeatherValue(v,key)).filter(v=>v!==null);
  if(!nums.length) return {value:null,count:0,used:0,dropped:0,method:"none",min:null,max:null,spread:0,disagree:false};
  const min=Math.min(...nums), max=Math.max(...nums), spread=max-min;
  if(nums.length<4) return {value:meanNums(nums),count:nums.length,used:nums.length,dropped:0,method:"mean",min,max,spread,disagree:spread>=weatherThreshold(key,medianNums(nums))*1.5};
  const med=medianNums(nums), threshold=weatherThreshold(key,med);
  let kept=nums.filter(v=>Math.abs(v-med)<=threshold);
  let method="trimmed mean";
  if(kept.length<Math.max(3,Math.ceil(nums.length*0.6))){ kept=nums; method="median"; }
  const value=method==="median"?med:meanNums(kept);
  return {value,count:nums.length,used:kept.length,dropped:nums.length-kept.length,method,min,max,spread,median:med,disagree:spread>=threshold*1.5};
}
function blendPrecipProbability(vals){
  const nums=(vals||[]).map(v=>clampWeatherValue(v,"precipitation_probability_max")).filter(v=>v!==null);
  if(!nums.length) return {value:null,count:0,used:0,dropped:0,method:"none",min:null,max:null,spread:0,disagree:false};
  const min=Math.min(...nums), max=Math.max(...nums), spread=max-min, med=medianNums(nums), mean=meanNums(nums);
  const wet=nums.filter(v=>v>=30).length, dry=nums.filter(v=>v<=10).length;
  return {value:mean,count:nums.length,used:nums.length,dropped:0,method:"mean + disagreement",min,max,spread,median:med,disagree:nums.length>=3 && spread>=30 && wet>0 && dry>0, wetSources:wet};
}
function blendWeatherCode(vals){
  const nums=finiteNums(vals).map(v=>Math.round(v));
  if(!nums.length) return 0;
  const counts={}; for(const n of nums) counts[n]=(counts[n]||0)+1;
  return +Object.keys(counts).sort((a,b)=>counts[b]-counts[a]||Math.abs(a)-Math.abs(b))[0];
}
function blendHourlySources(sources){
  const byTime={};
  for(const src of sources){
    const h=src.hourly||{};
    (h.time||[]).forEach((time,i)=>{ if(time){ (byTime[time]=byTime[time]||[]).push({h,i}); } });
  }
  const times=Object.keys(byTime).sort();
  if(!times.length) return (sources[0]&&sources[0].hourly)||null;
  const out={time:[],temperature_2m:[],weather_code:[],precipitation_probability:[]};
  for(const time of times){
    const arr=byTime[time]; out.time.push(time);
    out.temperature_2m.push(robustNumeric(arr.map(x=>x.h.temperature_2m&&x.h.temperature_2m[x.i]),"temperature_2m").value);
    out.weather_code.push(blendWeatherCode(arr.map(x=>x.h.weather_code&&x.h.weather_code[x.i])));
    out.precipitation_probability.push(blendPrecipProbability(arr.map(x=>x.h.precipitation_probability&&x.h.precipitation_probability[x.i])).value);
  }
  return out;
}


function blendWeatherSources(sources){
  const allSources=(sources||[]).map(src=>cloneWeatherSource(src));
  const disabledIds=new Set([...WEATHER_DISABLED_SOURCE_IDS]);
  const active=allSources.filter(src=>!disabledIds.has(String(src&&src._source||"").trim().toLowerCase()));
  sources=active.length?active:allSources;
  if(sources.length===1){
    const only=cloneWeatherSource(sources[0]);
    only._sources=allSources;
    only._activeSources=sources;
    only._sourceLabel=only._sourceLabel;
    only._blend={current:{},daily:{}};
    return only;
  }
  const byDate={};
  for(const src of sources){
    const d=src.daily||{};
    (d.time||[]).forEach((date,i)=>{ if(date){ (byDate[date]=byDate[date]||[]).push({src,d,i}); } });
  }
  const out={current:{},daily:emptyDaily(),hourly:blendHourlySources(sources),_sources:allSources,_activeSources:sources,_sourceLabel:"Combined forecast from "+sources.length+" sources",_blend:{current:{},daily:{}}};
  function setCurrent(key){ const st=robustNumeric(sources.map(s=>s.current&&s.current[key]),key); out.current[key]=st.value; out._blend.current[key]=st; }
  setCurrent("temperature_2m"); setCurrent("apparent_temperature"); setCurrent("wind_speed_10m"); setCurrent("relative_humidity_2m");
  out.current.weather_code=blendWeatherCode(sources.map(s=>s.current&&s.current.weather_code));
  for(const date of Object.keys(byDate).sort().slice(0,Math.max(1,Number(CONFIG.weatherForecastMaxDays)||16))){
    const arr=byDate[date]; out.daily.time.push(date); out._blend.daily[date]={};
    for(const key of ["temperature_2m_max","temperature_2m_min","apparent_temperature_max","precipitation_sum","wind_speed_10m_max","uv_index_max"]){
      const st=robustNumeric(arr.map(x=>x.d[key]&&x.d[key][x.i]),key); out.daily[key].push(st.value); out._blend.daily[date][key]=st;
    }
    const high=out.daily.temperature_2m_max[out.daily.temperature_2m_max.length-1],low=out.daily.temperature_2m_min[out.daily.temperature_2m_min.length-1];
    if(Number.isFinite(high)&&Number.isFinite(low)&&low>high){
      out.daily.temperature_2m_min[out.daily.temperature_2m_min.length-1]=high;
      out._blend.daily[date].temperature_2m_min={...out._blend.daily[date].temperature_2m_min,value:high,coherenceAdjusted:true};
    }
    const pst=blendPrecipProbability(arr.map(x=>x.d.precipitation_probability_max&&x.d.precipitation_probability_max[x.i]));
    out.daily.precipitation_probability_max.push(pst.value); out._blend.daily[date].precipitation_probability_max=pst;
    out.daily.weather_code.push(blendWeatherCode(arr.map(x=>x.d.weather_code&&x.d.weather_code[x.i])));
    out.daily.sunrise.push(firstGood(arr.map(x=>x.d.sunrise&&x.d.sunrise[x.i]))||null);
    out.daily.sunset.push(firstGood(arr.map(x=>x.d.sunset&&x.d.sunset[x.i]))||null);
  }
  return out;
}
function weatherSourceRowsForDay(i){
  const date=WX&&WX.daily&&WX.daily.time&&WX.daily.time[i];
  if(!date) return [];
  const byId={};
  for(const src of (WX._sources||[])) byId[String(src._source||"").trim().toLowerCase()]=src;
  const ids=weatherProviderList();
  const statusById={};
  for(const s of (WEATHER_LAST_SOURCE_STATUS||[])) statusById[s.id]=s;
  return ids.map(id=>{
    const sid=String(id||"").trim().toLowerCase();
    const src=byId[sid], meta=WEATHER_SOURCE_META[sid]||{}, st=statusById[sid]||{};
    const disabled=weatherSourceDisabled(sid);
    if(src){
      const d=src.daily||{}, idx=weatherSourceDailyIndexFor(src,date), c=src.current||{};
      return {id:sid,label:src._sourceLabel||meta.label||sid,tier:meta.tier||"", ok:true, disabled, current:c, daily:d, idx};
    }
    return {id:sid,label:st.label||meta.label||sid,tier:st.tier||meta.tier||"", ok:false, disabled, error:st.error||"No response", daily:null, idx:-1};
  });
}
