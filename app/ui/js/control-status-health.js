function ago(t){ return t?fmtAge(Date.now()-t)+" ago":"never"; }
function renderCtrlStatus(st){
  if(typeof syncTerminalAccessCard==="function")syncTerminalAccessCard(!st||st.terminalAccessEnabled!==false);
  const wrap=$("#ctrlstatus"); wrap.innerHTML=""; wrap.className="ctrlgrid";
  const wifi=st.wifi||{}, map=st.map||{}, mp=map.provider||{};
  const now=Date.now();
  // RAW values only — the render loop below escapes exactly once. (A previous
  // version pre-escaped here AND escaped in the loop, which displayed a
  // literal "<wbr>" inside the IP address.) The IP needs no break helper:
  // its tile spans two grid columns and fits on one line.
  const items=[
    ["WiFi", wifi.ssid||"not connected"],
    ["Signal", wifi.signal!=null?wifi.signal+"%":"—"],
    ["IP address", wifi.ip||"—"],
    ["Hostname", st.hostname||"—"],
    ["Performance profile", st.profileLabel || profileLabel(st.profile || CONFIG.profile)],
    ["CPU temp", st.temp_c!=null?st.temp_c.toFixed(1)+"°C":"—"],
    ["CPU freq", st.freq_mhz!=null?st.freq_mhz+" MHz":"—"],
    ["CPU load", st.load!=null?st.load:"—"],
    ["Available RAM", st.mem_avail_mb!=null?st.mem_avail_mb+" MB":"—"],
    ["Swap used", st.swap_used_mb!=null?st.swap_used_mb+" MB":"—"],
    ["Disk free", st.disk_free_mb!=null?(st.disk_free_mb>=2048?(st.disk_free_mb/1024).toFixed(1)+" GB":st.disk_free_mb+" MB"):"—"],
    ["Throttling", st.throttled==null?"—":(st.throttled==="0x0"?"none":st.throttled)],
    ["Uptime", st.uptime||"—"],
    // Client-side data freshness — the dashboard knows these itself.
    ["Calendar data", ago(lastCalOK)],
    ["Weather data", ago(lastWxOK)],
    ["Events loaded", String(EVENTS.length)],
    ["Map provider", mp.primaryLabel||mp.primary||"auto"],
    ["Map images", (map.imageCount!=null?map.imageCount:"—")+" files · "+(map.imageLocationCount!=null?map.imageLocationCount:"—")+" locations"],
    ["Map prewarm", map.prewarm&&map.prewarm.running?"running":(map.prewarm&&map.prewarm.lastEnd?ago(map.prewarm.lastEnd*1000):"not yet")],
    ["Display fonts", st.fontsPresent===false?"missing · using fallbacks":"present"],
    ["Page running", fmtAge(now-BOOT_TS)],
  ];
  for(const [k,v] of items){
    // The IP tile spans two columns so the full address sits on one line
    // instead of wrapping with a dangling digit.
    const d=el("div","stat"+(k==="IP address"?" wide":""));
    d.innerHTML=`<div class="k">${escapeHTML(k)}</div><div class="v">${escapeHTML(String(v))}</div>`;
    wrap.appendChild(d);
  }
}
function fmtDateTime(ms){
  if(!ms) return "—";
  try{ return FMT.popDay.format(new Date(ms))+" "+FMT.time.format(new Date(ms)); }
  catch(_){ return new Date(ms).toLocaleString(); }
}
function ctrlHealthLevelRank(level){
  return {bad:4,warn:3,info:2,ok:1,unknown:0}[level]||0;
}
function ctrlHealthAge(ts,warnMs,badMs){
  if(!ts) return {level:"bad", label:"never"};
  const age=Date.now()-ts;
  if(age>=badMs) return {level:"bad", label:fmtAge(age)+" ago"};
  if(age>=warnMs) return {level:"warn", label:fmtAge(age)+" ago"};
  return {level:"ok", label:fmtAge(age)+" ago"};
}
function ctrlHealthPill(label,level,detail){
  const d=el("div","healthpill "+(level||"unknown"));
  d.innerHTML=`<span class="hl">${escapeHTML(label)}</span><span class="hv">${escapeHTML(detail||"—")}</span>`;
  return d;
}
function doctorStateLevel(state){
  if(state==="action") return "bad";
  if(state==="check") return "warn";
  if(state==="healthy") return "ok";
  return "unknown";
}
function doctorStatusDetail(d){
  if(!d || !d.exists) return "not checked yet";
  const bits=[];
  if(d.fixCount) bits.push(d.fixCount+" fixed");
  bits.push((d.failCount||0)+" fail");
  bits.push((d.warnCount||0)+" warn");
  return bits.join(" · ");
}
function renderDoctorSummaryCard(wrap,d){
  const level=doctorStateLevel(d&&d.state);
  const card=el("div","doctorcard "+level);
  const checked=d&&d.checkedAt?fmtDateTime(d.checkedAt*1000):"Not run yet";
  card.innerHTML=`<div class="doctorhead"><div><div class="doctorlabel">Full health check</div><div class="doctorstate">${escapeHTML((d&&d.label)||"Not checked yet")}</div></div><div class="doctorstamp">${escapeHTML(checked)}</div></div>`;
  const meta=el("div","doctormeta",doctorStatusDetail(d)); card.appendChild(meta);
  if(d && d.issues && d.issues.length){
    const list=el("div","doctorissues");
    for(const it of d.issues.slice(0,5)){
      list.appendChild(el("div","doctorissue "+(it.level==="fail"?"bad":"warn"),`${it.section||"Check"}: ${it.message||""}`));
    }
    card.appendChild(list);
  }
  wrap.appendChild(card);
}
async function runFullHealthCheck(showOutput,target,fix,plan){
  const outputTarget=target || (ctrlActivePageName()==="overview" ? "ctrlhealth" : "ctrldiag");
  const repairing=!!fix, planning=!!plan;
  const title=planning?"Doctor repair plan":(repairing?"Doctor safe-repair output":"Health check output");
  const intro=planning?"Finding and explaining possible repairs; no changes are being made…":(repairing?"Applying only safe, reversible Doctor repairs…":"Running health check…");
  ctrlMsg(intro);
  ctrlShowOutputConsole("ctrldoctor",title,intro,outputTarget);
  try{
    const r=await api("/api/doctor","POST",{fix:repairing,plan:planning});
    delete CTRL_CACHE["/api/doctor/status"];
    await renderCtrlHealthOverview();
    await renderCtrlDiagnostics();
    await renderCtrlActionHistory();
    if(showOutput!==false) ctrlShowOutputConsole("ctrldoctor",title,r.output||"Doctor finished, but no text output was returned.",outputTarget);
    else ctrlHideOutputConsole("ctrldoctor");
    const label=(r.summary&&r.summary.label)||"Health check finished";
    ctrlMsg(planning?"Repair plan ready — review the details below.":(r.ok?label+".":label+" — see results below."));
    return r;
  }catch(e){ ctrlShowOutputConsole("ctrldoctor",title,"Doctor unavailable right now: "+e.message,outputTarget); ctrlMsg("Doctor unavailable: "+e.message); throw e; }
}

async function renderCtrlHealthOverview(){
  const wrap=$("#ctrlhealth"); if(!wrap) return;
  ctrlSetLoading(wrap,"Loading health-check status…","This reads only the last saved doctor summary. The full check runs only when you tap the button.");
  try{
    const d=await api("/api/doctor/status");
    wrap.innerHTML="";
    const card=el("div","doctorcard compact");
    const checked=d&&d.checkedAt?fmtDateTime(d.checkedAt*1000):"Not run yet";
    const fail=d&&d.exists?(d.failCount||0):0;
    const warn=d&&d.exists?(d.warnCount||0):0;
    const label=d&&d.exists?"Last health check":"Health check has not been run";
    card.innerHTML=`<div class="doctorhead"><div><div class="doctorlabel">Health check</div><div class="doctorstate small">${escapeHTML(label)}</div></div><div class="doctorstamp">${escapeHTML(checked)}</div></div>`;
    const fixed=d&&d.exists?(d.fixCount||0):0;
    card.appendChild(el("div","doctormeta",d&&d.exists?`${fixed} fixed · ${fail} fail · ${warn} warn`:"No saved result yet."));
    card.appendChild(el("div","doctorpolicy","Runs only when requested to keep low-memory kiosks responsive."));
    if(d && d.issues && d.issues.length){
      const list=el("div","doctorissues");
      for(const it of d.issues.slice(0,4)){
        list.appendChild(el("div","doctorissue "+(it.level==="fail"?"bad":"warn"),`${it.section||"Check"}: ${it.message||""}`));
      }
      card.appendChild(list);
    }
    wrap.appendChild(card);
    const actions=actionGroup("Health actions","Run a check here, then use Diagnostics for repair details and support tools.","doctor-overview-actions");
    actions.grid.append(
      caction("Run health check","Inspect the dashboard now.","",async()=>{ await runFullHealthCheck(true,"ctrlhealth"); }),
      caction("Open Diagnostics","Review plans, safe repairs, memory, and export tools.","",()=>{ ctrlOpenSection("system","diagnostics"); })
    );
    wrap.appendChild(actions.group);
  }catch(e){
    ctrlSetError(wrap,"Health-check status unavailable",e,[cbtn("Try again","",async()=>{ await renderCtrlHealthOverview(); })]);
  }
}

function fmtStatusDate(s){
  if(!s) return "—";
  const t=Date.parse(s);
  if(Number.isNaN(t)) return String(s);
  return fmtDateTime(t);
}
function updateStateClass(state){
  if(state==="success") return "ok";
  if(state==="failed") return "bad";
  if(state==="running") return "warn";
  if(state==="never" || state==="empty") return "unknown";
  return "warn";
}
function renderMaintenanceNotice(st){
  let cls="ok", title="Up to date", detail="";
  const av=st.availability||{};
  detail="No newer dashboard release is currently available.";
  if(st.error){ cls="bad"; title="Status unavailable"; detail=st.error; }
  else if(av.updateAvailable){ cls="warn"; title="Update Available"; detail=(av.availableVersion||"A newer release")+" is available."; }
  else if(av.status==="blocked" || av.status==="unreachable" || av.status==="unconfigured" || av.ok===false){ cls="warn"; title=av.label||"Update check needs attention"; detail=av.detail||"The saved update source could not be checked."; }
  else if(st.problems && st.problems.length){ cls="warn"; title="Check before updating"; detail=st.problems.join(" · "); }
  else if(!st.updateReady){ cls="warn"; title="Setup needed"; detail="The updater needs ~/install.sh and saved update credentials before unattended updates can run."; }
  const d=el("div","maintnotice "+cls);
  d.innerHTML=`<div class="mtitle">${escapeHTML(title)}</div><div class="mdetail">${escapeHTML(detail)}</div>`;
  return d;
}

function renderBackupKind(kind){
  if(kind==="pre-update") return "Pre-update";
  if(kind==="pre-restore") return "Pre-restore";
  if(kind==="manual") return "Manual";
  return kind?String(kind).replace(/-/g," "):"Backup";
}
async function restoreBackupByName(name){
  if(!name){ ctrlMsg("No backup selected."); return; }
  const updateUi=typeof ctrlUpdateUi==="function"?ctrlUpdateUi($("#ctrlupdate")):null;
  if(updateUi&&typeof ctrlRunBackupMutation==="function") return ctrlRunBackupMutation(updateUi,"restore",name);
  ctrlMsg("Restoring selected backup…");
  try{
    const r=await api("/api/backup/restore","POST",{name:name});
    ctrlMsg("Safety backup saved, then restored "+(r.restored||0)+" files from "+(r.name||name)+".");
    await discoverCalendars(); await loadCalendars(); await renderCtrlUpdateRestore(); await renderCtrlActionHistory();
  }catch(e){ ctrlMsg("Restore failed: "+e.message); await renderCtrlActionHistory(); }
}
async function deleteBackupByName(name){
  if(!name){ ctrlMsg("No backup selected."); return; }
  const updateUi=typeof ctrlUpdateUi==="function"?ctrlUpdateUi($("#ctrlupdate")):null;
  if(updateUi&&typeof ctrlRunBackupMutation==="function") return ctrlRunBackupMutation(updateUi,"delete",name);
  ctrlMsg("Deleting selected backup…");
  try{
    const r=await api("/api/backup/delete","POST",{name:name});
    ctrlMsg("Deleted backup: "+(r.deleted||name)+".");
    await renderCtrlUpdateRestore(); await renderCtrlActionHistory();
  }catch(e){ ctrlMsg("Delete failed: "+e.message); await renderCtrlActionHistory(); }
}
function renderBackupRows(backups,st,options){
  const opts=options||{},keep=st&&st.backupKeep?Number(st.backupKeep):50;
  const details=document.createElement("details");
  details.className="ctrlbackupcard";
  if(opts.open)details.open=true;
  const count=backups&&backups.length?backups.length:0;
  const total=st&&st.backupTotalSize?fmtBytes(st.backupTotalSize):"";
  details.innerHTML=`<summary><span>Local backups</span><span class="backupcount">${count}${total?" · "+escapeHTML(total):""}</span></summary>`;
  const body=el("div","backupcardbody"),mutationControls=[];
  if(!backups || !backups.length){
    body.appendChild(ctrlStateCard("empty","No local backups yet","Create a backup before major changes. Updates and restores will also create safety backups automatically."));
  }else{
    const list=el("div","backuplist");
    for(const b of backups){
      const created=(b.createdAt||b.mtime||0)*1000;
      const item=el("div","backupitem");
      const main=el("div","backupmain");
      main.innerHTML=`<div class="backuptitle">${escapeHTML(b.name||"")}</div>`+
        `<div class="backupreason">${escapeHTML(b.reason||"")}</div>`+
        `<div class="backupmeta">${escapeHTML(renderBackupKind(b.kind))} · ${escapeHTML(b.version||"—")} · ${escapeHTML(fmtDateTime(created))} · ${escapeHTML(fmtBytes(b.size||0))}</div>`;
      const actions=el("div","backupactions");
      const restore=confirmBtn("Restore","Confirm restore with safety backup",async()=>{
        if(typeof opts.onRestore==="function")return opts.onRestore(b.name);
        return restoreBackupByName(b.name);
      });
      restore.classList.add("backupbtn","restore");
      restore.title="Creates a pre-restore safety backup before restoring this selected archive.";
      restore.setAttribute("aria-label","Restore "+String(b.name||"selected backup")+" after creating a pre-restore safety backup");
      const del=confirmBtn("Delete","Confirm permanent delete",async()=>{
        if(typeof opts.onDelete==="function")return opts.onDelete(b.name);
        return deleteBackupByName(b.name);
      });
      del.classList.add("backupbtn","delete");
      del.title="Permanently deletes only this selected local backup archive.";
      del.setAttribute("aria-label","Permanently delete "+String(b.name||"selected backup"));
      if(opts.disabled){ restore.disabled=true; del.disabled=true; restore.setAttribute("aria-disabled","true"); del.setAttribute("aria-disabled","true"); }
      mutationControls.push(restore,del);
      actions.appendChild(restore); actions.appendChild(del);
      item.appendChild(main); item.appendChild(actions);
      list.appendChild(item);
    }
    body.appendChild(list);
    body.appendChild(el("div","ctrlmini","Scroll this card to review backups. Retention keeps the newest "+keep+" backups by default."));
  }
  details.appendChild(body);
  details._backupMutationControls=mutationControls;
  details._backupList=body.querySelector?body.querySelector(".backuplist"):null;
  return details;
}
