// Holiday-aware rotating-message eligibility. Calendar events are the source of
// truth: a layer contributes greetings only when its enabled, loaded calendar
// has an event today. No date table or guessed observance is used here.
"use strict";
const HOLIDAY_LAYER_SOURCES=[
  ["jewish-holidays.","jewish"],["islamic-holidays.","islamic"],
  ["christian-holidays.","christian"],["orthodox-holidays.","orthodox"],
  ["hindu-holidays.","hindu"],["holidays.","civil"]
];
function holidayTextKey(value){
  return String(value||"").trim().replace(/[’‘]/g,"'").replace(/[–—]/g,"-").replace(/\s+/g," ").toLowerCase();
}
function holidayLayerForEvent(ev){
  const cal=(ev&&ev.cal&&typeof ev.cal==="object")?ev.cal:{};
  const explicit=holidayTextKey(cal.holidayLayer||cal.layer);
  if(["civil","jewish","islamic","christian","orthodox","hindu","general"].includes(explicit)) return explicit;
  const source=String(cal.url||ev&&ev.calUrl||"").toLowerCase();
  for(const [needle,layer] of HOLIDAY_LAYER_SOURCES) if(source.includes(needle)) return layer;
  return holidayTextKey(cal.tag)==="holiday"||holidayTextKey(cal.name).includes("holiday") ? "general" : "";
}
function holidayEventIsToday(ev,now){
  if(!ev||!ev.title||!holidayLayerForEvent(ev)) return false;
  const start=new Date(ev.start),today=now||new Date();
  return !Number.isNaN(+start)&&start.getFullYear()===today.getFullYear()&&start.getMonth()===today.getMonth()&&start.getDate()===today.getDate();
}
function holidayContextsForEvents(events,now){
  const found=new Map();
  for(const ev of Array.isArray(events)?events:[]){
    if(!holidayEventIsToday(ev,now)) continue;
    const name=String(ev.title||"").trim(),key=holidayTextKey(name),layer=holidayLayerForEvent(ev);
    if(!key) continue;
    const prior=found.get(key)||{name,key,layers:[]};
    if(!prior.layers.includes(layer)) prior.layers.push(layer);
    found.set(key,prior);
  }
  return [...found.values()];
}
function todaysHolidayContexts(now){ return holidayContextsForEvents(typeof EVENTS==="undefined"?[]:EVENTS,now); }
function holidayContextMatches(entry,context){
  const layers=Array.isArray(entry&&entry.holidayLayers)?entry.holidayLayers.map(holidayTextKey).filter(Boolean):[];
  const names=Array.isArray(entry&&entry.holidayNames)?entry.holidayNames.map(holidayTextKey).filter(Boolean):[];
  return (!layers.length||layers.some(layer=>context.layers.includes(layer)))&&(!names.length||names.includes(context.key));
}
function holidayEntryMatches(entry,contexts){
  if(!entry||!entry.holiday) return true;
  if(!contexts.length) return false;
  const overlap=contexts.length>1;
  if(entry.holidayOverlap===true) return overlap;
  if(entry.holidayOverlap===false&&overlap) return false;
  return contexts.some(context=>holidayContextMatches(entry,context));
}
function holidayDisplayName(contexts){
  if(!contexts||!contexts.length) return "";
  if(contexts.length===1) return contexts[0].name;
  const names=contexts.map(context=>context.name);
  return names.length===2 ? names.join(" and ") : names.slice(0,-1).join(", ")+", and "+names[names.length-1];
}
function applyHolidayMessageShare(entries){
  const holiday=entries.filter(item=>item&&item.holiday),ordinary=entries.filter(item=>item&&!item.holiday);
  if(!holiday.length||!ordinary.length) return entries;
  const normalWeight=ordinary.reduce((sum,item)=>sum+Math.max(.01,+item.weight||1),0);
  const holidayWeight=holiday.reduce((sum,item)=>sum+Math.max(.01,+item.weight||1),0);
  if(!(normalWeight>0&&holidayWeight>0)) return entries;
  const target=holiday.some(item=>item.holidayMajor)?0.60:0.40;
  const scale=target*normalWeight/((1-target)*holidayWeight);
  return entries.map(item=>item&&item.holiday?{...item,weight:Math.max(.01,(+item.weight||1)*scale)}:item);
}
