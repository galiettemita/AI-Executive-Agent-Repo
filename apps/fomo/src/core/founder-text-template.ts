// Founder Text Template — Phase 3E.1.
//
// The deterministic, no-LLM text the outbound sender worker hands to
// SendBlue. Founder directive 2026-05-25:
//
//   "Do not add a new LLM or freeform 'assistant voice generation' for
//    3E.1. Use deterministic/template-based copy from safe fields only."
//
// Allowed inputs:
//   * sender display + masked email (egress-redacted via SlackEgressView)
//   * subject line
//   * body snippet (already egress-truncated)
//   * ranker label / score / reason
//
// Forbidden inputs (never touch this function):
//   * raw body_plain / body_html
//   * raw headers
//   * attachment filenames
//   * hallucinated explanations
//   * any model-generated text other than rank.reason
//
// The output is a single bounded string under ~280 characters
// (iMessage-friendly; one SMS segment-equivalent for SendBlue parity).
// We deliberately do NOT include action verbs ("Should I reply?"); v0.1
// just texts the founder the fact. Phase 3F adds reply handling.
//
// Template version is stamped into audit detail so future template
// changes are traceable.

import { type SlackEgressView } from './egress-policy.js';
import { type RankLabel } from '../memory/rank-results.js';

// Bump when the rendered shape changes. Audit detail captures this so
// operators can diff founder texts across versions during 3E.2 smoke.
export const FOUNDER_TEXT_TEMPLATE_VERSION = 'founder-text-v0.1.0';

// Total bounded length for the rendered string. Below the SMS 7-bit
// 160-char single-segment threshold is the safest target, but iMessage
// supports far more; we use 280 (Twitter-equivalent) so the founder
// gets enough signal without leaking long body excerpts.
const MAX_TOTAL_CHARS = 280;
// Sub-bounds protect against any one field dominating the message.
const MAX_SUBJECT_CHARS = 80;
const MAX_SNIPPET_CHARS = 120;
// Sender display is already short; cap defensively.
const MAX_SENDER_CHARS = 60;

export interface FounderTextTemplateInput {
  readonly view: SlackEgressView;
  readonly rank: {
    readonly label: RankLabel;
    readonly score: number;
  };
}

export interface FounderTextTemplateOutput {
  readonly text: string;
  readonly template_version: string;
}

// Collapse + truncate a field to maxChars, appending an ellipsis on
// truncation. Centralized so every field in the template gets the same
// safety treatment.
function clip(raw: string, maxChars: number): string {
  const collapsed = raw.replace(/\s+/g, ' ').trim();
  if (collapsed.length <= maxChars) return collapsed;
  return collapsed.slice(0, maxChars).trimEnd() + '…';
}

function senderLine(view: SlackEgressView): string {
  if (view.sender_name && view.sender_name.length > 0) {
    return clip(`${view.sender_name} <${view.sender_email_masked}>`, MAX_SENDER_CHARS);
  }
  return clip(view.sender_email_masked, MAX_SENDER_CHARS);
}

// Renders the founder text. PURE FUNCTION — no I/O, no clock, no random.
//
// Output shape (each segment separated by newline):
//
//   FOMO · <label uppercase> (<score>)
//   <sender>
//   <subject>
//   <snippet>
//
// Total length is bounded; if the assembled string exceeds MAX_TOTAL_CHARS
// we drop the snippet first, then the subject, never the label/sender
// line (the founder must always know WHO it's from and that this is a
// FOMO alert, even if every other field is empty).
export function renderFounderText(
  input: FounderTextTemplateInput
): FounderTextTemplateOutput {
  const labelText = input.rank.label === 'important' ? 'IMPORTANT' : 'NOT_IMPORTANT';
  const score = Number.isFinite(input.rank.score)
    ? input.rank.score.toFixed(2)
    : '?';

  const header = `FOMO · ${labelText} (${score})`;
  const sender = senderLine(input.view);
  const subject = clip(input.view.subject, MAX_SUBJECT_CHARS);
  const snippet = clip(input.view.body_snippet, MAX_SNIPPET_CHARS);

  // Assemble in fallback order. The header and sender are mandatory; the
  // subject and snippet drop if we exceed the total cap.
  const candidate = [
    header,
    sender,
    subject.length > 0 ? subject : null,
    snippet.length > 0 ? snippet : null
  ]
    .filter((s): s is string => s !== null)
    .join('\n');

  let text: string;
  if (candidate.length <= MAX_TOTAL_CHARS) {
    text = candidate;
  } else {
    // Try without snippet first.
    const noSnippet = [header, sender, subject].filter((s) => s.length > 0).join('\n');
    if (noSnippet.length <= MAX_TOTAL_CHARS) {
      text = noSnippet;
    } else {
      // Last resort: header + sender only. Both must fit; if even this
      // overflows (impossibly large sender_name), hard-clip.
      const stripped = `${header}\n${sender}`;
      text = stripped.length <= MAX_TOTAL_CHARS
        ? stripped
        : stripped.slice(0, MAX_TOTAL_CHARS - 1) + '…';
    }
  }

  return Object.freeze({
    text,
    template_version: FOUNDER_TEXT_TEMPLATE_VERSION
  });
}
