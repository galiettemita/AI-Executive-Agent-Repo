#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

bash scripts/deploy/external_closeout_check.sh
bash scripts/deploy/generate_go_live_signoff.sh
bash scripts/deploy/generate_manual_closeout_todo.sh

echo "external phase artifacts synced:"
echo "  - artifacts/deploy/external_closeout_status.json"
echo "  - artifacts/deploy/go_live_signoff_status.json"
echo "  - artifacts/deploy/manual_closeout_todo.md"
