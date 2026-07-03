#!/usr/bin/env bash
set -euo pipefail
root=${1:-.}
script="$root/bin/setup-vdirsyncer.sh"
bash -n "$script"
grep -q 'Broad discovered mirrors stay read-only' "$script"
grep -q 'Allow Dashboard add/edit/skip for this one collection' "$script"
grep -q 'exact="\$collection/\$local_id"' "$script"
grep -q 'calendar writeback registry written (Dashboard edits start disabled)' "$script"
grep -qF 'LOCK_DIR="\$VDIR_HOME/sync.lock"' "$script"
echo 'calendar writeback setup smoke: syntax, explicit remote-to-local collection mapping, and shared lock contract hold'
