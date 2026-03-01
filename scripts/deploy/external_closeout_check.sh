#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

if ! command -v aws >/dev/null 2>&1; then
  echo "aws CLI is required for external closeout checks" >&2
  exit 1
fi

if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 is required for external closeout checks" >&2
  exit 1
fi

REGION="${AWS_REGION:-${REGION:-us-east-1}}"
OUTPUT_PATH="${OUTPUT_PATH:-artifacts/deploy/external_closeout_status.json}"

PLAID_OAUTH_SECRET_NAME="${PLAID_OAUTH_SECRET_NAME:-executive-os/prod/oauth_client_secrets}"
GATEWAY_WEBHOOK_SECRET_NAME="${GATEWAY_WEBHOOK_SECRET_NAME:-executive-os/prod/gateway_webhook_secret}"
BILLING_SECRET_NAME="${BILLING_SECRET_NAME:-executive-os/prod/oauth_client_secrets}"
DOC_PARSING_SECRET_NAME="${DOC_PARSING_SECRET_NAME:-executive-os/prod/oauth_client_secrets}"
ALERTING_SECRET_NAME="${ALERTING_SECRET_NAME:-executive-os/prod/oauth_client_secrets}"
REMOTE_CATALOG_SECRET_NAME="${REMOTE_CATALOG_SECRET_NAME:-executive-os/prod/oauth_client_secrets}"
ANALYTICS_EVENT_BUS="${ANALYTICS_EVENT_BUS:-}"
LOCAL_LLM_ENDPOINT="${LOCAL_LLM_ENDPOINT:-}"
ELEVENLABS_API_KEY="${ELEVENLABS_API_KEY:-}"

PARTNER_APPS_CONFIRMED="${PARTNER_APPS_CONFIRMED:-0}"

TMP_FILE="$(mktemp)"
cleanup() {
  rm -f "$TMP_FILE"
}
trap cleanup EXIT

append_result() {
  local id="$1"
  local required="$2"
  local status="$3"
  local detail="$4"
  python3 - "$TMP_FILE" "$id" "$required" "$status" "$detail" <<'PY'
import json
import os
import sys

path, item_id, required, status, detail = sys.argv[1:6]
payload = {"results": []}
if os.path.exists(path):
    with open(path, "r", encoding="utf-8") as fh:
        raw = fh.read().strip()
        if raw:
            payload = json.loads(raw)
payload.setdefault("results", [])
payload["results"].append(
    {
        "id": item_id,
        "required": required.lower() == "true",
        "status": status,
        "detail": detail,
    }
)
with open(path, "w", encoding="utf-8") as fh:
    json.dump(payload, fh)
PY
}

secret_exists() {
  local secret_name="$1"
  if aws --region "$REGION" secretsmanager describe-secret --secret-id "$secret_name" >/dev/null 2>&1; then
    return 0
  fi
  return 1
}

secret_has_nonempty_key() {
  local secret_name="$1"
  local key_name="$2"
  local secret_string
  if ! secret_string="$(aws --region "$REGION" secretsmanager get-secret-value --secret-id "$secret_name" --query SecretString --output text 2>/dev/null)"; then
    return 1
  fi
  python3 - "$secret_string" "$key_name" <<'PY'
import json
import sys

raw = sys.argv[1]
key = sys.argv[2]
try:
    data = json.loads(raw)
except Exception:
    sys.exit(1)
value = data.get(key)
if isinstance(value, str) and value.strip():
    sys.exit(0)
sys.exit(1)
PY
}

event_bus_exists() {
  local bus="$1"
  if [[ -z "$bus" ]]; then
    return 1
  fi
  if aws --region "$REGION" events describe-event-bus --name "$bus" >/dev/null 2>&1; then
    return 0
  fi
  return 1
}

echo '{}' >"$TMP_FILE"

if [[ "$PARTNER_APPS_CONFIRMED" == "1" ]]; then
  append_result "partner_applications_submitted" "true" "pass" "PARTNER_APPS_CONFIRMED=1"
else
  append_result "partner_applications_submitted" "true" "manual" "Set PARTNER_APPS_CONFIRMED=1 after submitting Zoom/Instacart/Canva/Booking.com apps."
fi

if secret_exists "$PLAID_OAUTH_SECRET_NAME" && secret_has_nonempty_key "$PLAID_OAUTH_SECRET_NAME" "PLAID_SECRET_PROD"; then
  append_result "plaid_secret_prod" "true" "pass" "Found PLAID_SECRET_PROD in ${PLAID_OAUTH_SECRET_NAME}"
else
  append_result "plaid_secret_prod" "true" "fail" "Missing PLAID_SECRET_PROD in ${PLAID_OAUTH_SECRET_NAME}"
fi

if secret_exists "$GATEWAY_WEBHOOK_SECRET_NAME" && secret_has_nonempty_key "$GATEWAY_WEBHOOK_SECRET_NAME" "PLAID_WEBHOOK_SECRET"; then
  append_result "plaid_webhook_secret" "true" "pass" "Found PLAID_WEBHOOK_SECRET in ${GATEWAY_WEBHOOK_SECRET_NAME}"
else
  append_result "plaid_webhook_secret" "true" "fail" "Missing PLAID_WEBHOOK_SECRET in ${GATEWAY_WEBHOOK_SECRET_NAME}"
fi

if secret_exists "$BILLING_SECRET_NAME" && secret_has_nonempty_key "$BILLING_SECRET_NAME" "STRIPE_SECRET_KEY" && secret_has_nonempty_key "$BILLING_SECRET_NAME" "STRIPE_WEBHOOK_SECRET"; then
  append_result "stripe_billing_keys" "true" "pass" "Found STRIPE_SECRET_KEY + STRIPE_WEBHOOK_SECRET in ${BILLING_SECRET_NAME}"
else
  append_result "stripe_billing_keys" "true" "fail" "Missing STRIPE_SECRET_KEY and/or STRIPE_WEBHOOK_SECRET in ${BILLING_SECRET_NAME}"
fi

if secret_exists "$DOC_PARSING_SECRET_NAME" && secret_has_nonempty_key "$DOC_PARSING_SECRET_NAME" "UNSTRUCTURED_API_KEY"; then
  append_result "unstructured_api_key" "true" "pass" "Found UNSTRUCTURED_API_KEY in ${DOC_PARSING_SECRET_NAME}"
else
  append_result "unstructured_api_key" "true" "fail" "Missing UNSTRUCTURED_API_KEY in ${DOC_PARSING_SECRET_NAME}"
fi

if secret_exists "$ALERTING_SECRET_NAME" && secret_has_nonempty_key "$ALERTING_SECRET_NAME" "PAGERDUTY_ROUTING_KEY"; then
  append_result "pagerduty_routing_key" "true" "pass" "Found PAGERDUTY_ROUTING_KEY in ${ALERTING_SECRET_NAME}"
else
  append_result "pagerduty_routing_key" "true" "fail" "Missing PAGERDUTY_ROUTING_KEY in ${ALERTING_SECRET_NAME}"
fi

if event_bus_exists "$ANALYTICS_EVENT_BUS"; then
  append_result "analytics_event_bus" "true" "pass" "EventBridge bus exists: ${ANALYTICS_EVENT_BUS}"
else
  append_result "analytics_event_bus" "true" "fail" "Missing/invalid ANALYTICS_EVENT_BUS (${ANALYTICS_EVENT_BUS:-unset})"
fi

if secret_exists "$REMOTE_CATALOG_SECRET_NAME" && secret_has_nonempty_key "$REMOTE_CATALOG_SECRET_NAME" "REMOTE_CATALOG_PRIVATE_KEY" && secret_has_nonempty_key "$REMOTE_CATALOG_SECRET_NAME" "REMOTE_CATALOG_PUBLIC_KEY"; then
  append_result "remote_catalog_signing_keys" "true" "pass" "Found REMOTE_CATALOG_PRIVATE_KEY + REMOTE_CATALOG_PUBLIC_KEY in ${REMOTE_CATALOG_SECRET_NAME}"
else
  append_result "remote_catalog_signing_keys" "true" "fail" "Missing REMOTE_CATALOG_PRIVATE_KEY and/or REMOTE_CATALOG_PUBLIC_KEY in ${REMOTE_CATALOG_SECRET_NAME}"
fi

if [[ -n "$LOCAL_LLM_ENDPOINT" ]]; then
  append_result "local_llm_endpoint" "false" "pass" "LOCAL_LLM_ENDPOINT set"
else
  append_result "local_llm_endpoint" "false" "skip" "Optional item unset"
fi

if [[ -n "$ELEVENLABS_API_KEY" ]]; then
  append_result "elevenlabs_api_key" "false" "pass" "ELEVENLABS_API_KEY set"
else
  append_result "elevenlabs_api_key" "false" "skip" "Optional item unset"
fi

mkdir -p "$(dirname "$OUTPUT_PATH")"
python3 - "$TMP_FILE" "$OUTPUT_PATH" "$REGION" <<'PY'
import json
import os
import sys
from datetime import datetime, timezone

src, out, region = sys.argv[1:4]
with open(src, "r", encoding="utf-8") as fh:
    payload = json.load(fh)
results = payload.get("results", [])
required = [r for r in results if r.get("required")]
summary = {
    "generated_at_utc": datetime.now(timezone.utc).isoformat(),
    "region": region,
    "required_total": len(required),
    "required_passed": sum(1 for r in required if r.get("status") == "pass"),
    "required_failed": sum(1 for r in required if r.get("status") == "fail"),
    "required_manual": sum(1 for r in required if r.get("status") == "manual"),
    "results": results,
}
os.makedirs(os.path.dirname(out), exist_ok=True)
with open(out, "w", encoding="utf-8") as fh:
    json.dump(summary, fh, indent=2)
    fh.write("\n")
print(json.dumps(summary, indent=2))
PY

if python3 - "$OUTPUT_PATH" <<'PY'
import json
import sys
with open(sys.argv[1], "r", encoding="utf-8") as fh:
    data = json.load(fh)
if data.get("required_failed", 0) > 0:
    sys.exit(1)
PY
then
  echo "external closeout checks passed (required items)."
else
  echo "external closeout checks failed; see ${OUTPUT_PATH}" >&2
  exit 1
fi
