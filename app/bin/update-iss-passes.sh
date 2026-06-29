#!/bin/bash
# Go wrapper: refresh optional ISS visible-pass calendar via N2YO.
set -Eeuo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
exec "$SCRIPT_DIR/dashboard-control-server" --update-iss-passes "$@"
