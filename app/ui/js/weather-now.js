function uvCategory(v){
  v=Math.round(v);
  if(v<=2)  return ["Low","uv-low"];
  if(v<=5)  return ["Moderate","uv-mod"];
  if(v<=7)  return ["High","uv-high"];
  if(v<=10) return ["Very High","uv-vhigh"];
  return ["Extreme","uv-ext"];
}
function aqiCategory(v){
  v=Math.round(v);
  if(v<=50)  return ["Good","aqi-good"];
  if(v<=100) return ["Moderate","aqi-mod"];
  if(v<=150) return ["Sensitive","aqi-usg"];
  if(v<=200) return ["Unhealthy","aqi-unh"];
  if(v<=300) return ["Very Unhealthy","aqi-vunh"];
  return ["Hazardous","aqi-haz"];
}
function wxNum(v,digits){
  if(v==null || !Number.isFinite(+v)) return "—";
  const n=Number(v), d=(digits==null?1:digits), r=Math.round(n*Math.pow(10,d))/Math.pow(10,d);
  return Math.abs(r-Math.round(r))<0.0001 ? String(Math.round(r)) : r.toFixed(d).replace(/0+$/,"").replace(/\.$/,"");
}
function wxPercent(v){ return wxNum(v,1)+"%"; }
function weatherBindDelegatedOpen(strip){
  if(!strip||strip.dataset.weatherDelegated==="1")return;
  strip.dataset.weatherDelegated="1";
  // One tap-aware handler keeps a drag over forecast rows from becoming an
  // open action and avoids recreating listeners whenever weather refreshes.
  bindTap(strip,e=>{
    const cell=e.target&&e.target.closest&&e.target.closest("[data-weather-day]");
    if(!cell||!strip.contains(cell))return;
    const index=strip._weatherIndexByDay&&strip._weatherIndexByDay[cell.dataset.weatherDay];
    if(Number.isInteger(index))showWxDayPopup(index);
  });
}
function renderWeather(){
  if(!WX)return;
  const c=WX.current,[desc,ic]=wmo(c.weather_code);
  const now=$("#wxnow");
  if(now){
    now.replaceChildren();
    const icon=el("div","ico");icon.innerHTML=ic;
    const meta=el("div","meta"),metrics=el("span","sub wx-current-metrics");
    meta.appendChild(el("b",null,desc));
    metrics.append(el("span","wx-metric-token wx-feels-token",Number.isFinite(c.apparent_temperature)?"Feels "+Math.round(c.apparent_temperature)+"°":"Feels —"));
    const wind=el("span","wx-metric-token wx-wind-token");
    const sep=el("span","wx-metric-sep","·");sep.setAttribute("aria-hidden","true");
    wind.append(sep,document.createTextNode(Math.round(c.wind_speed_10m)+" "+CONFIG.windUnit));metrics.appendChild(wind);
    meta.append(metrics,el("span","sub",c.relative_humidity_2m!=null?Math.round(c.relative_humidity_2m)+"% humidity":"Humidity —"));
    const pills=el("span","wxpills");
    if(CONFIG.showUV&&WX.daily&&WX.daily.uv_index_max&&cleanUv(WX.daily.uv_index_max[0])!=null){const v=cleanUv(WX.daily.uv_index_max[0]),[lbl,cls]=uvCategory(v);pills.appendChild(el("span","pill "+cls,"UV "+Math.round(v)+" "+lbl));}
    if(CONFIG.showAQI&&AQI&&AQI.current&&cleanAqi(AQI.current.us_aqi)!=null){const v=cleanAqi(AQI.current.us_aqi),[lbl,cls]=aqiCategory(v);pills.appendChild(el("span","pill "+cls,"AQI "+Math.round(v)+" "+lbl));}
    if(pills.childNodes.length)meta.appendChild(pills);
    now.append(icon,el("div","big",Math.round(c.temperature_2m)+"°"),meta);
  }

  const strip=$("#wx14");
  if(!strip)return;
  if(typeof scrollRootState==="function")scrollRootState(strip,"hot-list");
  const anchor=typeof captureScrollAnchor==="function"?captureScrollAnchor(strip,"[data-weather-day]","weatherDay"):null;
  if(typeof dashboardListOverscanClear==="function")dashboardListOverscanClear(strip);
  strip.replaceChildren();weatherBindDelegatedOpen(strip);
  const d=WX.daily||{},times=Array.isArray(d.time)?d.time:[];
  const configuredDays=Math.max(1,Number(CONFIG.weatherForecastMaxDays)||16);
  const inlineLimit=typeof dashboardFitWeatherDayLimit==="function"?dashboardFitWeatherDayLimit():0;
  const n=Math.min(configuredDays,times.length,inlineLimit>0?inlineLimit:Infinity);
  if(!n){
    const state=el("div","dashstate warn");
    state.append(el("div","title","Today’s forecast is catching up"),el("div","detail","Cached days are from before today. Dash-Go is requesting a fresh forecast."));
    strip.appendChild(state);
    return;
  }
  const frag=document.createDocumentFragment();
  strip._weatherIndexByDay=Object.create(null);
  for(let i=0;i<n;i++){
    const dayKey=d.time[i];
    const [ddesc,dic]=(typeof wmoSidebar==="function"?wmoSidebar:wmo)(d.weather_code[i]);
    const cell=el("div","wxday");
    cell.dataset.weatherDay=dayKey;cell.dataset.weatherIndex=String(i);
    strip._weatherIndexByDay[dayKey]=i;
    const dayIcon=el("div","ic");dayIcon.innerHTML=dic;
    const temps=el("div","temps");
    temps.append(el("span","hi",Math.round(d.temperature_2m_max[i])+"°"),el("span","lo",Math.round(d.temperature_2m_min[i])+"°"));
    cell.append(el("div","dd",weatherDayLabel(dayKey,WX._weatherLocalDay||new Date())),dayIcon,el("div","desc",ddesc),temps);
    frag.appendChild(cell);
  }
  strip.appendChild(frag);
  if(typeof restoreScrollAnchor==="function")restoreScrollAnchor(strip,anchor,"[data-weather-day]","weatherDay");
  if(typeof dashboardListOverscanAfterRender==="function")dashboardListOverscanAfterRender(strip,".wxday");
}
function renderSun(){
  const sunrise=$("#sunrise"), sunset=$("#sunset");
  if(!WX||!WX.daily||!Array.isArray(WX.daily.sunrise)||!WX.daily.sunrise.length){
    if(sunrise) sunrise.textContent="↑ —";
    if(sunset) sunset.textContent="↓ —";
    const moon=$("#sun")&&$("#sun").querySelector(".moon"); if(moon) moon.innerHTML=moonSVG();
    return;
  }
  const tf=FMT.hm2;
  const sr=new Date(WX.daily.sunrise[0]);
  sunrise.textContent="↑ "+tf.format(sr);
  if(Array.isArray(WX.daily.sunset)&&WX.daily.sunset.length){
    const ss=new Date(WX.daily.sunset[0]);
    sunset.textContent="↓ "+tf.format(ss);
  }
  $("#sun").querySelector(".moon").innerHTML=moonSVG();
}
