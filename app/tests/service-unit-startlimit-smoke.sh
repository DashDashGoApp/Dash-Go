#!/usr/bin/env bash
# Release-blocking regression: systemd start-limit keys belong in [Unit].
set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
INSTALLER="$ROOT/../installer/install.sh"
python3 - "$ROOT/bin/doctor.sh" "$INSTALLER" <<'PY'
from pathlib import Path
import sys

def blocks(text):
    return [block for block in text.split('[Unit]')[1:] if '[Service]' in block]

for path in map(Path, sys.argv[1:]):
    text=path.read_text(encoding='utf-8')
    found=False
    for block in blocks(text):
        unit, rest=block.split('[Service]', 1)
        service=rest.split('[Install]', 1)[0]
        if 'StartLimitIntervalSec=120' in unit and 'StartLimitBurst=5' in unit:
            found=True
        if 'StartLimitIntervalSec=' in service or 'StartLimitBurst=' in service:
            raise SystemExit(f'{path}: start-limit key remains in [Service]')
    if not found:
        raise SystemExit(f'{path}: no canonical [Unit] start-limit contract found')
print('PASS: dashboard-server start-limit settings are scoped to [Unit]')
PY
