// 04-calendar-03-seasonal-decor.js — lightweight holiday SVG accents.
// Decorations only enter empty calendar cells, keeping event chips, day numbers,
// weather text, and agenda/sidebar content unobstructed.
const SEASONAL_DECOR_SVGS={
  halloween:[
    '<svg viewBox="0 0 48 48"><path d="M8 24c7-9 13-9 16-2 3-7 9-7 16 2-7-2-11 0-13 5h-6c-2-5-6-7-13-5z" fill="currentColor"/><circle cx="20" cy="25" r="1.4" fill="var(--bg)"/><circle cx="28" cy="25" r="1.4" fill="var(--bg)"/></svg>',
    '<svg viewBox="0 0 48 48"><path d="M4 8h28M4 8v28M4 8c14 7 21 14 28 28M8 20c8 0 16 8 16 16M18 9c0 6 7 13 14 13" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"/><path d="M29 27l3-4 3 4-3 7z" fill="currentColor"/></svg>',
    '<svg viewBox="0 0 48 48"><path d="M10 28c4-10 20-10 28 0-5 4-12 6-20 6-4 0-7-2-8-6z" fill="currentColor"/><path d="M20 16h8l4 6H16z" fill="currentColor"/><path d="M17 28c4 2 10 2 14 0" stroke="var(--bg)" stroke-width="2" fill="none" stroke-linecap="round"/></svg>'
  ],
  christmas:[
    '<svg viewBox="0 0 48 48"><path d="M24 8l12 16h-7l9 12H10l9-12h-7L24 8z" fill="currentColor"/><path d="M12 27c8 3 16 3 24 0" stroke="var(--bg)" stroke-width="3" fill="none" stroke-linecap="round"/><rect x="21" y="36" width="6" height="7" rx="1" fill="var(--pay)"/></svg>',
    '<svg viewBox="0 0 48 48"><path d="M16 20c5-9 11-9 16 0" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round"/><circle cx="18" cy="25" r="5" fill="currentColor"/><circle cx="30" cy="25" r="5" fill="currentColor"/><circle cx="24" cy="32" r="4" fill="var(--pay)"/><path d="M24 20c1-6 5-9 10-9" stroke="currentColor" stroke-width="2.5" fill="none" stroke-linecap="round"/></svg>',
    '<svg viewBox="0 0 48 48"><circle cx="24" cy="27" r="13" fill="currentColor"/><path d="M17 14c3-5 11-5 14 0" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round"/><path d="M24 15v25M15 27h18" stroke="var(--bg)" stroke-width="2.4" stroke-linecap="round"/><circle cx="24" cy="27" r="4" fill="var(--pay)"/></svg>'
  ],
  thanksgiving:[
    '<svg viewBox="0 0 48 48"><path d="M10 29c8-11 20-14 31-7-3 12-14 17-28 14l-5 4" fill="none" stroke="currentColor" stroke-width="5" stroke-linecap="round" stroke-linejoin="round"/><path d="M17 28c8 0 14-3 21-8" stroke="var(--bg)" stroke-width="2" fill="none" stroke-linecap="round"/></svg>',
    '<svg viewBox="0 0 48 48"><path d="M12 31c0-7 5-13 12-13s12 6 12 13v6H12z" fill="currentColor"/><path d="M12 22c2-9 8-12 12-3 4-9 10-6 12 3 3-3 8 0 7 6" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round"/><circle cx="29" cy="28" r="1.5" fill="var(--bg)"/></svg>',
    '<svg viewBox="0 0 48 48"><path d="M24 13c8 0 13 8 9 17-2 6-6 10-9 10s-7-4-9-10c-4-9 1-17 9-17z" fill="currentColor"/><path d="M18 13c2-7 10-7 12 0" stroke="currentColor" stroke-width="3" fill="none" stroke-linecap="round"/><path d="M18 29c4 2 8 2 12 0" stroke="var(--bg)" stroke-width="2" fill="none" stroke-linecap="round"/></svg>'
  ],
  autumn:[
    '<svg viewBox="0 0 48 48"><path d="M24 6l4 10 10-4-4 10 10 4-11 3 5 11-11-5-3 9-4-9-10 5 4-11-10-3 10-4-4-10 10 4z" fill="currentColor"/><path d="M16 35c8-7 13-14 16-23" stroke="var(--bg)" stroke-width="2" fill="none" stroke-linecap="round"/></svg>',
    '<svg viewBox="0 0 48 48"><path d="M24 12c7 0 11 8 8 16-2 5-5 9-8 9s-6-4-8-9c-3-8 1-16 8-16z" fill="currentColor"/><path d="M19 12c1-6 9-6 10 0" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round"/><path d="M16 28c5 2 11 2 16 0" stroke="var(--bg)" stroke-width="2" fill="none" stroke-linecap="round"/></svg>',
    '<svg viewBox="0 0 48 48"><path d="M25 5c5 10 13 12 18 14-6 4-10 10-11 19-5-6-12-6-22-3 5-7 5-16 15-30z" fill="currentColor"/><path d="M15 34c8-8 14-15 22-18" stroke="var(--bg)" stroke-width="2" stroke-linecap="round"/></svg>'
  ],
  winter:[
    '<svg viewBox="0 0 48 48"><g stroke="currentColor" stroke-width="3" stroke-linecap="round"><path d="M24 6v36M8 15l32 18M40 15L8 33"/><path d="M17 9l7 7 7-7M17 39l7-7 7 7"/></g></svg>',
    '<svg viewBox="0 0 48 48"><g stroke="currentColor" stroke-width="2.6" stroke-linecap="round"><path d="M24 7v34M7 24h34M12 12l24 24M36 12L12 36"/><path d="M18 11l6 6 6-6M18 37l6-6 6 6"/></g></svg>',
    '<svg viewBox="0 0 48 48"><path d="M15 20c0-8 10-10 12-2l3 16c1 6-5 10-10 6l-8-6c-5-4-3-14 3-14z" fill="currentColor"/><path d="M16 24c5 2 9 1 12-3" stroke="var(--bg)" stroke-width="2" fill="none" stroke-linecap="round"/><path d="M34 8v26M40 12v20M28 15v17" stroke="currentColor" stroke-width="3" stroke-linecap="round"/></svg>'
  ],
  spring:[
    '<svg viewBox="0 0 48 48"><path d="M24 38V20" stroke="currentColor" stroke-width="3" stroke-linecap="round"/><path d="M15 20c7 0 9 5 9 12-7 0-12-4-12-9 0-2 1-3 3-3zM33 20c-7 0-9 5-9 12 7 0 12-4 12-9 0-2-1-3-3-3z" fill="currentColor"/><path d="M18 12c0 9 12 9 12 0 0-4-3-7-6-7s-6 3-6 7z" fill="var(--pay)"/></svg>',
    '<svg viewBox="0 0 48 48"><path d="M24 40V22" stroke="currentColor" stroke-width="3" stroke-linecap="round"/><path d="M23 28c-8-1-12-5-12-11 7 0 12 4 12 11zM25 29c8-1 12-5 12-11-7 0-12 4-12 11z" fill="currentColor"/><path d="M24 8c5 5 8 10 8 15 0 4-4 7-8 7s-8-3-8-7c0-5 3-10 8-15z" fill="var(--accent)"/></svg>',
    '<svg viewBox="0 0 48 48"><path d="M28 7c7 10 9 17 6 22-3 6-12 6-15 0-3-5 0-12 9-22z" fill="currentColor"/><path d="M21 34c4-7 9-11 17-13" stroke="var(--bg)" stroke-width="2" fill="none" stroke-linecap="round"/><path d="M12 38c10 0 15-5 16-13-10 0-16 5-16 13z" fill="var(--accent)"/></svg>'
  ],
  summer:[
    '<svg viewBox="0 0 48 48"><circle cx="24" cy="24" r="9" fill="currentColor"/><g stroke="currentColor" stroke-width="3" stroke-linecap="round"><path d="M24 5v7M24 36v7M5 24h7M36 24h7M11 11l5 5M32 32l5 5M37 11l-5 5M16 32l-5 5"/></g></svg>',
    '<svg viewBox="0 0 48 48"><path d="M7 25c5-12 28-12 34 0-8-2-12 2-17 0-5 2-9-2-17 0z" fill="currentColor"/><path d="M24 25v17" stroke="currentColor" stroke-width="3" stroke-linecap="round"/><path d="M14 25c4-8 16-8 20 0" stroke="var(--bg)" stroke-width="2" fill="none"/></svg>',
    '<svg viewBox="0 0 48 48"><path d="M16 11h16v22c0 6-4 10-8 10s-8-4-8-10z" fill="currentColor"/><path d="M16 20h16M20 11v27" stroke="var(--bg)" stroke-width="2" stroke-linecap="round"/><path d="M7 32c6-5 10 5 16 0s10 5 18 0" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round"/></svg>'
  ],
  valentine:[
    '<svg viewBox="0 0 48 48"><path d="M24 41S9 31 9 19c0-9 11-12 15-4 4-8 15-5 15 4 0 12-15 22-15 22z" fill="currentColor"/></svg>',
    '<svg viewBox="0 0 48 48"><path d="M18 34S7 27 7 18c0-7 8-9 11-3 3-6 11-4 11 3 0 9-11 16-11 16z" fill="currentColor"/><path d="M32 38s-8-5-8-12c0-5 6-7 8-2 2-5 8-3 8 2 0 7-8 12-8 12z" fill="var(--pay)"/></svg>',
    '<svg viewBox="0 0 48 48"><rect x="8" y="15" width="32" height="22" rx="3" fill="currentColor"/><path d="M9 17l15 12 15-12" fill="none" stroke="var(--bg)" stroke-width="2.4"/><path d="M24 33s-6-4-6-8c0-4 5-5 6-1 1-4 6-3 6 1 0 4-6 8-6 8z" fill="var(--pay)"/></svg>'
  ],
  stpatricks:[
    '<svg viewBox="0 0 48 48"><path d="M24 20c-4-8-15-4-12 4 2 5 8 5 12 1 4 4 10 4 12-1 3-8-8-12-12-4z" fill="currentColor"/><path d="M24 20c5-7-5-16-10-8-4 7 4 11 10 8zM24 20c-5-7 5-16 10-8 4 7-4 11-10 8zM24 24c0 7-3 12-8 16h16c-5-4-8-9-8-16z" fill="currentColor"/></svg>',
    '<svg viewBox="0 0 48 48"><path d="M12 33h24v5c0 3-3 5-6 5H18c-3 0-6-2-6-5z" fill="currentColor"/><path d="M18 33c0-8 4-13 12-13" fill="none" stroke="currentColor" stroke-width="4" stroke-linecap="round"/><circle cx="35" cy="37" r="4" fill="var(--pay)"/><circle cx="29" cy="37" r="4" fill="var(--pay)"/></svg>',
    '<svg viewBox="0 0 48 48"><path d="M13 34c0-18 22-18 22 0" fill="none" stroke="currentColor" stroke-width="7" stroke-linecap="round"/><path d="M13 34h8M27 34h8" stroke="var(--bg)" stroke-width="3" stroke-linecap="round"/></svg>'
  ],
  easter:[
    '<svg viewBox="0 0 48 48"><path d="M24 7c8 9 13 18 13 27 0 8-6 12-13 12S11 42 11 34c0-9 5-18 13-27z" fill="currentColor"/><path d="M14 31c4-4 7-4 10 0s7 4 10 0" stroke="var(--bg)" stroke-width="2.3" fill="none"/><path d="M16 23h16" stroke="var(--bg)" stroke-width="2.3" stroke-linecap="round"/></svg>',
    '<svg viewBox="0 0 48 48"><path d="M18 22C10 5 19 3 23 20c6-17 15-14 7 2 6 2 10 8 8 14-3 9-25 9-28 0-2-6 2-12 8-14z" fill="currentColor"/><circle cx="19" cy="31" r="1.5" fill="var(--bg)"/><circle cx="29" cy="31" r="1.5" fill="var(--bg)"/><path d="M22 35h4" stroke="var(--bg)" stroke-width="2" stroke-linecap="round"/></svg>',
    '<svg viewBox="0 0 48 48"><path d="M10 29h28l-4 13H14z" fill="currentColor"/><path d="M14 29c2-11 18-11 20 0" fill="none" stroke="currentColor" stroke-width="4" stroke-linecap="round"/><circle cx="19" cy="30" r="5" fill="var(--pay)"/><circle cx="29" cy="30" r="5" fill="var(--accent)"/></svg>'
  ],
  newyear:[
    '<svg viewBox="0 0 48 48"><path d="M24 5l4 14 14 5-14 5-4 14-5-14-14-5 14-5z" fill="currentColor"/><circle cx="39" cy="10" r="3" fill="var(--pay)"/><circle cx="9" cy="38" r="2.5" fill="var(--accent)"/></svg>',
    '<svg viewBox="0 0 48 48"><circle cx="24" cy="24" r="16" fill="none" stroke="currentColor" stroke-width="3"/><path d="M24 13v12l8-8" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"/><path d="M15 38h18" stroke="var(--pay)" stroke-width="3" stroke-linecap="round"/></svg>',
    '<svg viewBox="0 0 48 48"><path d="M18 9h12l-2 24c-.5 6-7.5 6-8 0z" fill="currentColor"/><path d="M19 19h10" stroke="var(--bg)" stroke-width="2"/><path d="M24 36v7M18 43h12" stroke="currentColor" stroke-width="3" stroke-linecap="round"/></svg>'
  ],
  america:[
    '<svg viewBox="0 0 48 48"><path d="M9 11c10-5 19 5 30 0v25c-11 5-20-5-30 0z" fill="currentColor"/><path d="M9 18c10-5 19 5 30 0M9 25c10-5 19 5 30 0" stroke="var(--bg)" stroke-width="2" fill="none"/><path d="M12 12v27" stroke="currentColor" stroke-width="3" stroke-linecap="round"/></svg>',
    '<svg viewBox="0 0 48 48"><path d="M24 6l3 9 10-3-6 8 8 6-10 1 1 10-6-8-7 8 2-10-10-1 8-6-6-8 10 3z" fill="currentColor"/><circle cx="38" cy="10" r="3" fill="var(--pay)"/><circle cx="10" cy="38" r="2.5" fill="var(--accent)"/></svg>',
    '<svg viewBox="0 0 48 48"><g fill="currentColor"><path d="M14 8l3 7 7 1-5 5 1 7-6-4-6 4 1-7-5-5 7-1z"/><path d="M32 17l2 5 6 1-4 4 1 6-5-3-5 3 1-6-4-4 6-1z"/><path d="M19 31l2 5 5 1-4 3 1 5-4-3-5 3 1-5-4-3 5-1z"/></g></svg>'
  ],
  mardigras:[
    '<svg viewBox="0 0 48 48"><path d="M24 6c5 7 10 9 15 9-2 9-8 17-15 27C17 32 11 24 9 15c5 0 10-2 15-9z" fill="currentColor"/><path d="M24 9v29M14 18c5 4 7 4 10-1 3 5 5 5 10 1" stroke="var(--bg)" stroke-width="2" fill="none" stroke-linecap="round"/></svg>',
    '<svg viewBox="0 0 48 48"><path d="M8 30c5-12 27-12 32 0-2 7-8 10-16 10S10 37 8 30z" fill="currentColor"/><path d="M8 29L3 25M40 29l5-4" stroke="currentColor" stroke-width="3" stroke-linecap="round"/><circle cx="18" cy="30" r="3" fill="var(--bg)"/><circle cx="30" cy="30" r="3" fill="var(--bg)"/></svg>',
    '<svg viewBox="0 0 48 48"><path d="M9 34l4-20 11 11 11-11 4 20z" fill="currentColor"/><circle cx="13" cy="14" r="4" fill="var(--pay)"/><circle cx="24" cy="25" r="4" fill="var(--accent)"/><circle cx="35" cy="14" r="4" fill="var(--pay)"/></svg>'
  ],
  lunar:[
    '<svg viewBox="0 0 48 48"><path d="M15 12h18l4 8-4 18H15l-4-18z" fill="currentColor"/><path d="M24 8v34M16 20h16" stroke="var(--bg)" stroke-width="2"/><path d="M15 12h18" stroke="currentColor" stroke-width="3" stroke-linecap="round"/></svg>',
    '<svg viewBox="0 0 48 48"><rect x="12" y="15" width="24" height="22" rx="3" fill="currentColor"/><path d="M12 20l12 9 12-9" stroke="var(--bg)" stroke-width="2" fill="none"/><circle cx="24" cy="27" r="5" fill="var(--pay)"/></svg>',
    '<svg viewBox="0 0 48 48"><path d="M14 35c10-2 16-9 17-20 4 5 6 11 3 17-4 8-13 10-20 3z" fill="currentColor"/><path d="M28 12c5 0 8 4 8 8" stroke="var(--pay)" stroke-width="3" fill="none" stroke-linecap="round"/></svg>'
  ],
  earthday:[
    '<svg viewBox="0 0 48 48"><circle cx="24" cy="24" r="17" fill="currentColor"/><path d="M15 18c5 1 7-5 13-2 4 2 1 6 6 8 3 1 3 5 0 8-5 6-10 2-12-3-1-3-7-2-8-6-.5-2 0-4 1-5z" fill="var(--bg)" opacity=".5"/></svg>',
    '<svg viewBox="0 0 48 48"><path d="M12 34c7 7 19 7 26-1" fill="none" stroke="currentColor" stroke-width="4" stroke-linecap="round"/><path d="M17 30c2-12 13-16 23-14-2 13-11 20-23 14z" fill="currentColor"/><path d="M20 29c6-3 10-7 14-12" stroke="var(--bg)" stroke-width="2" fill="none" stroke-linecap="round"/></svg>',
    '<svg viewBox="0 0 48 48"><path d="M24 41V23" stroke="currentColor" stroke-width="4" stroke-linecap="round"/><path d="M24 27c-10-1-15-7-15-15 9 0 15 6 15 15zM24 27c10-1 15-7 15-15-9 0-15 6-15 15z" fill="currentColor"/><path d="M14 41h20" stroke="var(--pay)" stroke-width="3" stroke-linecap="round"/></svg>'
  ],
  pride:[
    '<svg viewBox="0 0 48 48"><path d="M24 40S9 30 9 18c0-8 10-11 15-3 5-8 15-5 15 3 0 12-15 22-15 22z" fill="currentColor"/><path d="M14 23h20M17 29h14" stroke="var(--bg)" stroke-width="2" stroke-linecap="round"/></svg>',
    '<svg viewBox="0 0 48 48"><path d="M8 36c0-16 10-28 16-28s16 12 16 28h-8c0-10-5-19-8-19s-8 9-8 19z" fill="currentColor"/><path d="M12 36c2-14 9-23 12-23s10 9 12 23" fill="none" stroke="var(--bg)" stroke-width="2" stroke-linecap="round"/></svg>',
    '<svg viewBox="0 0 48 48"><path d="M24 7l5 11 12 2-9 8 3 12-11-6-11 6 3-12-9-8 12-2z" fill="currentColor"/><circle cx="24" cy="24" r="5" fill="var(--bg)"/></svg>'
  ],
  oktoberfest:[
    '<svg viewBox="0 0 48 48"><rect x="11" y="14" width="22" height="27" rx="4" fill="currentColor"/><path d="M33 22h5c4 0 5 12 0 12h-5" fill="none" stroke="currentColor" stroke-width="4"/><path d="M16 14V9h12v5M17 22h10M17 30h10" stroke="var(--bg)" stroke-width="2" stroke-linecap="round"/></svg>',
    '<svg viewBox="0 0 48 48"><path d="M14 24c-8-5-2-14 6-9 2-8 14-8 16 0 8-5 14 4 6 9-6 4-22 4-28 0z" fill="none" stroke="currentColor" stroke-width="4" stroke-linecap="round"/><path d="M12 30c7 7 17 7 24 0" stroke="currentColor" stroke-width="4" fill="none" stroke-linecap="round"/></svg>',
    '<svg viewBox="0 0 48 48"><path d="M16 12h16l3 26H13z" fill="currentColor"/><path d="M17 18h14M18 26h12" stroke="var(--bg)" stroke-width="2"/><circle cx="18" cy="38" r="3" fill="var(--pay)"/><circle cx="30" cy="38" r="3" fill="var(--pay)"/></svg>'
  ],
  cincodemayo:[
    '<svg viewBox="0 0 48 48"><path d="M7 12h34v20l-7-5-7 5-7-5-6 5-7-5z" fill="currentColor"/><path d="M11 17h26M14 22h20" stroke="var(--bg)" stroke-width="2" stroke-linecap="round"/></svg>',
    '<svg viewBox="0 0 48 48"><path d="M16 9c6 0 10 9 6 16l-8 14c-2 4-8 1-6-3l8-14c-8-1-8-13 0-13z" fill="currentColor"/><path d="M31 9c-6 0-10 9-6 16l8 14c2 4 8 1 6-3l-8-14c8-1 8-13 0-13z" fill="var(--pay)"/></svg>',
    '<svg viewBox="0 0 48 48"><path d="M27 8c7 7 8 16 2 26-3 5-8 7-13 5 8-7 9-13 7-22-1-4 1-7 4-9z" fill="currentColor"/><path d="M24 14c5 5 5 12 1 19" stroke="var(--bg)" stroke-width="2" fill="none" stroke-linecap="round"/></svg>'
  ],
  juneteenth:[
    '<svg viewBox="0 0 48 48"><path d="M24 6l4 12 12-4-6 11 10 7-12 1 1 12-9-8-9 8 1-12-12-1 10-7-6-11 12 4z" fill="currentColor"/><circle cx="24" cy="25" r="5" fill="var(--bg)"/></svg>',
    '<svg viewBox="0 0 48 48"><path d="M9 13h30v22H9z" fill="currentColor"/><path d="M9 20h30M9 28h30" stroke="var(--bg)" stroke-width="2"/><path d="M24 15l3 7 7 1-5 5 1 7-6-4-6 4 1-7-5-5 7-1z" fill="var(--pay)"/></svg>',
    '<svg viewBox="0 0 48 48"><path d="M24 8l3 9 9-3-5 8 8 5-9 1 1 9-7-6-7 6 1-9-9-1 8-5-5-8 9 3z" fill="currentColor"/><path d="M10 39h28" stroke="var(--pay)" stroke-width="4" stroke-linecap="round"/></svg>'
  ],
  muertos:[
    '<svg viewBox="0 0 48 48"><circle cx="24" cy="24" r="6" fill="var(--pay)"/><g fill="currentColor"><ellipse cx="24" cy="10" rx="4" ry="8"/><ellipse cx="24" cy="38" rx="4" ry="8"/><ellipse cx="10" cy="24" rx="8" ry="4"/><ellipse cx="38" cy="24" rx="8" ry="4"/><ellipse cx="14" cy="14" rx="4" ry="7" transform="rotate(-45 14 14)"/><ellipse cx="34" cy="34" rx="4" ry="7" transform="rotate(-45 34 34)"/><ellipse cx="34" cy="14" rx="4" ry="7" transform="rotate(45 34 14)"/><ellipse cx="14" cy="34" rx="4" ry="7" transform="rotate(45 14 34)"/></g></svg>',
    '<svg viewBox="0 0 48 48"><path d="M24 8c10 0 16 7 16 16 0 7-5 12-10 14v4H18v-4C13 36 8 31 8 24 8 15 14 8 24 8z" fill="currentColor"/><circle cx="18" cy="24" r="4" fill="var(--bg)"/><circle cx="30" cy="24" r="4" fill="var(--bg)"/><path d="M19 34h10M24 29v10" stroke="var(--bg)" stroke-width="2" stroke-linecap="round"/></svg>',
    '<svg viewBox="0 0 48 48"><rect x="18" y="16" width="12" height="23" rx="3" fill="currentColor"/><path d="M19 16c1-7 9-7 10 0" fill="var(--pay)"/><path d="M24 39v5M16 44h16" stroke="currentColor" stroke-width="3" stroke-linecap="round"/></svg>'
  ]
};
const THEME_DECOR_MAP={
  christmas:'christmas',halloween:'halloween',thanksgiving:'thanksgiving',
  winter:'winter',spring:'spring',autumn:'autumn',summer:'summer',
  valentine:'valentine',stpatricks:'stpatricks',easter:'easter',newyear:'newyear',
  america:'america',pride:'pride',mardigras:'mardigras',lunar:'lunar',
  oktoberfest:'oktoberfest',earthday:'earthday',cincodemayo:'cincodemayo',
  juneteenth:'juneteenth',muertos:'muertos'
};
function seasonalDecorKind(){
  const theme=String(CURRENT_THEME||document.documentElement.getAttribute('data-theme')||'').toLowerCase();
  return THEME_DECOR_MAP[theme]||'';
}
let _SEASONAL_DECOR_SIGNATURE="";
let _SEASONAL_DECOR_RECONCILE=0;
function seasonalDecorMode(){
  const settings=typeof dashboardRuntimeSettings==="function"?dashboardRuntimeSettings():null;
  return (settings&&settings.seasonalDecor)||CONFIG.seasonalDecor||'off';
}
function seasonalDecorIsLite(){
  if(typeof liteVisualProfile==="function") return liteVisualProfile();
  return ["lite","zero2","low","low-power"].includes(String(CONFIG.profile||"").toLowerCase());
}
function seasonalDecorEnabledForCurrentTheme(){
  const mode=seasonalDecorMode();
  if(mode==='off') return false;
  const kind=seasonalDecorKind();
  const svgs=SEASONAL_DECOR_SVGS[kind];
  return !!(svgs && svgs.length);
}
function seasonalDecorSignature(){
  if(!seasonalDecorEnabledForCurrentTheme())return "off";
  const mode=seasonalDecorMode(),effective=seasonalDecorIsLite()?"subtle":mode;
  return effective+":"+seasonalDecorKind();
}
function clearSeasonalDecor(){
  const root=document.querySelector('#calscroll');
  if(!root) return false;
  root.querySelectorAll('.seasonal-decor').forEach(n=>n.remove());
  _SEASONAL_DECOR_SIGNATURE="off";
  return true;
}
function applySeasonalDecor(){
  const root=document.querySelector('#calscroll');
  if(!root) return false;
  const signature=seasonalDecorSignature();
  clearSeasonalDecor();
  if(signature==='off') return true;
  const mode=seasonalDecorMode();
  const kind=seasonalDecorKind();
  const svgs=SEASONAL_DECOR_SVGS[kind];
  if(!svgs || !svgs.length) return false;
  const lite=seasonalDecorIsLite();
  const effectiveMode=lite ? 'subtle' : mode;
  const cells=[...root.querySelectorAll('.daycell')].filter(c=>{
    if(c.classList.contains('today') || c.classList.contains('other')) return false;
    if(c.dataset.cellLanes && +c.dataset.cellLanes>0) return false;
    const evlist=c.querySelector('.evlist');
    return evlist && !evlist.querySelector('.ev,.more:not(.autofit-hidden)');
  });
  if(!cells.length){_SEASONAL_DECOR_SIGNATURE=signature;return true;}
  const count=Math.min(effectiveMode==='standard'?6:3,cells.length);
  const used=new Set();
  for(let i=0;i<count;i++){
    let idx=Math.round((i+1)*cells.length/(count+1))-1;
    idx=Math.max(0,Math.min(cells.length-1,idx));
    while(used.has(idx) && idx<cells.length-1) idx++;
    used.add(idx);
    const cls='seasonal-decor decor-'+kind+(effectiveMode==='subtle'?' decor-subtle':'')+(lite?' decor-lite':'');
    const d=el('div',cls);
    d.setAttribute('aria-hidden','true');
    d.innerHTML=svgs[i%svgs.length];
    cells[idx].appendChild(d);
  }
  _SEASONAL_DECOR_SIGNATURE=signature;
  return true;
}
// Theme color changes are early-safe; seasonal decoration is reconciled after
// runtime settings exist and only when its identity actually changes. This
// patches decoration nodes without rebuilding the calendar grid.
function reconcileSeasonalDecor(){
  _SEASONAL_DECOR_RECONCILE=0;
  const next=seasonalDecorSignature();
  if(next===_SEASONAL_DECOR_SIGNATURE) return false;
  return next==='off'?clearSeasonalDecor():applySeasonalDecor();
}
function scheduleSeasonalDecorReconcile(){
  if(_SEASONAL_DECOR_RECONCILE) return;
  const run=()=>reconcileSeasonalDecor();
  _SEASONAL_DECOR_RECONCILE=typeof requestAnimationFrame==="function"?requestAnimationFrame(run):setTimeout(run,0);
}
