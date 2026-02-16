# Executive OS v5 — Master Implementation Checklist

Single source of truth: `EXECUTIVE_BLUEPRINT.pdf`

This checklist is the full end-to-end plan to take the current codebase from “where it is now” to the final hyper-aware agent described in the v5 blueprint, in the exact build order specified in **Section 38**.

Legend
- `[x]` Done (verified in code and/or deployed)
- `[ ]` Not done / blocked

Migration rules (must follow)
- Preserve already-working preserved components unchanged unless v5 explicitly requires changes (per user instructions).
- Do not jump ahead: build order is Phase 1 → Phase 4 (Section 38).

Feature Flag Rollout Reminders (Appendix A)
- [x] Phase 1 exit gate: set `FEATURE_MULTI_PROVIDER_LLM=true` (router v1+v2 done and all LLM calls migrated)
- [x] Phase 1 exit gate: set `FEATURE_VOICE_INPUT=true` (voice-in pipeline + `/api/v1/voice/transcribe` validated)
- [x] Phase 1 exit gate: set `FEATURE_IMAGE_PROCESSING=true` (image pipeline validated end-to-end)
- [x] Phase 1 exit gate: keep `FEATURE_DOCUMENT_PROCESSING=false` until Phase 4 document ingestion/generation is complete
- [x] Phase 2 exit gate: set `FEATURE_PRIVILEGE_ISOLATION=true` (provenance + capability-token enforcement validated)
- [ ] Phase 3 exit gate: set `FEATURE_CONSOLIDATION_ENABLED=true` (nightly consolidation job validated)
- [ ] Phase 3 exit gate: set `FEATURE_SELF_REVIEW_ENABLED=true` (weekly self-review job validated)
- [ ] Phase 4 exit gate: set `FEATURE_DOCUMENT_PROCESSING=true` (document ingestion + document generation validated)

---

## Current State Snapshot (Verified)
- [x] 3-plane entrypoints exist: Gateway, Brain, Hands (Section 1, Section 6)
- [x] Internal Brain/Hands APIs exist and Gateway supports SSE streaming proxying (Section 1, Section 5, Section 7)
- [x] Staging ECS deploy is working (Phase 1 M1 S1-2)
- [x] Production ECS deploy listener conflict mitigated in IaC (`ALB_CREATE_HTTPS_LISTENER` gate + existing-listener-safe path)
- [x] OpenTelemetry (OTEL) wiring exists in app and Axiom ingest is configured (Section 33)
- [x] Multi-Provider LLM Router implemented; direct OpenAI imports migrated behind router proxy (Section 9)
- [x] v5 database schema migration authored (19 tables + enums + enhancement columns + RLS policy baseline) (Section 3)

---

# PHASE 1 — FOUNDATION (Month 1–3) (Section 38)

## M1 S1–2: AWS Foundation + Monitoring + LLM Router v1 (Section 38, Section 2, Section 9, Section 33, Section 35)
- [x] Staging: VPC + ECS + RDS Postgres 16 + Redis 7 deployed
- [x] Production: resolve HTTPS listener conflict path in IaC (`ALB_CREATE_HTTPS_LISTENER` + existing listener compatibility)
- [x] OpenTelemetry (OTEL) tracing enabled per plane (`OTEL_SERVICE_NAME`) and exported to Axiom
- [x] Create/confirm S3 buckets per Appendix A: attachments, knowledge snapshots, voice, documents
- [x] Add v5 feature flags from Appendix A to both staging + prod config (user-confirmed in AWS Secrets Manager)
- [x] CI: GitHub Actions runs unit tests + type checks on PRs and on `main` (Section 38 M1 S1-2, Section 34)
- [x] CD (staging): GitHub Actions builds + pushes plane images to ECR and deploys to ECS (OIDC) (Section 38 M1 S1-2, Section 35)
- [x] CD (prod): GitHub Actions manual deploy with environment protection (OIDC) (Section 38 M1 S1-2, Section 35)
- [x] Implement `LLMRouter` v1 unified interface (OpenAI primary) (Section 9.1–9.3)
- [x] Implement provider health probe + Redis-cached health state (30s) (Section 9.3)
- [x] Add `/internal/llm/health` and `/internal/llm/route-test` endpoints (Section 5.5)
- [x] Implement cost/latency tracking per LLM call (Section 9, Section 33, Section 37)
- [x] Replace ALL direct OpenAI calls with the router wrapper (Section 9 “Every LLM call goes through this router”)

## M1 S3–4: Gateway Plane v1 + Progress Streaming (Section 38, Section 5.1, Section 21, Section 22)
- [x] WhatsApp webhook receiver exists with signature verification + dedup + async ACK
- [x] Implement Gateway `/api/v1/message` (web/test) endpoint (Section 5.1)
- [x] Implement `/api/v1/stream/{run_id}` SSE progress endpoint (Section 5.1)
- [x] Implement “typing indicator”/processing feedback strategy across channels (Section 38 M1 S3-4)
- [x] Implement Redis Session Manager for cross-channel sessions (Section 1.2, Section 6, Section 28)
- [x] Implement per-channel adapters for WhatsApp templates/buttons/splitting (Section 22, Appendix C)
- [x] Implement WhatsApp Template Library (Appendix C) and store template IDs in runtime config registry
- [x] Validate template send flow with approved `hello_world` template path (Appendix C, Section 22)
- [x] Implement Clerk auth integration in Gateway (Section 2, Appendix A)
- [x] Implement rate limiting targets (per-user and per-IP) + graceful fallback strategy (Section 32)

## M1–2 S5–8: Google Calendar Connector (Section 38, Section 23, Section 31)
- [x] Implement OAuth token storage in `oauth_tokens` with AES-256-GCM + rotation (Section 23, Section 32)
- [x] Implement Calendar read/write tool specs + executor (list/create/update/delete/find_free_slots) (Section 38)
- [x] Implement delta-sync cursoring in `sync_cursors` and async worker jobs (Celery equivalent in current stack) (Section 23)
- [x] Implement Calendar tool tests (hands executor coverage added) (Section 34)

## M2 S9–10: Brain Plane v1 + LLM Router v2 (Section 38, Section 7, Section 8, Section 10)
- [x] Implement Intent Classification contract + endpoint wiring using LLM Router task types (Section 4.2, Section 7)
- [x] Implement Tier Router v1 (Tier 0 + Tier 1 baseline with heuristic escalation support) (Section 7.1)
- [x] Implement Context Compiler v1 with Tier-1 token budget and knowledge file injection plumbing (Section 10)
- [x] Implement LLM Router v2: add Anthropic failover + routing table (Section 38 M2 S9-10, Section 9.2, Appendix H)
- [x] Implement degraded modes per Appendix H baseline (Tier0 deterministic fallback + provider failover fallback path)

## M2 S11–12: Gmail Connector + Voice-In Pipeline (Section 38, Section 21.1, Section 23, Section 32)
- [x] Implement Gmail tools (list/search/get/draft) + delta-sync hooks (Section 38)
- [x] Implement Voice-In pipeline for WhatsApp voice notes using Whisper (Section 21.1, Appendix A feature flags)
- [x] Persist voice metadata on `messages`: `input_modality`, `original_media_url`, `transcription_confidence` (Section 3.2)
- [x] Implement `/api/v1/voice/transcribe` endpoint (Section 5.1)
- [x] Implement content provenance tagging for voice transcriptions (Section 32, Section 3.2)

## M3 S13–14: Image Processing Pipeline (Section 38, Section 21.2)
- [x] Implement image download + GPT-4o vision extraction path in multimodal pipeline (Section 21.2)
- [x] Persist extracted entities + modality metadata in `messages.extracted_entities` (Section 3.2)
- [x] Add image classification/extraction prompt baseline for Section 21.2

## M3 S15–16: ACE Engine v1 (Section 12, Section 38)
- [x] Implement ACE action classification + autonomy score baseline (Section 12)
- [x] Implement approval flow path in WhatsApp button/template channel adapter (Section 12, Section 22, Appendix C)
- [x] Implement `side_effects` ledger write path in schema + executor plumbing (Section 3.2 table 10)

## Behavioral Intelligence (Parallel) — Phase 1 (Section 38, Section 8, Section 13–17, Appendix D/E/F)
- [x] Implement full v5 DB schema: all 19 tables + enums + indexes + RLS baseline migration (Section 3.1–3.4)
- [x] Implement v5 schema enhancements to existing tables (messages/runs/tool_executions/proactive_triggers/accounts) (Section 3.2)
- [x] Implement Knowledge File templates + DB versioning in `knowledge_files` (`USER.md`, `SOUL.md`, `IDENTITY.md`, `AGENTS.md`, `MEMORY.md`, `HEARTBEAT.md`, `TOOLS.md`, `TEAM.md`, `WORKFLOWS.md`) (Section 8, Section 14)
- [x] Implement knowledge file hot cache in Redis for IDENTITY/SOUL/AGENTS (Section 8)
- [x] Implement Knowledge completeness scoring + tracking (Appendix D, Section 33)
- [x] Implement Profiling Conversation Engine v1 (first 4 dimensions) + extraction pipeline (Section 13, Appendix E)
- [x] Implement Context Injection Engine: dynamic prompt assembly replaces static system prompt (Section 10, Section 38)
- [x] Implement correction-to-rule feedback ingestion skeleton (feedback_signals + behavioral_rules hooks) (Section 17)
- [x] Implement onboarding flow as first profiling session (Section 36, Section 38 M3 S17-18)

## MCP Readiness Overlay (Months 1–7) (Integration Spec Section 17.1 + Deployment Plan)
- [x] `ToolSpec` includes MCP fields from day one (`is_mcp`, `mcp_server_id`, `capability_scope`) (Blueprint Section 4.3, Section A.1)
- [x] `ToolCall` includes `input_provenance` and `required_capabilities` (Blueprint Section 4.3, Section A.1)
- [x] `ToolResult` includes `compensating_action` (Blueprint Section 4.3, Section A.1)
- [x] `tool_executions` schema path includes `is_mcp` + `mcp_server_id` + `input_provenance` (Blueprint Section 3.2, Section A.2)
- [x] Dynamic shared `ToolRegistry` implemented (native tools register now; MCP tools register in Month 8) (Section A.3)
- [x] Tool execution path has explicit `if spec.is_mcp` branching stub (`NotImplementedError`) (Section A.4)
- [x] Context compiler tool schemas are registry-driven (`compile_tool_schemas`) (Section A.5)
- [x] Privilege isolation policy includes MCP provenance restrictions (`mcp_result`) (Blueprint Section 32, Section B)
- [x] MCP readiness test added (`tests/test_mcp_readiness.py`) validating registry injection, MCP branch routing, and privilege handling (Section C.2)
- [x] Read all MCP specs in full: `MCP_Integration_Specification.docx`, `MCP_Server_Deployment_Plan.docx`, `MCP_Wave5_6_Expansion.docx` (Section C.3)
- [ ] Deploy DB enum migration `q1w2e3r4t5y6_add_mcp_result_provenance_enum.py` to staging + prod and verify values live (Section B/C)

---

# PHASE 2 — INTELLIGENCE (Month 4–6) (Section 38)

## M4 S1–2: Tier 2 ReAct + Self-Reflection (Section 7.1, Section 38)
- [x] Replace fixed T2 max-5 loop with adaptive iteration limit + hard cap 10 (Section 7.1)
- [x] Add semantic validation step before tool execution in T2 (Section 7.1)
- [x] Add self-reflection loop after T2+ completion (Section 7.1, Section 38)

## M4 S3–4: Email Sending (Section 38, Section 12, Section 32)
- [x] Implement email send tool with approval flow + recipient verification + draft-review-send cycle (Section 38)
- [x] Implement output validation pass for outbound side-effects (Section 32)

## M4–5 S5–8: Microsoft 365 Connector (Section 38, Section 23)
- [x] Implement Microsoft Calendar + Outlook + Contacts tools and delta-sync
- [x] Add connector status endpoint `/api/v1/connectors` and OAuth start `/api/v1/connectors/{provider}/auth` (Section 5.2)

## M5 S9–10: Proactive Intelligence + Privilege Isolation (Section 20, Section 32, Section 38)
- [x] Implement Proactive Trigger Engine with scheduling + daily caps + quiet hours (Section 19–20, Section 3.2 accounts)
- [x] Implement morning briefing / pre-meeting prep / follow-up nudges (Section 38)
- [x] Implement content provenance tagging for every context chunk in every LLM call (Section 32)
- [x] Implement privilege isolation policy enforcement by provenance (Section 32.1)
- [x] Implement capability tokens at run creation and enforce on every tool call (Section 32)
- [x] Implement Pydantic structured output validation + retries for every LLM call (Section 32)
- [x] Add prompt injection fuzzing to CI (Section 32 table, Section 34)

## M5–6 S11–14: Tavily + Tier 3 Planner + Saga Compensation (Section 25, Section 26, Section 31, Section 38)
- [x] Implement Tavily web search tool (Section 2, Section 38)
- [x] Implement Tier 3 hierarchical decomposition with subtask tiering (Section 7.1)
- [x] Implement Tier 3 MCTS planning path for high-stakes tasks (num_rollouts, best-plan selection) (Section 7.1)
- [x] Implement Tier 3 critic review + checkpoints (Section 7.1)
- [x] Implement saga compensation using `tool_executions.compensating_action` + `side_effects` ledger (Section 3.2, Section 7.1)
- [x] Implement Tier 3 re-plan on failure (max 3 replans) with subtask-level retry rules (Section 7.1)
- [x] Run Tier 3 orchestration on Temporal (Section 2, Section 31, Section 38)

## M6 S15–16: Emotion Classifier + Tone Adaptation (Section 29, Section 3.2)
- [x] Implement inbound emotion detection pipeline (text + voice metadata) (Section 29)
- [x] Persist `messages.emotion_detected` and apply `accounts.emotion_sensitivity` to response style (Section 3.2, Section 29)

## M6 S17–18: TEAM.md Population + Delegation v1 (Section 24, Section 8.8, Section 38)
- [x] Populate TEAM.md from contacts + profiling + interaction graph (Section 38)
- [x] Implement delegations table CRUD endpoints `/api/v1/delegations` (Section 5.4)
- [x] Implement basic delegation flow + reminders + HEARTBEAT delegation tracker updates (Section 24, Section 19)

## Behavioral Intelligence (Parallel) — Phase 2 (Section 11, Section 12, Section 18)
- [x] Implement Memory dual-path: episodic memory + knowledge file updates + knowledge graph edge writes (Section 11)
- [x] Implement implicit preference learning from approvals/edits/outcomes (Section 18)
- [x] Upgrade ACE: AGENTS.md overrides Beta priors; dual memory path integration (Section 38 M6 S11-12)
- [x] Implement agentic eval suite (50 golden scenarios) + personalization scoring (Section 34)

---

# PHASE 3 — EXPANSION (Month 7–9) (Section 38)

## M7 S1–4: Apple Messages for Business (Section 22, Section 38)
- [ ] Implement iMessage webhook receiver + cert chain validation (Section 5.1, Section 32)
- [ ] Implement iMessage-native UX constraints in Channel Adapter (Section 22)

## M7 S5–6: Slack Connector + Workflows v1 (Section 26, Section 38)
- [ ] Implement Slack connector read/send + channel summaries (Section 23)
- [ ] Implement user-defined workflow engine: NL → workflow definition stored in WORKFLOWS.md + workflows table (Section 26, Section 8.9)
- [ ] Implement workflows CRUD endpoints `/api/v1/workflows` + dry-run `/api/v1/workflows/{id}/test` (Section 5.4)

## M7–8 S7–10: Advanced Scheduling (Section 24.2, Section 38)
- [ ] Implement multi-person availability and timezone intelligence using TEAM.md preferences (Section 24.2)
- [ ] Implement conflict resolution, buffers, ranking by preference alignment (Section 24.2)

## M8 S11–12: MCP Foundation Build (Integration Spec Section 3–5, 14, 15)
- [ ] Create DB tables `mcp_servers` + `mcp_user_servers` with RLS + indexes (Integration Spec Section 14)
- [ ] Implement MCP Pydantic contracts (`MCPServerConfig`, `MCPToolSchema`, `MCPToolResult`, `MCPContentBlock`, `MCPServerHealth`) (Integration Spec Section 15)
- [ ] Build transport interfaces + implementations: streamable HTTP + stdio (Integration Spec Section 5.1–5.3)
- [ ] Implement `MCPClientHub` singleton with connection pool + initialize handshake + `tools/list` discovery (Integration Spec Section 3)
- [ ] Implement `MCPServerRegistry` CRUD + capability probe + manifest validation (Integration Spec Section 4)
- [ ] Add mock MCP servers (echo/error/slow) for deterministic integration tests (Integration Spec Section 13.2)

## M8 S13–14: MCP Normalization + Security + Runtime Wiring (Integration Spec Section 6, 7, 10, 11, 16)
- [ ] Implement `normalize_mcp_tool()` (MCP schema → `ToolSpec`) and `normalize_mcp_result()` (MCP result → `ToolResult`) (Integration Spec Section 6.1–6.2)
- [ ] Replace MCP stub in tool executor with live invocation bridge (Integration Spec Section 6.3)
- [ ] Implement MCP sandbox controls (container isolation for stdio, network allowlist, scoped tool lists) (Integration Spec Section 7)
- [ ] Enforce MCP provenance tagging (`ContentProvenance.MCP_RESULT`) and privilege isolation on all MCP-origin tool calls (Integration Spec Section 7.4)
- [ ] Implement MCP capability tokens + sampling validation path (Integration Spec Section 7.5)
- [ ] Implement MCP cost tracking + per-server daily budgets + per-server rate limits (Redis) (Integration Spec Section 10)
- [ ] Add MCP API surface `/api/v1/mcp/*` (server CRUD, connect/disconnect, tool discovery/execute) (Integration Spec Section 16)
- [ ] Implement MCP health monitor loop + reconnect/backoff logic (Integration Spec Section 5.4, Section 9)
- [ ] Add MCP OTEL spans/attributes + logs for server_id/tool_name/latency/cost/error (Integration Spec Section 11.2)
- [ ] Implement semantic caching + prompt caching + model cascade optimization (Section 38)

## M9 S15–16: Plaid Financial Connector + Research Engine v1 (Section 25, Section 38)
- [ ] Implement Plaid connector with high-risk approval flow (Section 38, Section 12)
- [ ] Implement Research Engine (Temporal workflow) + research_jobs table usage + scheduled delivery (Section 25)
- [ ] Implement research CRUD endpoints `/api/v1/research` (Section 5.4)

## M9 S15–18: MCP Advanced Features + Wave 1 Server Rollout (Integration Spec Section 8 + Deployment Plan Section 4)
- [ ] Complete MCP resources injection pipeline (`resources/list` + context injection) (Integration Spec Section 8.1)
- [ ] Complete MCP resource subscription handling (`resources/subscribe`) (Integration Spec Section 8.2)
- [ ] Complete MCP prompt merge pipeline (`prompts/list` + layered prompt assembly) (Integration Spec Section 8.3)
- [ ] Complete SSE transport support for legacy MCP servers (Integration Spec Section 5)
- [ ] Deploy Wave 1 servers 1–3: Google Calendar MCP, Google Drive MCP, Gmail MCP (Deployment Plan Section 4.1–4.3)
- [ ] Deploy Wave 1 servers 4–8: Notion, Todoist, Brave Search, GitHub, Apple Reminders (Deployment Plan Section 4.4–4.8)
- [ ] Start Apple Reminders custom server build in parallel with Wave 1 rollout (Deployment Plan Section 4.8)
- [ ] Run 12-step server deployment checklist for every Wave 1 server (Deployment Plan Section 15)

## M9 S17–18: Red-Team Eval Suite (Section 34, Section 38)
- [ ] Implement prompt injection red-team scenarios for email/calendar/web/MCP
- [ ] Implement wrong-recipient + data exfil + privilege escalation test harness

## Behavioral Intelligence (Parallel) — Phase 3 (Section 15, Section 16, Section 19)
- [ ] Implement nightly consolidation job via BullMQ with contradiction detection across knowledge files (Section 15)
- [ ] Implement weekly self-review + gap detection + question generation (Section 16)
- [ ] Implement HEARTBEAT system with goal tracking, milestones, delegation tracking (Section 19)
- [ ] Implement Bones layer: repository scanning + SKILL.md generation + TOOLS.md mapping + MCP catalog (Section 1.3, Section 38 M9 S11-14)
- [ ] Implement Muscles layer: model inventory, cost routing, circuit breakers, provider health monitoring (Section 1.3, Section 9, Section 37)
- [ ] Implement monthly embedding re-embed audit + backfill job (text-embedding-3-small) (Section 2)

---

# PHASE 4 — POLISH & SCALE (Month 10–12) (Section 38)

## M10 S1–4: Document Generation + Voice Output + Connectors (Section 27, Section 21.3, Section 38)
- [ ] Implement document generation engine (Markdown → PDF/DOCX) with templates and S3 output (Section 27)
- [ ] Implement document ingestion pipeline (PDF/DOCX/XLSX → Unstructured.io → chunks + entities + action items → ProcessedMessage) (Section 1.4, Section 21, Section 2)
- [ ] Implement Voice TTS output pipeline (OpenAI TTS-1 and optional ElevenLabs) (Section 21.3, Appendix A)
- [ ] Implement travel connectors, smart home, Notion/Google Docs search (Section 38)

## M10: MCP Wave 2 — Communication & Collaboration (Deployment Plan Section 5, Section 14.1)
- [ ] Deploy Wave 2 servers: Slack, Outlook, Teams, Linear, Asana, Discord, WhatsApp Business MCP
- [ ] Complete onboarding UX block for app ecosystem detection + OAuth consolidation + confirmation flow (Deployment Plan Section 10)
- [ ] Run 12-step deployment checklist for every Wave 2 server (Deployment Plan Section 15)

## M11: MCP Wave 3 — Business Intelligence & Finance (Deployment Plan Section 6, Section 14.1)
- [ ] Deploy Wave 3 servers: Stripe, QuickBooks, HubSpot, Salesforce, Google Sheets, Airtable, Jira, Sentry
- [ ] Enforce high-risk approval flows for financial write operations (Stripe/QuickBooks) before execution
- [ ] Load test MCP fleet at 100 concurrent calls + failover simulation while Wave 3 is active
- [ ] Run 12-step deployment checklist for every Wave 3 server (Deployment Plan Section 15)

## M12: MCP Wave 4 — Lifestyle & Specialized + Launch (Deployment Plan Section 7, Section 14.1)
- [ ] Deploy Wave 4 servers: Google Maps, Uber/Lyft, OpenTable/Resy, HomeAssistant, Spotify, Evernote, Dropbox
- [ ] Complete full-wave red-team across all launch servers (Waves 1–4, total 30) including injection + exfiltration scenarios
- [ ] Run 12-step deployment checklist for every Wave 4 server (Deployment Plan Section 15)
- [ ] Submit partner applications during Month 12: Zoom Marketplace, Instacart Connect, Canva Connect, Booking.com Demand API (Deployment Plan + Wave 5–6 Section 8/12)
- [ ] Prepare fallback server choices if partner approvals are denied (Zoom PAT, Amazon Fresh/DoorDash, Figma/design fallback, Booking affiliate fallback)

## M10–11 S5–8: Advanced Proactive + Cross-Channel Continuity (Section 28, Section 38)
- [ ] Implement HEARTBEAT-driven proactive triggers + research delivery (Section 19–20)
- [ ] Implement cross-channel context continuity: unified session, channel preference learning (Section 28)

## M11 S9–12: Performance + LLM Router Optimization (Section 33, Section 37, Section 38)
- [ ] Meet p95 latency target (<3s) and blended cost target (<$0.035/interaction) (Section 33, Section 37)
- [ ] Implement latency-based and cost-based routing, batch routing for bulk tasks (Section 38)
- [ ] Improve cache hit rate >20% with precomputed context blocks (Section 33, Section 38)

## M11–12 S13–14: Load Testing + Graceful Degradation (Section 34, Section 38)
- [ ] Load test at 10K concurrent; ensure multi-provider failover under load (Section 38)
- [ ] Implement graceful degradation modes for outages (Section 9.3, Appendix H)

## M12 S15–18: Security Hardening + Launch (Section 32, Section 35, Section 38)
- [ ] Pentest + OWASP LLM Top 10 coverage + privilege isolation audit (Section 32, Section 38)
- [ ] Production launch readiness: runbooks, incident playbooks, on-call, documentation (Section 38)

## Behavioral Intelligence (Parallel) — Phase 4 (Section 28, Section 33)
- [ ] Implement “What do you know about me?” command + knowledge file viewer/editor via chat (Section 38)
- [ ] Implement knowledge graph query interface + `/api/v1/knowledge/graph` (Section 5.3)
- [ ] Implement personalization eval dashboards (profiling coverage, knowledge accuracy, correction frequency, satisfaction) (Section 38)
- [ ] Implement opt-in cross-user anonymized insights (Section 38)
- [ ] Implement A/B testing framework + experiments endpoints `/internal/experiments` (Section 5.5, Section 38)
- [ ] Implement GDPR/CCPA data export endpoint `/api/v1/export` (Section 5.4, Section 38)
- [ ] Ship documentation bundle: knowledge format spec, question bank, signal catalog, MCP guide, delegation protocol, research API guide (Section 38)

---

# PHASE 5 — MCP POST-LAUNCH EXPANSION (Month 13–15) (Wave 5–6 Expansion)

## M13–14: Wave 5 (5 Servers) (Wave 5–6 Section 4, Section 6.1–6.5)
- [ ] Build + deploy Duffel MCP (custom) with approval-gated booking flow (`create_order` risk critical) and WhatsApp list-message offer selection
- [ ] Build + deploy Zoom MCP (custom) with User-Level OAuth and meeting transcript retrieval support
- [ ] Build + deploy Calendly MCP (custom) with duplicate-event prevention against Google Calendar events
- [ ] Build + deploy Plaid MCP (custom) + Plaid Link widget page hosting (S3/CloudFront or equivalent)
- [ ] Build + deploy Crunchbase MCP (custom) and wire to research engine ingestion
- [ ] Validate Wave 5 extras: Duffel sandbox e2e booking, Plaid network audit (no external LLM PII egress), contextual discovery triggers

## M15: Wave 6 (5 Servers) (Wave 5–6 Section 5, Section 6.6–6.10)
- [ ] Build + deploy Booking.com MCP (custom) with explicit booking approval details (hotel/room/dates/price/cancellation policy)
- [ ] Deploy + harden DocuSign MCP (existing community server fork) with envelope approval gate
- [ ] Build + deploy Canva MCP (custom) with curated template-based flows (no unsupported free-form flows)
- [ ] Build + deploy Instacart MCP (custom) OR approved fallback (Amazon Fresh/DoorDash) with checkout approval gate
- [ ] Deploy + harden Tesla MCP (existing server fork) with physical-security approvals + strict rate limits + optional geo-fencing
- [ ] Validate Wave 6 extras: Tesla physical operation tests in staging, Instacart checkout approval details, DocuSign recipient/document confirmation

## M15: Full Fleet Validation (Waves 1–6, 40 Servers)
- [ ] Verify all 40 servers pass health checks simultaneously
- [ ] Run 100-concurrent MCP-call load test across mixed servers
- [ ] Run failover simulation by killing 5 random servers and verifying reconnect/degradation behavior
- [ ] Confirm TOOLS.md regeneration pipeline reflects all connected/disconnected servers

## MCP Deployment Checklist (Apply to Every Server, Waves 1–6)
- [ ] Build/push server image (ECR or bundled sidecar) and deploy runtime
- [ ] Register server manifest in `mcp_servers` and validate capability probe (`tools/list`)
- [ ] Configure OAuth/authentication and callback routing
- [ ] Apply risk classification + approval thresholds per tool
- [ ] Pass normalization test (Brain → MCP call → normalized `ToolResult`)
- [ ] Pass security tests (evil-server suite, provenance guardrails, privilege isolation)
- [ ] Verify cost tracking (per-call/per-run/per-server-daily) + rate limit counters
- [ ] Ensure `tool_executions` records include `is_mcp=true` and `mcp_server_id`
- [ ] Flag TOOLS.md refresh and verify nightly regeneration
- [ ] Add onboarding card (Waves 1–4) or contextual discovery trigger (Waves 5–6)
- [ ] Pass 3 golden scenario tests per server
- [ ] Update operational docs/runbooks for server-specific failure handling

## MCP Architecture Invariants (Month 1 → Month 15)
- [ ] Brain plane remains MCP-agnostic (no MCP-specific branching/imports in Brain logic)
- [ ] Shared ToolRegistry remains single source of truth for native + MCP tool schemas
- [ ] Content provenance tagging enforced end-to-end, including `mcp_result`
- [ ] Every MCP invocation recorded in shared `tool_executions` table path
- [ ] OAuth tokens for MCP servers stored in existing `oauth_tokens` table with `provider=<server_id>`
- [ ] Financial/booking tools always require explicit approval before write operations
- [ ] Sensitive financial data routes only through local-model path (`pii_content=true`)

---

# Cross-Cutting Requirements (Must Be Covered) (Section 3–34)

## Database + RLS (Section 3)
- [x] Create all enums exactly as Section 3.1 (channel_type, input_modality, run_state, llm_provider, etc.)
- [x] Create/alter all 19 tables exactly as Section 3.2–3.4 including enhanced columns and indexes
- [x] Ensure `channel_connections` is implemented and used for multi-channel identity linking (Section 3.2, Section 22, Section 28)
- [x] Ensure `profiling_sessions` table is implemented and used by the profiling engine (Section 3.3, Section 13)
- [x] Ensure `knowledge_graph_edges` table exists and is populated by memory + team inference baseline path (Section 3.4, Section 11, Section 24)
- [x] Ensure `accounts.voice_enabled` toggle exists and is enforced for voice I/O (Section 3.2, Section 21, Appendix A)
- [x] Enable RLS on all 19 tables and enforce `app.user_id` session variable on DB access paths (Section 3, Section 32)

## Contracts + Schemas (Section 4, Section 5)
- [x] Implement all Pydantic contracts from Section 4 and generate OpenAPI schemas
- [x] Implement `InboundMessage` (Section 4.1)
- [x] Implement `ProcessedMessage` (Section 4.1)
- [x] Implement `OutboundMessage` (Section 4.1)
- [x] Implement `IntentClassification` (Section 4.2)
- [x] Implement `TierRoutingConfig` (Section 4.2)
- [x] Implement `ToolSpec` (Section 4.3)
- [x] Implement `ToolCall` (Section 4.3)
- [x] Implement `ToolResult` (Section 4.3)
- [x] Implement `LLMRequest` (Section 4.4)
- [x] Implement `LLMResponse` (Section 4.4)
- [x] Implement `ProviderHealth` (Section 4.4)
- [x] Implement `DelegationRequest` (Section 4.5)
- [x] Implement `DelegationUpdate` (Section 4.5)
- [x] Implement `WorkflowDefinition` (Section 4.6)
- [x] Implement `WorkflowTrigger` (Section 4.6)
- [x] Implement `WorkflowAction` (Section 4.6)
- [x] Implement `WorkflowCondition` (Section 4.6)
- [ ] Implement unified API endpoints from Section 5 (Gateway/Core/Behavioral/New/Internal)
- [x] Implement `POST /webhook/whatsapp` (Phase 1) (Section 5.1)
- [ ] Implement `POST /webhook/imessage` (Phase 3) (Section 5.1)
- [ ] Implement `POST /webhook/slack` (Phase 3) (Section 5.1)
- [x] Implement `POST /api/v1/message` (Phase 1) (Section 5.1)
- [x] Implement `GET /api/v1/stream/{run_id}` (SSE) (Phase 1) (Section 5.1)
- [x] Implement `POST /api/v1/voice/transcribe` (Phase 1) (Section 5.1)
- [x] Implement `GET /api/v1/runs/{id}` (Phase 1) (Section 5.2)
- [x] Implement `POST /api/v1/runs/{id}/approve` (Phase 1–2) (Section 5.2)
- [x] Implement `POST /api/v1/memories/search` (Phase 2) (Section 5.2)
- [x] Implement `GET /api/v1/connectors` (Phase 1) (Section 5.2)
- [x] Implement `GET /api/v1/connectors/{provider}/auth` (Phase 1) (Section 5.2)
- [x] Implement `GET /api/v1/knowledge/{file_path}` (Phase 1) (Section 5.3)
- [x] Implement `PUT /api/v1/knowledge/{file_path}` (Phase 1) (Section 5.3)
- [x] Implement `GET /api/v1/knowledge` (Phase 1) (Section 5.3)
- [ ] Implement `POST /api/v1/knowledge/review` (Phase 3) (Section 5.3)
- [x] Implement `GET /api/v1/knowledge/graph` (Phase 2–4) (Section 5.3)
- [x] Implement `GET /api/v1/profiling/next` (Phase 1) (Section 5.3)
- [x] Implement `POST /api/v1/profiling/answer` (Phase 1) (Section 5.3)
- [x] Implement `GET /api/v1/goals` (Phase 1–3) (Section 5.3)
- [x] Implement `POST /api/v1/goals` (Phase 1–3) (Section 5.3)
- [x] Implement `PUT /api/v1/goals/{id}` (Phase 1–3) (Section 5.3)
- [x] Implement `DELETE /api/v1/goals/{id}` (Phase 1–3) (Section 5.3)
- [x] Implement `GET /api/v1/heartbeat` (Phase 3) (Section 5.3)
- [x] Implement `POST /api/v1/feedback` (Phase 2) (Section 5.3)
- [x] Implement `GET /api/v1/rules` (Phase 2) (Section 5.3)
- [x] Implement `PUT /api/v1/rules/{id}` (Phase 2) (Section 5.3)
- [x] Implement `DELETE /api/v1/rules/{id}` (Phase 2) (Section 5.3)
- [x] Implement `GET /api/v1/delegations` (Phase 2) (Section 5.4)
- [x] Implement `POST /api/v1/delegations` (Phase 2) (Section 5.4)
- [x] Implement `GET /api/v1/delegations/{id}` (Phase 2) (Section 5.4)
- [x] Implement `PUT /api/v1/delegations/{id}` (Phase 2) (Section 5.4)
- [ ] Implement `GET /api/v1/research` (Phase 3) (Section 5.4)
- [ ] Implement `POST /api/v1/research` (Phase 3) (Section 5.4)
- [ ] Implement `GET /api/v1/research/{id}` (Phase 3) (Section 5.4)
- [ ] Implement `PUT /api/v1/research/{id}` (Phase 3) (Section 5.4)
- [ ] Implement `DELETE /api/v1/research/{id}` (Phase 3) (Section 5.4)
- [ ] Implement `GET /api/v1/workflows` (Phase 3) (Section 5.4)
- [ ] Implement `POST /api/v1/workflows` (Phase 3) (Section 5.4)
- [ ] Implement `GET /api/v1/workflows/{id}` (Phase 3) (Section 5.4)
- [ ] Implement `PUT /api/v1/workflows/{id}` (Phase 3) (Section 5.4)
- [ ] Implement `DELETE /api/v1/workflows/{id}` (Phase 3) (Section 5.4)
- [ ] Implement `POST /api/v1/workflows/{id}/test` (Phase 3) (Section 5.4)
- [x] Implement `GET /api/v1/team` (Phase 2–3) (Section 5.4)
- [ ] Implement `GET /api/v1/export` (Phase 4) (Section 5.4)
- [x] Implement `GET /internal/health` (Phase 1) (Section 5.5)
- [x] Implement `GET /internal/health/deep` (Phase 1) (Section 5.5)
- [x] Implement `GET /internal/metrics` (Phase 1) (Section 5.5)
- [ ] Implement `POST /internal/runs/{id}/replay` (Phase 3–4) (Section 5.5)
- [ ] Implement `POST /internal/cache/flush` (Phase 3–4) (Section 5.5)
- [ ] Implement `POST /internal/triggers/fire` (Phase 3–4) (Section 5.5)
- [ ] Implement `POST /internal/knowledge/consolidate` (Phase 3) (Section 5.5)
- [ ] Implement `POST /internal/knowledge/review` (Phase 3) (Section 5.5)
- [x] Implement `GET /internal/llm/health` (Phase 1) (Section 5.5)
- [x] Implement `POST /internal/llm/route-test` (Phase 1) (Section 5.5)
- [ ] Implement `GET /internal/experiments` (Phase 4) (Section 5.5)
- [ ] Implement `POST /internal/experiments` (Phase 4) (Section 5.5)
- [ ] Implement Appendix B error codes taxonomy across endpoints + logs

## Knowledge Files (Section 8, Appendix D)
- [x] Implement 9 knowledge files with versioning, S3 snapshots, Redis hot cache
- [x] Implement completeness scoring and surface via `/api/v1/knowledge`

## Security (Section 32)
- [x] Implement content provenance tagging end-to-end (DB + runtime context)
- [x] Implement privilege isolation and capability tokens for tools
- [x] Implement output validation model pass for side-effecting actions
- [ ] Implement MCP sandboxing and network scoping

## Observability (Section 33)
- [ ] Implement all metrics in Section 33 (latency, error rate, tier distribution, cache hit rate, provider failover, etc.)
- [ ] Implement alerts thresholds matching Section 33

## Testing (Section 34)
- [ ] Unit tests for tier router, context compiler, knowledge merge, profiling extraction, LLM router selection
- [ ] Integration tests for Gateway→Brain→Hands and multi-modal pipeline
- [ ] Contract tests for Pydantic schemas and tool schemas
- [x] Agentic eval suite + red-team injection fuzzing in CI

---

# External Accounts / Services Needed (Section 2, Appendix A)
- [x] Clerk (auth) account + keys (`CLERK_SECRET_KEY`)
- [x] Anthropic account + `ANTHROPIC_API_KEY`
- [x] Google AI Studio key + `GOOGLE_AI_API_KEY`
- [x] Tavily account + `TAVILY_API_KEY`
- [ ] Unstructured.io account + API key (document parsing)
- [ ] Optional: local vLLM endpoint + `LOCAL_LLM_ENDPOINT`
- [ ] Optional: ElevenLabs key + `ELEVENLABS_API_KEY`

---

# Team & Ownership (Section 39)
- [ ] Define ownership matrix by plane: Gateway, Brain, Hands, Data, Infra, Security, Observability (Section 39)
- [ ] Define connector ownership: Google, Microsoft, Slack, Apple, Tavily, Plaid, MCP (Section 39)
- [ ] Define on-call rotation, escalation policy, and incident severity levels (Section 39)
