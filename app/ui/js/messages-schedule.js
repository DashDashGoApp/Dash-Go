// Editable message list: when compliments.json (managed from the control
// overlay) has entries, it REPLACES the built-in list — except birthday
// messages, which always come from the installer's birthday list. When the
// file is empty/missing, the built-ins in CONFIG remain the source.
let COMP_LIST=null;
function activeTempMessages(items){
  const now=Date.now();
  return (Array.isArray(items)?items:[]).filter(m=>+m.expiresAt>now && m.text).map(m=>({
    text:m.text, weight:+m.weight||500, temporary:true, id:m.id
  }));
}
function _schedDate(dateStr,timeStr){
  const m=String(dateStr||"").match(/^(\d{4})-(\d{2})-(\d{2})$/);
  const t=String(timeStr||"00:00").match(/^(\d{1,2}):(\d{2})$/);
  if(!m||!t) return null;
  return new Date(+m[1], +m[2]-1, +m[3], +t[1], +t[2], 0, 0);
}
function _dateOnlyMs(d){ return new Date(d.getFullYear(),d.getMonth(),d.getDate()).getTime(); }
function _minutesOfDay(d){ return d.getHours()*60+d.getMinutes(); }
function _sameMonthDay(a,b){ return a && b && a.getDate()===b.getDate(); }
function _sameYearDay(a,b){ return a && b && a.getMonth()===b.getMonth() && a.getDate()===b.getDate(); }
function _insideTimeWindow(now,startTime,endTime){
  const sm=String(startTime||"00:00").split(":").map(Number);
  const em=String(endTime||"23:59").split(":").map(Number);
  if(sm.length<2||em.length<2||sm.some(isNaN)||em.some(isNaN)) return false;
  const s=sm[0]*60+sm[1], e=em[0]*60+em[1], n=_minutesOfDay(now);
  return e>=s ? (n>=s && n<=e) : (n>=s || n<=e);
}
function activeScheduledMessages(items){
  const now=new Date(), today=_dateOnlyMs(now), out=[];
  for(const m of Array.isArray(items)?items:[]){
    if(!m || !m.text) continue;
    const start=_schedDate(m.startDate,m.startTime); if(!start) continue;
    const startDay=_dateOnlyMs(start); if(today<startDay) continue;
    const rec=String(m.recurrence||"once");
    if(rec==="once"){
      let end=_schedDate(m.endDate||m.startDate,m.endTime||"23:59");
      if(!end) continue;
      if(end<start) end=new Date(end.getTime()+86400000);
      if(now<start || now>end) continue;
    }else{
      const stop=m.endDate ? _schedDate(m.endDate,"23:59") : null;
      if(stop && now>stop) continue;
      if(!_insideTimeWindow(now,m.startTime,m.endTime)) continue;
      if(rec==="weekly" || rec==="biweekly" || rec==="xweeks"){
        const days=Array.isArray(m.days)&&m.days.length ? m.days.map(Number) : [start.getDay()];
        if(!days.includes(now.getDay())) continue;
        const weeks=Math.floor((today-startDay)/(86400000*7));
        if(rec==="biweekly" && weeks%2!==0) continue;
        if(rec==="xweeks"){
          const interval=Math.max(3,Math.min(4,+m.intervalWeeks||3));
          if(weeks<0 || weeks%interval!==0) continue;
        }
      }else if(rec==="monthly"){
        if(!_sameMonthDay(now,start)) continue;
      }else if(rec==="xmonths"){
        if(!_sameMonthDay(now,start)) continue;
        const diff=(now.getFullYear()-start.getFullYear())*12+(now.getMonth()-start.getMonth());
        const interval=Math.max(2,Math.min(11,+m.intervalMonths||2));
        if(diff<0 || diff%interval!==0) continue;
      }else if(rec==="yearly"){
        if(!_sameYearDay(now,start)) continue;
      }
    }
    out.push({text:m.text, weight:+m.weight||25, scheduled:true, id:m.id});
  }
  return out;
}
function messageWordCount(text){
  const words=String(text||"").trim().match(/[A-Za-z0-9]+(?:['’][A-Za-z0-9]+)?/g);
  return words ? words.length : 0;
}
function complimentDelayMs(text){
  const minSec=Math.max(3,+CONFIG.complimentSeconds||12);
  const threshold=Math.max(0,+CONFIG.complimentWordThreshold||10);
  const stepWords=Math.max(1,+CONFIG.complimentExtraWordStep||8);
  const stepSec=Math.max(0,+CONFIG.complimentExtraSecondsPerStep||2);
  const maxSec=Math.max(minSec,+CONFIG.complimentMaxSeconds||45);
  const extraWords=Math.max(0,messageWordCount(text)-threshold);
  const extra=extraWords ? Math.ceil(extraWords/stepWords)*stepSec : 0;
  return Math.min(maxSec,minSec+extra)*1000;
}
function legacyNsfwStatusText(text){
  const s=String(text||"").trim().toLowerCase();
  return /^nsfw:\s*(adult humor source enabled|this source is intentionally labeled|adult joke fallback is local|adult humor stays labeled)/.test(s);
}
function cleanFeedText(item){
  if(!item || !item.text) return null;
  if(item.nsfw && legacyNsfwStatusText(item.text)) return null;
  let text=String(item.text||"").trim();
  if(item.nsfw) text=text.replace(/^nsfw:\s*/i,"").trim();
  return text ? {...item,text} : null;
}

async function fetchLocalJson(path, fallback){
  try{
    const res=await fetch(path+"?t="+Date.now(),{cache:"no-store"});
    if(!res.ok) return fallback;
    return await res.json();
  }catch(_){ return fallback; }
}
async function loadCompliments(){
  const [payload,feed,temp,sched]=await Promise.all([
    fetchLocalJson("config/compliments.json",{messages:[],defaultsCleared:false,defaultsSeeded:false,removedDefaults:[],defaultEdits:{},version:4}),
    fetchLocalJson("config/message-cache.json",{items:[]}),
    fetchLocalJson("config/temp-messages.json",[]),
    fetchLocalJson("config/scheduled-messages.json",[])
  ]);
  let msgs=[];
  let cleared=false;
  let removedDefaults=[];
  let defaultEdits={};
  if(payload && typeof payload==="object"){
    msgs=Array.isArray(payload.messages)?payload.messages:[];
    cleared=payload.defaultsCleared===true;
    removedDefaults=Array.isArray(payload.removedDefaults)?payload.removedDefaults:[];
    defaultEdits=(payload.defaultEdits && typeof payload.defaultEdits==="object")?payload.defaultEdits:{};
  }
  const defaultSet=new Set((CONFIG.compliments||[]).filter(c=>c&&!c._bday&&c.text).map(c=>String(c.text).trim().replace(/\s+/g," ").toLowerCase()));
  const customs=msgs.filter(m=>m && m.origin==="custom" && !defaultSet.has(String(m.text||"").trim().replace(/\s+/g," ").toLowerCase()));
  const removed=new Set(removedDefaults.map(x=>String(x||"").trim().toLowerCase()).filter(Boolean));
  let defaults=[];
  if(!cleared){
    defaults=(CONFIG.compliments||[]).filter(c=>c&&!c._bday&&c.text).filter(c=>!removed.has(String(c.text).trim().replace(/\s+/g," ").toLowerCase())).map(c=>{
      const key=String(c.text).trim().replace(/\s+/g," ").toLowerCase();
      return {...c, ...(defaultEdits[key]||{}), origin:"default"};
    });
  }
  const base=[...customs,...defaults];
  const sourceTime=Number(feed&&(feed.lastSuccessAt||feed.generatedAt)||0);
  if(sourceTime>0) lastMessageOK=typeof normalizeEpochMs==="function"?normalizeEpochMs(sourceTime):(sourceTime>20000000000?sourceTime:sourceTime*1000);
  const feedItems=Array.isArray(feed.items) ? feed.items.map(x=>cleanFeedText({
    id:x.id, text:x.text, weight:+x.weight||1, source:x.source||"feed", nsfw:!!x.nsfw
  })).filter(Boolean) : [];
  const tmpItems=activeTempMessages(temp);
  const schedItems=activeScheduledMessages(sched);
  const bdays=CONFIG.compliments.filter(c=>c._bday);
  COMP_LIST=[...tmpItems, ...schedItems, ...base, ...feedItems, ...bdays];
  if(typeof updateStale==="function") updateStale();
}
function pickCompliment(){
  const pool=eligibleCompliments();
  if(!pool.length){ _lastCompItem=null; _lastCompText=""; return ""; }
  let candidates=fairMessageCandidates(pool);
  candidates=adjustedMessagePool(candidates);
  const total=candidates.reduce((s,c)=>s+Math.max(0.01,+c.weight||1),0);
  let chosen=candidates[candidates.length-1], r=Math.random()*total;
  for(const c of candidates){ r-=Math.max(0.01,+c.weight||1); if(r<=0){ chosen=c; break; } }
  _lastCompItem=chosen; _lastCompText=chosen.text; recordMessagePick(chosen); return chosen.text;
}
function forceNextCompliment(){
  const t=$("#comptext");
  if(!t) return;
  if(_compFadeTimer){ clearTimeout(_compFadeTimer); _compFadeTimer=null; }
  if(_compRotateTimer){ clearTimeout(_compRotateTimer); _compRotateTimer=null; }
  const text=pickCompliment();
  t.style.opacity=1;
  fitCompliment(text);
  scheduleNextCompliment(text);
}
