/* =====================================================================
   ============================  COMPLIMENTS  ==========================
   Time-of-day buckets, weighted random selection, and date/holiday
   specials. A compliment is eligible if its time bucket matches now,
   its date matches today (if set), and its holiday flag is satisfied.
   Among eligible ones, selection is weighted (higher weight = more
   likely). Specials (birthdays/holidays) use high weights so they
   dominate their day without fully crowding out the general pool.
   ===================================================================== */
function timeBucket(d){
  const h=(d||new Date()).getHours();
  if(h<4)  return "latenight";    // 00:00–03:59 (late night runs until 4am)
  if(h<8)  return "earlymorning"; // 04:00–07:59
  if(h<12) return "morning";      // 08:00–11:59
  if(h<17) return "afternoon";    // 12:00–16:59
  if(h<21) return "evening";      // 17:00–20:59
  return "night";                 // 21:00–23:59
}
// Today's holiday name from loaded calendar events tagged "holiday", if any.
function todaysHolidayName(){
  if(!Array.isArray(EVENTS)) return null;
  const today=new Date(); today.setHours(0,0,0,0);
  for(const ev of EVENTS){
    const isHol = (ev.cal && ev.cal.tag==="holiday") || /holiday/i.test((ev.cal&&ev.cal.name)||"");
    if(!isHol) continue;
    const s=new Date(ev.start); s.setHours(0,0,0,0);
    if(+s===+today && ev.title) return ev.title.trim();
  }
  return null;
}
function messageNormKey(text){ return String(text||"").trim().replace(/\s+/g," ").toLowerCase(); }
function messageKind(c){
  if(c && c.temporary) return "temporary";
  if(c && c.scheduled) return "scheduled";
  if(c && c._bday) return "birthday";
  if(c && c.origin==="custom") return "custom";
  if(c && c.origin==="default") return "default";
  if(c && c.source) return "feed";
  return "default";
}
function messageCategory(c){
  const kind=messageKind(c);
  if(kind==="feed") return "feed:"+String((c&&c.source)||"feed").toLowerCase();
  return kind;
}
function messageIdentity(c,text){
  const kind=messageKind(c), norm=messageNormKey(text);
  if((kind==="feed" || kind==="custom" || kind==="temporary" || kind==="scheduled") && c && c.id!=null)
    return kind+":"+String(c.id);
  if(kind==="default") return "default:"+norm;
  return kind+":"+norm;
}
function eligibleMessage(c,text,weight){
  const kind=messageKind(c), category=messageCategory(c);
  return {
    text,
    weight: weight || 1,
    share: (typeof (c&&c.share)==="number" ? c.share : null),
    kind,
    category,
    key: messageIdentity(c,text),
    id: c && c.id!=null ? c.id : null,
    source: c && c.source ? String(c.source) : "",
    defaultKey: kind==="default" ? messageNormKey(text) : "",
    priority: kind==="temporary" || kind==="scheduled" || kind==="birthday"
  };
}
// Build the list of eligible compliments for right now, with resolved text.
function eligibleCompliments(){
  const bucket=timeBucket();
  const now=new Date();
  const md=String(now.getMonth()+1).padStart(2,"0")+"-"+String(now.getDate()).padStart(2,"0");
  const holiday=todaysHolidayName();
  const out=[];
  for(const c of (COMP_LIST||CONFIG.compliments)){
    if(!c || !c.text) continue;
    if(c.when && !c.when.includes(bucket)) continue;   // wrong time of day
    if(c.date && c.date!==md) continue;                // not its date
    if(c.holiday && !holiday) continue;                // no holiday today
    let text=String(c.text||"");
    if(text.includes("%holiday%")){
      if(!holiday) continue;
      text=text.replace(/%holiday%/g, holiday);
    }
    out.push(eligibleMessage(c, text, c.weight||1));
  }
  // Resolve any `share` entries: set the entry's weight so it is ~share of the
  // pool's total weight. We account for the picker softening the last-shown
  // entry by factor s (0.15): a dominant entry is the last-shown most of the
  // time, so its *effective* contribution is reduced.
  const shareEntries=out.filter(c=>c.share!=null);
  if(shareEntries.length){
    const fixedSum=out.filter(c=>c.share==null).reduce((s,c)=>s+c.weight,0) || 1;
    for(const c of shareEntries){
      const f=Math.min(0.9, Math.max(0.01, c.share));
      c.weight = 1.55 * f*fixedSum/(1-f);
    }
  }
  for(const c of out){ delete c.share; }
  // Fallback: if somehow nothing matched, allow the anytime-untagged ones.
  if(!out.length){
    for(const c of CONFIG.compliments){
      if(c && c.text && !c.when && !c.date && !c.holiday && !String(c.text).includes("%holiday%"))
        out.push(eligibleMessage(c, c.text, c.weight||1));
    }
  }
  return out;
}
// Weighted random pick with a small persisted recent-history memory. This
// keeps the kiosk feeling random without letting a small feed cache repeat the
// same quote/fact/word too often.
let _lastCompText=null;
let _lastCompItem=null;
let _compFadeTimer=null;
let _compRotateTimer=null;
const _messageRotationPauses=new Set();
const _messageRotationPauseTimers=new Map();
const MESSAGE_ROTATION_STORE="dashboard:messageRotation:v1";
const MESSAGE_CATEGORY_PREF_STORE="dashboard:messageCategoryPrefs:v1";
const MESSAGE_RECENT_LIMIT=50;
const MESSAGE_EXACT_COOLDOWN=24;
function _safeJsonLoad(key,fallback){
  try{ const raw=localStorage.getItem(key); return raw?JSON.parse(raw):fallback; }catch(_){ return fallback; }
}
function _safeJsonSave(key,val){ try{ localStorage.setItem(key,JSON.stringify(val)); }catch(_){} }
let _MESSAGE_HISTORY_CACHE=null;
let _MESSAGE_PREFS_CACHE=null;
let _MESSAGE_HISTORY_SAVE_TIMER=null;
let _MESSAGE_PREFS_SAVE_TIMER=null;
function normalizeMessageHistory(h){
  h=(h&&typeof h==="object")?h:{recent:[]};
  h.recent=Array.isArray(h.recent)?h.recent.filter(x=>x&&x.key&&x.category).slice(-MESSAGE_RECENT_LIMIT):[];
  return h;
}
function normalizeMessagePrefs(p){
  p=(p&&typeof p==="object")?p:{less:{}};
  p.less=(p.less&&typeof p.less==="object")?p.less:{};
  return p;
}
function messageRotationHistory(){
  if(_MESSAGE_HISTORY_CACHE) return _MESSAGE_HISTORY_CACHE;
  _MESSAGE_HISTORY_CACHE=normalizeMessageHistory(_safeJsonLoad(MESSAGE_ROTATION_STORE,{recent:[]}));
  return _MESSAGE_HISTORY_CACHE;
}
function flushMessageHistorySave(){
  if(_MESSAGE_HISTORY_SAVE_TIMER){ clearTimeout(_MESSAGE_HISTORY_SAVE_TIMER); _MESSAGE_HISTORY_SAVE_TIMER=null; }
  if(_MESSAGE_HISTORY_CACHE) _safeJsonSave(MESSAGE_ROTATION_STORE,_MESSAGE_HISTORY_CACHE);
}
function scheduleMessageHistorySave(){
  if(_MESSAGE_HISTORY_SAVE_TIMER) clearTimeout(_MESSAGE_HISTORY_SAVE_TIMER);
  _MESSAGE_HISTORY_SAVE_TIMER=setTimeout(flushMessageHistorySave,1200);
}
function saveMessageRotationHistory(h){
  _MESSAGE_HISTORY_CACHE=normalizeMessageHistory(h);
  scheduleMessageHistorySave();
}
function messageCategoryPrefs(){
  if(_MESSAGE_PREFS_CACHE) return _MESSAGE_PREFS_CACHE;
  _MESSAGE_PREFS_CACHE=normalizeMessagePrefs(_safeJsonLoad(MESSAGE_CATEGORY_PREF_STORE,{less:{}}));
  return _MESSAGE_PREFS_CACHE;
}
function flushMessagePrefsSave(){
  if(_MESSAGE_PREFS_SAVE_TIMER){ clearTimeout(_MESSAGE_PREFS_SAVE_TIMER); _MESSAGE_PREFS_SAVE_TIMER=null; }
  if(_MESSAGE_PREFS_CACHE) _safeJsonSave(MESSAGE_CATEGORY_PREF_STORE,_MESSAGE_PREFS_CACHE);
}
function scheduleMessagePrefsSave(){
  if(_MESSAGE_PREFS_SAVE_TIMER) clearTimeout(_MESSAGE_PREFS_SAVE_TIMER);
  _MESSAGE_PREFS_SAVE_TIMER=setTimeout(flushMessagePrefsSave,1200);
}
function saveMessageCategoryPrefs(p){
  _MESSAGE_PREFS_CACHE=normalizeMessagePrefs(p);
  scheduleMessagePrefsSave();
}
if(typeof window!=="undefined"){
  window.addEventListener("pagehide",()=>{ flushMessageHistorySave(); flushMessagePrefsSave(); });
  document.addEventListener("visibilitychange",()=>{ if(document.hidden){ flushMessageHistorySave(); flushMessagePrefsSave(); } });
}
function showFewerMessagesLike(item){
  if(!item || !item.category) return;
  const p=messageCategoryPrefs();
  const cur=Number.isFinite(+p.less[item.category]) ? +p.less[item.category] : 1;
  p.less[item.category]=Math.max(0.25, Math.min(1, cur*0.65));
  saveMessageCategoryPrefs(p);
}
function recordMessagePick(item){
  if(!item) return;
  const h=messageRotationHistory();
  h.recent.push({key:item.key,category:item.category,kind:item.kind,ts:Date.now()});
  saveMessageRotationHistory(h);
}
function fairMessageCandidates(pool){
  let candidates=pool.slice();
  const h=messageRotationHistory();
  const recent=h.recent||[];
  if(candidates.length>1){
    const recentKeys=new Set(recent.slice(-MESSAGE_EXACT_COOLDOWN).map(x=>x.key));
    const noRecent=candidates.filter(c=>!recentKeys.has(c.key));
    if(noRecent.length>=Math.min(3,candidates.length) || (candidates.length<=4 && noRecent.length)) candidates=noRecent;
  }
  const last2=recent.slice(-2);
  if(last2.length===2 && last2[0].category===last2[1].category){
    const cat=last2[0].category;
    const alt=candidates.filter(c=>c.priority || c.category!==cat);
    if(alt.length) candidates=alt;
  }
  return candidates.length ? candidates : pool;
}
function adjustedMessagePool(pool){
  const h=messageRotationHistory();
  const recent=h.recent||[];
  const prefs=messageCategoryPrefs();
  const lastCat=recent.length ? recent[recent.length-1].category : "";
  const last8=recent.slice(-8);
  const feedHeavy=last8.filter(x=>String(x.category||"").startsWith("feed:")).length>=5;
  const hasDefaultish=pool.some(c=>c.kind==="default" || c.kind==="custom");
  return pool.map(c=>{
    let weight=Math.max(0.01,+c.weight||1);
    if(c.key===(_lastCompItem&&_lastCompItem.key)) weight*=0.05;
    if(c.category===lastCat && !c.priority) weight*=0.60;
    if(prefs.less[c.category]) weight*=Math.max(0.25,Math.min(1,+prefs.less[c.category]||1));
    if(feedHeavy && hasDefaultish && String(c.category).startsWith("feed:")) weight*=0.42;
    if(feedHeavy && (c.kind==="default" || c.kind==="custom")) weight*=1.85;
    return {...c,weight};
  });
}
function pauseComplimentMotion(){
  if(_compFadeTimer){ clearTimeout(_compFadeTimer); _compFadeTimer=null; }
  const t=$("#comptext");
  if(t) t.style.opacity=1;
}
function messagePauseActive(){ return _messageRotationPauses.size>0; }
function acquireMessageRotationPause(reason, ttlMs){
  const key=String(reason||"message-pause");
  _messageRotationPauses.add(key);
  if(_messageRotationPauseTimers.has(key)){ clearTimeout(_messageRotationPauseTimers.get(key)); _messageRotationPauseTimers.delete(key); }
  if(Number.isFinite(+ttlMs) && +ttlMs>0){
    _messageRotationPauseTimers.set(key,setTimeout(()=>{
      _messageRotationPauseTimers.delete(key);
      releaseMessageRotationPause(key,true,"ttl");
    },+ttlMs));
  }
  if(_compRotateTimer){ clearTimeout(_compRotateTimer); _compRotateTimer=null; }
  pauseComplimentMotion();
  return key;
}
function releaseMessageRotationPause(reason, reschedule=true, why="release"){
  const key=String(reason||"message-pause");
  if(_messageRotationPauseTimers.has(key)){ clearTimeout(_messageRotationPauseTimers.get(key)); _messageRotationPauseTimers.delete(key); }
  _messageRotationPauses.delete(key);
  try{ if(localStorage.getItem("dashboard:messageLongPressDebug")==="1") console.log("message-rotation-pause",why,{key,remaining:[..._messageRotationPauses]}); }catch(_){}
  if(reschedule && !complimentsPaused() && !_compRotateTimer) scheduleNextCompliment(_lastCompText||"",3000);
}
function complimentsPaused(){
  return messagePauseActive() || (CONFIG.pauseWhileOpen!==false && typeof uiOverlayActive === "function" && uiOverlayActive());
}
