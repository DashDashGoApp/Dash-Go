#!/usr/bin/env bash
# Release-blocking regression: ordinary mmcblk boot chatter is not an error.
set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TMP="$(mktemp -d)"
DASH_ROOT="$TMP/dashboard"
FAKE_BIN="$TMP/fake-bin"
JOURNAL="$TMP/journal.txt"
cleanup(){ rm -rf "$TMP"; }
trap cleanup EXIT INT TERM

mkdir -p "$DASH_ROOT" "$FAKE_BIN"
cat > "$FAKE_BIN/journalctl" <<'FAKE_JOURNAL'
#!/bin/sh
cat "${DASHGO_TEST_JOURNAL:?}"
FAKE_JOURNAL
cat > "$FAKE_BIN/findmnt" <<'FAKE_FINDMNT'
#!/bin/sh
printf '%s\n' rw
FAKE_FINDMNT
cat > "$FAKE_BIN/df" <<'FAKE_DF'
#!/bin/sh
printf '%s\n' 'Filesystem 1024-blocks Used Available Capacity Mounted on'
printf '%s\n' '/dev/mmcblk0p2 1048576 98304 950272 10% /dashboard'
FAKE_DF
chmod +x "$FAKE_BIN/journalctl" "$FAKE_BIN/findmnt" "$FAKE_BIN/df"

run_case(){
  local fixture="$1" expected="$2"
  printf '%s\n' "$fixture" > "$JOURNAL"
  rm -rf "$DASH_ROOT/cache" "$DASH_ROOT/logs"
  env PATH="$FAKE_BIN:$PATH" DASH="$DASH_ROOT" DASHGO_TEST_JOURNAL="$JOURNAL" \
    sh "$ROOT/bin/dashboard-storage-wear.sh"
  python3 - "$DASH_ROOT/cache/storage-wear-state.json" "$expected" <<'PY'
import json, sys
payload=json.load(open(sys.argv[1], encoding='utf-8'))
expected=int(sys.argv[2])
actual=payload.get('kernelErrorsCurrentBoot')
if actual != expected:
    raise SystemExit(f'kernelErrorsCurrentBoot={actual}, want {expected}: {payload}')
PY
}

run_case 'mmcblk0: mmc0:0001 SD16G 14.8 GiB
 mmcblk0: p1 p2
EXT4-fs (mmcblk0p2): mounted filesystem with ordered data mode. Quota mode: none.' 0
run_case 'mmcblk0: timeout waiting for hardware interrupt
mmcblk0: error -110 whilst initialising SD card
EXT4-fs error (device mmcblk0p2): ext4_find_entry: reading directory lblock 0' 3

python3 - "$ROOT/bin/doctor.sh" <<'PY_DOCTOR'
from pathlib import Path
import sys
source = Path(sys.argv[1]).read_text(encoding='utf-8')
required = 'mmcblk[^:]*:.*(I/O error|timeout|timed out|error -[0-9]+|reset)'
if required not in source:
    raise SystemExit('Doctor no longer shares the concrete mmc storage-error matcher')
if "grep -Ei 'mmcblk|" in source:
    raise SystemExit('Doctor still treats every mmcblk boot line as an error')
PY_DOCTOR

grep -Fq 'dashboard-storage-wear.sh' "$ROOT/kiosk.sh" || { echo 'FAIL: kiosk must refresh storage canary after boot'; exit 1; }
grep -Fq '7 3 * * *' "$ROOT/bin/dashboard-common.sh" || { echo 'FAIL: canonical storage housekeeping must be daily'; exit 1; }
grep -Fq '7 3 * * *' "$ROOT/bin/doctor.sh" || { echo 'FAIL: Doctor canonical storage housekeeping must be daily'; exit 1; }

echo 'PASS: storage wear and Doctor ignore ordinary mmcblk boot messages, refresh current-boot state after startup, and count concrete storage errors only'
