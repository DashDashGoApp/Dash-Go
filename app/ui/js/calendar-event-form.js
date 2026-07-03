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
  if(parts.length!==3||clock.length!==2||parts.some(n=>!Number.isFinite(n))||clock[0]<0||clock[0]>23||clock[1]<0||clock[1]>59)return NaN;
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
function calendarWritebackFormScope(scope){
  return ["single","occurrence","series"].includes(scope)?scope:"single";
}
function calendarWritebackFormLabels(event,scope){
  if(!event)return {title:"New event",when:"Add to a writable calendar",save:"Save event",note:"Changes appear on Dash-Go immediately. Remote sync follows in the background."};
  if(scope==="occurrence")return {title:"Edit this occurrence",when:"Recurring calendar event",save:"Save this occurrence",note:"This changes only the selected occurrence. The repeating series stays unchanged; remote sync follows in the background."};
  if(scope==="series")return {title:"Edit entire series",when:"Recurring calendar event",save:"Save entire series",note:"This changes the title, date, time, location, and notes for the series. Its repeat rule stays unchanged; remote sync follows in the background."};
  return {title:"Manage event",when:"Calendar event",save:"Save event",note:"Changes appear on Dash-Go immediately. Remote sync follows in the background."};
}
async function openCalendarEventForm(options){
  options=options||{};const event=options.event||null,scope=calendarWritebackFormScope(options.scope);
  let status=options.status||null;
  if(!status)status=await calendarWritebackStatus();
  const calendars=calendarWritebackActiveCalendars(status);
  if(!calendars.length){return;}
  const source=event&&(event.cal&&event.cal.url||event.calUrl)||calendars[0].source;
  const base=event?new Date(event.start):(options.day?new Date(options.day):new Date());
  const start=new Date(base);if(!event)start.setHours(9,0,0,0);
  const end=event&&event.end?new Date(event.end):new Date(+start+60*60000);
  const isAllDayInitial=!!(event&&event.allDay),labels=calendarWritebackFormLabels(event,scope);
  // Stored all-day ends are exclusive (the midnight after the event), but a
  // household user thinks in inclusive terms: a one-day event ends on its own
  // date. The form always displays the inclusive last day and converts back
  // to the exclusive contract on save.
  const endDisplay=new Date(end);
  if(isAllDayInitial)endDisplay.setDate(endDisplay.getDate()-1);
  function exclusiveEndDate(value){
    const parts=String(value||"").split("-").map(Number);
    if(parts.length!==3||parts.some(n=>!Number.isFinite(n)))return "";
    return calendarWritebackLocalDate(new Date(parts[0],parts[1]-1,parts[2]+1));
  }
  popupOpenTransaction({mode:"calendareventform",title:labels.title,when:labels.when,loading:"Preparing event form…"},()=>{
    const root=el("form","calendar-writeback-form");root.noValidate=true;
    const calendar=document.createElement("select");calendar.className="calendar-writeback-select";
    for(const item of calendars){const option=document.createElement("option");option.value=item.source;option.textContent=item.name||item.source;if(item.source===source)option.selected=true;calendar.appendChild(option);}
    // Editing retains ownership in the original remote collection. Moving an
    // event between calendars would be a separate copy/delete operation and
    // is deliberately outside the private-calendar writeback contract.
    if(event)calendar.disabled=true;
    const title=calendarWritebackInput("Event title",event&&event.title||"","text");title.maxLength=300;
    const desc=calendarWritebackInput("Notes (optional)",event&&event.desc||"","text");desc.maxLength=2000;
    const location=calendarWritebackInput("Location (optional)",event&&event.location||"","text");location.maxLength=2000;
    const mode=el("div","calendar-writeback-mode");
    const timed=calendarWritebackButton("At a time","",()=>{setAllDay(false);});
    const allday=calendarWritebackButton("All day","",()=>{setAllDay(true);});mode.append(timed,allday);
    const times=el("div","calendar-writeback-time-grid");
    const startDate=calendarWritebackInput("YYYY-MM-DD",calendarWritebackLocalDate(start),"date"),startTime=calendarWritebackInput("HH:MM",calendarWritebackLocalTime(start),"time");
    const endDate=calendarWritebackInput("YYYY-MM-DD",calendarWritebackLocalDate(isAllDayInitial?endDisplay:end),"date"),endTime=calendarWritebackInput("HH:MM",calendarWritebackLocalTime(end),"time");
    // Quick-access time changes: the on-screen keyboard is precise but slow
    // for a standing kitchen touch. These chips cover the common adjustments
    // (nudge the start, pick a usual length) while keeping the fields as the
    // single source of truth — every tap rewrites the same four inputs, so
    // save/validation paths are unchanged. Durations preserve the start; start
    // nudges preserve the duration; both carry date rollovers across midnight.
    function calendarWritebackFormTimes(){
      const startMs=calendarWritebackDateTime(startDate.value,startTime.value);
      const endMs=calendarWritebackDateTime(endDate.value,endTime.value);
      if(!Number.isFinite(startMs))return null;
      return {startMs,endMs:Number.isFinite(endMs)?endMs:startMs+60*60000};
    }
    function calendarWritebackWriteTimes(startMs,endMs){
      const startAt=new Date(startMs),endAt=new Date(Math.max(endMs,startMs+5*60000));
      startDate.value=calendarWritebackLocalDate(startAt);startTime.value=calendarWritebackLocalTime(startAt);
      endDate.value=calendarWritebackLocalDate(endAt);endTime.value=calendarWritebackLocalTime(endAt);
    }
    function calendarWritebackQuickChip(label,fn){
      const chip=el("button","calendar-writeback-quick-chip",label);chip.type="button";
      chip.addEventListener("click",()=>{const current=calendarWritebackFormTimes();if(current)fn(current);});
      return chip;
    }
    const quickStart=el("div","calendar-writeback-quick");
    quickStart.append(el("span","calendar-writeback-quick-label","Start"));
    for(const [label,delta] of [["−1 hr",-60],["−15 min",-15],["+15 min",15],["+1 hr",60]]){
      quickStart.appendChild(calendarWritebackQuickChip(label,current=>{
        const duration=current.endMs-current.startMs;
        calendarWritebackWriteTimes(current.startMs+delta*60000,current.startMs+delta*60000+duration);
      }));
    }
    quickStart.appendChild(calendarWritebackQuickChip("Now",current=>{
      const duration=current.endMs-current.startMs;
      const now=new Date();now.setSeconds(0,0);
      const rounded=+now+((5-(now.getMinutes()%5))%5)*60000;
      calendarWritebackWriteTimes(rounded,rounded+duration);
    }));
    const quickLength=el("div","calendar-writeback-quick");
    quickLength.append(el("span","calendar-writeback-quick-label","Length"));
    for(const [label,minutes] of [["30 min",30],["1 hr",60],["90 min",90],["2 hr",120],["4 hr",240]]){
      quickLength.appendChild(calendarWritebackQuickChip(label,current=>{
        calendarWritebackWriteTimes(current.startMs,current.startMs+minutes*60000);
      }));
    }
    times.append(calendarWritebackField("Start date",startDate),calendarWritebackField("Start time",startTime),calendarWritebackField("End date",endDate),calendarWritebackField("End time",endTime));
	    const originalSeriesDate=event&&scope==="series"?calendarWritebackLocalDate(start):"";
	    const exclusionWarning=el("div","calendar-writeback-form-note");
	    function refreshExclusionWarning(){
	      const hasExclusions=!!(event&&event.writeback&&event.writeback.hasExclusions===true);
	      const movedDate=originalSeriesDate&&startDate.value!==originalSeriesDate;
	      exclusionWarning.hidden=!(hasExclusions&&movedDate);
	      if(!exclusionWarning.hidden)exclusionWarning.textContent="This series has skipped dates. They stay on their existing calendar dates; review them after changing the series start date or weekday.";
	    }
	    startDate.addEventListener("input",refreshExclusionWarning);startDate.addEventListener("change",refreshExclusionWarning);refreshExclusionWarning();
    let isAllDay=isAllDayInitial;
    function setAllDay(value){
      isAllDay=!!value;
      // Inclusive display: a one-day all-day event legitimately shows the same
      // start and end date; the exclusive +1 day is applied only on save.
      if(isAllDay&&endDate.value<startDate.value)endDate.value=startDate.value;
      timed.classList.toggle("is-active",!isAllDay);allday.classList.toggle("is-active",isAllDay);startTime.closest(".calendar-writeback-field").hidden=isAllDay;endTime.closest(".calendar-writeback-field").hidden=isAllDay;quickStart.hidden=isAllDay;quickLength.hidden=isAllDay;
    }
    setAllDay(isAllDay);
    const more=document.createElement("details");more.className="calendar-writeback-more";
    const summary=document.createElement("summary");summary.textContent="More details";
    const moreBody=el("div","calendar-writeback-more-body");moreBody.append(calendarWritebackField("Location",location),calendarWritebackField("Notes",desc));
    more.append(summary,moreBody);
    root.append(calendarWritebackField("Calendar",calendar,event?"This edit stays in the original private calendar.":"Only calendars explicitly enabled for Dashboard edits appear here."),calendarWritebackField("Title",title),mode,times,quickStart,quickLength,more);
	    const note=el("div","calendar-writeback-form-note",labels.note);root.append(note,exclusionWarning);
    const actions=el("div","calendar-writeback-form-actions"),cancel=calendarWritebackButton("Cancel","",()=>{if(event)showEventPopup(event);else closeScrim();}),save=calendarWritebackButton(labels.save,"primary",async button=>{
      const payload={calUrl:calendar.value,title:title.value.trim(),desc:desc.value,location:location.value,allDay:isAllDay};
      if(event)payload.uid=event.uid;
      if(scope==="occurrence")payload.occurrenceMs=Number(options.occurrenceMs)||Number(event&&event.writeback&&event.writeback.occurrenceMs)||Number(event&&event.start)||0;
      if(isAllDay){payload.startDate=startDate.value.trim();payload.endDate=exclusiveEndDate(endDate.value.trim());}
      else {payload.startMs=calendarWritebackDateTime(startDate.value,startTime.value);payload.endMs=calendarWritebackDateTime(endDate.value,endTime.value);}
      button.disabled=true;
      try{
        const path=!event?"/api/calendar/event/create":scope==="occurrence"?"/api/calendar/event/occurrence/update":scope==="series"?"/api/calendar/event/series/update":"/api/calendar/event/update";
        const result=await calendarWritebackRequest(path,payload);
        if(result.warning)calendarWritebackShowError(root,result.warning);
        await calendarWritebackRefresh();closeScrim();
      }
      catch(error){button.disabled=false;calendarWritebackShowError(root,error.message);}
    });actions.append(cancel,save);root.appendChild(actions);
    title._oskSubmit=()=>save.click();return root;
  });
}
