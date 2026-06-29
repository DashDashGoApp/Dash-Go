function renderCtrlMetricGrid(wrap,items){
  wrap.innerHTML=""; wrap.className="ctrlgrid";
  for(const [k,v] of items){
    const d=el("div","stat");
    d.innerHTML=`<div class="k">${escapeHTML(k)}</div><div class="v">${escapeHTML(String(v))}</div>`;
    wrap.appendChild(d);
  }
}

function actionStateClass(state){
  if(state==="success") return "ok";
  if(state==="failed" || state==="error") return "bad";
  if(state==="running" || state==="check" || state==="rolledback") return "warn";
  return "unknown";
}
function actionLabel(state){
  if(state==="success") return "Done";
  if(state==="failed" || state==="error") return "Failed";
  if(state==="rolledback") return "Rolled back";
  if(state==="running") return "In progress";
  if(state==="check") return "Check soon";
  if(state==="unknown") return "Outcome unknown";
  return "Logged";
}
async function renderCtrlActionHistory(){
  const wrap=$("#ctrlhistory"); if(!wrap) return;
  ctrlSetLoading(wrap,"Loading recent actions…","Reading the local maintenance history.");
  let data;
  try{ data=await api("/api/action-history"); }
  catch(e){ wrap.innerHTML=""; ctrlSetError(wrap,"Recent actions unavailable",e); return; }
  const entries=(data.entries||[]);
  wrap.innerHTML="";
  if(!entries.length){
    wrap.appendChild(ctrlStateCard("empty","No recent actions yet","Updates, backups, restores, health checks, terminal opens, and similar maintenance actions will appear here."));
  }else{
    const list=el("div","actionhistory");
    for(const it of entries.slice(0,12)){
      const cls=actionStateClass(it.state);
      const row=el("div","actionitem "+cls);
      const when=it.at?fmtDateTime(it.at*1000):"unknown time";
      row.innerHTML=`<div class="actionmain"><div class="actiontitle">${escapeHTML(it.label||it.kind||"Action")}</div><div class="actiondetail">${escapeHTML(it.detail||"")}</div></div><div class="actionmeta"><span class="actionbadge ${cls}">${escapeHTML(actionLabel(it.state))}</span><span>${escapeHTML(when)}</span></div>`;
      list.appendChild(row);
    }
    wrap.appendChild(list);
  }
  const actions=el("div","ctrlrow compact historyactions");
  actions.appendChild(cbtn("Refresh history","",async()=>{ await renderCtrlActionHistory(); ctrlMsg("Action history refreshed."); }));
  wrap.appendChild(actions);
}
async function renderCtrlCache(){
  const wrap=$("#ctrlcache");if(!wrap)return;
  ctrlSetLoading(wrap,"Checking event cache…","Reading cache age, coverage, issues, and calendar source health.");
  try{
    const st=await api("/api/cache/status");
    const state=st.valid?(st.stale?"stale":(st.using?"using JSON cache":"valid")):"missing/invalid";
    renderCtrlMetricGrid(wrap,[
      ["Mode", EVENT_CACHE_INFO.source==="cache"?"browser using JSON cache":(EVENT_CACHE_INFO.source||state)],
      ["Cache file", state],
      ["Generated", fmtDateTime(st.generatedAt)],
      ["Coverage", fmtDateTime(st.windowStart)+" → "+fmtDateTime(st.windowEnd)],
      ["Expanded events", st.eventCount||0],
      ["Issues", (st.issues&&st.issues.length)?st.issues.length:"none"],
    ]);
    if(!st.valid){ wrap.appendChild(ctrlStateCard("bad","Event cache needs rebuilding",st.exists?"The cache file exists but could not be read cleanly.":"No event cache file was found.")); }
    else if(st.stale){ wrap.appendChild(ctrlStateCard("warn","Event cache is stale","Calendar files changed or the cache is older than expected. Rebuild the cache to refresh the dashboard.")); }
    else if(st.issues&&st.issues.length){ wrap.appendChild(ctrlStateCard("warn","Event cache has notes",st.issues.slice(0,3).join(" · "))); }
    const buttons=el("div","ctrlrow");
    buttons.appendChild(cbtn("Rebuild cache","",async()=>{
      ctrlMsg("Rebuilding event cache…");
      try{
        const r=await api("/api/cache/rebuild","POST",{force:true});
        delete CTRL_CACHE["/api/cache/status"];
        await discoverCalendars(); await loadCalendars(); await renderCtrlCache();
        ctrlMsg(r.ok?"Event cache rebuilt: "+(r.eventCount||0)+" events":"Cache rebuild finished with warnings.");
      }catch(e){ ctrlMsg("Cache rebuild failed: "+e.message); }
    }));
    buttons.appendChild(confirmBtn("Clear & rebuild","Tap again to rebuild",async()=>{
      ctrlMsg("Clearing and rebuilding event cache…");
      try{ await api("/api/cache/rebuild","POST",{force:true,clear:true}); await loadCalendars(); await renderCtrlCache(); ctrlMsg("Event cache cleared and rebuilt."); }
      catch(e){ ctrlMsg("Cache repair failed: "+e.message); }
    }));
    wrap.appendChild(buttons);
  }catch(e){ wrap.className=""; ctrlSetError(wrap,"Event cache status unavailable",e,[cbtn("Try again","",async()=>{ await renderCtrlCache(); }) ]); }
}
async function renderCtrlMapCache(){
  const wrap=$("#ctrlmapcache");if(!wrap)return;wrap.replaceChildren();
  const mapsOn=!!CONFIG.showInteractiveMaps;
  const behavior=actionGroup("Map behavior","Choose whether event locations may open an interactive full-screen map.","displaygroup grid-1-feature");
  behavior.grid.append(caction(`Interactive event maps: ${mapsOn?"On":"Off"}`,"Static event-map previews remain available in every profile.",mapsOn?"on":"",async()=>{try{await ctrlSaveProfileOwned("showInteractiveMaps",!mapsOn,"Interactive event maps","mapcache");await renderCtrlMapCache();}catch(e){ctrlMsg("Could not change Interactive event maps: "+(e.message||String(e)));await renderCtrlMapCache();}}));
  const actions=el("div","mapmaintenance");
  const clean=cbtn("Clean expired","",async()=>{try{const r=await api("/api/maps/cleanup","POST",{});ctrlMsg(`Map cleanup removed ${((r.cache&&r.cache.removed)||0)} previews and ${((r.tileCache&&r.tileCache.removed)||0)} tiles.`);await renderCtrlMapCache();}catch(e){ctrlMsg(e.message||String(e));}});
  const clearImages=confirmBtn("Clear event maps","Tap again to clear",async()=>{try{const r=await api("/api/maps/clear","POST",{});ctrlMsg(`Cleared ${r.removed||0} event maps.`);await renderCtrlMapCache();}catch(e){ctrlMsg(e.message||String(e));}});
  const clearTiles=confirmBtn("Clear tiles","Tap again to clear",async()=>{try{const r=await api("/api/maps/clear","POST",{clearTiles:true});ctrlMsg(`Cleared ${r.tilesRemoved||0} tiles.`);await renderCtrlMapCache();}catch(e){ctrlMsg(e.message||String(e));}});
  const clearAll=confirmBtn("Clear all cache","Tap again to clear all",async()=>{try{await api("/api/maps/clear","POST",{clearGeocodes:true,clearProvider:true,clearTiles:true});ctrlMsg("Map caches cleared.");await renderCtrlMapCache();}catch(e){ctrlMsg(e.message||String(e));}});
  actions.append(clean,clearImages,clearTiles,clearAll);
  ctrlSetLoading(wrap,"Checking event maps & cache…","Reading concise cache health and maintenance state.");
  try{
    const st=await api("/api/maps/status"),p=st.provider||{},pre=st.prewarm||{},last=pre.lastResult||{};
    wrap.appendChild(behavior.group);
    const summaryCard=el("section","mapcachesummary");summaryCard.appendChild(el("div","mapcachesummary-title","Cache health"));
    const statGrid=el("div","mapcacheprimarygrid");statGrid.append(
      el("div","mapcachemetric",`Rendered event maps: ${st.imageCount||0}/${st.imageMaxFiles||"—"} · ${fmtBytes(st.imageBytes||0)}/${fmtBytes(st.imageMaxBytes||0)} · oldest ${st.oldestImage?ago(st.oldestImage*1000):"none"}`),
      el("div","mapcachemetric",`Tile cache: ${st.tileCount||0}/${st.tileMaxFiles||"—"} · ${fmtBytes(st.tileBytes||0)}/${fmtBytes(st.tileMaxBytes||0)} · oldest ${st.oldestTile?ago(st.oldestTile*1000):"none"}`),
      el("div","mapcachemetric",`Provider: ${p.primaryLabel||p.primary||"auto"} · ${p.lastError?"needs attention":"healthy"}`),
      el("div","mapcachemetric",`Last prewarm: ${last.locationsConsidered!=null?`${last.rendered||0} rendered · ${last.alreadyCached||0} cached · ${last.failed||0} failed`:(pre.lastEnd?ago(pre.lastEnd*1000):"not yet")}`)
    );summaryCard.appendChild(statGrid);wrap.appendChild(summaryCard);
    if(p.lastError)wrap.appendChild(ctrlStateCard("warn","Map provider recently had trouble",p.lastError));
    const prof=String(CONFIG.profile||"balanced").toLowerCase(),limit=prof==="enhanced"?48:(prof==="balanced"?32:12);
    const prewarm=caction("Prewarm visible maps","Build previews for the current event range.","",async()=>{const now=new Date(),windowStart=+new Date(now.getFullYear(),now.getMonth(),now.getDate()-14),windowEnd=+new Date(now.getFullYear(),now.getMonth(),now.getDate()+90);try{await api("/api/maps/prewarm","POST",{windowStart,windowEnd,limit,eventMaps:true,interactiveMaps:!!CONFIG.showInteractiveMaps});ctrlMsg("Map prewarm started.");}catch(e){ctrlMsg(e.message||String(e));}});prewarm.classList.add("mapprewarm");wrap.appendChild(prewarm);
    const maintenance=actionGroup("Maintenance","Cache actions stay explicit and destructive clears still require confirmation.","displaygroup mapmaintgroup");maintenance.grid.append(clean,clearImages,clearTiles,clearAll);wrap.appendChild(maintenance.group);
    const details=document.createElement("details");details.className="mapcachedetails";details.appendChild(el("summary","","Cache details"));const detail=el("div","mapcachedetails-body");detail.appendChild(el("div","ctrlmini",`Map locations ${st.imageLocationCount||0}/${st.imageMaxLocations||"—"} · styles ${Object.entries(st.imageStyleCounts||{}).map(([k,v])=>`${k}:${v}`).join(" · ")||"none"} · geocode entries ${st.geocodeCount||0} · newest tile ${st.newestTile?ago(st.newestTile*1000):"none"}.`));if(Array.isArray(p.providers)&&p.providers.length){const list=el("div","mapproviderlist");for(const pr of p.providers)list.appendChild(el("div","mapproviderrow",`${pr.label||pr.name} · ${(pr.styles||[]).join(", ")||"default"} · ${pr.lastError?`warning: ${pr.lastError}`:"healthy"}`));detail.appendChild(list);}details.appendChild(detail);wrap.appendChild(details);
  }catch(e){wrap.replaceChildren(behavior.group,ctrlStateCard("bad","Map cache status unavailable",e.message||String(e),[cbtn("Try again","",()=>renderCtrlMapCache())]));}
}
