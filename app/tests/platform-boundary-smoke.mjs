#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import {fileURLToPath} from "node:url";
const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const read=rel=>fs.readFileSync(path.join(root,rel),"utf8");
const platformFiles=["internal/platform/service.go","internal/platform/terminal.go","internal/platform/system.go","internal/platform/health.go","internal/platform/health_storage.go","internal/platform/silences.go","internal/platform/cache.go","internal/platform/diagnostics.go","internal/platform/doctor.go"];
const platform=platformFiles.map(read).join("\n");
const main=read("cmd/dashboard-control-server/main.go");
const facade=read("cmd/dashboard-control-server/platform_facade.go");
const updates=read("cmd/dashboard-control-server/updates.go")+read("cmd/dashboard-control-server/update_status.go");
for(const rel of platformFiles){const source=read(rel);assert.match(source,/^package platform\b/m,`${rel} must belong to the platform service`);const body=source.slice(source.indexOf("package platform"));assert.doesNotMatch(body,/\*app\b/,`${rel} must not receive the core app`);assert.doesNotMatch(body,/cmd\/dashboard-control-server/,`${rel} must not import the core executable`);}
for(const token of ["TerminalAccessFile","DeviceHealth","SystemStatus","BuildDiagnostics","RunDoctorDataCLI","WarningSilence"]){assert.ok(platform.includes(token),`platform boundary missing ${token}`);}
assert.match(main,/platformInitMu\s+sync\.Mutex[\s\S]*?platform\s+\*platformpkg\.Service/,"core must own only lazy platform-service construction state");
assert.match(facade,/func \(a \*app\) platformService\(\) \*platformpkg\.Service/,"core must construct platform with explicit narrow ports");
for(const port of ["CalendarEntries","MapCacheStatus","SystemUpdateStatus","WeatherDoctorFindings"]){assert.ok(facade.includes(port+":"),`platform construction must declare ${port} callback explicitly`);}
assert.match(updates,/func \(a \*app\) systemUpdateStatus\(/,"update/release status must remain core-owned");
for(const old of ["terminal_access.go","terminal_manager.go","system_memory.go","cache_status.go","device_health.go","device_health_storage.go","health_warning_silences.go","diagnostics.go","doctor_data.go","system_status.go"]){assert.ok(!fs.existsSync(path.join(root,"cmd/dashboard-control-server",old)),`retired core platform implementation must be absent: ${old}`);}
console.log("PASS: platform services own terminal, health, diagnostics, doctor, cache, and status state while update orchestration remains core-owned");
