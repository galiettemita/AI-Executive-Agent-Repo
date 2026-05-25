// Audit log helper. Writes to public.audit_log via injected store; in dev mode uses
// in-memory ring buffer so the simulator can read its own audit history without Postgres.

import { redact } from './safe-logger.js';

export type AuditAction =
  // Lifecycle (Phase 2A)
  | 'consent.grant'
  | 'consent.revoke'
  | 'consent.snooze'
  | 'oauth.connect'
  | 'oauth.disconnect'
  | 'oauth.refresh'
  | 'oauth.revoke_failed'
  | 'token.decrypt_failure'
  | 'session.created'
  | 'onboarding.dismissed'
  // Kernel-touch events (Phase 2F.1) — written by the integrated kernel
  // path so the audit log participates in every meaningful substrate
  // operation, not just lifecycle events. Callers MUST pass sanitized
  // detail only: no raw email body, no headers, no attachment filenames,
  // no prompt text, no full reply text. Operational identifiers
  // (tool_id, model_name, prompt_version, alert_id, from/to_state) are
  // OK; user-payload content is not.
  | 'policy.decided'
  | 'tool.invoked'
  | 'state.transitioned'
  | 'feedback.written'
  | 'memory.upserted'
  | 'model.routed'
  // Workflow events (Phase 3B.2) — one entry per polling worker cycle.
  // Per-message reads continue to surface as policy.decided + tool.invoked
  // for tool_id='gmail.read'; this aggregate cycle entry exists so ops
  // can answer "is polling alive?" without correlating dispatch events.
  | 'gmail.poll.cycle'
  // Ranker events (Phase 3C.3) — one entry per dispatched gmail.read
  // result when the polling worker has a ranker wired. Sanitized detail
  // only: model_name + prompt_version + label + score + token counts +
  // latency + cost. NEVER body content; the ranker's input is already
  // egress-redacted but this audit row does not include input either.
  // already_ranked surfaces idempotency hits (rank_results unique
  // constraint matched); failed surfaces ranker timeouts/schema errors.
  | 'fomo.rank.completed'
  | 'fomo.rank.already_ranked'
  | 'fomo.rank.failed';

export type AuditResult = 'success' | 'failure';

export interface AuditEntry {
  id?: number;
  occurred_at: string;
  actor_user_id: string | null;
  actor_ip: string | null;
  actor_user_agent: string | null;
  action: AuditAction;
  target: string | null;
  result: AuditResult;
  detail: Record<string, unknown> | null;
}

export interface AuditStore {
  write(entry: Omit<AuditEntry, 'id' | 'occurred_at'> & { occurred_at?: string }): Promise<void>;
  recent(userId: string, limit?: number): Promise<AuditEntry[]>;
}

export class InMemoryAuditStore implements AuditStore {
  private entries: AuditEntry[] = [];
  private nextId = 1;
  private readonly capacity: number;

  constructor(capacity = 5000) {
    this.capacity = capacity;
  }

  async write(entry: Omit<AuditEntry, 'id' | 'occurred_at'> & { occurred_at?: string }): Promise<void> {
    const detail = entry.detail ? (redact(entry.detail) as Record<string, unknown>) : null;
    this.entries.push({
      id: this.nextId++,
      occurred_at: entry.occurred_at ?? new Date().toISOString(),
      actor_user_id: entry.actor_user_id,
      actor_ip: entry.actor_ip,
      actor_user_agent: entry.actor_user_agent,
      action: entry.action,
      target: entry.target,
      result: entry.result,
      detail
    });
    if (this.entries.length > this.capacity) {
      this.entries.splice(0, this.entries.length - this.capacity);
    }
  }

  async recent(userId: string, limit = 100): Promise<AuditEntry[]> {
    const filtered = this.entries.filter((e) => e.actor_user_id === userId);
    return filtered.slice(-limit).reverse();
  }
}
