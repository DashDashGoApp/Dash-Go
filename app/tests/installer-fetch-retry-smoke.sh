#!/usr/bin/env bash
# Regression: transient verified-release downloads retry before failing.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
INSTALLER="$ROOT/../installer/install.sh"
TMP="$(mktemp -d)"
cleanup(){ rm -rf "$TMP"; }
trap cleanup EXIT INT TERM
mkdir -p "$TMP/fake"
awk '/^fetch\(\)\{/{p=1} p{print} p && /^}$/{exit}' "$INSTALLER" > "$TMP/fetch.sh"
cat > "$TMP/fake/curl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
count_file="${FETCH_TEST_TMP:?}/calls"
count=0
[ -f "$count_file" ] && count="$(cat "$count_file")"
count=$((count + 1))
printf '%s\n' "$count" > "$count_file"
printf '%s\n' "$*" >> "${FETCH_TEST_TMP:?}/args"
out=""
while [ "$#" -gt 0 ]; do
  if [ "$1" = "-o" ]; then out="$2"; shift 2; continue; fi
  shift
done
if [ "$count" -lt 3 ]; then
  printf '%s\n' 'simulated transient Wi-Fi failure' >&2
  exit 7
fi
printf '%s\n' 'verified payload' > "$out"
EOF
chmod +x "$TMP/fake/curl"
# shellcheck disable=SC1090
source "$TMP/fetch.sh"
warn(){ printf '%s\n' "$*" >> "$TMP/warnings"; }
UA='Dash-Go installer'
PATH="$TMP/fake:$PATH" FETCH_TEST_TMP="$TMP" DASHGO_FETCH_ATTEMPTS=3 DASHGO_FETCH_RETRY_DELAY=0 fetch 'https://example.invalid/release.tar.gz' "$TMP/release.tar.gz"
[ "$(cat "$TMP/calls")" = 3 ] || { echo "FAIL: fetch did not retry twice" >&2; exit 1; }
grep -Fx 'verified payload' "$TMP/release.tar.gz" >/dev/null || { echo "FAIL: fetch did not retain the successful final download" >&2; exit 1; }
grep -F -- '--retry-connrefused' "$TMP/args" >/dev/null || { echo "FAIL: fetch is missing retry-connrefused" >&2; exit 1; }
grep -F -- '--retry-all-errors' "$TMP/args" >/dev/null || { echo "FAIL: fetch is missing retry-all-errors" >&2; exit 1; }
echo 'PASS: fetch retries transient failures and preserves HTTPS/curl retry hardening'
