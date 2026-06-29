function ctrlCalendarProfileNumberCard(key,label,detail,min,max,step,unit){
  const current=Math.max(min,Math.min(max,Number(CONFIG[key])||min));
  const card=el("div","settingcard settingcard-profile-owned");card.dataset.settingKey=key;
  const value=el("div","settingvalue",`${current} ${unit}`),sub=el("div","settingsub",detail),buttons=el("div","ctrlrow compact settingbuttons");
  const set=async delta=>{
    const now=Math.max(min,Math.min(max,Number(CONFIG[key])||min)),next=Math.max(min,Math.min(max,now+delta));if(next===now)return;
    minus.disabled=true;plus.disabled=true;
    try{
      await ctrlSaveProfileOwned(key,next,label,"calendarlayout");
      if(!ctrlUpdateSettingCard(key,{value:`${next} ${unit}`,minusDisabled:next<=min,plusDisabled:next>=max}))renderCtrlUiSettings();
    }catch(err){
      ctrlMsg(`Could not change ${label}: ${err&&err.message?err.message:String(err)}`);
      renderCtrlUiSettings();
    }
  };
  const minus=cbtn("−","",()=>set(-step)),plus=cbtn("+","",()=>set(step));minus.disabled=current<=min;plus.disabled=current>=max;buttons.append(minus,plus);
  card.append(el("div","settinglabel",label),value,sub,buttons);return card;
}
function ctrlCalendarProfileGroup(title,detail,cards){
  const group=el("section","calendarprofilegroup"),head=el("div","calendarprofilegroup-head");head.append(el("div","calendarprofilegroup-title",title),el("div","calendarprofilegroup-detail",detail));const grid=el("div","settinggrid settinggrid-calendar-profile");for(const card of cards)grid.appendChild(card);group.append(head,grid);return group;
}
function renderCtrlUiSettings(){
  const wrap=$("#ctrluisettings");if(!wrap)return;wrap.replaceChildren();
  wrap.appendChild(ctrlStateCard("info","Calendar layout, range & text","Calendar range, dimensions, start day, week numbers, and event typography are personal choices. Density and refresh safeguards remain automatic."));
  const range=ctrlCalendarProfileGroup("Calendar range","Agenda follows Calendar ahead automatically (weeks × 7 days).",[
    ctrlCalendarProfileNumberCard("weeksBelow","Calendar ahead","Weeks below the current week.",2,16,1,"weeks"),
    ctrlCalendarProfileNumberCard("weeksAbove","Calendar back","Weeks above the current week.",0,6,1,"weeks"),
  ]);
  const geometry=ctrlCalendarProfileGroup("Calendar dimensions","Tune the calendar cell height and sidebar width together. Smaller screens may temporarily constrain the active geometry to protect Calendar columns; your saved selection returns when space allows.",[
    ctrlCalendarProfileNumberCard("rowHeight","Calendar row height","Previews the grid outline immediately; event fitting and Today positioning settle when Control closes.",150,280,5,"px"),
    ctrlCalendarProfileNumberCard("sidebarWidth","Sidebar width","Previews the Calendar/sidebar outline immediately; final Calendar fitting settles when Control closes.",300,520,10,"px"),
  ]);
  const general=el("div","settinggrid settinggrid-calendar-general"),startLabels={0:"Sunday",6:"Saturday",1:"Monday"},startDesc={0:"Normal view",6:"Weekend start",1:"Weekday start"};
  let startValue=+(SETTINGS.firstDayOfWeek!=null?SETTINGS.firstDayOfWeek:CONFIG.firstDayOfWeek||0);
  const setStartDay=v=>{if(v===startValue)return;SETTINGS.firstDayOfWeek=v;CONFIG.firstDayOfWeek=v;applySettings(SETTINGS);postSettings();startValue=v;if(!ctrlUpdateSettingCard("firstDayOfWeek",{value:startLabels[v]||"Sunday",selectedChoice:v}))renderCtrlUiSettings();loadCalendars();};
  const startCard=el("div","settingcard settingcard-calendar-start");startCard.dataset.settingKey="firstDayOfWeek";
  startCard.innerHTML=`<div class="settinglabel">Calendar starts</div><div class="settingvalue">${escapeHTML(startLabels[startValue]||"Sunday")}</div><div class="settingsub">Sunday normal · Saturday weekend start · Monday weekday start</div>`;
  const startRow=el("div","ctrlrow compact startdaybuttons");startRow.setAttribute("role","group");startRow.setAttribute("aria-label","Calendar start day");for(const [v,label] of [[0,"Sunday"],[6,"Start Saturday"],[1,"Start Monday"]]){const selected=v===startValue,b=cbtn(label,selected?"on":"",()=>setStartDay(v));b.dataset.settingChoice=String(v);b.setAttribute("aria-label",(startDesc[v]||"Calendar start day")+": "+label);b.setAttribute("aria-pressed",String(selected));startRow.appendChild(b);}startCard.appendChild(startRow);general.appendChild(startCard);
  let weekOn=SETTINGS.showIsoWeekNumbers!==false;const weekCard=el("div","settingcard settingcard-compact settingcard-weeknumbers");weekCard.dataset.settingKey="showIsoWeekNumbers";
  weekCard.innerHTML=`<div class="settinglabel">Week numbers</div><div class="settingvalue">${weekOn?"On":"Off"}</div><div class="settingsub">ISO week number overlay in the calendar grid.</div>`;
  const setWeekNumbers=on=>{if(on===weekOn)return;applySettings({showIsoWeekNumbers:on});postSettings();weekOn=on;if(!ctrlUpdateSettingCard("showIsoWeekNumbers",{value:on?"On":"Off",selectedChoice:on?"true":"false"}))renderCtrlUiSettings();};
  const weekChoices=el("div","weeknumber-buttons");weekChoices.setAttribute("role","group");weekChoices.setAttribute("aria-label","Week number visibility");for(const [on,label] of [[true,"On"],[false,"Off"]]){const selected=on===weekOn,b=cbtn(label,selected?"on":"",()=>setWeekNumbers(on));b.dataset.settingChoice=String(on);b.setAttribute("aria-label","Week numbers: "+label);b.setAttribute("aria-pressed",String(selected));weekChoices.appendChild(b);}weekCard.appendChild(weekChoices);general.appendChild(weekCard);
  const textCard=el("div","settingcard settingcard-calendar-text settingcard--event-text"),calendarTypography={size:"calendarTextSize",weight:"calendarTextWeight",font:"calendarTextFont"};
  textCard.innerHTML=`<div class="settinglabel">Calendar event text</div><div class="settingvalue">${escapeHTML(typeof dashboardTypographySummary==="function"?dashboardTypographySummary(calendarTypography):"Default size · Default weight · Default font")}</div><div class="settingsub">Event cards, multiday bars, and agenda titles are customized in Display → Dashboard display. Date and time labels stay fixed so the grid remains stable.</div>`;
  const openTypography=cbtn("Open Dashboard typography →","calendar-typography-route",()=>{ if(typeof openDashboardTypographyTarget==="function") openDashboardTypographyTarget("calendar"); });
  openTypography.setAttribute("aria-label","Open Dashboard display typography for Calendar events");textCard.appendChild(openTypography);
  wrap.append(range,geometry,general,textCard);
}
function renderCtrlScreenSettings(){
  const wrap=$("#ctrlscreensettings");if(!wrap)return;wrap.replaceChildren();
  const power=actionGroup("Screen controls","Blanking only affects the display; the dashboard keeps running.","displaygroup grid-3-screen");
  const screenOff=caction("Turn screen off","Touch wakes it.","",async()=>{try{await api("/api/display/off","POST",{});ctrlMsg("Screen off requested. Touch should wake it.");}catch(e){ctrlMsg("Screen off failed: "+e.message);}});
  const screenOn=caction("Wake screen","Force DPMS on.","",async()=>{try{await api("/api/display/on","POST",{});ctrlMsg("Screen wake requested.");}catch(e){ctrlMsg("Screen wake failed: "+e.message);}});
  const nightDim=caction(`Night dim: ${SETTINGS.nightDimEnabled!==false?"on":"off"}`,"Dim dashboard background after sunset.",SETTINGS.nightDimEnabled!==false?"on":"",()=>{SETTINGS.nightDimEnabled=SETTINGS.nightDimEnabled===false;applyNightDim();postSettings();const on=SETTINGS.nightDimEnabled!==false;if(!ctrlUpdateSettingCard("nightDim",{title:`Night dim: ${on?"on":"off"}`,pressed:on}))renderCtrlScreenSettings();});
  nightDim.dataset.settingKey="nightDim";nightDim.setAttribute("aria-pressed",String(SETTINGS.nightDimEnabled!==false));
  power.grid.append(screenOff,screenOn,nightDim);
  const cardRow=el("div","ctrlcardrow ctrlcardrow-screen");
  cardRow.appendChild(power.group);
  const schedule=actionGroup("Sleep schedule","Automatically turn the display off and back on at set times.","displaygroup grid-3-screen");
  schedule.grid.append(
    caction(`Schedule: ${SETTINGS.displaySleepEnabled?"on":"off"}`,"Use the times below.",SETTINGS.displaySleepEnabled?"on":"",()=>{SETTINGS.displaySleepEnabled=!SETTINGS.displaySleepEnabled;postSettings();checkDisplaySleep();renderCtrlScreenSettings();}),
    caction(`Off ${SETTINGS.displaySleepOff||"22:30"}`,"Set bedtime.","",()=>showInlineTimeEditor("displaySleepOff")),
    caction(`On ${SETTINGS.displaySleepOn||"06:00"}`,"Set wake time.","",()=>showInlineTimeEditor("displaySleepOn"))
  );
  cardRow.appendChild(schedule.group);
  wrap.appendChild(cardRow);
  function showInlineTimeEditor(key){
    const old=wrap.querySelector(".inline-time-editor");if(old)old.remove();const origin=[...schedule.grid.querySelectorAll("button")].find(b=>b.textContent.includes(key==="displaySleepOff"?"Off ":"On "));
    const form=el("div","inline-time-editor"),input=oskInput("HH:MM",SETTINGS[key]||"",{mode:"time"});
    const saveTime=cbtn("Save","on",()=>{if(!/^([01]?\d|2[0-3]):[0-5]\d$/.test(input.value)){ctrlMsg("Use HH:MM, e.g. 22:30");showOSKFor(input);return;}SETTINGS[key]=input.value;postSettings();checkDisplaySleep();renderCtrlScreenSettings();setTimeout(()=>{const prefix=key==="displaySleepOff"?"Off ":"On ";const next=[...wrap.querySelectorAll("button")].find(b=>b.textContent.includes(prefix));if(next)next.focus();},0);});
    oskSetSubmit(input,"Save",()=>saveTime.click());
    form.append(el("div","inline-time-copy",key==="displaySleepOff"?"Set screen-off time":"Set screen-on time"),input,saveTime,cbtn("Cancel","",()=>{form.remove();origin&&origin.focus();}));schedule.group.appendChild(form);buildOSK(form);input.click();
  }
}
function openDefaultCtrlSection(page){
  // Intentionally no-op: Dashboard Control opens calm/collapsed. Sections load
  // only after the user expands a card, keeping first paint stable on Pi Zero.
}
const CTRL_LAST_OPEN_SECTION=new Map();
function ctrlPageName(page){return page&&page.id?String(page.id).replace("ctrlpage-",""):"";}
function ctrlRememberLastOpenSection(d){
  if(ctrlLiteProfile()||!d||!d.dataset||!d.dataset.lazy)return;
  const page=d.closest(".ctrlpage");const name=ctrlPageName(page);
  if(name)CTRL_LAST_OPEN_SECTION.set(name,d.dataset.lazy);
}
function ctrlForgetLastOpenSection(d){
  if(!d||!d.dataset||!d.dataset.lazy)return;
  const page=d.closest(".ctrlpage"),name=ctrlPageName(page);
  if(name&&CTRL_LAST_OPEN_SECTION.get(name)===d.dataset.lazy)CTRL_LAST_OPEN_SECTION.delete(name);
}
function ctrlClosePageSectionsForSession(page,preserveLast){
  if(!page)return;
  page.querySelectorAll("details.ctrlsec, details.actiondrawer, details.ctrlbackupcard").forEach(d=>{
    if(preserveLast)d._ctrlPreserveLastOpen=true;
    d.open=false;
  });
}
function collapseCtrlPageSections(page){ctrlClosePageSectionsForSession(page,true);}
function restoreCtrlLastOpenSection(page,requested){
  if(!page||requested||ctrlLiteProfile())return null;
  const lazy=CTRL_LAST_OPEN_SECTION.get(ctrlPageName(page));
  const section=ctrlSectionFor(page,lazy);
  if(section)section.open=true;
  return section||null;
}
let CTRL_SECTION_FOCUS_TOKEN=0;
function focusCtrlSection(d){
  if(!d||!d.open)return;
  const page=d.closest(".ctrlpage.show");
  if(!page||page.id==="ctrlpage-content")return; // Messages owns its explicit page anchor.
  const state=typeof scrollRootState==="function"?scrollRootState(page,"control-page"):null;
  const inputEpoch=state?state.inputEpoch:0,token=++CTRL_SECTION_FOCUS_TOKEN;
  ctrlAfterPaint(()=>{
    if(token!==CTRL_SECTION_FOCUS_TOKEN||!d.open||!page.classList.contains("show"))return;
    if(state&&state.inputEpoch!==inputEpoch)return; // user began scrolling; never fight it.
    const pr=page.getBoundingClientRect(),dr=d.getBoundingClientRect();
    if(dr.top>=pr.top+6&&dr.bottom<=pr.bottom-6)return;
    const behavior=ctrlLiteProfile()?"auto":"smooth";
    try{d.scrollIntoView({behavior,block:"nearest",inline:"nearest"});}
    catch(_){d.scrollIntoView(false);}
  });
}
function clampMessagesScroll(page){
  page=page||document.querySelector("#ctrlpage-content");
  if(!page) return;
  const doClamp=()=>{
    const max=Math.max(0,page.scrollHeight-page.clientHeight);
    if(page.scrollTop>max) page.scrollTop=max;
    if(max===0) page.scrollTop=0;
  };
  ctrlAfterPaint(doClamp);
}
function keepMessagesAnchor(page,anchor,beforeTop){
  if(!page || !anchor || beforeTop==null) return;
  ctrlAfterPaint(()=>{
    try{ page.scrollTop += anchor.getBoundingClientRect().top-beforeTop; }catch(_){}
    clampMessagesScroll(page);
  });
}
let CTRL_PAGE_RENDER_TIMER=0;
function ctrlAfterPaint(fn){ (typeof requestAnimationFrame==="function"?requestAnimationFrame:setTimeout)(()=>setTimeout(fn,0)); }
function markCtrlTabPressed(btn,on){ if(btn) btn.classList.toggle("tapdown",!!on); }
function setCtrlPageVisual(name,page){
  document.querySelectorAll(".ctrlpage").forEach(p=>p.classList.toggle("show",p===page));
  document.querySelectorAll("[data-ctrlpage]").forEach(b=>{ b.classList.toggle("on",b.dataset.ctrlpage===name); if(b.dataset.ctrlpage!==name) markCtrlTabPressed(b,false); });
}
function renderCtrlPageSoon(name,seq){
  clearTimeout(CTRL_PAGE_RENDER_TIMER);
  CTRL_PAGE_RENDER_TIMER=setTimeout(()=>{
    if(seq!==CTRL_PAGE_RENDER_SEQ) return;
    renderCtrlPage(name,seq).catch(e=>{
      if(typeof ctrlCancelledError==="function"&&ctrlCancelledError(e))return;
      if(String(e.message).toLowerCase().includes("locked")){showPinLock();return;}
      ctrlMsg("Control panel section unavailable: "+e.message);
    });
  }, ctrlLiteProfile()?20:0);
}
function ctrlSectionFor(page,lazyKey){
  if(!page||!lazyKey)return null;
  return page.querySelector(`details.ctrlsec[data-lazy="${lazyKey}"]`);
}
function switchCtrlPage(name,opts){
  if(typeof hideOSK==="function") hideOSK();
  const page=document.querySelector("#ctrlpage-"+name);
  if(!page)return null;
  const lazyKey=opts&&opts.openLazy;
  const requested=ctrlSectionFor(page,lazyKey);
  if(page.classList.contains("show")){
    if(requested&&!requested.open)requested.open=true;
    return requested;
  }
  const oldPage=document.querySelector(".ctrlpage.show");
  if(oldPage&&oldPage.id==="ctrlpage-system"&&typeof stopCtrlUpdatePoll==="function")stopCtrlUpdatePoll();
  const seq=++CTRL_PAGE_RENDER_SEQ;
  // Prepare the requested card while its page is still hidden. This removes the
  // Lite-only all-collapsed paint that contextual “Open …” buttons exposed.
  collapseCtrlPageSections(page);
  restoreCtrlLastOpenSection(page,requested);
  if(requested)requested.open=true;
  // Paint the target tab before cancellation/hibernate work. The old page owns
  // only its own requests, so rapid taps leave the final selected page alone.
  setCtrlPageVisual(name,page);
  if(name==="system"&&typeof refreshTerminalAccessCard==="function")ctrlAfterPaint(()=>{if(seq===CTRL_PAGE_RENDER_SEQ&&page.classList.contains("show"))refreshTerminalAccessCard();});
  if(typeof scrollRootState==="function")scrollRootState(page,"control-page");
  if(oldPage){
    const oldName=(oldPage.id||"").replace("ctrlpage-","");
    if(typeof ctrlAbortPageRequests==="function")ctrlAbortPageRequests(oldName);
    if(typeof ctrlCancelPageDeferred==="function")ctrlCancelPageDeferred(oldPage);
  }
  if(oldPage)ctrlAfterPaint(()=>{
    // Do not let a rapid later tab tap skip old-page hibernation. If the user
    // returned to this page before the frame, it is active again and is retained.
    if(!oldPage.classList.contains("show"))cleanupCtrlPage(oldPage,"tab");
  });
  ctrlAfterPaint(()=>{
    if(seq!==CTRL_PAGE_RENDER_SEQ)return;
    ctrlHideAllOutputConsoles();
    renderCtrlPageSoon(name,seq);
  });
  return requested;
}
function ctrlOpenPendingSection(){
  const pending=window.DASH_CONTROL_PENDING_SECTION;
  if(!pending||typeof ctrlOpenSection!=="function")return false;
  if(typeof CTRL_LOCK_STATUS!=="undefined"&&CTRL_LOCK_STATUS&&CTRL_LOCK_STATUS.enabled&&!CTRL_TOKEN)return false;
  window.DASH_CONTROL_PENDING_SECTION=null;
  ctrlOpenSection(pending.page,pending.lazy);
  return true;
}
function ctrlHandleSectionLoadError(d,e){
  if(typeof ctrlCancelledError==="function"&&ctrlCancelledError(e))return;
  if(String(e&&e.message||e).toLowerCase().includes("locked")){showPinLock();return;}
  const target=d&&((d.querySelector(".ctrlsecbody > div"))||d.querySelector(".ctrlsecbody"));
  if(target&&typeof ctrlSetError==="function")ctrlSetError(target,"Section unavailable",e,[cbtn("Try again","",async()=>{d.dataset.loaded="0";await loadCtrlSection(d,true);})]);
  ctrlMsg("Section unavailable: "+(e&&e.message?e.message:String(e)));
}
function queueCtrlSectionLoad(d,delay,after){
  if(!d)return;
  clearTimeout(d._lazyTimer);
  const seq=CTRL_PAGE_RENDER_SEQ;
  d._lazyTimer=setTimeout(()=>{
    const page=d.closest(".ctrlpage");
    if(ctrlLiteProfile()&&(!page||!page.classList.contains("show")||!d.open||seq!==CTRL_PAGE_RENDER_SEQ))return;
    loadCtrlSection(d).then(()=>{if(typeof after==="function")after();}).catch(e=>ctrlHandleSectionLoadError(d,e));
  },delay==null?(ctrlLiteProfile()?30:0):Math.max(0,Number(delay)||0));
}
function ctrlOpenSection(pageName,lazyKey){
  if(!pageName||!lazyKey)return;
  const section=switchCtrlPage(pageName,{openLazy:lazyKey})||ctrlSectionFor(document.querySelector("#ctrlpage-"+pageName),lazyKey);
  if(!section)return;
  if(!section.open)section.open=true;
  // Queue immediately, but never expose an all-collapsed target page while the
  // lazy renderer waits for the next Lite-safe turn.
  queueCtrlSectionLoad(section,0);
}
function initCtrlTabs(){
  document.querySelectorAll("[data-ctrlpage]").forEach(b=>{
    if(!b._bound){
      b._bound=true;
      if(b.tagName==="BUTTON") b.type="button";
      const clear=()=>setTimeout(()=>markCtrlTabPressed(b,false),120);
      b.addEventListener("touchstart",()=>markCtrlTabPressed(b,true),{passive:true});
      ["touchend","touchcancel"].forEach(ev=>b.addEventListener(ev,clear,{passive:true}));
      b.addEventListener("mousedown",()=>markCtrlTabPressed(b,true));
      ["mouseup","mouseleave"].forEach(ev=>b.addEventListener(ev,clear));
      bindTap(b,()=>switchCtrlPage(b.dataset.ctrlpage));
    }
  });
  initCtrlLazySections();
  bindCtrlSummaryTaps();
}
function initCtrlLazySections(){
  document.querySelectorAll("#ctrl details.ctrlsec[data-lazy]").forEach(d=>{
    if(d._lazyBound) return;
    d._lazyBound=true;
    d.addEventListener("toggle",()=>{
      const msgPage=d.closest && d.closest("#ctrlpage-content");
      const accordionPage=d.closest && d.closest(".ctrlpage[data-accordion]");
      const summary=d.querySelector("summary")||d;
      const beforeTop=msgPage && d.open ? summary.getBoundingClientRect().top : null;
      if(!d.open){
        const preserveLast=!!d._ctrlPreserveLastOpen;d._ctrlPreserveLastOpen=false;
        if(!preserveLast)ctrlForgetLastOpenSection(d);
        if(d.dataset&&d.dataset.lazy==="update"&&typeof stopCtrlUpdatePoll==="function")stopCtrlUpdatePoll();
        if(typeof hideOSK==="function") hideOSK();
        if(msgPage) clampMessagesScroll(msgPage);
        return;
      }
      ctrlRememberLastOpenSection(d);
      if(accordionPage){
        accordionPage.querySelectorAll("details.ctrlsec[data-lazy]").forEach(o=>{ if(o!==d) o.open=false; });
      }
      if(typeof hideOSK==="function") hideOSK();
      if(msgPage) keepMessagesAnchor(msgPage,summary,beforeTop);
      else focusCtrlSection(d);
      queueCtrlSectionLoad(d,undefined,msgPage?()=>keepMessagesAnchor(msgPage,summary,beforeTop):null);
    });
  });
}
function bindCtrlSummaryTaps(){
  document.querySelectorAll("#ctrl details.ctrlsec > summary, #ctrl details.actiondrawer > summary").forEach(s=>{
    if(s._fastSummaryBound) return;
    s._fastSummaryBound=true;
    bindTap(s,()=>{
      const d=s.parentElement;
      if(d && d.tagName==="DETAILS") d.open=!d.open;
    },{preventDefault:true});
  });
}

async function loadCtrlSection(d,force){
  if(!d || !d.dataset) return;
  if(d._lazyPromise)return d._lazyPromise;
  const page=d.closest(".ctrlpage");
  if(ctrlLiteProfile() && page && !page.classList.contains("show")) return;
  const key=d.dataset.lazy;
  if(!key||(!force&&d.dataset.loaded==="1"))return;
  const scope=typeof ctrlPageRequestScope==="function"?ctrlPageRequestScope():null;
  const pending=(async()=>{
    await renderCtrlLazy(key,force);
    if((scope&&typeof ctrlPageScopeCurrent==="function"&&!ctrlPageScopeCurrent(scope))||(ctrlLiteProfile()&&page&&!page.classList.contains("show")))return;
    d.dataset.loaded="1";
  })();
  d._lazyPromise=pending;
  try{return await pending;}
  finally{if(d._lazyPromise===pending)d._lazyPromise=null;}
}
async function renderCtrlLazy(key,force){
  switch(key){
    case "health": await renderCtrlHealthOverview(); break;
    case "status": await cachedApi("/api/status",st=>renderCtrlStatus(st)); break;
    case "calhealth": await renderCtrlCalendarHealthPanel(); break;
    case "cache": await renderCtrlCache(); break;
    case "weatheralerts": await renderCtrlWeatherAlerts(); break;
    case "dashboarddisplay": renderCtrlDashboardDisplay(); break;
    case "mapcache": await renderCtrlMapCache(); break;
    case "update": await renderCtrlUpdateRestore(); break;
    case "profile": await renderCtrlProfile(); break;
    case "security": await renderCtrlSecurity(); break;
    case "systemupdate": await renderCtrlSystemUpdate(); break;
    case "terminal": await renderCtrlTerminal(); break;
    case "history": await renderCtrlActionHistory(); break;
    case "diagnostics": await renderCtrlDiagnostics(); break;
    case "location": await renderCtrlLocation(); break;
    case "people": await renderCtrlPeople(); break;
    case "todo": await renderCtrlTodo(); break;
    case "moon": await renderCtrlMoon(); break;
    case "screen": renderCtrlScreenSettings(); break;
    case "calendarlayout": renderCtrlUiSettings(); break;
    case "theme": await renderCtrlTheme(); break;
    case "visuals": renderCtrlVisualStyle(); break;
    case "cals": await renderCtrlCals(); break;
    case "content": await renderCtrlComp(); break;
    case "builtins": await renderCtrlBuiltins(); break;
    case "birthdays": await renderCtrlBirthdays(); break;
    case "celebrations": await renderCtrlCelebrations(); break;
    case "feeds": await renderCtrlFeeds(); break;
    case "sources": await renderCtrlSources(); break;
    case "tempmsg": await renderCtrlTempMessages(); break;
    case "schedmsg": await renderCtrlScheduledMessages(); break;
  }
}
async function renderCtrlPage(name,seq){
  initCtrlTabs();
  if(seq && seq!==CTRL_PAGE_RENDER_SEQ) return;
  if(name==="overview") renderCtrlQuickActions();
  else if(name==="system") renderCtrlPowerActions();
  bindCtrlSummaryTaps();
  const page=document.querySelector("#ctrlpage-"+name);
  if(!page || !page.classList.contains("show")) return;
  const opened=[...page.querySelectorAll("details.ctrlsec[data-lazy]")].filter(d=>d.open);
  for(const d of opened){
    if(seq && seq!==CTRL_PAGE_RENDER_SEQ) return;
    await loadCtrlSection(d);
  }
}
