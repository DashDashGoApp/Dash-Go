#!/bin/bash
# Go wrapper: refresh selected public/observance holiday calendars.
set -Eeuo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
exec "$SCRIPT_DIR/dashboard-control-server" --update-holidays "$@"
