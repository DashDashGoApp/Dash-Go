#!/usr/bin/env bash
# Release-blocking regression for system package-update directory-lock ownership.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TMP="$(mktemp -d)"
DASH="$TMP/dashboard"
FAKE="$TMP/fake-bin"
cleanup(){ rm -rf "$TMP"; }
trap cleanup EXIT INT TERM
mkdir -p "$DASH/bin" "$DASH/cache" "$DASH/logs" "$FAKE"
cp "$ROOT/bin/dashboard-system-update.sh" "$ROOT/bin/dashboard-housekeeping.sh" "$DASH/bin/"
cat > "$DASH/bin/dashboard-control-server" <<'EOF'
#!/bin/sh
printf '%s\n' "$*" >> "${DASH:?}/cache/status-calls.log"
exit 0
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
cat > "$FAKE/apt-get" <<'EOF'
#!/bin/sh
case "${1:-}" in
  update)
    : > "$DASH/cache/apt-update-started"
    while [ ! -f "$DASH/cache/release-apt" ]; do sleep 0.02; done
    ;;
  -y) exit 0;;
  *) exit 0;;
esac
EOF
chmod +x "$DASH/bin/dashboard-control-server" "$FAKE/sudo" "$FAKE/apt-get"
run_update(){ env PATH="$FAKE:$PATH" DASH="$DASH" TEST_APT="$FAKE/apt-get" "$DASH/bin/dashboard-system-update.sh"; }
run_housekeeping(){ env PATH="$FAKE:$PATH" DASH="$DASH" TEST_APT="$FAKE/apt-get" "$DASH/bin/dashboard-housekeeping.sh"; }
wait_for(){ local f="$1"; for _ in $(seq 1 200); do [ -e "$f" ] && return 0; sleep 0.02; done; echo "FAIL: timed out waiting for $f" >&2; return 1; }

run_update &
owner_shell=$!
wait_for "$DASH/cache/system-update.lock/pid"
owner="$(tr -d '[:space:]' < "$DASH/cache/system-update.lock/pid")"
[[ "$owner" =~ ^[0-9]+$ ]] || { echo "FAIL: update lock PID is invalid" >&2; exit 1; }
kill -0 "$owner" 2>/dev/null || { echo "FAIL: update lock PID is not live" >&2; exit 1; }
grep -F -- '--command-pid' "$DASH/cache/status-calls.log" >/dev/null || { echo "FAIL: update status did not record command PID" >&2; exit 1; }
run_housekeeping
[ -d "$DASH/cache/system-update.lock" ] || { echo "FAIL: housekeeping removed active system-update lock" >&2; exit 1; }
[ "$(tr -d '[:space:]' < "$DASH/cache/system-update.lock/pid")" = "$owner" ] || { echo "FAIL: housekeeping changed active lock owner" >&2; exit 1; }
set +e
run_update
second_rc=$?
set -e
[ "$second_rc" -eq 75 ] || { echo "FAIL: overlapping update exit=$second_rc, want 75" >&2; exit 1; }
touch "$DASH/cache/release-apt"
wait "$owner_shell"
[ ! -e "$DASH/cache/system-update.lock" ] || { echo "FAIL: normal update left lock directory behind" >&2; exit 1; }

mkdir "$DASH/cache/system-update.lock"
printf '999999\n' > "$DASH/cache/system-update.lock/pid"
run_housekeeping
[ ! -e "$DASH/cache/system-update.lock" ] || { echo "FAIL: housekeeping did not remove recorded-dead lock" >&2; exit 1; }

mkdir "$DASH/cache/system-update.lock"
printf '999999\n' > "$DASH/cache/system-update.lock/pid"
rm -f "$DASH/cache/release-apt" "$DASH/cache/apt-update-started"
run_update &
reclaimed_shell=$!
wait_for "$DASH/cache/apt-update-started"
reclaimed_owner="$(tr -d '[:space:]' < "$DASH/cache/system-update.lock/pid")"
[[ "$reclaimed_owner" =~ ^[0-9]+$ ]] && [ "$reclaimed_owner" != 999999 ] || { echo "FAIL: update helper did not reclaim recorded-dead lock" >&2; exit 1; }
touch "$DASH/cache/release-apt"
wait "$reclaimed_shell"
[ ! -e "$DASH/cache/system-update.lock" ] || { echo "FAIL: reclaimed update left lock behind" >&2; exit 1; }

mkdir "$DASH/cache/system-update.lock"
printf 'not-a-pid\n' > "$DASH/cache/system-update.lock/pid"
run_housekeeping
[ -d "$DASH/cache/system-update.lock" ] || { echo "FAIL: housekeeping removed ambiguous lock" >&2; exit 1; }
set +e
run_update
ambiguous_rc=$?
set -e
[ "$ambiguous_rc" -eq 75 ] || { echo "FAIL: ambiguous lock update exit=$ambiguous_rc, want 75" >&2; exit 1; }
[ -d "$DASH/cache/system-update.lock" ] || { echo "FAIL: update helper removed ambiguous lock" >&2; exit 1; }
rm -rf "$DASH/cache/system-update.lock"

# A failed apt command must record a terminal failure rather than exiting under
# set -e with a permanently-running status.
rm -f "$DASH/cache/release-apt" "$DASH/cache/status-calls.log"
cat > "$FAKE/apt-get" <<'EOF'
#!/bin/sh
[ "${1:-}" = "update" ] && exit 42
exit 0
EOF
chmod +x "$FAKE/apt-get"
set +e
run_update
failed_rc=$?
set -e
[ "$failed_rc" -eq 42 ] || { echo "FAIL: failed apt exit=$failed_rc, want 42" >&2; exit 1; }
grep -F -- '--state failed' "$DASH/cache/status-calls.log" >/dev/null || { echo "FAIL: failed apt did not write terminal failed status" >&2; exit 1; }

echo 'PASS: system update PID lock ownership, stale recovery, housekeeping safety, and terminal failure status are preserved'
