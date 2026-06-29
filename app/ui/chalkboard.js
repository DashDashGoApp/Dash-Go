(function(){
const DAY=86400000, FADE=12*3600000, CAP=300, LITE_CAP=120, MIN_SEG=2.5, TOUCH_MIN_SEG=0.7, STORE_SIMPLIFY=0.45, LITE_STORE_SIMPLIFY=0.75, IDLE=10*60000, UNDO_CAP=6, LITE_UNDO_CAP=3, SAVE_DELAY=1500, LITE_SAVE_DELAY=5200, WARM_REOPEN=15*60000, FILL_MAX_SPANS=980, LITE_FILL_MAX_SPANS=520;
const SOFT=[
  {key:"auto",label:"Auto"},
  {key:"#8f969c",label:"Grey"},
  {key:"#e58a84",label:"Red"},
  {key:"#86bae5",label:"Blue"},
  {key:"#8fc9a9",label:"Green"},
  {key:"#ead77e",label:"Yellow"}
];
const BG={light:"var(--card)",dark:"var(--bg)"};
let S={board:"dark",strokes:[],open:false,persist:false,dirty:false,saveT:null,saving:false,bandT:null,idleT:null,undo:[],redo:[],selected:null,w:0,h:0,resizeFn:null,loaded:false,closedAt:0,lite:false,nextId:1,priorFocus:null,clearConfirm:false};
let tool="pen", color="auto", width=3, win=2*DAY, cur=null, liveCtx=null, activeTouchId=null, touchActiveTs=0, mouseActive=false, activeRect=null, liveStroke=false, lastIdleNote=0, rafId=0, liveQueue=[], nodeIndex=new Map(), selectedNode=null;
function E(t,c,txt){const e=document.createElement(t); if(c)e.className=c; if(txt!=null)e.textContent=txt; return e;}
function bind(b,fn,opts){
  if(window.bindTap){bindTap(b,fn,opts);return;}
  b.addEventListener("click",event=>{if(opts&&typeof opts.ignore==="function"&&opts.ignore(event))return;fn(event);});
}
function now(){return Date.now();}
function root(){return document.getElementById("chalkboard");}
function board(){return document.getElementById("cbboard");}
function svg(){return document.getElementById("cbsvg");}
function normBoard(b){return (b==="white"||b==="grey"||b==="light")?"light":"dark";}
function contrast(){try{return getComputedStyle(document.documentElement).getPropertyValue("--fg").trim()||"currentColor";}catch(_){return "currentColor";}}
function paint(c){return c==="auto"?contrast():c;}
function isNeutral(c){c=String(c||"").toLowerCase(); return c==="auto"||["#fff","#ffffff","#f5f5f5","#f4f4f0","#f3f1ea","#e8e8e8","#eee9dd","#eeeae0","#1a1a1a","#111111","#000000","#202326","#24282d"].includes(c);}
function normalizeStroke(st){if(st&&isNeutral(st.color))st.color="auto"; return st;}
function status(t){const s=document.getElementById("cbstatus"); if(s)s.textContent=t||"";}
function cloneStroke(st){const o={}; if(!st)return o; Object.keys(st).forEach(k=>{if(k==="pts")o.pts=(st.pts||[]).map(p=>[+p[0]||0,+p[1]||0]); else if(k==="spans")o.spans=(st.spans||[]).map(sp=>[+sp[0]||0,+sp[1]||0,+sp[2]||0,+sp[3]||0]); else if(k!=="bb")o[k]=st[k];}); return o;}
function cloneStrokesOf(arr){return (Array.isArray(arr)?arr:[]).map(cloneStroke);}
function cloneStrokes(){return cloneStrokesOf(S.strokes);}
function cloneState(){return {board:S.board,strokes:cloneStrokes()};}
function syncNextId(){let m=0; (S.strokes||[]).forEach(st=>{const id=+st.id||0; if(id>m)m=id;}); S.nextId=Math.max(+S.nextId||1,m+1);}
function resetScratch(){S.board="dark";S.strokes=[];S.undo=[];S.redo=[];S.dirty=false;S.loaded=false;S.nextId=1; nodeIndex.clear(); selectedNode=null; if(S.saveT){clearTimeout(S.saveT);S.saveT=null;} updateUndoButtons(); clearSelection(false); clearLive();}
function sameState(a,b){if(!a||!b||a.board!==b.board)return false; const A=a.strokes||[],B=b.strokes||[]; if(A.length!==B.length)return false; for(let i=0;i<A.length;i++){const x=A[i]||{},y=B[i]||{}; if(x.id!==y.id||x.tool!==y.tool||x.color!==y.color||x.persistent!==y.persistent||(+x.expiresAt||0)!==(+y.expiresAt||0)||(+x.opacity||0)!==(+y.opacity||0))return false; if((x.pts||[]).length!==(y.pts||[]).length||(x.spans||[]).length!==(y.spans||[]).length)return false;} return true;}
function updateUndoButtons(){const u=document.getElementById("cbundo"), r=document.getElementById("cbredo"); if(u)u.disabled=!S.undo.length; if(r)r.disabled=!S.redo.length;}
function undoCap(){return S.lite?LITE_UNDO_CAP:UNDO_CAP;}
function pushSnapshot(before,after,msg,knownChanged){if(!knownChanged&&sameState(before,after))return false; S.undo.push({before,after}); while(S.undo.length>undoCap())S.undo.shift(); S.redo=[]; updateUndoButtons(); scheduleSave(); if(msg)status(msg); return true;}
function restoreState(st){S.board=normBoard(st.board); S.strokes=cloneStrokesOf(st.strokes).map(normalizeStroke); syncNextId(); clearSelection(false); applyBoard(); redraw();}
function undo(){const a=S.undo.pop(); if(!a){status("Nothing to undo"); return;} S.redo.push(a); while(S.redo.length>undoCap())S.redo.shift(); restoreState(a.before); updateUndoButtons(); scheduleSave(); status("Undone"); note();}
function redo(){const a=S.redo.pop(); if(!a){status("Nothing to redo"); return;} S.undo.push(a); while(S.undo.length>undoCap())S.undo.shift(); restoreState(a.after); updateUndoButtons(); scheduleSave(); status("Redone"); note();}
function build(){ if(root()) return; const r=E("div",""); r.id="chalkboard"; r.setAttribute("aria-hidden","true"); r.setAttribute("role","dialog"); r.setAttribute("aria-modal","true"); r.setAttribute("aria-labelledby","chalkboard-title");
  const p=E("section","cb-panel"); p.setAttribute("role","document"); const h=E("div","cb-head");
  const title=E("h2","cb-title"); title.id="chalkboard-title"; title.innerHTML='<span>Chalkboard</span><span id="cbboardstate" class="cb-board-state"><i></i><b>Dark board</b></span>'; h.appendChild(title);
  const tools=E("div","cb-tools"); [["pen","Pen"],["marker","Marker"],["highlighter","Highlighter"],["fill","Fill"],["edit","Edit"],["eraser","Eraser"]].forEach(x=>{const b=E("button","cb-btn",x[1]); b.type="button"; b.dataset.tool=x[0]; bind(b,()=>setTool(x[0])); tools.appendChild(b);}); h.appendChild(tools);
  const cols=E("div","cb-colors"); SOFT.forEach(c=>{const b=E("button","cb-color"); b.type="button"; b.setAttribute("aria-label",c.label); b.dataset.color=c.key; bind(b,()=>setColor(c.key)); cols.appendChild(b);}); h.appendChild(cols);
  const undoBtn=E("button","cb-btn cb-small","↶ Undo"); undoBtn.id="cbundo"; bind(undoBtn,undo); h.appendChild(undoBtn);
  const redoBtn=E("button","cb-btn cb-small","↷ Redo"); redoBtn.id="cbredo"; bind(redoBtn,redo); h.appendChild(redoBtn);
  const close=E("button","cb-btn cb-close","×"); close.id="chalkboard-close"; close.type="button"; close.setAttribute("aria-label","Close chalkboard"); bind(close,closeBoard); h.appendChild(close); p.appendChild(h);
  const b=E("div","cb-board dark"); b.id="cbboard"; b.innerHTML='<svg id="cbsvg" preserveAspectRatio="none"><g id="cb-persistent"></g><g id="cb-band-0" class="cb-band-0"></g><g id="cb-band-1" class="cb-band-1"></g><g id="cb-band-2" class="cb-band-2"></g></svg><canvas id="cblive"></canvas><div id="cberaser" class="cb-eraser-dot"></div>'; p.appendChild(b);
  const f=E("div","cb-foot"); f.appendChild(E("span","cb-label","Board")); const bp=E("div","cb-board-pick"); [["dark","Dark"],["light","Light"]].forEach(k=>{const x=E("button","cb-board-choice",k[1]); x.type="button"; x.dataset.board=k[0]; bind(x,()=>{const before=cloneState(); S.board=normBoard(k[0]); applyBoard(); redraw(); pushSnapshot(before,cloneState(),k[1]+" board",true); note();}); bp.appendChild(x);}); f.appendChild(bp);
  const clear=E("button","cb-btn cb-small","Clear temp"); bind(clear,()=>{const before=cloneState(); clearSelection(false); const removed=clearTemporaryStrokes(); if(removed)pushSnapshot(before,cloneState(),"Temporary strokes cleared",true); else status("No temporary strokes"); note();}); f.appendChild(clear);
  const clearAll=E("button","cb-btn cb-small","Clear all"); clearAll.id="cbclearall"; bind(clearAll,showClearAllConfirm); f.appendChild(clearAll);
  const actions=E("div","cb-actions"); actions.id="cbactions"; f.appendChild(actions);
  const st=E("div","cb-status",""); st.id="cbstatus"; f.appendChild(st); p.appendChild(f); r.appendChild(p); document.body.appendChild(r);
  b.addEventListener("touchstart",touchStart,{passive:false}); b.addEventListener("touchmove",touchMove,{passive:false}); b.addEventListener("touchend",touchEnd,{passive:false}); b.addEventListener("touchcancel",touchCancel,{passive:false});
  b.addEventListener("mousedown",mouseDown); bind(r,closeBoard,{ignore:event=>event.target!==r}); document.addEventListener("keydown",event=>{if(event.key!=="Escape"||!S.open)return;event.preventDefault();if(S.clearConfirm)resolveClearAllConfirm(false);else closeBoard();}); setTool("pen"); setColor(color); updateUndoButtons(); applyBoard(); }
function resize(scaleExisting){const b=board(), c=document.getElementById("cblive"), s=svg(); if(!b||!c||!s)return; const r=b.getBoundingClientRect(), nw=Math.max(1,Math.round(r.width)), nh=Math.max(1,Math.round(r.height)); const ow=S.w||nw, oh=S.h||nh; c.width=nw; c.height=nh; s.setAttribute("viewBox","0 0 "+nw+" "+nh); liveCtx=c.getContext("2d",{alpha:true}); if(scaleExisting && S.w && S.h && (Math.abs(nw-S.w)>2||Math.abs(nh-S.h)>2)){const sx=nw/ow, sy=nh/oh; S.strokes.forEach(st=>{(st.pts||[]).forEach(p=>{p[0]=+(p[0]*sx).toFixed(1); p[1]=+(p[1]*sy).toFixed(1);}); (st.spans||[]).forEach(sp=>{sp[0]=+(sp[0]*sy).toFixed(1); sp[1]=+(sp[1]*sx).toFixed(1); sp[2]=+(sp[2]*sx).toFixed(1); sp[3]=+(sp[3]*sy).toFixed(1);}); invalidateBounds(st);}); rebuildBounds(); redraw(); scheduleSave();} S.w=nw; S.h=nh; clearBoardRect();}
function touchFromList(list,id){if(!list)return null; for(let i=0;i<list.length;i++){const t=list[i]; if(id==null || t.identifier===id)return t;} return null;}
function touchPoint(ev){return touchFromList(ev&&ev.changedTouches,activeTouchId) || touchFromList(ev&&ev.touches,activeTouchId);}
function srcPoint(ev){return touchPoint(ev)||ev;}
function prevent(ev){if(ev&&ev.preventDefault)ev.preventDefault();}
function cacheBoardRect(){const b=board(); activeRect=b?b.getBoundingClientRect():null; return activeRect;}
function clearBoardRect(){activeRect=null;}
function pt(ev){const e=srcPoint(ev), r=activeRect||cacheBoardRect()||board().getBoundingClientRect(); return [Math.max(0,Math.min(S.w||r.width,e.clientX-r.left)),Math.max(0,Math.min(S.h||r.height,e.clientY-r.top))];}
function touchThreshold(){return activeTouchId!=null?TOUCH_MIN_SEG:MIN_SEG;}
function detectLite(){try{return typeof startupLiteProfile==="function"&&startupLiteProfile();}catch(e){return false;}}
function refreshRuntimeFlags(){S.lite=!!detectLite(); const r=root(); if(r)r.classList.toggle("lite",S.lite);}
function isLite(){return !!S.lite;}
function strokeCap(){return S.lite?LITE_CAP:CAP;}
function storeSimplifyTol(){return S.lite?LITE_STORE_SIMPLIFY:STORE_SIMPLIFY;}
function fillMaxSpans(){return S.lite?LITE_FILL_MAX_SPANS:FILL_MAX_SPANS;}
function fillScale(){const area=(S.w||1)*(S.h||1); if(S.lite)return area>1400000?.28:.40; return area>1800000?.46:.56;}
function fillOpacity(st){return st.opacity!=null?Math.max(.12,Math.min(.72,+st.opacity||.38)):(st.color==="auto"?.24:.42);}
function boundsOf(st){if(!st)return null; if(st.bb&&st.bb.length===4)return st.bb; let minx=Infinity,miny=Infinity,maxx=-Infinity,maxy=-Infinity; if(st.tool==="fill"){(st.spans||[]).forEach(sp=>{const y=+sp[0]||0,x1=+sp[1]||0,x2=+sp[2]||0,h=Math.max(.5,+sp[3]||1); if(x1<minx)minx=x1; if(x2>maxx)maxx=x2; if(y<miny)miny=y; if(y+h>maxy)maxy=y+h;});} else {(st.pts||[]).forEach(p=>{const x=+p[0]||0,y=+p[1]||0; if(x<minx)minx=x; if(x>maxx)maxx=x; if(y<miny)miny=y; if(y>maxy)maxy=y;}); const pad=Math.max(8,strokeWidth(st)*.7); minx-=pad; miny-=pad; maxx+=pad; maxy+=pad;} st.bb=(minx===Infinity)?[0,0,0,0]:[minx,miny,maxx,maxy]; return st.bb;}
function invalidateBounds(st){if(st)delete st.bb;}
function rebuildBounds(){(S.strokes||[]).forEach(st=>{invalidateBounds(st); boundsOf(st);});}
function bboxHit(p,st,r){const b=boundsOf(st), pad=r||0; return !!b && p[0]>=b[0]-pad && p[0]<=b[2]+pad && p[1]>=b[1]-pad && p[1]<=b[3]+pad;}
function idleNote(force){const t=now(); if(force||!lastIdleNote||(t-lastIdleNote)>4000){lastIdleNote=t; note();}}
function prepLive(st){if(!liveCtx)return; liveCtx.globalAlpha=st.tool==="highlighter"?.32:1; liveCtx.strokeStyle=strokeColor(st); liveCtx.fillStyle=strokeColor(st); liveCtx.lineWidth=strokeWidth(st); liveCtx.lineCap="round"; liveCtx.lineJoin="round"; liveStroke=true;}
function drawDot(p,st){if(!liveCtx)return; prepLive(st); const r=Math.max(1.2,strokeWidth(st)/2); liveCtx.beginPath(); liveCtx.arc(p[0],p[1],r,0,Math.PI*2); liveCtx.fill();}
function simplifyPts(pts,tol){if(!Array.isArray(pts)||pts.length<5||tol<=0)return pts; const t2=tol*tol; const dist2=(p,a,b)=>{const vx=b[0]-a[0],vy=b[1]-a[1],wx=p[0]-a[0],wy=p[1]-a[1],c=vx*vx+vy*vy; let u=c?((wx*vx+wy*vy)/c):0; u=Math.max(0,Math.min(1,u)); const x=a[0]+u*vx,y=a[1]+u*vy,dx=p[0]-x,dy=p[1]-y; return dx*dx+dy*dy;}; const keep=new Uint8Array(pts.length); keep[0]=keep[pts.length-1]=1; const stack=[[0,pts.length-1]]; while(stack.length){const seg=stack.pop(),a=seg[0],b=seg[1]; let best=-1,bd=0; for(let i=a+1;i<b;i++){const d=dist2(pts[i],pts[a],pts[b]); if(d>bd){bd=d; best=i;}} if(best>0&&bd>t2){keep[best]=1; stack.push([a,best],[best,b]);}} return pts.filter((_,i)=>keep[i]);}
function recentTouch(){return touchActiveTs && (now()-touchActiveTs)<700;}
function touchStart(ev){if(cur)return; const t=touchFromList(ev&&ev.changedTouches,null); if(!t)return; activeTouchId=t.identifier; touchActiveTs=now(); prevent(ev); down(ev);}
function touchMove(ev){if(!cur||activeTouchId==null)return; const t=touchPoint(ev); if(!t)return; touchActiveTs=now(); prevent(ev); move(ev);}
function touchEnd(ev){if(activeTouchId==null&&!cur)return; const ended=touchFromList(ev&&ev.changedTouches,activeTouchId); if(!ended && ev&&ev.changedTouches&&ev.changedTouches.length)return; try{touchActiveTs=now(); prevent(ev); if(cur)up();}finally{activeTouchId=null; hideEraserDot();}}
function touchCancel(ev){const cancelled=touchFromList(ev&&ev.changedTouches,activeTouchId); if(!cancelled && ev&&ev.changedTouches&&ev.changedTouches.length)return; try{if(cur){prevent(ev); up();}}finally{activeTouchId=null; touchActiveTs=now(); hideEraserDot();}}
function mouseDown(ev){if(recentTouch()||cur)return; mouseActive=true; down(ev); window.addEventListener("mousemove",mouseMove); window.addEventListener("mouseup",mouseUp);}
function mouseMove(ev){if(!mouseActive||recentTouch())return; move(ev);}
function mouseUp(ev){if(!mouseActive)return; mouseActive=false; try{if(!recentTouch())up();}finally{window.removeEventListener("mousemove",mouseMove); window.removeEventListener("mouseup",mouseUp); hideEraserDot();}}
function setTool(t){tool=t; document.querySelectorAll('#chalkboard [data-tool]').forEach(b=>b.classList.toggle('on',b.dataset.tool===t)); clearSelection(false); status(t==="edit"?"Tap a stroke to edit":(t==="eraser"?"Drag over strokes to erase":(t==="fill"?"Tap an enclosed area to fill":""))); note();}
function updateColorButtons(){document.querySelectorAll('#chalkboard [data-color]').forEach(b=>{b.style.background=paint(b.dataset.color); b.classList.toggle('on',b.dataset.color===color);});}
function setColor(c){color=c||"auto"; updateColorButtons(); note();}
function updateBoardUI(){const st=document.getElementById("cbboardstate"); if(st){st.classList.toggle("temp",!S.persist); const i=st.querySelector("i"), b=st.querySelector("b"); if(i)i.style.background=BG[S.board]; if(b)b.textContent=(S.board==="light"?"Light":"Dark")+" board"+(S.persist?"":" · temporary");} document.querySelectorAll('#chalkboard [data-board]').forEach(x=>x.classList.toggle('on',x.dataset.board===S.board)); updateColorButtons();}
function applyBoard(){S.board=normBoard(S.board); const b=board(); if(!b)return; b.className="cb-board "+S.board; updateBoardUI();}
function bandId(st){if(st.persistent)return "cb-persistent"; const left=(st.expiresAt||0)-now(); if(left>FADE)return "cb-band-0"; if(left>FADE/2)return "cb-band-1"; return "cb-band-2";}
function groupFor(st){return document.getElementById(bandId(st));}
function strokeWidth(st){return st.tool==="highlighter"?st.w*7:(st.tool==="marker"?st.w*4:st.w);}
function strokeColor(st){return paint(st.color||"auto");}
function spanPath(spans){let d=""; (spans||[]).forEach(sp=>{const y=+sp[0]||0,x1=+sp[1]||0,x2=+sp[2]||0,h=Math.max(.5,+sp[3]||1),bleed=Math.max(.55,Math.min(1.6,h*.38)); const yy=y-bleed,xx1=x1-bleed,xx2=x2+bleed,hh=h+bleed*2; d+="M"+xx1.toFixed(1)+" "+yy.toFixed(1)+"H"+xx2.toFixed(1)+"V"+(yy+hh).toFixed(1)+"H"+xx1.toFixed(1)+"Z";}); return d;}
function makeFillNode(st){normalizeStroke(st); const p=document.createElementNS("http://www.w3.org/2000/svg","path"); p.setAttribute("d",spanPath(st.spans)); p.setAttribute("fill",strokeColor(st)); p.setAttribute("stroke",strokeColor(st)); p.setAttribute("stroke-width",S.lite?"0.6":"0.8"); p.setAttribute("stroke-linejoin","round"); p.setAttribute("stroke-linecap","round"); p.setAttribute("opacity",fillOpacity(st)); p.classList.add("cb-fill"); p.dataset.id=st.id; return p;}
function commitFill(st){const p=makeFillNode(st), g=groupFor(st); if(g&&p)g.insertBefore(p,g.firstChild); return p;}
function drawStrokeOnMask(ctx,st,scale){if(!st||st.tool==="fill")return; const pts=st.pts||[]; if(!pts.length)return; ctx.globalAlpha=1; ctx.strokeStyle="#000"; ctx.lineWidth=Math.max(2.5,(strokeWidth(st)+1.5)*scale); ctx.lineCap="round"; ctx.lineJoin="round"; ctx.beginPath(); ctx.moveTo(pts[0][0]*scale,pts[0][1]*scale); for(let i=1;i<pts.length;i++)ctx.lineTo(pts[i][0]*scale,pts[i][1]*scale); if(pts.length===1){ctx.lineTo((pts[0][0]+.2)*scale,(pts[0][1]+.2)*scale);} ctx.stroke();}
function reduceFillSpans(spans,max){if(spans.length<=max)return spans; const step=Math.ceil(spans.length/max), out=[]; for(let i=0;i<spans.length;i+=step){let a=spans[i].slice(), last=a; for(let j=1;j<step&&i+j<spans.length;j++){last=spans[i+j];} a[3]=((last[0]||a[0])+(last[3]||a[3]||1))-a[0]; out.push(a);} return out;}
function floodFillSpans(seed){const scale=fillScale(), cw=Math.max(1,Math.round((S.w||1)*scale)), ch=Math.max(1,Math.round((S.h||1)*scale)); const maxCells=S.lite?300000:480000; if(cw*ch>maxCells){status("Fill skipped — area too large"); return null;} const can=document.createElement("canvas"); can.width=cw; can.height=ch; const ctx=can.getContext("2d",{willReadFrequently:true}); ctx.fillStyle="#fff"; ctx.fillRect(0,0,cw,ch); S.strokes.forEach(st=>drawStrokeOnMask(ctx,st,scale)); const sx=Math.max(0,Math.min(cw-1,Math.floor(seed[0]*scale))), sy=Math.max(0,Math.min(ch-1,Math.floor(seed[1]*scale))); const img=ctx.getImageData(0,0,cw,ch), data=img.data, total=cw*ch, seen=new Uint8Array(total); function blocked(i){const o=i*4; return data[o]<180||data[o+1]<180||data[o+2]<180;} const start=sy*cw+sx; if(blocked(start)){status("Tap inside an enclosed area, not on a line"); return null;} const queue=new Int32Array(total); let head=0,tail=0; queue[tail++]=start; seen[start]=1; let count=0, minx=sx,maxx=sx,miny=sy,maxy=sy; while(head<tail){const i=queue[head++], x=i%cw, y=(i/cw)|0; count++; if(x<minx)minx=x; if(x>maxx)maxx=x; if(y<miny)miny=y; if(y>maxy)maxy=y; const n1=i-1,n2=i+1,n3=i-cw,n4=i+cw; if(x>0&&!seen[n1]&&!blocked(n1)){seen[n1]=1; queue[tail++]=n1;} if(x<cw-1&&!seen[n2]&&!blocked(n2)){seen[n2]=1; queue[tail++]=n2;} if(y>0&&!seen[n3]&&!blocked(n3)){seen[n3]=1; queue[tail++]=n3;} if(y<ch-1&&!seen[n4]&&!blocked(n4)){seen[n4]=1; queue[tail++]=n4;} }
  if(count<3){status("Fill area too small"); return null;} if(count>total*.88){status("Fill needs a more enclosed area"); return null;} const spans=[]; const inv=1/scale; for(let y=miny;y<=maxy;y++){let x=minx; while(x<=maxx){while(x<=maxx&&!seen[y*cw+x])x++; if(x>maxx)break; const x1=x; while(x<=maxx&&seen[y*cw+x])x++; const x2=x; if(x2>x1)spans.push([+(y*inv).toFixed(1),+(x1*inv).toFixed(1),+(x2*inv).toFixed(1),+inv.toFixed(1)]);}} return reduceFillSpans(spans,fillMaxSpans());}
function fillAt(p){if(!canAdd()){status("Board full — clear or pin fewer strokes"); return;} const spans=floodFillSpans(p); if(!spans||!spans.length)return; const before=cloneState(); const st={id:nextId(),tool:"fill",color,w:width,pts:[[p[0],p[1]],[p[0]+.1,p[1]+.1]],spans,persistent:false,createdAt:now(),expiresAt:now()+win,opacity:color==="auto"?.24:.42}; normalizeStroke(st); boundsOf(st); S.strokes.push(st); if(!enforceCap()){restoreState(before); status("Board full — clear or pin fewer strokes"); return;} commit(st); pushSnapshot(before,cloneState(),"Area filled",true); note();}
function makeStrokeNode(st){normalizeStroke(st); const pl=document.createElementNS("http://www.w3.org/2000/svg","polyline"); const pts=(st.pts&&st.pts.length===1)?[st.pts[0],[st.pts[0][0]+.1,st.pts[0][1]+.1]]:st.pts; pl.setAttribute("points",pts.map(p=>p[0].toFixed(1)+","+p[1].toFixed(1)).join(" ")); pl.setAttribute("fill","none"); pl.setAttribute("stroke",strokeColor(st)); pl.setAttribute("stroke-width",strokeWidth(st)); pl.setAttribute("stroke-linecap","round"); pl.setAttribute("stroke-linejoin","round"); if(st.tool==="highlighter")pl.setAttribute("opacity","0.32"); pl.dataset.id=st.id; return pl;}
function rememberNode(st,n){if(st&&st.id!=null&&n)nodeIndex.set(String(st.id),n); return n;}
function commit(st){boundsOf(st); const n=st&&st.tool==="fill"?makeFillNode(st):makeStrokeNode(st), g=groupFor(st); if(g&&n){if(st&&st.tool==="fill")g.insertBefore(n,g.firstChild); else g.appendChild(n); rememberNode(st,n);} return n;}
function nodeFor(st){if(!st||st.id==null)return null; const key=String(st.id), n=nodeIndex.get(key); if(n&&n.isConnected)return n; const found=document.querySelector('#cbsvg [data-id="'+st.id+'"]'); if(found)nodeIndex.set(key,found); return found;}
function removeNode(st){const n=nodeFor(st); if(st&&st.id!=null)nodeIndex.delete(String(st.id)); if(n)n.remove(); if(selectedNode===n)selectedNode=null;}
function clearSvgNodes(){nodeIndex.clear(); selectedNode=null; ["cb-persistent","cb-band-0","cb-band-1","cb-band-2"].forEach(id=>{const g=document.getElementById(id); if(g)g.textContent="";});}
function clearTemporaryStrokes(){let removed=0; S.strokes=(S.strokes||[]).filter(st=>{if(st&&st.persistent)return true; removeNode(st); removed++; return false;}); return removed;}
function updateNode(st){invalidateBounds(st); boundsOf(st); let n=nodeFor(st); const g=groupFor(st); if(!g)return null; if(!n)return commit(st); if(st.tool==="fill"){n.setAttribute("d",spanPath(st.spans)); n.setAttribute("fill",strokeColor(st)); n.setAttribute("stroke",strokeColor(st)); n.setAttribute("stroke-width",S.lite?"0.6":"0.8"); n.setAttribute("stroke-linejoin","round"); n.setAttribute("stroke-linecap","round"); n.setAttribute("opacity",fillOpacity(st)); if(n.parentNode!==g)g.insertBefore(n,g.firstChild);} else {const fresh=makeStrokeNode(st); Array.from(fresh.attributes).forEach(a=>n.setAttribute(a.name,a.value)); if(st.tool!=="highlighter")n.removeAttribute("opacity"); if(n.parentNode!==g)g.appendChild(n);} rememberNode(st,n); return n;}
function redraw(){nodeIndex.clear(); selectedNode=null; rebuildBounds(); const ids=["cb-persistent","cb-band-0","cb-band-1","cb-band-2"], fills={}, lines={}; ids.forEach(id=>{const g=document.getElementById(id); if(g){g.textContent=""; fills[id]=document.createDocumentFragment(); lines[id]=document.createDocumentFragment();}}); S.strokes.forEach(st=>{const id=bandId(st), n=st.tool==="fill"?makeFillNode(st):makeStrokeNode(st); if(n&&fills[id]){rememberNode(st,n); (st.tool==="fill"?fills[id]:lines[id]).appendChild(n);}}); ids.forEach(id=>{const g=document.getElementById(id); if(g){if(fills[id])g.appendChild(fills[id]); if(lines[id])g.appendChild(lines[id]);}}); updateSelected(); status("");}
function flushLiveQueue(){if(rafId){cancelAnimationFrame(rafId); rafId=0;} if(!liveQueue.length||!liveCtx)return; const q=liveQueue.splice(0); for(const x of q)drawSegNow(x[0],x[1],x[2]);}
function clearLive(){if(rafId){cancelAnimationFrame(rafId); rafId=0;} liveQueue=[]; const c=document.getElementById("cblive"); if(liveCtx&&c){liveCtx.globalAlpha=1; liveCtx.clearRect(0,0,c.width,c.height);} liveStroke=false;}
function drawSegNow(a,b,st){if(!liveCtx)return; if(!liveStroke)prepLive(st); liveCtx.beginPath(); liveCtx.moveTo(a[0],a[1]); liveCtx.lineTo(b[0],b[1]); liveCtx.stroke();}
function scheduleLiveFlush(){if(rafId)return; rafId=requestAnimationFrame(()=>{rafId=0; flushLiveQueue();});}
function drawSeg(a,b,st){if(!liveCtx)return; liveQueue.push([a,b,st]); scheduleLiveFlush();}
function canAdd(){const cap=strokeCap(); return S.strokes.length<cap || S.strokes.some(st=>!st.persistent);}
function enforceCap(){const cap=strokeCap(); while(S.strokes.length>cap){const idx=S.strokes.findIndex(st=>!st.persistent); if(idx<0)break; const old=S.strokes.splice(idx,1)[0]; removeNode(old);} return S.strokes.length<=cap;}
function setDrawing(on){const r=root(); if(r)r.classList.toggle("drawing",!!on);}
function down(ev){prevent(ev); idleNote(true); cacheBoardRect(); const p=pt(ev); if(tool==="edit"){selectAt(p); clearBoardRect(); return;} if(tool==="fill"){fillAt(p); clearBoardRect(); return;} setDrawing(true); if(tool==="eraser"){cur={erase:true,before:null,changed:false}; showEraserDot(p); const st=hit(p,26); if(st){cur.before=cloneState(); if(eraseAt(p))cur.changed=true;} return;} cur={id:nextId(),tool,color,w:width,pts:[p],persistent:false,createdAt:now(),expiresAt:now()+win}; drawDot(p,cur);}
function move(ev){if(!cur)return; prevent(ev); idleNote(false); const p=pt(ev); if(cur.erase){showEraserDot(p); const st=hit(p,26); if(st&&!cur.before)cur.before=cloneState(); if(st&&eraseAt(p))cur.changed=true; return;} const last=cur.pts[cur.pts.length-1]; if(Math.hypot(p[0]-last[0],p[1]-last[1])<touchThreshold())return; cur.pts.push(p); drawSeg(last,p,cur);}
function up(){if(!cur)return; idleNote(true); try{if(cur.erase){if(cur.changed&&cur.before)pushSnapshot(cur.before,cloneState(),"Erased",true); cur=null; return;} if(cur.pts.length>=1){if(!canAdd()){status("Board full — clear or pin fewer strokes"); cur=null; clearLive(); return;} const before=cloneState(); cur.pts=simplifyPts(cur.pts,storeSimplifyTol()); normalizeStroke(cur); boundsOf(cur); S.strokes.push(cur); if(!enforceCap()){restoreState(before); status("Board full — clear or pin fewer strokes");} else {commit(cur); pushSnapshot(before,cloneState(),"Stroke added",true);}} cur=null; flushLiveQueue(); clearLive();}finally{setDrawing(false); clearBoardRect();}}
function nextId(){const id=Math.max(1,+S.nextId||1); S.nextId=id+1; return id;}
function distSeg(p,a,b){const vx=b[0]-a[0], vy=b[1]-a[1], wx=p[0]-a[0], wy=p[1]-a[1], c=vx*vx+vy*vy; let t=c?((wx*vx+wy*vy)/c):0; t=Math.max(0,Math.min(1,t)); const x=a[0]+t*vx, y=a[1]+t*vy; return Math.hypot(p[0]-x,p[1]-y);}
function hitFill(p,st){if(!bboxHit(p,st,1))return false; const y=p[1],x=p[0]; return (st.spans||[]).some(sp=>y>=(+sp[0]||0)&&y<=((+sp[0]||0)+Math.max(.5,+sp[3]||1))&&x>=(+sp[1]||0)&&x<=(+sp[2]||0));}
function hit(p,r){let best=null,bd=r||18; for(const st of S.strokes){if(!bboxHit(p,st,bd))continue; if(st.tool==="fill"){if(hitFill(p,st))best=st; continue;} const pts=st.pts||[]; for(let i=0;i<pts.length;i++){let d=i?distSeg(p,pts[i-1],pts[i]):Math.hypot(p[0]-pts[i][0],p[1]-pts[i][1]); if(d<bd){bd=d;best=st;}}} return best;}
function eraseAt(p){const st=hit(p,26); if(!st)return false; if(S.selected===st.id)clearSelection(false); S.strokes=S.strokes.filter(x=>x!==st); const n=nodeFor(st); if(n)n.remove(); return true;}
function selectAt(p){const st=hit(p,30); if(!st){clearSelection(true); status("No stroke selected"); return;} S.selected=st.id; updateSelected(); showActions(st); status(st.persistent?"Persistent stroke":"Temporary stroke");}
function selectedStroke(){return S.strokes.find(st=>st.id===S.selected);}
function clearSelection(msg){S.selected=null; updateSelected(); hideActions(); if(msg)status("");}
function updateSelected(){if(selectedNode){selectedNode.classList.remove("cb-selected"); selectedNode=null;} const st=selectedStroke(); if(st){const n=nodeFor(st); if(n){n.classList.add("cb-selected"); selectedNode=n;}}}
function hideActions(){const a=document.getElementById("cbactions"); if(a){a.classList.remove("show"); a.innerHTML="";}}
function resolveClearAllConfirm(confirmed){
  if(!S.clearConfirm)return;
  S.clearConfirm=false;hideActions();
  if(!confirmed){status("Clear all canceled");return;}
  const before=cloneState();S.strokes=[];clearSelection(false);clearSvgNodes();pushSnapshot(before,cloneState(),"Board cleared",true);note();
}
function showClearAllConfirm(){
  if(S.clearConfirm)return;
  clearSelection(false);S.clearConfirm=true;
  const a=document.getElementById("cbactions");if(!a)return;
  a.innerHTML="";a.classList.add("show");
  const label=E("span","cb-action-label","Clear the entire chalkboard?");
  const keep=E("button","cb-action","Keep board");keep.type="button";bind(keep,()=>resolveClearAllConfirm(false));
  const clear=E("button","cb-action cb-danger","Clear all");clear.type="button";bind(clear,()=>resolveClearAllConfirm(true));
  a.append(label,keep,clear);requestAnimationFrame(()=>keep.focus?.());
}
function showActions(st){const a=document.getElementById("cbactions"); if(!a)return; a.innerHTML=""; a.classList.add("show"); const lab=E("span","cb-action-label",st.persistent?"Persistent":"Temporary"); a.appendChild(lab);
  function btn(txt,fn){const b=E("button","cb-action",txt); b.type="button"; bind(b,()=>{fn(); note();}); a.appendChild(b);}
  if(st.persistent) btn("Unpin",()=>mutateSelected(x=>{x.persistent=false; x.expiresAt=now()+2*DAY;},"Unpinned"));
  else {btn("Pin",()=>mutateSelected(x=>{x.persistent=true; x.expiresAt=null;},"Pinned")); [1,2,3].forEach(d=>btn("+"+d+"d",()=>mutateSelected(x=>{x.persistent=false; x.expiresAt=now()+d*DAY;},"Extended +"+d+"d")));}
  btn("Delete",()=>{const before=cloneState(); const id=S.selected, st=selectedStroke(); if(st)removeNode(st); S.strokes=S.strokes.filter(x=>x.id!==id); clearSelection(false); pushSnapshot(before,cloneState(),"Deleted",true);});
  btn("Done",()=>clearSelection(true));}
function mutateSelected(fn,msg){const st=selectedStroke(); if(!st)return; const before=cloneState(); fn(st); normalizeStroke(st); updateNode(st); S.selected=st.id; updateSelected(); showActions(st); pushSnapshot(before,cloneState(),msg,true);}
function showEraserDot(p){const d=document.getElementById("cberaser"); if(!d)return; d.style.display="block"; d.style.left=p[0]+"px"; d.style.top=p[1]+"px";}
function hideEraserDot(){const d=document.getElementById("cberaser"); if(d)d.style.display="none";}
function fadePass(save){const t=now(); let changed=false; S.strokes=(S.strokes||[]).filter(st=>{if(st.persistent||st.expiresAt>t)return true; removeNode(st); changed=true; return false;}); S.strokes.forEach(st=>{const n=nodeFor(st), g=groupFor(st); if(!n){commit(st); changed=true;} else if(g&&n.parentNode!==g){if(st.tool==="fill")g.insertBefore(n,g.firstChild); else g.appendChild(n); changed=true;}}); if(changed && save!==false)scheduleSave();}
function scheduleSave(){S.dirty=true; if(!S.persist)return; if(S.saveT||S.saving)return; S.saveT=setTimeout(flush,S.lite?LITE_SAVE_DELAY:SAVE_DELAY);}
async function flush(){if(S.saveT){clearTimeout(S.saveT); S.saveT=null;} if(!S.persist){S.dirty=false; return;} if(S.saving){S.dirty=true; return;} if(!S.dirty)return; S.dirty=false; S.saving=true; const payload={board:S.board,strokes:cloneStrokes()}; try{await api("/api/chalkboard","POST",payload); status("Saved");}catch(e){S.dirty=true; const m=String(e.message||e); status(m.toLowerCase().includes("locked")?"Unlock Dashboard Control to save":"Save failed: "+m);} finally{S.saving=false; if(S.dirty&&S.persist)scheduleSave();}}
function note(){if(S.idleT)clearTimeout(S.idleT); if(S.open)S.idleT=setTimeout(closeBoard,IDLE);}
async function load(){try{const d=await api("/api/chalkboard","GET"); S.persist=true; S.loaded=true; S.board=normBoard(d.board); S.strokes=(Array.isArray(d.strokes)?d.strokes:[]).map(normalizeStroke); rebuildBounds(); syncNextId(); S.undo=[]; S.redo=[]; updateUndoButtons(); status("");}catch(e){S.persist=false; resetScratch(); status("");}}
async function openBoard(){build(); refreshRuntimeFlags(); S.priorFocus=document.activeElement; S.clearConfirm=false; S.open=true; if(typeof pauseUiAnimations==="function") pauseUiAnimations(); const r=root(); r.classList.add("show"); r.setAttribute("aria-hidden","false"); if(window.DashGoAppDialog)window.DashGoAppDialog.focusInitial(r,"#chalkboard-close");else requestAnimationFrame(()=>document.getElementById("chalkboard-close")?.focus()); resize(false); const warm=S.loaded&&S.persist&&S.closedAt&&(now()-S.closedAt<WARM_REOPEN); if(!warm){await load(); redraw();} applyBoard(); fadePass(false); note(); if(S.bandT)clearInterval(S.bandT); S.bandT=setInterval(fadePass,5*60000); S.resizeFn=()=>resize(true); window.addEventListener("resize",S.resizeFn); window.addEventListener("orientationchange",S.resizeFn);}
function closeBoard(){if(typeof completeAppLauncherHandoff==="function")completeAppLauncherHandoff();const r=root(); if(!r)return; r.classList.remove("show","drawing"); r.setAttribute("aria-hidden","true"); const wasPersist=S.persist; S.open=false;S.clearConfirm=false; S.closedAt=now(); clearSelection(false); hideEraserDot(); clearLive(); clearBoardRect(); if(S.bandT)clearInterval(S.bandT); S.bandT=null; if(S.idleT)clearTimeout(S.idleT); S.idleT=null; if(S.resizeFn){window.removeEventListener("resize",S.resizeFn); window.removeEventListener("orientationchange",S.resizeFn); S.resizeFn=null;} if(wasPersist) flush(); else resetScratch(); if(typeof resumeUiAfterOverlay==="function"&&!(typeof overlayIsOpen==="function"&&overlayIsOpen())) resumeUiAfterOverlay(); const trigger=document.getElementById("cblaunch");if(window.DashGoAppDialog)window.DashGoAppDialog.restoreFocus(S.priorFocus,trigger);else (trigger&&!trigger.hidden?trigger:S.priorFocus)?.focus?.();}
window.openChalkboardImpl=openBoard; window.closeChalkboard=closeBoard; window.chalkboardIsOpen=function(){return !!(root()&&root().classList.contains("show"));};
}());
