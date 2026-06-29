async function renderCtrlTempMessages(){
  const wrap=$("#ctrltempmsg"); if(!wrap) return;
  ctrlSetLoading(wrap,"Loading temporary messages…","Checking active high-priority messages.");
  let data;
  try{ data=await api("/api/temporary-messages"); }
  catch(e){ ctrlSetError(wrap,"Temporary messages unavailable",friendlyUnavailable("Temporary messages",e)); return; }
  wrap.innerHTML="";
  _oskTarget=null;
  const intro=ctrlStateCard("info","Temporary high-priority message","Shows often until it expires. Choose a preset duration; only the message text uses the on-screen keyboard.");
  wrap.appendChild(intro);
  const form=el("div","tempmsgform messageform");
  const text=oskInput("temporary message text","");
  form.appendChild(msgField("Message",text));
  const durPick=msgPresetPicker([[30,"30 min"],[60,"1 hr"],[120,"2 hr"],[360,"6 hr"],[720,"12 hr"],[1440,"24 hr"]],120,{title:"Duration",detail:"Temporary messages use fixed presets."});
  form.appendChild(durPick.root);
  const weightPick=msgWeightPicker(500,{title:"Temporary priority",detail:"Temporary messages usually need Spotlight or Dominant."});
  form.appendChild(weightPick.root);
  const row=el("div","crow msgactiongrid singleaction");
  const addTemporary=cbtn("Add temporary message","on",async()=>{
    try{
      await api("/api/temporary-messages/add","POST",{text:text.value,expires:String(durPick.get())+"m",weight:weightPick.get()});
      await loadCompliments(); ctrlMsg("Temporary message added."); await stableMessageAction(()=>renderCtrlTempMessages());
    }catch(e){ ctrlMsg("Could not add temporary message: "+e.message); }
  });
  oskSetSubmit(text,"Add",()=>addTemporary.click());
  row.appendChild(addTemporary);
  form.appendChild(row); wrap.appendChild(form);
  const list=el("div","templist"); wrap.appendChild(list);
  const now=Date.now();
  const items=(data.items||[]).filter(x=>+x.expiresAt>now);
  if(!items.length) list.appendChild(ctrlStateCard("empty","No active temporary messages","Add one above when something should dominate rotation for a while."));
  for(const m of items){
    const r=el("div","comprow tempmsgrow");
    r.appendChild(el("span","ct",m.text||""));
    r.appendChild(el("span","cm","expires "+fmtExpiry(m.expiresAt)+" · "+msgWeightLabel(m.weight||500)));
    r.appendChild(cbtn("Del","danger",async()=>{
      try{ await api("/api/temporary-messages/delete","POST",{id:m.id}); await loadCompliments(); ctrlMsg("Temporary message removed."); await stableMessageAction(()=>renderCtrlTempMessages()); }
      catch(e){ ctrlMsg(e.message); }
    }));
    list.appendChild(r);
  }
}

async function renderCtrlScheduledMessages(){
  const wrap=$("#ctrlschedmsg"); if(!wrap) return;
  ctrlSetLoading(wrap,"Loading scheduled messages…","Reading local time-window rules.");
  let data;
  try{ data=await api("/api/scheduled-messages"); }
  catch(e){ ctrlSetError(wrap,"Scheduled messages unavailable",friendlyUnavailable("Scheduled messages",e)); return; }
  wrap.innerHTML=""; _oskTarget=null;
  const intro=ctrlStateCard("info","Scheduled messages","Show a message only during a time window, optionally recurring daily, weekly, bi-weekly, every X weeks, monthly, every X months, or yearly.");
  wrap.appendChild(intro);
  const list=el("div","schedlist");
  const add=cbtn("Add scheduled message","on",()=>editor(null));
  const top=el("div","crow msgactiongrid singleaction"); top.appendChild(add); wrap.appendChild(top); wrap.appendChild(list);
  const items=data.items||[];
  function drawList(){
    list.innerHTML="";
    if(!items.length) list.appendChild(ctrlStateCard("empty","No scheduled messages","Add a message that only appears during a chosen window."));
    for(const m of items){
      const r=el("div","comprow schedmsgrow");
      r.appendChild(el("span","ct",m.text||""));
      r.appendChild(el("span","cm",schedSummary(m)+" · "+msgWeightLabel(m.weight||25)));
      r.appendChild(cbtn("Edit","",()=>editor(m)));
      r.appendChild(cbtn("Del","danger",async()=>{
        try{ await api("/api/scheduled-messages/delete","POST",{id:m.id}); await loadCompliments(); ctrlMsg("Scheduled message removed."); await stableMessageAction(()=>renderCtrlScheduledMessages()); }
        catch(e){ ctrlMsg(e.message); }
      }));
      list.appendChild(r);
    }
  }
  function editor(m){
    list.innerHTML=""; top.style.display="none"; _oskTarget=null;
    const form=el("div","schededitor messageform");
    const text=oskInput("scheduled message text",m?m.text:"");
    const startDate=oskInput("start date YYYY-MM-DD",m&&m.startDate?m.startDate:localDateISO(),{mode:"date"});
    const startTime=oskInput("start time HH:MM",m&&m.startTime?m.startTime:"08:00",{mode:"time"});
    const endTime=oskInput("end time HH:MM",m&&m.endTime?m.endTime:"21:00",{mode:"time"});
    const endDate=oskInput("optional end date YYYY-MM-DD",m&&m.endDate?m.endDate:"",{mode:"date"});
    form.appendChild(msgField("Message",text));
    const timegrid=el("div","msgfieldgrid");
    timegrid.append(msgField("Start date",startDate),msgField("Start time",startTime),msgField("End time",endTime),msgField("End date",endDate,"For one-time messages this is the stop date; for recurring messages it is optional."));
    form.appendChild(timegrid);
    let recurrence=(m&&m.recurrence)||"once";
    let days=new Set((m&&m.days)||[dayFromISO((m&&m.startDate)||localDateISO())]);
    let intervalMonths=(m&&m.intervalMonths)||2;
    let intervalWeeks=(m&&m.intervalWeeks)||3;
    const recWrap=el("div","recurrencebox"); form.appendChild(recWrap);
    function drawRecurrence(){
      recWrap.innerHTML="";
      recWrap.appendChild(el("div","msgfield-label","Repeat"));
      const opts=[["once","One time"],["daily","Daily"],["weekly","Weekly"],["biweekly","Bi-weekly"],["xweeks","Every X weeks"],["monthly","Monthly"],["xmonths","Every X months"],["yearly","Yearly"]];
      const grid=el("div","recurrencegrid");
      for(const [key,label] of opts){
        const b=cbtn(label,key===recurrence?"on":"",()=>{recurrence=key;drawRecurrence();});
        grid.appendChild(b);
      }
      recWrap.appendChild(grid);
      if(recurrence==="weekly" || recurrence==="biweekly" || recurrence==="xweeks"){
        recWrap.appendChild(el("div","msgfield-detail","Choose which days of the week are active."));
        const daygrid=el("div","daygrid");
        MSG_DAYS.forEach((name,idx)=>{
          const b=cbtn(name,days.has(idx)?"on":"",()=>{days.has(idx)?days.delete(idx):days.add(idx);drawRecurrence();});
          daygrid.appendChild(b);
        });
        recWrap.appendChild(daygrid);
      }
      if(recurrence==="xweeks"){
        recWrap.appendChild(el("div","msgfield-detail","Choose an interval greater than bi-weekly and less than monthly."));
        const wgrid=el("div","daygrid weekgrid");
        for(const n of [3,4]){
          const b=cbtn(String(n)+" wk",intervalWeeks===n?"on":"",()=>{intervalWeeks=n;drawRecurrence();});
          wgrid.appendChild(b);
        }
        recWrap.appendChild(wgrid);
      }
      if(recurrence==="xmonths"){
        recWrap.appendChild(el("div","msgfield-detail","Choose an interval less than 12 months."));
        const mgrid=el("div","daygrid monthgrid");
        for(const n of [2,3,4,5,6,7,8,9,10,11]){
          const b=cbtn(String(n)+" mo",intervalMonths===n?"on":"",()=>{intervalMonths=n;drawRecurrence();});
          mgrid.appendChild(b);
        }
        recWrap.appendChild(mgrid);
      }
    }
    drawRecurrence();
    const weightPick=msgWeightPicker(m&&m.weight?m.weight:25,{title:"Scheduled message priority",detail:"Priority is the usual starting point for scheduled messages."});
    form.appendChild(weightPick.root);
    const btns=el("div","crow");
    const saveScheduled=cbtn(m?"Save scheduled message":"Add scheduled message","on",async()=>{
      const body={text:text.value,startDate:startDate.value,startTime:startTime.value,endTime:endTime.value,endDate:endDate.value,recurrence,weight:weightPick.get(),days:[...days],intervalMonths,intervalWeeks};
      if(m) body.id=m.id;
      try{
        await api(m?"/api/scheduled-messages/update":"/api/scheduled-messages/add","POST",body);
        await loadCompliments(); ctrlMsg(m?"Scheduled message updated.":"Scheduled message added."); await stableMessageAction(()=>renderCtrlScheduledMessages());
      }catch(e){ ctrlMsg("Could not save scheduled message: "+e.message); }
    });
    [text,startDate,startTime,endTime,endDate].forEach(input=>oskSetSubmit(input,m?"Save":"Add",()=>saveScheduled.click()));
    btns.appendChild(saveScheduled);
    btns.appendChild(cbtn("Cancel","",()=>renderCtrlScheduledMessages()));
    form.appendChild(btns);
    list.appendChild(form);
  }
  drawList();
}
