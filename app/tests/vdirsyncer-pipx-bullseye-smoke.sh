#!/usr/bin/env bash
# Keep the one pinned vdirsyncer dependency path viable on Raspberry Pi OS
# Bullseye without adding Dash-Go-authored Python.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
SETUP="$ROOT/bin/setup-vdirsyncer.sh"
[ -f "$SETUP" ] || { echo "FAIL: setup-vdirsyncer.sh missing" >&2; exit 1; }
bash -n "$SETUP"
grep -Fq 'DASH_VDIR_APT_CODENAME' "$SETUP" || { echo 'FAIL: setup lacks a testable Debian codename path' >&2; exit 1; }
grep -Fq 'apt-get install -y -t bullseye-backports pipx python3-venv' "$SETUP" || { echo 'FAIL: Bullseye must install pipx and python3-venv from bullseye-backports' >&2; exit 1; }
grep -Fq 'apt-get install -y pipx python3-venv' "$SETUP" || { echo 'FAIL: non-Bullseye APT setup must include python3-venv with pipx' >&2; exit 1; }
grep -Fq 'vdirsyncer[google]==' "$SETUP" || { echo 'FAIL: pinned pipx vdirsyncer install disappeared' >&2; exit 1; }
echo 'PASS: pipx installation covers Debian Bullseye backports and keeps vdirsyncer pinned in pipx.'
