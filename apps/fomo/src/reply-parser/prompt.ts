// Reply-parser classifier prompt — Phase 3F.1.
//
// Versioning: PROMPT_VERSION is recorded in every cost_records row via
// the model router (mirrors the ranker pattern). When the prompt
// changes (wording, intents, examples), bump the version so future
// bake-off comparisons can attribute wins/regressions to specific
// prompt revisions.
//
// Founder directive 2026-05-26:
//   * Compliance commands (STOP / UNSUBSCRIBE / CANCEL / START) are
//     handled by parseReplyDeterministic BEFORE this classifier runs.
//     The classifier never sees compliance commands and never decides
//     whether STOP means stop.
//   * Soft intents only: snooze / ignore / ignore_sender / why /
//     false_positive / unclear.
//   * If confidence is low (< 0.7), fail safe — orchestrator forces
//     'unclear' regardless of the model's intent choice.
//   * Use egress-redacted reply context only — no raw email body,
//     no raw headers, no attachment filenames, no PII.
//
// Input shape: ReplyParserEgressView from core/egress-policy.ts.
// The view has already stripped body content and headers; the prompt
// only sees the user's reply text plus minimal alert-context
// (subject, sender display name, message_id).
//
// Output shape: a single-line JSON object —
//   { intent, confidence, reason, snooze_hint }
// The classifier validator (validator.ts) enforces this; OpenAI
// strict mode (openai-response-format.ts) enforces it server-side too.

import { type ReplyParserEgressView } from '../core/egress-policy.js';

export const PROMPT_VERSION = 'reply-parser-v0.1.0';

// What the classifier is doing, in v0.1.
//
// The user received a FOMO iMessage about an email and texted back.
// The classifier picks ONE of six soft intents. The orchestrator
// already screened out STOP/UNSUBSCRIBE/CANCEL/START deterministically;
// the classifier MUST NOT echo those — pick the closest soft intent
// or 'unclear' instead.
//
// Conservative bias: when the message is short, ambiguous, or
// off-topic, choose 'unclear'. The downstream state machine refuses
// to act on unclear replies (no state transition past `replied`); a
// false-positive intent that triggers the wrong feedback event or
// memory_signal flip is worse than acknowledging receipt without
// action.
export const REPLY_PARSER_SYSTEM_PREAMBLE = [
  `You are Brevio FOMO, classifying a single user reply to a FOMO alert iMessage.`,
  ``,
  `The user previously received a FOMO iMessage about an important email and texted back. Your job is to map their reply to ONE intent so the system can act on it.`,
  ``,
  `Intents (pick EXACTLY one):`,
  `- "snooze"          — user wants the alert re-surfaced later (e.g. "later", "tomorrow", "remind me at 5pm", "not now", "hit me up after lunch").`,
  `- "ignore"          — user wants to dismiss this one alert (e.g. "ignore", "skip", "dismiss", "got it, not now").`,
  `- "ignore_sender"   — user wants to suppress THIS sender going forward (e.g. "never alert me about this sender again", "mute Sarah", "stop pinging me about emails from this person").`,
  `- "why"             — user wants to understand WHY this email was flagged (e.g. "why", "why this one?", "how come?").`,
  `- "false_positive"  — user is telling Brevio the email was NOT actually important (e.g. "not important", "this isn't urgent", "not worth flagging", "shouldn't have alerted me").`,
  `- "unclear"         — the reply doesn't map cleanly to any of the above. Use freely when in doubt.`,
  ``,
  `Strict rules:`,
  `- Output ONLY a single-line JSON object, no markdown, no commentary.`,
  `- If the message contains the literal compliance words STOP / UNSUBSCRIBE / CANCEL / START (case-insensitive, with or without punctuation), the deterministic pre-pass already handled them. DO NOT output snooze / ignore / false_positive for those; pick "unclear" if you somehow see them.`,
  `- Set confidence in [0, 1]. Use < 0.7 when the reply is short, vague, or off-topic. The orchestrator forces "unclear" when confidence < 0.7 regardless of your intent choice.`,
  `- snooze_hint: only set when intent === "snooze". One of: "later" (next few hours), "tomorrow" (next morning), "remind_me_later" (no specific time), "unspecified" (snooze but timing unclear). Use null for non-snooze intents.`,
  `- "reason" is a brief operational note about which signal you matched. Do NOT quote the user's reply verbatim; describe the signal at a high level (e.g. "asked for tomorrow", "requested explanation").`
].join('\n');

export const REPLY_PARSER_OUTPUT_SCHEMA = [
  `Output schema (single-line JSON, exact keys):`,
  `{"intent":<one of the 6>,"confidence":<number 0..1>,"reason":<short string, <=120 chars, no PII>,"snooze_hint":<"later"|"tomorrow"|"remind_me_later"|"unspecified"|null>}`,
  `- "intent" must be one of: snooze, ignore, ignore_sender, why, false_positive, unclear.`,
  `- "confidence" reflects how confident you are in this intent (0..1). Low confidence triggers a safe fallback downstream.`,
  `- "snooze_hint" is null when intent !== "snooze".`
].join('\n');

// Build the user-message portion of the prompt for a single reply
// view. The egress view already strips forbidden fields; this
// function only formats what's allowed.
//
// Privacy invariant: this prompt is the ONLY place the classifier
// sees the user's reply text. It does NOT see the original email
// body, headers, attachments, or any PII beyond what the egress
// policy explicitly permits (subject, sender display name,
// message_id, reply text).
export function buildReplyParserPrompt(view: ReplyParserEgressView): string {
  const senderLine = view.alert_sender_name
    ? `Sender of original email: ${view.alert_sender_name}`
    : `Sender of original email: (not available)`;

  return [
    REPLY_PARSER_SYSTEM_PREAMBLE,
    '',
    REPLY_PARSER_OUTPUT_SCHEMA,
    '',
    'Context (egress-redacted):',
    `Original alert subject: ${view.alert_subject}`,
    senderLine,
    '',
    'User reply text:',
    view.user_reply_text
  ].join('\n');
}
