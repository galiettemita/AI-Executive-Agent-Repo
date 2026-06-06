// Human Message Renderer — Phase v0.5.7.
//
// FIRST surface of the founder-locked Human Message Renderer product
// principle (memory feedback_brevio-human-message-renderer-principle).
// Closes the gap from v0.5.6's "less robotic" three-field shape to the
// founder example bar:
//
//   "Galiette emailed you about the Q3 board deck. It looks
//    time-sensitive — she needs sign-off by tomorrow."
//
// (Subject naturalization in the example is `the Q3 board deck` —
// noun rewriting. v0.5.7 locks Q3.B = strip-prefixes-only, NO noun
// rewriting. So the actual rendered output is closer to:
//
//   `Galiette emailed you about "Q3 board deck final draft". <reason>`
//
// — the quoted form. The next phase can revisit noun naturalization.)
//
// 3E.1 PRESERVED (founder directive 2026-05-25):
//   This module imports NO LLM / OpenAI / Anthropic client. Body
//   composition is fully deterministic. The only model-generated text
//   in the final output is `rank.reason` (the existing 3E.1 carve-out,
//   unchanged — see ranker-v0.2.0 prompt).
//   The human-message-renderer test suite contains a load-bearing
//   assertion that no LLM-shaped import exists in this file.
//
// Locked scope (memory project_v05-7-scope):
//   * Q1.A — two-sentence canonical:
//       `<Sender> emailed you about "<subject_phrase>". <Why_clause>.`
//     Single-sentence fallback when subject is empty:
//       `<Sender> emailed you. <Why_clause>.`
//   * Modified Q2.B — sender chain. See sender-resolution.ts.
//   * Q3.B — strip [bracketed] / Re:/Fwd: prefixes. NO noun rewriting.
//   * Q4.A — ranker prompt → ranker-v0.2.0 (2nd-person). Renderer uses
//     rank.reason verbatim and reads PROMPT_VERSION from input to stamp
//     reason_voice in audit detail.
//   * Q5.A — locked degradation matrix; audit-visible per fallback.
//   * Q6.A (restraint) — `email_alert` surface only. No multi-surface
//     framework. The `surface` discriminator exists for forwards-compat.

import {
  resolveSender,
  type SenderResolutionPath
} from './sender-resolution.js';

// Re-export so the legacy founder-text-template wrapper + outbound-
// sender can import the path enum without reaching past this module.
export type { SenderResolutionPath } from './sender-resolution.js';
import { type HumanMessageEgressView } from './egress-policy.js';
import { type RankLabel } from '../memory/rank-results.js';

/* -------------------------------------------------------------------- */
/* Versioning + length policy                                           */
/* -------------------------------------------------------------------- */

// Phase v0.5.7 bumps the renderer's template version to surface the
// HMR rename in audit. Format is intentionally `human-message-v0.3.0`
// (not `founder-text-v0.3.0`) so operators can grep `template_version`
// from old runs and immediately see when HMR took over.
export const HUMAN_MESSAGE_TEMPLATE_VERSION = 'human-message-v0.3.0';

// Carry-forward from v0.5.6 (founder Q4 lock 2026-06-05):
//   * Target: 220–280 chars (informational floor; not enforced)
//   * Hard max: 320 chars (truncate reason at sentence boundary)
//   * Absolute max: 340 chars (hard-clip at word boundary)
// The 220 target floor is intentionally NOT enforced here — that's the
// "short-body length policy" candidate documented in
// memory project_v05-6-pass as a separate future gate.
export const HUMAN_MESSAGE_TARGET_MIN_CHARS = 220;
export const HUMAN_MESSAGE_TARGET_MAX_CHARS = 280;
export const HUMAN_MESSAGE_HARD_MAX_CHARS = 320;
export const HUMAN_MESSAGE_ABSOLUTE_MAX_CHARS = 340;

// Reason field hard cap (matches v0.5.6 REASON_HARD_CAP_FOR_RENDER and
// the ranker validator's reason cap). If rank.reason exceeds this, the
// Q5.A fallback path fires (reason_voice='fallback').
export const REASON_HARD_CAP_FOR_RENDER = 180;

// Deterministic fallback substituted when rank.reason fails schema.
// Carry-forward from v0.5.6: never LLM-generated, never uses email
// content, length within budget by construction.
export const REASON_FALLBACK_STRING = 'Marked important by Brevio.';

// Ranker prompt version that produces 2nd-person, action-oriented
// reasons (Q4.A lock). Any reason produced by a prior prompt version
// (e.g. ranker-v0.1.0) is rendered verbatim but stamped
// reason_voice='legacy_3p' for transitional observability.
//
// Keep this string in sync with apps/fomo/src/ranker/prompt.ts
// `PROMPT_VERSION`. If the prompt version bumps in a future phase, this
// constant likely needs to bump too — surface that decision in the
// next phase's 6Q gate, do not silent-edit.
export const RANKER_V2_PROMPT_VERSION = 'ranker-v0.2.0';

/* -------------------------------------------------------------------- */
/* Q5.A audit-field enums                                               */
/* -------------------------------------------------------------------- */

export type SubjectStripApplied =
  | 'none'
  | 'bracket_prefix'
  | 're_fwd'
  | 'multiple'
  | 'subject_empty';

export type ReasonVoice = '2p_action' | 'legacy_3p' | 'fallback';

export type TemplateShape =
  | 'two_sentence'
  | 'single_sentence_no_subject'
  | 'fallback_string';

export type ReasonViolationKind = 'empty' | 'too_long';

/* -------------------------------------------------------------------- */
/* Input / output types                                                 */
/* -------------------------------------------------------------------- */

export type HumanMessageSurface = 'email_alert';

export interface HumanMessageInput {
  // Q6.A restraint — only 'email_alert' implemented this phase. The
  // discriminator exists so future surfaces (calendar, drafts, tasks)
  // can be added as new variants without refactoring this module.
  readonly surface: HumanMessageSurface;
  readonly view: HumanMessageEgressView;
  readonly rank: {
    readonly label: RankLabel;
    readonly score: number;
    readonly reason: string;
  };
  // Ranker PROMPT_VERSION at the time `rank.reason` was produced.
  // Read from the rank_results / cost_records row in production; pass
  // the current ranker version in offline fixtures. Determines
  // reason_voice ('2p_action' vs 'legacy_3p') in the output.
  readonly prompt_version: string;
}

export interface HumanMessageOutput {
  readonly text: string;
  readonly template_version: string;

  // Q6.A audit-field outputs — structural enums only, NEVER user content.
  // Outbound-sender writes these into fomo.send.attempted detail.
  readonly sender_resolution_path: SenderResolutionPath;
  readonly subject_strip_applied: SubjectStripApplied;
  readonly reason_voice: ReasonVoice;
  readonly template_shape: TemplateShape;

  // True when ANY Q5.A degradation rule triggered (sender→generic,
  // subject empty, reason fallback, or transitional legacy_3p voice).
  // Outbound-sender fires `fomo.alert.hmr_degradation_applied` when
  // this is true. Best-effort audit, no retry.
  readonly degradation_applied: boolean;

  // Original length of rank.reason BEFORE any fallback substitution.
  // Surfaced to audit detail so operators can size-tune the schema cap
  // if too many fallbacks fire. Never the original text content itself
  // (privacy: no leakage path).
  readonly original_reason_length: number;
  readonly reason_violation_kind: ReasonViolationKind | null;
}

/* -------------------------------------------------------------------- */
/* Pure helpers                                                         */
/* -------------------------------------------------------------------- */

// Collapse interior whitespace + trim. Same as v0.5.6 helper.
function collapse(raw: string): string {
  return raw.replace(/\s+/g, ' ').trim();
}

// Q3.B subject strip — iteratively strip leading [bracketed] / Re: /
// Fwd: prefixes until no match. Returns the stripped subject and the
// `subject_strip_applied` enum value:
//   * 'none' — subject unchanged
//   * 'bracket_prefix' — only bracket prefix(es) stripped
//   * 're_fwd' — only Re:/Fwd:/FW: stripped
//   * 'multiple' — both stripped (any combination)
//   * 'subject_empty' — subject was empty before OR became empty after
export function stripSubject(
  raw: string | undefined
): { stripped: string; applied: SubjectStripApplied } {
  // Collapse interior whitespace + trim so "foo    bar" → "foo bar".
  // v0.5.6 carry-forward behavior.
  let s = collapse(raw ?? '');
  if (!s) return { stripped: '', applied: 'subject_empty' };
  let bracketApplied = false;
  let reFwdApplied = false;
  let changed = true;
  while (changed) {
    changed = false;
    const beforeBracket = s;
    s = s.replace(/^\[[^\]]*\]\s*/, '');
    if (s !== beforeBracket) {
      bracketApplied = true;
      changed = true;
    }
    const beforeReFwd = s;
    s = s.replace(/^(?:re|fwd|fw):\s*/i, '');
    if (s !== beforeReFwd) {
      reFwdApplied = true;
      changed = true;
    }
  }
  s = s.trim();
  if (!s) return { stripped: '', applied: 'subject_empty' };
  if (bracketApplied && reFwdApplied) return { stripped: s, applied: 'multiple' };
  if (bracketApplied) return { stripped: s, applied: 'bracket_prefix' };
  if (reFwdApplied) return { stripped: s, applied: 're_fwd' };
  return { stripped: s, applied: 'none' };
}

// Classify rank.reason for the Q5.A schema. Returns the violation kind
// on failure, or null if reason is acceptable for use as-is.
function classifyReason(reason: string): ReasonViolationKind | null {
  if (collapse(reason).length === 0) return 'empty';
  if (reason.length > REASON_HARD_CAP_FOR_RENDER) return 'too_long';
  return null;
}

// Ensure a sentence ends with `.` `!` or `?`. Does NOT append `…`. Used
// for the why-clause so the two-sentence template doesn't dangle when
// rank.reason was produced without a terminator.
function ensureSentenceTerminator(s: string): string {
  const trimmed = collapse(s);
  if (!trimmed) return '';
  return /[.!?]$/.test(trimmed) ? trimmed : trimmed + '.';
}

// Truncate at the last sentence terminator within `maxChars`. If no
// terminator is in range, falls back to the last word boundary. NEVER
// appends `…` (founder Q5 carry-forward from v0.5.6); NEVER cuts mid-
// word. Returned string is ≤ maxChars and ends on a complete sentence
// or word.
function truncateAtSentenceBoundary(raw: string, maxChars: number): string {
  const collapsed = collapse(raw);
  if (collapsed.length <= maxChars) return collapsed;
  const window = collapsed.slice(0, maxChars + 1);
  const sentenceEnders = /[.!?](?=\s|$)/g;
  let lastSentenceEnd = -1;
  let match: RegExpExecArray | null;
  while ((match = sentenceEnders.exec(window)) !== null) {
    if (match.index < maxChars) lastSentenceEnd = match.index;
  }
  if (lastSentenceEnd >= 0) {
    return collapsed.slice(0, lastSentenceEnd + 1).trimEnd();
  }
  const truncated = collapsed.slice(0, maxChars);
  const lastSpace = truncated.lastIndexOf(' ');
  if (lastSpace > 0) return truncated.slice(0, lastSpace).trimEnd();
  return truncated.trimEnd();
}

/* -------------------------------------------------------------------- */
/* Composition (deterministic — no LLM, no I/O, no clock, no random)    */
/* -------------------------------------------------------------------- */

interface ReasonResolution {
  readonly text: string;
  readonly voice: ReasonVoice;
  readonly violation_kind: ReasonViolationKind | null;
}

function resolveReason(
  rawReason: string,
  promptVersion: string
): ReasonResolution {
  const violation = classifyReason(rawReason);
  if (violation !== null) {
    return Object.freeze({
      text: REASON_FALLBACK_STRING,
      voice: 'fallback',
      violation_kind: violation
    });
  }
  // Reason passes schema. Stamp voice based on the ranker prompt that
  // produced it. The renderer does NOT rewrite reason text per Q4.A
  // (ranker prompt does the voice work).
  const voice: ReasonVoice =
    promptVersion === RANKER_V2_PROMPT_VERSION ? '2p_action' : 'legacy_3p';
  return Object.freeze({ text: rawReason, voice, violation_kind: null });
}

// Composes the two-sentence canonical (Q1.A) or single-sentence-no-subject
// fallback. Does not apply length budget — that's applyLengthBudget below.
function composeSentence(
  senderDisplay: string,
  strippedSubject: string,
  reasonText: string,
  isSubjectEmpty: boolean
): { text: string; shape: 'two_sentence' | 'single_sentence_no_subject' } {
  const why = ensureSentenceTerminator(reasonText);
  if (isSubjectEmpty) {
    return {
      text: `${senderDisplay} emailed you. ${why}`,
      shape: 'single_sentence_no_subject'
    };
  }
  return {
    text: `${senderDisplay} emailed you about "${strippedSubject}". ${why}`,
    shape: 'two_sentence'
  };
}

// Applies HARD_MAX / ABSOLUTE_MAX budgets. If text is over HARD_MAX,
// shrinks the reason line at a sentence boundary first; if still over,
// drops the subject (collapsing two-sentence → single-sentence-no-subject).
// Last resort: hard-clip the WHOLE text at ABSOLUTE_MAX (still sentence-
// boundary, NEVER ellipsis).
function applyLengthBudget(
  initialText: string,
  senderDisplay: string,
  strippedSubject: string,
  reasonText: string,
  initialShape: 'two_sentence' | 'single_sentence_no_subject'
): { text: string; shape: 'two_sentence' | 'single_sentence_no_subject' } {
  if (initialText.length <= HUMAN_MESSAGE_HARD_MAX_CHARS) {
    return { text: initialText, shape: initialShape };
  }

  // Step 1: tighten the reason at a sentence boundary while keeping the
  // current shape.
  const senderPart =
    initialShape === 'two_sentence'
      ? `${senderDisplay} emailed you about "${strippedSubject}". `
      : `${senderDisplay} emailed you. `;
  const reasonBudget = Math.max(0, HUMAN_MESSAGE_HARD_MAX_CHARS - senderPart.length);
  const tightenedReason = truncateAtSentenceBoundary(reasonText, reasonBudget);
  const tightenedWhy = ensureSentenceTerminator(tightenedReason);
  let text = `${senderPart}${tightenedWhy}`;
  let shape = initialShape;

  if (text.length <= HUMAN_MESSAGE_HARD_MAX_CHARS) {
    return { text, shape };
  }

  // Step 2: if still over (subject was very long), collapse to
  // single-sentence-no-subject and retry tighten.
  if (initialShape === 'two_sentence') {
    const senderPartShort = `${senderDisplay} emailed you. `;
    const reasonBudgetShort = Math.max(0, HUMAN_MESSAGE_HARD_MAX_CHARS - senderPartShort.length);
    const tightenedReasonShort = truncateAtSentenceBoundary(reasonText, reasonBudgetShort);
    const tightenedWhyShort = ensureSentenceTerminator(tightenedReasonShort);
    text = `${senderPartShort}${tightenedWhyShort}`;
    shape = 'single_sentence_no_subject';
  }

  // Step 3: hard-clip at ABSOLUTE_MAX (still sentence-boundary).
  if (text.length > HUMAN_MESSAGE_ABSOLUTE_MAX_CHARS) {
    text = truncateAtSentenceBoundary(text, HUMAN_MESSAGE_ABSOLUTE_MAX_CHARS);
  }

  return { text, shape };
}

/* -------------------------------------------------------------------- */
/* Public entry point                                                   */
/* -------------------------------------------------------------------- */

// Render the user-facing iMessage text + audit-field outputs.
//
// PURE: no I/O, no clock, no random, no LLM. Returning a frozen object.
//
// Q5.A degradation paths and their audit signals:
//   1. sender_resolution_path='generic' (Q2.B chain fell through to
//      "Someone") → degradation_applied=true
//   2. subject_strip_applied='subject_empty' (subject was empty or only
//      contained strippable prefixes) → degradation_applied=true,
//      template_shape='single_sentence_no_subject'
//   3. reason_voice='fallback' (rank.reason failed schema; deterministic
//      fallback string substituted) → degradation_applied=true,
//      template_shape='fallback_string'
//   4. reason_voice='legacy_3p' (rank produced by ranker-v0.1.0 or
//      earlier; transitional) → degradation_applied=true
//
// Outbound-sender writes `fomo.alert.hmr_degradation_applied` when
// degradation_applied=true (best-effort, no retry). The companion
// v0.5.6 audit `fomo.alert.drafter_schema_failed` still fires
// separately when reason_voice='fallback' (reason violation).
export function renderHumanMessage(input: HumanMessageInput): HumanMessageOutput {
  const { view, rank, prompt_version } = input;

  const senderRes = resolveSender({
    sender_name: view.sender_name,
    sender_email: view.sender_email
  });
  const subjectRes = stripSubject(view.subject);
  const reasonRes = resolveReason(rank.reason, prompt_version);
  const isSubjectEmpty = subjectRes.applied === 'subject_empty';

  let template_shape: TemplateShape;
  let text: string;

  if (reasonRes.voice === 'fallback') {
    // Q5.A fallback path: the body uses the fallback STRING as the
    // why-clause. We still wrap it in the HMR shell so the message
    // doesn't read as "Marked important by Brevio." alone.
    const composition = composeSentence(
      senderRes.display,
      subjectRes.stripped,
      reasonRes.text,
      isSubjectEmpty
    );
    const budgeted = applyLengthBudget(
      composition.text,
      senderRes.display,
      subjectRes.stripped,
      reasonRes.text,
      composition.shape
    );
    text = budgeted.text;
    // Q5.A: when the fallback string fired, surface 'fallback_string'
    // as the template_shape (not the underlying two_sentence shape) so
    // operators can grep audit for "fallback rate."
    template_shape = 'fallback_string';
  } else {
    const composition = composeSentence(
      senderRes.display,
      subjectRes.stripped,
      reasonRes.text,
      isSubjectEmpty
    );
    const budgeted = applyLengthBudget(
      composition.text,
      senderRes.display,
      subjectRes.stripped,
      reasonRes.text,
      composition.shape
    );
    text = budgeted.text;
    template_shape = budgeted.shape;
  }

  const degradation_applied =
    senderRes.path === 'generic' ||
    isSubjectEmpty ||
    reasonRes.voice === 'fallback' ||
    reasonRes.voice === 'legacy_3p';

  return Object.freeze({
    text,
    template_version: HUMAN_MESSAGE_TEMPLATE_VERSION,
    sender_resolution_path: senderRes.path,
    subject_strip_applied: subjectRes.applied,
    reason_voice: reasonRes.voice,
    template_shape,
    degradation_applied,
    original_reason_length: rank.reason.length,
    reason_violation_kind: reasonRes.violation_kind
  });
}
