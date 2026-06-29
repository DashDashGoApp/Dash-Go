// maintenance-core.js — local date labels/grouping for the Maintenance app.
(function(){
  function text(v,fallback){return String(v==null?fallback||"":v).trim();}
  function dateKey(v){const out=text(v);return /^\d{4}-\d{2}-\d{2}$/.test(out)?out:"";}
  function today(){const now=new Date();return `${now.getFullYear()}-${String(now.getMonth()+1).padStart(2,"0")}-${String(now.getDate()).padStart(2,"0")}`;}
  function dateLabel(v){const key=dateKey(v);if(!key)return "—";const [y,m,d]=key.split("-").map(Number);return new Date(y,m-1,d).toLocaleDateString(undefined,{month:"short",day:"numeric",year:"numeric"});}
  function status(task,summary){const now=(summary&&summary.today)||today(),soon=Number(summary&&summary.dueSoonDays)||30,day=dateKey(task.nextDueOn);if(!day)return"later";if(day<now)return"overdue";if(day===now)return"today";const a=new Date(`${now}T00:00:00`),b=new Date(`${day}T00:00:00`);return (b-a)/(864e5)<=soon?"soon":"later";}
  function dueLabel(task,summary){const kind=status(task,summary),day=dateLabel(task.nextDueOn);if(kind==="overdue"){const diff=Math.max(1,Math.round((new Date(`${(summary&&summary.today)||today()}T00:00:00`)-new Date(`${task.nextDueOn}T00:00:00`))/864e5));return `Due ${day} · ${diff} day${diff===1?"":"s"} overdue`;};if(kind==="today")return `Due today · ${day}`;return `Due ${day}`;}
  function cadenceLabel(task){const c=task&&task.cadence||{},n=Number(c.every)||1,u=String(c.unit||"months");return `Every ${n} ${u}`;}
  function active(tasks){return(Array.isArray(tasks)?tasks:[]).filter(task=>task&&task.state==="active").slice().sort((a,b)=>String(a.nextDueOn||"").localeCompare(String(b.nextDueOn||"")));}
  window.maintenanceCore={text,dateKey,today,dateLabel,status,dueLabel,cadenceLabel,active};
})();
