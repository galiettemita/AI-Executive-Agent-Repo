// Deterministic safety/control pre-pass for the SendBlue reply parser.
// Phase 3F.1 + Phase v0.5.10 (Q3.C founder lock).
//
// Founder directives:
//   * 2026-05-26: "STOP / UNSUBSCRIBE / CANCEL / START-style compliance/
//     control commands must be handled deterministically before any LLM
//     call. Do not use OpenAI to decide whether STOP means stop."
//   * 2026-06-06 (Q3.C): "Any reply ≤ 3 words and not a deterministic
//     command / explicit feedback phrase → unclear. Absorb the canonical
//     short feedback phrases into the deterministic pre-pass so the LLM
//     classifier never sees them."
//
// This file is the FIRST pass the reply parser runs. If it returns a
// non-null result, the LLM classifier is NEVER called.
//
// Two match categories:
//   1. COMPLIANCE commands → intent ∈ {'stop', 'start'}. These trigger
//      the existing v0.5.5 STOP enforcement path. NEVER become a
//      preference feedback per founder lock — STOP/START stay separate
//      from preference learning.
//   2. SOFT EXPLICIT-FEEDBACK PHRASES (Q3.C v0.5.10) → intent ∈ the
//      ReplyIntent union (minus 'unclear'). These bypass the LLM
//      classifier but flow through the same downstream routing as the
//      classifier's output. The route handler treats them as "intent
//      with parser_confidence=1.0 and intent_source=
//      'reply_parser_deterministic'".
//
// Normalization (applied before matching, NOT to the persisted text):
//   * trim leading/trailing whitespace
//   * collapse internal whitespace to single spaces
//   * lowercase (case-insensitive match)
//   * strip trailing punctuation `!.?,` (e.g. "STOP!" → "stop")
//
// Match rule: the normalized string must be EXACTLY one of the listed
// forms. "STOP please" or "I want to STOP" do NOT match deterministically.
// Same for soft phrases — "ignore this sender" matches; "could you
// ignore senders like this one" falls through to the classifier.
//
// PRIVACY: this module operates on the normalized form. The raw reply
// text is NEVER persisted by this module. The caller (route handler)
// audits only the matched intent + source.

import type { ReplyIntent } from './validator.js';

// Phase v0.5.10 — DeterministicIntent extends from 'stop'|'start' to
// also include the explicit-feedback-phrase soft intents per Q3.C lock.
// 'unclear' is NEVER returned by the deterministic pre-pass; it's
// produced only by the orchestrator's safe-rules (low confidence
// classifier OR the ≤3-word fail-safe).
export type DeterministicIntent =
  | 'stop'
  | 'start'
  | Exclude<ReplyIntent, 'unclear'>;

export interface DeterministicMatch {
  readonly intent: DeterministicIntent;
  // Always 'deterministic' for results from this module.
  readonly source: 'deterministic';
}

// Frozen, exact-match word lists.
const STOP_FORMS: ReadonlySet<string> = new Set([
  'stop',
  'unsubscribe',
  'cancel',
  'end',
  'quit'
]);

const START_FORMS: ReadonlySet<string> = new Set([
  'start',
  'unstop',
  'resume'
]);

// Phase v0.5.10 (Q3.C) — explicit-feedback-phrase allowlist. Each
// normalized phrase maps to a specific ReplyIntent. The LLM classifier
// is NEVER consulted for these.
//
// Founder-locked phrases (project_v05-10-scope §Q3 explicit-feedback-
// phrase allowlist). Adding to this list requires a separate phase +
// founder review (the LLM classifier is the safety net for natural
// variations; the allowlist captures only canonical short forms).
const SOFT_ALLOWLIST: ReadonlyMap<string, Exclude<ReplyIntent, 'unclear'>> = new Map([
  // false_positive — negative correction phrases. The downstream
  // routing module maps this intent to verb='corrected',
  // dimension='ranker_label', previous_label='important',
  // corrected_label='not_important' (v0.5.10 vocabulary; founder
  // correction from the v0.5.9 'positive'/'negative' shape).
  ['not important', 'false_positive'],
  ["wasn't important", 'false_positive'],
  ['this wasn’t important', 'false_positive'],
  ['this wasnt important', 'false_positive'],
  ['this wasn\'t important', 'false_positive'],
  ['not worth it', 'false_positive'],
  ['not relevant', 'false_positive'],
  // this_mattered — positive confirmation (founder major correction:
  // NOT a negative correction; this is a positive truth-confirmation).
  // Downstream mapping: verb='approved', dimension='importance',
  // value='confirmed_important'.
  ['this mattered', 'this_mattered'],
  ['really mattered', 'this_mattered'],
  ['very important', 'this_mattered'],
  ['that mattered', 'this_mattered'],
  ['good catch', 'this_mattered'],
  ['important catch', 'this_mattered'],
  // more_like_this — positive forward-looking signal. Downstream:
  // verb='approved', dimension='pattern', value='more_like_this'.
  ['more like this', 'more_like_this'],
  ['show me more', 'more_like_this'],
  ['more of these', 'more_like_this'],
  ['keep these coming', 'more_like_this'],
  // ignore_sender — the Q5.B trigger that flows into the v0.5.9
  // consumer pipe → sender_feedback_ignored memory_signal upsert +
  // brevio.feedback.applied audit. THIS is the ONLY soft-allowlist
  // intent that triggers a memory_signal write in v0.5.10.
  ['ignore this sender', 'ignore_sender'],
  ['mute sender', 'ignore_sender'],
  ['quiet sender', 'ignore_sender'],
  ['quiet this sender', 'ignore_sender'],
  ['mute this sender', 'ignore_sender'],
  ['silence sender', 'ignore_sender'],
  // ignore — per-alert ignore (not per-sender). Captured as a
  // feedback_event with dimension='alert'; NO memory_signal write per
  // Q4.A.
  ['ignore this', 'ignore'],
  ['skip this', 'ignore'],
  ['dismiss', 'ignore'],
  ['not now', 'ignore'],
  // why — user wants an explanation. Captured as feedback_event with
  // verb='asked_why'. The HMR-rendered "here's why I flagged it" reply
  // is a future phase; v0.5.10 captures the signal only.
  ['why', 'why'],
  ['why this', 'why'],
  ['why this one', 'why'],
  ['how come', 'why'],
  ['why did you send this', 'why']
]);

// Strip trailing punctuation we want to ignore. We do NOT strip internal
// punctuation — that's what differentiates "STOP" from "I want to STOP,
// please" (the second is not a deterministic match).
function normalize(rawText: string): string {
  return rawText.trim().replace(/\s+/g, ' ').toLowerCase().replace(/[!.?,]+$/u, '');
}

// Exported so the orchestrator's ≤3-word safe rule can word-count the
// SAME normalized form the matcher considered. (Tokenization mismatch
// between matcher and safe rule would create false-negative gaps.)
export function countNormalizedWordTokens(rawText: string): number {
  if (typeof rawText !== 'string') return 0;
  const normalized = normalize(rawText);
  if (normalized.length === 0) return 0;
  return normalized.split(' ').length;
}

export function parseReplyDeterministic(rawText: string): DeterministicMatch | null {
  if (typeof rawText !== 'string') return null;
  const normalized = normalize(rawText);
  if (normalized.length === 0) return null;

  // COMPLIANCE first — STOP/START win over any soft phrase that
  // might accidentally collide (no collisions today; defensive ordering).
  if (STOP_FORMS.has(normalized)) {
    return Object.freeze({ intent: 'stop' as const, source: 'deterministic' as const });
  }
  if (START_FORMS.has(normalized)) {
    return Object.freeze({ intent: 'start' as const, source: 'deterministic' as const });
  }

  // SOFT allowlist (Phase v0.5.10 Q3.C).
  const softIntent = SOFT_ALLOWLIST.get(normalized);
  if (softIntent !== undefined) {
    return Object.freeze({ intent: softIntent, source: 'deterministic' as const });
  }

  return null;
}

// Exported for unit testing. The matched-phrase list (without intent
// mapping) lets tests verify the v0.5.10 founder-locked allowlist size
// without exposing the implementation Map.
export const SOFT_ALLOWLIST_PHRASES: readonly string[] = Object.freeze([...SOFT_ALLOWLIST.keys()]);
