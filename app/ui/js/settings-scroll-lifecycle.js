/* =====================================================================
   ===============  SCROLL ROOT LIFECYCLE / ANCHORS  ===================
   Known roots only: this helper never scans the document or observes DOM
   mutations. It records user input and preserves keyed reading position only
   around explicit render transactions, never while a finger is scrolling.
   ===================================================================== */
const DASH_SCROLL_ROOT_STATE=new WeakMap();
function scrollRootState(root,policy){
  if(!root)return null;
  let state=DASH_SCROLL_ROOT_STATE.get(root);
  if(!state){
    state={inputEpoch:0,restoreToken:0,lastInputAt:0};
    const mark=()=>{state.inputEpoch++;state.lastInputAt=Date.now();};
    // These listeners only record an epoch. They do not read layout, mutate
    // DOM, prevent scrolling, or attach a touchmove/pointermove blocker.
    root.addEventListener("pointerdown",mark,{passive:true});
    root.addEventListener("touchstart",mark,{passive:true});
    root.addEventListener("wheel",mark,{passive:true});
    DASH_SCROLL_ROOT_STATE.set(root,state);
  }
  if(policy)root.dataset.scrollPolicy=policy;
  return state;
}
function scrollRootInputEpoch(root){const state=scrollRootState(root);return state?state.inputEpoch:0;}
function captureScrollAnchor(root,itemSelector,keyName){
  if(!root)return null;
  const state=scrollRootState(root);
  const snapshot={key:"",relativeTop:0,fallbackTop:root.scrollTop,inputEpoch:state.inputEpoch};
  const rootRect=root.getBoundingClientRect();
  for(const item of root.querySelectorAll(itemSelector)){
    const key=item.dataset?item.dataset[keyName]:"";
    if(!key)continue;
    const rect=item.getBoundingClientRect();
    if(rect.bottom>rootRect.top+1&&rect.top<rootRect.bottom-1){
      snapshot.key=key;
      snapshot.relativeTop=rect.top-rootRect.top;
      break;
    }
  }
  return snapshot;
}
function restoreScrollAnchor(root,snapshot,itemSelector,keyName,onComplete){
  if(!root||!snapshot)return;
  const state=scrollRootState(root);
  const token=++state.restoreToken;
  const defer=typeof requestAnimationFrame==="function"?requestAnimationFrame:fn=>setTimeout(fn,0);
  const complete=()=>{
    if(typeof scrollIdleReturnReconcile==="function")scrollIdleReturnReconcile(root);
    if(typeof onComplete==="function")onComplete();
  };
  defer(()=>{
    if(!root.isConnected||state.restoreToken!==token)return;
    // A gesture during the rebuild invalidates the element anchor (the captured
    // node may already be detached), so the anchor path is skipped — but it must
    // NOT skip the restore itself. Callers clear the list before rebuilding,
    // which clamps scrollTop to 0, so returning here would snap the list to the
    // top under the viewer's finger. Unless the viewer has since moved the list
    // themselves, fall back to the numeric offset captured before the wipe.
    const gestureActive = state.inputEpoch!==snapshot.inputEpoch;
    if(gestureActive && root.scrollTop>4){ complete(); return; }
    let target=null;
    if(!gestureActive && snapshot.key){
      for(const item of root.querySelectorAll(itemSelector)){
        if(item.dataset&&item.dataset[keyName]===snapshot.key){target=item;break;}
      }
    }
    if(target){
      const rootTop=root.getBoundingClientRect().top;
      root.scrollTop+=target.getBoundingClientRect().top-rootTop-snapshot.relativeTop;
      complete();
      return;
    }
    const max=Math.max(0,root.scrollHeight-root.clientHeight);
    root.scrollTop=Math.max(0,Math.min(max,snapshot.fallbackTop));
    complete();
  });
}
