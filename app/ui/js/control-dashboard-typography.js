// Target-scoped Dashboard Display typography. This editor deliberately owns
// the four configurable dashboard text areas; Calendar and Visual style route
// here rather than retaining duplicate font/weight/size controls.
const DASHBOARD_TYPOGRAPHY_TARGETS={
  calendar:{label:"Calendar events",detail:"Event cards, multiday bars, and agenda titles.",keys:{size:"calendarTextSize",weight:"calendarTextWeight",font:"calendarTextFont"},sizes:[[-1,"Compact"],[-0.5,"Small"],[0,"Default"],[0.5,"Large"],[1,"Extra large"]],weights:[[400,"Light"],[600,"Regular"],[700,"Default"],[800,"Strong"],[900,"Bold"]]},
  clock:{label:"Clock & date",detail:"Sidebar date and main time.",keys:{size:"clockTextSize",weight:"clockTextWeight",font:"clockTextFont"},sizes:[[-2,"Compact"],[-1,"Small"],[0,"Default"],[1,"Large"],[2,"Extra large"]],weights:[[400,"Light"],[500,"Regular"],[600,"Default"],[700,"Strong"],[800,"Bold"]]},
  weather:{label:"Weather text",detail:"Current conditions and forecast rows.",keys:{size:"weatherTextSize",weight:"weatherTextWeight",font:"weatherTextFont"},sizes:[[-2,"Compact"],[-1,"Small"],[0,"Default"],[1,"Large"],[2,"Extra large"]],weights:[[400,"Light"],[500,"Regular"],[600,"Default"],[700,"Strong"],[800,"Bold"]]},
  messages:{label:"Rotating messages",detail:"Bottom message text; fitting still protects long messages.",keys:{size:"messageTextSize",weight:"messageTextWeight",font:"messageTextFont"},sizes:[[-2,"Compact"],[-1,"Small"],[0,"Default"],[1,"Large"],[2,"Extra large"]],weights:[[600,"Light"],[700,"Regular"],[800,"Default"],[850,"Strong"],[900,"Bold"]]}
};
const DASHBOARD_TYPOGRAPHY_FONT_CHOICES=[["system","System"],["rounded","Rounded"],["default","Default"],["readable","Readable"],["mono","Mono"]];
let CTRL_DASHBOARD_TYPOGRAPHY_TARGET="calendar";
function dashboardTypographyTarget(target){ return DASHBOARD_TYPOGRAPHY_TARGETS[target]||DASHBOARD_TYPOGRAPHY_TARGETS.calendar; }
function dashboardTypographyTargetSummary(target){ return dashboardTypographySummary(dashboardTypographyTarget(target).keys); }
function dashboardTypographyOptionGroup(target,dimension,label,choices){
  const meta=dashboardTypographyTarget(target), key=meta.keys[dimension], group=el("section","dashboard-typography-group");
  group.appendChild(el("div","dashboard-typography-label",label));
  const grid=el("div","dashboard-typography-options");
  grid.setAttribute("role","radiogroup"); grid.setAttribute("aria-label",meta.label+" "+label);
  for(const [value,text] of choices){
    const selected=dimension==="font"?dashboardTypographyEffectiveFont(key)===value:Number(SETTINGS[key])===Number(value);
    const state=dimension==="font"?dashboardFontInfo(value):null; const caption=state&&state.state==="missing"?text+" ↓":text;
    const b=cbtn(caption,"dashboard-typography-option"+(selected?" on":"")+(state&&state.state==="missing"?" missing-font":""),()=>saveDashboardTypographyChoice(target,dimension,value));
    b.setAttribute("role","radio"); b.setAttribute("aria-checked",String(selected));
    b.setAttribute("aria-label",meta.label+" "+label+": "+text);
    if(text==="Extra large") b.classList.add("dashboard-typography-option-extra");
    grid.appendChild(b);
  }
  group.appendChild(grid); return group;
}
async function saveDashboardTypographyChoice(target,dimension,value){
  const meta=dashboardTypographyTarget(target), key=meta.keys[dimension];
  const next=dimension==="font"?dashboardTypographyFont(value):dashboardTypographyNumber(key,value);
  const before=SETTINGS[key], wasExplicit=dimension==="font"?dashboardTypographyFontIsExplicit(key):false;
  const already=dimension==="font"?dashboardTypographyEffectiveFont(key)===next:Number(before)===Number(next);
  if(dimension==="font" && dashboardFontInfo(next).state==="missing"){
    if(!already){
      SETTINGS[key]=next;dashboardTypographySetExplicitFont(key,true);syncDashboardRuntimeSettings();applyDashboardTypographyTarget(target);renderCtrlDashboardTypography();
      if(!await postSettings()){ SETTINGS[key]=before;dashboardTypographySetExplicitFont(key,wasExplicit);syncDashboardRuntimeSettings();applyDashboardTypographyTarget(target);renderCtrlDashboardTypography();ctrlMsg("Typography change was not saved. The previous setting was restored.");return; }
    }
    ctrlMsg("Downloading "+(FONT_PRESETS[next]||{}).label+" font…");
    try{ await downloadDashboardFont(next);applyDashboardTypographyTarget(target);renderCtrlDashboardTypography();ctrlMsg("Installed "+(FONT_PRESETS[next]||{}).label+" font."); }
    catch(_){renderCtrlDashboardTypography();ctrlMsg("Couldn't download font · using closest font. Select it again to retry.");}
    return;
  }
  if(already) return;
  SETTINGS[key]=next;
  if(dimension==="font") dashboardTypographySetExplicitFont(key,true);
  syncDashboardRuntimeSettings();
  applyDashboardTypographyTarget(target);
  renderCtrlDashboardTypography();
  const saved=await postSettings();
  if(saved) return;
  SETTINGS[key]=before;
  if(dimension==="font") dashboardTypographySetExplicitFont(key,wasExplicit);
  syncDashboardRuntimeSettings();
  applyDashboardTypographyTarget(target);
  renderCtrlDashboardTypography();
  ctrlMsg("Typography change was not saved. The previous setting was restored.");
}
function renderCtrlDashboardTypography(){
  const host=$("#ctrldashboarddisplay"); if(!host) return;
  let wrap=host.querySelector("#ctrldashboardtypography");
  if(!wrap){ wrap=el("section","dashboard-typography"); wrap.id="ctrldashboardtypography"; host.prepend(wrap); }
  wrap.replaceChildren();
  wrap.appendChild(el("div","dashboard-typography-title","Dashboard typography"));
  wrap.appendChild(el("div","dashboard-typography-detail","Choose one dashboard text area, then set its size, weight, and font. Default is the centered choice in every row."));
  const picker=el("div","dashboard-typography-targets"); picker.setAttribute("role","radiogroup"); picker.setAttribute("aria-label","Choose dashboard text area");
  for(const [id,meta] of Object.entries(DASHBOARD_TYPOGRAPHY_TARGETS)){
    const selected=id===CTRL_DASHBOARD_TYPOGRAPHY_TARGET;
    const summary=dashboardTypographyTargetSummary(id)+(selected?" · Active":"");
    const b=caction(meta.label,summary,"dashboard-typography-target"+(selected?" on":""),()=>{ if(id===CTRL_DASHBOARD_TYPOGRAPHY_TARGET) return; CTRL_DASHBOARD_TYPOGRAPHY_TARGET=id; renderCtrlDashboardTypography(); });
    b.setAttribute("role","radio"); b.setAttribute("aria-checked",String(selected)); b.setAttribute("aria-label",meta.label+": "+summary);
    picker.appendChild(b);
  }
  wrap.appendChild(picker);
  const active=dashboardTypographyTarget(CTRL_DASHBOARD_TYPOGRAPHY_TARGET), editor=el("div","dashboard-typography-editor");
  editor.appendChild(el("div","dashboard-typography-current","Current: "+dashboardTypographyTargetSummary(CTRL_DASHBOARD_TYPOGRAPHY_TARGET)));
  editor.appendChild(dashboardTypographyOptionGroup(CTRL_DASHBOARD_TYPOGRAPHY_TARGET,"size","Size",active.sizes));
  editor.appendChild(dashboardTypographyOptionGroup(CTRL_DASHBOARD_TYPOGRAPHY_TARGET,"weight","Weight",active.weights));
  editor.appendChild(dashboardTypographyOptionGroup(CTRL_DASHBOARD_TYPOGRAPHY_TARGET,"font","Font",DASHBOARD_TYPOGRAPHY_FONT_CHOICES));
  const note=CTRL_DASHBOARD_TYPOGRAPHY_TARGET==="messages"?"Long messages still fit to the fixed footer safely; larger choices are applied when the available space permits.":active.detail;
  editor.appendChild(el("div","dashboard-typography-note",note)); wrap.appendChild(editor);
}
function openDashboardTypographyTarget(target){
  CTRL_DASHBOARD_TYPOGRAPHY_TARGET=DASHBOARD_TYPOGRAPHY_TARGETS[target]?target:"calendar";
  ctrlOpenSection("display","dashboarddisplay");
  let tries=0;
  const show=()=>{
    const page=$("#ctrlpage-display"), section=page&&page.querySelector('details.ctrlsec[data-lazy="dashboarddisplay"]'), host=$("#ctrldashboarddisplay");
    if((!section||!section.open||!host)&&tries++<8){ setTimeout(show,60); return; }
    renderCtrlDashboardDisplay();
    ctrlAfterPaint(()=>{
      const editor=$("#ctrldashboardtypography"); if(!editor) return;
      try{ editor.scrollIntoView({behavior:ctrlLiteProfile()?"auto":"smooth",block:"nearest",inline:"nearest"}); }catch(_){ editor.scrollIntoView(false); }
    });
  };
  setTimeout(show,60);
}
