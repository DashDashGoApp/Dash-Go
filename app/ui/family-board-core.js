// family-board-core.js — local ordering and display helpers for Family Message Board.
(function(){
  function text(value,fallback){return String(value==null?fallback||"":value).trim();}
  function dateKey(value){const out=text(value);return /^\d{4}-\d{2}-\d{2}$/.test(out)?out:"";}
  function localToday(){const now=new Date();return `${now.getFullYear()}-${String(now.getMonth()+1).padStart(2,"0")}-${String(now.getDate()).padStart(2,"0")}`;}
  function expiryDate(value){const ms=Date.parse(text(value));return Number.isFinite(ms)?new Date(ms):null;}
  function sameLocalDay(a,b){return a&&b&&a.getFullYear()===b.getFullYear()&&a.getMonth()===b.getMonth()&&a.getDate()===b.getDate();}
  function expirationLabel(value){
    const date=expiryDate(value);if(!date)return "Keeps until archived";
    const now=new Date(),tomorrow=new Date(now.getFullYear(),now.getMonth(),now.getDate()+1);
    const hasClock=date.getHours()!==0||date.getMinutes()!==0;
    const time=date.toLocaleTimeString(undefined,{hour:"numeric",minute:"2-digit"});
    if(sameLocalDay(date,now))return `Expires today at ${time}`;
    if(sameLocalDay(date,tomorrow))return `Expires tomorrow at ${time}`;
    const day=date.toLocaleDateString(undefined,{month:"short",day:"numeric"});
    return hasClock?`Expires ${day} at ${time}`:`Expires after ${day}`;
  }
  function active(notes){return (Array.isArray(notes)?notes:[]).filter(note=>note&&note.state==="active");}
  function archived(notes){return (Array.isArray(notes)?notes:[]).filter(note=>note&&note.state==="archived");}
  function activeOrder(notes){
    return active(notes).slice().sort((a,b)=>{
      const au=a.priority==="urgent",bu=b.priority==="urgent";if(au!==bu)return au?-1:1;
      const ap=!!a.pinned,bp=!!b.pinned;if(ap!==bp)return ap?-1:1;
      return String(b.updatedAt||"").localeCompare(String(a.updatedAt||""));
    });
  }
  function archiveOrder(notes){return archived(notes).slice().sort((a,b)=>String(b.archivedAt||b.updatedAt||"").localeCompare(String(a.archivedAt||a.updatedAt||"")));}
  function archiveLabel(note){
    const archivedAt=expiryDate(note&&note.archivedAt),expiresAt=expiryDate(note&&note.expiresAt);
    if(archivedAt&&expiresAt&&archivedAt>=expiresAt)return `Expired ${expirationLabel(note.expiresAt).replace(/^Expires\s+/,"")}`;
    const day=archivedAt?archivedAt.toLocaleDateString(undefined,{month:"short",day:"numeric"}):"";
    return day?`Archived ${day}`:"Archived";
  }
  window.familyBoardCore={text,dateKey,localToday,expiryDate,expirationLabel,dateLabel:expirationLabel,activeOrder,archiveOrder,archiveLabel};
})();
