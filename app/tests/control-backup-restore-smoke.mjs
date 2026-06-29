#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import {fileURLToPath} from "node:url";

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const read=rel=>fs.readFileSync(path.join(root,rel),"utf8");
const core=read("ui/js/control-ui.js");
const health=read("ui/js/control-status-health.js");
const updates=read("ui/js/control-updates.js");
const backups=read("ui/js/control-backups.js");
const css=read("ui/css/dashboard/control-status-maintenance.css");

assert.match(updates,/ctrlBuildBackupRestoreSection/,"Update card must compose Backup & Restore instead of orphaning backup helpers");
assert.match(backups,/function ctrlRefreshBackupRestoreSection/,"backup mutations must patch only their own subsection");
assert.match(backups,/function ctrlRunBackupMutation/,"manual backup, restore, delete, and prune need one serialized mutation path");
assert.match(backups,/if\(status\.backupPruneAvailable\)/,"cleanup must stay conditional on actual retention overflow");
assert.match(backups,/Backup changes are locked while the dashboard update is in progress/,"active update locking must be visible and explicit");
assert.match(backups,/loadCalendars\(\)/,"restore must refresh the local calendar model");
assert.match(backups,/loadSettings\(\)/,"restore must re-apply safely loadable dashboard settings");
assert.match(health,/details\._backupMutationControls=mutationControls/,"backup rows must expose Restore/Delete controls for scoped locking");
assert.match(css,/\.ctrlbackuprestore/,"Backup & Restore needs its own stable layout surface");
assert.match(css,/\.backuplocknotice/,"active update lock state needs a quiet explanatory note");

function node(tag,cls,text){
  const classes=new Set(String(cls||"").split(/\s+/).filter(Boolean));
  return {
    tag,className:cls||"",textContent:text||"",innerHTML:"",disabled:false,open:false,
    children:[],dataset:{},attrs:{},style:{display:"",minHeight:"",removeProperty(name){if(name==="min-height")this.minHeight="";}},
    isConnected:true,parentElement:null,scrollTop:0,
    appendChild(child){child.parentElement=this;this.children.push(child);return child;},
    append(...children){for(const child of children)this.appendChild(child);},
    replaceCount:0,
    replaceChildren(...children){this.replaceCount++;this.children=[];for(const child of children)this.appendChild(child);},
    setAttribute(key,value){this.attrs[key]=String(value);},getAttribute(key){return this.attrs[key]??null;},removeAttribute(key){delete this.attrs[key];},
    getBoundingClientRect(){return {height:280};},querySelector(){return null;},
    classList:{add(...names){for(const name of names)classes.add(name);},remove(...names){for(const name of names)classes.delete(name);},contains(name){return classes.has(name);}},
  };
}
const wrap=node("div","","");
const log=node("pre","","");
const page={classList:{contains:name=>name==="show"}};
const section={open:true};
wrap.closest=selector=>selector==="details.ctrlsec"?section:(selector===".ctrlpage"?page:null);
const requests=[];let actionHistoryRefreshes=0,calendarReloads=0,settingsReloads=0;
const initialBackups=[{name:"manual-old.zip",kind:"manual",reason:"Manual backup",version:"1.4.3-beta.96",createdAt:1,size:128}];
const afterCreate=[{name:"manual-new.zip",kind:"manual",reason:"Manual backup from Dashboard Control",version:"1.4.3-beta.102",createdAt:2,size:256},...initialBackups];
const context={
  CTRL_CACHE:{},CTRL_OPEN:true,clearTimeout(){},setTimeout(fn){return 1;},
  escapeHTML:value=>String(value).replace(/[&<>"']/g,ch=>({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;"}[ch])),
  el:node,document:{createElement:tag=>node(tag,"","")},bindTap(button,handler){button.handler=handler;},
  confirmBtn(label,armed,handler){const b=node("button","",label);b.handler=handler;return b;},
  $:id=>id==="#ctrlupdate"?wrap:(id==="#ctrlupdatelog"?log:null),fmtDateTime:()=>"now",fmtBytes:n=>String(n)+" B",
  ctrlMsg(){},ctrlCancelledError:()=>false,console:{warn(){}},
  discoverCalendars:async()=>{calendarReloads++;},loadCalendars:async()=>{calendarReloads++;},loadSettings:async()=>{settingsReloads++;},renderCtrlActionHistory:async()=>{actionHistoryRefreshes++;},
  api:async(url,method,body)=>{
    requests.push([url,method||"GET",body||null]);
    if(url==="/api/update/status")return {installedVersion:"1.4.3-beta.102",availability:{ok:true,status:"current",track:"beta",fetchedAt:1},preflight:{ready:true,label:"Ready",problems:[]},job:{state:"success",label:"Complete"},backups:initialBackups,backupKeep:50,backupCount:1,backupTotalSize:128,backupOverLimit:false};
    if(url==="/api/backup")return {ok:true,name:"manual-new.zip",files:4,size:256,backupKeep:50,pruned:0,backups:afterCreate};
    if(url==="/api/backup/restore")return {ok:true,name:body.name,restored:4,preBackup:"pre-restore-new.zip",backups:afterCreate};
    if(url==="/api/backup/delete")return {ok:true,deleted:body.name,backups:initialBackups};
    throw new Error("unexpected request "+url);
  },
};
vm.createContext(context);
vm.runInContext(core,context,{filename:"control-ui.js"});
vm.runInContext(health,context,{filename:"control-status-health.js"});
vm.runInContext(updates,context,{filename:"control-updates.js"});
vm.runInContext(backups,context,{filename:"control-backups.js"});
await vm.runInContext("renderCtrlUpdateRestore()",context);

assert.deepEqual(requests,[["/api/update/status","GET",null]],"initial card must read the complete backup-capable update status once");
assert.equal(wrap.children.length,5,"Backup & Restore must remain part of the stable Update-card shell");
const backupSection=wrap.children[3];
assert.equal(backupSection.className,"ctrlbackuprestore");
const create=backupSection.children[2].children[0];
assert.match(create.innerHTML,/Create backup/,"manual backup action must stay visible even when no dashboard update is available");
const cardReplacesBefore=wrap.replaceCount;
await create.handler();
assert.equal(requests[1][0],"/api/backup","Create backup must use the existing protected backup endpoint");
assert.equal(requests[1][1],"POST");
assert.equal(JSON.stringify(requests[1][2]),"{}");
assert.equal(wrap.replaceCount,cardReplacesBefore,"manual backup must patch only Backup & Restore, not rebuild the whole Update card");
assert.equal(wrap.children[3],backupSection,"manual backup must preserve the section shell and its open/scroll state");
assert.equal(actionHistoryRefreshes,1,"successful manual backup must refresh Recent Actions");

const afterCreateDetails=backupSection.children[3];
afterCreateDetails.open=true;
const afterCreateBody=afterCreateDetails.children[0];
const afterCreateList=afterCreateBody.children[0];
const afterCreateItem=afterCreateList.children[0];
const restore=afterCreateItem.children[1].children[0];
await restore.handler();
assert.equal(requests[2][0],"/api/backup/restore","Restore must use the exact selected backup name");
assert.equal(requests[2][1],"POST");
assert.equal(requests[2][2].name,"manual-new.zip");
assert.ok(calendarReloads>=2&&settingsReloads===1,"restore must reload calendar data and safe dashboard settings after the staged restore");
assert.equal(wrap.replaceCount,cardReplacesBefore,"restore must not tear down the complete Update card");
assert.equal(backupSection.children[3].open,true,"backup drawer open state must survive a scoped restore refresh");
const overflow=vm.runInContext('ctrlBackupStatusPatch({backupKeep:1,backups:[{size:1},{size:2}]},{})',context);
assert.equal(overflow.backupPruneAvailable,true,"retention cleanup must be offered only from an actual overflow state");

const afterRestoreDetails=backupSection.children[3];
const afterRestoreItem=afterRestoreDetails.children[0].children[0].children[0];
const deleteButton=afterRestoreItem.children[1].children[1];
await deleteButton.handler();
assert.equal(requests[3][0],"/api/backup/delete","Delete must apply only to the exact selected local archive");
assert.equal(requests[3][1],"POST");
assert.equal(requests[3][2].name,"manual-new.zip");
assert.equal(wrap.replaceCount,cardReplacesBefore,"delete must patch only Backup & Restore");
assert.ok(actionHistoryRefreshes>=3,"every completed backup mutation must refresh Recent Actions");

console.log("Control Backup & Restore smoke: visible, scoped, stable, and mutation-safe ok");
