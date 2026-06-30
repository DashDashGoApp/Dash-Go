async function renderCtrlTheme(){
  const row=$("#ctrltheme");
  try{
    await cachedApi("/api/themes",t=>renderCtrlThemeData(row,t));
  }catch(e){ ctrlSetError(row,"Theme controls unavailable",friendlyUnavailable("Theme controls",e)); }
}
function renderCtrlThemeData(row,t){
  row.innerHTML="";
  let chosen=t.current;
  const currentInfo=themeInfo(t.current);
  const baseInfo=themeInfo(t.base||"basic");
  const info=el("div","themeintro","");
  const optionalHint="Optional observance themes appear only when their matching enabled holiday calendar reports the occasion today.";
  info.innerHTML=`<div class="themeintrotitle">Active: ${escapeHTML(currentInfo.label)} · Selected: ${escapeHTML(currentInfo.label)} · Seasonal base: ${escapeHTML(baseInfo.label)}</div>
    <div class="themeintrosub">Choose a preview, then apply it. Readability themes are tuned for distance, glare, or low-light use. ${escapeHTML(optionalHint)}</div>`;
  row.appendChild(info);

  const valid=new Set((t.themes||[]).filter(name=>name && name!=="dark" && name!=="default"));
  const groups={};
  for(const name of valid){
    const gi=themeInfo(name);
    const g=gi.group||"Other";
    (groups[g]||(groups[g]=[])).push(name);
  }
  const order=[...THEME_GROUP_ORDER, ...Object.keys(groups).filter(g=>!THEME_GROUP_ORDER.includes(g)).sort()];
  const btns={};
  const grid=el("div"); grid.id="themegrid"; grid.className="themegroups";
  function makeThemeCard(name){
    const ti=themeInfo(name), tv=themeVars(name);
    const b=el("button","themebtn preview"+(name===t.current?" cur sel":""),"");
    b.setAttribute("aria-label","Choose theme "+ti.label+" — "+ti.summary);
    b.innerHTML=`
      <span class="themepreview" style="--pbg:${escapeHTML(tv["--bg"]||"#0a0a0d")};--ppanel:${escapeHTML(tv["--panel"]||"rgba(255,255,255,.04)")};--pfg:${escapeHTML(tv["--fg"]||"#e8e8ea")};--paccent:${escapeHTML(tv["--accent"]||"#8fc4a6")};--ptoday:${escapeHTML(tv["--today"]||"#d9c074")};--psat:${escapeHTML(tv["--sat"]||"#8bb4d4")};--psun:${escapeHTML(tv["--sun"]||"#d99a9a")};">
        <span class="tpbar"></span><span class="tpgrid"><i></i><i></i><i></i></span><span class="tpchip"></span>
      </span>
      <span class="themename">${escapeHTML(ti.label)}</span>
      <span class="themesummary">${escapeHTML(ti.summary)}</span>`;
    btns[name]=b;
    b.addEventListener("click",()=>{
      if(btns[chosen]) btns[chosen].classList.remove("sel");
      chosen=name; b.classList.add("sel");
      const active=chosen===t.current,baseLabel=themeInfo(t.base||"basic").label;
      info.querySelector(".themeintrotitle").textContent=`Active: ${themeInfo(t.current).label} · Selected: ${themeInfo(chosen).label} · Seasonal base: ${baseLabel}`;
      applyButton.textContent=active?"Already active":`Apply ${themeInfo(chosen).label}`;applyButton.disabled=active;
    });
    return b;
  }
  const openGroups=new Set(["Readability","Core"]);
  for(const group of order){
    const names=(groups[group]||[]);
    if(!names.length) continue;
    const sec=el("details","themegroup","");
    sec.open=openGroups.has(group);
    const sum=el("summary","themegrouphead",group+" · "+names.length);
    sec.appendChild(sum);
    const cards=el("div","themegroupcards","");
    const wideCols=Math.max(1,themeGroupColumns(group,names.length));
    const mediumCols=Math.min(wideCols,5);
    const compactCols=Math.min(wideCols,4);
    const narrowCols=Math.min(compactCols,3);
    cards.dataset.themeGroup=group;
    cards.style.setProperty("--theme-cols-wide",String(wideCols));
    cards.style.setProperty("--theme-cols-medium",String(mediumCols));
    cards.style.setProperty("--theme-cols-compact",String(compactCols));
    cards.style.setProperty("--theme-cols-narrow",String(narrowCols));
    for(const name of names) cards.appendChild(makeThemeCard(name));
    sec.appendChild(cards);
    grid.appendChild(sec);
  }
  row.appendChild(grid);

  const actions=el("div","ctrlrow themeactions");
  const applyButton=cbtn("Already active","primary",async()=>{
    if(chosen===t.current && chosen===CURRENT_THEME){ ctrlMsg("That's already the active theme."); return; }
    const label=themeInfo(chosen).label;
    ctrlMsg("Applying "+label+"…");
    try{
      delete CTRL_CACHE["/api/themes"];
      await api("/api/theme","POST",{name:chosen});
      applyTheme(chosen); // local atomic style swap; reconciliation is deliberately non-blocking.
      setTimeout(()=>checkTheme(true),350);
      ctrlMsg("Theme applied: "+label);
      if(btns[t.current]) btns[t.current].classList.remove("cur");
      if(btns[chosen])    btns[chosen].classList.add("cur");
      t.current=chosen;applyButton.textContent="Already active";applyButton.disabled=true;
      const ci=themeInfo(t.current), bi=themeInfo(t.base||"basic");
      info.querySelector(".themeintrotitle").textContent="Active: "+ci.label+" · Selected: "+ci.label+" · Seasonal base: "+bi.label;
    }catch(e){ ctrlMsg(e.message); }
  });
  applyButton.disabled=true;actions.appendChild(applyButton);
  actions.appendChild(cbtn("Seasonal rotation: "+(t.seasonal?"on":"off"), t.seasonal?"on":"", async()=>{
    try{
      delete CTRL_CACHE["/api/themes"];
      const r=await api("/api/seasonal","POST",{enabled:!t.seasonal});
      setTimeout(()=>checkTheme(true),350); // server reconciliation; do not block the visible control.
      ctrlMsg(r.enabled ? "Seasonal auto-theming ON — holiday themes apply automatically."
                        : "Seasonal auto-theming OFF — the theme stays as set.");
      renderCtrlTheme();
    }catch(e){ ctrlMsg(e.message); }
  }));
  actions.appendChild(cbtn("Use selected theme as seasonal base","",async()=>{
    try{
      const r=await api("/api/theme/base","POST",{name:chosen});
      if(r.applied)applyTheme(chosen);
      setTimeout(()=>checkTheme(true),350);
      const active=themeInfo(CURRENT_THEME), base=themeInfo(r.base);
      info.querySelector(".themeintrotitle").textContent="Active: "+active.label+" · Selected: "+themeInfo(chosen).label+" · Seasonal base: "+base.label;
      ctrlMsg("Between-holidays base set to "+base.label+
              (r.applied?" — applied now.":" — a holiday theme is active today; it returns to this afterward."));
    }catch(e){ ctrlMsg(e.message); }
  }));
  row.appendChild(actions);
}
