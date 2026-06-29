#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import {fileURLToPath} from "node:url";

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const read=p=>fs.readFileSync(path.join(root,p),"utf8");
const health=read("ui/js/display-health.js");
const actions=read("ui/js/health-warning-actions.js");
const schedule=read("ui/js/messages-schedule.js");
const feeds=read("ui/js/control-special-feeds.js");
const html=read("index.html");
const footerCss=read("ui/css/dashboard/sidebar-weather-messages.css")+read("ui/css/dashboard/control-theme-actions-lite.css");
const popupCss=read("ui/css/dashboard/popups-alerts-maps.css");
const overlays=read("ui/js/popup-overlays.js");
const taps=read("ui/js/tap.js");
const server=read("internal/platform/silences.go")+read("cmd/dashboard-control-server/platform_facade.go")+read("cmd/dashboard-control-server/http_routes_public_post.go");
const deviceHealth=read("internal/platform/health.go")+read("internal/platform/health_storage.go");

for(const token of ["ACTIVE_STALE_WARNINGS","healthMonitorsMessageFeeds","healthWarningMuted","staleWarning","MESSAGE_CHECK_SOON_MS","warningSilences","warningKeys","silenceableWarningKeys","deviceHealthWarningEntries","DEVICE_WARNING_SILENCEABLE_KEYS"]){
  assert.ok(health.includes(token),`missing structured health warning contract ${token}`);
}
assert.match(health,/if\(healthMonitorsMessageFeeds\(\) && lastMessageOK && now-lastMessageOK>MESSAGE_CHECK_SOON_MS\)/,"messages must require current health/source-selection state");
assert.ok(!health.includes('if(lastMessageOK && now-lastMessageOK>msgMax) parts.push'),"legacy unguarded message-cache footer warning must be removed");
assert.match(schedule,/feed\.lastSuccessAt\|\|feed\.generatedAt/,"frontend cache freshness must prefer lastSuccessAt");
assert.match(feeds,/setMessageFeedFreshnessEnabled\(confirmed\)[\s\S]*updateStale\(\)/,"source save must update message warning eligibility immediately");
assert.match(html,/<button id="stale" type="button" hidden aria-live="polite"/,"footer warning must be accessible/actionable");
for(const token of ["bindTripleTap(pill,openHealthWarningSilencePopup,700", "moveTol:28", "HEALTH_WARNING_SILENCE_DURATIONS", "minutes:15", "1 hour", "24 hours", "Silence this notice for:", "/api/health/warnings/silence", "Could not save the temporary silence. The notice is still active."]){
  assert.ok(actions.includes(token),`missing bounded silence interaction ${token}`);
}
assert.match(actions,/entry&&entry\.tier==="device"/,"device notices need an explanatory temporary-silence treatment");
assert.match(actions,/critical device failures remain visible/,"critical device failures must remain visible");
assert.ok(!actions.includes("Dismiss forever"),"temporary silence must never offer a permanent dismissal");
assert.ok(!actions.includes("Until reboot"),"temporary silence must remain bounded");
for(const token of ["min-width:44px","min-height:44px","healthwarning-duration-grid","grid-template-columns:repeat(2,minmax(0,1fr))","min-height:48px"]){
  assert.ok((footerCss+popupCss).includes(token),`missing touch/layout treatment ${token}`);
}
assert.ok(overlays.includes('"healthwarningpop"'),"shared popup lifecycle must clear health-warning mode");
assert.match(taps,/const clusterTol=\(\)=>\{/,"tap primitive must keep cluster tolerance distinct from release movement");
assert.match(taps,/Math\.hypot\(px-lastX,py-lastY\)<=clusterTol\(\)/,"wide triple-tap targets must cluster using their own footprint");
assert.match(taps,/const validRelease=.*<=moveTol/,"tight swipe rejection must stay on moveTol");
for(const token of ["allowedWarningSilenceKeys", "\"messages\":", "\"calendar\":", "\"weather\":", "\"storage\":", "allowedWarningSilenceMinutes", "minutes must be 15, 60, 240, 720, or 1440", "body[\"hours\"]", "/api/health/warnings/silence"]){
  assert.ok(server.includes(token),`missing narrow server persistence/route contract ${token}`);
}
assert.ok(!server.includes('"device": true'),"generic device health must never be accepted as a silence key");
assert.ok(server.includes('fact.Tier == "device"')||server.includes('fact.Tier=="device"'),"only device facts may be considered for silencing");
assert.ok(server.includes('return fact.Level == "degraded"')||server.includes('return fact.Level=="degraded"'),"only degraded device facts may be silenced");
assert.match(deviceHealth,/StorageKernelWarningPredatesBoot/,"storage health must suppress stale prior-boot kernel evidence only");
assert.match(deviceHealth,/kernelErrorsCurrentBoot/,"storage stale suppression must retain explicit current-boot proof");
assert.ok(deviceHealth.includes('fact.Tier != "device" || HealthRank(fact.Level) < 2')||deviceHealth.includes('fact.Tier!="device"||HealthRank(fact.Level)<2'),"status line must be selected only from actionable device facts");
console.log("PASS: health-warning silences remain bounded while critical health visibility is preserved");
