#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
NAMESPACE="${NAMESPACE:-default}"
VALUES_DIR="${VALUES_DIR:-${ROOT_DIR}/artifacts/deploy}"
FALLBACK_VALUES_DIR="${FALLBACK_VALUES_DIR:-${ROOT_DIR}/config/deploy}"
WAIT_FOR_ROLLOUT="${WAIT_FOR_ROLLOUT:-false}"

GATEWAY_VALUES_FILE="${GATEWAY_VALUES_FILE:-${VALUES_DIR}/gateway-prod-values.yaml}"
ADMIN_FRONTEND_VALUES_FILE="${ADMIN_FRONTEND_VALUES_FILE:-${VALUES_DIR}/admin-frontend-prod-values.yaml}"
FALLBACK_GATEWAY_VALUES_FILE="${FALLBACK_GATEWAY_VALUES_FILE:-${FALLBACK_VALUES_DIR}/gateway-prod-values.yaml}"
FALLBACK_ADMIN_FRONTEND_VALUES_FILE="${FALLBACK_ADMIN_FRONTEND_VALUES_FILE:-${FALLBACK_VALUES_DIR}/admin-frontend-prod-values.yaml}"

if ! command -v helm >/dev/null 2>&1; then
  echo "helm is required but was not found in PATH" >&2
  exit 1
fi

if ! command -v kubectl >/dev/null 2>&1; then
  echo "kubectl is required but was not found in PATH" >&2
  exit 1
fi

if [[ ! -f "${GATEWAY_VALUES_FILE}" ]]; then
  if [[ -f "${FALLBACK_GATEWAY_VALUES_FILE}" ]]; then
    echo "missing gateway values file: ${GATEWAY_VALUES_FILE}; using fallback ${FALLBACK_GATEWAY_VALUES_FILE}"
    GATEWAY_VALUES_FILE="${FALLBACK_GATEWAY_VALUES_FILE}"
  else
    echo "missing gateway values file: ${GATEWAY_VALUES_FILE}" >&2
    echo "missing gateway fallback values file: ${FALLBACK_GATEWAY_VALUES_FILE}" >&2
    exit 1
  fi
fi

if [[ ! -f "${ADMIN_FRONTEND_VALUES_FILE}" ]]; then
  if [[ -f "${FALLBACK_ADMIN_FRONTEND_VALUES_FILE}" ]]; then
    echo "missing admin frontend values file: ${ADMIN_FRONTEND_VALUES_FILE}; using fallback ${FALLBACK_ADMIN_FRONTEND_VALUES_FILE}"
    ADMIN_FRONTEND_VALUES_FILE="${FALLBACK_ADMIN_FRONTEND_VALUES_FILE}"
  else
    echo "missing admin frontend values file: ${ADMIN_FRONTEND_VALUES_FILE}" >&2
    echo "missing admin frontend fallback values file: ${FALLBACK_ADMIN_FRONTEND_VALUES_FILE}" >&2
    exit 1
  fi
fi

declare -a gateway_args=("-f" "${GATEWAY_VALUES_FILE}")
declare -a admin_frontend_args=("-f" "${ADMIN_FRONTEND_VALUES_FILE}")

if [[ -n "${GATEWAY_IMAGE_REPOSITORY:-}" ]]; then
  gateway_args+=("--set" "image.repository=${GATEWAY_IMAGE_REPOSITORY}")
fi
if [[ -n "${GATEWAY_IMAGE_TAG:-}" ]]; then
  gateway_args+=("--set" "image.tag=${GATEWAY_IMAGE_TAG}")
fi
if [[ -n "${GATEWAY_SERVICE_PORT:-}" ]]; then
  gateway_args+=("--set" "service.port=${GATEWAY_SERVICE_PORT}")
fi

if [[ -n "${ADMIN_FRONTEND_IMAGE_REPOSITORY:-}" ]]; then
  admin_frontend_args+=("--set" "image.repository=${ADMIN_FRONTEND_IMAGE_REPOSITORY}")
fi
if [[ -n "${ADMIN_FRONTEND_IMAGE_TAG:-}" ]]; then
  admin_frontend_args+=("--set" "image.tag=${ADMIN_FRONTEND_IMAGE_TAG}")
fi
if [[ -n "${ADMIN_FRONTEND_SERVICE_PORT:-}" ]]; then
  admin_frontend_args+=("--set" "service.port=${ADMIN_FRONTEND_SERVICE_PORT}")
fi

deploy_chart() {
  local release="$1"
  local chart_path="$2"
  shift 2
  echo "==> Deploying ${release}"
  helm upgrade --install "${release}" "${chart_path}" -n "${NAMESPACE}" "$@"
}

deploy_chart "brevio-gateway" "${ROOT_DIR}/helm/BREVIO-gateway" "${gateway_args[@]}"
deploy_chart "brevio-brain" "${ROOT_DIR}/helm/BREVIO-brain"
deploy_chart "brevio-control" "${ROOT_DIR}/helm/BREVIO-control"
deploy_chart "brevio-executor" "${ROOT_DIR}/helm/BREVIO-executor"
deploy_chart "brevio-canvas" "${ROOT_DIR}/helm/BREVIO-canvas"
deploy_chart "brevio-temporal-worker" "${ROOT_DIR}/helm/BREVIO-temporal-worker"
deploy_chart "brevio-admin-api" "${ROOT_DIR}/helm/BREVIO-admin-api"
deploy_chart "brevio-admin-frontend" "${ROOT_DIR}/helm/BREVIO-admin-frontend" "${admin_frontend_args[@]}"
deploy_chart "brevio-rag-worker" "${ROOT_DIR}/helm/BREVIO-rag-worker"
deploy_chart "brevio-guardrails" "${ROOT_DIR}/helm/BREVIO-guardrails"
deploy_chart "brevio-health-checker" "${ROOT_DIR}/helm/BREVIO-health-checker"

if [[ "${WAIT_FOR_ROLLOUT}" == "true" ]]; then
  echo "==> Waiting for deployment rollouts"
  while read -r deployment; do
    if kubectl get deployment "${deployment}" -n "${NAMESPACE}" >/dev/null 2>&1; then
      kubectl rollout status "deployment/${deployment}" -n "${NAMESPACE}" --timeout=300s
    fi
  done <<'EOF'
brevio-gateway
brevio-brain
brevio-control
brevio-executor
brevio-canvas
brevio-temporal-worker
brevio-admin-api
brevio-admin-frontend
brevio-rag-worker
brevio-guardrails
brevio-health-checker
EOF
fi

echo "==> Current ingress resources"
kubectl get ingress -n "${NAMESPACE}" -o wide || true

echo "==> Rollout command completed"
