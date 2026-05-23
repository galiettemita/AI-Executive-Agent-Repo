// Gmail Polling Worker — Phase 3B.2.
//
// One cycle = one runOnce() call. For each user with a stored Gmail
// cursor:
//   1. Skip silently when the user has no Google token, or the token
//      row is marked needs_reauth.
//   2. Call GmailClient.listHistorySince(token, cursor.history_id) to
//      discover new message IDs. listHistorySince is a metadata-only
//      operation (no message bodies) and is invoked DIRECTLY through
//      the client, NOT through the dispatch table — discovering which
//      messages exist is a polling-implementation concern, not a tool
//      invocation.
//   3. On GmailUnauthorizedError from listHistorySince: mark
//      needs_reauth, surface in the cycle report, and continue to the
//      next user.
//   4. For each new message_id, decide policy + mint AuthorizedToolCall
//      + dispatch.execute('gmail.read', { message_id }). Each dispatch
//      writes audit (policy.decided + tool.invoked) and tool_invocations
//      via injected helpers. The returned RawEmailContext is DISCARDED
//      in 3B.2 (no ranker yet); Phase 3C adds the ranker consumer.
//   5. Persist the advanced cursor (cursorStore.upsert with the new
//      latest_history_id).
//   6. Write one aggregate 'gmail.poll.cycle' audit entry summarizing
//      the cycle.
//
// Audit footprint per cycle:
//   - 1 'gmail.poll.cycle' entry, always
//   - 2 entries per new message ('policy.decided' + 'tool.invoked')
//   - 0 per-user audit; per-user outcomes summarized in the cycle entry
//
// Kill-switch coupling: the worker itself does NOT consult
// switches.polling_enabled — the bootstrap in apps/fomo/src/index.ts is
// the only place that reads the flag (gating whether the interval is
// installed). runOnce() can still be called by tests / admin endpoints
// when the flag is off.

import { GmailClient, GmailUnauthorizedError, GmailApiError } from '../adapters/gmail/client.js';
import { type AuditStore } from '../core/audit.js';
import { decidePolicy, type PolicyGateDeps } from '../core/policy-gate.js';
import { type ToolInvocationStore } from '../core/tool-invocations.js';
import { AuthorizedToolCall, type DispatchTable } from '../dispatch/dispatcher.js';
import { type GmailCursorStore } from '../memory/gmail-cursors.js';
import { type TokenStore } from '../security/oauth/token-store.js';

export interface GmailPollDeps {
  readonly gmailClient: GmailClient;
  readonly tokenStore: TokenStore;
  readonly cursorStore: GmailCursorStore;
  readonly dispatch: DispatchTable;
  readonly auditStore: AuditStore;
  readonly toolInvocationStore: ToolInvocationStore;
  readonly gateDeps: PolicyGateDeps;
  // ID generator for invocation_id. Defaults to a counter-prefixed
  // string; tests inject a deterministic one.
  readonly newInvocationId?: () => string;
  // Optional clock; defaults to Date.now.
  readonly now?: () => number;
  // Max history items fetched per user per cycle. Forwarded to
  // listHistorySince.maxResults. Defaults to 100 (Gmail's own page size).
  readonly maxResultsPerUser?: number;
}

export type GmailPollUserOutcome =
  | { readonly user_id: string; readonly status: 'skipped_no_token' }
  | { readonly user_id: string; readonly status: 'skipped_needs_reauth' }
  | { readonly user_id: string; readonly status: 'unauthorized'; readonly previous_history_id: string }
  | {
      readonly user_id: string;
      readonly status: 'api_error';
      readonly previous_history_id: string;
      readonly error: string;
      readonly retryable: boolean;
    }
  | {
      readonly user_id: string;
      readonly status: 'polled';
      readonly previous_history_id: string;
      readonly new_history_id: string;
      readonly messages_observed: number;
      readonly messages_dispatched: number;
      readonly messages_failed: number;
    };

export interface GmailPollCycleReport {
  readonly started_at: string;
  readonly finished_at: string;
  readonly users_total: number;
  readonly users_polled: number;
  readonly users_skipped: number;
  readonly users_unauthorized: number;
  readonly users_api_error: number;
  readonly messages_observed: number;
  readonly messages_dispatched: number;
  readonly messages_failed: number;
  readonly outcomes: readonly GmailPollUserOutcome[];
}

function defaultInvocationIdGenerator(): () => string {
  let n = 0;
  const seed = Math.random().toString(36).slice(2, 8);
  return () => `gmail-poll-${seed}-${++n}`;
}

export async function runOnce(deps: GmailPollDeps): Promise<GmailPollCycleReport> {
  const now = deps.now ?? Date.now;
  const newInvocationId = deps.newInvocationId ?? defaultInvocationIdGenerator();
  const maxResultsPerUser = deps.maxResultsPerUser ?? 100;

  const started_at = new Date(now()).toISOString();
  const outcomes: GmailPollUserOutcome[] = [];
  let users_polled = 0;
  let users_skipped = 0;
  let users_unauthorized = 0;
  let users_api_error = 0;
  let messages_observed = 0;
  let messages_dispatched = 0;
  let messages_failed = 0;

  const userIds = await deps.cursorStore.listUserIds();

  for (const user_id of userIds) {
    const cursor = await deps.cursorStore.get(user_id);
    if (!cursor) {
      // A user appeared in listUserIds() but the cursor row vanished
      // between the list and the get — treat as skipped, do not throw.
      outcomes.push({ user_id, status: 'skipped_no_token' });
      users_skipped++;
      continue;
    }

    // Check token presence + needs_reauth flag via TokenStore.list.
    const tokens = await deps.tokenStore.list(user_id);
    const googleToken = tokens.find((t) => t.provider === 'google');
    if (!googleToken) {
      outcomes.push({ user_id, status: 'skipped_no_token' });
      users_skipped++;
      continue;
    }
    if (googleToken.needs_reauth) {
      outcomes.push({ user_id, status: 'skipped_needs_reauth' });
      users_skipped++;
      continue;
    }
    const accessToken = await deps.tokenStore.loadAccessToken(user_id, 'google');
    if (accessToken === null) {
      // Defense-in-depth: token row exists but decrypt failed.
      outcomes.push({ user_id, status: 'skipped_no_token' });
      users_skipped++;
      continue;
    }

    // Discover new message IDs since the cursor.
    let history;
    try {
      history = await deps.gmailClient.listHistorySince(accessToken, cursor.history_id, {
        maxResults: maxResultsPerUser
      });
    } catch (err) {
      if (err instanceof GmailUnauthorizedError) {
        await deps.tokenStore.markNeedsReauth(user_id, 'google');
        outcomes.push({
          user_id,
          status: 'unauthorized',
          previous_history_id: cursor.history_id
        });
        users_unauthorized++;
        continue;
      }
      if (err instanceof GmailApiError) {
        outcomes.push({
          user_id,
          status: 'api_error',
          previous_history_id: cursor.history_id,
          error: err.message,
          retryable: err.retryable
        });
        users_api_error++;
        continue;
      }
      // Unknown error class — treat as non-retryable api_error to keep
      // the cycle alive for other users.
      outcomes.push({
        user_id,
        status: 'api_error',
        previous_history_id: cursor.history_id,
        error: err instanceof Error ? err.message : String(err),
        retryable: false
      });
      users_api_error++;
      continue;
    }

    const newIds = history.added_message_ids;
    messages_observed += newIds.length;

    let dispatched = 0;
    let failed = 0;
    for (const message_id of newIds) {
      const decision = decidePolicy(
        { tool_id: 'gmail.read', user_id, intent: 'read' },
        deps.gateDeps
      );

      const invocation_id = newInvocationId();

      // Audit policy.decided regardless of allow/deny.
      await deps.auditStore.write({
        actor_user_id: user_id,
        actor_ip: null,
        actor_user_agent: null,
        action: 'policy.decided',
        target: 'tool:gmail.read',
        result: 'success',
        detail: {
          tool_id: 'gmail.read',
          code: decision.code,
          allowed: decision.allowed
        }
      });

      const authorized = AuthorizedToolCall.fromDecision(decision);
      if (!authorized) {
        // Gate denied. Should not happen for a user with cursor + token +
        // consent + non-needs_reauth, but a defense-in-depth path is
        // still needed.
        await deps.toolInvocationStore.write({
          user_id,
          tool_id: 'gmail.read',
          invocation_id,
          policy_decision: decision.code,
          status: 'denied',
          latency_ms: 0
        });
        await deps.auditStore.write({
          actor_user_id: user_id,
          actor_ip: null,
          actor_user_agent: null,
          action: 'tool.invoked',
          target: 'tool:gmail.read',
          result: 'failure',
          detail: {
            tool_id: 'gmail.read',
            invocation_id,
            policy_decision: decision.code,
            status: 'denied'
          }
        });
        failed++;
        continue;
      }

      const result = await deps.dispatch.execute(
        authorized,
        { message_id },
        { user_id, invocation_id }
      );

      await deps.toolInvocationStore.write({
        user_id,
        tool_id: 'gmail.read',
        invocation_id,
        policy_decision: decision.code,
        status: result.ok ? 'success' : 'failure',
        latency_ms: result.latency_ms,
        error_code: result.ok ? null : result.code,
        error_reason: result.ok ? null : result.reason
      });

      await deps.auditStore.write({
        actor_user_id: user_id,
        actor_ip: null,
        actor_user_agent: null,
        action: 'tool.invoked',
        target: 'tool:gmail.read',
        result: result.ok ? 'success' : 'failure',
        detail: {
          tool_id: 'gmail.read',
          invocation_id,
          policy_decision: decision.code,
          status: result.ok ? 'success' : 'failure'
        }
      });

      if (result.ok) {
        dispatched++;
      } else {
        failed++;
      }
    }

    // Advance the cursor whether or not any messages were observed.
    // Gmail's listHistorySince returns the current global historyId
    // even when no items matched the filter, so this acks the read
    // window and prevents re-fetching the same range next cycle.
    await deps.cursorStore.upsert({
      user_id,
      history_id: history.latest_history_id
    });

    outcomes.push({
      user_id,
      status: 'polled',
      previous_history_id: cursor.history_id,
      new_history_id: history.latest_history_id,
      messages_observed: newIds.length,
      messages_dispatched: dispatched,
      messages_failed: failed
    });
    users_polled++;
    messages_dispatched += dispatched;
    messages_failed += failed;
  }

  const finished_at = new Date(now()).toISOString();

  const report: GmailPollCycleReport = Object.freeze({
    started_at,
    finished_at,
    users_total: userIds.length,
    users_polled,
    users_skipped,
    users_unauthorized,
    users_api_error,
    messages_observed,
    messages_dispatched,
    messages_failed,
    outcomes: Object.freeze(outcomes.map((o) => Object.freeze(o)))
  });

  // Aggregate audit. Operator-facing — answers "did the cycle run, and
  // how did it go." No per-user email content; outcomes carry only
  // operational identifiers (user_id + status + history_id + counts).
  await deps.auditStore.write({
    actor_user_id: null, // system action, no user actor
    actor_ip: null,
    actor_user_agent: null,
    action: 'gmail.poll.cycle',
    target: 'worker:gmail-poll',
    result: report.users_api_error + report.users_unauthorized + messages_failed > 0
      ? 'failure'
      : 'success',
    detail: {
      users_total: report.users_total,
      users_polled: report.users_polled,
      users_skipped: report.users_skipped,
      users_unauthorized: report.users_unauthorized,
      users_api_error: report.users_api_error,
      messages_observed: report.messages_observed,
      messages_dispatched: report.messages_dispatched,
      messages_failed: report.messages_failed
    }
  });

  return report;
}

/* ---------------------------------------------------------------------- */
/* startPolling — interval wrapper                                        */
/* ---------------------------------------------------------------------- */

export interface PollingHandle {
  // Stops the interval. Awaitable so callers can ensure any in-flight
  // runOnce has settled before returning. NEVER throws.
  stop(): Promise<void>;
}

export interface StartPollingOptions {
  readonly intervalMs: number;
  // Optional: called once with the report after each successful cycle
  // and with the error after each failure. Tests inject this to
  // observe cycles without polluting stdout. Defaults to no-op.
  readonly onCycle?: (report: GmailPollCycleReport) => void;
  readonly onError?: (err: unknown) => void;
}

export function startPolling(deps: GmailPollDeps, opts: StartPollingOptions): PollingHandle {
  if (!Number.isInteger(opts.intervalMs) || opts.intervalMs <= 0) {
    throw new Error(`startPolling: intervalMs must be a positive integer (got ${opts.intervalMs})`);
  }

  let stopped = false;
  let inflight: Promise<void> = Promise.resolve();
  let timer: ReturnType<typeof setTimeout> | null = null;

  const tick = (): void => {
    if (stopped) return;
    inflight = (async () => {
      try {
        const report = await runOnce(deps);
        if (!stopped) opts.onCycle?.(report);
      } catch (err) {
        if (!stopped) opts.onError?.(err);
      }
    })();
    void inflight.finally(() => {
      if (stopped) return;
      timer = setTimeout(tick, opts.intervalMs);
    });
  };

  // First tick runs immediately.
  tick();

  return {
    async stop() {
      stopped = true;
      if (timer !== null) {
        clearTimeout(timer);
        timer = null;
      }
      // Await the in-flight cycle if one is running.
      await inflight.catch(() => undefined);
    }
  };
}
