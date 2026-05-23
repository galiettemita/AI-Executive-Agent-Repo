import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { SAFE_DEFAULT_KILL_SWITCHES, loadKillSwitches } from '../core/kill-switches.ts';
import { decidePolicy, type PolicyGateDeps } from '../core/policy-gate.ts';
import { createToolRegistry, type ToolId, type ToolRegistry } from '../core/tool-registry.ts';
import { runKernelIntegrationScenario } from './integration-harness.ts';

/* ====================================================================== *
 * Gate test — one big scenario assertion.                                *
 *                                                                        *
 * If any kernel piece regresses (Phase 2A through 2E), at least one      *
 * field of the KernelScenarioReport will not match its expected value    *
 * and this suite fails. This is the load-bearing assertion the founder   *
 * uses to gate Phase 3.                                                  *
 * ====================================================================== */

describe('Kernel Integration Gate — full scenario report', () => {
  it('runKernelIntegrationScenario produces the expected substrate-only report', async () => {
    const report = await runKernelIntegrationScenario({ env: {} });

    // ---- Tool Registry (Phase 2B) ----
    assert.equal(report.registry.total_tools, 6);
    assert.deepEqual(
      [...report.registry.external_tool_ids].sort(),
      ['gmail.read', 'sendblue.send_user_message']
    );
    assert.deepEqual(
      [...report.registry.internal_tool_ids].sort(),
      ['audit.write', 'feedback.write', 'memory_signal.write', 'slack.founder_review']
    );
    // Phase 2 invariant: every tool ships 'declared'. Phase 3 flips
    // executor_status as dispatch wiring lands.
    assert.equal(report.registry.declared_tool_ids.length, 6);
    assert.equal(report.registry.implemented_tool_ids.length, 0);

    // ---- Kill Switches (Phase 2B) — safe defaults all-disabled ----
    assert.equal(report.kill_switches.send_enabled, false);
    assert.equal(report.kill_switches.auto_send_enabled, false);
    assert.equal(report.kill_switches.friend_beta_enabled, false);
    assert.equal(report.kill_switches.max_users, 1);

    // ---- Permission Gate (Phase 2B + 2B.1 + 2C.1) ----
    // 7 decisions: 6 known tools (all deny not_implemented) + 1 off-registry
    // probe (denies unknown_tool). None allowed in Phase 2 substrate state.
    assert.equal(report.policy_decisions.length, 7);
    for (const d of report.policy_decisions) {
      assert.equal(d.allowed, false, `${d.tool_id} should be denied — got code=${d.code}`);
    }
    const byId = new Map(report.policy_decisions.map((d) => [d.tool_id, d.code]));
    assert.equal(byId.get('gmail.read'), 'not_implemented');
    assert.equal(byId.get('sendblue.send_user_message'), 'not_implemented');
    assert.equal(byId.get('slack.founder_review'), 'not_implemented');
    assert.equal(byId.get('audit.write'), 'not_implemented');
    assert.equal(byId.get('feedback.write'), 'not_implemented');
    assert.equal(byId.get('memory_signal.write'), 'not_implemented');
    assert.equal(byId.get('booking.flights'), 'unknown_tool');

    // ---- Egress Policy (Phase 2C) ----
    // No body_html, headers, attachment filenames, or harness leak canary
    // appears in ANY view. Slack sender is masked.
    assert.deepEqual(
      [...report.egress.forbidden_leaks],
      [],
      `egress leaked: ${report.egress.forbidden_leaks.join(', ')}`
    );
    assert.equal(report.egress.slack_sender_masked, 's***@school.edu');
    // The reply parser view does not include any email body field.
    for (const banned of ['body_plain', 'body_html', 'body_snippet']) {
      assert.equal(
        report.egress.reply_parser_view_keys.includes(banned),
        false,
        `reply_parser view leaked ${banned}`
      );
    }

    // ---- Alert State Machine (Phase 2C) ----
    assert.equal(report.state_machine.initial_state, 'detected');
    assert.equal(report.state_machine.terminal_state, 'snoozed');
    // 7 states in the path → 6 transitions written.
    assert.equal(report.state_machine.transition_records_written, 6);
    assert.deepEqual(
      [...report.state_machine.path],
      ['detected', 'ranked', 'queued_for_review', 'approved', 'sent', 'replied', 'snoozed']
    );

    // ---- Feedback Events (Phase 2C) ----
    assert.equal(report.feedback.events_written, 2);
    assert.equal(report.feedback.approved_count, 1);
    assert.equal(report.feedback.snoozed_count, 1);

    // ---- Memory Signals (Phase 2C) ----
    assert.equal(report.memory.signals_written, 2);
    assert.equal(report.memory.sender_importance_value, 'high');
    assert.equal(report.memory.quietness_max_per_day, 5);

    // ---- Model Router (Phase 2D) via mock backend ----
    assert.equal(report.model.capability, 'classification');
    assert.equal(report.model.model_name, 'mock-classifier-tiny');
    assert.equal(report.model.output_label, 'important');
    assert.equal(report.model.schema_valid, true);

    // ---- Cost Tracking (Phase 2D) ----
    // One model call → one cost record, with non-zero estimated_cost_usd.
    assert.equal(report.cost.records_written, 1);
    assert.ok(report.cost.total_usd > 0, `expected non-zero cost, got ${report.cost.total_usd}`);

    // ---- Tool Invocations (Phase 2E) ----
    // One invocation per gate decision = 7.
    assert.equal(report.tool_invocations.entries_written, 7);

    // ---- Audit Log (Phase 2A) ----
    // The harness performs zero audit-worthy lifecycle actions.
    // Phase 3 flows (consent grants, OAuth connects) will move this above 0.
    assert.equal(report.audit.entries_written, 0);

    // ---- Store factory (Phase 2E) ----
    // CI has no DATABASE_URL → in-memory selection.
    assert.equal(report.store_backend, 'in_memory');
  });

  it('the report is frozen — callers cannot mutate it', async () => {
    const report = await runKernelIntegrationScenario({ env: {} });
    assert.throws(() => {
      (report as unknown as { store_backend: string }).store_backend = 'mutated';
    });
    assert.throws(() => {
      (report.egress as unknown as { slack_sender_masked: string }).slack_sender_masked = 'mutated';
    });
  });

  it('two runs are independent — each constructs its own in-memory stores', async () => {
    const a = await runKernelIntegrationScenario({ env: {} });
    const b = await runKernelIntegrationScenario({ env: {} });
    // Both runs report the same counts; neither sees the other's writes.
    assert.equal(a.feedback.events_written, 2);
    assert.equal(b.feedback.events_written, 2);
    assert.equal(a.tool_invocations.entries_written, 7);
    assert.equal(b.tool_invocations.entries_written, 7);
  });
});

/* ====================================================================== *
 * Permission Gate honest-semantics — supplementary assertions outside    *
 * the scenario. These prove the gate's allow path is reachable when      *
 * Phase 3 flips executor_status to 'implemented', without actually       *
 * wiring any executor in Phase 2.                                        *
 * ====================================================================== */

function withImplemented(...toolIds: ToolId[]): PolicyGateDeps {
  const baseRegistry = createToolRegistry();
  const set = new Set<ToolId>(toolIds);
  const registry: ToolRegistry = {
    ...baseRegistry,
    getTool(id: string) {
      const t = baseRegistry.getTool(id);
      if (!t) return null;
      if (set.has(t.id)) {
        return Object.freeze({ ...t, executor_status: 'implemented' as const });
      }
      return t;
    }
  };
  return {
    registry,
    switches: SAFE_DEFAULT_KILL_SWITCHES,
    hasConsent: () => true,
    hasOAuth: () => true
  };
}

describe('Kernel Integration Gate — Permission Gate honest semantics', () => {
  it('declared external tool (gmail.read) denies not_implemented', () => {
    const d = decidePolicy(
      { tool_id: 'gmail.read', user_id: 'u1' },
      {
        registry: createToolRegistry(),
        switches: SAFE_DEFAULT_KILL_SWITCHES,
        hasConsent: () => true,
        hasOAuth: () => true
      }
    );
    assert.equal(d.code, 'not_implemented');
  });

  it('declared internal capability (audit.write) denies not_implemented (no surface bypass)', () => {
    const d = decidePolicy(
      { tool_id: 'audit.write', user_id: 'u1' },
      {
        registry: createToolRegistry(),
        switches: SAFE_DEFAULT_KILL_SWITCHES,
        hasConsent: () => true,
        hasOAuth: () => true
      }
    );
    assert.equal(d.code, 'not_implemented');
  });

  it('implemented internal capability (audit.write) → allow when policy passes', () => {
    const d = decidePolicy(
      { tool_id: 'audit.write', user_id: 'u1' },
      withImplemented('audit.write')
    );
    assert.equal(d.allowed, true);
    assert.equal(d.code, 'allowed');
  });

  it('implemented external send tool (sendblue) under default kill switches → send_disabled', () => {
    const d = decidePolicy(
      { tool_id: 'sendblue.send_user_message', user_id: 'u1' },
      withImplemented('sendblue.send_user_message')
    );
    assert.equal(d.code, 'send_disabled');
  });

  it('implemented external send tool + FOMO_SEND_ENABLED=true + manual_send → allow', () => {
    const deps = withImplemented('sendblue.send_user_message');
    const d = decidePolicy(
      { tool_id: 'sendblue.send_user_message', user_id: 'u1', intent: 'manual_send' },
      { ...deps, switches: loadKillSwitches({ FOMO_SEND_ENABLED: 'true' }) }
    );
    assert.equal(d.allowed, true);
  });

  it('implemented gmail.read + no consent → consent_missing', () => {
    const d = decidePolicy(
      { tool_id: 'gmail.read', user_id: 'u1' },
      { ...withImplemented('gmail.read'), hasConsent: () => false }
    );
    assert.equal(d.code, 'consent_missing');
  });

  it('implemented gmail.read + consent + no oauth → oauth_not_connected', () => {
    const d = decidePolicy(
      { tool_id: 'gmail.read', user_id: 'u1' },
      { ...withImplemented('gmail.read'), hasConsent: () => true, hasOAuth: () => false }
    );
    assert.equal(d.code, 'oauth_not_connected');
  });

  it('off-registry tool denies unknown_tool — checked before everything else', () => {
    const d = decidePolicy(
      { tool_id: 'booking.flights', user_id: 'u1' },
      withImplemented() // implemented set is empty
    );
    assert.equal(d.code, 'unknown_tool');
  });
});
