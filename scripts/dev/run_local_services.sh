#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

GO_EXEC="./scripts/dev/go_exec.sh"
LOG_DIR="${ROOT_DIR}/artifacts/dev-logs"
mkdir -p "$LOG_DIR"

PIDS=()

start_service() {
  local name="$1"
  local cmd_path="$2"
  local log_file="${LOG_DIR}/${name}.log"

  if command -v watchexec >/dev/null 2>&1; then
    echo "[dev] starting ${name} in watch mode (watchexec)"
    # shellcheck disable=SC2016
    watchexec --restart --watch "${ROOT_DIR}/internal" --watch "${ROOT_DIR}/cmd" -- "${GO_EXEC}" run "${cmd_path}" >"${log_file}" 2>&1 &
  else
    echo "[dev] starting ${name} (no watcher installed)"
    "${GO_EXEC}" run "${cmd_path}" >"${log_file}" 2>&1 &
  fi
  PIDS+=("$!")
}

cleanup() {
  echo "[dev] stopping services..."
  for pid in "${PIDS[@]:-}"; do
    if kill -0 "$pid" >/dev/null 2>&1; then
      kill "$pid" >/dev/null 2>&1 || true
    fi
  done
}

trap cleanup EXIT INT TERM

start_service "gateway" "./cmd/gateway"
start_service "brain" "./cmd/brain"
start_service "control" "./cmd/control"
start_service "executor" "./cmd/executor"
start_service "canvas" "./cmd/canvas"
start_service "temporal-worker" "./cmd/temporal-worker"

echo "[dev] services started. logs: ${LOG_DIR}"
echo "[dev] tailing logs (Ctrl+C to stop)"

tail -n 50 -f "${LOG_DIR}/gateway.log" "${LOG_DIR}/brain.log" "${LOG_DIR}/control.log" "${LOG_DIR}/executor.log" "${LOG_DIR}/canvas.log" "${LOG_DIR}/temporal-worker.log"
