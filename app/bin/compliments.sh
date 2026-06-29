#!/bin/sh
set -eu
DASH="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
exec "$DASH/bin/dashboard-control-server" --compliments "$@"
