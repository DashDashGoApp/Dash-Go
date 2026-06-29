// Calendar fit completion callbacks stay outside the grid renderer so the
// renderer remains focused on DOM construction and the builder source cap.
const CALENDAR_LAYOUT_FIT_WAITERS=[];
function calendarAfterLayoutFit(renderSerial,callback){
  if(typeof callback!=="function")return;
  CALENDAR_LAYOUT_FIT_WAITERS.push({renderSerial:Math.max(0,Number(renderSerial)||0),callback});
}
function calendarLayoutFitDidComplete(renderSerial){
  const complete=Math.max(0,Number(renderSerial)||0);
  const ready=CALENDAR_LAYOUT_FIT_WAITERS.splice(0,CALENDAR_LAYOUT_FIT_WAITERS.length);
  for(const waiter of ready){
    if(waiter.renderSerial<=complete){
      try{waiter.callback();}catch(err){console.warn("calendar fit completion failed",err);}
    }else{
      CALENDAR_LAYOUT_FIT_WAITERS.push(waiter);
    }
  }
}
