(function(){
  "use strict";
  window.dashboardFitTierFromViewport=function(width,height){
    var w=Math.max(1,Math.round(Number(width)||0));
    var h=Math.max(1,Math.round(Number(height)||0));
    if(w>=2400||h>=1500)return "xl";
    if(w>=1280&&h>=720)return "base";
    if(w>=1024&&h>=600)return "compact";
    if(w>=860&&h>=520)return "dense";
    return "min";
  };
  try{document.documentElement.dataset.fit=window.dashboardFitTierFromViewport(window.innerWidth,window.innerHeight);}catch(e){}
}());
