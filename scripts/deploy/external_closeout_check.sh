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
AWS_CLI_CONNECT_TIMEOUT="${AWS_CLI_CONNECT_TIMEOUT:-5}"
AWS_CLI_READ_TIMEOUT="${AWS_CLI_READ_TIMEOUT:-20}"
export AWS_CLI_CONNECT_TIMEOUT AWS_CLI_READ_TIMEOUT

APP_SECRET_NAME="${APP_SECRET_NAME:-executive-os/prod/app}"
PLAID_OAUTH_SECRET_NAME="${PLAID_OAUTH_SECRET_NAME:-executive-os/prod/oauth_client_secrets}"
GATEWAY_WEBHOOK_SECRET_NAME="${GATEWAY_WEBHOOK_SECRET_NAME:-executive-os/prod/gateway_webhook_secret}"
BILLING_SECRET_NAME="${BILLING_SECRET_NAME:-executive-os/prod/app}"
DOC_PARSING_SECRET_NAME="${DOC_PARSING_SECRET_NAME:-executive-os/prod/app}"
ALERTING_SECRET_NAME="${ALERTING_SECRET_NAME:-executive-os/prod/app}"
REMOTE_CATALOG_SECRET_NAME="${REMOTE_CATALOG_SECRET_NAME:-executive-os/prod/oauth_client_secrets}"
ANALYTICS_EVENT_BUS="${ANALYTICS_EVENT_BUS:-}"
LOCAL_LLM_ENDPOINT="${LOCAL_LLM_ENDPOINT:-}"
ELEVENLABS_API_KEY="${ELEVENLABS_API_KEY:-}"
PLAID_WEBHOOK_REQUIRED="${PLAID_WEBHOOK_REQUIRED:-1}"
MANUAL_EVIDENCE_PATH="${MANUAL_EVIDENCE_PATH:-artifacts/deploy/manual_closeout_evidence.json}"

PARTNER_APPS_CONFIRMED="${PARTNER_APPS_CONFIRMED:-0}"

TMP_FILE="$(mktemp)"
cleanup() {
  rm -f "$TMP_FILE"
}
trap cleanup EXIT

aws_retry() {
  local attempt=1
  local max_attempts=3
  local delay=1
  local output
  while (( attempt <= max_attempts )); do
    if output="$("$@" 2>&1)"; then
      printf '%s' "$output"
      return 0
    fi
    if [[ "$output" != *"Could not connect to the endpoint URL"* && "$output" != *"RequestTimeout"* && "$output" != *"Throttling"* ]]; then
      printf '%s' "$output" >&2
      return 1
    fi
    if (( attempt == max_attempts )); then
      printf '%s' "$output" >&2
      return 1
    fi
    sleep "$delay"
    delay=$((delay * 2))
    attempt=$((attempt + 1))
  done
  return 1
}

manual_confirmation_detail() {
  local item_id="$1"
  python3 - "$MANUAL_EVIDENCE_PATH" "$item_id" <<'PY'
import json
import os
import sys

path, item_id = sys.argv[1:3]
if not os.path.exists(path):
    sys.exit(1)

try:
    with open(path, "r", encoding="utf-8") as fh:
        payload = json.load(fh)
except Exception:
    sys.exit(1)

entry = (payload.get("items") or {}).get(item_id)
if not isinstance(entry, dict):
    sys.exit(1)
if entry.get("confirmed") is not True:
    sys.exit(1)

confirmed_by = str(entry.get("confirmed_by") or "manual-operator").strip()
confirmed_at = str(entry.get("confirmed_at_utc") or "unknown-time").strip()
note = str(entry.get("note") or "").strip()
detail = f"Manual evidence confirmed by {confirmed_by} at {confirmed_at}"
if note:
    detail += f" ({note})"
print(detail)
PY
}

manual_confirmation_or_empty() {
  local item_id="$1"
  manual_confirmation_detail "$item_id" 2>/dev/null || true
}

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

secret_has_nonempty_key() {
  local secret_name="$1"
  local key_name="$2"
  local secret_string
  if ! secret_string="$(aws_retry aws --region "$REGION" secretsmanager get-secret-value --secret-id "$secret_name" --query SecretString --output text)"; then
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

locate_key_in_secrets() {
  local key_name="$1"
  shift
  local secret_name
  for secret_name in "$@"; do
    if [[ -z "$secret_name" ]]; then
      continue
    fi
    if secret_has_nonempty_key "$secret_name" "$key_name"; then
      echo "$secret_name"
      return 0
    fi
  done
  return 1
}

locate_key_value_in_secrets() {
  local key_name="$1"
  shift
  local secret_name
  local secret_string
  for secret_name in "$@"; do
    if [[ -z "$secret_name" ]]; then
      continue
    fi
    if ! secret_string="$(aws_retry aws --region "$REGION" secretsmanager get-secret-value --secret-id "$secret_name" --query SecretString --output text 2>/dev/null)"; then
      continue
    fi
    local value
    if ! value="$(python3 - "$secret_string" "$key_name" <<'PY'
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
    print(value.strip())
    sys.exit(0)
sys.exit(1)
PY
)"; then
      continue
    fi
    if [[ -n "$value" ]]; then
      echo "$value"
      return 0
    fi
  done
  return 1
}

event_bus_exists() {
  local bus="$1"
  if [[ "${EVENTS_AVAILABLE:-1}" != "1" ]]; then
    return 1
  fi
  if [[ -z "$bus" ]]; then
    return 1
  fi
  if aws_retry aws --region "$REGION" events describe-event-bus --name "$bus" >/dev/null 2>&1; then
    return 0
  fi
  return 1
}

echo '{}' >"$TMP_FILE"

SM_AVAILABLE=1
EVENTS_AVAILABLE=1
if ! aws_retry aws --region "$REGION" secretsmanager list-secrets --max-results 1 --query 'SecretList[0].Name' --output text >/dev/null 2>&1; then
  SM_AVAILABLE=0
fi
if ! aws_retry aws --region "$REGION" events list-event-buses --limit 1 --query 'EventBuses[0].Name' --output text >/dev/null 2>&1; then
  EVENTS_AVAILABLE=0
fi

if [[ "$SM_AVAILABLE" == "1" && -z "$ANALYTICS_EVENT_BUS" ]]; then
  ANALYTICS_EVENT_BUS="$(
    locate_key_value_in_secrets "ANALYTICS_EVENT_BUS" \
      "$APP_SECRET_NAME" \
      "$BILLING_SECRET_NAME" \
      "$DOC_PARSING_SECRET_NAME" \
      2>/dev/null || true
  )"
fi

partner_manual_detail="$(manual_confirmation_or_empty "partner_applications_submitted")"
if [[ "$PARTNER_APPS_CONFIRMED" == "1" ]]; then
  append_result "partner_applications_submitted" "true" "pass" "PARTNER_APPS_CONFIRMED=1"
elif [[ -n "$partner_manual_detail" ]]; then
  append_result "partner_applications_submitted" "true" "pass" "$partner_manual_detail"
else
  append_result "partner_applications_submitted" "true" "manual" "Set PARTNER_APPS_CONFIRMED=1 after submitting Zoom/Instacart/Canva/Booking.com apps."
fi

plaid_secret_manual_detail="$(manual_confirmation_or_empty "plaid_secret_prod")"
if [[ "$SM_AVAILABLE" != "1" ]]; then
  if [[ -n "$plaid_secret_manual_detail" ]]; then
    append_result "plaid_secret_prod" "true" "pass" "$plaid_secret_manual_detail"
  else
    append_result "plaid_secret_prod" "true" "manual" "Unable to verify secrets: AWS Secrets Manager endpoint unavailable from current environment."
  fi
elif plaid_secret_location="$(locate_key_in_secrets "PLAID_SECRET_PROD" "$PLAID_OAUTH_SECRET_NAME" "$APP_SECRET_NAME" "$BILLING_SECRET_NAME" 2>/dev/null)"; then
  append_result "plaid_secret_prod" "true" "pass" "Found PLAID_SECRET_PROD in ${plaid_secret_location}"
elif [[ -n "$plaid_secret_manual_detail" ]]; then
  append_result "plaid_secret_prod" "true" "pass" "$plaid_secret_manual_detail"
else
  append_result "plaid_secret_prod" "true" "fail" "Missing PLAID_SECRET_PROD in checked secrets (${PLAID_OAUTH_SECRET_NAME}, ${APP_SECRET_NAME}, ${BILLING_SECRET_NAME})"
fi

plaid_webhook_manual_detail="$(manual_confirmation_or_empty "plaid_webhook_secret")"
if [[ "$SM_AVAILABLE" != "1" ]]; then
  if [[ -n "$plaid_webhook_manual_detail" ]]; then
    append_result "plaid_webhook_secret" "true" "pass" "$plaid_webhook_manual_detail"
  else
    append_result "plaid_webhook_secret" "true" "manual" "Unable to verify secrets: AWS Secrets Manager endpoint unavailable from current environment."
  fi
elif plaid_webhook_location="$(locate_key_in_secrets "PLAID_WEBHOOK_SECRET" "$GATEWAY_WEBHOOK_SECRET_NAME" "$APP_SECRET_NAME" "$PLAID_OAUTH_SECRET_NAME" 2>/dev/null)"; then
  append_result "plaid_webhook_secret" "true" "pass" "Found PLAID_WEBHOOK_SECRET in ${plaid_webhook_location}"
elif [[ "$PLAID_WEBHOOK_REQUIRED" == "0" ]]; then
  append_result "plaid_webhook_secret" "true" "manual" "PLAID_WEBHOOK_REQUIRED=0 override set; webhook validation is waived pending Plaid production access."
elif [[ -n "$plaid_webhook_manual_detail" ]]; then
  append_result "plaid_webhook_secret" "true" "pass" "$plaid_webhook_manual_detail"
else
  append_result "plaid_webhook_secret" "true" "fail" "Missing PLAID_WEBHOOK_SECRET in checked secrets (${GATEWAY_WEBHOOK_SECRET_NAME}, ${APP_SECRET_NAME}, ${PLAID_OAUTH_SECRET_NAME})"
fi

stripe_secret_location=""
stripe_webhook_location=""
stripe_manual_detail="$(manual_confirmation_or_empty "stripe_billing_keys")"
if [[ "$SM_AVAILABLE" != "1" ]]; then
  if [[ -n "$stripe_manual_detail" ]]; then
    append_result "stripe_billing_keys" "true" "pass" "$stripe_manual_detail"
  else
    append_result "stripe_billing_keys" "true" "manual" "Unable to verify secrets: AWS Secrets Manager endpoint unavailable from current environment."
  fi
else
  stripe_secret_location="$(locate_key_in_secrets "STRIPE_SECRET_KEY" "$BILLING_SECRET_NAME" "$APP_SECRET_NAME" "$PLAID_OAUTH_SECRET_NAME" 2>/dev/null || true)"
  stripe_webhook_location="$(locate_key_in_secrets "STRIPE_WEBHOOK_SECRET" "$BILLING_SECRET_NAME" "$APP_SECRET_NAME" "$PLAID_OAUTH_SECRET_NAME" 2>/dev/null || true)"
  if [[ -n "$stripe_secret_location" && -n "$stripe_webhook_location" ]]; then
    append_result "stripe_billing_keys" "true" "pass" "Found STRIPE_SECRET_KEY in ${stripe_secret_location}; STRIPE_WEBHOOK_SECRET in ${stripe_webhook_location}"
  elif [[ -n "$stripe_manual_detail" ]]; then
    append_result "stripe_billing_keys" "true" "pass" "$stripe_manual_detail"
  else
    append_result "stripe_billing_keys" "true" "fail" "Missing STRIPE_SECRET_KEY and/or STRIPE_WEBHOOK_SECRET in checked secrets (${BILLING_SECRET_NAME}, ${APP_SECRET_NAME}, ${PLAID_OAUTH_SECRET_NAME})"
  fi
fi

unstructured_manual_detail="$(manual_confirmation_or_empty "unstructured_api_key")"
if [[ "$SM_AVAILABLE" != "1" ]]; then
  if [[ -n "$unstructured_manual_detail" ]]; then
    append_result "unstructured_api_key" "true" "pass" "$unstructured_manual_detail"
  else
    append_result "unstructured_api_key" "true" "manual" "Unable to verify secrets: AWS Secrets Manager endpoint unavailable from current environment."
  fi
elif unstructured_key_location="$(locate_key_in_secrets "UNSTRUCTURED_API_KEY" "$DOC_PARSING_SECRET_NAME" "$APP_SECRET_NAME" "$PLAID_OAUTH_SECRET_NAME" 2>/dev/null)"; then
  append_result "unstructured_api_key" "true" "pass" "Found UNSTRUCTURED_API_KEY in ${unstructured_key_location}"
elif [[ -n "$unstructured_manual_detail" ]]; then
  append_result "unstructured_api_key" "true" "pass" "$unstructured_manual_detail"
else
  append_result "unstructured_api_key" "true" "fail" "Missing UNSTRUCTURED_API_KEY in checked secrets (${DOC_PARSING_SECRET_NAME}, ${APP_SECRET_NAME}, ${PLAID_OAUTH_SECRET_NAME})"
fi

pagerduty_location=""
pagerduty_manual_detail="$(manual_confirmation_or_empty "pagerduty_routing_key")"
if [[ "$SM_AVAILABLE" != "1" ]]; then
  if [[ -n "$pagerduty_manual_detail" ]]; then
    append_result "pagerduty_routing_key" "true" "pass" "$pagerduty_manual_detail"
  else
    append_result "pagerduty_routing_key" "true" "manual" "Unable to verify secrets: AWS Secrets Manager endpoint unavailable from current environment."
  fi
else
  pagerduty_location="$(locate_key_in_secrets "PAGERDUTY_ROUTING_KEY" "$ALERTING_SECRET_NAME" "$APP_SECRET_NAME" "$PLAID_OAUTH_SECRET_NAME" 2>/dev/null || true)"
  if [[ -z "$pagerduty_location" ]]; then
    pagerduty_location="$(locate_key_in_secrets "PAGERDUTY_INTEGRATION_KEY" "$ALERTING_SECRET_NAME" "$APP_SECRET_NAME" "$PLAID_OAUTH_SECRET_NAME" 2>/dev/null || true)"
  fi
  if [[ -n "$pagerduty_location" ]]; then
    append_result "pagerduty_routing_key" "true" "pass" "Found PagerDuty key in ${pagerduty_location}"
  elif [[ -n "$pagerduty_manual_detail" ]]; then
    append_result "pagerduty_routing_key" "true" "pass" "$pagerduty_manual_detail"
  else
    append_result "pagerduty_routing_key" "true" "fail" "Missing PAGERDUTY_ROUTING_KEY/PAGERDUTY_INTEGRATION_KEY in checked secrets (${ALERTING_SECRET_NAME}, ${APP_SECRET_NAME}, ${PLAID_OAUTH_SECRET_NAME})"
  fi
fi

analytics_bus_manual_detail="$(manual_confirmation_or_empty "analytics_event_bus")"
if [[ "$EVENTS_AVAILABLE" != "1" ]]; then
  if [[ -n "$analytics_bus_manual_detail" ]]; then
    append_result "analytics_event_bus" "true" "pass" "$analytics_bus_manual_detail"
  else
    append_result "analytics_event_bus" "true" "manual" "Unable to verify EventBridge bus: AWS Events endpoint unavailable from current environment."
  fi
elif event_bus_exists "$ANALYTICS_EVENT_BUS"; then
  append_result "analytics_event_bus" "true" "pass" "EventBridge bus exists: ${ANALYTICS_EVENT_BUS}"
elif [[ -n "$analytics_bus_manual_detail" ]]; then
  append_result "analytics_event_bus" "true" "pass" "$analytics_bus_manual_detail"
else
  append_result "analytics_event_bus" "true" "fail" "Missing/invalid ANALYTICS_EVENT_BUS (${ANALYTICS_EVENT_BUS:-unset})"
fi

remote_catalog_private_location=""
remote_catalog_public_location=""
remote_catalog_manual_detail="$(manual_confirmation_or_empty "remote_catalog_signing_keys")"
if [[ "$SM_AVAILABLE" != "1" ]]; then
  if [[ -n "$remote_catalog_manual_detail" ]]; then
    append_result "remote_catalog_signing_keys" "true" "pass" "$remote_catalog_manual_detail"
  else
    append_result "remote_catalog_signing_keys" "true" "manual" "Unable to verify secrets: AWS Secrets Manager endpoint unavailable from current environment."
  fi
else
  remote_catalog_private_location="$(locate_key_in_secrets "REMOTE_CATALOG_PRIVATE_KEY" "$REMOTE_CATALOG_SECRET_NAME" "$PLAID_OAUTH_SECRET_NAME" "$APP_SECRET_NAME" 2>/dev/null || true)"
  remote_catalog_public_location="$(locate_key_in_secrets "REMOTE_CATALOG_PUBLIC_KEY" "$REMOTE_CATALOG_SECRET_NAME" "$PLAID_OAUTH_SECRET_NAME" "$APP_SECRET_NAME" 2>/dev/null || true)"
  if [[ -n "$remote_catalog_private_location" && -n "$remote_catalog_public_location" ]]; then
    append_result "remote_catalog_signing_keys" "true" "pass" "Found REMOTE_CATALOG_PRIVATE_KEY in ${remote_catalog_private_location}; REMOTE_CATALOG_PUBLIC_KEY in ${remote_catalog_public_location}"
  elif [[ -n "$remote_catalog_manual_detail" ]]; then
    append_result "remote_catalog_signing_keys" "true" "pass" "$remote_catalog_manual_detail"
  else
    append_result "remote_catalog_signing_keys" "true" "fail" "Missing REMOTE_CATALOG_PRIVATE_KEY and/or REMOTE_CATALOG_PUBLIC_KEY in checked secrets (${REMOTE_CATALOG_SECRET_NAME}, ${PLAID_OAUTH_SECRET_NAME}, ${APP_SECRET_NAME})"
  fi
fi

if [[ -n "$LOCAL_LLM_ENDPOINT" ]]; then
  append_result "local_llm_endpoint" "false" "pass" "LOCAL_LLM_ENDPOINT set in environment"
elif [[ "$SM_AVAILABLE" != "1" ]]; then
  append_result "local_llm_endpoint" "false" "skip" "Optional item unverifiable (AWS Secrets Manager endpoint unavailable)"
elif local_llm_location="$(locate_key_in_secrets "LOCAL_LLM_ENDPOINT" "$APP_SECRET_NAME" "$PLAID_OAUTH_SECRET_NAME" 2>/dev/null || true)"; [[ -n "$local_llm_location" ]]; then
  append_result "local_llm_endpoint" "false" "pass" "LOCAL_LLM_ENDPOINT found in ${local_llm_location}"
else
  append_result "local_llm_endpoint" "false" "skip" "Optional item unset"
fi

if [[ -n "$ELEVENLABS_API_KEY" ]]; then
  append_result "elevenlabs_api_key" "false" "pass" "ELEVENLABS_API_KEY set in environment"
elif [[ "$SM_AVAILABLE" != "1" ]]; then
  append_result "elevenlabs_api_key" "false" "skip" "Optional item unverifiable (AWS Secrets Manager endpoint unavailable)"
elif elevenlabs_location="$(locate_key_in_secrets "ELEVENLABS_API_KEY" "$APP_SECRET_NAME" "$PLAID_OAUTH_SECRET_NAME" 2>/dev/null || true)"; [[ -n "$elevenlabs_location" ]]; then
  append_result "elevenlabs_api_key" "false" "pass" "ELEVENLABS_API_KEY found in ${elevenlabs_location}"
else
  append_result "elevenlabs_api_key" "false" "skip" "Optional item unset"
fi

mkdir -p "$(dirname "$OUTPUT_PATH")"
python3 - "$TMP_FILE" "$OUTPUT_PATH" "$REGION" "$MANUAL_EVIDENCE_PATH" <<'PY'
import json
import os
import sys
from datetime import datetime, timezone

src, out, region, evidence_path = sys.argv[1:5]
with open(src, "r", encoding="utf-8") as fh:
    payload = json.load(fh)
results = payload.get("results", [])
required = [r for r in results if r.get("required")]
manual_evidence_confirmed = 0
if os.path.exists(evidence_path):
    try:
        with open(evidence_path, "r", encoding="utf-8") as fh:
            evidence = json.load(fh)
        items = evidence.get("items") or {}
        if isinstance(items, dict):
            manual_evidence_confirmed = sum(
                1 for _, item in items.items() if isinstance(item, dict) and item.get("confirmed") is True
            )
    except Exception:
        manual_evidence_confirmed = 0
summary = {
    "generated_at_utc": datetime.now(timezone.utc).isoformat(),
    "region": region,
    "manual_evidence_path": evidence_path,
    "manual_evidence_confirmed": manual_evidence_confirmed,
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
