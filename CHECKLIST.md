# Product Launch Checklist

This checklist is organized for **fastest time to market**:
1. **PRE-LAUNCH CRITICAL** - Must complete before going live
2. **LAUNCH** - Deployment and verification steps
3. **POST-LAUNCH ROADMAP** - Additional features to add after launch

---

## PRE-LAUNCH CRITICAL ✅ (Complete before deploying)

### Stage 0: Landing Page & Domain Setup (NEW - Required for Public Launch)
**Goal**: Professional website where users can discover and access your AI assistant

#### Domain & Infrastructure
- [x] **CRITICAL**: Purchase domain name (GoDaddy)
- [x] Configure DNS records to point to Vercel (for landing page)
- [ ] Configure custom domain on Render (for backend API)
  - Example: `api.yourproductname.com` → Render backend

#### WhatsApp Business Configuration (SKIPPED FOR NOW)
- [ ] **CRITICAL**: Confirm WhatsApp Business phone number (production-ready)
- [ ] Switch Meta WhatsApp app from Test mode to Production mode (if applicable)
- [ ] Verify WhatsApp webhook URL is configured in Meta dashboard
- [ ] Test webhook delivery from Meta to Render backend
- [ ] Generate WhatsApp direct link: `https://wa.me/YOUR_NUMBER?text=Hi`
- [ ] Test link opens WhatsApp correctly on mobile and desktop

#### Landing Page Development
- [x] **CRITICAL**: Build Next.js landing page with:
  - Hero section with product value proposition
  - Feature highlights (travel booking, shopping, food delivery)
  - QR code that opens WhatsApp with your bot
  - "Start Chatting" button (mobile-friendly WhatsApp link)
  - Pricing tiers section
  - FAQ section
  - Privacy policy & Terms of Service links
  - Responsive design (mobile-first)
- [x] Deploy landing page to Vercel
- [x] Configure custom domain on Vercel
- [x] Test landing page on mobile and desktop
- [x] Verify all links on landing page work correctly
- [ ] Generate QR code image from WhatsApp link (skipped - no WhatsApp yet)
- [ ] Test QR code scanning (skipped - no WhatsApp yet)
- [ ] Add Google Analytics or Plausible for traffic tracking (optional)

#### Integration Testing (WhatsApp - SKIPPED FOR NOW)
- [ ] End-to-end flow: User scans QR → WhatsApp opens → Bot responds
- [ ] Test onboarding flow through WhatsApp

---

### Stage 11: Travel Booking - Final Items
- [x] Add `bookings` table (id, user_id, proposal_id, booking_type, status, confirmation_number, provider, provider_booking_id, payload_json)
- [x] Register for Amadeus Self-Service API (free tier: 2000 requests/month)
- [x] Implement flight search via Amadeus Flight Offers Search API
- [x] Implement flight booking via Amadeus Flight Create Orders API
- [x] Implement hotel search via Amadeus Hotel Search API
- [x] Implement hotel booking via Amadeus Hotel Booking API
- [x] Add PNR (Passenger Name Record) storage and retrieval
- [x] Handle booking failures with automatic refund trigger
- [x] Add traveler profile storage (passport info, TSA PreCheck, loyalty numbers)
- [x] Add environment vars: `AMADEUS_API_KEY`, `AMADEUS_API_SECRET`
- [x] **CRITICAL**: Implement booking confirmation email/WhatsApp delivery with e-ticket
- [x] **CRITICAL**: Add cancellation/modification logic with provider API

### Stage 15: Safety & Security - Production Essentials
- [x] Enforce "no purchases without proposal + approval" (verify JWT signature)
- [x] Add spending limits table (user_id, period, limit_amount, spent_amount, reset_date)
- [x] Implement daily/weekly/monthly spending caps per user tier
- [x] Add transaction amount limits (e.g., max $500 per transaction for free tier)
- [x] Add velocity checks (max 5 transactions per hour per user)
- [x] Implement structured audit logging with `conversation_id`, `user_id`, `transaction_id`
- [x] Implement transaction state persistence (in-flight tracking)
- [x] Implement idempotency for all payment and booking operations
- [x] **CRITICAL**: Implement rate limiting (10 req/min per user, 100 req/min per IP)
- [x] **CRITICAL**: Implement PII encryption for payment methods and traveler info
- [x] **CRITICAL**: Add circuit breakers for external API failures (Amadeus, Stripe)
- [x] **CRITICAL**: Add GDPR-compliant data deletion endpoint
- [ ] Add merchant/category allowlist (block crypto, gambling, etc.)
- [ ] Implement fraud detection (unusual patterns, new device, geo mismatch)
- [ ] Add 2FA requirement for transactions over threshold (e.g., $200)
- [ ] Add feature flags for gradual rollout (launch-darkly or custom)
- [ ] Add abuse detection (block users with >3 chargebacks)

### Stage 16: Transaction Management - Critical Items Only
- [x] Implement automatic refund on booking failure (within 24 hours)
- [x] Implement partial refund logic (e.g., cancellation fees)
- [ ] Add dispute management integration with Stripe
- [ ] Implement chargeback notification and response handling
- [ ] Add webhook replay for missed payment/booking events
- [ ] Add transaction receipt generation (PDF with itemized breakdown)
- [ ] Implement notification delivery for transaction lifecycle events

### Stage 17: Testing - Pre-Launch Validation
- [x] Create test suite for payment flow (mock Stripe)
- [x] Add end-to-end tests for complete proposal → approval → execution flow
- [x] Test rollback scenarios (payment fails, booking fails, etc.)
- [x] Test edge cases (expired approval link, insufficient funds, API timeout)
- [x] Test fraud detection scenarios (velocity limits, amount limits)
- [x] **CRITICAL**: Create test suite for travel booking flow (mock Amadeus)
- [x] **CRITICAL**: Test GDPR data deletion compliance
- [x] **CRITICAL**: Validate PII encryption and secure data handling
- [ ] Implement load testing for concurrent execution requests
- [ ] Add chaos engineering tests (random API failures, network issues)

---

## LAUNCH 🚀 (Deployment Steps)

### Pre-Deployment Checks
- [ ] Run full test suite locally: `pytest tests/ -v`
- [ ] Verify all environment variables are set on Render
- [ ] Review Render logs for any startup errors
- [ ] Confirm database migrations are up to date: `alembic current`
- [ ] Test webhook endpoints are accessible (WhatsApp, Stripe)

### Deployment to Render (Backend)
- [ ] Push latest backend code to GitHub main branch
- [ ] Verify Render auto-deploys from main branch
- [ ] Monitor deployment logs for errors
- [ ] Wait for health check to pass: `GET /health`
- [ ] Verify service is live at your backend URL (or custom domain if configured)

### Deployment to Vercel (Landing Page)
- [ ] Create Vercel account (free tier is sufficient)
- [ ] Connect Vercel to your frontend GitHub repo
- [ ] Configure build settings (Next.js auto-detected)
- [ ] Add environment variables if needed (WhatsApp number, analytics keys)
- [ ] Deploy landing page to Vercel
- [ ] Verify deployment succeeded
- [ ] Configure custom domain in Vercel dashboard
- [ ] Verify SSL certificate is active (auto-provisioned by Vercel)
- [ ] Test landing page at custom domain

### Post-Deployment Verification

#### Landing Page Tests
- [ ] Visit landing page at custom domain (e.g., `https://yourproductname.com`)
- [ ] Verify QR code is visible and properly sized
- [ ] Test QR code scanning from iPhone (opens WhatsApp correctly)
- [ ] Test QR code scanning from Android (opens WhatsApp correctly)
- [ ] Test "Start Chatting" button on mobile (opens WhatsApp app)
- [ ] Test "Start Chatting" button on desktop (opens WhatsApp Web)
- [ ] Verify all links work (Privacy Policy, Terms, Pricing, etc.)
- [ ] Test landing page responsiveness on mobile/tablet/desktop
- [ ] Check page load speed (use PageSpeed Insights)
- [ ] Verify meta tags and OpenGraph images (for social sharing)

#### Backend API Tests
- [ ] Test WhatsApp webhook: Send "Hello" message to bot via WhatsApp
- [ ] Verify bot responds correctly to first-time user
- [ ] Test user onboarding flow via WhatsApp
- [ ] Create test traveler profile via API
- [ ] Search for test flight (JFK → LAX)
- [ ] Create test proposal (without executing)
- [ ] Test approval link generation and validation
- [ ] Execute small test transaction ($1 flight search)
- [ ] Verify invoice PDF generation
- [ ] Test Stripe webhook delivery
- [ ] Check execution dashboard shows test data
- [ ] Verify logs are being captured properly
- [ ] Test error scenarios (invalid airport code, etc.)

#### End-to-End User Journey
- [ ] New user scans QR code from landing page
- [ ] WhatsApp opens with your bot
- [ ] User sends first message
- [ ] Bot responds with onboarding flow
- [ ] User completes onboarding (preferences, etc.)
- [ ] User requests a flight search
- [ ] Bot returns proposal with approval link
- [ ] User clicks approval link (opens in browser)
- [ ] User approves proposal
- [ ] Execution engine processes booking
- [ ] User receives confirmation via WhatsApp
- [ ] User receives invoice PDF

### Production Monitoring Setup
- [ ] Set up error tracking (Sentry or Rollbar)
- [ ] Configure log aggregation (Logtail, Papertrail, or ELK stack)
- [ ] Add monitoring and alerting (Datadog, New Relic, or CloudWatch)
- [ ] Set up uptime monitoring (Pingdom or UptimeRobot)
- [ ] Create incident response playbook
- [ ] Set up backup alerts for database

### Environment Variables Checklist (Render)
Verify all required env vars are set:
- [ ] `DATABASE_URL`
- [ ] `WHATSAPP_VERIFY_TOKEN`
- [ ] `WHATSAPP_TOKEN`
- [ ] `WHATSAPP_PHONE_NUMBER_ID`
- [ ] `OPENAI_API_KEY`
- [ ] `GOOGLE_CLIENT_ID`
- [ ] `GOOGLE_CLIENT_SECRET`
- [ ] `GOOGLE_REDIRECT_URI`
- [ ] `JWT_SECRET`
- [ ] `STRIPE_SECRET_KEY`
- [ ] `STRIPE_PUBLISHABLE_KEY`
- [ ] `STRIPE_WEBHOOK_SECRET`
- [ ] `AMADEUS_API_KEY`
- [ ] `AMADEUS_API_SECRET`
- [ ] `CORS_ORIGINS`

### Go-Live Decision
- [ ] All critical backend tests passing
- [ ] Landing page is live and accessible at custom domain
- [ ] QR code correctly opens WhatsApp with bot
- [ ] End-to-end user journey tested successfully
- [ ] No errors in production logs (backend and landing page)
- [ ] Monitoring dashboards operational
- [ ] WhatsApp Business account is in Production mode
- [ ] Custom domain DNS is fully propagated (check with DNS checker)
- [ ] Team has reviewed deployment checklist
- [ ] **DECISION**: Share landing page URL publicly 🎉
- [ ] **DECISION**: Announce launch on social media/Product Hunt/etc.

---

## POST-LAUNCH ROADMAP 📋 (Add after initial launch)

### Completed Stages (Reference)
These stages are already complete and deployed:

#### Stage 1: WhatsApp Webhook → Agent API → Simple Responses ✅
- [x] WhatsApp webhook endpoint accepts inbound messages
- [x] Webhook verification endpoint is implemented
- [x] Orchestrator returns a reply and sends it back to WhatsApp
- [x] Inbound message idempotency is enforced
- [x] Render service is live and reachable at public URL
- [x] Meta webhook URL points to correct endpoint
- [x] Verify token in Meta matches `WHATSAPP_VERIFY_TOKEN` in Render env

#### Stage 2: Users + Preferences + Memory Notes ✅
- [x] Add `preferences` storage (table or JSONB on `users`)
- [x] Add onboarding prompts to collect preferences
- [x] Persist preferences from WhatsApp onboarding
- [x] Store minimal chat history (last N turns)
- [x] Summarize older history into `memory_notes`
- [x] Use preferences + memory notes in orchestrator prompts

#### Stage 3: Subscription Gate + Usage Metering ✅
- [x] Add `subscriptions` table
- [x] Add `usage` table (monthly period buckets)
- [x] Implement usage counters (messages, tokens, proposals)
- [x] Add entitlements check before skill execution
- [x] Add "limit hit" response with upgrade link
- [x] Add webhook handler to update subscription state
- [x] Add billing provider env vars on Render (Stripe keys + webhook secret)

#### Stage 4: Proposal Engine + Approval Inbox ✅
- [x] Add `proposals` table with status + payload
- [x] Add proposal creation helper (service or model method)
- [x] Update agent output to emit `proposal` or `assistant_message`
- [x] Add signed, short-lived approval links (JWT)
- [x] Implement Approval Inbox page (mobile-first)
- [x] Implement approve/edit/cancel endpoints
- [x] Add audit log for proposal approvals
- [x] Configure CORS for Approval Inbox domain in Render

#### Stage 5: Email + Calendar (Read-Only) + Daily Brief ✅
- [x] Confirm OAuth token storage is encrypted at rest
- [x] Implement daily brief worker (calendar + email summaries)
- [x] Store brief as messages and/or memory notes
- [x] Add manual trigger endpoint for testing
- [x] Add scheduled job (cron/queue)
- [x] Add Google OAuth redirect URL pointing to correct domain

#### Stage 6: Creative Studio + Wardrobe ✅
- [x] Implement creative prompt flow + templates
- [x] Add wardrobe onboarding (vibe, sizes, colors, budget)
- [x] Add outfit generation responses
- [x] Add wardrobe shopping list proposals

#### Stage 7: Commerce Proposals (Shopping/Food/Travel) ✅
- [x] Add proposal types: `food_order`, `travel_itinerary`, `purchase_cart`
- [x] Implement shopping compare + cart proposal builder
- [x] Implement food order + cart proposal builder
- [x] Implement travel shortlist + itinerary proposal builder

#### Stage 8: Monitoring/Alerts Workers ✅
- [x] Add queue + worker for monitoring jobs
- [x] Implement price/availability alert logic
- [x] Add user notification delivery

#### Stage 10: Payment Infrastructure (Stripe) ✅
- [x] Add `payment_methods` table
- [x] Add `transactions` table
- [x] Implement Stripe SDK integration for payment intents
- [x] Add payment method registration endpoint
- [x] Add 3D Secure (SCA) authentication flow
- [x] Implement payment intent creation with idempotency keys
- [x] Add webhook handler for `payment_intent.succeeded` and `payment_intent.failed`
- [x] Add refund/cancellation logic via Stripe API
- [x] Store payment receipts and generate PDF invoices
- [x] Add Stripe environment vars

#### Stage 14: Proposal Approval & Execution Engine ✅
- [x] Add `execution_logs` table
- [x] Implement JWT verification for approval links
- [x] Add proposal state machine (pending → approved → executing → completed/failed)
- [x] Build centralized execution orchestrator service
- [x] Implement pre-execution validation (budget check, payment method, required fields)
- [x] Add atomic execution steps with rollback on failure
- [x] Implement retry logic with exponential backoff for transient failures
- [x] Add execution status webhooks to notify user of progress
- [x] Implement "dry run" mode for testing without actual charges
- [x] Add manual intervention queue for flagged transactions
- [x] Build execution dashboard for monitoring pending/failed executions

---

### Stage 12: Food Delivery Execution 🍕
**Priority: Post-Launch Phase 1**

- [ ] Research food delivery APIs (DoorDash Drive API or Uber Eats API)
- [ ] Register for DoorDash Developer Platform
- [ ] Implement restaurant search and menu retrieval
- [ ] Implement cart creation and item validation
- [ ] Implement delivery address validation and geocoding
- [ ] Implement order placement via API
- [ ] Add real-time order tracking integration
- [ ] Implement order status webhooks (confirmed, preparing, out_for_delivery, delivered)
- [ ] Add delivery instructions and contact preferences
- [ ] Implement order cancellation within allowed timeframe
- [ ] Add tip calculation and customization
- [ ] Store order receipts and send confirmation via WhatsApp
- [ ] Add environment vars: `DOORDASH_API_KEY`, `DOORDASH_DEVELOPER_ID`

### Stage 13: Retail Shopping Checkout Execution 🛒
**Priority: Post-Launch Phase 2**

- [ ] Add `shipping_addresses` table (user_id, name, street, city, state, zip, country, is_default)
- [ ] Implement shopping cart consolidation from multiple retailers
- [ ] Add address validation via USPS or SmartyStreets API
- [ ] Implement tax calculation via TaxJar or Avalara API
- [ ] Add shipping rate calculation (partner with ShipStation or EasyPost)
- [ ] Implement Amazon Product Advertising API for direct purchase links
- [ ] Implement Walmart Affiliate API for purchase tracking
- [ ] Add order tracking integration (parse retailer emails or use AfterShip API)
- [ ] Implement order confirmation scraping or email parsing
- [ ] Add return/refund request handling
- [ ] Store order history with itemized receipts
- [ ] Add environment vars: `TAXJAR_API_KEY`, `EASYPOST_API_KEY`, `AMAZON_ASSOCIATE_TAG`

### Stage 9: Apple Messages Adapter 📱
**Priority: Post-Launch Phase 3** (OPTIONAL)

- [ ] Research Apple Messages for Business API
- [ ] Implement iMessage webhook endpoint
- [ ] Add message routing logic
- [ ] Test with Apple Business Chat
- [ ] Deploy to production

### Enhanced Monitoring & Analytics 📊
**Priority: Ongoing Post-Launch**

- [ ] Add reconciliation job (daily comparison of Stripe vs internal ledger)
- [ ] Implement failed transaction retry queue with manual approval
- [ ] Add detailed analytics dashboard (conversion rates, revenue, user engagement)
- [ ] Implement A/B testing framework for proposals
- [ ] Add user behavior tracking and cohort analysis
- [ ] Create weekly business metrics reports
- [ ] Implement cost optimization tracking (API usage, LLM costs)

### Advanced Security Features 🔒
**Priority: Post-Launch Phase 4**

- [ ] Implement device fingerprinting for fraud detection
- [ ] Add IP geolocation verification
- [ ] Implement anomaly detection with ML models
- [ ] Add advanced rate limiting with Redis
- [ ] Implement session management and token rotation
- [ ] Add security audit logging with Splunk or similar
- [ ] Implement DDoS protection with Cloudflare
- [ ] Add penetration testing and security audits

### Performance & Scalability 🚄
**Priority: Post-Launch Phase 5**

- [ ] Containerize services (Dockerfile)
- [ ] Configure managed Postgres + Redis
- [ ] Configure message queue (SQS/RabbitMQ)
- [ ] Set up secrets manager (AWS Secrets Manager)
- [ ] Configure backups + retention (automated daily backups, 30-day retention)
- [ ] Configure CDN for static assets (Cloudflare or CloudFront)
- [ ] Implement blue-green deployment strategy for zero-downtime updates
- [ ] Add database query optimization and indexing
- [ ] Implement caching layer (Redis)
- [ ] Add horizontal scaling with load balancers
- [ ] Set up auto-scaling based on traffic

---

## Quick Reference: Testing Commands

```bash
# Run all tests
pytest tests/ -v

# Run specific test file
pytest tests/test_amadeus_integration.py -v

# Run interactive travel API tests
python test_travel_api.py

# Check database migration status
alembic current

# Apply pending migrations
alembic upgrade head

# Check code formatting
black app/ tests/

# Run linter
flake8 app/ tests/
```

## Quick Reference: API Endpoints

- **Health**: `GET /health`
- **WhatsApp Webhook**: `POST /webhooks/whatsapp`
- **Travel Search**: `POST /travel/flights/search`, `POST /travel/hotels/search/city`
- **Execution Dashboard**: `GET /dashboard/summary`, `GET /dashboard/health`
- **Intervention Queue**: `GET /intervention/queue`
- **API Documentation**: `https://your-domain.com/docs`

---

**Last Updated**: 2026-02-06
**Status**: PRE-LAUNCH - Domain + Landing Page LIVE, Stage 15 + 17 Complete, WhatsApp skipped
