#!/usr/bin/env bash
# Proves the novice installer no longer needs an ambient Python interpreter.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
INSTALL="$ROOT/../installer/install.sh"
MAIN="$ROOT/cmd/dashboard-control-server/main.go"
[ -f "$INSTALL" ] || { echo 'FAIL: installer missing' >&2; exit 1; }
bash -n "$INSTALL"
if grep -q 'python3' "$INSTALL"; then
  echo 'FAIL: installer still invokes or documents python3' >&2
  exit 1
fi
for required in \
  'installer_cli --geocode --name' \
  'installer_cli --pin-hash' \
  '--json-set' \
  'installer_cli --installer-config-local' \
  'store_optional_trimmed' \
  'Enabling Update the app first' \
  '--json-set' \
  '--pin-hash' \
  '--geocode'; do
  if ! grep -Fq -- "$required" "$INSTALL" "$MAIN"; then
    echo "FAIL: missing Go-owned installer replacement: $required" >&2
    exit 1
  fi
done
printf 'PASS: installer uses Dash-Go Go helpers and has zero python3 references\n'
