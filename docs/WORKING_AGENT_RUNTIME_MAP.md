# Working Agent Runtime Map

> Repo-grounded evidence of how the plan→execute→verify pipeline works.

## 1. Temporal Worker — Activity & Workflow Registration

**Worker construction:** `internal/temporal/worker.go:20` — `NewWorkerWithDeps()`

**Task queue:** `brevio-core` (`internal/temporal/client.go:19`)

**Entrypoint:** `cmd/temporal-worker/main.go:123` calls `NewWorkerWithDeps(temporalClient, TaskQueueCore, deps)`

### Core Message-Processing Workflows

| Workflow | File | Line |
|----------|------|------|
| `MessageProcessingWorkflow` | `internal/temporal/workflows.go` | 44 |
| `OutboxDispatchWorkflow` | `internal/temporal/workflows.go` | registered L36 |
| `ToolHealthEvaluationWorkflow` | `internal/temporal/workflows.go` | registered L37 |
| `OnboardingWorkflow` | `internal/temporal/workflows.go` | registered L38 |

### Core Activities (Plan→Execute→Verify Path)

| Activity | File | Line | Role |
|----------|------|------|------|
| `ValidateEnvelopeActivity` | `internal/temporal/activities.go` | 334 | **Validate** inbound message |
| `ClassifyIntentActivity` | `internal/temporal/activities.go` | 347 | **Classify** intent (LLM or keyword fallback) |
| `RetrieveMemoryActivity` | `internal/temporal/activities.go` | registered L83 | **Context** — fetch memory items |
| `SearchRAGActivity` | `internal/temporal/activities.go` | registered L84 | **Context** — RAG chunk retrieval |
| `ExecuteReasoningLoopActivity` | `internal/temporal/activities.go` | registered L85 | **Plan** — multi-step reasoning loop |
| `AssessCognitiveStateActivity` | `internal/temporal/activities.go` | registered L88 | **Assess** — cognitive load gate |
| `EvaluateCouncilActivity` | `internal/temporal/activities.go` | registered L86 | **Verify/Gate** — council evaluation |
| `GeneratePlanActivity` | `internal/temporal/activities.go` | 413 | **Plan** — generate tool execution plan |
| `AuthorizePlanActivity` | `internal/temporal/activities.go` | 447 | **Gate** — authorize plan + issue receipt |
| `ExecuteToolActivity` | `internal/temporal/activities.go` | 482 | **Execute** — run tool with receipt |
| `SynthesizeResponseActivity` | `internal/temporal/activities.go` | 505 | **Synthesize** — generate user response |
| `EnqueueOutboxActivity` | `internal/temporal/activities.go` | registered L87 | **Dispatch** — outbox event |

### Additional Registered Workflows (27 total)

Registered at `internal/temporal/worker.go:35–186`:
- V9.1 soft intelligence (8 workflows from `internal/workflows/`): TrustScoring, GoalProgress, LearningConsolidation, DailyIntrospection, DailyLogCapture, CrossRepoAnalysis, MissionControlRefresh, CapabilityExploration
- P8 feature closures (8 workflows): FederationNegotiation, EdgeOfflineSync, BrowserAutomation, FastPathPipeline, ExperimentAssignment, OnboardingProvisioning, BillingEnforcement, LoadSheddingTier
- V10.1 cost/revenue (1): SubscriptionReconciliation
- V10.2 intelligence (3): IntelligenceProcessing, AutonomyDemotion, MemoryContextMaintenance
- V10.3 cognitive (5): NightlyConsolidation, WeeklyDriftDetection, HeuristicUpdate, BeliefMaintenance, CognitiveSignalProcessing
- V10.4 voice (2): OutboundCall, CallWebhookProcessing

### Total Registered Activities: 99

Registered at `internal/temporal/worker.go:69–218`.

---

## 2. Main Message Processing Path

**Entry:** `cmd/brain/main.go:89` — `POST /v1/brain/ingest`

**Flow:**
1. Gateway decodes `MessageEnvelope` (`internal/gateway/service.go`)
2. Brain starts `MessageProcessingWorkflow` on task queue `brevio-main` (note: brain uses `"brevio-main"` at `cmd/brain/main.go:125`, worker listens on `"brevio-core"` — this is a **configuration mismatch** that must be resolved for end-to-end flow)
3. Workflow executes 11 sequential steps — see `internal/temporal/workflows.go:44–333`:

```
ValidateEnvelope → ClassifyIntent → RetrieveMemory → SearchRAG
    → ExecuteReasoningLoop → AssessCognitiveState → EvaluateCouncil
    → AuthorizePlan → ExecuteTool(s) → SynthesizeResponse → EnqueueOutbox
```

**Plan step:** `ExecuteReasoningLoopActivity` (L130–148) produces `PlanID`, `ToolKeys[]`, `QualityScore`, `RiskLevel`

**Execute step:** `ExecuteToolActivity` (L231–276) iterates `ToolKeys`, runs simulate→commit per tool

**Verify step:** `EvaluateCouncilActivity` (L165–182) evaluates risk; `AssessCognitiveStateActivity` (L150–163) can abort; `AuthorizePlanActivity` (L198–229) gates execution with receipt

---

## 3. Currently Stubbed Activities (Must Become Real for Plan→Execute→Verify)

| Activity | Current State | File:Line | What It Does Now |
|----------|--------------|-----------|-----------------|
| `ClassifyIntentActivity` | **Partial** — uses LLM when `IntelligenceService` is set, falls back to keyword matching | `activities.go:347` | Keyword fallback returns `intent: "general_query"` |
| `GeneratePlanActivity` | **Partial** — uses LLM when available, falls back to single-tool echo plan | `activities.go:413` | Fallback returns `PlanAction{tool_key: "echo", phase: "act"}` |
| `ExecuteReasoningLoopActivity` | **Stubbed** — returns hardcoded plan with quality score 0.85 | `activities.go:L85` (registered) | No real reasoning loop |
| `ExecuteToolActivity` | **Stubbed** — validates receipt, computes hash, returns success without actually executing | `activities.go:482` | No real connector/tool invocation |
| `SynthesizeResponseActivity` | **Partial** — uses LLM when available, falls back to template | `activities.go:505` | Template: "Processed message {id} with {n} tool results" |
| `RetrieveMemoryActivity` | **Stubbed** — returns empty items or mock data | `activities.go:L83` | No real memory store query |
| `SearchRAGActivity` | **Stubbed** — returns empty chunks or mock data | `activities.go:L84` | No real RAG search |
| `AssessCognitiveStateActivity` | **Stubbed** — always returns "proceed" | `activities.go:L88` | No real cognitive assessment |
| `EvaluateCouncilActivity` | **Stubbed** — returns "proceed" with convened=false | `activities.go:L86` | No real multi-agent council |

---

## 4. LLM Intelligence Layer

**Service:** `internal/llm/service.go:46` — `Service` struct

**Intelligence:** `internal/llm/intelligence.go:51` — `IntelligenceService` struct with three clients (classifier, planner, synthesizer)

**Bootstrap:** `internal/llm/bootstrap.go:109` — `BootstrapService()` wires intelligence from `ANTHROPIC_API_KEY` / `OPENAI_API_KEY` env vars

**Key methods on `IntelligenceService`:**
- `ClassifyIntent()` — `intelligence.go:74` (T0 tier: Haiku/GPT-4o-mini)
- `GeneratePlan()` — `intelligence.go:130` (T2 tier: Sonnet/GPT-4o), returns `GeneratedPlan` with `PlanAction[]`
- `SynthesizeResponse()` — `intelligence.go:207` (T2 tier)

**Plan schema:** `PlanAction` has `ToolKey`, `Operation`, `Parameters map[string]any`, `Phase` (gather|act|verify) — defined at `intelligence.go:24–29`

---

## 5. Tool Registry

**Primary:** `internal/executor/connectors/registry.go:16` — `Registry` struct with `Upsert()`, `Get()`, `Keys()` methods

**MCP Tools:** `internal/mcp/service.go:28` — `ToolSpec` struct with `ToolKey`, `Source`, `ServerID`, `ConnectorKey`, `AuthType`, `RiskLevel`

**Planner Catalog:** `internal/connectors/tool_resolution.go:28` — `PlannerToolCatalog()` returns sorted `ToolInventoryItem[]`

**Integration Layer:** `internal/integration/service.go:517` — `MCPToolRegistry()` returns all registered `ToolSpec`s (23+ default tools seeded at L92–117)

---

## 6. Task Queue Mismatch

| Component | Task Queue Used | Source |
|-----------|----------------|--------|
| `cmd/brain/main.go` | `"brevio-main"` | L125 |
| `cmd/temporal-worker/main.go` | `breviotemporal.TaskQueueCore` = `"brevio-core"` | L123 |

**Impact:** Workflows started by brain will not be picked up by the temporal-worker unless the task queue is aligned. This must be resolved.
