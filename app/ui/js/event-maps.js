// 05-popups-02-event-maps.js — bounded static event-map preview controls.
// Style and detail are independent local popup choices. Keep one image node,
// one current request, and no prefetch fan-out for Pi-safe event maps.
const EVENT_MAP_CACHE={};
const EVENT_MAP_ZOOMS=[
  {key:"area", label:"Area", z:13},
  {key:"street", label:"Street", z:15},
  {key:"close", label:"Close", z:17},
];
const EVENT_MAP_STYLES=[
  {key:"standard", label:"Standard"},
  {key:"hybrid", label:"Hybrid"},
];
function normalizeEventMapStyle(style){
  style=String(style||CONFIG.mapImageStyle||"standard").toLowerCase();
  return EVENT_MAP_STYLES.some(s=>s.key===style)?style:"standard";
}
function normalizeEventMapZoom(zoom){
  zoom=Number(zoom||15);
  return EVENT_MAP_ZOOMS.some(item=>item.z===zoom)?zoom:15;
}
function mapQueryAllowed(q){ return !!(q && String(q).trim().length>=3); }
function mapImageUrl(m,z,style){
  if(!m) return "";
  style=normalizeEventMapStyle(style);
  if(m.styleStaticUrls && m.styleStaticUrls[style]){
    const byZ=m.styleStaticUrls[style][String(z)] || m.styleStaticUrls[style][z];
    if(byZ) return byZ;
  }
  if(style==="standard" && m.staticUrls){
    const byZ=m.staticUrls[String(z)] || m.staticUrls[z];
    if(byZ) return byZ;
  }
  if(m.lat!=null && m.lon!=null){
    return "/api/event-map-img?lat="+encodeURIComponent(Number(m.lat).toFixed(6))+"&lon="+encodeURIComponent(Number(m.lon).toFixed(6))+"&z="+encodeURIComponent(z)+"&style="+encodeURIComponent(style);
  }
  return m.staticUrl||"";
}
function bindEventMapRadioKeys(row,buttons){
  row.addEventListener("keydown",event=>{
    const keys=["ArrowLeft","ArrowRight","ArrowUp","ArrowDown","Home","End"];
    if(!keys.includes(event.key)) return;
    event.preventDefault();
    let current=buttons.indexOf(document.activeElement);
    if(current<0) current=0;
    let next=current;
    if(event.key==="ArrowLeft"||event.key==="ArrowUp") next=(current+buttons.length-1)%buttons.length;
    else if(event.key==="ArrowRight"||event.key==="ArrowDown") next=(current+1)%buttons.length;
    else if(event.key==="Home") next=0;
    else if(event.key==="End") next=buttons.length-1;
    buttons[next].focus();
    buttons[next].click();
  });
}
async function loadEventMap(q,wrap,task){
  if(!mapQueryAllowed(q)||!wrap)return;
  const live=()=>!task||!task.isCurrent||task.isCurrent();
  const key=String(q).trim().toLowerCase();let abort=null,signal;
  if(typeof AbortController!=="undefined"){
    const ctrl=new AbortController();signal=ctrl.signal;abort=()=>ctrl.abort();if(task&&task.onCancel)task.onCancel(abort);
  }
  function draw(m){
    if(!live())return;
    if(!m||!m.ok||!(m.staticUrl||(m.lat!=null&&m.lon!=null))){wrap.remove();return;}
    wrap.className="eventmap";wrap.replaceChildren();wrap.setAttribute("aria-label","Map preview for "+(m.label||q));
    const controls=el("div","mapcontrols maptoolbar"),stage=el("div","mapstage"),msg=el("div","mapmsg",""),img=document.createElement("img");
    controls.setAttribute("aria-label","Map preview controls");
    img.alt="Map preview for "+(m.label||q);img.loading="eager";img.referrerPolicy="no-referrer";img.style.display="none";stage.append(msg,img);wrap.append(controls,stage);
    let currentZoom=normalizeEventMapZoom(m.defaultZoom),currentStyle=normalizeEventMapStyle(CONFIG.mapImageStyle||m.defaultStyle||"standard"),loadSeq=0,slowTimer=0;
    const cancelImage=()=>{loadSeq++;clearTimeout(slowTimer);img.onload=img.onerror=null;img.src="";};if(task&&task.onCancel)task.onCancel(cancelImage);
    function syncMapControlState(){
      controls.querySelectorAll(".mapstyle").forEach(button=>{
        const selected=String(button.dataset.style)===currentStyle;
        button.classList.toggle("on",selected);button.setAttribute("aria-checked",String(selected));button.tabIndex=selected?0:-1;
      });
      controls.querySelectorAll(".mapzoom").forEach(button=>{
        const selected=Number(button.dataset.z)===currentZoom;
        button.classList.toggle("on",selected);button.setAttribute("aria-checked",String(selected));button.tabIndex=selected?0:-1;
      });
    }
    function loadMap(){
      if(!live())return;loadSeq++;const seq=loadSeq;clearTimeout(slowTimer);syncMapControlState();
      const zLabel=(EVENT_MAP_ZOOMS.find(x=>x.z===currentZoom)||{}).label||"map",sLabel=(EVENT_MAP_STYLES.find(x=>x.key===currentStyle)||{}).label||"Map";
      msg.textContent="Loading "+sLabel.toLowerCase()+" "+zLabel.toLowerCase()+" map…";msg.style.display="block";img.style.display="none";
      img.onload=()=>{if(!live()||seq!==loadSeq)return;msg.style.display="none";img.style.display="block";};img.onerror=()=>{if(!live()||seq!==loadSeq)return;msg.textContent=sLabel+" map unavailable";};img.src=mapImageUrl(m,currentZoom,currentStyle);
      slowTimer=setTimeout(()=>{if(live()&&seq===loadSeq&&msg.parentNode&&img.style.display==="none")msg.textContent="Trying map providers…";},5000);
    }
    function addControlGroup(groupClass,heading,rowClass,choices,buttonClass){
      const group=el("section","mapcontrolgroup "+groupClass),label=el("div","mapcontrolheading",heading),row=el("div","mapcontrolrow "+rowClass);
      row.setAttribute("role","radiogroup");row.setAttribute("aria-label",heading);group.append(label,row);controls.appendChild(group);
      const buttons=choices.map(def=>{
        const button=el("button",buttonClass,def.label);button.type="button";button.setAttribute("role","radio");button.setAttribute("aria-checked","false");
        if(buttonClass==="mapstyle"){
          button.dataset.style=def.key;button.setAttribute("aria-label","Map style: "+def.label);button.addEventListener("click",()=>{const next=normalizeEventMapStyle(def.key);if(next===currentStyle)return;currentStyle=next;loadMap();});
        }else{
          button.dataset.z=String(def.z);button.setAttribute("aria-label","Map detail: "+def.label);button.addEventListener("click",()=>{if(def.z===currentZoom)return;currentZoom=def.z;loadMap();});
        }
        row.appendChild(button);return button;
      });
      bindEventMapRadioKeys(row,buttons);
      return {group,row,buttons};
    }
    addControlGroup("mapstylegroup","Map style","mapcontrolrow--style",EVENT_MAP_STYLES,"mapstyle");
    addControlGroup("mapzoomgroup","Map detail","mapcontrolrow--zoom",EVENT_MAP_ZOOMS,"mapzoom");
    loadMap();
  }
  try{
    if(EVENT_MAP_CACHE[key]){draw(EVENT_MAP_CACHE[key]);return;}
    const res=await fetch("/api/event-map?q="+encodeURIComponent(q),{cache:"no-store",...(signal?{signal}:{})});
    const m=await res.json().catch(()=>({}));if(!live())return;
    if(!res.ok||!m.ok){wrap.remove();return;}EVENT_MAP_CACHE[key]=m;draw(m);
  }catch(err){if(live()&&!(err&&err.name==="AbortError"))wrap.remove();}
}
