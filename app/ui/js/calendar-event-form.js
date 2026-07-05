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
function calendarWritebackGroup(label,content,help){
  const field=el("div","calendar-writeback-field");field.append(el("span","calendar-writeback-label",label),content);
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
function calendarWritebackProviderLabel(item){
  const provider=String(item&&item.provider||"").toLowerCase();
  if(provider==="google")return "Google calendar";
  if(provider==="caldav")return "iCloud / CalDAV";
  return "Private calendar";
}
function calendarWritebackCalendarChoices(calendars){
  const base=(Array.isArray(calendars)?calendars:[]).map(item=>({
    source:String(item&&item.source||""),
    name:String(item&&item.name||"Private calendar").trim()||"Private calendar",
    provider:calendarWritebackProviderLabel(item),
  })).filter(item=>item.source);
  const duplicateKey=item=>`${item.name.toLowerCase()}\u0000${item.provider.toLowerCase()}`;
  const totals=new Map();
  base.forEach(item=>{const key=duplicateKey(item);totals.set(key,(totals.get(key)||0)+1);});
  const seen=new Map();
  return base.map(item=>{
    const key=duplicateKey(item),ordinal=(seen.get(key)||0)+1;
    seen.set(key,ordinal);
    return {...item,ordinal:totals.get(key)>1?ordinal:0};
  });
}
function calendarWritebackCalendarCopy(choice,detail){
  const copy=el("span","calendar-writeback-calendar-copy");
  copy.appendChild(el("span","calendar-writeback-calendar-name",choice.name));
  let meta=choice.provider;
  if(choice.ordinal)meta+=` ${choice.ordinal}`;
  if(detail)meta+=` · ${detail}`;
  copy.appendChild(el("span","calendar-writeback-calendar-meta",meta));
  return copy;
}
function calendarWritebackFixedCalendarRow(choice,detail){
  // el() accepts text only as its third argument. Keep the provider/name copy
  // as a real nested node so fixed existing, single-choice, and unavailable
  // calendars never stringify to "[object HTMLSpanElement]".
  const row=el("div","calendar-writeback-calendar-fixed");
  row.appendChild(calendarWritebackCalendarCopy(choice,detail));
  return row;
}
function calendarWritebackPopupScrollRoot(){return document.getElementById("popbody");}
function calendarWritebackFormTap(node,handler){
  return bindTap(node,handler,{scrollRoot:calendarWritebackPopupScrollRoot});
}
function calendarWritebackCalendarPicker(calendars,source,event){
  const choices=calendarWritebackCalendarChoices(calendars);
  const requested=String(source||"");
  let selected=choices.find(item=>item.source===requested)||null;
  const shell=el("div","calendar-writeback-calendar-picker");
  // An existing event must never silently fall back to the first writable
  // calendar. A source can become unavailable between opening its event popup
  // and opening this form, and moving it would be an unsafe copy/delete action.
  if(!selected&&event){
    shell.classList.add("is-fixed","is-unavailable");
    shell.appendChild(calendarWritebackFixedCalendarRow({name:"Calendar unavailable",provider:"Dashboard edits are no longer enabled",ordinal:0},"Return to the event and refresh Calendar Manager."));
    return {node:shell,selected:()=>null,unavailable:true};
  }
  selected=selected||choices[0]||null;
  if(!selected)return {node:shell,selected:()=>null};
  const fixed=!!event||choices.length===1;
  if(fixed){
    const detail=event?"This edit stays here":"Selected writable calendar";
    shell.classList.add("is-fixed");shell.appendChild(calendarWritebackFixedCalendarRow(selected,detail));
    return {node:shell,selected:()=>selected};
  }
  const optionsID="calendar-writeback-calendar-options";
  const trigger=el("button","calendar-writeback-calendar-trigger");trigger.type="button";trigger.setAttribute("aria-controls",optionsID);trigger.setAttribute("aria-expanded","false");
  const chevron=el("span","calendar-writeback-calendar-chevron","⌄");chevron.setAttribute("aria-hidden","true");trigger.append(calendarWritebackCalendarCopy(selected,"Choose calendar"),chevron);
  const options=el("div","calendar-writeback-calendar-options");options.id=optionsID;options.hidden=true;options.setAttribute("role","group");options.setAttribute("aria-label","Calendar choices");
  function setOpen(open){
    const next=!!open;options.hidden=!next;shell.classList.toggle("is-open",next);trigger.setAttribute("aria-expanded",String(next));
  }
  function renderOptions(){
    options.replaceChildren();
    choices.forEach(choice=>{
      const option=el("button","calendar-writeback-calendar-option");option.type="button";
      const active=choice.source===selected.source;option.classList.toggle("is-selected",active);option.setAttribute("aria-pressed",String(active));
      option.appendChild(calendarWritebackCalendarCopy(choice,active?"Selected":""));
      calendarWritebackFormTap(option,event=>{event.preventDefault();event.stopPropagation();selected=choice;trigger.replaceChildren(calendarWritebackCalendarCopy(selected,"Choose calendar"),chevron);renderOptions();setOpen(false);});
      options.appendChild(option);
    });
  }
  calendarWritebackFormTap(trigger,event=>{event.preventDefault();event.stopPropagation();setOpen(options.hidden);});
  renderOptions();shell.append(trigger,options);
  return {node:shell,selected:()=>selected};
}
function calendarWritebackQuickRow(label){
  const row=el("div","calendar-writeback-quick");
  const options=el("div","calendar-writeback-quick-options");
  row.append(el("span","calendar-writeback-quick-label",label),options);
  return {row,options};
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
    const calendarPicker=calendarWritebackCalendarPicker(calendars,source,event);
    const calendarHelp=calendarPicker.unavailable?"This event cannot be moved to another calendar from Dash-Go.":event?"This edit stays in the original private calendar.":calendars.length===1?"This is the only calendar currently enabled for Dashboard edits.":"Choose a calendar explicitly enabled for Dashboard edits.";
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
      calendarWritebackFormTap(chip,event=>{event.preventDefault();event.stopPropagation();const current=calendarWritebackFormTimes();if(current)fn(current);});
      return chip;
    }
    const quickStart=calendarWritebackQuickRow("Start");
    for(const [label,delta] of [["−1 hr",-60],["−15 min",-15],["+15 min",15],["+1 hr",60]]){
      quickStart.options.appendChild(calendarWritebackQuickChip(label,current=>{
        const duration=current.endMs-current.startMs;
        calendarWritebackWriteTimes(current.startMs+delta*60000,current.startMs+delta*60000+duration);
      }));
    }
    quickStart.options.appendChild(calendarWritebackQuickChip("Now",current=>{
      const duration=current.endMs-current.startMs;
      const now=new Date();now.setSeconds(0,0);
      const rounded=+now+((5-(now.getMinutes()%5))%5)*60000;
      calendarWritebackWriteTimes(rounded,rounded+duration);
    }));
    const quickLength=calendarWritebackQuickRow("Length");
    for(const [label,minutes] of [["30 min",30],["1 hr",60],["90 min",90],["2 hr",120],["4 hr",240]]){
      quickLength.options.appendChild(calendarWritebackQuickChip(label,current=>{
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
      timed.classList.toggle("is-active",!isAllDay);allday.classList.toggle("is-active",isAllDay);
      startTime.closest(".calendar-writeback-field").hidden=isAllDay;
      endTime.closest(".calendar-writeback-field").hidden=isAllDay;
      quickStart.row.hidden=isAllDay;quickLength.row.hidden=isAllDay;
    }
    setAllDay(isAllDay);
    const more=document.createElement("details");more.className="calendar-writeback-more";
    const summary=document.createElement("summary");summary.textContent="More details";
    const moreBody=el("div","calendar-writeback-more-body");moreBody.append(calendarWritebackField("Location",location),calendarWritebackField("Notes",desc));
    more.append(summary,moreBody);
    root.append(calendarWritebackGroup("Calendar",calendarPicker.node,calendarHelp),calendarWritebackField("Title",title),mode,times,quickStart.row,quickLength.row,more);
    const note=el("div","calendar-writeback-form-note",labels.note);root.append(note,exclusionWarning);
    const actions=el("div","calendar-writeback-form-actions"),cancel=calendarWritebackButton("Cancel","",()=>{if(event)showEventPopup(event);else closeScrim();}),save=calendarWritebackButton(labels.save,"primary",async button=>{
      const selected=calendarPicker.selected();
      if(!selected){calendarWritebackShowError(root,calendarPicker.unavailable?"This event’s original calendar is no longer enabled for Dashboard edits.":"Choose a calendar before saving this event.");return;}
      const payload={calUrl:selected.source,title:title.value.trim(),desc:desc.value,location:location.value,allDay:isAllDay};
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
    });
    if(calendarPicker.unavailable)save.disabled=true;
    actions.append(cancel,save);root.appendChild(actions);
    title._oskSubmit=()=>save.click();return root;
  });
}
