// 04-calendar-01a-span-bars.js — multi-day span bar text/display helpers.
// Split from the Calendar grid renderer to keep the render transaction focused.
function spanSegmentTime(ev,which){
  if(!ev || ev.allDay) return "";
  if(which==="start" && ev.start) return FMT.time.format(ev.start);
  if(which==="end" && ev.end) return FMT.time.format(ev.end);
  return "";
}
function fillSpanBar(bar,it){
  const ev=it.ev;
  const leftTime=it.contL ? "" : spanSegmentTime(ev,"start");
  const rightTime=it.contR ? "" : spanSegmentTime(ev,"end");
  if(leftTime || rightTime || it.contL || it.contR){
    if(it.contL) bar.appendChild(el("span","spancont","‹"));
    if(leftTime) bar.appendChild(el("span","spantime spantime-start",leftTime));
    bar.appendChild(el("span","spantitle",ev.title||"(no title)"));
    if(rightTime) bar.appendChild(el("span","spantime spantime-end",rightTime));
    if(it.contR) bar.appendChild(el("span","spancont","›"));
  } else {
    bar.textContent=ev.title||"(no title)";
  }
}
