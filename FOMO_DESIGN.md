# Design: FOMO-Killer — Minimal MCP OS Demo + FOMO Trust Workflow v0.1

Generated: 2026-05-22
Last cleanup pass: 2026-06-04
Repo: Executive AI Agent / Brevio monorepo
Status: **Approved long-term constitution.** Directionally locked. Cleanups via PR; principle changes require explicit founder directive.
Mode: Startup, foundation-aware, OpenClaw-style runtime direction
Supersedes: prior FOMO-only and foundation-aware drafts

---

## 0. Current Status / How to Use This Document

**This document is the long-term Brevio / FOMO design constitution.** It describes the product vision, the MCP OS architecture laws, the permanent product principles (assistant voice, safe-learning tiers, personalized importance learning, memory-first architecture, API-first / browser-fallback / approval-required execution policy), and the eight architecture rules every future capability must compose with.

**It is NOT the active phase tracker.** Do not read sections like the "Historical v0.1 Build Path" (formerly "The Assignment") as a list of phases still to do. v0.1 has already proven the founder loop, and the v0.5.x friend-beta substrate (multi-tenant, real-friend, production-hardening, second-friend cross-tenant smokes) has already shipped — see the latest `docs/SMOKE_REPORT_v0.5.*.md` for what's landed.

**Active phase state lives in three other places:**

* **[FOMO_PLAN.md](FOMO_PLAN.md)** — the implementation plan, the milestone ladder (§17), the day-by-day phase breakdown, the per-phase gates, and the current set of v0.5.x+ future-phase candidates.
* **[apps/fomo/KERNEL.md](apps/fomo/KERNEL.md)** — the kernel-completeness map: what's wired, what's reserved, what's intentionally absent.
* **PR reports under [docs/](docs/)** — `SMOKE_REPORT_*.md` for what each phase actually proved; `personalized-importance-learning.md` for the long-term product principle and proposed future-phase shape.

**How to use this doc when designing or shipping anything:**

* Before changing a permanent rule (anything in §6 architecture laws, §10 assistant voice, §13 safe-learning tiers, §13.5 personalized importance learning, §22 safety lessons), read this whole doc first. Permanent rules are not casually rewritten — they survived because they were earned.
* Before introducing a new tool, capability, or external dependency, check §6 Rule 8 (API-first / browser-fallback / approval-required) and §23 stack rules (reserved ≠ active). Both are load-bearing constraints.
* Before treating a "reserved" stack entry (Redis, pgvector, R2, Langfuse, Sentry, PostHog, etc.) as available, read §23 — "reserved" means "preserved as a future option, NOT active." Activating any reserved item requires an approved current phase with a real caller.
* When in doubt between this doc and FOMO_PLAN.md, this doc wins on *principles*; FOMO_PLAN.md wins on *what to build next*.

---

## 1. Executive Summary

FOMO v0.1 is being re-cast again.

The product is still narrow. The first user-facing promise is still:

> **Brevio watches your Gmail and texts you only when there is something you would be sad to miss.**

But the architecture goal is now clearer and more ambitious:

> **v0.1 should be the smallest real demo of the larger Brevio agent — a humanistic iMessage assistant running on a minimal MCP-style agent operating system.**

This means v0.1 is not just a Gmail alert script. It is also not the full future assistant. It is the smallest safe kernel slice of the long-term product.

The user should experience something that feels like the future Brevio agent:

* they text with it naturally through iMessage-style messaging,
* it understands what they want in plain English,
* it knows which tools it has,
* it knows which tools are missing,
* it asks for permission before connecting Gmail,
* it reads Gmail only after OAuth consent,
* it routes work through a tool/context gateway,
* it creates candidate alerts,
* Slack acts as the first trust checkpoint,
* SendBlue sends the iMessage alert after approval,
* user replies become feedback and memory signals.

This is the important shift:

> **FOMO is the first workflow. The real v0.1 product demo is the minimal Brevio MCP OS running that workflow safely.**

The long-term direction is OpenClaw-like, not Claude Code-like. Brevio should not be a short-lived one-prompt process. It should become a long-running personal-agent runtime: gateway, tools, context, memory, sessions, policies, approvals, observability, and eventually delegated agents.

But v0.1 must stay narrow. It may only activate Gmail read and SendBlue user messaging. It must not ship bookings, purchases, email sends, calendar writes, browser control, shell access, payments, or broad autonomous execution.

The core constitution remains:

> **The AI may decide what tools would help.**
> **The system decides what tools are allowed.**
> **The user approves anything risky.**

The new architecture constitution is:

> **Think like an MCP operating system. Ship only the smallest real, exercised, tested slice.**

The personality rule is:

> **Text like a human. Think like an agent. Execute like a safety-critical system.**

The structural rule is:

> **Real or absent. Never half-wired.**

The stack is cheap and AWS-free:

* Vercel + Next.js for onboarding,
* Render Starter or equivalent always-on Node runtime,
* Neon Postgres + Drizzle ORM,
* pgvector reserved for future semantic memory,
* Upstash Redis for locks/rate limits/short-term state,
* Cloudflare R2 reserved for files/artifacts,
* SendBlue active for iMessage/SMS,
* Linq documented as a future messaging adapter if access/pricing/reliability are better,
* OpenAI + Anthropic APIs through a narrow model router,
* structured logs and Postgres audit first; Langfuse/Sentry/PostHog later if needed.

No AWS direct dependency in `apps/fomo`.

---

## 2. Problem Statement

The founder is building a personal AI assistant. The long-term dream is big: an iMessage-native AI assistant that knows the user, remembers what matters, reasons through goals, chooses tools, asks for missing access, and safely gets things done with permission.

But the original broad framing — “build Poke, but more powerful” — is too wide for a first product.

The real pain is simpler and more emotional:

> “There are days in which I never open my inbox and I get scared that this makes me miss out on things.”

That is the first product.

The first user-facing value is not “automation.” It is relief:

> **I can ignore my inbox without being scared that I missed something important.**

The assistant earns trust by being quiet. It should not spam the user. It should not act recklessly. It should only interrupt when it has a strong reason.

At the same time, the architecture cannot be a dead-end Gmail script. If v0.1 hardcodes Gmail → ranker → text with no proper tool/context/permission/memory foundation, the project will have to be rebuilt when it expands.

So the real problem v0.1 must solve is two-sided:

1. **Product problem:** Can an AI assistant safely watch Gmail and text only about emails the user would regret missing?
2. **Architecture problem:** Can we build the smallest real slice of the future Brevio agent OS without recreating the old Brevio mistake of fake, half-wired, overbuilt infrastructure?

The answer is this design:

> **FOMO is the first workflow running on the minimal Brevio MCP OS.**

---

## 3. New Product Direction

### FOMO v0.1 = Minimal MCP OS Demo + FOMO Trust Workflow

The new v0.1 is no longer only:

> “Gmail alert app.”

It is:

> **A simple demo of the bigger Brevio personal agent, using FOMO as the first real workflow.**

The user should be able to text naturally:

> “Can you make sure I don’t miss important school emails?”

If Gmail is not connected, the assistant should understand:

> “This needs Gmail read access. Gmail is not connected. I should ask for permission and open OAuth.”

Then the assistant should say in normal language:

> “I can do that. I’ll need read-only Gmail access. I won’t send, delete, or change emails.”

The user connects Gmail through OAuth.

Then Brevio’s minimal MCP OS runs the FOMO workflow:

1. Gmail context provider finds new email.
2. Model ranks whether the user would be sad to miss it.
3. Policy gate checks consent, caps, suppressions, and safety.
4. Slack acts as the first human trust checkpoint.
5. SendBlue sends the iMessage alert after approval.
6. User replies naturally.
7. Reply parser maps the message to a safe intent.
8. Feedback and memory signals update.
9. Audit logs record the full path.

This is small, but it demonstrates the future shape:

* user request,
* tool/context need detection,
* permission request,
* OAuth connection,
* tool execution,
* human approval,
* iMessage response,
* memory and learning.

That is the future Brevio agent in miniature.

---

## 4. Long-Term Vision

Brevio should become a humanistic, iMessage-native personal AI assistant that feels like texting a real person but internally behaves like a safe operating system for intelligent agents.

To the user, it should feel like:

> “I text Brevio, and it helps me handle life.”

Internally, it should behave like:

> **A modular MCP-style agent OS.**

The long-term assistant should eventually:

* live mainly in iMessage/text,
* understand natural language,
* remember the user deeply,
* reason about goals,
* decide which tools would help,
* know which tools/accounts are connected,
* know which tools/accounts are missing,
* safely ask for missing access,
* open OAuth/MCP-style connection flows,
* use tools through permission gates,
* maintain session and workflow state,
* ask for approval before risky actions,
* learn from feedback,
* adapt safely to the user,
* route simple tasks to fast models and complex tasks to stronger models,
* recover from failures,
* log everything important,
* run continuous evals.

The long-term product is not “chatbot with APIs.”

It is closer to:

> **a personal operating layer for your life.**

But the first version must be humble:

> **Gmail read + Slack trust checkpoint + SendBlue iMessage alert + memory/feedback signals.**

---

## 5. Definition of Autonomy

Autonomy does not mean the agent secretly does whatever it wants.

Autonomy means the assistant can reason about what is needed and ask for the right permission.

### Autonomy means the assistant can:

* understand a user request,
* infer the likely tool/context needed,
* notice missing access,
* ask for permission,
* open OAuth or tool-connection flows,
* propose a plan,
* classify messages and replies,
* update low-risk preferences,
* suggest safer customizations,
* learn from feedback.

### Autonomy does not mean the assistant can:

* ask for passwords,
* secretly access accounts,
* expand OAuth scopes silently,
* self-authorize tool access,
* bypass policy gates,
* send emails without approval,
* text third parties without approval,
* book flights without approval,
* buy things without approval,
* store payment methods without explicit setup,
* use browser/shell/computer-control without sandboxing,
* treat fake/stub tools as real.

### Constitution

> **The AI may decide what tools would help.**
> **The system decides what tools are allowed.**
> **The user approves anything risky.**

This is the permanent rule.

---

## 6. MCP OS Architecture Constitution

Brevio should evolve toward an MCP-style operating system for personal AI agents.

MCP is not just “tool calling.” It is a way to build modular, governed, observable, production-ready agent infrastructure.

The eight architecture laws below must shape Brevio long-term. The first seven describe the *structure* of the agent OS (what kinds of components exist and how they relate). The eighth — **API-first, browser-fallback, approval-required** — is the *execution policy* that gates how high-risk actions are chosen and carried out across every one of the first seven. It is not optional and not separable; it is a permanent companion rule to the others. In v0.1, only the smallest real slice of any of them is implemented.

> **API first. Browser fallback only when sandboxed. User approval before final commitment.**
>
> Brevio may decide that a missing tool is needed, but it may not silently obtain access, silently use browser automation, or silently complete high-risk actions. The AI may propose; the system gates; the user approves.
>
> Mock tests prove code. Smoke tests prove reality. Every real-world capability needs a founder-only smoke test before it is trusted.

### 1. Tool Lean-in

One capability should map to one clean tool/server/adapter.

Examples:

* Gmail read,
* SendBlue user messaging,
* future Calendar read,
* future email draft,
* future flight search,
* future restaurant booking,
* future purchase tool.

Each capability should be modular, versionable, permissioned, observable, and replaceable.

**v0.1:** Gmail read and SendBlue user messaging are the only active tools.

### 2. Context Providers

Agents do not only need action tools. They need context.

Context providers include:

* Gmail,
* memory,
* user preferences,
* future calendar,
* future contacts,
* future files,
* future personal graph,
* future docs and databases.

Reading context is different from taking action.

**v0.1:** Gmail is the first external context provider. Memory signals are the first internal context provider.

#### Email is the first context category — Gmail is the first email provider, not the final email strategy (founder directive 2026-05-30)

Brevio must not be architected as Gmail-only forever. Email is one *category* of context; Gmail is the v0.1 / v0.5 implementation of that category. Long-term Brevio must have an `EmailContextProvider` abstraction:

```text
EmailContextProvider
├── GmailProvider        — v0.1 / v0.5 (first; fastest, cleanest API path)
├── OutlookProvider      — likely next official provider (Microsoft Graph)
├── iCloudMailProvider   — later (weaker provider support; extra security review)
├── YahooMailProvider    — later (weaker provider support; extra security review)
└── GenericIMAPProvider  — later (no OAuth in many cases; extra security review)
```

Long-term invariants:

* Every provider normalizes inbound email into the same `RawEmailContext` shape (or its future equivalent). The ranker, egress policy, alert pipeline, memory signals, feedback events, and SendBlue outbound flows MUST NOT depend on Gmail-specific assumptions.
* Every provider passes through the same gateway shape the v0.1 Gmail path passes through: Tool Registry, Permission Gate, OAuth / credential manager, Egress Policy, Audit Log, kill switches, founder-only smoke-test gate, and provider-specific failure handling (`*UnauthorizedError`, `*ApiError`, retryable vs terminal).
* Outlook / Microsoft is likely the next official provider after Gmail because its API path (Microsoft Graph) is the closest to Gmail in OAuth, scope, and reliability terms.
* iCloud, Yahoo, and generic IMAP must receive extra security review because provider OAuth support, polling semantics, and UX surface area (re-auth flows, app-specific passwords) are weaker.
* No raw email passwords may be collected unless explicitly approved after a dedicated security review and a founder-only smoke gate.
* No browser automation for webmail inbox access unless Rule 8 below (§6 Rule 8 — API-first, browser-fallback only when sandboxed, explicit user approval, founder-only smoke test) is satisfied.

**v0.5 scope (locked):** Gmail is the only active `EmailContextProvider`. Outlook, iCloud, Yahoo, generic IMAP, and webmail browser automation are NOT in scope for v0.5 — they are documented here so the ranker, egress policy, memory, feedback, and SendBlue flows are not built with Gmail-only assumptions that would have to be unwound later.

### 3. Gateway Connectors

Every tool and context provider should eventually pass through a gateway layer for:

* auth,
* OAuth,
* consent,
* permissions,
* routing,
* rate limits,
* egress policy,
* logging,
* observability,
* audit.

This prevents tools from becoming backdoors.

**v0.1:** Tool Registry, Permission Manager, OAuth Connection Manager, Egress Policy, Tool Invocations, Audit Log, and kill switches form the first gateway.

### 4. Stateful Session Managers

Brevio must not behave like a stateless chatbot.

Real assistants track:

* active sessions,
* task state,
* alerts,
* replies,
* approvals,
* snoozes,
* retries,
* failures,
* workflow progress.

**v0.1:** Alert State Machine is the first stateful session/workflow manager.

### 5. Sandboxed Executors

Any future browser, shell, code, computer-use, payment, booking, or high-risk external execution must run inside a sandbox with strict permissions and audit logs.

**v0.1:** no sandboxed executors. No browser, shell, computer-use, bookings, purchases, or payment execution.

### 6. Workflow Packagers

Future multi-step tasks should become packaged workflows.

Example future workflow:

> Search flights → compare options → ask approval → book → add to calendar → text confirmation.

**v0.1:** FOMO is the first workflow package:

> Gmail → rank → gate → Slack review → SendBlue alert → reply parser → feedback/memory update.

### 7. Delegated Reasoners

Future Brevio may have specialized sub-agents:

* inbox agent,
* calendar agent,
* travel agent,
* shopping agent,
* memory agent,
* safety agent.

Each sub-agent must have isolated tools, isolated context, tests, and audit logs.

**v0.1:** no delegated reasoners. The FOMO workflow is the only active workflow.

### 8. API-first, Browser-fallback, Approval-required (High-Risk Tool Execution Rule)

**This is a permanent Brevio architecture law, on the same level of importance as the first seven rules.** It is the execution policy that sits on top of Tool Lean-in, Context Providers, Gateway Connectors, Stateful Session Managers, Sandboxed Executors, Workflow Packagers, and Delegated Reasoners — it does not replace them, it constrains how they may be used when real-world consequences are at stake.

Long-term Brevio must choose how to act in this order:

1. **Prefer an official API, MCP server, or trusted adapter** whenever one exists. The structured, documented, permissioned path is always the first choice.
2. **If no official API or MCP tool exists, browser automation may be considered — but only through a sandboxed executor.** Browser automation is a fallback, never a default.
3. **Browser automation must go through the full gateway: Tool Registry, Permission Gate, audit logging, egress policy, kill switches, state tracking, and human approval.** Browser tools are not exempt from any of the first seven rules.
4. **For high-risk actions — payments, purchases, bookings, legal, healthcare, financial actions, account changes, destructive actions, permission expansion, or anything irreversible — Brevio must require explicit user approval before the final action.**
5. **For some high-risk actions, Brevio may prepare the action but require the user to complete the final confirmation manually.** Preparing is not committing.
6. **The system must never silently connect accounts, silently expand permissions, silently use browser automation, silently complete payments, silently purchase items, or silently submit bookings.** Silence at the moment of consequence is forbidden.
7. **Every new browser-automation capability needs its own founder-only smoke test before production use.** Mock tests prove code. Smoke tests prove reality.
8. **Every high-risk workflow must have a clear rollback / cancel / manual-review story where possible.** "What does the user do if this goes wrong?" must have an answer before the workflow ships.
9. **If the risk is too high or the system cannot verify the action safely, Brevio must guide the user instead of executing.** The right response to uncertainty is help, not autonomy.

#### Required wording (so the rule survives rewording over time)

> Brevio may decide that a missing tool is needed, but it may not silently obtain access, silently use browser automation, or silently complete high-risk actions. The AI may propose; the system gates; the user approves.
>
> API first. Browser fallback only when sandboxed. User approval before final commitment.

#### How this composes with the first seven rules

This rule is not separate from rules 1–7. It is an execution policy built on top of them:

* **Tool Lean-in (#1):** API / MCP / browser executor are each a *tool capability*; each enters the registry, schema, permission, audit, and risk-tier model. Browser automation does not bypass that.
* **Context Providers (#2):** Brevio uses structured user/account context (preferences, prior approvals, suppression lists) to *decide* what is needed before it asks. Context informs the question; context never replaces approval.
* **Gateway Connectors (#3):** All auth, OAuth, consent, permissions, routing, rate limits, egress policy, logging, observability, and audit pass through the gateway pattern. No high-risk tool has a back door.
* **Stateful Session Managers (#4):** High-risk workflows need *explicit* workflow state — what was proposed, what was approved, what was sent, what can still be rolled back. No "did we ship this yet?" ambiguity.
* **Sandboxed Executors (#5):** Browser automation must be isolated. The sandbox is the technical enforcement layer for rule #2 above.
* **Workflow Packagers (#6):** Payments, bookings, purchases, account changes — each becomes its own explicit workflow package with named states, approval gates, and rollback affordances.
* **Delegated Reasoners (#7):** Future specialized agents (inbox, calendar, travel, shopping) must remain permissioned and audited. A sub-agent does not get to silently do what the kernel forbids. The kill switches apply to the agent, not just the user.

#### Worked example — payments

If a user asks:

> "Pay John $25 on Venmo."

Brevio must reason in this order:

1. This is a *payment*, so it is *high-risk*.
2. First check whether there is an official Venmo API, MCP server, or trusted adapter.
3. If no safe API exists, browser automation may be considered only as a sandboxed fallback.
4. Brevio must not silently connect Venmo.
5. Brevio must not silently submit payment.
6. Brevio must ask for exact confirmation:
   * recipient,
   * amount,
   * note,
   * funding source, if visible / required,
   * final submit action.
7. If the browser automation risk is too high, Brevio should guide the user to complete the final payment manually rather than execute it.
8. Every step (proposal, approval, attempt, outcome) must be audited.
9. A founder-only smoke test is required before any such capability is trusted in production.

**Correct behavior:**

> "I can help prepare this, but I need your approval before anything is sent. Confirm: send $25 to John Smith on Venmo with note 'lunch'?"

**Incorrect behavior:**

> Brevio opens Venmo and sends money without exact user confirmation.

The same shape applies to: connecting a new account, expanding OAuth scopes, booking flights / hotels / restaurants, making purchases, signing legal forms, submitting healthcare data, changing account settings, deleting anything.

**v0.1:** the rule is in force as design discipline. The Permission Gate, Egress Policy, kill switches, audit log, and founder-only smoke tests are the v0.1 *implementations* of this rule for Gmail (read-only) and SendBlue (founder-only outbound after Slack approval). The substrate already enforces "no silent send" — `FOMO_SEND_ENABLED=false` by default, `FOMO_AUTO_SEND_ENABLED=false` always in v0.1, founder review in Slack required before SendBlue fires. As Brevio grows beyond FOMO into calendar, drafting, payments, bookings, browser/computer-use, MCP tools, and delegated agents, the same shape extends: the AI may propose; the system gates; the user approves.

### Practical interpretation

Brevio should feel like one friendly assistant to the user.

Internally, it should behave like a safe operating system:

* tools are capabilities,
* memory/context is the filesystem,
* permissions are the security model,
* policy is the kernel,
* workflows are processes,
* audit logs are observability,
* sandboxes isolate dangerous execution,
* delegated reasoners are future workers,
* **the API-first / browser-fallback / approval-required rule is the kernel's execution policy** — every irreversible action passes through it.

---

## 7. v0.1 Product Experience

v0.1 should give a simple demo of what Brevio will become.

### Example flow

User texts:

> “Can you make sure I don’t miss important emails?”

Brevio replies:

> “Yes. I’ll need read-only Gmail access. I won’t send, delete, or change emails.”

User connects Gmail through OAuth.

Brevio watches Gmail.

A new email arrives from a school counselor:

> “Reminder: interview form due tonight.”

Brevio ranks it as important.

Slack receives a founder review card:

> Candidate alert for Albert.
> Sender: Counselor
> Subject: Interview form due tonight
> Score: 0.91
> Approve / reject

Founder approves.

Brevio texts the user through SendBlue:

> “This looks important. It’s from your counselor, and it sounds like it’s due tonight.”

User replies:

> “Remind me later tonight.”

Brevio replies:

> “Got it. I’ll bring it back tonight.”

The system stores:

* alert sent,
* user snoozed,
* sender likely important,
* topic likely time-sensitive,
* memory signal,
* feedback event,
* audit log.

This is the v0.1 demo.

It is not the full future agent. But it shows the future pattern.

---

## 8. v0.1 Scope

v0.1 includes:

### User-facing features

* iMessage-style interface through SendBlue,
* simple human assistant voice,
* Gmail read-only connection through OAuth,
* important-email alerts,
* Slack founder review as trust checkpoint,
* natural-language replies,
* safe internal reply intents,
* STOP/START handling,
* basic `/help`,
* no broad dashboard.

### Active tools

* Gmail read-only context provider,
* SendBlue user messaging tool,
* Slack founder review tool,
* audit writer,
* memory signal writer,
* feedback event writer.

### Active foundation

* Tool Registry,
* Capability Discovery limited to FOMO needs,
* OAuth Connection Manager for Google,
* Permission Manager / Policy Gate,
* Egress Policy,
* Alert State Machine,
* Model Router for classification,
* Cost Tracking,
* Feedback Events,
* Memory Signals,
* Tool Invocations,
* Audit Log,
* Kill Switches,
* Safe Logger,
* Assistant Voice fixtures.

### Data and memory

* sender importance,
* suppressions,
* user preferences,
* alert history,
* reply outcomes,
* false positives,
* false negatives,
* snooze behavior,
* ignored alerts,
* why requests,
* no-response signals,
* source-linked memory signals.

---

## 9. v0.1 Explicit Non-Goals

v0.1 must not include:

* sending emails,
* drafting emails,
* modifying Gmail,
* deleting Gmail,
* calendar writes,
* restaurant booking,
* flight booking,
* hotel booking,
* food ordering,
* shopping,
* purchases,
* payments,
* arbitrary browser control,
* computer-use execution,
* shell access,
* full MCP marketplace,
* delegated sub-agents,
* full Personal Context Engine,
* full semantic memory search,
* full OpenClaw-style execution,
* cross-user training without explicit consent,
* raw private email bodies in repo/logs/Slack,
* fake/stub tools on user-reachable paths,
* AWS direct dependency.

If a feature requires risky action, it is future-only.

If a foundation seam has no active v0.1 caller, it is future-only.

---

## 10. Assistant Voice: Text Like A Human

The assistant must not sound like a backend system.

It should sound like a calm, smart person texting.

### Rules

* short messages,
* simple words,
* no jargon,
* no long explanations unless asked,
* no “policy engine” in user-facing text,
* no “tool registry” in user-facing text,
* no “canonical intent” in user-facing text,
* no “unsupported tier” language,
* one clear next step,
* honest limitations,
* warm but not fake,
* helpful but not overeager.

### Good and bad examples

Bad:

> “OAuth authorization is required to enable Gmail capability access.”

Good:

> “I’ll need read-only Gmail access to do that. I won’t send, delete, or change emails.”

Bad:

> “Your request maps to an unsupported Tier 3 action.”

Good:

> “I can’t book flights yet. For now, I can help make sure you don’t miss important Gmail messages.”

Bad:

> “This email was surfaced due to high semantic salience and sender-priority weighting.”

Good:

> “This looks important. It’s from Sarah, and it sounds like she needs an answer today.”

### Voice principle

> **Text like a human. Think like an agent. Execute like a safety-critical system.**

---

## 11. Natural Language Reply UX

Users should not have to type robotic commands.

The assistant can show suggested replies, but the user can text naturally.

Examples:

* “remind me later tonight” → `later`
* “bring this back tomorrow” → `tomorrow`
* “nah not important” → `ignore`
* “don’t text me about LinkedIn anymore” → `ignore_from_sender`
* “why did you send me this?” → `why`
* “stop texting me” → `stop`

The LLM may classify the reply.

The LLM may not execute the action.

Execution must happen through deterministic code.

### Canonical safe intents

* `open`
* `later`
* `tomorrow`
* `ignore`
* `ignore_from_sender`
* `why`
* `stop`
* `unparsed_reply`
* `clarify`

If confidence is low, ask one clarification.

If sender matching is ambiguous, ask which sender.

If unknown, log but do nothing risky.

---

## 12. Slack as the First Trust Mechanism

Slack is not the final product surface.

Slack is a v0.1 trust mechanism.

In early versions, the model should not text users automatically. It should first propose an alert. Slack lets the founder approve or reject before the SendBlue iMessage goes out.

Founder review serves three purposes:

1. **Safety:** prevents bad alerts from reaching users.
2. **Training data:** approvals and rejections become feedback events.
3. **Trust calibration:** shows whether the agent is useful before auto-send.

Over time, founder review should change:

* first: every alert requires founder approval,
* later: only uncertain alerts need review,
* later: random samples are reviewed for quality,
* eventually: user approval replaces founder approval for user-representing actions.

Founder review should not leak private data.

For friends, Slack should show only minimal metadata, never full email bodies.

---

## 13. Human Feedback and Adaptive Learning

Brevio must learn from the user on the fly.

This is not full RLHF in v0.1.

It is practical adaptive personalization.

The assistant should learn from:

* founder approved,
* founder rejected,
* user opened,
* user snoozed,
* user ignored,
* user ignored sender,
* user asked why,
* user sent STOP,
* no response,
* false positive,
* false negative,
* repeated timing patterns,
* repeated sender/category behavior,
* explicit corrections.

These signals should update:

* sender importance,
* suppressions,
* alert thresholds,
* daily cap,
* quiet-hour preferences,
* category preferences,
* topic importance,
* future feature proposals.

### Safe learning tiers

| Tier | Learning type                            | Approval needed?               |
| ---- | ---------------------------------------- | ------------------------------ |
| L0   | Store feedback event                     | no                             |
| L1   | Adjust sender importance/ranking         | usually no, must be reversible |
| L2   | Infer preference or custom rule          | notify or ask depending risk   |
| L3   | Change alert behavior significantly      | yes                            |
| L4   | Connect new account/tool or expand scope | explicit approval              |
| L5   | Send, book, buy, delete, pay             | enhanced approval              |

### Feature proposal behavior

The assistant may safely propose personalized improvements.

Example:

> “I noticed you keep snoozing school emails until evening. Want me to hold non-urgent school emails until 7 PM?”

That is safe.

Unsafe:

> “I noticed you travel often, so I connected flight booking and stored your payment info.”

That is forbidden.

### Principle

> **Learn like a human assistant. Adapt like a cautious system. Ask before changing anything that matters.**

---

## 13.5 Personalized Importance Learning (long-term product principle)

Brevio is not a generic email-alert bot. It is not a "for everyone, ranked by a global model" surface. The first user-facing promise — *I can ignore my inbox without being scared that I missed something important* — only holds if Brevio learns each user's *personal* definition of important, and keeps each user's learning isolated from every other user's.

This section is the design-side anchor for that capability. The canonical doc lives at [docs/personalized-importance-learning.md](docs/personalized-importance-learning.md). Read it before changing ranker behavior, commercial / spam handling, the reply parser, feedback events, or memory signals.

### Why this is a permanent product principle, not a ranker tweak

During v0.5.x testing, Brevio classified urgent-sounding spam and commercial emails as important. The reflexive fix — "ignore all commercial mail" — is wrong. It is wrong because some commercial and transactional emails are exactly the ones a user would be sad to miss:

* bank fraud alerts,
* flight changes,
* invoices,
* subscription / account warnings,
* shipping issues,
* school / payment notices,
* customer-support replies,
* job / application updates,
* security alerts.

A category-level suppression would silently turn Brevio blind to the most consequential interruptions in the user's life. That is a worse failure mode than the noise we are trying to reduce.

The right response is not a smarter prompt. It is a per-user, signal-backed, reversible, auditable learning loop — feedback events feed memory signals, memory signals feed the ranker, the user can correct the system in plain English, and every correction is per-user and never bleeds across users.

### The permanent quotes

> **Brevio must learn each user's personal definition of important.**

> **Brevio should become less noisy over time without becoming blind.**

> **Personalized learning must improve interruption quality without silently suppressing genuinely important messages.**

These three sentences are on the same load-bearing tier as the §6 architecture laws and the §13 safe-learning-tier discipline. Every future change to ranker behavior, commercial-handling rules, feedback intent parsing, or memory-signal kinds must serve them.

### What this means at the architecture level

* **Per-user keyspace, always.** Every learned signal is keyed by `user_id`. The v0.5.x cross-tenant isolation invariants — Morris's data untouched by Sheila's smoke; one user's `stop_active` row never modifying another's — apply unchanged. There is no global learning, no "users like you also suppressed…" aggregation, no cross-user inference. See [v0.5.4 cross-tenant proof](docs/SMOKE_REPORT_v0.5.4.md) §6 for the existing isolation pattern.
* **Reversible by the user.** Every learned preference must be undoable by a plain-English reply. A learning system the user can't undo is not safe.
* **Auditable.** The user (and the founder, during review) can ask "why did you send / not send me this?" and get an answer that names the contributing signals.
* **Single corrections do not flip.** One "not important" reply lowers a score; it does not flip a suppression. Hard suppressions require an explicit `ignore_sender` reply or N≥k consistent corrections within a recency window.
* **Real or absent.** A feedback event kind without an aggregator, or a memory signal kind without a ranker consumer, must not ship. The learning loop is end-to-end or it does not exist. (Carry-forward from [§22 Safety Lessons](#22-safety-lessons-from-brevio-audit).)

### What this does NOT change about the existing safe-learning tier discipline

The L0–L5 safe-learning tier table in §13 above still governs *all* learning, including Personalized Importance Learning. Specifically:

* Recording a feedback event (L0): no approval needed.
* Adjusting sender importance from explicit user replies (L1): no approval, but must be reversible.
* Inferring a preference Brevio wants to *propose* to the user ("hold school emails until 7pm"): L2 — Brevio proposes; the user decides.
* Changing alert behavior significantly: L3 — explicit user approval required.

Personalized Importance Learning operates entirely within L0–L2. Anything that wants to escalate to L3+ runs its own approval prompt and its own 6-question gate.

### Scope boundary

This section establishes the *principle*. It does not implement anything. The implementation is a future phase (see [FOMO_PLAN.md §17 — Personalized Importance Learning / False-Positive Reduction](FOMO_PLAN.md)). Implementation must pass a dedicated 6-question gate; the proposed gate questions are in [docs/personalized-importance-learning.md §13](docs/personalized-importance-learning.md).

### Version not locked — may be pulled forward before v1.0

**This phase may be pulled forward before v1.0 if friend-beta false positives damage trust.** It is not a post-v1.0 concept by default. The issue is already appearing in v0.5.x testing (urgent-sounding spam and commercial emails classified as important during the v0.5.2 Morris and v0.5.4 Sheila smokes). If similar false positives surface as a trust blocker in further v0.5.x friend-beta work, this phase is scheduled ahead of the v1.0 wedge decision rather than after it. Position in [FOMO_PLAN.md §17](FOMO_PLAN.md) reflects this: the future-phase candidate is intentionally listed *before* v1.0, not after.

---

## 14. Memory-First Architecture

The memory promise is:

> **Brevio remembers what matters, links memories to sources, learns from feedback, and lets the user inspect, correct, delete, or restrict memories.**

Do not say “never forgets.”

The assistant should feel like it remembers well, but memory must be safe, sourced, correctable, and deletable.

### Memory layers

> **Reserved ≠ active.** A "reserved" entry in the table below is a *future option preserved by design*, NOT a live dependency. As of v0.5.x, only Neon Postgres + Postgres event tables are active for memory. Adding a reserved layer (Upstash Redis, pgvector, Cloudflare R2) requires an approved current phase with a real caller — see §23 for the canonical rule.

| Layer                    | v0.5.x state           | Purpose                                     |
| ------------------------ | ---------------------- | ------------------------------------------- |
| Short-term working state | Upstash Redis — RESERVED (no active caller in v0.5.x) | locks, rate limits, active sessions (future) |
| Exact long-term memory   | Neon Postgres — ACTIVE | users, tools, permissions, alerts, feedback |
| Semantic memory          | pgvector — RESERVED (no active caller) | future meaning-based memory search          |
| Raw artifacts            | Cloudflare R2 — RESERVED (no active caller) | future files, attachments, exports          |
| Episodic memory          | Postgres events — ACTIVE | successful/failed workflows               |

### v0.1 memory signals

* sender importance,
* suppressions,
* user preferences,
* alert replies,
* snooze behavior,
* ignored alerts,
* why requests,
* false positives,
* false negatives,
* alert reasons,
* source event IDs,
* audit trail.

Every memory-like signal should have:

* user_id,
* source,
* timestamp,
* confidence if inferred,
* sensitivity where relevant,
* audit trace,
* delete/restrict path later.

---

## 15. Tool-First Architecture

Brevio should act through tools/APIs, not pure text generation.

Every action should be:

* deterministic,
* traceable,
* reversible where possible,
* permissioned,
* logged,
* schema-validated,
* rate-limited,
* tested.

### Tool classes

#### Context tools

They read information.

Examples:

* Gmail read,
* future Calendar read,
* future memory search,
* future contacts read.

#### Action tools

They do something.

Examples:

* SendBlue send to user,
* future email send,
* future calendar create,
* future booking,
* future purchase.

#### Control tools

They manage safety and state.

Examples:

* request permission,
* ask approval,
* write audit log,
* revoke consent,
* pause alerts,
* update memory signal.

v0.1 should only activate safe context/control tools and the user’s own messaging channel.

---

## 16. Planning + Execution Separation

The model should not do everything.

In v0.1:

* model ranks emails,
* model classifies replies,
* deterministic code executes actions,
* policy gate allows/blocks,
* Slack/founder approves,
* state machine tracks outcome.

Long-term:

* planner model creates plans,
* executor performs tool calls,
* critic checks outputs,
* validator enforces rules/sources,
* human approves risky steps.

But v0.1 must stay narrow.

The v0.1 planner is not a full planner.

The v0.1 planner is mostly:

* email importance classifier,
* reply intent classifier,
* tool-need recognizer for Gmail connection.

---

## 17. Context Compression

Do not keep stuffing entire histories into prompts.

The assistant should retrieve and include only high-signal context.

For v0.1 ranking, the model should see:

* sender,
* subject,
* limited safe snippet if allowed,
* sender importance,
* suppressions,
* relevant user preferences,
* recent feedback patterns,
* daily cap state.

The model should not see:

* full inbox history,
* unrelated memories,
* all prior chats,
* full raw email bodies unless explicitly allowed in founder-only dev.

This keeps cost lower and privacy safer.

---

## 18. Model Routing and Cost Optimization

Do not blindly use the cheapest model.

Do not blindly use the fanciest model.

Run a bake-off.

### Current active provider direction

> **Brevio is OpenAI-first as of v0.5.x.** The active ranker model is an OpenAI GPT-class model selected after the v0.1 bake-off (see `fomo.ranker.enabled` boot log + KERNEL.md for the live model id). Other model providers (Anthropic Claude small/mid/strong, future open-weight providers, future MCP-served models) remain documented as **future router options** — they are NOT active and are NOT a fallback path that fires silently.

> **Any provider change requires an explicit eval + smoke gate.** Swapping the active provider, adding a second active provider, or introducing a router that splits traffic between providers is a phase-scale change. It needs (a) a fresh model bake-off across the comparison axes below, (b) a per-phase 6-question gate, and (c) a founder-only smoke before it goes live. No silent provider swaps. No "we'll see how it does in production" router behavior.

### Future-option candidates (NOT active — for the next router bake-off, when one is scheduled)

* GPT small/mini model — currently active for `classification`
* GPT stronger model — future router upgrade path
* Claude Haiku / small appropriate model — future router option, not active
* Claude Sonnet / strong appropriate model — future router option, not active
* Open-weight or self-hosted models — future option only; no current commitment

### Compare on (every bake-off)

* precision,
* recall,
* false positives,
* false negatives,
* JSON reliability,
* explanation quality,
* latency,
* cost per 1,000 emails.

v0.5.x uses one active capability tag:

* `classification`

Future capability tags should be added only when real callers exist.

Track:

* model_name,
* prompt_version,
* latency,
* estimated cost,
* schema_valid,
* rank score,
* final gate decision.

If all model routes fail, fail closed. Do not send.

---

## 19. Failure Recovery Layer

Every external step needs a recovery story.

v0.1 recovery examples:

| Failure                         | Recovery                                             |
| ------------------------------- | ---------------------------------------------------- |
| Gmail OAuth expired             | mark connection revoked, stop polling, ask reconnect |
| model fails                     | mark alert failed, do not send                       |
| model output invalid            | fail closed                                          |
| SendBlue send fails             | retry safely with idempotency                        |
| SendBlue 2xx but DB write fails | mark `send_status_unknown`, manual reconciliation    |
| duplicate webhook               | ignore via unique ID                                 |
| unclear reply                   | clarify or log only                                  |
| daily cap hit                   | gate out                                             |
| kill switch off                 | no send                                              |

The system should prefer no action over unsafe action.

---

## 20. Observability Everywhere

If we cannot debug the agent, we cannot scale it.

v0.1 should trace:

* user_id,
* request_id,
* alert_id,
* tool_id,
* model_name,
* prompt_version,
* tokens,
* cost,
* latency,
* policy decision,
* state transition,
* error,
* retry count,
* user feedback event.

Use:

* Postgres audit tables,
* tool_invocations,
* alert_state_transitions,
* structured JSON logs.

Langfuse/Sentry/PostHog can come later.

---

## 21. Continuous Evals

Prompting is not enough.

The agent needs continuous tests.

v0.1 evals:

* email importance eval,
* false-positive eval,
* false-negative eval,
* reply parser eval,
* STOP/START eval,
* safety-block eval,
* assistant voice eval,
* model JSON reliability eval,
* regression eval.

Production drift can silently ruin an agent. Evals are not optional.

---

## 22. Safety Lessons From Brevio Audit

The old Brevio codebase had ambitious ideas but dangerous runtime gaps.

This design must not repeat:

* disconnected memory persistence,
* fake/stub tools pretending to work,
* LLM-self-asserted autonomy,
* unchecked tool arguments,
* missing schema validation,
* consent bypass,
* spoofable consent/auth headers,
* fail-open security checks,
* global TLS disable,
* silenced security/compliance errors,
* missing audit logs,
* raw email body leakage,
* committing private emails to repo,
* cross-user learning without consent,
* half-wired cost tracking,
* phantom services,
* multiple tables for the same concern.

The rule:

> **Real or absent. Never half-wired.**

---

## 23. No-AWS Cheap Stack

v0.1 stack:

> **Reserved ≠ active. Permanent rule.** A "reserved" entry in the table below is a *future option preserved by design* — it appears here so that when its time comes, the architecture choice is already made and consistent. It does NOT mean the dependency is live, available, or safe to import into runtime code. **Do NOT add Redis, R2, pgvector, Langfuse, Sentry, PostHog, or any other reserved capability to `apps/fomo` unless an approved current phase has a real caller for it.** Adding an unused dependency creates supply-chain risk, secrets to manage, observability noise, and false signal that "this is wired" when it isn't. If you find yourself reaching for a reserved entry mid-phase, stop and re-gate the phase scope.

| Layer                    | v0.5.x state                                                                      |
| ------------------------ | --------------------------------------------------------------------------------- |
| Frontend/onboarding      | Vercel + Next.js — ACTIVE                                                         |
| Backend/webhooks/workers | Render Starter or equivalent always-on Node runtime — ACTIVE                      |
| Database                 | Neon Postgres — ACTIVE                                                            |
| ORM                      | Drizzle ORM — ACTIVE                                                              |
| Vector memory            | pgvector — RESERVED (no active caller in v0.5.x)                                  |
| Cache/locks/rate limits  | Upstash Redis — RESERVED (no active caller in v0.5.x)                             |
| File/object storage      | Cloudflare R2 — RESERVED (no active caller in v0.5.x)                             |
| Messaging                | SendBlue — ACTIVE; Linq — RESERVED (future adapter, no active caller)             |
| Models                   | OpenAI — ACTIVE (see §18); Anthropic + others — RESERVED (future router options)  |
| Observability            | Structured logs + Postgres audit — ACTIVE; Langfuse / Sentry / PostHog — RESERVED |

No AWS direct dependency in `apps/fomo`.

---

## 24. Target User and Wedge

v0.1 users:

* founder first,
* then 3–5 close friends after gates pass.

The first test is not:

> “Can we build a cool agent?”

The first test is:

> “Can this assistant catch emails users would regret missing while staying quiet enough to trust?”

If users do not say “keep this running,” the wedge is wrong.

---

## 25. Success Criteria

After four weeks, FOMO is promising if:

1. founder uses it daily,
2. founder checks inbox less,
3. at least 3 of 5 friends remain active,
4. at least one friend says “keep this running,”
5. false positives are under target,
6. false negatives are under target,
7. no duplicate sends,
8. no raw email leakage,
9. memory signals are being updated,
10. feedback events are being stored,
11. model evals are running,
12. safety gates are working,
13. users can text naturally,
14. no fake/stub active code ships.

If FOMO works, L2 follows.

If not, restart the wedge.

---

## 26. Distribution Plan

No public launch in v0.1.

Distribution:

1. founder self-use,
2. manual friend onboarding,
3. 3–5 close friends,
4. no marketing until pull exists.

The friend beta should feel personal and careful.

The founder should explain:

* what FOMO reads,
* what it does not do,
* what founder review means,
* what data is visible,
* how STOP works,
* how to leave.

---

## 27. Historical v0.1 Build Path

> **Historical reference only. Do NOT interpret Phase 0–6 below as pending work.** Every phase listed here has already shipped: v0.1 founder demo (KERNEL + FOMO workflow + Slack approval + SendBlue iMessage), v0.5.1 multi-tenant substrate, v0.5.2 first real-friend smoke (Morris), v0.5.3 production hardening, v0.5.4 second-friend cross-tenant smoke (Sheila) — all with VERDICT: PASS. The "Decide" gate (Phase 6) is folded into the standing 6-question gate that runs before every new phase. Active phase work lives in [FOMO_PLAN.md §17 Implementation Milestones](FOMO_PLAN.md) and the latest `docs/SMOKE_REPORT_v0.5.*.md`.

The sub-sections below are preserved as a record of the build sequence Claude was originally given. They show *how the v0.1 substrate got built* — not what is currently open. A future reader debugging "why does the kernel look like this?" should read them; a future reader looking for "what should I do next?" should NOT.

### Phase 0 — Rewrite plan before coding (HISTORICAL — COMPLETE)

Update `FOMO_PLAN.md` around this new design.

Do not code until the plan is approved.

### Phase 1 — Repo cleanup and salvage audit (HISTORICAL — COMPLETE)

* new branch,
* no history rewrite,
* build green,
* classify modules:

  * KEEP_NOW,
  * PRUNE_NOW,
  * ARCHIVE_FOR_FUTURE,
  * KILL_CONFIDENTLY,
* preserve future concepts in `docs/future-architecture-notes.md`,
* no fake active code.

### Phase 2 — Minimal MCP OS kernel (HISTORICAL — COMPLETE)

Build the smallest real kernel slice:

* Tool Registry,
* Gmail OAuth,
* Permission Gate,
* Audit Log,
* Model Router,
* Memory Signals,
* Feedback Events,
* State Machine,
* Safe Logger.

### Phase 3 — FOMO workflow (HISTORICAL — COMPLETE)

Build:

* Gmail polling,
* ranking,
* policy gate,
* Slack founder review,
* SendBlue alert,
* reply parser,
* feedback/memory update.

### Phase 4 — Founder demo (HISTORICAL — COMPLETE)

Founder receives a real SendBlue iMessage for a real Gmail alert, after Slack approval.

### Phase 5 — Friend beta (HISTORICAL — COMPLETE through v0.5.4)

Substrate, first real friend (v0.5.2 Morris), production hardening (v0.5.3), and second-friend cross-tenant proof (v0.5.4 Sheila) all PASS. Three-friend beta cap (locked 2026-06-03): Friend B was the last GUARANTEED smoke; Friend C is OPTIONAL, not auto-scheduled.

### Phase 6 — Decide (FOLDED INTO STANDING 6Q GATE)

The original "continue, pivot, or kill" gate has graduated into the standing **6-question gate** every phase must pass before code is written (see [feedback_two-question-gate](.) memory and [docs/personalized-importance-learning.md §13](docs/personalized-importance-learning.md)). It is no longer a one-shot v0.1 milestone; it runs at the start of every new phase.

---

## 28. Decision Summary

* FOMO remains the first user-facing wedge.
* v0.1 is now a simple demo of the larger Brevio agent.
* The architecture is a minimal MCP-style agent OS.
* Gmail is the first context provider.
* SendBlue is the first messaging tool.
* Slack is the first trust checkpoint.
* Memory signals and feedback events are the first learning layer.
* Tool Registry, Permission Manager, OAuth, Egress, Audit, and State Machine are the first gateway/kernel pieces.
* v0.1 must stay narrow.
* No AWS.
* No bookings, purchases, email sends, calendar writes, browser control, shell access, or full MCP marketplace.
* The long-term goal is a humanistic iMessage assistant that learns the user, reasons about goals, safely asks for access, and uses tools through governed, observable, permissioned workflows.

The final rule:

> **Think like OpenClaw. Ship like FOMO.**

Or even more precisely:

> **Build the smallest safe MCP OS kernel, and run FOMO as its first real workflow.**
