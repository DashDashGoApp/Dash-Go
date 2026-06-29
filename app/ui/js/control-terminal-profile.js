function collapseCtrlSections(){
  document.querySelectorAll("#ctrl details.ctrlsec").forEach(d=>{
    d.open=false;
    if(ctrlLiteProfile() && d.dataset && d.dataset.lazy) d.dataset.loaded="0";
  });
  openDefaultCtrlSection();
}

// Two-tap arm/confirm wrapper for destructive actions (stray-touch-proof).
function confirmBtn(label,armedLabel,fn){
  const b=cbtn(label,"danger",async()=>{
    const normalHTML=b.dataset.normalHtml || b.innerHTML || escapeHTML(label);
    b.dataset.normalHtml=normalHTML;
    if(!b.classList.contains("armed")){
      b.classList.add("armed");
      if(b.classList.contains("actionbtn")){
        b.innerHTML=`<span class="bt">${escapeHTML(armedLabel)}</span><span class="bd">Tap once more to confirm.</span>`;
      }else{
        b.textContent=armedLabel;
      }
      setTimeout(()=>{
        b.classList.remove("armed");
        if(b.classList.contains("actionbtn")) b.innerHTML=b.dataset.normalHtml || normalHTML;
        else b.textContent=label;
      },5000);
      return;
    }
    b.classList.remove("armed");
    if(b.classList.contains("actionbtn")) b.innerHTML=b.dataset.normalHtml || normalHTML;
    else b.textContent=label;
    await fn(b);
  });
  return b;
}
function ctrlTerminalSection(){return document.querySelector("#ctrlpage-system .ctrlsec-terminal");}
function syncTerminalAccessCard(enabled){
  const section=ctrlTerminalSection(),available=enabled!==false;
  if(!section)return available;
  section.hidden=!available;
  if(!available){
    section.open=false;
    section.dataset.loaded="0";
    if(typeof ctrlClearNode==="function")ctrlClearNode($("#ctrlterminal"));
  }
  return available;
}
async function refreshTerminalAccessCard(){
  const section=ctrlTerminalSection();if(!section)return;
  const cached=CTRL_CACHE["/api/terminal/status"];
  if(cached&&typeof cached.enabled==="boolean"){syncTerminalAccessCard(cached.enabled);return;}
  try{
    const st=await api("/api/terminal/status","GET",null,null);
    CTRL_CACHE["/api/terminal/status"]=st;
    syncTerminalAccessCard(st&&st.enabled!==false);
  }catch(_){
    // A transient Control request must not hide a previously available local
    // recovery card. The server remains the authorization boundary.
    syncTerminalAccessCard(true);
  }
}
function terminalStateClass(st){
  if(!st) return "unknown";
  return doctorStateLevel(st.state);
}
function terminalStatusText(st){
  if(!st) return "not reported";
  if(st.enabled===false) return "Disabled by SSH administrator";
  if(st.ready && st.shortcutReady) return "Ready — button and Ctrl+Alt+T";
  if(st.ready) return "Button ready — shortcut needs setup";
  return st.label||"Setup needed";
}
async function openDashboardTerminal(){
  ctrlMsg("Opening terminal…");
  try{
    const r=await api("/api/terminal/open","POST",{});
    delete CTRL_CACHE["/api/terminal/status"];
    await renderCtrlTerminal();
    await renderCtrlActionHistory();
    ctrlMsg((r&&r.opened)?"Terminal opened. Type exit or close it to return to the dashboard.":"Terminal request sent.");
  }catch(e){ ctrlMsg("Terminal unavailable right now: "+e.message); }
}
function renderTerminalStatusCard(wrap,st){
  const level=terminalStateClass(st);
  const top=el("div","terminaltop "+level);
  top.innerHTML=`<div><div class="terminallabel">Terminal access</div><div class="terminalstate">${escapeHTML(terminalStatusText(st))}</div></div><div class="terminalstamp">${escapeHTML(st&&st.pinEnabled?"PIN protected":"No PIN required")}</div>`;
  wrap.appendChild(top);
  const grid=el("div","ctrlgrid terminalgrid");
  const items=[
    ["Open button", st&&st.ready?"Ready":"Unavailable", st&&st.ready?"ok":"bad"],
    ["Ctrl+Alt+T", st&&st.shortcutReady?"Running":(st&&st.xbindkeys?"Not running":"Needs xbindkeys"), st&&st.shortcutReady?"ok":"warn"],
    ["xterm", st&&st.xterm?"Installed":"Missing", st&&st.xterm?"ok":"bad"],
    ["xbindkeys", st&&st.xbindkeys?"Installed":"Missing", st&&st.xbindkeys?"ok":"warn"],
    ["Display", st&&st.displayAvailable?"Available":"No X display", st&&st.displayAvailable?"ok":"bad"],
    ["Dashboard browser", st&&st.browserRunning?"Running":"Not running", st&&st.browserRunning?"ok":"warn"],
  ];
  for(const [k,v,state] of items){
    const d=el("div","stat "+state);
    d.innerHTML=`<div class="k">${escapeHTML(k)}</div><div class="v">${escapeHTML(String(v))}</div>`;
    grid.appendChild(d);
  }
  wrap.appendChild(grid);
  if(st && st.problems && st.problems.length){
    const list=el("div","terminalproblems");
    for(const p of st.problems.slice(0,5)) list.appendChild(el("div","terminalproblem",p));
    wrap.appendChild(list);
  }
  wrap.appendChild(el("div","ctrlmini",(st&&st.hint)||"Type exit or close the terminal to return to the fullscreen dashboard."));
  if(st && (!st.xterm || !st.xbindkeys)){
    wrap.appendChild(el("div","ctrlmini repairhint","Repair hint: "+(st.repairHint||"sudo apt-get install -y xterm xbindkeys")));
  }
}
async function renderCtrlTerminal(){
  const wrap=$("#ctrlterminal"); if(!wrap) return;
  ctrlSetLoading(wrap,"Checking terminal access…","Reading xterm, xbindkeys, display, browser, and shortcut status.");
  let st=null;
  try{ st=await api("/api/terminal/status"); }
  catch(e){ wrap.innerHTML=""; ctrlSetError(wrap,"Terminal status unavailable",e,[cbtn("Try again","",async()=>{ await renderCtrlTerminal(); })]); return; }
  if(!syncTerminalAccessCard(st&&st.enabled!==false)){wrap.innerHTML="";return;}
  wrap.innerHTML="";
  renderTerminalStatusCard(wrap,st);
  const actions=el("div","ctrlrow actiongrid terminalactions");
  actions.appendChild(caction("Open terminal","Open a local maintenance shell over the dashboard.",st.ready?"primary":"",openDashboardTerminal));
  actions.appendChild(caction("Refresh terminal status","Re-check xterm, xbindkeys, display, and shortcut readiness.","",async()=>{
    delete CTRL_CACHE["/api/terminal/status"]; await renderCtrlTerminal(); ctrlMsg("Terminal status refreshed.");
  }));
  wrap.appendChild(actions);
}
