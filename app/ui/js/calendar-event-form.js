// Lazy calendar event create/edit form. It is loaded only from a Day/Event
// popup and uses the shared OSK contract rather than browser-native dialogs.
function calendarWritebackLocalDate(date){
  const d=new Date(date);return `${d.getFullYear()}-${String(d.getMonth()+1).padStart(2,"0")}-${String(d.getDate()).padStart(2,"0")}`;
}
function calendarWritebackLocalTime(date){
  const d=new Date(date);return `${String(d.getHours()).padStart(2,"0")}:${String(d.getMinutes()).padStart(2,"0")}`;
}
function calendarWritebackDateTime(date,time){
  const parts=String(date||"").split("-").map(Number),clock=String(time||"").split(":").map(Number);
  if(parts.length!==3||clock.length!==2||parts.some(n=>!Number.isFinite(n))||clock.some(n=>!Number.isFinite(n))||clock[0]<0||clock[0]>23||clock[1]<0||clock[1]>59)return NaN;
  const value=new Date(parts[0],parts[1]-1,parts[2],clock[0],clock[1],0,0);
  // Date() normalizes inputs such as February 31; reject that typo rather
  // than silently placing a household event on a different day.
  if(value.getFullYear()!==parts[0]||value.getMonth()!==parts[1]-1||value.getDate()!==parts[2])return NaN;
  return value.getTime();
}
function calendarWritebackInput(label,value,mode){
  const input=document.createElement("input");input.type="text";input.readOnly=true;input.className="oskfield calendar-writeback-input";
  input.value=value||"";input.placeholder=label;input.dataset.oskMode=mode||"text";
  input.addEventListener("focus",()=>showOSKFor(input));input.addEventListener("click",event=>{event.stopPropagation();showOSKFor(input);});
  return input;
}
function calendarWritebackField(label,input,help){
  const field=el("label","calendar-writeback-field");field.append(el("span","calendar-writeback-label",label),input);
  if(help)field.appendChild(el("span","calendar-writeback-help",help));return field;
}
async function openCalendarEventForm(options){
  options=options||{};const event=options.event||null;
  let status=options.status||null;
  if(!status)status=await calendarWritebackStatus();
  const calendars=calendarWritebackActiveCalendars(status);
  if(!calendars.length){return;}
  const source=event&&(event.cal&&event.cal.url)||calendars[0].source;
  const base=event?new Date(event.start):(options.day?new Date(options.day):new Date());
  const start=new Date(base);if(!event&&!options.day){start.setHours(9,0,0,0);}else if(!event){start.setHours(9,0,0,0);}
  const end=event&&event.end?new Date(event.end):new Date(+start+60*60000);
  const allDay=!!(event&&event.allDay);
  popupOpenTransaction({mode:"calendareventform",title:event?"Edit event":"New event",when:event?"Calendar event":"Add to a writable calendar",loading:"Preparing event form…"},()=>{
    const root=el("form","calendar-writeback-form");root.noValidate=true;
    const calendar=document.createElement("select");calendar.className="calendar-writeback-select";
    for(const item of calendars){const option=document.createElement("option");option.value=item.source;option.textContent=item.name||item.source;if(item.source===source)option.selected=true;calendar.appendChild(option);}
    // Editing retains ownership in the original remote collection. Moving an
    // event between calendars would be a separate copy/delete operation and
    // is deliberately outside the beta.3 writeback contract.
    if(event)calendar.disabled=true;
    const title=calendarWritebackInput("Event title",event&&event.title||"","text");title.maxLength=300;
    const desc=calendarWritebackInput("Notes (optional)",event&&event.desc||"","text");desc.maxLength=2000;
    const location=calendarWritebackInput("Location (optional)",event&&event.location||"","text");location.maxLength=2000;
    const mode=el("div","calendar-writeback-mode");
    const timed=calendarWritebackButton("At a time","",()=>{setAllDay(false);});
    const allday=calendarWritebackButton("All day","",()=>{setAllDay(true);});mode.append(timed,allday);
    const times=el("div","calendar-writeback-time-grid");
    const startDate=calendarWritebackInput("YYYY-MM-DD",calendarWritebackLocalDate(start),"date"),startTime=calendarWritebackInput("HH:MM",calendarWritebackLocalTime(start),"time");
    const endDate=calendarWritebackInput("YYYY-MM-DD",calendarWritebackLocalDate(end),"date"),endTime=calendarWritebackInput("HH:MM",calendarWritebackLocalTime(end),"time");
    times.append(calendarWritebackField("Start date",startDate),calendarWritebackField("Start time",startTime),calendarWritebackField("End date",endDate),calendarWritebackField("End time",endTime));
    let isAllDay=allDay;
    function setAllDay(value){
      isAllDay=!!value;
      if(isAllDay&&endDate.value<=startDate.value){
        const next=new Date(calendarWritebackDateTime(startDate.value,"00:00")+24*60*60000);
        endDate.value=calendarWritebackLocalDate(next);
      }
      timed.classList.toggle("is-active",!isAllDay);allday.classList.toggle("is-active",isAllDay);startTime.closest(".calendar-writeback-field").hidden=isAllDay;endTime.closest(".calendar-writeback-field").hidden=isAllDay;
    }
    setAllDay(isAllDay);
    const more=document.createElement("details");more.className="calendar-writeback-more";
    const summary=document.createElement("summary");summary.textContent="More details";
    const moreBody=el("div","calendar-writeback-more-body");moreBody.append(calendarWritebackField("Location",location),calendarWritebackField("Notes",desc));
    more.append(summary,moreBody);
    root.append(calendarWritebackField("Calendar",calendar,event?"This edit stays in the original private calendar.":"Only calendars explicitly enabled for Dashboard edits appear here."),calendarWritebackField("Title",title),mode,times,more);
    const note=el("div","calendar-writeback-form-note","Changes appear on Dash-Go immediately. Remote sync follows in the background.");root.appendChild(note);
    const actions=el("div","calendar-writeback-form-actions"),cancel=calendarWritebackButton("Cancel","",()=>{if(event)showEventPopup(event);else closeScrim();}),save=calendarWritebackButton(event?"Save event":"Save event","primary",async button=>{
      const payload={calUrl:calendar.value,title:title.value.trim(),desc:desc.value,location:location.value,allDay:isAllDay};
      if(event)payload.uid=event.uid;
      if(isAllDay){payload.startDate=startDate.value.trim();payload.endDate=endDate.value.trim();}
      else {payload.startMs=calendarWritebackDateTime(startDate.value,startTime.value);payload.endMs=calendarWritebackDateTime(endDate.value,endTime.value);}
      button.disabled=true;
      try{
        const result=await calendarWritebackRequest(event?"/api/calendar/event/update":"/api/calendar/event/create",payload);
        if(result.warning)calendarWritebackShowError(root,result.warning);
        await calendarWritebackRefresh();closeScrim();
      }
      catch(error){button.disabled=false;calendarWritebackShowError(root,error.message);}
    });actions.append(cancel,save);root.appendChild(actions);
    title._oskSubmit=()=>save.click();return root;
  });
}
