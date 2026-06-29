#!/usr/bin/env bash
# Pi Zero 2 W baseline must keep scarce RAM on the ARM side, while optional
# zram remains an explicit installer choice rather than a silent mutation.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
INSTALL="$ROOT/../installer/install.sh"
CONFIG="$ROOT/base/config.txt"
grep -Fxq 'gpu_mem=32' "$CONFIG"
! grep -Fxq 'gpu_mem=64' "$CONFIG"
grep -Fq 'gpu_mem=32' "$INSTALL"
grep -Fq 'configure_optional_zram_tuning(){' "$INSTALL"
grep -Fq 'Enable optional zram memory tuning? [y/N]' "$INSTALL"
grep -Fq 'apt-get install -y zram-tools' "$INSTALL"
grep -Fq 'PERCENT=50' "$INSTALL"
grep -Fq 'PRIORITY=100' "$INSTALL"
grep -Fq 'vm.swappiness=10' "$INSTALL"
grep -Fq 'enable --now zramswap.service' "$INSTALL"
printf 'PASS: Pi Zero GPU split and opt-in zram tuning contract are present\n'
