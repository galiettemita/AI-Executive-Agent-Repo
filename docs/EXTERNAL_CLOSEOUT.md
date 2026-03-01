# External Closeout Runbook (Remaining Prompt Items)

This runbook covers the only checklist items that remain outside repository automation.

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
1. Create key pair for catalog signing (Ed25519 recommended).
2. Store private key in Secrets Manager (restricted access).
3. Store public key in app configuration for verification.
4. Configure remote catalog API base URL in env.
5. Enable signature verification in provisioning path before enabling remote import in production.

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
