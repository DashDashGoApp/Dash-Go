// 05-popups-03-event-popups.js — event popup nodes and shared day-view primitives.
function eventPopupWhen(ev){
  const f=document.createDocumentFragment(),dfmt=FMT.popDay,tfmt=FMT.time;
  if(ev.allDay){f.appendChild(el("span","wstart",dfmt.format(ev.start)+" · All day"));return f;}
  f.appendChild(el("span","wstart","Starts "+dfmt.format(ev.start)+" · "+tfmt.format(ev.start)));
  if(ev.end&&+ev.end!==+ev.start){
    const end=sameDay(ev.start,ev.end)?tfmt.format(ev.end):dfmt.format(ev.end)+" · "+tfmt.format(ev.end);
    f.appendChild(el("span","wend","Ends "+end));
  }
  return f;
}
function showEventPopup(ev){
  // Agenda and day-list app-owned entries should open the same actionable
  // household surface as a grouped month-cell row, rather than a read-only
  // ICS detail card. Ownership is explicit metadata, never title inference.
  if(typeof appCalendarOwner==="function"&&typeof showActionableAppCalendarEvent==="function"&&appCalendarOwner(ev)){
    showActionableAppCalendarEvent(ev);
    return;
  }
  popupOpenTransaction({mode:"eventpop",title:ev.title||"(no title)",when:()=>eventPopupWhen(ev),loading:"Opening event…"},token=>{
    const frag=document.createDocumentFragment(),meta=el("div","eventpopupmeta");
    if(ev.location){
      const location=el("div","eventmetaitem eventlocation");
      location.appendChild(el("span","eventmetakey","Location"));
      if(CONFIG.showInteractiveMaps){
        const b=el("button","locbtn");b.type="button";
        b.append(document.createTextNode(ev.location),el("span","hint","Tap to open interactive Google Maps"));
        b.addEventListener("click",e=>{e.stopPropagation();openInteractiveMap(ev.location);});location.appendChild(b);
      }else location.appendChild(el("span","eventlocationtext",ev.location));
      meta.appendChild(location);
    }
    if(ev.cal&&ev.cal.name){
      const calendar=el("div","eventmetaitem eventcalendar");
      calendar.append(el("span","eventmetakey","Calendar"),el("span","eventcalname",ev.cal.name));meta.appendChild(calendar);
    }
    if(meta.childNodes.length)frag.appendChild(meta);
    let mapWrap=null;
    if(ev.location){mapWrap=el("div","eventmap loading","Map loads after event details…");frag.appendChild(mapWrap);}
    if(ev.desc){const d=el("div");d.style.marginTop="10px";d.textContent=ev.desc;frag.appendChild(d);}
    if(!ev.location&&!ev.desc)frag.appendChild(el("div",null,"No additional details."));
    if(mapWrap)popupDefer(token,task=>{if(task.isCurrent()&&mapWrap.isConnected)loadEventMap(ev.location,mapWrap,task);});
    return frag;
  });
}
function escapeHTML(s){ return (s||"").replace(/[&<>]/g,c=>({"&":"&amp;","<":"&lt;",">":"&gt;"}[c])); }

function dtClamp(n,min,max){ return Math.max(min,Math.min(max,n)); }
function dtMinutes(d){ return d.getHours()*60+d.getMinutes(); }
function dtFormatHour(h){
  if(CONFIG.clock24) return h===24?"24:00":String(h).padStart(2,"0")+":00";
  if(h===0 || h===24) return "12 AM";
  if(h===12) return "Noon";
  return (h>12?h-12:h)+(h>11?" PM":" AM");
}
function dtParseColor(value){
  const c=String(value||"").trim();
  if(/^#([0-9a-f]{3})$/i.test(c)){
    const m=c.slice(1).split("").map(x=>parseInt(x+x,16));return {rgb:m,alpha:1};
  }
  if(/^#([0-9a-f]{6})$/i.test(c)){
    const n=parseInt(c.slice(1),16);return {rgb:[(n>>16)&255,(n>>8)&255,n&255],alpha:1};
  }
  const rgb=c.match(/^rgba?\(\s*([\d.]+)\s*,\s*([\d.]+)\s*,\s*([\d.]+)(?:\s*,\s*([\d.]+))?\s*\)$/i);
  if(rgb)return {rgb:[+rgb[1],+rgb[2],+rgb[3]].map(n=>Math.max(0,Math.min(255,Math.round(n)))),alpha:rgb[4]===undefined?1:Math.max(0,Math.min(1,+rgb[4]))};
  return null;
}
function dtBlendRGB(foreground,background,alpha){
  return foreground.map((value,index)=>Math.max(0,Math.min(255,Math.round(value*alpha+background[index]*(1-alpha)))));
}
function dtOpaque(colorRGB,baseRGB,alpha){
  const rgb=dtBlendRGB(colorRGB,baseRGB,alpha);return `rgb(${rgb[0]},${rgb[1]},${rgb[2]})`;
}
function dtPanelBaseRGB(style){
  const bg=dtParseColor(style.getPropertyValue("--bg"))||{rgb:[20,32,41],alpha:1};
  const panel=dtParseColor(style.getPropertyValue("--panel"));
  return panel?dtBlendRGB(panel.rgb,bg.rgb,panel.alpha):bg.rgb;
}
function dtColorToRgba(color,alpha){
  const parsed=dtParseColor(color);
  if(parsed){const rgb=parsed.rgb;return `rgba(${rgb[0]},${rgb[1]},${rgb[2]},${alpha})`;}
  return alpha>.25?"rgba(127,214,168,0.30)":"rgba(127,214,168,0.14)";
}
function dtTimeRangeText(seg){
  if(seg.ev.allDay) return "All day";
  return FMT.time.format(seg.realStart)+" – "+FMT.time.format(seg.realEnd);
}
function dtDaySegment(day,ev){
  const ds=startOfDay(day), de=addDays(ds,1);
  const st=ev.start>ds?ev.start:ds;
  let en=ev.end?ev.end:new Date(+ev.start+30*60000);
  if(en>de) en=de;
  if(en<=st) en=new Date(+st+30*60000);
  return {ev,realStart:st,realEnd:en,startMin:dtMinutes(st),endMin:(+en===+de)?1440:dtMinutes(en)};
}
function dtAssignLanes(segs){
  const sorted=[...segs].sort((a,b)=>a.startMin-b.startMin || b.endMin-a.endMin);
  const clusters=[]; let cur=null;
  for(const s of sorted){
    if(!cur || s.startMin>=cur.end){ cur={id:clusters.length,start:s.startMin,end:s.endMin,items:[]}; clusters.push(cur); }
    cur.items.push(s); cur.start=Math.min(cur.start,s.startMin); cur.end=Math.max(cur.end,s.endMin);
  }
  for(const cl of clusters){
    const laneEnds=[];
    for(const s of cl.items){
      let lane=laneEnds.findIndex(end=>end<=s.startMin);
      if(lane<0){ lane=laneEnds.length; laneEnds.push(s.endMin); }
      laneEnds[lane]=s.endMin;
      s.lane=lane;
    }
    const laneCount=Math.max(1,laneEnds.length);
    for(let i=0;i<cl.items.length;i++){
      const s=cl.items[i];
      s.laneCount=laneCount;
      s.clusterId=cl.id;
      s.clusterIndex=i;
      s.clusterSize=cl.items.length;
      s.clusterStart=cl.start;
      s.clusterEnd=cl.end;
    }
  }
  return sorted;
}

function dtLaneLayout(lane,lanes,gapPx){
  if(lanes<=1) return {left:"0px",width:"100%"};
  const pct=(100/lanes);
  const widthAdj=(gapPx*(lanes-1)/lanes).toFixed(2);
  const leftAdj=(gapPx*lane/lanes).toFixed(2);
  return {
    left:`calc(${(pct*lane).toFixed(4)}% + ${leftAdj}px)`,
    width:`calc(${pct.toFixed(4)}% - ${widthAdj}px)`
  };
}

function dtMaxReadableLanes(){
  const mobile=window.innerWidth<900;
  const popW=Math.min(window.innerWidth*(mobile?0.94:0.92),980);
  const bodyPad=mobile?24:36;
  const eventsInset=mobile?74:86;
  const gap=mobile?8:10;
  const minW=mobile?138:168;
  const available=Math.max(1,popW-bodyPad-eventsInset);
  return Math.max(1,Math.min(mobile?3:4,Math.floor((available+gap)/(minW+gap))));
}
function dtAddEventContents(card,seg,mode){
  const ev=seg.ev;
  const title=ev.title||"(no title)";
  if(mode==="tiny") card.appendChild(el("div","dt-event-title",title+" · "+FMT.time.format(seg.realStart)));
  else card.appendChild(el("div","dt-event-title",title));
  if(mode==="normal" && ev.location) card.appendChild(el("div","dt-event-loc",ev.location));
  if(mode!=="tiny") card.appendChild(el("div","dt-event-time",dtTimeRangeText(seg)));
  if(ev.cal&&ev.cal.name) card.setAttribute("aria-label",ev.cal.name);
}
function dtIsLightTheme(){
  const t=(document.documentElement.getAttribute("data-theme")||"").toLowerCase();
  return t==="paper" || t==="softmorning" || t==="daylight";
}
function dtEventColor(ev){ return (ev && ev.cal && ev.cal.color) || "var(--accent)"; }
function dtBeginDayCardPaintContext(model){
  if(!model)return;
  const lite=typeof dtLiteDayPopupProfile==="function"&&dtLiteDayPopupProfile();
  const light=dtIsLightTheme();
  if(!lite){model.dtCardPaint={lite:false,light};return;}
  const style=getComputedStyle(document.documentElement),accent=dtParseColor(style.getPropertyValue("--accent"));
  model.dtCardPaint={lite:true,light,panelRGB:dtPanelBaseRGB(style),accentRGB:(accent&&accent.rgb)||[127,214,168]};
}
function dtApplyCardColor(card,color,kind,model){
  const paint=model&&model.dtCardPaint,light=paint?paint.light:dtIsLightTheme();
  const bgAlpha=kind==="list" ? (light?0.13:0.15) : (light?0.14:0.17);
  const borderAlpha=light?0.42:0.34;
  card.style.setProperty("--dt-cal-color",color);
  card.style.borderLeftColor=color;
  if(paint&&paint.lite){
    const parsed=dtParseColor(color),rgb=(parsed&&parsed.rgb)||paint.accentRGB;
    card.style.backgroundColor=dtOpaque(rgb,paint.panelRGB,bgAlpha);
    card.style.borderColor=dtOpaque(rgb,paint.panelRGB,borderAlpha);
  }else{
    card.style.backgroundColor=dtColorToRgba(color,bgAlpha);
    card.style.borderColor=dtColorToRgba(color,borderAlpha);
  }
}
function dtCalendarMeta(ev){
  if(!(ev && ev.cal && ev.cal.name)) return null;
  const meta=el("div","dt-card-cal");
  const dot=el("span","dt-card-dot");
  dot.style.backgroundColor=dtEventColor(ev);
  meta.appendChild(dot);
  meta.appendChild(el("span",null,ev.cal.name));
  return meta;
}
function dtEventIndex(model,ev){return model&&model.eventKeys?model.eventKeys.get(ev):undefined;}
function dtMarkEventCard(card,ev,model){
  const idx=dtEventIndex(model,ev);
  if(idx!==undefined)card.dataset.dtEvid=String(idx);
}
function dtBuildAllDayCard(ev,model){
  const card=el("button","dt-allday-card "+classify(ev)); card.type="button";dtMarkEventCard(card,ev,model);
  const color=dtEventColor(ev);
  dtApplyCardColor(card,color,"allday",model);
  const main=el("div","dt-card-main");
  main.appendChild(el("span","dt-card-title",ev.title||"(no title)"));
  const meta=dtCalendarMeta(ev);
  if(meta) main.appendChild(meta);
  card.appendChild(main);
  return card;
}
function dtBuildListCard(ev,day,model){
  const card=el("button","dt-list-card "+classify(ev)); card.type="button";dtMarkEventCard(card,ev,model);
  const color=dtEventColor(ev);
  dtApplyCardColor(card,color,"list",model);
  const head=el("div","dt-list-top");
  head.appendChild(el("div","dt-list-title",ev.title||"(no title)"));
  const time=el("div","dt-list-time",ev.allDay?"All day":dtTimeRangeText(dtDaySegment(day,ev)));
  head.appendChild(time);
  card.appendChild(head);
  const meta=el("div","dt-list-meta");
  const cal=dtCalendarMeta(ev);
  if(cal) meta.appendChild(cal);
  if(ev.location){
    const loc=el("div","dt-list-loc",ev.location);
    meta.appendChild(loc);
  }
  if(meta.childNodes.length) card.appendChild(meta);
  return card;
}
function dtBuildListView(day,evs,model){
  const wrap=el("div","dt-list");
  const sorted=[...evs].sort((a,b)=>{
    const aa=a.allDay?0:1, bb=b.allDay?0:1;
    if(aa!==bb) return aa-bb;
    return (+a.start)-(+b.start) || (+((a.end)||a.start))-(+((b.end)||b.start)) || String(a.title||"").localeCompare(String(b.title||""));
  });
  for(const ev of sorted) wrap.appendChild(dtBuildListCard(ev,day,model));
  return wrap;
}
function dtBuildViewBar(selected,onPick,showTimeline){
  const bar=el("div","dt-viewbar"),group=el("div","dt-viewtoggle"),buttons={};
  function setActive(next){
    selected=next;
    for(const [key,button] of Object.entries(buttons)){
      const active=key===selected;button.classList.toggle("is-active",active);button.setAttribute("aria-pressed",active?"true":"false");
    }
  }
  function addBtn(key,label){
    const b=el("button","dt-viewbtn",label);b.type="button";
    b.addEventListener("click",e=>{e.stopPropagation();if(key!==selected)onPick(key);});buttons[key]=b;group.appendChild(b);
  }
  if(showTimeline)addBtn("timeline","Timeline");
  addBtn("list","List");bar.appendChild(group);setActive(selected);
  return {bar,setActive};
}
