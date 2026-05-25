# FOMO-Killer v0.1 Plan — Minimal MCP OS Demo + FOMO Trust Workflow

Status: Draft for Claude execution review
Project direction: OpenClaw-style personal-agent runtime, not Claude Code-style one-off chatbot
Core demo: Gmail context → model ranking → policy gate → Slack trust checkpoint → SendBlue iMessage → natural reply → feedback/memory update
Stack: Vercel + Render + Neon + Upstash + Cloudflare R2 + SendBlue/Linq-ready adapter + OpenAI/Anthropic + Drizzle
AWS: no direct AWS dependency in `apps/fomo`

---

## 1. Executive Verdict

This plan turns the new `FOMO_DESIGN.md` into an implementation path Claude can follow without drifting.

The new goal is not merely to build a Gmail alert app. The goal is to build the **smallest safe demo of the future Brevio agent**:

> **A humanistic iMessage assistant running on a minimal MCP-style personal-agent operating system, with FOMO as the first real workflow.**

The user-facing promise stays narrow:

> **Brevio watches your Gmail and texts you only when there is something you would be sad to miss.**

The architecture promise is bigger:

> **Brevio should eventually become an OpenClaw-like personal-agent runtime: tools, context, memory, permissions, state, workflows, audit, learning, and safe action — all through a governed MCP-style operating layer.**

v0.1 must prove this pattern in miniature:

1. User talks to Brevio naturally through iMessage-style messaging.
2. Brevio understands that important-email monitoring requires Gmail read access.
3. Brevio asks for read-only Gmail permission through OAuth.
4. Gmail becomes the first context provider.
5. SendBlue becomes the first user messaging tool.
6. Slack becomes the first human trust checkpoint.
7. The model ranks emails and classifies replies.
8. Deterministic code executes allowed actions.
9. Feedback and memory signals are stored.
10. Audit logs make the whole workflow traceable.

v0.1 must stay narrow. It must not include email sending, calendar writes, bookings, purchases, payments, browser control, shell access, full MCP marketplace, delegated agents, or broad autonomous execution.

The rule for Claude:

> **Think like OpenClaw. Ship like FOMO.**

More precisely:

> **Build the smallest safe MCP OS kernel, and run FOMO as its first real workflow.**

---

## 2. Claude Mental Model: What You Are Building

Claude must internalize this before touching code.

### Wrong mental model

Do **not** build:

* a chatbot,
* a simple Gmail script,
* a broad Poke clone,
* a Claude Code-style one-off command agent,
* a fake MCP platform with unused abstractions,
* a giant Brevio rebuild.

### Correct mental model

Build:

> **A long-running personal-agent runtime with one real workflow.**

The first workflow is FOMO:

```text
Gmail → rank → policy gate → Slack review → SendBlue alert → reply parser → feedback/memory update
```

The future system will eventually support many workflows, but v0.1 only proves one.

### The operating-system analogy

To the user, Brevio feels like one friendly assistant.

Internally, Brevio should behave like a small safe OS:

| OS idea     | Brevio equivalent                            |
| ----------- | -------------------------------------------- |
| Apps        | tools / capabilities                         |
| Filesystem  | context providers and memory                 |
| Permissions | OAuth scopes, consent, tool tiers            |
| Kernel      | policy gate, egress policy, approval rules   |
| Processes   | workflows and state machines                 |
| Logs        | audit log, tool invocations, feedback events |
| Workers     | future delegated reasoners                   |
| Sandboxes   | future secure executors for risky tools      |

v0.1 builds only the first tiny kernel slice.

---

## 3. Non-Negotiable Principles

### 3.1 Constitution

> **The AI may decide what tools would help.**
> **The system decides what tools are allowed.**
> **The user approves anything risky.**

### 3.2 Runtime principle

> **Text like a human. Think like an agent. Execute like a safety-critical system.**

### 3.3 Implementation principle

> **Real or absent. Never half-wired.**

No fake tools. No fake MCP servers. No fake safety gates. No placeholder modules on user-reachable paths. No “we’ll wire it later” code that looks active now.

### 3.4 Scope principle

> **v0.1 may build safe foundation primitives early, but it may not expose risky capabilities early.**

Safe to build now:

* tool registry,
* permission gate,
* OAuth manager,
* egress policy,
* audit log,
* state machine,
* memory signals,
* feedback events,
* model router for classification,
* Slack trust checkpoint,
* SendBlue user messaging.

Not safe to build now:

* booking,
* buying,
* payment,
* email sending,
* calendar writes,
* broad autonomous action,
* browser/shell/computer control,
* delegated sub-agents.

---

## 4. The Seven MCP OS Architecture Laws

These laws must shape implementation decisions. They are not all features to build in v0.1.

### 4.1 Tool Lean-in

One capability equals one clean tool/adapter.

v0.1 active tools:

* `gmail.read` / `gmail.watch.read`
* `sendblue.send_user_message`
* `slack.founder_review`
* `audit.write`
* `feedback.write`
* `memory_signal.write`

Future tools:

* Calendar read/write,
* email draft/send,
* restaurant booking,
* flight search/booking,
* hotel search/booking,
* shopping/purchase,
* reminders,
* contacts,
* files,
* browser/computer-use.

Rule: future tools must enter through registry, schema, permission, audit, and risk tier. No random one-off integrations.

### 4.2 Context Providers

The agent needs structured context, not just action tools.

v0.1 context providers:

* Gmail metadata/content for ranking,
* user preferences,
* sender importance,
* suppressions,
* feedback history,
* alert history.

Future context providers:

* calendar,
* contacts,
* files,
* docs,
* personal graph,
* semantic memory,
* prior conversations,
* workflow history.

Rule: context access and action execution are different risk classes.

### 4.3 Gateway Connectors

Every tool/context provider must pass through a gateway layer.

v0.1 gateway pieces:

* Tool Registry,
* OAuth Connection Manager,
* Permission Manager / Policy Gate,
* Egress Policy,
* Audit Log,
* Tool Invocations,
* Kill Switches,
* Rate Limits.

Rule: no tool should bypass the gateway.

### 4.4 Stateful Session Managers

Brevio cannot be stateless.

v0.1 state:

* OAuth connection state,
* Gmail cursor,
* alert lifecycle,
* founder review state,
* SendBlue send state,
* reply parsing state,
* snooze state,
* feedback application state.

Rule: every workflow has explicit states and recovery paths.

### 4.5 Sandboxed Executors

Future dangerous execution needs sandboxing.

v0.1 does **not** include:

* shell execution,
* browser automation,
* code execution,
* computer use,
* payments,
* booking execution.

Rule: no high-risk executor exists until sandbox, policy, audit, and approval are proven.

### 4.6 Workflow Packagers

Future tasks become packaged workflows.

v0.1 packaged workflow:

```text
FOMO Workflow:
Gmail ingest → rank → gate → Slack approval → SendBlue alert → parse reply → update feedback/memory
```

Future workflows:

* plan trip,
* schedule meeting,
* draft reply,
* book dinner,
* buy product,
* follow up with person.

Rule: future workflows should extend the same state/audit/permission pattern.

### 4.7 Delegated Reasoners

Future Brevio may use sub-agents, but not v0.1.

Future examples:

* Inbox Agent,
* Calendar Agent,
* Travel Agent,
* Shopping Agent,
* Memory Agent,
* Safety Agent.

Rule: no sub-agent exists until it has isolated tools, isolated context, tests, and audit logs.

---

## 5. What “Minimal MCP OS” Means In v0.1

This section prevents overbuilding.

In v0.1, “MCP OS” does **not** mean:

* full MCP marketplace,
* plugin platform,
* delegated agents,
* browser control,
* shell execution,
* workflow engine for everything,
* arbitrary tool calling,
* full semantic memory graph.

It means building the smallest real operating-system pattern:

| OS piece          | v0.1 version                                        |
| ----------------- | --------------------------------------------------- |
| Tool Registry     | Gmail + SendBlue + Slack/internal writers only      |
| Context Provider  | Gmail + memory signals                              |
| Gateway           | OAuth + permission + egress + audit + kill switches |
| State Manager     | Alert State Machine + connection state              |
| Workflow          | FOMO workflow only                                  |
| Human Trust Layer | Slack founder review                                |
| User Interface    | SendBlue iMessage-style texting                     |
| Learning Layer    | feedback events + memory signals                    |
| Observability     | audit log + tool_invocations + state transitions    |

Rule:

> **Do not build the full OS. Build the first working kernel slice.**

---

## 6. Build Now vs Preserve For Later

| Area           | Build in v0.1                                              | Preserve for later                                    |
| -------------- | ---------------------------------------------------------- | ----------------------------------------------------- |
| Tools          | Gmail read, SendBlue user messaging, Slack review          | Calendar, flights, restaurants, purchases, email send |
| Context        | Gmail, sender importance, suppressions, feedback           | contacts, files, personal graph, semantic memory      |
| Gateway        | OAuth, Permission Gate, Egress Policy, Audit               | full MCP gateway, external tool marketplace           |
| State          | alert state machine, connection state                      | multi-step workflow orchestration                     |
| Execution      | deterministic TypeScript code                              | sandboxed browser/shell/code execution                |
| Reasoning      | email ranking, reply classification, tool-need recognition | full planning, delegated reasoners                    |
| Learning       | feedback events, memory signals, adaptive threshold        | fine-tuning, RLHF, autonomous feature generation      |
| Human approval | Slack founder review                                       | user approval engine for risky actions                |
| Messaging      | SendBlue active                                            | Linq as future adapter if better                      |
| Storage        | Neon Postgres, Upstash Redis                               | pgvector semantic search, R2 artifacts when needed    |

---

## 7. v0.1 Product Flow

### 7.1 Primary demo flow

User texts Brevio through SendBlue/iMessage:

> “Can you make sure I don’t miss important emails?”

Brevio replies:

> “Yes. I’ll need read-only Gmail access. I won’t send, delete, or change emails.”

User connects Gmail through OAuth.

Brevio runs:

```text
Gmail new email
→ extract safe context
→ model rank
→ policy gate
→ Slack founder review
→ SendBlue iMessage alert
→ user reply
→ reply parser
→ feedback event
→ memory signal
→ audit log
```

User receives:

> “This looks important. It’s from your counselor, and it sounds like it’s due tonight.”

User replies:

> “Remind me later tonight.”

Brevio replies:

> “Got it. I’ll bring it back tonight.”

This is the v0.1 demo of the future Brevio agent.

### 7.2 Request handling examples

| User says                                  | v0.1 behavior                                                                                                      |
| ------------------------------------------ | ------------------------------------------------------------------------------------------------------------------ |
| “Make sure I don’t miss important emails.” | Allowed. Ask for Gmail OAuth if missing. Run FOMO workflow.                                                        |
| “Why did you text me this?”                | Allowed. Explain briefly in normal language.                                                                       |
| “Don’t text me about LinkedIn anymore.”    | Allowed. Suppress sender/category if safe.                                                                         |
| “Remind me later tonight.”                 | Allowed. Snooze alert.                                                                                             |
| “Book me a Miami flight.”                  | Understand intent, block action. Say: “I can’t book flights yet. For now, I can help with important Gmail alerts.” |
| “Send Sarah an email.”                     | Understand intent, block action. Say: “I can’t send emails yet. Later, I’ll ask before sending anything.”          |
| “Connect my Gmail.”                        | Allowed through OAuth. Never ask for password.                                                                     |
| “Buy this shirt.”                          | Block. No purchases in v0.1.                                                                                       |
| “Open my calendar.”                        | Block unless future Calendar read is explicitly approved.                                                          |

---

## 8. Active v0.1 Product Scope

v0.1 includes:

* SendBlue iMessage-style interface,
* human assistant voice,
* Gmail read-only OAuth,
* important-email monitoring,
* Slack founder review before alerts,
* SendBlue user alert after approval,
* natural-language reply parser,
* STOP/START,
* `/help`,
* sender importance,
* suppressions,
* feedback events,
* memory signals,
* audit log,
* kill switches,
* no AWS stack.

v0.1 does not mean the whole future assistant. It means one real workflow on one small agent runtime.

---

## 9. Active v0.1 Platform Foundations

Each foundation must have:

* real v0.1 caller,
* tests,
* explicit non-overbuild boundary,
* audit where relevant.

### 9.1 Tool Registry

Purpose: declare active capabilities.

Active tools:

* `gmail.read`
* `sendblue.send_user_message`
* `slack.founder_review`
* `audit.write`
* `feedback.write`
* `memory_signal.write`

Non-real or future tools must not be user-reachable.

### 9.2 OAuth Connection Manager

Purpose: safe account access.

v0.1 provider:

* Google Gmail read-only.

Rules:

* OAuth only,
* no passwords,
* minimum scopes,
* encrypted tokens,
* revocation support,
* no tokens in logs/browser/Slack.

### 9.3 Permission Manager / Policy Gate

Purpose: decide what is allowed.

Checks:

* tool exists,
* tool is real,
* user consent exists,
* OAuth connected,
* kill switch allows,
* daily cap not hit,
* sender not suppressed,
* model output valid,
* Slack approval exists when needed.

### 9.4 Egress Policy

Purpose: control what leaves Brevio for model calls.

Rules:

* no raw full Gmail body by default,
* ranker receives safe context,
* reply parser receives user text + limited alert context,
* no friend email body to Slack,
* no private email bodies in repo.

### 9.5 Alert State Machine

Purpose: track workflow state.

Example states:

* detected,
* ranked,
* gated_out,
* queued_for_review,
* approved,
* sent,
* replied,
* snoozed,
* ignored,
* failed,
* send_status_unknown.

### 9.6 Feedback Events

Purpose: learning data.

Events:

* founder_approved,
* founder_rejected,
* user_opened,
* user_snoozed,
* user_ignored,
* ignored_sender,
* asked_why,
* stop,
* no_response,
* false_positive,
* false_negative.

### 9.7 Memory Signals

Purpose: early personalization.

Signals:

* sender importance,
* suppressions,
* timing preferences,
* topic importance,
* alert usefulness,
* quietness preference.

### 9.8 Model Router

Purpose: pick cost/quality model for classification.

v0.1 active capability:

* classification.

Do not add planning/extraction/synthesis tags until there is a caller.

### 9.9 Cost Tracking

Purpose: know model cost.

Track:

* model_name,
* prompt_version,
* latency,
* tokens,
* estimated cost,
* schema_valid.

### 9.10 Tool Invocations and Audit Log

Purpose: make actions traceable.

Record:

* tool calls,
* model calls,
* policy decisions,
* sends,
* failures,
* state transitions,
* consent changes,
* kill switch changes.

---

## 10. Explicit Non-Goals

v0.1 must not include:

* sending emails,
* drafting emails,
* calendar writes,
* restaurant booking,
* flight booking,
* hotel booking,
* food ordering,
* shopping,
* purchases,
* payments,
* arbitrary browser/computer control,
* shell access,
* code execution,
* full MCP marketplace,
* delegated sub-agents,
* full Personal Context Engine,
* full semantic memory search,
* broad autonomous execution,
* cross-user training without consent,
* raw private email bodies in repo/logs/Slack,
* fake/stub tools on user-reachable paths,
* AWS direct dependency.

---

## 11. Target Tech Stack

### 11.1 v0.1 no-AWS stack

| Layer                    | Service                                                 |
| ------------------------ | ------------------------------------------------------- |
| Frontend/onboarding      | Vercel + Next.js                                        |
| Backend/webhooks/workers | Render Starter or equivalent always-on Node runtime     |
| Database                 | Neon Postgres                                           |
| ORM                      | Drizzle ORM                                             |
| Vector memory            | pgvector reserved, not active unless real caller exists |
| Cache/locks/rate limits  | Upstash Redis                                           |
| File/object storage      | Cloudflare R2 reserved                                  |
| Messaging                | SendBlue active; Linq future adapter                    |
| Models                   | OpenAI + Anthropic APIs                                 |
| Observability            | structured logs + Postgres audit first                  |
| Future eval/trace        | Langfuse later if needed                                |

No AWS direct dependency in `apps/fomo`.

### 11.2 Why this stack

This stack is cheap, practical, and good enough for founder/friend beta.

It avoids overbuilding while preserving the future direction:

* Neon + Drizzle = exact state and memory substrate,
* pgvector later = semantic memory,
* Upstash = locks/rate limits/session state,
* SendBlue = first iMessage-like interface,
* Vercel = onboarding,
* Render = always-on runtime,
* OpenAI/Anthropic = model bake-off and routing.

---

## 12. Repository Shape

`apps/fomo` should be a modular monolith.

```text
apps/fomo/src/
  routes/
    onboarding/
    webhooks/
    health.ts
  workers/
    gmail-poll.ts
    cost-digest.ts
    retention-prune.ts
  adapters/
    gmail.ts
    sendblue.ts
    slack.ts
    models.ts
  core/
    tool-registry.ts
    policy-gate.ts
    egress-policy.ts
    model-router.ts
    state-machine.ts
    approval.ts
    audit.ts
    safe-logger.ts
    kill-switches.ts
  memory/
    feedback-events.ts
    memory-signals.ts
    sender-importance.ts
    suppressions.ts
  db/
    schema.ts
    client.ts
    migrations/
  eval/
    ranker-fixtures/
    reply-parser-fixtures/
    assistant-voice-fixtures/
  security/
    webhook-auth.ts
    token-crypto.ts
```

One deployable app. Clear internal boundaries.

Do not create microservices yet.

---

## 13. Database Plan

v0.1 tables should exist only if they have active callers.

Required tables:

* `users`
* `connections`
* `tools`
* `oauth_tokens`
* `consent`
* `gmail_cursors`
* `message_events`
* `rank_results`
* `alerts`
* `alert_state_transitions`
* `replies`
* `sender_importance`
* `suppressions`
* `user_preferences`
* `feedback_events`
* `memory_signals`
* `tool_invocations`
* `audit_log`

Each table must define:

* purpose,
* active caller,
* key fields,
* PII/sensitivity,
* retention,
* indexes,
* constraints.

Do not create unused future tables.

`connection_requests` is future-only unless v0.1 actually uses it for Gmail onboarding. If Gmail onboarding is implemented as a direct OAuth route, no `connection_requests` table yet.

---

## 14. Model and Eval Plan

### 14.1 Model bake-off

Test:

* GPT mini/small model,
* GPT stronger model,
* Claude Haiku/small model,
* Claude Sonnet/strong model.

Compare:

* precision,
* recall,
* false positives,
* false negatives,
* JSON reliability,
* explanation quality,
* latency,
* cost per 1,000 emails.

Use the cheapest model that passes quality.

### 14.2 Required metadata

Every rank/classification stores:

* model_name,
* prompt_version,
* latency_ms,
* estimated_cost,
* schema_valid,
* score,
* reason,
* final_gate_decision.

### 14.3 Evals

Required v0.1 evals:

* email importance eval,
* reply parser eval,
* assistant voice eval,
* safety-block eval,
* no-leak eval,
* model JSON reliability eval,
* regression eval.

No raw real email bodies in repo.

Use anonymized, synthetic, or local-only fixtures.

---

## 15. Assistant Voice Plan

User-facing text must be tested.

### Rules

* short,
* human,
* simple,
* direct,
* no jargon,
* no architecture terms,
* no long robotic explanations,
* one clear next step.

### Voice fixtures

Bad:

> “Your request maps to an unsupported Tier 3 action.”

Good:

> “I can’t book flights yet. For now, I can help make sure you don’t miss important Gmail messages.”

Bad:

> “OAuth authorization is required to enable Gmail capability access.”

Good:

> “I’ll need read-only Gmail access to do that. I won’t send, delete, or change emails.”

Assistant voice evals should fail if user-facing copy exposes internal language like:

* policy engine,
* canonical intent,
* tool tier,
* egress,
* model router,
* ranker,
* MCP server.

---

## 16. Safety and Security Plan

### 16.1 SendBlue webhook security

Friend beta requires real webhook signing/HMAC or an explicitly approved equivalent.

Dev/founder fallback can use secret path/header, but friend beta should not proceed unless webhook auth is strong.

### 16.2 Token security

* OAuth tokens encrypted,
* no tokens in logs,
* no tokens in browser,
* no tokens in Slack,
* revocation supported,
* read-only Gmail scope only.

### 16.3 Policy safety

* fail closed,
* no unknown tool allowed,
* no unknown tier allowed,
* no LLM-self-authorized autonomy,
* no direct state mutation from model output.

### 16.4 Data safety

* no raw email bodies in logs,
* no raw email bodies in Slack,
* no raw email bodies in repo,
* friend Slack cards should not include private body content,
* cross-user learning requires explicit consent and must be behavioral/content-free.

### 16.5 Kill switches

* `FOMO_SEND_ENABLED`
* `FOMO_AUTO_SEND_ENABLED`
* `FOMO_FRIEND_BETA_ENABLED`
* `FOMO_MAX_USERS`

Defaults must be safe.

---

## 17. Implementation Milestones

### v0.1 — Minimal MCP OS + Founder FOMO Demo

Goal:

Founder can ask Brevio to monitor important Gmail, connect Gmail, have Gmail ranked, see Slack trust checkpoint, approve, receive SendBlue iMessage, reply naturally, and produce feedback/memory signals.

Must prove:

* Gmail OAuth works,
* Gmail read-only works,
* Tool Registry works,
* Permission Gate works,
* model ranking works,
* Slack review works,
* SendBlue outbound works after Slack approval,
* reply parser works,
* feedback events write,
* memory signals write,
* audit logs write,
* no raw body leakage,
* kill switches work.

### v0.3 — Friend-safe hardening

Goal:

Founder flow stable enough to consider one friend.

Must add or prove:

* SendBlue inbound security,
* STOP/START reliability,
* friend Slack privacy rules,
* no duplicate sends,
* model eval threshold,
* assistant voice evals,
* user memory visibility path if promised.

### v0.5 — Friend beta

Goal:

3–5 close friends after gates pass.

Must prove:

* consent is clear,
* friend privacy safe,
* founder review manageable,
* no webhook spoofing risk,
* no raw private email leakage,
* user can stop anytime.

### v0.8 — Conditional auto-send

Goal:

Only high-confidence alerts auto-send.

Must prove:

* false positives below target,
* no STOP events recently,
* kill switch tested,
* user consent for auto-send,
* founder spot-checking works.

### v1.0 — Wedge decision

Continue, pivot, or kill.

---

## 18. Day-by-Day Implementation Plan

Be honest: v0.1 is likely **3–4 weeks**, not a fake 2-week sprint.

### Phase 0 — Plan approval

* Review this plan.
* Do not code until approved.

### Phase 1 — Repo cleanup and salvage audit

Goals:

* create new branch,
* build green,
* classify modules,
* preserve future concepts,
* kill fake active code.

Deliverables:

* `SALVAGE_DECISIONS.md`,
* `docs/future-architecture-notes.md`,
* clean repo shape,
* no history rewrite.

### Phase 2 — Minimal MCP OS kernel

Build:

* Tool Registry,
* OAuth Manager,
* Permission Gate,
* Egress Policy,
* Audit Log,
* Tool Invocations,
* Kill Switches,
* State Machine,
* Feedback Events,
* Memory Signals,
* Model Router.

### Phase 3 — FOMO workflow

Build:

* Gmail poll,
* ranker,
* Slack founder review,
* SendBlue outbound,
* reply parser,
* feedback/memory updates.

### Phase 4 — Founder demo

Founder uses the full flow.

### Phase 5 — Hardening

Prove:

* idempotency,
* webhook auth,
* privacy,
* evals,
* kill switches,
* no raw body leakage.

### Phase 6 — Friend beta

One friend first.

Then 3–5 only if stable.

---

## 19. Gates

### Repo Cleanup Gate

Pass criteria:

* build passes,
* tests pass,
* salvage decisions documented,
* future concepts archived,
* no fake active code.

### Model Bake-Off Gate

Pass criteria:

* model candidates tested,
* precision/recall measured,
* JSON reliability measured,
* cost measured,
* chosen model documented.

### Founder Demo Gate

Pass criteria:

* Gmail connected,
* ranker works,
* Slack review works,
* SendBlue send works,
* reply parser works,
* memory/feedback writes,
* no duplicate sends,
* no raw body leakage.

### Friend Beta Gate

Pass criteria:

* webhook security proven,
* privacy copy clear,
* STOP tested,
* friend-safe Slack cards tested,
* kill switches tested,
* no raw email bodies leaked,
* founder flow stable.

### Auto-Send Gate

Pass criteria:

* false positives low,
* no recent STOP events,
* users consent,
* idempotency proven,
* kill switch tested,
* founder spot-check works.

---

## Smoke Test Gates

> **Rule (founder directive 2026-05-24):** every real external
> integration must have a founder-only smoke test before the next
> dependent phase begins. Born from the 3B.3 Gmail and 3C.2 OpenAI
> experiences — code shipping is not the same as substrate working
> against a real provider. Smoke tests close that gap.

### Inventory

The v0.1 milestone has six smoke-test gates. Each one is a hard
prerequisite for the next dependent phase.

| Gate     | What it proves                                      | Unblocks |
| -------- | --------------------------------------------------- | -------- |
| **3B.3** | Founder Real Gmail Smoke Test                       | 3C.x     |
| **3C.2** | OpenAI Ranker Smoke Eval                            | 3C.3     |
| **3D.x** | Slack Founder Review Smoke Test                     | 3E       |
| **3E.x** | SendBlue Outbound Founder-Only Smoke Test           | 3F       |
| **3F.x** | SendBlue Inbound Reply Smoke Test                   | 3G       |
| **3G**   | Full Founder Demo Smoke Test (end-to-end v0.1)      | v0.3     |

Numbering convention: a `.x` suffix denotes the specific smoke-test
sub-phase that lands at the end of the named subphase. For example,
3D's Slack adapter PR is `3D` and its smoke test is `3D.x` (concrete
number assigned when scheduled).

### Required deliverables per smoke gate

Every smoke test in the inventory MUST ship the following before its
PASS report is committable:

1. **Runbook** — step-by-step founder procedure under `docs/`. Covers
   external-provider setup (Google Cloud, OpenAI, Slack workspace,
   SendBlue account), env vars, commands, verification, stop.
2. **Preflight script where possible** — pure config inspection under
   `apps/fomo/scripts/preflight-<gate>.ts`. No network, no DB.
   Validates every required env var; warns on forbidden flags. Exit 1
   on any missing/invalid input.
3. **Evidence script where possible** — read-only post-run query script
   under `apps/fomo/scripts/<smoke>-evidence-<gate>.ts` OR the smoke
   script itself writes a structured artifact to `docs/<gate>-results.json`.
   Captures the gate-specific PASS/INVESTIGATE evidence.
4. **Smoke report template** — `docs/<GATE>_REPORT_TEMPLATE.md` under
   `docs/`. Founder copies to `docs/<GATE>_REPORT.md` (drop `_TEMPLATE`)
   and fills in.
5. **Committed PASS report before the next phase begins** — the report
   commits to the SAME PR branch as the smoke scaffolding (mirrors
   3B.3 and 3C.2 patterns). Only then does the PR merge, and only then
   does the next dependent subphase get branched.

### Rules

- **Smoke tests are founder-only.** No friend, no beta user, no
  delegated runner. The founder is the only person who knows whether
  Brevio actually feels right against their own data.
- **No friend beta smoke test until founder smoke tests pass.** v0.5
  cannot start until 3G is green.
- **No secrets committed.** Use gitignored `.env.*.local` files for
  real keys; commit only `.env.*.example` templates with placeholders.
- **No raw Gmail bodies committed.** Synthetic or anonymized only,
  per §14.3. Fixtures must round-trip through `applyEgressForRanker`
  in tests to prove no leak.
- **No raw private data in logs/audit.** Evidence scripts MUST scan
  recent audit + tool_invocations records for forbidden keys
  (`body_plain`, `body_html`, `attachments`, header dumps, long
  base64 blobs) and fail the gate on hits.
- **Kill switches must be tested.** Every gate proves the relevant
  switch defaults to "no effect on the world" and that flipping it
  off actually halts the substrate.
- **Idempotency must be tested for messaging.** 3E and 3F gates must
  prove that duplicate send/receive events do not double-fire.
- **Webhook auth must be tested for inbound providers.** 3F's gate
  proves SendBlue inbound webhooks reject unsigned / mis-signed
  payloads.
- **External API failures must fail closed.** Every gate exercises
  at least one upstream failure path (401, 429 quota, 5xx) and
  proves the substrate aborts cleanly without partial-state
  corruption.

### Why this rule exists (lessons from 3B.3 and 3C.2)

The first two smoke gates surfaced bugs that no amount of unit testing
caught:

- **3B.3** revealed that the runbook's session-mint one-liner was
  missing required token fields (`session_id`, `expires_at`); valid
  HMAC, invalid payload. Only a live OAuth handshake against real
  Google could find this.
- **3C.2** revealed that gpt-5 reasoning models reject any explicit
  `temperature` value (400 unsupported_value); only a live call to
  real OpenAI surfaced it. Same run also surfaced that
  `insufficient_quota` is a 429 that retries cannot fix — required a
  one-line backend amendment to fail fast.

Unit tests caught zero of these. The smoke-test gate caught all of
them before they could damage a downstream phase. The pattern is
formalized here so 3D / 3E / 3F / 3G each get the same protection.

### Phase-map dependency (informal; tracked formally in `apps/fomo/KERNEL.md`)

```
3B.1 → 3B.2 → 3B.3 ──gate──┐
                            ↓
                          3C.1 → 3C.2 ──gate──┐
                                              ↓
                                            3C.3 → 3C.4 ──gate──┐
                                                                 ↓
                                                                3D ──gate──┐
                                                                            ↓
                                                                          3E ──gate──┐
                                                                                      ↓
                                                                                    3F ──gate──┐
                                                                                                ↓
                                                                                              3G ──gate──→ v0.1 done
```

Each `──gate──` arrow represents a committed PASS report. The
prerequisite arrow may not be crossed without it. Subphases that do
NOT involve a real external integration (3C.1 substrate, 3C.3 worker
wiring) do not require a smoke gate but still require the standard
build/test/lint CI pass.

---

## 20. Testing Plan

Required tests:

* tool registry startup gate,
* OAuth scope test,
* token encryption test,
* policy gate fail-closed test,
* egress redaction test,
* Gmail ingest idempotency test,
* alert state transition test,
* SendBlue send idempotency test,
* send_status_unknown test,
* reply parser intent tests,
* STOP/START tests,
* assistant voice tests,
* model JSON reliability tests,
* model bake-off eval,
* no raw body in logs test,
* no raw body in Slack test,
* cross-user isolation test,
* kill switch tests,
* webhook bad-signature test,
* feedback event write tests,
* memory signal update tests.

---

## 21. Cut List If Behind Schedule

Can cut or delay:

1. daily cost digest UI/post,
2. adaptive threshold rule,
3. weekly FP/FN digest,
4. model bake-off artifact formatting,
5. `/help` if SendBlue inbound is delayed,
6. memory inspect command,
7. friend beta.

Cannot cut:

* idempotency,
* kill switches,
* permission gate,
* OAuth token encryption,
* no raw body leakage,
* tool registry startup gate,
* no fake/stub active code,
* model eval before production use,
* build-green requirement,
* Slack trust checkpoint,
* audit logging.

---

## 22. Risks Still Open

* SendBlue webhook signing may be insufficient.
* Linq may be better long-term but is not v0.1 active.
* Model quality may not meet false-positive target.
* User privacy concerns may block friend beta.
* Unit economics may be high because iMessage rails cost money.
* Claude may overbuild unless the Active Seam Gate is enforced.
* Claude may underbuild by making a Gmail script instead of a kernel slice.
* Assistant may sound robotic unless voice tests are real.
* Adaptive learning could become creepy unless user control is respected.

---

## 23. Questions Needing Founder Decision

1. SendBlue confirmed for v0.1, with Linq as future adapter?
2. Is Slack founder review acceptable for v0.1 demo?
3. Should v0.1 include friend beta, or founder demo only?
4. How strict should memory inspection be before friends join?
5. Which model candidates should be included in the bake-off?
6. What is the monthly budget cap for founder beta?
7. Are any friends comfortable with founder seeing sender/subject metadata?

---

## 24. Final Recommendation

Approve this plan only if the goal is:

> **minimal Brevio MCP OS kernel + FOMO as first workflow.**

Safe to approve next:

1. rewrite/replace old `FOMO_PLAN.md` with this plan,
2. Day 1 repo cleanup and build-green work,
3. salvage audit,
4. no live friend beta yet.

Still blocked:

* real friend onboarding,
* auto-send,
* email sending,
* calendar writes,
* bookings,
* purchases,
* full MCP marketplace,
* delegated agents,
* browser/shell/computer control.

The final instruction for Claude:

> **Do not freestyle. Do not build the whole future assistant. Do not make a Gmail script. Build the smallest safe OpenClaw-style kernel and prove it through the FOMO workflow.**
