## Brevio Orientation: What Claude Must Read First

These files are long. Do not skim them. Do not read them vaguely. Do not treat them like background noise.

Read them slowly, section by section, as if they are a prized possession of the project. They are the operating memory that prevents Brevio from becoming a shallow Gmail alert script.

Before designing, coding, or proposing any new phase, read these files in this order:

### 1. `FOMO_DESIGN.md` — the constitution

This is the long-term Brevio constitution. It defines the product vision, architecture laws, safety philosophy, and the difference between FOMO as the wedge and Brevio as the full proactive/autonomous personal agent.

Read it thoroughly before making product or architecture decisions.

Use this to understand **why** we are building Brevio.

### 2. `FOMO_PLAN.md` — the active implementation map

This is the current implementation plan. It contains the phase structure, smoke gates, idempotency rules, reply-parser pipeline, safety gates, and current build path.

Read it thoroughly before deciding what work comes next.

Use this to understand **what phase we are in** and **what is allowed now**.

Read `apps/fomo/KERNEL.md` after `FOMO_PLAN.md`.

It is the current live-state map of the FOMO kernel slice: active invariants, phase state, gates, completed work, blocked work, and what must not regress.

Do not treat `KERNEL.md` as the full Brevio vision. It is the working kernel slice that should accelerate the full Brevio build. The long-term vision lives in `FOMO_DESIGN.md` and `docs/brevio-core-agent-dimensions.md`.

Use `KERNEL.md` to avoid stale assumptions, duplicate work, and regressions while expanding from FOMO into the full Brevio agent OS.

### 3. `docs/brevio-core-agent-dimensions.md` — the full Brevio direction

This is the permanent map of the 12 core Brevio agent dimensions:

* Autonomy
* Proactivity
* Memory Architecture
* Agent Core + Reasoning
* Tool / Workflow Orchestration
* Security / Permission Gates
* Multimodal + Perception
* Feedback + Learn/Grow Loop
* Human Message Renderer
* Observability / Evals / Reliability
* Production Scale
* User Trust / Consent

Read every dimension. Do not skim.

Before starting any phase, Claude must state which dimensions the phase:

* advances,
* preserves,
* intentionally defers.

If a phase cannot name at least one Brevio core dimension it advances, it is probably FOMO-only polish and must be surfaced to the founder before proceeding.

### 4. `docs/brevio-product-philosophy.md` — the permanent product layers

Three permanent Brevio product layers guide every future phase:

1. **Human Message Renderer**
   Brevio turns structured context into natural, useful, human-feeling messages.

2. **Personalized Importance Learning**
   Brevio learns each user’s personal definition of important without cross-user leakage or over-aggressive suppression.

3. **Feedback + Learn/Grow Loop**
   Brevio learns from user feedback and grows better at serving the user over time.

Read this carefully before touching user-facing behavior.

#### Brevio core agent dimensions standing rule

Brevio has 12 permanent core agent dimensions defined in `docs/brevio-core-agent-dimensions.md`. FOMO is the wedge, not the product. Before starting any phase, Claude must state which Brevio core dimensions the phase: advances, preserves, intentionally defers. If Claude cannot name at least one dimension being advanced, the phase is FOMO-only polish — surface that and reconfirm before proceeding. Every future 6-question gate must include this Core Dimension Check in addition to the three principle-gate questions from `docs/brevio-product-philosophy.md`.

Before changing user-facing messages, ranker behavior, reply parsing, feedback events, memory signals, or workflow behavior, read this file.

Every future 6-question gate must include the three principle-gate questions from this doc in addition to the phase-specific Q1–Q6.

### 5. `docs/BREVIO_MEMORY_AND_SKILL_OS.md` — M0 memory + skill doctrine

This is the canonical architecture document for Brevio's external typed memory, retrieval, consolidation, skill registry, and skill lifecycle. It defines what Brevio learns, where learning lives, what stays outside the base model, and the M1–M6 implementation roadmap.

Read this BEFORE proposing any memory write path, skill candidate, skill execution, autonomy expansion, or Composio integration. M1 (Typed Memory Substrate) is the canonical next implementation step after v0.7.0A.

### 6. `docs/personalized-importance-learning.md` — false-positive and personalization doctrine

Do not treat Brevio’s false-positive problem as a prompt-only bug.

Personalized Importance Learning is a permanent product principle. Before changing ranker behavior, commercial/spam handling, reply parser behavior, feedback events, or memory signals, read this doc thoroughly.

Brevio must learn each user’s definition of important while avoiding:

* cross-user leakage,
* over-aggressive suppression,
* raw private email storage,
* global rules from one user’s preference,
* treating all commercial email as unimportant.

### 7. `SALVAGE_MAP.md` — repo history and what was kept/pruned

Read this before reviving old code or assuming archived modules are active. It explains what was kept, pruned, archived, or killed.

Use it to avoid reintroducing half-wired old Brevio code.

### 8. `docs/future-architecture-notes.md` — long-term archived concepts

Read this before designing anything above L1, including:

* Calendar
* Drafting
* Sending
* MCP tools
* Browser automation
* Autonomous workflows
* Memory expansion
* Delegated agents

This file preserves future ideas without making them active runtime code.

### 9. `FRIENDS.md` and `OUTREACH.md`

These are fill-in-the-blanks templates for human-conversation tasks. They are not architecture docs.

Use them only for friend onboarding, outreach, briefing, and aftercare.

---

## Permanent Rule: FOMO Is The Wedge, Not The Destination

FOMO is valuable only because it builds reusable Brevio agent OS layers.

Do not let Brevio collapse into a polished Gmail alert product.

Before proposing any phase, ask:

1. Which Brevio core dimensions does this phase advance?
2. Which dimensions does it preserve but not advance?
3. Which dimensions are intentionally deferred?
4. Does this phase risk shrinking Brevio into FOMO/email-only work?
5. Is this only FOMO polish? If yes, why should it not be deferred?
6. What smoke/eval proves the reusable Brevio layer works?

If the answer is weak, pause and surface the concern before coding.

---

## Permanent Rule: Real Or Absent

No half-wired features.

A feature is either:

* real,
* tested,
* gated,
* audited,
* smoke-proven when it touches a real provider,

or it is absent.

Do not add fake future infrastructure, phantom tools, inactive abstractions, or dead-end scaffolding unless it is explicitly part of an approved docs-only or scaffolding phase.

---

## Permanent Rule: Speed With Scope

Move fast, but only inside locked scopes.

Claude should not do “while I’m here” work.

Every implementation phase requires:

* 6-question gate,
* Core Dimension Check,
* locked scope,
* explicit out-of-scope list,
* tests,
* evidence,
* smoke gate if provider/user-facing,
* founder approval before merge.

---

## Permanent Rule: Human, Learning, Safe

Brevio’s long-term identity is:

> Speak like a human.
> Learn what matters.
> Grow from feedback.
> Act only when allowed.

That means:

* do not dump metadata as UX,
* do not treat learning as analytics only,
* do not make feedback robotic,
* do not silently infer and act,
* do not silently expand access,
* do not silently perform high-risk actions.

The AI may propose.
The system gates.
The user approves.

---

## Current Strategic Bias

After active HMR / runway work, the next serious build phase should be chosen by **core Brevio dimension impact**, not by “what FOMO bug is next.”

Likely next strategic candidate:

**Feedback + Learn/Grow Loop substrate**

Reason: it supports proactivity, memory architecture, Personalized Importance Learning, Human Message Renderer, future calendar, coffee/stocks/travel behaviors, and the assistant’s ability to serve the user better over time.

Do not implement it without its own 6-question gate.
