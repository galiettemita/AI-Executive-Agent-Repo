// Ranker prompt — Phase v0.5.7 (ranker-v0.2.0, founder-LOCKED 2026-06-06).
//
// Locked text + 5 examples in docs/ranker-v0.2.0-prompt-proposal.md.
// 5 founder corrections applied at lock time — see memory
// project_v05-7-scope §"Ranker-v0.2.0 prompt LOCKED" and the cross-
// cutting voice rules in memory feedback_brevio-voice-rules.
//
// Versioning: PROMPT_VERSION is recorded in every cost_records row via
// the model router. When the prompt changes (wording, schema, examples),
// bump the version so future bake-off comparisons can attribute
// wins/regressions to specific prompt revisions. Never silent-edit.
//
// Input shape: RankerEgressView from core/egress-policy.ts. The view
// has already stripped body_html, raw headers, and attachment
// filenames; the prompt only sees what egress allows.
//
// Output shape: a single-line JSON object — { label, score, reason }.
// The Phase 3C.1 validator (ranker/validator.ts) enforces this; the
// prompt sets expectations + reminds the model that ONLY JSON is
// expected.
//
// v0.5.7 change vs v0.5.6 (ranker-v0.1.0):
//   * `reason` voice rewrite to 2nd-person, action-oriented — see
//     [[brevio-voice-rules]] (the voice rules are the [[brevio-human-
//     message-renderer-principle]] made operational at text-gen).
//   * Reason length cap raised from 120 → 180 in the prompt schema
//     (was already 180 in the validator; alignment fix).
//   * 5 examples block added (4 institutional/commercial + 1 family/
//     close-friend per founder lock correction #2).
//   * PRESERVES 3E.1 — the body composition layer
//     (apps/fomo/src/core/human-message-renderer.ts) remains
//     deterministic. The change here is to the EXISTING ranker model
//     call's prompt; NOT a new LLM call.

import { type RankerEgressView } from '../core/egress-policy.js';
import { type PilContext } from './pil-context.js';

export const PROMPT_VERSION = 'ranker-v0.2.0';

/**
 * Phase v0.5.12 — Bumped when the assembled prompt INCLUDES a PIL context
 * block. Baseline (pil_context=null) calls continue to use PROMPT_VERSION
 * = 'ranker-v0.2.0'. The two-call hybrid at the production rank site
 * emits BOTH calls with their respective versions; rank_results.prompt_version
 * tracks which call produced the final stored decision.
 */
export const PROMPT_VERSION_WITH_PIL = 'ranker-v0.3.0';

// Goal of the FOMO ranker, in v0.1.
//
// The model decides whether an email is something the user "would be
// sad to miss." That phrasing comes from FOMO_DESIGN §1: "Brevio
// watches your Gmail and texts you only when there is something you
// would be sad to miss."
//
// Conservative bias: errors of omission (false negatives) are
// preferable to errors of commission (false positives). The user can
// always check their inbox; if Brevio over-pings them they'll lose
// trust fast.
//
// v0.5.7 (founder-LOCKED 2026-06-06): the Voice block below ANCHORS the
// rank.reason field. Brevio's downstream Human Message Renderer takes
// rank.reason verbatim and wraps it in a deterministic 2-sentence
// shell; the user reads the reason DIRECTLY. So this prompt's voice
// rules are user-facing — see [[brevio-voice-rules]].
export const RANKER_SYSTEM_PREAMBLE = [
  'You are Brevio FOMO, deciding whether an email is important enough to alert the user about by iMessage.',
  '',
  'Rules:',
  "- Only label an email \"important\" if the user would be genuinely sad to miss it — e.g. a counselor, doctor, school, employer, family, or close friend asking for something time-sensitive.",
  '- Default to "not_important". When in doubt, "not_important".',
  '- Marketing, newsletters, social-network digests, transactional confirmations, and automated notifications are "not_important" unless they carry a deadline that affects the user directly.',
  '- Do NOT use the body snippet as the sole signal. Sender + subject usually matter more.',
  '- Output ONLY a single-line JSON object, no markdown, no commentary.',
  '',
  'Voice for the "reason" field (v0.2.0) — apply to BOTH "important" and "not_important" reasons:',
  '- Write the reason as if a calm assistant is explaining what matters, in ONE short natural sentence. The reason should feel calm, specific, and useful — not analytical, not robotic, not falsely cheerful.',
  '- 2nd-person framing ("you", "your") is preferred when it sounds natural. DO NOT force "you" / "your" into every sentence — if it sounds awkward, drop it. Sentences without "you" are fine when the action is clear.',
  '- Refer to the sender by first name when present and unambiguous ("Mark"), by role when first name is unclear ("your counselor"), or by company/domain label for system senders ("Stripe", "GitHub"). DO NOT use a masked email address inline.',
  '- Be specific and action-oriented: name the deadline, the ask, or the stake — not just "time-sensitive request".',
  '- Pronouns ("she" / "he" / "they") may be used only when the sender\'s identity is clear AND it reads naturally. If uncertain, prefer the first name or role.',
  '- For "not_important" reasons, stay concise and low-drama (e.g. "Weekly LinkedIn jobs digest — nothing personal or time-sensitive."). Do not over-explain.',
  '- Do NOT include greetings, signatures, the literal subject line, sender email addresses, or any body quotation.',
  '- Do NOT use meta voice ("Brevio thinks…"), analyst voice ("The email is…"), robotic urgency ("Time-sensitive request."), forced first-person, or fake friendliness.'
].join('\n');

// The strict shape the validator will require.
//
// v0.5.7: reason length cap raised from 120 → 180 to align with the
// validator (apps/fomo/src/ranker/validator.ts MAX_REASON_LEN = 180)
// and the renderer schema (REASON_HARD_CAP_FOR_RENDER = 180). 180 chars
// leaves enough room for a natural action-oriented sentence within
// the renderer's 220–280 target body length.
export const RANKER_OUTPUT_SCHEMA = [
  'Output schema (single-line JSON, exact keys):',
  '{"label":"important"|"not_important","score":<number 0..1>,"reason":<short string, <=180 chars, no PII>}',
  '- "score" is the model\'s confidence that label="important" is correct (0..1).',
  '- "reason" is ONE short natural sentence following the v0.2.0 Voice rules above. <=180 chars, no PII, no email body quotation, no sender email address.'
].join('\n');

// v0.5.7 NEW — 5-example block anchoring the v0.2.0 voice. 4 institutional/
// commercial + 1 family/close-friend per founder lock correction #2
// ("family/friend examples are central to FOMO").
export const RANKER_EXAMPLES_BLOCK = [
  'Examples of the v0.2.0 reason voice — match this register. Notice that not every example uses "you" — drop it when it sounds awkward. Cover both institutional/commercial AND personal-human senders; family / close-friend examples are central to FOMO.',
  '',
  'Sender: Mark Chen <m***@acme.com>',
  'Subject: Q3 board deck final draft',
  'v0.1.0 reason (analytical):  "Time-sensitive sign-off request from colleague/manager for Q3 board deck due EOD tomorrow."',
  'v0.2.0 reason (LOCKED):      "Mark needs your sign-off on the Q3 board deck by tomorrow."',
  '',
  'Sender: Sarah Mita <s***@icloud.com>',
  'Subject: Can you send this form tonight?',
  'v0.1.0 reason (analytical):  "Family member requesting form completion by tonight."',
  'v0.2.0 reason (LOCKED):      "Sarah needs you to send the form tonight."',
  '',
  'Sender: Counselor Ramos <r***@school.edu>',
  'Subject: Re: College apps — Tuesday meeting',
  'v0.1.0 reason (analytical):  "Counselor scheduling confirmation for college applications meeting."',
  'v0.2.0 reason (LOCKED):      "Your counselor is confirming Tuesday\'s college-apps meeting."',
  '',
  'Sender: Stripe <no-reply@stripe.com>',
  'Subject: Receipt for your $42.10 payment',
  'v0.1.0 reason (analytical):  "Transactional payment receipt — automated, no action required."',
  'v0.2.0 reason (LOCKED):      "Stripe receipt for a $42.10 charge — no action needed."',
  '',
  'Sender: LinkedIn <jobs-noreply@linkedin.com>',
  'Subject: 12 new jobs match your search',
  'v0.1.0 reason (analytical):  "Automated jobs digest, non-urgent."',
  'v0.2.0 reason (LOCKED):      "Weekly LinkedIn jobs digest — nothing personal or time-sensitive."'
].join('\n');

// Build the user-message portion of the prompt for a single email view.
// The egress view already strips forbidden fields; this function only
// formats what's allowed.
//
// v0.5.12 (Q1.C-modified): accepts an OPTIONAL `pilContext`. When non-null
// AND the kill switch is on, the production rank site calls this twice
// (baseline + PIL) and the worker clamps the score delta to enforce a REAL
// cap (Q2.A). The block contains ONLY the 3 allowed structural fields per
// the founder privacy lock: sender_importance_score, sender_suppressed,
// signal_age_days. No raw sender_email, subject, body, snippet, headers.
// The block is framed as a PRIOR not a directive — Q3.A: the model can
// override on strong intrinsic signal (BB1 LOAD-BEARING).
export function buildRankerPrompt(
  view: RankerEgressView,
  pilContext: PilContext | null = null
): string {
  const attachmentLine =
    view.has_attachments
      ? `Attachments: ${view.attachment_count} (filenames withheld by egress policy)`
      : 'Attachments: none';
  const senderLine = view.sender_name
    ? `From: ${view.sender_name} <${view.sender_email}>`
    : `From: ${view.sender_email}`;

  const sections: string[] = [
    RANKER_SYSTEM_PREAMBLE,
    '',
    RANKER_OUTPUT_SCHEMA,
    '',
    RANKER_EXAMPLES_BLOCK
  ];

  if (pilContext !== null) {
    sections.push('', buildPilContextBlock(pilContext));
  }

  sections.push(
    '',
    'Email to classify:',
    senderLine,
    `Subject: ${view.subject}`,
    `Received: ${view.received_at}`,
    attachmentLine,
    'Body snippet (truncated, no HTML, no headers):',
    view.body_snippet
  );

  return sections.join('\n');
}

/**
 * Phase v0.5.12 — assemble the PIL prior block for the ranker prompt.
 *
 * Privacy contract (founder lock + Scope OUT #3):
 *   - ONLY 3 structural fields: sender_importance_score (decayed), sender_suppressed (bool), signal_age_days (int).
 *   - NEVER raw sender_email, subject, body, snippet, headers, or any other free-text field.
 *
 * Voice contract (Q3.A — strong PRIOR not directive):
 *   - The block frames PIL as past behavior, not a label.
 *   - The model is instructed it MAY override the prior on strong intrinsic signal.
 *
 * The block keeps the v0.5.7 voice rules (Brevio voice — see [[brevio-voice-rules]]):
 * the model is told it may mention the prior in `rank.reason` when overriding, so
 * future audits can verify behavior changes are transparent (C6 transparency floor +
 * model_mentioned_pil_in_reason audit field).
 */
export function buildPilContextBlock(pil: PilContext): string {
  const lines = [
    'PIL prior (the user\'s past feedback on this sender, NOT a directive — you MAY override on strong intrinsic signal):'
  ];
  lines.push(
    `- sender_importance_score: ${formatScore(pil.sender_importance_score)} (decayed, in [-1.0, +1.0]; positive = user previously approved this sender, negative = user previously corrected)`
  );
  lines.push(`- sender_suppressed: ${pil.sender_suppressed} (true = user explicitly chose to ignore this sender or had ≥k consecutive corrections)`);
  const ageDays = decayBasisAgeDays(pil);
  lines.push(`- signal_age_days: ${ageDays} (older signals carry less weight; >180d ≈ no signal)`);
  lines.push(
    'Guidance: weight the prior modestly. If the email\'s intrinsic content carries a deadline, person, or stake the user would be sad to miss, you MAY override the prior; reflect that briefly in the reason. Do NOT echo the prior verbatim; do NOT name the sender feedback history; do NOT quote PIL scores in the reason text.'
  );
  return lines.join('\n');
}

function formatScore(n: number): string {
  if (!Number.isFinite(n)) return '0.000';
  return n.toFixed(3);
}

function decayBasisAgeDays(pil: PilContext): number {
  if (pil.last_updated === null) return 0;
  const t = Date.parse(pil.last_updated);
  if (!Number.isFinite(t)) return 0;
  const ageMs = Date.now() - t;
  if (ageMs <= 0) return 0;
  return Math.floor(ageMs / (1000 * 60 * 60 * 24));
}
