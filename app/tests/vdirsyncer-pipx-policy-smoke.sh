#!/usr/bin/env bash
# Keep the private-calendar tool ladder safe and explicit.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
SETUP="$ROOT/bin/setup-vdirsyncer.sh"
fail(){ echo "FAIL: $*" >&2; exit 1; }
bash -n "$SETUP"
grep -Fq 'pipx_works(){ have pipx && pipx_run --version' "$SETUP" || fail 'pipx must be tested by running it, not merely found on PATH'
grep -Fq 'Install Dash-Go' "$SETUP" || fail 'existing pipx path must explain that only Dash-Go vdirsyncer is being installed'
grep -Fq 'install_pinned_vdirsyncer_pipx' "$SETUP" || fail 'pipx remains the preferred managed install'
grep -Fq 'install_pinned_vdirsyncer_venv' "$SETUP" || fail 'isolated virtual-environment fallback missing'
grep -Fq 'install_pinned_vdirsyncer_user_pip' "$SETUP" || fail 'explicit last-resort user pip fallback missing'
grep -Fq 'python3 -m pip install --user' "$SETUP" || fail 'last-resort fallback must remain user scoped'
! grep -Eq 'sudo[[:space:]]+(python3[[:space:]]+-m[[:space:]]+)?pip(3)?[[:space:]]+install|pip[[:space:]]+install.*--break-system-packages' "$SETUP" || fail 'unsafe system pip mutation is forbidden'
! grep -Fq 'pip3 install' "$SETUP" || fail 'unqualified pip3 install is forbidden'
! grep -Fq 'apt-get install -y vdirsyncer' "$SETUP" || fail 'system vdirsyncer package must not be installed'
grep -Fq 'vdirsyncer[google]==$VDIRSYNCER_VERSION' "$SETUP" || fail 'all install paths must retain the exact pinned vdirsyncer version'
echo 'PASS: private-calendar tooling prefers verified pipx, then isolated venv, and exposes user pip only as an explicit non-root last resort.'
