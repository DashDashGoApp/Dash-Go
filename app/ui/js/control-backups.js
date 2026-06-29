// 09-control-04a-backups.js — stable Update-card backup and restore subsection.
// This stays separate from active-update polling: backup controls refresh only
// after a direct user mutation or a full final Update-card status read.
function ctrlBackupNumber(value,fallback){
  const n=Number(value);
  return Number.isFinite(n)&&n>=0?n:fallback;
}
function ctrlBackupStatusPatch(current,patch){
  const base=current&&typeof current==="object"?current:{},next={...base,...(patch||{})};
  const backups=Array.isArray(next.backups)?next.backups:(Array.isArray(base.backups)?base.backups:[]);
  const keep=Math.max(1,Math.floor(ctrlBackupNumber(next.backupKeep,ctrlBackupNumber(base.backupKeep,50))));
  next.backups=backups;
  next.backupKeep=keep;
  next.backupCount=backups.length;
  next.backupTotalSize=backups.reduce((total,item)=>total+ctrlBackupNumber(item&&item.size,0),0);
  next.backupOverLimit=backups.length>keep;
  next.backupPruneAvailable=next.backupOverLimit;
  next.restoreAvailable=backups.length>0;
  return next;
}
function ctrlBackupViewState(ui){
  const details=ui&&ui.backupDetails,list=details&&details._backupList;
  return {open:!!(details&&details.open),scrollTop:list&&Number.isFinite(list.scrollTop)?list.scrollTop:0};
}
function ctrlBackupRestoreView(details,state){
  if(!details||!state||!state.open)return;
  details.open=true;
  const list=details._backupList;
  if(!list||!state.scrollTop)return;
  const restore=()=>{try{list.scrollTop=state.scrollTop;}catch(_){}};
  if(typeof requestAnimationFrame==="function")requestAnimationFrame(restore);else setTimeout(restore,0);
}
function ctrlUpdateSetBackupMutationLocked(ui,locked,updateBusy){
  if(!ui)return;
  const controls=Array.isArray(ui.backupMutationControls)?ui.backupMutationControls:[];
  for(const button of controls)ctrlUpdateSetButtonDisabled(button,locked);
  const note=ui.backupLockNotice;
  if(!note)return;
  const text=updateBusy?"Backup changes are locked while the dashboard update is in progress.":(ui.backupBusy?"Backup action in progress. Other backup and update changes are locked until it finishes.":"");
  note.textContent=text;
  note.style.display=text?"":"none";
}
function ctrlBackupUiIsLive(ui){
  return !!(ui&&ui.wrap&&typeof ctrlUpdateUi==="function"&&ctrlUpdateUi(ui.wrap)===ui);
}
function ctrlBackupControlsLocked(ui){
  const progress=ui&&ui.lastProgress||{};
  return !!(ui&&ui.backupBusy)||!!progress.active;
}
function ctrlBackupSummary(status){
  const count=ctrlBackupNumber(status&&status.backupCount,0),total=ctrlBackupNumber(status&&status.backupTotalSize,0),keep=ctrlBackupNumber(status&&status.backupKeep,50);
  return count?`${count} local backup${count===1?"":"s"} · ${fmtBytes(total)} · keep newest ${keep}`:`No local backups · keep newest ${keep}`;
}
function ctrlBuildBackupRestoreSection(ui,status){
  const section=el("div","ctrlbackuprestore");
  ui.backupSection=section;
  ctrlRefreshBackupRestoreSection(ui,status,{initial:true});
  return section;
}
function ctrlRefreshBackupRestoreSection(ui,patch,options){
  if(!ui||!ui.backupSection)return null;
  const opts=options||{},view=opts.initial?{open:false,scrollTop:0}:ctrlBackupViewState(ui);
  const status=ctrlBackupStatusPatch(ui.backupStatus,patch);
  ui.backupStatus=status;
  const section=ui.backupSection;
  const heading=el("div","ctrlsubhead","Backup & Restore");
  const summary=el("div","ctrlmini backupsummary",ctrlBackupSummary(status));
  const actions=el("div","ctrlrow actiongrid backuprestoreactions");
  const create=caction("Create backup","Save current settings and calendar links locally.","",async()=>ctrlRunBackupMutation(ui,"create"));
  create.classList.add("backupcreatebtn");
  const controls=[create];
  actions.appendChild(create);
  if(status.backupPruneAvailable){
    const prune=confirmBtn("Clean old backups","Confirm cleanup",async()=>ctrlRunBackupMutation(ui,"prune"));
    prune.classList.add("actionbtn","backupprunebtn");
    prune.innerHTML=`<span class="bt">Clean old backups</span><span class="bd">Keep the newest ${escapeHTML(String(status.backupKeep))}; remove older local archives.</span>`;
    prune.dataset.normalHtml=prune.innerHTML;
    controls.push(prune);actions.appendChild(prune);
  }
  const details=renderBackupRows(status.backups,status,{open:view.open,disabled:ctrlBackupControlsLocked(ui),onRestore:name=>ctrlRunBackupMutation(ui,"restore",name),onDelete:name=>ctrlRunBackupMutation(ui,"delete",name)});
  const rowControls=Array.isArray(details._backupMutationControls)?details._backupMutationControls:[];
  const lockNotice=el("div","ctrlmini backuplocknotice");
  section.replaceChildren(heading,summary,actions,details,lockNotice);
  ui.backupCreateButton=create;
  ui.backupPruneButton=controls.length>1?controls[1]:null;
  ui.backupDetails=details;
  ui.backupMutationControls=controls.concat(rowControls);
  ui.backupLockNotice=lockNotice;
  ctrlUpdateSetBackupMutationLocked(ui,ctrlBackupControlsLocked(ui),!!(ui.lastProgress&&ui.lastProgress.active));
  ctrlBackupRestoreView(details,view);
  return status;
}
function ctrlBackupMutationMessage(kind,result,name){
  if(kind==="create")return "Backup created: "+(result.name||"local backup")+" · "+(result.files||0)+" files."+(result.pruned?" Cleaned "+result.pruned+" older backup"+(result.pruned===1?".":"s."):"");
  if(kind==="restore")return "Restored "+(result.name||name||"selected backup")+". Safety backup saved as "+(result.preBackup||"pre-restore backup")+".";
  if(kind==="delete")return "Deleted backup: "+(result.deleted||name||"selected backup")+".";
  return "Cleaned old backups: removed "+(result.removedCount||0)+"; kept newest "+(result.keep||"configured")+".";
}
async function ctrlRunBackupMutation(ui,kind,name){
  if(!ui)return null;
  if(ui.backupBusy){ctrlMsg("A backup action is already running.");return null;}
  if(ui.lastProgress&&ui.lastProgress.active){ctrlMsg("Backup changes are locked while the dashboard update is in progress.");return null;}
  ui.backupBusy=true;
  if(ctrlBackupUiIsLive(ui))ctrlUpdateSetActionState(ui,ui.lastProgress||ctrlUpdateProgressState(ui.backupStatus||{}));
  ctrlMsg(kind==="restore"?"Creating a safety backup, then restoring the selected backup…":kind==="create"?"Creating local backup…":kind==="delete"?"Deleting selected backup…":"Cleaning old local backups…");
  try{
    let result;
    if(kind==="create")result=await api("/api/backup","POST",{});
    else if(kind==="restore")result=await api("/api/backup/restore","POST",{name:name});
    else if(kind==="delete")result=await api("/api/backup/delete","POST",{name:name});
    else result=await api("/api/backup/prune","POST",{keep:ui.backupStatus&&ui.backupStatus.backupKeep});
    ui.backupStatus=ctrlBackupStatusPatch(ui.backupStatus,result);
    if(kind==="restore"){
      await discoverCalendars();
      await loadCalendars();
      if(typeof loadSettings==="function")await loadSettings();
    }
    ctrlMsg(ctrlBackupMutationMessage(kind,result||{},name));
    if(typeof renderCtrlActionHistory==="function")try{await renderCtrlActionHistory();}catch(_){/* mutation succeeded; history refresh is secondary */}
    return result;
  }catch(error){
    ctrlMsg((kind==="restore"?"Restore":kind==="create"?"Backup":kind==="delete"?"Delete":"Backup cleanup")+" failed: "+(error&&error.message?error.message:error));
    if(typeof renderCtrlActionHistory==="function")try{await renderCtrlActionHistory();}catch(_){/* preserve original mutation error */}
    return null;
  }finally{
    ui.backupBusy=false;
    if(ctrlBackupUiIsLive(ui)){
      ctrlRefreshBackupRestoreSection(ui,ui.backupStatus);
      ctrlUpdateSetActionState(ui,ui.lastProgress||ctrlUpdateProgressState(ui.backupStatus||{}));
    }
  }
}
