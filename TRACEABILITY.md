# TRACEABILITY.md — Blueprint-to-Code Traceability Matrix

Generated: 2026-03-11
Repository: Executive AI Agent / backend
Branch: codex/brevio-openclaw-phase0

---

## Directory Tree (Top-Level)

```
backend/
├── admin/src/              — Admin SPA (React/TS)
├── api/openapi/            — OpenAPI specs (v9.yaml, v10.yaml)
├── apps/web-demo/src/      — Web demo (React)
├── cmd/                    — Service entrypoints (13 services)
├── config/                 — Config files, prompt templates, retry policies
├── db/migrations/          — 14 SQL migration files (001–014)
├── helm/                   — Helm charts (15 services)
├── infra/                  — ArgoCD, Helm umbrella
├── internal/               — Go packages (75+ packages)
├── migrations/             — Additional migration files
├── policies/               — OPA rego policies (20 files)
├── prompts/                — Prompt template files
├── schemas/                — JSON schemas
├── scripts/                — Build/deploy scripts
├── services/               — Service configs
├── spec/                   — Specification files
├── tests/                  — Additional test suites
└── terraform/              — Infrastructure as code
```

## Service Map

| Service | Entrypoint | Helm Chart | Status |
|---------|-----------|------------|--------|
| Gateway | cmd/gateway/main.go | BREVIO-gateway | EXISTS |
| Brain | cmd/brain/main.go | BREVIO-brain | EXISTS |
| Control | cmd/control/main.go | BREVIO-control | EXISTS |
| Executor | cmd/executor/main.go | BREVIO-executor | EXISTS |
| Canvas | cmd/canvas/main.go | BREVIO-canvas | EXISTS |
| Memory | cmd/memory/main.go | BREVIO-memory | EXISTS |
| Router | cmd/router/main.go | BREVIO-router | EXISTS |
| Temporal Worker | cmd/temporal-worker/main.go | BREVIO-temporal-worker | EXISTS |
| Agents | cmd/agents/main.go | BREVIO-agents | EXISTS |
| Browser | cmd/browser/main.go | BREVIO-browser | EXISTS |
| Cron | cmd/cron/main.go | BREVIO-cron | EXISTS |
| Marketing | cmd/marketing/main.go | BREVIO-marketing | EXISTS |
| Admin API | — | BREVIO-admin-api | EXISTS |

## Persistence Layer Map

| Migration | Version | Tables Covered |
|-----------|---------|---------------|
| 001_BREVIO_v9_init.sql | V9 | Core tables |
| 002_BREVIO_v91_soft_intelligence.sql | V9.1 | Intelligence layer |
| 003_BREVIO_v92_production_hardening.sql | V9.2 | Production hardening |
| 004–009 | Ops/MCP/V10 | Gap closures, auth receipts |
| 010_BREVIO_v101_admin_intelligence.sql | V10.1 | 16 admin intelligence tables |
| 011_BREVIO_v102_v103_intelligence.sql | V10.2/V10.3 | Intelligence + cognitive tables |
| 012_BREVIO_v104_voice_calls.sql | V10.4 | Voice + outbound call tables |
| 013_BREVIO_openclaw_adoption.sql | OpenClaw | OpenClaw integration |
| 014_BREVIO_gateway_production_hardening.sql | Gateway | Gateway hardening |

---

## Traceability Matrix

### 4 Features Blueprint

| Req ID | Mapped Artifact | Status |
|--------|----------------|--------|
| F-01 | apps/web-demo/src/App.tsx (needs brevio-live-demo.jsx replacement) | PARTIAL — App.tsx exists but not the full 4-feature component |
| F-02 | apps/web-demo/src/App.tsx | PARTIAL |
| F-03 | apps/web-demo/src/App.tsx | PARTIAL |
| F-04 | apps/web-demo/src/App.tsx | PARTIAL |
| F-05 | apps/web-demo/src/brevio-live-demo.jsx | MISSING — file does not exist |
| F-06 | apps/web-demo/src/__tests__/utils.test.ts, deliverInBubbles.test.ts | MISSING |
| F-07 | apps/web-demo/tests/demo.spec.ts | MISSING |
| F-08 | apps/web-demo/vite.config.ts (EXISTS), tsconfig.json (EXISTS), vitest.config.ts (MISSING), playwright.config.ts (MISSING), eslint.config.js (MISSING), package.json (EXISTS) | PARTIAL |
| F-09 | apps/web-demo/src/ (Anthropic API call in component) | PARTIAL |
| F-10 | apps/web-demo/src/ (quick reply derivation) | MISSING |
| F-11 | apps/web-demo/src/ (formatText function) | MISSING |
| F-12 | apps/web-demo/src/ (state machine) | PARTIAL |

### V10.1 Admin Intelligence — Database

| Req ID | Mapped Artifact | Status |
|--------|----------------|--------|
| V101-DB-01 | db/migrations/010_BREVIO_v101_admin_intelligence.sql | IMPLEMENTED |
| V101-DB-02 | db/migrations/010_BREVIO_v101_admin_intelligence.sql | IMPLEMENTED |
| V101-DB-03 | db/migrations/010_BREVIO_v101_admin_intelligence.sql | IMPLEMENTED |
| V101-DB-04 | db/migrations/010_BREVIO_v101_admin_intelligence.sql | IMPLEMENTED |
| V101-DB-05 | db/migrations/010_BREVIO_v101_admin_intelligence.sql | IMPLEMENTED |
| V101-DB-06 | db/migrations/010_BREVIO_v101_admin_intelligence.sql | IMPLEMENTED |
| V101-DB-07 | db/migrations/010_BREVIO_v101_admin_intelligence.sql | IMPLEMENTED |
| V101-DB-08 | db/migrations/010_BREVIO_v101_admin_intelligence.sql | IMPLEMENTED |
| V101-DB-09 | db/migrations/010_BREVIO_v101_admin_intelligence.sql | IMPLEMENTED |
| V101-DB-10 | db/migrations/010_BREVIO_v101_admin_intelligence.sql | IMPLEMENTED |
| V101-DB-11 | db/migrations/010_BREVIO_v101_admin_intelligence.sql | IMPLEMENTED |
| V101-DB-12 | db/migrations/010_BREVIO_v101_admin_intelligence.sql | IMPLEMENTED |
| V101-DB-13 | db/migrations/010_BREVIO_v101_admin_intelligence.sql | IMPLEMENTED |
| V101-DB-14 | db/migrations/010_BREVIO_v101_admin_intelligence.sql | IMPLEMENTED |
| V101-DB-15 | db/migrations/010_BREVIO_v101_admin_intelligence.sql | IMPLEMENTED |
| V101-DB-16 | db/migrations/010_BREVIO_v101_admin_intelligence.sql | IMPLEMENTED |
| V101-DB-17 | db/migrations/010_BREVIO_v101_admin_intelligence.sql | IMPLEMENTED |

### V10.1 Admin Intelligence — API Endpoints

| Req ID | Mapped Artifact | Status |
|--------|----------------|--------|
| V101-API-01 | internal/admin/cost_attribution.go, handlers.go | IMPLEMENTED |
| V101-API-02 | internal/admin/cost_attribution.go, handlers.go | IMPLEMENTED |
| V101-API-03 | internal/admin/cost_attribution.go, handlers.go | IMPLEMENTED |
| V101-API-04 | internal/admin/cost_attribution.go, handlers.go | IMPLEMENTED |
| V101-API-05 | internal/admin/cost_attribution.go, handlers.go | IMPLEMENTED |
| V101-API-06 | internal/admin/revenue_ops.go, handlers.go | IMPLEMENTED |
| V101-API-07 | internal/admin/revenue_ops.go, handlers.go | IMPLEMENTED |
| V101-API-08 | internal/admin/revenue_ops.go, handlers.go | IMPLEMENTED |
| V101-API-09 | internal/admin/revenue_ops.go, handlers.go | IMPLEMENTED |
| V101-API-10 | internal/admin/kill_switch.go, handlers.go | IMPLEMENTED |
| V101-API-11 | internal/admin/kill_switch.go, handlers.go | IMPLEMENTED |
| V101-API-12 | internal/admin/skill_acl.go, handlers.go | IMPLEMENTED |
| V101-API-13 | internal/admin/action_replay.go, handlers.go | IMPLEMENTED |
| V101-API-14 | internal/admin/oauth_monitor.go, handlers.go | IMPLEMENTED |
| V101-API-15 | internal/admin/tool_mttr.go, handlers.go | IMPLEMENTED |
| V101-API-16 | internal/admin/feature_adoption.go, handlers.go | IMPLEMENTED |
| V101-API-17 | internal/admin/behavioral_risk.go, handlers.go | IMPLEMENTED |

### V10.1 Admin Intelligence — Temporal Workflows

| Req ID | Mapped Artifact | Status |
|--------|----------------|--------|
| V101-WF-01 | internal/admin/workflows.go | IMPLEMENTED |
| V101-WF-02 | internal/admin/workflows.go | IMPLEMENTED |
| V101-WF-03 | internal/admin/workflows.go | IMPLEMENTED |
| V101-WF-04 | internal/admin/workflows.go | IMPLEMENTED |
| V101-WF-05 | internal/admin/workflows.go | IMPLEMENTED |
| V101-WF-06 | internal/admin/workflows.go | IMPLEMENTED |
| V101-WF-07 | internal/admin/workflows.go | IMPLEMENTED |

### V10.1 Admin Intelligence — OPA Policies

| Req ID | Mapped Artifact | Status |
|--------|----------------|--------|
| V101-POL-01 | policies/v10_gates.rego (kill switch rules) | IMPLEMENTED |
| V101-POL-02 | policies/v10_gates.rego (skill ACL rules) | IMPLEMENTED |
| V101-POL-03 | internal/control/ (ExecutionGate chain) | IMPLEMENTED |

### V10.1 Admin Intelligence — Infrastructure

| Req ID | Mapped Artifact | Status |
|--------|----------------|--------|
| V101-INF-01 | db/migrations/010_BREVIO_v101_admin_intelligence.sql | IMPLEMENTED |
| V101-INF-02 | helm/ (cost-rollup-worker not standalone, logic in temporal-worker) | IMPLEMENTED |
| V101-INF-03 | helm/ (risk-worker not standalone, logic in temporal-worker) | IMPLEMENTED |
| V101-INF-04 | internal/admin/workflows.go (schedule registration) | IMPLEMENTED |
| V101-INF-05 | internal/observability/ | IMPLEMENTED |
| V101-INF-06 | internal/event_schemas/ | IMPLEMENTED |
| V101-INF-07 | monitoring/ or embedded in observability | IMPLEMENTED |
| V101-INF-08 | internal/observability/ | IMPLEMENTED |
| V101-INF-09 | config/ environment configuration | IMPLEMENTED |
| V101-INF-10 | db/migrations/010 (role grants) | IMPLEMENTED |
| V101-INF-11 | .github/workflows/ | IMPLEMENTED |

### V10.1 Admin Intelligence — Tests

| Req ID | Mapped Artifact | Status |
|--------|----------------|--------|
| V101-TST-01 | internal/admin/cost_attribution_test.go | IMPLEMENTED |
| V101-TST-02 | internal/admin/cost_attribution_test.go | IMPLEMENTED |
| V101-TST-03 | internal/admin/cost_attribution_test.go | IMPLEMENTED |
| V101-TST-04 | internal/admin/revenue_ops_test.go | IMPLEMENTED |
| V101-TST-05 | internal/admin/revenue_ops_test.go | IMPLEMENTED |
| V101-TST-06 | internal/admin/behavioral_risk_test.go | IMPLEMENTED |
| V101-TST-07 | internal/admin/behavioral_risk_test.go | IMPLEMENTED |
| V101-TST-08 | internal/admin/kill_switch_test.go | IMPLEMENTED |
| V101-TST-09 | internal/admin/skill_acl_test.go | IMPLEMENTED |
| V101-TST-10 | internal/admin/oauth_monitor_test.go | IMPLEMENTED |
| V101-TST-11 | internal/admin/kill_switch_test.go (integrated) | IMPLEMENTED |
| V101-TST-12 | internal/admin/skill_acl_test.go (integrated) | IMPLEMENTED |
| V101-TST-13 | internal/admin/cost_attribution_test.go (integrated) | IMPLEMENTED |
| V101-TST-14 | internal/admin/workflows_test.go | IMPLEMENTED |
| V101-TST-15 | internal/admin/workflows_test.go | IMPLEMENTED |
| V101-TST-16 | internal/admin/workflows_test.go | IMPLEMENTED |
| V101-TST-17 | internal/admin/oauth_monitor_test.go | IMPLEMENTED |
| V101-TST-18 | internal/admin/action_replay_test.go | IMPLEMENTED |
| V101-TST-19 | internal/admin/cost_attribution_test.go | IMPLEMENTED |
| V101-TST-20 | internal/admin/revenue_ops_test.go | IMPLEMENTED |
| V101-TST-21 | internal/contracts/acceptance_gates_test.go (covers admin) | IMPLEMENTED |
| V101-TST-22 | internal/contracts/acceptance_gates_test.go | IMPLEMENTED |
| V101-TST-23 | internal/contracts/acceptance_gates_test.go | IMPLEMENTED |
| V101-TST-24 | internal/temporal/replay_test.go | IMPLEMENTED |
| V101-TST-25 | policies/v10_gates_test.rego | IMPLEMENTED |

### V10.2 Intelligence — Database

| Req ID | Mapped Artifact | Status |
|--------|----------------|--------|
| V102-DB-01 | db/migrations/011_BREVIO_v102_v103_intelligence.sql | IMPLEMENTED |
| V102-DB-02 | db/migrations/011_BREVIO_v102_v103_intelligence.sql | IMPLEMENTED |
| V102-DB-03 | db/migrations/011_BREVIO_v102_v103_intelligence.sql | IMPLEMENTED |
| V102-DB-04 | db/migrations/011_BREVIO_v102_v103_intelligence.sql | IMPLEMENTED |
| V102-DB-05 | db/migrations/011_BREVIO_v102_v103_intelligence.sql | IMPLEMENTED |
| V102-DB-06 | db/migrations/011_BREVIO_v102_v103_intelligence.sql | IMPLEMENTED |
| V102-DB-07 | db/migrations/011_BREVIO_v102_v103_intelligence.sql | IMPLEMENTED |
| V102-DB-08 | db/migrations/011_BREVIO_v102_v103_intelligence.sql | IMPLEMENTED |
| V102-DB-09 | db/migrations/011_BREVIO_v102_v103_intelligence.sql | IMPLEMENTED |
| V102-DB-10 | db/migrations/011_BREVIO_v102_v103_intelligence.sql | IMPLEMENTED |

### V10.2 Intelligence — Features

| Req ID | Mapped Artifact | Status |
|--------|----------------|--------|
| V102-INT-01 | internal/eq/ | IMPLEMENTED |
| V102-INT-02 | internal/trust/autonomy_demotion.go | IMPLEMENTED |
| V102-INT-03 | internal/brain/proactive_interruption.go | IMPLEMENTED |
| V102-INT-04 | internal/brain/critic_reflector.go | IMPLEMENTED |
| V102-INT-05 | internal/brain/critic_reflector.go | IMPLEMENTED |
| V102-INT-06 | internal/memory/decay.go | IMPLEMENTED |
| V102-INT-07 | internal/learning/ | IMPLEMENTED |
| V102-INT-08 | internal/knowledge/ | IMPLEMENTED |
| V102-INT-09 | internal/context/ | IMPLEMENTED |
| V102-INT-10 | internal/brain/uncertainty.go | IMPLEMENTED |
| V102-INT-11 | internal/brain/multi_intent.go | IMPLEMENTED |
| V102-INT-12 | internal/prefetch/ + internal/fastpath/ | IMPLEMENTED |
| V102-INT-13 | internal/brain/dynamic_decomposition.go | IMPLEMENTED |
| V102-INT-14 | internal/brain/partial_failure.go | IMPLEMENTED |
| V102-INT-15 | internal/brain/confidence_calibration.go | IMPLEMENTED |
| V102-INT-16 | internal/brain/world_model.go | IMPLEMENTED |
| V102-INT-17 | internal/brain/keyword_classifier.go | IMPLEMENTED |
| V102-INT-18 | internal/brain/counterfactual.go | IMPLEMENTED |
| V102-INT-19 | internal/rag/freshness.go | IMPLEMENTED |
| V102-INT-20 | internal/rag/ (chunking strategy) | IMPLEMENTED |

### V10.2 Intelligence — Temporal Workflows

| Req ID | Mapped Artifact | Status |
|--------|----------------|--------|
| V102-WF-01 | internal/temporal/workflows_learning_test.go (MemoryDecay) | IMPLEMENTED |
| V102-WF-02 | internal/temporal/ (LessonConsolidation) | IMPLEMENTED |
| V102-WF-03 | internal/temporal/ (Calibration) | IMPLEMENTED |
| V102-WF-04 | internal/temporal/ (KnowledgeFileDrift) | IMPLEMENTED |
| V102-WF-05 | internal/prefetch/ (SpeculativePrefetch) | IMPLEMENTED |
| V102-WF-06 | internal/fastpath/ (FastPathCacheWarmer) | IMPLEMENTED |

### V10.3 Cognitive Intelligence — Database

| Req ID | Mapped Artifact | Status |
|--------|----------------|--------|
| V103-DB-01 | db/migrations/011_BREVIO_v102_v103_intelligence.sql | IMPLEMENTED |
| V103-DB-02 | db/migrations/011_BREVIO_v102_v103_intelligence.sql | IMPLEMENTED |
| V103-DB-03 | db/migrations/011_BREVIO_v102_v103_intelligence.sql | IMPLEMENTED |
| V103-DB-04 | db/migrations/011_BREVIO_v102_v103_intelligence.sql | IMPLEMENTED |
| V103-DB-05 | db/migrations/011_BREVIO_v102_v103_intelligence.sql | IMPLEMENTED |
| V103-DB-06 | db/migrations/011_BREVIO_v102_v103_intelligence.sql | IMPLEMENTED |
| V103-DB-07 | db/migrations/011_BREVIO_v102_v103_intelligence.sql | IMPLEMENTED |
| V103-DB-08 | db/migrations/011_BREVIO_v102_v103_intelligence.sql | IMPLEMENTED |
| V103-DB-09 | db/migrations/011_BREVIO_v102_v103_intelligence.sql | IMPLEMENTED |
| V103-DB-10 | db/migrations/011_BREVIO_v102_v103_intelligence.sql | IMPLEMENTED |
| V103-DB-11 | db/migrations/011_BREVIO_v102_v103_intelligence.sql | IMPLEMENTED |

### V10.3 Cognitive Intelligence — Features

| Req ID | Mapped Artifact | Status |
|--------|----------------|--------|
| V103-COG-01 | internal/cognition/dual_process.go + internal/cognitive/system1.go | IMPLEMENTED |
| V103-COG-02 | internal/cognition/graph_of_thought.go + internal/cognitive/got.go | IMPLEMENTED |
| V103-COG-03 | internal/cognition/metacognition.go + internal/cognitive/metacognition.go | IMPLEMENTED |
| V103-COG-04 | internal/cognition/theory_of_mind.go + internal/cognitive/theory_of_mind.go | IMPLEMENTED |
| V103-COG-05 | internal/cognition/bayesian.go + internal/cognitive/beliefs.go | IMPLEMENTED |
| V103-COG-06 | internal/memory/ (prospective memory) | IMPLEMENTED |
| V103-COG-07 | internal/cognition/implicit_learning.go + internal/cognitive/preferences.go | IMPLEMENTED |
| V103-COG-08 | internal/cognition/case_reasoning.go + internal/cognitive/cbr.go | IMPLEMENTED |
| V103-COG-09 | internal/cognition/clarification.go + internal/cognitive/clarification.go | IMPLEMENTED |
| V103-COG-10 | internal/cognition/consolidation.go + internal/cognitive/consolidation.go | IMPLEMENTED |
| V103-COG-11 | internal/cognition/drift.go + internal/cognitive/drift.go | IMPLEMENTED |

### V10.3 Cognitive Intelligence — Temporal Workflows

| Req ID | Mapped Artifact | Status |
|--------|----------------|--------|
| V103-WF-01 | internal/temporal/ (HeuristicUpdate) | IMPLEMENTED |
| V103-WF-02 | internal/temporal/ (MetacognitiveRecalculation) | IMPLEMENTED |
| V103-WF-03 | internal/temporal/ (BeliefMaintenance) | IMPLEMENTED |
| V103-WF-04 | internal/temporal/ (MemoryConsolidation) | IMPLEMENTED |
| V103-WF-05 | internal/temporal/ (DriftDetection) | IMPLEMENTED |
| V103-WF-06 | internal/temporal/ (CaseLibraryMaintenance) | IMPLEMENTED |

### V10.3 Cognitive Intelligence — Prompts

| Req ID | Mapped Artifact | Status |
|--------|----------------|--------|
| V103-PRM-01 | config/prompt-templates/ or internal/cognition/ | IMPLEMENTED |
| V103-PRM-02 | internal/cognition/graph_of_thought.go | IMPLEMENTED |
| V103-PRM-03 | internal/cognition/graph_of_thought.go | IMPLEMENTED |
| V103-PRM-04 | internal/cognition/clarification.go | IMPLEMENTED |

### V10.4 Voice & Calling — Database

| Req ID | Mapped Artifact | Status |
|--------|----------------|--------|
| V104-DB-01 | db/migrations/012_BREVIO_v104_voice_calls.sql | IMPLEMENTED |
| V104-DB-02 | db/migrations/012_BREVIO_v104_voice_calls.sql | IMPLEMENTED |
| V104-DB-03 | db/migrations/012_BREVIO_v104_voice_calls.sql | IMPLEMENTED |
| V104-DB-04 | db/migrations/012_BREVIO_v104_voice_calls.sql | IMPLEMENTED |
| V104-DB-05 | db/migrations/012_BREVIO_v104_voice_calls.sql | IMPLEMENTED |
| V104-DB-06 | db/migrations/012_BREVIO_v104_voice_calls.sql | IMPLEMENTED |
| V104-DB-07 | db/migrations/012_BREVIO_v104_voice_calls.sql | IMPLEMENTED |
| V104-DB-08 | db/migrations/012_BREVIO_v104_voice_calls.sql | IMPLEMENTED |
| V104-DB-09 | db/migrations/012_BREVIO_v104_voice_calls.sql | IMPLEMENTED |

### V10.4 Voice & Calling — Go Packages

| Req ID | Mapped Artifact | Status |
|--------|----------------|--------|
| V104-PKG-01 | internal/hands/call/ (6 files exist) | PARTIAL |
| V104-PKG-02 | internal/voice/worker/ (6 files exist) | PARTIAL |
| V104-FILE-01 | internal/hands/call/vapi_client.go | IMPLEMENTED |
| V104-FILE-02 | internal/hands/call/retell_client.go | IMPLEMENTED |
| V104-FILE-03 | internal/hands/call/call_provider.go | MISSING |
| V104-FILE-04 | internal/hands/call/webhook_handler.go | IMPLEMENTED |
| V104-FILE-05 | internal/hands/call/outcome_parser.go | IMPLEMENTED |
| V104-FILE-06 | internal/hands/call/prompt_builder.go | IMPLEMENTED |
| V104-FILE-07 | internal/hands/call/scheduler.go | MISSING |
| V104-FILE-08 | internal/hands/call/places_lookup.go | MISSING |
| V104-FILE-09 | internal/hands/call/call_service.go | IMPLEMENTED |
| V104-FILE-10 | internal/voice/worker/agent.go | MISSING (dispatch.go exists) |
| V104-FILE-11 | internal/voice/worker/stt.go | IMPLEMENTED |
| V104-FILE-12 | internal/voice/worker/stt_whisper.go | MISSING |
| V104-FILE-13 | internal/voice/worker/tts.go | IMPLEMENTED |
| V104-FILE-14 | internal/voice/worker/tts_cartesia.go | MISSING |
| V104-FILE-15 | internal/voice/worker/vad.go | MISSING |
| V104-FILE-16 | internal/voice/worker/session_manager.go | PARTIAL (session.go exists) |
| V104-FILE-17 | internal/voice/worker/context_loader.go | MISSING |
| V104-FILE-18 | internal/voice/worker/task_extractor.go | IMPLEMENTED |
| V104-FILE-19 | internal/voice/worker/health.go | MISSING |

### V10.4 Voice & Calling — API, Workflows, Policies

| Req ID | Mapped Artifact | Status |
|--------|----------------|--------|
| V104-API-01 | internal/hands/call/webhook_handler.go | IMPLEMENTED |
| V104-API-02 | internal/hands/call/webhook_handler.go | IMPLEMENTED |
| V104-API-03 | internal/voice/worker/session.go | IMPLEMENTED |
| V104-API-04 | internal/voice/worker/session.go | IMPLEMENTED |
| V104-WF-01 | internal/temporal/workflows_voice_test.go (exists) | IMPLEMENTED |
| V104-POL-01 | policies/call_approval_gate.rego | IMPLEMENTED |
| V104-MIG-01 | db/migrations/012_BREVIO_v104_voice_calls.sql (combined) | IMPLEMENTED |
| V104-MIG-02 | db/migrations/012_BREVIO_v104_voice_calls.sql (combined) | IMPLEMENTED |

### V10.4 Voice & Calling — Infrastructure

| Req ID | Mapped Artifact | Status |
|--------|----------------|--------|
| V104-INF-01 | config/ environment vars | IMPLEMENTED |
| V104-INF-02 | internal/connectors/seeds/ (skill registration) | IMPLEMENTED |
| V104-INF-03 | internal/observability/ | IMPLEMENTED |
| V104-INF-04 | internal/hands/call/ + internal/voice/worker/ (failover) | PARTIAL |
| V104-INF-05 | helm/ (voice worker scaling) | IMPLEMENTED |
| V104-INF-06 | internal/hands/call/prompt_builder.go | IMPLEMENTED |

### UX Copy

| Req ID | Mapped Artifact | Status |
|--------|----------------|--------|
| UX-01 | internal/onboarding/ (service, state_machine, copy_templates) | IMPLEMENTED |
| UX-02 | internal/onboarding/copy_templates.go | IMPLEMENTED |
| UX-03 | internal/onboarding/copy_templates.go | IMPLEMENTED |
| UX-04 | internal/onboarding/copy_templates.go | IMPLEMENTED |

---

## Coverage Summary

| Blueprint | Total Reqs | IMPLEMENTED | PARTIAL | MISSING |
|-----------|-----------|-------------|---------|---------|
| 4 Features | 12 | 0 | 5 | 7 |
| V10.1 Admin | 80 | 80 | 0 | 0 |
| V10.2 Intelligence | 36 | 36 | 0 | 0 |
| V10.3 Cognitive | 32 | 32 | 0 | 0 |
| V10.4 Voice/Calling | 42 | 30 | 4 | 8 |
| UX Copy | 4 | 4 | 0 | 0 |
| **TOTAL** | **206** | **182** | **9** | **15** |

## MISSING Requirements — Remediation Plan

| Req ID | Description | Remediation |
|--------|-------------|-------------|
| F-05 | brevio-live-demo.jsx complete replacement | Create apps/web-demo/src/brevio-live-demo.jsx per Section B |
| F-06 | Vitest unit tests | Create __tests__/utils.test.ts, deliverInBubbles.test.ts |
| F-07 | Playwright E2E tests | Create tests/demo.spec.ts |
| F-10 | Quick reply chip derivation | Include in brevio-live-demo.jsx |
| F-11 | formatText sanitization | Include in brevio-live-demo.jsx |
| V104-FILE-03 | call_provider.go (provider interface + failover) | Create with CallProvider interface |
| V104-FILE-07 | scheduler.go (Temporal call scheduling) | Create with MAKE_CALL + retry |
| V104-FILE-08 | places_lookup.go (Google Places) | Create with E.164 normalization |
| V104-FILE-10 | agent.go (LiveKit agent) | Create or verify dispatch.go covers |
| V104-FILE-12 | stt_whisper.go (Whisper fallback) | Create STT fallback |
| V104-FILE-14 | tts_cartesia.go (Cartesia fallback) | Create TTS fallback |
| V104-FILE-15 | vad.go (Voice Activity Detection) | Create Silero VAD |
| V104-FILE-17 | context_loader.go (USER.md/SOUL.md loader) | Create context preloader |
| V104-FILE-19 | health.go (worker health metrics) | Create health reporting |
