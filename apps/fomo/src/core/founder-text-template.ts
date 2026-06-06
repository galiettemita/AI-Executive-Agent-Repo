// Founder Text Template — Phase v0.5.7 thin wrapper around the
// Human Message Renderer.
//
// HISTORY: this file used to host the full deterministic renderer (Phase
// 3E.1 → v0.5.6). With v0.5.7 the renderer is the FIRST surface of the
// founder-locked Human Message Renderer product principle (memory
// feedback_brevio-human-message-renderer-principle) and lives in
// apps/fomo/src/core/human-message-renderer.ts.
//
// This file remains as a THIN WRAPPER per Q6.A — preserves the legacy
// `FOUNDER_TEXT_*` constant names + the `renderFounderText` function +
// the legacy `reason_source` output field — so any existing call sites
// or test suites that reference the old API continue to work. New code
// SHOULD call `renderHumanMessage` directly from human-message-renderer.
//
// 3E.1 directive (founder-locked 2026-05-25) PRESERVED — the wrapper
// adds no new LLM call; it delegates to renderHumanMessage which is
// itself pure deterministic.

import {
  HUMAN_MESSAGE_ABSOLUTE_MAX_CHARS,
  HUMAN_MESSAGE_HARD_MAX_CHARS,
  HUMAN_MESSAGE_TARGET_MAX_CHARS,
  HUMAN_MESSAGE_TARGET_MIN_CHARS,
  HUMAN_MESSAGE_TEMPLATE_VERSION,
  RANKER_V2_PROMPT_VERSION,
  REASON_FALLBACK_STRING,
  renderHumanMessage,
  type ReasonViolationKind,
  type ReasonVoice,
  type SenderResolutionPath,
  type SubjectStripApplied,
  type TemplateShape
} from './human-message-renderer.js';
import { type SlackEgressView } from './egress-policy.js';
import { type RankLabel } from '../memory/rank-results.js';

/* -------------------------------------------------------------------- */
/* Legacy constants — re-exports of HMR values                           */
/* -------------------------------------------------------------------- */

// v0.5.7 rename: 'founder-text-v0.2.0' → 'human-message-v0.3.0'.
// Operators grep audit `template_version` to see when HMR took over.
export const FOUNDER_TEXT_TEMPLATE_VERSION = HUMAN_MESSAGE_TEMPLATE_VERSION;

export const FOUNDER_TEXT_TARGET_MIN_CHARS = HUMAN_MESSAGE_TARGET_MIN_CHARS;
export const FOUNDER_TEXT_TARGET_MAX_CHARS = HUMAN_MESSAGE_TARGET_MAX_CHARS;
export const FOUNDER_TEXT_HARD_MAX_CHARS = HUMAN_MESSAGE_HARD_MAX_CHARS;
export const FOUNDER_TEXT_ABSOLUTE_MAX_CHARS = HUMAN_MESSAGE_ABSOLUTE_MAX_CHARS;

export { REASON_FALLBACK_STRING };
export type { ReasonViolationKind };

/* -------------------------------------------------------------------- */
/* Legacy input / output shapes (v0.5.6) with v0.5.7 extensions          */
/* -------------------------------------------------------------------- */

export interface FounderTextTemplateInput {
  readonly view: SlackEgressView;
  readonly rank: {
    readonly label: RankLabel;
    readonly score: number;
    readonly reason: string;
  };
  // v0.5.7 OPTIONAL extension — ranker PROMPT_VERSION at the time
  // rank.reason was produced. Defaults to 'ranker-v0.1.0' for back-
  // compat (legacy_3p reason voice). New callers SHOULD pass the
  // current ranker version from cost_records.
  readonly prompt_version?: string;
}

export interface FounderTextTemplateOutput {
  readonly text: string;
  readonly template_version: string;
  // Legacy v0.5.6 field — keep so existing tests + audit consumers
  // continue to read 'rank' | 'fallback'. Derived from reason_voice
  // ('2p_action' | 'legacy_3p' → 'rank', 'fallback' → 'fallback').
  readonly reason_source: 'rank' | 'fallback';
  readonly reason_violation_kind: ReasonViolationKind | null;
  readonly original_reason_length: number;

  // v0.5.7 audit-field outputs — Q6.A locked. Outbound-sender writes
  // these into fomo.send.attempted detail.
  readonly sender_resolution_path: SenderResolutionPath;
  readonly subject_strip_applied: SubjectStripApplied;
  readonly reason_voice: ReasonVoice;
  readonly template_shape: TemplateShape;
  // True when ANY Q5.A degradation rule triggered.
  readonly degradation_applied: boolean;
}

/* -------------------------------------------------------------------- */
/* Thin wrapper — delegates to renderHumanMessage                        */
/* -------------------------------------------------------------------- */

// renderFounderText (legacy entry point). New code should call
// renderHumanMessage directly with a HumanMessageEgressView so the
// full Q2.B chain (email-local first-name extraction) is available.
// This wrapper builds an HMR view from the legacy SlackEgressView
// which carries only `sender_email_masked` — the email-local path
// of Q2.B step 3 cannot match against a masked address, so the
// chain degrades to step 4 (generic) for senders without a
// human-looking `sender_name`.
//
// Note: in v0.5.6, this function was load-bearing for the outbound
// path. In v0.5.7 the outbound-sender calls renderHumanMessage
// directly (with the raw RawEmailContext-projected view) — see
// apps/fomo/src/workers/outbound-sender.ts. This wrapper remains for
// (a) the v0.5.6 test suite, (b) any future legacy caller.
export function renderFounderText(
  input: FounderTextTemplateInput
): FounderTextTemplateOutput {
  const out = renderHumanMessage({
    surface: 'email_alert',
    view: {
      purpose: 'human_message_renderer',
      sender_name: input.view.sender_name,
      // Pass the masked email — degrades Q2.B step 3 (email-local
      // first-name extraction) but preserves step 1 (sender_name) and
      // most of step 2 (domain still extractable from masked address).
      sender_email: input.view.sender_email_masked,
      subject: input.view.subject,
      received_at: input.view.received_at,
      message_id: input.view.message_id
    },
    rank: input.rank,
    prompt_version: input.prompt_version ?? 'ranker-v0.1.0'
  });
  const reason_source: 'rank' | 'fallback' =
    out.reason_voice === 'fallback' ? 'fallback' : 'rank';
  return Object.freeze({
    text: out.text,
    template_version: out.template_version,
    reason_source,
    reason_violation_kind: out.reason_violation_kind,
    original_reason_length: out.original_reason_length,
    sender_resolution_path: out.sender_resolution_path,
    subject_strip_applied: out.subject_strip_applied,
    reason_voice: out.reason_voice,
    template_shape: out.template_shape,
    degradation_applied: out.degradation_applied
  });
}

// Re-export for callers that want the new ranker version constant.
export { RANKER_V2_PROMPT_VERSION };
