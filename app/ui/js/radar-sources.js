// 06-radar-00-sources.js — browser-safe radar provider metadata. Secrets stay in ~/.dashboard-radar.env.
const RADAR_KEY_REQUIRED=new Set(["tomorrow","weatherbit","xweather"]);
const RADAR_SOURCE_META={
  rainviewer:{label:"RainViewer",tier:"free · no key · global · observed history",kind:"rainviewer",animate:true,frameMode:"source"},
  nws:{label:"NWS / NOAA",tier:"free · no key · US-only · latest reflectivity",kind:"nws",animate:false,frameMode:"latest"},
  tomorrow:{label:"Tomorrow.io",tier:"key required · metered map tiles",kind:"proxy",animate:false,frameMode:"latest"},
  weatherbit:{label:"Weatherbit Maps",tier:"key required · Maps plan",kind:"proxy",animate:false,frameMode:"latest"},
  xweather:{label:"Xweather Maps",tier:"key required · metered raster tiles",kind:"proxy",animate:false,frameMode:"latest"},
  custom_xyz:{label:"Custom XYZ / WMS",tier:"advanced · your endpoint · browser fetched",kind:"custom",animate:false,frameMode:"latest"},
};
const RADAR_DEFAULT_PROVIDER="rainviewer";
function radarBaseProfileTier(){
  const p=String((CONFIG&&CONFIG.profile)||"balanced").toLowerCase();
  if(["lite","zero2","low","low-power"].includes(p)) return "lite";
  if(["enhanced","maximum","x86","high"].includes(p)) return "enhanced";
  return "balanced";
}
// Radar rendering follows the real dashboard profile. The former user-facing
// Lite/Enhanced override was a performance-tuning fork and is intentionally gone.
function radarConfiguredTier(){ return radarBaseProfileTier(); }
function radarProfileTier(){
  return RADAR_STATE&&RADAR_STATE.open&&RADAR_STATE.renderTier?RADAR_STATE.renderTier:radarConfiguredTier();
}
// A source timeline is authoritative: RainViewer frames are retained exactly as
// advertised by its public frame index. Single-frame sources stay single-frame.
function radarFrameCount(provider){
  const meta=RADAR_SOURCE_META[String(provider||"").toLowerCase()]||{};
  if(!meta.animate)return 1;
  const requested=String(provider||"").toLowerCase(),active=String((RADAR_STATE&&RADAR_STATE.provider)||"").toLowerCase(),frames=RADAR_STATE&&RADAR_STATE.frames;
  return requested===active&&Array.isArray(frames)&&frames.length?frames.length:1;
}
function radarMeta(provider){ return RADAR_SOURCE_META[String(provider||"").toLowerCase()]||RADAR_SOURCE_META[RADAR_DEFAULT_PROVIDER]; }
