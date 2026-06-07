// Phase v0.5.14 — HMR Feedback Acknowledgment renderer.
//
// Pure deterministic renderer. ONE testable place for the 4 ack bodies
// the user receives over iMessage when their feedback intent lands. No
// LLM, no PII inputs (only the parser intent enum), no metadata leakage.
// The renderer is consumed by the feedback-routing module, which decides
// when to send and writes the audit; this file owns the text + version
// only.
//
// Bound contract (founder-locked 2026-06-07 — see [[brevio-voice-rules]]
// + [[brevio-human-message-renderer-principle]] + [[dont-oversell-
// renderer-tone]]):
//   * calm-assistant register
//   * 1st-person Brevio voice ("I'll …")
//   * no sender_email / subject / body / snippet reference
//   * no fake-friendly tone ("amazing!", "perfect!", "awesome!")
//   * 6–12 words, single sentence
//   * deterministic: identical input → identical output
//
// Acknowledgment-eligible intents (founder-locked, MUST match the v0.5.10
// reply-parser intent vocabulary):
//   * ignore_sender   — negative; user wants this sender quieted
//   * this_mattered   — positive; user confirms an alert was worth catching
//   * more_like_this  — positive; user wants similar catches surfaced
//   * false_positive  — negative correction; user wants Brevio more careful
//
// Intents NOT acknowledged here (and the reason):
//   * stop / start / unclear — these never reach routeReplyFeedback (see
//     v0.5.5 STOP compliance path + v0.5.10 unclear short-circuit). The
//     v0.5.5 STOP_CONFIRMATION_BODY is a separate deterministic ack on
//     the compliance side.
//   * snooze / ignore / why — declared as v0.5.10 intents but NOT in the
//     v0.5.14 ackable set. The founder-locked v0.5.14 scope ships 4
//     intents only; future tier classifications decide additions.

export const FEEDBACK_ACK_TEMPLATE_VERSION = 'feedback-ack-v0.1.0' as const;

/**
 * The 4 v0.5.14 ackable intents. Subset of v0.5.10 ReplyIntent (which
 * also includes stop/start/snooze/ignore/why/unclear that this surface
 * intentionally does NOT acknowledge).
 */
export type AckableFeedbackIntent =
  | 'ignore_sender'
  | 'this_mattered'
  | 'more_like_this'
  | 'false_positive';

// Locked text table — single source of truth.
const ACK_BODIES: Readonly<Record<AckableFeedbackIntent, string>> = Object.freeze({
  ignore_sender: "Got it — I'll quiet that sender.",
  this_mattered: "Thanks — I'll remember this was worth catching.",
  more_like_this: "Got it — I'll catch more like that.",
  false_positive: "Got it — I'll be more careful with emails like that."
});

/**
 * Type guard for "is this intent acknowledged by v0.5.14?". The
 * feedback-routing call site uses this to decide whether to attempt
 * a render + send. Returns false for stop/start/snooze/ignore/why/
 * unclear and any future-added intent strings.
 */
export function isAckableFeedbackIntent(
  intent: string
): intent is AckableFeedbackIntent {
  return (
    intent === 'ignore_sender' ||
    intent === 'this_mattered' ||
    intent === 'more_like_this' ||
    intent === 'false_positive'
  );
}

export interface RenderedFeedbackAck {
  readonly template_version: typeof FEEDBACK_ACK_TEMPLATE_VERSION;
  readonly body: string;
}

/**
 * Deterministic renderer. Takes the intent enum and returns the
 * frozen ack body + template version. Same input → same output, always.
 */
export function renderFeedbackAck(
  intent: AckableFeedbackIntent
): RenderedFeedbackAck {
  return Object.freeze({
    template_version: FEEDBACK_ACK_TEMPLATE_VERSION,
    body: ACK_BODIES[intent]
  });
}

// Exported read-only for unit-test introspection. Tests assert each
// body is bounded, structurally safe, and free of forbidden substrings.
export const _ACK_BODIES_FOR_TESTS = ACK_BODIES;
