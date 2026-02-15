# Executive OS Blueprint 2026 — Implementation Checklist (Scale/Compliance/Ops)

This checklist replaces the previous scope and aligns the current codebase to every requirement in BLUEPRINT.pdf. It includes explicit migration plans for all service replacements.

**Service Stack — Final Decisions (Replace Where Needed)**
- [x] Move all backend services off Render and onto AWS ECS Fargate (Gateway, Brain, Hands, Workers)
- [x] Standardize on AWS RDS PostgreSQL 16 as the single source of truth
- [x] Replace Pinecone with pgvector on Postgres (no external vector DB)
- [x] Replace MongoDB usage with Postgres tables or Redis caches
- [x] Use AWS ElastiCache Redis 7 for sessions, pub/sub, rate limits, BullMQ
- [x] Use AWS Secrets Manager for OAuth tokens and API keys
- [x] Use AWS S3 for attachments, exports, backups
- [x] Replace SerpAPI with Tavily for web search (LLM-optimized)
- [x] Adopt OpenTelemetry + Axiom for logs, traces, metrics (Sentry stays for errors; PostHog stays for product analytics)
- [ ] Keep Vercel only for the public marketing site (backend runs on AWS)
- [ ] Use Cloudflare for DNS + WAF; GoDaddy remains registrar
- [ ] Primary channels: WhatsApp Cloud API + Apple Messages for Business
- [x] LLM provider: OpenAI GPT-4o + GPT-4o-mini + text-embedding-3-small
- [x] Email provider: AWS SES (replace SendGrid for compliance and ops consolidation)
- [ ] Auth provider: Clerk (phone-based auth + JWT)
- [ ] Workflow engine: Temporal (self-hosted on ECS)
- [ ] Job queue: BullMQ (Redis-backed)
- [x] Keep Stripe for billing

---

# Migration Plans (Explicit Cutovers)

**M1: Render → AWS ECS Fargate (Backend Services)**
- [x] Define target service layout: gateway, brain, hands, workers
- [x] Create AWS VPC (public/private subnets, NAT, ALB)
- [x] Provision ECS cluster and task definitions for each plane
- [x] Configure environment variables and secrets in Secrets Manager
- [x] Deploy each plane to staging ECS and validate health checks
- [x] Configure Cloudflare DNS for new ALB endpoint
- [x] Shadow traffic from Render → ECS (staging) for validation
- [x] Cutover production traffic to ECS (blue/green or canary)
- [x] Decommission Render services after stability window

**M2: SQLite → Postgres 16 (RDS)**
- [x] Create RDS Postgres 16 (Multi-AZ) and apply pgvector/pgcrypto/pg_trgm
- [x] Implement blueprint schema + enums + RLS policies
- [x] Write data migration scripts from SQLite to Postgres
- [x] Update ORM/database config to Postgres DSN
- [x] Validate data integrity and query performance
- [x] Cutover production to Postgres

**M3: Pinecone → pgvector**
- [x] Add pgvector extension and HNSW indexes to Postgres
- [x] Implement vector queries in memory engine + semantic cache
- [x] Backfill embeddings from Pinecone into Postgres
- Migration note: N/A by blueprint-aligned cutover decision (no legacy Pinecone data kept). New embeddings are generated directly into Postgres pgvector.
- [x] Switch vector queries to pgvector
- [x] Remove Pinecone client configuration and secrets

**M4: MongoDB → Postgres/Redis**
- [x] Inventory MongoDB collections and usage
- [x] Model equivalent tables in Postgres or Redis caches
- [x] Migrate historical data into Postgres
- Migration note: N/A by blueprint-aligned cutover decision (no legacy Mongo historical import required for current environment).
- [x] Remove MongoDB client dependency from codebase

**M5: SerpAPI → Tavily**
- [x] Implement Tavily connector
- [x] Swap discovery/search calls to Tavily
- [x] Verify search quality and latency
- [x] Remove SerpAPI config and secrets

**M6: SendGrid → AWS SES**
- [x] Verify domain with SES + configure DKIM/SPF
- [x] Implement SES email sender wrapper
- [x] Switch all outbound emails to SES
- [x] Decommission SendGrid API keys

**M7: Observability → OpenTelemetry + Axiom**
- [x] Add OTEL instrumentation across all planes
- [x] Configure Axiom datasets and ingest pipeline
- [x] Validate dashboards for latency, error rate, tier mix
  - Validation script: `python3 scripts/validate_axiom_m7.py --stage prod --lookback 10m`
- Migration note: requires an Axiom API token with query/read scope.
- [x] Keep Sentry for error reporting
- [x] Keep PostHog for product analytics

---

# Core Blueprint Implementation

**Architecture — 3-Plane Model**
- [ ] Split monolith into three FastAPI services: Gateway, Brain, Hands
- [ ] Define internal REST APIs between planes
- [ ] Add Redis pub/sub for Hands→Brain and Hands→Gateway events
- [ ] Add SSE streaming from Gateway to Brain
- [ ] Enforce per-plane timeouts (Gateway→Brain 30s, Brain→Hands 10s tool, 5m workflows)
- [ ] Add separate scaling policies and health checks per plane
- [ ] Enforce failure isolation: Gateway never crashes on Brain/Hands failures

**Data Layer — Postgres + pgvector + RLS**
- [x] Implement blueprint schema: users, conversations, messages, runs, memories, contacts, tool_executions, approvals, connected_accounts, proactive_triggers, semantic_cache
- [x] Enable pgcrypto, pgvector, pg_trgm extensions
- [x] Create required enums: channel_type, message_direction, run_state, memory_type, sensitivity_level, approval_status, tool_exec_status
- [x] Enforce RLS on every table
- [x] Set `app.user_id` session variable on every DB connection
- [x] Add HNSW indexes for memories and semantic_cache embeddings
- [x] Add all required indexes and dedup constraints (messages.channel_msg_id, tool idempotency)

**Contracts — Pydantic v2 Strict**
- [ ] Create core contracts: CapabilityEnvelope, InboundMessage, OutboundMessage
- [ ] Create intent/tier contracts: IntentClassification, TierRoutingConfig
- [ ] Create tool contracts: ToolSpec, ToolCall, ToolResult
- [ ] Create memory contracts: MemoryEntry, MemoryQuery, MemoryRetrievalResult
- [ ] Generate OpenAPI and tool schemas from these contracts

**Gateway Plane**
- [ ] Webhook receiver for WhatsApp with HMAC verification
- [ ] Webhook receiver for Apple Messages for Business with cert validation
- [ ] Deduplicate inbound messages by channel_msg_id
- [ ] Send typing indicator within 500ms
- [ ] Queue inbound messages for async processing (ack within 5s)
- [ ] ChannelAdapter for WhatsApp and iMessage (message splitting, button limits, templates)
- [ ] AuthService using Clerk
- [ ] SessionManager in Redis (conversation state + entity resolution)
- [ ] RateLimiter per-user and per-IP with gradual degradation
- [ ] Abuse detection for high-volume users

**Brain Plane — Tiered Reasoning**
- [ ] TierRouter (T0-T3) with regex prefilter + GPT-4o-mini classifier
- [ ] ContextCompiler with tier budgets and token accounting
- [ ] Planner T0: templates, T1: single-shot, T2: ReAct (max 5), T3: plan+critic+checkpoint
- [ ] LLMRouter with model routing, retries, streaming, and cost tracking
- [ ] PromptRegistry with versions and feature flags
- [ ] GuardrailValidators for structured output and content policies
- [ ] Degrade to cached responses + read-only mode when Brain fails

**LLM Integration**
- [ ] Implement model routing rules per blueprint
- [ ] Implement semantic cache with TTL per tier
- [ ] Check cache before any LLM call (except intent classification)
- [ ] Build system prompt from persona + preferences + memories + tools

**Memory Engine**
- [ ] Retrieval pipeline: structured lookup + contacts + vector search + recency decay
- [ ] Write pipeline after each run with dedup/versioning
- [ ] Nightly consolidation and confidence decay

**Autonomy Engine (ACE)**
- [ ] Implement Beta distributions per action class
- [ ] Determine auto-exec vs approval based on autonomy score
- [ ] WhatsApp-native approval flow with buttons and expiry
- [ ] Update autonomy profile after approve/deny events

**Hands Plane — Tool Execution**
- [ ] ToolExecutor with idempotency and tool_executions logging
- [ ] OAuthVault with AES-256-GCM encryption and auto-refresh
- [ ] Delta-sync workers for connected accounts (calendar 2m, email 5m, contacts daily)
- [ ] JobScheduler with BullMQ and dead-letter handling
- [ ] ProactiveTriggerEngine with value vs interruption scoring

**Workflow Orchestration**
- [ ] Use BullMQ for T2 ReAct loops and background jobs
- [ ] Use Temporal for T3 workflows (plan, critic, execute, checkpoints)

**Connectors (Priority Order)**
- [ ] Google Calendar connector: list/create/update/delete/find_free_slots
- [ ] Google Gmail connector: list/search/get/draft/send/label
- [ ] Google Contacts connector: list/search/get
- [ ] Tavily web search connector
- [ ] Microsoft Calendar connector
- [ ] Microsoft Outlook connector
- [ ] Slack connector (send_message, list_channels, summaries)
- [ ] Plaid connector (balances, transactions, spending summary)

**Proactive Intelligence**
- [ ] Morning briefing trigger
- [ ] Pre-meeting prep trigger
- [ ] Follow-up nudge trigger
- [ ] Travel alert trigger
- [ ] Conflict detection trigger
- [ ] Deadline warning trigger
- [ ] Weekly digest trigger
- [ ] Implement value > interruption scoring and daily caps

**Conversation Design System**
- [ ] Implement Exec persona system prompt
- [ ] Implement 20 core interaction patterns
- [ ] Enforce response length and WhatsApp limits

**Context Compiler**
- [ ] Tier-specific token budgets (T1/T2/T3)
- [ ] History compression + rolling summary
- [ ] Tool schema injection by intent

**Security**
- [ ] Input sanitization (prompt injection patterns)
- [ ] Output validation (structured output + content policy)
- [ ] OpenAI moderation on user input and LLM output
- [ ] OAuth token encryption with rotation
- [ ] Idempotency guarantees in tool execution
- [ ] Row-level security enforced for all queries
- [ ] Webhook signature/cert verification
- [ ] PII redaction in logs

**Observability & Monitoring**
- [ ] OTEL traces and metrics across planes
- [ ] Dashboard for latency, error rate, tier mix, cache hit rate
- [ ] Approval response time tracking
- [ ] Proactive message helpfulness tracking
- [ ] Cost per interaction and budget alarms

**Testing Strategy**
- [ ] Unit tests for tier router, context compiler, memory, ACE
- [ ] Integration tests for Gateway→Brain→Hands flows
- [ ] Contract tests for schemas and tool definitions
- [ ] Connector tests with record/replay
- [ ] Agentic eval suite (50 golden scenarios)
- [ ] Red-team eval harness and weekly runs
- [ ] Load tests (100, 1K, 10K concurrent)
- [ ] Chaos tests for Redis/DB/LLM failures

**Deployment Architecture (AWS)**
- [ ] VPC with public/private subnets and NAT
- [ ] ECS cluster with services: gateway, brain, hands, workers
- [ ] RDS Postgres 16 with Multi-AZ
- [ ] ElastiCache Redis 7
- [x] ALB with TLS termination
- [ ] S3 for attachments/backups
- [ ] Secrets Manager for keys
- [x] CI: GitHub Actions runs `pytest` on PRs + main (Python 3.12)
- [ ] CD (staging): GitHub Actions builds + pushes ECR images and forces ECS redeploy (OIDC role)
- [ ] CD (production): GitHub Actions manual deploy with environment protection (OIDC role)
- [ ] Rollback: documented procedure + one-click “redeploy previous image” action
- [ ] Evals: nightly golden-suite run + regressions gate merges

**Onboarding Flow (5 Minutes)**
- [ ] Step 1: ask name
- [ ] Step 2: connect Google Calendar
- [ ] Step 3: show tomorrow’s calendar + offer prep
- [ ] Step 4: ask work hours
- [ ] Step 5: ask response verbosity
- [ ] Step 6: connect Gmail
- [ ] Step 7: scan last 7 days for unanswered emails

**WhatsApp Templates (Submit to Meta)**
- [ ] exec_morning_brief
- [ ] exec_meeting_prep
- [ ] exec_followup
- [ ] exec_travel
- [ ] exec_conflict
- [ ] exec_approval
- [ ] exec_complete
- [ ] exec_weekly

**Error Codes & Taxonomy**
- [ ] Implement E1001–E6001 errors and map to user-facing messages

**Performance Targets (Must Be Met)**
- [ ] p95 WhatsApp e2e latency < 3s
- [ ] Blended cost < $0.03 per interaction
- [ ] Semantic cache hit rate > 20%
- [ ] Tier distribution within target bands

**Non-Blueprint Features (Keep Isolated)**
- [ ] Keep existing non-blueprint services behind feature flags so they do not block 3-plane build
