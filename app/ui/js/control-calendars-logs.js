function calendarHealthLevelClass(level){
  if(level==="action") return "bad";
  if(level==="check") return "warn";
  if(level==="healthy") return "ok";
  return "unknown";
}
function calendarHealthLabel(level,label){
  if(label) return label;
  if(level==="action") return "Action needed";
  if(level==="check") return "Check soon";
  if(level==="healthy") return "Healthy";
  if(level==="info") return "Hidden";
  return "Unknown";
}
function calendarSourceDetail(c){
  const bits=[];
  if(c.enabled===false) bits.push("hidden"); else bits.push("shown");
  if(c.size!=null) bits.push(fmtBytes(c.size));
  if(c.mtimeMs) bits.push("updated "+ago(c.mtimeMs));
  if(c.isSymlink) bits.push("symlink");
  if(c.cacheFresh===false) bits.push("cache stale");
  return bits.join(" · ");
}
function renderCalendarSummaryCard(st,wrap){
  const sum=st.calendarSummary||{};
  const cls=calendarHealthLevelClass(sum.level);
  const card=el("div","calhealthtop "+cls);
  const detail=(sum.enabled||0)+" shown · "+(sum.hidden||0)+" hidden · "+(st.eventCount||0)+" cached events";
  card.innerHTML=`<div><div class="calhealthlabel">Calendar source health</div><div class="calhealthstate">${escapeHTML(calendarHealthLabel(sum.level,sum.label))}</div><div class="calhealthdetail">${escapeHTML(detail)}</div></div><div class="calhealthstamp">Cache ${escapeHTML(st.generatedAt?ago(st.generatedAt):"not built")}</div>`;
  wrap.appendChild(card);
  const grid=el("div","healthgrid calhealthgrid");
  grid.appendChild(ctrlHealthPill("Sources", sum.total!=null?"ok":"unknown", String(sum.total||0)));
  grid.appendChild(ctrlHealthPill("Shown", "ok", String(sum.enabled||0)));
  grid.appendChild(ctrlHealthPill("Check soon", sum.check?"warn":"ok", String(sum.check||0)));
  grid.appendChild(ctrlHealthPill("Action needed", sum.action?"bad":"ok", String(sum.action||0)));
  wrap.appendChild(grid);
}

async function renderCtrlCalendarHealthPanel(){
  const wrap=$("#ctrlcalhealth");if(!wrap)return;
  ctrlSetLoading(wrap,"Checking calendar source health…","Reading local calendar and event-cache state.");
  try{
    await cachedApi("/api/cache/status",st=>renderCtrlCalendarHealth(st,wrap,true));
  }catch(e){
    wrap.innerHTML="";ctrlSetError(wrap,"Calendar source health unavailable",e,[cbtn("Try again","",()=>renderCtrlCalendarHealthPanel())]);
  }
}

function renderCtrlCalendarHealth(st,wrap,withSummary){
  if(!wrap) return;
  const rows=st.calendars||[];
  wrap.innerHTML="";
  if(withSummary) renderCalendarSummaryCard(st,wrap);
  if(!rows.length){ wrap.appendChild(ctrlStateCard("empty","No calendar sources yet","Add .ics files under ~/dashboard/calendars or use installer option 5 to set up calendar sync.")); return; }
  if(withSummary){
    const actions=el("div","ctrlrow compact calhealthactions");
    actions.appendChild(cbtn("Sync calendars","",async()=>{
      ctrlMsg("Pulling calendars from the web…");
      try{
        const r=await api("/api/calendars/sync","POST",{});
        delete CTRL_CACHE["/api/cache/status"];
        await discoverCalendars(); await loadCalendars(); await renderCtrlCalendarHealthPanel(); await renderCtrlCals();
        const cacheSection=document.querySelector('#ctrlpage-calendars details.ctrlsec[data-lazy="cache"]');
        if(cacheSection&&cacheSection.open)await renderCtrlCache();
        ctrlMsg(r.ran&&r.ran.length ? "Synced via "+r.ran.join(", ")+" — calendar health refreshed." : "No sync script installed.");
      }catch(e){ ctrlMsg("Sync failed: "+e.message); }
    }));
    actions.appendChild(cbtn("Rebuild event cache","",async()=>{
      ctrlMsg("Rebuilding event cache…");
      try{ await api("/api/cache/rebuild","POST",{force:true}); delete CTRL_CACHE["/api/cache/status"]; await loadCalendars(); await renderCtrlCalendarHealthPanel(); const cacheSection=document.querySelector('#ctrlpage-calendars details.ctrlsec[data-lazy="cache"]'); if(cacheSection&&cacheSection.open)await renderCtrlCache(); ctrlMsg("Event cache rebuilt and calendar health refreshed."); }
      catch(e){ ctrlMsg("Cache rebuild failed: "+e.message); }
    }));
    actions.appendChild(cbtn("Refresh health","",async()=>{ await renderCtrlCalendarHealthPanel(); ctrlMsg("Calendar source health refreshed."); }));
    wrap.appendChild(actions);
  }
  const tbl=document.createElement("table"); tbl.className="ctrltable calhealthtable";
  const thead=document.createElement("thead"), headRow=document.createElement("tr");
  for(const label of ["Calendar","Health","Events","Source"]) headRow.appendChild(el("th","",label));
  thead.appendChild(headRow); tbl.appendChild(thead);
  const tb=document.createElement("tbody"); tbl.appendChild(tb);
  for(const c of rows){
    const tr=document.createElement("tr");
    const lvl=calendarHealthLevelClass(c.level);
    const problems=(c.problems&&c.problems.length)?c.problems.join(" · "):calendarSourceDetail(c);
    const name=(c.name||c.url||"");
    tr.className="calrow "+lvl;
    const srcMain=c.mtimeMs?fmtDateTime(c.mtimeMs):"missing";
    const srcMeta=c.isSymlink&&c.realPath?("symlink → "+c.realPath):(c.source&&c.source.sha256?("content hash " + String(c.source.sha256).slice(0,12)):"");

    const nameCell=el("td","","");
    const nameLine=el("div","calname","");
    const color=String(c.color||"").trim();
    if(color && color.length<=80){
      const dot=el("span","caldot","");
      dot.style.backgroundColor=color;
      nameLine.appendChild(dot);
    }
    nameLine.appendChild(el("span","",name));
    nameCell.appendChild(nameLine);
    nameCell.appendChild(el("div","calmeta",c.tag||c.url||""));
    tr.appendChild(nameCell);

    const healthCell=el("td","","");
    healthCell.appendChild(el("span","calbadge "+lvl,calendarHealthLabel(c.level,c.label)));
    healthCell.appendChild(el("div","calissue",problems||"—"));
    tr.appendChild(healthCell);
    tr.appendChild(el("td","",String(c.events||0)));

    const sourceCell=el("td","","");
    sourceCell.appendChild(el("div","",srcMain));
    sourceCell.appendChild(el("div","calmeta",srcMeta));
    tr.appendChild(sourceCell);
    tb.appendChild(tr);
  }
  const tableWrap=el("div","ctrltable-scroll");
  tableWrap.dataset.scrollPolicy="horizontal";
  tableWrap.appendChild(tbl);
  wrap.appendChild(tableWrap);
}
function formatMemorySnapshot(r){
  const when=r.capturedAt?fmtDateTime(r.capturedAt*1000):"now";
  return ["Memory snapshot — "+when,"===== free -h =====",r.free||"", "===== swapon --show =====",r.swap||"", "===== vmstat =====",r.vmstat||"", "===== top RSS =====",r.top||"", "===== dashboard/browser tree =====",r.tree||"", "===== cache/log sizes =====",r.cache||""].join("\n");
}
async function renderCtrlDiagnostics(){
  const wrap=$("#ctrldiag"); if(!wrap) return;
  wrap.innerHTML="";
  const note=el("div","ctrlmini","Doctor results are saved into the diagnostics bundle with cache status, settings, and recent logs.");
  wrap.appendChild(note);
  try{
    const d=await api("/api/doctor/status");
    renderDoctorSummaryCard(wrap,d);
  }catch(e){
    wrap.appendChild(ctrlStateCard("warn","Health check summary unavailable",e.message));
  }
  const repair=actionGroup("Inspect & repair","Run a check, review its repair plan, or apply only safe reversible repairs.","doctor-actiongroup doctor-actiongroup-repair");
  repair.grid.append(
    caction("Run health check","Inspect the dashboard now.","",async()=>{ await runFullHealthCheck(true,"ctrldiag"); }),
    caction("Review repair plan","Explain possible repairs without changing anything.","",async()=>{ await runFullHealthCheck(true,"ctrldiag",false,true); }),
    doctorSafeRepairAction("Run safe repairs","Back up invalid settings, then apply only safe reversible repairs.","Tap again to apply",async()=>{ await runFullHealthCheck(true,"ctrldiag",true); })
  );
  const support=actionGroup("Diagnostics bundle","Collect local evidence only when it is useful.","doctor-actiongroup doctor-actiongroup-support");
  support.grid.append(
    caction("Memory snapshot","Show memory and process state.","",async()=>{
      ctrlShowOutputConsole("ctrlmemory","Memory snapshot","Collecting memory snapshot…","ctrldiag");
      try{ const r=await api("/api/memory/status"); ctrlShowOutputConsole("ctrlmemory","Memory snapshot",formatMemorySnapshot(r),"ctrldiag"); }
      catch(e){ ctrlShowOutputConsole("ctrlmemory","Memory snapshot","Memory snapshot failed: "+e.message,"ctrldiag"); }
    }),
    caction("Export diagnostics","Create a private support bundle for SSH collection.","",async()=>{
      ctrlMsg("Building diagnostics bundle…");
      try{
        const r=await api("/api/diagnostics","POST",{});
        const location=r.location||("~/.dashboard-diagnostics/"+(r.file||"dashboard-diagnostics.zip"));
        ctrlMsg("Diagnostics ready: "+location+" ("+Math.round((r.size||0)/1024)+" KB)");
        note.textContent="Created "+r.file+" — copy it from ~/.dashboard-diagnostics/ if needed.";
        await renderCtrlDiagnostics();
        await renderCtrlActionHistory();
      }catch(e){ ctrlMsg("Diagnostics failed: "+e.message); await renderCtrlActionHistory(); }
    })
  );
  const actionRow=el("div","ctrlcardrow ctrlcardrow-doctor");
  actionRow.append(repair.group,support.group);
  wrap.appendChild(actionRow);
}
