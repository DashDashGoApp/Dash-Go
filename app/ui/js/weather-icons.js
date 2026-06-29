// 05-popups-04-weather-icons.js — split from 05-popups-maps.js.
// Weather icons as inline SVG — font-independent so they render identically
// on the Pi regardless of which emoji font is (or isn't) installed.
// Each WMO code maps to [description, iconKey]; icon variants remain lightweight
// shared inline SVG strings, selected by the visual-style setting.
function buildWeatherIconSet(style){
  const palettes={
    soft:{sun:'#d9c074',cloud:'#c4c8d0',cloudDk:'#9aa0ab',rain:'#8bb4d4',snow:'#dfe6ee',bolt:'#d9c074',ray:1.6,drop:1.6,outline:false},
    bold:{sun:'#f2cf64',cloud:'#d1d6df',cloudDk:'#aab2bf',rain:'#81bff0',snow:'#f0f6ff',bolt:'#ffd45a',ray:2.1,drop:2.2,outline:false},
    outline:{sun:'#e5cf77',cloud:'#cfd6e2',cloudDk:'#aeb8c7',rain:'#92c7ef',snow:'#edf5ff',bolt:'#f4d35e',ray:1.8,drop:1.8,outline:true},
    contrast:{sun:'#ffe169',cloud:'#dbe6f4',cloudDk:'#b9c9dc',rain:'#5bbcff',snow:'#f2f7ff',bolt:'#ffe04f',ray:2.1,drop:2.2,outline:false},
    playful:{sun:'#ffce5c',cloud:'#b9d7f6',cloudDk:'#8faed2',rain:'#68d8ff',snow:'#f4fbff',bolt:'#ffe15c',ray:1.9,drop:1.9,outline:false}
  };
  const p=palettes[style]||palettes.soft;
  const wrap=(inner)=>`<svg class="wxsvg wxsvg-${style}" viewBox="0 0 24 24" width="1em" height="1em" style="display:block;overflow:visible" aria-hidden="true" focusable="false">${inner}</svg>`;
  const iconEdge='rgba(28,36,44,.26)';
  const fillOrStroke=(c)=>p.outline?`fill="rgba(0,0,0,.08)" stroke="${c}" stroke-width="1.55" stroke-linejoin="round"`:`fill="${c}" stroke="${iconEdge}" stroke-width="0.35" stroke-linejoin="round"`;
  const sunC=`<circle cx="12" cy="12" r="${p.outline?4.8:5}" ${fillOrStroke(p.sun)}/>`+
    [...Array(8)].map((_,i)=>{const a=i*Math.PI/4,x=12+Math.cos(a)*8.5,y=12+Math.sin(a)*8.5,x2=12+Math.cos(a)*6.7,y2=12+Math.sin(a)*6.7;return `<line x1="${x2.toFixed(1)}" y1="${y2.toFixed(1)}" x2="${x.toFixed(1)}" y2="${y.toFixed(1)}" stroke="${p.sun}" stroke-width="${p.ray}" stroke-linecap="round"/>`;}).join('');
  const cloudPath=(c)=>`<path d="M7 17.5h9a3.5 3.5 0 0 0 .3-6.98A5 5 0 0 0 6.5 11 3.25 3.25 0 0 0 7 17.5z" ${fillOrStroke(c)}/>`;
  const cloudShift=(c,dx,dy)=>`<g transform="translate(${dx} ${dy})">${cloudPath(c)}</g>`;
  const drops=(c)=>[8,12,16].map(x=>`<line x1="${x}" y1="18.4" x2="${x-1.6}" y2="22.1" stroke="${c}" stroke-width="${p.drop}" stroke-linecap="round"/>`).join('');
  const flake=(x,y,c)=>`<g stroke="${c}" stroke-width="1.1" stroke-linecap="round" opacity=".98"><line x1="${x}" y1="${y-2.2}" x2="${x}" y2="${y+2.2}"/><line x1="${x-1.9}" y1="${y-1.1}" x2="${x+1.9}" y2="${y+1.1}"/><line x1="${x-1.9}" y1="${y+1.1}" x2="${x+1.9}" y2="${y-1.1}"/></g>`;
  const flakes=(c)=>[8,12,16].map(x=>flake(x,21.1,c)).join('');
  const partlySun=`<circle cx="9" cy="10" r="4" ${fillOrStroke(p.sun)}/>`+
      [...Array(8)].map((_,i)=>{const a=i*Math.PI/4,x=9+Math.cos(a)*6.8,y=10+Math.sin(a)*6.8,x2=9+Math.cos(a)*5.4,y2=10+Math.sin(a)*5.4;return `<line x1="${x2.toFixed(1)}" y1="${y2.toFixed(1)}" x2="${x.toFixed(1)}" y2="${y.toFixed(1)}" stroke="${p.sun}" stroke-width="${Math.max(1.25,p.ray-.25)}" stroke-linecap="round"/>`;}).join('');
  return {
    sun: wrap(sunC),
    partly: wrap(partlySun+cloudPath(p.cloud)),
    cloud: wrap(cloudPath(p.cloud)),
    overcast: wrap(cloudPath(p.cloudDk)+cloudShift(p.cloud,-2,-3)),
    fog: wrap(cloudPath(p.cloud)+[19,21].map(y=>`<line x1="5" y1="${y}" x2="19" y2="${y}" stroke="${p.cloudDk}" stroke-width="1.5" stroke-linecap="round"/>`).join('')),
    drizzle: wrap(cloudPath(p.cloud)+drops(p.rain).replace(/22/g,'21')),
    rain: wrap(cloudPath(p.cloudDk)+drops(p.rain)),
    snow: wrap(cloudPath(p.cloud)+flakes(p.snow)),
    storm: wrap(cloudPath(p.cloudDk)+`<path d="M12 18l-2.5 3.5h2L11 24l3-4h-2z" fill="${p.bolt}" stroke="rgba(0,0,0,.18)" stroke-width=".25"/>`),
  };
}
const ICON_CACHE={};
function iconSetForStyle(style){
  const key=(typeof WEATHER_ICON_STYLES!=="undefined" && WEATHER_ICON_STYLES[style]) ? style : "soft";
  if(!ICON_CACHE[key]) ICON_CACHE[key]=buildWeatherIconSet(key);
  return ICON_CACHE[key];
}
function currentWeatherIconSet(){
  const settings=typeof dashboardRuntimeSettings==="function"?dashboardRuntimeSettings():null;
  const s=(settings&&settings.weatherIconStyle)||CONFIG.weatherIconStyle||"soft";
  return iconSetForStyle(s);
}
function weatherIconFor(key){
  const set=currentWeatherIconSet();
  const fallback=iconSetForStyle("soft");
  return set[key]||set.cloud||fallback.cloud;
}
const WMO={0:["Clear","sun"],1:["Mostly clear","partly"],2:["Partly cloudy","partly"],3:["Overcast","overcast"],
45:["Fog","fog"],48:["Rime fog","fog"],51:["Light drizzle","drizzle"],53:["Drizzle","drizzle"],55:["Heavy drizzle","rain"],
61:["Light rain","drizzle"],63:["Rain","rain"],65:["Heavy rain","rain"],66:["Freezing rain","rain"],67:["Freezing rain","rain"],
71:["Light snow","snow"],73:["Snow","snow"],75:["Heavy snow","snow"],77:["Snow grains","snow"],
80:["Showers","drizzle"],81:["Showers","rain"],82:["Violent showers","storm"],85:["Snow showers","snow"],86:["Snow showers","snow"],
95:["Thunderstorm","storm"],96:["Thunderstorm w/ hail","storm"],99:["Severe thunderstorm","storm"]};
function wmo(c){ const e=WMO[c]||["—","cloud"]; return [e[0], weatherIconFor(e[1])]; }
function wmoSidebar(c){
  const brief={96:"Storm + hail",99:"Severe storms"};
  const [desc,ic]=wmo(c);
  return [brief[c]||desc,ic];
}

function weatherSourceUvText(dsrc,idx){
  if(idx<0 || !dsrc || !dsrc.uv_index_max) return "UV —";
  const raw=dsrc.uv_index_max[idx], uv=(typeof cleanUv==="function"?cleanUv(raw):raw);
  return uv==null ? "UV ignored" : "UV "+wxNum(uv,1);
}
function refreshWeatherAfterSourceToggle(dateStr,fallbackIndex){
  if(!WX || !Array.isArray(WX._sources) || typeof blendWeatherSources!=="function") return;
  setWeatherPayload(typeof normalizeWeatherDayRollover==="function"?normalizeWeatherDayRollover(blendWeatherSources(WX._sources),new Date()):blendWeatherSources(WX._sources));
  if(typeof renderWeather==="function") renderWeather();
  const idx=weatherDailyIndexFor(dateStr);
  showWxDayPopup(idx>=0?idx:fallbackIndex);
}
function appendWeatherSourceNotes(body,i){
  if(!WX || !WX._blend || !WX._blend.daily || !WX.daily || !WX.daily.time) return;
  const dateStr=WX.daily.time[i];
  const b=WX._blend.daily[dateStr];
  if(!b) return;
  const notes=[];
  for(const [label,key] of [["High","temperature_2m_max"],["Low","temperature_2m_min"],["Feels","apparent_temperature_max"],["Wind","wind_speed_10m_max"],["UV","uv_index_max"]]){
    const st=b[key];
    if(st && st.count>1 && st.dropped>0) notes.push(`${label}: ${st.used}/${st.count} sources used, ${st.dropped} outlier${st.dropped===1?"":"s"} ignored`);
  }
  const pp=b.precipitation_probability_max;
  if(pp && pp.count>1 && pp.disagree) notes.push(`Precipitation sources disagree: ${wxPercent(pp.min)}–${wxPercent(pp.max)} across ${pp.count} sources; shown value is the average`);
  if(!notes.length) return;
  const card=el("div","wxsourcecompare wxsourcenotes");
  card.appendChild(el("div","wxsourcehead","Source notes"));
  for(const note of notes){
    const r=el("div","row wxnoterow");
    r.innerHTML=`<span>${escapeHTML(note)}</span><span></span>`;
    card.appendChild(r);
  }
  body.appendChild(card);
}
function appendWeatherSourceDetails(body,i){
  if(typeof weatherSourceRowsForDay!=="function") return;
  const dateStr=WX&&WX.daily&&WX.daily.time&&WX.daily.time[i];
  const srcRows=weatherSourceRowsForDay(i);
  if(!srcRows.length) return;
  const card=el("details","wxsourcecompare wxsources wxsources-collapsed");
  card.open=false;
  const sourceCountText=srcRows.length+" source"+(srcRows.length===1?"":"s");
  const summary=el("summary","wxsourcehead");
  const updateSummary=()=>{
    const open=!!card.open;
    summary.textContent="Weather sources · "+sourceCountText+" · tap to "+(open?"hide":"show");
    summary.setAttribute("aria-label",(open?"Hide":"Show")+" weather source details. Source rows support double-tap to toggle inclusion.");
  };
  updateSummary();
  card.addEventListener("toggle",updateSummary);
  card.appendChild(summary);
  const grid=el("div","wxsourcesgrid");
  for(const src of srcRows){
    const idx=src.idx, dsrc=src.daily||{};
    const line=el("div","wxsourcerow"+(src.ok===false?" wxsourcefail":"")+(src.disabled?" wxsourcedisabled":""));
    line.dataset.sourceId=src.id||"";
    // Avoid native browser title tooltips on the kiosk WebKit path; they can leave
    // a ghosted tooltip/outline over modal content. The visible header carries
    // the instruction, while aria-label keeps the row descriptive.
    line.setAttribute("aria-label",src.disabled?"Weather source excluded. Double-tap to include this source":"Weather source included. Double-tap to exclude this source");
    if(src.ok===false){
      const state=src.disabled?"Excluded":"Unavailable";
      const detail=src.disabled?"double-tap to include":(src.error||"No response");
      line.innerHTML=`<div class="wxsourcecardtop"><b>${escapeHTML(src.label)}</b><strong>${state}</strong></div><div class="wxsourcecardmeta"><span>${escapeHTML(src.tier||"")}</span><span>${escapeHTML(detail)}</span></div>`;
    }else if(src.disabled){
      line.innerHTML=`<div class="wxsourcecardtop"><b>${escapeHTML(src.label)}</b><strong>Excluded</strong></div><div class="wxsourcecardmeta"><span>${escapeHTML(src.tier||"")}</span><span>double-tap to include</span></div>`;
    }else{
      const hi=idx>=0&&dsrc.temperature_2m_max?wxNum(dsrc.temperature_2m_max[idx],1)+"°":"—";
      const lo=idx>=0&&dsrc.temperature_2m_min?wxNum(dsrc.temperature_2m_min[idx],1)+"°":"—";
      const pp=idx>=0&&dsrc.precipitation_probability_max&&dsrc.precipitation_probability_max[idx]!=null?wxPercent(dsrc.precipitation_probability_max[idx]):"—";
      const wind=idx>=0&&dsrc.wind_speed_10m_max&&dsrc.wind_speed_10m_max[idx]!=null?wxNum(dsrc.wind_speed_10m_max[idx],1)+" "+CONFIG.windUnit:"—";
      const uv=weatherSourceUvText(dsrc,idx);
      line.innerHTML=`<div class="wxsourcecardtop"><b>${escapeHTML(src.label)}</b><strong>${hi} / ${lo}</strong></div><div class="wxsourcecardmeta"><span>${escapeHTML(src.tier||"")}</span><span>rain ${pp} · wind ${wind} · ${uv}</span></div>`;
    }
    const toggle=()=>{
      if(typeof toggleWeatherSourceDisabled!=="function") return;
      if(toggleWeatherSourceDisabled(src.id,WX&&WX._sources)) refreshWeatherAfterSourceToggle(dateStr,i);
    };
    let lastTap=0;
    line.addEventListener("click",()=>{
      const now=Date.now();
      if(now-lastTap<420){ toggle(); lastTap=0; }
      else lastTap=now;
    });
    grid.appendChild(line);
  }
  card.appendChild(grid);
  body.appendChild(card);
}
function compactHourlyIndexes(idxs){
  const wide=(typeof window!=="undefined" && window.matchMedia && window.matchMedia("(min-width: 1200px) and (min-height: 760px)").matches);
  const limit=wide?10:6;
  if(idxs.length<=limit) return idxs.slice();
  const step=Math.max(1,Math.ceil(idxs.length/limit));
  return idxs.filter((_,pos)=>pos%step===0).slice(0,limit);
}

function weatherHourlyGridColumns(){
  // Weather day popup should always present hourly rows as a two-column
  // down-first grid. Earlier large-screen CSS/JS allowed three columns,
  // which made the reading order awkward and pushed source details aside.
  return 2;
}
function applyWeatherHourlyColumnOrder(rows,count){
  const cols=weatherHourlyGridColumns();
  rows.dataset.hourColumns=String(cols);
  if(cols>1){
    const rowCount=Math.max(1,Math.ceil(Math.max(1,count)/cols));
    rows.style.gridAutoFlow="column";
    rows.style.gridTemplateRows="repeat("+rowCount+", minmax(0, auto))";
  }else{
    rows.style.gridAutoFlow="";
    rows.style.gridTemplateRows="";
  }
}

function appendWeatherHourlySection(body,dateStr){
  if(!WX.hourly) return false;
  const idxs=[];
  WX.hourly.time.forEach((t,j)=>{ if(t.startsWith(dateStr)) idxs.push(j); });
  if(!idxs.length){
    const note=el("div"); note.style.marginTop="12px"; note.style.color="var(--dimmer)";
    note.textContent="Hourly detail isn't published this far out — showing daily summary only.";
    body.appendChild(note);
    return false;
  }
  const h=el("div","wxhourblock");
  const head=el("div","wxhourhead");
  head.appendChild(el("div",null,"Hourly"));
  const toggle=el("button","wxhourtoggle","Show hourly");
  toggle.type="button";
  toggle.setAttribute("aria-expanded","false");
  if(idxs.length>6) head.appendChild(toggle);
  h.appendChild(head);
  const rows=el("div","wxhourrows");
  h.appendChild(rows);
  const render=(expanded)=>{
    rows.innerHTML="";
    const show=expanded?idxs:compactHourlyIndexes(idxs);
    applyWeatherHourlyColumnOrder(rows,show.length);
    for(const j of show){
      const r=el("div","row hourrow wxhourrow");
      const hr=FMT.hour.format(new Date(WX.hourly.time[j]));
      const [,hic]=wmo(WX.hourly.weather_code?WX.hourly.weather_code[j]:0);
      const temp=Array.isArray(WX.hourly.temperature_2m)?wxDegree(WX.hourly.temperature_2m[j]):"—";
      const ppRaw=Array.isArray(WX.hourly.precipitation_probability)?WX.hourly.precipitation_probability[j]:null;
      const pp=ppRaw==null?"":wxPercent(ppRaw);
      const ppClass=Number(ppRaw)>=10?" wxhourprecip-on":"";
      r.innerHTML=`<span class="wxhourtime">${hr}</span><span class="wxhouricon">${hic}</span><span class="wxhourtemp">${temp}</span><span class="wxhourprecip${ppClass}">${pp}</span>`;
      rows.appendChild(r);
    }
  };
  let expanded=false;
  render(false);
  toggle.addEventListener("click",()=>{
    expanded=!expanded;
    toggle.textContent=expanded?"Show fewer":"Show hourly";
    toggle.setAttribute("aria-expanded",expanded?"true":"false");
    render(expanded);
  });
  body.appendChild(h);
  return true;
}

function wxDegree(v){
  if(v==null || !Number.isFinite(+v)) return "—";
  return Math.round(Number(v))+"°";
}
function wxTimeOnly(v){
  if(!v) return ["—",""];
  const parts=FMT.time.format(new Date(v)).split(/\s+/);
  return [parts[0]||"—",parts.slice(1).join(" ")];
}
function wxPrecipTotalText(v){
  if(v==null || !Number.isFinite(+v)) return "— total";
  return wxNum(v,1)+'" total';
}
function wxMetricCard(row){
  const r=el("div","row wxsummaryrow"+(row.extraClass?" "+row.extraClass:""));
  const label=escapeHTML(String(row.label==null?"":row.label));
  const value=escapeHTML(String(row.value==null?"":row.value));
  const unit=escapeHTML(String(row.unit==null?"":row.unit));
  r.innerHTML=`${wxMetricIcon(row.key)}<span class="wxsummarylabel">${label}</span><span class="wxsummaryvalue${row.valueClass?" "+row.valueClass:""}">${value}</span><span class="wxsummaryunit${row.unitClass?" "+row.unitClass:""}">${unit}</span>`;
  return r;
}

function wxMetricIcon(kind){
  const base='class="wxstaticon" viewBox="0 0 24 24" aria-hidden="true" focusable="false"';
  const svg=(body)=>`<svg ${base}>${body}</svg>`;
  const common='fill="none" stroke="currentColor" stroke-width="2.1" stroke-linecap="round" stroke-linejoin="round" vector-effect="non-scaling-stroke"';
  if(kind==="high") return svg(`<path ${common} d="M14 14.8V5.5a4 4 0 1 0-8 0v9.3a6 6 0 1 0 8 0Z"/><path ${common} d="M10 6v9"/><path ${common} d="M18 5v8"/><path ${common} d="m15 8 3-3 3 3"/>`);
  if(kind==="low") return svg(`<path ${common} d="M14 14.8V5.5a4 4 0 1 0-8 0v9.3a6 6 0 1 0 8 0Z"/><path ${common} d="M10 6v9"/><path ${common} d="M18 5v8"/><path ${common} d="m15 10 3 3 3-3"/>`);
  if(kind==="feels") return svg(`<path ${common} d="M14 14.8V5.5a4 4 0 1 0-8 0v9.3a6 6 0 1 0 8 0Z"/><path ${common} d="M10 7v8"/><path ${common} d="M18.5 6.5c1.2 1 1.2 2.1 0 3.1s-1.2 2.1 0 3.1"/>`);
  if(kind==="precipChance") return svg(`<path ${common} d="M7 17.5c-2 0-3.6-1.5-3.6-3.4 0-1.8 1.4-3.3 3.2-3.4A5.8 5.8 0 0 1 18 10.2a3.6 3.6 0 0 1-.5 7.3H7Z"/><path ${common} d="M8 20 18 10"/><circle cx="8.5" cy="11.5" r="1.1" fill="currentColor"/><circle cx="17.5" cy="18.5" r="1.1" fill="currentColor"/>`);
  if(kind==="precipTotal") return svg(`<path ${common} d="M12 3.5C9 7.4 7 10.4 7 13.2a5 5 0 0 0 10 0c0-2.8-2-5.8-5-9.7Z"/><path ${common} d="M8 20h8"/><path ${common} d="M18.5 7v10"/><path ${common} d="M17 9h3"/><path ${common} d="M17 15h3"/>`);
  if(kind==="wind") return svg(`<path ${common} d="M3 8h11.5a2.5 2.5 0 1 0-2.2-3.7"/><path ${common} d="M3 12h16.2a2.8 2.8 0 1 1-2.5 4"/><path ${common} d="M3 16h8"/>`);
  if(kind==="uv") return svg(`<circle cx="12" cy="12" r="3.5" fill="none" stroke="currentColor" stroke-width="2.1" vector-effect="non-scaling-stroke"/><path ${common} d="M12 2.5v2.2M12 19.3v2.2M4.6 4.6l1.6 1.6M17.8 17.8l1.6 1.6M2.5 12h2.2M19.3 12h2.2M4.6 19.4l1.6-1.6M17.8 6.2l1.6-1.6"/>`);
  if(kind==="sunrise") return svg(`<path ${common} d="M4 18h16"/><path ${common} d="M7 15a5 5 0 0 1 10 0"/><path ${common} d="M12 4v8"/><path ${common} d="m8.8 7.2 3.2-3.2 3.2 3.2"/>`);
  if(kind==="sunset") return svg(`<path ${common} d="M4 18h16"/><path ${common} d="M7 15a5 5 0 0 1 10 0"/><path ${common} d="M12 4v8"/><path ${common} d="m8.8 8.8 3.2 3.2 3.2-3.2"/>`);
  return svg(`<circle cx="12" cy="12" r="7" fill="none" stroke="currentColor" stroke-width="2.1" vector-effect="non-scaling-stroke"/>`);
}

function showWxDayPopup(i){
  if(typeof setPopupMode==="function") setPopupMode("weatherpop");
  if(!WX) return;
  const d=WX.daily, day=new Date(d.time[i]+"T00:00");
  $("#poptitle").textContent=FMT.dayLong.format(day);
  const [desc,ic]=wmo(d.weather_code[i]);
  $("#popwhen").innerHTML=`<span style="display:inline-flex;align-items:center;gap:8px"><span style="font-size:30px;line-height:1">${ic}</span>${desc}</span>`;
  const body=$("#popbody"); body.innerHTML="";
  const high=Array.isArray(d.temperature_2m_max)?wxDegree(d.temperature_2m_max[i]):"—";
  const low=Array.isArray(d.temperature_2m_min)?wxDegree(d.temperature_2m_min[i]):"—";
  const feels=Array.isArray(d.apparent_temperature_max)?wxDegree(d.apparent_temperature_max[i]):"—";
  const precipChance=Array.isArray(d.precipitation_probability_max)?wxPercent(d.precipitation_probability_max[i]):"—";
  const precipTotal=Array.isArray(d.precipitation_sum)?wxPrecipTotalText(d.precipitation_sum[i]):"— total";
  const windValue=Array.isArray(d.wind_speed_10m_max)?wxNum(d.wind_speed_10m_max[i],0):"—";
  const uvRaw=Array.isArray(d.uv_index_max)?cleanUv(d.uv_index_max[i]):null;
  const uvInfo=uvRaw!=null?uvCategory(uvRaw):["",""];
  const sunrise=d.sunrise?wxTimeOnly(d.sunrise[i]):["—",""];
  const sunset=d.sunset?wxTimeOnly(d.sunset[i]):["—",""];

  const hero=el("div","wxhero");
  hero.innerHTML=`<div class="wxherotemps"><span class="wxherohigh">${high}</span><span class="wxherolow">/ ${low}</span></div><div class="wxherometa"><span>High / low</span><span>Feels up to ${feels}</span></div>`;
  body.appendChild(hero);

  const rows=[
    {key:"precipTotal",label:"Precip",value:precipChance,unit:precipTotal},
    {key:"wind",label:"Wind",value:windValue,unit:CONFIG.windUnit+" max"},
    {key:"uv",label:"UV",value:uvRaw!=null?wxNum(uvRaw,1):"—",unit:uvInfo[0],unitClass:uvInfo[1]?"wxuvsev "+uvInfo[1]:""},
    {key:"sunrise",label:"Sunrise",value:sunrise[0],unit:sunrise[1]},
    {key:"sunset",label:"Sunset",value:sunset[0],unit:sunset[1]},
  ];
  const summary=el("div","wxsummarygrid");
  for(const row of rows) summary.appendChild(wxMetricCard(row));
  body.appendChild(summary);

  const detailWrap=el("div","wxdetailstack");
  appendWeatherHourlySection(detailWrap,d.time[i]);
  appendWeatherSourceDetails(detailWrap,i);
  appendWeatherSourceNotes(detailWrap,i);
  body.appendChild(detailWrap);
  openScrim();
}
