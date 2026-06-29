// 05-popups-01a-map-keyboard-controls.js — Google Maps OSK and overlay controls.
let _mapOskLayer="letters";
let _mapOskShift=false;
let _mapOskCapsLock=false;
let _mapOskLastShiftTap=0;
function mapKeyboardRows(){
  const letterRows=[
    ["1","2","3","4","5","6","7","8","9","0"],
    ["q","w","e","r","t","y","u","i","o","p"],
    ["a","s","d","f","g","h","j","k","l","'"],
    ["z","x","c","v","b","n","m",",",".",":"],
  ];
  const symbolRows=[
    ["!","@","#","$","%","&","*","(",")","/"],
    ["-","_","=","+","\\","?",";",":","'","\""],
    [",",".","<",">","[","]","{","}","|","`"],
  ];
  return _mapOskLayer==="symbols"?symbolRows:letterRows;
}
function mapKeyboardButton(label,cls,fn){
  const b=el("button","oskkey"+(cls?" "+cls:""),label);
  b.type="button";
  b.addEventListener("pointerdown",e=>e.preventDefault());
  b.addEventListener("click",e=>{ e.preventDefault(); e.stopPropagation(); fn(); });
  return b;
}
function mapKeyboardType(ch){
  if(ch==="⌫") return mapKeyType("\b");
  const out=(_mapOskLayer==="letters" && /^[a-z]$/.test(ch) && (_mapOskShift||_mapOskCapsLock))?ch.toUpperCase():ch;
  mapKeyType(out);
  if(_mapOskShift && !_mapOskCapsLock){ _mapOskShift=false; buildMapKeyboard(true); }
}
function buildMapKeyboard(force){
  const container=$("#mapkeyboard"); if(!container) return;
  if(container._built && !force) return;
  container.innerHTML="";
  for(const r of mapKeyboardRows()){
    const row=el("div","oskrow");
    for(const ch of r){
      const label=(_mapOskLayer==="letters" && /^[a-z]$/.test(ch) && (_mapOskShift||_mapOskCapsLock))?ch.toUpperCase():ch;
      row.appendChild(mapKeyboardButton(label,"",()=>mapKeyboardType(ch)));
    }
    container.appendChild(row);
  }
  const last=el("div","oskrow maplastrow");
  last.appendChild(mapKeyboardButton(_mapOskLayer==="symbols"?"ABC":"?123","wide",()=>{ _mapOskLayer=_mapOskLayer==="symbols"?"letters":"symbols"; buildMapKeyboard(true); }));
  const shiftKey=mapKeyboardButton(_mapOskCapsLock?"CAPS":"SHIFT","wide",()=>{
    const now=Date.now();
    if(_mapOskShift && now-_mapOskLastShiftTap<700){ _mapOskCapsLock=!_mapOskCapsLock; _mapOskShift=_mapOskCapsLock; }
    else if(_mapOskCapsLock){ _mapOskCapsLock=false; _mapOskShift=false; }
    else { _mapOskShift=!_mapOskShift; }
    _mapOskLastShiftTap=now;
    buildMapKeyboard(true);
  });
  if(_mapOskShift||_mapOskCapsLock) shiftKey.classList.add("on");
  last.appendChild(shiftKey);
  last.appendChild(mapKeyboardButton("space","space",()=>mapKeyType(" ")));
  last.appendChild(mapKeyboardButton("clear","wide",()=>mapKeyType("clear")));
  last.appendChild(mapKeyboardButton("⌫","wide",()=>mapKeyType("\b")));
  last.appendChild(mapKeyboardButton("Search","wide on",runInteractiveMapSearch));
  container.appendChild(last);
  const hint=el("div","maposkhint","Double-tap SHIFT for caps lock. Use Search to reload the Google Maps view.");
  container.appendChild(hint);
  container._built=true;
}
function mapToolButton(label,title,cls,fn){
  const b=el("button","maptoolbtn"+(cls?" "+cls:""),label);
  b.type="button"; b.setAttribute("aria-label",title||label);
  b.addEventListener("pointerdown",e=>e.preventDefault());
  b.addEventListener("click",e=>{ e.preventDefault(); e.stopPropagation(); fn(); });
  return b;
}
function buildMapTools(){
  const wrap=$("#mapframewrap"); if(!wrap) return;
  let tools=$("#maptools");
  if(!tools){ tools=el("div","maptools"); tools.id="maptools"; wrap.appendChild(tools); }
  tools.innerHTML="";
  const zoom=el("div","maptoolrow maptoolzoom");
  (MAP_ZOOM_PRESETS||[]).forEach(preset=>{
    const btn=mapToolButton(preset.label,preset.title||preset.label,"zoom",()=>mapSetZoom(preset.zoom));
    btn.dataset.zoom=String(preset.zoom);
    zoom.appendChild(btn);
  });
  tools.appendChild(zoom);
  const utility=el("div","maptoolrow maptoolutility");
  utility.appendChild(mapToolButton("−","Zoom out one level","",()=>mapZoomBy(-1)));
  utility.appendChild(mapToolButton("+","Zoom in one level","",()=>mapZoomBy(1)));
  utility.appendChild(mapToolButton("Center","Recenter on the searched location","wide",mapRecenter));
  tools.appendChild(utility);
  tools.appendChild(el("div","maptoolhint","Choose a zoom level. Center is available after the local lookup resolves the searched location."));
  updateMapTools();
}
function updateMapTools(){
  const tools=$("#maptools"); if(!tools) return;
  const z=Number(MAP_STATE.zoom)||15;
  const hasBase=mapCoord(MAP_STATE.baseLat)&&mapCoord(MAP_STATE.baseLon);
  const center=Array.from(tools.querySelectorAll(".maptoolbtn")).find(b=>b.textContent==="Center");
  if(center) center.disabled=!hasBase;
  tools.querySelectorAll(".maptoolbtn").forEach(b=>b.classList.remove("on"));
  tools.querySelectorAll(".maptoolbtn.zoom").forEach(b=>{
    if(Number(b.dataset.zoom)===z) b.classList.add("on");
  });
  const hint=tools.querySelector(".maptoolhint");
  if(hint) hint.textContent=hasBase ? `Zoom ${z}. Center returns to the searched location.` : `Zoom ${z}. Center enables after local lookup resolves this place.`;
}
