// Feedback Events — discrete events emitted by founder review or by user
// behavior, used as the v0.1 learning signal.
//
// FOMO_PLAN §9.6 enumerates the 11 event kinds. This module ships the type
// surface + a Store interface + an in-memory implementation (parallels
// services/brevio-gateway/src/audit.ts's InMemoryAuditStore that was
// migrated in Phase 2A). No caller yet — Phase 3 wires writers when the
// founder review handler, SendBlue send path, and reply parser land.
//
// Memory-vs-event split (FOMO_DESIGN §14):
//   * Feedback events are INSTANTANEOUS records ("at time T, user u snoozed
//     alert a"). They are append-only, never updated, never overwritten.
//   * Memory signals (memory-signals.ts) are CURRENT STATE ("sender s is
//     important to user u right now"). They get upserted as the system
//     learns.
//
// The detail field is redacted with the same safe-logger redact() the
// audit log uses, so a careless caller writing { access_token: 'plain' }
// into a feedback detail does not leak.

import { redact } from '../core/safe-logger.js';

export const FEEDBACK_EVENT_KINDS = [
  'founder_approved',
  'founder_rejected',
  'user_opened',
  'user_snoozed',
  'user_ignored',
  'ignored_sender',
  'asked_why',
  'stop',
  'no_response',
  'false_positive',
  'false_negative'
] as const;

export type FeedbackEventKind = (typeof FEEDBACK_EVENT_KINDS)[number];

export function isFeedbackEventKind(value: unknown): value is FeedbackEventKind {
  return typeof value === 'string' && (FEEDBACK_EVENT_KINDS as readonly string[]).includes(value);
}

export interface FeedbackEvent {
  readonly id?: number;
  // ISO 8601 timestamp.
  readonly occurred_at: string;
  readonly user_id: string;
  // Some events (stop, no_response system-wide signals) are not tied to a
  // specific alert.
  readonly alert_id: string | null;
  // Set for events about sender behavior (ignored_sender, false_positive
  // tied to a sender). Null when the event has no sender context.
  readonly sender_email: string | null;
  readonly kind: FeedbackEventKind;
  // Free-form details, redacted via safe-logger.redact on write.
  readonly detail: Record<string, unknown> | null;
}

export interface FeedbackEventInput {
  user_id: string;
  alert_id: string | null;
  sender_email: string | null;
  kind: FeedbackEventKind;
  detail?: Record<string, unknown> | null;
  occurred_at?: string;
}

export interface FeedbackStore {
  write(input: FeedbackEventInput): Promise<void>;
  recent(userId: string, limit?: number): Promise<readonly FeedbackEvent[]>;
  countByKind(userId: string, kind: FeedbackEventKind): Promise<number>;
  countBySender(userId: string, senderEmail: string): Promise<number>;
}

export class InMemoryFeedbackStore implements FeedbackStore {
  private entries: FeedbackEvent[] = [];
  private nextId = 1;
  private readonly capacity: number;

  constructor(capacity = 10_000) {
    this.capacity = capacity;
  }

  async write(input: FeedbackEventInput): Promise<void> {
    const detail = input.detail ? (redact(input.detail) as Record<string, unknown>) : null;
    this.entries.push(
      Object.freeze({
        id: this.nextId++,
        occurred_at: input.occurred_at ?? new Date().toISOString(),
        user_id: input.user_id,
        alert_id: input.alert_id,
        sender_email: input.sender_email,
        kind: input.kind,
        detail
      })
    );
    if (this.entries.length > this.capacity) {
      this.entries.splice(0, this.entries.length - this.capacity);
    }
  }

  async recent(userId: string, limit = 100): Promise<readonly FeedbackEvent[]> {
    const filtered = this.entries.filter((e) => e.user_id === userId);
    return filtered.slice(-limit).reverse();
  }

  async countByKind(userId: string, kind: FeedbackEventKind): Promise<number> {
    return this.entries.filter((e) => e.user_id === userId && e.kind === kind).length;
  }

  async countBySender(userId: string, senderEmail: string): Promise<number> {
    return this.entries.filter((e) => e.user_id === userId && e.sender_email === senderEmail).length;
  }
}
