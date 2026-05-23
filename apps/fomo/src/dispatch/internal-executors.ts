// Internal-capability executors — Phase 3A wiring.
//
// Each executor is a thin adapter over the matching substrate store.
// Args mirror the store's input shape; the executor pulls user_id from
// the DispatchContext (so dispatch callers do not have to duplicate it
// in every args payload).
//
// Flipping the three internal capabilities to executor_status='implemented'
// in the tool registry requires that these executors actually exist and
// are wired before any caller goes through the Permission Gate. The
// wireInternalExecutors() helper does both registrations in one call so
// no caller can flip the executor_status without also wiring the dispatch.

import { type AuditAction, type AuditResult, type AuditStore } from '../core/audit.js';
import {
  type FeedbackEventKind,
  type FeedbackStore
} from '../memory/feedback-events.js';
import {
  type MemorySignalKind,
  type MemorySignalSource,
  type MemorySignalStore
} from '../memory/memory-signals.js';

import { type DispatchTable, type Executor } from './dispatcher.js';

/* ---------------------------------------------------------------------- */
/* audit.write                                                            */
/* ---------------------------------------------------------------------- */

export interface AuditWriteArgs {
  action: AuditAction;
  target?: string | null;
  result?: AuditResult;
  detail?: Record<string, unknown> | null;
  // Optional override; defaults to context.user_id. Null is explicit
  // "system action, no user actor".
  actor_user_id?: string | null;
  actor_ip?: string | null;
  actor_user_agent?: string | null;
  occurred_at?: string;
}

export function auditWriteExecutor(audit: AuditStore): Executor<AuditWriteArgs, void> {
  return async (args, context) => {
    await audit.write({
      actor_user_id: args.actor_user_id === undefined ? context.user_id : args.actor_user_id,
      actor_ip: args.actor_ip ?? null,
      actor_user_agent: args.actor_user_agent ?? null,
      action: args.action,
      target: args.target ?? null,
      result: args.result ?? 'success',
      detail: args.detail ?? null,
      occurred_at: args.occurred_at
    });
  };
}

/* ---------------------------------------------------------------------- */
/* feedback.write                                                         */
/* ---------------------------------------------------------------------- */

export interface FeedbackWriteArgs {
  alert_id: string | null;
  sender_email: string | null;
  kind: FeedbackEventKind;
  detail?: Record<string, unknown> | null;
  occurred_at?: string;
}

export function feedbackWriteExecutor(feedback: FeedbackStore): Executor<FeedbackWriteArgs, void> {
  return async (args, context) => {
    await feedback.write({
      user_id: context.user_id,
      alert_id: args.alert_id,
      sender_email: args.sender_email,
      kind: args.kind,
      detail: args.detail,
      occurred_at: args.occurred_at
    });
  };
}

/* ---------------------------------------------------------------------- */
/* memory_signal.write                                                    */
/* ---------------------------------------------------------------------- */

export interface MemorySignalUpsertArgs {
  kind: MemorySignalKind;
  scope_key: string | null;
  detail: Record<string, unknown>;
  source: MemorySignalSource;
  confidence?: number;
  updated_at?: string;
}

export function memorySignalUpsertExecutor(
  memory: MemorySignalStore
): Executor<MemorySignalUpsertArgs, void> {
  return async (args, context) => {
    await memory.upsert({
      user_id: context.user_id,
      kind: args.kind,
      scope_key: args.scope_key,
      detail: args.detail,
      source: args.source,
      confidence: args.confidence,
      updated_at: args.updated_at
    });
  };
}

/* ---------------------------------------------------------------------- */
/* Wireup helper                                                          */
/* ---------------------------------------------------------------------- */

export interface InternalExecutorStores {
  readonly audit: AuditStore;
  readonly feedback: FeedbackStore;
  readonly memory: MemorySignalStore;
}

// Single entry point. Registers all three internal-capability executors on
// the dispatch table. Callers that flipped the tool registry's three
// internal tools to 'implemented' MUST call this — otherwise the gate
// allows but dispatch returns no_executor_for_tool, which is fail-closed
// but uninformative.
export function wireInternalExecutors(
  table: DispatchTable,
  stores: InternalExecutorStores
): void {
  table.register('audit.write', auditWriteExecutor(stores.audit));
  table.register('feedback.write', feedbackWriteExecutor(stores.feedback));
  table.register('memory_signal.write', memorySignalUpsertExecutor(stores.memory));
}
