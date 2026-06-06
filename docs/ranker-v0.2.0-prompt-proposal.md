# Ranker Prompt v0.2.0 — LOCKED

> **Status:** LOCKED 2026-06-06 — founder-approved with 5 corrections (see "Founder review answers" below).
> **Phase:** v0.5.7 — Human Message Renderer (Q4.A lock).
> **3E.1 compliance:** This is a CHANGE to the existing ranker prompt, NOT a new LLM call. The body composition (`renderHumanMessage`) remains deterministic. The only model-generated text in the body remains `rank.reason` (the 3E.1 carve-out, unchanged).
>
> **Next step:** runtime commit applies the LOCKED text below verbatim and bumps `PROMPT_VERSION` from `'ranker-v0.1.0'` to `'ranker-v0.2.0'`.

## Founder review answers (locked 2026-06-06)

| # | Question | Lock |
|---|---|---|
| 1 | Voice tightening | Direction right; **DO NOT force "you/your" into every reason if it sounds awkward**. Calm assistant register, not forced 2nd person. Preferred style anchored by 3 founder examples; avoid meta voice, analyst voice, robotic urgency, forced first-person, fake friendliness. |
| 2 | Examples | Keep all 4. **ADD a 5th: family / close-friend asking for something time-sensitive** (founder-provided example below). Need both institutional/commercial AND personal-human examples — family/friend examples are central to FOMO. |
| 3 | Length cap | **Keep 180 chars.** Do NOT tighten to 140 — risks recreating the "too clipped" problem. Renderer has its own overall body budget. |
| 4 | Apply to `not_important` reasons too | **YES — apply v0.2.0 voice to BOTH `important` and `not_important`.** `not_important` reasons feed rank_results, evals, future PIL, and audit/debugging — they must be human-readable and consistent. Keep concise and low-drama. |
| 5 | Pronouns | Use **only when safe and natural**. Default to first name or role. Use "she/he/they" only when sender identity is clear AND it reads naturally. For system senders, use company/domain label, not pronouns. If uncertain → first name / role. |

---

## What changes

| Aspect | v0.1.0 (current) | v0.2.0 (proposed) |
|---|---|---|
| `PROMPT_VERSION` | `'ranker-v0.1.0'` | `'ranker-v0.2.0'` |
| `label` behavior | unchanged | unchanged |
| `score` behavior | unchanged | unchanged |
| `reason` voice | 3rd-person analytical ("Time-sensitive sign-off request from colleague/manager for Q3 board deck due EOD tomorrow.") | **2nd-person action-oriented** ("It looks time-sensitive — she needs sign-off by tomorrow.") |
| `reason` length cap | `<=120 chars, no PII` (in prompt; validator allowed up to 180) | `<=180 chars, no PII` (aligned with v0.5.6 `REASON_HARD_CAP_FOR_RENDER`) |
| `reason` framing | "brief operational explanation" / "describe the signal at a high level" | "natural 1-sentence explanation in 2nd-person of what the user needs to do or know" |
| Examples added | none | 4 before/after pairs to anchor the voice |

## What does NOT change

- Conservative bias: errors of omission preferred over commission (FOMO_DESIGN §1)
- Default to `not_important`; when in doubt, `not_important`
- Marketing / newsletters / digests / transactional → `not_important` unless personal deadline
- Sender + subject usually matter more than body snippet
- Output: single-line JSON only, no markdown, no commentary
- No PII in `reason` (no email addresses, no body quotations)
- Egress projection (`RankerEgressView`) — the prompt only sees what egress allows

## Why 2nd-person, action-oriented

v0.5.6 smoke surfaced that the rendered iMessage body is still field-shaped. The Human Message Renderer (v0.5.7) wraps the reason into a natural sentence like *"Galiette emailed you about the Q3 board deck. It looks time-sensitive — she needs sign-off by tomorrow."* For that second sentence to read naturally to the **user**, the ranker's `reason` field must already be written **about the user** (2nd person) and **about what they need to do**, not as an analyst's note about the sender. See [memory: brevio-human-message-renderer-principle](../.claude/projects/-Users-galiettemita-Downloads-Executive-AI-Agent-backend/memory/feedback_brevio-human-message-renderer-principle.md).

This preserves [3E.1](../.claude/projects/-Users-galiettemita-Downloads-Executive-AI-Agent-backend/memory/feedback_3e1-no-llm-body-generation.md) by design: the renderer stays deterministic and gets a more-conversational input rather than rewriting the output. The only model-generated text in the body is still `rank.reason` — same carve-out, different voice.

## Proposed prompt text (FOR REVIEW)

### `RANKER_SYSTEM_PREAMBLE` (v0.2.0 LOCKED)

```
You are Brevio FOMO, deciding whether an email is important enough to alert the user about by iMessage.

Rules:
- Only label an email "important" if the user would be genuinely sad to miss it — e.g. a counselor, doctor, school, employer, family, or close friend asking for something time-sensitive.
- Default to "not_important". When in doubt, "not_important".
- Marketing, newsletters, social-network digests, transactional confirmations, and automated notifications are "not_important" unless they carry a deadline that affects the user directly.
- Do NOT use the body snippet as the sole signal. Sender + subject usually matter more.
- Output ONLY a single-line JSON object, no markdown, no commentary.

Voice for the "reason" field (v0.2.0) — apply to BOTH "important" and "not_important" reasons:
- Write the reason as if a calm assistant is explaining what matters, in ONE short natural sentence. The reason should feel calm, specific, and useful — not analytical, not robotic, not falsely cheerful.
- 2nd-person framing ("you", "your") is preferred when it sounds natural. DO NOT force "you" / "your" into every sentence — if it sounds awkward, drop it. Sentences without "you" are fine when the action is clear.
- Refer to the sender by first name when present and unambiguous ("Mark"), by role when first name is unclear ("your counselor"), or by company/domain label for system senders ("Stripe", "GitHub"). DO NOT use a masked email address inline.
- Be specific and action-oriented: name the deadline, the ask, or the stake — not just "time-sensitive request".
- Pronouns ("she" / "he" / "they") may be used only when the sender's identity is clear AND it reads naturally. If uncertain, prefer the first name or role.
- For "not_important" reasons, stay concise and low-drama (e.g. "Weekly LinkedIn jobs digest — nothing personal or time-sensitive."). Do not over-explain.
- Do NOT include greetings, signatures, the literal subject line, sender email addresses, or any body quotation.
- Do NOT use meta voice ("Brevio thinks…"), analyst voice ("The email is…"), robotic urgency ("Time-sensitive request."), forced first-person, or fake friendliness.
```

### `RANKER_OUTPUT_SCHEMA` (v0.2.0 draft)

```
Output schema (single-line JSON, exact keys):
{"label":"important"|"not_important","score":<number 0..1>,"reason":<short string, <=180 chars, no PII>}
- "score" is the model's confidence that label="important" is correct (0..1).
- "reason" is ONE short natural sentence following the v0.2.0 Voice rules above. <=180 chars, no PII, no email body quotation, no sender email address.
```

### Examples block (NEW in v0.2.0 — append after schema, before `Email to classify:`)

```
Examples of the v0.2.0 reason voice — match this register. Notice that
not every example uses "you" — drop it when it sounds awkward. Cover both
institutional/commercial AND personal-human senders; family / close-friend
examples are central to FOMO.

Sender: Mark Chen <m***@acme.com>
Subject: Q3 board deck final draft
v0.1.0 reason (analytical):  "Time-sensitive sign-off request from colleague/manager for Q3 board deck due EOD tomorrow."
v0.2.0 reason (LOCKED):      "Mark needs your sign-off on the Q3 board deck by tomorrow."

Sender: Sarah Mita <s***@icloud.com>
Subject: Can you send this form tonight?
v0.1.0 reason (analytical):  "Family member requesting form completion by tonight."
v0.2.0 reason (LOCKED):      "Sarah needs you to send the form tonight."

Sender: Counselor Ramos <r***@school.edu>
Subject: Re: College apps — Tuesday meeting
v0.1.0 reason (analytical):  "Counselor scheduling confirmation for college applications meeting."
v0.2.0 reason (LOCKED):      "Your counselor is confirming Tuesday's college-apps meeting."

Sender: Stripe <no-reply@stripe.com>
Subject: Receipt for your $42.10 payment
v0.1.0 reason (analytical):  "Transactional payment receipt — automated, no action required."
v0.2.0 reason (LOCKED):      "Stripe receipt for a $42.10 charge — no action needed."

Sender: LinkedIn <jobs-noreply@linkedin.com>
Subject: 12 new jobs match your search
v0.1.0 reason (analytical):  "Automated jobs digest, non-urgent."
v0.2.0 reason (LOCKED):      "Weekly LinkedIn jobs digest — nothing personal or time-sensitive."
```

### `buildRankerPrompt` (unchanged structurally)

The function shape stays the same; only the system preamble and schema text change (and the examples block appends). The egress projection is unchanged.

---

## Anti-patterns the v0.2.0 voice must NOT produce

- ❌ "Brevio thinks this is important because…" (meta voice)
- ❌ "The email is a request from Mark Chen for…" (analyst voice — 3rd-person about everything)
- ❌ "You should sign off on the Q3 board deck because Mark Chen <m***@acme.com> sent it at 14:32 UTC about Q3 board deck final draft" (PII + literal field dumping)
- ❌ "Time-sensitive!" (no specifics — vague urgency adjectives)
- ❌ "Subject: Q3 board deck final draft" (literal subject quotation)
- ❌ "Hi! Just wanted to let you know…" (greetings)

## Edge cases to think through

1. **Sender first name unknown / ambiguous** — the renderer (Q2.B chain) might surface "Someone" or a domain label. The reason should still read naturally even if it doesn't say a name. Drop "she/he" framing in that case; lean on the action.
2. **Reason runs long (>180)** — validator enforces 180 cap (existing v0.5.6 schema). Q5.A fallback fires → renderer substitutes `'Marked important by Brevio.'`, `reason_voice='fallback'`, `fomo.alert.hmr_degradation_applied` audits the path. **DO NOT have the model truncate with `…`** — let the validator catch it; the deterministic fallback is the right behavior.
3. **Non-English email** — current scope is English-only (matches v0.5 substrate). If a non-English email lands and the model writes the reason in the email's language, that's acceptable; render quality may suffer but no failure mode.
4. **"Not important" reasons** — even though those don't go to iMessage, they DO go into rank_results and `[[personalized-importance-learning]]` will eventually tune from them. 2nd-person voice still applies: "Weekly LinkedIn jobs digest — nothing personal or time-sensitive." reads better than "Automated jobs digest, non-urgent."

## Roll-out behavior (Q5.A `reason_voice` audit field)

The runtime commit captures the ranker's `PROMPT_VERSION` in `fomo.send.attempted` detail (already does via cost_records; v0.5.7 surfaces it into the send-attempt detail explicitly). The renderer reads it and stamps:

- `reason_voice = '2p_action'` if `PROMPT_VERSION == 'ranker-v0.2.0'` AND `rank.reason` passes schema
- `reason_voice = 'legacy_3p'` if `PROMPT_VERSION == 'ranker-v0.1.0'` AND `rank.reason` passes schema (transitional — pre-rollout rows)
- `reason_voice = 'fallback'` if `rank.reason` fails schema and the deterministic fallback string is substituted

Smoke-evidence C9 reads the distribution. Mix of `2p_action` and `legacy_3p` is acceptable during rollout; `fallback` rate is operator-judged in §11.

---

## Questions for founder before lock — ANSWERED + LOCKED 2026-06-06

See "Founder review answers" table at top of doc. All 5 answered; prompt LOCKED.

---

## After founder approval (LOCKED — runtime commit MAY proceed)

1. Runtime commit edits `apps/fomo/src/ranker/prompt.ts` — replaces `RANKER_SYSTEM_PREAMBLE` (with the locked voice rules including "don't force you/your" + "apply to both important AND not_important"), `RANKER_OUTPUT_SCHEMA` (180-char cap retained), appends the 5-example block (including the family/close-friend example), bumps `PROMPT_VERSION` to `'ranker-v0.2.0'`.
2. Existing ranker test suite (`apps/fomo/src/ranker/*.test.ts`) updated with new schema cap (180) + new prompt-version assertion.
3. New runtime test: `human-message-renderer.test.ts` "reason_voice routing" suite — asserts `reason_voice` audit field is populated correctly for each `(PROMPT_VERSION, reason_validity)` combination.
4. v0.5.7 smoke (§6 Test 3 fixture) generates N samples with both `ranker-v0.1.0` (legacy_3p) and `ranker-v0.2.0` (2p_action) fixtures so founder can eye-test the voice change side-by-side.
