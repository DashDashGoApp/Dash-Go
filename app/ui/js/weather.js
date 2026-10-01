// 06-weather.js — generated from dashboard.js for maintainability.
/* =====================================================================
   ============================  WEATHER FETCH  ========================
   ===================================================================== */
function weatherLocalDateKey(now){
  if(typeof now==="string" && /^\d{4}-\d{2}-\d{2}/.test(now)) return now.slice(0,10);
  const d=now instanceof Date?now:new Date();
  return d.getFullYear()+"-"+String(d.getMonth()+1).padStart(2,"0")+"-"+String(d.getDate()).padStart(2,"0");
}
function normalizeWeatherDayRollover(wx,now){
  if(!wx || !wx.daily || !Array.isArray(wx.daily.time)) return wx;
  const today=weatherLocalDateKey(now);
  const times=wx.daily.time;
  let start=times.findIndex(value=>String(value||"").slice(0,10)>=today);
  if(start<0) start=times.length;
  if(start>0){
    const daily={...wx.daily};
    for(const [key,value] of Object.entries(daily)) if(Array.isArray(value)) daily[key]=value.slice(start);
    wx.daily=daily;
  }
  wx._weatherLocalDay=today;
  wx._weatherDroppedPastDays=start;
  return wx;
}
function weatherDayLabel(date,now){
  if(String(date||"").slice(0,10)===weatherLocalDateKey(now)) return "Today";
  return FMT.wxDay.format(new Date(String(date||"").slice(0,10)+"T00:00"));
}
async function refreshAQINonblocking(attempt){
  if(!CONFIG.showAQI)return;
  const tries=Number(attempt)||0;
  try{
    const res=await fetch("/api/weather/aqi",{cache:"no-store"});
    if(!res.ok)throw new Error("HTTP "+res.status);
    const payload=await res.json();
    const value=payload&&payload.current?cleanAqi(payload.current.us_aqi):null;
    if(value!=null){
      payload.current.us_aqi=value;
      AQI=payload;
      renderWeather();
      return;
    }
  }catch(_){ }
  if(tries<2)setTimeout(()=>refreshAQINonblocking(tries+1),[1500,3000,6000][tries]);
}
async function loadWeather(){
  if(typeof deferDashboardWork==="function" && deferDashboardWork("weather-refresh",()=>loadWeather())) return;
  if(loadWeather._busy) return;
  loadWeather._busy=true;
  try{
    try{
      const sources=await fetchWeatherSources();
      const beforeWeatherSignature=typeof calendarWeatherSignature==="function"?calendarWeatherSignature():"";
      setWeatherPayload(normalizeWeatherDayRollover(blendWeatherSources(sources),new Date()));
      const afterWeatherSignature=typeof calendarWeatherSignature==="function"?calendarWeatherSignature():"";
      const calendarWeatherChanged=beforeWeatherSignature!==afterWeatherSignature;
      lastWxOK=Date.now();
      loadWeather._retry=0;
      const paint=()=>{
        renderWeather(); renderSun(); updateStale();
        if(!loadWeather._didCal || calendarWeatherChanged){ loadWeather._didCal=true; renderCalendar(); }
      };
      if(!(typeof deferDashboardWork==="function" && deferDashboardWork("weather-render",paint))) paint();
      // Forecast paint never waits on AQI. A bounded same-origin follow-up can
      // add the pill as soon as the server cache is available.
      refreshAQINonblocking(0);
    }catch(err){
      console.warn("weather failed",err);
      const retryNo=Math.min((loadWeather._retry||0)+1,8);
      const delay=Math.min(15000*retryNo,120000);
      const paint=()=>{
        updateStale();
        const current=$("#wxnow");
        if(current){
          current.replaceChildren();
          const state=el("div","dashstate warn");
          state.append(el("div","title","Weather is catching up"),el("div","detail","Network or weather service did not answer. Retrying in "+Math.round(delay/1000)+" seconds."));
          current.appendChild(state);
        }
        const strip=$("#wx14");
        if(strip){
          strip.replaceChildren();
          const state=el("div","dashstate warn");
          state.append(el("div","title","Forecast unavailable"),el("div","detail","The dashboard will refill this automatically when weather data returns."));
          strip.appendChild(state);
        }
      };
      if(!(typeof deferDashboardWork==="function" && deferDashboardWork("weather-error",paint))) paint();
      loadWeather._retry=retryNo;
      clearTimeout(loadWeather._timer);
      loadWeather._timer=setTimeout(loadWeather,delay);
    }
  } finally { loadWeather._busy=false; }
}
