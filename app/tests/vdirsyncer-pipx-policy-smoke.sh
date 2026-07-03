#!/usr/bin/env bash
# Exercise Dash-Go's managed pipx installation policy without reaching PyPI.
# Proves vdirsyncer is installed as an exact 0.20.0 [google] environment in a
# private home, pinned where supported, and never installed through raw pip or
# the system vdirsyncer package.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
SETUP="$ROOT/bin/setup-vdirsyncer.sh"
TMP="$(mktemp -d)"
cleanup(){ rm -rf "$TMP"; }
trap cleanup EXIT
HOME="$TMP/home"
export HOME
mkdir -p "$HOME/dashboard/bin" "$TMP/fake-bin"
cp "$SETUP" "$HOME/dashboard/bin/setup-vdirsyncer.sh"
chmod +x "$HOME/dashboard/bin/setup-vdirsyncer.sh"

cat > "$TMP/fake-bin/vdir-python" <<'PY'
#!/usr/bin/env bash
set -eu
[ "${1:-}" = "-c" ] && exit 0
exec /usr/bin/env bash "$@"
PY
cat > "$TMP/fake-bin/pipx" <<'PIPX'
#!/usr/bin/env bash
set -euo pipefail
: "${PIPX_HOME:?}" "${PIPX_BIN_DIR:?}" "${FAKE_PIPX_LOG:?}"
printf '%s\n' "$*" >> "$FAKE_PIPX_LOG"
case "${1:-}" in
  install)
    [ "${2:-}" = "--force" ] || exit 31
    [ "${3:-}" = "vdirsyncer[google]==0.20.0" ] || exit 32
    mkdir -p "$PIPX_HOME/venvs/vdirsyncer" "$PIPX_BIN_DIR"
    cat > "$PIPX_BIN_DIR/vdirsyncer" <<'VDIR'
#!/usr/bin/env bash
set -eu
if [ "${1:-}" = "--version" ]; then printf 'vdirsyncer, version 0.20.0\n'; exit 0; fi
case " $* " in
  *' sync '*)
    root="$(dirname "${VDIRSYNCER_CONFIG:?}")"
    mkdir -p "$root/collections/family/collection-1"
    printf 'BEGIN:VCALENDAR\nVERSION:2.0\nBEGIN:VEVENT\nUID:one\nSUMMARY:Private event\nEND:VEVENT\nEND:VCALENDAR\n' > "$root/collections/family/collection-1/one.ics"
    ;;
esac
exit 0
VDIR
    chmod 700 "$PIPX_BIN_DIR/vdirsyncer"
    ;;
  pin)
    [ "${2:-}" = "vdirsyncer" ] || exit 33
    ;;
  *) exit 34;;
esac
PIPX
cat > "$TMP/fake-bin/crontab" <<'CRON'
#!/usr/bin/env bash
set -eu
: "${FAKE_CRONTAB:?}"
if [ "${1:-}" = "-l" ]; then [ -f "$FAKE_CRONTAB" ] && cat "$FAKE_CRONTAB"; exit 0; fi
cp "$1" "$FAKE_CRONTAB"
CRON
cat > "$HOME/dashboard/bin/gen-calendars.sh" <<'GEN'
#!/usr/bin/env bash
exit 0
GEN
cat > "$HOME/dashboard/bin/dashboard-control-server" <<'SERVER'
#!/usr/bin/env bash
exit 0
SERVER
chmod +x "$TMP/fake-bin/vdir-python" "$TMP/fake-bin/pipx" "$TMP/fake-bin/crontab" "$HOME/dashboard/bin/gen-calendars.sh" "$HOME/dashboard/bin/dashboard-control-server"

export PATH="$TMP/fake-bin:$PATH" FAKE_PIPX_LOG="$TMP/pipx.log" FAKE_CRONTAB="$TMP/crontab" DASH_VDIRSYNCER_PYTHON="$TMP/fake-bin/vdir-python"
: > "$FAKE_PIPX_LOG"
printf '\nfamily\nblue\nn\n1\nhttps://caldav.example/\nfamily@example.com\nfixture-password\ncollection-1\n\n' \
  | "$HOME/dashboard/bin/setup-vdirsyncer.sh" >"$TMP/setup.out" 2>&1 || { cat "$TMP/setup.out" >&2; exit 1; }

fail(){ echo "FAIL: $*" >&2; cat "$TMP/setup.out" >&2; exit 1; }
VDIR_HOME="$HOME/.dashboard-vdirsyncer"
WRAPPER="$HOME/dashboard/bin/sync-vdir.sh"
[ -x "$VDIR_HOME/bin/vdirsyncer" ] || fail 'known private vdirsyncer wrapper missing'
[ "$(stat -c '%a' "$VDIR_HOME/pipx")" = 700 ] || fail 'private pipx home must be owner-only'
[ "$(stat -c '%a' "$VDIR_HOME/bin")" = 700 ] || fail 'private pipx bin must be owner-only'
grep -Fxq 'install --force vdirsyncer[google]==0.20.0' "$FAKE_PIPX_LOG" || fail 'pipx must install exact vdirsyncer[google] 0.20.0'
grep -Fxq 'pin vdirsyncer' "$FAKE_PIPX_LOG" || fail 'pipx pin must be attempted'
! grep -qi 'upgrade' "$FAKE_PIPX_LOG" || fail 'setup must not upgrade vdirsyncer'
grep -Fq "VDIRSYNCER_BIN=$VDIR_HOME/bin/vdirsyncer" "$WRAPPER" || fail 'generated wrapper must use the known private vdirsyncer path'
! grep -q 'command -v vdirsyncer' "$WRAPPER" || fail 'generated wrapper must not fall back to an arbitrary PATH vdirsyncer'
grep -Fq 'vdirsyncer[google]==$VDIRSYNCER_VERSION' "$HOME/dashboard/bin/setup-vdirsyncer.sh" || fail 'source must retain exact managed install spec'
! grep -q 'pip3 install' "$HOME/dashboard/bin/setup-vdirsyncer.sh" || fail 'raw pip must not be an installer path'
! grep -q 'apt-get install -y vdirsyncer' "$HOME/dashboard/bin/setup-vdirsyncer.sh" || fail 'system vdirsyncer package must not be installed'
! grep -q 'pipx inject' "$HOME/dashboard/bin/setup-vdirsyncer.sh" || fail 'Google support must be installed with the original pinned environment'
grep -Fq 'apt-get install -y pipx' "$HOME/dashboard/bin/setup-vdirsyncer.sh" || fail 'APT may install pipx on Debian-family systems'
bash -n "$WRAPPER"
# Existing beta.4-style connections can migrate by reopening setup and ending
# the add loop immediately. This must regenerate the wrapper without another
# package install or any credential prompt/rewrite.
printf '\n' | "$HOME/dashboard/bin/setup-vdirsyncer.sh" >"$TMP/migrate.out" 2>&1 || { cat "$TMP/migrate.out" >&2; exit 1; }
[ "$(grep -Fc 'install --force vdirsyncer[google]==0.20.0' "$FAKE_PIPX_LOG")" -eq 1 ] || fail 'migration must reuse the existing pinned pipx environment'
grep -Fq 'refreshing existing pipx-managed sync configuration' "$TMP/migrate.out" || fail 'empty add loop must refresh existing private calendar configuration'
grep -Fq 'fixture-password' "$HOME/.dashboard-vdirsyncer/passwords/family" || fail 'migration must preserve existing private credentials'
echo 'PASS: private calendar sync uses pinned pipx vdirsyncer without raw pip or system vdirsyncer installation'
