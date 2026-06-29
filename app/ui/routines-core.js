// routines-core.js — pure local helpers for the Routines overlay.
(function(){
  const dateKey=value=>{
    const date=value instanceof Date?value:new Date(value);
    if(Number.isNaN(+date))return "";
    return date.getFullYear()+"-"+String(date.getMonth()+1).padStart(2,"0")+"-"+String(date.getDate()).padStart(2,"0");
  };
  const list=value=>Array.isArray(value)?value:[];
  const completed=(session)=>new Set(list(session&&session.completedStepIds).map(String));
  const progress=session=>{
    const steps=list(session&&session.steps),done=completed(session);
    return {done:steps.filter(step=>done.has(String(step&&step.id))).length,total:steps.length};
  };
  const scheduleLabel=schedule=>{
    const row=schedule||{},kind=String(row.kind||"weekdays"),every=Math.max(1,Number(row.every)||1),time=row.allDay!==false?"All day":(row.time||"Any time");
    const labels={days:`Every ${every} day${every===1?"":"s"}`,weekdays:`Every ${every} week${every===1?"":"s"} on weekdays`,weekly:`Every ${every} week${every===1?"":"s"}`,monthly:`Every ${every} month${every===1?"":"s"}`,yearly:`Every ${every} year${every===1?"":"s"}`,once:"One time"};
    return (labels[kind]||"Scheduled")+" · "+time;
  };
  window.routinesCore=Object.freeze({dateKey,list,completed,progress,scheduleLabel});
})();
