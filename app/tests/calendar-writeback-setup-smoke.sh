#!/usr/bin/env bash
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
script="${1:-$ROOT}/bin/setup-vdirsyncer.sh"
bash -n "$script"
grep -q 'Allow Dash-Go to add, edit, or skip events in' "$script"
grep -q 'exact="\$collection/\$local_id"' "$script"
grep -qF 'LOCK_DIR="\$VDIR_HOME/sync.lock"' "$script"
grep -q 'private_choose_and_activate' "$script"
grep -q 'private_cleanup_transaction' "$script"
echo 'calendar writeback setup smoke: staged exact collection selection and shared lock contract hold'
