#!/bin/bash
# Go wrapper: generate built-in/default calendars and refresh indexes/cache.
set -Eeuo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
exec "$SCRIPT_DIR/dashboard-control-server" --gen-default-calendars "$@"
