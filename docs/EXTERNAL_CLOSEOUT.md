# External Closeout Runbook (Remaining Prompt Items)

This runbook covers the only checklist items that remain outside repository automation.

## Current Phase Status (Latest Gate Run)

Latest gate run: `make external-closeout-check` at `2026-03-05T03:33:33Z`

- Required checks: `8`
- Passed: `0`
- Failed: `0`
- Manual pending: `8`

Active required blockers right now:

1. `partner_applications_submitted` (`PARTNER_APPS_CONFIRMED=1` still required)
2. `PLAID_SECRET_PROD` manual verification required (AWS endpoint-unverifiable from current runtime context)
3. `PLAID_WEBHOOK_SECRET` manual verification required (AWS endpoint-unverifiable from current runtime context)
4. `STRIPE_SECRET_KEY`/`STRIPE_WEBHOOK_SECRET` manual verification required (AWS endpoint-unverifiable from current runtime context)
5. `UNSTRUCTURED_API_KEY` manual verification required (AWS endpoint-unverifiable from current runtime context)
6. `PAGERDUTY_ROUTING_KEY` (or `PAGERDUTY_INTEGRATION_KEY`) manual verification required (AWS endpoint-unverifiable from current runtime context)
7. `ANALYTICS_EVENT_BUS` manual verification required (AWS Events endpoint-unverifiable from current runtime context)
8. `REMOTE_CATALOG_PRIVATE_KEY`/`REMOTE_CATALOG_PUBLIC_KEY` manual verification required (AWS endpoint-unverifiable from current runtime context)

Authoritative status artifact:
- `artifacts/deploy/external_closeout_status.json`
- `artifacts/deploy/go_live_signoff_status.json` (`status=CONDITIONAL_MANUAL` from `make go-live-signoff` at `2026-03-05T03:33:33Z`)
- `artifacts/deploy/manual_closeout_todo.md` (generated from signoff at `2026-03-05T03:33:33Z`)
- `artifacts/deploy/manual_closeout_evidence.json` (`manual_evidence_confirmed=0` in latest run)
- `artifacts/deploy/external_closeout_regression_report.json` (`status=PASS`, no regressions in latest run)

Stability behavior:
- If AWS endpoints are transiently unavailable, the closeout checker reuses last-known `pass` results from the previous status artifact for required items (unless manually revoked), reducing pass/manual oscillation between runs.

## 1) Partner Applications (Zoom/Instacart/Canva/Booking.com)

### Zoom Marketplace
1. Open [marketplace.zoom.us](https://marketplace.zoom.us/) and sign in.
2. Click `Develop` (top right).
3. Click `Build App`.
4. Choose `General App` or `Server-to-Server OAuth` based on your Zoom MCP auth model.
5. Fill app name: `BREVIO Zoom MCP`.
6. Add redirect URI(s): your production OAuth callback URL.
7. Add required scopes for meetings + transcripts.
8. Click `Submit` for review.
9. Copy and store client id/secret in Secrets Manager.

### Instacart Connect
1. Open Instacart developer/partner portal.
2. Click `Create Application`.
3. Set app name: `BREVIO Instacart MCP`.
4. Add redirect URI(s) and webhook endpoint.
5. Request checkout/order scopes needed by `instacart.create_checkout`.
6. Submit for production review.
7. Save issued credentials in Secrets Manager.

### Canva Connect
1. Open [canva.com/developers](https://www.canva.com/developers/).
2. Click `Create app`.
3. Enter app name: `BREVIO Canva MCP`.
4. Configure OAuth redirect URI(s).
5. Configure template-only permissions required for curated flow.
6. Submit app for review.
7. Store credentials in Secrets Manager.

### Booking.com Demand API
1. Open [developers.booking.com](https://developers.booking.com/).
2. Create partner/application profile.
3. Register `BREVIO Booking MCP`.
4. Configure redirect URI(s) and webhook URL(s).
5. Request hotel search + reservation scopes.
6. Submit partner onboarding forms and compliance docs.
7. Store credentials in Secrets Manager.

## 2) Plaid Production Verification + Webhook Secret
1. Open [dashboard.plaid.com](https://dashboard.plaid.com/).
2. Select your app.
3. Go to `Team settings` or `Production access`.
4. Complete Plaid production review checklist.
5. After approval, copy production secret.
6. In AWS Console, open `Secrets Manager`.
7. Open secret `executive-os/prod/oauth_client_secrets` (or your canonical oauth secret bundle).
8. Click `Retrieve secret value` -> `Edit`.
9. Add/update key `PLAID_SECRET_PROD` with production value.
10. Save.
11. In Plaid dashboard, open `Webhooks`.
12. Generate/fetch webhook verification secret.
13. In Secrets Manager, open `executive-os/prod/gateway_webhook_secret`.
14. Add/update `PLAID_WEBHOOK_SECRET`.
15. Save and redeploy the affected workloads.

## 3) Stripe Production Billing Keys
1. Open [dashboard.stripe.com](https://dashboard.stripe.com/).
2. Switch to `Live mode`.
3. Go to `Developers` -> `API keys`.
4. Copy `Secret key`.
5. Go to `Developers` -> `Webhooks` -> `Add endpoint`.
6. Set endpoint URL to your billing webhook URL.
7. Select required events.
8. Create endpoint and copy `Signing secret`.
9. Go to `Product catalog`, create/verify plan products and prices.
10. Copy price ids for Personal/Professional.
11. Save `STRIPE_SECRET_KEY`, `STRIPE_WEBHOOK_SECRET`, and price ids in Secrets Manager.

## 4) Unstructured.io API Key
1. Open [app.unstructured.io](https://app.unstructured.io/).
2. Create account/sign in.
3. Click `API Keys`.
4. Click `Create key`.
5. Copy key and store in Secrets Manager under your document parsing secret bundle.

## 5) PagerDuty Integration
1. Open [pagerduty.com](https://www.pagerduty.com/) and sign in.
2. Go to `Services` -> `New Service`.
3. Service name: `BREVIO Quality Alerts`.
4. Add integration type `Events API v2`.
5. Save service.
6. Copy integration key.
7. Store integration key in Secrets Manager.
8. Wire alerts destination env vars and redeploy alerting service.

## 6) Analytics Event Bus (`ANALYTICS_EVENT_BUS`)
1. Open AWS Console -> search `EventBridge`.
2. Click `Event buses`.
3. Click `Create event bus`.
4. Name: `brevio-analytics-bus` (or your chosen canonical name).
5. Create bus.
6. Copy bus ARN.
7. Put ARN into your deployment env/secret as `ANALYTICS_EVENT_BUS`.
8. Update IAM policy for runtime roles to allow `events:PutEvents` on this bus.
9. Redeploy services that emit analytics events.

## 7) Remote Catalog Signing Keys
### Step A — Generate keys (copy/paste safe)
1. Open Terminal.
2. Run:
```bash
cd /Users/galiettemita/Downloads/Executive AI Agent/backend
make generate-remote-catalog-keys
```
3. You will get JSON output with:
   - `REMOTE_CATALOG_PRIVATE_KEY`
   - `REMOTE_CATALOG_PUBLIC_KEY`
4. Keep this terminal window open. You will paste those two values into AWS in Step B.

### Step B — Put keys in AWS Secrets Manager (button-by-button)
1. Open AWS Console.
2. In the top search bar, type `Secrets Manager` and click it.
3. Click `Secrets` (left sidebar).
4. Find and click your secret (default in this repo is `executive-os/prod/oauth_client_secrets`).
5. Click `Retrieve secret value`.
6. Click `Edit`.
7. If you see key/value rows, switch to `Plaintext` tab for easier paste.
8. Add these JSON fields (or update if they already exist):
   - `"REMOTE_CATALOG_PRIVATE_KEY": "<paste private key from terminal>"`
   - `"REMOTE_CATALOG_PUBLIC_KEY": "<paste public key from terminal>"`
9. Click `Save`.
10. Click `Retrieve secret value` again and verify both keys are present.

### Step C — Verify it passed
1. In Terminal, run:
```bash
cd /Users/galiettemita/Downloads/Executive AI Agent/backend
ANALYTICS_EVENT_BUS=brevio-analytics-bus make external-closeout-check
```
2. Open:
   - `artifacts/deploy/external_closeout_status.json`
3. Confirm `remote_catalog_signing_keys` is `pass`.

## 8) Optional Keys (vLLM / ElevenLabs)
### local vLLM
1. Deploy your vLLM endpoint.
2. Copy endpoint URL.
3. Set `LOCAL_LLM_ENDPOINT` in environment/secret config.

### ElevenLabs
1. Open [elevenlabs.io](https://elevenlabs.io/).
2. Create API key from dashboard.
3. Store as `ELEVENLABS_API_KEY` in Secrets Manager.

## 9) Final Verify Commands
Run external readiness verification (required items fail-fast):

```bash
cd /Users/galiettemita/Downloads/Executive AI Agent/backend
PARTNER_APPS_CONFIRMED=0 \
ANALYTICS_EVENT_BUS=brevio-analytics-bus \
make external-closeout-check
```

Output report:
- `artifacts/deploy/external_closeout_status.json`

When all required checks pass and partner apps are submitted, rerun with:

```bash
PARTNER_APPS_CONFIRMED=1 ANALYTICS_EVENT_BUS=brevio-analytics-bus make external-closeout-check
```

Then run:

```bash
cd /Users/galiettemita/Downloads/Executive AI Agent/backend
make ci
make security-validate
```

## 10) Generate Go-Live Signoff Artifact

After each external closeout run, generate the next-phase signoff status artifact:

```bash
cd /Users/galiettemita/Downloads/Executive AI Agent/backend
make go-live-signoff
```

Output:
- `artifacts/deploy/go_live_signoff_status.json`

Status meanings:
- `READY`: no required failed/manual items remain.
- `CONDITIONAL_MANUAL`: no required failures, but manual provider/account confirmations still required.
- `BLOCKED`: one or more required failed items still unresolved.

## 11) Generate Manual Closeout TODO Artifact

After go-live signoff generation, create a deterministic manual TODO list from pending required items:

```bash
cd /Users/galiettemita/Downloads/Executive AI Agent/backend
make manual-closeout-todo
```

Output:
- `artifacts/deploy/manual_closeout_todo.md`

This file maps each pending required item to the matching runbook section (`Section 1`-`Section 7`) and should be treated as the active closure checklist until signoff reaches `READY`.

## 12) Record Manual Evidence (Per Item)

When a human finishes a manual blocker in production context, record evidence so the gate can mark it `pass` even if the local runtime cannot reach AWS endpoints.

```bash
cd /Users/galiettemita/Downloads/Executive AI Agent/backend
make manual-closeout-confirm ITEM_ID=partner_applications_submitted CONFIRMED_BY=ops NOTE="Submitted all partner apps"
```

Example for key verification completed in production account:

```bash
make manual-closeout-confirm ITEM_ID=plaid_secret_prod CONFIRMED_BY=ops NOTE="Verified PLAID_SECRET_PROD in prod secrets"
```

If a confirmation was added in error, revoke it:

```bash
make manual-closeout-unconfirm ITEM_ID=plaid_secret_prod REVOKED_BY=ops NOTE="Entered wrong workspace evidence"
```

Evidence file:
- `artifacts/deploy/manual_closeout_evidence.json`
- includes:
  - `items` current confirmation state per required item
  - `events` append-only confirm/revoke history for audit traceability

Supported `ITEM_ID` values:
- `partner_applications_submitted`
- `plaid_secret_prod`
- `plaid_webhook_secret`
- `stripe_billing_keys`
- `unstructured_api_key`
- `pagerduty_routing_key`
- `analytics_event_bus`
- `remote_catalog_signing_keys`

Canonical source of allowed IDs:
- `config/external-closeout-required-item-ids.txt`

## 13) One-Command External Phase Sync

To refresh all three external-phase artifacts in one command:

```bash
cd /Users/galiettemita/Downloads/Executive AI Agent/backend
make external-phase-sync
```

This runs:
1. `make external-closeout-check`
2. `make go-live-signoff`
3. `make manual-closeout-todo`

## 14) Regression Check Between Runs

To detect required-item regressions (`pass -> manual/fail`) between closeout runs:

```bash
cd /Users/galiettemita/Downloads/Executive AI Agent/backend
make external-closeout-regression-check
```

Outputs:
- `artifacts/deploy/external_closeout_regression_report.json`
- `artifacts/deploy/external_closeout_status.last.json` (updated snapshot baseline)

Included by default in sync:

```bash
make external-phase-sync
```

Explicit enforced mode (equivalent behavior):

```bash
EXTERNAL_REGRESSION_CHECK=1 make external-phase-sync
```

## 15) Phase Transition Check (External -> Production Signoff)

To check if current artifacts allow transition to the next phase:

```bash
cd /Users/galiettemita/Downloads/Executive AI Agent/backend
make external-phase-transition-check
```

Output:
- `artifacts/deploy/external_phase_transition_check.json`

Behavior:
- exits `0` only when signoff status is `READY`
- exits non-zero while status remains `CONDITIONAL_MANUAL` or `BLOCKED`

Override for controlled manual acceptance:

```bash
ALLOW_CONDITIONAL_MANUAL=1 make external-phase-transition-check
```

Optional: disable regression check for troubleshooting only:

```bash
EXTERNAL_REGRESSION_CHECK=0 make external-phase-sync
```
