# Future Architecture Notes

Living index of concepts and patterns from modules archived during the FOMO v0.1 cleanup. Each section is one archived module (or one conceptual area). The active codebase stays small for the FOMO wedge; the *ideas* needed for the long-term Poke-like AI personal assistant are preserved here so they're not lost.

Companion to:
- [FOMO_DESIGN.md](../FOMO_DESIGN.md) — product vision, the L1-L8 trust ladder, "Future Agent Intelligence Requirement"
- [FOMO_PLAN.md](../FOMO_PLAN.md) — implementation plan with "Long-Term Assistant Preservation Rule"
- [SALVAGE_MAP.md](../SALVAGE_MAP.md) — original module inventory

## How to use this doc

**When designing a new layer** (L2 Calendar → L3 Drafting → L4 Sending → L5 Calendar create → L6 MCP tools → L7 Autonomous → L8 Memory), read every section here whose "Layer" line includes that layer. Each section tells you:

1. What concept was already attempted in this codebase
2. Why the prior implementation was deleted
3. What from that implementation is worth keeping when you re-encounter the same problem
4. Where to recover the original code with `git show`

**When archiving a new module**, add a section using the template at the bottom of this doc. Order entries by L-layer (L1 first, L7+ last), then alphabetically.

**Recovery commands** (every archived file is retrievable):
```bash
# See the file as it was when archived:
git show <archive-commit-sha>:<original-path>

# Restore the file to the working tree (do not commit unless intentional):
git checkout <archive-commit-sha> -- <original-path>

# Find when a file was last in HEAD:
git log --all --diff-filter=D --summary -- <original-path>
```

The `archive-commit-sha` for the initial FOMO cleanup is the SHA of commit 4 in [FOMO_PLAN.md](../FOMO_PLAN.md) §RC3 (filled in when that commit lands).

---

## Agent Orchestration

**Archived from:** `services/brevio-brain/src/planner.ts` (611 lines)
**Layer:** L6 (MCP/tool integrations), L7 (autonomous follow-throughs)
**Maps to "Future Agent Intelligence Requirement" component:** Agent Orchestration

### Concept

`planner.ts` built multi-step execution plans for an agent. Given a user intent and a set of available skills, it produced an ordered set of actions ("call gmail.search → call calendar.read → ask user → call flights.search → ..."). It supported parallel fanout (multiple skills running concurrently), reconciliation (combining their results), and optional LLM augmentation — calling a separate LLM to validate or rewrite the plan before execution.

This is the core of what the Future Agent Intelligence Requirement calls **Agent Orchestration**: the layer that turns a user request like *"book me a flight to Boston Thursday that doesn't conflict with my morning meeting"* into an ordered tool-call plan.

### What was in the file (prose summary, line ranges)

- **buildPlannerProposal** (the main entry point): orchestrated the 4-stage pipeline `classify → decompose → disambiguate → produce actions`. Took user intent, returned a `PlannerProposal` with ordered actions, fanout groups, and confidence.
- **augmentWithOpenAI** (lines 381-483 — the most interesting part): took the deterministic plan and called an external LLM to second-guess it. The LLM could rewrite the plan, add missing steps, flag unsafe steps, or reject the whole thing. Returned an annotated plan with `llm_confidence` and `llm_rewrites`.
- **Reconciliation step generation** (lines 323-349): when multiple skills ran in parallel, built a reconciliation action that synthesized their results into a single response.
- **Fanout group IDs**: tracking multiple parallel branches so the runtime could merge their results.

### Original-implementation critique

- **611 lines for what FOMO needs is wrong.** FOMO is a binary decision (alert or skip). The 4-stage pipeline is wasted scaffolding for that.
- **Built for too-broad a product surface.** Designed to plan across 10+ skill categories (music, flights, shopping, notes, etc.). The trust ladder commits us to one skill at a time, not ten at once.
- **Tightly coupled to `disambiguate.ts`, `decompose.ts`, `aggregate.ts`.** Reuse requires those three siblings too.
- **The `augmentWithOpenAI` pattern was the gold.** Using an LLM to validate a deterministic plan (rather than to generate one) is the right shape for safety: the LLM is a checker, not a generator. This pattern survives.

### Future-implementation guidance

When L6 starts (the first MCP tool integration beyond FOMO), do not restore this file. Build fresh with:

1. **Native LLM tool-calling first.** OpenAI / Anthropic native tool calling does 80% of what `disambiguate + planner` did, in 0 lines of orchestration code. Use it.
2. **Add `augmentWithOpenAI`-style validation only for risky plans.** Specifically: any plan that includes a tier ≥ "send" or "spend" action should be re-validated by a second LLM call before execution. This pattern is what makes the future Approval Engine load-bearing.
3. **Reconciliation goes from the planner to the LLM.** Don't write a reconciliation step builder. Let the LLM that's generating the user-facing response see all the tool results.
4. **Fanout / parallel execution is premature.** Don't build it until you have a real flow that benefits (e.g., L6 "compare flights" where 3 providers return in parallel). Even then, `Promise.all` is sufficient until you outgrow it.

### Recovery
`git show <archive-commit-sha>:services/brevio-brain/src/planner.ts`

---

## Workflow Runtime

**Archived from:** `services/brevio-brain/src/workflow-runtime.ts` (340 lines) + `services/brevio-gateway/src/workflow-runtime.ts` (91 lines)
**Layer:** L7 (autonomous follow-throughs)

### Concept

A pair of HTTP clients (brain-side + gateway-side) that talked to an external Temporal worker. The gateway-side started workflows (`POST /workflows/message-processing` to begin); the brain-side recorded state transitions as the workflow progressed (`RECEIVED → CLASSIFYING → DECOMPOSING → EXECUTING → AGGREGATING`).

This is the L7 "autonomous follow-throughs" infrastructure: when an agent runs a multi-step task on its own ("book the flight + add it to calendar + email the client"), some runtime needs to remember where the workflow is, retry failed steps, and survive worker restarts.

### Original-implementation critique

- **Temporal-flavored, but unclear if Temporal was actually wired.** The HTTP client pattern works against any workflow engine; in practice it pointed at a worker we may or may not have actually run.
- **2 services for one concern.** The split between brain-side and gateway-side workflow clients is artifact of the prior multi-service architecture, not a real design principle.
- **Premature for v0.1-v0.5.** FOMO's "polling cron + one alert per email" is not a workflow. It's a function call. Even v0.5 (3-5 friends, one alert at a time) has no multi-step flow.

### Future-implementation guidance

Do not introduce a workflow engine until you have **at least 3 real multi-step flows in production**. Examples of what counts:

- "Book the flight, then add to calendar, then email the client" (L7)
- "Watch this email thread for a reply, then draft a response, then ask me to send" (L3-L4 hybrid)
- "Every Monday at 9am, summarize last week's important emails" (recurring; could also be cron)

When that bar is hit, evaluate:

- **Temporal** (managed: Temporal Cloud; or self-host). Best for long-running flows. Heavy operationally.
- **Inngest** or **Trigger.dev** — modern alternatives, lighter operationally, JS-native.
- **A Postgres-backed state machine** — usually correct first move. Tables: `workflows(id, kind, state, current_step, payload)`, `workflow_steps(workflow_id, step, started_at, finished_at, result)`. With idempotency keys per step and a cron worker that picks up `state='running'` rows. ~200 lines, no new dependency.

The choreography concepts (state transitions, idempotency, retry, deterministic step IDs) survive every choice.

### Recovery
- `git show <archive-commit-sha>:services/brevio-brain/src/workflow-runtime.ts`
- `git show <archive-commit-sha>:services/brevio-gateway/src/workflow-runtime.ts`

---

## Multi-step Task Decomposition

**Archived from:** `services/brevio-brain/src/decompose.ts` (166 lines)
**Layer:** L6 (MCP tools), L7 (autonomous)

### Concept

Parsed a user request into 1-10 subtasks. Detected sequential cues ("then", "after that", "once X is done") and parallel markers ("and", "also", "while"). Built a small dependency graph with an execution order (sequential / parallel / mixed).

This is the "given a complex request, break it into pieces" step that sits between intent and tool-router.

### Original-implementation critique

- **Regex-based decomposition is brittle.** "Book flight to Boston then add to calendar" works; "find me a flight to Boston that doesn't conflict with my Thursday morning standup and add the cheapest option to my calendar" doesn't.
- **Useful as a fallback or validation layer**, not as the primary decomposition mechanism.

### Future-implementation guidance

For the future agent, do decomposition with an LLM that returns a JSON DAG:

```json
{
  "subtasks": [
    {"id": "t1", "intent": "search_flights", "depends_on": []},
    {"id": "t2", "intent": "read_calendar", "depends_on": []},
    {"id": "t3", "intent": "filter_flights_by_calendar", "depends_on": ["t1", "t2"]},
    {"id": "t4", "intent": "ask_user_approve", "depends_on": ["t3"]},
    {"id": "t5", "intent": "book_flight", "depends_on": ["t4"]}
  ]
}
```

Then run the regex-based decomposition as a **validation step**: parse the user's text deterministically, compare the count and parallelism cues against the LLM's DAG, flag mismatches for review. The deterministic cues survive as a sanity check on the LLM.

### Recovery
`git show <archive-commit-sha>:services/brevio-brain/src/decompose.ts`

---

## Result Aggregation Across Tools

**Archived from:** `services/brevio-brain/src/aggregate.ts` (104 lines)
**Layer:** L6 (MCP tools), L7 (autonomous)

### Concept

Took results from N skill executions (e.g., the flights search returned 12 options, the calendar read returned 3 conflicts, the price-compare returned a winner) and synthesized them into one user-facing response. Channel-aware: formatted differently for iMessage vs WhatsApp vs Slack.

### Original-implementation critique

- 104 lines is fine; it's a small file. But it was premature for v0.1-v0.5 (one tool, one response).
- **Channel formatting belongs in adapter code**, not in a generic aggregator. The channel-awareness baked into `aggregate.ts` couples two concerns.

### Future-implementation guidance

When you have multi-tool flows (L6+), this layer is necessary. Build it as:

- Take N tool results (each `{tool_id, result, error}`)
- Pass to an LLM with the original user request as context
- LLM produces one user-facing response
- Channel-specific formatting happens in the adapter (`apps/fomo/src/adapters/sendblue.ts`, future `apps/fomo/src/adapters/slack-personal.ts`, etc.), not here

Keep the "single user-facing response from N internal results" pattern. Drop the channel-aware formatting.

### Recovery
`git show <archive-commit-sha>:services/brevio-brain/src/aggregate.ts`

---

## Tool Router

**Archived from:** `services/brevio-brain/src/disambiguate.ts` (219 lines)
**Layer:** cross-cutting, L2 (calendar) onward
**Maps to "Future Agent Intelligence Requirement" component:** Tool Router

### Concept

Rules engine routing a classified intent to one of 12 skill groups: apple-notes, spotify, flight-tracking, email-send, places-location, youtube, etc. Considered user tier (free/paid), deployment mode (sandbox/prod), and user preferences before picking a skill. Output: `{skill_id, confidence, fallback_skills}`.

This is the Tool Router named explicitly in [FOMO_DESIGN.md](../FOMO_DESIGN.md) §"Future Agent Intelligence Requirement".

### Original-implementation critique

- **Rules-based routing doesn't scale past ~20 tools.** Every new tool requires editing a rules table. The future assistant will have hundreds of MCP tools.
- **The user-tier and consent-gating overlays are good.** Those concerns are cross-cutting and need to live somewhere; the routing decision is the right place.
- **The 12-skill catalog is too specific.** "Spotify" and "apple-notes" being first-class entries in the router is a leak from product-thinking into routing-infrastructure.

### Future-implementation guidance

For L2+, replace rule-based routing with **LLM-native tool-calling**:

1. Tools are registered with name + description + JSON schema for inputs
2. The LLM sees the catalog + user request and chooses which tool(s) to call
3. **The user-tier and consent overlays from this file remain** as a wrapper: after the LLM picks a tool, run the safety check (`is this tool enabled for this user? has the user consented to this category? does the tier require approval?`) BEFORE executing.
4. Fallback: if the LLM picks a tool the safety wrapper rejects, return the rejection reason to the LLM and let it pick another (one retry, then surface to user).

Code excerpt worth remembering: the safety-wrapper shape (decide → check → execute or reject) is what makes the Policy/Safety Engine and Tool Router separable.

### Recovery
`git show <archive-commit-sha>:services/brevio-brain/src/disambiguate.ts`

---

## Intent Classification (concept only)

**Archived from:** `services/brevio-brain/src/classify.ts` (216 lines)
**Layer:** cross-cutting, L2 (calendar) onward
**Maps to "Future Agent Intelligence Requirement" component:** Tool Router input layer

### Concept

Mapped a user request to one of N skill intents (e.g., `email.search`, `music.playback`, `transport.flight_tracking`). For each candidate intent, scored the match using a hand-curated keyword pattern set.

This was supposed to be the front door of the Tool Router: classify the user's request → router picks the skill → planner executes.

### Original-implementation critique

- **The implementation was fake.** No LLM call. Pure keyword matching against hand-coded patterns ("play music" → music.playback, "find flight" → transport.flight_tracking). Brittle. Doesn't generalize.
- **The catalog structure (intent → skill_group → skill → adapter) was right.** That hierarchy is reusable.
- **The skill-tier safety overlay (lines that bind intent → tier) is reusable.** High-tier intents (send-email, spend-money) always require approval; this overlay enforces it at classification time.

### Future-implementation guidance

For L2+, do not restore the keyword router. Replace with:

1. **LLM-native tool selection** (as in §Tool Router above) — eliminates the classify step entirely most of the time.
2. **Keyword-based intent classification survives only as a fast-path for high-confidence cases** (e.g., the FOMO reply parser's exact-verb regex layer is an instance of this).
3. **Keep the catalog hierarchy** (`category → group → skill → adapter`) for the tool registry; the LLM tool-calling layer reads from this catalog.
4. **Keep the tier overlay**: every intent has a tier; high-tier always requires user approval before execution. This is load-bearing for L4 (sending) and beyond.

The 216 lines of keyword patterns are landfill. The catalog and tier overlay are gold.

### Recovery
`git show <archive-commit-sha>:services/brevio-brain/src/classify.ts`

---

## Capability Discovery / Inventory

**Archived from:** `packages/shared/src/capability-inventory.ts` (214 lines)
**Layer:** L6 (MCP tools), L7 (autonomous), L8 (memory)
**Maps to "Future Agent Intelligence Requirement" component:** Personal Context Engine (capabilities-known-to-this-user subset)

### Concept

Runtime discovery of which tools/skills are available, scoped to tenant → workspace → user → device. Merged static "enabled skills" lists with dynamic capability records. Tracked consent states per category (email/money/health) with `granted | revoked | snoozed | never_asked`.

This is the Personal Context Engine's "what can I do for THIS user right now?" sub-module.

### Original-implementation critique

- **Multi-tenant scope hierarchy (tenant → workspace → user → device) is premature.** Until you have paying customers (i.e., tenants), there's no reason to have tenants in the schema.
- **Type bugs in the implementation:** `noUncheckedIndexedAccess: true` is on but the code didn't handle the `undefined` case in two places. Indicates the file was written under different strictness settings.
- **The consent-state model (granted/revoked/snoozed/never_asked) is genuinely good.** Especially `snoozed` — *"don't ask about Spotify for 30 days"* is a real user need.

### Future-implementation guidance

When L6 starts (≥10 tools), build:

1. **Single-user scope only** for as long as possible. Add `workspace_id` when you onboard your first team customer. Don't build `tenant_id` until you have a real multi-tenant requirement.
2. **The consent state machine is the keeper.** Reuse the four states. Add `expires_at` (which the original had) so `snoozed` has a clean exit.
3. **Bind to the catalog from §Intent Classification** — capability_inventory rows reference catalog entries, don't duplicate metadata.

### Recovery
`git show <archive-commit-sha>:packages/shared/src/capability-inventory.ts`

---

## Agent-to-Agent Protocol

**Archived from:** `packages/shared/src/schemas/a2a-runtime.ts` (190 lines)
**Layer:** L7+ (very far future)

### Concept

Zod + JSONSchema validators for an inter-agent protocol:

- **AgentCard** — agent identity + capabilities + protocol version. *"I'm the calendar-agent v1.2, I can read/write calendars, I speak protocol v0.3."*
- **AgentTask** — execution state machine for a task handed off between agents (`pending → in_progress → blocked_on_user → done`).
- **CapabilityInventoryEntry** — per-agent capability tracking.

The shape of a future multi-agent architecture where, e.g., a calendar-agent and a flight-search-agent collaborate via structured messages instead of a single monolithic LLM call.

### Original-implementation critique

- **Pure scaffolding for an architecture we may never need.** Multi-agent systems are seductive but rarely necessary; a single capable LLM with good tools usually wins.
- **AgentCard concept is right.** When you DO need multi-agent (e.g., a remote MCP server is itself an agent), the AgentCard shape — identity + capabilities + protocol version — is the right contract.

### Future-implementation guidance

Don't build until you have a concrete multi-agent use case. Examples that would trigger this:

- You publish FOMO as a remote agent that other AI assistants can call (you're a tool in someone else's stack)
- You consume remote AI agents (e.g., a third-party "lawyer-bot" you delegate legal-review tasks to)
- You split FOMO into multiple specialized internal agents because a single LLM context can't hold the prompt budget

For any of those: borrow the AgentCard shape. Drop the AgentTask state machine if you're using a real workflow runtime.

### Recovery
`git show <archive-commit-sha>:packages/shared/src/schemas/a2a-runtime.ts`

---

## Safety Tier System

**Archived from:** `packages/shared/src/skill-tiers.ts` (119 lines)
**Layer:** cross-cutting, load-bearing at L4 (sending) and beyond
**Maps to "Future Agent Intelligence Requirement" component:** Policy/Safety Engine

### Concept

Tools / skills classified into risk tiers:

- **tier 0 — read** (Gmail read, Calendar read, web search) — no consent gate beyond the initial OAuth
- **tier 1 — write-low** (notes, reminders, internal-only writes) — consent at category level, no per-action approval
- **tier 2 — send** (send email, schedule meeting, post message) — per-action user approval required
- **tier 3 — spend** (purchases, bookings, payments) — per-action approval + budget cap
- **tier 4 — irreversible** (delete data, cancel subscriptions, sell stock) — per-action approval + cooling-off period + confirmation step

### Original-implementation critique

- **For 2 skills (gmail-watch, sms-send), 119 lines of tier infrastructure is overkill.**
- **The concept is essential as soon as you ship anything beyond read-only.** FOMO L1 is tier 0 (read Gmail) + tier 0-ish (send a per-user iMessage that the user explicitly opted into). L3 (drafting) is tier 1. L4 (sending) is tier 2. L6 (bookings) is tier 3.

### Future-implementation guidance

When L4 starts (sending), bring tier infrastructure back. Recommended shape:

1. Every tool registered with a `tier` field (0-4)
2. Approval Engine reads the tier and routes:
   - tier 0: no gate
   - tier 1: category consent
   - tier 2: per-action user approval (single iMessage: "send this draft to Sarah? yes / edit / no")
   - tier 3: per-action approval + budget check + budget cap
   - tier 4: per-action approval + 10-min cooling-off + "are you sure" confirmation
3. **The User Approval Engine and Policy/Safety Engine separate concerns:** Policy decides "this action requires approval"; Approval routes the request through the right UX. They share the tier as their interface.

### Recovery
`git show <archive-commit-sha>:packages/shared/src/skill-tiers.ts`

---

## Resume-After-OAuth Pattern

**Archived from:** `services/brevio-gateway/src/pending-message-store.ts` (89 lines)
**Layer:** cross-cutting

### Concept

In-memory TTL store (10 min) for messages that arrived during an OAuth flow. User types *"summarize my emails from Sarah"* → system realizes Gmail isn't connected yet → kicks off OAuth → original message is buffered → on OAuth callback, the original message resumes processing.

89 lines: `put(messageId, payload, ttl)`, `peek(messageId)`, `consume(messageId)`, `prune()` on a setInterval.

### Original-implementation critique

- **Built for the chat UI killed in April.** SMS doesn't have this problem (alerts are asynchronous push; user doesn't initiate a conversation that needs to pause for OAuth).
- **The pattern itself is reusable** for any tool that requires fresh auth mid-flow.
- **In-memory store doesn't survive restarts.** A Postgres-backed `pending_actions` table is the right shape for production.

### Future-implementation guidance

For any future tool that requires OAuth-during-conversation (Calendar OAuth at L2 if a user requests it via iMessage; MCP tool OAuth at L6; etc.):

1. **Persist pending actions in Postgres**, not in memory: `pending_actions(id, user_id, original_request, required_oauth_provider, created_at, expires_at, consumed_at)`.
2. **OAuth callback handler checks for pending actions** for the user that just authed → resumes processing.
3. **TTL of 10 min is too short for SMS** (user might OAuth on desktop, come back to phone later). 24h is reasonable; expire with audit log.
4. **Concurrent OAuth attempts**: dedupe by `(user_id, required_oauth_provider, original_request_hash)`.

The state machine: `pending → oauth_started → oauth_completed → resumed → completed`. Or `pending → expired → audit_log`.

### Recovery
`git show <archive-commit-sha>:services/brevio-gateway/src/pending-message-store.ts`

---

## Approval Gating (pattern reference)

**Source:** `services/brevio-brain/src/gating.ts` (165 lines) — PRUNED to ~40 lines for FOMO, full original archived
**Layer:** cross-cutting, load-bearing at L2 (calendar consent), L4 (per-send approval), L6 (per-tool approval)
**Maps to "Future Agent Intelligence Requirement" components:** Policy/Safety Engine, User Approval Engine

### Concept

Per-action gating policy. For each candidate action in a plan, check:

- Has the user granted **consent** for this category (email / money / health)?
- Are the required **credentials** present (OAuth provider connected, API keys configured)?
- Is the action allowed by **safety policy** (tier check, budget check, deny-list check)?

If any gate fails, return a structured response describing what's missing and what UX to show the user (an OAuth link, a consent prompt, a budget warning).

### Original-implementation critique

- **Multi-action plan loop is premature for FOMO** (one action per alert: send or don't). We extract a ~40-line subset.
- **Per-provider user-facing copy is product-specific** and was tied to the Brevio brand. Rewrite at every product evolution.
- **Bundle / upsell suggestions** baked into gating were a product decision, not a safety concern.
- **The deny-by-default principle is gold** and the structured-response shape (`{passed: bool, reason, remediation_url, remediation_copy}`) is the right contract.

### Future-implementation guidance

For L4 (sending), the per-action approval flow looks like:

1. Plan has action `send_email(to=sarah, body=...)`.
2. Gating checks: consent for `email.send` category? credentials for Gmail? tier check (tier 2 → approval required)?
3. If approval needed: structured response says "needs approval", with the action serialized for display
4. User Approval Engine sends iMessage: *"Send this draft to Sarah? yes / edit / no"* (or the modern Sendblue iMessage equivalent)
5. On `yes`: re-run gating with `approved_by_user=true` flag, execute action
6. On `no`: drop the action, log
7. On `edit`: route to drafting flow

**Keep**: deny-by-default; structured response shape; per-category consent.
**Build fresh**: multi-action plan loop, brand-specific copy, the per-provider explanations.

### Recovery
`git show <archive-commit-sha>:services/brevio-brain/src/gating.ts`

---

## Multi-Process Service Boundaries (architectural)

**Source:** `cmd/{agent,brain,executor,gateway,router,worker}/main.go` + `internal/` (~49k Go lines) + 11 TS services
**Layer:** cross-cutting infrastructure

### Concept

The pre-FOMO repo split functionality into ~11 services (`brevio-auth`, `brevio-brain`, `brevio-edge-relay`, `brevio-gateway`, `brevio-hands`, `brevio-metrics`, `brevio-profile`, `brevio-scheduler`, `brevio-temporal-worker`, `browser-mcp`, `hands-runtime`) plus a parallel Go implementation across 6 cmd binaries. Each service had its own database connection, own deploy, own scaling characteristics.

### Original-implementation critique

- **Premature for 0 users.** A multi-service architecture costs ~5× more to operate, debug, and deploy than a single binary. The pre-FOMO repo paid that cost with no users to amortize it across.
- **Service boundaries were drawn around components**, not around concerns. `brevio-hands` (action execution) and `hands-runtime` (action execution runtime) were two services for one concept — Conway's Law in action, against the team's interest.
- **Go + TS in parallel** was hedging. One language is correct until proven otherwise.

### Future-implementation guidance

Stay single-binary (`apps/fomo`) until at least one of:

- **Different scaling characteristics**: e.g., a real-time inbound webhook handler that needs to be globally distributed (Cloudflare Workers) and a heavyweight Gmail-poll worker (1 instance per N users) might justify a split.
- **Different deploy cadences**: e.g., a stable safety/audit service that ships rarely vs. a fast-moving ranker service that ships daily.
- **Different security boundaries**: e.g., a service that handles raw OAuth tokens vs. a public-internet-facing webhook receiver.

When you do split, draw boundaries around **concerns** (auth, ingestion, ranking, delivery, audit), not around **components** (controllers, services, models). Conway's Law applies to teams of 1+ — design your service map intentionally.

The Go binaries had no salvageable concepts that aren't already represented in the TS code. The decision to consolidate on TS holds.

### Recovery
- Service inventory: `git show <archive-commit-sha>:services/` (will list the 11 service directories)
- Go source: `git show <archive-commit-sha>:cmd/` and `git show <archive-commit-sha>:internal/`

---

## Future Model Router / Provider Registry

**Archived from:**
- `internal/llm/providers.go` (131 lines, Go)
- `internal/llm/service.go` (281 lines, Go)
- `internal/llm/providers_test.go`, `internal/llm/service_test.go` (Go tests)
- `services/brevio-brain/src/types.ts` — `PlannerProvider` enum (TS; `ExternalModelEgress` is KEEP_NOW, migrated to live code)
- `services/brevio-brain/src/config.ts` lines 54-57, 235-238 — planner-provider env wiring
- `config/prompt-templates/intent-classify-v1.yaml`, `response-gen-v1.yaml`, `task-decompose-v1.yaml`

**Layer:** cross-cutting, L2 (calendar) onward
**Maps to "Future Agent Intelligence Requirement":** the substrate beneath every other component (Agent Orchestration, Tool Router, Personal Context Engine, Policy/Safety Engine — all consume model calls)

### Concept

A multi-provider, multi-model abstraction with five separable concerns:

1. **Provider registry** — for each provider (anthropic, openai, future: gemini, mistral, local-llama): base URL, auth method, model list, per-workspace rate limits with Redis key patterns
2. **Model catalog** — for each model: provider, cost per input/output token, max context, **capability tags** (`planning`, `synthesis`, `extraction`, `critique`, `classification`, `routing`, `simple_synthesis`)
3. **Cost estimator** — converts token usage to USD per call (powers `rank_results.cost_usd_micro` and future training-cost reports)
4. **Failover policy** — when to switch from primary to fallback model on errors
5. **Prompt-template format** — YAML-as-config with `{id, model, temperature, max_tokens, system_prompt, input_schema}`, versioned by filename

Plus the cross-cutting **egress policy** (the `ExternalModelEgress` enum) — already KEPT in the active codebase because FOMO needs it, but documented here as part of the larger router surface.

This is the substrate every other agent component eventually consumes. The Tool Router needs it ("pick the cheapest model with the `routing` capability"). The Personal Context Engine needs it ("use a `synthesis`-capable model to summarize the user's recent decisions"). The Approval Engine needs it ("use a strong `critique`-capable model to validate this plan before execution"). The Agent Orchestration layer needs it ("planning-capable model for the main loop, classification-capable model for the fast paths").

### What's in the archived files

**`internal/llm/providers.go`** — five types and four functions. Verbatim struct shapes worth remembering:

```go
type ProviderConfig struct {
  ProviderID string
  BaseURL    string
  AuthMethod string         // "x-api-key" (anthropic), "authorization_bearer" (openai)
  Models     []string
}

type ProviderRateLimit struct {
  RequestsPerMinute int
  TokensPerMinute   int
  RedisKeyPattern   string  // "rl:llm:anthropic:{workspace_id}"
}

type ModelCatalogEntry struct {
  ModelKey           string
  ProviderID         string
  CostPerInputToken  float64
  CostPerOutputToken float64
  MaxContextTokens   int
  Capabilities       []string  // ["planning","synthesis","extraction","critique",
                                // "classification","routing","simple_synthesis"]
}

type TokenUsage struct { InputTokens, OutputTokens int }
```

**Default catalog at archive time** (subject to staleness — re-validate via the bake-off in [FOMO_PLAN.md](../FOMO_PLAN.md) §MR3 before relying on these numbers):

| Model | Provider | Input $/M | Output $/M | Max ctx | Capabilities |
|---|---|---|---|---|---|
| claude-sonnet-4-20250514 | anthropic | 3.00 | 15.00 | 200k | planning, synthesis, extraction, critique |
| claude-haiku-4-5-20250929 | anthropic | 0.80 | 4.00 | 200k | classification, extraction, routing, simple_synthesis |
| gpt-4o | openai | 2.50 | 10.00 | 128k | planning, synthesis, extraction, critique |
| gpt-4o-mini | openai | 0.15 | 0.60 | 128k | classification, extraction, routing, simple_synthesis |

The capability-tag pattern is the most interesting part. Most model abstractions just enumerate models. This one tags what each model is GOOD AT. That lets the Tool Router say *"give me a `routing`-capable model"* rather than *"give me claude-haiku-4-5-20250929"* — survives provider model churn.

**`ShouldFailoverOnPrimaryError` policy** (worth preserving verbatim — well-tuned):

- HTTP 5xx → failover (server-side; primary unhealthy)
- HTTP 429 with `retry-after > 10s` → failover (primary too slow to recover)
- HTTP 429 with `retry-after ≤ 10s` → wait, retry primary (cheaper than burning fallback)
- Timeout on T0/T1 (high-importance request) → failover
- Timeout on lower tier → retry primary (latency tolerable)

**`internal/llm/service.go`** (281 lines) — the runtime layer above providers.go. Selects a model for a given request (probably by capability tag + tier), enforces rate limits via Redis, makes the API call, captures token usage, applies the failover policy, normalizes the response.

**`config/prompt-templates/*.yaml`** — versioned prompt format:

```yaml
id: PROMPT_INTENT_CLASSIFY_V1
model: claude-haiku-4-5-20251001
temperature: 0.1
max_tokens: 256
system_prompt: |
  You are Brevio's intent classifier. Given a user message ...
input_schema:
  type: object
  required: [message_text, user_profile, context]
  properties:
    message_text: { type: string }
    ...
```

Each prompt has a stable `id` (`PROMPT_<name>_V<n>`), a pinned model, sampling parameters, a system prompt, and a JSONSchema for inputs. Versioning is by filename suffix (`-v1`, `-v2`) so multiple versions coexist during migration. FOMO v0.1 adopts this format for its single ranker prompt at `apps/fomo/src/prompts/ranker-v1.yaml` and reply-parser prompt at `apps/fomo/src/prompts/parser-v1.yaml`.

**`ExternalModelEgress`** (KEPT in active codebase, documented here for completeness of the Router surface):

```typescript
type ExternalModelEgress = 'allow' | 'redacted_only' | 'deny';
```

- `allow` — full payload to model (founder dev only)
- `redacted_only` — sender + subject + optional snippet, never full body (FOMO friend-beta default)
- `deny` — short-circuit; return `fomo='unsure'` (diagnostic / audit / paranoid-user mode)

For a multi-tool future, the egress policy is keyed by `(user_id, tool_id)` not just user — a user might allow `redacted_only` to the ranker but `deny` to a third-party MCP tool.

### Original-implementation critique

- **The registry/catalog split is correct.** Provider config (where + auth + rate limits) is operational; model catalog (capabilities + cost + context) is semantic. They evolve independently — Anthropic's API doesn't change when they release a new Haiku, but the catalog gets a new entry.
- **Capability tags are gold.** Routing by capability instead of by model ID survives provider model churn (every quarter when OpenAI silently swaps a checkpoint, the catalog updates; the consuming code doesn't).
- **Workspace-scoped rate limits (`rl:llm:anthropic:{workspace_id}`) are premature for FOMO** (single-user). For a multi-tenant assistant, this is correct.
- **The failover policy is conservative.** It doesn't consider cost — a failover from gpt-4o-mini to claude-sonnet is 20× more expensive than waiting. Future version should add `max_cost_multiplier_for_failover` (default 5×).
- **The catalog dates go stale fast.** Anthropic released `claude-haiku-4-5-20250929` and then `claude-haiku-4-5-20251001` two days apart per the prompt template. Keep the catalog current via quarterly bake-off review, or accept that capability tags + provider IDs (not specific model dates) are the durable identifiers.
- **Go-to-TS port is straightforward** but not free. The struct shapes translate one-for-one; the rate-limit-via-Redis pattern is reusable; the failover policy is pure logic. Plan ~1 day of work when L6 starts.

### Future-implementation guidance

When L6 starts (first MCP tool integration beyond FOMO), build the router as:

1. **Port the registry + catalog to TS.** Use `internal/llm/providers.go` as the spec. The struct shapes translate one-for-one.
2. **Keep the capability tag list short and stable.** Adding tags is cheap; renaming is expensive. Start with the 7 from the catalog (`planning`, `synthesis`, `extraction`, `critique`, `classification`, `routing`, `simple_synthesis`). Add new tags only when a new agent component requires a new dimension.
3. **Router API:**
   ```typescript
   router.pick({
     capability: 'routing',
     tier: 'T1',
     prefer_provider?: 'anthropic',
     max_cost_per_call_micro?: 5000,    // 0.5¢
   }): ModelSelection
   ```
   Returns the cheapest model with that capability at that tier, respecting preferences and cost caps. Defaults tunable per workspace.
4. **Egress policy becomes per-`(user_id, tool_id)`.** Not just per-user. A user might permit FOMO's ranker to see `redacted_only` content but deny a third-party MCP tool entirely.
5. **Quarterly bake-off** ([FOMO_PLAN.md](../FOMO_PLAN.md) §MR3 methodology) re-validates the catalog. When a provider silently changes a checkpoint, the bake-off detects the regression before users do.
6. **Prompt-template YAML ships now** for FOMO. When L6 lands, every tool has its own template with `id`, `capability_required`, `temperature`, `max_tokens`, `system_prompt`, `input_schema`, `output_schema`.
7. **Add cost-aware failover.** Extend `ShouldFailoverOnPrimaryError` to take a `cost_ratio` parameter; refuse failover if `fallback_cost > primary_cost × max_cost_multiplier`.

### Recovery

- `git show <archive-commit-sha>:internal/llm/providers.go`
- `git show <archive-commit-sha>:internal/llm/service.go`
- `git show <archive-commit-sha>:internal/llm/providers_test.go`
- `git show <archive-commit-sha>:internal/llm/service_test.go`
- `git show <archive-commit-sha>:services/brevio-brain/src/types.ts` (for `PlannerProvider`; `ExternalModelEgress` is live at `apps/fomo/src/core/egress-policy.ts`)
- `git show <archive-commit-sha>:services/brevio-brain/src/config.ts`
- `git show <archive-commit-sha>:config/prompt-templates/intent-classify-v1.yaml`
- `git show <archive-commit-sha>:config/prompt-templates/response-gen-v1.yaml`
- `git show <archive-commit-sha>:config/prompt-templates/task-decompose-v1.yaml`

---

## Index by Layer

Use this when planning a layer to find which archived concepts are relevant.

| Layer | Concept | Section |
|---|---|---|
| L2 (Calendar read) | Tool Router | §Tool Router |
| L2 | Capability Discovery (when adding the second OAuth provider) | §Capability Discovery |
| L3 (Drafting) | Approval Gating | §Approval Gating |
| L3 | Safety Tier System | §Safety Tier System |
| L4 (Sending) | Approval Gating + User Approval Engine | §Approval Gating |
| L4 | Safety Tier System | §Safety Tier System |
| L4 | Resume-After-OAuth Pattern (if drafting requires fresh auth) | §Resume-After-OAuth |
| L5 (Calendar write) | Approval Gating | §Approval Gating |
| L6 (MCP tools) | Tool Router | §Tool Router |
| L6 | Capability Discovery | §Capability Discovery |
| L6 | Multi-step Decomposition | §Multi-step Decomposition |
| L6 | Result Aggregation | §Result Aggregation |
| L6 | Workflow Runtime (only if multi-step flows justify) | §Workflow Runtime |
| L7 (Autonomous) | Agent Orchestration | §Agent Orchestration |
| L7 | Workflow Runtime | §Workflow Runtime |
| L7 | Approval Gating (autonomous still requires per-tier checks) | §Approval Gating |
| L7+ | Agent-to-Agent Protocol (only if external multi-agent) | §Agent-to-Agent Protocol |
| L8 (Memory) | Personal Context Engine (build from scratch; nothing archived covers this directly) | — |
| L8 | Future Model Router (Personal Context Engine consumes synthesis-capable models) | §Future Model Router |
| Cross-cutting | Multi-Process Service Boundaries (when to split) | §Multi-Process |
| Cross-cutting / L2+ | Future Model Router / Provider Registry (capability-tagged catalog, failover policy, prompt-template YAML, egress policy) | §Future Model Router |
| L6 (MCP tools) | Future Model Router (capability-based model selection per tool) | §Future Model Router |
| L7 (Autonomous) | Future Model Router (failover policy is load-bearing for long-running flows) | §Future Model Router |

## Template for new entries

When archiving a new module, append a section using this template:

```markdown
## <Concept Name>

**Archived from:** `<path>` (<N> lines)
**Layer:** <L_n> (<description>), <L_m> (<description>)
**Maps to "Future Agent Intelligence Requirement" component:** <component name, if any>

### Concept
<one paragraph: what it did, what problem it solved>

### Original-implementation critique
<what was wrong with how the deleted module solved it; what is actually reusable as a concept vs. just code>

### Future-implementation guidance
<when this concept becomes load-bearing, what to keep, what to redo, what trigger justifies bringing it back>

### Recovery
`git show <archive-commit-sha>:<path>`
```

Then add a row in the §Index by Layer.

## Maintenance

- This doc is updated on every cleanup PR that archives a new module.
- The `<archive-commit-sha>` placeholder in each Recovery line is replaced with the actual commit hash when the deletion lands. Until then, the file is still in the working tree and accessible at HEAD.
- Stale entries (e.g., a future-implementation guidance line that turns out to be wrong after the concept is re-implemented) are not removed — they are amended with a `### Update YYYY-MM-DD` block recording what changed and why. Institutional memory means recording corrections too.
