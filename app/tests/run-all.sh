#!/usr/bin/env bash
# Dash-Go's single local/CI release gate. It intentionally runs formatting,
# Go verification, shellcheck, every focused shell smoke, and every Node smoke
# from the submitted source tree. A release is not promotable if this exits
# non-zero.
set -euo pipefail
APP_ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
REPO_ROOT="$(CDPATH= cd -- "$APP_ROOT/.." && pwd)"
GO_BIN="${GO:-go}"
NODE_BIN="${NODE:-node}"
TMP="$(mktemp -d "${TMPDIR:-/tmp}/dash-go-tests.XXXXXX")"
trap 'rm -rf "$TMP"' EXIT INT TERM

fail(){ echo "FAIL: $*" >&2; exit 1; }
need(){ command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"; }
need "$GO_BIN"
need "$NODE_BIN"
need shellcheck
GOFMT_BIN="${GOFMT:-$($GO_BIN env GOROOT)/bin/gofmt}"
[ -x "$GOFMT_BIN" ] || fail "gofmt is unavailable next to $GO_BIN"

if [ -z "${DASHGO_CHROMIUM:-}" ]; then
  for browser in google-chrome chromium chromium-browser; do
    if command -v "$browser" >/dev/null 2>&1; then
      export DASHGO_CHROMIUM="$(command -v "$browser")"
      break
    fi
  done
fi
[ -n "${DASHGO_CHROMIUM:-}" ] || fail "a Chromium-family browser is required for browser smoke tests"

mapfile -d '' -t go_files < <(find "$APP_ROOT" -type f -name '*.go' -print0 | sort -z)
[ "${#go_files[@]}" -gt 0 ] || fail "no Go files found"
# gofmt is used directly rather than `go fmt` because the latter may resolve
# packages before reporting a formatting failure.
format_out="$("$GOFMT_BIN" -l "${go_files[@]}")"
[ -z "$format_out" ] || { printf '%s\n' "$format_out" >&2; fail "gofmt reported unformatted files"; }

printf '%s\n' '== Go module, vet, and test gate =='
(
  cd "$APP_ROOT"
  "$GO_BIN" mod verify
  "$GO_BIN" vet ./...
  "$GO_BIN" test -count=1 ./...
  "$GO_BIN" build -o "$TMP/dashboard-control-server" ./cmd/dashboard-control-server
)
export DASHGO_CONTROL_SERVER_BIN="$TMP/dashboard-control-server"

printf '%s\n' '== Shellcheck gate =='
shellcheck "$REPO_ROOT/installer/install.sh" "$APP_ROOT/bin/setup-vdirsyncer.sh"

printf '%s\n' '== Shell smoke gate =='
while IFS= read -r -d '' test_file; do
  printf '%s\n' "  > ${test_file#$APP_ROOT/}"
  bash "$test_file"
done < <(find "$APP_ROOT/tests" -maxdepth 1 -type f -name '*-smoke.sh' -print0 | sort -z)

printf '%s\n' '== Node smoke gate =='
while IFS= read -r -d '' test_file; do
  printf '%s\n' "  > ${test_file#$APP_ROOT/}"
  "$NODE_BIN" "$test_file"
done < <(find "$APP_ROOT/tests" -maxdepth 1 -type f \( -name '*-smoke.mjs' -o -name '*-smoke.js' \) -print0 | sort -z)

printf '%s\n' 'PASS: full Dash-Go source verification gate passed'
