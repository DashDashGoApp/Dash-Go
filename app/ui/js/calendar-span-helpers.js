// 04-calendar.js — generated from dashboard.js for maintainability.
/* =====================================================================
   ============================  CALENDAR RENDER  ======================
   ===================================================================== */
function eventsOnDay(day){
  // O(1) lookup against the prebuilt index (see rebuildDayIndex), instead of
  // filtering the whole EVENTS array once per day cell.
  return DAY_INDEX.get(localDateKey(startOfDay(day)))||[];
}
function classify(ev){
  if(ev.cal.tag==="holiday") return "holiday";
  if(ev.cal.tag==="birthday") return "birthday";
  if(CONFIG.payMatch.test(ev.title||"")) return "pay";
  return "";
}


// App-owned calendar events stay itemized in Agenda and day views, but the
// month grid intentionally groups the compact household feeds. Ownership comes
// from explicit cache/ICS metadata, never from a display title.
const DASH_APP_CALENDAR_GROUPS=Object.freeze({
  // Household app feeds deliberately stay after ordinary events in constrained
  // month cells. Agenda uses the same explicit owner policy in the opposite
  // direction so these actionable household rows are easy to find there.
  "chore-wheel":Object.freeze({owner:"chore-wheel",label:"Chores",action:"Open Chore Wheel",calendarRank:10,agendaRank:10}),
  "maintenance":Object.freeze({owner:"maintenance",label:"Maintenance",action:"Open Maintenance Tracker",calendarRank:20,agendaRank:20}),
  "routines":Object.freeze({owner:"routines",label:"Routines",action:"Open Routines",calendarRank:30,agendaRank:30})
});
function appCalendarOwner(ev){
  const owner=String((ev&&ev.appOwner)||((ev&&ev.cal&&ev.cal.owner)||"")).trim().toLowerCase();
  return DASH_APP_CALENDAR_GROUPS[owner]?owner:"";
}
function appCalendarGroupInfo(owner){ return DASH_APP_CALENDAR_GROUPS[String(owner||"").toLowerCase()]||null; }
function calendarCellDisplayRows(events){
  const normal=[],groups=new Map();
  for(const ev of (events||[])){
    const owner=appCalendarOwner(ev),info=appCalendarGroupInfo(owner);
    if(!info){normal.push({kind:"event",event:ev});continue;}
    let group=groups.get(owner);
    if(!group){group={kind:"app-group",owner,label:info.label,action:info.action,rank:Number(info.calendarRank)||999,color:(ev.cal&&ev.cal.color)||"var(--accent)",events:[]};groups.set(owner,group);}
    group.events.push(ev);
  }
  const grouped=[...groups.values()].sort((a,b)=>a.rank-b.rank || a.owner.localeCompare(b.owner));
  return normal.concat(grouped);
}
function appCalendarGroupTitle(group){
  const count=(group&&group.events&&group.events.length)||0;
  return `${group&&group.label||"Household"} · ${count}`;
}
function localDateKey(d){
  return d.getFullYear()+"-"+String(d.getMonth()+1).padStart(2,"0")+"-"+String(d.getDate()).padStart(2,"0");
}
function wxForDay(day){
  if(!WX||!WX.daily) return null;
  // NOTE: must be the LOCAL date — toISOString() shifts to UTC and returns the
  // wrong day for any timezone east of Greenwich.
  const key=localDateKey(day);
  const i=weatherDailyIndexFor(key);
  if(i<0) return null;
  return { hi:Math.round(WX.daily.temperature_2m_max[i]), lo:Math.round(WX.daily.temperature_2m_min[i]) };
}
function renderCalHead(){
  const head=$("#calhead"); head.innerHTML="";
  const base=startOfWeek(new Date());
  const fmt=FMT.weekday;
  const abbr=new Intl.DateTimeFormat(LOCALE,{weekday:"short"});
  for(let i=0;i<7;i++){
    const d=new Date(+base+i*DAY);
    const dow=d.getDay();
    const div=el("div",dow===6?"dow-sat":dow===0?"dow-sun":"");
    div.appendChild(el("span","dow-full",fmt.format(d)));
    div.appendChild(el("span","dow-abbr",abbr.format(d)));
    head.appendChild(div);
  }
}
/* ---- Multi-day span layout -------------------------------------------
   Events covering >=2 calendar days render as continuous bars across the
   week row instead of repeating in each cell. Overlapping spans stack into
   lanes (max 3 per week); if a week would need a 4th lane, the crowded-out
   event is demoted back to per-cell rendering EVERYWHERE so it never half-
   disappears. Returns {weeks:[{items,laneCount}], spanSet}. */
const SPAN_LANE_CAP=3;
function spanRange(ev){
  const sDay=startOfDay(ev.start);
  let last;
  if(ev.allDay){
    if(ev.end){
      const endDay=startOfDay(ev.end);
      last=(+ev.end===+endDay) ? addDays(endDay,-1) : endDay;  // DTEND exclusive
      if(last<sDay) last=sDay;
    } else last=sDay;
  } else {
    const e=ev.end||ev.start;
    last=startOfDay(new Date(+e-1));
    if(last<sDay) last=sDay;
  }
  return { d0:sDay, d1:last, days:Math.round((last-sDay)/DAY)+1 };
}
function buildSpanLayout(firstWeek,totalWeeks){
  const cand=[];
  for(const ev of EVENTS){
    const r=spanRange(ev);
    if(r.days>=2) cand.push({ev,d0:r.d0,d1:r.d1});
  }
  cand.sort((a,b)=>a.d0-b.d0 || b.d1-a.d1);
  const spanWeekPriority=(a,b)=>{
    // Keep the traditional continuous spanbar rendering, but pack longer /
    // later-reaching spans into lower visual lanes first. This reduces the
    // blank shelf caused when a short early overlap forces a longer span to
    // live in lane 1 for several later days. Non-overlapping spans still share
    // the same lane, so this is a packing heuristic rather than the per-day
    // compaction tried in beta.43.
    const ad=(a.ce-a.cs+1), bd=(b.ce-b.cs+1);
    if(bd!==ad) return bd-ad;
    if(b.ce!==a.ce) return b.ce-a.ce;
    if(a.cs!==b.cs) return a.cs-b.cs;
    return a.c.d0-b.c.d0 || b.c.d1-a.c.d1;
  };
  const demoted=new Set();
  let weeks=[];
  for(let pass=0; pass<2; pass++){
    weeks=[];
    let newDemotion=false;
    for(let w=0; w<totalWeeks; w++){
      const ws=addDays(firstWeek,w*7);
      const items=[]; const lanes=[];
      const weekCands=[];
      for(const c of cand){
        if(demoted.has(c.ev)) continue;
        const c0=Math.round((c.d0-ws)/DAY), c1=Math.round((c.d1-ws)/DAY);
        if(c1<0||c0>6) continue;                 // doesn't touch this week
        weekCands.push({c,c0,c1,cs:Math.max(0,c0),ce:Math.min(6,c1)});
      }
      weekCands.sort(spanWeekPriority);
      for(const wc of weekCands){
        const {c,c0,c1,cs,ce}=wc;
        let lane=0;
        while(lane<lanes.length && lanes[lane].some(([a,b])=>!(ce<a||cs>b))) lane++;
        if(lane>=SPAN_LANE_CAP){ demoted.add(c.ev); newDemotion=true; continue; }
        (lanes[lane]=lanes[lane]||[]).push([cs,ce]);
        items.push({ev:c.ev,c0:cs,c1:ce,lane,contL:c0<0,contR:c1>6});
      }
      items.sort((a,b)=>a.lane-b.lane || a.c0-b.c0 || a.c1-b.c1);
      weeks.push({items,laneCount:lanes.length});
    }
    if(!newDemotion) break;   // pass 2 only needed to rebuild without demotions
  }
  const spanSet=new Set(cand.filter(c=>!demoted.has(c.ev)).map(c=>c.ev));
  return {weeks,spanSet};
}
