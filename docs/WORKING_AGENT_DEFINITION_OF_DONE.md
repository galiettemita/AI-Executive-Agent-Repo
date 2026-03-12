# Working Agent — Definition of Done

> Non-negotiable gates that must pass before the agent system is considered "working".
> Each gate references the production code path it validates.

---

## Gate 1: LLM Planning Returns a Non-Empty Plan

**Requirement:** Given a user message, `GeneratePlan()` must return a `GeneratedPlan` containing at least one `PlanAction` with a valid `ToolKey`, `Operation`, and `Phase`.

**Production path:**
- `internal/llm/intelligence.go:130` — `IntelligenceService.GeneratePlan()`
- `internal/llm/intelligence.go:250` — `validatePlan()` enforces non-empty actions, valid risk levels, tool_key format
- `internal/llm/intelligence.go:287` — `canonicalizePlan()` enforces gather→act→verify ordering

**Verification:**
- `PlanAction` list is non-empty
- Each action has `ToolKey != ""`
- Each action has `Phase` in `{"gather", "act", "verify"}`
- `GeneratedPlan.Tools` is non-empty and sorted

**Test coverage required:** Unit test with fake LLM client returning structured JSON plan; validate schema compliance.

---

## Gate 2: At Least One Tool Executes Through Production Path

**Requirement:** `ExecuteToolActivity` must execute a tool through the non-quarantined runtime, passing receipt authorization, and returning a structured `ToolExecutionActivityResult`.

**Production path:**
- `internal/temporal/activities.go:482` — `ExecuteToolActivity()` validates receipt, checks kill switch
- `internal/executor/prod_service.go:38` — `ProdService.Simulate()` validates SSRF, records execution
- `internal/executor/prod_service.go:57` — `ProdService.Commit()` validates receipt → executes → increments side effects → creates trust receipt

**Verification:**
- `ReceiptID` is non-empty (authorization enforced)
- Kill switch is not active
- `ToolExecutionActivityResult.Success == true`
- `ToolExecutionActivityResult.Phase == "commit"`
- Idempotency key is deterministic

**Test coverage required:** Integration test that wires `ProdService` with in-memory repo, calls Simulate→Commit, asserts receipt creation.

---

## Gate 3: Verify Step Can Reject or Trigger Retry

**Requirement:** The pipeline must include a verification gate that can reject a plan or trigger re-planning when tool output is insufficient.

**Production path:**
- `internal/temporal/workflows.go:150–163` — `AssessCognitiveStateActivity` returns `Strategy: "proceed"` or `"abort"`
- `internal/temporal/workflows.go:165–182` — `EvaluateCouncilActivity` returns `Decision` that can block execution
- `internal/temporal/workflows.go:198–229` — `AuthorizePlanActivity` returns `Decision: "deny"` to block

**Verification:**
- When `AssessCognitiveStateActivity` returns `"abort"`, workflow terminates with `FAILED` state
- When `EvaluateCouncilActivity` returns deny, execution is skipped
- When `AuthorizePlanActivity` returns `"deny"`, tools are not executed
- Workflow result includes terminal state (`COMPLETED`, `FAILED`, or `DEAD_LETTER`)

**Test coverage required:** Unit test with mock activities returning abort/deny; assert workflow skips execution.

---

## Gate 4: Tool Registry Is Non-Empty at Runtime

**Requirement:** The tool registry must contain at least one tool definition, accessible either via API endpoint or library call used by the planner.

**Production path:**
- `internal/mcp/service.go:106` — `Service.RegisterTool()` populates registry
- `internal/mcp/service.go` — `Service.ListTools()` returns all `ToolSpec`s
- `internal/integration/service.go:517` — `MCPToolRegistry()` exposes registered tools
- `internal/integration/service.go:92–117` — 23+ default tools seeded on construction
- `internal/connectors/tool_resolution.go:28` — `PlannerToolCatalog()` returns sorted inventory

**Verification:**
- `len(ListTools()) > 0`
- Each tool has `ToolKey != ""`, `Source` in `{"native", "mcp"}`, `RiskLevel` in `{"LOW", "MEDIUM", "ELEVATED", "CRITICAL"}`
- `PlannerToolCatalog()` returns items sorted by `ToolKey`

**Test coverage required:** Unit test constructing integration service, verifying default tools are present and well-formed.

---

## Gate 5: All Gates Covered by Automated Tests

**Requirement:** Gates 1–4 must each have at least one automated test that runs as part of `make local-verify` (i.e., `go test ./... -count=1`).

**Test locations (target):**
- Gate 1: `tests/integration/working_agent_smoke_test.go` — plan output schema
- Gate 2: `tests/integration/working_agent_smoke_test.go` — execute request format
- Gate 3: `tests/integration/working_agent_smoke_test.go` — verify output schema
- Gate 4: `internal/integration/service_test.go` or `tests/integration/working_agent_smoke_test.go`

**Verification:**
- `make local-verify` exits 0
- All gate tests are enumerable via `go test -list "TestWorkingAgent" ./tests/integration/`
