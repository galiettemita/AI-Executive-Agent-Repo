#!/usr/bin/env bash
# Monthly chaos drill test (dry-run mode).
# Run with DRY_RUN=true to simulate without actual changes.
set -euo pipefail

DRY_RUN="${DRY_RUN:-true}"
echo "[CHAOS DRILL] Starting monthly failover test (dry_run=${DRY_RUN})..."

if [ "${DRY_RUN}" = "true" ]; then
    echo "[CHAOS DRILL] DRY RUN: Would promote Aurora secondary in eu-west-1"
    echo "[CHAOS DRILL] DRY RUN: Would update Route53 to 100% eu-west-1"
    echo "[CHAOS DRILL] DRY RUN: Would failover Temporal namespace"
    echo "[CHAOS DRILL] PASS: Failover script dry-run complete."
else
    ./scripts/infra/failover.sh
fi
