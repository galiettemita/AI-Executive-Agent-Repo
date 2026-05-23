// Kernel Integration Harness — Phase 2F's gate.
//
// One in-process function that exercises every kernel piece Phase 2A
// through 2E shipped, captures the substrate's observable behavior in a
// KernelScenarioReport, and lets the gate test assert end-to-end that
// the substrate cooperates as designed.
//
// No HTTP route, no real Gmail / SendBlue / Slack / model calls. Mock
// backends and in-memory stores only. No executor_status flips.
//
// Honest-semantics invariant (FOMO_PLAN §9.3 + Phase 2C.1 amendment):
//   every tool in the v0.1 registry is executor_status='declared', so
//   every gate decision in this default scenario denies — either
//   'not_implemented' for the 6 known tools, or 'unknown_tool' for the
//   one off-registry probe. Phase 3 wires real dispatch and the
//   gate-only tests in integration-harness.test.ts cover what happens
//   once tools flip to 'implemented'.
//
// The harness DOES NOT write to the audit log. Per FOMO_PLAN §9.10 the
// audit log is for high-level lifecycle events (consent.grant,
// oauth.connect, session.created); none of those happen here, so its
// entry count stays 0 and the test asserts it. The per-dispatch detail
// goes to tool_invocations instead, which is exactly what it's for.

import {
  type AlertState,
  INITIAL_STATE,
  transition
} from '../core/state-machine.js';
import { applyEgressForRanker, applyEgressForReplyParser, applyEgressForSlackCard, type RawEmailContext } from '../core/egress-policy.js';
import { type KillSwitches, loadKillSwitches } from '../core/kill-switches.js';
import { type PolicyDecision, decidePolicy } from '../core/policy-gate.js';
import { createToolRegistry, type ToolDescriptor, type ToolId, type ToolRegistry } from '../core/tool-registry.js';
import { MockModelBackend } from '../core/model-backends/mock.js';
import { type ModelBackend, type ModelOutputValidator, createModelRouter } from '../core/model-router.js';
import { type SubstrateStoresHandle, createStores } from '../db/store-factory.js';

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
  readonly audit: {
    // Always 0 in v0.1's substrate scenario — nothing audit-worthy happens
    // (no consent grant, no oauth connect, no session create). Phase 3
    // flows that DO trigger those events will see this count rise.
    readonly entries_written: number;
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

function summarizeTool(t: ToolDescriptor): ToolId {
  return t.id;
}

function detectForbiddenLeaks(serialized: string): readonly string[] {
  return FORBIDDEN_STRINGS.filter((s) => serialized.includes(s));
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
   * 3. Permission Gate — 6 known tools + 1 unknown (Phase 2B + 2C.1)
   *    All known tools deny 'not_implemented' under default state.
   *    Each decision is also recorded in tool_invocations.
   * ------------------------------------------------------------------ */
  const toolProbes: readonly string[] = [
    ...all.map((t) => t.id),
    'booking.flights' // off-registry probe → unknown_tool
  ];
  const policyDecisions = await Promise.all(
    toolProbes.map(async (tool_id, idx) => {
      const decision = decidePolicy(
        { tool_id, user_id: SYNTHETIC_USER_ID },
        {
          registry,
          switches,
          hasConsent: () => true,
          hasOAuth: () => true
        }
      );
      await stores.toolInvocations.write({
        user_id: SYNTHETIC_USER_ID,
        tool_id,
        invocation_id: `harness-inv-${idx}`,
        policy_decision: decision.code,
        status: decision.allowed ? 'success' : 'denied',
        latency_ms: 0
      });
      return {
        tool_id,
        code: decision.code,
        allowed: decision.allowed
      };
    })
  );

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
    transition_records_written++;
  }
  const terminal_state = (await stores.transitions.currentState(SYNTHETIC_ALERT_ID)) ?? INITIAL_STATE;

  /* --------------------------------------------------------------------
   * 6. Feedback Events (Phase 2C) — founder_approved + user_snoozed
   * ------------------------------------------------------------------ */
  await stores.feedback.write({
    user_id: SYNTHETIC_USER_ID,
    alert_id: SYNTHETIC_ALERT_ID,
    sender_email: SYNTHETIC_RAW_EMAIL.sender_email,
    kind: 'founder_approved',
    detail: { score: 0.91 }
  });
  await stores.feedback.write({
    user_id: SYNTHETIC_USER_ID,
    alert_id: SYNTHETIC_ALERT_ID,
    sender_email: SYNTHETIC_RAW_EMAIL.sender_email,
    kind: 'user_snoozed',
    detail: { until: '2026-05-22T22:00:00.000Z' }
  });
  const approved_count = await stores.feedback.countByKind(SYNTHETIC_USER_ID, 'founder_approved');
  const snoozed_count = await stores.feedback.countByKind(SYNTHETIC_USER_ID, 'user_snoozed');
  const events_written = (await stores.feedback.recent(SYNTHETIC_USER_ID)).length;

  /* --------------------------------------------------------------------
   * 7. Memory Signals (Phase 2C) — sender_importance + quietness_preference
   * ------------------------------------------------------------------ */
  await stores.memory.upsert({
    user_id: SYNTHETIC_USER_ID,
    kind: 'sender_importance',
    scope_key: SYNTHETIC_RAW_EMAIL.sender_email,
    detail: { importance: 'high' },
    source: 'user_confirmed'
  });
  await stores.memory.upsert({
    user_id: SYNTHETIC_USER_ID,
    kind: 'quietness_preference',
    scope_key: null,
    detail: { max_per_day: 5 },
    source: 'user_confirmed'
  });
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

  /* --------------------------------------------------------------------
   * 9. Cost Tracking (Phase 2D)
   * ------------------------------------------------------------------ */
  const costRecords = await stores.cost.recent(SYNTHETIC_USER_ID);
  const cost_total_usd = await stores.cost.sumByModel(SYNTHETIC_USER_ID, modelName);

  /* --------------------------------------------------------------------
   * 10. Audit Log (Phase 2A) — 0 entries because the harness does NOT
   *     perform any audit-worthy lifecycle action. Asserted in the test.
   * ------------------------------------------------------------------ */
  const audit_entries_written = (await stores.audit.recent(SYNTHETIC_USER_ID)).length;

  /* --------------------------------------------------------------------
   * 11. Tool Invocations (Phase 2E) — one per gate decision above
   * ------------------------------------------------------------------ */
  const invocations = await stores.toolInvocations.recent(SYNTHETIC_USER_ID);

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
      entries_written: invocations.length
    }),
    audit: Object.freeze({
      entries_written: audit_entries_written
    }),
    store_backend: handle.backend
  });
}
