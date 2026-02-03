# Product Build Checklist (2-Week Plan)

This checklist maps directly to your written plan and the current repo status.
Check items as you complete them.

## Stage 1: WhatsApp Webhook → Agent API → Simple Responses (Render)
- [x] WhatsApp webhook endpoint accepts inbound messages.
- [x] Webhook verification endpoint is implemented.
- [x] Orchestrator returns a reply and sends it back to WhatsApp.
- [x] Inbound message idempotency is enforced.
- [x] Render service is live and reachable at your public URL.
- [x] Meta webhook URL points to `https://ai-shopping-assistant-backend-6bgf.onrender.com/webhooks/whatsapp`.
- [x] Verify token in Meta matches `WHATSAPP_VERIFY_TOKEN` in Render env.
- [x] Confirm `GET /` returns ok from Render (no 404).

## Stage 2: Users + Preferences + Memory Notes
- [x] Add `preferences` storage (table or JSONB on `users`).
- [x] Add onboarding prompts to collect preferences.
- [x] Persist preferences from WhatsApp onboarding.
- [x] Store minimal chat history (last N turns).
- [x] Summarize older history into `memory_notes`.
- [x] Use preferences + memory notes in orchestrator prompts.

## Stage 3: Subscription Gate + Usage Metering
- [x] Add `subscriptions` table.
- [x] Add `usage` table (monthly period buckets).
- [x] Implement usage counters (messages, tokens, proposals).
- [x] Add entitlements check before skill execution.
- [x] Add “limit hit” response with upgrade link.
- [x] Add webhook handler to update subscription state.
- [x] Add billing provider env vars on Render (e.g., Stripe keys + webhook secret).

## Stage 4: Proposal Engine + Approval Inbox
- [x] Add `proposals` table with status + payload.
- [x] Add proposal creation helper (service or model method).
- [x] Update agent output to emit `proposal` or `assistant_message`.
- [ ] Add signed, short-lived approval links (JWT).
- [ ] Implement Approval Inbox page (mobile-first).
- [ ] Implement approve/edit/cancel endpoints.
- [ ] Add audit log for proposal approvals.
- [ ] Configure CORS for Approval Inbox domain in Render (`CORS_ORIGINS`).

## Stage 5: Email + Calendar (Read-Only) + Daily Brief
- [ ] Confirm OAuth token storage is encrypted at rest.
- [ ] Implement daily brief worker (calendar + email summaries).
- [ ] Store brief as messages and/or memory notes.
- [ ] Add manual trigger endpoint for testing.
- [ ] Add scheduled job (cron/queue).
- [ ] Add Google OAuth redirect URL pointing to `https://<render-domain>/admin/google/callback`.

## Stage 6: Creative Studio + Wardrobe
- [ ] Implement creative prompt flow + templates.
- [ ] Add wardrobe onboarding (vibe, sizes, colors, budget).
- [ ] Add outfit generation responses.
- [ ] Add wardrobe shopping list proposals.

## Stage 7: Commerce Proposals (Shopping/Food/Travel)
- [ ] Add proposal types: `purchase_cart`, `booking_plan`.
- [ ] Implement shopping compare + cart proposal builder.
- [ ] Implement food reorder + cart proposal builder.
- [ ] Implement travel shortlist + itinerary proposal builder.

## Stage 8: Monitoring/Alerts Workers
- [ ] Add queue + worker for monitoring jobs.
- [ ] Implement price/availability alert logic.
- [ ] Add user notification delivery.

## Stage 9: Apple Messages Adapter
- [ ] Add Messages for Business adapter (provider webhook).
- [ ] Normalize inbound events to core agent format.
- [ ] Configure outbound messages.

## Safety, Privacy, Reliability (Non-Negotiable)
- [ ] Enforce “no purchases without proposal + approval.”
- [ ] Add spending caps + merchant allowlist checks.
- [ ] Add data deletion endpoint.
- [ ] Add structured logs with `conversation_id`.
- [ ] Add feature flags + gradual rollout.
- [ ] Add abuse detection + blocklist.

## Deployment Checklist
- [ ] Containerize services (Dockerfile).
- [ ] Configure managed Postgres + Redis.
- [ ] Configure queue (SQS/RabbitMQ/etc.).
- [ ] Set up secrets manager.
- [ ] Add monitoring and alerting.
- [ ] Configure backups + retention.
- [ ] Render env vars set: `DATABASE_URL`, `WHATSAPP_VERIFY_TOKEN`, `WHATSAPP_TOKEN`, `WHATSAPP_PHONE_NUMBER_ID`, `OPENAI_API_KEY`, `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GOOGLE_REDIRECT_URI`, `JWT_SECRET`.
- [ ] Render health check path set to `/health` (or `/` if preferred).
