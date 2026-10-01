import assert from "node:assert/strict";
import fs from "node:fs";

const read=(path)=>fs.readFileSync(path,"utf8");
const featureContract=JSON.parse(read("release/source-features.json"));
const releaseContract=JSON.parse(read("release/release.json"));
const version=read("VERSION").trim();
const expected=[
  "scoped-settings-invalidation-v1",
  "focused-jsonv2-gate-v1",
  "typed-json-boundaries-v1",
  "server-aqi-cache-v1",
  "same-origin-weather-boundary-v1",
  "nonblocking-aqi-status-v1",
  "partial-weather-provider-results-v1",
  "provider-location-cache-v1",
  "canonical-weather-provider-cache-v1",
  "calendar-source-diagnostics-v1",
  "calendar-snapshot-v2",
  "calendar-parser-parity-v1",
  "calendar-periodic-rehash-v1",
  "operation-limiter-post-validation-v1",
  "security-host-policy-v2",
  "rooted-static-serving-v1",
  "narrowed-csp-v1",
  "data-bearing-innerhtml-hardening-v1",
  "seasonal-decor-clearance-v1",
  "display-sleep-state-machine-v2",
  "route-domain-split-v1",
  "pin-guidance-v1",
  "sha256-local-identifiers-v1",
  "repository-source-handoff-v2",
  "message-provider-health-v1",
  "message-history-and-trivia-v1",
  "message-refresh-budget-v1",
  "weather-hourly-providers-v1",
  "weather-hourly-horizon-v1",
  "weather-sun-derivation-v1",
  "weather-dead-alerts-removal-v1",
  "oauth-loopback-test-stability-v1",
  "weather-feels-guard-v1",
  "weather-hourly-local-clock-v1",
  "weather-provider-key-redaction-v1",
  "scroll-gesture-aware-restore-v1",
  "scroll-settle-coalescing-v1",
  "popup-live-data-refresh-v1",
  "app-calendar-popup-prefetch-v1",
  "popup-list-staging-v1",
  "popup-latency-budget-v1",
  "aqi-failure-cooldown-v1",
  "weather-review-notes-v1",
];

assert.equal(featureContract.schema,1);
assert.equal(featureContract.version,version);
assert.equal(releaseContract.version,version);
assert.deepEqual(featureContract.requiredSourceFeatures,expected,"required source feature list changed or lost ordering");

const settingsRuntime=read("ui/js/settings-runtime.js");
assert.match(settingsRuntime,/DASHBOARD_SETTINGS_IMPACT/);
assert.match(settingsRuntime,/dashboardSettingsImpact/);

const runAll=read("tests/run-all.sh");
assert.match(runAll,/GOEXPERIMENT=jsonv2/);
const todoTypes=read("internal/todo/todo_types.go");
assert.match(todoTypes,/type todoDateTimeTimeZone struct/);
assert.match(todoTypes,/json:"_syncFailed,omitzero"/);

const aqi=read("internal/weather/weather_aqi.go");
assert.match(aqi,/weatherAQICacheTTL = 30 \* time\.Minute/);
assert.match(aqi,/func \(s \*Service\) aqiPayload/);
const weatherSources=read("ui/js/weather-sources.js");
assert.match(weatherSources,/fetch\("\/api\/weather"/);
assert.doesNotMatch(weatherSources,/https:\/\//,"browser Weather source adapter must remain same-origin only");
const weatherUI=read("ui/js/weather.js");
assert.match(weatherUI,/fetch\("\/api\/weather\/aqi"/);
assert.match(weatherUI,/refreshAQINonblocking\(0\)/);

const partial=read("internal/weather/weather_partial.go");
assert.match(partial,/_partialErrors/);
assert.match(partial,/weatherParallelCalls/);
assert.match(read("internal/weather/weather_location_cache.go"),/weatherLocationCacheRead/);
const providerCache=read("internal/weather/weather_provider_cache.go");
assert.match(providerCache,/parts := struct \{/);
assert.doesNotMatch(providerCache,/TempUnit\s+string/);
assert.match(read("internal/weather/weather_units.go"),/weatherCanonicalFetchConfigGo/);

const eventTypes=read("internal/calendar/events/types.go");
assert.match(eventTypes,/type SourceDiagnostics struct/);
assert.match(eventTypes,/const CacheVersion = 11/);
assert.match(eventTypes,/const FingerprintVersion = 9/);
const eventCache=read("ui/js/event-cache.js");
assert.match(eventCache,/LAST_KNOWN_EVENTS_SCHEMA=2/);
assert.match(eventCache,/persist:false,source:"last-known"/);
assert.ok(fs.existsSync("tests/fixtures/calendar-parity/corpus.json"));
assert.match(read("internal/calendar/events/parser_parity_test.go"),/calendar-parity/);
assert.match(read("internal/calendar/events/sources.go"),/fullRehashInterval/);

const limiter=read("cmd/dashboard-control-server/request_security.go");
assert.match(limiter,/type operationLease struct/);
assert.match(limiter,/if lease\.started/);
assert.doesNotMatch(read("cmd/dashboard-control-server/http_server.go"),/operationLimiter\.begin/);
const outbound=read("internal/weather/outbound_security.go");
assert.match(outbound,/net\/netip/);
assert.match(outbound,/100\.64\.0\.0\/10/);
assert.match(outbound,/198\.18\.0\.0\/15/);
assert.match(outbound,/64:ff9b:1::\/48/);
assert.match(outbound,/2002::\/16/);
assert.match(outbound,/3fff::\/20/);
assert.doesNotMatch(read("internal/maps/maps_render.go"),/http\.ServeFile/);
assert.match(read("internal/maps/maps_render.go"),/os\.OpenRoot/);

const server=read("cmd/dashboard-control-server/http_static.go");
assert.match(server,/connect-src 'self' https:\/\/api\.rainviewer\.com/);
assert.doesNotMatch(server,/connect-src 'self' https:;/);
const weatherIcons=read("ui/js/weather-icons.js");
assert.doesNotMatch(weatherIcons,/innerHTML=`<[^`]*\$\{(?:escapeHTML\()?[^`]+/,
  "provider-derived Weather text must not be interpolated through innerHTML");
assert.match(read("ui/js/control-visual-style.js"),/event days with enough clear space/);
const displaySleep=read("ui/js/settings-display-sleep.js");
assert.match(displaySleep,/DISPLAY_SLEEP_RECONCILE_PENDING/);
assert.match(displaySleep,/DISPLAY_SLEEP_WAKE_OVERRIDE/);
assert.match(displaySleep,/DISPLAY_SLEEP_REQUEST_SERIAL/);
assert.match(displaySleep,/bindDisplayWakeReconciliation/);
assert.ok(fs.existsSync("tests/display-sleep-state-machine-smoke.mjs"));
assert.match(read("ui/js/boot.js"),/DISPLAY_SLEEPING/);
assert.ok(fs.existsSync("cmd/dashboard-control-server/http_routes_post_operations.go"));
assert.ok(fs.existsSync("cmd/dashboard-control-server/http_routes_post_presentation.go"));
assert.match(read("ui/js/control-location-lock.js"),/For a new PIN, use 6–8 digits when practical/);
assert.doesNotMatch(read("internal/calendar/events/serialize.go"),/sha1/i);
assert.match(read("internal/calendar/events/serialize.go"),/sha256\.Sum256/);
assert.ok(fs.existsSync("tests/source-handoff-layout-smoke.sh"));

// 1.5.13: rotating-message provider health, history and trivia, and the refresh
// budget.
assert.ok(fs.existsSync("internal/messages/messages_providers_history.go"),"the history and trivia adapter was removed");
assert.match(read("internal/messages/messages_core.go"),/"history", "This day in history"/);
assert.match(read("internal/messages/messages_core.go"),/"trivia", "Trivia"/);
assert.match(read("internal/messages/messages_providers_quotes.go"),/api\.quotable\.kurokeita\.dev/);
assert.doesNotMatch(read("internal/messages/messages_core.go"),/numbersapi_https/);
assert.match(read("internal/messages/messages_refresh.go"),/messageRefreshBudget = 25 \* time\.Second/);
assert.ok(fs.existsSync("tests/message-source-catalog-smoke.mjs"));

// 1.5.13: hourly from every capable provider, one bounded horizon, derived
// sunrise/sunset, and no dead alerts field.
assert.ok(fs.existsSync("internal/weather/weather_hourly.go"));
const hourly=read("internal/weather/weather_hourly.go");
assert.match(hourly,/weatherHourlyMaxRows = 72/);
assert.match(hourly,/func weatherHourlyForProviderGo/);
for(const id of ["openweather","tomorrow","pirateweather","weatherapi","visualcrossing","weatherbit","accuweather","xweather","googleweather"]){
  assert.match(hourly,new RegExp(`"${id}":`),`hourly mapper for ${id} is missing`);
}
assert.match(read("internal/weather/weather_keyed_nws.go"),/forecastHourly/);
assert.match(read("internal/weather/weather_openmeteo.go"),/trimOpenMeteoHourlyGo/);
assert.match(read("ui/js/weather-blend.js"),/WEATHER_HOURLY_MAX_ROWS=72/);
assert.match(read("ui/js/weather-blend.js"),/hourly:hourly\.stats/);
assert.match(read("internal/weather/weather_sun.go"),/weatherSunTimesForDay/);
assert.match(read("internal/weather/weather_blend_go.go"),/fillDerivedSunTimesGo/);
assert.doesNotMatch(read("internal/weather/weather_blend_go.go"),/mergeAlertsGo/);
assert.doesNotMatch(read("internal/weather/weather_payload.go"),/"alerts":/);
assert.match(read("cmd/dashboard-control-server/google_oauth_loopback_test.go"),/loopbackTestServerWait/);
assert.match(read("cmd/dashboard-control-server/google_oauth_loopback_test.go"),/loopbackTestPollDeadline/);

console.log(`PASS: ${version} required source-feature contract (${expected.length} features)`);
