#!/usr/bin/env bash
# Release-blocking source contract for the first native Dash-Go Showcase
# handoff. The local Builder owns final package construction; this source gate
# proves the compiled server can read the shipped declaration from its app root.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
SERVER="${DASHGO_CONTROL_SERVER_BIN:-}"
CONTRACT="$ROOT/release/showcase-contract.json"
DOC="$ROOT/SHOWCASE_CONTRACT.md"
MAIN="$ROOT/cmd/dashboard-control-server/main.go"
[ -x "$SERVER" ] || { echo 'FAIL: DASHGO_CONTROL_SERVER_BIN must name the run-all compiled server' >&2; exit 1; }
for file in "$CONTRACT" "$DOC" "$MAIN"; do
  [ -f "$file" ] || { echo "FAIL: missing Showcase Contract v1 release component: $file" >&2; exit 1; }
done
need(){ grep -Fq -- "$2" "$1" || { echo "FAIL: missing $3" >&2; exit 1; }; }
need "$CONTRACT" '"contract": "dashgo-showcase/v1"' 'native contract identifier'
need "$CONTRACT" '"statusEndpoint": true' 'status readiness capability'
need "$DOC" 'GET /api/showcase/status' 'native status contract documentation'
need "$MAIN" 'case "--showcase-contract":' 'contract probe CLI entry'
out="$(cd "$ROOT" && env -u DASHGO_RUNTIME_PROFILE -u DASHGO_SHOWCASE_MANIFEST -u DASHGO_DATA_ROOT "$SERVER" --showcase-contract)"
printf '%s\n' "$out" | grep -Fq '"contract":"dashgo-showcase/v1"' || { echo "FAIL: compiled server did not report Contract v1: $out" >&2; exit 1; }
printf '%s\n' "$out" | grep -Fq '"statusEndpoint":true' || { echo "FAIL: compiled server omitted Contract v1 status capability: $out" >&2; exit 1; }
printf '%s\n' 'PASS: Dash-Go Showcase Contract v1 declaration is shipped and readable by the compiled control server'
