import assert from "node:assert/strict";
import fs from "node:fs";
import vm from "node:vm";

const source=fs.readFileSync("ui/js/settings-display-sleep.js","utf8");
const listeners=new Map();
const calls=[];
const counts={calendar:0,agenda:0,weather:0,night:0,stale:0,pixel:0,theme:0};
const clock={hour:23,minute:0};
let apiMode="resolve";
let pendingOffResolve;
class FakeDate{ getHours(){return clock.hour;} getMinutes(){return clock.minute;} static now(){return 0;} }
function loadWeather(){counts.weather++;}
loadWeather._timer=0; loadWeather._retry=0;
const context=vm.createContext({
  SETTINGS:{displaySleepEnabled:true,displaySleepOff:"22:30",displaySleepOn:"06:00"},
  CTRL_OPEN:false,Date:FakeDate,clearTimeout(){},
  document:{hidden:false,addEventListener(type,handler){listeners.set(type,handler);}},
  api(path,method,body){
    calls.push({path,method,body});
    if(path==="/api/display/off"&&apiMode==="reject-off")return Promise.reject(new Error("off failed"));
    if(path==="/api/display/on"&&apiMode==="reject-on")return Promise.reject(new Error("on failed"));
    if(path==="/api/display/off"&&apiMode==="defer-off")return new Promise(resolve=>{pendingOffResolve=resolve;});
    return Promise.resolve({ok:true});
  },
  renderCalendar(){counts.calendar++;},renderAgenda(){counts.agenda++;},loadWeather,
  applyNightDim(){counts.night++;},updateStale(){counts.stale++;},applyPixelShift(){counts.pixel++;},checkTheme(){counts.theme++;},console,
});
vm.runInContext(source,context,{filename:"settings-display-sleep.js"});
const run=expression=>vm.runInContext(expression,context);
const state=()=>run('({sleeping:DISPLAY_SLEEPING,pending:DISPLAY_SLEEP_PENDING,override:DISPLAY_SLEEP_WAKE_OVERRIDE,reconcile:DISPLAY_SLEEP_RECONCILE_PENDING})');
assert.ok(listeners.has("touchstart")); assert.ok(listeners.has("pointerdown"));
apiMode="reject-off"; await run("checkDisplaySleep()");
assert.equal(state().sleeping,false); assert.equal(state().pending,""); assert.equal(state().override,false); assert.equal(state().reconcile,false);
apiMode="resolve"; context.CTRL_OPEN=true; const before=calls.length; await run("checkDisplaySleep()"); assert.equal(calls.length,before); context.CTRL_OPEN=false;
await run("checkDisplaySleep()"); assert.equal(state().sleeping,true); assert.equal(calls.at(-1).path,"/api/display/off");
await run("requestDisplayWake({override:true})"); assert.equal(state().sleeping,false); assert.equal(state().override,true); assert.equal(calls.at(-1).path,"/api/display/on"); assert.ok(counts.calendar>0&&counts.weather>0);
context.SETTINGS.displaySleepOff="22:31"; await run("checkDisplaySleep()"); assert.equal(state().sleeping,true);
listeners.get("touchstart")({}); assert.equal(state().sleeping,false); assert.equal(state().override,true);
const afterWake=calls.length; await run("checkDisplaySleep()"); assert.equal(calls.length,afterWake);
clock.hour=7; await run("checkDisplaySleep()"); assert.equal(state().override,false);
clock.hour=23; context.SETTINGS.displaySleepOff="22:32"; await run("checkDisplaySleep()"); assert.equal(state().sleeping,true);
clock.hour=7; await run("checkDisplaySleep()"); assert.equal(state().sleeping,false); assert.equal(calls.at(-1).path,"/api/display/on");
clock.hour=23; context.SETTINGS.displaySleepOff="22:33"; await run("checkDisplaySleep()"); assert.equal(state().sleeping,true);
context.SETTINGS.displaySleepEnabled=false; await run("checkDisplaySleep()"); assert.equal(state().sleeping,false); assert.equal(calls.at(-1).path,"/api/display/on");
context.SETTINGS.displaySleepEnabled=true; context.SETTINGS.displaySleepOff="22:34"; apiMode="defer-off";
const offCheck=run("checkDisplaySleep()"); await Promise.resolve(); assert.equal(state().pending,"off");
listeners.get("pointerdown")({}); assert.equal(state().sleeping,false); assert.equal(state().override,true);
pendingOffResolve({ok:true}); await offCheck; await Promise.resolve(); assert.equal(state().sleeping,false); assert.ok(calls.some(call=>call.path==="/api/display/on"));
console.log("PASS: display sleep uses success-based state, Control suppression, manual/physical/scheduled wake reconciliation, schedule disable recovery, and race-safe input override");
