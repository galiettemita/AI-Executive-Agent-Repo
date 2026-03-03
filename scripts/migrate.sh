#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

if [ -d migrations ]; then
  echo "[migrate] OpenClaw migration directory present: migrations/"
else
  echo "[migrate] migrations directory missing"
  exit 1
fi

echo "[migrate] legacy migration verification"
bash scripts/database/verify_postgres_migrations.sh || true
