// 00-app-dialog.js — tiny shared focus helpers for Dashboard app dialogs.
// App shells still own their own open/close/escape semantics; this only keeps
// focus behavior consistent on the touch-first kiosk.
(function(){
  function nextFrame(fn){
    if(typeof requestAnimationFrame==="function")requestAnimationFrame(fn);
    else setTimeout(fn,0);
  }
  function focusInitial(root,selector){
    nextFrame(()=>{
      if(!root||!root.classList||!root.classList.contains("show"))return;
      root.querySelector(selector)?.focus?.();
    });
  }
  function restoreFocus(prior,fallback){
    const target=fallback&&fallback.isConnected&&!fallback.hidden?fallback:prior;
    target?.focus?.();
  }
  window.DashGoAppDialog=Object.freeze({focusInitial,restoreFocus});
})();
