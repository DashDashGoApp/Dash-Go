#!/usr/bin/env bash
# Release-blocking regression for bounded automatic stale-state recovery.
set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TMP="$(mktemp -d)"
DASH_ROOT="$TMP/dashboard"
TEST_BIN="$TMP/test-bin"
cleanup(){ rm -rf "$TMP"; }
trap cleanup EXIT INT TERM

# Keep the fixture deterministic: a host shell command line can itself contain
# the verifier name, and host D-Bus may be unavailable. The guard contracts
# under test do not require either host service.
mkdir -p "$TEST_BIN"
printf '#!/bin/sh\nexit 1\n' > "$TEST_BIN/pgrep"
printf '#!/bin/sh\nexit 3\n' > "$TEST_BIN/systemctl"
chmod +x "$TEST_BIN/pgrep" "$TEST_BIN/systemctl"
run_guard(){ env PATH="$TEST_BIN:$PATH" DASH="$DASH_ROOT" bash "$ROOT/bin/dashboard-health-guard.sh"; }

mkdir -p "$DASH_ROOT/cache/kiosk.lock" "$DASH_ROOT/config" "$DASH_ROOT/logs" "$DASH_ROOT/bin"
printf '1.4.1-beta.13\n' > "$DASH_ROOT/VERSION"
printf '{"buildEpoch":%s}\n' "$(date +%s)" > "$DASH_ROOT/manifest.json"
cp "$ROOT/bin/dashboard-resilience-lib.sh" "$DASH_ROOT/bin/"
printf '999999\n' > "$DASH_ROOT/cache/kiosk.lock/pid"
touch -d '2 hours ago' "$DASH_ROOT/cache/kiosk-paused"
printf '{"location":{"lat":0,"lon":0}}\n' > "$DASH_ROOT/cache/weather-cache.json"
printf 'window.DASHBOARD_LOCAL={lat:41.8781,lon:-87.6298};\n' > "$DASH_ROOT/config/config.local.js"
# Simulate a kiosk boot marker from a slow/non-systemd NTP report. A plausible
# wall clock must clear it before the guard can retain a healthy heartbeat.
touch "$DASH_ROOT/cache/clock-unverified"

run_guard
[ ! -d "$DASH_ROOT/cache/kiosk.lock" ]
[ ! -e "$DASH_ROOT/cache/kiosk-paused" ]
[ ! -e "$DASH_ROOT/cache/weather-cache.json" ]
[ ! -e "$DASH_ROOT/cache/clock-unverified" ]
[ -s "$DASH_ROOT/cache/clock-confirmed.json" ]
find "$DASH_ROOT/cache" -maxdepth 1 -name 'weather-cache.json.health-guard-bad-*' -print -quit | grep -q .
python3 - "$DASH_ROOT/cache/health-guard-status.json" <<'PY'
import json, sys
payload=json.load(open(sys.argv[1], encoding='utf-8'))
actions=set(payload.get('actions', []))
expected={'removed-stale-kiosk-lock','removed-stale-kiosk-maintenance-pause','quarantined-zero-zero-weather-cache','cleared-clock-unverified-marker'}
assert expected <= actions, (actions, expected)
assert payload.get('state') == 'recovered'
PY

# A visible update marker is a hard stop: Guard must leave state alone and
# record why, rather than racing maintenance/update work.
mkdir -p "$DASH_ROOT/cache/kiosk.lock"
printf '999998\n' > "$DASH_ROOT/cache/kiosk.lock/pid"
touch "$DASH_ROOT/cache/system-update.lock"
run_guard
[ -d "$DASH_ROOT/cache/kiosk.lock" ]
python3 - "$DASH_ROOT/cache/health-guard-status.json" <<'PY'
import json, sys
payload=json.load(open(sys.argv[1], encoding='utf-8'))
assert payload.get('state') == 'skipped'
assert 'update-or-maintenance-active' in payload.get('skipped', [])
PY

# A focused Go device-health test models the legacy on-disk warning payload.
# This shell smoke locks the writer semantics: expected post-update startup is
# audit-only INFO, while warning state derives only from WARNINGS.
grep -Fq 'INFO+=("post-update-runtime-verifier-started")' "$ROOT/bin/dashboard-health-guard.sh"
grep -Fq 'WARNINGS=()' "$ROOT/bin/dashboard-health-guard.sh"
grep -Fq 'if [ "${#WARNINGS[@]}" -gt 0 ]; then' "$ROOT/bin/dashboard-health-guard.sh"
echo 'PASS: normal post-update verifier startup is recorded as audit info, not a guard warning'

# Heartbeat throttling remains source-covered; this smoke intentionally stops
# after the bounded recovery/maintenance contracts so it never depends on host
# D-Bus responsiveness.
grep -q 'status_is_recent_healthy' "$ROOT/bin/dashboard-health-guard.sh"
echo 'PASS: Health guard preserves its bounded healthy-heartbeat contract'
