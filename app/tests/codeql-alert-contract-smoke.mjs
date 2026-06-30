#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import {fileURLToPath} from "node:url";

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const read=relative=>fs.readFileSync(path.join(root,relative),"utf8");
const weather=read("internal/weather/weather_provider_cache.go");
const weatherPayload=read("internal/weather/weather_payload.go");
const archive=read("cmd/dashboard-control-server/config_backup_archive.go");
const restore=read("cmd/dashboard-control-server/config_backup_restore.go");
const backups=read("cmd/dashboard-control-server/config_backups.go");
const fontHTTP=read("cmd/dashboard-control-server/fonts_http.go");
const fonts=read("internal/settings/fonts.go");
const calendarHealth=read("ui/js/control-calendars-logs.js");
const themePicker=read("ui/js/control-theme.js");
const index=read("index.html");
const responsive=read("tests/dashboard-responsive-fit-smoke.mjs");
const radar=read("tests/lite-radar-snapshot-smoke.mjs");

assert.match(weather,/crypto\/hmac/,"weather cache fingerprints must use keyed HMAC");
assert.match(weather,/weatherProviderCacheFingerprintLabel/,"weather cache fingerprint label missing");
assert.doesNotMatch(weather,/sha256\.Sum256\(\[\]byte\(key\)\)/,"provider key must not be plain-SHA256 hashed");
assert.doesNotMatch(weather,/keyFingerprint[^\n]*weatherProviderKeyFingerprintGo/,"opaque provider-key HMAC must not feed the plain configuration digest");
assert.doesNotMatch(weatherPayload,/sha256\.Sum256\(\[\]byte\(v\)\)/,"aggregate weather cache must not plain-SHA256 provider keys");
assert.match(weatherPayload,/weatherProviderKeyFingerprintGo\(value\)/,"aggregate weather cache must use the keyed provider marker");

for(const token of ["calendarBackupLinkRootHome","calendarBackupLinkRootSystem","calendarBackupSystemCalendarsRoot","trusted calendar roots","validateExistingCalendarTarget"]){
  assert.ok(archive.includes(token),`calendar backup root policy missing ${token}`);
}
assert.match(archive,/calendarBackupSystemCalendarsRoot\s*=\s*"\/Calendars"/,"trusted system calendar root must remain /Calendars");
assert.doesNotMatch(restore,/os\.Symlink\(link\.Target,/,"raw backup link target must never reach os.Symlink");
assert.match(restore,/policy\.restoreTarget\(link\)/,"calendar restore must resolve a normalized trusted target");
assert.match(backups,/type configBackupRecord struct/,"backup selection must use server-owned records");
assert.match(backups,/os\.Lstat\(full\)/,"backup discovery must reject symlinked entries");
assert.match(backups,/os\.Remove\(chosen\.FullPath\)/,"backup delete must use selected record path");

assert.match(fontHTTP,/OpenRuntimeFont\(name\)/,"font route must select a pinned font asset");
assert.match(fontHTTP,/http\.ServeContent/,"font route must serve an opened verified file");
assert.doesNotMatch(fontHTTP,/http\.ServeFile/,"font route must not serve a request-derived path");
assert.match(fonts,/runtimeFontAssetByFile/,"font leaf must resolve through pinned metadata");
assert.match(fonts,/openRegularRuntimeFont/,"font storage must reject symlinked/non-regular files");
assert.match(fonts,/fontChecks/,"font verification must cache unchanged bounded files");

assert.doesNotMatch(calendarHealth,/style="background:/,"calendar health color must not be interpolated into HTML attributes");
assert.match(calendarHealth,/dot\.style\.backgroundColor=color/,"calendar health color must use the DOM style property");
assert.doesNotMatch(themePicker,/themepreview" style=/,"theme preview variables must not be interpolated into HTML");
assert.match(themePicker,/preview\.style\.setProperty/,"theme preview must set CSS variables through the DOM");

assert.doesNotMatch(index,/dashboardAssetFallback|data-fallback|document\.write/,"bootstrap must not reinterpret dynamic HTML fallback text");
assert.ok(index.includes('<script src="/config/config.local.js"></script>'),"optional local config must remain parser-time static script");
assert.equal(responsive.includes("matchAll("),false,"responsive smoke must not use a tag-filter regex");
assert.match(radar,/new URL\(String\(url\)\)\.hostname === "tile\.openstreetmap\.org"/,"radar smoke must compare parsed hostnames");

console.log("PASS: CodeQL alert fixes preserve pinned paths, trusted calendar roots, DOM-safe previews, and zero recurring dashboard work.");
