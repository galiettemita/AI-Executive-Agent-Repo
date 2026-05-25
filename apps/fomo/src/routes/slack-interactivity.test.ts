import assert from 'node:assert/strict';
import { createHmac } from 'node:crypto';
import { describe, it } from 'node:test';

import { SlackClient, type SlackPostResult } from '../adapters/slack/client.ts';
import { InMemoryAuditStore, type AuditEntry } from '../core/audit.ts';
import { InMemoryAlertStateTransitionStore } from '../core/alert-state-transitions.ts';
import { SAFE_DEFAULT_KILL_SWITCHES, type KillSwitches } from '../core/kill-switches.ts';
import { type RawEmailContext } from '../core/egress-policy.ts';
import { InMemoryAlertStore } from '../memory/alerts.ts';
import { InMemoryFeedbackStore } from '../memory/feedback-events.ts';
import { InMemoryRankResultStore } from '../memory/rank-results.ts';

import { handleSlackInteractivity, type SlackInteractivityRouteDeps } from './slack-interactivity.ts';

const SIGNING_SECRET = 'test_secret_xxxxxxxxxxxxxxxxxxxxxxxxx';
const CHANNEL_ID = 'C_FOUNDER_REVIEW';
const FOUNDER_USER_ID = 'U_FOUNDER';
const NOW_SECONDS = 1748054400;

function signedRequest(opts: {
  body: string;
  secret?: string;
  timestamp?: number;
}): { body: string; timestamp: string; signature: string } {
  const ts = String(opts.timestamp ?? NOW_SECONDS);
  const secret = opts.secret ?? SIGNING_SECRET;
  const base = `v0:${ts}:${opts.body}`;
  const hex = createHmac('sha256', secret).update(base).digest('hex');
  return { body: opts.body, timestamp: ts, signature: `v0=${hex}` };
}

function payloadBody(opts: {
  alert_id?: string;
  action_id?: 'fomo.approve' | 'fomo.reject';
  user_id?: string;
  channel_id?: string;
  message_ts?: string;
  type?: string;
  withBlockId?: boolean;
}): string {
  const action_id = opts.action_id ?? 'fomo.approve';
  const alert_id = opts.alert_id ?? 'alert-test-1';
  const useBlockId = opts.withBlockId !== false; // default true
  const payload = {
    type: opts.type ?? 'block_actions',
    user: { id: opts.user_id ?? FOUNDER_USER_ID, username: 'founder' },
    channel: { id: opts.channel_id ?? CHANNEL_ID, name: 'fomo-review' },
    actions: [
      {
        type: 'button',
        action_id,
        block_id: useBlockId ? `fomo_alert:${alert_id}` : '',
        value: alert_id
      }
    ],
    message: { ts: opts.message_ts ?? '1748054400.000100' },
    container: { message_ts: opts.message_ts ?? '1748054400.000100', channel_id: opts.channel_id ?? CHANNEL_ID }
  };
  const encoded = encodeURIComponent(JSON.stringify(payload));
  return `payload=${encoded}`;
}

interface Harness {
  deps: SlackInteractivityRouteDeps;
  auditStore: InMemoryAuditStore;
  alertStore: InMemoryAlertStore;
  rankResultStore: InMemoryRankResultStore;
  transitions: InMemoryAlertStateTransitionStore;
  feedbackStore: InMemoryFeedbackStore;
  slackUpdates: Array<{ ts: string; channel: string; alert_id: string; decision: string }>;
}

const SYNTHETIC_RAW: RawEmailContext = Object.freeze({
  message_id: 'msg-1',
  thread_id: 'thr-1',
  sender_email: 'counselor@school.edu',
  sender_name: 'Counselor',
  subject: 'Reminder',
  body_plain: 'Reminder body.',
  received_at: new Date(0)
});

function makeHarness(opts: {
  switches?: KillSwitches;
  founderUserId?: string | null;
  withResolver?: boolean;
  preSeed?: {
    rank_result_id?: number;
    alert_state?: 'detected' | 'ranked' | 'queued_for_review' | 'approved' | 'rejected' | 'failed';
  };
} = {}): Harness {
  const auditStore = new InMemoryAuditStore();
  const alertStore = new InMemoryAlertStore();
  const rankResultStore = new InMemoryRankResultStore();
  const transitions = new InMemoryAlertStateTransitionStore();
  const feedbackStore = new InMemoryFeedbackStore();
  const slackUpdates: Array<{ ts: string; channel: string; alert_id: string; decision: string }> = [];

  const stubSlackClient = {
    channel: () => CHANNEL_ID,
    postFounderReviewCard: async (): Promise<SlackPostResult> => {
      throw new Error('not used in this test');
    },
    updateFounderReviewCard: async (input: {
      ts: string;
      channel: string;
      alert_id: string;
      decision: { kind: 'approved' | 'rejected' };
    }): Promise<SlackPostResult> => {
      slackUpdates.push({
        ts: input.ts,
        channel: input.channel,
        alert_id: input.alert_id,
        decision: input.decision.kind
      });
      return Object.freeze({ ts: input.ts, channel: input.channel });
    }
  };

  const deps: SlackInteractivityRouteDeps = {
    signingSecret: SIGNING_SECRET,
    founderChannelId: CHANNEL_ID,
    founderUserId: opts.founderUserId === null ? undefined : opts.founderUserId ?? FOUNDER_USER_ID,
    killSwitches: opts.switches ?? Object.freeze({ ...SAFE_DEFAULT_KILL_SWITCHES, slack_review_enabled: true }),
    alertStore,
    rankResultStore,
    transitions,
    feedbackStore,
    auditStore,
    slackClient: stubSlackClient as unknown as SlackClient,
    resolveEmailContext: opts.withResolver === false ? undefined : async () => SYNTHETIC_RAW,
    now: () => new Date(NOW_SECONDS * 1000)
  };

  // Pre-seed alert + rank_result for the typical happy path test.
  if (opts.preSeed !== undefined || opts.preSeed === undefined) {
    void (async () => {
      const rankResultId = opts.preSeed?.rank_result_id ?? 1;
      await rankResultStore.write({
        user_id: 'founder',
        message_id: 'msg-1',
        invocation_id: 'inv-1',
        model_name: 'gpt-5-mini',
        prompt_version: 'ranker-v0.1.0',
        label: 'important',
        score: 0.85,
        reason: 'deadline',
        latency_ms: 400,
        input_tokens: 380,
        output_tokens: 24,
        estimated_cost_usd: 0.0009
      });
      await alertStore.create({
        alert_id: 'alert-test-1',
        user_id: 'founder',
        message_id: 'msg-1',
        rank_result_id: rankResultId,
        label: 'important',
        score: 0.85
      });
      // State machine: detected → ranked → queued_for_review (mirrors what
      // the worker does in 3D.1).
      const state = opts.preSeed?.alert_state ?? 'queued_for_review';
      if (state !== 'detected') {
        await transitions.write({
          alert_id: 'alert-test-1',
          user_id: 'founder',
          from_state: 'detected',
          to_state: 'ranked',
          reason: 'pre-seed'
        });
      }
      if (state === 'queued_for_review' || state === 'approved' || state === 'rejected') {
        await transitions.write({
          alert_id: 'alert-test-1',
          user_id: 'founder',
          from_state: 'ranked',
          to_state: 'queued_for_review',
          reason: 'pre-seed'
        });
      }
      if (state === 'approved') {
        await transitions.write({
          alert_id: 'alert-test-1',
          user_id: 'founder',
          from_state: 'queued_for_review',
          to_state: 'approved',
          reason: 'pre-seed'
        });
      } else if (state === 'rejected') {
        await transitions.write({
          alert_id: 'alert-test-1',
          user_id: 'founder',
          from_state: 'queued_for_review',
          to_state: 'rejected',
          reason: 'pre-seed'
        });
      }
    })();
  }

  return { deps, auditStore, alertStore, rankResultStore, transitions, feedbackStore, slackUpdates };
}

async function readAudit(store: InMemoryAuditStore): Promise<readonly AuditEntry[]> {
  // recent() is user-scoped; system-actor entries (actor_user_id=null)
  // require entries[] field access. The route writes both kinds.
  return (store as unknown as { entries: AuditEntry[] }).entries;
}

/* ====================================================================== */
/* Happy path                                                              */
/* ====================================================================== */

describe('handleSlackInteractivity — approve happy path', () => {
  it('transitions queued_for_review → approved, writes feedback, audits, fires chat.update', async () => {
    const h = makeHarness();
    // Wait one microtask so pre-seed async writes settle.
    await new Promise((r) => setImmediate(r));

    const req = signedRequest({ body: payloadBody({ action_id: 'fomo.approve' }) });
    const resp = await handleSlackInteractivity(req, h.deps);

    assert.equal(resp.status, 200);
    const respBody = JSON.parse(resp.body);
    assert.equal(respBody.ok, true);
    assert.equal(respBody.decision, 'approved');

    // State transition queued_for_review → approved
    assert.equal(await h.transitions.currentState('alert-test-1'), 'approved');

    // Feedback event
    assert.equal(await h.feedbackStore.countByKind('founder', 'founder_approved'), 1);
    assert.equal(await h.feedbackStore.countByKind('founder', 'founder_rejected'), 0);

    // chat.update fired (visual feedback)
    assert.equal(h.slackUpdates.length, 1);
    assert.equal(h.slackUpdates[0]?.decision, 'approved');

    // Audit footprint
    const audit = await readAudit(h.auditStore);
    assert.ok(audit.some((e) => e.action === 'fomo.slack.interaction_received'));
    assert.ok(audit.some((e) => e.action === 'fomo.slack.approval_captured'));
    // No signature_invalid, no unauthorized
    assert.equal(audit.filter((e) => e.action === 'fomo.slack.signature_invalid').length, 0);
    assert.equal(audit.filter((e) => e.action === 'fomo.slack.approval_unauthorized').length, 0);
  });

  it('reject happy path: transitions to rejected, writes founder_rejected feedback', async () => {
    const h = makeHarness();
    await new Promise((r) => setImmediate(r));
    const req = signedRequest({ body: payloadBody({ action_id: 'fomo.reject' }) });
    const resp = await handleSlackInteractivity(req, h.deps);
    assert.equal(resp.status, 200);
    assert.equal(await h.transitions.currentState('alert-test-1'), 'rejected');
    assert.equal(await h.feedbackStore.countByKind('founder', 'founder_rejected'), 1);
    assert.equal(await h.feedbackStore.countByKind('founder', 'founder_approved'), 0);
  });
});

/* ====================================================================== */
/* Signature verification                                                  */
/* ====================================================================== */

describe('handleSlackInteractivity — signature verification (defense-in-depth)', () => {
  it('rejects requests with an invalid signature → 401 + audit signature_invalid', async () => {
    const h = makeHarness();
    await new Promise((r) => setImmediate(r));
    const bad = signedRequest({ body: payloadBody({}), secret: 'WRONG_SECRET' });
    const resp = await handleSlackInteractivity(bad, h.deps);
    assert.equal(resp.status, 401);
    const audit = await readAudit(h.auditStore);
    assert.ok(audit.some((e) => e.action === 'fomo.slack.signature_invalid'));
    // No state change, no feedback, no chat.update
    assert.equal(await h.transitions.currentState('alert-test-1'), 'queued_for_review');
    assert.equal(await h.feedbackStore.countByKind('founder', 'founder_approved'), 0);
    assert.equal(h.slackUpdates.length, 0);
  });

  it('rejects stale timestamps (>300s) → 401', async () => {
    const h = makeHarness();
    await new Promise((r) => setImmediate(r));
    const stale = signedRequest({ body: payloadBody({}), timestamp: NOW_SECONDS - 301 });
    const resp = await handleSlackInteractivity(stale, h.deps);
    assert.equal(resp.status, 401);
    const audit = await readAudit(h.auditStore);
    const sig = audit.find((e) => e.action === 'fomo.slack.signature_invalid');
    assert.equal((sig?.detail as Record<string, unknown>)?.error_code, 'stale_timestamp');
  });

  it('audits interaction_received BEFORE verifying signature (every inbound logged)', async () => {
    const h = makeHarness();
    await new Promise((r) => setImmediate(r));
    const bad = signedRequest({ body: payloadBody({}), secret: 'WRONG_SECRET' });
    await handleSlackInteractivity(bad, h.deps);
    const audit = await readAudit(h.auditStore);
    // interaction_received entry appears BEFORE signature_invalid
    const received = audit.findIndex((e) => e.action === 'fomo.slack.interaction_received');
    const invalid = audit.findIndex((e) => e.action === 'fomo.slack.signature_invalid');
    assert.ok(received >= 0);
    assert.ok(invalid > received, 'interaction_received must precede signature_invalid');
  });

  it('NEVER persists the raw body in audit detail', async () => {
    const h = makeHarness();
    await new Promise((r) => setImmediate(r));
    const req = signedRequest({ body: payloadBody({}) });
    await handleSlackInteractivity(req, h.deps);
    const audit = await readAudit(h.auditStore);
    for (const e of audit) {
      const serialized = JSON.stringify(e.detail ?? {});
      assert.ok(!serialized.includes('payload='), `audit detail must not contain raw payload`);
      assert.ok(!serialized.includes('block_actions'), `audit detail must not contain payload-type leak`);
    }
  });
});

/* ====================================================================== */
/* Authorization (channel + user)                                          */
/* ====================================================================== */

describe('handleSlackInteractivity — channel + user authorization', () => {
  it('rejects requests from the wrong channel → 403 + audit approval_unauthorized', async () => {
    const h = makeHarness();
    await new Promise((r) => setImmediate(r));
    const req = signedRequest({ body: payloadBody({ channel_id: 'C_WRONG' }) });
    const resp = await handleSlackInteractivity(req, h.deps);
    assert.equal(resp.status, 403);
    const audit = await readAudit(h.auditStore);
    const unauthorized = audit.find((e) => e.action === 'fomo.slack.approval_unauthorized');
    assert.ok(unauthorized);
    assert.equal((unauthorized?.detail as Record<string, unknown>)?.error_code, 'wrong_channel');
    // No state change
    assert.equal(await h.transitions.currentState('alert-test-1'), 'queued_for_review');
  });

  it('rejects requests from the wrong user → 403 (when SLACK_FOUNDER_USER_ID is set)', async () => {
    const h = makeHarness({ founderUserId: FOUNDER_USER_ID });
    await new Promise((r) => setImmediate(r));
    const req = signedRequest({ body: payloadBody({ user_id: 'U_INTRUDER' }) });
    const resp = await handleSlackInteractivity(req, h.deps);
    assert.equal(resp.status, 403);
    const audit = await readAudit(h.auditStore);
    const unauthorized = audit.find((e) => e.action === 'fomo.slack.approval_unauthorized');
    assert.equal((unauthorized?.detail as Record<string, unknown>)?.error_code, 'wrong_user');
  });

  it('skips user check when founderUserId is undefined (best-effort mode)', async () => {
    const h = makeHarness({ founderUserId: null });
    await new Promise((r) => setImmediate(r));
    const req = signedRequest({ body: payloadBody({ user_id: 'U_RANDO' }) });
    const resp = await handleSlackInteractivity(req, h.deps);
    // No user restriction → approval succeeds
    assert.equal(resp.status, 200);
    assert.equal(await h.transitions.currentState('alert-test-1'), 'approved');
  });

  it('NEVER persists the full Slack user_id (only the 4-char slug suffix)', async () => {
    const h = makeHarness();
    await new Promise((r) => setImmediate(r));
    const req = signedRequest({ body: payloadBody({ user_id: 'U01ABCDEFGHIJ' }) });
    await handleSlackInteractivity(req, h.deps);
    const audit = await readAudit(h.auditStore);
    for (const e of audit) {
      const serialized = JSON.stringify(e.detail ?? {});
      assert.ok(
        !serialized.includes('U01ABCDEFGHIJ'),
        'audit detail must NOT contain the full Slack user_id; only a 4-char slug'
      );
    }
  });
});

/* ====================================================================== */
/* Idempotency (first-wins) — LOAD-BEARING founder directive               */
/* ====================================================================== */

describe('handleSlackInteractivity — idempotency / first-wins (Phase 3D.2 invariant)', () => {
  it('duplicate approve click on an already-approved alert: audit approval_duplicate, no state change', async () => {
    const h = makeHarness({ preSeed: { alert_state: 'approved' } });
    await new Promise((r) => setImmediate(r));
    const req = signedRequest({ body: payloadBody({ action_id: 'fomo.approve' }) });
    const resp = await handleSlackInteractivity(req, h.deps);
    assert.equal(resp.status, 200);
    const respBody = JSON.parse(resp.body);
    assert.equal(respBody.duplicate, true);
    assert.equal(respBody.current_state, 'approved');
    // State still approved, NOT re-written
    assert.equal(await h.transitions.currentState('alert-test-1'), 'approved');
    // No new feedback event from this duplicate
    assert.equal(await h.feedbackStore.countByKind('founder', 'founder_approved'), 0);
    const audit = await readAudit(h.auditStore);
    assert.ok(audit.some((e) => e.action === 'fomo.slack.approval_duplicate'));
    // chat.update should NOT fire on duplicate
    assert.equal(h.slackUpdates.length, 0);
  });

  it('reject click after approved: idempotent — first decision wins, no state regression', async () => {
    const h = makeHarness({ preSeed: { alert_state: 'approved' } });
    await new Promise((r) => setImmediate(r));
    // Click reject AFTER an approve already landed
    const req = signedRequest({ body: payloadBody({ action_id: 'fomo.reject' }) });
    const resp = await handleSlackInteractivity(req, h.deps);
    assert.equal(resp.status, 200);
    const respBody = JSON.parse(resp.body);
    assert.equal(respBody.duplicate, true);
    // State remains approved — first-wins.
    assert.equal(await h.transitions.currentState('alert-test-1'), 'approved');
  });
});

/* ====================================================================== */
/* Malformed payloads                                                      */
/* ====================================================================== */

describe('handleSlackInteractivity — malformed payloads', () => {
  it('rejects unknown alert_id → 400 + audit payload_invalid', async () => {
    const h = makeHarness();
    await new Promise((r) => setImmediate(r));
    const req = signedRequest({ body: payloadBody({ alert_id: 'alert-does-not-exist' }) });
    const resp = await handleSlackInteractivity(req, h.deps);
    assert.equal(resp.status, 400);
    const audit = await readAudit(h.auditStore);
    const invalid = audit.find((e) => e.action === 'fomo.slack.payload_invalid');
    assert.equal((invalid?.detail as Record<string, unknown>)?.error_code, 'unknown_alert_id');
  });

  it('rejects unexpected payload type → 400', async () => {
    const h = makeHarness();
    await new Promise((r) => setImmediate(r));
    const req = signedRequest({ body: payloadBody({ type: 'view_submission' }) });
    const resp = await handleSlackInteractivity(req, h.deps);
    assert.equal(resp.status, 400);
    const audit = await readAudit(h.auditStore);
    const invalid = audit.find((e) => e.action === 'fomo.slack.payload_invalid');
    assert.equal((invalid?.detail as Record<string, unknown>)?.error_code, 'unexpected_payload_type');
  });

  it('rejects unknown action_id → 400', async () => {
    const h = makeHarness();
    await new Promise((r) => setImmediate(r));
    // action_id is neither fomo.approve nor fomo.reject
    const payload = JSON.stringify({
      type: 'block_actions',
      user: { id: FOUNDER_USER_ID },
      channel: { id: CHANNEL_ID },
      actions: [{ type: 'button', action_id: 'fomo.delete', block_id: 'fomo_alert:alert-test-1', value: 'alert-test-1' }]
    });
    const body = `payload=${encodeURIComponent(payload)}`;
    const req = signedRequest({ body });
    const resp = await handleSlackInteractivity(req, h.deps);
    assert.equal(resp.status, 400);
  });
});

/* ====================================================================== */
/* Kill-switch defense-in-depth                                            */
/* ====================================================================== */

describe('handleSlackInteractivity — kill switch off (defense-in-depth)', () => {
  it('returns 200 (does NOT leak route existence) but does NOT process the request', async () => {
    const h = makeHarness({
      switches: Object.freeze({ ...SAFE_DEFAULT_KILL_SWITCHES, slack_review_enabled: false })
    });
    await new Promise((r) => setImmediate(r));
    const req = signedRequest({ body: payloadBody({}) });
    const resp = await handleSlackInteractivity(req, h.deps);
    assert.equal(resp.status, 200);
    // No state transition, no feedback, no chat.update
    assert.equal(await h.transitions.currentState('alert-test-1'), 'queued_for_review');
    assert.equal(await h.feedbackStore.countByKind('founder', 'founder_approved'), 0);
    assert.equal(h.slackUpdates.length, 0);
    // Audit shows interaction_received + signature_invalid(kill_switch_off)
    const audit = await readAudit(h.auditStore);
    const sig = audit.find((e) => e.action === 'fomo.slack.signature_invalid');
    assert.equal((sig?.detail as Record<string, unknown>)?.error_code, 'kill_switch_off');
  });
});
