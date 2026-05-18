#!/usr/bin/env bash
# Delegate to the canonical forward-only migration runner.
# Per DECISIONS.md D6, only db/migrations/ is used in production.
set -euo pipefail
exec bash "$(dirname "${BASH_SOURCE[0]}")/database/migrate.sh" "$@"
