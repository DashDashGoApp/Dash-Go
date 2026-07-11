import assert from "node:assert/strict";
import fs from "node:fs";

const ui=fs.readFileSync("ui/js/control-location-lock.js","utf8");
const auth=fs.readFileSync("internal/auth/auth.go","utf8");

assert.match(ui,/Existing 4-digit PINs remain supported\./,"new-PIN guidance must preserve legacy four-digit compatibility");
assert.match(ui,/For a new PIN, use 6–8 digits when practical\./,"new-PIN guidance must recommend a stronger length");
assert.match(auth,/len\(pin\) < 4 \|\| len\(pin\) > 8/,"server must continue accepting existing four-digit PINs through eight digits");

console.log("PASS: PIN guidance recommends 6–8 digits without breaking existing 4-digit PINs");
