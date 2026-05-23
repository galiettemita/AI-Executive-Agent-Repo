// Kernel Integration Harness — Phase 2F's gate, extended for 3A and 3B.2.
//
// One in-process function that exercises every kernel piece Phase 2A
// through 3B.2 shipped, captures the substrate's observable behavior in
// a KernelScenarioReport, and lets the gate test assert end-to-end that
// the substrate cooperates as designed.
//
// No real Gmail / SendBlue / Slack / model calls. Mock backends and
// in-memory stores only. The Gmail HTTP layer is exercised through an
// injected mock fetch so the Phase 3B.2 polling worker can drive a real
// gmail.read dispatch end-to-end without hitting the network.
//
// Honest-semantics invariant (FOMO_PLAN §9.3 + Phase 2C.1 amendment):
//   Phase 3A flipped audit.write / feedback.write / memory_signal.write
//   to 'implemented'. Phase 3B.2 flipped gmail.read to 'implemented'.
//   sendblue.send_user_message and slack.founder_review remain
//   'declared' and the gate denies them with 'not_implemented' in this
//   scenario. The gate-only tests in integration-harness.test.ts cover
//   what happens once those tools flip to 'implemented'.
//
// Audit invariant (Phase 2F.1 + 3B.2):
//   The harness writes an audit entry at every meaningful kernel touch
//   (gate decision, tool invocation, state transition, feedback write,
//   memory upsert, model route). The polling worker writes its own
//   per-message policy.decided + tool.invoked entries plus one
//   aggregate gmail.poll.cycle entry per cycle. Every audit detail is
//   SANITIZED — only operational identifiers (tool_id, model_name,
//   prompt_version, alert_id, from/to_state, kind, history_id) appear.
//   Raw email body, raw headers, attachment filenames, prompt text, and
//   full reply text are never passed into audit. The test asserts this
//   end-to-end by serializing every audit entry — including the
//   polling cycle's — and looking for known leak canaries.

import {
  type AlertState,
  INITIAL_STATE,
  transition
} from '../core/state-machine.js';
import { GMAIL_READONLY_SCOPE, GmailClient } from '../adapters/gmail/client.js';
import { type AuditAction, type AuditStore } from '../core/audit.js';
import { applyEgressForRanker, applyEgressForReplyParser, applyEgressForSlackCard, type RawEmailContext } from '../core/egress-policy.js';
import { type KillSwitches, loadKillSwitches } from '../core/kill-switches.js';
import { type PolicyDecision, decidePolicy } from '../core/policy-gate.js';
import { createToolRegistry, type ToolDescriptor, type ToolId, type ToolRegistry } from '../core/tool-registry.js';
import { MockModelBackend } from '../core/model-backends/mock.js';
import { type ModelBackend, type ModelOutputValidator, createModelRouter } from '../core/model-router.js';
import { type ToolInvocationStatus } from '../core/tool-invocations.js';
import { AuthorizedToolCall, createDispatchTable } from '../dispatch/dispatcher.js';
import { wireInternalExecutors } from '../dispatch/internal-executors.js';
import { wireExternalExecutors } from '../dispatch/external-executors.js';
import { type SubstrateStoresHandle, createStores } from '../db/store-factory.js';
import { runOnce as runPollOnce, type GmailPollCycleReport } from '../workers/gmail-poll.js';

/* ---------------------------------------------------------------------- */
/* Inputs and outputs                                                     */
/* ---------------------------------------------------------------------- */

export interface KernelScenarioDeps {
  // env override for createStores (defaults to {} so CI takes the in-memory path).
  env?: NodeJS.ProcessEnv;
  // KEK so the test never has to set BREVIO_TOKEN_KEK process-wide.
  cryptoKek?: Buffer;
}

export interface KernelScenarioReport {
  readonly registry: {
    readonly total_tools: number;
    readonly external_tool_ids: readonly ToolId[];
    readonly internal_tool_ids: readonly ToolId[];
    readonly declared_tool_ids: readonly ToolId[];
    readonly implemented_tool_ids: readonly ToolId[];
  };
  readonly kill_switches: KillSwitches;
  readonly policy_decisions: readonly {
    readonly tool_id: string;
    readonly code: PolicyDecision['code'];
    readonly allowed: boolean;
  }[];
  readonly egress: {
    readonly ranker_view_keys: readonly string[];
    readonly slack_view_keys: readonly string[];
    readonly reply_parser_view_keys: readonly string[];
    readonly slack_sender_masked: string;
    // Strings that MUST NOT appear in any view. Empty when the egress
    // policy held; populated with the leaked keys/strings otherwise.
    readonly forbidden_leaks: readonly string[];
  };
  readonly state_machine: {
    readonly initial_state: AlertState;
    readonly path: readonly AlertState[];
    readonly terminal_state: AlertState;
    readonly transition_records_written: number;
  };
  readonly feedback: {
    readonly events_written: number;
    readonly approved_count: number;
    readonly snoozed_count: number;
  };
  readonly memory: {
    readonly signals_written: number;
    readonly sender_importance_value: string | null;
    readonly quietness_max_per_day: number | null;
  };
  readonly model: {
    readonly capability: 'classification';
    readonly model_name: string;
    readonly output_label: string | null;
    readonly schema_valid: boolean;
  };
  readonly cost: {
    readonly records_written: number;
    readonly total_usd: number;
  };
  readonly tool_invocations: {
    readonly entries_written: number;
  };
  // Phase 3B.2: the polling worker runs one cycle for the synthetic user.
  // Captures the worker's report so the gate test can prove the
  // gmail.read dispatch path is reachable end-to-end (gate → AuthorizedToolCall
  // → dispatch → executor → GmailClient → cursor advance).
  readonly polling: {
    readonly users_total: number;
    readonly users_polled: number;
    readonly messages_observed: number;
    readonly messages_dispatched: number;
    readonly messages_failed: number;
    readonly cursor_before: string;
    readonly cursor_after: string;
  };
  readonly audit: {
    // Total entries the harness wrote. > 0 in Phase 2F.1 — every
    // meaningful kernel touch is audited.
    readonly entries_written: number;
    // Count per AuditAction so the test verifies each required category
    // (policy.decided, tool.invoked, state.transitioned, feedback.written,
    // memory.upserted, model.routed) is exercised at least once.
    readonly by_action: Readonly<Record<string, number>>;
    // Strings that MUST NOT appear in any audit entry's serialized form.
    // Empty when audit-write discipline held; populated with leaked
    // strings otherwise. Mirrors the egress forbidden_leaks check.
    readonly forbidden_leaks: readonly string[];
  };
  readonly store_backend: 'in_memory' | 'postgres';
}

/* ---------------------------------------------------------------------- */
/* Harness                                                                */
/* ---------------------------------------------------------------------- */

const SYNTHETIC_USER_ID = 'kernel-harness-user';
const SYNTHETIC_ALERT_ID = 'kernel-harness-alert';

// The path the harness walks through the alert state machine. Mirrors the
// FOMO_DESIGN §7.1 demo flow: detected → ranked → queued_for_review →
// approved → sent → replied → snoozed.
const STATE_PATH: readonly AlertState[] = Object.freeze([
  'detected',
  'ranked',
  'queued_for_review',
  'approved',
  'sent',
  'replied',
  'snoozed'
]);

// Synthetic email used to exercise the Egress Policy. Carries every
// forbidden-leak shape — body_html, raw headers, attachment filenames,
// raw sender — so the harness can prove none of them survive the redaction.
const SYNTHETIC_RAW_EMAIL: RawEmailContext = Object.freeze({
  message_id: 'msg_kernel_harness',
  thread_id: 'thr_kernel_harness',
  sender_email: 'sarah.j@school.edu',
  sender_name: 'Sarah Johnson',
  subject: 'Interview form due tonight',
  body_plain: 'Hi Albert, please submit the interview form by 9pm. — Sarah',
  body_html: '<html><body><b>Hi Albert</b>, please submit the interview form by 9pm. — Sarah</body></html>',
  headers: {
    'Authentication-Results': 'spf=pass',
    'X-Harness-Tag': 'kernel-harness-leak-canary-9f7c2a',
    'Received': 'from school.edu by gmail.com'
  },
  attachments: [{ filename: 'interview-form.pdf', size_bytes: 12345 }],
  received_at: new Date('2026-05-22T18:30:00.000Z')
});

const FORBIDDEN_STRINGS: readonly string[] = Object.freeze([
  '<html>',
  '<b>',
  'Authentication-Results',
  'X-Harness-Tag',
  'kernel-harness-leak-canary-9f7c2a',
  'Received',
  'interview-form.pdf'
]);

// Additional strings the AUDIT log specifically must not contain. The
// harness passes a known-recognizable prompt and reply text so the test
// can verify these never reach an audit detail.
const HARNESS_REPLY_TEXT = 'kernel-harness reply canary — remind me later tonight';
const AUDIT_FORBIDDEN_STRINGS: readonly string[] = Object.freeze([
  ...FORBIDDEN_STRINGS,
  // Prompt text passed to the model router. audit 'model.routed' must
  // record prompt_version (a stable ID), never the prompt itself.
  'classify: Interview form due tonight',
  // Reply parser user text. audit 'feedback.written' for user_snoozed
  // must record kind + alert_id, never the user's reply text.
  HARNESS_REPLY_TEXT,
  // Synthetic body (also covered by FORBIDDEN_STRINGS via 'Sarah' for
  // most cases, but explicit here for clarity).
  'please submit the interview form by 9pm'
]);

function summarizeTool(t: ToolDescriptor): ToolId {
  return t.id;
}

function detectForbiddenLeaks(serialized: string): readonly string[] {
  return FORBIDDEN_STRINGS.filter((s) => serialized.includes(s));
}

function detectAuditForbiddenLeaks(serialized: string): readonly string[] {
  return AUDIT_FORBIDDEN_STRINGS.filter((s) => serialized.includes(s));
}

// Inline helper: write a sanitized audit entry. Centralized so every
// kernel-touch audit goes through one call site and the harness is
// auditable itself.
async function logKernelAudit(
  audit: AuditStore,
  action: AuditAction,
  target: string,
  detail: Record<string, unknown>
): Promise<void> {
  await audit.write({
    actor_user_id: SYNTHETIC_USER_ID,
    actor_ip: null,
    actor_user_agent: null,
    action,
    target,
    result: 'success',
    detail
  });
}

function defaultKek(): Buffer {
  // Stable test KEK so reruns are deterministic.
  return Buffer.alloc(32, 0x7e);
}

// The model output the mock backend returns. Caller asserts shape matches.
interface ClassifierOutput {
  label: 'important' | 'not_important';
}

const validateClassifierOutput: ModelOutputValidator<ClassifierOutput> = (text) => {
  try {
    const parsed = JSON.parse(text) as unknown;
    if (typeof parsed !== 'object' || parsed === null) {
      return { ok: false, reason: 'output is not an object' };
    }
    const label = (parsed as Record<string, unknown>).label;
    if (label === 'important' || label === 'not_important') {
      return { ok: true, value: { label } };
    }
    return { ok: false, reason: `unknown label: ${String(label)}` };
  } catch (err) {
    return { ok: false, reason: `not JSON: ${err instanceof Error ? err.message : String(err)}` };
  }
};

const HARNESS_PROMPT = 'classify: Interview form due tonight';

function buildMockBackend(): ModelBackend {
  return new MockModelBackend({
    model_name: 'mock-classifier-tiny',
    responses: {
      [HARNESS_PROMPT]: {
        text: JSON.stringify({ label: 'important' }),
        input_tokens: 42,
        output_tokens: 6,
        latency_ms: 5
      }
    }
  });
}

export async function runKernelIntegrationScenario(
  deps: KernelScenarioDeps = {}
): Promise<KernelScenarioReport> {
  const env = deps.env ?? {};
  const cryptoKek = deps.cryptoKek ?? defaultKek();

  /* --------------------------------------------------------------------
   * Substrate construction — exercises store-factory (Phase 2E) end-to-end
   * ------------------------------------------------------------------ */
  const handle: SubstrateStoresHandle = createStores({
    env,
    crypto: { kek: cryptoKek, devMode: false }
  });
  const stores = handle.stores;

  /* --------------------------------------------------------------------
   * 1. Tool Registry (Phase 2B)
   * ------------------------------------------------------------------ */
  const registry: ToolRegistry = createToolRegistry();
  const all = registry.getActiveTools();
  const external = registry.getExternalTools().map(summarizeTool);
  const internal = registry.getInternalCapabilities().map(summarizeTool);
  const declared = all.filter((t) => t.executor_status === 'declared').map(summarizeTool);
  const implemented = all.filter((t) => t.executor_status === 'implemented').map(summarizeTool);

  /* --------------------------------------------------------------------
   * 2. Kill Switches (Phase 2B)
   * ------------------------------------------------------------------ */
  const switches = loadKillSwitches({});

  /* --------------------------------------------------------------------
   * 2.5. Dispatch table (Phase 3A + 3B.2)
   *    Wires the three internal-capability executors AND the gmail.read
   *    executor. The dispatch table is independent of the Permission
   *    Gate — the gate decides ALLOW or DENY, and only when allowed
   *    does the harness call dispatch. No recursive audit: the harness's
   *    policy.decided / tool.invoked writes go DIRECTLY to stores.audit,
   *    NOT through dispatch.
   *
   *    The Gmail HTTP layer is exercised via an injected mock fetch.
   *    The mock handles /users/me/history (returns one new message id)
   *    and /users/me/messages/{id} (returns a synthetic message body
   *    carrying the same forbidden-leak canaries as SYNTHETIC_RAW_EMAIL
   *    so the audit-leak test surfaces any worker-side regression).
   * ------------------------------------------------------------------ */
  const HARNESS_POLL_HISTORY_START = 'h-harness-1';
  const HARNESS_POLL_HISTORY_END = 'h-harness-2';
  const HARNESS_POLL_MESSAGE_ID = 'msg-harness-poll-1';
  const HARNESS_GMAIL_ACCESS_TOKEN = 'gmail-harness-token';

  // Synthetic Gmail messages.get response. Carries the same leak canaries
  // as SYNTHETIC_RAW_EMAIL so any worker-side leak surfaces in audit.
  const SYNTHETIC_GMAIL_MESSAGE_JSON = Object.freeze({
    id: HARNESS_POLL_MESSAGE_ID,
    threadId: 'thr_harness_poll',
    internalDate: '1748000000000',
    payload: {
      headers: [
        { name: 'From', value: 'Sarah Johnson <sarah.j@school.edu>' },
        { name: 'Subject', value: 'Interview form due tonight' },
        { name: 'X-Harness-Tag', value: 'kernel-harness-leak-canary-9f7c2a' },
        { name: 'Authentication-Results', value: 'spf=pass' },
        { name: 'Received', value: 'from school.edu by gmail.com' }
      ],
      mimeType: 'text/plain',
      // base64url('Hi Albert, please submit the interview form by 9pm. — Sarah')
      body: { data: 'SGkgQWxiZXJ0LCBwbGVhc2Ugc3VibWl0IHRoZSBpbnRlcnZpZXcgZm9ybSBieSA5cG0uIOKAlCBTYXJhaA' }
    }
  });

  const harnessFetchImpl: typeof fetch = (async (input: string | URL | Request) => {
    const url = typeof input === 'string' ? input : input.toString();
    if (url.includes('/users/me/history')) {
      return new Response(
        JSON.stringify({
          history: [
            {
              id: HARNESS_POLL_HISTORY_END,
              messagesAdded: [
                {
                  message: {
                    id: HARNESS_POLL_MESSAGE_ID,
                    threadId: 'thr_harness_poll'
                  }
                }
              ]
            }
          ],
          historyId: HARNESS_POLL_HISTORY_END
        }),
        { status: 200, headers: { 'content-type': 'application/json' } }
      );
    }
    if (url.includes(`/users/me/messages/${HARNESS_POLL_MESSAGE_ID}`)) {
      return new Response(JSON.stringify(SYNTHETIC_GMAIL_MESSAGE_JSON), {
        status: 200,
        headers: { 'content-type': 'application/json' }
      });
    }
    // Defensive — anything else is unexpected in the harness scenario.
    return new Response(JSON.stringify({}), {
      status: 404,
      headers: { 'content-type': 'application/json' }
    });
  }) as typeof fetch;

  const gmailClient = new GmailClient({ fetchImpl: harnessFetchImpl });

  const dispatch = createDispatchTable();
  wireInternalExecutors(dispatch, {
    audit: stores.audit,
    feedback: stores.feedback,
    memory: stores.memory
  });
  wireExternalExecutors(dispatch, {
    gmailClient,
    tokenStore: stores.tokens
  });

  // Seed the harness user's OAuth token + Gmail cursor so the Phase 3B.2
  // polling worker has the substrate it expects after a real OAuth
  // connect would have run.
  await stores.tokens.save({
    user_id: SYNTHETIC_USER_ID,
    provider: 'google',
    scopes: [GMAIL_READONLY_SCOPE],
    access_token: HARNESS_GMAIL_ACCESS_TOKEN
  });
  await stores.gmailCursors.upsert({
    user_id: SYNTHETIC_USER_ID,
    history_id: HARNESS_POLL_HISTORY_START
  });

  /* --------------------------------------------------------------------
   * 3. Permission Gate → Dispatch → Store (Phase 3A + 3B.2 integrated path)
   *
   *    Section A (3 invocations) — denied at the gate, no dispatch fires:
   *      sendblue.send_user_message  → not_implemented (external+declared)
   *      slack.founder_review        → not_implemented (internal+declared)
   *      booking.flights             → unknown_tool
   *
   *      gmail.read used to live here as a 'declared' denial. Phase 3B.2
   *      flipped it to 'implemented' and the polling worker (section 3.5
   *      below) drives the green path.
   *
   *    Section B (5 invocations) — allowed at the gate, dispatch fires:
   *      audit.write                 → executor writes 'session.created' audit
   *      feedback.write × 2          → executor writes feedback events
   *      memory_signal.write × 2     → executor upserts memory signals
   * ------------------------------------------------------------------ */
  const gateDeps = {
    registry,
    switches,
    hasConsent: () => true,
    hasOAuth: () => true
  };

  interface Invocation {
    readonly tool_id: string;
    readonly args: unknown;
    readonly domain_audit?: () => Promise<void>;
  }

  const invocations: readonly Invocation[] = [
    // Section A — denials
    { tool_id: 'sendblue.send_user_message', args: null },
    { tool_id: 'slack.founder_review', args: null },
    { tool_id: 'booking.flights', args: null },
    // Section B — internal dispatches.
    // audit.write does NOT produce a separate domain audit — its
    // executor IS the audit write (action='session.created' below).
    {
      tool_id: 'audit.write',
      args: {
        action: 'session.created' as AuditAction,
        target: 'session:kernel-harness',
        detail: { source: 'kernel-harness' }
      }
    },
    // feedback.write × 2 — first founder_approved, then user_snoozed.
    // The user_snoozed detail intentionally carries HARNESS_REPLY_TEXT so
    // the audit-leak test can verify it never reaches an audit entry
    // (the full reply text belongs in feedback_events, NOT in audit).
    {
      tool_id: 'feedback.write',
      args: {
        alert_id: SYNTHETIC_ALERT_ID,
        sender_email: SYNTHETIC_RAW_EMAIL.sender_email,
        kind: 'founder_approved' as const,
        detail: { score: 0.91 }
      },
      domain_audit: async () =>
        logKernelAudit(stores.audit, 'feedback.written', `alert:${SYNTHETIC_ALERT_ID}`, {
          kind: 'founder_approved',
          alert_id: SYNTHETIC_ALERT_ID,
          sender_present: true
        })
    },
    {
      tool_id: 'feedback.write',
      args: {
        alert_id: SYNTHETIC_ALERT_ID,
        sender_email: SYNTHETIC_RAW_EMAIL.sender_email,
        kind: 'user_snoozed' as const,
        detail: { reply_text: HARNESS_REPLY_TEXT, until: '2026-05-22T22:00:00.000Z' }
      },
      domain_audit: async () =>
        logKernelAudit(stores.audit, 'feedback.written', `alert:${SYNTHETIC_ALERT_ID}`, {
          kind: 'user_snoozed',
          alert_id: SYNTHETIC_ALERT_ID,
          sender_present: true
        })
    },
    {
      tool_id: 'memory_signal.write',
      args: {
        kind: 'sender_importance' as const,
        scope_key: SYNTHETIC_RAW_EMAIL.sender_email,
        detail: { importance: 'high' },
        source: 'user_confirmed' as const
      },
      domain_audit: async () =>
        logKernelAudit(stores.audit, 'memory.upserted', `signal:sender_importance`, {
          kind: 'sender_importance',
          scope_present: true,
          source: 'user_confirmed'
        })
    },
    {
      tool_id: 'memory_signal.write',
      args: {
        kind: 'quietness_preference' as const,
        scope_key: null,
        detail: { max_per_day: 5 },
        source: 'user_confirmed' as const
      },
      domain_audit: async () =>
        logKernelAudit(stores.audit, 'memory.upserted', `signal:quietness_preference`, {
          kind: 'quietness_preference',
          scope_present: false,
          source: 'user_confirmed'
        })
    }
  ];

  const policyDecisions: { tool_id: string; code: PolicyDecision['code']; allowed: boolean }[] = [];
  for (let idx = 0; idx < invocations.length; idx++) {
    const { tool_id, args, domain_audit } = invocations[idx]!;
    const decision = decidePolicy({ tool_id, user_id: SYNTHETIC_USER_ID }, gateDeps);

    // Audit the gate decision FIRST. Direct stores.audit.write call (NOT
    // through dispatch) — preserves the no-recursive-audit invariant when
    // tool_id happens to be 'audit.write'.
    await logKernelAudit(stores.audit, 'policy.decided', `tool:${tool_id}`, {
      tool_id,
      code: decision.code,
      allowed: decision.allowed
    });

    const invocation_id = `harness-inv-${idx}`;
    let dispatchStatus: ToolInvocationStatus = decision.allowed ? 'success' : 'denied';
    let dispatchLatency = 0;

    // Phase 3A.1 integrated path: gate decided → mint an
    // AuthorizedToolCall (returns null unless allowed) → dispatch executes.
    // dispatch.execute() refuses anything that isn't an AuthorizedToolCall
    // — there is no structural way to bypass the gate.
    const authorized = AuthorizedToolCall.fromDecision(decision);
    if (authorized !== null) {
      const result = await dispatch.execute(authorized, args, {
        user_id: SYNTHETIC_USER_ID,
        invocation_id
      });
      dispatchStatus = result.ok ? 'success' : 'failure';
      dispatchLatency = result.latency_ms;
    }

    await stores.toolInvocations.write({
      user_id: SYNTHETIC_USER_ID,
      tool_id,
      invocation_id,
      policy_decision: decision.code,
      status: dispatchStatus,
      latency_ms: dispatchLatency
    });

    await logKernelAudit(stores.audit, 'tool.invoked', `tool:${tool_id}`, {
      tool_id,
      invocation_id,
      policy_decision: decision.code,
      status: dispatchStatus
    });

    // Domain-specific post-dispatch audit (feedback.written /
    // memory.upserted). audit.write does not need one because its
    // executor IS the domain audit write — adding another here would
    // be the "recursive audit logging" the founder explicitly forbade.
    if (decision.allowed && dispatchStatus === 'success' && domain_audit) {
      await domain_audit();
    }

    policyDecisions.push({
      tool_id,
      code: decision.code,
      allowed: decision.allowed
    });
  }

  /* --------------------------------------------------------------------
   * 3.5. Gmail polling worker — runOnce (Phase 3B.2)
   *    One cycle, one user, one new message. The worker calls
   *    GmailClient.listHistorySince → dispatches gmail.read once →
   *    advances the cursor. Writes its own policy.decided + tool.invoked
   *    audits (under SYNTHETIC_USER_ID) plus one aggregate
   *    gmail.poll.cycle audit (system actor, actor_user_id=null).
   * ------------------------------------------------------------------ */
  let pollInvId = 0;
  const pollReport: GmailPollCycleReport = await runPollOnce({
    gmailClient,
    tokenStore: stores.tokens,
    cursorStore: stores.gmailCursors,
    dispatch,
    auditStore: stores.audit,
    toolInvocationStore: stores.toolInvocations,
    gateDeps,
    newInvocationId: () => `harness-poll-inv-${++pollInvId}`
  });
  const pollCursorAfter = (await stores.gmailCursors.get(SYNTHETIC_USER_ID))?.history_id
    ?? HARNESS_POLL_HISTORY_START;

  /* --------------------------------------------------------------------
   * 4. Egress Policy (Phase 2C) — 3 views over a synthetic email
   *    Verify no body_html / headers / attachment-filenames leak
   *    and the Slack view masks the sender.
   * ------------------------------------------------------------------ */
  const rankerView = applyEgressForRanker(SYNTHETIC_RAW_EMAIL);
  const slackView = applyEgressForSlackCard(SYNTHETIC_RAW_EMAIL);
  const replyParserView = applyEgressForReplyParser('remind me later tonight', {
    subject: SYNTHETIC_RAW_EMAIL.subject,
    sender_name: SYNTHETIC_RAW_EMAIL.sender_name,
    message_id: SYNTHETIC_RAW_EMAIL.message_id
  });
  const allViewsSerialized = JSON.stringify({ rankerView, slackView, replyParserView });
  const forbidden_leaks = detectForbiddenLeaks(allViewsSerialized);

  /* --------------------------------------------------------------------
   * 5. Alert State Machine (Phase 2C) — walk the v0.1 demo path
   *    Each transition is validated by the pure transition() function and
   *    persisted via the AlertStateTransitionStore.
   * ------------------------------------------------------------------ */
  let transition_records_written = 0;
  for (let i = 1; i < STATE_PATH.length; i++) {
    const from = STATE_PATH[i - 1]!;
    const to = STATE_PATH[i]!;
    const result = transition(from, to, `kernel-harness step ${i}`);
    if ('error' in result) {
      throw new Error(
        `kernel harness: built an invalid state-machine path: ${from} → ${to} (${result.reason})`
      );
    }
    await stores.transitions.write({
      alert_id: SYNTHETIC_ALERT_ID,
      user_id: SYNTHETIC_USER_ID,
      from_state: from,
      to_state: to,
      reason: result.reason
    });
    // Audit the transition. Sanitized: alert_id + from/to_state only.
    // result.reason is operator-authored harness text (no user payload).
    await logKernelAudit(stores.audit, 'state.transitioned', `alert:${SYNTHETIC_ALERT_ID}`, {
      alert_id: SYNTHETIC_ALERT_ID,
      from_state: from,
      to_state: to
    });
    transition_records_written++;
  }
  const terminal_state = (await stores.transitions.currentState(SYNTHETIC_ALERT_ID)) ?? INITIAL_STATE;

  /* --------------------------------------------------------------------
   * 6. Feedback Events + Memory Signals (Phase 2C + 3A)
   *    Both stores were written via dispatch in section 3 above. Here we
   *    just read back the counts and signal values for the report.
   * ------------------------------------------------------------------ */
  const approved_count = await stores.feedback.countByKind(SYNTHETIC_USER_ID, 'founder_approved');
  const snoozed_count = await stores.feedback.countByKind(SYNTHETIC_USER_ID, 'user_snoozed');
  const events_written = (await stores.feedback.recent(SYNTHETIC_USER_ID)).length;

  const senderSignal = await stores.memory.get(
    SYNTHETIC_USER_ID,
    'sender_importance',
    SYNTHETIC_RAW_EMAIL.sender_email
  );
  const quietnessSignal = await stores.memory.get(SYNTHETIC_USER_ID, 'quietness_preference');
  const signals_written = (await stores.memory.list(SYNTHETIC_USER_ID)).length;

  /* --------------------------------------------------------------------
   * 8. Model Router (Phase 2D) — classification via mock backend
   * ------------------------------------------------------------------ */
  const modelRouter = createModelRouter({ costStore: stores.cost });
  modelRouter.registerBackend('classification', buildMockBackend());
  const routeResult = await modelRouter.route({
    capability: 'classification',
    prompt: HARNESS_PROMPT,
    prompt_version: 'kernel-harness-v1',
    user_id: SYNTHETIC_USER_ID,
    validate: validateClassifierOutput
  });
  const modelOutputLabel = routeResult.ok ? routeResult.output.label : null;
  const modelSchemaValid = routeResult.ok;
  const modelName = routeResult.model_name ?? 'mock-classifier-tiny';
  // Audit the model call — sanitized: capability + model_name +
  // prompt_version + schema_valid. NOT the prompt text, NOT the model
  // output text. Cost lives in cost_records; this audit confirms the
  // call happened.
  await logKernelAudit(stores.audit, 'model.routed', `capability:classification`, {
    capability: 'classification',
    model_name: modelName,
    prompt_version: 'kernel-harness-v1',
    schema_valid: modelSchemaValid
  });

  /* --------------------------------------------------------------------
   * 9. Cost Tracking (Phase 2D)
   * ------------------------------------------------------------------ */
  const costRecords = await stores.cost.recent(SYNTHETIC_USER_ID);
  const cost_total_usd = await stores.cost.sumByModel(SYNTHETIC_USER_ID, modelName);

  /* --------------------------------------------------------------------
   * 10. Audit Log (Phase 2A + 2F.1) — every kernel touch above wrote an
   *     audit entry. Count, classify, and run the leak-canary check.
   * ------------------------------------------------------------------ */
  const auditEntries = await stores.audit.recent(SYNTHETIC_USER_ID, 1000);
  const audit_entries_written = auditEntries.length;
  const audit_by_action: Record<string, number> = {};
  for (const e of auditEntries) {
    audit_by_action[e.action] = (audit_by_action[e.action] ?? 0) + 1;
  }
  const auditSerialized = JSON.stringify(auditEntries);
  const audit_forbidden_leaks = detectAuditForbiddenLeaks(auditSerialized);

  /* --------------------------------------------------------------------
   * 11. Tool Invocations (Phase 2E) — one per gate decision above
   * ------------------------------------------------------------------ */
  const toolInvocationEntries = await stores.toolInvocations.recent(SYNTHETIC_USER_ID);

  return Object.freeze({
    registry: Object.freeze({
      total_tools: all.length,
      external_tool_ids: Object.freeze(external),
      internal_tool_ids: Object.freeze(internal),
      declared_tool_ids: Object.freeze(declared),
      implemented_tool_ids: Object.freeze(implemented)
    }),
    kill_switches: switches,
    policy_decisions: Object.freeze(policyDecisions.map((d) => Object.freeze(d))),
    egress: Object.freeze({
      ranker_view_keys: Object.freeze(Object.keys(rankerView)),
      slack_view_keys: Object.freeze(Object.keys(slackView)),
      reply_parser_view_keys: Object.freeze(Object.keys(replyParserView)),
      slack_sender_masked: slackView.sender_email_masked,
      forbidden_leaks: Object.freeze(forbidden_leaks)
    }),
    state_machine: Object.freeze({
      initial_state: INITIAL_STATE,
      path: STATE_PATH,
      terminal_state,
      transition_records_written
    }),
    feedback: Object.freeze({
      events_written,
      approved_count,
      snoozed_count
    }),
    memory: Object.freeze({
      signals_written,
      sender_importance_value:
        senderSignal && typeof (senderSignal.detail as Record<string, unknown>).importance === 'string'
          ? ((senderSignal.detail as Record<string, unknown>).importance as string)
          : null,
      quietness_max_per_day:
        quietnessSignal && typeof (quietnessSignal.detail as Record<string, unknown>).max_per_day === 'number'
          ? ((quietnessSignal.detail as Record<string, unknown>).max_per_day as number)
          : null
    }),
    model: Object.freeze({
      capability: 'classification' as const,
      model_name: modelName,
      output_label: modelOutputLabel,
      schema_valid: modelSchemaValid
    }),
    cost: Object.freeze({
      records_written: costRecords.length,
      total_usd: cost_total_usd
    }),
    tool_invocations: Object.freeze({
      entries_written: toolInvocationEntries.length
    }),
    polling: Object.freeze({
      users_total: pollReport.users_total,
      users_polled: pollReport.users_polled,
      messages_observed: pollReport.messages_observed,
      messages_dispatched: pollReport.messages_dispatched,
      messages_failed: pollReport.messages_failed,
      cursor_before: HARNESS_POLL_HISTORY_START,
      cursor_after: pollCursorAfter
    }),
    audit: Object.freeze({
      entries_written: audit_entries_written,
      by_action: Object.freeze({ ...audit_by_action }),
      forbidden_leaks: Object.freeze(audit_forbidden_leaks)
    }),
    store_backend: handle.backend
  });
}
