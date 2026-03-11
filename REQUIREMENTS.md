# REQUIREMENTS.md — Canonical Blueprint Requirements

Generated from: AI Agent (5).zip blueprint corpus
Date: 2026-03-11

---

## 4 Features Blueprint (Brevio_4Features_Blueprint.docx)

| ID | Source | Description | Subsystem | Type |
|----|--------|-------------|-----------|------|
| F-01 | 4Features | Multi-Bubble Delivery — responses split into 2-3 paced chat bubbles with typing indicator | web-demo | Feature |
| F-02 | 4Features | Send Button Pulse — haptic-equivalent glow animation on message arrival | web-demo | Feature |
| F-03 | 4Features | Scenario Starter Chips — 3 tappable scenario chips on splash screen | web-demo | Feature |
| F-04 | 4Features | Name Personalization — detect user name and inject into system prompt | web-demo | Feature |
| F-05 | 4Features | Complete brevio-live-demo.jsx replacement with all 4 features | web-demo | Implementation |
| F-06 | 4Features | Vitest unit tests: utils.test.ts, deliverInBubbles.test.ts | web-demo | Test |
| F-07 | 4Features | Playwright E2E tests: demo.spec.ts (6 journeys) | web-demo | Test |
| F-08 | 4Features | Config files: vite.config.ts, tsconfig.json, vitest.config.ts, playwright.config.ts, eslint.config.js, package.json | web-demo | Config |
| F-09 | 4Features | API integration with Anthropic Claude (POST /v1/messages, 8s timeout) | web-demo | Integration |
| F-10 | 4Features | Quick reply chip derivation from LLM response pattern matching | web-demo | Feature |
| F-11 | 4Features | formatText sanitization (XSS prevention, bold syntax, newlines) | web-demo | Security |
| F-12 | 4Features | State machine: SPLASH → CHAT with typing mutex and error handling | web-demo | Architecture |

## V10.1 Admin Intelligence (Brevio_V101_Admin_Blueprint.docx)

### Database Tables (16 new)

| ID | Source | Description | Subsystem | Type |
|----|--------|-------------|-----------|------|
| V101-DB-01 | V10.1 | llm_cost_ledger — token-level cost record per LLM call | database | Schema |
| V101-DB-02 | V10.1 | task_cost_rollup — per-workflow cost aggregation | database | Schema |
| V101-DB-03 | V10.1 | connector_cost_ledger — per-call cost for paid connectors | database | Schema |
| V101-DB-04 | V10.1 | user_cost_daily_rollup — pre-aggregated daily cost per user | database | Schema |
| V101-DB-05 | V10.1 | operator_margin_report — daily P&L snapshot | database | Schema |
| V101-DB-06 | V10.1 | agent_kill_switches — per-user kill switch state | database | Schema |
| V101-DB-07 | V10.1 | agent_kill_switch_log — audit log for kill switch actions | database | Schema |
| V101-DB-08 | V10.1 | skill_acl_overrides — per-user per-skill enable/disable | database | Schema |
| V101-DB-09 | V10.1 | subscription_events — raw Stripe webhook events | database | Schema |
| V101-DB-10 | V10.1 | mrr_snapshots — daily MRR/ARR snapshot | database | Schema |
| V101-DB-11 | V10.1 | user_cohorts — cohort assignment per user | database | Schema |
| V101-DB-12 | V10.1 | cohort_retention — weekly retention computation | database | Schema |
| V101-DB-13 | V10.1 | oauth_token_registry — OAuth token expiry tracking | database | Schema |
| V101-DB-14 | V10.1 | behavioral_risk_scores + behavioral_risk_history — risk scoring | database | Schema |
| V101-DB-15 | V10.1 | feature_adoption_events — skill usage tracking | database | Schema |
| V101-DB-16 | V10.1 | tool_mttr_log — tool quarantine recovery tracking | database | Schema |
| V101-DB-17 | V10.1 | agent_action_replay_log — workflow replay audit | database | Schema |

### API Endpoints

| ID | Source | Description | Subsystem | Type |
|----|--------|-------------|-----------|------|
| V101-API-01 | V10.1 | GET /admin/costs/summary — system-wide cost summary | admin-api | Endpoint |
| V101-API-02 | V10.1 | GET /admin/costs/users — ranked user list by cost | admin-api | Endpoint |
| V101-API-03 | V10.1 | GET /admin/costs/users/{id}/breakdown — per-user cost detail | admin-api | Endpoint |
| V101-API-04 | V10.1 | GET /admin/costs/margin — operator gross margin dashboard | admin-api | Endpoint |
| V101-API-05 | V10.1 | GET /admin/costs/projections — burn rate projections | admin-api | Endpoint |
| V101-API-06 | V10.1 | GET /admin/revenue/mrr — MRR/ARR dashboard | admin-api | Endpoint |
| V101-API-07 | V10.1 | GET /admin/revenue/cohorts — D7/D30/D60/D90 retention cohorts | admin-api | Endpoint |
| V101-API-08 | V10.1 | GET /admin/revenue/ltv — LTV, LTV:CAC, ARPU | admin-api | Endpoint |
| V101-API-09 | V10.1 | PUT /admin/revenue/cac — set customer acquisition cost | admin-api | Endpoint |
| V101-API-10 | V10.1 | POST /admin/users/{id}/kill-switch — activate kill switch | admin-api | Endpoint |
| V101-API-11 | V10.1 | DELETE /admin/users/{id}/kill-switch — deactivate kill switch | admin-api | Endpoint |
| V101-API-12 | V10.1 | PUT /admin/users/{id}/skill-acl — set skill ACL overrides | admin-api | Endpoint |
| V101-API-13 | V10.1 | POST /admin/agent/replay — enqueue workflow replay | admin-api | Endpoint |
| V101-API-14 | V10.1 | GET /admin/oauth/expiry — OAuth token expiry dashboard | admin-api | Endpoint |
| V101-API-15 | V10.1 | GET /admin/tools/mttr — tool MTTR statistics | admin-api | Endpoint |
| V101-API-16 | V10.1 | GET /admin/features/adoption — feature adoption heatmap | admin-api | Endpoint |
| V101-API-17 | V10.1 | GET /admin/users/{id}/risk — behavioral risk score detail | admin-api | Endpoint |

### Temporal Workflows (7)

| ID | Source | Description | Subsystem | Type |
|----|--------|-------------|-----------|------|
| V101-WF-01 | V10.1 | CostRollupWorkflow — hourly cost aggregation | temporal | Workflow |
| V101-WF-02 | V10.1 | DailyUserCostRollup — daily user cost rollup at 01:00 UTC | temporal | Workflow |
| V101-WF-03 | V10.1 | MarginSnapshotWorkflow — daily margin at 02:00 UTC | temporal | Workflow |
| V101-WF-04 | V10.1 | MRRSnapshotWorkflow — daily MRR at 03:00 UTC | temporal | Workflow |
| V101-WF-05 | V10.1 | CohortComputeWorkflow — weekly cohort at Sun 04:00 UTC | temporal | Workflow |
| V101-WF-06 | V10.1 | BehavioralRiskWorkflow — every 6h risk scoring | temporal | Workflow |
| V101-WF-07 | V10.1 | OAuthExpiryCheckWorkflow — daily at 06:00 UTC | temporal | Workflow |

### OPA Policies

| ID | Source | Description | Subsystem | Type |
|----|--------|-------------|-----------|------|
| V101-POL-01 | V10.1 | policies/agent_kill_switch.rego — kill switch gate (priority 1) | policies | Policy |
| V101-POL-02 | V10.1 | policies/skill_acl.rego — skill ACL gate (priority 2) | policies | Policy |
| V101-POL-03 | V10.1 | ExecutionGate chain: kill_switch → skill_acl → autonomy → budget → tool_health → content_safety → rate_limit | control | Architecture |

### Infrastructure & Observability

| ID | Source | Description | Subsystem | Type |
|----|--------|-------------|-----------|------|
| V101-INF-01 | V10.1 | Migration: 004_brevio_v101_admin_intelligence.sql | database | Migration |
| V101-INF-02 | V10.1 | Helm: cost-rollup-worker (2 replicas, HPA) | infrastructure | Deployment |
| V101-INF-03 | V10.1 | Helm: risk-worker (1 replica) | infrastructure | Deployment |
| V101-INF-04 | V10.1 | 7 Temporal cron schedule registrations | infrastructure | Config |
| V101-INF-05 | V10.1 | 13 new Prometheus metrics (BREVIO_ prefix) | observability | Metrics |
| V101-INF-06 | V10.1 | 10 canonical events (BREVIO.* namespace) | observability | Events |
| V101-INF-07 | V10.1 | Alert rules: margin, cost spike, MRR drop, kill switch volume, OAuth expiry | observability | Alerts |
| V101-INF-08 | V10.1 | 7 SLO definitions (V101-01 through V101-07) | observability | SLO |
| V101-INF-09 | V10.1 | 8 new environment variables | infrastructure | Config |
| V101-INF-10 | V10.1 | Database role grants: brain_writer, executor_writer, rollup_worker, control_writer, admin_reader | security | RBAC |
| V101-INF-11 | V10.1 | CI extensions: hotpath checker, metrics auditor, OPA tests, migration safety, contract tests | ci-cd | Pipeline |

### Tests

| ID | Source | Description | Subsystem | Type |
|----|--------|-------------|-----------|------|
| V101-TST-01 | V10.1 | internal/cost/ledger_test.go — LLM cost computation | testing | Unit |
| V101-TST-02 | V10.1 | internal/cost/rollup_test.go — task cost rollup | testing | Unit |
| V101-TST-03 | V10.1 | internal/cost/projection_test.go — burn rate projection | testing | Unit |
| V101-TST-04 | V10.1 | internal/revenue/mrr_test.go — MRR waterfall | testing | Unit |
| V101-TST-05 | V10.1 | internal/revenue/cohort_test.go — cohort retention | testing | Unit |
| V101-TST-06 | V10.1 | internal/risk/scorer_test.go — risk score formula | testing | Unit |
| V101-TST-07 | V10.1 | internal/risk/spike_test.go — cost spike detection | testing | Unit |
| V101-TST-08 | V10.1 | internal/policies/kill_switch_test.go — OPA kill switch | testing | Unit |
| V101-TST-09 | V10.1 | internal/policies/skill_acl_test.go — OPA skill ACL | testing | Unit |
| V101-TST-10 | V10.1 | internal/oauth/expiry_test.go — OAuth expiry | testing | Unit |
| V101-TST-11 | V10.1 | internal/integration/kill_switch_gate_test.go | testing | Integration |
| V101-TST-12 | V10.1 | internal/integration/skill_acl_gate_test.go | testing | Integration |
| V101-TST-13 | V10.1 | internal/integration/cost_pipeline_test.go | testing | Integration |
| V101-TST-14 | V10.1 | internal/integration/daily_rollup_test.go | testing | Integration |
| V101-TST-15 | V10.1 | internal/integration/mrr_snapshot_test.go | testing | Integration |
| V101-TST-16 | V10.1 | internal/integration/margin_snapshot_test.go | testing | Integration |
| V101-TST-17 | V10.1 | internal/integration/oauth_workflow_test.go | testing | Integration |
| V101-TST-18 | V10.1 | internal/integration/replay_test.go | testing | Integration |
| V101-TST-19 | V10.1 | internal/integration/cost_api_test.go | testing | Integration |
| V101-TST-20 | V10.1 | internal/integration/revenue_api_test.go | testing | Integration |
| V101-TST-21 | V10.1 | internal/contract/admin_cost_test.go | testing | Contract |
| V101-TST-22 | V10.1 | internal/contract/admin_revenue_test.go | testing | Contract |
| V101-TST-23 | V10.1 | internal/contract/agent_control_test.go | testing | Contract |
| V101-TST-24 | V10.1 | internal/temporal/replay_safety_test.go | testing | Replay |
| V101-TST-25 | V10.1 | policies/agent_kill_switch_test.rego — 5 OPA test cases | testing | Policy |

## V10.2 Intelligence & Performance (Brevio_V102_Intelligence_Addendum.docx)

### Database Tables (5 new + 5 extended)

| ID | Source | Description | Subsystem | Type |
|----|--------|-------------|-----------|------|
| V102-DB-01 | V10.2 | eq_response_strategy_matrix — EQ behavioral response policies | database | Schema |
| V102-DB-02 | V10.2 | confidence_calibration_config — classifier calibration | database | Schema |
| V102-DB-03 | V10.2 | lesson_conflict_resolutions — semantic conflict resolution | database | Schema |
| V102-DB-04 | V10.2 | world_model_cache — agent world state cache | database | Schema |
| V102-DB-05 | V10.2 | keyword_routing_rules — keyword classifier fallback | database | Schema |
| V102-DB-06 | V10.2 | memory_items extended: relevance_score, access_count, decay fields | database | Schema |
| V102-DB-07 | V10.2 | autonomy_trust_scores extended: demotion fields | database | Schema |
| V102-DB-08 | V10.2 | context_budgets extended: selection_config JSONB | database | Schema |
| V102-DB-09 | V10.2 | rag_chunks extended: temporal_relevance_score, temporal_lambda | database | Schema |
| V102-DB-10 | V10.2 | rag_collections extended: temporal_config, chunking_strategy_config | database | Schema |

### Intelligence Features (34 gaps)

| ID | Source | Description | Subsystem | Type |
|----|--------|-------------|-----------|------|
| V102-INT-01 | V10.2 | EQ response strategy — emotional intelligence responses | brain/eq | Feature |
| V102-INT-02 | V10.2 | Autonomy demotion — trust score demotion on incidents/drift | brain/trust | Feature |
| V102-INT-03 | V10.2 | Proactive interruption rules — quiet hours, priority thresholds | brain | Feature |
| V102-INT-04 | V10.2 | CRITIC prompt (PROMPT_CRITIC_V1) — evaluation dimensions | brain | Prompt |
| V102-INT-05 | V10.2 | REFLECTOR prompt (PROMPT_REFLECTOR_V1) — lesson extraction | brain | Prompt |
| V102-INT-06 | V10.2 | Memory decay/forgetting mechanism | memory | Feature |
| V102-INT-07 | V10.2 | Lesson consolidation — feedback clustering + conflict detection | learning | Feature |
| V102-INT-08 | V10.2 | Knowledge file drift detection (SOUL.md/USER.md staleness) | knowledge | Feature |
| V102-INT-09 | V10.2 | Context allocation algorithm — ranking strategy per source | brain/context | Feature |
| V102-INT-10 | V10.2 | Uncertainty quantification — confidence expressions | brain | Feature |
| V102-INT-11 | V10.2 | Multi-intent classification | brain | Feature |
| V102-INT-12 | V10.2 | Speculative prefetch on fast-path | fastpath | Feature |
| V102-INT-13 | V10.2 | Dynamic task decomposition with complexity scoring | brain | Feature |
| V102-INT-14 | V10.2 | Partial failure handling policies | brain | Feature |
| V102-INT-15 | V10.2 | Confidence calibration (Platt scaling) | brain | Feature |
| V102-INT-16 | V10.2 | World model — derived state from tool execution | brain | Feature |
| V102-INT-17 | V10.2 | Keyword classifier fallback routing | brain | Feature |
| V102-INT-18 | V10.2 | Counterfactual reasoning | brain | Feature |
| V102-INT-19 | V10.2 | RAG temporal freshness scoring | rag | Feature |
| V102-INT-20 | V10.2 | RAG chunking strategy per content type | rag | Feature |

### Temporal Workflows (6)

| ID | Source | Description | Subsystem | Type |
|----|--------|-------------|-----------|------|
| V102-WF-01 | V10.2 | MemoryDecayWorkflow — weekly relevance decay | temporal | Workflow |
| V102-WF-02 | V10.2 | LessonConsolidationWorkflow — weekly feedback clustering | temporal | Workflow |
| V102-WF-03 | V10.2 | CalibrationWorkflow — weekly confidence calibration | temporal | Workflow |
| V102-WF-04 | V10.2 | KnowledgeFileDriftDetector — monthly staleness check | temporal | Workflow |
| V102-WF-05 | V10.2 | SpeculativePrefetchWorker — 15-min predictive prefetch | temporal | Workflow |
| V102-WF-06 | V10.2 | FastPathCacheWarmer — nightly cache warming | temporal | Workflow |

## V10.3 Cognitive Intelligence (Brevio_V103_Cognitive_Intelligence.docx)

### Database Tables (11 new + 2 extended)

| ID | Source | Description | Subsystem | Type |
|----|--------|-------------|-----------|------|
| V103-DB-01 | V10.3 | system1_heuristics — learned instant-tier patterns | database | Schema |
| V103-DB-02 | V10.3 | thought_graphs + thought_nodes — GoT reasoning | database | Schema |
| V103-DB-03 | V10.3 | domain_performance_history — per-domain success rates | database | Schema |
| V103-DB-04 | V10.3 | user_knowledge_model — belief/knowledge gap tracking | database | Schema |
| V103-DB-05 | V10.3 | belief_distributions — Bayesian preference distributions | database | Schema |
| V103-DB-06 | V10.3 | prospective_memories — event-triggered reminders | database | Schema |
| V103-DB-07 | V10.3 | implicit_behavior_signals — behavioral preference capture | database | Schema |
| V103-DB-08 | V10.3 | case_library — problem-solution cases for analogical reasoning | database | Schema |
| V103-DB-09 | V10.3 | clarification_candidates — optimal question scoring | database | Schema |
| V103-DB-10 | V10.3 | consolidation_runs — episodic-to-semantic audit | database | Schema |
| V103-DB-11 | V10.3 | behavioral_baselines — drift baseline computation | database | Schema |

### Cognitive Features (11 systems)

| ID | Source | Description | Subsystem | Type |
|----|--------|-------------|-----------|------|
| V103-COG-01 | V10.3 | Dual-process (System 1/2) — fast heuristic path | cognition | Feature |
| V103-COG-02 | V10.3 | Graph-of-Thought reasoning — branch/merge/evaluate | cognition | Feature |
| V103-COG-03 | V10.3 | Metacognitive monitor — domain confidence + tier upgrade | cognition | Feature |
| V103-COG-04 | V10.3 | Theory of Mind — user knowledge model | cognition | Feature |
| V103-COG-05 | V10.3 | Bayesian belief maintenance — preference distributions | cognition | Feature |
| V103-COG-06 | V10.3 | Prospective memory — event-triggered injection | cognition | Feature |
| V103-COG-07 | V10.3 | Implicit preference learning — behavioral signal capture | cognition | Feature |
| V103-COG-08 | V10.3 | Case-based reasoning — analogical problem solving | cognition | Feature |
| V103-COG-09 | V10.3 | Clarification optimizer — information gain scoring | cognition | Feature |
| V103-COG-10 | V10.3 | Memory consolidation — episodic → semantic extraction | cognition | Feature |
| V103-COG-11 | V10.3 | Concept drift detection — JS-divergence baseline | cognition | Feature |

### Temporal Workflows (6)

| ID | Source | Description | Subsystem | Type |
|----|--------|-------------|-----------|------|
| V103-WF-01 | V10.3 | HeuristicUpdateWorkflow — daily System 1 update | temporal | Workflow |
| V103-WF-02 | V10.3 | MetacognitiveRecalculationWorkflow — weekly domain recalc | temporal | Workflow |
| V103-WF-03 | V10.3 | BeliefMaintenanceWorkflow — daily belief decay | temporal | Workflow |
| V103-WF-04 | V10.3 | MemoryConsolidationWorkflow — nightly episodic→semantic | temporal | Workflow |
| V103-WF-05 | V10.3 | DriftDetectionWorkflow — weekly behavioral drift | temporal | Workflow |
| V103-WF-06 | V10.3 | CaseLibraryMaintenanceWorkflow — weekly case pruning | temporal | Workflow |

### Prompts (4)

| ID | Source | Description | Subsystem | Type |
|----|--------|-------------|-----------|------|
| V103-PRM-01 | V10.3 | PROMPT_CONSOLIDATION_V1 — episodic pattern extraction | prompts | Prompt |
| V103-PRM-02 | V10.3 | PROMPT_GOT_BRANCH_V1 — thought branching | prompts | Prompt |
| V103-PRM-03 | V10.3 | PROMPT_GOT_MERGE_V1 — thought merging | prompts | Prompt |
| V103-PRM-04 | V10.3 | PROMPT_CLARIFICATION_SCORER_V1 — info gain estimation | prompts | Prompt |

## V10.4 Voice & Calling (Brevio_V104_Blueprint.docx)

### Database Tables (9 new)

| ID | Source | Description | Subsystem | Type |
|----|--------|-------------|-----------|------|
| V104-DB-01 | V10.4 | outbound_calls — main call record | database | Schema |
| V104-DB-02 | V10.4 | call_assistants — provider assistant templates | database | Schema |
| V104-DB-03 | V10.4 | call_phone_numbers — provisioned outbound numbers | database | Schema |
| V104-DB-04 | V10.4 | call_approval_requests — user approval records | database | Schema |
| V104-DB-05 | V10.4 | call_retry_log — retry attempts | database | Schema |
| V104-DB-06 | V10.4 | call_scheduled — scheduled retries | database | Schema |
| V104-DB-07 | V10.4 | voice_sessions — session metadata | database | Schema |
| V104-DB-08 | V10.4 | voice_turns — per-turn transcript | database | Schema |
| V104-DB-09 | V10.4 | voice_worker_health — worker pod metrics | database | Schema |

### Go Packages & Files

| ID | Source | Description | Subsystem | Type |
|----|--------|-------------|-----------|------|
| V104-PKG-01 | V10.4 | internal/hands/call/ — outbound calling (9 files) | hands/call | Package |
| V104-PKG-02 | V10.4 | internal/voice/worker/ — real-time voice (10 files) | voice/worker | Package |
| V104-FILE-01 | V10.4 | hands/call/vapi_client.go — VAPI API client | hands/call | File |
| V104-FILE-02 | V10.4 | hands/call/retell_client.go — Retell AI client | hands/call | File |
| V104-FILE-03 | V10.4 | hands/call/call_provider.go — provider interface + failover | hands/call | File |
| V104-FILE-04 | V10.4 | hands/call/webhook_handler.go — webhook HMAC handler | hands/call | File |
| V104-FILE-05 | V10.4 | hands/call/outcome_parser.go — transcript → outcome | hands/call | File |
| V104-FILE-06 | V10.4 | hands/call/prompt_builder.go — 5 system prompt templates | hands/call | File |
| V104-FILE-07 | V10.4 | hands/call/scheduler.go — Temporal call scheduling | hands/call | File |
| V104-FILE-08 | V10.4 | hands/call/places_lookup.go — Google Places lookup | hands/call | File |
| V104-FILE-09 | V10.4 | hands/call/call_service.go — main orchestrator | hands/call | File |
| V104-FILE-10 | V10.4 | voice/worker/agent.go — LiveKit agent | voice/worker | File |
| V104-FILE-11 | V10.4 | voice/worker/stt.go — Deepgram STT client | voice/worker | File |
| V104-FILE-12 | V10.4 | voice/worker/stt_whisper.go — Whisper fallback | voice/worker | File |
| V104-FILE-13 | V10.4 | voice/worker/tts.go — ElevenLabs TTS client | voice/worker | File |
| V104-FILE-14 | V10.4 | voice/worker/tts_cartesia.go — Cartesia fallback | voice/worker | File |
| V104-FILE-15 | V10.4 | voice/worker/vad.go — Voice Activity Detection | voice/worker | File |
| V104-FILE-16 | V10.4 | voice/worker/session_manager.go — room/token lifecycle | voice/worker | File |
| V104-FILE-17 | V10.4 | voice/worker/context_loader.go — USER.md/SOUL.md loader | voice/worker | File |
| V104-FILE-18 | V10.4 | voice/worker/task_extractor.go — post-session extraction | voice/worker | File |
| V104-FILE-19 | V10.4 | voice/worker/health.go — worker health metrics | voice/worker | File |

### API Endpoints

| ID | Source | Description | Subsystem | Type |
|----|--------|-------------|-----------|------|
| V104-API-01 | V10.4 | POST /webhooks/vapi/call-end — VAPI webhook handler | hands/call | Endpoint |
| V104-API-02 | V10.4 | POST /webhooks/retell/call-end — Retell webhook handler | hands/call | Endpoint |
| V104-API-03 | V10.4 | POST /voice/sessions — create voice session | voice | Endpoint |
| V104-API-04 | V10.4 | DELETE /voice/sessions/:id — end voice session | voice | Endpoint |

### Temporal Workflows

| ID | Source | Description | Subsystem | Type |
|----|--------|-------------|-----------|------|
| V104-WF-01 | V10.4 | ScheduleCallRetryWorkflow — retry scheduling with max 3 retries | temporal | Workflow |

### OPA Policies

| ID | Source | Description | Subsystem | Type |
|----|--------|-------------|-----------|------|
| V104-POL-01 | V10.4 | policies/outbound_call.rego — call authorization (type, approval, retry limit, wallet) | policies | Policy |

### Migrations

| ID | Source | Description | Subsystem | Type |
|----|--------|-------------|-----------|------|
| V104-MIG-01 | V10.4 | migrations/016_brevio_v104_outbound.sql — 5 enums, 6 tables | database | Migration |
| V104-MIG-02 | V10.4 | migrations/017_brevio_v104_voice.sql — 1 enum, 3 tables | database | Migration |

### Infrastructure

| ID | Source | Description | Subsystem | Type |
|----|--------|-------------|-----------|------|
| V104-INF-01 | V10.4 | 17 new environment variables (8 call + 9 voice) | infrastructure | Config |
| V104-INF-02 | V10.4 | 5 skill registrations (reservation, appointment, pharmacy, quote, general) | connectors | Config |
| V104-INF-03 | V10.4 | 18 Prometheus metrics (10 call + 8 voice) | observability | Metrics |
| V104-INF-04 | V10.4 | Provider failover: VAPI→Retell, LiveKit→Daily.co, Deepgram→Whisper, ElevenLabs→Cartesia | infrastructure | Resilience |
| V104-INF-05 | V10.4 | Voice worker K8s HPA scaling (max 50 pods, 10 sessions/pod) | infrastructure | Scaling |
| V104-INF-06 | V10.4 | 5 call system prompt templates (RESERVATION, APPOINTMENT, PHARMACY, QUOTE, GENERAL) | hands/call | Config |

## UX Copy (4 JSX files)

| ID | Source | Description | Subsystem | Type |
|----|--------|-------------|-----------|------|
| UX-01 | onboarding-copy | 6-stage onboarding flow (welcome, discovery, OAuth, calibration, first value, wrap up) | ux-copy | Content |
| UX-02 | copy-part2 | Morning briefing (4 scenarios), approval requests (5), skip paths (6) | ux-copy | Content |
| UX-03 | copy-part3 | Proactive suggestions (5), corrections (4), EOD recap (4), trust/autonomy (4), billing (4), re-auth (3), outages (3), learning loop (4), recurring tasks (5) | ux-copy | Content |
| UX-04 | error-states | Silent user (3), OAuth errors (2), unexpected responses (2), task failures (3), system limits (2) | ux-copy | Content |

---

**Total Requirements: ~180+**
- 4 Features: 12
- V10.1: 17 DB + 17 API + 7 WF + 3 POL + 11 INF + 25 TST = 80
- V10.2: 10 DB + 20 INT + 6 WF = 36
- V10.3: 11 DB + 11 COG + 6 WF + 4 PRM = 32
- V10.4: 9 DB + 19 FILES + 4 API + 1 WF + 1 POL + 2 MIG + 6 INF = 42
- UX: 4
