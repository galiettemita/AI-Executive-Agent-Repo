#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

if ! command -v kubectl >/dev/null 2>&1; then
  echo "kubectl is required for staging smoke tests" >&2
  exit 1
fi
if ! command -v curl >/dev/null 2>&1; then
  echo "curl is required for staging smoke tests" >&2
  exit 1
fi
if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 is required for staging smoke tests" >&2
  exit 1
fi

NAMESPACE="${NAMESPACE:-default}"
OUTPUT_PATH="${OUTPUT_PATH:-artifacts/deploy/staging_smoke_test_report.json}"
RESULTS_FILE="$(mktemp)"
TMP_DIR="$(mktemp -d)"
LOCAL_PORT_BASE="${LOCAL_PORT_BASE:-18080}"

cleanup() {
  rm -f "$RESULTS_FILE"
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

mkdir -p "$(dirname "$OUTPUT_PATH")"

append_result() {
  local id="$1"
  local status="$2"
  local detail="$3"
  printf '%s\t%s\t%s\n' "$id" "$status" "$detail" >>"$RESULTS_FILE"
}

next_local_port() {
  local current="$LOCAL_PORT_BASE"
  LOCAL_PORT_BASE=$((LOCAL_PORT_BASE + 1))
  echo "$current"
}

deployment_ready_check() {
  local deployment="$1"
  local id="deployment_${deployment}"

  if ! kubectl get deployment "$deployment" -n "$NAMESPACE" >/dev/null 2>&1; then
    append_result "$id" "FAIL" "deployment not found"
    return
  fi

  local replicas ready available
  replicas="$(kubectl get deployment "$deployment" -n "$NAMESPACE" -o jsonpath='{.status.replicas}' 2>/dev/null || true)"
  ready="$(kubectl get deployment "$deployment" -n "$NAMESPACE" -o jsonpath='{.status.readyReplicas}' 2>/dev/null || true)"
  available="$(kubectl get deployment "$deployment" -n "$NAMESPACE" -o jsonpath='{.status.availableReplicas}' 2>/dev/null || true)"

  replicas="${replicas:-0}"
  ready="${ready:-0}"
  available="${available:-0}"

  if [[ "$ready" =~ ^[0-9]+$ ]] && [[ "$available" =~ ^[0-9]+$ ]] && (( ready >= 1 )) && (( available >= 1 )); then
    append_result "$id" "PASS" "ready=${ready} available=${available} replicas=${replicas}"
  else
    append_result "$id" "FAIL" "not ready (ready=${ready} available=${available} replicas=${replicas})"
  fi
}

service_port() {
  local service="$1"
  kubectl get service "$service" -n "$NAMESPACE" -o jsonpath='{.spec.ports[0].port}' 2>/dev/null || true
}

probe_get() {
  local service="$1"
  local path="$2"
  local id="$3"
  local mode="$4"

  local port
  port="$(service_port "$service")"
  if [[ -z "$port" ]]; then
    append_result "$id" "FAIL" "service not found or has no ports"
    return
  fi

  local local_port pf_log pf_pid code
  local_port="$(next_local_port)"
  pf_log="$TMP_DIR/portforward-${service}-${local_port}.log"

  kubectl port-forward -n "$NAMESPACE" "svc/$service" "${local_port}:${port}" >"$pf_log" 2>&1 &
  pf_pid=$!
  sleep 2

  if ! kill -0 "$pf_pid" >/dev/null 2>&1; then
    append_result "$id" "FAIL" "port-forward failed ($(tr '\n' ' ' <"$pf_log"))"
    return
  fi

  code="$(curl -sS -o /dev/null -w '%{http_code}' --max-time 10 "http://127.0.0.1:${local_port}${path}" || true)"

  kill "$pf_pid" >/dev/null 2>&1 || true
  wait "$pf_pid" >/dev/null 2>&1 || true

  case "$mode" in
    expect_200)
      if [[ "$code" == "200" ]]; then
        append_result "$id" "PASS" "HTTP 200"
      else
        append_result "$id" "FAIL" "expected 200, got ${code}"
      fi
      ;;
    expect_route)
      case "$code" in
        200|400|401|403|405)
          append_result "$id" "PASS" "route exists (HTTP ${code})"
          ;;
        *)
          append_result "$id" "FAIL" "route missing/unexpected (HTTP ${code})"
          ;;
      esac
      ;;
    *)
      append_result "$id" "FAIL" "invalid probe mode"
      ;;
  esac
}

probe_post() {
  local service="$1"
  local path="$2"
  local id="$3"
  local expected_code="$4"
  local body="$5"

  local port
  port="$(service_port "$service")"
  if [[ -z "$port" ]]; then
    append_result "$id" "FAIL" "service not found or has no ports"
    return
  fi

  local local_port pf_log pf_pid code
  local_port="$(next_local_port)"
  pf_log="$TMP_DIR/portforward-${service}-${local_port}.log"

  kubectl port-forward -n "$NAMESPACE" "svc/$service" "${local_port}:${port}" >"$pf_log" 2>&1 &
  pf_pid=$!
  sleep 2

  if ! kill -0 "$pf_pid" >/dev/null 2>&1; then
    append_result "$id" "FAIL" "port-forward failed ($(tr '\n' ' ' <"$pf_log"))"
    return
  fi

  code="$(curl -sS -o /dev/null -w '%{http_code}' --max-time 10 -H 'content-type: application/json' -d "$body" "http://127.0.0.1:${local_port}${path}" || true)"

  kill "$pf_pid" >/dev/null 2>&1 || true
  wait "$pf_pid" >/dev/null 2>&1 || true

  if [[ "$code" == "$expected_code" ]]; then
    append_result "$id" "PASS" "HTTP ${code}"
  else
    append_result "$id" "FAIL" "expected ${expected_code}, got ${code}"
  fi
}

deployments=(
  "brevio-gateway"
  "brevio-brain"
  "brevio-control"
  "brevio-executor"
  "brevio-canvas"
  "brevio-temporal-worker"
)

for deployment in "${deployments[@]}"; do
  deployment_ready_check "$deployment"
done

probe_get "brevio-gateway" "/health" "gateway_health" "expect_200"
probe_get "brevio-gateway" "/health/deep" "gateway_health_deep" "expect_200"
probe_get "brevio-gateway" "/api/v1/webhooks/whatsapp" "gateway_webhook_route" "expect_route"

SMOKE_MESSAGE_ID="smoke-$(date -u +%Y%m%d%H%M%S)"
probe_post "brevio-temporal-worker" "/api/v1/temporal-worker/workflows/message-processing" "temporal_message_workflow_start" "202" "{\"message_id\":\"${SMOKE_MESSAGE_ID}\",\"user_id\":\"staging-smoke\"}"

python3 - "$RESULTS_FILE" "$OUTPUT_PATH" "$NAMESPACE" <<'PY'
import json
import sys
from datetime import datetime, timezone

results_path, output_path, namespace = sys.argv[1:4]

results = []
with open(results_path, "r", encoding="utf-8") as fh:
    for raw in fh:
        line = raw.rstrip("\n")
        if not line:
            continue
        parts = line.split("\t", 2)
        if len(parts) != 3:
            continue
        check_id, status, detail = parts
        results.append(
            {
                "id": check_id,
                "status": status,
                "detail": detail,
            }
        )

failed = [item for item in results if item["status"] == "FAIL"]
passed = [item for item in results if item["status"] == "PASS"]
status = "PASS" if not failed else "FAIL"

payload = {
    "generated_at_utc": datetime.now(timezone.utc).isoformat(),
    "namespace": namespace,
    "status": status,
    "summary": {
        "total": len(results),
        "passed": len(passed),
        "failed": len(failed),
    },
    "results": results,
}

with open(output_path, "w", encoding="utf-8") as fh:
    json.dump(payload, fh, indent=2)
    fh.write("\n")

print(json.dumps(payload, indent=2))

if failed:
    sys.exit(1)
sys.exit(0)
PY

echo "$OUTPUT_PATH"
