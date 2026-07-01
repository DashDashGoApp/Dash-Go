function systemUpdateLevel(st){
  if(!st) return "unknown";
  if(st.running || st.rebootRecommended) return "warn";
  if(st.state==="failed") return "bad";
  if(st.state==="setup") return "warn";
  if(st.state==="success") return "ok";
  if(st.state==="never") return "unknown";
  return "warn";
}
function systemUpdateMetricClass(st){
  const l=systemUpdateLevel(st);
  return l==="bad"?"bad":(l==="warn"?"warn":(l==="ok"?"ok":"unknown"));
}
async function startSystemUpdateFromControl(){
  ctrlMsg("Starting system update… this can take a while.");
  let r=null;
  try{
    delete CTRL_CACHE["/api/system-update/status"];
    // No request body is needed for this endpoint. Keeping the POST simple
    // avoids older WebKit/surf request-pattern issues seen on Pi Zero testing.
    r=await api("/api/system-update","POST");
  }catch(e){
    ctrlMsg("System update could not start: "+e.message);
    try{ await renderCtrlActionHistory(); }catch(_){}
    throw e;
  }
  const st=(r&&r.status)||{};
  if(st.state==="failed" || st.state==="setup") ctrlMsg((st.label||"System update could not start")+": "+(st.detail||"Open the system update log for details."));
  else ctrlMsg("System update started. You can keep this panel open and refresh the log/status.");
  delete CTRL_CACHE["/api/system-update/status"];
  try{ await renderCtrlSystemUpdate(); }
  catch(e){ ctrlMsg("System update started. Status display hit a browser issue: "+e.message+". Open the log or refresh this section."); }
  try{ await renderCtrlActionHistory(); }catch(_){}
  setTimeout(()=>{ renderCtrlSystemUpdate().catch(()=>{}); },2500);
  return r;
}
async function renderCtrlSystemUpdate(){
  const wrap=$("#ctrlsystemupdate"), log=$("#ctrlsystemupdatelog"); if(!wrap) return;
  delete CTRL_CACHE["/api/system-update/status"];
  ctrlSetLoading(wrap,"Checking system update status…","Reading apt helper readiness, last package update, and reboot recommendation.");
  let st=null;
  try{ st=await api("/api/system-update/status"); }
  catch(e){ wrap.innerHTML=""; ctrlSetError(wrap,"System update status unavailable",e,[cbtn("Try again","",async()=>{ await renderCtrlSystemUpdate(); })]); return; }
  wrap.innerHTML="";
  const cls=systemUpdateMetricClass(st);
  const notice=el("div","maintnotice "+cls);
  const title=st.running?"System update running":(st.label||"System update");
  const detail=st.detail || "Runs apt-get update, then apt-get -y upgrade with installer-granted sudo permission.";
  notice.innerHTML=`<div class="mtitle">${escapeHTML(title)}</div><div class="mdetail">${escapeHTML(detail)}</div>`;
  wrap.appendChild(notice);
  const grid=el("div","ctrlgrid maintenancegrid systemupdategrid");
  const stateLabel=st.running?"Running":(st.label||"Never run");
  const last=(st.completedAt||st.updatedAt||st.logMtime||0);
  const items=[
    ["State", stateLabel, cls],
    ["Last run", last?fmtDateTime(last*1000):"never", st.state==="failed"?"bad":"unknown"],
    ["Helper", st.scriptPresent?"installed":"missing", st.scriptPresent?"ok":"bad"],
    ["sudo", st.sudoPresent?"installed":"missing", st.sudoPresent?"ok":"bad"],
    ["apt-get", st.aptGetPresent?"installed":"missing", st.aptGetPresent?"ok":"bad"],
    ["Reboot", st.rebootRecommended?"recommended":"not needed", st.rebootRecommended?"warn":"ok"],
    ["Log", st.logExists?fmtBytes(st.logSize||0):"none", st.logExists?"ok":"unknown"],
  ];
  for(const [k,v,c] of items){
    const d=el("div","stat "+(c||"unknown"));
    d.innerHTML=`<div class="k">${escapeHTML(k)}</div><div class="v">${escapeHTML(String(v))}</div>`;
    grid.appendChild(d);
  }
  wrap.appendChild(grid);
  if(st.problems && st.problems.length){
    wrap.appendChild(ctrlStateCard("warn","Setup needed",st.problems.join(" · ")));
  }
  wrap.appendChild(el("div","ctrlmini",st.hint||"This updates the operating system packages only; dashboard app updates stay in Update / Restore / Backup."));
  const actions=el("div","ctrlrow actiongrid systemupdateactions");
  const runBtn=confirmBtn("Run system update", st.running?"Already running":"Tap again to run apt update && apt upgrade", startSystemUpdateFromControl);
  runBtn.classList.add("actionbtn");
  if(st.ready && !st.running) runBtn.classList.add("primary");
  runBtn.innerHTML=`<span class="bt">Run system update</span><span class="bd">${escapeHTML(st.running?"A package update is already running.":"Run apt-get update, then apt-get -y upgrade.")}</span>`;
  actions.appendChild(runBtn);
  actions.appendChild(caction("View system update log","Show apt output from the latest package update.","",async()=>{
    if(!log) return; ctrlShowOutputConsole("ctrlsystemupdatelog","System update log","Loading system update log…");
    try{ const r=await api("/api/logs?name=system-update"); ctrlShowOutputConsole("ctrlsystemupdatelog","System update log",r.log||"No system update log entries yet."); }
    catch(e){ ctrlShowOutputConsole("ctrlsystemupdatelog","System update log","System update log unavailable right now: "+e.message); }
  }));
  actions.appendChild(caction("Refresh status","Re-check update progress, log size, and reboot recommendation.","",async()=>{ await renderCtrlSystemUpdate(); ctrlMsg("System update status refreshed."); }));
  if(st.rebootRecommended){
    actions.appendChild(confirmBtn("Reboot device","Tap again to reboot",async()=>{ ctrlMsg("Rebooting…"); try{ await api("/api/reboot","POST",{}); }catch(e){ ctrlMsg(e.message); } }));
  }
  wrap.appendChild(actions);
}

function renderCtrlQuickActions(){
  const row=$("#ctrlactions");
  if(!row || row.dataset.rendered==="1")return;
  row.dataset.rendered="1"; row.replaceChildren();
  const common=actionGroup("","","actiongroup-common");
  common.grid.appendChild(caction("Sync calendars","Fetch every configured calendar now.","",async()=>{
    ctrlMsg("Pulling calendars from the web… (can take up to a minute)");
    try{
      const r=await api("/api/calendars/sync","POST",{});
      await discoverCalendars(); await loadCalendars();
      ctrlMsg(r.ran&&r.ran.length?"Synced via "+r.ran.join(", ")+" — calendar updated.":"No sync script installed (installer option 5 sets one up).");
    }catch(e){ctrlMsg("Sync failed: "+e.message);}
  }));
  common.grid.appendChild(caction("Refresh data","Refresh calendar, weather, and alert data.","",async()=>{
    ctrlMsg("Refreshing calendars, weather, and alerts…");
    try{await Promise.all([discoverCalendars().then(loadCalendars),loadWeather(),loadAlerts()]);checkTheme();updateStale();ctrlMsg("Everything refreshed.");}
    catch(e){ctrlMsg("Refresh hit a problem: "+e.message);}
  }));
  common.grid.appendChild(caction("Restart browser","Restart the kiosk browser without rebooting.","",async()=>{
    ctrlMsg("Restarting browser…");try{await api("/api/browser/restart","POST",{});}catch(e){ctrlMsg(e.message);}
  }));
  common.grid.appendChild(caction("Screen off","Blank the display now; touch wakes it.","",async()=>{
    try{await api("/api/display/off","POST",{});closeCtrl();}catch(e){ctrlMsg(e.message);}
  }));
  row.appendChild(common.group);
}
function renderCtrlPowerActions(){
  const row=$("#ctrlpoweractions");
  if(!row || row.dataset.rendered==="1")return;
  row.dataset.rendered="1"; row.replaceChildren();
  const danger=actionGroup("","","actiongroup-danger");
  danger.grid.appendChild(confirmAction("System update","Install available operating-system packages.","Tap again to run",async()=>{await startSystemUpdateFromControl();}));
  danger.grid.appendChild(confirmAction("Reboot device","Restart this device safely.","Tap again to reboot",async()=>{
    ctrlMsg("Rebooting…");try{await api("/api/reboot","POST",{});}catch(e){ctrlMsg(e.message);}
  }));
  danger.grid.appendChild(confirmAction("Shut down","Power off safely. Use power to start again.","Tap again to shut down",async()=>{
    ctrlMsg("Shutting down… (unplug/replug power to start again)");try{await api("/api/poweroff","POST",{});}catch(e){ctrlMsg(e.message);}
  }));
  row.appendChild(danger.group);
}
