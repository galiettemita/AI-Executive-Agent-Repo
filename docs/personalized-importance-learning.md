# Personalized Importance Learning

Status: Permanent product-direction doc. Documentation only — no runtime code is implied or implemented by this file.
Created: 2026-06-04 (founder directive)
Companion to: [FOMO_DESIGN.md](../FOMO_DESIGN.md), [FOMO_PLAN.md](../FOMO_PLAN.md), [CLAUDE.md](../CLAUDE.md), [docs/future-architecture-notes.md](./future-architecture-notes.md)

---

## 0. Why this doc exists

During v0.5.x testing — including the v0.5.2 Morris smoke and the v0.5.4 Sheila smoke — Brevio's ranker classified urgent-sounding spam and commercial emails as important. False positives like that are dangerous: they make Brevio feel noisy and untrustworthy, and they erode the only thing the product is trying to earn — the user's permission to stop checking their own inbox.

The reflexive fix — "just teach the ranker to ignore commercial mail" — is wrong. It is wrong because some commercial and transactional emails are exactly the ones a user would be sad to miss:

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

This doc establishes the *permanent product principle* Brevio must follow in response, the *architecture* it implies, the *guardrails* that prevent it from going sideways, and the *future implementation phases* required to make it real.

It is a lifeline document. Before any change to ranker behavior, commercial / spam handling, feedback events, memory signals, or user-correction parsing, read this file.

---

## 1. The permanent product principle

> **Brevio must learn each user's personal definition of important.**

The long-term standard:

> **Brevio should become less noisy over time without becoming blind.**

And:

> **Personalized learning must improve interruption quality without silently suppressing genuinely important messages.**

These three sentences are the permanent product direction. They are not a v0.5.5 sprint goal, not a ranker tweak, and not a one-quarter project. They are the contract Brevio makes with every user over the lifetime of the product.

Every design decision below — what feedback events to record, what memory signals to store, what the ranker is allowed to see, how user corrections are interpreted, what the evals look like, what gets shipped and what stays deferred — exists to make those three sentences true.

---

## 2. What this is, and what this is not

### 2.1 What Personalized Importance Learning IS

* A per-user model of which senders, topics, content patterns, and contexts that *specific* user finds important.
* A growing set of structured signals (feedback events + memory signals) built up from the user's actual replies and behavior, not from inferred behavior alone.
* A ranker context input — the ranker reads the user's accumulated signals and uses them to score incoming email *for that user*.
* A trust mechanism — the user can ask Brevio "why did you send me this?" and the answer should refer to learned signals, not magic.
* Reversible — every learned preference must be undoable by the user.

### 2.2 What Personalized Importance Learning is NOT

* **Not a global rule.** One user marking a sender unimportant must never affect another user. Cross-user leakage is forbidden.
* **Not a prompt-only patch.** Personalization that lives only inside the ranker prompt — without persistent feedback events, memory signals, and evals — is fake learning. It will drift, regress, and be impossible to debug.
* **Not category suppression.** "All commercial mail is unimportant" is not a learnable preference. The system must learn at the level of sender, topic, content pattern, and context — never at the level of a coarse category.
* **Not autonomous correction.** Brevio may *propose* changes ("I noticed you keep snoozing newsletters until evening — want me to hold them until 7 PM?"), but it may not silently apply structural changes to a user's preferences.
* **Not over-fitting on one signal.** A single "not important" reply on a single message must not silently teach Brevio to suppress every future message from that sender, unless the user explicitly says so ("ignore this sender", "never alert me about X again"). Single corrections lower the signal; they do not flip it.
* **Not cross-tenant.** The v0.5.x cross-tenant isolation invariants (Morris's data untouched by Sheila's smoke, etc.) apply unchanged. Personalized learning is per-user keyed.

---

## 3. Why this is a product-defining capability, not a ranker bug

There is a real temptation to treat the false-positive problem as a prompt-engineering chore: rewrite the ranker system prompt, add three more `<rules>` lines, ship.

That approach has three permanent failure modes:

1. **It can't remember.** A prompt-only change has no per-user memory. Every user gets the same blanket rule. The next user who *does* want bank-alert emails will be just as frustrated as the current user who doesn't want LinkedIn promo emails.

2. **It can't be reversed by the user.** A prompt change cannot be undone by an individual user texting Brevio "actually, please alert me about LinkedIn from now on." The user has no surface to push back on a rule that lives in a system prompt.

3. **It can't be audited.** When Brevio surfaces or suppresses an alert, the user (and the founder, during review) deserve a reason. "The ranker decided" is not a reason. "Three weeks ago you replied 'not important' to two emails from this sender, so Brevio is suppressing similar emails — want to reverse that?" is a reason.

Personalized Importance Learning is the *only* mechanism that can earn the product's first promise — *I can ignore my inbox without being scared that I missed something important* — while staying reversible, auditable, and per-user safe.

It is therefore a permanent product capability, on the same load-bearing tier as Tool Registry, Permission Gate, Audit Log, and the API-first / browser-fallback / approval-required execution rule.

---

## 4. The texture of the problem

To make the design choices concrete, here are seven illustrative scenarios. None of these is a real user email; each is a synthetic representative case the eval suite must cover (see §10).

| # | Email pattern | Brevio's mistake | The right learned response |
|---|---|---|---|
| 1 | "URGENT: your account will be suspended in 24 hours — click here" from a marketing list | Currently ranks as important because of urgency words | Learn at the sender + content-pattern level that this *specific* sender's "urgent" language is promotional noise for this user |
| 2 | "Your Chase fraud alert — unrecognized transaction on your card ending 4242" from chase.com | Currently might rank either way | Learn that this user *always* wants bank fraud alerts; never suppress |
| 3 | "Final reminder: free trial expires tonight" from a SaaS the user actively uses | Currently might rank as important (urgency) or unimportant (commercial) | Learn from the user's actual reply pattern — if they consistently snooze or ignore, lower the score; if they consistently open, raise it |
| 4 | "Your flight UA1234 to SFO has been delayed by 2 hours" from united.com | Currently might rank either way | Learn that flight-status emails matter to this user (if they travel) and don't matter to a user who doesn't |
| 5 | "Newsletter: this week in tech" with subject line "Why your startup is doomed" | Currently might rank as important (urgency) | Learn at the sender level that this newsletter is opt-in noise — but only after the user has actually marked it so, not on a hunch |
| 6 | "Re: invoice #4421 — overdue, please remit" from the user's accountant | Currently might rank either way | Learn the sender is important for this user's accounting topic; never suppress |
| 7 | "Your package was returned to sender" from amazon.com | Currently might rank as unimportant (commercial) | Learn the user cares about shipping issues if they consistently engage; don't suppress without evidence |

The common thread: **the right response in every row depends on what this user has done before.** Without personalization, every row collapses to a coin flip.

---

## 5. User-correction language → safe internal intents

Users do not text in JSON. They text like humans:

* "not important"
* "this mattered"
* "ignore this sender"
* "show me more like this"
* "commercial but useful"
* "marketing — don't text me about these"
* "always alert me for this sender"
* "only alert me if this sender mentions payment / security / deadline"

The reply parser (already a v0.1 component — see [FOMO_DESIGN.md §11](../FOMO_DESIGN.md)) must learn to map this kind of message to a *canonical, deterministic safe intent*, the same way it already does for `later`, `tomorrow`, `ignore`, `ignore_from_sender`, `why`, `stop`.

The set of canonical importance-learning intents proposed for the future implementation phase:

| Canonical intent | User utterances it covers | Effect when fired (future phase) |
|---|---|---|
| `mark_unimportant` | "not important", "nah didn't care", "this didn't matter" | Records a feedback event with low importance for THIS message; *does not* silently extend to the sender |
| `mark_important` | "this mattered", "good catch", "yes please surface these" | Records a feedback event with high importance for THIS message; lifts the sender's score within bounded confidence |
| `ignore_sender` | "ignore this sender", "marketing — don't text me about these", "never alert me about X again" | Adds a per-user suppression for the sender; reversible |
| `surface_more_like_this` | "show me more like this", "always alert me for this sender" | Adds a per-user surface preference for the sender; bounded confidence |
| `commercial_but_useful` | "commercial but useful", "this is a newsletter but I want it", "marketing but keep these" | Records that THIS sender + topic combination is opt-in even though it pattern-matches commercial; never extrapolated to other senders |
| `conditional_surface` | "only alert me if this sender mentions payment / security / deadline" | Records a per-user conditional rule keyed to sender + topic / keyword cluster; bounded recall (must surface a clarification before applying) |
| `why` | "why did you send me this?" | Returns the human-readable reason: which signals fired, which sender / topic match, which prior corrections informed the score |
| `clarify` | low-confidence parse | Asks one short clarification ("just this one, or all emails from <sender>?") rather than guessing |

Reply-parser principles (carried forward from [FOMO_DESIGN.md §11](../FOMO_DESIGN.md)):

* The LLM may *classify* the reply.
* The LLM may not *execute* the action.
* Execution happens through deterministic code, gated by per-user policy and audited.
* On low confidence, **ask one clarification rather than guess.** Single guesses are how learning systems silently overfit.

---

## 6. Feedback events

A *feedback event* is a structured row that records what happened — what alert, what reply, what the user did, what timing, what confidence — without storing the raw email body. It is the substrate every learned signal is derived from.

The v0.1 system already has a `feedback_events` table (see [FOMO_DESIGN.md §13](../FOMO_DESIGN.md), [FOMO_PLAN.md §9.6](../FOMO_PLAN.md)). Personalized Importance Learning extends the *kinds* of feedback events, not the table itself.

### 6.1 Existing feedback signals (v0.1)

* founder approved (Slack approval)
* founder rejected (Slack reject)
* user opened (inferred from app interaction in future; not in v0.1 because no app)
* user snoozed (`later`, `tomorrow`)
* user ignored (`ignore` on a single message)
* user ignored sender (`ignore_from_sender`)
* user asked why (`why`)
* user sent STOP
* no response (timeout signal)

### 6.2 Proposed importance-learning feedback signals

| Event kind | When fired | What it records |
|---|---|---|
| `importance.mark_unimportant` | User replies "not important" / synonyms | (user_id, message_id, sender_hash, topic_tags, score_at_time_of_alert, timestamp, source='user_reply') |
| `importance.mark_important` | User replies "this mattered" / synonyms | same shape, opposite polarity |
| `importance.ignore_sender` | User replies `ignore_sender` | (user_id, sender_hash, scope='sender', reversible=true, timestamp) |
| `importance.surface_sender` | User replies `surface_more_like_this` | same shape, opposite polarity |
| `importance.commercial_kept` | User replies `commercial_but_useful` | (user_id, message_id, sender_hash, topic_tags, signal='kept_despite_commercial', timestamp) |
| `importance.conditional_rule_proposed` | User replies `conditional_surface` | (user_id, sender_hash, topic_keywords, requires_clarification=true) |
| `importance.proposal_accepted` / `.proposal_rejected` | Brevio proposed a rule, user accepted or declined | (user_id, rule_text, decision, timestamp) |

Every feedback event must carry:

* `user_id` (per-user, never global)
* `source` (always one of `user_reply`, `user_approval`, `user_rejection`, `system_inferred`, `founder_set`)
* `confidence` (if inferred; explicit user actions are confidence 1.0)
* `recorded_at` (timestamp)
* `audit trace pointer` (alert_id, message_id, slack_ts where applicable)

Feedback events are append-only. They are the source of truth; memory signals (§7) are the *aggregated derivation* over them.

### 6.3 What must NOT be in a feedback event

* Raw email body.
* Raw email headers beyond `From` (hashed) and `Subject` (truncated where needed).
* Sender's real phone or email in plain text — use the same hashing pattern v0.5.1 established (`phone_e164_hash`, sender hash). See [feedback_multitenant-design-principles](../README.md) memory.
* Anything that would allow cross-user inference of one user's correction by another user reading raw logs.

---

## 7. Memory signals

A *memory signal* is the aggregated, query-shaped state derived from feedback events. The ranker reads memory signals; the ranker does not read raw feedback events.

The v0.1 system already has a `memory_signals` table with `kind`, `scope_key`, `detail`, `source`, `confidence`, `updated_at` (see [FOMO_DESIGN.md §14](../FOMO_DESIGN.md), [FOMO_PLAN.md §9.7](../FOMO_PLAN.md)). Personalized Importance Learning extends the *kinds* it carries.

### 7.1 Existing memory signal kinds (v0.1 / v0.5.x)

* `stop_active` — user has texted STOP; outbound gated until START
* `sendblue_contact_status` — v0.5.3 hardening contact registration state

### 7.2 Proposed importance-learning memory signal kinds

| Kind | Scope key | Detail shape (proposed) | Source(s) it derives from |
|---|---|---|---|
| `sender_importance` | hashed sender | `{score: -1.0…+1.0, n_events: int, last_updated: ts}` | `importance.mark_important`, `importance.mark_unimportant`, `importance.ignore_sender`, `importance.surface_sender` |
| `sender_suppressed` | hashed sender | `{suppressed: bool, set_at: ts, source: 'user_reply'/'system_proposed'}` | `importance.ignore_sender`, accepted proposals |
| `topic_importance` | topic_tag (e.g. "billing", "school", "shipping") | `{score, n_events, last_updated}` | aggregated across messages tagged with that topic |
| `commercial_kept` | hashed sender | `{kept_topics: [tag,...], set_at, source}` | `importance.commercial_kept` |
| `conditional_rule` | hashed sender | `{requires_keywords: [str,...], set_at, source}` | `importance.conditional_rule_proposed` after user clarification |
| `quiet_hours_pref` | (none / global per user) | `{start_hour, end_hour, tz, inferred_or_explicit}` | snooze-pattern inference (low-confidence) or explicit user rule |
| `daily_cap_pref` | (none / global per user) | `{cap: int, set_at, source}` | explicit only |

Every memory signal continues to carry the v0.1 invariants: `user_id`, `source`, `confidence`, `updated_at`, audit pointer.

### 7.3 The aggregation rules

* **One correction does not flip a signal.** A single `importance.mark_unimportant` should *lower* a sender's score, not zero it out. Only an explicit `ignore_sender` reply (or N≥k consistent `mark_unimportant` events on the same sender within a recency window) should trigger a hard suppression.
* **Recency decays older signals.** A "this mattered" reply from 8 months ago is weaker evidence than the same reply yesterday. Recency weights must be explicit, tested, and documented in the implementation phase.
* **Confidence travels with the signal.** Signals derived from explicit user replies are confidence 1.0. Signals inferred from behavior (snoozes, opens, ignores) are <1.0 and must be marked as such.
* **The ranker sees the aggregated signal, never the raw events.** This keeps the ranker prompt bounded and prevents context-window inflation as a user's history grows.

---

## 8. How learned signals feed the ranker

The ranker is the model call that decides "would this user be sad to miss this email?" Today it sees (per [FOMO_DESIGN.md §17](../FOMO_DESIGN.md)):

* sender,
* subject,
* limited safe snippet (if allowed),
* sender importance (currently coarse),
* suppressions (currently coarse),
* relevant user preferences,
* recent feedback patterns,
* daily cap state.

Personalized Importance Learning enriches the *sender importance*, *suppressions*, and *relevant user preferences* inputs with the structured signals from §7. It does NOT change the ranker's contract:

* The ranker still returns a single score + a structured `why`.
* The ranker still never sees raw private bodies in production.
* The ranker is still per-call stateless; persistence lives in memory_signals.
* The ranker prompt budget is bounded; the aggregated signals (not raw events) are passed in.

What the ranker should be able to read, when this capability lands:

* `sender_importance.score` for the incoming sender (if a row exists)
* `sender_suppressed` boolean for the incoming sender (hard suppression)
* `commercial_kept.kept_topics` for the incoming sender (if any)
* `conditional_rule.requires_keywords` for the incoming sender (if any)
* `topic_importance.score` for the dominant topic tag of the email (if known)
* `quiet_hours_pref` and `daily_cap_pref` (for downstream gating, not ranking)

What the ranker should still NOT see:

* Raw feedback event history (only the aggregated signal).
* Other users' signals (per-user keyspace strictly).
* Anything that would let it write back to memory (the ranker reads; the deterministic post-processor writes).
* Cross-user "all users hate this sender" aggregates — there is no such signal in the system; if one is ever proposed, it requires its own 6-question gate.

---

## 9. Guardrails

These are load-bearing. Implementation cannot deviate without a new 6-question gate.

### 9.1 One correction must not overfit

A single user reply of "not important" must lower a score, not flip a suppression. Only explicit `ignore_sender` or N≥k consistent corrections (k and recency window to be set in the implementation phase) trigger a suppression.

### 9.2 Per-user first

Every learned signal is keyed by `user_id`. There is no global learning, no cross-user aggregation, no "users like you also suppressed…" pattern. The v0.5.x cross-tenant isolation invariants (Morris untouched by Sheila, etc.) apply unchanged.

### 9.3 No cross-user leakage

A test in the eval suite must explicitly verify: "user A marks sender X unimportant; user B receives an email from sender X; user B's ranker score is unchanged." This is the same shape as the v0.5.4 cross-tenant diff test (§6 of the SMOKE_REPORT) and must be in every implementation-phase smoke.

### 9.4 No raw email bodies in memory

Feedback events and memory signals must never store raw email bodies. They store sender hashes, subject fragments (truncated), topic tags, and structured signals. Raw bodies stay in transit through the ranker call and are not persisted. (Carry-forward from [FOMO_DESIGN.md §22](../FOMO_DESIGN.md): "no raw private email leakage.")

### 9.5 No real inbox examples committed

Eval fixtures must be synthetic. No real email from any user — founder included — may be committed to the repo as a test case. Synthetic fixtures must cover the same patterns (§10) without containing real PII.

### 9.6 Synthetic eval fixtures only

This is a corollary of 9.5. The eval suite (§10) is built from hand-authored or LLM-generated synthetic emails. They must be diverse enough to cover the texture of the problem (§4) but contain zero real-world data.

### 9.7 Every signal carries confidence, recency, and source

`{score, n_events, last_updated}` is not optional. Every signal Brevio writes is timestamped and source-attributed. Without this, debugging "why was this email scored 0.91?" is impossible.

### 9.8 User reversibility

The user must always be able to undo a learned preference: "actually, surface emails from <sender> again", "stop suppressing my newsletters", etc. The reply parser must support a `reverse_preference` intent (or similar) in the implementation phase. Reversibility is a hard requirement — a learning system the user can't undo is not safe.

### 9.9 Brevio may propose, Brevio may not silently apply

Structural changes — "shall I hold all newsletters until evening?" — are user-prompted, never silent. This is the safe-learning-tier discipline already in [FOMO_DESIGN.md §13](../FOMO_DESIGN.md). The new memory kinds in §7 inherit that discipline.

### 9.10 No global suppression rule from one user's preference

A single user's `ignore_sender` for `noreply@somecompany.com` must not bleed into a global "Brevio always suppresses noreply@somecompany.com" rule. The signal lives in that user's `memory_signals` row. Other users' rankers do not read it.

### 9.11 No half-wired learning

Per the standing [Real or Absent — No Half-Wired](../README.md) rule: if a feedback event kind is registered but no consumer aggregates it into a memory signal, it must not ship. If a memory signal kind is registered but the ranker doesn't read it, it must not ship. The learning loop is end-to-end or it does not exist.

---

## 10. Evaluation fixtures

Evals are how Brevio knows it is becoming less noisy without becoming blind. The implementation phase must ship the eval suite *with* the runtime — not afterward.

Mandatory eval categories, synthetic fixtures only:

| Category | What it tests | Pass criterion (proposed) |
|---|---|---|
| Urgent spam | "URGENT: account suspension" promotional spam | Ranker scores LOW for users who have no signal supporting it; does not over-trigger on urgency words |
| Promotional fake urgency | "Final hours! 50% off!" with high urgency vocabulary | LOW score baseline; LOW for users who have marked similar senders unimportant; respects `commercial_but_useful` if user has set it |
| Newsletter with urgent headline | "Why your startup is doomed" weekly newsletter | LOW score for users who have consistently ignored that sender; HIGH if user has explicitly `surface_more_like_this` |
| Commercial but important | Subscription renewal warning, account limit warning | NOT auto-suppressed; respects user signals if any; defaults to mid-confidence surface in absence of signal |
| Security alert | "Suspicious sign-in attempt from new device" | HIGH score regardless of category (commercial-bucket suppression must not apply); never auto-suppressed |
| Bank fraud alert | "Possible fraud on your card ending 4242" | HIGH score regardless of category; never auto-suppressed |
| Flight change | "Your flight has been delayed/cancelled" | HIGH score for users with prior travel-pattern signal; mid-confidence in absence of signal; never auto-suppressed |
| Invoice | "Invoice #1234 — payment due" from the user's accountant or service provider | HIGH score; never auto-suppressed without explicit `ignore_sender` |
| Sender user marked important | Any email from a sender the user has `mark_important`'d ≥k times | HIGH score, even if subject pattern would otherwise rank lower |
| Sender user marked unimportant | Any email from a sender the user has `mark_unimportant`'d ≥k times within recency window | LOW score, even if subject is urgent |
| **Cross-user contamination** | User A marks sender X unimportant; user B's ranker scores an email from sender X | User B's score must be unchanged (the load-bearing privacy / isolation test) |

The cross-user contamination test is the most important eval in the suite. It is the same shape as the v0.5.4 cross-tenant diff (§6 of SMOKE_REPORT_v0.5.4.md) — a regression there means a privacy failure, not just an accuracy failure.

---

## 11. Scope boundaries (what this doc does NOT do)

Per the founder directive that authored this doc (2026-06-04):

* No runtime code is introduced.
* No ranker rewrite is begun.
* No auto-send capability is unlocked.
* No friend-beta expansion is implied.
* No browser automation is implied.
* No new email providers are activated. (Email-provider abstraction still per [FOMO_DESIGN.md §6.2](../FOMO_DESIGN.md): Gmail is the only active provider through v0.5; Outlook / iCloud / Yahoo / IMAP are documented future work.)
* No raw private email examples are committed.
* No global suppression rule is created from one user's preference.

This doc is the *design and product contract* for a future implementation phase. It is not the implementation. The implementation requires its own 6-question gate (§13) before any code lands.

---

## 12. Future implementation: phase candidate

The implementation of Personalized Importance Learning is a candidate future phase. Exact phase number is left unlocked because the current phase map (post-v0.5.4 PASS) has multiple competing v0.5.5+ candidates surfaced by Friend B feedback (Google verification, iMessage tone + summary length, STOP confirmation reply — see [SMOKE_REPORT_v0.5.4.md §12](./SMOKE_REPORT_v0.5.4.md)).

Recommended phase name when it is scheduled:

> **`Personalized Importance Learning / False-Positive Reduction`**

Proposed phase shape (high-level, not commit-ready):

1. **Phase X.1 — Substrate.** Register the new feedback event kinds (§6.2) and memory signal kinds (§7.2) in `FOMO_AUDIT_ACTIONS` and `MEMORY_SIGNAL_KINDS`. Write the aggregation code that turns feedback events into memory signal updates. Migrations + columns + drizzle schema. Unit tests + gated PG tests (per `feedback_inmemory-mock-divergence` memory — the InMemory store must not silently diverge from Postgres on these new kinds).

2. **Phase X.2 — Reply parser extension.** Extend the reply parser to recognize the importance-learning intents (§5). Write the deterministic post-processor that turns each intent into the right feedback event write. Carry-forward: the LLM classifies; the LLM does not execute.

3. **Phase X.3 — Ranker context.** Plumb the aggregated signals (§7.2) into the ranker prompt context. Bake-off: does this improve precision / recall on the eval suite (§10) without regressing the existing v0.1 eval categories?

4. **Phase X.4 — Eval suite.** Build the synthetic eval fixtures for every category in §10, including the cross-user contamination test. Run the eval before *and* after every change to the ranker or memory aggregation in subsequent phases.

5. **Phase X.5 — Smoke gate.** Founder-only smoke test that exercises the full loop: send a synthetic email, get an alert, reply "not important", confirm a feedback event was written, confirm the memory signal was updated, confirm the next similar email scores lower. Per the v0.5.x smoke discipline ([feedback_smoke-test-gates](../README.md)): mock tests prove code; smoke tests prove reality.

Each sub-phase is its own scoped change. Per [Scope Isolation + Hardening Discipline](../README.md) (founder-validated 2026-05-29): do NOT bundle these with unrelated milestone work.

---

## 13. The 6-question gate for the implementation phase

Before any code lands implementing this capability, run the standard 6-question pre-phase gate. Proposed wording:

1. **Does this implementation phase make FOMO more real for users today (less noisy, more trustworthy), AND preserve the long-term Brevio agent OS direction?** (The two-question gate every phase must pass per [feedback_two-question-gate](../README.md).)

2. **Is the scope strictly within Personalized Importance Learning as defined in this doc, or has scope expanded to ranker rewrites, new email providers, or new tool capabilities?** If expanded, stop and re-gate separately.

3. **Are the new feedback-event kinds and memory-signal kinds end-to-end wired in this PR — producer, aggregator, ranker consumer, and eval — or is anything stubbed?** Per [feedback_real-or-absent-no-half-wired](../README.md): real or absent. No half-wired.

4. **Does the smoke test cover the cross-user contamination case (§10), and does the implementation pass it?** This is the load-bearing privacy / isolation test.

5. **Is every learned preference reversible by the user through a natural-language reply, and is that reply path tested?** (Guardrail 9.8.)

6. **Are the eval fixtures (§10) all synthetic? Has anyone committed a real user email — founder included — as a test case?** If yes, stop and replace before merge.

A "no" on any of the six blocks the phase until resolved.

---

## 14. Open questions for the founder (to resolve before scheduling)

These are NOT decisions for this doc. They are flagged so the founder can answer them before the implementation phase is scheduled.

1. **What is k** (the minimum count of consistent `mark_unimportant` events on the same sender within the recency window before Brevio promotes them to a hard suppression)? Suggested starting point: k = 3.
2. **What is the recency window** for signal decay? Suggested starting point: 90 days for full weight, linear decay over the following 90 days, near-zero by 180.
3. **Does Brevio propose conditional rules automatically** ("I noticed you snooze school emails until 7pm — want me to hold them?"), or only when asked? Suggested starting point: propose, but rate-limited to one proposal per week per user, and only when the underlying pattern has high confidence.
4. **Does the ranker see topic tags from a topic-extractor model**, or only from explicit user `conditional_surface` keyword rules? This affects compute cost (a topic extractor is another model call per email) and accuracy.
5. **How does Brevio expose the "why" answer to the user** — full structured explanation, or short human-readable summary? Suggested starting point: short summary by default ("you've replied 'this mattered' to this sender 3 times"), with an "explain more" follow-up that lists the contributing signals.
6. **What is the user-facing surface for inspecting and editing learned preferences?** iMessage replies only (per the no-dashboard rule in v0.1), or eventually a minimal web page? Defer to the dashboard-or-not future decision.

---

## 15. Where this principle lives in the codebase

Wiring summary (for future-author orientation):

* **This doc:** `docs/personalized-importance-learning.md` — canonical source of truth.
* **[FOMO_DESIGN.md §29 (or appropriate next section)](../FOMO_DESIGN.md):** the product-direction section pointing back here; established as a permanent product principle on the same tier as the existing learning-tier discipline (§13).
* **[FOMO_PLAN.md §17 — Implementation Milestones](../FOMO_PLAN.md):** future phase candidate `Personalized Importance Learning / False-Positive Reduction` listed (phase number unlocked).
* **[CLAUDE.md](../CLAUDE.md):** one-line standing instruction pointing future Claude sessions here before they touch ranker, feedback, memory, or commercial / spam handling.
* **Existing v0.1 substrate this builds on:** `feedback_events` table (FOMO_DESIGN.md §13, FOMO_PLAN.md §9.6), `memory_signals` table (FOMO_DESIGN.md §14, FOMO_PLAN.md §9.7), reply parser (FOMO_DESIGN.md §11), ranker context budget (FOMO_DESIGN.md §17).
* **Existing memories this respects:** [feedback_multitenant-design-principles] (per-user keyspace, no cross-user leakage), [feedback_real-or-absent-no-half-wired] (end-to-end wired or absent), [feedback_inmemory-mock-divergence] (gated PG tests required for new kinds), [feedback_smoke-test-gates] (founder-only smoke before next phase), [feedback_two-question-gate] (FOMO more real + long-term direction preserved), [feedback_scope-isolation-and-hardening-discipline] (don't bundle with milestone PRs), [feedback_email-context-provider-abstraction] (don't bake Gmail-specific assumptions).

---

## 16. The permanent quotes

For preservation across future rewrites:

> **Brevio must learn each user's personal definition of important.**

> **Brevio should become less noisy over time without becoming blind.**

> **Personalized learning must improve interruption quality without silently suppressing genuinely important messages.**

These three are the contract. Every implementation decision must serve them. When in doubt during a future phase: read this doc first; let these three quotes pick the tie-breaker; if a proposed change violates any of them, the change is wrong.
