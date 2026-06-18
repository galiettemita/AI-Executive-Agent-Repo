# Brevio Product Philosophy

> Founder-locked 2026-06-06. Permanent product layers that must guide every future phase of Brevio. Read this BEFORE designing any user-facing message, ranker behavior, reply parser logic, memory signal, feedback event, or new agentic surface.

## The motto

**Brevio should speak like a human, learn what matters, and grow from feedback.**

## The only goal

**Brevio's only goal is to serve the user better over time.**

Every decision — every prompt, every audit field, every retry, every fallback, every UX choice — must be evaluated against "does this make Brevio serve THIS user better over time?" If a feature does not contribute to that, it does not belong in Brevio.

## The three permanent product layers

These are not features. They are not phases. They are permanent foundations. Every Brevio surface — current and future — must preserve all three.

### 1. Human Message Renderer (HMR)

Brevio must turn structured context into natural, useful, human-feeling messages. The internal system may reason in metadata, but the user should receive meaning.

- Brevio must NOT dump sender / subject / reason / raw fields as the user experience.
- The user-facing output should always feel like a calm assistant explaining what matters — not an alert system emitting structured logs.
- HMR is Brevio-owned, not outsourced to a vendor — it touches user trust, privacy, memory, tone, and the feeling that Brevio is actually a personal assistant.
- HMR composes deterministically (no LLM at body-render); model-generated text is restricted to safe, structured fields like `rank.reason`.

First surface: email alerts (v0.5.7 — `apps/fomo/src/core/human-message-renderer.ts`).

### 2. Personalized Importance Learning (PIL)

Brevio must learn each user's personal definition of important. The signal lives in the user, not in the email.

- Urgent wording does not always mean important.
- Commercial does not always mean unimportant.
- A counselor email may be important for one user and noise for another.
- Learning must be:
  - **per-user** — no cross-user leakage
  - **auditable** — every adjustment reviewable in audit_log
  - **reversible** — the user must be able to undo any learned signal
  - **safe** — no over-aggressive suppression that hides genuinely important things

Substrate sketched in [docs/personalized-importance-learning.md](personalized-importance-learning.md). No implementation yet.

### 3. Feedback + Learn/Grow Loop

Brevio must learn and grow from user feedback so it can serve the user better over time.

- Every Brevio interaction should be learnable, but **not every interaction should nag the user**.
- The user should always be able to correct Brevio naturally — not via robotic command syntax.
- Brevio should ask for feedback **more during onboarding / early training**, **less as confidence improves**, and **intelligently when uncertain** (high-information-gain moments).
- Feedback storage must be per-user, auditable, reversible — same invariants as PIL.
- Feedback is the input signal PIL learns from. They are designed together; ship them in coordination.

No implementation yet.

## Feedback UX correction (founder-locked)

When asking for feedback, never sound robotic. Feedback prompts are themselves Human Message Renderer output — they must feel like a person checking in, not a survey form.

**Bad (forbidden):**

- "Reply 'not important' if I got this wrong."
- "Send 'mark as spam' to suppress this sender."
- "Type 'feedback yes' to confirm."

**Better (the bar):**

- "Was this the kind of thing you want me to catch?"
- "Too much? Just tell me and I'll learn."
- "If this wasn't worth the ping, you can say so."
- "Am I getting the right kinds of things so far?"

## Cross-cutting: every Brevio surface

These three layers apply to EVERY user-facing Brevio surface, present and future:

- email alerts (v0.5.x — first HMR surface)
- calendar reminders
- draft suggestions
- task updates
- booking / payment preparation
- tool results
- browser automation summaries
- "why did you send this?" answers
- memory explanations
- future delegated agents

## What this is NOT

- NOT email-only
- NOT a one-off iMessage feature
- NOT a prompt tweak
- NOT an A/B test of copy
- NOT a SaaS / vendor renderer
- NOT cross-user learning ("users like you also flagged…")
- NOT auto-send
- NOT a replacement for founder review during early phases

## Phase-gate addition — every future 6-question gate must answer

In addition to the existing two-question-gate discipline (does this make FOMO more real AND preserve the long-term Brevio agent-OS direction), every future 6-question gate must answer:

1. **Does this preserve the Human Message Renderer principle?** Does the change produce field-shaped metadata anywhere the user might see it? If yes, the change is not ready.
2. **Does this preserve Personalized Importance Learning?** Does the change hardcode "important = X" or "spam = Y" globally? If yes, the change must be scoped per-user OR explicitly carved out with founder approval.
3. **Does this preserve the Feedback + Learn/Grow Loop?** Does the change add a forced nag, a robotic command syntax, or a feedback path that ignores the user's signal? If yes, the change is not ready.

These three principle-gates do NOT replace the per-phase Q1-Q6. They are checks every gate must clear BEFORE locking scope.

## Implementation status (as of 2026-06-06)

| Layer | Status | First Surface |
|---|---|---|
| Human Message Renderer | v0.5.7 in flight (PR #46 runtime; smoke in progress) | email alerts |
| Personalized Importance Learning | substrate scoped per docs/personalized-importance-learning.md; no runtime | future phase |
| Feedback + Learn/Grow Loop | conceptual; no runtime | future phase |

## When to read this doc

Read this BEFORE designing or editing:

- any user-facing message text
- any ranker prompt / behavior change
- any reply parser pipeline change
- any new memory signal kind
- any new feedback event kind
- any new audit kind that touches user-facing surfaces
- any new agentic surface (calendar, drafts, tasks, browser, etc.)
- any per-user state machine that affects what the user sees

## Related references

Auto-memory:
- `feedback_brevio-human-message-renderer-principle` — HMR principle (operational)
- `feedback_brevio-voice-rules` — voice rules (cross-cutting)
- `feedback_3e1-no-llm-body-generation` — 3E.1 deterministic-body invariant
- `feedback_two-question-gate` — pre-phase confirmation discipline
- `feedback_smoke-test-gates` — smoke discipline
- `feedback_scope-isolation-and-hardening-discipline` — phase isolation
- `feedback_real-or-absent-no-half-wired` — no half-wired features

Project docs:
- [FOMO_DESIGN.md](../FOMO_DESIGN.md)
- [FOMO_PLAN.md](../FOMO_PLAN.md)
- [docs/personalized-importance-learning.md](personalized-importance-learning.md)
- [docs/BREVIO_MEMORY_AND_SKILL_OS.md](BREVIO_MEMORY_AND_SKILL_OS.md) — M0 doctrine for typed memory + reusable skills (canonical "what Brevio learns and how").
- [CLAUDE.md](../CLAUDE.md)

---

The next phase is decided at the next 6-question gate, with these three principle-gates as additional checks. No exceptions.
