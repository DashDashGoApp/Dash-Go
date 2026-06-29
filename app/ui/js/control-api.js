const CTRL_INFLIGHT=new Set();
const CTRL_PAGE_INFLIGHT=new Map();
function ctrlPageRequestScope(){
  if(typeof CTRL_OPEN==="undefined"||!CTRL_OPEN)return null;
  return {page:typeof ctrlActivePageName==="function"?ctrlActivePageName():"",seq:typeof CTRL_PAGE_RENDER_SEQ!=="undefined"?CTRL_PAGE_RENDER_SEQ:0};
}
function ctrlPageScopeCurrent(scope){
  return !scope||(!CTRL_OPEN?false:(scope.seq===CTRL_PAGE_RENDER_SEQ&&scope.page===ctrlActivePageName()));
}
function ctrlTrackRequest(ctrl,scope){
  if(!ctrl)return;if(scope&&scope.page){const set=CTRL_PAGE_INFLIGHT.get(scope.page)||new Set();set.add(ctrl);CTRL_PAGE_INFLIGHT.set(scope.page,set);ctrl._ctrlPageScope=scope;}CTRL_INFLIGHT.add(ctrl);
}
function ctrlForgetRequest(ctrl){
  if(!ctrl)return;CTRL_INFLIGHT.delete(ctrl);const scope=ctrl._ctrlPageScope;if(scope&&CTRL_PAGE_INFLIGHT.has(scope.page)){const set=CTRL_PAGE_INFLIGHT.get(scope.page);set.delete(ctrl);if(!set.size)CTRL_PAGE_INFLIGHT.delete(scope.page);}}
function ctrlAbortPageRequests(page){const set=CTRL_PAGE_INFLIGHT.get(page);if(!set)return;for(const ctrl of set){try{ctrl.abort();}catch(_){}}CTRL_PAGE_INFLIGHT.delete(page);}
function ctrlAbortRequests(){CTRL_INFLIGHT.forEach(c=>{try{c.abort();}catch(_){}});CTRL_INFLIGHT.clear();CTRL_PAGE_INFLIGHT.clear();}
function ctrlCancelledError(e){return !!(e&&(e.name==="AbortError"||/request cancelled|abort/i.test(String(e.message||e))));}
function ctrlHandleLockedApi(status,payload){const err=String((payload&&payload.error)||"").toLowerCase();if(status!==401||err!=="locked")return false;CTRL_TOKEN="";try{SAFE_SESSION.remove("dashboardControlToken");}catch(_){}CTRL_LOCK_STATUS={...(CTRL_LOCK_STATUS||{}),unlocked:false};if(CTRL_OPEN&&typeof showPinLock==="function")showPinLock();return true;}
function ctrlObserveCachedPayload(path,payload){
  if(path==="/api/status"&&typeof ctrlObserveCacheBudgetStatus==="function")ctrlObserveCacheBudgetStatus(payload);
}
async function cachedApi(path,onData,opts){
  opts=opts||{};const scope=opts.scope||ctrlPageRequestScope(),had=CTRL_CACHE[path];
  if(had){ctrlObserveCachedPayload(path,had);if(ctrlPageScopeCurrent(scope))onData(had,true);}
  if(had&&ctrlLiteProfile()&&CTRL_OPEN&&!opts.force)return;
  try{const fresh=await api(path,"GET",null,scope);const same=had&&JSON.stringify(had)===JSON.stringify(fresh);CTRL_CACHE[path]=fresh;ctrlObserveCachedPayload(path,fresh);if(!same&&ctrlPageScopeCurrent(scope))onData(fresh,false);}catch(e){if(ctrlCancelledError(e))return;if(String(e.message).toLowerCase().includes("locked")||!had)throw e;}
}
function apiXhr(path,method,body,headers,scope){
  return new Promise((resolve,reject)=>{
    let xhr=null,done=false;
    const finish=(fn,value)=>{if(done)return;done=true;ctrlForgetRequest(xhr);fn(value);};
    try{
      xhr=new XMLHttpRequest();
      if(scope)ctrlTrackRequest(xhr,scope);
      xhr.open(method||"GET",path,true);xhr.setRequestHeader("Accept","application/json");
      if(headers)for(const k of Object.keys(headers))xhr.setRequestHeader(k,headers[k]);
      xhr.onreadystatechange=()=>{
        if(xhr.readyState!==4)return;
        if(scope&&!ctrlPageScopeCurrent(scope)){try{xhr.abort();}catch(_){}finish(reject,new Error("request cancelled"));return;}
        let j={};try{j=xhr.responseText?JSON.parse(xhr.responseText):{};}catch(_){j={};}
        if(xhr.status>=200&&xhr.status<300)finish(resolve,j);
        else if(ctrlHandleLockedApi(xhr.status,j))finish(reject,new Error("locked"));
        else finish(reject,new Error(j.error||("HTTP "+xhr.status)));
      };
      xhr.onerror=()=>finish(reject,new Error("Network request failed"));
      xhr.onabort=()=>finish(reject,new Error("request cancelled"));
      xhr.send(body?JSON.stringify(body):null);
    }catch(e){finish(reject,e);}
  });
}
async function api(path,method,body,scope){
  const verb=method||"GET",headers={};if(body)headers["Content-Type"]="application/json";if(CTRL_TOKEN)headers["X-Dashboard-Token"]=CTRL_TOKEN;
  const requestScope=verb==="GET"?(scope||ctrlPageRequestScope()):null,opts={method:verb};if(Object.keys(headers).length)opts.headers=headers;if(body)opts.body=JSON.stringify(body);
  let ctrl=null;if(typeof AbortController!=="undefined"&&String(path||"").startsWith("/api/")){ctrl=new AbortController();opts.signal=ctrl.signal;ctrlTrackRequest(ctrl,requestScope);}
  try{const res=await fetch(path,opts),j=await res.json().catch(()=>({}));if(requestScope&&!ctrlPageScopeCurrent(requestScope))throw new Error("request cancelled");if(!res.ok){const err=new Error(ctrlHandleLockedApi(res.status,j)?"locked":(j.error||("HTTP "+res.status)));err._http=true;throw err;}return j;}
  catch(e){
    if(requestScope&&!ctrlPageScopeCurrent(requestScope))throw new Error("request cancelled");
    if(e&&(e.name==="AbortError"||/abort/i.test(String(e.message||""))))throw new Error("request cancelled");
    if(e&&e._http)throw e;
    return await apiXhr(path,verb,body,headers,requestScope);
  }
  finally{ctrlForgetRequest(ctrl);}
}
