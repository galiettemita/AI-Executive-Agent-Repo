// Memory Signals — current-state personalization signals about a user.
//
// FOMO_PLAN §9.7 enumerates the 6 v0.1 signal kinds. This module ships the
// type surface + Store interface + in-memory implementation. No caller yet —
// Phase 3 wires writes when the founder review handler, reply parser, and
// background derivation jobs land.
//
// Distinction from feedback events (FOMO_DESIGN §14):
//   * Feedback events (feedback-events.ts) are APPEND-ONLY, INSTANTANEOUS
//     records ("at time T, user u snoozed alert a"). Never updated.
//   * Memory signals are UPSERTED, CURRENT-STATE records ("sender s has
//     importance=high for user u right now"). They are the working personal
//     model, and they can be revised as the system learns.
//
// Provenance fields (source, confidence) are mandatory so a future "show me
// what you remember about me" surface can explain why a signal exists and
// whether the user confirmed it.

import { redact } from '../core/safe-logger.js';

export const MEMORY_SIGNAL_KINDS = [
  'sender_importance',
  'sender_suppressed',
  'timing_preference',
  'topic_importance',
  'alert_usefulness',
  'quietness_preference',
  // Phase 3F.1 — TCPA-style STOP / UNSUBSCRIBE / CANCEL compliance.
  // Identity: (user_id, kind='stop_active', scope_key=null). The
  // outbound-sender worker consults this signal at the top of every
  // send cycle and refuses to dispatch when active. START flips the
  // signal back via the same upsert path. Per founder directive
  // 2026-05-26: STOP enforcement is deterministic — no LLM decides
  // whether STOP means stop. Detail shape: { active: boolean,
  // recorded_at: ISO, source_event_id?: number, source_text_slug?:
  // string }. NEVER the founder's raw reply text, NEVER the full
  // from-phone.
  'stop_active'
] as const;

export type MemorySignalKind = (typeof MEMORY_SIGNAL_KINDS)[number];

export function isMemorySignalKind(value: unknown): value is MemorySignalKind {
  return typeof value === 'string' && (MEMORY_SIGNAL_KINDS as readonly string[]).includes(value);
}

export type MemorySignalSource =
  | 'user_confirmed'   // user explicitly told us
  | 'founder_set'      // founder configured during onboarding/admin
  | 'feedback_derived' // computed from feedback events
  | 'inferred'         // best-effort guess from behavior
  // Phase 3G.1 item #2 — the outbound-sender detected a SendBlue
  // OPTED_OUT response, meaning the carrier-level opt-out list and
  // our local stop_active memory have drifted. We re-write
  // stop_active=true with this source so the operator can
  // distinguish "user said STOP" (user_confirmed) from "carrier
  // already had them opted out and local cache was wrong"
  // (opt_out_drift_carrier).
  | 'opt_out_drift_carrier';

export const MEMORY_SIGNAL_SOURCES: readonly MemorySignalSource[] = Object.freeze([
  'user_confirmed',
  'founder_set',
  'feedback_derived',
  'inferred',
  'opt_out_drift_carrier'
] as const);

export function isMemorySignalSource(value: unknown): value is MemorySignalSource {
  return typeof value === 'string' && (MEMORY_SIGNAL_SOURCES as readonly string[]).includes(value);
}

export interface MemorySignal {
  readonly id?: number;
  // ISO 8601.
  readonly updated_at: string;
  readonly user_id: string;
  readonly kind: MemorySignalKind;
  // Narrows the signal within its kind. e.g. sender email for
  // sender_importance, topic string for topic_importance, null for
  // user-wide preferences (timing_preference, quietness_preference).
  readonly scope_key: string | null;
  readonly detail: Record<string, unknown>;
  // 0..1. Defaulted by source if not explicitly given (see defaultConfidence).
  readonly confidence: number;
  readonly source: MemorySignalSource;
}

export interface MemorySignalInput {
  user_id: string;
  kind: MemorySignalKind;
  scope_key: string | null;
  detail: Record<string, unknown>;
  source: MemorySignalSource;
  confidence?: number;
  updated_at?: string;
}

export interface MemorySignalStore {
  upsert(input: MemorySignalInput): Promise<void>;
  get(userId: string, kind: MemorySignalKind, scopeKey?: string | null): Promise<MemorySignal | null>;
  list(userId: string): Promise<readonly MemorySignal[]>;
  listByKind(userId: string, kind: MemorySignalKind): Promise<readonly MemorySignal[]>;
  delete(userId: string, kind: MemorySignalKind, scopeKey: string | null): Promise<boolean>;
}

export function defaultConfidence(source: MemorySignalSource): number {
  switch (source) {
    case 'user_confirmed':
      return 1.0;
    case 'founder_set':
      return 1.0;
    case 'feedback_derived':
      return 0.7;
    case 'inferred':
      return 0.5;
    case 'opt_out_drift_carrier':
      // The carrier authoritatively declined to deliver — same
      // confidence as a user-confirmed STOP. Operator can still
      // override by texting START.
      return 1.0;
  }
}

function clamp01(n: number): number {
  if (Number.isNaN(n)) return 0;
  if (n < 0) return 0;
  if (n > 1) return 1;
  return n;
}

export class InMemoryMemorySignalStore implements MemorySignalStore {
  private readonly signals = new Map<string, MemorySignal>();
  private nextId = 1;

  private key(userId: string, kind: MemorySignalKind, scopeKey: string | null): string {
    // The empty-string suffix encodes the null case unambiguously — '' is
    // not a valid scope_key (callers pass real ids or null) so collisions
    // are impossible.
    return `${userId}|${kind}|${scopeKey ?? ''}`;
  }

  async upsert(input: MemorySignalInput): Promise<void> {
    const detail = redact(input.detail) as Record<string, unknown>;
    const key = this.key(input.user_id, input.kind, input.scope_key);
    const existing = this.signals.get(key);
    const id = existing?.id ?? this.nextId++;
    const confidence = clamp01(input.confidence ?? defaultConfidence(input.source));
    this.signals.set(
      key,
      Object.freeze({
        id,
        updated_at: input.updated_at ?? new Date().toISOString(),
        user_id: input.user_id,
        kind: input.kind,
        scope_key: input.scope_key,
        detail,
        confidence,
        source: input.source
      })
    );
  }

  async get(
    userId: string,
    kind: MemorySignalKind,
    scopeKey: string | null = null
  ): Promise<MemorySignal | null> {
    return this.signals.get(this.key(userId, kind, scopeKey)) ?? null;
  }

  async list(userId: string): Promise<readonly MemorySignal[]> {
    const out: MemorySignal[] = [];
    for (const s of this.signals.values()) {
      if (s.user_id === userId) out.push(s);
    }
    return out;
  }

  async listByKind(userId: string, kind: MemorySignalKind): Promise<readonly MemorySignal[]> {
    const out: MemorySignal[] = [];
    for (const s of this.signals.values()) {
      if (s.user_id === userId && s.kind === kind) out.push(s);
    }
    return out;
  }

  async delete(userId: string, kind: MemorySignalKind, scopeKey: string | null): Promise<boolean> {
    return this.signals.delete(this.key(userId, kind, scopeKey));
  }
}
