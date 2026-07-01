#!/usr/bin/env bash
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
INSTALL="$ROOT/../installer/install.sh"
require(){ grep -Fq -- "$1" "$INSTALL" || { printf 'FAIL: missing repair contract: %s\n' "$1" >&2; exit 1; }; }
require 'REPAIR_BACKUP_DIR="$DASHGO_STATE_DIR/repair-backups"'
require 'mkdir -p "$REPAIR_BACKUP_DIR" "$LOG_DIR" "$DASH" || return 1'
require 'repair_bundle_recovery_recipe(){'
require 'sha256sum -c SHA256SUMS --ignore-missing'
require './install.sh --repair'
require '[ "$REPAIR_MODE" = "1" ] && repair_bundle_recovery_recipe "$target"'
require '[ "$REPAIR_MODE" = "1" ] && [ "$REPAIR_UPDATE_REQUESTED" != "1" ]; then'
require 'repair_newest_valid_candidate "$REPAIR_BACKUP_DIR/dashboard-repair-*.tar.gz"'
require 'Compatibility fallback for archives made before the state-directory'
if grep -Fq 'settings.json is merged with the current server defaults below' "$INSTALL"; then
  printf 'FAIL: stale settings merge comment remains\n' >&2; exit 1
fi
printf 'PASS: repair backups are external, broken-binary recovery is actionable, and update repair tolerates a missing installed version\n'
