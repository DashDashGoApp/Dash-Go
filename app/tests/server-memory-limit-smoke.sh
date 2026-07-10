#!/usr/bin/env bash
set -euo pipefail
APP_ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
INSTALLER="$APP_ROOT/../installer/install.sh"
python3 - "$INSTALLER" <<'PY'
from pathlib import Path
import re
import sys

source = Path(sys.argv[1]).read_text(encoding="utf-8")
setting = "Environment=GOMEMLIMIT=96MiB"
if source.count(setting) != 3:
    raise SystemExit(f"FAIL: expected two service unit entries plus one convergence check for {setting}; found {source.count(setting)}")
units = [
    body
    for body in re.findall(r"<<UNIT\n(?P<body>[\s\S]*?)\nUNIT(?:\n|$)", source)
    if "ExecStart=$DASH/bin/dashboard-control-server" in body
]
if len(units) != 2:
    raise SystemExit(f"FAIL: expected exactly two dashboard service unit heredocs; found {len(units)}")
for index, unit in enumerate(units, 1):
    if unit.count(setting) != 1:
        raise SystemExit(f"FAIL: service unit heredoc {index} does not contain exactly one {setting}")
    if unit.index(setting) > unit.index("ExecStart="):
        raise SystemExit(f"FAIL: service unit heredoc {index} must set GOMEMLIMIT before ExecStart")
if not re.search(r"service_unit_section_has_setting \"\$svc\" Service 'Environment=GOMEMLIMIT=96MiB'", source):
    raise SystemExit("FAIL: service-unit repair does not converge the Go memory limit")
print("PASS: dashboard-server.service applies and repairs the distinct 96 MiB soft Go memory limit")
PY
