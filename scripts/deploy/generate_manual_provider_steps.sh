#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 is required to generate manual provider steps" >&2
  exit 1
fi

SIGNOFF_PATH="${SIGNOFF_PATH:-artifacts/deploy/go_live_signoff_status.json}"
OUTPUT_PATH="${OUTPUT_PATH:-artifacts/deploy/manual_provider_steps.md}"

if [[ ! -f "$SIGNOFF_PATH" ]]; then
  echo "missing go-live signoff artifact: $SIGNOFF_PATH" >&2
  exit 1
fi

mkdir -p "$(dirname "$OUTPUT_PATH")"

python3 - "$SIGNOFF_PATH" "$OUTPUT_PATH" <<'PY'
import json
import sys
from datetime import datetime, timezone

signoff_path, output_path = sys.argv[1:3]

with open(signoff_path, "r", encoding="utf-8") as fh:
    signoff = json.load(fh)

manual_items = signoff.get("manual_required_items", [])

instructions = {
    "partner_applications_submitted": [
        "Open your browser and go to partner dashboards for Zoom, Instacart, Canva, and Booking.com.",
        "For each dashboard, click the button labeled Create App or New App.",
        "Fill the required fields, set redirect URL to https://auth.brevio.app/callback/{service}, and click Submit.",
        "Wait for each dashboard status to show Submitted or In Review.",
        "Run: PARTNER_APPS_CONFIRMED=1 make external-phase-sync",
    ],
    "plaid_secret_prod": [
        "Open AWS Console and click Services -> Secrets Manager.",
        "In the search box, type brevio/production/plaid/client_secret and open that secret.",
        "Click Retrieve secret value and verify it is the current Plaid production secret.",
        "If incorrect, click Edit, paste the correct value, and click Save.",
        "Run: make manual-closeout-confirm ITEM_ID=plaid_secret_prod CONFIRMED_BY=<your-name> NOTE=\"verified in AWS Secrets Manager\"",
    ],
    "plaid_webhook_secret": [
        "Open AWS Console -> Secrets Manager.",
        "Search for brevio/production/plaid/webhook_secret and open it.",
        "Click Retrieve secret value and compare with Plaid dashboard webhook secret.",
        "If mismatch, click Edit, update value, and click Save.",
        "Run: make manual-closeout-confirm ITEM_ID=plaid_webhook_secret CONFIRMED_BY=<your-name> NOTE=\"verified webhook secret\"",
    ],
    "stripe_billing_keys": [
        "Open Stripe Dashboard and click Developers -> API keys.",
        "Copy live secret key and webhook signing secret from Developers -> Webhooks.",
        "Open AWS Console -> Secrets Manager.",
        "Update brevio/production/stripe/client_secret and brevio/production/stripe/webhook_secret.",
        "Run: make manual-closeout-confirm ITEM_ID=stripe_billing_keys CONFIRMED_BY=<your-name> NOTE=\"verified Stripe live keys\"",
    ],
    "unstructured_api_key": [
        "Open Unstructured dashboard and click API Keys.",
        "Copy the active production key.",
        "Open AWS Console -> Secrets Manager and search brevio/production/unstructured/api_key.",
        "Click Edit and paste the key if needed, then click Save.",
        "Run: make manual-closeout-confirm ITEM_ID=unstructured_api_key CONFIRMED_BY=<your-name> NOTE=\"verified Unstructured key\"",
    ],
    "pagerduty_routing_key": [
        "Open PagerDuty and click Services -> Service Directory -> your alerting service.",
        "Click Integrations and copy the Events API v2 integration key.",
        "Open AWS Console -> Secrets Manager and search brevio/production/pagerduty/routing_key.",
        "Update the key if needed and click Save.",
        "Run: make manual-closeout-confirm ITEM_ID=pagerduty_routing_key CONFIRMED_BY=<your-name> NOTE=\"verified PagerDuty routing key\"",
    ],
    "analytics_event_bus": [
        "Open AWS Console and click Services -> Amazon EventBridge.",
        "Click Event buses and select the configured production bus.",
        "Confirm the bus exists and is active.",
        "Open AWS Console -> Systems Manager Parameter Store or Secrets Manager for ANALYTICS_EVENT_BUS config and verify ARN/name.",
        "Run: make manual-closeout-confirm ITEM_ID=analytics_event_bus CONFIRMED_BY=<your-name> NOTE=\"verified EventBridge bus\"",
    ],
    "remote_catalog_signing_keys": [
        "Open AWS Console -> Secrets Manager.",
        "Search for brevio/production/remote-catalog/signing_private_key and signing_public_key.",
        "Open both secrets and confirm values are present and match your key rotation record.",
        "If missing/outdated, click Edit and paste rotated keys, then click Save.",
        "Run: make manual-closeout-confirm ITEM_ID=remote_catalog_signing_keys CONFIRMED_BY=<your-name> NOTE=\"verified remote catalog signing keys\"",
    ],
}

lines = []
lines.append("# Manual Provider Steps")
lines.append("")
lines.append(f"Generated (UTC): {datetime.now(timezone.utc).isoformat()}")
lines.append(f"Source: `{signoff_path}`")
lines.append("")

if not manual_items:
    lines.append("No pending manual required items.")
else:
    for idx, item in enumerate(manual_items, start=1):
        item_id = str(item.get("id", "unknown"))
        detail = str(item.get("detail", ""))
        steps = instructions.get(item_id, [
            "Open the provider console and verify production configuration for this item.",
            f"Run: make manual-closeout-confirm ITEM_ID={item_id} CONFIRMED_BY=<your-name> NOTE=\"manual verification completed\"",
        ])

        lines.append(f"## {idx}) `{item_id}`")
        if detail:
            lines.append(f"Context: {detail}")
        for step_index, step in enumerate(steps, start=1):
            lines.append(f"{step_index}. {step}")
        lines.append("")

lines.append("After completing all items:")
lines.append("1. Run: make external-phase-sync")
lines.append("2. Run: ALLOW_CONDITIONAL_MANUAL=1 make external-phase-transition-check")
lines.append("3. Run: make production-phase-sync")
lines.append("4. Run: make phase-closure-manifest")
lines.append("5. Run: make phase-status")

with open(output_path, "w", encoding="utf-8") as fh:
    fh.write("\n".join(lines) + "\n")

print(output_path)
PY

echo "$OUTPUT_PATH"
