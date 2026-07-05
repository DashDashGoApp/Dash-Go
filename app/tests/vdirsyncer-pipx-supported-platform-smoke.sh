#!/usr/bin/env bash
# Post-Bullseye private-calendar bootstrap must use only supported normal APT
# repositories, then retain Dash-Go's isolated pipx/venv/user-pip ladder.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
SETUP="$ROOT/bin/setup-vdirsyncer.sh"
[ -f "$SETUP" ] || { echo "FAIL: setup-vdirsyncer.sh missing" >&2; exit 1; }
bash -n "$SETUP"
if grep -Eiq 'bullseye-backports|DASH_VDIR_APT_CODENAME' "$SETUP"; then
  echo 'FAIL: unsupported Bullseye backports handling remains in private-calendar setup' >&2; exit 1
fi
grep -Fq 'apt_with_lock_wait install -y pipx python3-venv' "$SETUP" || { echo 'FAIL: supported APT setup must install pipx plus python3-venv together' >&2; exit 1; }
grep -Fq 'DPkg::Lock::Timeout' "$SETUP" || { echo 'FAIL: private-calendar APT setup must wait for a package lock' >&2; exit 1; }
grep -Fq 'vdirsyncer[google]==' "$SETUP" || { echo 'FAIL: pinned pipx vdirsyncer install disappeared' >&2; exit 1; }
grep -Fq 'python3 -m pip install --user' "$SETUP" || { echo 'FAIL: explicit last-resort user pip fallback disappeared' >&2; exit 1; }
echo 'PASS: post-Bullseye pipx bootstrap uses supported repositories and keeps isolated fallbacks.'
