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
    return { date:new Date(+y,+mo-1,+d), allDay:true };
  }
  if(z){ return { date:new Date(Date.UTC(+y,+mo-1,+d,+h,+mi,+s)), allDay:false }; }
  return { date:new Date(+y,+mo-1,+d,+h,+mi,+s), allDay:false };
}
function parseICS(text, cal){
  const out=[];
  const lines=unfold(text).split("\n");
  let cur=null;
  for(const line of lines){
    if(line==="BEGIN:VEVENT"){ cur={cal}; continue; }
    if(line==="END:VEVENT"){ if(cur&&cur.start) out.push(cur); cur=null; continue; }
    if(!cur) continue;
    const ci=line.indexOf(":"); if(ci<0) continue;
    const key=line.slice(0,ci), val=line.slice(ci+1);
    const name=key.split(";")[0];
    if(name==="DTSTART"){ const p=parseICSDate(val); if(p){cur.start=p.date;cur.allDay=p.allDay;} }
    else if(name==="DTEND"){ const p=parseICSDate(val); if(p) cur.end=p.date; }
    else if(name==="SUMMARY") cur.title=icsUnescape(val);
    else if(name==="DESCRIPTION") cur.desc=icsUnescape(val);
    else if(name==="LOCATION") cur.location=icsUnescape(val);
    else if(name==="RRULE") cur.rrule=val;
    else if(name==="UID") cur.uid=val;
    else if(name==="X-DASHGO-APP-OWNER") cur.appOwner=icsUnescape(val).trim();
    else if(name==="EXDATE"){
      // Comma-separated list of excluded occurrence starts.
      (cur.exdates=cur.exdates||[]).push(
        ...val.split(",").map(v=>parseICSDate(v)).filter(Boolean).map(p=>+p.date));
    }
    else if(name==="RECURRENCE-ID"){
      // This VEVENT is an edited single occurrence of a recurring event; it
      // replaces the master's instance at this original start time.
      const p=parseICSDate(val); if(p) cur.recurId=+p.date;
    }
  }
  // Post-process: Apple/others sometimes write all-day events as timed spans
  // from local midnight to local midnight (e.g. 00:00:00 -> next day 00:00:00).
  // Reclassify those as all-day so they render on the correct single day(s)
  // instead of bleeding onto the day the end-midnight touches.
  for(const ev of out){
    if(ev.start && ev.end && !ev.allDay){
      const s=ev.start, e=ev.end;
      const startAtMidnight = (s.getHours()===0 && s.getMinutes()===0 && s.getSeconds()===0);
      const endAtMidnight   = (e.getHours()===0 && e.getMinutes()===0 && e.getSeconds()===0);
      if(startAtMidnight && endAtMidnight && e>s){
        ev.allDay=true;   // eventsOnDay() handles exclusive end correctly for all-day
      }
    }
  }
  // Recurrence exceptions: a master event must NOT emit instances that were
  // either excluded (EXDATE) or replaced by an edited copy (a sibling VEVENT
  // with the same UID and a RECURRENCE-ID). Build each master's skip-set here
  // so expand() can honor it.
  const overridesByUid={};
  for(const ev of out){
    if(ev.recurId && ev.uid) (overridesByUid[ev.uid]=overridesByUid[ev.uid]||[]).push(ev.recurId);
  }
  for(const ev of out){
    if(!ev.rrule) continue;
    const skips=[...(ev.exdates||[]), ...((ev.uid&&overridesByUid[ev.uid])||[])];
    if(skips.length) ev._skip=new Set(skips);
  }
  return out;
}
