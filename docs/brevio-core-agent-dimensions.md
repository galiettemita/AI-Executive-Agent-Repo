# Brevio Core Agent Dimensions

> Founder-locked 2026-06-06. **Permanent** dimensions that define what Brevio IS as an agent. FOMO is the wedge — not the product. Brevio's founding goal is a proactive, autonomous, memory-rich, permissioned personal agent. Every phase must advance, preserve, or intentionally defer specific dimensions below. No phase exists outside this frame.

## Read this BEFORE picking the next phase

The next serious Brevio phase is not picked by "what FOMO bug is next." It is picked by **which core dimension it advances.** FOMO-only polish that does not advance a core dimension is deferred.

The current strategic candidate (as of 2026-06-06) is **Feedback + Learn/Grow Loop substrate** — it advances dimensions [Feedback + Learn/Grow Loop], [Memory Architecture], [Agent Core + Reasoning], [Proactivity], [Personalization-as-PIL], and underwrites every future coffee / stocks / travel / calendar autonomy. See "What comes next" section at the bottom.

## The 12 permanent dimensions

Every dimension has seven required fields:

1. **Long-term meaning** — what this dimension means in the full Brevio agent vision
2. **What exists today in FOMO** — the honest current state in `apps/fomo/`
3. **What is missing** — the gap between current and the long-term meaning
4. **Future build path** — the rough sequence that closes the gap
5. **What must never be faked** — anti-patterns that would destroy user trust
6. **Smoke/eval proof required** — the bar a phase claiming to advance this dimension must clear
7. **Examples / risks if skipped** — what happens to Brevio if this dimension is never built

---

### 1. Autonomy

**Long-term meaning.** Brevio acts on the user's behalf without constant prompting. The user delegates outcomes (book the coffee, file the form, draft the reply), not micro-tasks. Autonomy is tiered (Watch → Suggest → Draft → Send) and bounded by per-user, per-tool trust and permission grants.

**What exists today in FOMO.** Zero real autonomy. v0.5.x is fully approval-gated — every iMessage send requires the founder to click Approve in Slack. `FOMO_AUTO_SEND_ENABLED=true` is explicitly a hard error in preflight.

**What is missing.** Bounded autonomy tiers. Per-user trust levels. Per-tool autonomy budgets. Reversibility timer ("I'll send X in 60 seconds — say stop to cancel"). Risk-class-based gating.

**Future build path.** After PIL + Feedback substrates ship, tier-1 autonomy can unlock for low-stakes, reversible actions ("I auto-suggested this draft — want me to send?"). Higher tiers gated by trust score + per-tool risk class. Reversibility is the entrance ticket.

**What must never be faked.** NO claiming autonomy where the behavior is scripted. NO dark patterns (auto-send disguised as suggestion). NO "Brevio decided X" when the decision was deterministic. NO unbounded autonomy without explicit per-tool consent.

**Smoke/eval proof required.** For each autonomy tier: real-user smoke that proves (a) user understood the tier before grant, (b) revocation works mid-flight, (c) Brevio's actions match the granted tier, (d) reversal/undo path is real and tested.

**Examples / risks if skipped.** Brevio remains an alert system, never becomes an agent. The user must manually approve every action forever — that ceiling caps Brevio's product value. Competitors with even tier-1 autonomy ("I drafted this, want me to send?") win the user's day-to-day. The "personal assistant" promise reads as marketing copy, not a real product.

---

### 2. Proactivity

**Long-term meaning.** Brevio surfaces things the user *should* know but didn't ask about. Calibrated by user context (busy/free, awake/asleep, working/personal). Not nagging. Not silent. Calibration is per-user and learned.

**What exists today in FOMO.** Interval-based polling of one signal source (Gmail), one classifier (FOMO ranker), one delivery (iMessage via SendBlue). Founder approval gate. Hourly cadence — interval-based, not signal-based.

**What is missing.** Event-triggered proactivity (calendar conflicts, deadline approaching, package never arrived). Cross-tool reasoning ("email says deposit needed" + "calendar says you're traveling" → surface the conflict). Cadence learning (user wants more vs less proactivity at certain times). Quiet hours.

**Future build path.** Migrate from interval-based polling to signal-based triggers (calendar events, inbound webhooks, deadline approaching). Cadence informed by PIL. Multi-tool reasoning when an event implies action across tools. Quiet-hours respect baked in.

**What must never be faked.** NO proactive ping without a real signal. NO inventing context to look smart. If Brevio doesn't have data, it stays quiet. NO nag-cycles ignoring user silence.

**Smoke/eval proof required.** Proactivity smokes need REAL trigger events (a real calendar conflict, a real deadline approaching). Eval: did the surfaced proactivity match a user-needed action vs noise? Precision/recall over time tracked.

**Examples / risks if skipped.** Brevio is reactive — the user has to ask it to check things. The user misses a real deadline because Brevio waited politely. Cross-tool reasoning never happens, so the "agent that connects email + calendar" promise stays vapor. Competitors with even crude calendar-conflict detection look smarter.

---

### 3. Memory Architecture

> **Canonical doctrine: [`docs/BREVIO_MEMORY_AND_SKILL_OS.md`](BREVIO_MEMORY_AND_SKILL_OS.md) (M0).** The eleven typed memory kinds, retrieval rules, consolidation loop, and explainability surfaces live there.

**Long-term meaning.** Brevio remembers what matters to each user across time and surfaces. Per-user, multi-modal (facts, preferences, past corrections, decisions). Reversible. Explainable to the user.

**What exists today in FOMO.** `memory_signals` table with 8 kinds (`stop_active`, `sendblue_contact_status`, etc.). Per-user keying. Audit-logged via `audit_log`. Reversible (DELETE / UPDATE active=false). `rank_results` and `audit_log` provide short-term decision memory.

**What is missing.** Long-term factual memory ("user prefers digest at 8am Mondays"). Cross-conversation memory ("you asked me about this last week — here's what we decided"). Memory decay policy. Memory-explanation UX ("here's why I'm doing X — see what I remember about you"). Privacy export / delete.

**Future build path.** Add `memory_facts` (long-term store) + `memory_preferences` (explicit user preferences) + memory-explanation surface (HMR-rendered). PIL writes its learned signals into memory. Decay policy per memory kind. Export + erase per user.

**What must never be faked.** NO inventing remembered facts. NO claiming to remember when it doesn't. NO cross-user memory leakage. NO opaque "Brevio remembers" without the user being able to view, edit, or delete what Brevio remembers.

**Smoke/eval proof required.** Memory smoke verifies (a) memory survives restart, (b) per-user isolation byte-identical (carry-forward from v0.5.4 cross-tenant checks), (c) reversal/undo works, (d) memory explanation reads honestly (matches what's stored, not aspirational).

**Examples / risks if skipped.** Brevio re-learns the user every session. Users tell Brevio the same preference 50 times and nothing sticks. PIL has nothing to read from. "I told you last week" failures destroy trust. The user can't ask "what do you know about me?" because the answer is: nothing durable.

---

### 4. Agent Core + Reasoning

**Long-term meaning.** Brevio's reasoning loop — perceive → plan → act → observe → learn. Multi-step plans with rollback. LLM-driven decisions bounded by deterministic guardrails. Auditable reasoning.

**What exists today in FOMO.** The ranker is the only "reasoning" — a single classification step (important vs not). No multi-step plans. No rollback. The outbound-sender is a fixed state machine, not a planner.

**What is missing.** Multi-step planner ("read email → check calendar → identify conflict → draft reply → ask user"). Plan persistence (the agent's plan is auditable). Rollback (if step 3 fails, step 1's side effects unwind). Reasoning observability.

**Future build path.** Per-task plan objects (stored, auditable). Step-level audit. Per-step gate via existing policy-gate. LLM-driven plans bounded by allowed-tool lists. Rollback hooks per side-effect. ACP (Agent Client Protocol) may become an operator/developer interface for Brevio agents, IDE agents, and internal coding-worker handoffs, but it is not a replacement for Brevio's product runtime, tool gateway, permission gate, or audit model.

**What must never be faked.** NO presenting a plan that wasn't actually executed. NO hiding reasoning steps from audit. NO unbounded LLM autonomy. NO "the agent decided X" without a captured chain-of-decisions.

**Smoke/eval proof required.** For each new reasoning capability: smoke proves the plan-execute-observe loop produces the right outcome AND the audit trail captures every step. Adversarial: inject failure mid-plan, verify rollback.

**Examples / risks if skipped.** Brevio is a one-shot classifier — every new capability requires a hard-coded state machine. Multi-step delegations are impossible ("plan my Thursday around this email"). The agent cannot recover from partial failures; every error becomes "user, please retry." Brevio's intelligence ceiling is set by what a single LLM call can decide.

---

### 5. Tool / Workflow Orchestration

> **Skill discipline: [`docs/BREVIO_MEMORY_AND_SKILL_OS.md`](BREVIO_MEMORY_AND_SKILL_OS.md) §6–§7 + §12.** Brevio skills are versioned, founder-approved, permission-classified artifacts that COMPOSE provider tools (including Composio actions) through Brevio's own wrapper — not raw tool calls.

**Long-term meaning.** Brevio chains tools (Gmail, Calendar, Drive, web, SMS, payments, browser, etc.) to complete user-outcomes. MCP-style tool calls. Cross-tool reasoning. Tool failures degrade gracefully.

**What exists today in FOMO.** Three tools wired: Gmail (read), Slack (review card), SendBlue (iMessage out). Each tool is single-step. No chaining. `EmailContextProvider` abstraction sketched for future Outlook/iCloud/IMAP.

**What is missing.** Calendar tool (read events, check conflicts). Drafts tool (write Gmail drafts). Memory tool (read/write per-user facts). Web tool (browse a URL the email referenced). Cross-tool plans. Tool failure → graceful degradation.

**Future build path.** Add tools as separate phases (each its own 6Q gate). Each tool gets its own egress policy, audit kinds, kill switch, smoke. The agent's planner picks the tool chain. Per-tool risk class + permission gate. Keep MCP-style tools/adapters as the user-product spine; if ACP is introduced, use it as an internal/operator control plane and route any ACP-exposed capability through the same Brevio registry, risk-tier, consent, egress, audit, and kill-switch rules.

**What must never be faked.** NO claiming a tool was called when it wasn't. NO silent tool fallback. NO ignoring tool errors. Every tool call audited. NO bypassing egress policy "just this once."

**Smoke/eval proof required.** Each new tool: smoke proves (a) tool works against real provider, (b) failure modes audited (no retry on idempotency-unsafe), (c) egress policy enforced (zero raw content leakage to logs / Slack / model), (d) kill switch works.

**Examples / risks if skipped.** Each tool stays a silo. The "agent that connects your tools" promise is hollow. Bolt-on integrations multiply complexity (N tools × M flows). Cross-tool reasoning never lands so the killer "read email + check calendar + draft reply" flow never ships. Brevio's value collapses to "one inbox, one alert."

---

### 6. Security / Permission Gates

**Long-term meaning.** Per-tool, per-action, per-user permission gates. Auditable consent. Revocable. Brevio NEVER acts beyond what the user explicitly granted. Risk-class-based gates (read-only / draft-only / send-on-behalf / spend-money).

**What exists today in FOMO.** Kill switches (`FOMO_SEND_ENABLED`, `FOMO_AUTO_SEND_ENABLED`, etc.). Per-user OAuth tokens (encrypted with `BREVIO_TOKEN_KEK`). `policy-gate` for tool dispatch. STOP enforcement (v0.5.5). SendBlue OPTED_OUT drift detection (v0.5.3). Founder phone allowlist. Per-user encrypted columns vs hashed columns (v0.5.1 multi-tenant principles).

**What is missing.** Per-action consent (not just per-tool). Consent UX surfaces (HMR-rendered "Brevio wants to do X — approve?"). Consent revocation flow. Consent expiry. Risk-class-based gates. Per-user consent_grants table.

**Future build path.** Build `consent_grants` table (per-user, per-action, per-risk-class, expiry, audit). Build consent UX (HMR-rendered ask). Build revocation. Tie autonomy tiers to consent. Consent renewal.

**What must never be faked.** NO acting without explicit consent. NO claiming consent when it's stale/revoked. NO "implicit consent" for high-risk actions. NO consent UX that pressures the user. NO defaults set to broadest grant.

**Smoke/eval proof required.** Per-consent: smoke verifies grant → action → audit → revoke → no further action. Adversarial: try to act after revoke, confirm refusal. Stale grant expires, audit fires.

**Examples / risks if skipped.** First scary action collapses user trust ("Brevio just sent an email to my boss without asking"). User revokes everything in panic and never returns. Without per-action gating, Brevio is forced to stay at tier-0 (approval-gated) forever — autonomy is locked out. Legal / compliance surfaces (HIPAA, FERPA, GDPR) become impossible to clear.

---

### 7. Multimodal + Perception

**Long-term meaning.** Brevio reads not just email but PDFs, images, voice notes, calendar events, web pages, screenshots. Understands the full user context across modalities.

**What exists today in FOMO.** Plain-text email body only. Subject + sender. HTML stripped. NO attachments parsed. NO images. NO PDFs. NO voice. NO web pages. Single modality.

**What is missing.** PDF parsing (e.g., contract attachments). Image understanding (screenshots, photos). Voice note transcription. Web page reading (when an email links to a doc the user needs). Calendar event content. Screen recording perception (future).

**Future build path.** Add perception inputs as separate phases. Each input type: egress policy + ranker integration + audit. Pure-deterministic OCR / transcription where possible; LLM where unavoidable (with explicit 3E.1-style carve-outs).

**What must never be faked.** NO claiming to have read content that wasn't actually read. NO inventing image descriptions. NO transcription that wasn't actually transcribed. NO hallucinating PDF contents.

**Smoke/eval proof required.** Each input type: smoke verifies (a) real provider read, (b) egress policy enforced (no raw content to model / Slack / logs), (c) error modes (corrupt PDF, unreadable image) gracefully degrade with audit.

**Examples / risks if skipped.** Brevio is blind to half the modern inbox — a PDF contract arrives and Brevio shrugs; user voice-notes an idea and Brevio ignores; user shares a screenshot and Brevio can't tell what's in it. "I read your email but the important part was in the attachment" failures recur. Every new modality competitors handle widens the gap.

---

### 8. Feedback + Learn/Grow Loop

**Long-term meaning.** Brevio learns each user's preferences from natural feedback. Onboarding-heavy ("am I getting the right kinds of things?"), steady-state light, intelligent when uncertain. Feedback prompts are HMR output — calm and human, never robotic. Feedback is the input signal PIL learns from.

**What exists today in FOMO.** Conceptual only. No implementation. Reply parser exists for STOP/START commands (v0.5.5) but does NOT parse feedback intents.

**What is missing.** `feedback_event` kinds. Reply parser → feedback handoff. Per-user feedback storage. HMR-shaped feedback prompts (see voice rules). Onboarding-heavy ask-cadence policy. Reversibility / undo for feedback.

**Future build path.** v0.5.8 — Feedback substrate (capture + storage + audit + prompts). v0.5.9 — PIL layer on top reading from feedback signal. See [brevio-product-philosophy.md](brevio-product-philosophy.md) for the 3-layer principle.

**What must never be faked.** NO ignoring user feedback. NO storing feedback the user can't review. NO robotic feedback prompts ("Reply 'not important' if I got this wrong"). NO cross-user feedback leakage. NO inferring feedback from silence without explicit policy.

**Smoke/eval proof required.** Feedback smoke verifies (a) feedback captured per-user, (b) audit visible, (c) reversible, (d) prompts pass HMR voice rules ([brevio-voice-rules](../.claude/projects/-Users-galiettemita-Downloads-Executive-AI-Agent-backend/memory/feedback_brevio-voice-rules.md)), (e) onboarding-heavy ask-cadence demonstrable, (f) cross-tenant isolation byte-identical.

**Examples / risks if skipped.** Brevio stays exactly as smart as the day it shipped. User says "this isn't important" twenty times and Brevio keeps sending the same kind of alert. PIL cannot ship — there's no signal to learn from. Autonomy stays locked at tier-0 because trust can't grow without learning. Every coffee / stocks / travel / calendar autonomy idea hits the same wall.

---

### 9. Human Message Renderer

**Long-term meaning.** Brevio's user-facing voice across every surface. Natural, human, never robotic. Cross-surface consistency. Deterministic body composition; structured input only.

**What exists today in FOMO.** v0.5.7 in flight (PR #46). First surface: email alerts. `renderHumanMessage()` with Q1.A two-sentence + Q2.B sender chain + Q3.B subject strip + Q4.A ranker-v0.2.0 + Q5.A degradation + Q6.A audit fields. 3E.1 deterministic-body invariant preserved.

**What is missing.** HMR surfaces for calendar reminders, draft suggestions, task updates, booking/payment prep, tool results, browser summaries, "why did you send this?" answers, memory explanations, feedback prompts.

**Future build path.** Each new HMR surface its own 6Q gate. Same shape (sentence-shaped, deterministic body, voice-rules-compliant) but per-surface templates. Q6.A surface discriminator extended per phase.

**What must never be faked.** NO field-dumping as UX. NO LLM body-generation (3E.1 invariant). NO outsourcing to a vendor (Brevio owns HMR end-to-end). NO "feels like a person" framing for output that's still field-shaped (see [dont-oversell-renderer-tone](../.claude/projects/-Users-galiettemita-Downloads-Executive-AI-Agent-backend/memory/feedback_dont-oversell-renderer-tone.md)).

**Smoke/eval proof required.** Each surface: taste-check fixture (load-bearing per v0.5.7 C10 lock) + audit-field smoke + real-delivery smoke (opportunistic). 3E.1 import-tripwire test in the renderer's test suite.

**Examples / risks if skipped.** Every new surface re-creates Friend B's "robotic" reaction. Users experience Brevio as an alert system, not an assistant. The "speak like a human" promise of the brand collapses on the first calendar reminder that reads as a structured log. Each surface needs an ad-hoc copywriter instead of a shared rendering layer.

---

### 10. Observability / Evals / Reliability

**Long-term meaning.** Brevio's behavior is auditable, evaluatable, and reliable. Every decision logged. Every smoke reproducible. Every eval automated. SLOs measured and tracked.

**What exists today in FOMO.** Comprehensive `audit_log` (50+ audit kinds). 7 smoke-evidence scripts (v0.5.1 – v0.5.7). 1179 unit tests. CI green. Smoke-evidence verdicts (PASS / FAIL / PENDING). Memory-signals for state changes.

**What is missing.** Per-user observability dashboards. Eval harness for ranker (precision/recall over time on a labeled gold set). Continuous evals (not just one-shot smokes). Reliability SLOs (poll-cycle success rate, send latency p95, etc.). Trend tracking across audits.

**Future build path.** Build eval harness (gold-standard labeled email set + ranker eval over time). Build reliability dashboards. Per-user observability surface (founder/friend sees what Brevio did). SLO definitions per phase.

**What must never be faked.** NO smoke evidence padded with mock data. NO ignored audit gaps. NO claimed reliability without measurement. NO eval results without a labeled gold set.

**Smoke/eval proof required.** Eval harness has its own smoke (gold set evals run, results stored, baseline comparison). Reliability dashboards backed by real metrics, not mocked. Trend regression alerts.

**Examples / risks if skipped.** Ranker silently regresses for three months and nobody notices. Per-user issues hide in aggregate metrics. PIL trains on bad data because there's no eval to flag drift. "We tested it" claims become unfalsifiable. The first paying user hits a reliability cliff in week two and churns.

---

### 11. Production Scale

**Long-term meaning.** Brevio runs reliably for N users (10, 100, 1000+) with bounded latency + cost. Per-tenant isolation. Multi-region availability. Disaster recovery.

**What exists today in FOMO.** Single-Neon-pg substrate. Per-user keying (v0.5.1). v0.5.3 hardening (OAuth refresh, pg pool resilience, SendBlue reconciliation). Single-region (Render staging + Neon AWS us-east). Three real users (founder + Friend A + Friend B).

**What is missing.** Load testing. Concurrency limits per-user. Multi-region failover. Cost-per-user dashboards. Tenant quotas. Database scaling story (Neon → Postgres scaled cluster).

**Future build path.** Build load-test harness (N synthetic users, measure latency / cost / errors). Define per-tenant quotas. Cost-per-user dashboard. Disaster recovery runbook. Multi-region after N=100 users.

**What must never be faked.** NO claiming scale without load tests. NO ignored cost growth. NO "we'll figure out DR later" silence. NO masking per-tenant failures as global issues.

**Smoke/eval proof required.** Scale smoke: N synthetic users run for X hours; all SLOs met; no per-tenant data leakage; cost-per-user matches projection.

**Examples / risks if skipped.** Brevio works for 3 users and breaks at 100. The 50th user signs up and the poll cycle takes 10 minutes. Per-tenant data leaks under load. Cost-per-user balloons silently and the unit economics collapse. The first outage with paying users is unrecoverable because there's no DR runbook.

---

### 12. User Trust / Consent

**Long-term meaning.** Brevio earns user trust through consistent, honest, helpful behavior. Trust is per-user, measurable, slowly built, easily broken. Trust gates autonomy tiers and per-tool risk classes.

**What exists today in FOMO.** Founder + Friend A + Friend B onboarded. Three-friend cap. Per-user STOP/START (v0.5.5). Reversible OAuth (user can revoke). No cross-user data sharing. Privacy redaction in Slack cards (v0.5.1 multi-tenant principles).

**What is missing.** Trust-score model (per-user). Trust UX ("here's what I've learned about you — review/correct"). Trust-gated autonomy tiers. Consent renewal flow. Privacy export ("download everything I know about you"). Privacy delete ("forget me").

**Future build path.** Build `trust_score` per-user (computed from feedback + correction signals). Build trust UX (HMR-rendered explanation). Tie autonomy tiers to trust. Build privacy export + delete. Consent renewal cadence.

**What must never be faked.** NO claiming high trust where it isn't earned. NO acting beyond trust budget. NO opaque "Brevio knows best" disregarding user's stated preferences. NO trust-score nudges that pressure the user.

**Smoke/eval proof required.** Trust smoke: real user onboarded → trust grows from feedback signals → autonomy tier expands → user revokes → trust resets → behavior degrades to safe baseline. Per-user data isolation byte-identical.

**Examples / risks if skipped.** Trust is binary — granted on day 1, revoked on day 30 over a single bad action, no path back. There's no way to expand what Brevio does for a user because there's no measured trust to gate on. Privacy export / delete becomes a regulatory blocker (GDPR, future state privacy laws). Power users hit Brevio's ceiling because they can't grow Brevio's autonomy budget through good behavior.

---

## Phase-gate addition: Core Dimension Check

**Every future 6-question gate must include a Core Dimension Check before the per-phase Q1–Q6.** This is in addition to the three principle-gate questions from [brevio-product-philosophy.md](brevio-product-philosophy.md) ("does this preserve HMR / PIL / Feedback Loop?").

The Core Dimension Check asks:

1. **Which dimension(s) does this phase ADVANCE?** Name them explicitly. A phase that advances no dimension is FOMO-only polish — defer it.
2. **Which dimension(s) does this phase PRESERVE?** Name them explicitly. Especially the ones it touches but doesn't move forward (don't regress).
3. **Which dimension(s) does this phase INTENTIONALLY DEFER?** Name them explicitly. "We're not advancing X this phase" is fine; "we forgot about X" is not.

If the Core Dimension Check has no clear ADVANCE, the phase should not run. If it intentionally defers a dimension that the strategic context says should be next, surface the conflict with the founder before locking scope.

## Standing rule for Claude

Before starting any phase, Claude must explicitly state which Brevio core dimensions the phase:
- advances,
- preserves,
- intentionally defers.

If Claude cannot name at least one dimension being advanced, the phase is FOMO-only polish — surface that and reconfirm before proceeding.

## What comes next (as of 2026-06-06)

The strategic next-phase candidate is **Feedback + Learn/Grow Loop substrate** (v0.5.8).

Why this dimension is next:

- It advances [Feedback + Learn/Grow Loop] directly.
- It underwrites [Memory Architecture] (feedback events become memory facts over time).
- It underwrites [Agent Core + Reasoning] (the agent's plans get scored by user feedback).
- It underwrites [Proactivity] (cadence learning needs feedback as its input signal).
- It underwrites [Personalized Importance Learning] (PIL CANNOT ship before Feedback — see [brevio-product-philosophy.md](brevio-product-philosophy.md)).
- It underwrites every future autonomy tier (autonomy budgets are tied to trust, trust is earned via feedback).
- It is foundational for future coffee / stocks / travel / calendar / browser actions — every one of those surfaces will need to capture and learn from user reactions.

What it does NOT advance (deferred):

- [Autonomy] — no autonomy tiers in v0.5.8; substrate only.
- [Tool / Workflow Orchestration] — no new tools.
- [Multimodal + Perception] — no new modalities.
- [Production Scale] — no load testing this phase.

This pattern is the model for every future phase: declare what's advanced, what's preserved, what's deferred. Make the trade-offs explicit.

## Cross-references

Auto-memory:
- `feedback_brevio-human-message-renderer-principle` — HMR principle (operational)
- `feedback_brevio-voice-rules` — voice rules (cross-cutting)
- `feedback_3e1-no-llm-body-generation` — 3E.1 deterministic-body invariant
- `feedback_two-question-gate` — pre-phase confirmation discipline
- `feedback_smoke-test-gates` — smoke discipline
- `feedback_scope-isolation-and-hardening-discipline` — phase isolation
- `feedback_real-or-absent-no-half-wired` — no half-wired features
- `feedback_multitenant-design-principles` — per-user keying invariants

Project docs:
- [brevio-product-philosophy.md](brevio-product-philosophy.md) — the three permanent product layers (HMR, PIL, Feedback)
- [personalized-importance-learning.md](personalized-importance-learning.md) — PIL substrate sketch
- [FOMO_DESIGN.md](../FOMO_DESIGN.md)
- [FOMO_PLAN.md](../FOMO_PLAN.md)
- [CLAUDE.md](../CLAUDE.md)

---

FOMO is the wedge. Brevio is the product. Every phase must advance a core dimension. No exceptions.
