// Deterministic safety/control pre-pass for the SendBlue reply parser.
// Phase 3F.1. Founder directive 2026-05-26:
//
//   "STOP / UNSUBSCRIBE / CANCEL / START-style compliance/control
//    commands must be handled deterministically before any LLM call.
//    Do not use OpenAI to decide whether STOP means stop."
//
// This file is the FIRST pass the reply parser runs. If it returns a
// non-null result, the LLM classifier is NEVER called. The match must
// be exact-shape-after-normalization — no fuzzy matching, no LLM
// inference, no synonym expansion. The set of recognized commands is
// frozen and small. Adding to it requires a separate phase + founder
// review.
//
// Normalization (applied before matching, NOT to the persisted text):
//   * trim leading/trailing whitespace
//   * collapse internal whitespace to single spaces
//   * lowercase (case-insensitive match)
//   * strip trailing punctuation `!.?,` (e.g. "STOP!" → "stop")
//
// Match rule: the normalized string must be EXACTLY one of the
// listed forms. "STOP please" or "I want to STOP" do NOT match
// deterministically — they fall through to the classifier (which may
// or may not recognize them; the classifier's "ignore" / "false_positive"
// intents are not safety-critical, so the LLM gate is acceptable
// for them). Only the bare command stops here.
//
// This is intentionally conservative: false-negative is safer than
// false-positive for STOP-style commands. A user who types "STOP"
// gets compliance enforcement; a user who types "Can you stop
// sending these?" gets the classifier path (and likely lands on
// 'ignore' or 'false_positive', which the outbound-sender treats as
// a per-alert preference signal, not as a global compliance command).
//
// Privacy: the matcher operates on the normalized form. The raw
// reply text is NOT persisted by this module; the caller (route
// handler) audits only the matched intent + the source='deterministic'
// flag. The reply text itself never leaves the route handler except
// through the egress-redacted classifier path.

export type DeterministicIntent = 'stop' | 'start';

export interface DeterministicMatch {
  readonly intent: DeterministicIntent;
  // Always 'deterministic' for results from this module. Provided so
  // downstream code can branch on `result.source === 'deterministic'`
  // without comparing strings against the intent set.
  readonly source: 'deterministic';
}

// Frozen, exact-match word lists. Compliance commands traditionally
// include these literals; the founder directive enumerates them
// explicitly. Order is not significant.
const STOP_FORMS: ReadonlySet<string> = new Set([
  'stop',
  'unsubscribe',
  'cancel',
  // Common compliance variants that SMS-style services typically
  // honor. Kept narrow per the founder directive.
  'end',
  'quit'
]);

const START_FORMS: ReadonlySet<string> = new Set([
  'start',
  'unstop',
  'resume'
]);

// Strip trailing punctuation we want to ignore at the end of a
// compliance command. We do NOT strip internal punctuation — that's
// what differentiates "STOP" from "I want to STOP, please" (the
// second is not a deterministic match).
function normalize(rawText: string): string {
  return rawText.trim().replace(/\s+/g, ' ').toLowerCase().replace(/[!.?,]+$/u, '');
}

export function parseReplyDeterministic(rawText: string): DeterministicMatch | null {
  if (typeof rawText !== 'string') return null;
  const normalized = normalize(rawText);
  if (normalized.length === 0) return null;
  if (STOP_FORMS.has(normalized)) {
    return Object.freeze({ intent: 'stop' as const, source: 'deterministic' as const });
  }
  if (START_FORMS.has(normalized)) {
    return Object.freeze({ intent: 'start' as const, source: 'deterministic' as const });
  }
  return null;
}
