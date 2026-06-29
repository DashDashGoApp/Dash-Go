// 01-config.js — generated from dashboard.js for maintainability.
"use strict";
// Pin date/time formatting to a fixed locale so the Pi renders identically to
// Chrome regardless of the system locale. "en-US" => month/day order, 12-hour
// AM/PM times. Change to your preference (e.g. "en-GB" for day/month).
const LOCALE = "en-US";
/* =====================================================================
   ============================  CONFIG  ===============================
   Edit these and nothing else for normal tweaks.
   ===================================================================== */
const CONFIG = {
  version: "1.5.0-beta.38",
  // ICS files served from the same origin (you SFTP them next to this file).
  // Give each a color + optional name. Add as many as you like.
  calendars: [
    { url: "calendars/work.ics",     name: "Work",      color: "#7fd6a8" },
    { url: "calendars/family.ics",   name: "Family",    color: "#5ab0ff" },
    { url: "calendars/holidays.ics", name: "Holidays",  color: "#5ab0ff", tag:"holiday" },
  ],

  // Weather: Chicago, IL
  lat: 41.8781,
  lon: -87.6298,
  tempUnit: "fahrenheit",     // "fahrenheit" | "celsius"
  windUnit: "mph",            // "mph" | "kmh" | "ms"

  weeksBelow: 10,             // rows below current week
  weeksAbove: 2,              // extra past weeks you can scroll up into (~2 weeks)
  // Calendar density and Agenda length are automatic. The grid keeps a fixed
  // bounded candidate budget and Agenda follows Calendar ahead (weeksBelow × 7).
  calendarAutoFitCandidateCap: 16,
  firstDayOfWeek: 0,          // 0 = Sunday

  refreshCalMinutes: 10,      // automatic ICS refresh; no Control tuning
  refreshWxMinutes: 30,       // automatic profile/provider guard computes the effective cadence
  weatherForecastMaxDays: 16, // providers return as many daily rows as they support, up to this ceiling
  snapBackSeconds: 35,        // idle time before scroll snaps back to today
  // Rotating message timing. complimentSeconds is the minimum dwell time.
  // Longer messages stay on screen a little longer for readability.
  complimentSeconds: 18,
  complimentFadeMs: 750,
  complimentWordThreshold: 10,
  complimentExtraWordStep: 8,
  complimentExtraSecondsPerStep: 2,
  complimentMaxSeconds: 45,

  // Birthdays entered in the installer also appear as all-day calendar events
  // (every year), not just as compliments. Set false to disable.
  showBirthdaysOnCalendar: true,

  // Stale-data warning (bottom-right corner). A synced calendar file that
  // hasn't been rewritten in this many hours gets flagged — this is how a
  // silently-dead secret iCal URL shows up, since the sync cron keeps the
  // last good copy. Holiday-tagged calendars are exempt (they update
  // monthly). Set 0 to disable, e.g. if you copy .ics files on by hand.
  staleCalHours: 48,

  // NWS severe-weather alerts (api.weather.gov — free, keyless, US only).
  // A banner appears while a watch/warning covering your coordinates is
  // active, colored by severity; tap it for the full alert text. Outside the
  // US the request simply finds nothing and the banner stays hidden.
  // minSeverity: "extreme" | "severe" | "moderate" | "minor" — "moderate"
  // keeps watches/warnings and meaningful advisories, drops routine ones.
  weatherAlerts: { enabled: true, refreshMinutes: 5, minSeverity: "moderate" },

  clock24: true,   // big clock format; false = 12-hour with AM/PM. The control
                   // overlay can flip this (and event-time formats follow).
  // Weather and air-quality endpoints. Forecast providers are selected in the
  // installer. NWS is also available as an optional US-only forecast source
  // with unsupported-location cooldown, and remains the severe-alert source.
  wxApi: "https://api.open-meteo.com",
  aqApi: "https://air-quality-api.open-meteo.com",
  nwsApi: "https://api.weather.gov",
  apiKey: "",        // legacy Open-Meteo-compatible key
  // Multiple weather forecast sources can be selected by the installer.
  // Open-Meteo remains the default; NWS/NOAA can be added for US coordinates.
  weatherProviders: ["openmeteo"],
  weatherProviderKeys: {},
  weatherPrimary: "average",

  // Radar is intentionally on-demand: triple-tap the current-weather panel.
  // Keys, when a paid/metered provider needs them, stay in ~/.dashboard-radar.env.
  radarProvider: "auto",
  // RainViewer uses every observed frame returned by its public timeline. The
  // renderer remains profile-safe and user-facing frame/render tuning is retired.
  radarCustomTiles: "", // advanced public HTTPS {z}/{x}/{y} template
  radarCustomWms: "",   // advanced public HTTPS template; never proxied

  pauseWhileOpen: true, // freeze the clock + message rotation while the
                        // control panel is up: those updates are hidden
                        // behind it anyway, and skipping them keeps panel
                        // taps snappy on low-power boards. Set false in
                        // config.local.js to keep them running.
  showSeconds: true, // retained user-facing clock preference; all profiles default on
  weatherDetailMode: "expanded", // "standard" | "expanded"; expanded includes UV and air quality
  showUV: true,    // derived from weatherDetailMode for existing renderer helpers
  showAQI: true,   // derived from weatherDetailMode for existing renderer helpers
  showEventMaps: true, // static event-map previews are always available
  showInteractiveMaps: false, // higher-performance optional: tap event location to open full-screen Google Maps
  mapImageStyle: "standard", // standard or hybrid; popup can temporarily switch styles

  // Lightweight visual style options controlled from Dashboard Control.
  // These use local/system fonts and inline SVG variants only — no network
  // font loading or heavy image assets.
  fontPreset: "default",
  weatherIconStyle: "soft",
  seasonalDecor: "off",

  // Burn-in mitigation is always on: nudge the UI by ±2px around a slow
  // minute-by-minute cycle. It is intentionally not a household tuning choice.
  pixelShift: 2,

  // Night dimming: reduce panel brightness between start and end hours
  // (24h clock; range may wrap midnight). level = brightness 0..1;
  // 1 or null disables. Dim hours: 11 PM to 4 AM.
  nightDim: { start: 23, end: 4, level: 0.45 },

  // ---- COMPLIMENTS ----
  // Each entry: { text, when?, weight?, share?, date?, holiday? }
  //   text   : the phrase. "%holiday%" is replaced with today's holiday name.
  //   when   : array of time-of-day buckets it may show in. Omit = anytime.
  //            buckets: "earlymorning" "morning" "afternoon" "evening"
  //                     "night" "latenight"
  //   weight : relative likelihood (default 1). Higher = shows more often.
  //   share  : target fraction of picks in its eligible pool (e.g. 0.33 =
  //            ~33%). Auto-balanced regardless of how many others are eligible,
  //            so it stays on target as you add phrases. Overrides weight.
  //   date   : "MM-DD" — only eligible on that calendar date.
  //   holiday: true — only eligible when today is a holiday (from holidays.ics)
  // Time buckets (local time):
  //   latenight  00:00–03:59   (runs until 4am)
  //   earlymorning 04:00–07:59
  //   morning    08:00–11:59
  //   afternoon  12:00–16:59
  //   evening    17:00–20:59
  //   night      21:00–23:59
  compliments: [],

  // Treat events whose title matches as special styles
  payMatch: /pay\s?day/i,
};
