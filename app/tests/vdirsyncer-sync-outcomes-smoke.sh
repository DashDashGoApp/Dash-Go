#!/usr/bin/env bash
# Keep durable sync safety contracts visible while guided setup owns enrollment.
set -euo pipefail
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
SETUP="$ROOT/bin/setup-vdirsyncer.sh"
SELECT="$ROOT/bin/private-calendar-selection.sh"
fail(){ echo "FAIL: $*" >&2; exit 1; }
bash -n "$SETUP"
bash -n "$SELECT"
grep -Fq 'classify_failure' "$SETUP" || fail 'per-pair failure classification missing'
grep -Fq 'attention-empty' "$SETUP" || fail 'empty collection guard outcome missing'
grep -Fq 'attention-auth' "$SETUP" || fail 'provider authorization outcome missing'
grep -Fq 'conflict_resolution' "$SETUP" || fail 'one-shot conflict resolution contract missing'
grep -Fq 'TARGET_PAIR' "$SETUP" || fail 'targeted sync contract missing'
grep -Fq 'sync --force-delete' "$SETUP" || fail 'final-delete safety boundary missing'
grep -Fq 'discover "$pair"' "$SELECT" || fail 'exact selection must perform explicit discovery'
grep -Fq 'initial="waiting"' "$SELECT" || fail 'failed initial sync must not be reported as ready'
grep -Fq 'private_cleanup_transaction' "$SETUP" || fail 'guided enrollment must roll back partial activation'
echo 'PASS: private calendar sync retains explicit per-pair outcomes, guarded recovery, and staged enrollment rollback.'
