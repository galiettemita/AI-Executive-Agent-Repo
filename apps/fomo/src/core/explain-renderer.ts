// Phase v0.7.0A — "Why?" Reply Intent + Explainability Surface renderer.
//
// PRODUCT PRINCIPLE (founder-locked 2026-06-11):
//   This is NOT metadata display. The user replied "why?" to a Brevio
//   alert; Brevio answers like a helpful assistant, not a database
//   schema. The shape MUST be natural prose, not "From X, '<subject>' —
//   I flagged this because ...".
//
//   Preferred shape (founder example):
//     "I flagged Mark's email about the Q3 board deck because he needs
//      your sign-off by tomorrow."
//
//   Schema reality: alerts + rank_results store NO plaintext sender or
//   subject (3D.1 privacy design — only sender_email_hash and message_id).
//   We therefore COMPOSE from rank.reason ALONE. The v0.5.7 HMR reason
//   is already a natural-prose sentence that typically embeds the
//   sender's display name and subject context (e.g. "Galiette needs you
//   to review the Q4 board deck tonight."). Prefixing it with
//   "I flagged it because " yields a single natural sentence in
//   Brevio's voice.
//
//   Avoid field-shaped output unconditionally — no labels, no quotes
//   around subjects, no "From <X>" headers.
//
// HARD CONTRACT (founder-locked v0.7.0A scope):
//   1. composeExplanationFromReason(reason) is deterministic. No LLM
//      call. Same input → same output.
//   2. Empty / whitespace-only reason → returns null. The caller renders
//      EMPTY_STATE_COPY instead.
//   3. Output length ≤ EXPLAIN_BODY_MAX_CHARS (320). Truncation is
//      word-boundary clean (no ellipsis — [[dont-oversell-renderer-tone]]
//      reminds us ellipsis reads as robotic truncation).
//   4. The frame "I flagged it because " is the ONLY transform applied.
//      We do not invent sender names, do not add metadata, do not append
//      explanatory tail text.
//   5. Audit detail for the new brevio.explain.served kind carries
//      hashAlertIdForAudit(alert_id) — sha256 sliced to 16 hex chars —
//      so the audit row never carries the raw alert_id and is still
//      correlatable. alert_id itself is already an opaque identifier,
//      but the founder spec named alert_id_hash explicitly.

import { createHash } from 'node:crypto';

/* ====================================================================== */
/* Constants — exported so callers + tests reference the SAME literals.    */
/* ====================================================================== */

/**
 * Template version stamped in the brevio.explain.served audit detail.
 * Bump this string ONLY when the composition shape changes materially —
 * NOT for cosmetic edits. v0.7.0A ships v0.1.0.
 */
export const EXPLAIN_TEMPLATE_VERSION = 'brevio-explain-v0.1.0';

/**
 * Empty-state copy used when:
 *   - no eligible alert is matched within the lookup window, OR
 *   - the matched alert's rank_result is missing / has empty reason, OR
 *   - composeExplanationFromReason returns null.
 * Founder-locked literal text.
 */
export const EMPTY_STATE_COPY =
  "I haven't sent you an alert I can explain yet.";

/**
 * Whole-body length cap for the SendBlue outbound. Matches the v0.5.6
 * iMessage hard cap so the explain body fits in a single send and
 * doesn't fight existing SendBlue path assumptions.
 */
export const EXPLAIN_BODY_MAX_CHARS = 320;

/**
 * Lookup window for "most recent eligible alert". Founder-locked at 24
 * hours: an alert older than this is treated as "no eligible alert"
 * (empty-state path). Caller compares against alert.created_at.
 */
export const EXPLAIN_LOOKUP_WINDOW_MS = 24 * 60 * 60 * 1000;

/**
 * The static source_surface value stamped into brevio.explain.served
 * audit detail. v0.7.0A only fires this audit from the SendBlue inbound
 * route; future surfaces would add new allowed values here.
 */
export const EXPLAIN_SOURCE_SURFACE = 'sendblue_inbound' as const;
export type ExplainSourceSurface = typeof EXPLAIN_SOURCE_SURFACE;

/* ====================================================================== */
/* Deterministic composition                                               */
/* ====================================================================== */

const EXPLAIN_FRAME_PREFIX = 'I flagged it because ';
const EXPLAIN_FRAME_SUFFIX = '.';
const EXPLAIN_FRAME_OVERHEAD =
  EXPLAIN_FRAME_PREFIX.length + EXPLAIN_FRAME_SUFFIX.length;

/**
 * Common-English words that read awkwardly mid-sentence with a leading
 * capital. When rank.reason starts with one of these, we lowercase the
 * first letter so the composed sentence "...because your CEO is asking..."
 * reads naturally. v0.5.7 HMR reasons frequently begin with these.
 *
 * Proper nouns (sender names like "Galiette", "Mark") are NOT in this
 * set, so their capitalization is preserved — "because Mark needs you
 * to..." reads correctly.
 */
const COMMON_PRONOUN_STARTERS: ReadonlySet<string> = new Set([
  'You', 'Your', 'The', 'A', 'An', 'This', 'That', 'It',
  'They', 'These', 'Those', 'My', 'Our', 'Their', 'There',
  'Here', 'Some', 'Several', 'Many', 'Most', 'Both'
]);

function stripTrailingTerminalPunctuation(s: string): string {
  return s.replace(/[.!?;:]+\s*$/u, '');
}

function adjustReasonCapitalization(reason: string): string {
  // Match the leading [A-Za-z]+ — stops at apostrophes / hyphens / punctuation
  // so "You're" → "You" and "Theresa's" → "Theresa". This lets us safely
  // detect "You're being asked..." as a pronoun-starter while preserving
  // "Theresa's request..." as a proper-noun starter.
  const firstWordMatch = reason.match(/^([A-Za-z]+)/);
  if (firstWordMatch === null) return reason;
  const firstWord = firstWordMatch[1] ?? '';
  if (firstWord.length === 0) return reason;
  if (!COMMON_PRONOUN_STARTERS.has(firstWord)) return reason;
  const firstChar = reason.charAt(0);
  if (firstChar !== firstChar.toUpperCase()) return reason;
  return firstChar.toLowerCase() + reason.slice(1);
}

/**
 * Word-boundary clean truncation. Slices at the last whitespace at or
 * before `maxLen`, with a fallback to a hard cut when no whitespace
 * exists in the prefix. No ellipsis appended.
 *
 * Pure helper; exported for tests.
 */
export function truncateAtWordBoundary(s: string, maxLen: number): string {
  if (s.length <= maxLen) return s;
  const sliced = s.slice(0, maxLen);
  const lastSpace = sliced.lastIndexOf(' ');
  // Require the last-space to be far enough in that we don't truncate
  // to a 2-character stub on a freak input. Empirically: at least half
  // the slice. When no good word boundary exists, hard-cut at maxLen.
  if (lastSpace > maxLen / 2) return sliced.slice(0, lastSpace);
  return sliced;
}

/**
 * Compose the "Why?" explanation body from a rank_results.reason value.
 *
 *   composeExplanationFromReason('Mark needs you to review the Q3 board deck tonight.')
 *     → 'I flagged it because Mark needs you to review the Q3 board deck tonight.'
 *   composeExplanationFromReason('Your CEO is asking for a response by 5pm.')
 *     → 'I flagged it because your CEO is asking for a response by 5pm.'
 *   composeExplanationFromReason('') → null
 *   composeExplanationFromReason('   ') → null
 *
 * Returns null when the reason is missing or empty — caller renders
 * EMPTY_STATE_COPY in that case.
 *
 * Pure function. No I/O.
 */
export function composeExplanationFromReason(reason: string | null | undefined): string | null {
  if (typeof reason !== 'string') return null;
  const trimmed = reason.trim();
  if (trimmed.length === 0) return null;

  const withoutTerminal = stripTrailingTerminalPunctuation(trimmed);
  if (withoutTerminal.length === 0) return null;

  const adjusted = adjustReasonCapitalization(withoutTerminal);

  // Budget the reason portion against the body cap.
  const reasonBudget = EXPLAIN_BODY_MAX_CHARS - EXPLAIN_FRAME_OVERHEAD;
  const reasonForBody =
    adjusted.length > reasonBudget
      ? truncateAtWordBoundary(adjusted, reasonBudget)
      : adjusted;

  return `${EXPLAIN_FRAME_PREFIX}${reasonForBody}${EXPLAIN_FRAME_SUFFIX}`;
}

/* ====================================================================== */
/* Audit helpers                                                           */
/* ====================================================================== */

/**
 * Hash an alert_id for the brevio.explain.served audit detail. The
 * alert_id is already an opaque random identifier (3D.1 alert table
 * design), but the founder's v0.7.0A spec named alert_id_hash explicitly
 * so we honor the contract literally — sha256 sliced to 16 hex chars,
 * unkeyed (no secret needed; the goal is "no raw alert_id in audit
 * detail", not authenticated hash).
 *
 * Returns null when alertId is null — empty_state audit rows carry
 * alert_id_hash=null deliberately.
 *
 * Pure function. No I/O beyond the standard library hash.
 */
export function hashAlertIdForAudit(alertId: string | null): string | null {
  if (alertId === null) return null;
  return createHash('sha256').update(alertId, 'utf8').digest('hex').slice(0, 16);
}
