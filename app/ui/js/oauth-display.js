// oauth-display.js — one-shot kiosk QR presentation for SSH-armed Google OAuth.
// The server returns 404 unless a short-lived phone sign-in display is armed. The dashboard only
// polls a tiny loopback response; a one-second countdown runs while visible.
let OAUTH_DISPLAY_POLL_TIMER=null;
let OAUTH_DISPLAY_CLOCK_TIMER=null;
let OAUTH_DISPLAY_META=null;
let OAUTH_DISPLAY_BUSY=false;

function oauthDisplayRoot(){ return document.getElementById("oauthdisplay"); }
function oauthDisplayClock(){
  const countdown=document.getElementById("oauthdisplay-countdown");
  if(!countdown||!OAUTH_DISPLAY_META)return;
  const remaining=Math.max(0,Math.ceil((Number(OAUTH_DISPLAY_META.expires_at)*1000-Date.now())/1000));
  if(!remaining){ oauthDisplayHide(); return; }
  const minutes=Math.floor(remaining/60),seconds=String(remaining%60).padStart(2,"0");
  countdown.textContent=`This connection expires in ${minutes}:${seconds}.`;
}
function oauthDisplayStartClock(){
  oauthDisplayClock();
  if(OAUTH_DISPLAY_CLOCK_TIMER)return;
  OAUTH_DISPLAY_CLOCK_TIMER=setInterval(oauthDisplayClock,1000);
}
function oauthDisplayStopClock(){
  if(OAUTH_DISPLAY_CLOCK_TIMER){ clearInterval(OAUTH_DISPLAY_CLOCK_TIMER); OAUTH_DISPLAY_CLOCK_TIMER=null; }
}
function oauthDisplayHide(){
  OAUTH_DISPLAY_META=null;
  oauthDisplayStopClock();
  const root=oauthDisplayRoot();
  if(!root)return;
  root.classList.remove("show"); root.setAttribute("aria-hidden","true");
  const image=document.getElementById("oauthdisplay-qr");
  if(image){ image.removeAttribute("src"); image.hidden=true; }
}
function oauthDisplayShow(meta){
  const root=oauthDisplayRoot();
  if(!root||!meta||!meta.connection||!meta.url||!Number(meta.expires_at))return;
  const changed=!OAUTH_DISPLAY_META||OAUTH_DISPLAY_META.url!==meta.url||Number(OAUTH_DISPLAY_META.expires_at)!==Number(meta.expires_at);
  OAUTH_DISPLAY_META=meta;
  document.getElementById("oauthdisplay-connection").textContent=`Connect Google Calendar: ${meta.connection}`;
  const fallback=document.getElementById("oauthdisplay-fallback");
  if(fallback){
    try{ fallback.textContent=`1. Scan this code. 2. Sign in to Google. 3. When your phone cannot open 127.0.0.1, copy the complete browser address and paste it into the setup terminal (${new URL(meta.url).host}).`; }
    catch(_){ fallback.textContent="Scan this code, sign in to Google, then copy the complete failed browser address and paste it into the setup terminal."; }
  }
  const image=document.getElementById("oauthdisplay-qr");
  if(image&&changed){ image.src=`/api/oauth-display/qr.png?t=${encodeURIComponent(meta.expires_at)}`; image.hidden=false; }
  root.classList.add("show"); root.setAttribute("aria-hidden","false");
  oauthDisplayStartClock();
}
async function oauthDisplayPoll(){
  if(OAUTH_DISPLAY_BUSY)return;
  OAUTH_DISPLAY_BUSY=true;
  let active=false,error=false;
  try{
    // Long poll: the server holds the request (up to 30s) until a display
    // arms, so the panel appears in the same present as the QR write.
    const response=await fetch("/api/oauth-display?wait=30",{cache:"no-store"});
    if(response.status===404){ oauthDisplayHide(); }
    else{
      if(!response.ok)throw new Error("OAuth display unavailable");
      const meta=await response.json();
      oauthDisplayShow(meta);
      active=!!(meta&&meta.url);
    }
  }catch(_){ oauthDisplayHide(); error=true; }
  finally{ OAUTH_DISPLAY_BUSY=false; }
  // One request at a time; the successor is scheduled immediately, so the
  // panel's lifetime is a tight chain of long polls instead of a periodic sweep.
  oauthDisplayScheduleNext(error?5000:active?1000:100);
}
function oauthDisplayScheduleNext(delay){
  if(OAUTH_DISPLAY_POLL_TIMER)clearTimeout(OAUTH_DISPLAY_POLL_TIMER);
  OAUTH_DISPLAY_POLL_TIMER=setTimeout(()=>{
    OAUTH_DISPLAY_POLL_TIMER=null;
    const poll=()=>oauthDisplayPoll();
    if(typeof runOrDeferDashboardWork==="function")runOrDeferDashboardWork("oauth-display",poll); else poll();
  },delay);
}
function oauthDisplayBoot(){
  if(OAUTH_DISPLAY_POLL_TIMER)return;
  oauthDisplayPoll();
  document.addEventListener("visibilitychange",()=>{ if(!document.hidden)oauthDisplayPoll(); });
}
