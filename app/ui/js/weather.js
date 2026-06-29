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
async function loadWeather(){
  if(typeof deferDashboardWork==="function" && deferDashboardWork("weather-refresh",()=>loadWeather())) return;
  if(loadWeather._busy) return;
  loadWeather._busy=true;
  try{
    try{
      const aqiReq = CONFIG.showAQI
        ? fetch(CONFIG.aqApi+"/v1/air-quality?latitude="+
                CONFIG.lat+"&longitude="+CONFIG.lon+"&current=us_aqi&timezone=auto"+
                (CONFIG.apiKey?"&apikey="+encodeURIComponent(CONFIG.apiKey):""))
            .then(r=>r.ok?r.json():null)
            .then(j=>{
              const v=j&&j.current?cleanAqi(j.current.us_aqi):null;
              if(v==null) return null;
              j.current.us_aqi=v;
              return j;
            }).catch(()=>null)
        : Promise.resolve(null);
      const sources=await fetchWeatherSources();
      if(!sources.length) throw new Error("no selected weather source answered");
      const beforeWeatherSignature=typeof calendarWeatherSignature==="function"?calendarWeatherSignature():"";
      setWeatherPayload(normalizeWeatherDayRollover(blendWeatherSources(sources),new Date()));
      const afterWeatherSignature=typeof calendarWeatherSignature==="function"?calendarWeatherSignature():"";
      const calendarWeatherChanged=beforeWeatherSignature!==afterWeatherSignature;
      AQI=await aqiReq;
      lastWxOK=Date.now();
      loadWeather._retry=0;
      const paint=()=>{
        renderWeather(); renderSun(); updateStale();
        if(!loadWeather._didCal || calendarWeatherChanged){ loadWeather._didCal=true; renderCalendar(); }
      };
      if(!(typeof deferDashboardWork==="function" && deferDashboardWork("weather-render",paint))) paint();
    }catch(err){
      console.warn("weather failed",err);
      const retryNo=Math.min((loadWeather._retry||0)+1,8);
      const delay=Math.min(15000*retryNo,120000);
      const paint=()=>{
        updateStale();
        $("#wxnow").innerHTML='<div class="dashstate warn"><div class="title">Weather is catching up</div><div class="detail">Network or weather service did not answer. Retrying in '+Math.round(delay/1000)+' seconds.</div></div>';
        const strip=$("#wx14"); if(strip) strip.innerHTML='<div class="dashstate warn"><div class="title">Forecast unavailable</div><div class="detail">The dashboard will refill this automatically when weather data returns.</div></div>';
      };
      if(!(typeof deferDashboardWork==="function" && deferDashboardWork("weather-error",paint))) paint();
      loadWeather._retry=retryNo;
      clearTimeout(loadWeather._timer);
      loadWeather._timer=setTimeout(loadWeather,delay);
    }
  } finally { loadWeather._busy=false; }
}
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
  let pills="";
  if(CONFIG.showUV&&WX.daily&&WX.daily.uv_index_max&&cleanUv(WX.daily.uv_index_max[0])!=null){
    const v=cleanUv(WX.daily.uv_index_max[0]),[lbl,cls]=uvCategory(v);
    pills+=`<span class="pill ${cls}">UV ${Math.round(v)} ${lbl}</span>`;
  }
  if(CONFIG.showAQI&&AQI&&AQI.current&&cleanAqi(AQI.current.us_aqi)!=null){
    const v=cleanAqi(AQI.current.us_aqi),[lbl,cls]=aqiCategory(v);
    pills+=`<span class="pill ${cls}">AQI ${Math.round(v)} ${lbl}</span>`;
  }
  $("#wxnow").innerHTML=
    `<div class="ico">${ic}</div>`+
    `<div class="big">${Math.round(c.temperature_2m)}°</div>`+
    `<div class="meta"><b>${desc}</b>`+
    `<span class="sub wx-current-metrics">`+
      `<span class="wx-metric-token wx-feels-token">Feels&nbsp;${Math.round(c.apparent_temperature)}°</span>`+
      `<span class="wx-metric-token wx-wind-token"><span class="wx-metric-sep" aria-hidden="true">·</span>${Math.round(c.wind_speed_10m)}&nbsp;${CONFIG.windUnit}</span>`+
    `</span>`+
    `<span class="sub">${c.relative_humidity_2m!=null?Math.round(c.relative_humidity_2m)+"% humidity":"Humidity —"}</span>`+
    (pills?`<span class="wxpills">${pills}</span>`:"")+
    `</div>`;

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
    cell.innerHTML=`<div class="dd">${weatherDayLabel(dayKey,WX._weatherLocalDay||new Date())}</div>`+
      `<div class="ic">${dic}</div>`+
      `<div class="desc">${ddesc}</div>`+
      `<div class="temps"><span class="hi">${Math.round(d.temperature_2m_max[i])}°</span>`+
      `<span class="lo">${Math.round(d.temperature_2m_min[i])}°</span></div>`;
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

/* =====================================================================
   ============================  MOON PHASE (local math)  ==============
   ===================================================================== */
function moonSVG(){
  return moonPhaseSVG(moonPhaseAt(Date.now()));
}

function moonPhaseAt(nowMs){
  const lp=2551443;
  const now=Number.isFinite(nowMs)?nowMs/1000:Date.now()/1000;
  const newMoonRef=592500;
  return((now-newMoonRef)%lp+lp)%lp/lp;
}

function moonPhaseSVG(phase){
  const raw=Number.isFinite(Number(phase))?Number(phase):0;
  const p=((raw%1)+1)%1;
  const size=32, r=size*0.44, cx=size/2, cy=size/2;
  const uid="mp"+Math.round(p*10000);
  const litLeft=p>0.5,lightSide=litLeft?-1:1;
  const illumination=(1-Math.cos(2*Math.PI*p))/2;
  const detailRelief=0.44+0.56*Math.abs(Math.cos(Math.PI*p));
  const earthshineOpacity=0.035+0.075*(1-illumination);
  const earthDetailOpacity=0.42+0.28*(1-illumination);
  const terminatorOpacity=0.06+0.13*Math.sqrt(1-illumination);
  const lightReach=Math.abs(Math.cos(Math.PI*p));
  const lx=cx+lightSide*r*0.55*lightReach, ly=cy-r*(0.28+0.17*detailRelief), lz=r*(1.35+0.65*detailRelief);

  function litClip(){
    if(p<=0.0001||p>=0.9999)return`<circle cx="${cx}" cy="${cy}" r="0.01"/>`;
    if(Math.abs(p-0.5)<0.0001)return`<circle cx="${cx}" cy="${cy}" r="${r}"/>`;
    const top=cy-r,bot=cy+r,k=Math.cos(2*Math.PI*p),rx=Math.max(0.01,Math.abs(k)*r),wax=p<0.5;
    const oS=wax?1:0,iS=wax?(p<0.25?0:1):(p>0.75?1:0);
    return`<path d="M${cx},${top} A${r},${r} 0 0,${oS} ${cx},${bot} A${rx},${r} 0 0,${iS} ${cx},${top} Z"/>`;
  }
  function terminatorPath(){
    if(p<=0.0001||p>=0.9999||Math.abs(p-0.5)<0.0001)return"";
    const top=cy-r,bot=cy+r,k=Math.cos(2*Math.PI*p),rx=Math.max(0.01,Math.abs(k)*r),wax=p<0.5;
    const iS=wax?(p<0.25?0:1):(p>0.75?1:0);
    return`M${cx},${bot} A${rx},${r} 0 0,${iS} ${cx},${top}`;
  }

  function mp(pts){
    return pts.map((point,i)=>`${i===0?"M":"L"}${(cx+point[0]*r).toFixed(2)},${(cy+point[1]*r).toFixed(2)}`).join(" ")+" Z";
  }
  const maria=[
    {d:mp([[-0.05,-0.55],[-0.25,-0.48],[-0.42,-0.30],[-0.45,-0.08],[-0.32,0.04],[-0.10,-0.02],[0.05,-0.15],[0.10,-0.35],[0.02,-0.52]]),fill:"#2e2c24",op:0.50},
    {d:mp([[0.10,-0.12],[0.20,-0.28],[0.38,-0.28],[0.45,-0.10],[0.35,0.05],[0.14,0.05]]),fill:"#323028",op:0.44},
    {d:mp([[0.08,0.02],[0.24,-0.02],[0.40,0.10],[0.38,0.26],[0.20,0.28],[0.06,0.18]]),fill:"#302e26",op:0.40},
    {d:mp([[0.52,-0.12],[0.62,-0.20],[0.70,-0.10],[0.66,0.06],[0.54,0.08]]),fill:"#2c2a22",op:0.48},
    {d:mp([[-0.08,0.22],[-0.28,0.20],[-0.40,0.36],[-0.28,0.50],[-0.06,0.42],[0.06,0.30]]),fill:"#302e26",op:0.42},
    {d:mp([[0.20,0.30],[0.42,0.24],[0.50,0.42],[0.36,0.54],[0.16,0.50]]),fill:"#2e2c24",op:0.38},
    {d:mp([[-0.44,-0.24],[-0.60,-0.10],[-0.66,0.10],[-0.60,0.30],[-0.44,0.36],[-0.30,0.22],[-0.28,0.02],[-0.36,-0.12]]),fill:"#2a2820",op:0.36},
  ];

  const ldir=lightSide;
  function crater(x,y,cr,depth){
    const contrast=depth*detailRelief,px=(cx+x*r).toFixed(2),py=(cy+y*r).toFixed(2),crs=cr*r;
    const sdx=(ldir*crs*0.55).toFixed(2),sdy=(crs*0.30).toFixed(2);
    const sdx7=((ldir*crs*0.55)*0.7).toFixed(2),sdy7=((crs*0.30)*0.7).toFixed(2);
    return `<circle cx="${px}" cy="${py}" r="${(crs*1.22).toFixed(2)}" fill="#d8d6c8" fill-opacity="${(contrast*0.55).toFixed(2)}"/>`+
      `<circle cx="${px}" cy="${py}" r="${(crs*1.06).toFixed(2)}" fill="#b0ae9e" fill-opacity="${(contrast*0.40).toFixed(2)}"/>`+
      `<circle cx="${px}" cy="${py}" r="${crs.toFixed(2)}" fill="#181610" fill-opacity="${(contrast*0.82).toFixed(2)}"/>`+
      `<ellipse cx="${(parseFloat(px)+parseFloat(sdx7)).toFixed(2)}" cy="${(parseFloat(py)+parseFloat(sdy7)).toFixed(2)}" rx="${(crs*0.78).toFixed(2)}" ry="${(crs*0.52).toFixed(2)}" fill="#08060a" fill-opacity="${(contrast*0.70).toFixed(2)}"/>`+
      `<ellipse cx="${(parseFloat(px)-ldir*crs*0.28).toFixed(2)}" cy="${(parseFloat(py)-crs*0.22).toFixed(2)}" rx="${(crs*0.22).toFixed(2)}" ry="${(crs*0.16).toFixed(2)}" fill="#f0eedc" fill-opacity="${(contrast*0.68).toFixed(2)}"/>`;
  }

  function rays(x,y,cr,len,n,alpha){
    const rx2=cx+x*r,ry2=cy+y*r;let out="";
    for(let i=0;i<n;i++){
      const ang=(i/n)*Math.PI*2+i*0.31,rlen=cr*r+len*r*(0.5+((i*7)%10)/10),sp=0.020;
      const ex=rx2+Math.cos(ang)*rlen,ey=ry2+Math.sin(ang)*rlen;
      const p1x=(rx2+Math.cos(ang+sp)*cr*r).toFixed(2),p1y=(ry2+Math.sin(ang+sp)*cr*r).toFixed(2);
      const p2x=(rx2+Math.cos(ang-sp)*cr*r).toFixed(2),p2y=(ry2+Math.sin(ang-sp)*cr*r).toFixed(2);
      out+=`<path d="M${p1x},${p1y} L${ex.toFixed(2)},${ey.toFixed(2)} L${p2x},${p2y} Z" fill="url(#ray${uid})" fill-opacity="${(alpha*(0.54+0.46*detailRelief)).toFixed(3)}"/>`;
    }
    return out;
  }

  const mariaTerrain=maria.map(m=>`<path d="${m.d}" fill="${m.fill}" fill-opacity="${(m.op*(0.78+0.22*detailRelief)).toFixed(3)}"/>`).join("");
  const craterTerrain=crater(-0.22,-0.36,0.105,1.0)+
    crater(0.06,0.50,0.092,1.0)+
    crater(-0.52,0.18,0.085,0.92)+
    crater(0.44,-0.28,0.072,0.88)+
    crater(0.14,-0.54,0.062,0.82)+
    crater(-0.08,0.18,0.052,0.78)+
    crater(0.56,0.12,0.050,0.80)+
    crater(-0.38,-0.12,0.044,0.74)+
    crater(0.30,0.42,0.042,0.74)+
    crater(-0.16,0.56,0.038,0.70)+
    crater(0.60,-0.46,0.034,0.66)+
    crater(-0.60,-0.40,0.040,0.68)+
    crater(0.28,-0.18,0.036,0.65)+
    crater(-0.34,0.44,0.034,0.65)+
    crater(0.50,-0.52,0.030,0.62);
  const rayTerrain=rays(-0.22,-0.36,0.105,0.76,12,0.050)+rays(0.06,0.50,0.092,0.82,14,0.058);
  const terminator=terminatorPath();

  let starStr="";
  let seed=7;
  for(let i=0;i<28;i++){
    seed=(seed*1664525+1013904223)&0x7fffffff;const sx=(seed%1000)/1000*size;
    seed=(seed*1664525+1013904223)&0x7fffffff;const sy=(seed%1000)/1000*size;
    if(Math.hypot(sx-cx,sy-cy)>r+2){
      seed=(seed*1664525+1013904223)&0x7fffffff;
      const a=(0.15+(seed%100)/100*0.55).toFixed(2),sr=(0.12+(seed%10)/10*0.28).toFixed(2);
      starStr+=`<circle cx="${sx.toFixed(1)}" cy="${sy.toFixed(1)}" r="${sr}" fill="white" fill-opacity="${a}"/>`;
    }
  }

  return `<svg class="moonphase-svg" viewBox="0 0 ${size} ${size}" width="1em" height="1em" style="display:block" aria-hidden="true">`+
`<defs>`+
`<clipPath id="disc${uid}"><circle cx="${cx}" cy="${cy}" r="${r}"/></clipPath>`+
`<clipPath id="lit${uid}">${litClip()}</clipPath>`+
`<mask id="night${uid}" maskUnits="userSpaceOnUse" x="0" y="0" width="${size}" height="${size}"><rect x="0" y="0" width="${size}" height="${size}" fill="#000"/><circle cx="${cx}" cy="${cy}" r="${r}" fill="#fff"/><g fill="#000">${litClip()}</g></mask>`+
`<radialGradient id="sky${uid}" cx="50%" cy="50%" r="55%"><stop offset="0%" stop-color="#0c1020"/><stop offset="100%" stop-color="#040610"/></radialGradient>`+
`<radialGradient id="surf${uid}" cx="38%" cy="32%" r="75%"><stop offset="0%" stop-color="#eceade"/><stop offset="30%" stop-color="#d6d4c8"/><stop offset="62%" stop-color="#c0beb2"/><stop offset="82%" stop-color="#aeac9e"/><stop offset="100%" stop-color="#949290"/></radialGradient>`+
`<radialGradient id="limb${uid}" cx="50%" cy="50%" r="50%"><stop offset="72%" stop-color="#f5f2e8" stop-opacity="0"/><stop offset="90%" stop-color="#f0eedc" stop-opacity="0.16"/><stop offset="100%" stop-color="#e8e6d0" stop-opacity="0.32"/></radialGradient>`+
`<radialGradient id="earth${uid}" cx="50%" cy="50%" r="55%"><stop offset="0%" stop-color="#31537d" stop-opacity="0.24"/><stop offset="65%" stop-color="#102a51" stop-opacity="0.14"/><stop offset="100%" stop-color="#04080e" stop-opacity="0.03"/></radialGradient>`+
`<linearGradient id="ray${uid}" x1="0%" y1="0%" x2="100%" y2="0%"><stop offset="0%" stop-color="#dedad0" stop-opacity="1"/><stop offset="100%" stop-color="#dedad0" stop-opacity="0"/></linearGradient>`+
`<filter id="noise${uid}" x="-5%" y="-5%" width="110%" height="110%" color-interpolation-filters="sRGB">`+
`<feTurbulence type="fractalNoise" baseFrequency="0.70 0.65" numOctaves="4" seed="8" result="turb"/>`+
`<feColorMatrix type="saturate" values="0" in="turb" result="grey"/>`+
`<feComposite in="grey" in2="SourceGraphic" operator="in" result="masked"/>`+
`<feBlend in="SourceGraphic" in2="masked" mode="multiply" result="textured"/>`+
`<feComponentTransfer in="textured"><feFuncR type="linear" slope="0.95" intercept="0.03"/><feFuncG type="linear" slope="0.95" intercept="0.03"/><feFuncB type="linear" slope="0.93" intercept="0.02"/></feComponentTransfer>`+
`</filter>`+
`<filter id="spec${uid}" x="-10%" y="-10%" width="120%" height="120%" color-interpolation-filters="sRGB">`+
`<feTurbulence type="fractalNoise" baseFrequency="0.52 0.48" numOctaves="3" seed="12" result="bumpMap"/>`+
`<feColorMatrix type="matrix" values="0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 1 0 0 0 -0.25" in="bumpMap" result="bumpA"/>`+
`<feComposite in="bumpA" in2="SourceGraphic" operator="in" result="bumpC"/>`+
`<feSpecularLighting in="bumpC" surfaceScale="4.2" specularConstant="0.85" specularExponent="26" lighting-color="#f0eedc" result="spec"><fePointLight x="${lx.toFixed(2)}" y="${ly.toFixed(2)}" z="${lz.toFixed(2)}"/></feSpecularLighting>`+
`<feComposite in="spec" in2="SourceGraphic" operator="in" result="specC"/>`+
`<feBlend in="SourceGraphic" in2="specC" mode="screen"/>`+
`</filter>`+
`<filter id="tglow${uid}"><feGaussianBlur stdDeviation="${(r*0.038).toFixed(2)}"/></filter>`+
`</defs>`+
`<rect x="0" y="0" width="${size}" height="${size}" rx="${Math.round(size*0.13)}" fill="url(#sky${uid})"/>`+
starStr+
`<g clip-path="url(#disc${uid})">`+
`<circle cx="${cx}" cy="${cy}" r="${r}" fill="#0c0e18"/>`+
`<circle cx="${cx}" cy="${cy}" r="${r}" fill="url(#earth${uid})"/>`+
`<g mask="url(#night${uid})" opacity="${earthshineOpacity.toFixed(3)}"><circle cx="${cx}" cy="${cy}" r="${r}" fill="url(#surf${uid})"/><g opacity="${earthDetailOpacity.toFixed(3)}">${mariaTerrain}${crater(-0.22,-0.36,0.105,0.46)+crater(0.06,0.50,0.092,0.42)+crater(0.44,-0.28,0.072,0.36)}</g></g>`+
`<g clip-path="url(#lit${uid})">`+
`<circle cx="${cx}" cy="${cy}" r="${r}" fill="url(#surf${uid})"/>`+
`<g filter="url(#noise${uid})"><circle cx="${cx}" cy="${cy}" r="${r}" fill="url(#surf${uid})"/></g>`+
mariaTerrain+craterTerrain+rayTerrain+
`<g filter="url(#spec${uid})" opacity="${(0.28+0.72*detailRelief).toFixed(3)}"><circle cx="${cx}" cy="${cy}" r="${r}" fill="url(#surf${uid})"/></g>`+
`<circle cx="${cx}" cy="${cy}" r="${r}" fill="url(#limb${uid})"/>`+
`</g>`+
(terminator?
`<g clip-path="url(#lit${uid})"><path d="${terminator}" stroke="#c9d7ee" stroke-opacity="${terminatorOpacity.toFixed(3)}" stroke-width="${(r*0.040).toFixed(2)}" fill="none"/><path d="${terminator}" stroke="#9bb9e8" stroke-opacity="${(terminatorOpacity*0.72).toFixed(3)}" stroke-width="${(r*0.070).toFixed(2)}" fill="none" filter="url(#tglow${uid})"/></g>`
:"")+
`</g>`+
`<circle cx="${cx}" cy="${cy}" r="${r}" fill="none" stroke="rgba(180,176,158,0.20)" stroke-width="0.6"/>`+
`</svg>`;
}
