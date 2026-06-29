// 05-popups-03a-day-popup.js — staged, cached full-day Timeline/List popup.
const DT_CARD_STAGE_CHUNK=16;
function dtBuildDayModel(day,evs){
  const dayStart=startOfDay(day),dayEnd=addDays(dayStart,1);
  const allDay=evs.filter(ev=>ev.allDay);
  const timed=evs.filter(ev=>!ev.allDay&&ev.start<dayEnd&&(ev.end||ev.start)>=dayStart).map(ev=>dtDaySegment(day,ev));
  const model={day,evs:[...evs],eventKeys:new Map(),allDay,timed,canTimeline:timed.length>0||allDay.length>0,timeline:null,dtCardPaint:null};
  model.evs.forEach((ev,i)=>model.eventKeys.set(ev,i));
  if(!timed.length)return model;
  let minMin=1440,maxMin=0;
  for(const seg of timed){minMin=Math.min(minMin,seg.startMin);maxMin=Math.max(maxMin,seg.endMin);}
  const startHour=dtClamp(Math.floor(minMin/60)-1,0,23),endHour=dtClamp(Math.ceil(maxMin/60)+1,startHour+1,24);
  const hourPx=window.innerHeight<650?68:(window.innerWidth<900?76:86),totalMinutes=(endHour-startHour)*60;
  const laid=dtAssignLanes(timed),laneGap=window.innerWidth<900?8:10,maxReadableLanes=dtMaxReadableLanes(),clusters=new Map();
  for(const seg of laid)if((seg.laneCount||1)>maxReadableLanes){const key=seg.clusterId||0;if(!clusters.has(key))clusters.set(key,[]);clusters.get(key).push(seg);}
  const stacks=new Map();
  for(const [key,items] of clusters){
    items.sort((a,b)=>a.startMin-b.startMin||a.endMin-b.endMin);
    const span=Math.max(1,(items[0].clusterEnd||items[items.length-1].endMin)-(items[0].clusterStart||items[items.length-1].startMin));
    items.forEach((seg,index)=>{seg._stackIdx=index;});
    stacks.set(key,{items,extra:Math.max(0,items.length*54+Math.max(0,items.length-1)*7-(span/60)*hourPx)});
  }
  const extraStackPx=[...stacks.values()].reduce((sum,x)=>sum+x.extra,0),gridHeightPx=(endHour-startHour)*hourPx+extraStackPx;
  model.timeline={startHour,endHour,totalMinutes,hourPx,laid,laneGap,stacks,extraStackPx,gridHeightPx};
  return model;
}
function dtBuildTimelineCard(seg,model){
  const t=model.timeline,ev=seg.ev,color=dtEventColor(ev),card=el("button","dt-event "+classify(ev));card.type="button";
  const duration=seg.endMin-seg.startMin;let height=Math.max(42,(duration/60)*t.hourPx),mode=(height<=48||duration<=35)?"tiny":((height<76||duration<=55)?"compact":"normal");
  const stack=t.stacks.get(seg.clusterId||0);
  if(stack){
    const idx=Number.isInteger(seg._stackIdx)?seg._stackIdx:0,top=((seg.clusterStart-t.startHour*60)/60)*t.hourPx;
    card.style.top=(top+idx*61)+"px";height=54;mode="compact";card.style.height=height+"px";card.style.left="0px";card.style.width="100%";card.classList.add("dt-event-stacked");
  }else{
    const layout=dtLaneLayout(seg.lane||0,seg.laneCount||1,t.laneGap);
    card.style.top=((seg.startMin-t.startHour*60)/t.totalMinutes)*100+"%";card.style.height=height+"px";card.style.left=layout.left;card.style.width=layout.width;
  }
  card.classList.add(mode==="tiny"?"dt-event-tiny":mode==="compact"?"dt-event-compact":"dt-event-normal");
  dtApplyCardColor(card,color,"timeline",model);dtMarkEventCard(card,ev,model);card.style.zIndex="2";
  dtAddEventContents(card,seg,mode);
  if(mode!=="tiny"){const meta=dtCalendarMeta(ev);if(meta)card.appendChild(meta);}
  return card;
}
function dtBuildTimelineView(model){
  if(!model.timed.length){
    const all=el("div","dt-allday dt-allday-only");all.appendChild(el("div","dt-section-label","All day"));
    for(const ev of model.allDay)all.appendChild(dtBuildAllDayCard(ev,model));return all;
  }
  const t=model.timeline,wrap=el("div","dt-wrap");
  if(model.allDay.length){const all=el("div","dt-allday");all.appendChild(el("div","dt-section-label","All day"));for(const ev of model.allDay)all.appendChild(dtBuildAllDayCard(ev,model));wrap.appendChild(all);}
  const grid=el("div","dt-grid");grid.style.minHeight=t.gridHeightPx+"px";
  const hourcol=el("div","dt-hours"),lines=el("div","dt-lines"),hours=document.createDocumentFragment(),lineFrag=document.createDocumentFragment();
  for(let h=t.startHour;h<=t.endHour;h++){
    const top=((h-t.startHour)*60/t.totalMinutes)*100,lab=el("div","dt-hour",dtFormatHour(h)),line=el("div","dt-line");
    lab.style.top=top+"%";line.style.top=top+"%";hours.appendChild(lab);lineFrag.appendChild(line);
  }
  hourcol.appendChild(hours);lines.appendChild(lineFrag);
  const events=el("div","dt-events");
  // Lite uses a software painter. Append cards only during the one-time opening
  // stage; scrolling this static grid must never add/remove nodes or read layout.
  wrap._dtTimelineStage={events,laid:t.laid,model,index:0,task:null,complete:false};
  grid.append(hourcol,lines,events);wrap.appendChild(grid);return wrap;
}
function dtCancelTimelineStage(view){
  const stage=view&&view._dtTimelineStage;
  if(stage&&stage.task){stage.task.cancel();stage.task=null;}
}
function dtStageTimelineCards(token,view){
  const stage=view&&view._dtTimelineStage;
  if(!stage||stage.complete||stage.task)return;
  function step(ctx){
    stage.task=null;
    if(!ctx.isCurrent()||!view.isConnected)return;
    const frag=document.createDocumentFragment(),limit=Math.min(stage.index+DT_CARD_STAGE_CHUNK,stage.laid.length);
    for(;stage.index<limit;stage.index++)frag.appendChild(dtBuildTimelineCard(stage.laid[stage.index],stage.model));
    stage.events.appendChild(frag);
    if(stage.index<stage.laid.length)stage.task=popupDefer(token,step);
    else stage.complete=true;
  }
  stage.task=popupDefer(token,step);
}
function dtBuildCachedView(model,kind){
  const content=el("div","dt-viewcontent");
  if(kind==="timeline"){
    const timeline=dtBuildTimelineView(model);content.appendChild(timeline);
    content._dtTimelineStage=timeline._dtTimelineStage||null;
  }else content.appendChild(dtBuildListView(model.day,model.evs,model));
  return content;
}
function dtBindUserScrollGuard(body,token){
  if(!body)return;
  if(typeof body._dtRemoveScrollGuard==="function")body._dtRemoveScrollGuard();
  body._dtUserScrolled=false;
  const mark=()=>{body._dtUserScrolled=true;};
  const types=["pointerdown","wheel","touchstart"];
  for(const type of types)body.addEventListener(type,mark,{passive:true,once:true});
  const remove=()=>{for(const type of types)body.removeEventListener(type,mark);};
  body._dtRemoveScrollGuard=remove;
  popupDefer(token,ctx=>ctx.onCancel(()=>{remove();if(body._dtRemoveScrollGuard===remove)body._dtRemoveScrollGuard=null;}));
}
function dtScrollInitialView(body,view,kind,model){
  if(!body||!view||view.dataset.dtScrolled)return;
  view.dataset.dtScrolled="pending";
  popupNextFrame(()=>{
    if(!body.contains(view))return;
    view.dataset.dtScrolled="1";
    if(body._dtUserScrolled)return;
    const offset=kind==="list"?Math.max(18,body.clientHeight*.10):Math.max(28,body.clientHeight*.22);
    if(kind==="list"){
      const first=view.querySelector(".dt-list-card");if(first)body.scrollTop=Math.max(0,first.offsetTop-offset);
      return;
    }
    const t=model&&model.timeline,grid=view.querySelector(".dt-grid");
    if(!t||!grid||!t.laid||!t.laid.length)return;
    let firstStart=1440;for(const seg of t.laid)firstStart=Math.min(firstStart,seg.startMin);
    const withinGrid=((firstStart-t.startHour*60)/60)*t.hourPx;
    body.scrollTop=Math.max(0,grid.offsetTop+withinGrid-offset);
  });
}
function dtLiteDayPopupProfile(){
  const profile=typeof CONFIG!=="undefined"&&CONFIG?String(CONFIG.profile||"").toLowerCase():"";
  return ["lite","zero2","low","low-power"].includes(profile);
}

// App-owned household calendar entries use a focused, actionable popup. The
// specialized renderer lives in the adjacent module so Timeline/List behavior
// remains small and independently testable.
function showAppCalendarGroupPopup(day,group){
  if(typeof showActionableAppCalendarGroupPopup==="function"){
    showActionableAppCalendarGroupPopup(day,group);
    return;
  }
  showDayPopup(day,(group&&group.events)||[]);
}

function showDayPopup(day,evs){
  const model=dtBuildDayModel(day,evs),initial=(dtLiteDayPopupProfile()&&model.timed.length)?"list":(model.timed.length?"timeline":"list");let session=null;
  popupOpenTransaction({
    mode:"daytimelinepop",title:FMT.dayLong.format(day),when:evs.length+(evs.length===1?" event":" events"),loading:"Preparing day timeline…",
    afterCommit:(token,body)=>{if(session&&session.token===token)session.afterCommit(body);}
  },token=>{
    if(!evs.length)return el("div",null,"No events.");
    dtBeginDayCardPaintContext(model);
    const root=el("div","dt-popup-session"),host=el("div","dt-viewhost"),state={view:initial,views:{},body:null};
    root.addEventListener("click",e=>{
      const card=e.target.closest("[data-dt-evid]");
      if(!card||!root.contains(card))return;
      const ev=model.evs[Number(card.dataset.dtEvid)];
      if(ev){e.stopPropagation();showEventPopup(ev);}
    });
    function viewFor(kind){return state.views[kind]||(state.views[kind]=dtBuildCachedView(model,kind));}
    function mountView(kind){
      const previous=state.view;
      if(previous==="timeline"&&kind!=="timeline")dtCancelTimelineStage(state.views.timeline);
      state.view=kind;toolbar.setActive(kind);
      const view=viewFor(kind);host.replaceChildren(view);
      if(state.body){
        dtScrollInitialView(state.body,view,kind,model);
        if(kind==="timeline")dtStageTimelineCards(token,view);
      }
    }
    const toolbar=dtBuildViewBar(initial,next=>{if(next!==state.view&&popupIsCurrent(token))mountView(next);},model.timed.length>0);
    root.append(toolbar.bar,host);const first=viewFor(initial);host.replaceChildren(first);
    session={token,afterCommit(body){
      state.body=body;
      // The day timeline is a special static root: input only suppresses the
      // initial position jump; scroll itself never stages, virtualizes, or
      // measures event cards.
      body.dataset.scrollPolicy="day-timeline";
      dtBindUserScrollGuard(body,token);
      popupDefer(token,ctx=>ctx.onCancel(()=>dtCancelTimelineStage(state.views.timeline)));
      dtScrollInitialView(body,first,initial,model);
      if(initial==="timeline")dtStageTimelineCards(token,first);
    }};
    return root;
  });
}
