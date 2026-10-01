function resetCalendarSpanOffsets(){
  document.querySelectorAll("#calscroll .evlist").forEach(evlist=>{
    evlist.style.marginTop="";
    evlist.style.setProperty("--cell-span-offset","0px");
  });
}
function collectCalendarSpanOffsets(){
  // Day event lists are absolute-positioned at the same top origin as
  // multi-day span bars. For cells crossed by spans, push only that cell's
  // list down to just under the real rendered span stack.
  const updates=[];
  const rows=document.querySelectorAll("#calscroll .weekrow");
  rows.forEach(row=>{
    const children=Array.from(row.children||[]);
    const bars=children.filter(x=>x.classList && x.classList.contains("spanbar"));
    const cells=children.filter(x=>x.classList && x.classList.contains("daycell"));
    if(!bars.length || !cells.length) return;
    const rowTop=row.getBoundingClientRect().top;
    const barRects=bars.map(bar=>({
      c0:+(bar.dataset.c0||0),
      c1:+(bar.dataset.c1||-1),
      bottom:bar.getBoundingClientRect().bottom-rowTop
    }));
    cells.forEach((cell,idx)=>{
      const evlist=cell.querySelector && cell.querySelector(".evlist");
      if(!evlist || +(cell.dataset.cellLanes||0)<=0) return;
      const evTop=evlist.getBoundingClientRect().top-rowTop;
      let spanBottom=0;
      for(const br of barRects){
        if(idx<br.c0 || idx>br.c1) continue;
        if(br.bottom>spanBottom) spanBottom=br.bottom;
      }
      const offset=spanBottom?Math.max(0,Math.ceil(spanBottom+2-evTop)):0;
      updates.push({evlist,offset});
    });
  });
  return updates;
}
function applyCalendarSpanOffsets(updates){
  for(const u of updates){
    u.evlist.style.marginTop="";
    u.evlist.style.setProperty("--cell-span-offset",(u.offset||0)+"px");
  }
}
function fitCalendarSpanOffsets(){
  resetCalendarSpanOffsets();
  const updates=collectCalendarSpanOffsets();
  applyCalendarSpanOffsets(updates);
  return updates.length;
}
