#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

echo "[setup-local] installing Node dependencies"
if command -v pnpm >/dev/null 2>&1; then
  pnpm install
else
  echo "pnpm is not installed; skipping pnpm install"
fi

echo "[setup-local] starting local dependencies via docker compose"
if command -v docker >/dev/null 2>&1; then
  docker compose up -d postgres redis temporal
else
  echo "docker is not installed; skipping docker compose"
fi

echo "[setup-local] verifying postgres migrations"
bash scripts/database/verify_postgres_migrations.sh || true

echo "[setup-local] done"
