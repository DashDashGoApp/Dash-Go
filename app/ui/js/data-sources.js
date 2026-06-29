/* =====================================================================
   ============================  CALENDAR FETCH  =======================
   ===================================================================== */
// The active calendar list. Starts as the hardcoded CONFIG.calendars (so the
// dashboard works even with no calendars.json), but discoverCalendars() will
// replace it with the auto-generated list when calendars.json is present.
let ACTIVE_CALENDARS = CONFIG.calendars;

// Synthetic yearly birthday events from the installer's birthday list.
const BIRTHDAY_CAL={ name:"Birthdays", color:"#d9c074", tag:"birthday" };
function birthdayEvents(winStart,winEnd){
  if(!CONFIG.showBirthdaysOnCalendar) return [];
  const L=(typeof window!=="undefined")&&window.DASHBOARD_LOCAL;
  const list=(L&&Array.isArray(L.birthdays))?L.birthdays:[];
  const out=[];
  for(let y=winStart.getFullYear(); y<=winEnd.getFullYear(); y++){
    for(const b of list){
      if(!b||!b.name||!b.date) continue;
      const m=String(b.date).match(/^(\d{2})-(\d{2})$/); if(!m) continue;
      let d=new Date(y,+m[1]-1,+m[2]);
      if(d.getMonth()!==+m[1]-1) d=new Date(y,+m[1],0);  // 02-29 -> Feb 28 off leap years
      if(d>=winStart&&d<=winEnd)
        out.push({ cal:BIRTHDAY_CAL, title:b.name+"'s Birthday", allDay:true, start:d });
    }
  }
  return out;
}

// Fetch calendars.json (written by gen-calendars.sh from the .ics filenames).
// If present and valid, it becomes the calendar list — so dropping a new .ics
// in the folder and regenerating is all it takes to add a calendar, with no
// edits here. Falls back silently to CONFIG.calendars if absent/invalid.
async function discoverCalendars(){
  try{
    const res=await fetch("calendars/calendars.json?t="+Date.now(),{cache:"no-store"});
    if(!res.ok) return;
    const list=await res.json();
    // An empty array is a VALID manifest ("no calendars yet") — use it, so we
    // don't fall back to CONFIG defaults that 404 and trip the stale warning.
    if(Array.isArray(list) && list.every(c=>c&&c.url)){
      ACTIVE_CALENDARS=list;
    }
  }catch(err){ /* no manifest -> keep CONFIG.calendars */ }
}
