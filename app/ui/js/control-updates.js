let CTRL_UPDATE_POLL=0;
let CTRL_UPDATE_SETTLE=0;
let CTRL_UPDATE_POLL_BUSY=false;
let CTRL_UPDATE_POLL_EPOCH=0;
let CTRL_UPDATE_TRACK_SWITCH_PENDING=false;
const CTRL_UPDATE_POLL_DELAY=1500;
const CTRL_UPDATE_POLL_RETRY_DELAY=2200;

function updateStateClass(state){
  if(state==="success")return "ok";
  if(state==="rolledback"||state==="failed")return "bad";
  if(["preflight","queued","starting","running","validating-payload","committing","checking-runtime","recycling-browser","post-verify-pending","rollback-requested"].includes(state))return "warn";
  return "unknown";
}
function updateJobIsActive(state){return ["preflight","queued","starting","running","validating-payload","committing","checking-runtime","recycling-browser","post-verify-pending","rollback-requested"].includes(state);}
function ctrlUpdateUi(wrap){
  const ui=wrap&&wrap._ctrlUpdateUi;
  return ui&&ui.wrap===wrap&&ui.detail&&ui.detail.parentElement===wrap?ui:null;
}
function ctrlUpdatePollCanRun(){
  const wrap=$("#ctrlupdate"),ui=ctrlUpdateUi(wrap);
  if(!wrap||!ui)return false;
  if(typeof CTRL_OPEN!=="undefined"&&!CTRL_OPEN)return false;
  if(wrap.isConnected===false)return false;
  const section=typeof wrap.closest==="function"?wrap.closest("details.ctrlsec"):null;
  if(section&&section.open===false)return false;
  const page=typeof wrap.closest==="function"?wrap.closest(".ctrlpage"):null;
  if(page&&page.classList&&typeof page.classList.contains==="function"&&!page.classList.contains("show"))return false;
  return true;
}
function stopCtrlUpdatePoll(){
  if(CTRL_UPDATE_POLL){clearTimeout(CTRL_UPDATE_POLL);CTRL_UPDATE_POLL=0;}
  if(CTRL_UPDATE_SETTLE){clearTimeout(CTRL_UPDATE_SETTLE);CTRL_UPDATE_SETTLE=0;}
  CTRL_UPDATE_POLL_EPOCH++;
}
function ctrlUpdateDispose(wrap){
  // A fresh status render replaces the card with a loading shell before it
  // builds again. Use the raw retained shell reference here so the hidden
  // track gesture always releases its pointer and visibility listeners even
  // after its former DOM nodes have been detached.
  const ui=wrap&&wrap._ctrlUpdateUi;
  if(ui&&typeof ui.trackTapDispose==="function"){
    try{ui.trackTapDispose();}catch(_){}
    ui.trackTapDispose=null;
  }
  if(wrap)wrap._ctrlUpdateUi=null;
}
function ctrlUpdateToggleTrack(wrap){
  const ui=ctrlUpdateUi(wrap);
  if(!ui||CTRL_UPDATE_TRACK_SWITCH_PENDING||(ui.lastProgress&&ui.lastProgress.active))return Promise.resolve(null);
  CTRL_UPDATE_TRACK_SWITCH_PENDING=true;
  return Promise.resolve(api("/api/update/track/toggle","POST",{})).then(()=>renderCtrlUpdateRestore({fresh:true})).catch(error=>{
    if(typeof console!=="undefined"&&console.warn)console.warn("dashboard update track toggle failed",error);
    return null;
  }).finally(()=>{CTRL_UPDATE_TRACK_SWITCH_PENDING=false;});
}
function ctrlUpdateBindTrackToggle(tile,wrap){
  if(!tile||typeof attachTaps!=="function")return ()=>{};
  return attachTaps(tile,{maxTaps:6,gap:650,onTaps:()=>ctrlUpdateToggleTrack(wrap)});
}
function updateCatalogPresentation(av){
  const status=String(av&&av.status||"");
  if(av&&av.updateAvailable)return {label:av.availableVersion||"Update available",state:"warn"};
  if(status==="current"&&av&&av.ok)return {label:"Up to date",state:"ok"};
  if(status==="unconfigured")return {label:"Not configured",state:"unknown"};
  if(status==="unreachable"||status==="forbidden")return {label:av.label||"Check failed",state:"bad"};
  return {label:(av&&av.label)||"Not checked",state:(av&&av.ok)?"warn":"unknown"};
}
function updateActionPresentation(pre,active){
  if(active)return {label:"In progress",state:"warn"};
  if(pre&&pre.ready)return {label:"Ready",state:"ok"};
  return {label:(pre&&pre.label)==="Update blocked"?"Update setup needed":((pre&&pre.label)||"Update setup needed"),state:"warn"};
}
// The updater keeps durable state names deliberately coarse so a dashboard can
// safely display an update while its own release is being replaced. The label
// is the precise human-facing phase: catalog, package, archive, staged files,
// safe replacement, runtime, and browser recovery.
function updateJobPresentation(progress){
  const state=String(progress&&progress.state||"");
  const label=String(progress&&progress.label||"");
  return {label:label||state||"never",state:updateStateClass(state)};
}
function updateCheckMessage(st){
  const av=st&&st.availability||{},pre=st&&st.preflight||{};
  if(av.updateAvailable)return `${av.availableVersion||"A newer release"} is available${pre.ready?" and ready to install.":". Update installation still needs setup."}`;
  if(av.status==="current"&&av.ok)return "No newer release is available on the selected track.";
  return `Could not complete the update check: ${av.detail||av.label||"the selected update source needs attention"}`;
}
function ctrlUpdateProgressState(st){
  const job=st&&st.job||{};
  const state=String((st&&st.state)||job.state||(st&&st.updateLogState)||"");
  const active=st&&typeof st.active==="boolean"?st.active:updateJobIsActive(state);
  return {
    state,active,terminal:!!(st&&st.terminal)||!!(state&&!active),
    label:String((st&&st.label)||job.label||(st&&st.updateLogLabel)||""),
    detail:String((st&&st.detail)||job.detail||(st&&st.updateLogDetail)||""),
    source:String((st&&st.source)||job.source||(st&&st.updateLogSource)||""),
    target:String((st&&st.target)||job.target||(st&&st.updateLogTarget)||""),
    track:String((st&&st.track)||job.track||(st&&st.updateLogTrack)||""),
    version:String((st&&st.version)||job.version||(st&&st.updateLogVersion)||""),
    backup:String((st&&st.backup)||job.backup||"")
  };
}
function ctrlUpdateSetStat(tile,label,value,state){
  if(!tile)return;
  tile.className="stat "+(state||"unknown");
  tile.innerHTML=`<div class="k">${escapeHTML(String(label||""))}</div><div class="v">${escapeHTML(String(value==null?"—":value))}</div>`;
}
function ctrlUpdateSetButtonDisabled(button,disabled){
  if(!button)return;
  button.disabled=!!disabled;
  if(disabled)button.setAttribute("aria-disabled","true");
  else if(typeof button.removeAttribute==="function")button.removeAttribute("aria-disabled");
}
function ctrlUpdateSetDetail(node,text,visible){
  if(!node)return;
  node.textContent=text||"";
  node.style.display=visible&&text?"":"none";
}
function ctrlUpdateDetailText(progress){
  const bits=[];
  if(progress.detail)bits.push(progress.detail);
  if(progress.source)bits.push("source: "+(progress.source==="ssh"?"SSH / terminal":progress.source));
  if(progress.backup)bits.push("backup: "+progress.backup);
  return bits.join(" · ");
}
function ctrlUpdateSetActionState(ui,progress,options){
  if(!ui)return;
  const opts=options||{},updateBusy=!!progress.active||!!opts.finalizing,backupBusy=!!ui.backupBusy,busy=updateBusy||backupBusy;
  const action=updateActionPresentation(ui.preflight,!!progress.active),job=updateJobPresentation(progress);
  ctrlUpdateSetStat(ui.rows.action,"Update action",action.label,action.state);
  ctrlUpdateSetStat(ui.rows.job,"Job",job.label,job.state);
  ctrlUpdateSetButtonDisabled(ui.checkButton,busy);
  ctrlUpdateSetButtonDisabled(ui.updateButton,busy||!ui.preflight.ready);
  if(typeof ctrlUpdateSetBackupMutationLocked==="function")ctrlUpdateSetBackupMutationLocked(ui,busy,updateBusy);
  ui.updateButton.classList.remove("armed");
  if(updateBusy){
    ui.updateButton.innerHTML=`<span class="bt">${escapeHTML(opts.finalizing?"Finalizing update":"Update in progress")}</span><span class="bd">${escapeHTML(opts.finalizing?"Refreshing the final dashboard status.":"The dedicated updater is working. Controls stay locked until it reaches a terminal state.")}</span>`;
  }else{
    ui.updateButton.innerHTML=ui.updateButtonNormalHTML;
    ui.updateButton.dataset.normalHtml=ui.updateButtonNormalHTML;
  }
}
function ctrlUpdateApplyProgress(wrap,status,options){
  const ui=ctrlUpdateUi(wrap);if(!ui)return null;
  const progress=ctrlUpdateProgressState(status);
  if(!progress.state)return null;
  ui.lastProgress=progress;
  ctrlUpdateSetActionState(ui,progress,options);
  ctrlUpdateSetDetail(ui.detail,ctrlUpdateDetailText(progress),true);
  return progress;
}
function ctrlUpdateShowPollError(wrap,error){
  const ui=ctrlUpdateUi(wrap);if(!ui)return;
  const prior=ui.lastProgress&&ui.lastProgress.state?" Last known state: "+ui.lastProgress.state+".":"";
  const base=ctrlUpdateDetailText(ui.lastProgress||{});
  ctrlUpdateSetDetail(ui.detail,(base?base+" · ":"")+"Waiting for update service; retrying shortly."+prior,true);
  ctrlUpdateSetButtonDisabled(ui.checkButton,true);
  ctrlUpdateSetButtonDisabled(ui.updateButton,true);
  if(error&&typeof console!=="undefined"&&console.warn)console.warn("dashboard update progress poll failed",error);
}
function scheduleCtrlUpdatePoll(delay){
  if(CTRL_UPDATE_POLL||CTRL_UPDATE_POLL_BUSY)return;
  if(!ctrlUpdatePollCanRun()){stopCtrlUpdatePoll();return;}
  const epoch=CTRL_UPDATE_POLL_EPOCH;
  CTRL_UPDATE_POLL=setTimeout(()=>runCtrlUpdatePoll(epoch),Math.max(0,Number(delay)||CTRL_UPDATE_POLL_DELAY));
}
async function runCtrlUpdatePoll(epoch){
  CTRL_UPDATE_POLL=0;
  if(epoch!==CTRL_UPDATE_POLL_EPOCH||!ctrlUpdatePollCanRun())return;
  CTRL_UPDATE_POLL_BUSY=true;
  let nextDelay=0;
  try{
    const progressPayload=await api("/api/update/progress");
    if(epoch!==CTRL_UPDATE_POLL_EPOCH||!ctrlUpdatePollCanRun())return;
    const progress=ctrlUpdateApplyProgress($("#ctrlupdate"),progressPayload);
    if(progress&&progress.active)nextDelay=CTRL_UPDATE_POLL_DELAY;
    else if(progress&&progress.terminal)ctrlUpdateFinishProgress($("#ctrlupdate"),progress);
    else{ctrlUpdateShowPollError($("#ctrlupdate"));nextDelay=CTRL_UPDATE_POLL_RETRY_DELAY;}
  }catch(error){
    if(epoch!==CTRL_UPDATE_POLL_EPOCH||(typeof ctrlCancelledError==="function"&&ctrlCancelledError(error)))return;
    ctrlUpdateShowPollError($("#ctrlupdate"),error);
    nextDelay=CTRL_UPDATE_POLL_RETRY_DELAY;
  }finally{
    CTRL_UPDATE_POLL_BUSY=false;
    if(nextDelay&&epoch===CTRL_UPDATE_POLL_EPOCH&&ctrlUpdatePollCanRun())scheduleCtrlUpdatePoll(nextDelay);
  }
}
function ctrlUpdateFinishProgress(wrap,progress){
  const ui=ctrlUpdateUi(wrap);if(!ui)return;
  stopCtrlUpdatePoll();
  ctrlUpdateApplyProgress(wrap,{...progress,active:false,terminal:true},{finalizing:true});
  ctrlUpdateSetDetail(ui.detail,(ctrlUpdateDetailText(progress)?ctrlUpdateDetailText(progress)+" · ":"")+"Update reached a terminal state. Refreshing final dashboard status…",true);
  const epoch=CTRL_UPDATE_POLL_EPOCH;
  CTRL_UPDATE_SETTLE=setTimeout(async()=>{
    CTRL_UPDATE_SETTLE=0;
    if(epoch!==CTRL_UPDATE_POLL_EPOCH||!ctrlUpdatePollCanRun())return;
    const refreshed=await renderCtrlUpdateRestore({terminal:true});
    if(refreshed)return;
    const live=ctrlUpdateUi($("#ctrlupdate"));
    if(!live)return;
    ctrlUpdateSetDetail(live.detail,(ctrlUpdateDetailText(live.lastProgress||{})?ctrlUpdateDetailText(live.lastProgress||{})+" · ":"")+"Final status refresh is unavailable. Check for updates is available to retry when the dashboard service is ready.",true);
    ctrlUpdateSetButtonDisabled(live.checkButton,false);
  },900);
}
function ctrlBuildUpdateCard(wrap,st,log){
  const av=st.availability||{},pre=st.preflight||{},progress=ctrlUpdateProgressState(st);
  const catalog=updateCatalogPresentation(av),action=updateActionPresentation(pre,progress.active);
  ctrlUpdateDispose(wrap);
  wrap.replaceChildren();
  const cards=el("div","ctrlgrid updategrid"),rows={};
  const metrics=[
    ["installed","Installed",st.installedVersion||"—","quiet"],
    ["catalog","Update check",catalog.label,catalog.state],
    ["track","Track",av.track||st.updateLogTrack||"—",av.ok?"quiet":"unknown"],
    ["action","Update action",action.label,action.state],
    ["job","Job",updateJobPresentation(progress).label,updateJobPresentation(progress).state],
    ["checked","Last check",av.fetchedAt?fmtDateTime(Number(av.fetchedAt)*1000):"not yet",av.fetchedAt?"quiet":"unknown"],
  ];
  metrics.forEach(([key,label,value,state])=>{const tile=ctrlStat(label,value,state);rows[key]=tile;cards.appendChild(tile);});
  wrap.appendChild(cards);
  const detail=el("div","ctrlmini update-detail");
  detail.style.display="none";
  wrap.appendChild(detail);
  if(Array.isArray(pre.problems)&&pre.problems.length){
    const suffix=av.ok?" Catalog checks remain available.":"";
    wrap.appendChild(el("div","ctrlmini update-detail","Update installation setup: "+pre.problems.join(" · ")+suffix));
  }
  const actions=el("div","ctrlrow actiongrid");
  const checkButton=caction("Check for updates","Fetch the selected track catalog now. This does not install anything.","",async()=>{
    const refreshed=await renderCtrlUpdateRestore({fresh:true});
    if(refreshed)ctrlMsg(updateCheckMessage(refreshed));
  });
  actions.appendChild(checkButton);
  const updateDesc=pre.ready?(av.updateAvailable?"Create a verified safety backup, install "+(av.availableVersion||"the newer release")+", then restart the dashboard.":"Create a verified safety backup and reinstall the selected release after integrity checks."):"The selected GitHub Release can still be checked. Update installation stays disabled until the setup issue above is corrected.";
  const updateButton=confirmBtn("Update dashboard",pre.ready?"Tap again to start the dedicated updater":"Update setup needed",async()=>{
    if(!pre.ready){ctrlMsg(pre.detail||"Updater installation is not ready. Review the setup detail above.");return;}
    try{
      ctrlUpdateSetButtonDisabled(checkButton,true);
      ctrlUpdateSetButtonDisabled(updateButton,true);
      const r=await api("/api/update","POST",{}),started=ctrlUpdateProgressState(r||{});
      ctrlUpdateApplyProgress(wrap,{...(r||{}),...started,active:true},{finalizing:false});
      ctrlMsg(started.job&&started.job.id?"Safety backup verified. Update job started; the browser will restart only after local readiness passes.":"Safety backup verified. Update started.");
      scheduleCtrlUpdatePoll(350);
    }catch(e){
      ctrlUpdateSetButtonDisabled(checkButton,false);
      ctrlUpdateSetButtonDisabled(updateButton,!pre.ready);
      ctrlMsg("Could not start update: "+e.message);
    }
  });
  updateButton.classList.add("actionbtn");
  updateButton.innerHTML=`<span class="bt">Update dashboard</span><span class="bd">${escapeHTML(updateDesc)}</span>`;
  const viewLog=caction("View update log","Show updater evidence. Job state above is authoritative.","",async()=>{if(!log)return;ctrlShowOutputConsole("ctrlupdatelog","Update log","Loading update log…");try{const r=await api("/api/update/log");ctrlShowOutputConsole("ctrlupdatelog","Update log",r.log||"No update log entries yet.");}catch(e){ctrlShowOutputConsole("ctrlupdatelog","Update log","Update log unavailable right now: "+e.message);}});
  actions.append(updateButton,viewLog);wrap.appendChild(actions);
  const ui={wrap,rows,detail,checkButton,updateButton,updateButtonNormalHTML:updateButton.innerHTML,preflight:pre,lastProgress:progress,backupBusy:false,backupStatus:st,backupSection:null,backupDetails:null,backupMutationControls:[],trackTapDispose:null};
  wrap._ctrlUpdateUi=ui;
  ui.trackTapDispose=ctrlUpdateBindTrackToggle(rows.track,wrap);
  if(typeof ctrlBuildBackupRestoreSection!=="function")throw new Error("Backup & Restore controls failed to load before the Update card");
  wrap.appendChild(ctrlBuildBackupRestoreSection(ui,st));
  wrap.appendChild(el("div","ctrlmini","Catalog checks are read-only. Updates run in a dedicated system service; the release is staged and verified before commit, and a verified backup is required before each update."));
  ctrlUpdateApplyProgress(wrap,progress);
  return {ui,progress};
}
async function renderCtrlUpdateRestore(options){
  const fresh=!!(options&&options.fresh),wrap=$("#ctrlupdate"),log=$("#ctrlupdatelog");if(!wrap)return null;
  stopCtrlUpdatePoll();
  ctrlUpdateDispose(wrap);
  ctrlSetLoading(wrap,fresh?"Checking for updates…":"Checking update readiness…",fresh?"Fetching the selected GitHub Release now. This does not install anything.":"Reading release metadata, the dedicated updater state, safety backup readiness, and restore targets.");
  const statusPath=fresh?`/api/update/status?fresh=1&_=${Date.now()}`:"/api/update/status";
  delete CTRL_CACHE["/api/update/status"];delete CTRL_CACHE[statusPath];
  let st={};
  try{st=await api(statusPath);}
  catch(e){
    const ui=ctrlUpdateUi(wrap);
    if(ui){
      ctrlUpdateSetDetail(ui.detail,(ctrlUpdateDetailText(ui.lastProgress||{})?ctrlUpdateDetailText(ui.lastProgress||{})+" · ":"")+"Update status unavailable: "+e.message+". Existing details remain visible.",true);
      ctrlUpdateSetButtonDisabled(ui.checkButton,false);
      ctrlUpdateSetButtonDisabled(ui.updateButton,true);
      return null;
    }
    wrap.replaceChildren(el("div","ctrlmsg","Update status unavailable: "+e.message));return null;
  }
  const built=ctrlBuildUpdateCard(wrap,st,log);
  if(built.progress.active)scheduleCtrlUpdatePoll();
  return st;
}
