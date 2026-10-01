#!/usr/bin/env bash
# Release-blocking regression for update-rollback retention in housekeeping:
# age-prunes stale failed/pending payload trees, keeps the newest failed
# snapshot and its metadata evidence, and never touches a fresh snapshot.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TMP="$(mktemp -d)"
cleanup(){ rm -rf "$TMP"; }
trap cleanup EXIT INT TERM
DASH="$TMP/dashboard"
mkdir -p "$DASH/bin" "$DASH/cache/update-rollback" "$DASH/logs"

# Minimal stand-in for the status writer so the real binary is not required.
cat > "$DASH/bin/dashboard-control-server" <<'EOF'
#!/bin/sh
exit 0
EOF
chmod +x "$DASH/bin/dashboard-control-server"
cp "$ROOT/bin/dashboard-housekeeping.sh" "$DASH/bin/"

new_file(){ mkdir -p "$(dirname "$1")"; printf '%s\n' "$2" > "$1"; }

old=$(( $(date +%s) - 30*86400 ))
new=$(( $(date +%s) - 3600 ))

# Two stale failed trees (June-style), one fresh failed tree, one stale pending.
mkdir -p "$DASH/cache/update-rollback/failed-stale1/payload" \
         "$DASH/cache/update-rollback/failed-stale2/extract" \
         "$DASH/cache/update-rollback/failed-fresh/backup" \
         "$DASH/cache/update-rollback/pending-stale/backup"
printf 'release payload\n' > "$DASH/cache/update-rollback/failed-stale1/payload/release.tar.gz"
new_file "$DASH/cache/update-rollback/failed-stale1/failure-reason.txt" "Runtime readiness failed"
new_file "$DASH/cache/update-rollback/failed-stale1/payload-files.txt" "ui/js/app.bundle.js"
new_file "$DASH/cache/update-rollback/failed-stale2/failure-reason.txt" "Rollback failed"
new_file "$DASH/cache/update-rollback/failed-fresh/failure-reason.txt" "Fresh failure"
new_file "$DASH/cache/update-rollback/pending-stale/payload-files.txt" "bin/dashboard-control-server"
touch -d "@$old" "$DASH/cache/update-rollback/failed-stale1" "$DASH/cache/update-rollback/failed-stale2" "$DASH/cache/update-rollback/pending-stale"
touch -d "@$new" "$DASH/cache/update-rollback/failed-fresh"

# A fresh pending snapshot (the running update) must survive untouched.
mkdir -p "$DASH/cache/update-rollback/pending-fresh/backup"
new_file "$DASH/cache/update-rollback/pending-fresh/payload-files.txt" "VERSION"
touch -d "@$new" "$DASH/cache/update-rollback/pending-fresh"

# Two doctor-bad weather caches; the older must go, newest stays.
new_file "$DASH/cache/weather-cache.json.doctor-bad-old" "{}"
new_file "$DASH/cache/weather-cache.json.doctor-bad-new" "{}"
touch -d "@$old" "$DASH/cache/weather-cache.json.doctor-bad-old"
touch -d "@$new" "$DASH/cache/weather-cache.json.doctor-bad-new"

DASH="$DASH" sh "$DASH/bin/dashboard-housekeeping.sh"

RB="$DASH/cache/update-rollback"
fail(){ printf 'housekeeping-rollback-smoke: %s\n' "$1" >&2; exit 1; }
[ ! -d "$RB/failed-stale1" ] || fail "stale failed tree survived"
[ ! -d "$RB/failed-stale2" ] || fail "stale failed tree survived"
[ ! -d "$RB/pending-stale" ] || fail "stale pending tree survived"
[ -d "$RB/failed-fresh" ] || fail "fresh failed tree was pruned"
[ -d "$RB/pending-fresh" ] || fail "fresh pending tree (running update) was pruned"
[ -f "$RB/failed-fresh/failure-reason.txt" ] || fail "fresh failed metadata lost"
# Metadata from a pruned tree is retained beside the rollback dir.
[ -f "$RB/pruned-failed-stale1-failure-reason.txt" ] || fail "pruned failure-reason.txt not retained"
[ -f "$RB/pruned-failed-stale1-payload-files.txt" ] || fail "pruned payload-files.txt not retained"
[ ! -f "$DASH/cache/weather-cache.json.doctor-bad-old" ] || fail "old doctor-bad cache survived"
[ -f "$DASH/cache/weather-cache.json.doctor-bad-new" ] || fail "new doctor-bad cache was pruned"

# Rollback still works with a pending snapshot present: the retained fresh
# pending tree keeps its payload manifest and files.
[ -f "$RB/pending-fresh/payload-files.txt" ] || fail "pending payload manifest lost"

printf 'housekeeping-rollback-smoke: ok\n'
