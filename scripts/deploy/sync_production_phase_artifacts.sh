#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

ALLOW_CONDITIONAL_MANUAL="${ALLOW_CONDITIONAL_MANUAL:-1}"

ALLOW_CONDITIONAL_MANUAL="$ALLOW_CONDITIONAL_MANUAL" bash scripts/deploy/check_external_phase_transition.sh
bash scripts/deploy/check_production_deployment_signoff.sh
ALLOW_CONDITIONAL_MANUAL="$ALLOW_CONDITIONAL_MANUAL" bash scripts/deploy/check_production_canary_window.sh
bash scripts/deploy/generate_production_deployment_todo.sh
ALLOW_CONDITIONAL_MANUAL="$ALLOW_CONDITIONAL_MANUAL" bash scripts/deploy/check_production_post_deploy_validation.sh

echo "production phase artifacts synced:"
echo "  - artifacts/deploy/external_phase_transition_check.json"
echo "  - artifacts/deploy/production_deployment_signoff_check.json"
echo "  - artifacts/deploy/production_canary_check.json"
echo "  - artifacts/deploy/production_deployment_todo.md"
echo "  - artifacts/deploy/production_post_deploy_validation.json"
