// weather-review-notes.js — the per-day "Source notes" review surface.
// Split out of weather-icons.js so both files stay well inside the 400-line
// non-generated limit: the notes builder explains the blend and does not depend
// on the icon set, so it is a coherent unit on its own. Callers resolve it from
// the shared bundle scope, exactly as before the split.
function appendWeatherSourceNotes(body,i){
  if(!WX || !WX._blend || !WX._blend.daily || !WX.daily || !WX.daily.time) return;
  const dateStr=WX.daily.time[i];
  const b=WX._blend.daily[dateStr];
  if(!b) return;
  const notes=[];
  if(WX._sourceFallbackNote) notes.push(WX._sourceFallbackNote);
  if(b.contributors!=null&&b.totalSources!=null&&b.contributors<b.totalSources) notes.push(`This day uses ${b.contributors} of ${b.totalSources} available sources.`);
  if(b.staleExcluded>0) notes.push(`${b.staleExcluded} stale source${b.staleExcluded===1?" was":"s were"} excluded because fresh data was available.`);
  if(b.usedStaleFallback) notes.push("Only stale provider data was available for this day.");
  for(const [label,key] of [["High","temperature_2m_max"],["Low","temperature_2m_min"],["Feels","apparent_temperature_max"],["Wind","wind_speed_10m_max"],["UV","uv_index_max"]]){
    const st=b[key];
    if(st && st.count>1 && st.dropped>0) notes.push(`${label}: ${st.used}/${st.count} sources used, ${st.dropped} outlier${st.dropped===1?"":"s"} ignored`);
    else if(st&&st.method==="three-source median") notes.push(`${label}: a three-source disagreement was resolved with the median.`);
  }
  const total=b.precipitation_sum;
  if(total&&total.count>1&&total.disagree) notes.push(`Precipitation totals range from ${wxPrecipTotalText(total.min).replace(" total","")} to ${wxPrecipTotalText(total.max).replace(" total","")}; the median is shown.`);
  const pp=b.precipitation_probability_max;
  if(pp && pp.count>1 && pp.disagree) notes.push(`Precipitation sources disagree: ${wxPercent(pp.min)}–${wxPercent(pp.max)} across ${pp.count} sources; ${pp.count>=3?"the median":"the mean"} is shown.`);
  // The blend computes current-condition agreement and hourly provenance as well;
  // these notes used to cover only the daily keys, so a disagreement on the number
  // shown right now — or an hourly view that quietly fell back to one provider —
  // was invisible to the household.
  const cur=WX._blend.current||null;
  for(const [label,key,fmt] of [["Temperature now","temperature_2m",wxDegree],["Feels now","apparent_temperature",wxDegree],["Humidity now","relative_humidity_2m",wxPercent]]){
    const st=cur&&cur[key];
    if(st&&st.count>1&&st.disagree) notes.push(`${label}: ${fmt(st.min)}–${fmt(st.max)} across ${st.count} sources; the ${st.method} is shown.`);
  }
  const hours=WX._blend.hourly;
  if(hours&&hours.rows){
    const tail=(hours.usedFallback||hours.singleSource)?" — no shared timestamps, so one provider's hours are shown":"";
    notes.push(`Hourly: ${hours.rows} hours from ${hours.sources} source${hours.sources===1?"":"s"}${tail}.`);
  }
  if(!notes.length) return;
  const card=el("div","wxsourcecompare wxsourcenotes");
  card.appendChild(el("div","wxsourcehead","Source notes"));
  for(const note of notes){
    const r=el("div","row wxnoterow");
    r.append(el("span",null,note),el("span"));
    card.appendChild(r);
  }
  body.appendChild(card);
}
