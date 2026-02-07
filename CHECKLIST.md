# Executive AI Agent — Development Checklist

**This checklist is for the actual product — not an MVP.** We are building the full product from PLAN.md.

**Vision** (from PLAN.md): The most comprehensive AI executive assistant — managing every aspect of life from travel to finances, relationships to health, career to home. 35 feature categories across 7 phases over 36–48 months.

**Strategy**: Build the **full product** from the plan. Architecture and features align with the plan's target stack (PostgreSQL + MongoDB + Redis, message queue, Kubernetes at scale). No "MVP-only" shortcuts.

---

## Current state snapshot

### What exists today (verified against codebase)

| Area | Implementation | Files | Status |
|------|---------------|-------|--------|
| **App framework** | FastAPI monolith, 28 route files, 49 services | `app/main.py`, `app/api/routes/`, `app/services/` | ✅ |
| **Database** | SQLAlchemy + Alembic, 22 tables, 12 migrations | `app/db/models.py`, `alembic/versions/` | ✅ |
| **AI orchestration** | OpenAI LLM, intent routing, memory, context | `app/services/agent.py`, `orchestrator.py`, `intent.py`, `memory.py` | ✅ |
| **WhatsApp** | Webhooks, routing, idempotency | `app/channels/whatsapp.py`, `app/api/routes/webhooks_whatsapp.py` | ✅ |
| **Users & memory** | Users, preferences, memory notes, chat history | `app/db/models.py` (User, UserPreference, MemoryNote, Conversation, ChatMessage) | ✅ |
| **Subscriptions & billing** | Stripe subscriptions, usage metering, entitlements | `app/services/subscriptions.py`, `usage.py`, `stripe_service.py` | ✅ |
| **Proposals & execution** | Proposals, JWT approval links, execution engine, rollback, intervention | `app/services/execution_engine.py` (31k lines), `intervention_service.py` | ✅ |
| **Email & calendar** | Google OAuth, Gmail, Google Calendar, daily brief | `app/services/google_oauth.py`, `google_gmail.py`, `google_calendar.py`, `daily_brief.py` | ✅ |
| **Payments** | Stripe payment methods, intents, 3DS, refunds, invoice PDF | `app/services/stripe_service.py`, `invoice_service.py` | ✅ |
| **Travel** | Amadeus flights/hotels, traveler profiles (PII encrypted), e-tickets | `app/services/amadeus_service.py` (21k lines), `eticket_service.py` | ✅ |
| **Security** | Rate limiting, PII encryption (Fernet), circuit breakers, GDPR, spending limits | `app/services/encryption_service.py`, `circuit_breaker.py`, `gdpr_service.py` | ✅ |
| **Voice** | Twilio Voice, ElevenLabs TTS, Deepgram STT, voice_calls table, scenarios | `app/api/routes/voice.py`, `app/services/twilio_voice_service.py`, `deepgram_service.py`, `elevenlabs_service.py` | ✅ |
| **Webhooks** | User webhook registration, event subscriptions, delivery with retries | `app/services/webhook_service.py`, `app/api/routes/webhooks.py` | ✅ |
| **Landing** | Next.js on Vercel, domain | External repo | ✅ |

### Architecture gaps (must fix for production)

| Gap | Current state | Target | Priority |
|-----|--------------|--------|----------|
| **PostgreSQL everywhere** | SQLite in dev (`app.db`) | PostgreSQL in all environments | **P0** |
| **Redis** | Referenced in rate_limiter but not fully integrated | Redis for sessions, context, cache, job locks | **P0** |
| **MongoDB** | Not present | Document/event store for raw payloads, logs, flexible data | **P1** |
| **Message queue** | In-process APScheduler (`app/services/scheduler.py`) | Redis + Celery (or RabbitMQ) for background workers | **P1** |
| **Config hardening** | `JWT_SECRET = "dev_only_change_me"` in `app/core/config.py` | Production guard, centralized env validation | **P0** |
| **Structured logging** | `print()` in `scheduler.py` and others | JSON structured logging with correlation IDs | **P1** |
| **App identity** | `title="Shopping Assistant Backend"` in `app/main.py` | "Executive AI Agent" | **P0** |

---

## IMMEDIATE: Codebase corrections (do first)

These are specific changes to existing files to fix identity, config, and production readiness.

### App identity & naming

- [ ] **`app/main.py`**: Change `title="Shopping Assistant Backend"` → `"Executive AI Agent"`
- [ ] **`app/main.py`**: Update root endpoint `"service"` value to `"Executive AI Agent"`
- [ ] **`app/main.py`**: Remove comment `"Create tables at startup (MVP)"` — Alembic handles migrations

### Config hardening (`app/core/config.py`)

- [ ] Add production guard: raise error if `JWT_SECRET == "dev_only_change_me"` when `ENV != "dev"`
- [ ] Add `ENV: str = "dev"` setting to distinguish environments
- [ ] Centralize ALL env vars into `Settings` class — currently `STRIPE_*`, `AMADEUS_*`, `OPENAI_API_KEY`, `PII_ENCRYPTION_KEY` are read directly from `os.environ` in service files instead of from Settings
- [ ] Add `REDIS_URL: str | None = None` to Settings
- [ ] Add `MONGODB_URI: str | None = None` to Settings
- [ ] Add `CELERY_BROKER_URL: str | None = None` to Settings
- [ ] Add validation: warn on startup if critical keys are missing (OPENAI_API_KEY, DATABASE_URL, PII_ENCRYPTION_KEY)

### Health & readiness (`app/api/routes/health.py`)

- [ ] Add **readiness** endpoint (`/health/ready`): check DB connectivity (`SELECT 1`), Redis ping (when added), critical env vars present
- [ ] Add **liveness** endpoint (`/health/live`): simple 200 (process is up) — for Kubernetes probes
- [ ] Return `503` if any readiness check fails
- [ ] Include version/commit hash in health response for deployment tracking

### Execution engine TODOs (`app/services/execution_engine.py`)

- [ ] Replace `# TODO: Call DoorDash API to place order` — either integrate a real food delivery API or remove the mock flow and mark as "coming soon" in the orchestrator
- [ ] Replace `# TODO: Implement retail checkout` — either integrate a real retail/affiliate API or remove mock
- [ ] These are the only remaining placeholder TODOs in the execution engine

### Logging cleanup

- [ ] **`app/services/scheduler.py`**: Replace all `print(...)` with `logging.getLogger(__name__).info/error`
- [ ] Audit all 49 service files: ensure no `print()` used for operational messages — use structured logging
- [ ] Add a logging config module (`app/core/logging.py`) with JSON formatter for production

### API documentation

- [ ] Add OpenAPI tags and descriptions for all 28 route groups in `app/main.py` where `app.include_router()` is called
- [ ] Add `/v1/` API version prefix for all routes (future-proofing)

---

## FOUNDATION: Architecture upgrades (build production stack)

These implement the plan's full infrastructure. Required before scaling.

### PostgreSQL (all environments)

- [ ] Ensure `DATABASE_URL` always points to PostgreSQL (never SQLite) outside local dev
- [ ] Add a startup check in `app/main.py`: if `DATABASE_URL` starts with `sqlite` and `ENV != "dev"`, raise an error
- [ ] Document the Alembic migration workflow for team use
- [ ] Plan for read replicas when read load justifies it (future)

### Redis integration

- [ ] Deploy Redis (local: Docker, production: managed Redis — e.g., Render Redis, AWS ElastiCache, Upstash)
- [ ] Add `redis` async client setup in `app/core/redis.py` — connection pool, health check
- [ ] **Rate limiting**: `app/middleware/rate_limiter.py` already supports Redis — ensure `REDIS_URL` is set and used in production
- [ ] **Session/conversation context**: Move conversation state from in-memory/DB to Redis for low-latency access across instances
  - Store active conversation context (last N messages, current intent, pending confirmations) in Redis with TTL
  - Keep long-term history in PostgreSQL (as now)
- [ ] **Cache layer**: Cache frequently-read data (user preferences, subscription tier, active proposals) in Redis
- [ ] **Job locks**: Use Redis for distributed locks in background workers (prevent duplicate job execution)

### MongoDB integration

- [ ] Deploy MongoDB (local: Docker, production: MongoDB Atlas or managed)
- [ ] Add `motor` (async MongoDB driver) to `requirements.txt`
- [ ] Create `app/core/mongodb.py` — connection setup, health check
- [ ] Use MongoDB for:
  - Raw inbound message payloads (WhatsApp, SMS, voice transcripts) — currently these are partly in PostgreSQL
  - Event/audit logs (high-volume, flexible schema)
  - AI conversation traces (full prompt/response pairs for debugging and improvement)
  - File/document metadata index (for Phase 2.5 file search)
- [ ] Keep PostgreSQL as source of truth for relational data (users, subscriptions, proposals, bookings, transactions)

### Message queue & background workers

- [ ] Choose: **Redis + Celery** (simplest — reuses Redis, good for our scale) — recommended
- [ ] Add `celery[redis]` to `requirements.txt`
- [ ] Create `app/core/celery_app.py` — Celery configuration, task autodiscovery
- [ ] Create `app/tasks/` directory for Celery task modules
- [ ] Migrate background jobs from `app/services/scheduler.py` (APScheduler) to Celery tasks:
  - `daily_brief_task` — daily brief generation
  - `price_refresh_task` — price watch refresh
  - `calendar_sync_task` — calendar sync
- [ ] Add new async worker tasks:
  - `email_monitor_task` — proactive email monitoring (Phase 2.3)
  - `calendar_monitor_task` — proactive calendar alerts (Phase 2.4)
  - `webhook_delivery_task` — async webhook delivery (currently sync in `webhook_service.py`)
  - `notification_task` — notification dispatch
- [ ] Add Celery Beat for scheduled/periodic tasks (replaces APScheduler cron)
- [ ] Add flower or similar for Celery monitoring dashboard

### Platform abstraction layer

- [ ] Create `app/services/messaging.py` — unified interface for sending messages across channels:
  ```
  send_message(user_id, channel, content) → dispatches to WhatsApp / SMS / Email / Push
  ```
- [ ] This abstracts `app/channels/whatsapp.py`, future SMS (Twilio), email (`email_service.py`), and push notifications
- [ ] All services should call the abstraction, not individual channel implementations

---

## Phase 1: Foundation (plan Months 1–6) — COMPLETE ✅

### 1.1 Messaging infrastructure ✅
- [x] WhatsApp webhook endpoint and verification (`app/api/routes/webhooks_whatsapp.py`)
- [x] Message routing and responses (`app/channels/whatsapp.py`)
- [x] Inbound message idempotency (`InboundEvent` model)
- [x] Deployment on Render

### 1.2 Users & memory ✅
- [x] User preferences storage (`UserPreference` model)
- [x] Onboarding flow
- [x] Chat history persistence (`Conversation`, `ChatMessage` models)
- [x] Memory notes summarization (`MemoryNote`, `app/services/memory.py`)
- [x] Context-aware prompts (orchestrator injects memory, preferences, entitlements)

### 1.3 Subscriptions & billing ✅
- [x] Subscriptions table (Stripe-backed) (`Subscription` model)
- [x] Usage metering (`Usage` model, `app/services/usage.py`)
- [x] Entitlements and limit enforcement
- [x] Stripe webhook handlers (`app/api/routes/billing_stripe.py`)

### 1.4 Proposals & execution engine ✅
- [x] Proposals table, JWT-signed approval links (`Proposal` model, `app/services/proposal_links.py`)
- [x] Approval inbox UI and approve/edit/cancel endpoints (`app/api/routes/proposals.py`)
- [x] Execution logs, proposal state machine, pre-execution validation (`ExecutionLog` model)
- [x] Atomic execution with rollback, retry, manual intervention queue (`app/services/execution_engine.py`)

### 1.5 Email & calendar ✅
- [x] Google OAuth (`app/services/google_oauth.py`)
- [x] Gmail API integration (`app/services/google_gmail.py`)
- [x] Google Calendar integration (`app/services/google_calendar.py`)
- [x] Daily brief worker (`app/services/daily_brief.py`, `scheduler.py`)

### 1.6 Payment infrastructure ✅
- [x] Payment methods and transactions tables (`PaymentMethod`, `Transaction` models)
- [x] Stripe payment intents, 3D Secure (`app/services/stripe_service.py`)
- [x] Refund logic and PDF invoice generation (`app/services/invoice_service.py`)

### 1.7 Travel booking ✅
- [x] Amadeus API — flights and hotels (`app/services/amadeus_service.py`)
- [x] Traveler profiles with PII encryption (`TravelerProfile` model, `app/services/encryption_service.py`)
- [x] E-ticket PDF generation (`app/services/eticket_service.py`)
- [x] Confirmation emails, cancellation/modification (`app/api/routes/bookings.py`)

### 1.8 Security ✅
- [x] Rate limiting — user + IP (`app/middleware/rate_limiter.py`)
- [x] PII encryption — Fernet (`app/services/encryption_service.py`)
- [x] Circuit breakers — all external APIs (`app/services/circuit_breaker.py`)
- [x] GDPR data deletion (`app/services/gdpr_service.py`, `app/api/routes/gdpr.py`)
- [x] Spending limits and velocity checks (`SpendingLimit` model)

### 1.9 Landing & identity ✅
- [x] Domain and Next.js landing on Vercel

---

## Phase 2: Core services (plan Months 7–12)

### 2.1 Voice calling ✅ COMPLETE
- [x] Twilio Voice: webhooks, outbound call initiation, call state (`app/services/twilio_voice_service.py`)
- [x] ElevenLabs TTS — 5 voice profiles, streaming audio (`app/services/elevenlabs_service.py`, `tts_elevenlabs.py`)
- [x] Deepgram STT — real-time streaming transcription (`app/services/deepgram_service.py`, `stt_deepgram.py`)
- [x] `VoiceCall` model (direction, to/from, purpose, status, duration, recording_url, transcript, summary, action_items)
- [x] Call scripting and scenarios (`app/services/voice_scenarios.py`)
- [x] Real-time AI conversation via WebSocket (`app/api/routes/voice.py`)
- [x] Transcript and summary storage
- [ ] **Production hardening**: Verify Twilio/Deepgram/ElevenLabs error handling in production; document all voice env vars in `RENDER_ENV_VARS.md`

### 2.2 SMS & multi-platform messaging

**Goal**: Send/receive SMS via Twilio, manage contacts, message on user's behalf.

- [ ] **Twilio SMS setup**: Configure Twilio SMS in `app/core/config.py` (already have `TWILIO_*` vars) — add SMS webhook endpoint
- [ ] **New route file**: `app/api/routes/sms.py` — receive SMS (Twilio webhook), send SMS, delivery status webhook
- [ ] **New service**: `app/services/sms_service.py` — send SMS via Twilio REST API, handle delivery receipts
- [ ] **Contacts table**: Add `Contact` model to `app/db/models.py`:
  ```
  Contact: id, user_id, name, phone, email, relationship, notes, tags,
           last_contact_date, importance_score, source, created_at, updated_at
  ```
- [ ] **Alembic migration**: Create migration for `contacts` table
- [ ] **New route**: `app/api/routes/contacts.py` — CRUD for contacts, import, search, deduplication
- [ ] **New service**: `app/services/contact_service.py` — contact management, deduplication logic, enrichment
- [ ] **Send-on-behalf**: Draft/preview SMS before sending (reuse proposal confirmation pattern), delivery tracking
- [ ] **MMS support**: Handle media messages (images, documents) via Twilio MMS
- [ ] Wire SMS into platform abstraction (`app/services/messaging.py`)

### 2.3 Proactive email intelligence

**Goal**: Background monitoring, AI-scored alerts, smart search, autonomous email management.

- [ ] **Email monitoring config**: Add `EmailMonitorConfig` model to `app/db/models.py`:
  ```
  EmailMonitorConfig: id, user_id, enabled, check_interval_minutes, quiet_hours_start,
                      quiet_hours_end, importance_threshold, keywords, sender_whitelist
  ```
- [ ] **Email alerts table**: Add `EmailAlert` model:
  ```
  EmailAlert: id, user_id, email_id, subject, sender, importance_score, category,
              summary, suggested_action, status (new/read/acted), created_at
  ```
- [ ] **Alembic migration**: Create migration for email monitoring tables
- [ ] **Celery task**: `app/tasks/email_monitor.py` — periodic email check via Gmail API, AI importance scoring (OpenAI), generate alerts
- [ ] **Enhance** `app/services/google_gmail.py`: Add methods for email categorization, thread analysis, attachment handling
- [ ] **Smart response suggestions**: Generate draft responses using AI, route through proposal/approval engine for user confirmation
- [ ] **Semantic email search** (requires vector DB):
  - [ ] Add `pinecone-client` (or `chromadb` for self-hosted) to `requirements.txt`
  - [ ] Create `app/services/embedding_service.py` — generate embeddings via OpenAI `text-embedding-3-small`
  - [ ] Create `app/services/vector_search.py` — index emails, semantic search endpoint
  - [ ] **Celery task**: `app/tasks/email_indexer.py` — background indexing of new emails into vector DB
- [ ] **Email templates**: `app/services/email_templates.py` — reusable templates for common responses
- [ ] **Follow-up reminders**: Track sent emails without replies, generate follow-up suggestions

### 2.4 Proactive calendar intelligence

**Goal**: Multi-calendar support, proactive alerts, meeting prep, smart scheduling.

- [ ] **Multi-calendar**: Add Microsoft Graph API integration for Outlook Calendar
  - [ ] Add `msal` to `requirements.txt`
  - [ ] Create `app/services/microsoft_graph.py` — OAuth, calendar read/write
  - [ ] Apple CalDAV support (if needed — via `caldav` library)
- [ ] **Calendar alerts table**: Add `CalendarAlert` model:
  ```
  CalendarAlert: id, user_id, event_id, alert_type (conflict/prep/travel/followup),
                 message, scheduled_for, sent_at, status
  ```
- [ ] **Alembic migration**: Create migration for calendar alert tables
- [ ] **Celery task**: `app/tasks/calendar_monitor.py` — check upcoming events, detect conflicts, calculate travel time, generate pre-meeting briefs
- [ ] **Enhance** `app/services/google_calendar.py`:
  - Smart scheduling: find optimal meeting times considering buffer time, travel time, timezone
  - Conflict detection with suggested resolutions
- [ ] **Meeting prep automation**: For upcoming meetings, pull relevant emails/contacts/notes and generate briefing
- [ ] **Follow-up automation**: After meetings, extract action items (from voice transcript if call, or from notes), create follow-up tasks/drafts

### 2.5 File & photo management (plan Month 9)

**Goal**: Index and search files/photos across devices using semantic search.

- [ ] **File metadata table**: Add `FileMetadata` model to `app/db/models.py`:
  ```
  FileMetadata: id, user_id, filename, file_type, file_size, path, cloud_url,
                tags, description, embedding_id, indexed_at, created_at
  ```
- [ ] **Alembic migration**: Create migration for file metadata
- [ ] **New service**: `app/services/file_service.py` — file indexing, metadata extraction, search
- [ ] **New route**: `app/api/routes/files.py` — upload, search, retrieve file metadata
- [ ] **Vector embeddings for files**: Reuse `app/services/embedding_service.py` to embed file content/descriptions
- [ ] **Photo search**: Integrate OpenAI Vision or similar for image description → embedding → search
- [ ] **Companion app API**: Endpoints for mobile app to sync local file/photo index

### 2.6 Smart home integration (plan Month 11)

**Goal**: Control smart home devices via natural language through the AI agent.

- [ ] **Device table**: Add `SmartDevice` model:
  ```
  SmartDevice: id, user_id, device_name, device_type, platform (homekit/google/alexa/smartthings),
               room, status, capabilities (JSON), last_seen, external_id
  ```
- [ ] **Scene table**: Add `SmartScene` model:
  ```
  SmartScene: id, user_id, name, devices_config (JSON), trigger_type, schedule
  ```
- [ ] **Alembic migration**: Create migration for smart home tables
- [ ] **New service**: `app/services/smart_home.py` — unified interface for device control across platforms
- [ ] **Platform adapters**: `app/services/smart_home_google.py`, `smart_home_homekit.py` (start with Google Home API)
- [ ] **New route**: `app/api/routes/smart_home.py` — list devices, control device, manage scenes
- [ ] **Voice integration**: Wire smart home commands into voice AI (`app/services/voice_ai.py`) for voice-controlled devices
- [ ] **Energy monitoring**: Track energy usage data from smart devices

### 2.7 Execution engine — complete real flows

- [ ] **Food delivery**: Research and integrate a food delivery API (DoorDash Drive, Uber Eats, or similar partner API) in `app/services/execution_engine.py` — replace `# TODO: Call DoorDash API`
- [ ] **Retail checkout**: Research and integrate an affiliate/retail API (Amazon Product Advertising, or similar) — replace `# TODO: Implement retail checkout`
- [ ] If no API is available yet, create proper "not yet available" responses in the orchestrator instead of mock confirmations

### 2.8 Beta launch prep (plan Month 12)

- [ ] Onboard beta testers — create invite system (invite codes table, limited access)
- [ ] Build help docs and in-app FAQ (can be served from a static endpoint or integrated into chat)
- [ ] **Usage analytics**: Track feature usage per user (which services they use, frequency, success rate)
  - Add `UsageEvent` model or extend `Usage` model
  - Celery task to aggregate analytics
- [ ] Feedback collection: endpoint for user feedback, stored in MongoDB
- [ ] Performance profiling: identify slow endpoints, optimize DB queries, add indexes
- [ ] Cost audit: track per-user API costs (OpenAI, Twilio, Amadeus, etc.)

---

## Phase 3: Lifestyle features (plan Months 13–18)

### 3.1 Wardrobe & style

- [ ] **Tables**: `WardrobeItem` (user_id, category, color, brand, season, image_url, wear_count, last_worn), `StylePreference` (user_id, style_type, preferred_colors, body_type, budget_range)
- [ ] **Alembic migration**
- [ ] **Service**: `app/services/wardrobe_service.py` — image upload + OpenAI Vision for clothing recognition, categorization
- [ ] **Route**: `app/api/routes/wardrobe.py` — CRUD items, get outfit suggestions
- [ ] Integrate weather API (OpenWeatherMap) for weather-based outfit suggestions — add `OPENWEATHER_API_KEY` to config
- [ ] Outfit recommendation algorithm: occasion + weather + style preference + wear history
- [ ] Shopping recommendations: integrate e-commerce API for similar items
- [ ] Note: `app/services/wardrobe_agent.py` already exists (3.8k lines) — extend it with real wardrobe data

### 3.2 Gift management

- [ ] **Tables**: `GiftOccasion` (user_id, contact_id, occasion_type, date, recurring), `GiftIdea` (occasion_id, description, url, price, status, purchased_at)
- [ ] **Alembic migration**
- [ ] **Service**: `app/services/gift_service.py` — occasion tracking, AI-powered gift recommendations based on contact interests/relationship
- [ ] **Route**: `app/api/routes/gifts.py` — manage occasions, get recommendations, track purchases
- [ ] Integrate gift purchasing through existing proposal + payment flow (Stripe)
- [ ] Reminder system via Celery: send reminders N days before occasions
- [ ] Thank you note drafting via AI

### 3.3 Relationship manager (personal CRM)

- [ ] **Tables**: `Interaction` (user_id, contact_id, type (call/text/email/meeting), date, notes, sentiment), extend `Contact` model with `relationship_score`, `birthday`, `anniversary`
- [ ] **Alembic migration**
- [ ] **Service**: `app/services/relationship_service.py` — communication frequency tracking, relationship health scoring, reach-out suggestions
- [ ] **Route**: `app/api/routes/relationships.py` — relationship dashboard, suggestions, interaction history
- [ ] NLP extraction: automatically extract important details from conversations (birthdays, preferences, family info)
- [ ] Proactive reach-out suggestions via notification system

### 3.4 Fitness & nutrition

- [ ] **Tables**: `Workout` (user_id, type, duration, calories, date), `Meal` (user_id, description, calories, macros, date), `FitnessGoal` (user_id, goal_type, target, current, deadline)
- [ ] **Alembic migration**
- [ ] **Service**: `app/services/fitness_service.py` — workout tracking, meal logging, goal progress
- [ ] **Route**: `app/api/routes/fitness.py` — log workouts/meals, get suggestions, view progress
- [ ] Integrate Apple Health / Google Fit via their APIs (if companion app is built)
- [ ] AI-powered workout suggestions and meal planning based on goals
- [ ] Nutritionix API integration for food nutrition data — add `NUTRITIONIX_API_KEY` to config

### 3.5 Entertainment curator

- [ ] **Tables**: `ContentPreference` (user_id, genre_preferences, streaming_services), `ContentItem` (user_id, title, type (movie/show/book/music), status (watched/reading/queue), rating, notes)
- [ ] **Alembic migration**
- [ ] **Service**: `app/services/entertainment_service.py` — recommendations, tracking, mood-based suggestions
- [ ] **Route**: `app/api/routes/entertainment.py` — manage queue, get recommendations, log watched/read
- [ ] Integrate TMDB API for movies/shows — add `TMDB_API_KEY` to config
- [ ] Integrate Spotify API for music — add `SPOTIFY_*` to config
- [ ] Event discovery: Ticketmaster or Eventbrite API for local events + booking

### 3.6 Learning & skills

- [ ] **Tables**: `LearningGoal` (user_id, skill, target_level, current_level, deadline), `LearningResource` (goal_id, title, type, url, completed), `PracticeSession` (goal_id, duration, notes, date)
- [ ] **Alembic migration**
- [ ] **Service**: `app/services/learning_service.py` — goal tracking, resource curation, practice scheduling
- [ ] **Route**: `app/api/routes/learning.py` — manage goals, log practice, get resources
- [ ] AI-powered resource recommendations based on skill and level
- [ ] Practice reminders via Celery scheduled tasks
- [ ] Language learning: conversational practice via AI chat (reuse chat infrastructure)

---

## Phase 4: Advanced intelligence (plan Months 19–24)

### 4.1 Pattern recognition & proactive intelligence

- [ ] **Tables**: `UserPattern` (user_id, pattern_type, data JSON, confidence, first_seen, last_seen, occurrences)
- [ ] **Alembic migration**
- [ ] **Service**: `app/services/pattern_service.py` — behavioral analysis, pattern detection, predictive suggestions
- [ ] Celery task: analyze user activity patterns periodically (time-of-day preferences, common requests, spending patterns)
- [ ] Feed patterns into AI context for proactive suggestions

### 4.2 Context-aware notifications

- [ ] **Tables**: `NotificationRule` (user_id, trigger_type, conditions JSON, channel, priority), extend `NotificationQueue` model
- [ ] **Alembic migration**
- [ ] **Service**: `app/services/smart_notifications.py` — context detection (location, calendar, time, weather), priority scoring, batching, DND intelligence
- [ ] Route notifications through platform abstraction (`app/services/messaging.py`)
- [ ] Respect user quiet hours and preferences

### 4.3 Decision support system

- [ ] **Tables**: `Decision` (user_id, title, description, options JSON, pros_cons JSON, recommendation, outcome, decided_at)
- [ ] **Alembic migration**
- [ ] **Service**: `app/services/decision_service.py` — pros/cons analysis, scenario comparison, risk assessment via AI
- [ ] **Route**: `app/api/routes/decisions.py` — create decision, analyze options, track outcomes
- [ ] Outcome learning: track what decisions worked out and feed back into future recommendations

### 4.4 Habit formation

- [ ] **Tables**: `Habit` (user_id, name, frequency, target, current_streak, best_streak, created_at), `HabitLog` (habit_id, completed_at, notes)
- [ ] **Alembic migration**
- [ ] **Service**: `app/services/habit_service.py` — streak tracking, reminders, failure analysis, strategy adjustment
- [ ] **Route**: `app/api/routes/habits.py` — CRUD habits, log completions, view streaks/analytics
- [ ] Celery task: daily habit reminders, weekly habit reviews
- [ ] Celebration messages for milestones (streaks, goals met)

### 4.5 Life logging & memory

- [ ] **Tables**: `JournalEntry` (user_id, date, content, mood, tags, auto_generated, source), `Milestone` (user_id, title, date, category, description, photo_url)
- [ ] **Alembic migration**
- [ ] **Service**: `app/services/journal_service.py` — automatic journaling from daily activity, "on this day" memories, yearly summaries
- [ ] **Route**: `app/api/routes/journal.py` — view/edit entries, milestones, summaries
- [ ] Celery task: end-of-day auto-journal generation from chat history, calendar events, interactions

### 4.6 Goal manifestation

- [ ] **Tables**: `Goal` (user_id, title, description, category, target_date, status, progress_pct), `GoalMilestone` (goal_id, title, target_date, completed_at, order)
- [ ] **Alembic migration**
- [ ] **Service**: `app/services/goal_service.py` — goal breakdown, milestone tracking, progress monitoring, obstacle identification
- [ ] **Route**: `app/api/routes/goals.py` — CRUD goals/milestones, progress updates, accountability reports
- [ ] Celery task: weekly goal progress check-ins, proactive suggestions

---

## Phase 5: Financial & career (plan Months 25–30)

### 5.1 Financial integration (Plaid)

- [ ] Add `plaid-python` to `requirements.txt`
- [ ] Add `PLAID_CLIENT_ID`, `PLAID_SECRET`, `PLAID_ENV` to `app/core/config.py`
- [ ] **Tables**: `FinancialAccount` (user_id, plaid_item_id, institution, account_type, name, balance, last_synced), `FinancialTransaction` (account_id, plaid_transaction_id, amount, category, merchant, date, description)
- [ ] **Alembic migration**
- [ ] **Service**: `app/services/plaid_service.py` — account linking (Link token), transaction sync, balance refresh
- [ ] **Route**: `app/api/routes/finance.py` — link accounts, view balances, view/search transactions
- [ ] **Celery task**: `app/tasks/finance_sync.py` — periodic transaction sync from Plaid
- [ ] AI-powered transaction categorization and spending analysis
- [ ] Budget tracking: set budgets by category, track against actuals, alert on overages

### 5.2 Bill management & negotiation

- [ ] **Tables**: `Bill` (user_id, name, provider, amount, due_date, frequency, category, auto_pay, status), `BillNegotiation` (bill_id, original_amount, negotiated_amount, savings, status, call_id FK to VoiceCall)
- [ ] **Alembic migration**
- [ ] **Service**: `app/services/bill_service.py` — recurring bill detection from transactions, payment reminders, negotiation scripting
- [ ] **Route**: `app/api/routes/bills.py` — manage bills, initiate negotiation, track savings
- [ ] **Voice integration**: Use existing voice calling (Twilio + ElevenLabs + Deepgram) for automated bill negotiation calls — this is a killer feature leveraging existing infrastructure
- [ ] Subscription audit: detect subscriptions from transactions, identify unused/duplicate subscriptions

### 5.3 Investment management

- [ ] **Tables**: `InvestmentAccount` (user_id, broker, account_type, balance, last_synced), `Holding` (account_id, symbol, shares, avg_cost, current_price, value)
- [ ] **Alembic migration**
- [ ] **Service**: `app/services/investment_service.py` — portfolio tracking, market data (Alpha Vantage or similar), performance analysis
- [ ] **Route**: `app/api/routes/investments.py` — view portfolio, performance, alerts
- [ ] Rebalancing suggestions, tax-loss harvesting alerts, dividend tracking
- [ ] Retirement projection calculator

### 5.4 Crypto & alternative investments

- [ ] **Tables**: `CryptoHolding` (user_id, exchange, asset, amount, avg_cost, current_price)
- [ ] **Alembic migration**
- [ ] **Service**: `app/services/crypto_service.py` — exchange API integration (Coinbase, Binance), portfolio tracking, price alerts
- [ ] **Route**: `app/api/routes/crypto.py` — view holdings, performance, alerts
- [ ] Gas fee optimization suggestions, staking tracking, tax reporting helpers

### 5.5 Career development & job search

- [ ] **Tables**: `CareerProfile` (user_id, current_role, target_role, skills JSON, achievements JSON), `JobApplication` (user_id, company, role, status, applied_at, source, notes, resume_version)
- [ ] **Alembic migration**
- [ ] **Service**: `app/services/career_service.py` — skill tracking, job market monitoring, resume/cover letter generation via AI
- [ ] **Route**: `app/api/routes/career.py` — manage profile, track applications, get recommendations
- [ ] Job board integration: scrape/API for job matching (LinkedIn, Indeed APIs)
- [ ] Interview prep: AI-powered mock interviews using existing voice infrastructure
- [ ] Salary benchmarking and negotiation support

---

## Phase 6: Family & health (plan Months 31–36)

### 6.1 Childcare coordination

- [ ] **Tables**: `Child` (user_id, name, age, school, grade), `ChildActivity` (child_id, name, schedule, location, instructor, cost), `Carpool` (activity_id, participants JSON, schedule)
- [ ] **Alembic migration**
- [ ] **Service**: `app/services/childcare_service.py` — schedule management, activity booking, carpool coordination
- [ ] **Route**: `app/api/routes/childcare.py` — manage children, activities, carpools
- [ ] School calendar integration (iCal import)
- [ ] Parent coordination: share schedules with co-parents/caregivers via messaging

### 6.2 Elderly parent care

- [ ] **Tables**: `CareRecipient` (user_id, name, relationship, conditions JSON), `Medication` (recipient_id, name, dosage, schedule, pharmacy, refill_date), `CareAppointment` (recipient_id, doctor, specialty, date, notes)
- [ ] **Alembic migration**
- [ ] **Service**: `app/services/elderly_care_service.py` — medication tracking, appointment scheduling, health monitoring, sibling coordination
- [ ] **Route**: `app/api/routes/elderly_care.py` — manage recipients, medications, appointments
- [ ] Medication refill reminders, appointment reminders (via messaging/voice)
- [ ] Emergency contacts and alert system

### 6.3 Medical management

- [ ] **Tables**: `MedicalRecord` (user_id, type, provider, date, description, file_url), `Prescription` (user_id, medication, dosage, prescriber, pharmacy, refill_date, status), `HealthInsurance` (user_id, provider, plan, member_id, coverage JSON)
- [ ] **Alembic migration**
- [ ] **Service**: `app/services/medical_service.py` — record management, appointment booking, prescription tracking, insurance navigation
- [ ] **Route**: `app/api/routes/medical.py` — manage records, prescriptions, find providers
- [ ] Pharmacy API integration for prescription refills
- [ ] Insurance claim tracking and benefits explanation

### 6.4 Mental wellness

- [ ] **Tables**: `MoodLog` (user_id, mood_score, energy_level, notes, date), `WellnessCheckin` (user_id, stress_level, sleep_hours, exercise, date)
- [ ] **Alembic migration**
- [ ] **Service**: `app/services/wellness_service.py` — mood tracking, stress detection from patterns, work-life balance monitoring, burnout detection
- [ ] **Route**: `app/api/routes/wellness.py` — log mood/wellness, view trends, get suggestions
- [ ] AI-powered wellness insights and suggestions (meditation, breaks, vacation)
- [ ] Integration with fitness data for holistic health view

---

## Phase 7: Home, travel, safety, polish (plan Months 37–48)

### 7.1 Smart home advanced

- [ ] Energy optimization algorithms (cost-aware scheduling)
- [ ] Predictive maintenance alerts (based on device usage patterns)
- [ ] Advanced security monitoring integration
- [ ] Automated routines based on patterns (wake up, leave home, arrive home, sleep)

### 7.2 Home maintenance

- [ ] **Tables**: `HomeAppliance` (user_id, name, brand, model, purchase_date, warranty_until, maintenance_schedule), `MaintenanceTask` (appliance_id, task, due_date, completed_at, provider, cost), `ServiceProvider` (user_id, name, specialty, phone, rating, notes)
- [ ] **Alembic migration**
- [ ] **Service**: `app/services/home_maintenance_service.py` — maintenance scheduling, service provider booking (via voice calls), warranty tracking
- [ ] **Route**: `app/api/routes/home.py`

### 7.3 Travel enhancements (build on existing Amadeus infrastructure)

- [ ] **Tables**: `Trip` (user_id, name, destination, start_date, end_date, status, bookings JSON), `PackingList` (trip_id, items JSON, checked JSON)
- [ ] **Alembic migration**
- [ ] **Service**: `app/services/trip_service.py` — trip planning (combine flights + hotels + activities), packing list generation via AI, document checklist (passport, visa, insurance)
- [ ] **Route**: `app/api/routes/trips.py`
- [ ] Travel preparation: credit card travel notifications, travel insurance suggestions
- [ ] During-trip: local recommendations, currency conversion, emergency contacts
- [ ] Post-trip: expense summary, photo organization

### 7.4 Vehicle management

- [ ] **Tables**: `Vehicle` (user_id, make, model, year, mileage, vin, license_plate), `VehicleMaintenance` (vehicle_id, service, date, mileage, cost, provider, next_due)
- [ ] **Alembic migration**
- [ ] **Service**: `app/services/vehicle_service.py` — maintenance tracking, service reminders, cost tracking
- [ ] **Route**: `app/api/routes/vehicles.py`

### 7.5 Emergency & safety

- [ ] **Tables**: `EmergencyContact` (user_id, name, phone, relationship, priority), `SafetyCheckin` (user_id, location, status, expected_next, escalation_plan)
- [ ] **Alembic migration**
- [ ] **Service**: `app/services/safety_service.py` — check-in system, emergency response, crisis management
- [ ] **Route**: `app/api/routes/safety.py`
- [ ] Automated escalation: if user misses check-in, notify emergency contacts
- [ ] Crisis response: connect with emergency services, share location

### 7.6 Legal assistant

- [ ] **Tables**: `LegalDocument` (user_id, type, title, file_url, expiry_date, notes), `LegalDeadline` (document_id, deadline_type, date, status)
- [ ] **Alembic migration**
- [ ] **Service**: `app/services/legal_service.py` — document tracking, deadline management, contract review (AI-assisted), lawyer finding
- [ ] **Route**: `app/api/routes/legal.py`

### 7.7 Personal brand & legacy

- [ ] **Online presence**: Social media management, content suggestions, reputation monitoring
- [ ] **Life logging**: Extend journal service with yearly summaries, memory books, timeline visualization
- [ ] **Legacy building**: Digital estate planning, password inheritance prep, legacy letters
- [ ] **Service**: `app/services/brand_service.py`, extend `app/services/journal_service.py`

### 7.8 Enterprise & scale

- [ ] **Team accounts**: `Team`, `TeamMember` models; multi-user workspaces, roles and permissions
- [ ] **Admin dashboard**: Internal admin for users, usage, proposals, executions, support
- [ ] **API access**: Public API with API keys, rate limits, docs (OpenAPI)
- [ ] **White-label**: Branding/config for partners
- [ ] **SSO**: SAML/OIDC for enterprise customers
- [ ] **SLA**: Defined uptime/support guarantees, incident process

### 7.9 Infrastructure at scale

- [ ] **Kubernetes**: Migrate to K8s (EKS/GKE/AKS) with load balancers, ingress, auto-scaling
- [ ] **CDN**: CloudFront or similar for static assets and API edge caching
- [ ] **Database sharding**: When data volume requires it
- [ ] **Global multi-region**: Deploy in multiple regions for latency and reliability
- [ ] **Cost optimization**: Per-user cost tracking, API call optimization, caching strategy

### 7.10 Final polish

- [ ] UI/UX refinements across all interfaces
- [ ] Accessibility (WCAG compliance)
- [ ] Multi-language support (i18n)
- [ ] Advanced personalization (per-user AI models/preferences)
- [ ] ML optimization (model tuning, A/B tests, cost/quality tradeoffs)

---

## Testing & production readiness

### Testing strategy

- [ ] **Unit tests**: Expand `/tests/` — target all new services (SMS, contacts, financial, habits, etc.)
- [ ] **Integration tests**: End-to-end flows (proposal → approval → execution → booking → payment)
- [ ] **Security tests**: Auth bypass, PII exposure, rate limit enforcement, injection attacks
- [ ] **Performance tests**: Load testing on critical endpoints, baseline latency metrics
- [ ] **Voice tests**: End-to-end voice call flow with Twilio test credentials
- [ ] **CI/CD**: GitHub Actions pipeline — lint, test, build, deploy to staging, then production

### Observability

- [ ] **Structured logging**: `app/core/logging.py` — JSON format, correlation IDs, no PII in logs
- [ ] **Error tracking**: Integrate Sentry (`sentry-sdk[fastapi]` in `requirements.txt`)
- [ ] **APM/Metrics**: Datadog or Prometheus for latency, throughput, error rates per endpoint
- [ ] **Uptime monitoring**: External health checks, status page
- [ ] **Alerting**: On-call runbooks, alerts on errors, latency spikes, queue depth, DB issues

### Security & compliance

- [ ] **Security audit**: External pen test, fix critical/high findings
- [ ] **Dependency scanning**: Automated CVE checks on `requirements.txt` (Dependabot or Snyk)
- [ ] **Compliance**: GDPR (already started), CCPA, document data flows and retention policies
- [ ] **Secrets management**: Use cloud secrets manager (AWS Secrets Manager, GCP Secret Manager) — no secrets in code/env files

### WhatsApp Business (production)

- [ ] Meta Business verification
- [ ] WhatsApp Business API production approval
- [ ] Production webhook URL and verified message templates
- [ ] End-to-end testing for full user journey

---

## Environment variables reference

```bash
# === CURRENT (in use) ===
DATABASE_URL          # PostgreSQL connection string
JWT_SECRET            # JWT signing key (CHANGE from default!)
PII_ENCRYPTION_KEY    # Fernet key for PII encryption
OPENAI_API_KEY        # OpenAI API key
ENV                   # "dev" | "staging" | "production"

# Stripe
STRIPE_SECRET_KEY
STRIPE_WEBHOOK_SECRET
STRIPE_PUBLISHABLE_KEY

# Amadeus (Travel)
AMADEUS_API_KEY
AMADEUS_API_SECRET

# Google (Email & Calendar)
GOOGLE_CLIENT_ID
GOOGLE_CLIENT_SECRET

# WhatsApp
WHATSAPP_TOKEN
WHATSAPP_VERIFY_TOKEN

# Twilio (Voice & SMS)
TWILIO_ACCOUNT_SID
TWILIO_AUTH_TOKEN
TWILIO_PHONE_NUMBER

# ElevenLabs (TTS)
ELEVENLABS_API_KEY
ELEVENLABS_DEFAULT_VOICE

# Deepgram (STT)
DEEPGRAM_API_KEY

# === FOUNDATION (add now) ===
REDIS_URL             # Redis for sessions, cache, rate limiting, Celery broker
MONGODB_URI           # MongoDB for document/event store
CELERY_BROKER_URL     # Celery broker (usually same as REDIS_URL)

# === PHASE 2 (add when implementing) ===
PINECONE_API_KEY      # Vector DB for semantic search (or CHROMADB_PATH for self-hosted)

# === PHASE 3+ (add when implementing) ===
OPENWEATHER_API_KEY   # Weather for wardrobe/context
NUTRITIONIX_API_KEY   # Nutrition data
TMDB_API_KEY          # Movies/shows
SPOTIFY_CLIENT_ID     # Music
SPOTIFY_CLIENT_SECRET

# === PHASE 5 (financial) ===
PLAID_CLIENT_ID       # Financial account linking
PLAID_SECRET
PLAID_ENV             # sandbox | development | production

# === OBSERVABILITY ===
SENTRY_DSN            # Error tracking
```

---

## Plan alignment summary

| Plan phase | Months | Checklist section | Status |
|------------|--------|-------------------|--------|
| **1 Foundation** | 1–6 | Phase 1 | ✅ Complete — add Redis + MongoDB + queue for production stack |
| **2 Core services** | 7–12 | Phase 2 | Voice ✅ — SMS, contacts, proactive email/calendar, file, smart home, beta prep remaining |
| **3 Lifestyle** | 13–18 | Phase 3 | Not started — wardrobe, gifts, relationships, fitness, entertainment, learning |
| **4 Advanced intelligence** | 19–24 | Phase 4 | Not started — patterns, notifications, decisions, habits, journaling, goals |
| **5 Financial & career** | 25–30 | Phase 5 | Not started — Plaid, bills, investments, crypto, career, job search |
| **6 Family & health** | 31–36 | Phase 6 | Not started — childcare, elderly care, medical, wellness |
| **7 Polish & scale** | 37–48 | Phase 7 | Not started — smart home advanced, home maintenance, travel enhanced, vehicle, safety, legal, brand, enterprise, K8s |

---

**Current priority order**:
1. **Immediate codebase fixes** (app identity, config hardening, health checks, logging)
2. **Foundation architecture** (Redis, MongoDB, Celery, platform abstraction)
3. **Phase 2 features** (SMS/contacts, proactive email, proactive calendar, file search, smart home)
4. **Testing & observability** (structured logging, Sentry, CI/CD)
5. **Phase 3+** (lifestyle → intelligence → financial → family → polish)

**Last updated**: 2026-02-06
**Strategy**: Full product, no MVP. Build on existing integrations (Amadeus, Stripe, Twilio, ElevenLabs, Deepgram, Google, OpenAI). Add Redis + MongoDB + Celery for production foundation. Then expand feature-by-feature per plan.
