#!/usr/bin/env bash
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
INSTALL="$ROOT/../installer/install.sh"
fail(){ printf 'FAIL: %s\n' "$*" >&2; exit 1; }
[ -f "$INSTALL" ] || fail "missing installer"
helper="$(awk '/^normalize_app_visibility_preferences\(\)/,/^todo_azure_cli_supported_architecture\(\)/{print}' "$INSTALL")"
[ -n "$helper" ] || fail "missing visibility migration helper"
printf '%s' "$helper" | grep -q 'showChalkboard' || fail "migration must remove legacy Chalkboard visibility"
printf '%s' "$helper" | grep -q 'radarEnabled' || fail "migration must remove legacy Radar visibility"
printf '%s' "$helper" | grep -q 'todo.pop("enabled"' || fail "migration must remove legacy To Do visibility"
printf '%s' "$helper" | grep -q 'local-todo' || fail "migration must restore empty To Do mapping"
printf '%s' "$helper" | grep -q 'local-grocery' || fail "migration must restore empty Grocery mapping"
! grep -q 'todo\["enabled"\] = True' "$INSTALL" || fail "fresh To Do setup must not recreate retired visibility"
! grep -q 'Disable radar' "$INSTALL" || fail "installer must not offer a Radar disable option"
awk '/if download_app_files; then/{seen=1} seen&&/normalize_app_visibility_preferences/{ok=1} END{exit ok?0:1}' "$INSTALL" || fail "successful app-file updates must run visibility cleanup"
printf 'PASS: installer permanent-app visibility migration contract\n'
