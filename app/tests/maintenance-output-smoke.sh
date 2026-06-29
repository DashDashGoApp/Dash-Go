#!/usr/bin/env bash
# Maintenance rebuild status is user-facing; raw Go cache warnings belong in
# the task log rather than bubbling into the terminal/control response.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
TMP="$(mktemp -d)"
cleanup(){ rm -rf "$TMP"; }
trap cleanup EXIT
mkdir -p "$TMP/dashboard/bin" "$TMP/dashboard/cache" "$TMP/dashboard/logs"
cp "$ROOT/bin/dashboard-maintenance.sh" "$TMP/dashboard/bin/"
cat > "$TMP/dashboard/bin/dashboard-control-server" <<'SERVER'
#!/usr/bin/env bash
case "${1:-}" in
  --write-status) exit 0;;
  --gen-events-cache) printf 'WARN fixture cache detail\n' >&2; printf '{"ok":true}\n'; exit 0;;
esac
SERVER
chmod +x "$TMP/dashboard/bin/dashboard-control-server" "$TMP/dashboard/bin/dashboard-maintenance.sh"
"$TMP/dashboard/bin/dashboard-maintenance.sh" rebuild-event-cache >"$TMP/terminal.out" 2>&1
if grep -Fq 'WARN fixture cache detail' "$TMP/terminal.out"; then
  echo 'FAIL: cache warning leaked to maintenance terminal output' >&2
  exit 1
fi
grep -Fq 'WARN fixture cache detail' "$TMP/dashboard/logs/maintenance.log"
printf 'PASS: maintenance cache diagnostics are retained in the task log\n'
