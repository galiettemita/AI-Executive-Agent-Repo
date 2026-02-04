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
- [x] Add signed, short-lived approval links (JWT).
- [x] Implement Approval Inbox page (mobile-first).
- [x] Implement approve/edit/cancel endpoints.
- [x] Add audit log for proposal approvals.
- [x] Configure CORS for Approval Inbox domain in Render (`CORS_ORIGINS`).

## Stage 5: Email + Calendar (Read-Only) + Daily Brief
- [x] Confirm OAuth token storage is encrypted at rest.
- [x] Implement daily brief worker (calendar + email summaries).
- [x] Store brief as messages and/or memory notes.
- [x] Add manual trigger endpoint for testing.
- [x] Add scheduled job (cron/queue).
- [x] Add Google OAuth redirect URL pointing to `https://<render-domain>/admin/google/callback`.

## Stage 6: Creative Studio + Wardrobe
- [x] Implement creative prompt flow + templates.
- [x] Add wardrobe onboarding (vibe, sizes, colors, budget).
- [x] Add outfit generation responses.
- [x] Add wardrobe shopping list proposals.

## Stage 7: Commerce Proposals (Shopping/Food/Travel)
- [x] Add proposal types: `food_order`, `travel_itinerary`, `purchase_cart`.
- [x] Implement shopping compare + cart proposal builder.
- [x] Implement food order + cart proposal builder.
- [x] Implement travel shortlist + itinerary proposal builder.

## Stage 8: Monitoring/Alerts Workers
- [x] Add queue + worker for monitoring jobs.
- [x] Implement price/availability alert logic.
- [x] Add user notification delivery.

## Stage 9: Apple Messages Adapter (DEFERRED)
- [ ] SKIPPED - Focus on execution engine first
- [ ] Can revisit after booking/checkout is production-ready

---

# EXECUTION ENGINE - Production-Grade Booking & Checkout

## Stage 10: Payment Infrastructure (Stripe) ✅
- [x] Add `payment_methods` table (user_id, stripe_payment_method_id, type, last4, exp_date).
- [x] Add `transactions` table (id, user_id, proposal_id, amount, currency, status, stripe_payment_intent_id, created_at).
- [x] Implement Stripe SDK integration for payment intents.
- [x] Add payment method registration endpoint (tokenize card via Stripe Elements).
- [x] Add 3D Secure (SCA) authentication flow.
- [x] Implement payment intent creation with idempotency keys.
- [x] Add webhook handler for `payment_intent.succeeded` and `payment_intent.failed`.
- [x] Add refund/cancellation logic via Stripe API.
- [x] Store payment receipts and generate PDF invoices.
- [x] Add Stripe environment vars: `STRIPE_SECRET_KEY`, `STRIPE_PUBLISHABLE_KEY`, `STRIPE_WEBHOOK_SECRET`.

## Stage 11: Travel Booking Execution
- [x] Add `bookings` table (id, user_id, proposal_id, booking_type, status, confirmation_number, provider, provider_booking_id, payload_json).
- [x] Research and select travel API provider (Amadeus Self-Service or Duffel recommended).
- [x] Register for Amadeus Self-Service API (free tier: 2000 requests/month).
- [ ] Implement flight search via Amadeus Flight Offers Search API.
- [ ] Implement flight booking via Amadeus Flight Create Orders API.
- [ ] Implement hotel search via Amadeus Hotel Search API.
- [ ] Implement hotel booking via Amadeus Hotel Booking API.
- [ ] Add PNR (Passenger Name Record) storage and retrieval.
- [ ] Implement booking confirmation email/WhatsApp delivery with e-ticket.
- [ ] Add cancellation/modification logic with provider API.
- [x] Handle booking failures with automatic refund trigger.
- [ ] Add traveler profile storage (passport info, TSA PreCheck, loyalty numbers).
- [x] Add environment vars: `AMADEUS_API_KEY`, `AMADEUS_API_SECRET`.

## Stage 12: Food Delivery Execution
- [ ] Research food delivery APIs (DoorDash Drive API or Uber Eats API).
- [ ] Register for DoorDash Developer Platform.
- [ ] Implement restaurant search and menu retrieval.
- [ ] Implement cart creation and item validation.
- [ ] Implement delivery address validation and geocoding.
- [ ] Implement order placement via API.
- [ ] Add real-time order tracking integration.
- [ ] Implement order status webhooks (confirmed, preparing, out_for_delivery, delivered).
- [ ] Add delivery instructions and contact preferences.
- [ ] Implement order cancellation within allowed timeframe.
- [ ] Add tip calculation and customization.
- [ ] Store order receipts and send confirmation via WhatsApp.
- [ ] Add environment vars: `DOORDASH_API_KEY`, `DOORDASH_DEVELOPER_ID`.

## Stage 13: Retail Shopping Checkout Execution
- [ ] Add `shipping_addresses` table (user_id, name, street, city, state, zip, country, is_default).
- [ ] Implement shopping cart consolidation from multiple retailers.
- [ ] Add address validation via USPS or SmartyStreets API.
- [ ] Implement tax calculation via TaxJar or Avalara API.
- [ ] Add shipping rate calculation (partner with ShipStation or EasyPost).
- [ ] Implement Amazon Product Advertising API for direct purchase links.
- [ ] Implement Walmart Affiliate API for purchase tracking.
- [ ] Add order tracking integration (parse retailer emails or use AfterShip API).
- [ ] Implement order confirmation scraping or email parsing.
- [ ] Add return/refund request handling.
- [ ] Store order history with itemized receipts.
- [ ] Add environment vars: `TAXJAR_API_KEY`, `EASYPOST_API_KEY`, `AMAZON_ASSOCIATE_TAG`.

## Stage 14: Proposal Approval & Execution Engine ✅
- [x] Add `execution_logs` table (id, proposal_id, transaction_id, step, status, error_message, timestamp).
- [x] Implement JWT verification for approval links (check expiry, signature).
- [x] Add proposal state machine (pending → approved → executing → completed/failed).
- [x] Build centralized execution orchestrator service.
- [x] Implement pre-execution validation (budget check, payment method, required fields).
- [x] Add atomic execution steps with rollback on failure.
- [x] Implement retry logic with exponential backoff for transient failures.
- [x] Add execution status webhooks to notify user of progress.
- [x] Implement "dry run" mode for testing without actual charges.
- [ ] Add manual intervention queue for flagged transactions.
- [ ] Build execution dashboard for monitoring pending/failed executions.

## Stage 15: Safety, Security & Compliance
- [x] Enforce "no purchases without proposal + approval" (verify JWT signature).
- [x] Add spending limits table (user_id, period, limit_amount, spent_amount, reset_date).
- [x] Implement daily/weekly/monthly spending caps per user tier.
- [x] Add transaction amount limits (e.g., max $500 per transaction for free tier).
- [ ] Implement merchant/category allowlist (block crypto, gambling, etc.).
- [x] Add velocity checks (max 5 transactions per hour per user).
- [ ] Implement fraud detection (unusual patterns, new device, geo mismatch).
- [ ] Add 2FA requirement for transactions over threshold (e.g., $200).
- [ ] Implement PII encryption for payment methods and traveler info.
- [ ] Add GDPR-compliant data deletion endpoint.
- [x] Implement structured audit logging with `conversation_id`, `user_id`, `transaction_id`.
- [ ] Add feature flags for gradual rollout (launch-darkly or custom).
- [ ] Implement rate limiting (10 req/min per user, 100 req/min per IP).
- [ ] Add abuse detection (block users with >3 chargebacks).
- [ ] Implement circuit breakers for external API failures.

## Stage 16: Transaction Management & Recovery
- [x] Implement transaction state persistence (in-flight tracking).
- [x] Add automatic refund on booking failure (within 24 hours).
- [x] Implement partial refund logic (e.g., cancellation fees).
- [ ] Add dispute management integration with Stripe.
- [ ] Implement chargeback notification and response handling.
- [ ] Add reconciliation job (daily comparison of Stripe vs internal ledger).
- [ ] Implement failed transaction retry queue with manual approval.
- [ ] Add webhook replay for missed payment/booking events.
- [x] Implement idempotency for all payment and booking operations.
- [ ] Add transaction receipt generation (PDF with itemized breakdown).
- [ ] Implement notification delivery for transaction lifecycle events.

## Stage 17: Testing & Validation
- [x] Create test suite for payment flow (mock Stripe).
- [ ] Create test suite for travel booking flow (mock Amadeus).
- [ ] Create test suite for food delivery flow (mock DoorDash).
- [ ] Create test suite for retail checkout flow.
- [x] Add end-to-end tests for complete proposal → approval → execution flow.
- [x] Test rollback scenarios (payment fails, booking fails, etc.).
- [x] Test edge cases (expired approval link, insufficient funds, API timeout).
- [ ] Implement load testing for concurrent execution requests.
- [ ] Add chaos engineering tests (random API failures, network issues).
- [x] Test fraud detection scenarios (velocity limits, amount limits).
- [ ] Validate PII encryption and secure data handling.
- [ ] Test GDPR data deletion compliance.

## Deployment Checklist
- [ ] Containerize services (Dockerfile).
- [ ] Configure managed Postgres + Redis.
- [ ] Configure queue (SQS/RabbitMQ/etc.).
- [ ] Set up secrets manager (AWS Secrets Manager or Render env encryption).
- [ ] Add monitoring and alerting (Datadog, New Relic, or CloudWatch).
- [ ] Configure backups + retention (automated daily backups, 30-day retention).
- [ ] Set up error tracking (Sentry or Rollbar).
- [ ] Configure log aggregation (Logtail, Papertrail, or ELK stack).
- [ ] Render env vars set: `DATABASE_URL`, `WHATSAPP_VERIFY_TOKEN`, `WHATSAPP_TOKEN`, `WHATSAPP_PHONE_NUMBER_ID`, `OPENAI_API_KEY`, `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GOOGLE_REDIRECT_URI`, `JWT_SECRET`, `STRIPE_SECRET_KEY`, `STRIPE_PUBLISHABLE_KEY`, `STRIPE_WEBHOOK_SECRET`, `AMADEUS_API_KEY`, `AMADEUS_API_SECRET`, `DOORDASH_API_KEY`, `TAXJAR_API_KEY`.
- [ ] Render health check path set to `/health` (or `/` if preferred).
- [ ] Set up SSL/TLS certificates (auto-managed by Render).
- [ ] Configure CDN for static assets (Cloudflare or CloudFront).
- [ ] Implement blue-green deployment strategy for zero-downtime updates.