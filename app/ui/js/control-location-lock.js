function locationPreviewSrc(lat,lon,z){
  const la=Number(lat), lo=Number(lon), zoom=z||10;
  if(!Number.isFinite(la)||!Number.isFinite(lo)) return "";
  return "/api/event-map-img?lat="+encodeURIComponent(la.toFixed(6))+"&lon="+encodeURIComponent(lo.toFixed(6))+"&z="+encodeURIComponent(zoom);
}
function locationMapPreview(lat,lon,label){
  const card=el("div","locmapcard");
  const img=document.createElement("img");
  img.className="locmapimg";
  img.alt="Static OpenStreetMap preview for "+(label||"configured location");
  img.loading="lazy";
  img.src=locationPreviewSrc(lat,lon,10);
  const fallback=el("div","locmapfallback","Map preview unavailable");
  img.onerror=()=>{ img.style.display="none"; fallback.classList.add("show"); };
  card.append(img,fallback);
  return card;
}
async function renderCtrlLocation(){
  const row=$("#ctrlloc"); if(!row)return;
  row.replaceChildren(); row.classList.add("locpanel");
  const lat=Number(CONFIG.lat),lon=Number(CONFIG.lon),name=CONFIG.locationName||"Configured location";
  const top=el("div","loccard"),info=el("div","locinfo");
  info.append(el("div","loceyebrow","Current location"),el("div","loctitle",name),el("div","loccoords",Number.isFinite(lat)&&Number.isFinite(lon)?lat.toFixed(4)+", "+lon.toFixed(4):"Coordinates unavailable"));
  const actions=el("div","locactions"); actions.appendChild(caction("Change location","Search by city.","",()=>locEditor(row))); info.appendChild(actions);
  top.append(locationMapPreview(lat,lon,name),info); row.appendChild(top);
}
async function renderCtrlMoon(){
  const row=$("#ctrlmoon"); if(!row)return;
  row.replaceChildren();
  const actions=actionGroup("Moon calendar","Generated phase events follow the configured location.","displaygroup displaycompact");
  actions.grid.appendChild(caction("Update moon","Refresh moon calendar.","",async()=>{
    ctrlMsg("Updating location-aware moon calendar…");
    try{const m=await api("/api/moon/update","POST",{});ctrlMsg("Moon calendar updated"+(m.eventCount!=null?": "+m.eventCount+" phase events":"")+".");await discoverCalendars();await loadCalendars();await renderCtrlMoon();}
    catch(e){ctrlMsg("Moon calendar update failed: "+e.message);}
  }));
  row.appendChild(actions.group);
  try{
    const m=await api("/api/moon/status");
    const state=m.stale?"warn":(m.enabled?"ok":"unknown");
    const title=m.enabled?(m.stale?"Moon calendar needs update":"Moon calendar current"):"Moon calendar disabled";
    const detail=m.enabled?((m.city?m.city+" · ":"")+(m.eventCount!=null?m.eventCount+" phase events":"generated when enabled")+(m.timezone?" · "+m.timezone:"")):"Enable Moon phases in generated calendars to show moon events.";
    row.appendChild(ctrlStateCard(state,title,detail));
  }catch(e){row.appendChild(ctrlStateCard("warn","Moon calendar status unavailable",e.message));}
}
function locEditor(row){
  row.innerHTML=""; row.classList.add("locpanel"); _oskTarget=null;
  const shell=el("div","loceditor");
  shell.appendChild(ctrlStateCard("info","Change location","Search a city, then use the static map preview to verify the region before saving."));
  const q=oskInput("city name (e.g. Chicago)","");
  shell.appendChild(msgField("Search",q,"Use city name first; add state only if needed."));
  const res=el("div","locresults");
  const buttons=el("div","ctrlrow compact ctrlstickyactions");
  const searchLocation=async()=>{
    const term=q.value.trim();
    if(!term){ ctrlMsg("Type a city name first."); showOSKFor(q); return; }
    ctrlMsg("Searching…"); res.innerHTML="";
    try{
      const j=await api("/api/geocode?q="+encodeURIComponent(term));
      if(!j.results||!j.results.length){ ctrlMsg("No matches — try just the city name, no state/ZIP."); return; }
      ctrlMsg("Pick the matching location.");
      for(const m of j.results){ res.appendChild(locationResultCard(m)); }
    }catch(e){ ctrlMsg("Search failed: "+e.message); }
  };
  const go=cbtn("Search","on",searchLocation);
  oskSetSubmit(q,"Search",()=>go.click());
  const cancel=cbtn("Cancel","",()=>renderCtrlLocation());
  buttons.append(go,cancel); shell.appendChild(buttons); shell.appendChild(res); row.appendChild(shell); buildOSK(row);
  function locationResultCard(m){
    const card=el("div","locresult");
    card.appendChild(locationMapPreview(m.lat,m.lon,m.label));
    const info=el("div","locresultinfo");
    info.appendChild(el("div","loctitle",m.label||m.city||"Location"));
    info.appendChild(el("div","loccoords",Number(m.lat).toFixed(4)+", "+Number(m.lon).toFixed(4)));
    info.appendChild(cbtn("Use this location","on",async()=>{
      try{
        const r=await api("/api/location","POST",{lat:m.lat,lon:m.lon,city:m.city});
        CONFIG.lat=m.lat; CONFIG.lon=m.lon; CONFIG.locationName=m.city||m.label||"";
        const moon=r && r.moon && r.moon.ok ? " Moon calendar refreshed." : "";
        ctrlMsg("Location set to "+(m.label||m.city)+" — updating weather."+moon);
        loadWeather(); loadAlerts(); discoverCalendars(); loadCalendars(); renderCtrlLocation();
      }catch(e){ ctrlMsg(e.message); }
    }));
    card.appendChild(info);
    return card;
  }
}

async function controlLockStatus(){
  try{
    const headers=CTRL_TOKEN?{"X-Dashboard-Token":CTRL_TOKEN}:{};
    const res=await fetch("/api/lock/status",{cache:"no-store",headers});
    CTRL_LOCK_STATUS=await res.json();
    return CTRL_LOCK_STATUS;
  }catch(_){ CTRL_LOCK_STATUS={enabled:false}; return CTRL_LOCK_STATUS; }
}
async function controlLockEnabled(){
  const st=await controlLockStatus();
  return !!st.enabled;
}

// Keep Control shell geometry in CSS. PIN state only decides whether the
// main region participates in the accessibility tree and focus order; when it
// becomes visible again, the stylesheet restores its required flex layout.
function setCtrlMainVisible(visible){
  const main=$("#ctrlmain");
  if(!main)return;
  main.hidden=!visible;
  main.setAttribute("aria-hidden",visible?"false":"true");
}

function pinDigits(v){return String(v||"").replace(/\D/g,"").slice(0,8);}
function pinMask(v){v=pinDigits(v); return v ? "•".repeat(v.length) : "—";}
function renderPinForm(wrap,mode,status){
  wrap.innerHTML="";
  const title = mode==="remove" ? "Remove PIN protection" : (mode==="change" ? "Change PIN" : "Set PIN");
  wrap.appendChild(ctrlStateCard(mode==="remove"?"warn":"info",title,
    mode==="remove" ? "Removing the PIN makes Dashboard Control and the saved chalkboard available without unlocking." :
    "Use 4–8 digits. The PIN is hashed on the dashboard and is not stored as plain text."));
  const fields = mode==="set"
    ? [{k:"pin",label:"New PIN"},{k:"confirm",label:"Confirm PIN"}]
    : (mode==="remove" ? [{k:"currentPin",label:"Current PIN"}] :
       [{k:"currentPin",label:"Current PIN"},{k:"pin",label:"New PIN"},{k:"confirm",label:"Confirm PIN"}]);
  const vals={}; fields.forEach(f=>vals[f.k]="");
  let active=fields[0].k;
  const form=el("div","pinmanage");
  const fieldBox=el("div","pinfields");
  function drawFields(){
    fieldBox.innerHTML="";
    fields.forEach(f=>{
      const row=el("button","pinfield"+(active===f.k?" on":""));
      row.type="button";
      row.innerHTML=`<span>${escapeHTML(f.label)}</span><b>${escapeHTML(pinMask(vals[f.k]))}</b>`;
      bindTap(row,()=>{active=f.k; drawFields();});
      fieldBox.appendChild(row);
    });
  }
  const pad=el("div","pingrid pinmanagegrid");
  function drawNotice(msg,kind){
    const n=form.querySelector(".pinstatus");
    if(n){ n.className="pinstatus"+(kind?" "+kind:""); n.textContent=msg||""; }
  }
  function addDigit(n){
    vals[active]=pinDigits((vals[active]||"")+n);
    drawFields(); drawNotice("");
  }
  async function submitPinForm(){
    try{
      if(mode!=="remove"){
        if(pinDigits(vals.pin).length<4){ drawNotice("New PIN must be 4–8 digits.","warn"); return; }
        if(vals.pin!==vals.confirm){ drawNotice("New PIN and confirmation do not match.","warn"); return; }
      }
      if(mode!=="set" && status && status.enabled && pinDigits(vals.currentPin).length<4){
        drawNotice("Enter the current PIN.","warn"); return;
      }
      const body={};
      if(vals.currentPin) body.currentPin=pinDigits(vals.currentPin);
      if(vals.pin) body.pin=pinDigits(vals.pin);
      if(status && status.timeout) body.timeout=status.timeout;
      const path=mode==="remove"?"/api/lock/remove":(mode==="change"?"/api/lock/change":"/api/lock/set");
      const r=await api(path,"POST",body);
      if(r.token){ CTRL_TOKEN=r.token; SAFE_SESSION.set("dashboardControlToken",CTRL_TOKEN); }
      if(mode==="remove"){ CTRL_TOKEN=""; SAFE_SESSION.remove("dashboardControlToken"); }
      CTRL_LOCK_STATUS={...(CTRL_LOCK_STATUS||{}),...r};
      ctrlMsg(mode==="remove"?"PIN removed.":"PIN saved.");
      await renderCtrlSecurity();
    }catch(e){ drawNotice(e.message||String(e),"warn"); }
  }
  ["1","2","3","4","5","6","7","8","9"].forEach(n=>pad.appendChild(cbtn(n,"pinbtn",()=>addDigit(n))));
  pad.appendChild(cbtn("Clear","pinbtn small",()=>{ vals[active]=""; drawFields(); }));
  pad.appendChild(cbtn("0","pinbtn",()=>addDigit("0")));
  pad.appendChild(cbtn("Next","pinbtn small on",async()=>{
    const i=fields.findIndex(f=>f.k===active);
    if(i>=fields.length-1) return submitPinForm();
    active=fields[Math.min(fields.length-1,i+1)].k;
    drawFields();
  }));
  const notice=el("div","pinstatus","");
  const actions=el("div","ctrlrow compact ctrlstickyactions");
  actions.appendChild(cbtn(mode==="remove"?"Remove PIN":(mode==="change"?"Save PIN":"Set PIN"),mode==="remove"?"danger":"on",submitPinForm));
  actions.appendChild(cbtn("Cancel","",()=>renderCtrlSecurity()));
  drawFields();
  form.append(fieldBox,pad,notice,actions);
  wrap.appendChild(form);
}
function lockTimingChoice(opt,current,onPick){
  const selected=current===opt.value;
  return caction((selected?"✓ ":"")+opt.label,
    opt.seconds==null?"Valid until reboot":(opt.seconds?opt.seconds+" seconds":"Requires PIN every open"),
    selected?"on":"",()=>onPick(opt));
}
function lockTimingCluster(label,kind,options,current,onPick){
  const cluster=el("section","locktiming-cluster");
  const grid=el("div","locktiming-grid locktiming-grid-"+kind);
  cluster.appendChild(el("div","locktiming-label",label));
  options.forEach(opt=>grid.appendChild(lockTimingChoice(opt,current,onPick)));
  cluster.appendChild(grid);
  return cluster;
}
async function renderCtrlSecurity(){
  const wrap=$("#ctrlsecurity"); if(!wrap) return;
  let st=CTRL_LOCK_STATUS;
  try{ st=await controlLockStatus(); }catch(_){ st={enabled:false}; }
  wrap.innerHTML="";
  const unavailable=!!(st&&st.available===false);
  const enabled=!!(st&&st.enabled);
  const unlocked=!unavailable&&(!enabled || !!CTRL_TOKEN || !!(st&&st.unlocked));
  if(unavailable){
    wrap.appendChild(ctrlStateCard("warn","PIN protection unavailable",st.error||"The local PIN configuration cannot be read. Use the documented local recovery flag, then restart the dashboard control server."));
    return;
  }
  wrap.appendChild(ctrlStateCard(enabled?"good":"info",
    "PIN protection: "+(enabled?"On":"Off"),
    enabled ? "Persistent chalkboard storage and Dashboard Control settings require an unlocked session. Locked users get a temporary scratch board only." :
              "No PIN is configured. Dashboard Control and the saved chalkboard are available without unlocking."));
  if(enabled && st && st.timeoutLabel){
    wrap.appendChild(ctrlStateCard("info","Unlock duration",st.timeoutLabel));
  }
  const row=el("div","ctrlrow compact");
  if(!enabled){
    row.appendChild(cbtn("Set PIN","on",()=>renderPinForm(wrap,"set",st)));
  }else{
    row.appendChild(cbtn("Change PIN","",()=>renderPinForm(wrap,"change",st)));
    row.appendChild(cbtn("Remove PIN","danger",()=>renderPinForm(wrap,"remove",st)));
    row.appendChild(cbtn("Lock now","",async()=>{
      const tok=CTRL_TOKEN; CTRL_TOKEN=""; SAFE_SESSION.remove("dashboardControlToken");
      if(tok) await fetch("/api/lock/revoke",{method:"POST",headers:{"X-Dashboard-Token":tok,"Content-Type":"application/json"},body:"{}"}).catch(()=>{});
      CTRL_LOCK_STATUS=await controlLockStatus();
      ctrlMsg("Dashboard Control locked.");
      closeCtrl();
    }));
  }
  wrap.appendChild(row);
  if(enabled && Array.isArray(st.options) && st.options.length){
    const timing=actionGroup("Lock timing","How long an unlock session remains valid.","displaygroup displaycompact locktiming");
    timing.grid.className="locktiming-body";
    const applyTiming=async opt=>{
      try{
        const r=await api("/api/lock/config","POST",{timeout:opt.value});
        if(r.token){ CTRL_TOKEN=r.token; SAFE_SESSION.set("dashboardControlToken",CTRL_TOKEN); }
        else if(r.sessionRefreshed && CTRL_TOKEN){ SAFE_SESSION.set("dashboardControlToken",CTRL_TOKEN); }
        CTRL_LOCK_STATUS={...(CTRL_LOCK_STATUS||{}),...r,unlocked:true};
        if(typeof ctrlRefreshEveryOpenHeartbeat==="function")ctrlRefreshEveryOpenHeartbeat();
        ctrlMsg("PIN timing updated.");
        await renderCtrlSecurity();
      }catch(e){ ctrlMsg(e.message); }
    };
    const shortOptions=st.options.filter(opt=>opt.seconds!=null&&Number(opt.seconds)<=300);
    const longOptions=st.options.filter(opt=>!shortOptions.includes(opt));
    if(shortOptions.length) timing.grid.appendChild(lockTimingCluster("Short unlocks","short",shortOptions,st.timeout,applyTiming));
    if(longOptions.length) timing.grid.appendChild(lockTimingCluster("Longer unlocks","long",longOptions,st.timeout,applyTiming));
    wrap.appendChild(timing.group);
  }
}
function showPinLock(){
  const pin=$("#ctrlpin"), panel=$("#ctrlpanel");
  if(panel) panel.classList.add("pinlocked");
  setCtrlMainVisible(false);
  pin.classList.add("show"); pin.innerHTML="";
  let val="", lockoutUntil=0, lockTimer=null;
  const title=el("div","pintitle","Enter dashboard passcode");
  const dots=el("div","dots","");
  const notice=el("div","pinstatus","");
  const grid=el("div","pingrid");
  pin.append(title,dots,notice,grid);
  function remaining(){ return Math.max(0, Math.ceil((lockoutUntil-Date.now())/1000)); }
  function setButtonsDisabled(disabled){
    grid.querySelectorAll("button").forEach(b=>{
      if((b.textContent||"").trim()==="Clear") b.disabled=false;
      else b.disabled=!!disabled;
    });
  }
  function draw(){ dots.textContent=val?"•".repeat(val.length):"—"; }
  function setNotice(msg,kind){
    notice.textContent=msg||"";
    notice.className="pinstatus"+(kind?" "+kind:"");
  }
  function clearTimer(){ if(lockTimer){ clearInterval(lockTimer); lockTimer=null; } }
  function setLockout(seconds){
    const sec=Math.max(1, parseInt(seconds||0,10)||1);
    lockoutUntil=Date.now()+sec*1000;
    val=""; draw();
    clearTimer();
    const tick=()=>{
      const rem=remaining();
      if(rem<=0){
        clearTimer(); lockoutUntil=0; setButtonsDisabled(false);
        setNotice("Lockout ended. Enter the passcode again.","ok");
        return;
      }
      setButtonsDisabled(true);
      setNotice("Too many wrong passcode attempts. Try again in "+rem+" second"+(rem===1?"":"s")+".","warn");
    };
    tick(); lockTimer=setInterval(tick,1000);
  }
  async function submit(){
    if(remaining()>0){ setLockout(remaining()); return; }
    if(val.length<4){ setNotice("Enter 4–8 digits.","warn"); return; }
    try{
      const r=await api("/api/lock/unlock","POST",{pin:val});
      clearTimer();
      CTRL_TOKEN=r.token||""; SAFE_SESSION.set("dashboardControlToken",CTRL_TOKEN);
      if(r.timeout) CTRL_LOCK_STATUS={...(CTRL_LOCK_STATUS||{}), timeout:r.timeout, timeoutLabel:r.timeoutLabel};
      pin.classList.remove("show");
      if(panel) panel.classList.remove("pinlocked");
      setCtrlMainVisible(true);
      ctrlMsg(""); await renderCtrlAll();
      if(typeof ctrlRefreshEveryOpenHeartbeat==="function")ctrlRefreshEveryOpenHeartbeat();
      if(typeof ctrlOpenPendingSection==="function")ctrlOpenPendingSection();
      if(typeof ctrlScheduleCacheBudgetProbe==="function")ctrlScheduleCacheBudgetProbe();
    }catch(e){
      const msg=String(e&&e.message?e.message:e||"");
      val=""; draw();
      let sec=0;
      const m=msg.match(/(?:try again in|in)\s+(\d+)\s*s/i);
      if(m) sec=parseInt(m[1],10);
      if(/too many|lockout|429/i.test(msg)){
        setLockout(sec||60);
      }else{
        setNotice("Wrong passcode. Try again.","warn");
      }
      ctrlMsg("");
    }
  }
  const nums=["1","2","3","4","5","6","7","8","9"];
  for(const n of nums){ grid.appendChild(cbtn(n,"pinbtn",()=>{ if(remaining()>0){ setLockout(remaining()); return; } if(val.length<8){ val+=n; draw(); setNotice("",""); } })); }
  grid.appendChild(cbtn("Clear","pinbtn small",()=>{ val=""; draw(); if(!remaining()) setNotice("",""); }));
  grid.appendChild(cbtn("0","pinbtn",()=>{ if(remaining()>0){ setLockout(remaining()); return; } if(val.length<8){ val+="0"; draw(); setNotice("",""); } }));
  grid.appendChild(cbtn("OK","pinbtn small on",submit));
  draw();
  if(CTRL_LOCK_STATUS&&CTRL_LOCK_STATUS.available===false){
    setButtonsDisabled(true);
    setNotice(CTRL_LOCK_STATUS.error||"PIN protection configuration is unavailable. Use local recovery.","warn");
    return;
  }
  const wait=(CTRL_LOCK_STATUS&&CTRL_LOCK_STATUS.lockoutRemaining)||0;
  if(wait>0) setLockout(wait);
}
