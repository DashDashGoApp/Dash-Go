#!/bin/bash
# Go wrapper: auto-discover calendars from .ics files and write calendars.json.
set -Eeuo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
exec "$SCRIPT_DIR/dashboard-control-server" --gen-calendars "$@"
