# Prompt Task Checklist — Prompt Guard Layer

> All tasks extracted from Prompts A–G are tracked here.
> A prompt is complete only when every task from that prompt is COMPLETE.

## Prompt A — Discovery + Harness + Gates

| Task ID | Description | Source | Status |
|---------|-------------|--------|--------|
| A-1 | Identify Makefile targets and CI expectations | Task A | COMPLETE |
| A-2 | Create `make local-verify` command (vet + build + test) | Task A | COMPLETE |
| A-3 | Fix stale test failures (`internal/workflows`, `tests/contract`) | Task A (prerequisite) | COMPLETE |
| A-4 | Build runtime map: Temporal worker registrations | Task B | COMPLETE |
| A-5 | Build runtime map: main message processing workflow path | Task B | COMPLETE |
| A-6 | Build runtime map: stubbed activities inventory | Task B | COMPLETE |
| A-7 | Create Definition of Done gates (5 gates) | Task C | COMPLETE |
| A-8 | Create integration test skeleton — plan output schema | Task D | COMPLETE |
| A-9 | Create integration test skeleton — execute request format | Task D | COMPLETE |
| A-10 | Create integration test skeleton — verify output schema | Task D | COMPLETE |
| A-11 | Create integration test skeleton — tool registry contract | Task D | COMPLETE |
| A-12 | Scaffold FakeLLMService + FakeToolServer interfaces | Task D | COMPLETE |
| A-13 | Integration smoke test compiles and runs | Task D (acceptance) | COMPLETE |
| A-14 | `make local-verify` runs successfully | Task A (acceptance) | COMPLETE |

## Prompt B — Real LLM Layer + Verify Critic

| Task ID | Description | Source | Status |
|---------|-------------|--------|--------|
| B-1 | Production-grade Anthropic Messages API client with retries, request IDs, error classification | Task 1 | COMPLETE |
| B-2 | Production-grade OpenAI Responses API client with structured output (json_schema, strict=true) | Task 1 | COMPLETE |
| B-3 | FailoverClient with retryable error detection (429, 5xx, timeout) | Task 1 | COMPLETE |
| B-4 | Canonical plan schema: plan_id, risk_level, steps[] with tool_key/parameters/phase/depends_on | Task 2 | COMPLETE |
| B-5 | final_answer_requirements field in plan schema for verify step | Task 2 | COMPLETE |
| B-6 | ValidateStrictPlanJSON with non-retryable errors for structural failures | Task 2 | COMPLETE |
| B-7 | Replace stubbed ClassifyIntentActivity with real LLM + keyword fallback | Task 3 | COMPLETE |
| B-8 | Replace stubbed GeneratePlanActivity with real LLM + deterministic fallback | Task 3 | COMPLETE |
| B-9 | VerifyExecutionActivity: LLM-based critic with pass/fail + retry_hints | Task 4 | COMPLETE |
| B-10 | Verify→replan loop in MessageProcessingWorkflow (max 2 iterations) | Task 4 | COMPLETE |
| B-11 | VerifyExecutionActivity registered in Temporal worker | Task 4 | COMPLETE |
| B-12 | httptest unit tests for Anthropic client (success, 429, 500, 400, timeout) | Task 5 | COMPLETE |
| B-13 | httptest unit tests for OpenAI client (success, 429, 500, structured output, role rewrite) | Task 5 | COMPLETE |
| B-14 | FailoverClient unit tests (primary success, failover on 429/500, non-retryable, both fail) | Task 5 | COMPLETE |
| B-15 | Strict plan JSON validation tests (valid, empty intent, no actions, >8 actions, bad key, garbage) | Task 5 | COMPLETE |
| B-16 | Activity tests: deterministic fallback has steps, verify pass/fail, classify intents | Task 5 | COMPLETE |
| B-17 | End-to-end intelligence tests with httptest (classify, plan, verify pass/fail) | Task 5 | COMPLETE |
| B-18 | `make local-verify` passes (excluding pre-existing failures) | Acceptance | COMPLETE |

## Prompt C — Redis-Backed Runtime Guarantees

| Task ID | Description | Source | Status |
|---------|-------------|--------|--------|
| C-1 | Add go-redis/v9 and miniredis/v2 to go.mod | Task 1 | COMPLETE |
| C-2 | Create internal/cache/redis.go with Get/Set/Incr/Ping, replay cache, rate limiting, feature flags | Task 2 | COMPLETE |
| C-3 | Replace in-memory replay map in service.go with Redis (key: `replay:{hash}`, TTL configurable, default 24h) | Task 3 | COMPLETE |
| C-4 | Service checks Redis first, falls back to in-memory on miss/error, stores in both | Task 3 | COMPLETE |
| C-5 | RateLimitedClient using Redis fixed-window counters (`rl:llm:{provider}:{workspace}:req:{window}`) | Task 4 | COMPLETE |
| C-6 | Token rate limiting via IncrBy (`rl:llm:{provider}:{workspace}:tok:{window}`) | Task 4 | COMPLETE |
| C-7 | Typed RateLimitError for Temporal retryable classification | Task 4 | COMPLETE |
| C-8 | Feature flags Service.SetRedisCache with Redis as source of truth (`ff:{key}`) | Task 5 | COMPLETE |
| C-9 | Feature flags local miss falls through to Redis, backfills local cache | Task 5 | COMPLETE |
| C-10 | Deep health RedisPinger interface for protocol-level PING/PONG validation | Task 6 | COMPLETE |
| C-11 | Deep health uses RedisPinger when set, skips TCP dial for Redis | Task 6 | COMPLETE |
| C-12 | Unit tests: URL parsing (valid, empty, invalid) | Tests | COMPLETE |
| C-13 | Unit tests: replay cache hit/miss/TTL expiry/prevents second call | Tests | COMPLETE |
| C-14 | Unit tests: rate limiting (requests/min, tokens/min, workspace isolation, concurrency safety, window expiry) | Tests | COMPLETE |
| C-15 | Integration tests: Redis replay cross-instance idempotency, in-memory fallback on Redis down | Tests | COMPLETE |
| C-16 | Deep health PING tests (success and failure) | Tests | COMPLETE |
| C-17 | `make local-verify` passes (excluding pre-existing failures) | Acceptance | COMPLETE |

## Prompt D — Tool Registry Powers Planning + Execution

| Task ID | Description | Source | Status |
|---------|-------------|--------|--------|
| D-1 | `ConnectorRegistryRepository` with `UpsertConnector`, `UpsertTool`, `ListAllTools` using pgx/v5 | Task 1 | COMPLETE |
| D-2 | `SeedToRepository` method on Service — upserts in-memory seed into DB repo | Task 1 | COMPLETE |
| D-3 | Deterministic seed loader: `parseSeed` YAML/JSON, validation (key format, risk_level, data_class, autonomy_floor) | Task 2 | COMPLETE |
| D-4 | `brevioctl seed tools` CLI command — loads connectors.yaml and upserts into PostgreSQL | Task 2 | COMPLETE |
| D-5 | `ListAllTools()`, `ToolKeys()`, `HasTool()` methods on in-memory Service | Task 3 | COMPLETE |
| D-6 | `GET /v1/tools` API endpoint returning full tool inventory with connector/tool counts | Task 3 | COMPLETE |
| D-7 | `RegisterRoutes(mux)` on connectors.Service for HTTP endpoint registration | Task 3 | COMPLETE |
| D-8 | `ToolRegistry` interface in llm package (`ToolKeys()`, `HasTool()`) | Task 4 | COMPLETE |
| D-9 | Planner prompt injects available tools list from registry | Task 4 | COMPLETE |
| D-10 | Planner validates all planned `tool_key` values against registry; rejects unknown tools | Task 4 | COMPLETE |
| D-11 | Backward-compatible: no registry = no validation (existing tests unaffected) | Task 4 | COMPLETE |
| D-12 | Tests: YAML parsing (valid, invalid YAML, invalid tool_key, unknown connector) | Tests | COMPLETE |
| D-13 | Tests: `ListAllTools` sorted order, `ToolKeys`, `HasTool` positive/negative | Tests | COMPLETE |
| D-14 | Tests: `GET /v1/tools` handler (seeded + empty registry) | Tests | COMPLETE |
| D-15 | Tests: `SeedToRepository` with mock repo (61 connectors, 64 tools) | Tests | COMPLETE |
| D-16 | Tests: Planner refuses unknown tools, accepts known tools, skips validation without registry | Tests | COMPLETE |
| D-17 | `make local-verify` passes (excluding pre-existing failures) | Acceptance | COMPLETE |

## Prompt E — Hands Runtime + Temporal Wiring

| Task ID | Description | Source | Status |
|---------|-------------|--------|--------|
| E-1 | Go hands runtime `Service` with skill registry synced from connectors, `ListSkills`, `GetSchema`, `Execute` | Task 1 | COMPLETE |
| E-2 | `MCPClient` interface + `HTTPMCPClient` production implementation with timeout, error classification | Task 2 | COMPLETE |
| E-3 | `FakeMCPClient` test double with configurable responses and call recording | Task 2 | COMPLETE |
| E-4 | HTTP endpoints: `GET /v1/skills`, `GET /v1/skills/{id}/schema`, `POST /v1/skills/{id}/execute` | Task 1 | COMPLETE |
| E-5 | Health endpoints: `GET /healthz/live`, `GET /healthz/ready` with skill count | Task 1 | COMPLETE |
| E-6 | `cmd/hands/main.go` HTTP server loading connectors from seed file | Task 1 | COMPLETE |
| E-7 | `ExecutorAdapter` bridging `hands.Service` to Temporal `HandsExecutor` interface | Task 3 | COMPLETE |
| E-8 | `HandsExecutor` interface in temporal package with `ExecuteTool` method | Task 3 | COMPLETE |
| E-9 | `ExecuteToolActivity` wired to call Go hands runtime (replaces stub) | Task 3 | COMPLETE |
| E-10 | `ToolOutput` field added to `ToolExecutionActivityResult` for structured output to verifier | Task 4 | COMPLETE |
| E-11 | Graceful fallback: stub execution when `handsExecutor` is nil | Task 3 | COMPLETE |
| E-12 | Receipt enforcement and kill switch checks preserved in activity | Task 3 | COMPLETE |
| E-13 | Service matrix contract tests updated for `cmd/hands` | Task 5 | COMPLETE |
| E-14 | Unit tests: ListSkills, GetSchema (found/not found), Execute (success, not found, missing receipt, MCP error) | Task 6 | COMPLETE |
| E-15 | Unit tests: ExecutorAdapter (success/failure), FakeMCPClient (default/custom error) | Task 6 | COMPLETE |
| E-16 | HTTP handler tests: list skills, get schema, execute, liveness, readiness | Task 6 | COMPLETE |
| E-17 | Integration test: seeded registry → execute → MCP fake → structured output | Task 6 | COMPLETE |
| E-18 | `make local-verify` passes (excluding pre-existing failures) | Acceptance | COMPLETE |

## Prompt F — Policy Enforcement + No-Stubs E2E Gate + Operational Polish

| Task ID | Description | Source | Status |
|---------|-------------|--------|--------|
| F-1 | Fix env var mismatch: `OPA_ENDPOINT` → `OPA_URL` in `config/registry.go` | Task 1 | COMPLETE |
| F-2 | Add `OPAEvaluator` to `MuxDependencies` and wire in `cmd/control/main.go` | Task 1 | COMPLETE |
| F-3 | Wrap `/v1/` routes with `OPAPolicyMiddleware`, exempt health/docs endpoints | Task 1 | COMPLETE |
| F-4 | Deny-by-default when OPA configured but unreachable (NNR-107) | Task 1 | COMPLETE |
| F-5 | Test: middleware allows when policy allows (embedded gates, read request) | Task 1 | COMPLETE |
| F-6 | Test: middleware denies when OPA unreachable and configured | Task 1 | COMPLETE |
| F-7 | Test: nil evaluator passes through (no policy enforcement) | Task 1 | COMPLETE |
| F-8 | Test: health endpoints exempt from OPA enforcement (mux integration) | Task 1 | COMPLETE |
| F-9 | Test: `/v1/` routes denied when OPA unreachable (mux integration) | Task 1 | COMPLETE |
| F-10 | No-stubs E2E test: seed registry from connectors.yaml | Task 2 | COMPLETE |
| F-11 | No-stubs E2E test: fake LLM server (httptest) for classify/plan/verify | Task 2 | COMPLETE |
| F-12 | No-stubs E2E test: spy hands server records tool execution calls | Task 2 | COMPLETE |
| F-13 | No-stubs E2E test: full chain classify→plan→authorize→execute→verify | Task 2 | COMPLETE |
| F-14 | No-stubs E2E test: fails if planning returns deterministic (LLM not called) | Task 2 | COMPLETE |
| F-15 | No-stubs E2E test: fails if hands spy receives zero calls (execution stubbed) | Task 2 | COMPLETE |
| F-16 | No-stubs E2E test: fails if verify returns empty verdict (verification skipped) | Task 2 | COMPLETE |
| F-17 | RUNBOOK.md: required env vars table (core, LLM, OPA, hands, messaging, observability) | Task 3 | COMPLETE |
| F-18 | RUNBOOK.md: local development quick start (docker compose, migrate, seed, verify) | Task 3 | COMPLETE |
| F-19 | RUNBOOK.md: troubleshooting (replay cache, rate limiting, OPA enforcement) | Task 3 | COMPLETE |
| F-20 | `make local-verify` passes (excluding pre-existing failures) | Acceptance | COMPLETE |
