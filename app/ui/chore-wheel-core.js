// chore-wheel-core.js — pure local schedule, fairness, and payload helpers.
(function(){
"use strict";
const DAY_NAMES=["Sunday","Monday","Tuesday","Wednesday","Thursday","Friday","Saturday"];
const asArray=value=>Array.isArray(value)?value:[];
const text=value=>String(value==null?"":value).trim();
const clamp=(value,low,high)=>Math.max(low,Math.min(high,Number(value)||low));

function localDateKey(value){
  const date=value instanceof Date?value:new Date(value);
  return `${date.getFullYear()}-${String(date.getMonth()+1).padStart(2,"0")}-${String(date.getDate()).padStart(2,"0")}`;
}
function validDateKey(value){ return /^\d{4}-\d{2}-\d{2}$/.test(text(value)); }
function dateFromKey(key){
  const parts=text(key).split("-").map(Number);
  return new Date(parts[0]||1970,(parts[1]||1)-1,parts[2]||1,12,0,0,0);
}
function addDays(key,amount){
  const date=dateFromKey(key);
  date.setDate(date.getDate()+Number(amount||0));
  return localDateKey(date);
}
function dayIndex(key){
  const [year,month,day]=text(key).split("-").map(Number);
  return Math.floor(Date.UTC(year,month-1,day)/86400000);
}
function legacyAnchor(value){
  const date=value?new Date(value):new Date();
  return localDateKey(Number.isNaN(date.getTime())?new Date():date);
}
function cleanCadence(value,createdAt){
  const raw=value&&typeof value==="object"?value:{};
  const type=["daily","weekdays","weekly","days"].includes(raw.type)?raw.type:"daily";
  return {
    type,
    day:clamp(raw.day,0,6),
    every:clamp(raw.every,1,365),
    anchorDate:validDateKey(raw.anchorDate)?text(raw.anchorDate):legacyAnchor(createdAt),
  };
}
function uniqueIds(values,allowed){
  const seen=new Set();
  return asArray(values).map(text).filter(id=>allowed.has(id)&&!seen.has(id)&&(seen.add(id),true));
}
function normalize(raw){
  const source=raw&&typeof raw==="object"&&!Array.isArray(raw)?raw:{};
  const people=[];
  const seenPeople=new Set();
  for(const item of asArray(source.people)){
    const row=item&&typeof item==="object"?item:{};
    const id=text(row.id),name=text(row.name).slice(0,64);
    if(!id||!name||seenPeople.has(id))continue;
    seenPeople.add(id);people.push({id,name});
  }
  const peopleIds=new Set(people.map(person=>person.id));
  const chores=[];
  const seenChores=new Set();
  for(const item of asArray(source.chores)){
    const row=item&&typeof item==="object"?item:{};
    const id=text(row.id),name=text(row.name).slice(0,96);
    if(!id||!name||seenChores.has(id))continue;
    seenChores.add(id);
    const createdAt=text(row.createdAt)||new Date().toISOString();
    chores.push({
      id,name,createdAt,
      cadence:cleanCadence(row.cadence,createdAt),
      effort:clamp(row.effort,1,3),
      eligible:uniqueIds(row.eligible,peopleIds),
    });
  }
  const assignments=[];
  const seenAssignments=new Set();
  for(const item of asArray(source.assignments)){
    const row=item&&typeof item==="object"?item:{};
    const id=text(row.id),date=text(row.date),choreId=text(row.choreId),personId=text(row.personId);
    if(!id||!validDateKey(date)||!choreId||!personId||seenAssignments.has(id))continue;
    seenAssignments.add(id);
    const status=["assigned","completed","skipped"].includes(row.status)?row.status:"assigned";
    assignments.push({
      id,date,choreId,personId,status,
      choreName:text(row.choreName).slice(0,96),
      personName:text(row.personName).slice(0,64),
      source:text(row.source).slice(0,24),
    });
  }
  const settings=source.settings&&typeof source.settings==="object"?source.settings:{};
  return {schema:1,revision:Math.max(0,Math.floor(Number(source.revision)||0)),people,chores,assignments,settings:{horizonDays:clamp(settings.horizonDays,1,30),calendarOutputEnabled:settings.calendarOutputEnabled!==false}};
}
function cadenceText(cadence){
  const c=cleanCadence(cadence,"");
  if(c.type==="weekly")return `Weekly · ${DAY_NAMES[c.day]}`;
  if(c.type==="days")return `Every ${c.every} days`;
  return c.type==="weekdays"?"Weekdays":"Daily";
}
function isDue(chore,key){
  const c=cleanCadence(chore.cadence,chore.createdAt);
  const date=dateFromKey(key),weekday=date.getDay();
  if(c.type==="daily")return true;
  if(c.type==="weekdays")return weekday>0&&weekday<6;
  if(c.type==="weekly")return weekday===c.day;
  const delta=dayIndex(key)-dayIndex(c.anchorDate);
  return delta>=0&&delta%c.every===0;
}
function dueChores(data,key){ return data.chores.filter(chore=>isDue(chore,key)); }
function choreIndexes(data,key){
  const peopleByID=new Map(),choreByID=new Map(),assignmentByChoreDate=new Map();
  const eligibleByChore=new Map(),monthLoad=new Map(),lastByPerson=new Map(),lastByChore=new Map();
  const monthPrefix=text(key).slice(0,7);
  for(const person of data.people)peopleByID.set(person.id,person);
  for(const chore of data.chores){
    choreByID.set(chore.id,chore);
    const allowed=asArray(chore.eligible);
    eligibleByChore.set(chore.id,allowed.length?new Set(allowed):new Set(peopleByID.keys()));
  }
  for(const item of data.assignments){
    assignmentByChoreDate.set(`${item.choreId}|${item.date}`,item);
    if(item.status==="skipped")continue;
    const currentChore=choreByID.get(item.choreId);
    if(item.date.slice(0,7)===monthPrefix){
      monthLoad.set(item.personId,(monthLoad.get(item.personId)||0)+clamp(currentChore&&currentChore.effort,1,3));
    }
    if(item.date<=key&&item.date>(lastByPerson.get(item.personId)||""))lastByPerson.set(item.personId,item.date);
    const prior=lastByChore.get(item.choreId);
    if(!prior||item.date>prior.date||(item.date===prior.date&&item.id>prior.id))lastByChore.set(item.choreId,item);
  }
  return {key,monthPrefix,peopleByID,choreByID,assignmentByChoreDate,eligibleByChore,monthLoad,lastByPerson,lastByChore};
}
function eligiblePeople(data,chore,index){
  const view=index||choreIndexes(data,"");
  const ids=view.eligibleByChore.get(chore.id);
  return data.people.filter(person=>!ids||ids.has(person.id));
}
function assignmentFor(data,choreId,key,index){
  const view=index||null;
  if(view)return view.assignmentByChoreDate.get(`${choreId}|${key}`)||null;
  return data.assignments.find(item=>item.choreId===choreId&&item.date===key)||null;
}
function fairLoad(data,personId,key,index){
  const view=index||choreIndexes(data,key);
  return view.monthLoad.get(personId)||0;
}
function stableHash(value){
  let hash=2166136261;
  for(const char of text(value)){ hash^=char.charCodeAt(0); hash=Math.imul(hash,16777619); }
  return hash>>>0;
}
function lastAssignedKey(data,personId,key,index){
  const view=index||choreIndexes(data,key);
  return view.lastByPerson.get(personId)||"";
}
function chooseFairCandidate(data,chore,key,excludeId,index){
  const view=index||choreIndexes(data,key);
  const candidates=eligiblePeople(data,chore,view);
  if(!candidates.length)return null;
  const prior=view.lastByChore.get(chore.id);
  let pool=candidates.filter(person=>(!prior||prior.personId!==person.id)&&person.id!==excludeId);
  if(!pool.length)pool=candidates.filter(person=>person.id!==excludeId);
  if(!pool.length)pool=candidates;
  // Decorate once, sort tiny immutable tuples, then return the person. This
  // keeps fairness inputs O(assignments) per decision rather than repeatedly
  // scanning assignments inside each sort comparison.
  return pool.map(person=>({
    person,
    load:view.monthLoad.get(person.id)||0,
    recent:view.lastByPerson.get(person.id)||"",
    seed:stableHash(`${key}|${chore.id}|${person.id}`),
  })).sort((left,right)=>
    left.load-right.load ||
    left.recent.localeCompare(right.recent) ||
    left.seed-right.seed ||
    left.person.id.localeCompare(right.person.id)
  )[0]?.person||null;
}
function noteFairAssignment(index,chore,person,key){
  if(!index||!chore||!person)return;
  const item={id:assignmentID(chore,person,key),date:key,choreId:chore.id,personId:person.id,status:"assigned"};
  index.assignmentByChoreDate.set(`${chore.id}|${key}`,item);
  if(key.slice(0,7)===index.monthPrefix)index.monthLoad.set(person.id,(index.monthLoad.get(person.id)||0)+clamp(chore.effort,1,3));
  if(key<=index.key&&key>(index.lastByPerson.get(person.id)||""))index.lastByPerson.set(person.id,key);
  const prior=index.lastByChore.get(chore.id);
  if(!prior||key>prior.date||(key===prior.date&&item.id>prior.id))index.lastByChore.set(chore.id,item);
}
function assignmentID(chore,person,key){ return `a-${key}-${stableHash(`${key}|${chore.id}|${person.id}`).toString(36)}`; }
function makeAssignment(chore,person,key,source){
  return {
    id:assignmentID(chore,person,key),
    date:key,choreId:chore.id,choreName:chore.name,
    personId:person.id,personName:person.name,status:"assigned",source:source||"manual",
  };
}
function recentAssignments(data,limit){
  return data.assignments.slice().sort((left,right)=>right.date.localeCompare(left.date)||right.id.localeCompare(left.id)).slice(0,Math.max(1,limit||60));
}
function isFutureAssigned(item,key){ return !!(item&&item.status==="assigned"&&item.date>key); }
function removeChoreAssignments(data,choreId,key){
  const id=text(choreId);let removed=0;
  data.chores=data.chores.filter(chore=>chore.id!==id);
  data.assignments=data.assignments.filter(item=>{
    if(item.choreId===id&&isFutureAssigned(item,key)){removed++;return false;}
    return true;
  });
  return {removed};
}
function removePersonAssignments(data,personId,key,mode){
  const id=text(personId);let reassigned=0,removed=0;
  data.people=data.people.filter(person=>person.id!==id);
  data.chores.forEach(chore=>{chore.eligible=asArray(chore.eligible).filter(value=>value!==id);});
  const pending=[],remaining=[];
  for(const item of data.assignments){
    if(item.personId!==id||!isFutureAssigned(item,key)){remaining.push(item);continue;}
    if(mode!=="reassign"){removed++;continue;}
    pending.push(item);
  }
  // Reassign in a stable order against a working schedule. Each saved choice
  // affects fairness scoring for every subsequent replacement.
  data.assignments=remaining;
  const choreByID=new Map(data.chores.map(chore=>[chore.id,chore]));
  const plansByDate=new Map();
  pending.sort((left,right)=>left.date.localeCompare(right.date)||left.id.localeCompare(right.id));
  for(const item of pending){
    const chore=choreByID.get(item.choreId);
    const plan=plansByDate.get(item.date)||choreIndexes(data,item.date);
    plansByDate.set(item.date,plan);
    const winner=chore&&chooseFairCandidate(data,chore,item.date,id,plan);
    if(!winner){removed++;continue;}
    reassigned++;
    data.assignments.push({...item,personId:winner.id,personName:winner.name,status:"assigned"});
    noteFairAssignment(plan,chore,winner,item.date);
  }
  return {reassigned,removed};
}


window.ChoreWheelCore={
  DAY_NAMES,addDays,assignmentFor,cadenceText,choreIndexes,chooseFairCandidate,cleanCadence,
  dateFromKey,dayIndex,dueChores,eligiblePeople,fairLoad,isDue,lastAssignedKey,localDateKey,
  makeAssignment,normalize,noteFairAssignment,recentAssignments,removeChoreAssignments,removePersonAssignments,stableHash,validDateKey,
};
})();
