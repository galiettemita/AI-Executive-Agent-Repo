#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

resolve_docker_bin() {
  if [[ -x "/Applications/Docker.app/Contents/Resources/bin/docker" ]]; then
    echo "/Applications/Docker.app/Contents/Resources/bin/docker"
    return 0
  fi
  if command -v docker >/dev/null 2>&1; then
    command -v docker
    return 0
  fi
  return 1
}

echo "[setup-local] project root: $ROOT_DIR"

if command -v pnpm >/dev/null 2>&1; then
  echo "[setup-local] installing node dependencies with pnpm"
  pnpm install
else
  echo "[setup-local] pnpm not found; skipping node dependency install"
fi

docker_bin="$(resolve_docker_bin || true)"
if [[ -n "$docker_bin" ]]; then
  echo "[setup-local] starting docker compose dependencies (postgres, redis, temporal)"
  "$docker_bin" compose up -d postgres redis temporal
else
  echo "[setup-local] docker not available; skipping docker compose startup"
fi

echo "[setup-local] running migration baseline checks"
bash scripts/migrate.sh

echo "[setup-local] validating OpenClaw seed manifest"
if command -v node >/dev/null 2>&1; then
  node scripts/seed-skills.ts --json-output=artifacts/seed-skills-summary.json >/dev/null
  echo "[setup-local] skill seed summary written to artifacts/seed-skills-summary.json"
else
  echo "[setup-local] node unavailable; skipping seed-skills.ts execution"
fi

if [[ "${START_SERVICES:-1}" == "0" ]]; then
  echo "[setup-local] START_SERVICES=0; bootstrap finished without running services"
  exit 0
fi

echo "[setup-local] launching local services"
echo "[setup-local] press Ctrl+C to stop all services"
bash scripts/dev/run_local_services.sh
