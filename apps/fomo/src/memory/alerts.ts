// Alert Store — one row per candidate alert posted (or to be posted)
// to the founder Slack channel for review.
//
// Phase 3D.1 adds this store as the caller for the `alerts` table
// declared in schema.ts. The polling worker creates one row per
// label='important' rank_results row when FOMO_SLACK_REVIEW_ENABLED is
// on. Phase 3D.2 will add the inbound approve/reject capture path that
// transitions the alert state from queued_for_review →
// approved | rejected.
//
// Idempotency: UNIQUE on rank_result_id. A given rank can produce AT
// MOST one alert. The store's write path is ON CONFLICT DO NOTHING and
// returns { inserted, alert } so the caller can distinguish first-post
// from idempotency hit. This protects the founder's Slack channel from
// re-spamming on cursor rewinds, history replays, or worker restarts.
//
// Privacy: this store accepts ONLY operational identifiers. No body
// content, no subject, no sender_email. The Slack card payload is
// applied via applyEgressForSlackCard at post time and never persisted
// here.

export type AlertLabel = 'important' | 'not_important';

export interface Alert {
  readonly alert_id: string;
  readonly user_id: string;
  readonly message_id: string;
  readonly rank_result_id: number;
  readonly label: AlertLabel;
  readonly score: number;
  // ISO 8601 — DB-side default now() at write time.
  readonly created_at: string;
  // Phase v0.5.11 — HMAC(sender_email, BREVIO_SENDER_HASH_KEY). Nullable for
  // pre-migration rows + for paths that don't supply sender_email (e.g.
  // synthetic test fixtures). NEVER the cleartext sender_email.
  readonly sender_email_hash: string | null;
}

export interface AlertInput {
  readonly alert_id: string;
  readonly user_id: string;
  readonly message_id: string;
  readonly rank_result_id: number;
  readonly label: AlertLabel;
  readonly score: number;
  // Phase v0.5.11 — populated forward by the rank step. Optional; callers
  // that don't have a sender (test fixtures, manual triggers) pass null or
  // omit the field.
  readonly sender_email_hash?: string | null;
}

export interface AlertWriteOutcome {
  // True when the row was newly inserted. False when rank_result_id
  // already had an alert — the existing row is unchanged and is
  // returned in `alert`.
  readonly inserted: boolean;
  readonly alert: Alert;
}

export interface AlertStore {
  // ON CONFLICT (rank_result_id) DO NOTHING. Returns { inserted, alert }.
  // The returned `alert` is always populated — either the newly-inserted
  // row or the existing one — so the caller can pick the alert_id to
  // surface in audit/UX without a second query.
  create(input: AlertInput): Promise<AlertWriteOutcome>;
  // Returns the alert for an alert_id, or null.
  get(alertId: string): Promise<Alert | null>;
  // Returns the alert for a rank_result_id, or null. Used by the
  // worker's idempotency check before generating a new alert_id.
  getByRankResult(rankResultId: number): Promise<Alert | null>;
  // Count of alerts for a user.
  count(userId: string): Promise<number>;
  // Most recent N alerts for a user, newest first.
  recent(userId: string, limit: number): Promise<readonly Alert[]>;
}

export class InMemoryAlertStore implements AlertStore {
  private readonly rows: Alert[] = [];

  async create(input: AlertInput): Promise<AlertWriteOutcome> {
    const existing = this.rows.find((r) => r.rank_result_id === input.rank_result_id);
    if (existing) {
      return Object.freeze({ inserted: false, alert: existing });
    }
    const alert: Alert = Object.freeze({
      alert_id: input.alert_id,
      user_id: input.user_id,
      message_id: input.message_id,
      rank_result_id: input.rank_result_id,
      label: input.label,
      score: input.score,
      created_at: new Date().toISOString(),
      sender_email_hash: input.sender_email_hash ?? null
    });
    this.rows.push(alert);
    return Object.freeze({ inserted: true, alert });
  }

  async get(alertId: string): Promise<Alert | null> {
    return this.rows.find((r) => r.alert_id === alertId) ?? null;
  }

  async getByRankResult(rankResultId: number): Promise<Alert | null> {
    return this.rows.find((r) => r.rank_result_id === rankResultId) ?? null;
  }

  async count(userId: string): Promise<number> {
    return this.rows.filter((r) => r.user_id === userId).length;
  }

  async recent(userId: string, limit: number): Promise<readonly Alert[]> {
    if (!Number.isInteger(limit) || limit <= 0) return Object.freeze([]);
    const userRows = this.rows.filter((r) => r.user_id === userId);
    userRows.sort((a, b) => b.created_at.localeCompare(a.created_at));
    return Object.freeze(userRows.slice(0, limit));
  }
}
