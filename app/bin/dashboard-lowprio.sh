#!/bin/bash
# Dash-Go — run a helper with gentle CPU/IO priority when available.
# Keeps maintenance work from fighting surf/WebKit on low-power devices.
set -u
if [ "$#" -lt 1 ]; then
  echo "usage: $0 command [args...]" >&2
  exit 64
fi
if command -v ionice >/dev/null 2>&1 && command -v nice >/dev/null 2>&1; then
  exec ionice -c2 -n7 nice -n 10 "$@"
elif command -v nice >/dev/null 2>&1; then
  exec nice -n 10 "$@"
elif command -v ionice >/dev/null 2>&1; then
  exec ionice -c2 -n7 "$@"
else
  exec "$@"
fi
