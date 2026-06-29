function visualStyleButton(map,key,current,apply){
  const item=map[key]||{label:key,summary:""};
  const b=cbtn("","visualchoice"+(key===current?" on":""),()=>apply(key));
  b.innerHTML=`<span class="visualchoice-title">${escapeHTML(item.label)}</span><span class="visualchoice-sub">${escapeHTML(item.summary||"")}</span>`;
  return b;
}
function renderCtrlVisualStyle(){
  const wrap=$("#ctrlvisualstyle"); if(!wrap) return;
  wrap.innerHTML="";
  wrap.appendChild(ctrlStateCard("info","Weather icons & seasonal décor","Choose local SVG icon and seasonal decoration styling. Dashboard text typography is customized in Display → Dashboard display."));

  const icons=actionGroup("Weather SVG style","Affects the large current icon, forecast rows, and weather popups.","visualstylegroup");
  const curIcon=SETTINGS.weatherIconStyle||CONFIG.weatherIconStyle||"soft";
  for(const key of ["soft","bold","outline","contrast","playful"]){
    const item=WEATHER_ICON_STYLES[key];
    const b=visualStyleButton(WEATHER_ICON_STYLES,key,curIcon,(value)=>{
      if(value===(SETTINGS.weatherIconStyle||CONFIG.weatherIconStyle||"soft")) return;
      SETTINGS.weatherIconStyle=value;
      const changed=applyVisualSettings();
      if(changed.icon) renderWeather();
      postSettings(); renderCtrlVisualStyle();
    });
    const sample=el("span","visualiconpreview","");
    if(typeof iconSetForStyle==="function"){
      const set=iconSetForStyle(key);
      sample.innerHTML=set.storm||set.cloud||"";
    }
    b.insertBefore(sample,b.firstChild);
    icons.grid.appendChild(b);
  }
  wrap.appendChild(icons.group);

  const decor=actionGroup("Seasonal décor","Holiday SVG accents are only placed in empty calendar cells for seasonal themes.","visualstylegroup");
  const curDecor=SETTINGS.seasonalDecor||CONFIG.seasonalDecor||"off";
  for(const key of ["off","subtle","standard"]){
    decor.grid.appendChild(visualStyleButton(SEASONAL_DECOR_MODES,key,curDecor,(value)=>{
      if(value===(SETTINGS.seasonalDecor||CONFIG.seasonalDecor||"off")) return;
      SETTINGS.seasonalDecor=value;
      const changed=applyVisualSettings();
      if(changed.decor) renderCalendar();
      postSettings(); renderCtrlVisualStyle();
    }));
  }
  wrap.appendChild(decor.group);
}
