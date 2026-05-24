// Ranker prompt — Phase 3C.1.
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

import { type RankerEgressView } from '../core/egress-policy.js';

export const PROMPT_VERSION = 'ranker-v0.1.0';

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
export const RANKER_SYSTEM_PREAMBLE = [
  'You are Brevio FOMO, deciding whether an email is important enough to alert the user about by iMessage.',
  '',
  'Rules:',
  "- Only label an email \"important\" if the user would be genuinely sad to miss it — e.g. a counselor, doctor, school, employer, family, or close friend asking for something time-sensitive.",
  '- Default to "not_important". When in doubt, "not_important".',
  '- Marketing, newsletters, social-network digests, transactional confirmations, and automated notifications are "not_important" unless they carry a deadline that affects the user directly.',
  '- Do NOT use the body snippet as the sole signal. Sender + subject usually matter more.',
  '- Output ONLY a single-line JSON object, no markdown, no commentary.'
].join('\n');

// The strict shape the validator will require.
export const RANKER_OUTPUT_SCHEMA = [
  'Output schema (single-line JSON, exact keys):',
  '{"label":"important"|"not_important","score":<number 0..1>,"reason":<short string, <=120 chars, no PII>}',
  '- "score" is the model\'s confidence that label="important" is correct (0..1).',
  '- "reason" is a brief operational explanation. Do NOT quote the email body or sender address in "reason"; describe the signal at a high level (e.g. "counselor / time-sensitive request", "weekly newsletter digest").'
].join('\n');

// Build the user-message portion of the prompt for a single email view.
// The egress view already strips forbidden fields; this function only
// formats what's allowed.
export function buildRankerPrompt(view: RankerEgressView): string {
  const attachmentLine =
    view.has_attachments
      ? `Attachments: ${view.attachment_count} (filenames withheld by egress policy)`
      : 'Attachments: none';
  const senderLine = view.sender_name
    ? `From: ${view.sender_name} <${view.sender_email}>`
    : `From: ${view.sender_email}`;

  return [
    RANKER_SYSTEM_PREAMBLE,
    '',
    RANKER_OUTPUT_SCHEMA,
    '',
    'Email to classify:',
    senderLine,
    `Subject: ${view.subject}`,
    `Received: ${view.received_at}`,
    attachmentLine,
    'Body snippet (truncated, no HTML, no headers):',
    view.body_snippet
  ].join('\n');
}
