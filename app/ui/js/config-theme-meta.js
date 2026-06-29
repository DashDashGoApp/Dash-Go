// 01-config-08-theme-meta.js — theme labels, groups, and defaults.
"use strict";
// Theme picker metadata: grouped, friendly labels for the Dashboard Control theme cards.
// Keep this intentionally lightweight — themes remain CSS-variable sets, not a new app system.
const THEME_META = {
  basic:{label:"Basic",group:"Core",summary:"Original balanced dark theme."},
  slate:{label:"Slate",group:"Core",summary:"Neutral, clean, low-clutter."},
  midnight:{label:"Midnight",group:"Core",summary:"Cool dark blue for evening use."},
  meadow:{label:"Meadow",group:"Core",summary:"Fresh green and warm daylight."},
  ocean:{label:"Ocean",group:"Core",summary:"Aqua, blue, and clean contrast."},
  forest:{label:"Forest",group:"Core",summary:"Deep pine and soft teal."},
  sunset:{label:"Sunset",group:"Core",summary:"Warm orange and coral."},
  coffee:{label:"Coffee",group:"Core",summary:"Warm brown, calm, readable."},
  highcontrast:{label:"High Contrast",group:"Readability",summary:"Maximum contrast for distance viewing."},
  paper:{label:"Paper",group:"Readability",summary:"Light, soft, bright-room display."},
  lowlight:{label:"Low Light",group:"Readability",summary:"Dim night palette with gentle contrast."},
  warmwall:{label:"Warm Wall",group:"Readability",summary:"Cozy warm display with less blue."},
  softmorning:{label:"Soft Morning",group:"Readability",summary:"Gentle daytime light palette."},
  daylight:{label:"Daylight",group:"Readability",summary:"Neutral light palette with crisp ink."},
  christmas:{label:"Christmas",group:"Seasonal",summary:"Pine, red, and gold."},
  halloween:{label:"Halloween",group:"Seasonal",summary:"Orange and purple night palette."},
  thanksgiving:{label:"Thanksgiving",group:"Seasonal",summary:"Harvest gold and cranberry."},
  winter:{label:"Winter",group:"Seasons",summary:"Frosty blue and soft white."},
  spring:{label:"Spring",group:"Seasons",summary:"Fresh bloom colors."},
  autumn:{label:"Autumn",group:"Seasons",summary:"Rust, amber, and dark gold."},
  summer:{label:"Summer",group:"Seasons",summary:"Sky blue, sun gold, and grass green."},
  valentine:{label:"Valentine",group:"Seasonal",summary:"Deep red and rose."},
  stpatricks:{label:"St. Patrick's",group:"Seasonal",summary:"Emerald and gold."},
  easter:{label:"Easter",group:"Seasonal",summary:"Soft pastel celebration."},
  newyear:{label:"New Year",group:"Seasonal",summary:"Midnight, silver, and gold."},
  america:{label:"Independence Day",group:"Seasonal",summary:"Patriotic red, white, and blue."},
  pride:{label:"Pride",group:"Seasonal",summary:"Bright rainbow accent theme."},
  mardigras:{label:"Mardi Gras",group:"Seasonal",summary:"Purple, green, and gold."},
  lunar:{label:"Lunar",group:"Seasonal",summary:"Red, gold, and night sky."},
  oktoberfest:{label:"Oktoberfest",group:"Seasonal",summary:"Bavarian blue and amber."},
  earthday:{label:"Earth Day",group:"Seasonal",summary:"Ocean blue and land green."},
  cincodemayo:{label:"Cinco de Mayo",group:"Seasonal",summary:"Festive papel-picado color."},
  juneteenth:{label:"Juneteenth",group:"Seasonal",summary:"Red, green, black, and gold."},
  muertos:{label:"Día de los Muertos",group:"Seasonal",summary:"Marigold and magenta night."},
  unicorn:{label:"Unicorn",group:"Fun",summary:"Pastel candy and lavender."},
  galaxy:{label:"Galaxy",group:"Fun",summary:"Violet, magenta, and electric cyan."},
  lavender:{label:"Lavender",group:"Fun",summary:"Soft violet and calm contrast."},
  mint:{label:"Mint",group:"Fun",summary:"Fresh mint and cool green."},
  flamingo:{label:"Flamingo",group:"Fun",summary:"Pink, coral, and warm accents."},
  aurora:{label:"Aurora",group:"Fun",summary:"Northern-light greens and blues."},
  synthwave:{label:"Synthwave",group:"Fun",summary:"Retro neon night palette."},
  neapolitan:{label:"Neapolitan",group:"Fun",summary:"Vanilla, strawberry, chocolate."},
  honey:{label:"Honey",group:"Fun",summary:"Gold, amber, and soft warmth."},
  storm:{label:"Storm",group:"Fun",summary:"Cloudy gray with blue accents."},
  tropics:{label:"Tropics",group:"Fun",summary:"Bright green and ocean color."},
  cherry:{label:"Cherry",group:"Fun",summary:"Red fruit and soft contrast."},
  citrus:{label:"Citrus",group:"Fun",summary:"Lime, lemon, and orange splash."},
  cosmos:{label:"Cosmos",group:"Fun",summary:"Deep indigo and periwinkle."},
  lagoon:{label:"Lagoon",group:"Fun",summary:"Turquoise and clean tropical water."},
  orchid:{label:"Orchid",group:"Fun",summary:"Plum and orchid magenta."},
  desert:{label:"Desert",group:"More",summary:"Muted sand and clay."},
  rose:{label:"Rose",group:"More",summary:"Warm rosy red and coral."},
  jade:{label:"Jade",group:"More",summary:"Cool green gemstone palette."},
  ruby:{label:"Ruby",group:"More",summary:"Deep red gemstone palette."},
  sapphire:{label:"Sapphire",group:"More",summary:"Rich blue gemstone palette."},
  noir:{label:"Noir",group:"More",summary:"Dramatic dark monochrome."},
  terminal:{label:"Terminal",group:"More",summary:"Green-on-dark console feel."},
  denim:{label:"Denim",group:"More",summary:"Soft blue everyday display."},
  moss:{label:"Moss",group:"More",summary:"Earthy green and brown."},
  firefly:{label:"Firefly",group:"More",summary:"Dark green with warm glows."},
  canyon:{label:"Canyon",group:"More",summary:"Red rock and warm dusk."},
  harbor:{label:"Harbor",group:"More",summary:"Steel blue-gray nautical palette."},
  olive:{label:"Olive",group:"More",summary:"Warm khaki, olive, and brass."},
  plum:{label:"Plum",group:"More",summary:"Deep wine gemstone palette."},
  glacier:{label:"Glacier",group:"More",summary:"Pale ice-cyan on navy."},
  ember:{label:"Ember",group:"More",summary:"Smoldering charcoal and ember red."}
};
const THEME_GROUP_ORDER=["Readability","Core","Seasons","Seasonal","Fun","More"];
const THEME_BASE={
  "--bg":"#0a0a0d","--panel":"rgba(255,255,255,0.025)","--fg":"#e8e8ea",
  "--dim":"#8a8a93","--dimmer":"#5a5a62","--line":"#43454f","--line-soft":"#303239",
  "--today":"#d9c074","--sat":"#8bb4d4","--sun":"#d99a9a","--accent":"#8fc4a6",
  "--pay":"#cda76a","--endtime":"#9a8fb0","--altweek":"rgba(255,255,255,0.022)",
  "--todaywash":"rgba(217,192,116,0.12)"
};
function themeInfo(name){
  const raw=String(name||"basic");
  const fallback=raw.replace(/[-_]/g," ").replace(/\b\w/g,c=>c.toUpperCase());
  return Object.assign({label:fallback,group:"More",summary:"Dashboard color theme."}, THEME_META[raw]||{});
}
function themeVars(name){ return Object.assign({}, THEME_BASE, THEMES[name]||{}); }
