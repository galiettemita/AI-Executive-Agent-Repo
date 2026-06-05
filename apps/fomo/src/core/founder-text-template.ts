// Founder Text Template — Phase 3E.1 → v0.5.6 hybrid rewrite.
//
// Founder directive 2026-05-25 (3E.1, PRESERVED):
//
//   "Do not add a new LLM or freeform 'assistant voice generation' for
//    3E.1. Use deterministic/template-based copy from safe fields only."
//
// Allowed inputs:
//   * sender display + masked email (egress-redacted via SlackEgressView)
//   * subject line
//   * ranker label / score / reason (rank.reason is the ONLY allowed
//     model-generated text in the body; the 3E.1 carve-out)
//
// Forbidden inputs (never touch this function):
//   * raw body_plain / body_html
//   * raw headers
//   * attachment filenames
//   * hallucinated explanations
//   * any model-generated text other than rank.reason
//   * body_snippet (v0.5.6: REPLACED by rank.reason as the prose-y bit
//     — the body snippet is email-content leakage that Friend B's
//     §10 feedback flagged. The reason field, by contrast, is the
//     ranker's natural-language "why this matters" explanation.)
//
// v0.5.6 founder-locked length policy (Q4, 2026-06-05):
//   * Target: 220–280 chars
//   * Hard max: 320 chars (340 absolute max only if implementation buffer)
//   * 1–2 short sentences max
//   * NO mid-sentence truncation
//   * NO ellipsis from arbitrary truncation
//
// Reason: "Brevio should feel like a helpful iMessage nudge, not a full
// email preview."
//
// v0.5.6 shape change (founder Q1–Q6 lock, 2026-06-05):
//   - Drop the "FOMO · IMPORTANT (0.92)" telemetry header entirely
//     (the most "robotic" thing Friend B reacted to in §10).
//   - Sentence-shaped composition, not newline-separated raw fields.
//   - Sentence-boundary truncation; never arbitrary ellipsis.
//   - Substitute rank.reason for body_snippet as the prose-y line.
//   - Bump FOUNDER_TEXT_TEMPLATE_VERSION so the shape change is
//     traceable in audit.detail.template_version.
//
// v0.5.6 Q6 failure-mode lock: if rank.reason is empty/whitespace OR
// length > REASON_HARD_CAP_FOR_RENDER, substitute the deterministic
// fallback string and return reason_source='fallback'. The caller
// (outbound-sender) writes fomo.alert.drafter_schema_failed audit on
// fallback. Best-effort audit, NO retry. PRESERVES 3E.1 by keeping
// the fallback deterministic, never LLM-generated.

import { type SlackEgressView } from './egress-policy.js';
import { type RankLabel } from '../memory/rank-results.js';

// Bump when the rendered shape changes. Audit detail captures this so
// operators can diff founder texts across versions during smoke.
//
// v0.5.6: bumped from 'founder-text-v0.1.0' to mark the deterministic-
// shell rewrite (drops header, sentence-shaped, sentence-boundary
// truncation, rank.reason substituted for body_snippet).
export const FOUNDER_TEXT_TEMPLATE_VERSION = 'founder-text-v0.2.0';

// v0.5.6 founder-locked length policy (Q4).
export const FOUNDER_TEXT_TARGET_MIN_CHARS = 220;
export const FOUNDER_TEXT_TARGET_MAX_CHARS = 280;
export const FOUNDER_TEXT_HARD_MAX_CHARS = 320;
export const FOUNDER_TEXT_ABSOLUTE_MAX_CHARS = 340;

// Per-line caps. The whole body must still respect HARD_MAX; the
// per-line caps keep any one field from dominating the message.
const MAX_SENDER_LINE_CHARS = 60;
const MAX_SUBJECT_LINE_CHARS = 80;
// rank.reason is the prose-y line; gets the biggest slot so the
// "why this matters" can read like a sentence. Aligns with
// MAX_REASON_LEN in apps/fomo/src/ranker/validator.ts (180); if
// the validator ever changes, this stays the source of truth at
// the body-render layer.
const REASON_HARD_CAP_FOR_RENDER = 180;

// Deterministic fallback substituted when rank.reason fails the
// body-render schema. PRESERVES 3E.1: never LLM-generated; never
// uses email content. Length is within budget by construction.
export const REASON_FALLBACK_STRING = 'Marked important by Brevio.';

export type ReasonViolationKind = 'empty' | 'too_long';

export interface FounderTextTemplateInput {
  readonly view: SlackEgressView;
  readonly rank: {
    readonly label: RankLabel;
    readonly score: number;
    // v0.5.6: rank.reason is now part of the template input (was
    // previously only used in Slack cards). The 3E.1 carve-out
    // permits this as the ONLY model-generated text in the body.
    readonly reason: string;
  };
}

export interface FounderTextTemplateOutput {
  readonly text: string;
  readonly template_version: string;
  // v0.5.6: tells the caller (outbound-sender) whether the body
  // text used the ranker's reason as-is or the deterministic
  // fallback. On 'fallback', the caller writes
  // fomo.alert.drafter_schema_failed (Q6 lock).
  readonly reason_source: 'rank' | 'fallback';
  // When reason_source='fallback', surfaces which schema violation
  // triggered the fallback. Null when reason_source='rank'.
  readonly reason_violation_kind: ReasonViolationKind | null;
  // Original length of rank.reason BEFORE any fallback substitution.
  // Surfaced to audit detail so operators can size-tune the schema
  // cap if too many fallbacks fire. Never the original text content
  // itself (privacy: no leakage path).
  readonly original_reason_length: number;
}

// Collapse interior whitespace and trim. Pure helper.
function collapse(raw: string): string {
  return raw.replace(/\s+/g, ' ').trim();
}

// v0.5.6 sentence-boundary truncation. NEVER appends an ellipsis;
// NEVER cuts mid-word. If the collapsed string fits, returns it as-is.
// Otherwise, finds the last sentence terminator (`.`, `!`, `?`)
// followed by whitespace or end-of-string within maxChars; failing
// that, the last word boundary within maxChars. The returned string
// is guaranteed to be ≤ maxChars and to end on a complete word.
//
// Founder Q4 lock 2026-06-05: NO arbitrary ellipsis, NO mid-sentence
// truncation. This is the load-bearing function for those two rules.
function truncateAtSentenceBoundary(raw: string, maxChars: number): string {
  const collapsed = collapse(raw);
  if (collapsed.length <= maxChars) return collapsed;

  // Look for a sentence-terminator within maxChars.
  const window = collapsed.slice(0, maxChars + 1);
  // Match the LAST occurrence of a sentence-ender followed by a space
  // or end-of-window. /g + manual scan finds the rightmost.
  const sentenceEnders = /[.!?](?=\s|$)/g;
  let lastSentenceEnd = -1;
  let match: RegExpExecArray | null;
  while ((match = sentenceEnders.exec(window)) !== null) {
    if (match.index < maxChars) lastSentenceEnd = match.index;
  }
  if (lastSentenceEnd >= 0) {
    return collapsed.slice(0, lastSentenceEnd + 1).trimEnd();
  }

  // No sentence-ender in range. Fall back to last word boundary
  // within maxChars. NEVER append ellipsis; NEVER cut mid-word.
  const truncated = collapsed.slice(0, maxChars);
  const lastSpace = truncated.lastIndexOf(' ');
  if (lastSpace > 0) return truncated.slice(0, lastSpace).trimEnd();
  // Pathological: single token longer than maxChars. Hard-clip at
  // word boundary unavailable; still NO ellipsis (founder Q4 lock).
  return truncated.trimEnd();
}

function senderLine(view: SlackEgressView): string {
  const raw =
    view.sender_name && view.sender_name.length > 0
      ? `${view.sender_name} <${view.sender_email_masked}>`
      : view.sender_email_masked;
  return truncateAtSentenceBoundary(raw, MAX_SENDER_LINE_CHARS);
}

// Classify rank.reason for the v0.5.6 schema. Returns the violation
// kind on failure, or null if reason is acceptable for use as-is.
function classifyReason(reason: string): ReasonViolationKind | null {
  if (collapse(reason).length === 0) return 'empty';
  if (reason.length > REASON_HARD_CAP_FOR_RENDER) return 'too_long';
  return null;
}

// Renders the founder text. PURE FUNCTION — no I/O, no clock, no random.
//
// v0.5.6 output shape (sentence-shaped, NO header, NO telemetry):
//
//   <sender>
//   <subject>
//   <reason or deterministic fallback>
//
// The output is bounded by FOUNDER_TEXT_HARD_MAX_CHARS (320) with a
// safety margin to FOUNDER_TEXT_ABSOLUTE_MAX_CHARS (340). Truncation
// happens at sentence boundaries (never mid-word, never ellipsis).
// If the assembled string exceeds HARD_MAX, we truncate the reason
// line (the longest variable-length slot) at a sentence boundary
// first, then the subject if still over, never the sender (the
// "who" is the load-bearing identifier).
export function renderFounderText(
  input: FounderTextTemplateInput
): FounderTextTemplateOutput {
  const violationKind = classifyReason(input.rank.reason);
  const reason_source: 'rank' | 'fallback' = violationKind === null ? 'rank' : 'fallback';
  const reasonText = violationKind === null ? input.rank.reason : REASON_FALLBACK_STRING;
  const original_reason_length = input.rank.reason.length;

  const sender = senderLine(input.view);
  const subject = truncateAtSentenceBoundary(input.view.subject, MAX_SUBJECT_LINE_CHARS);
  const reason = truncateAtSentenceBoundary(reasonText, REASON_HARD_CAP_FOR_RENDER);

  const parts = [sender, subject, reason].filter((s) => s.length > 0);
  let text = parts.join('\n');

  // If over HARD_MAX, tighten the reason line further (sentence-boundary).
  if (text.length > FOUNDER_TEXT_HARD_MAX_CHARS) {
    const fixed = sender.length + 1 + subject.length + 1; // newlines
    const reasonBudget = Math.max(0, FOUNDER_TEXT_HARD_MAX_CHARS - fixed);
    const tightenedReason = truncateAtSentenceBoundary(reason, reasonBudget);
    text = [sender, subject, tightenedReason].filter((s) => s.length > 0).join('\n');
  }
  // If STILL over HARD_MAX (extremely short reasonBudget produced no
  // text or subject too long), drop subject. Sender always survives.
  if (text.length > FOUNDER_TEXT_HARD_MAX_CHARS) {
    const fixed = sender.length + 1; // newline
    const reasonBudget = Math.max(0, FOUNDER_TEXT_HARD_MAX_CHARS - fixed);
    const tightenedReason = truncateAtSentenceBoundary(reason, reasonBudget);
    text = [sender, tightenedReason].filter((s) => s.length > 0).join('\n');
  }
  // Last resort: hard-clip to ABSOLUTE_MAX at a word boundary. NEVER
  // append ellipsis (founder Q4 lock).
  if (text.length > FOUNDER_TEXT_ABSOLUTE_MAX_CHARS) {
    text = truncateAtSentenceBoundary(text, FOUNDER_TEXT_ABSOLUTE_MAX_CHARS);
  }

  return Object.freeze({
    text,
    template_version: FOUNDER_TEXT_TEMPLATE_VERSION,
    reason_source,
    reason_violation_kind: violationKind,
    original_reason_length
  });
}
