// Calendar write-back status, popup actions, and day-popup Add affordance.
// This module is loaded with the dashboard but performs no network work until a
// user opens an event/day popup or initiates a calendar change.
let CALENDAR_WRITEBACK_STATUS=null;
let CALENDAR_WRITEBACK_STATUS_PROMISE=null;

async function calendarWritebackStatus(){
  // Query only when a Calendar popup asks for it. Do not cache across Control
  // changes: enabling a private collection takes effect on the next tap
  // without a dashboard reload, and this endpoint has no remote I/O.
  if(CALENDAR_WRITEBACK_STATUS_PROMISE)return CALENDAR_WRITEBACK_STATUS_PROMISE;
  CALENDAR_WRITEBACK_STATUS_PROMISE=fetch("/api/calendar/writeback/status",{cache:"no-store"})
    .then(res=>res.ok?res.json():{enabled:false,calendars:[]})
    .catch(()=>({enabled:false,calendars:[]}))
    .then(status=>{CALENDAR_WRITEBACK_STATUS=status||{enabled:false,calendars:[]};return CALENDAR_WRITEBACK_STATUS;})
    .finally(()=>{CALENDAR_WRITEBACK_STATUS_PROMISE=null;});
  return CALENDAR_WRITEBACK_STATUS_PROMISE;
}
function calendarWritebackActiveCalendars(status){
  return status&&status.enabled===true&&Array.isArray(status.calendars)
    ?status.calendars.filter(cal=>cal&&cal.writable===true&&cal.enabled!==false):[];
}
function calendarWritebackButton(label,kind,run){
  const button=el("button","calendar-writeback-btn "+(kind||""),label);button.type="button";
  bindTap(button,event=>{event.preventDefault();event.stopPropagation();run(button);});
  return button;
}
function calendarWritebackRefresh(){
  CALENDAR_WRITEBACK_STATUS=null;
  return Promise.resolve().then(()=>discoverCalendars()).then(()=>loadCalendars());
}
async function calendarWritebackRequest(path,payload){
  const headers={"Content-Type":"application/json","Accept":"application/json"};
  if(typeof CTRL_TOKEN!=="undefined"&&CTRL_TOKEN)headers["X-Dashboard-Token"]=CTRL_TOKEN;
  const res=await fetch(path,{method:"POST",headers,body:JSON.stringify(payload||{})});
  const body=await res.json().catch(()=>({}));
  if(!res.ok)throw new Error(body.error||"Calendar change failed.");
  return body;
}
function calendarWritebackShowError(root,message){
  const note=el("div","calendar-writeback-note error",message||"Calendar change failed.");
  root.appendChild(note);setTimeout(()=>note.remove(),6000);
}
function calendarWritebackEventCapability(ev,status){
  const cached=ev&&ev.writeback;
  if(!cached||cached.candidate!==true)return null;
  const source=String(ev&&ev.cal&&ev.cal.url||ev&&ev.calUrl||"");
  const calendars=Array.isArray(status&&status.calendars)?status.calendars:[];
  const calendar=calendars.find(item=>item&&String(item.source||"")===source);
  if(!calendar||calendar.writable!==true)return null;
  if(status&&status.enabled!==true)return {state:"master-off"};
  if(calendar.enabled===false)return {state:"calendar-off"};
  return {
    state:"ready",
    canEdit:cached.canEdit===true,
    canSkip:cached.canSkip===true,
    canDelete:cached.canEdit===true&&calendar.deleteAllowed===true,
    deleteRequiresPin:cached.canEdit===true&&calendar.deleteAllowed!==true,
  };
}
function calendarWritebackEventActions(ev,token){
  // Eligibility stays in the cache, but the current local writeback registry is
  // checked only when the user opens this popup. That prevents a stale cache
  // record from hiding actions after Calendar Manager changes an edit setting.
  if(!ev||!ev.writeback||ev.writeback.candidate!==true)return null;
  const root=el("section","calendar-writeback-actions");
  root.hidden=true;
  calendarWritebackStatus().then(status=>{
    if(!popupIsCurrent(token)||!root.isConnected)return;
    const cap=calendarWritebackEventCapability(ev,status);
    if(!cap){root.remove();return;}
    root.hidden=false;root.replaceChildren();
    if(cap.state==="master-off"){
      root.appendChild(el("div","calendar-writeback-note","Dashboard calendar edits are off. Turn them on in Calendar Manager to edit this private calendar."));
      return;
    }
    if(cap.state==="calendar-off"){
      root.appendChild(el("div","calendar-writeback-note","Dashboard edits are disabled for this calendar. Enable them in Calendar Manager to change this event."));
      return;
    }
    root.appendChild(el("div","calendar-writeback-note","Dashboard edits save locally first. Remote calendar sync follows."));
    const row=el("div","calendar-writeback-action-row");
    if(cap.canEdit)row.appendChild(calendarWritebackButton("Edit event","",()=>openCalendarEventForm({event:ev})));
    if(cap.canDelete)row.appendChild(calendarWritebackButton("Delete event","danger",()=>calendarWritebackConfirm(ev,"delete",token)));
    if(cap.canSkip)row.appendChild(calendarWritebackButton("Skip this occurrence","",()=>calendarWritebackConfirm(ev,"skip",token)));
    if(row.childNodes.length)root.appendChild(row);
    if(cap.deleteRequiresPin)root.appendChild(el("div","calendar-writeback-note","Set and unlock a Dashboard Control PIN to allow deleting one-time events."));
  }).catch(()=>{if(root.isConnected)root.remove();});
  return root;
}
function calendarWritebackConfirm(ev,action){
  const skipping=action==="skip", title=skipping?"Skip this occurrence?":"Delete this event?";
  const copy=skipping?`Skip ${FMT.popDay.format(ev.start)} only. Future occurrences remain unchanged.`:`Delete “${ev.title||"this event"}” from ${ev.cal&&ev.cal.name||"this calendar"}?`;
  popupOpenTransaction({mode:"calendarconfirm",title,when:"Calendar change",loading:"Preparing confirmation…"},()=>{
    const root=el("div","calendar-writeback-confirm");root.appendChild(el("p","",copy));
    const actions=el("div","calendar-writeback-action-row");
    actions.append(calendarWritebackButton("Keep event","",()=>showEventPopup(ev)),calendarWritebackButton(skipping?"Skip this date":"Delete permanently","danger",async button=>{
      button.disabled=true;
      try{
        const payload={calUrl:ev.cal&&ev.cal.url||ev.calUrl,uid:ev.uid};
        const path=skipping?"/api/calendar/event/skip-occurrence":"/api/calendar/event/delete";
        if(skipping)payload.occurrenceMs=+ev.start;
        const result=await calendarWritebackRequest(path,payload);
        if(result.warning)calendarWritebackShowError(root,result.warning);
        await calendarWritebackRefresh();closeScrim();
      }catch(error){button.disabled=false;calendarWritebackShowError(root,error.message);}
    }));root.appendChild(actions);return root;
  });
}
function calendarWritebackMountDayAdd(root,day,token){
  calendarWritebackStatus().then(status=>{
    if(!popupIsCurrent(token)||!root.isConnected||!calendarWritebackActiveCalendars(status).length)return;
    const add=calendarWritebackButton("+ Add event","primary",()=>openCalendarEventForm({day,status}));
    const row=el("div","calendar-writeback-day-add");row.appendChild(add);root.insertBefore(row,root.firstChild);
  });
}
