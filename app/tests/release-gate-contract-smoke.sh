#!/usr/bin/env bash
# Keep the build/test gate discoverable and release-blocking rather than an
# undocumented one-off. The source archive must carry its runner, CI workflow,
# and maintainer instruction together.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
REPO="$(CDPATH= cd -- "$ROOT/.." && pwd)"
RUNNER="$ROOT/tests/run-all.sh"
WORKFLOW="$REPO/.github/workflows/verify.yml"
RELEASING="$REPO/RELEASING.md"
for file in "$RUNNER" "$WORKFLOW" "$RELEASING"; do
  [ -f "$file" ] || { echo "FAIL: missing release gate component: $file" >&2; exit 1; }
done
bash -n "$RUNNER"
need(){ grep -Fq -- "$2" "$1" || { echo "FAIL: missing $3" >&2; exit 1; }; }
need "$RUNNER" 'GOFMT_BIN' 'gofmt gate'
need "$RUNNER" 'mod verify' 'Go module verification'
need "$RUNNER" 'vet ./...' 'Go vet gate'
need "$RUNNER" 'test -count=1 ./...' 'Go test gate'
need "$RUNNER" 'shellcheck' 'shellcheck gate'
need "$RUNNER" "-name '*-smoke.sh'" 'shell smoke discovery'
need "$RUNNER" "-name '*-smoke.mjs'" 'Node smoke discovery'
need "$WORKFLOW" 'actions/checkout@v5' 'checkout action'
need "$WORKFLOW" 'actions/setup-go@v6' 'Go setup action'
need "$WORKFLOW" 'app/tests/run-all.sh' 'CI gate invocation'
need "$RELEASING" 'app/tests/run-all.sh' 'release checklist gate requirement'
need "$RELEASING" '--bootstrap-selector-integrity' 'beta.6 bridge release instruction'
printf '%s\n' 'PASS: full local/CI gate and beta.6 migration requirement remain release-documentation contracts'
