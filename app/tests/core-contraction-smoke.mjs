#!/usr/bin/env node
// Beta.19 regression contract: Dashboard Control auth runtime
// state belongs to internal/auth; core keeps only a narrow facade plus actual
// update/release and HTTP/SSE orchestration state.
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import {fileURLToPath} from "node:url";

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const read=rel=>fs.readFileSync(path.join(root,rel),"utf8");
const main=read("cmd/dashboard-control-server/main.go");
const facade=read("cmd/dashboard-control-server/auth_facade.go");
const service=read("internal/auth/service.go");
const config=read("internal/auth/config.go");
const sessions=read("internal/auth/sessions.go");
const auth=service+"\n"+config+"\n"+sessions;

assert.match(main,/authInitMu\s+sync\.Mutex[\s\S]*?auth\s+\*controlauth\.Service/,"core must retain only lazy auth-service construction state");
for(const retired of ["sessions                map[string]sessionMeta","oneShots                map[string]oneShotMeta","failTimes               []time.Time","mu                      sync.Mutex","controlEnv              string"]){
  assert.ok(!main.includes(retired),`core must not retain migrated auth state: ${retired}`);
}
for(const required of ["updateMu","updateAvailabilityMu","todoStreamMu","todoStreams","releaseVersion"]){
  assert.ok(main.includes(required),`core must retain actual orchestration/transport state: ${required}`);
}
assert.match(facade,/func \(a \*app\) authService\(\) \*controlauth\.Service/,"core must construct auth from the narrow facade");
assert.match(facade,/func \(a \*app\) controlEnvPath\(\) string/,"the private control env path must be derived rather than retained as mutable app state");
for(const token of ["EnvPath","Config","SetPIN","RemovePIN","IssueToken","TokenOK","IssueOneShot","ConsumeOneShot","PINLockoutRemaining"]){
  assert.ok(auth.includes(token),`internal/auth must own ${token}`);
}
assert.match(service,/configMu\s+sync\.RWMutex[\s\S]*?mu\s+sync\.Mutex[\s\S]*?sessions\s+map\[string\]sessionMeta[\s\S]*?oneShots\s+map\[string\]oneShotMeta[\s\S]*?lockout\s+pinLockoutState/,"auth must own its credential lock plus bounded session and persistent-lockout state");
assert.match(service,/lockoutPath\s+string/,"auth must retain its private persistent-lockout path");
assert.doesNotMatch(auth,/\*app\b|cmd\/dashboard-control-server|package main/,"internal/auth must not depend on core");
for(const retired of ["auth_pin.go","auth_session.go"]){
  assert.ok(!fs.existsSync(path.join(root,"cmd/dashboard-control-server",retired)),`retired core auth implementation must be absent: ${retired}`);
}
console.log("PASS: beta.19 keeps core auth ownership structural and formatter-independent");
