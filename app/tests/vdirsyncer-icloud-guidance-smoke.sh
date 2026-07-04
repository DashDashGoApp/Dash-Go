#!/usr/bin/env bash
# Provider-aware discovery errors must tell an iCloud user the practical fix.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
DISCOVERY="$ROOT/bin/private-calendar-discovery.sh"
[ -x "$DISCOVERY" ] || { echo 'FAIL: discovery helper missing' >&2; exit 1; }
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
mkdir -p "$TMP/home/passwords" "$TMP/home/google-tokens" "$TMP/bin"
cat > "$TMP/bin/vdirsyncer" <<'SH'
#!/usr/bin/env bash
printf '401 Unauthorized\n' >&2
exit 1
SH
chmod +x "$TMP/bin/vdirsyncer"
printf '%s\n' 'icloudtest|blue||connect_icloudtest|/tmp/unused|https://caldav.icloud.com/|person@example.com||caldav||Apple iCloud|icloudtest|' > "$TMP/home/pairs"
set +e
output="$(DASH_VDIR_HOME="$TMP/home" DASH_VDIR_PAIRS="$TMP/home/pairs" DASH_VDIR_PASSWORDS="$TMP/home/passwords" DASH_VDIR_GOOGLE_TOKENS="$TMP/home/google-tokens" DASH_VDIRSYNCER_BIN="$TMP/bin/vdirsyncer" "$DISCOVERY" 2>&1)"
status=$?
set -e
# Discovery reports provider notices as a safe inventory result; callers decide
# whether an empty calendar list is actionable. The important contract is the
# named, provider-specific notice below.
: "$status"
printf '%s\n' "$output" | grep -Fq 'iCloud rejected the sign-in' || { printf '%s\n' "$output" >&2; echo 'FAIL: missing iCloud 401 guidance' >&2; exit 1; }
printf '%s\n' "$output" | grep -Fq 'app-specific password' || { echo 'FAIL: iCloud notice lacks app-password action' >&2; exit 1; }
printf 'PASS: iCloud discovery 401 produces an actionable app-password notice\n'
