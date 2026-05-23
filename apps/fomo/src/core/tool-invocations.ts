// Tool Invocations — the per-dispatch call log. Records every tool execution
// the kernel mediates: which tool, which user, what the Permission Gate
// decided, whether the executor succeeded, and how long it took.
//
// FOMO_PLAN §9.10 lists this as a Phase 2 kernel piece. It is NOT a
// substitute for the audit log — audit_log records high-level events
// (consent.grant, oauth.connect, session.created); tool_invocations
// records the per-call dispatch outcome at the gate boundary. Phase 3
// callers will write here after the Permission Gate returns a decision
// and (if allowed) the executor runs.
//
// Privacy invariant — load-bearing:
//   The metadata field MUST NOT carry raw payload content (email body,
//   reply text, model prompt, etc.). It is sanitized through safe-logger
//   redact() on write, and the test suite asserts that obvious payload
//   shapes ('body_plain', 'body_html', 'reply_text', 'prompt') get
//   redacted or are simply not part of the documented schema.

import { redact } from './safe-logger.js';

export type ToolInvocationStatus = 'success' | 'failure' | 'denied';

export interface ToolInvocationRecord {
  readonly id?: number;
  // ISO 8601.
  readonly occurred_at: string;
  readonly user_id: string;
  readonly tool_id: string;
  // Caller-supplied dedup key (e.g. UUID per dispatch attempt). Used by
  // future idempotency logic; here we just store it.
  readonly invocation_id: string;
  // The PolicyDecision code from the gate (e.g. 'allowed', 'not_implemented',
  // 'send_disabled'). Stored as a string to avoid coupling this module to
  // the gate's enum — the gate's PolicyDecisionCode flows through here as
  // opaque text for auditability.
  readonly policy_decision: string;
  readonly status: ToolInvocationStatus;
  readonly latency_ms: number | null;
  readonly error_code: string | null;
  readonly error_reason: string | null;
  // Sanitized metadata only. Examples: { tier_used: 'send' },
  // { model_name: 'mock-classifier-tiny' }. NEVER raw payload content.
  // Redacted via safe-logger on write.
  readonly metadata: Record<string, unknown> | null;
}

export interface ToolInvocationInput {
  user_id: string;
  tool_id: string;
  invocation_id: string;
  policy_decision: string;
  status: ToolInvocationStatus;
  latency_ms?: number | null;
  error_code?: string | null;
  error_reason?: string | null;
  metadata?: Record<string, unknown> | null;
  occurred_at?: string;
}

export interface ToolInvocationStore {
  write(input: ToolInvocationInput): Promise<void>;
  recent(userId: string, limit?: number): Promise<readonly ToolInvocationRecord[]>;
  countByTool(userId: string, toolId: string): Promise<number>;
  countByStatus(userId: string, status: ToolInvocationStatus): Promise<number>;
  byInvocationId(invocationId: string): Promise<ToolInvocationRecord | null>;
}

export class InMemoryToolInvocationStore implements ToolInvocationStore {
  private records: ToolInvocationRecord[] = [];
  private nextId = 1;
  private readonly capacity: number;

  constructor(capacity = 50_000) {
    this.capacity = capacity;
  }

  async write(input: ToolInvocationInput): Promise<void> {
    const metadata = input.metadata ? (redact(input.metadata) as Record<string, unknown>) : null;
    this.records.push(
      Object.freeze({
        id: this.nextId++,
        occurred_at: input.occurred_at ?? new Date().toISOString(),
        user_id: input.user_id,
        tool_id: input.tool_id,
        invocation_id: input.invocation_id,
        policy_decision: input.policy_decision,
        status: input.status,
        latency_ms: input.latency_ms ?? null,
        error_code: input.error_code ?? null,
        error_reason: input.error_reason ?? null,
        metadata
      })
    );
    if (this.records.length > this.capacity) {
      this.records.splice(0, this.records.length - this.capacity);
    }
  }

  async recent(userId: string, limit = 100): Promise<readonly ToolInvocationRecord[]> {
    const filtered = this.records.filter((r) => r.user_id === userId);
    return filtered.slice(-limit).reverse();
  }

  async countByTool(userId: string, toolId: string): Promise<number> {
    return this.records.filter((r) => r.user_id === userId && r.tool_id === toolId).length;
  }

  async countByStatus(userId: string, status: ToolInvocationStatus): Promise<number> {
    return this.records.filter((r) => r.user_id === userId && r.status === status).length;
  }

  async byInvocationId(invocationId: string): Promise<ToolInvocationRecord | null> {
    return this.records.find((r) => r.invocation_id === invocationId) ?? null;
  }
}
