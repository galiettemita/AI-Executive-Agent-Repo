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
    // Phase 3A + 3B.2 + 3D.1 + 3E.1 invariant: all six v0.1 tools are
    // 'implemented'. No tool remains 'declared' after Phase 3E.1
    // flipped sendblue.send_user_message alongside the SendBlueClient
    // adapter wireup.
    assert.equal(report.registry.declared_tool_ids.length, 0);
    assert.equal(report.registry.implemented_tool_ids.length, 6);
    assert.deepEqual(
      [...report.registry.implemented_tool_ids].sort(),
      [
        'audit.write',
        'feedback.write',
        'gmail.read',
        'memory_signal.write',
        'sendblue.send_user_message',
        'slack.founder_review'
      ]
    );
    assert.deepEqual([...report.registry.declared_tool_ids].sort(), []);

    // ---- Kill Switches (Phase 2B + 3B.2) — safe defaults all-disabled ----
    assert.equal(report.kill_switches.send_enabled, false);
    assert.equal(report.kill_switches.auto_send_enabled, false);
    assert.equal(report.kill_switches.friend_beta_enabled, false);
    assert.equal(report.kill_switches.polling_enabled, false);
    assert.equal(report.kill_switches.max_users, 1);
    assert.equal(report.kill_switches.polling_interval_ms, 60_000);

    // ---- Permission Gate → Dispatch (Phase 2B + 2B.1 + 2C.1 + 3A + 3B.2 + 3D.1 + 3E.1) ----
    // 7 explicit invocations through the loop:
    //   Section A — 2 denials at the gate (no dispatch):
    //     sendblue.send_user_message  → send_disabled (Phase 3E.1:
    //                                    now 'implemented' but the
    //                                    safe-defaults FOMO_SEND_ENABLED
    //                                    is false → the gate's
    //                                    risk_tier='send' branch denies)
    //     booking.flights             → unknown_tool
    //   Section B — 5 allows + dispatch executes:
    //     audit.write                 → executor writes 'session.created' audit
    //     feedback.write × 2          → executor writes feedback events
    //     memory_signal.write × 2     → executor upserts memory signals
    //
    // Phase 3D.1 note: slack.founder_review used to be in Section A as
    // a denied example ('declared' + send-tier). Post-3D.1 it is
    // 'implemented' and demoted to internal-tier, so the gate would
    // allow it. The harness deliberately keeps slack OUT of the
    // explicit loop — its exercise belongs to the polling-worker
    // workflow tests (workers/gmail-poll.test.ts), not the kernel.
    //
    // Plus 1 additional dispatch through the Phase 3B.2 polling worker
    // (section 3.5 of the harness): gmail.read on one new message. The
    // worker writes its own policy.decided + tool.invoked audit pair
    // under SYNTHETIC_USER_ID, plus a system-actor gmail.poll.cycle entry.
    assert.equal(report.policy_decisions.length, 7);
    const decisions = report.policy_decisions;
    // 2 denied + 5 allowed (explicit loop only).
    assert.equal(decisions.filter((d) => !d.allowed).length, 2);
    assert.equal(decisions.filter((d) => d.allowed).length, 5);
    // Each denied tool with the expected code.
    const deniedByTool = new Map(
      decisions.filter((d) => !d.allowed).map((d) => [d.tool_id, d.code])
    );
    assert.equal(deniedByTool.get('sendblue.send_user_message'), 'send_disabled');
    assert.equal(deniedByTool.get('booking.flights'), 'unknown_tool');
    // slack.founder_review removed from the explicit loop in Phase 3D.1.
    assert.equal(deniedByTool.has('slack.founder_review'), false);
    // gmail.read is no longer in the explicit loop (the polling worker
    // drives it in section 3.5).
    assert.equal(deniedByTool.has('gmail.read'), false);
    // Allowed tool decisions all carry code='allowed'.
    for (const d of decisions.filter((d) => d.allowed)) {
      assert.equal(d.code, 'allowed', `${d.tool_id} allowed but code=${d.code}`);
    }
    // Each implemented internal tool appears at least once in the allowed set.
    const allowedTools = new Set(decisions.filter((d) => d.allowed).map((d) => d.tool_id));
    assert.ok(allowedTools.has('audit.write'));
    assert.ok(allowedTools.has('feedback.write'));
    assert.ok(allowedTools.has('memory_signal.write'));

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

    // ---- Tool Invocations (Phase 2E + 3A + 3B.2 + 3D.1) ----
    // 7 from the explicit loop (2 denials + 5 dispatched; Phase 3D.1
    // removed slack.founder_review from this list) + 1 from the polling
    // worker (gmail.read on one new message) = 8.
    assert.equal(report.tool_invocations.entries_written, 8);

    // ---- Gmail polling worker (Phase 3B.2) ----
    // One cycle. One user (SYNTHETIC_USER_ID) has a seeded token + cursor.
    // The mock GmailClient returns one new message id. The worker calls
    // listHistorySince → dispatches gmail.read once → advances the
    // cursor. No failures.
    assert.equal(report.polling.users_total, 1);
    assert.equal(report.polling.users_polled, 1);
    assert.equal(report.polling.messages_observed, 1);
    assert.equal(report.polling.messages_dispatched, 1);
    assert.equal(report.polling.messages_failed, 0);
    assert.equal(report.polling.cursor_before, 'h-harness-1');
    assert.equal(report.polling.cursor_after, 'h-harness-2');

    // ---- Audit Log (Phase 2A + Phase 2F.1 + Phase 3A + Phase 3B.2) ----
    // The harness writes an audit entry at every meaningful kernel touch.
    // Required-count breakdown (must hold for the Kernel Integration Gate):
    //   policy.decided      9  (3 denied + 5 allowed from explicit loop,
    //                            + 1 from polling worker's gmail.read dispatch)
    //   tool.invoked        9  (one per tool_invocations write)
    //   session.created     1  (from the dispatched audit.write executor —
    //                            this IS the domain audit; no extra audit
    //                            wrapper around it, which is the
    //                            "no-recursive-audit" invariant)
    //   state.transitioned  6  (one per state machine transition)
    //   feedback.written    2  (founder_approved + user_snoozed)
    //   memory.upserted     2  (sender_importance + quietness_preference)
    //   model.routed        1  (single classification call)
    //                      —
    //                      30  total (under SYNTHETIC_USER_ID)
    //
    // Note: the polling worker also writes a 'gmail.poll.cycle' aggregate
    // entry with actor_user_id=null (system actor). audit.recent() is
    // user-scoped so that entry does NOT count toward entries_written
    // below. The worker's own unit tests assert the cycle entry's shape.
    assert.ok(report.audit.entries_written > 0, 'audit log must participate in the kernel path');
    // Phase 3D.1: removing slack from the explicit invocations list
    // dropped 2 audit entries (one policy.decided + one tool.invoked),
    // bringing the total from 30 to 28.
    assert.equal(report.audit.entries_written, 28);
    // Each required audit category must be exercised at least once.
    for (const requiredAction of [
      'policy.decided',
      'tool.invoked',
      'state.transitioned',
      'feedback.written',
      'memory.upserted',
      'model.routed'
    ]) {
      assert.ok(
        (report.audit.by_action[requiredAction] ?? 0) > 0,
        `audit log missing required action: ${requiredAction}`
      );
    }
    // Phase 3D.1 dropped slack from the explicit invocations list,
    // so policy.decided + tool.invoked each lost one entry (9 → 8).
    assert.equal(report.audit.by_action['policy.decided'], 8);
    assert.equal(report.audit.by_action['tool.invoked'], 8);
    assert.equal(report.audit.by_action['session.created'], 1);
    assert.equal(report.audit.by_action['state.transitioned'], 6);
    assert.equal(report.audit.by_action['feedback.written'], 2);
    assert.equal(report.audit.by_action['memory.upserted'], 2);
    assert.equal(report.audit.by_action['model.routed'], 1);
    // PRIVACY: audit entries must not carry raw email body, raw headers,
    // attachment filenames, prompt text, or full user reply text. The
    // harness intentionally passes a known-recognizable reply text and
    // prompt through the substrate so any leak into audit would surface
    // as a canary string here.
    assert.deepEqual(
      [...report.audit.forbidden_leaks],
      [],
      `audit log leaked forbidden content: ${report.audit.forbidden_leaks.join(', ')}`
    );

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
    assert.equal(a.tool_invocations.entries_written, 8);
    assert.equal(b.tool_invocations.entries_written, 8);
    assert.equal(a.audit.entries_written, 28);
    assert.equal(b.audit.entries_written, 28);
    // Polling cursor advance is independent per run.
    assert.equal(a.polling.cursor_after, 'h-harness-2');
    assert.equal(b.polling.cursor_after, 'h-harness-2');
    assert.equal(a.polling.messages_dispatched, 1);
    assert.equal(b.polling.messages_dispatched, 1);
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
  it('implemented external tool (gmail.read) + consent + oauth → allowed (Phase 3B.2 flip)', () => {
    // Phase 3B.2 replaced the "gmail.read denies not_implemented" gate
    // assertion with this allow-path proof. The substrate flip is the
    // load-bearing change: gate hits consent/oauth checks, not the
    // declared short-circuit.
    const d = decidePolicy(
      { tool_id: 'gmail.read', user_id: 'u1' },
      {
        registry: createToolRegistry(),
        switches: SAFE_DEFAULT_KILL_SWITCHES,
        hasConsent: () => true,
        hasOAuth: () => true
      }
    );
    assert.equal(d.allowed, true);
    assert.equal(d.code, 'allowed');
  });

  it('implemented external send tool (sendblue.send_user_message) denies send_disabled under safe defaults (Phase 3E.1)', () => {
    // Phase 3E.1 flipped sendblue to 'implemented'. Under safe defaults
    // (FOMO_SEND_ENABLED=false) the gate denies at the risk_tier='send'
    // branch — not at not_implemented. This is the action-boundary
    // kill-switch enforcement; bootstrap fail-closed is the matching
    // build-time layer.
    const d = decidePolicy(
      { tool_id: 'sendblue.send_user_message', user_id: 'u1' },
      {
        registry: createToolRegistry(),
        switches: SAFE_DEFAULT_KILL_SWITCHES,
        hasConsent: () => true,
        hasOAuth: () => true
      }
    );
    assert.equal(d.allowed, false);
    assert.equal(d.code, 'send_disabled');
  });

  it('implemented internal-tier tool (slack.founder_review) DENIES slack_review_disabled under safe-defaults (Phase 3D.1)', () => {
    // Phase 3D.1: the slack-review kill switch is enforced at the
    // policy gate too (defense-in-depth, not only at bootstrap).
    // Safe-defaults have slack_review_enabled=false → deny.
    const d = decidePolicy(
      { tool_id: 'slack.founder_review', user_id: 'u1' },
      {
        registry: createToolRegistry(),
        switches: SAFE_DEFAULT_KILL_SWITCHES,
        hasConsent: () => true,
        hasOAuth: () => true
      }
    );
    assert.equal(d.allowed, false);
    assert.equal(d.code, 'slack_review_disabled');
  });

  it('implemented internal-tier tool (slack.founder_review) ALLOWS when FOMO_SLACK_REVIEW_ENABLED=true (Phase 3D.1)', () => {
    const d = decidePolicy(
      { tool_id: 'slack.founder_review', user_id: 'u1' },
      {
        registry: createToolRegistry(),
        switches: Object.freeze({ ...SAFE_DEFAULT_KILL_SWITCHES, slack_review_enabled: true }),
        hasConsent: () => true,
        hasOAuth: () => true
      }
    );
    assert.equal(d.allowed, true);
    assert.equal(d.code, 'allowed');
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
