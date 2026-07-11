// 02-ics.js — generated from dashboard.js for maintainability.
/* =====================================================================
   ============================  ICS PARSER  ===========================
   Minimal: handles VEVENT, line unfolding, DTSTART/DTEND (date &
   datetime, with or without TZID), SUMMARY, DESCRIPTION, LOCATION,
   and simple daily/weekly RRULE expansion within the visible window.
   Not a full RFC5545 implementation — deliberately small for the Pi.
   ===================================================================== */
function unfold(text){
  // RFC5545 line folding: continuation lines start with space/tab.
  return text.replace(/\r\n/g,"\n").replace(/\n[ \t]/g,"");
}
function icsUnescape(v){
  return (v||"").replace(/\\n/gi,"\n").replace(/\\,/g,",").replace(/\\;/g,";").replace(/\\\\/g,"\\");
}
function parseICSDate(raw){
  // raw like "20260601" or "20260601T160000Z" or "20260601T160000"
  const m = raw.match(/(\d{4})(\d{2})(\d{2})(?:T(\d{2})(\d{2})(\d{2})(Z)?)?/);
  if(!m) return null;
  const [_,y,mo,d,h,mi,s,z] = m;
  if(h===undefined){
    // date-only => all-day (local midnight)
    return { date:new Date(+y,+mo-1,+d), allDay:true, utc:false };
  }
  if(z){ return { date:new Date(Date.UTC(+y,+mo-1,+d,+h,+mi,+s)), allDay:false, utc:true }; }
  return { date:new Date(+y,+mo-1,+d,+h,+mi,+s), allDay:false, utc:false };
}
function parseICSDuration(raw){
  const match=String(raw||"").trim().toUpperCase().match(/^([+-])?P(?:(\d+)W)?(?:(\d+)D)?(?:T(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?)?$/);
  if(!match) return null;
  const days=Number(match[2]||0)*7+Number(match[3]||0);
  const clockMs=((Number(match[4]||0)*60+Number(match[5]||0))*60+Number(match[6]||0))*1000;
  if(!Number.isFinite(days)||!Number.isFinite(clockMs)||(days===0&&clockMs<=0)||match[1]==="-") return null;
  return {days,clockMs};
}
function applyICSDuration(start,duration,utc){
  if(!start||!duration) return null;
  const end=new Date(+start);
  if(duration.days){
    if(utc) end.setUTCDate(end.getUTCDate()+duration.days);
    else end.setDate(end.getDate()+duration.days);
  }
  return new Date(+end+duration.clockMs);
}
function icsRevisionTime(ev){
  return Math.max(Number(ev.lastModified||0),Number(ev.dtstamp||0));
}
function icsRevisionKey(ev){
  if(!ev||!ev.uid) return "";
  if(ev.recurId!==null&&ev.recurId!==undefined) return ev.uid+"|"+String(ev.recurId);
  return ev.uid+"|master";
}
function reconcileICSRevisions(events){
  const selected=new Map();
  const out=[];
  for(const ev of events){
    const key=icsRevisionKey(ev);
    if(!key){ out.push(ev); continue; }
    const existingIndex=selected.get(key);
    if(existingIndex===undefined){
      selected.set(key,out.length);
      out.push(ev);
      continue;
    }
    const current=out[existingIndex];
    const newer=(Number(ev.sequence||0)>Number(current.sequence||0))||
      (Number(ev.sequence||0)===Number(current.sequence||0)&&icsRevisionTime(ev)>icsRevisionTime(current));
    if(newer) out[existingIndex]=ev;
  }
  return out;
}
function parseICS(text, cal){
  const parsed=[];
  const lines=unfold(text).split("\n");
  let cur=null;
  for(const line of lines){
    if(line==="BEGIN:VEVENT"){ cur={cal}; continue; }
    if(line==="END:VEVENT"){
      if(cur&&cur.start){
        if(!cur.end&&cur.duration){ cur.end=applyICSDuration(cur.start,cur.duration,cur.startUTC); }
        parsed.push(cur);
      }
      cur=null;
      continue;
    }
    if(!cur) continue;
    const ci=line.indexOf(":"); if(ci<0) continue;
    const key=line.slice(0,ci), val=line.slice(ci+1);
    const name=key.split(";")[0].toUpperCase();
    if(name==="DTSTART"){ const p=parseICSDate(val); if(p){cur.start=p.date;cur.allDay=p.allDay;cur.startUTC=p.utc;} }
    else if(name==="DTEND"){ const p=parseICSDate(val); if(p) cur.end=p.date; }
    else if(name==="DURATION") cur.duration=parseICSDuration(val);
    else if(name==="SUMMARY") cur.title=icsUnescape(val);
    else if(name==="DESCRIPTION") cur.desc=icsUnescape(val);
    else if(name==="LOCATION") cur.location=icsUnescape(val);
    else if(name==="RRULE") cur.rrule=val;
    else if(name==="UID") cur.uid=val;
    else if(name==="STATUS") cur.cancelled=String(val||"").trim().toUpperCase()==="CANCELLED";
    else if(name==="SEQUENCE") cur.sequence=Number.parseInt(val,10)||0;
    else if(name==="LAST-MODIFIED"){
      const p=parseICSDate(val); if(p) cur.lastModified=+p.date;
    }
    else if(name==="DTSTAMP"){
      const p=parseICSDate(val); if(p) cur.dtstamp=+p.date;
    }
    else if(name==="X-DASHGO-APP-OWNER") cur.appOwner=icsUnescape(val).trim();
    else if(name==="X-DASHGO-MANAGED-SCHEDULE"||name==="X-DASHGO-SCHEDULE-RULE-ID"||name==="X-DASHGO-NOMINAL-DATE"||name==="X-DASHGO-SCHEDULE-ACTUAL-DATE"||name==="X-DASHGO-SCHEDULE-REASON"){
      const managed=cur.managedSchedule||(cur.managedSchedule={});
      if(name==="X-DASHGO-MANAGED-SCHEDULE") managed.type=icsUnescape(val).trim();
      else if(name==="X-DASHGO-SCHEDULE-RULE-ID") managed.ruleId=icsUnescape(val).trim();
      else if(name==="X-DASHGO-NOMINAL-DATE") managed.nominalDate=icsUnescape(val).trim();
      else if(name==="X-DASHGO-SCHEDULE-ACTUAL-DATE") managed.actualDate=icsUnescape(val).trim();
      else managed.reason=icsUnescape(val).trim();
    }
    else if(name==="EXDATE"){
      (cur.exdates=cur.exdates||[]).push(
        ...val.split(",").map(v=>parseICSDate(v)).filter(Boolean).map(p=>+p.date));
    }
    else if(name==="RECURRENCE-ID"){
      const p=parseICSDate(val); if(p) cur.recurId=+p.date;
    }
  }
  const events=reconcileICSRevisions(parsed);
  const cancelledMasters=new Set(events.filter(ev=>ev.cancelled&&ev.uid&&(ev.recurId===null||ev.recurId===undefined)).map(ev=>ev.uid));
  // Apple/others sometimes write all-day events as timed spans from local
  // midnight to local midnight. Reclassify those so the exclusive end is
  // interpreted as a day boundary rather than an extra displayed day.
  for(const ev of events){
    if(ev.start&&ev.end&&!ev.allDay){
      const s=ev.start,e=ev.end;
      const startAtMidnight=s.getHours()===0&&s.getMinutes()===0&&s.getSeconds()===0;
      const endAtMidnight=e.getHours()===0&&e.getMinutes()===0&&e.getSeconds()===0;
      if(startAtMidnight&&endAtMidnight&&e>s) ev.allDay=true;
    }
  }
  // Include cancelled overrides in the master's skip-set, then remove the
  // cancelled component itself from display. This mirrors the authoritative
  // Go cache parser during emergency direct-ICS fallback.
  const overridesByUid={};
  for(const ev of events){
    if(ev.recurId!==null&&ev.recurId!==undefined&&ev.uid){
      (overridesByUid[ev.uid]=overridesByUid[ev.uid]||[]).push(ev.recurId);
    }
  }
  for(const ev of events){
    if(!ev.rrule) continue;
    const skips=[...(ev.exdates||[]),...((ev.uid&&overridesByUid[ev.uid])||[])];
    if(skips.length) ev._skip=new Set(skips);
  }
  return events.filter(ev=>!ev.cancelled&&!cancelledMasters.has(ev.uid));
}
