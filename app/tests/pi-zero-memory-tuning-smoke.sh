#!/usr/bin/env bash
# Display memory and drivers are Raspberry Pi OS policy, not installer defaults.
# Dash-Go keeps its Lite profile but must not force legacy FKMS/gpu_mem settings.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
CONFIG="$ROOT/base/config.txt"
INSTALL="$ROOT/../installer/install.sh"
[ -f "$CONFIG" ] && [ -f "$INSTALL" ]
grep -Fq 'not applied by the installer' "$CONFIG"
grep -Fq 'vc4-kms-v3d' "$CONFIG"
! grep -Fq 'gpu_mem=32' "$CONFIG"
! grep -Fq 'vc4-fkms-v3d' "$CONFIG"
grep -Fq 'KMS settings stay unchanged' "$INSTALL"
grep -Fq 'Create a reversible backup before any manual display troubleshooting?' "$INSTALL"
echo 'PASS: Pi Lite guidance preserves modern KMS and avoids automatic gpu-memory changes.'
