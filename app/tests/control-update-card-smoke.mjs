#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import {fileURLToPath} from "node:url";

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const read=rel=>fs.readFileSync(path.join(root,rel),"utf8");
const core=read("ui/js/control-ui.js");
const updates=read("ui/js/control-updates.js");
const backups=read("ui/js/control-backups.js");
const health=read("ui/js/control-status-health.js");
const navigation=read("ui/js/control-navigation.js");
const lock=read("ui/js/control-location-lock.js");
const lifecycle=read("ui/js/control-lifecycle.js");
assert.ok(!lock.includes("function closeCtrl()"),"location/PIN source must not retain the Dashboard Control close lifecycle");
assert.match(lifecycle,/async function openCtrl\(\)/,"Dashboard Control open lifecycle must live in its dedicated module");
const css=read("ui/css/dashboard/control-theme-actions-lite.css");

assert.match(core,/function ctrlStat\(label,value,state\)/,"Control core must export the shared escaped stat-tile factory");
assert.match(updates,/\/api\/update\/progress/,"active update polling must use the lightweight progress endpoint");
assert.match(updates,/function ctrlUpdateApplyProgress\(/,"update card must patch a stable live shell instead of rebuilding it on each poll");
assert.match(updates,/function updateJobPresentation\(/,"update card must show the updater's human-readable phase label rather than the coarse durable state name");
assert.match(updates,/ctrlUpdateSetStat\(ui\.rows\.job,"Job",job\.label,job\.state\)/,"live Job stat must render the phase label while retaining state-derived severity");
assert.match(updates,/function ctrlUpdatePollCanRun\(/,"update polling must stop outside the visible Update card");
assert.match(updates,/function ctrlUpdateBindTrackToggle\(/,"the existing Track tile must own the concealed channel gesture");
assert.match(updates,/maxTaps:6/,"the concealed channel gesture must require six consecutive taps");
assert.match(updates,/\/api\/update\/track\/toggle/,"the concealed channel gesture must use the dedicated protected endpoint");
assert.match(updates,/function ctrlUpdateDispose\([\s\S]*trackTapDispose/,"Update-card rebuilds must dispose the Track tile gesture listeners");
const pollSource=updates.slice(updates.indexOf("async function runCtrlUpdatePoll"),updates.indexOf("function ctrlUpdateFinishProgress"));
assert.doesNotMatch(pollSource,/renderCtrlUpdateRestore/,"active polling must not call the full card renderer");
assert.match(updates,/function ctrlUpdateFinishProgress[\s\S]*renderCtrlUpdateRestore\(\{terminal:true\}\)/,"only a terminal state may request one final full status refresh");
assert.match(navigation,/d\.dataset&&d\.dataset\.lazy===\"update\"&&typeof stopCtrlUpdatePoll===\"function\"/,"closing the Update card must stop its poll immediately");
assert.match(navigation,/oldPage&&oldPage\.id===\"ctrlpage-system\"&&typeof stopCtrlUpdatePoll===\"function\"/,"leaving the System tab must stop its update poll");
assert.match(lifecycle,/function closeCtrl\(\)[\s\S]*stopCtrlUpdatePoll\(\)/,"closing Dashboard Control must stop any update poll");
assert.match(updates,/Check for updates/,"update card must expose an explicit read-only catalog check independent of updater setup");
assert.match(updates,/\/api\/update\/status\?fresh=1/,"explicit catalog checks must bypass the short update-status cache");
assert.match(updates,/Update installation setup:/,"update card must distinguish privileged updater setup from catalog availability");
assert.doesNotMatch(health,/saved update credentials/i,"health messaging must not describe the token-free GitHub updater as credential-gated");
assert.match(health,/local update service/,"health messaging must name the local updater setup requirement");
assert.match(updates,/ctrlBuildBackupRestoreSection/,"Update card must compose the durable Backup & Restore subsection");
assert.match(backups,/function ctrlRunBackupMutation/,"Backup mutations must stay in the stable Update-card subsection");
assert.match(backups,/\/api\/backup\/restore/,"Backup subsection must retain restore support");
assert.match(backups,/\/api\/backup\/delete/,"Backup subsection must retain delete support");
assert.match(backups,/\/api\/backup\/prune/,"Backup subsection must retain conditional cleanup support");
assert.match(health,/function renderBackupRows/,"Backup row renderer must remain available to the Update card");
for(const token of [".updategrid .stat.ok",".updategrid .stat.warn",".updategrid .stat.bad"]){
  assert.ok(css.includes(token),`update metric styling missing ${token}`);
}

function node(tag,cls,text){
  const classes=new Set(String(cls||"").split(/\s+/).filter(Boolean));
  return {
    tag, className:cls||"", textContent:text||"", innerHTML:"", disabled:false,
    children:[], dataset:{}, attrs:{}, style:{display:"",minHeight:"",removeProperty(name){if(name==="min-height")this.minHeight="";}},
    isConnected:true, parentElement:null,
    appendChild(child){child.parentElement=this;this.children.push(child);return child;},
    append(...children){for(const child of children)this.appendChild(child);},
    replaceCount:0,
    replaceChildren(...children){this.replaceCount++;this.children=[];for(const child of children)this.appendChild(child);},
    setAttribute(key,value){this.attrs[key]=String(value);},
    getAttribute(key){return this.attrs[key]??null;},
    removeAttribute(key){delete this.attrs[key];},
    getBoundingClientRect(){return {height:280};},
    querySelector(){return null;},
    classList:{
      add(...names){for(const name of names)classes.add(name);},
      remove(...names){for(const name of names)classes.delete(name);},
      contains(name){return classes.has(name);}
    },
  };
}
const wrap=node("div","","");
const log=node("pre","","");
const page={classList:{contains:name=>name==="show"}};
const section={open:true};
wrap.closest=selector=>selector==="details.ctrlsec"?section:(selector===".ctrlpage"?page:null);
const requests=[];
let statusActive=true,selectedTrack="beta";
const timers=new Map();let timerID=0;
const context={
  CTRL_CACHE:{},CTRL_OPEN:true,
  clearTimeout(id){timers.delete(id);},
  setTimeout(fn,delay){const id=++timerID;timers.set(id,{fn,delay});return id;},
  escapeHTML:value=>String(value).replace(/[&<>"']/g,ch=>({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;"}[ch])),
  el:node,
  document:{createElement:tag=>node(tag,"","")},
  bindTap(button,handler){button.handler=handler;},
  attachTaps(tile,options){tile.tapOptions=options;return ()=>{tile.tapDisposed=true;};},
  confirmBtn(label,armed,handler){const b=node("button","",label);b.handler=handler;return b;},
  $:id=>id==="#ctrlupdate"?wrap:(id==="#ctrlupdatelog"?log:null),
  fmtDateTime:()=>"now",
  ctrlCancelledError:()=>false,
  ctrlShowOutputConsole(){},ctrlMsg(){},console:{warn(){}},
  api:async(url)=>{
    requests.push(url);
    if(url==="/api/update/progress")return {
      state:"validating-payload",active:true,label:"Downloading release package",detail:"Downloading Dash-Go available-test-version. Live dashboard files remain unchanged until every verification step passes.",source:"control",track:selectedTrack,
      job:{id:"update-test",state:"validating-payload",label:"Downloading release package",detail:"Downloading Dash-Go available-test-version. Live dashboard files remain unchanged until every verification step passes.",source:"control",track:selectedTrack},
    };
    if(url==="/api/update/track/toggle"){selectedTrack=selectedTrack==="beta"?"stable":"beta";return {ok:true,track:selectedTrack};}
    if(!url.startsWith("/api/update/status"))throw new Error("unexpected request "+url);
    return {
      installedVersion:"installed-test-version",
      availability:{availableVersion:"available-test-version",updateAvailable:true,ok:true,track:selectedTrack,fetchedAt:1,status:"available"},
      preflight:{ready:true,label:"Ready",problems:[]},
      job:statusActive?{id:"update-test",state:"starting",detail:"Dedicated updater service accepted the job.",source:"control",track:selectedTrack}:{id:"update-test",state:"success",label:"Complete",detail:"Update complete.",source:"control",track:selectedTrack},
      updateLogMtime:1,
    };
  },
};
vm.createContext(context);
vm.runInContext(core,context,{filename:"control-ui.js"});
vm.runInContext(health,context,{filename:"control-status-health.js"});
vm.runInContext(updates,context,{filename:"control-updates.js"});
vm.runInContext(backups,context,{filename:"control-backups.js"});
await vm.runInContext("renderCtrlUpdateRestore()",context);

assert.deepEqual(requests,["/api/update/status"],"initial Update card render must read full status once");
assert.equal(wrap.children.length,5,"live Update card must render metrics, detail, actions, Backup & Restore, and the operational note");
const grid=wrap.children[0];
assert.equal(grid.className,"ctrlgrid updategrid");
assert.equal(grid.children.length,6,"update card must render every status metric");
assert.equal(grid.children[1].className,"stat warn","an available catalog remains visible while an update is live");
const trackTile=grid.children[2];
assert.equal(trackTile.className,"stat quiet","the Track object must retain its ordinary stat-tile appearance");
assert.deepEqual(trackTile.attrs,{},"the Track object must not advertise a button, tooltip, or extra action attributes");
assert.equal(trackTile.tapOptions?.maxTaps,6,"the Track object must require six taps before any channel change");
assert.equal(trackTile.tapOptions?.gap,650,"the Track object must treat a fast six-tap sequence as one gesture");
await trackTile.tapOptions.onTaps();
assert.equal(requests.filter(url=>url==="/api/update/track/toggle").length,0,"the Track gesture must not switch channels while an update is active");
const actions=wrap.children.find(child=>child.className==="ctrlrow actiongrid");
const check=actions.children[0];
const update=actions.children[1];
assert.ok(check&&update,"update card must retain both update actions in its stable shell");
assert.match(check.innerHTML,/Check for updates/);
assert.match(update.innerHTML,/Update in progress/);
assert.equal(check.disabled,true,"catalog checks must lock while a live update is in progress");
assert.equal(update.disabled,true,"the update action must lock while the updater is active");
const backupSection=wrap.children[3];
assert.equal(backupSection.className,"ctrlbackuprestore","Update card must expose a stable Backup & Restore subsection");
const backupActions=backupSection.children[2];
assert.match(backupActions.children[0].innerHTML,/Create backup/,"manual backup must remain visible during a live update");
assert.equal(backupActions.children[0].disabled,true,"backup mutation controls must lock while an update is active");
assert.match(backupSection.children[4].textContent,/locked while the dashboard update is in progress/,"active updates must explain why backup mutations are disabled");

const firstTimer=[...timers.values()].find(item=>item.delay===1500);
assert.ok(firstTimer,"active update state must schedule one lightweight progress poll");
const replacesBeforePoll=wrap.replaceCount;
await firstTimer.fn();
assert.deepEqual(requests,["/api/update/status","/api/update/progress"],"a live poll must make one lightweight progress request, not a second full status request");
assert.equal(wrap.replaceCount,replacesBeforePoll,"progress polling must not replace the Update card shell or its action buttons");
assert.equal(wrap.children[3],backupSection,"progress polling must not rebuild or collapse the Backup & Restore subsection");
assert.equal(grid.children[4].className,"stat warn","the Job tile must patch in place to the active updater state");
assert.match(grid.children[4].innerHTML,/Downloading release package/,"the Job tile must show the precise human-readable phase instead of validating-payload");
assert.match(wrap.children[1].textContent,/Live dashboard files remain unchanged/,"the live detail line must explain the current transaction safety boundary");

statusActive=false;
vm.runInContext('ctrlUpdateApplyProgress($("#ctrlupdate"),{state:"success",active:false,terminal:true,label:"Complete",detail:"Update complete."});',context);
await trackTile.tapOptions.onTaps();
assert.equal(requests.filter(url=>url==="/api/update/track/toggle").length,1,"the sixth-tap gesture must toggle the saved channel once after the update is terminal");
assert.equal(trackTile.tapDisposed,true,"the former Track gesture must be cleaned up before the refreshed card replaces it");
const refreshedGrid=wrap.children[0];
assert.equal(refreshedGrid.children[2].className,"stat quiet","the refreshed Track tile must keep the original visual treatment");
assert.match(refreshedGrid.children[2].innerHTML,/Track[\s\S]*stable/,"the refreshed ordinary stat tile must show the newly selected channel");
assert.deepEqual(refreshedGrid.children[2].attrs,{},"the refreshed Track tile must not grow visible action attributes");
assert.equal(refreshedGrid.children[2].tapOptions?.maxTaps,6,"the refreshed Track tile must retain the concealed six-tap behavior");

console.log("Control update card smoke: stable progress polling, action locking, and concealed channel gesture ok");
