// Expand RRULEs across [winStart,winEnd].
// Supports FREQ=DAILY, WEEKLY (with BYDAY), MONTHLY (BYMONTHDAY or implicit
// day-of-month), and YEARLY (same month/day as DTSTART), with optional
// COUNT/UNTIL/INTERVAL. Honors EXDATE and RECURRENCE-ID overrides via the
// master's _skip set. Other BY* parts fall back to the single instance.
// COUNT is counted in occurrences from DTSTART (iCal semantics).
const ICAL_DOW={SU:0,MO:1,TU:2,WE:3,TH:4,FR:5,SA:6};
function expand(ev, winStart, winEnd){
  if(!ev.rrule) return [ev];
  const r={}; ev.rrule.split(";").forEach(p=>{const[k,v]=p.split("=");r[k]=v;});
  const freq=r.FREQ;
  if(freq!=="DAILY"&&freq!=="WEEKLY"&&freq!=="MONTHLY"&&freq!=="YEARLY") return [ev];
  const interval=+(r.INTERVAL||1);
  let until=winEnd;
  if(r.UNTIL){ const p=parseICSDate(r.UNTIL); if(p) until=p.date; }
  const count=r.COUNT?+r.COUNT:Infinity;
  // For all-day events, preserve the span in whole DAYS (DST-proof) rather than
  // a fixed millisecond duration, which drifts across spring/fall transitions.
  const durMs=(ev.end?ev.end-ev.start:0);
  const durDays = ev.allDay && ev.end
    ? Math.round((startOfDay(ev.end)-startOfDay(ev.start))/DAY) : 0;
  const s=ev.start;
  // Build an instance whose start is the given Date, recomputing end correctly.
  const makeInst=(d)=>{
    let end;
    if(ev.allDay){
      end = ev.end ? new Date(d.getFullYear(),d.getMonth(),d.getDate()+durDays) : ev.end;
    } else {
      end = durMs ? new Date(+d+durMs) : ev.end;
    }
    return {...ev, start:d, end, _recur:true};
  };
  const res=[];
  const skipped=(d)=> ev._skip && ev._skip.has(+d);
  const push=d=>{ if(d>=winStart && d<=winEnd && !skipped(d)) res.push(makeInst(d)); };

  // Weekly with BYDAY: iterate week by week (stepping `interval` weeks),
  // emitting each listed weekday within the week.
  if(freq==="WEEKLY" && r.BYDAY){
    const days=r.BYDAY.split(",").map(d=>ICAL_DOW[d.replace(/^[+-]?\d+/,"")]).filter(d=>d!==undefined).sort((a,b)=>a-b);
    if(!days.length) return [ev];
    let weekStart=startOfWeek(ev.start);
    let emitted=0, guard=0;
    // Fast-forward whole interval-steps to just before the window when COUNT
    // is unbounded (with COUNT we must walk from the start to count correctly).
    if(count===Infinity && weekStart<winStart){
      const stepMs=interval*7*DAY;
      const steps=Math.floor((startOfWeek(winStart)-weekStart)/stepMs);
      if(steps>0){
        const ws=new Date(weekStart.getFullYear(),weekStart.getMonth(),weekStart.getDate()+steps*interval*7);
        if(ws<=winStart) weekStart=ws;
      }
    }
    while(weekStart<=until && weekStart<=winEnd && emitted<count && guard<400){
      for(const wd of days){
        const offset=(wd-CONFIG.firstDayOfWeek+7)%7;
        const occ=new Date(weekStart.getFullYear(),weekStart.getMonth(),weekStart.getDate()+offset,
                           s.getHours(),s.getMinutes(),s.getSeconds());
        if(occ<ev.start) continue;
        if(occ>until) break;
        if(emitted>=count) break;
        push(occ); emitted++;
      }
      // step whole weeks by date components (DST-proof)
      weekStart=new Date(weekStart.getFullYear(),weekStart.getMonth(),weekStart.getDate()+interval*7);
      guard++;
    }
    return res;
  }

  if(freq==="MONTHLY"||freq==="YEARLY"){
    // MONTHLY: BYMONTHDAY if present, else DTSTART's day-of-month.
    // YEARLY: DTSTART's month + day. Months lacking the day (e.g. the 31st,
    // or Feb 29 off leap years) are skipped, matching common iCal behavior.
    const dom=freq==="MONTHLY" ? +(r.BYMONTHDAY||s.getDate()) : s.getDate();
    const stepMonths=freq==="MONTHLY" ? interval : 12*interval;
    let y=s.getFullYear(), m=s.getMonth();
    let emitted=0, guard=0;
    // Fast-forward in whole steps when COUNT is unbounded.
    if(count===Infinity){
      const monthsAhead=(winStart.getFullYear()-y)*12+(winStart.getMonth()-m);
      if(monthsAhead>stepMonths){
        const steps=Math.floor((monthsAhead-1)/stepMonths);
        m+=steps*stepMonths; y+=Math.floor(m/12); m%=12;
      }
    }
    while(guard<600 && emitted<count){
      const occ=new Date(y,m,dom,s.getHours(),s.getMinutes(),s.getSeconds());
      if(occ>until || occ>winEnd) break;
      // Skip rollovers (e.g. Apr 31 -> May 1) and anything before DTSTART.
      if(occ.getDate()===dom && occ>=ev.start){ push(occ); emitted++; }
      m+=stepMonths; y+=Math.floor(m/12); m%=12;
      guard++;
    }
    return res;
  }

  // Plain DAILY / WEEKLY (no BYDAY) — step by calendar days, not milliseconds.
  const stepDays=freq==="DAILY"?interval:7*interval;
  let t=new Date(s), n=0;
  // Fast-forward to just before the window when COUNT is unbounded, instead of
  // stepping one occurrence at a time from a possibly years-old DTSTART.
  if(count===Infinity && t<winStart){
    const steps=Math.floor((startOfDay(winStart)-startOfDay(t))/(stepDays*DAY));
    if(steps>0){
      t=new Date(t.getFullYear(),t.getMonth(),t.getDate()+steps*stepDays,
                 s.getHours(),s.getMinutes(),s.getSeconds());
    }
  }
  while(t<=until && t<=winEnd && n<count){
    push(t);
    t=new Date(t.getFullYear(),t.getMonth(),t.getDate()+stepDays,
               s.getHours(),s.getMinutes(),s.getSeconds());
    n++;
    if(res.length>500) break;
  }
  return res;
}
