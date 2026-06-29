// 07-compliments-00c-layout.js — semantic display planning for rotating messages.
// Raw message text remains unchanged for selection/history. This module only plans
// temporary display-only line breaks when a readable composition clearly wins.
const COMP_LAYOUT={maxCandidates:8,questionGain:1.06,strongGain:1.12,softGain:1.15,threeLineGain:1.16};
function complimentDisplayText(text){
  const lines=String(text??"").replace(/\r/g,"").split("\n").map(line=>complimentCleanText(line)).filter(Boolean);
  return lines.join("\n")||complimentCleanText(text);
}
function complimentLayoutWords(text){const clean=complimentCleanText(text);return clean?clean.split(" ").length:0;}
function complimentLayoutInfo(text){const clean=complimentCleanText(text);return {text:clean,words:complimentLayoutWords(clean),chars:Array.from(clean).length};}
function complimentLayoutBreakable(left,right,kind){
  const a=complimentLayoutInfo(left),b=complimentLayoutInfo(right);if(!a.text||!b.text)return false;
  if(kind==="question-answer")return a.words>=3&&b.words>=2&&a.chars>=10&&b.chars>=8&&b.words<=14;
  if(a.words<3||b.words<3||a.chars<11||b.chars<11)return false;
  if(kind==="comma")return Math.min(a.chars,b.chars)/Math.max(a.chars,b.chars)>=.38;
  return true;
}
function complimentLayoutBalance(displayText){
  const lengths=String(displayText||"").split("\n").map(line=>Math.max(1,Array.from(line.trim()).length)).filter(Boolean);
  if(lengths.length<2)return 1;return Math.min(...lengths)/Math.max(...lengths);
}
function complimentLayoutAddCandidate(list,displayText,kind,quality){
  const normalized=complimentDisplayText(displayText),lines=normalized.split("\n").filter(Boolean);if(!normalized||!lines.length)return;
  if(list.some(candidate=>candidate.displayText===normalized))return;
  list.push({displayText:normalized,kind,quality,lines:lines.length,balance:complimentLayoutBalance(normalized)});
}
function complimentLayoutAddBreak(list,clean,index,kind,quality,positions){
  const left=clean.slice(0,index).trim(),right=clean.slice(index).trim();if(!complimentLayoutBreakable(left,right,kind))return;
  complimentLayoutAddCandidate(list,left+"\n"+right,kind,quality);positions.push({index,kind,quality});
}
function complimentLayoutPunctuationCandidates(list,clean,positions){
  const scan=(pattern,kindFor,qualityFor)=>{pattern.lastIndex=0;let match;while((match=pattern.exec(clean))){const token=match[1]||"",kind=kindFor(token),quality=qualityFor(token);complimentLayoutAddBreak(list,clean,match.index+token.length,kind,quality,positions);}};
  scan(/([?!])\s+(?=\S)/g,token=>token==="?"?"question-answer":"exclamation",token=>token==="?"?7:5);
  scan(/([.;:])\s+(?=\S)/g,token=>token==="."?"sentence":"pause",token=>token==="."?5:4);
  scan(/([—–])\s+(?=\S)/g,()=>"pause",()=>4);
  scan(/(,)\s+(?=\S)/g,()=>"comma",()=>2);
}
function complimentLayoutAddBalancedCandidate(list,clean){
  const words=clean.split(" ");if(words.length<12)return;
  let best=null;
  for(let i=4;i<=words.length-4;i++){
    const left=words.slice(0,i).join(" "),right=words.slice(i).join(" ");
    if(!complimentLayoutBreakable(left,right,"balanced"))continue;
    const balance=Math.min(left.length,right.length)/Math.max(left.length,right.length),distance=Math.abs(.5-i/words.length),score=balance-distance*.45;
    if(!best||score>best.score)best={left,right,score};
  }
  if(best)complimentLayoutAddCandidate(list,best.left+"\n"+best.right,"balanced",1);
}
function complimentLayoutAddThreeLineCandidate(list,clean,positions){
  if(complimentLayoutWords(clean)<24||Array.from(clean).length<130)return;
  const strong=positions.filter(entry=>entry.kind!=="comma").sort((a,b)=>a.index-b.index);
  for(let i=0;i<strong.length;i++)for(let j=i+1;j<strong.length;j++){
    const first=strong[i].index,second=strong[j].index,left=clean.slice(0,first).trim(),middle=clean.slice(first,second).trim(),right=clean.slice(second).trim();
    if(!complimentLayoutBreakable(left,middle,"sentence")||!complimentLayoutBreakable(middle,right,"sentence"))continue;
    complimentLayoutAddCandidate(list,left+"\n"+middle+"\n"+right,"three-semantic",6);return;
  }
}
function complimentLayoutCandidates(text){
  const raw=String(text??""),clean=complimentCleanText(raw),manual=complimentDisplayText(raw);if(!clean)return [{displayText:"",kind:"single",quality:0,lines:1,balance:1}];
  if(/\r?\n/.test(raw)){const manualOnly=[];complimentLayoutAddCandidate(manualOnly,manual,"manual",9);return manualOnly;}
  const candidates=[];complimentLayoutAddCandidate(candidates,clean,"single",0);
  const positions=[];complimentLayoutPunctuationCandidates(candidates,clean,positions);complimentLayoutAddBalancedCandidate(candidates,clean);complimentLayoutAddThreeLineCandidate(candidates,clean,positions);
  return candidates.slice(0,COMP_LAYOUT.maxCandidates);
}
function complimentLayoutFitLines(trial){return Math.max(1,Number(trial&&trial.fit&&trial.fit.lines)||1,Number(trial&&trial.candidate&&trial.candidate.lines)||1);}
function complimentLayoutMinimumGain(base,trial){
  const baseLines=complimentLayoutFitLines(base),trialLines=complimentLayoutFitLines(trial),kind=trial.candidate.kind;
  if(kind==="manual")return .92;
  if(trialLines<baseLines)return .95;
  if(trialLines===baseLines)return trial.candidate.quality>=5?.98:1.04;
  if(baseLines===1&&trialLines===2)return kind==="question-answer"?COMP_LAYOUT.questionGain:trial.candidate.quality>=4?COMP_LAYOUT.strongGain:COMP_LAYOUT.softGain;
  if(baseLines===2&&trialLines===3)return kind==="three-semantic"?COMP_LAYOUT.threeLineGain:1.22;
  return 1.24;
}
function complimentLayoutScore(trial){
  const fit=trial&&trial.fit||{},candidate=trial&&trial.candidate||{},lines=complimentLayoutFitLines(trial);
  return (Number(fit.size)||0)*100+(Number(candidate.quality)||0)*3+(Number(candidate.balance)||0)*4-(lines-1)*.8;
}
function complimentLayoutChoose(text,assess){
  const candidates=complimentLayoutCandidates(text),trials=[];
  for(const candidate of candidates){let fit=null;try{fit=assess(candidate.displayText,candidate);}catch(_){}if(fit)trials.push({candidate,fit});}
  const base=trials[0]||{candidate:candidates[0],fit:null};if(!base.fit)return base;
  let winner=base,winnerScore=complimentLayoutScore(base);
  for(const trial of trials.slice(1)){
    if(!trial.fit.fits)continue;
    const gain=(Number(trial.fit.size)||0)/Math.max(1,Number(base.fit.size)||0);
    if(gain<complimentLayoutMinimumGain(base,trial))continue;
    const score=complimentLayoutScore(trial);if(score>winnerScore){winner=trial;winnerScore=score;}
  }
  return winner;
}
