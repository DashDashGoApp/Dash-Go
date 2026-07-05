#!/usr/bin/env bash
# Regression: Control-launched system updates retry package locks without
# changing the exact sudo command form that existing scoped sudoers permits.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TMP="$(mktemp -d)"
DASH="$TMP/dashboard"
FAKE="$TMP/fake-bin"
cleanup(){ rm -rf "$TMP"; }
trap cleanup EXIT INT TERM
mkdir -p "$DASH/bin" "$DASH/cache" "$DASH/logs" "$FAKE"
cp "$ROOT/bin/dashboard-system-update.sh" "$DASH/bin/"
cat > "$DASH/bin/dashboard-control-server" <<'EOF'
#!/bin/sh
printf '%s\n' "$*" >> "${DASH:?}/cache/status-calls.log"
EOF
cat > "$FAKE/sudo" <<'EOF'
#!/bin/sh
[ "${1:-}" = "-n" ] && shift
if [ "${1:-}" = "/usr/bin/apt-get" ]; then
  shift
  exec "${TEST_APT:?}" "$@"
fi
exec "$@"
EOF
cat > "$FAKE/apt-transient" <<'EOF'
#!/bin/sh
count_file="${DASH:?}/cache/apt-count"
count=0
[ -f "$count_file" ] && count="$(cat "$count_file")"
count=$((count + 1))
printf '%s\n' "$count" > "$count_file"
if [ "$count" -lt 3 ]; then
  printf '%s\n' 'E: Could not get lock /var/lib/dpkg/lock-frontend - open (11: Resource temporarily unavailable)' >&2
  exit 100
fi
exit 0
EOF
cat > "$FAKE/apt-locked" <<'EOF'
#!/bin/sh
printf '%s\n' 'E: Unable to acquire the dpkg frontend lock (/var/lib/dpkg/lock-frontend), is another process using it?' >&2
exit 100
EOF
chmod +x "$DASH/bin/dashboard-control-server" "$FAKE"/*
run_update(){ env PATH="$FAKE:$PATH" DASH="$DASH" TEST_APT="$1" DASHGO_SYSTEM_UPDATE_LOCK_RETRIES="$2" DASHGO_SYSTEM_UPDATE_LOCK_DELAY=0 "$DASH/bin/dashboard-system-update.sh"; }
run_update "$FAKE/apt-transient" 3
[ "$(cat "$DASH/cache/apt-count")" = 4 ] || { echo "FAIL: transient lock retry count is wrong" >&2; exit 1; }
grep -F 'APT is busy with background maintenance; waiting for the package lock' "$DASH/cache/status-calls.log" >/dev/null || { echo "FAIL: lock wait was not surfaced to Control" >&2; exit 1; }
rm -rf "$DASH/cache/system-update.lock" "$DASH/cache/apt-count" "$DASH/cache/status-calls.log"
set +e
run_update "$FAKE/apt-locked" 2
rc=$?
set -e
[ "$rc" -eq 75 ] || { echo "FAIL: lock timeout exit=$rc, want 75" >&2; exit 1; }
grep -F 'APT was busy with background maintenance for too long. Try again in a few minutes.' "$DASH/cache/status-calls.log" >/dev/null || { echo "FAIL: lock timeout was not explained" >&2; exit 1; }
grep -F 'sudo -n /usr/bin/apt-get' "$ROOT/bin/dashboard-system-update.sh" >/dev/null || { echo "FAIL: system update no longer preserves exact scoped sudo command form" >&2; exit 1; }
echo 'PASS: Control system update retries APT locks and preserves scoped sudo compatibility'
