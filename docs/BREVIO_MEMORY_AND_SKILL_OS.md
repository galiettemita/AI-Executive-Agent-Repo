# Brevio Memory + Skill OS Doctrine

> **Founder-locked: 2026-06-15. Documentation-only doctrine. No runtime memory write path, no skill execution, no DB migration is implied or implemented by this document.** When Brevio crosses from documentation into runtime, every step must enter through its own 6-question gate and Core Dimension Check (see [`CLAUDE.md`](../CLAUDE.md), [`docs/brevio-product-philosophy.md`](brevio-product-philosophy.md), [`docs/brevio-core-agent-dimensions.md`](brevio-core-agent-dimensions.md)).
>
> Status: M0 — canonical architecture doctrine for Brevio's external memory + skill OS. Companion to the permanent product layers ([HMR + PIL + Feedback](brevio-product-philosophy.md)), the 12 core agent dimensions ([`brevio-core-agent-dimensions.md`](brevio-core-agent-dimensions.md)), and the PIL invariants ([`personalized-importance-learning.md`](personalized-importance-learning.md)). Defines how Brevio learns, remembers, and reuses skill — without becoming a transcript-replay machine, an online-fine-tuned model, or a vendor's notion of memory.
>
> Read before designing **any** memory-related substrate, retrieval pass, consolidation job, skill candidate, skill execution path, or autonomy expansion.

---

## Table of contents

1. [Core thesis](#1-core-thesis)
2. [Memory architecture](#2-memory-architecture)
3. [What should NOT become memory](#3-what-should-not-become-memory)
4. [Retrieval architecture](#4-retrieval-architecture)
5. [Consolidation loop](#5-consolidation-loop)
6. [Skill system](#6-skill-system)
7. [Skill lifecycle](#7-skill-lifecycle)
8. [Explainability and feedback](#8-explainability-and-feedback)
9. [Permission model](#9-permission-model)
10. [Evaluation plan](#10-evaluation-plan)
11. [PR roadmap (M1 – M6)](#11-pr-roadmap-m1--m6)
12. [Composio relationship](#12-composio-relationship)
13. [Doctrine boundaries (M0 hard NOTs)](#13-doctrine-boundaries-m0-hard-nots)
14. [Cross-references](#14-cross-references)

---

## 1. Core thesis

### 1.1 What Brevio self-learning means

**Brevio self-learning means typed external memory + structured retrieval + bounded consolidation + approved reusable skills, all per-user, all reviewable, all reversible.**

Brevio does not "learn" by becoming a different model. Brevio learns by accumulating typed evidence about the user, indexing it for retrieval, consolidating it on a cadence, and turning repeated successful behaviors into versioned skill artifacts that ship through an approval gate.

### 1.2 What Brevio should learn

Brevio learns the user's **personalized model of importance, intent, preference, repeated workflow, and relationship context**, expressed as structured signals — not as conversation prose.

Specifically:

- which senders, topics, and patterns this user finds important ([Personalized Importance Learning — PIL](personalized-importance-learning.md));
- which surfaces (alerts, drafts, reminders, "why?", explanations) this user wants more of and less of ([Feedback + Learn/Grow Loop](brevio-product-philosophy.md#3-feedback--learngrow-loop));
- which contacts matter, in what role, and at what cadence;
- which projects, deadlines, and routines the user repeatedly cares about;
- which corrections the user has made and what was explicitly forbidden ("forget this", "do not do that again");
- which workflows the user has approved as reusable skills.

Brevio does not learn aesthetics or jokes. Brevio does not learn private inferences. Brevio does not learn anything the user has retracted.

### 1.3 Where learning lives

Learning lives in **external typed memory stores**, not inside the base model. The base model stays a stateless reasoner; everything that should persist between calls is written to disk through a typed write path with audit, evidence, confidence, and forget hooks.

Concretely, learning lives in three places:

1. **Typed memory tables** (per-user, per-type, evidence-linked) — the persistent store described in §2.
2. **An approved skill registry** (versioned, founder-approved, permission-classified) — described in §6.
3. **Per-user retrieval policies** (deterministic suppressions and preferences applied BEFORE any vector search) — described in §4.

### 1.4 What stays outside the base model

The base model never receives:

- Full transcript replay ("here is everything the user has ever said").
- Untyped chat-style memory dumps without source/confidence/recency annotations.
- Raw inbox bodies (existing 3D.1 privacy contract; PIL substrate; v0.5.9 feedback substrate; this doctrine).
- Cross-user signals or "users like you also flagged…" comparisons (PIL hard boundary).
- Speculative inferences that have not survived consolidation.
- Anything the user has explicitly retracted via the "forget this" surface (§8).

### 1.5 Why external memory + retrieval + skill artifacts are the first path

- **Reviewable.** Every fact has a row. Every row has a source, confidence, recency, and audit trail. The user can read, query, edit, and delete each row. A model fine-tune does none of this.
- **Reversible.** The user can correct or forget any fact. A weight update is permanent in the model; a typed row is `UPDATE active=false` or `DELETE WHERE id=…`.
- **Per-user safe.** Postgres rows scope to `user_id`. Cross-user contamination is structurally prevented (carrying forward the v0.5.4 cross-tenant proofs and the PIL HMAC-keyed scope_key discipline). A shared fine-tuned model cannot guarantee that.
- **Auditable.** Every write goes through an `audit_log` row with action, target, result, and structural detail. Compare to a fine-tune, which produces a binary blob.
- **Explainable.** Brevio can answer "what do you remember about me?" by querying memory rows. Brevio cannot answer that question from a model weight.
- **Bounded blast radius.** A bad fact affects one row and is one `DELETE` away from gone. A bad weight update poisons every future inference.

### 1.6 Why full-history prompt replay is not the first path

- **Blast radius.** A single accidental injection in any past message contaminates every future call.
- **Cost and latency.** Token budget grows linearly with history; "remember everything" is not a memory architecture, it is a budget killer.
- **No retraction.** "Forget this" is impossible if the model is re-reading the transcript every call.
- **No structure.** The model has to re-derive (sender X is important, you don't care about Y) from prose every time, which means brittle, irreproducible, and unauditable.
- **Privacy regression.** Every past raw body crosses the model boundary every call. The v0.5.x privacy contract forbids this.

### 1.7 Why online fine-tuning on user chats is not the first path

- **Irreversible.** A weight update on user data cannot be cleanly undone for that user without re-training from scratch.
- **Cross-user leakage by construction.** A shared fine-tuned model trained on one user's preferences shifts behavior for every other user.
- **No audit.** A weight update produces no per-fact audit row.
- **No explainability.** The user cannot ask "why do you think that about me?" and get an answer pointing to the cause.
- **Compliance.** GDPR deletion, CCPA correction, and HIPAA / FERPA isolation become unsolvable. With typed memory, deletion is `DELETE FROM …` and correction is `UPDATE … SET corrected_label=…`. With weights, neither has a defined operation.
- **The user did not consent.** Even if technically possible, training on a user's private chats without explicit, per-event consent violates the Brevio motto: **the AI may propose; the system gates; the user approves** ([`CLAUDE.md`](../CLAUDE.md)).

Online fine-tuning may eventually become a Brevio surface — but only when (a) it is per-user, (b) the user has explicitly consented per-event, (c) the deletion and correction operations are defined, (d) the audit trail captures every contributing event, and (e) a documented rollback path exists. None of this is in scope for M0–M6.

### 1.8 The motto, restated for memory

**Brevio remembers what matters, forgets what the user retracts, learns what improves service, and never silently expands what it knows.**

---

## 2. Memory architecture

Memory in Brevio is a set of **typed stores**, each with its own purpose, write rules, retrieval rules, and forget rules. A new memory write must always name its type. There is no untyped "memory dump" table.

Every typed memory store ships with these eleven attributes:

| Attribute | What it means |
|---|---|
| **Purpose** | The single product question this memory answers. |
| **Example** | A concrete, realistic Brevio row. |
| **Source of truth** | Which event produced this row (feedback event id, alert id, user reply id, calendar event id, etc.). |
| **Required evidence** | What must be true before this row may be written (e.g. ≥k confirming feedback events, explicit user statement, founder approval). |
| **Confidence** | How sure Brevio is. Stored as `low | medium | high` OR a numeric `0..1`. Every consumer of the memory reads confidence before acting. |
| **Staleness** | After what conditions/time this row decays or expires. |
| **Retrieval rules** | When this row enters a context pack (and when it MUST NOT). |
| **Write rules** | What gate must pass before the row is written. |
| **Delete/forget rules** | How the user retracts or Brevio forgets it. |
| **Security/privacy** | What this row must never contain; cross-user invariants; PII-class. |
| **MVP vs later** | What the first runtime version looks like; what is intentionally deferred. |

### 2.1 Session memory

- **Purpose.** Hold the context of the *current* user-Brevio exchange so multi-turn surfaces (Why?, feedback acknowledgement, soon: reply confirmations) read coherently without re-fetching everything.
- **Example.** "User replied 'why?' at 09:14:02 referencing alert `9c0e07…`, currently at state `replied`."
- **Source of truth.** The live route handler (e.g. `sendblue-inbound.ts`). Composed in memory; flushed at end of session/turn.
- **Required evidence.** The originating inbound_replies row + the matched alert row.
- **Confidence.** Always `high` (it is observed within this session).
- **Staleness.** Discard at end of session or after a bounded TTL (e.g. 30 minutes).
- **Retrieval rules.** Available only to the active session/route handler that produced it. Never written to long-term memory unless explicitly promoted via consolidation (§5).
- **Write rules.** No explicit write; lives in process memory.
- **Delete/forget rules.** Cleared automatically at session end.
- **Security/privacy.** Never persisted to disk in raw form. Never enters a context pack outside the session that owns it.
- **MVP.** Implicit in v0.7.0A `applyWhy` (the matched alert + most-recent rank.reason is session-bounded already).
- **Later.** Promote a per-session structured object so multi-turn surfaces (snooze→ack→follow-up) share a coherent in-process pack.

### 2.2 Episodic memory

- **Purpose.** Remember what *happened*, in time order: "Brevio alerted you about X on 2026-06-10; you replied 'why?'; Brevio explained Y; you marked 'this mattered'."
- **Example.** `{ kind: 'episodic', user_id, event_id: 'alert:…', occurred_at, surface: 'email_alert', outcome: 'this_mattered', confidence: high }`.
- **Source of truth.** `audit_log` rows + `feedback_events` rows + state-machine transitions.
- **Required evidence.** Already-durable audit row at minimum.
- **Confidence.** `high` (it actually happened).
- **Staleness.** Long-lived; episodic decay is by retrieval relevance, not by hard delete.
- **Retrieval rules.** Used to answer "why did you do that?" and "what was the last X you flagged?" Never directly fed to the ranker as a feature without going through semantic / preference memory first.
- **Write rules.** Strictly derived from existing audit rows. The audit row is the *source*; episodic memory is a *projection*.
- **Delete/forget rules.** Inherits from audit_log retention. User "forget this episode" causes BOTH the audit row and the projection to be marked retracted (not physically deleted, since audit is load-bearing for compliance — instead, marked `user_retracted=true` and excluded from retrieval).
- **Security/privacy.** Never carries raw sender/subject/body. Only the structural identifiers + outcome.
- **MVP.** Materialized view over `audit_log` + `feedback_events` joined by alert_id and inbound_reply_id. No new table.
- **Later.** Dedicated `episodic_events` projection with retrieval-optimized indices.

### 2.3 Semantic / user profile memory

- **Purpose.** Hold long-lived structured facts about the user: timezone, working hours, primary calendar, primary inbox, current employer, primary role.
- **Example.** `{ kind: 'semantic', user_id, attribute: 'working_hours', value: { tz: 'America/New_York', start: '09:00', end: '18:00' }, source: 'user_provided', confidence: high, retracted: false }`.
- **Source of truth.** Explicit user statements (onboarding, "from now on…" replies) OR derived from repeated observed behavior with founder review.
- **Required evidence.** For user-provided: a single explicit statement. For derived: ≥k observations over a window (founder-tuned).
- **Confidence.** `high` for user_provided; `medium` for derived-and-confirmed; `low` for derived-but-unconfirmed (low rows are NOT retrieved).
- **Staleness.** Decay when the user provides a contradicting fact OR after a long inactivity window (e.g. timezone unchanged for 12 months — request re-confirm).
- **Retrieval rules.** Enters the ranker / drafter / HMR context only when relevant (e.g. timezone enters Calendar-aware surfaces; working_hours enters proactive-reminder timing).
- **Write rules.** Write-through user approval surface (§8) OR consolidation job (§5) with founder-tuned threshold. NEVER inferred and acted on silently.
- **Delete/forget rules.** "Forget my role" → `UPDATE retracted=true`. Future retrievals exclude. User-facing surface confirms what was forgotten.
- **Security/privacy.** No PII beyond what is needed for the named attribute. No employer email, no home address, no medical, no financial. Each attribute schema is allowlisted.
- **MVP.** Single `user_profile_facts` table with `(user_id, attribute, value JSONB, source, confidence, retracted, created_at, updated_at)`. Allowlisted attribute names.
- **Later.** Per-attribute versioning + diff view in the "what do you remember?" surface.

### 2.4 Preference memory

- **Purpose.** Hold explicit user preferences about *how* Brevio behaves: alert tone, alert frequency, surfaces enabled, channels, do-not-disturb windows.
- **Example.** `{ kind: 'preference', user_id, attribute: 'alert_tone', value: 'calm_concise', source: 'user_stated', confidence: high }`.
- **Source of truth.** Explicit user statements ("be more concise"), explicit settings, OR founder defaults.
- **Required evidence.** Explicit statement OR a settings-surface write.
- **Confidence.** `high` for user-stated; never `low` (we don't retrieve low-confidence preferences — they would silently alter behavior).
- **Staleness.** Preferences are sticky. They decay only when the user contradicts them.
- **Retrieval rules.** Read by every surface that produces user-facing output. Read at the *top* of the context pack, before any vector retrieval.
- **Write rules.** User-driven only. Never auto-derived. Consolidation can *propose* a preference (§5), but writing requires an approval surface.
- **Delete/forget rules.** "I changed my mind on tone" → `UPDATE retracted=true` + the new preference is written.
- **Security/privacy.** Preferences are user-scoped strings/enums. No PII.
- **MVP.** `user_preferences` table with allowlisted attribute names.
- **Later.** Preference history + "you previously said X; you now say Y" diff in the explanation surface.

### 2.5 Negative / correction memory

- **Purpose.** Hold what Brevio has been *told not to do*, who not to flag, what surfaces to suppress, what assumptions to drop.
- **Example.** `{ kind: 'correction', user_id, scope: 'sender:<HMAC>', rule: 'ignore_sender', source: 'reply_parser_deterministic', evidence_event_id: 12345, confidence: high }`.
- **Source of truth.** Explicit user corrections via reply parser (v0.5.10 `ignore_sender`, `false_positive`), founder Slack rejection, ops_inject, or future "forget this" surface.
- **Required evidence.** A single confirmed feedback_event is sufficient for a *suppression*; reversing a suppression requires a fresh explicit statement.
- **Confidence.** Always `high` — corrections are first-class signals. We do not silently weaken them.
- **Staleness.** Corrections do not decay on a timer. They decay only on explicit retraction ("ok, you can alert me on that sender again").
- **Retrieval rules.** Applied **deterministically** BEFORE vector retrieval. A `sender_suppressed` row blocks the sender at the ranker boundary, regardless of any other learned signal (this is the PIL "no over-aggressive suppression" inverse: explicit user correction overrides everything).
- **Write rules.** Written through the existing v0.5.9/v0.5.10/v0.5.11 substrate (sender_feedback_ignored, sender_suppressed, ranker_label corrections). Future correction types added via this same gate.
- **Delete/forget rules.** "Actually I do want emails from X again" → reverse the suppression after explicit user confirmation. Audit captures both writes.
- **Security/privacy.** Sender / scope identifiers are HMAC-keyed per the PIL substrate. No plaintext sender_email in the row.
- **MVP.** Already shipped in `memory_signals` via PIL substrate (v0.5.11) + reply-parser feedback (v0.5.10).
- **Later.** Negative-pattern corrections beyond sender (subject-pattern, time-of-day, topic), each with their own evidence threshold.

### 2.6 Project / workflow memory

- **Purpose.** Hold *what* the user is currently working on, the projects with deadlines, the workflows they care about ("the Q3 board deck", "the EMEA hiring loop", "the apartment search").
- **Example.** `{ kind: 'project', user_id, project_id: 'q3-board-deck', title: 'Q3 board deck', stakeholders: ['mark@…' (HMAC)], deadline_iso: '2026-09-30', confidence: medium, status: 'active' }`.
- **Source of truth.** User explicitly tells Brevio OR multiple converging signals (calendar event title + email subject + repeated correspondence) crossed a consolidation threshold.
- **Required evidence.** User statement (`high`) OR ≥k convergent observations (`medium`).
- **Confidence.** Tracked per project; `medium` projects can enter retrieval but with a "Brevio is inferring this" hedge.
- **Staleness.** Projects expire when (a) explicit user retraction, (b) the deadline has passed by N days with no new evidence, (c) the user statement contradicts.
- **Retrieval rules.** Enters the ranker context for *related-email scoring* (a Stripe email matching a known "EMEA hiring" project's stakeholder set scores higher). Enters the HMR for "why?" explanations.
- **Write rules.** Consolidation job proposes; user approval surface writes for `high`. Founder approval may be required during early phases.
- **Delete/forget rules.** "Project's done" → retracted; future retrievals exclude. Audit captures.
- **Security/privacy.** Stakeholder identifiers are HMAC-keyed (no plaintext sender_email). Subject text is held only as a short title, never the body.
- **MVP.** `user_projects` table with `(user_id, project_id, title, deadline_iso, status, confidence, retracted)`. No stakeholder set — that joins via a separate `user_project_stakeholders` table.
- **Later.** Stakeholder-set, milestones, derived deadline reminders.

### 2.7 Relationship / contact memory

- **Purpose.** Remember who matters to the user and *how* (boss, direct report, spouse, advisor, vendor) — bound to the user's stated language for that role.
- **Example.** `{ kind: 'contact', user_id, contact_hmac: 'a93f…', role: 'manager', label: 'Mark (CEO)', source: 'user_provided', confidence: high }`.
- **Source of truth.** Explicit user statement OR repeated PIL signals (high importance + frequent reply latency → likely important relationship) crossed a consolidation threshold.
- **Required evidence.** User-stated (`high`) OR derived from PIL with founder-tuned threshold (`medium`).
- **Confidence.** `high` only when user-stated.
- **Staleness.** Roles drift (people change jobs). Decay when (a) user retracts, (b) Brevio observes contradicting evidence (e.g. sender domain shifts) and triggers a re-confirmation.
- **Retrieval rules.** Enters the ranker for *sender-importance scoring*. Enters the HMR for "why?" explanations ("I flagged Mark — your manager — because…"). Enters reminder surfaces ("you usually reply to your manager within 4 hours").
- **Write rules.** User-driven for `high`; consolidation-proposed for `medium`. Never inferred silently.
- **Delete/forget rules.** "Mark isn't my manager anymore" → role retracted; audit captures.
- **Security/privacy.** Contact identifier stored ONLY as HMAC of normalized email (mirroring PIL). User-facing label is the *user's stated label* ("Mark (CEO)") not the raw email. Cross-user isolation enforced via user_id scoping AND HMAC keyed per user (a future safeguard).
- **MVP.** `user_contacts` table with HMAC-only contact id + user-stated label.
- **Later.** Relationship graph (Mark → reports → Sarah), with consent surface for cross-relationship inferences.

### 2.8 Repeated behavior memory

- **Purpose.** Notice that the user repeatedly does X under condition Y — the *raw material* for the skill candidate pipeline (§7).
- **Example.** `{ kind: 'repeated_behavior', user_id, pattern_id: 'reply_with_meeting_link', trigger: 'inbound email asking to meet', action_observed: 'user replies with their calendar link', occurrences: 7, last_observed: '2026-06-12', confidence: medium }`.
- **Source of truth.** Episodic memory aggregation by pattern detector during consolidation.
- **Required evidence.** ≥k observations (founder-tuned, plausibly k=5) of the same pattern with the same user.
- **Confidence.** `low` until threshold, `medium` after, `high` only after founder review.
- **Staleness.** Patterns expire when not observed for a window OR when the user explicitly stops them.
- **Retrieval rules.** Read ONLY by the skill candidate pipeline (§7). NEVER acted on directly.
- **Write rules.** Consolidation-only. No direct writes.
- **Delete/forget rules.** "Stop tracking that pattern" → retracted; audit captures.
- **Security/privacy.** Patterns are typed — they describe *kinds* of action (reply with link, snooze, archive), not the message content. No body text in the row.
- **MVP.** `user_behavior_patterns` table with `(user_id, pattern_id, trigger_schema, action_schema, occurrences, last_observed, confidence, retracted)`.
- **Later.** Cross-pattern conjunctions ("when X AND Y, the user does Z").

### 2.9 Calendar-derived memory

- **Purpose.** Hold structured calendar context that supports timing-aware decisions (alerts, reminders, drafts).
- **Example.** `{ kind: 'calendar_derived', user_id, fact: 'has_recurring_team_standup', value: { day: 'monday', start: '09:00', tz: 'America/New_York' }, source: 'calendar_observation', confidence: medium, retracted: false }`.
- **Source of truth.** Calendar API (read-only; existing v0.6.0C substrate — DOES NOT activate Calendar live wiring, which is paused per the founder lock).
- **Required evidence.** Repeated observation OR explicit user statement.
- **Confidence.** `medium` for derived; `high` for user-stated.
- **Staleness.** Recurrence changes → decay. User retraction → immediate retract.
- **Retrieval rules.** Enters surfaces that need timing context only. NEVER carries event titles for *private* / `Busy`-masked events into a context pack (carry forward v0.6.0C adapter boundary).
- **Write rules.** Consolidation-only. Adapter boundary stays load-bearing: only `summary`, `start`, `end` ever reach this store.
- **Delete/forget rules.** "Drop my recurring standup memory" → retracted.
- **Security/privacy.** Same adapter boundary as v0.6.0C. Busy-masked events never produce facts.
- **MVP.** Deferred until M3+ consolidation. v0.6.0C substrate stays dormant per [[v0-6-0e-1c-pass-and-calendar-live-pause]].
- **Later.** Recurring-event-derived prep reminders, conflict suggestions, follow-up scheduling support.

### 2.10 Skill memory

- **Purpose.** Hold the registry of *approved reusable skills* this user has consented to. Distinct from §6 (the catalog of *all* skills); this is *which skills are armed for this user*.
- **Example.** `{ kind: 'skill_armed', user_id, skill_id: 'send_meeting_link_v0.1.0', armed_at, founder_approved: true, last_used, usage_count, retracted: false }`.
- **Source of truth.** The approval surface (§7) where the user (and during early phases the founder) approved this skill for this user.
- **Required evidence.** Explicit user approval. Founder approval until full autonomy ladder is unlocked.
- **Confidence.** Always `high` — armed-or-not is binary.
- **Staleness.** Skills go dormant if unused for a long window (re-confirm before next activation).
- **Retrieval rules.** Skill candidate pipeline reads this to know "what is already approved". Execution path reads this to know "may I run this skill for this user".
- **Write rules.** Approval surface ONLY. NEVER auto-armed.
- **Delete/forget rules.** "Disarm that skill" → retracted; future executions blocked; audit captures.
- **Security/privacy.** No skill body in this row — only the registered `skill_id`. The skill body lives in §6 registry.
- **MVP.** `user_skills` table with HMAC-keyed scope per skill_id. NOT in scope for M0; planned for M5/M6.
- **Later.** Per-skill usage stats + per-skill rollback triggers.

### 2.11 Stale / superseded memory

- **Purpose.** Track facts that were *previously* true but no longer are, without losing the historical record.
- **Example.** `{ kind: 'preference', user_id, attribute: 'alert_tone', value: 'punchy_short', retracted_at: '2026-06-01', superseded_by: <new_row_id>, source: 'user_stated' }`.
- **Source of truth.** Any time a memory row is retracted or replaced, the old row stays with `retracted=true` and `superseded_by` populated.
- **Required evidence.** Mirror of the source row.
- **Confidence.** Original confidence preserved.
- **Staleness.** "Stale" is the *kind*; these rows are never retrieved by the ranker. They are retrievable only by audit / explainability surfaces.
- **Retrieval rules.** Excluded from ranker / drafter / HMR retrieval. Visible in "what do you remember?" with a "previously" tag.
- **Write rules.** Implicit — the retracted row gets `retracted=true` and `superseded_by=<new_id>`. No new table.
- **Delete/forget rules.** "Forget this previously, too" → physical DELETE on the stale row (with audit). Default is retain for explainability.
- **Security/privacy.** Inherits from source row.
- **MVP.** Add `retracted boolean` + `superseded_by bigint nullable` to every memory table from M1 onward.
- **Later.** "I want to see a history of my memory changes" surface.

### 2.12 Type summary (one-line per type)

| Type | Source of truth | Default confidence | Retrieval default | Forget op |
|---|---|---|---|---|
| Session | live route handler | high | session-scoped | discard at end |
| Episodic | audit_log + feedback_events | high | "why did you?" | user_retracted=true |
| Semantic / profile | user statement + consolidation | high (user) / medium (derived) | relevant surfaces | retracted=true |
| Preference | user statement | high | every user-facing surface | retracted=true |
| Negative / correction | user reply + corrections | high | ALWAYS before vector | retracted=true |
| Project / workflow | user statement + consolidation | high / medium | ranker + HMR | retracted=true |
| Relationship / contact | user statement + consolidation | high / medium | ranker + HMR | retracted=true |
| Repeated behavior | consolidation | low → medium → high | skill candidate pipeline ONLY | retracted=true |
| Calendar-derived | calendar API | medium | timing-aware surfaces | retracted=true |
| Skill armed | approval surface | high | skill execution path | retracted=true (disarm) |
| Stale / superseded | mirror of source row | preserved | never to ranker | rare physical delete |

---

## 3. What should NOT become memory

Brevio must **refuse to write** the following into long-term memory:

- **Jokes.** "Brevio you're the best" / "kill it Brevio" / banter. Episodic at most; semantic never.
- **Typos and gibberish.** Reply parser unclear; v0.5.10 ≤3-word safe rule. No memory write.
- **Emotional one-offs.** "I hate Mondays today" is not "user hates Mondays".
- **Temporary preferences.** "Quiet me until tomorrow" is a session/state-machine event, not a permanent preference (Brevio already handles this via snooze state, NOT preference memory).
- **Sensitive inferences.** Brevio MUST NOT infer or record medical condition, religion, political view, sexual orientation, race, immigration status, financial distress, mental health state, or family conflict from emails or replies. Even if a user message touches these, Brevio does not abstract them into facts.
- **Private calendar details.** v0.6.0C adapter boundary: `visibility=private` calendar events are masked to `Busy` BEFORE crossing into Brevio. Calendar-derived memory NEVER carries titles from masked events.
- **Raw inbox bodies.** 3D.1 privacy contract. No memory row carries email body text. The closest allowed surface is `rank_results.reason` — model-generated short prose, not raw body.
- **Unsupported relationship assumptions.** Brevio does NOT decide "X is your spouse" from message tone. Relationship memory writes require *explicit user statement* or *consolidation crossing a founder-tuned evidence threshold*.
- **Anything the user corrected.** If the user said "no, that's wrong" or "forget that", the contradicted assertion stays in the audit (for accountability) but is `retracted=true` and excluded from retrieval. Brevio MUST NOT re-derive a retracted fact from the same observations later — see §5 contradiction handling.

These exclusions are **load-bearing**. The consolidation job (§5) and any future memory write path MUST encode them as pre-write rejections, not as post-hoc filters. The privacy canary tests (§10) verify that none of these slip in.

---

## 4. Retrieval architecture

### 4.1 Typed retrieval first

Every memory read is **typed first, vector second**. The retrieval order is:

1. **Resolve typed scopes** for the current surface. For a ranker call: `(user_id, sender_hmac, project_id?, contact_role?)`. For an HMR call: `(user_id, alert_id, project_id?)`.
2. **Apply deterministic suppressions and preferences.** Query the negative-correction store (`sender_suppressed`, `ranker_label corrections`) and preference store. These are *hard* — they bound retrieval and downstream behavior before any soft signal participates.
3. **Apply typed memory packs.** Project, contact, semantic, calendar-derived memory rows that match the typed scope. Each row is annotated `{source, confidence, recency}`.
4. **(Future)** vector retrieval over episodic memory, gated by the typed memory's answer first. Vector retrieval is a *last resort*, not a default.

The ranker, drafter, HMR, and explanation surfaces consume a **context pack** — the output of steps 1–4 — not raw retrieval results. The context pack carries every retrieved row's source, confidence, and recency so downstream code can decide *not* to use a low-confidence row.

### 4.2 Exact suppressions / preferences before vector retrieval

A `sender_suppressed` row for sender X blocks every subsequent retrieval and ranker scoring step for sender X. Even if vector retrieval would surface high-signal evidence of importance, the suppression wins. **The user's explicit correction is the highest-priority signal in the system.**

Same for preferences: the alert-tone preference is read at the *top* of the HMR context, before composition. A vector retrieval that proposes a different tone is overridden.

### 4.3 user_id scoped always

Every memory read MUST scope to `user_id`. There is **no** un-scoped query in the memory layer. The Postgres queries enforce this at the driver level (every typed query parameterizes `user_id`); the integration tests enforce it via cross-tenant LOAD-BEARING checks (carry forward v0.5.4 and v0.7.0A patterns).

### 4.4 Source / confidence / recency attached

Every row that enters a context pack carries:

- `source` — feedback event id, audit row id, user statement event id, calendar observation id.
- `confidence` — `low | medium | high` or numeric.
- `recency` — `last_observed_at` or `created_at`.

Downstream code reads these. The ranker (when it eventually consumes typed memory) may choose to ignore a `low` row, weight a `medium` row half, and trust a `high` row. The HMR composes the "I flagged this because…" body referencing only `high` rows. The "why did you remember this?" surface (§8) reads `source` to answer.

### 4.5 Evidence-linked context packs

A context pack is itself a structured object, not prose. Every consumer can:

- inspect which rows participated;
- inspect each row's source/confidence/recency;
- decide independently whether to trust the row;
- attach the row provenance to its audit detail.

This means the ranker's `rank.reason` can structurally claim "I flagged Mark because he's your manager (high confidence, user stated 2026-05-12)" — referencing rows by id, not by free-text recall.

### 4.6 No full transcript replay

Brevio never feeds prior conversation transcript prose into a new model call. The model sees:

- the current input (this email, this reply);
- the structured context pack derived from typed memory (per 4.1);
- the model's system instructions.

It does NOT see "here is every previous interaction the user had with Brevio". Brevio is *not* a chat-memory product; it is a typed-memory product.

### 4.7 No raw inbox body replay unless explicitly approved and egress-safe

Raw email bodies are excluded from every context pack by default. The only path by which body content reaches the model is the existing v0.5.7+ ranker egress view (`RankerEgressView` — short snippet, length-bounded, redacted), and that path stays unchanged. Future surfaces that need richer body content (e.g. a "summarize this email" surface) must enter their own gate with explicit user consent per surface AND extend the egress policy explicitly — never silently.

### 4.8 Retrieval audit

Every retrieval that produces a context pack writes an audit row with structural detail only:

- `pack_kind` (`ranker | hmr | explain | drafter | …`);
- `row_ids` participating;
- `row_kinds` (the types of memory consulted);
- `suppressions_applied` (count);
- `preferences_applied` (count).

The audit row NEVER carries the *content* of any retrieved memory — only its ids and kinds. The retrieval-audit kind is reserved (`brevio.memory.retrieved`) and ships in M1 (§11).

---

## 5. Consolidation loop

### 5.1 What it reads

Consolidation reads three sources, all per-user:

1. **Episodic memory** (audit_log + feedback_events projection): `what happened`.
2. **Existing typed memory** rows (semantic, preference, project, contact, behavior, calendar): `what we already know`.
3. **Reply parser output history** (v0.5.10+): `what the user explicitly said`.

### 5.2 What it writes

Consolidation writes:

- **Updates to existing rows** when new evidence reinforces them (incrementing `occurrences`, refreshing `last_observed`, raising `confidence` from `low → medium` after threshold).
- **Candidate proposals** for new rows — written to a `memory_candidates` table with `proposed_by='consolidation'` and `pending_approval=true`. The candidate row does NOT enter retrieval until the user/founder approves it.
- **Decay marks** when a row's evidence has been quiet beyond its staleness window (`stale_marked_at` populated; row excluded from retrieval; user-facing surface flags it for confirmation).

### 5.3 What it must NEVER write

- Anything from §3 (the never-memory list).
- Cross-user signals (a pattern observed in user A NEVER becomes evidence for user B's memory).
- Anything contradicting a `retracted=true` row in the same user's store — see §5.6.
- Anything that bypasses the typed write path (no untyped JSON blobs).
- A new `confidence='high'` row without explicit user approval (consolidation can propose `medium` at most; `high` requires the approval surface).
- Anything inferred from a private/`Busy`-masked calendar event.
- Anything inferred from an emotional one-off or a typo.

### 5.4 Cadence

Consolidation runs on a **bounded cadence**: daily by default, founder-tunable, with a hard upper bound (e.g. hourly). Per-user — never a global batch. Audit row `brevio.consolidation.cycle` fires once per user per run with summary counters (rows_updated, candidates_proposed, decay_marks_written).

Consolidation is **never** triggered inline by user actions. It is a separate worker; it is **safe to disable** by flipping `FOMO_CONSOLIDATION_ENABLED=false` without breaking any other surface. Default-off until M3 ships.

### 5.5 Duplicate merge

When two candidate rows would represent the same fact (e.g. two project candidates with the same title, two contact candidates with the same HMAC), consolidation merges them BEFORE proposing. The merge:

- preserves the higher confidence;
- preserves the earlier `first_observed`;
- preserves both source ids in a `sources` array;
- writes a `brevio.consolidation.merge` audit row.

Duplicate-merge is **idempotent**: running consolidation twice on the same input produces the same merged candidate.

### 5.6 Contradiction handling

When new evidence contradicts an existing `retracted=true` row:

1. Consolidation **does NOT** automatically re-write the old fact.
2. It writes a `memory_contradiction` row with the contradicting evidence + the retracted row's id.
3. It surfaces the contradiction to the user via the "what should I remember?" surface (§8).
4. The user/founder decides whether to un-retract.

When new evidence contradicts an existing non-retracted row:

1. Consolidation writes a candidate update with `confidence_after_contradiction` lowered by one level.
2. If the contradicting evidence accumulates beyond the threshold, the candidate is proposed for user approval.
3. The original row stays in retrieval until the user approves the replacement.

This means **Brevio defers to the user on contradictions**. Brevio does not silently flip-flop.

### 5.7 Stale memory handling

Each typed memory kind defines its own staleness window (§2). Consolidation walks rows past their window and:

1. Marks them `stale_marked_at = now`.
2. Excludes them from retrieval immediately.
3. Surfaces them to the user via the "what should I remember?" surface (§8) for confirm-or-retract.

User confirmation refreshes `last_observed` and clears the mark. User retraction sets `retracted=true`. No silent deletion.

### 5.8 Over-learning prevention

Three guardrails:

- **Per-kind write rate limit.** Consolidation may not write more than N candidates per user per cycle per kind (e.g. ≤5 new project candidates/day). Stops a runaway pattern detector.
- **Cross-kind dampening.** A new project candidate that shares ≥k stakeholders with an existing approved project gets MERGED at proposal time, not written as separate.
- **Founder-tuned thresholds, surfaced in audit.** Every consolidation cycle's audit row carries the exact thresholds in force, so changes are reviewable.

### 5.9 Evidence bundle

Every candidate proposal carries an **evidence bundle**: structural identifiers (event ids, audit ids, feedback event ids) that justify the proposal. The user/founder approval surface displays the bundle so the approver can see WHY consolidation is asking. The bundle is structural — it does not carry raw body text.

### 5.10 Disable / rollback

Consolidation is gated by `FOMO_CONSOLIDATION_ENABLED` (default false until M3). Disabling stops new consolidation writes immediately. Rows already in `memory_candidates` stay in the queue; no auto-execution. Per-user override (`FOMO_CONSOLIDATION_USER_ALLOWLIST`) for canary activation (mirroring the v0.5.13 PIL canary discipline).

A **rollback** of bad consolidation output is a typed operation: select rows by `proposed_by='consolidation'` AND `created_at>=<bad_start>`, set `retracted=true` for each, audit the bulk retract. Rollback is documented per consolidation phase.

### 5.11 Audit trail

Consolidation writes audit rows for every action it takes:

- `brevio.consolidation.cycle` (per user per cycle, structural counters);
- `brevio.consolidation.candidate_proposed` (per candidate);
- `brevio.consolidation.merge` (per merge);
- `brevio.consolidation.stale_marked` (per stale mark);
- `brevio.consolidation.contradiction` (per contradiction);
- `brevio.consolidation.cycle_aborted` (when a guardrail tripped).

All structural detail only. No raw row content.

---

## 6. Skill system

A Brevio **skill** is a **versioned, founder-approved, permission-classified, reusable artifact** that turns a structured input into a structured action proposal, with the constraint that *running the skill* always respects the permission model (§9).

A skill is **NOT** a prompt. It is **NOT** an ad-hoc model call. It is **NOT** discovered at runtime and silently armed. It is a *thing that lives in a registry, has a version, has tests, has owners, and is rolled out or rolled back like code*.

### 6.1 Skill schema (every approved skill ships with all of these)

| Field | Meaning |
|---|---|
| `skill_name` | Stable kebab-case identifier (e.g. `send-meeting-link`, `acknowledge-stop`, `propose-reschedule`). |
| `version` | Semver. Bumps follow the same rules as `EXPLAIN_TEMPLATE_VERSION` — patch for cosmetic, minor for behavior, major for breaking. |
| `purpose` | One-line product statement. "When user X asks to meet, propose a calendar link." |
| `trigger_conditions` | Structural conditions that must be true to consider this skill. E.g. `{inbound_intent: 'ask_to_meet', has_calendar_link_pref: true}`. **NOT** a model prompt. |
| `required_inputs` | Named structured inputs (typed). |
| `optional_inputs` | Same, optional. |
| `output_format` | Typed output: action proposal + summary. Never freeform prose without a renderer. |
| `memory_dependencies` | List of memory kinds (§2) the skill reads from. Each consumed row is annotated with source/confidence. |
| `tool_permissions` | List of Brevio-wrapped tools the skill is allowed to call (e.g. `calendar.read`, `composio:gmail.draft`). Each entry has a permission tier (§9). |
| `safety_constraints` | Hard rules ("never auto-send", "never reference private calendar events"). Checked at execution boundary, not "trust the prompt". |
| `failure_modes` | Enumerated typed failure outcomes (`missing_memory`, `tool_unavailable`, `user_revoked_consent`, `safety_violation`). Each has a defined response. |
| `tests` | Unit tests + integration tests + a smoke evidence template for activation. Must pass before approval. |
| `example_successful_runs` | At least 3 concrete examples with inputs and outputs, included in the registry. |
| `evidence_for_existence` | A pointer to the repeated_behavior memory rows (§2.8) and consolidation candidate that motivated this skill. NO skill ships without evidence the user actually does this. |
| `approval_status` | `draft | candidate | founder_approved | user_approved | armed | deprecated | retired`. |
| `version_history` | Append-only list of prior versions with the diff and rationale. |
| `rollback_plan` | The exact operation to disarm. For armed skills: set `armed=false` for all users; audit captures. |
| `owner` | Founder during early phases. Eventually a domain owner. Every change to the registry row writes an audit row with the changing actor. |

### 6.2 Why this shape

The shape is chosen to make every skill **independently reviewable** (you can read one row and understand what it does), **independently rollbackable** (disarm one skill without touching others), and **independently testable** (a skill comes with its own tests and example runs).

The shape **rejects** the v9-era pattern of "50 skills in a single catalog with prompts inlined" ([`docs/future-architecture-notes.md` §Tool Router / §Intent Classification](future-architecture-notes.md)). The catalog hierarchy (`category → group → skill → adapter`) survives; the inline-prompt practice does not.

### 6.3 Skill registry

The registry is a **Postgres table**, not a JSON file:

- `brevio_skills(skill_name, version, purpose, schema_json, approval_status, owner, created_at, …)`

with one row per `(skill_name, version)` pair. Approval transitions write audit rows. The table is read-only at runtime by the execution path; mutations happen through ops scripts with founder approval.

The schema JSON encodes the §6.1 fields. Validation is enforced by the ops script (no skill row may insert without all required fields).

### 6.4 What a skill is allowed to *do*

- Read typed memory (per `memory_dependencies`).
- Call permitted Brevio-wrapped tools (per `tool_permissions`).
- Compose an action *proposal*, never a direct action — the proposal flows to the permission model (§9).
- Emit structural audit detail.

### 6.5 What a skill is NOT allowed to do

- Mutate memory directly. (Memory writes go through the consolidation loop OR explicit user approval surfaces.)
- Call non-wrapped tools or raw HTTP. (Every tool call must go through Brevio's wrapper, which enforces permission + audit.)
- Carry raw email body / calendar private details / phone / sender plaintext.
- Bypass `tool_permissions` or `safety_constraints` "just this once".
- Write to a memory kind not in its `memory_dependencies`.

### 6.6 First-class principles

- **Versioned.** Every behavior change is a version bump. No silent in-place edits.
- **Founder-approved before user-armable.** The founder reviews every new skill version BEFORE it can be armed for any user.
- **User-armed per skill.** Even after founder approval, each user must individually arm a skill before it runs for them.
- **Disarmable in one operation.** Setting `armed=false` for a `(user_id, skill_id)` row stops further execution.
- **Per-user audit.** Every skill execution writes an audit row with `(user_id, skill_id, skill_version, action_proposed, action_taken, outcome)`.

---

## 7. Skill lifecycle

A skill enters Brevio through an explicit, gated pipeline. There is no path by which a skill auto-appears and auto-runs.

### 7.1 Repeated behavior

The user does something the same way more than once. Episodic memory accumulates rows. Consolidation's pattern detector sees a behavior pattern repeating, increments `occurrences` on a `repeated_behavior` row (§2.8), and crosses a founder-tuned threshold (e.g. 5 observations of the same pattern in 30 days).

### 7.2 Candidate detection

When a repeated_behavior row crosses the threshold, the **skill candidate detector** (a separate consolidation pass) examines the row, evaluates whether the pattern is *skillable* (some patterns are inherently un-automatable: emotional support, judgment-only decisions), and produces a candidate.

### 7.3 Candidate draft

The candidate is drafted into the **`brevio_skill_candidates`** table. The draft includes:

- the proposed `skill_name`;
- the proposed `purpose`;
- the proposed `trigger_conditions` (structural, from the pattern);
- the proposed `required_inputs / output_format` (structural, from observation);
- the proposed `memory_dependencies` (from what the pattern read);
- the proposed `tool_permissions` (from what the user did manually);
- a placeholder `approval_status='draft'`.

The draft does NOT include implementation; that comes after founder approval.

### 7.4 Evidence bundle

Every candidate carries an evidence bundle: the `repeated_behavior` row, the contributing episodic events, the relevant `feedback_events`, and any related typed memory (project, contact). The bundle is structural; no raw content.

### 7.5 Safety / permission review

The candidate is reviewed against three checks:

1. Do the proposed `tool_permissions` cross a permission tier the user has not granted (§9)? If yes, the candidate is rejected OR a permission upgrade is proposed to the user.
2. Do the `safety_constraints` cover the failure modes? If a known failure mode is not covered, the candidate is rejected.
3. Does the candidate violate any §3 "should NOT become memory" rule by depending on a forbidden inference? Reject.

### 7.6 Test generation

For each accepted candidate, the founder (during M4/M5) authors:

- Unit tests for input → output.
- Integration tests via a stub execution harness.
- A smoke evidence template.

Tests must pass BEFORE founder approval can flip `approval_status` past `candidate`.

### 7.7 Founder / user approval

Founder approval is recorded by an ops script that writes the registry row and audit. User approval is the user-facing surface ("Brevio noticed you do X repeatedly. Want Brevio to do that for you when conditions match?") that creates the `user_skills` armed row.

Both approvals must exist before execution.

### 7.8 Limited activation

After both approvals, the skill is **armed for that user only**, **at the lowest-risk tier first** (e.g. `draft_only` even if the user authorized `auto_send`). Limited activation runs for a founder-tuned observation window and writes a denser audit trail.

### 7.9 Usage logs

Every execution writes:

- `brevio.skill.proposed` when the trigger fires.
- `brevio.skill.gated` if a permission gate blocked it.
- `brevio.skill.executed` on completion.
- `brevio.skill.failed` on failure with typed failure mode.
- `brevio.skill.feedback` when the user reacts to a skill output (positive / negative correction).

### 7.10 Improvement / deprecation / retirement

- **Improvement.** Found a better trigger or output. Version bump. Re-approval if memory dependencies or tool permissions changed.
- **Deprecation.** Better skill exists. Old version goes `deprecated`. Existing armed users keep running it until they re-arm or retract; new arms blocked.
- **Retirement.** Stop entirely. `retired` status. Disarmed for all users. Audit row preserved.

### 7.11 Cross-skill discipline

- No skill may compose another skill without each composition being its own approved skill.
- No skill may have global side effects outside the user it ran for.
- A skill's failure mode MUST NOT include "try a different skill"; that's a planner-level decision the user must consent to.

---

## 8. Explainability and feedback

Brevio's memory and skill system is only trustworthy if the user can interrogate it. Every memory kind and every skill execution gets a surface.

### 8.1 "What do you remember?"

User asks Brevio (via reply, settings page, or future Founder Command Surface) what is stored. Brevio returns a structured answer per memory kind, with confidence + recency for each row, and a forget control per row.

**MVP:** ops script that emits a JSON dump scoped to user_id. **Later:** a user-facing settings surface.

### 8.2 "Why did you remember this?"

For any row, Brevio shows the `source` chain: the original feedback_event id, audit_log id, or user statement event id. The user reads "you said this on 2026-05-12 in reply ID 84," not "the model decided."

### 8.3 "Where did this come from?"

For any context-pack entry that influenced a Brevio output, the user can ask Brevio to enumerate which memory rows contributed. v0.7.0A's "Why?" reply surface is the first instance of this — currently bounded to `rank.reason`. Future versions enumerate row ids.

### 8.4 "Forget this."

User retracts a memory row. Brevio:

1. Writes the retraction (sets `retracted=true`, audit captures).
2. Replies with the deterministic confirmation (HMR-rendered): "Got it — I won't remember [the row's user-facing label] anymore."
3. Excludes from future retrieval.
4. Marks any derived rows as `confidence_after_retraction='low'`; consolidation re-evaluates next cycle.

### 8.5 "That is wrong."

Distinct from "forget this." "Wrong" means *replace* — the user supplies a corrected value. Brevio writes the corrected row, retracts the wrong one, and audits both. Subject to the same approval gates as a normal write.

### 8.6 "Do not do that again."

Negative correction targeted at a *behavior*, not a fact. Brevio:

1. Identifies the behavior pattern (the skill or surface that produced the output).
2. Writes a `do_not_repeat` correction row scoped to `(user_id, behavior_id)`.
3. The behavior is **suppressed** at the gate before next execution.
4. If the behavior was driven by an armed skill, the user is asked whether to disarm the skill entirely.

### 8.7 "Make this reusable."

User explicitly tells Brevio: "next time something like this happens, do that." This is the user-driven skill candidate path. Brevio:

1. Creates a `brevio_skill_candidates` row with `proposed_by='user'`.
2. Walks the founder/user approval pipeline (§7.5–§7.7).
3. Does NOT execute until approved.

### 8.8 "What skill did you use?"

For any Brevio output, the user can ask which skill produced it. Brevio enumerates the skill_name + skill_version + the input scope. This requires every skill execution to leave a structurally-linked audit row, which §7.9 guarantees.

### 8.9 HMR ownership

All eight surfaces are **HMR output**. The text the user sees is composed deterministically from typed memory + skill metadata. The body is never raw model output (3E.1 invariant), and never field-shaped metadata ([[brevio-human-message-renderer-principle]], [[dont-oversell-renderer-tone]]). The bar is "feels like a calm assistant" — not "feels like a SQL console".

### 8.10 Audit invariants for explainability surfaces

Every surface above writes a typed audit row (`brevio.explainability.<surface>`) with structural detail: surface, user_id, row_ids inspected, action taken (none / retract / replace / suppress / promote_to_skill_candidate). The surface NEVER writes the *content* of the retrieved rows to audit detail.

---

## 9. Permission model

Brevio's autonomy is **tiered**. Every action enters the system as a proposal and exits through a tier gate. No skill, memory action, or tool call escapes this gate.

### 9.1 Tier 0 — Read-only memory retrieval

- **Allowed.** Reading typed memory to assemble a context pack for surfaces (ranker, HMR, "why?", future explanations).
- **Forbidden.** Mutating memory. Calling any tool. Producing any user-visible output without going through HMR.
- **Approval required.** None; default-on at the per-user level once Brevio is connected.
- **Logging.** Each retrieval writes `brevio.memory.retrieved` (structural only).
- **Rollback.** N/A — retrieval has no side effect.

### 9.2 Tier 1 — Draft-only recommendations

- **Allowed.** Producing a *proposal* the user reads but Brevio does not send. Email drafts saved to the drafts folder (read-only to outside parties). Calendar event proposals with a `proposed=true` flag. iMessage *suggested replies* shown to the user but not sent.
- **Forbidden.** Anything that touches an outside party. Auto-send. Auto-confirm.
- **Approval required.** Per-skill, per-user, founder-approved.
- **Logging.** `brevio.action.drafted` + `brevio.action.draft_seen` + `brevio.action.draft_discarded` if the user dismisses.
- **Rollback.** Discard the draft.

### 9.3 Tier 2 — User-approved actions

- **Allowed.** Brevio proposes; the user explicitly approves via reply or surface; Brevio executes. (E.g. "send Mark a reply with my calendar link" → user replies "yes" → Brevio sends.)
- **Forbidden.** Acting without the explicit per-event approval. Caching approvals across events ("you said yes once, so always yes"). Approvals that auto-renew.
- **Approval required.** Per-event approval, captured in audit, time-bounded (within a reasonable window — e.g. 30 minutes of the proposal).
- **Logging.** `brevio.action.proposed`, `brevio.action.approved`, `brevio.action.executed`, `brevio.action.expired` if the approval window passed.
- **Rollback.** Per-action — the skill's `rollback_plan` (§6.1) defines the exact reversal.

### 9.4 Tier 3 — Founder-approved reusable skills

- **Allowed.** A skill that runs without per-event user approval *for the user that armed it* — but only AFTER founder approval (§7.7), user arming, and a limited-activation window proves safe behavior.
- **Forbidden.** Skills that exceed their `tool_permissions`. Skills that bypass `safety_constraints`. Skills that act on data not in their `memory_dependencies`.
- **Approval required.** Founder approval to publish the skill version. User approval to arm for self.
- **Logging.** Full execution audit per §7.9.
- **Rollback.** Disarm the skill per user; if pattern-wide, deprecate the skill version.

### 9.5 Tier 4 — Fully autonomous actions (later only)

- **Allowed.** A skill runs without per-event approval AND across all users with appropriate consent. This is **out of scope for M0–M6**. The doctrine names it so the rest of the tier system has a top to anchor against.
- **Forbidden.** Everything not explicitly authorized per-user per-skill. Money. Booking. Sending on behalf to a new recipient. Anything irreversible.
- **Approval required.** Founder + user + legal review + per-action override capability.
- **Logging.** Audit + a separate `brevio.autonomy.action` registry with per-action evidence.
- **Rollback.** Per-action reversal AND per-skill disarm AND per-user opt-out.

### 9.6 Tier transitions

A skill never auto-promotes between tiers. Promotion is an explicit founder action with an audit row. Demotion (e.g. "this skill went off the rails — drop it from Tier 3 to Tier 2") is also explicit and immediate; the executor reads the tier at execution time, not at boot time.

---

## 10. Evaluation plan

Every memory + skill phase ships with an evaluation harness. The harness is *evidence*, not theatre — the tests exist to catch regressions a reasonable engineer would otherwise miss.

### 10.1 Correct memory retrieval

For a curated fixture set of (user state, surface, expected pack), the harness asserts the retrieval pass produces the expected pack. Mirrors the v0.5.11 `pil-eval.ts` shape.

### 10.2 Incorrect memory abstention

When a user has insufficient confidence on a fact, the retrieval pass MUST NOT include it. Fixture: user with one observation of a pattern at `confidence=low` → that row is absent from the pack. **LOAD-BEARING:** false-positive retention destroys trust faster than under-retrieval.

### 10.3 Stale memory detection

Fixture: a `semantic` memory row whose `last_confirmed_at` exceeds its staleness window. The harness asserts consolidation sets `stale_marked_at` and the row is excluded from the next retrieval.

### 10.4 Contradiction handling

Fixture: two contradicting feedback events on the same scope. The harness asserts consolidation writes a `memory_contradiction` row and does NOT auto-overwrite. The surface (§8) is fed the contradiction.

### 10.5 Privacy leakage

Same shape as v0.5.9 / v0.5.11 / v0.5.15 privacy canary: for a fixture user with known SECRET substrings (sender, subject, body, raw reason, phone), the harness scans every audit row, every memory row, every retrieved pack, and asserts zero occurrences. **LOAD-BEARING.**

### 10.6 Cross-user isolation

For two fixture users A and B, the harness asserts that:

- A retrieval pass for A includes ZERO B rows.
- A consolidation cycle on A's data writes ZERO B rows.
- A skill execution for A reads ZERO B memory.

Mirrors the v0.5.4 cross-tenant proof and the v0.7.0A "Why?" cross-tenant LOAD-BEARING test.

### 10.7 User deletion enforcement

Fixture: user deletes account OR retracts a specific memory row. The harness asserts:

- The retracted row is excluded from retrieval immediately.
- Downstream consolidations do not re-derive the retracted fact.
- "What do you remember?" no longer surfaces it.
- Audit log retains the retraction event (compliance trail), but the retracted *content* is absent from retrieval-time queries.

### 10.8 Skill candidate quality

Fixture: a stream of episodic events known to either contain a real pattern OR contain only noise. The harness asserts:

- Real patterns produce candidates after threshold.
- Noise produces zero candidates.
- A candidate's `evidence_bundle` matches the events the founder would expect.

### 10.9 Skill misuse

Adversarial fixture: a skill is invoked with inputs designed to push it outside its `tool_permissions` or `safety_constraints`. The harness asserts:

- The execution path rejects the misuse BEFORE the tool call.
- A `brevio.skill.gated` audit row fires.
- No tool side effect occurred.

### 10.10 Regression from bad memory

Fixture: a known-bad memory row (e.g. a contradicted preference). Harness asserts:

- The bad row does not corrupt the ranker's score for unrelated emails.
- Downstream surfaces degrade gracefully when the bad row is detected.
- Rollback by `retracted=true` restores baseline behavior.

### 10.11 Tool permission boundaries

Fixture: a skill declares `tool_permissions: [calendar.read]`. The harness asserts:

- `calendar.read` is callable.
- `calendar.write` invocation by the same skill is rejected at the gate.
- An audit row captures the gate decision.

### 10.12 Harness conventions

- Tests live in `apps/fomo/src/memory/` and `apps/fomo/src/skills/` directories (created in M1+).
- Each phase adds tests to its phase suite — never to a "global" memory test file.
- LOAD-BEARING tests are tagged (in the test name or a marker) so the operator can grep them.
- Cross-tenant / privacy-canary patterns are reused from v0.5.4 / v0.5.9 / v0.5.11 / v0.5.15 / v0.7.0A — do not invent new ones.

---

## 11. PR roadmap (M1 – M6)

Each milestone is a separate phase with its own 6-question gate, Core Dimension Check, and TIER classification per [[risk-tiered-verification]]. The roadmap below is *direction*, not approval — none of M1+ is approved by this M0 document.

### M1 — Typed Memory Substrate

- **Goal.** Ship the typed memory tables (semantic, preference, correction-extension, project, contact, repeated-behavior, stale flagging) with audit kinds + retrieval audit + no consumer changes.
- **Scope.** Migrations (additive). Store classes (`UserProfileFactStore`, `UserPreferenceStore`, `UserProjectStore`, `UserContactStore`, `UserBehaviorPatternStore`). Stale-flag column on each + `superseded_by`. Audit kinds (`brevio.memory.retrieved`, `brevio.memory.retraction_recorded`). NO retrieval into ranker / HMR yet.
- **Non-goals.** Any consumer surface. Vector retrieval. Consolidation. Skill anything.
- **Data model implications.** 5 new tables; each with user_id, kind-specific schema, source/confidence/recency, retracted, superseded_by, audit timestamps. Unique index on `(user_id, kind, scope_key)` where applicable.
- **No-migration fallback.** Not applicable — typed memory requires typed tables. (Untyped JSON columns rejected by founder doctrine.)
- **Tests.** Per-table CRUD, retracted-exclusion, cross-tenant isolation, audit-emission, privacy canary on store inputs.
- **Acceptance.** Stores write/read correctly. No surface reads from them. Default state is "tables exist, empty". CI green.
- **Rollback.** Stores are dormant. Drop tables in a follow-up migration if needed (reversible because no consumer depends).
- **Risks.** Schema mistakes hard to fix post-write — keep allowlisted attribute names + JSON schema validation.
- **Do not touch.** Ranker. HMR. Reply parser. PIL. Calendar substrate. Outbound sender.
- **Tier.** TIER 1 (migration + cross-user privacy).

### M2 — Memory Explain / Control Surface

- **Goal.** "What do you remember?" + "Forget this." + "That is wrong." surfaces shipped through the existing reply-parser + HMR.
- **Scope.** New reply parser intents (`memory_show`, `memory_forget`, `memory_correct`). New HMR templates for each response. Read-only retrieval against the M1 stores. `retracted=true` writes via the existing audit chain.
- **Non-goals.** Memory writes from consolidation. Skill anything. New surfaces beyond SendBlue inbound + HMR.
- **Data model implications.** None — uses M1 tables.
- **No-migration fallback.** Yes — pure code change against M1 tables.
- **Tests.** Reply parser intent recognition + deterministic. HMR voice canary. Cross-tenant isolation on the retrieval. Privacy canary on the response body. "What do you remember?" returns a sensible empty when user has nothing.
- **Acceptance.** User can query, retract, correct via natural reply. Audit captures.
- **Rollback.** Kill switch `FOMO_MEMORY_CONTROL_SURFACE_ENABLED=false` falls through to v0.5.10 behavior.
- **Risks.** HMR voice on memory content — must read as a calm assistant, not as a database query result.
- **Do not touch.** Consolidation. Skill registry. Ranker.
- **Tier.** TIER 2 (internal surface + targeted privacy/cross-tenant tests).

### M3 — Consolidation Job

- **Goal.** Per-user consolidation worker that reads episodic + existing memory and writes *proposals* (`memory_candidates`) + decay marks. No auto-promotion to active memory.
- **Scope.** New worker `consolidation-worker.ts`. New table `memory_candidates`. Audit kinds (`brevio.consolidation.*`). Default-off kill switch + per-user allowlist (mirroring v0.5.13 canary discipline).
- **Non-goals.** Auto-write to active memory. Skill candidates. Calendar consolidation (deferred to a later milestone since Calendar live wiring is paused).
- **Data model implications.** One new table for candidates. Adds `stale_marked_at` + `last_confirmed_at` to M1 tables.
- **No-migration fallback.** Partial — candidate table is new; stale columns can be added with backfill defaults to keep existing rows valid.
- **Tests.** Threshold detection. Duplicate merge. Contradiction handling (writes contradiction row, no auto-overwrite). Stale marking. Cross-user containment (a B-user pattern NEVER contributes to A-user candidates). Privacy canary on candidate evidence_bundle.
- **Acceptance.** Worker runs, produces candidates, never writes to active memory, audit captures every cycle.
- **Rollback.** `FOMO_CONSOLIDATION_ENABLED=false` disables. Candidates persist; no auto-execution.
- **Risks.** Over-learning — guardrails in §5.8 are load-bearing. Founder watches the candidate queue during early activation.
- **Do not touch.** Ranker. HMR. Reply parser intent set.
- **Tier.** TIER 1 (cross-user privacy + bounded autonomy expansion).

### M4 — Skill Candidate Pipeline

- **Goal.** Detect skill candidates from repeated_behavior memory + propose drafts in `brevio_skill_candidates`. No execution.
- **Scope.** Pattern-to-candidate detector (a separate consolidation pass). New table `brevio_skill_candidates`. Evidence bundle schema. Audit kinds (`brevio.skill.candidate_proposed`, `brevio.skill.candidate_rejected`).
- **Non-goals.** Skill registry. Skill execution. User approval surface (deferred to M5).
- **Data model implications.** One new table.
- **No-migration fallback.** No — skill candidates need their own table for typing.
- **Tests.** Candidate quality (§10.8). Cross-user isolation. Privacy canary on evidence_bundle. Threshold tuning surfaced in audit.
- **Acceptance.** Worker proposes candidates from real episodic + behavior data. Founder reviews queue. No skill execution.
- **Rollback.** `FOMO_SKILL_CANDIDATE_DETECTOR_ENABLED=false`.
- **Risks.** Spurious candidates from noise. Founder review cadence load.
- **Do not touch.** Ranker. HMR. Memory write path.
- **Tier.** TIER 1 (new autonomy substrate).

### M5 — Approved Skill Registry

- **Goal.** Ship the `brevio_skills` registry table + ops scripts for founder approval. Approved skills can be armed by users but cannot execute yet.
- **Scope.** Registry table per §6.3. Ops script for founder approval (`ops:skill-approve --name X --version 0.1.0`). `user_skills` armed table per §2.10. Approval surface (HMR-rendered) for users. Audit kinds (`brevio.skill.approved`, `brevio.skill.armed`, `brevio.skill.disarmed`).
- **Non-goals.** Skill execution. Auto-armament. Tier 3 skills.
- **Data model implications.** Two new tables.
- **No-migration fallback.** Partial — `user_skills` could overlay onto existing `memory_signals` with a new kind, but the schema clarity benefit of a dedicated table outweighs.
- **Tests.** Registry schema validation. Approval state machine (draft → candidate → founder_approved → user_armed → … → retired). Cross-user containment (B can't arm a skill on behalf of A). Audit emission.
- **Acceptance.** Founder approves 1 example skill (e.g. `acknowledge-stop-v0.1.0`); user arms it; armed flag persists; NO execution.
- **Rollback.** Disarm all users; deprecate skill version.
- **Risks.** Approval ergonomics — needs to be ops-friendly without becoming auto-approval.
- **Do not touch.** Skill execution. Ranker.
- **Tier.** TIER 1 (autonomy substrate + permission gate).

### M6 — Limited Skill Execution

- **Goal.** Run armed skills in Tier 2 mode (user-approved per event) with full audit + permission gate. Tier 3 deferred.
- **Scope.** Skill executor (`skill-runner.ts`). Tool-call wrapper that enforces `tool_permissions` per skill. Per-event approval capture via reply parser. Audit chain per §7.9. Default-off kill switch.
- **Non-goals.** Tier 3 (autonomous) execution. Cross-skill composition. Multi-step plans.
- **Data model implications.** None — uses M1/M5 tables + audit + memory_signals.
- **No-migration fallback.** Yes.
- **Tests.** End-to-end execution of ONE example skill (e.g. propose-reschedule). Permission gate rejection on out-of-policy invocations (§10.9, §10.11). Cross-user containment. Privacy canary. Rollback works (skill output is reversible per its `rollback_plan`).
- **Acceptance.** One armed user receives one skill output via SendBlue + approves + Brevio executes via a Brevio-wrapped tool. Audit captures everything.
- **Rollback.** Disarm skill; kill switch off.
- **Risks.** The first end-to-end user-perceptible Brevio autonomy beyond v0.7.0A's response surface. Voice + tone must hold (HMR).
- **Do not touch.** Ranker. PIL. Calendar live wiring.
- **Tier.** TIER 1 (first non-response action).

### What every milestone preserves

- v0.5.x HMR principle ([[brevio-human-message-renderer-principle]]).
- 3E.1 deterministic-body invariant.
- v0.5.9 / v0.5.10 / v0.5.11 PIL substrate + reply-parser routing.
- v0.5.14 feedback-ack surface.
- v0.6.0C Calendar adapter boundary (Calendar live wiring stays paused).
- v0.7.0A "Why?" surface.

### What every milestone defers

- Multi-step planner across skills (the v9-era planner concept stays archived; see [`docs/future-architecture-notes.md` §Agent Orchestration](future-architecture-notes.md)).
- Browser / computer-use tools.
- Tier 4 autonomy.
- Cross-user analytics or evaluation.

---

## 12. Composio relationship

### 12.1 What Composio is for Brevio

[Composio](https://composio.dev) is **Brevio's integration layer**. It provides connected accounts, OAuth flows, and a registry of tool actions (Gmail, Calendar, Slack, Notion, Stripe, etc.) that Brevio can call without re-implementing each provider's auth, refresh, and pagination semantics.

This document **locks** Composio's role at the integration boundary. Composio is not Brevio's brain. Composio does not own Brevio's memory, importance learning, voice, approval gates, or trust model.

### 12.2 What Composio is allowed to provide

- Connected-account management (OAuth start / callback / refresh / revoke).
- Tool action invocation (e.g. `gmail.draft`, `calendar.event.create`).
- Per-account scope metadata.
- Per-tool error envelopes.
- A consistent retry / rate-limit story.

When Brevio calls a Composio tool, the call flows through a **Brevio wrapper** (`composio-tool-wrapper.ts`, not yet built) that:

- enforces `tool_permissions` from the calling skill (§6.5);
- sanitizes provider errors through Brevio's existing `sanitizeProviderError` chokepoint (v0.5.15);
- writes the Brevio-native audit row (e.g. `brevio.skill.executed`);
- maps Composio's response shape into Brevio's typed action proposal / outcome.

### 12.3 What Composio is NOT allowed to be

- **Not Brevio's memory.** Composio may cache OAuth tokens; it does not store Brevio's typed memory rows. Memory lives in Brevio's Postgres.
- **Not Brevio's skill registry.** Composio's tool catalog is a *tool* catalog. Brevio's skills (§6) are higher-level reusable artifacts that COMPOSE Composio tools under Brevio's gates. A Composio tool action is one possible *primitive* a Brevio skill calls; it is not a Brevio skill itself.
- **Not Brevio's permission model.** Composio's tool ACLs at the integration layer are insufficient. Brevio re-enforces permissions at the skill / tier boundary (§9). Even if Composio would allow a tool call, Brevio's wrapper may refuse it.
- **Not Brevio's user trust surface.** Brevio's HMR composes every user-facing message. Composio responses (including errors) NEVER reach the user as raw provider text; they pass through HMR and the sanitizer.
- **Not Brevio's audit log.** Composio may emit its own logs internally; Brevio's `audit_log` table is the source of truth for what Brevio did on a user's behalf.
- **Not Brevio's deletion / correction.** When a user retracts memory or asks Brevio to forget something, that operation happens against Brevio's tables. Composio's connected accounts are revoked separately when the user disconnects the provider entirely — a *different* user action.

### 12.4 Hard contract: no raw Composio tool access without Brevio's wrapper

No code path in Brevio may import a Composio SDK client and call a tool action directly. Every Composio tool call goes through `composio-tool-wrapper.ts`, which:

- requires a skill identity (`skill_name + skill_version`) at call time;
- requires the tier classification at call time;
- writes the Brevio-native audit row;
- sanitizes provider errors;
- returns Brevio's typed `ToolCallOutcome`.

This is symmetric with Brevio's existing SendBlue + Gmail patterns: providers are wrapped, never directly accessed.

### 12.5 Why this matters

Composio is excellent integration plumbing. Brevio is a personal agent. If Composio owned Brevio's memory, the product would be Composio's product. If Composio owned Brevio's permission model, the product would inherit Composio's autonomy ceiling. If Composio owned Brevio's audit and trust, the user would have nowhere to ask "why did you do that?".

**Brevio owns the brain. Composio is the body's nervous system.** Composio reaches the world; Brevio decides what to reach for.

### 12.6 Composio activation status

Composio is **not yet wired** into Brevio's runtime. The first Composio integration must enter through its own 6-question gate, must ship the wrapper described in §12.4, and must include cross-user isolation tests. The v0.6.0C Calendar substrate already implements the adapter-boundary discipline Composio will follow; reuse that pattern.

---

## 13. Doctrine boundaries (M0 hard NOTs)

This document is **architecture only**. The hard boundaries below apply to M0 specifically:

- **No runtime code.** M0 ships zero runtime changes.
- **No DB migration.** M0 ships zero migrations.
- **No memory write path.** No new write code, no new write tests.
- **No skill execution.** No skill runner. No skill candidate pipeline. No skill registry table.
- **No Tool Gateway.** No Composio wiring. No new tool calls.
- **No browser automation.** Out of scope.
- **No Calendar live activation.** Calendar live wiring stays paused per [[v0-6-0e-1c-pass-and-calendar-live-pause]]. The substrate is dormant; this doctrine does not flip it.
- **No action tools.** No drafting. No sending. No proposing irreversible actions.
- **No broad framework implementation.** The §11 milestones are direction, not promises.
- **No claims about Hermes / Nous internals.** This document does not assert what proprietary external memory systems do internally — only what Brevio will do. References to other systems are kept generic.
- **Keep it concrete and Brevio-specific, not academic.** Every section above references actual Brevio tables (`memory_signals`, `audit_log`, `feedback_events`, `rank_results`, `user_profile_facts`), actual Brevio audit kinds (`brevio.consolidation.cycle`, `brevio.skill.executed`), actual Brevio phases (v0.5.1, v0.5.7, v0.5.9, v0.5.10, v0.5.11, v0.5.13, v0.5.14, v0.5.15, v0.6.0C, v0.7.0A). Not a generic agent-OS treatise.

---

## 14. Cross-references

### 14.1 Brevio doctrine docs (READ FIRST)

- [`CLAUDE.md`](../CLAUDE.md) — Brevio orientation, permanent rules, 6Q gate discipline.
- [`docs/brevio-product-philosophy.md`](brevio-product-philosophy.md) — the three permanent product layers (HMR, PIL, Feedback) every memory + skill surface must preserve.
- [`docs/brevio-core-agent-dimensions.md`](brevio-core-agent-dimensions.md) — the 12 core dimensions; §3 Memory Architecture, §4 Agent Core + Reasoning, §5 Tool / Workflow Orchestration, §6 Security / Permission Gates, §8 Feedback + Learn/Grow Loop, §9 Human Message Renderer, §10 Observability / Evals / Reliability, §12 User Trust / Consent.
- [`docs/personalized-importance-learning.md`](personalized-importance-learning.md) — PIL's permanent invariants; THIS doctrine extends them, never relaxes them.
- [`docs/future-architecture-notes.md`](future-architecture-notes.md) — archived v9-era planner / skill catalog / tool router / Personal Context Engine concepts; the surviving concepts (catalog hierarchy, consent state machine, capability discovery) are absorbed into this doctrine in their current Brevio-shaped form.

### 14.2 Implementation docs (CONTEXT)

- [`FOMO_DESIGN.md`](../FOMO_DESIGN.md) — the FOMO product / architecture constitution; trust ladder L1–L8; **`L8 (Memory)` lands through this doctrine's M1 milestone** (see §11). All §1–§13 of this doctrine are consistent with FOMO_DESIGN.md's L1–L8 and architecture rules.
- [`FOMO_PLAN.md`](../FOMO_PLAN.md) — the active implementation map; M1–M6 will appear here once locked.
- [`apps/fomo/KERNEL.md`](../apps/fomo/KERNEL.md) — current kernel live-state; updated when M1+ ships.
- [`SALVAGE_MAP.md`](../SALVAGE_MAP.md) — archived modules; the surviving v9-era memory + skill ideas are noted in `docs/future-architecture-notes.md` and absorbed here.

### 14.3 Auto-memory (operational context)

These auto-memory entries are operational context, not doctrine:

- `[[risk-tiered-verification]]` — TIER classification used throughout §11.
- `[[brevio-human-message-renderer-principle]]` — preserved by every memory + skill surface.
- `[[dont-oversell-renderer-tone]]` — preserved by §8 surfaces.
- `[[end-of-fomo-hardening-pivot]]` — frames why M0 is the canonical next phase rather than further internal FOMO hardening.
- `[[v0-6-0e-1c-pass-and-calendar-live-pause]]` — Calendar pause holds throughout this roadmap.
- `[[real-or-absent-no-half-wired]]` — every M1+ phase must ship complete or stay absent.
- `[[scope-isolation-and-hardening-discipline]]` — every milestone is its own scope; cross-milestone bundling rejected.
- `[[email-context-provider-abstraction]]` — long-term provider abstraction; consistent with §12 Composio wrapping discipline.

---

**End of M0 doctrine.**

The next implementation step is **M1 — Typed Memory Substrate**, which lands through its own 6-question gate, its own scope lock, and its own founder approval. None of M1+ is implicitly approved by this doctrine; it is the *map*, not the *commit*.
