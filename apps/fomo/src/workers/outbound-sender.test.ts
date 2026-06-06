import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { SendBlueClient } from '../adapters/sendblue/client.ts';
import {
  InMemoryAlertStateTransitionStore
} from '../core/alert-state-transitions.ts';
import { InMemoryAuditStore } from '../core/audit.ts';
import { SAFE_DEFAULT_KILL_SWITCHES, loadKillSwitches } from '../core/kill-switches.ts';
import { type PolicyGateDeps } from '../core/policy-gate.ts';
import { createToolRegistry, type ToolId, type ToolRegistry } from '../core/tool-registry.ts';
import { InMemoryToolInvocationStore } from '../core/tool-invocations.ts';
import { createDispatchTable } from '../dispatch/dispatcher.ts';
import { wireExternalExecutors } from '../dispatch/external-executors.ts';
import { GmailClient, GMAIL_READONLY_SCOPE } from '../adapters/gmail/client.ts';
import { loadCryptoConfig } from '../security/token-crypto.ts';
import { InMemoryTokenStore } from '../security/oauth/token-store.ts';
import { InMemoryAlertStore } from '../memory/alerts.ts';
import { InMemoryGmailCursorStore } from '../memory/gmail-cursors.ts';
import { InMemoryMemorySignalStore } from '../memory/memory-signals.ts';
import { InMemoryRankResultStore } from '../memory/rank-results.ts';
import { runOutboundOnce } from './outbound-sender.ts';

const TEST_KEK = Buffer.alloc(32, 9).toString('base64');
const FOUNDER_USER = 'founder-1';
const FOUNDER_PHONE = '+14155551234';
const FOUNDER_PHONE_SLUG = '1234';
const ALERT_ID = 'alert-1';
const MESSAGE_ID = 'msg-abc';

function withEnv<T>(env: Record<string, string | undefined>, fn: () => T): T {
  const previous: Record<string, string | undefined> = {};
  for (const [k, v] of Object.entries(env)) {
    previous[k] = process.env[k];
    if (v === undefined) delete process.env[k];
    else process.env[k] = v;
  }
  try {
    return fn();
  } finally {
    for (const [k, v] of Object.entries(previous)) {
      if (v === undefined) delete process.env[k];
      else process.env[k] = v;
    }
  }
}

const cryptoConfig = withEnv(
  { BREVIO_TOKEN_KEK: TEST_KEK, BREVIO_DEV_MODE: undefined },
  () => loadCryptoConfig()
);

// Helper: builds a registry where the specified tools are forced to
// 'implemented' for the purposes of the gate (we still want real
// dispatch wiring, but the gate must allow).
function makeRegistry(implementedOverrides: readonly ToolId[] = []): ToolRegistry {
  const base = createToolRegistry();
  if (implementedOverrides.length === 0) return base;
  const set = new Set<ToolId>(implementedOverrides);
  return {
    ...base,
    getTool(id: string) {
      const t = base.getTool(id);
      if (!t) return null;
      if (set.has(t.id)) {
        return Object.freeze({ ...t, executor_status: 'implemented' as const });
      }
      return t;
    }
  };
}

interface HarnessOptions {
  // When false, the test omits the founder phone from destinationFor
  // to exercise the unauthorized-destination path.
  readonly destinationConfigured?: boolean;
  // Inject the SendBlue fetch mock.
  readonly sendBlueFetch: typeof fetch;
  // Inject the Gmail fetch mock.
  readonly gmailFetch: typeof fetch;
  // Kill switches (defaults: ranker off, send on, slack off, polling off).
  readonly switches?: ReturnType<typeof loadKillSwitches>;
  // v0.5.6: override the seeded rank_results.reason. Defaults to
  // 'deposit due tonight'. Used by v0.5.6 fallback-path tests to seed
  // a too-long or empty reason and assert
  // fomo.alert.drafter_schema_failed is written.
  readonly reasonOverride?: string;
}

async function buildHarness(opts: HarnessOptions) {
  const auditStore = new InMemoryAuditStore();
  const toolInvocationStore = new InMemoryToolInvocationStore();
  const transitions = new InMemoryAlertStateTransitionStore();
  const alertStore = new InMemoryAlertStore();
  const rankResultStore = new InMemoryRankResultStore();
  const cursorStore = new InMemoryGmailCursorStore();
  const memoryStore = new InMemoryMemorySignalStore();
  const tokenStore = new InMemoryTokenStore(cryptoConfig);

  // Seed the founder's google token + cursor so listUserIds returns them.
  await tokenStore.save({
    user_id: FOUNDER_USER,
    provider: 'google',
    scopes: [GMAIL_READONLY_SCOPE],
    access_token: 'real-token'
  });
  await cursorStore.upsert({ user_id: FOUNDER_USER, history_id: 'h1' });

  // Seed the rank_results row. v0.5.6: reason is overridable so the
  // fallback-path tests can seed an empty or too-long reason.
  const writeOutcome = await rankResultStore.write({
    user_id: FOUNDER_USER,
    message_id: MESSAGE_ID,
    invocation_id: 'rank-inv-1',
    model_name: 'gpt-5-mini',
    prompt_version: 'ranker-v0.1.0',
    label: 'important',
    score: 0.92,
    reason: opts.reasonOverride ?? 'deposit due tonight',
    latency_ms: 100,
    input_tokens: 50,
    output_tokens: 20,
    estimated_cost_usd: 0.0001
  });
  const rankResultId = writeOutcome.rank_result_id;

  // Seed the alert + walk it to 'approved'.
  await alertStore.create({
    alert_id: ALERT_ID,
    user_id: FOUNDER_USER,
    message_id: MESSAGE_ID,
    rank_result_id: rankResultId,
    label: 'important',
    score: 0.92
  });
  for (const [from, to] of [
    ['detected', 'ranked'],
    ['ranked', 'queued_for_review'],
    ['queued_for_review', 'approved']
  ] as const) {
    await transitions.write({
      alert_id: ALERT_ID,
      user_id: FOUNDER_USER,
      from_state: from,
      to_state: to,
      reason: 'test-setup'
    });
  }

  const gmailClient = new GmailClient({ fetchImpl: opts.gmailFetch });
  const sendBlueClient = new SendBlueClient({
    apiKeyId: 'k',
    apiSecretKey: 's',
    fromNumber: '+15555550001',
    fetchImpl: opts.sendBlueFetch
  });
  const dispatch = createDispatchTable();
  wireExternalExecutors(dispatch, { gmailClient, tokenStore, sendBlueClient });

  // Force gate to ALLOW gmail.read (override hasConsent + hasOAuth) AND
  // sendblue (FOMO_SEND_ENABLED=true).
  const switches =
    opts.switches ??
    loadKillSwitches({ FOMO_SEND_ENABLED: 'true' });
  const gateDeps: PolicyGateDeps = {
    registry: makeRegistry(),
    switches,
    hasConsent: () => true,
    hasOAuth: () => true
  };

  return {
    auditStore,
    toolInvocationStore,
    transitions,
    alertStore,
    rankResultStore,
    cursorStore,
    memoryStore,
    tokenStore,
    rankResultId,
    deps: {
      dispatch,
      auditStore,
      toolInvocationStore,
      gateDeps,
      cursorStore,
      alertStore,
      rankResultStore,
      transitions,
      memoryStore,
      destinationFor: (uid: string): string | null => {
        if (opts.destinationConfigured === false) return null;
        return uid === FOUNDER_USER ? FOUNDER_PHONE : null;
      }
    }
  };
}

// Minimal Gmail messages.get JSON; base64url-decodes to 'Hi Sarah'.
const FAKE_GMAIL_MSG = {
  id: MESSAGE_ID,
  threadId: 'thr-1',
  internalDate: '1700000000000',
  payload: {
    headers: [
      { name: 'From', value: 'Sarah <sarah@example.com>' },
      { name: 'Subject', value: 'lunch?' }
    ],
    mimeType: 'text/plain',
    body: { data: 'SGkgU2FyYWg' }
  }
};

function mockFetch(
  handler: (url: string, init: RequestInit) => Promise<{ status: number; body: unknown }>
): typeof fetch {
  return (async (input: string | URL | Request, init?: RequestInit) => {
    const url = typeof input === 'string' ? input : input.toString();
    const result = await handler(url, init ?? {});
    return new Response(JSON.stringify(result.body), {
      status: result.status,
      headers: { 'content-type': 'application/json' }
    });
  }) as typeof fetch;
}

describe('runOutboundOnce — happy path (sent)', () => {
  it('finds approved alert, dispatches sendblue, transitions to sent', async () => {
    const sendBlueFetch = mockFetch(async () => ({
      status: 200,
      body: { status: 'QUEUED', message_handle: 'mh-1' }
    }));
    const gmailFetch = mockFetch(async () => ({ status: 200, body: FAKE_GMAIL_MSG }));
    const h = await buildHarness({ sendBlueFetch, gmailFetch });

    const report = await runOutboundOnce(h.deps);

    assert.equal(report.alerts_considered, 1);
    assert.equal(report.alerts_sent, 1);
    assert.equal(report.alerts_failed, 0);
    assert.equal(report.alerts_status_unknown, 0);
    assert.equal(report.alerts_unauthorized, 0);

    // State machine reached 'sent'.
    assert.equal(await h.transitions.currentState(ALERT_ID), 'sent');

    // Audit footprint includes fomo.send.succeeded with the destination_slug
    // (not the full phone) and template_version.
    const events = await h.auditStore.recent(FOUNDER_USER, 500);
    const succeeded = events.find((e) => e.action === 'fomo.send.succeeded');
    assert.ok(succeeded);
    assert.equal((succeeded?.detail as { destination_slug: string }).destination_slug, FOUNDER_PHONE_SLUG);
    // v0.5.7 Q6.A: template_version renamed from 'founder-text-vN' to
    // 'human-message-vN' to surface the HMR product principle in audit.
    assert.match(
      (succeeded?.detail as { template_version: string }).template_version,
      /^(human-message|founder-text)-v/
    );

    // fomo.send.attempted fired BEFORE the send.
    const attemptedIndex = events.findIndex((e) => e.action === 'fomo.send.attempted');
    const succeededIndex = events.findIndex((e) => e.action === 'fomo.send.succeeded');
    assert.ok(attemptedIndex >= 0 && succeededIndex >= 0);
    // recent() returns newest-first, so attempted (earlier) has the LARGER index.
    assert.ok(attemptedIndex > succeededIndex);
  });

  it('does NOT leak the full destination phone number in audit detail', async () => {
    const sendBlueFetch = mockFetch(async () => ({
      status: 200,
      body: { status: 'SENT', message_handle: 'mh-2' }
    }));
    const gmailFetch = mockFetch(async () => ({ status: 200, body: FAKE_GMAIL_MSG }));
    const h = await buildHarness({ sendBlueFetch, gmailFetch });
    await runOutboundOnce(h.deps);

    const events = await h.auditStore.recent(FOUNDER_USER, 500);
    for (const e of events) {
      const detailStr = JSON.stringify(e.detail);
      assert.ok(
        !detailStr.includes(FOUNDER_PHONE),
        `audit leaked the full founder phone: ${detailStr}`
      );
    }
  });
});

describe('runOutboundOnce — idempotency (load-bearing)', () => {
  it('second cycle does NOT re-send a successfully sent alert', async () => {
    const sendBlueFetch = mockFetch(async () => ({
      status: 200,
      body: { status: 'QUEUED', message_handle: 'mh-1' }
    }));
    const gmailFetch = mockFetch(async () => ({ status: 200, body: FAKE_GMAIL_MSG }));
    const h = await buildHarness({ sendBlueFetch, gmailFetch });

    const r1 = await runOutboundOnce(h.deps);
    assert.equal(r1.alerts_sent, 1);

    const r2 = await runOutboundOnce(h.deps);
    assert.equal(r2.alerts_considered, 0, 'second cycle must not find the same alert');
    assert.equal(r2.alerts_sent, 0);
  });

  it('does NOT re-send an alert in send_status_unknown (never auto-retry)', async () => {
    // First call returns ambiguous (HTTP 500); second call would return
    // success if the worker auto-retried. Confirm it does NOT.
    let callCount = 0;
    const sendBlueFetch = mockFetch(async () => {
      callCount++;
      if (callCount === 1) return { status: 500, body: { error: 'internal' } };
      return { status: 200, body: { status: 'QUEUED', message_handle: 'mh-1' } };
    });
    const gmailFetch = mockFetch(async () => ({ status: 200, body: FAKE_GMAIL_MSG }));
    const h = await buildHarness({ sendBlueFetch, gmailFetch });

    const r1 = await runOutboundOnce(h.deps);
    assert.equal(r1.alerts_status_unknown, 1);
    assert.equal(await h.transitions.currentState(ALERT_ID), 'send_status_unknown');

    const r2 = await runOutboundOnce(h.deps);
    assert.equal(r2.alerts_considered, 0, 'send_status_unknown alert must NOT be re-found');
    assert.equal(callCount, 1, 'SendBlue must not have been called a second time');
  });

  it('does NOT re-send an alert in failed state', async () => {
    const sendBlueFetch = mockFetch(async () => ({
      status: 200,
      body: { status: 'FAILED', error: 'invalid_number' }
    }));
    const gmailFetch = mockFetch(async () => ({ status: 200, body: FAKE_GMAIL_MSG }));
    const h = await buildHarness({ sendBlueFetch, gmailFetch });

    const r1 = await runOutboundOnce(h.deps);
    assert.equal(r1.alerts_failed, 1);
    assert.equal(await h.transitions.currentState(ALERT_ID), 'failed');

    const r2 = await runOutboundOnce(h.deps);
    assert.equal(r2.alerts_considered, 0);
  });
});

describe('runOutboundOnce — three-outcome state-machine transitions', () => {
  it('clear failure → approved → failed', async () => {
    const sendBlueFetch = mockFetch(async () => ({
      status: 200,
      body: { status: 'FAILED', error: 'spam_filter' }
    }));
    const gmailFetch = mockFetch(async () => ({ status: 200, body: FAKE_GMAIL_MSG }));
    const h = await buildHarness({ sendBlueFetch, gmailFetch });

    const report = await runOutboundOnce(h.deps);
    assert.equal(report.alerts_failed, 1);
    assert.equal(await h.transitions.currentState(ALERT_ID), 'failed');

    const events = await h.auditStore.recent(FOUNDER_USER, 500);
    assert.ok(events.find((e) => e.action === 'fomo.send.failed'));
  });

  it('5xx → approved → send_status_unknown', async () => {
    const sendBlueFetch = mockFetch(async () => ({ status: 503, body: { error: 'unavailable' } }));
    const gmailFetch = mockFetch(async () => ({ status: 200, body: FAKE_GMAIL_MSG }));
    const h = await buildHarness({ sendBlueFetch, gmailFetch });

    const report = await runOutboundOnce(h.deps);
    assert.equal(report.alerts_status_unknown, 1);
    assert.equal(await h.transitions.currentState(ALERT_ID), 'send_status_unknown');

    const events = await h.auditStore.recent(FOUNDER_USER, 500);
    assert.ok(events.find((e) => e.action === 'fomo.send.status_unknown'));
  });

  it('rate-limited 429 → send_status_unknown (never auto-retry)', async () => {
    const sendBlueFetch = mockFetch(async () => ({ status: 429, body: { error: 'rate_limited' } }));
    const gmailFetch = mockFetch(async () => ({ status: 200, body: FAKE_GMAIL_MSG }));
    const h = await buildHarness({ sendBlueFetch, gmailFetch });

    const report = await runOutboundOnce(h.deps);
    assert.equal(report.alerts_status_unknown, 1);
    assert.equal(await h.transitions.currentState(ALERT_ID), 'send_status_unknown');
  });

  it('HTTP 401 (auth) → failed (operator must rotate; never auto-retry)', async () => {
    const sendBlueFetch = mockFetch(async () => ({ status: 401, body: { error: 'invalid_api_key' } }));
    const gmailFetch = mockFetch(async () => ({ status: 200, body: FAKE_GMAIL_MSG }));
    const h = await buildHarness({ sendBlueFetch, gmailFetch });

    const report = await runOutboundOnce(h.deps);
    assert.equal(report.alerts_failed, 1);
    assert.equal(await h.transitions.currentState(ALERT_ID), 'failed');
  });
});

describe('runOutboundOnce — defense-in-depth founder-phone allowlist', () => {
  it('refuses to dispatch when destinationFor returns null', async () => {
    let sendBlueCalls = 0;
    const sendBlueFetch = mockFetch(async () => {
      sendBlueCalls++;
      return { status: 200, body: { status: 'QUEUED' } };
    });
    const gmailFetch = mockFetch(async () => ({ status: 200, body: FAKE_GMAIL_MSG }));
    const h = await buildHarness({
      sendBlueFetch,
      gmailFetch,
      destinationConfigured: false
    });

    const report = await runOutboundOnce(h.deps);
    assert.equal(report.alerts_unauthorized, 1);
    assert.equal(report.alerts_sent, 0);
    assert.equal(sendBlueCalls, 0, 'SendBlue must NOT be called when destination is null');
    assert.equal(await h.transitions.currentState(ALERT_ID), 'failed');

    const events = await h.auditStore.recent(FOUNDER_USER, 500);
    assert.ok(events.find((e) => e.action === 'fomo.send.unauthorized_destination'));
  });
});

describe('runOutboundOnce — kill-switch defense at the gate', () => {
  it('FOMO_SEND_ENABLED=false denies at the gate (sendblue never called)', async () => {
    let sendBlueCalls = 0;
    const sendBlueFetch = mockFetch(async () => {
      sendBlueCalls++;
      return { status: 200, body: { status: 'QUEUED' } };
    });
    const gmailFetch = mockFetch(async () => ({ status: 200, body: FAKE_GMAIL_MSG }));
    const h = await buildHarness({
      sendBlueFetch,
      gmailFetch,
      switches: SAFE_DEFAULT_KILL_SWITCHES // send_enabled=false
    });

    const report = await runOutboundOnce(h.deps);
    assert.equal(report.alerts_failed, 1);
    assert.equal(sendBlueCalls, 0, 'SendBlue must NOT be called when gate denies');
    assert.equal(await h.transitions.currentState(ALERT_ID), 'failed');

    const events = await h.auditStore.recent(FOUNDER_USER, 500);
    const failed = events.find((e) => e.action === 'fomo.send.failed');
    assert.ok(failed);
    assert.equal((failed?.detail as { decision_code: string }).decision_code, 'send_disabled');
  });
});

describe('runOutboundOnce — gmail.read recovery failure → status_unknown', () => {
  it('gmail.read failing → approved → send_status_unknown (do not assume sent or failed)', async () => {
    let sendBlueCalls = 0;
    const sendBlueFetch = mockFetch(async () => {
      sendBlueCalls++;
      return { status: 200, body: { status: 'QUEUED' } };
    });
    const gmailFetch = mockFetch(async () => ({
      status: 404,
      body: { error: { code: 404, message: 'message not found' } }
    }));
    const h = await buildHarness({ sendBlueFetch, gmailFetch });

    const report = await runOutboundOnce(h.deps);
    assert.equal(report.alerts_status_unknown, 1);
    assert.equal(sendBlueCalls, 0, 'SendBlue must NOT be called when gmail re-read fails');
    assert.equal(await h.transitions.currentState(ALERT_ID), 'send_status_unknown');
  });
});

describe('runOutboundOnce — no approved alerts → no-op cycle', () => {
  it('returns empty cycle when no alert is in approved state', async () => {
    const sendBlueFetch = mockFetch(async () => ({ status: 200, body: { status: 'QUEUED' } }));
    const gmailFetch = mockFetch(async () => ({ status: 200, body: FAKE_GMAIL_MSG }));
    const h = await buildHarness({ sendBlueFetch, gmailFetch });

    // Knock the alert out of approved → rejected.
    await h.transitions.write({
      alert_id: ALERT_ID,
      user_id: FOUNDER_USER,
      from_state: 'approved',
      to_state: 'rejected',
      reason: 'test'
    });

    const report = await runOutboundOnce(h.deps);
    assert.equal(report.alerts_considered, 0);
    assert.equal(report.alerts_sent, 0);
    assert.equal(report.users_total, 1);
    assert.equal(report.users_with_approved_alerts, 0);
  });
});

describe('runOutboundOnce — audit privacy invariant', () => {
  it('audit detail never carries the rendered message text or rank reason verbatim', async () => {
    const sendBlueFetch = mockFetch(async () => ({
      status: 200,
      body: { status: 'QUEUED', message_handle: 'mh-1' }
    }));
    const gmailFetch = mockFetch(async () => ({ status: 200, body: FAKE_GMAIL_MSG }));
    const h = await buildHarness({ sendBlueFetch, gmailFetch });
    await runOutboundOnce(h.deps);

    const events = await h.auditStore.recent(FOUNDER_USER, 500);
    for (const e of events) {
      const detailStr = JSON.stringify(e.detail);
      // The rendered template would include the subject 'lunch?' from the
      // Gmail mock. Audit detail must not include the rendered text.
      assert.ok(!detailStr.includes('lunch?'), `audit leaked subject: ${detailStr}`);
      assert.ok(
        !detailStr.includes('Hi Sarah'),
        `audit leaked body snippet: ${detailStr}`
      );
    }
  });
});

/* ====================================================================== */
/* Phase 3F.1 — STOP enforcement (LOAD-BEARING)                           */
/* ====================================================================== */

describe('runOutboundOnce — STOP enforcement (Phase 3F.1)', () => {
  it('refuses to send when stop_active=true; audits fomo.send.stop_enforced; transitions approved→failed; NEVER calls SendBlue', async () => {
    let sendBlueCalls = 0;
    const sendBlueFetch = mockFetch(async () => {
      sendBlueCalls++;
      return { status: 200, body: { status: 'QUEUED' } };
    });
    const gmailFetch = mockFetch(async () => ({ status: 200, body: FAKE_GMAIL_MSG }));
    const h = await buildHarness({ sendBlueFetch, gmailFetch });

    // Pre-set stop_active=true (as the /sendblue/inbound route's
    // deterministic STOP handler would).
    await h.memoryStore.upsert({
      user_id: FOUNDER_USER,
      kind: 'stop_active',
      scope_key: null,
      detail: { active: true, recorded_at: new Date().toISOString() },
      source: 'user_confirmed',
      confidence: 1.0
    });

    const report = await runOutboundOnce(h.deps);

    // The approved alert was considered, then refused.
    assert.equal(report.alerts_considered, 1);
    assert.equal(report.alerts_sent, 0);
    assert.equal(report.alerts_stop_enforced, 1);

    // SendBlue was NEVER called.
    assert.equal(sendBlueCalls, 0, 'SendBlue must NOT be called when stop_active=true');

    // State transitioned approved → failed with reason stop_enforced.
    assert.equal(await h.transitions.currentState(ALERT_ID), 'failed');

    // fomo.send.stop_enforced audit fired.
    const events = await h.auditStore.recent(FOUNDER_USER, 500);
    const stopAudit = events.find((e) => e.action === 'fomo.send.stop_enforced');
    assert.ok(stopAudit, 'expected fomo.send.stop_enforced audit row');
    assert.equal((stopAudit?.detail as { alert_id: string }).alert_id, ALERT_ID);

    // fomo.send.attempted should NOT fire (the worker short-circuited
    // before reaching the normal send path).
    const attempted = events.find((e) => e.action === 'fomo.send.attempted');
    assert.equal(attempted, undefined, 'fomo.send.attempted must NOT fire when stop_enforced');
  });

  it('does NOT enforce when stop_active=false (the memory_signal was START-cleared)', async () => {
    const sendBlueFetch = mockFetch(async () => ({
      status: 200,
      body: { status: 'QUEUED', message_handle: 'mh-1' }
    }));
    const gmailFetch = mockFetch(async () => ({ status: 200, body: FAKE_GMAIL_MSG }));
    const h = await buildHarness({ sendBlueFetch, gmailFetch });

    // Pre-set stop_active=false (post-START state).
    await h.memoryStore.upsert({
      user_id: FOUNDER_USER,
      kind: 'stop_active',
      scope_key: null,
      detail: { active: false, recorded_at: new Date().toISOString() },
      source: 'user_confirmed',
      confidence: 1.0
    });

    const report = await runOutboundOnce(h.deps);

    // Normal send proceeds.
    assert.equal(report.alerts_sent, 1);
    assert.equal(report.alerts_stop_enforced, 0);
    assert.equal(await h.transitions.currentState(ALERT_ID), 'sent');
  });

  it('does NOT enforce when no stop_active signal exists at all (default)', async () => {
    const sendBlueFetch = mockFetch(async () => ({
      status: 200,
      body: { status: 'QUEUED', message_handle: 'mh-1' }
    }));
    const gmailFetch = mockFetch(async () => ({ status: 200, body: FAKE_GMAIL_MSG }));
    const h = await buildHarness({ sendBlueFetch, gmailFetch });

    // No memory signal at all.
    const report = await runOutboundOnce(h.deps);

    assert.equal(report.alerts_sent, 1);
    assert.equal(report.alerts_stop_enforced, 0);
  });

  it('checks stop_active ONCE per user per cycle (not per alert)', async () => {
    // If we had multiple approved alerts and stop_active=true, ALL
    // should be stop_enforced in the same cycle. The check is per-
    // user, not per-alert.
    const sendBlueFetch = mockFetch(async () => ({
      status: 200,
      body: { status: 'QUEUED' }
    }));
    const gmailFetch = mockFetch(async () => ({ status: 200, body: FAKE_GMAIL_MSG }));
    const h = await buildHarness({ sendBlueFetch, gmailFetch });

    // Seed a second approved alert.
    await h.rankResultStore.write({
      user_id: FOUNDER_USER,
      message_id: 'msg-second',
      invocation_id: 'rank-inv-2',
      model_name: 'gpt-5-mini',
      prompt_version: 'ranker-v0.1.0',
      label: 'important',
      score: 0.85,
      reason: 'second alert',
      latency_ms: 100,
      input_tokens: 50,
      output_tokens: 20,
      estimated_cost_usd: 0.0001
    });
    const r2 = await h.rankResultStore.get(FOUNDER_USER, 'msg-second');
    await h.alertStore.create({
      alert_id: 'alert-2',
      user_id: FOUNDER_USER,
      message_id: 'msg-second',
      rank_result_id: r2!.id,
      label: 'important',
      score: 0.85
    });
    for (const [from, to] of [
      ['detected', 'ranked'],
      ['ranked', 'queued_for_review'],
      ['queued_for_review', 'approved']
    ] as const) {
      await h.transitions.write({
        alert_id: 'alert-2',
        user_id: FOUNDER_USER,
        from_state: from,
        to_state: to,
        reason: 'test-setup'
      });
    }

    // STOP active.
    await h.memoryStore.upsert({
      user_id: FOUNDER_USER,
      kind: 'stop_active',
      scope_key: null,
      detail: { active: true, recorded_at: new Date().toISOString() },
      source: 'user_confirmed',
      confidence: 1.0
    });

    const report = await runOutboundOnce(h.deps);

    // Both alerts considered, both refused.
    assert.equal(report.alerts_considered, 2);
    assert.equal(report.alerts_sent, 0);
    assert.equal(report.alerts_stop_enforced, 2);

    // Both alerts ended in 'failed'.
    assert.equal(await h.transitions.currentState(ALERT_ID), 'failed');
    assert.equal(await h.transitions.currentState('alert-2'), 'failed');
  });

  it('does NOT leak the full founder phone in stop_enforced audit detail (privacy invariant)', async () => {
    const sendBlueFetch = mockFetch(async () => ({ status: 200, body: { status: 'QUEUED' } }));
    const gmailFetch = mockFetch(async () => ({ status: 200, body: FAKE_GMAIL_MSG }));
    const h = await buildHarness({ sendBlueFetch, gmailFetch });
    await h.memoryStore.upsert({
      user_id: FOUNDER_USER,
      kind: 'stop_active',
      scope_key: null,
      detail: { active: true, recorded_at: new Date().toISOString() },
      source: 'user_confirmed',
      confidence: 1.0
    });
    await runOutboundOnce(h.deps);
    const events = await h.auditStore.recent(FOUNDER_USER, 500);
    for (const e of events) {
      assert.ok(
        !JSON.stringify(e.detail).includes(FOUNDER_PHONE),
        `audit ${e.action} leaked founder phone`
      );
    }
  });
});

// Phase 3G.1 item #2 — OPTED_OUT drift detection.
//
// Original incident captured: 2026-05-29 01:12 UTC during the 3G smoke
// run. Alert d9728e57-… approved → failed with audit detail showing
// only `client_error: HTTP 400`, no usable error category. Local
// stop_active was false (we'd cleared it via SQL), but SendBlue's
// spam-rule firewall kept blocking because the carrier-level opt-out
// list still had the founder's phone marked. The runtime had no way
// to surface the drift — operator had to hand-roll curl to discover.
//
// This test set proves:
//   * When SendBlue returns 400 OPTED_OUT, the worker emits the new
//     fomo.send.opt_out_drift_detected audit row with named fields.
//   * The worker writes memory_signals.stop_active=true with
//     source='opt_out_drift_carrier' so the next outbound cycle short-
//     circuits via stop_enforced instead of round-tripping to SendBlue.
//   * The fomo.send.failed row STILL fires (unchanged behavior) and
//     gains the new provider_error_message/reason/code named fields.
//   * The alert STILL transitions approved → failed (unchanged).
//   * The raw response body (error_detail, content, accountEmail,
//     to_number, from_number) NEVER appears in any audit row.
describe('runOutboundOnce — OPTED_OUT drift detection (Phase 3G.1 item #2)', () => {
  function optedOutResponse(): Promise<{ status: number; body: object }> {
    return Promise.resolve({
      status: 400,
      body: {
        accountEmail: 'orbitai-labs',
        content: 'CANARY-PAYLOAD-MUST-NOT-LEAK-12345',
        is_outbound: true,
        status: 'ERROR',
        error_code: 402,
        error_message: 'OPTED_OUT',
        error_reason: 'SpamRule',
        error_detail: 'CANARY-DETAIL-MUST-NOT-LEAK-67890',
        to_number: '+19999999999',
        from_number: '+18888888888'
      }
    });
  }

  it('emits fomo.send.opt_out_drift_detected with named provider fields when SendBlue returns OPTED_OUT', async () => {
    const sendBlueFetch = mockFetch(optedOutResponse);
    const gmailFetch = mockFetch(async () => ({ status: 200, body: FAKE_GMAIL_MSG }));
    const h = await buildHarness({ sendBlueFetch, gmailFetch });

    await runOutboundOnce(h.deps);

    const events = await h.auditStore.recent(FOUNDER_USER, 500);
    const drift = events.find((e) => e.action === 'fomo.send.opt_out_drift_detected');
    assert.ok(drift, 'fomo.send.opt_out_drift_detected must be emitted');
    const d = drift.detail as Record<string, unknown>;
    assert.equal(d.provider_error_message, 'OPTED_OUT');
    assert.equal(d.provider_error_reason, 'SpamRule');
    assert.equal(d.provider_error_code, '402');
    assert.equal(d.stop_active_synced, true);
    assert.equal(d.alert_id, ALERT_ID);
  });

  it('writes stop_active=true with source=opt_out_drift_carrier so the next cycle short-circuits', async () => {
    const sendBlueFetch = mockFetch(optedOutResponse);
    const gmailFetch = mockFetch(async () => ({ status: 200, body: FAKE_GMAIL_MSG }));
    const h = await buildHarness({ sendBlueFetch, gmailFetch });

    // Local cache says active=false going in (the drift scenario).
    await h.memoryStore.upsert({
      user_id: FOUNDER_USER,
      kind: 'stop_active',
      scope_key: null,
      detail: { active: false },
      source: 'user_confirmed',
      confidence: 1.0
    });

    await runOutboundOnce(h.deps);

    const signal = await h.memoryStore.get(FOUNDER_USER, 'stop_active', null);
    assert.ok(signal, 'stop_active signal must exist after drift detection');
    assert.equal((signal.detail as { active?: boolean }).active, true);
    assert.equal(signal.source, 'opt_out_drift_carrier');
    assert.equal(signal.confidence, 1.0);
  });

  it('STILL emits fomo.send.failed with the provider error fields surfaced (no double-state-transition)', async () => {
    const sendBlueFetch = mockFetch(optedOutResponse);
    const gmailFetch = mockFetch(async () => ({ status: 200, body: FAKE_GMAIL_MSG }));
    const h = await buildHarness({ sendBlueFetch, gmailFetch });

    await runOutboundOnce(h.deps);

    const events = await h.auditStore.recent(FOUNDER_USER, 500);
    const failed = events.find((e) => e.action === 'fomo.send.failed');
    assert.ok(failed, 'fomo.send.failed must still fire on the same alert');
    const d = failed.detail as Record<string, unknown>;
    assert.equal(d.provider_error_message, 'OPTED_OUT');
    assert.equal(d.provider_error_reason, 'SpamRule');
    assert.equal(d.provider_error_code, '402');
    assert.equal(d.http_status, 400);
    // Same state-machine transition as any other clear failure.
    assert.equal(await h.transitions.currentState(ALERT_ID), 'failed');
  });

  it('does NOT leak error_detail, content, accountEmail, or any non-allowlisted field into ANY audit row', async () => {
    const sendBlueFetch = mockFetch(optedOutResponse);
    const gmailFetch = mockFetch(async () => ({ status: 200, body: FAKE_GMAIL_MSG }));
    const h = await buildHarness({ sendBlueFetch, gmailFetch });

    await runOutboundOnce(h.deps);

    const events = await h.auditStore.recent(FOUNDER_USER, 500);
    for (const e of events) {
      const dump = JSON.stringify(e.detail);
      assert.equal(dump.includes('CANARY-PAYLOAD-MUST-NOT-LEAK-12345'), false, `${e.action} leaked content`);
      assert.equal(dump.includes('CANARY-DETAIL-MUST-NOT-LEAK-67890'), false, `${e.action} leaked error_detail`);
      assert.equal(dump.includes('orbitai-labs'), false, `${e.action} leaked accountEmail`);
      assert.equal(dump.includes('+19999999999'), false, `${e.action} leaked to_number`);
      assert.equal(dump.includes('+18888888888'), false, `${e.action} leaked from_number`);
    }
  });

  it('does NOT emit opt_out_drift_detected for non-OPTED_OUT 400s (e.g. plain invalid_number)', async () => {
    const sendBlueFetch = mockFetch(async () => ({
      status: 400,
      body: { error_message: 'INVALID_NUMBER', error_reason: 'BadFormat', error_code: 100 }
    }));
    const gmailFetch = mockFetch(async () => ({ status: 200, body: FAKE_GMAIL_MSG }));
    const h = await buildHarness({ sendBlueFetch, gmailFetch });

    await runOutboundOnce(h.deps);

    const events = await h.auditStore.recent(FOUNDER_USER, 500);
    assert.equal(events.find((e) => e.action === 'fomo.send.opt_out_drift_detected'), undefined);
    // But the named fields still surface in fomo.send.failed.
    const failed = events.find((e) => e.action === 'fomo.send.failed');
    assert.ok(failed);
    assert.equal((failed.detail as { provider_error_message: unknown }).provider_error_message, 'INVALID_NUMBER');
  });
});

describe('runOutboundOnce — SendBlue contact-registration gate (Phase v0.5.3 item #1)', () => {
  // Regression for the v0.5.2 incident: friend was onboarded with
  // OAuth + users + oauth_tokens all in place, but SendBlue's contact
  // registration silently dropped both her inbound webhooks AND
  // outbound sends. Per founder correction #1: we do NOT roll back
  // OAuth on registration failure (friend's tokens are valuable);
  // instead memory_signals.sendblue_contact_status records
  // registered=false and THE OUTBOUND WORKER refuses to send.
  it('refuses to send when sendblue_contact_status.registered=false; audits fomo.send.contact_not_registered; transitions approved→failed; NEVER calls SendBlue', async () => {
    let sendBlueCalls = 0;
    const sendBlueFetch = mockFetch(async () => {
      sendBlueCalls++;
      return { status: 200, body: { status: 'QUEUED' } };
    });
    const gmailFetch = mockFetch(async () => ({ status: 200, body: FAKE_GMAIL_MSG }));
    const h = await buildHarness({ sendBlueFetch, gmailFetch });

    // Pre-set the gate: registered=false (as /onboard/callback would
    // when SendBlue's POST /api/v2/contacts returns a 4xx). The
    // error_reason is operator-visible diagnostic; never includes the
    // raw SendBlue response body.
    await h.memoryStore.upsert({
      user_id: FOUNDER_USER,
      kind: 'sendblue_contact_status',
      scope_key: null,
      detail: {
        registered: false,
        error_reason: 'client_error: HTTP 400 ContactHasNotBeenVerified',
        attempted_at: new Date().toISOString()
      },
      source: 'founder_set',
      confidence: 1.0
    });

    const report = await runOutboundOnce(h.deps);

    // The approved alert was considered, then refused at the gate.
    assert.equal(report.alerts_considered, 1);
    assert.equal(report.alerts_sent, 0);
    assert.equal(report.alerts_contact_not_registered, 1);
    assert.equal(report.alerts_stop_enforced, 0);

    // SendBlue API was NEVER called.
    assert.equal(sendBlueCalls, 0, 'SendBlue must NOT be called when contact registration is false');

    // Alert transitioned approved → failed with reason
    // contact_not_registered.
    assert.equal(await h.transitions.currentState(ALERT_ID), 'failed');

    // fomo.send.contact_not_registered audit fired.
    const events = await h.auditStore.recent(FOUNDER_USER, 500);
    const gateAudit = events.find((e) => e.action === 'fomo.send.contact_not_registered');
    assert.ok(gateAudit, 'expected fomo.send.contact_not_registered audit row');
    const detail = gateAudit?.detail as { alert_id: string; registration_error_reason: string };
    assert.equal(detail.alert_id, ALERT_ID);
    // Operator-visible error_reason carried forward; bounded to 120 chars.
    assert.match(detail.registration_error_reason, /HTTP 400 ContactHasNotBeenVerified/);

    // fomo.send.attempted should NOT fire (worker short-circuited).
    assert.equal(events.find((e) => e.action === 'fomo.send.attempted'), undefined);
  });

  it('proceeds normally when sendblue_contact_status.registered=true (the happy path)', async () => {
    const sendBlueFetch = mockFetch(async () => ({
      status: 200,
      body: { status: 'QUEUED', message_handle: 'mh-contact-ok-1' }
    }));
    const gmailFetch = mockFetch(async () => ({ status: 200, body: FAKE_GMAIL_MSG }));
    const h = await buildHarness({ sendBlueFetch, gmailFetch });

    await h.memoryStore.upsert({
      user_id: FOUNDER_USER,
      kind: 'sendblue_contact_status',
      scope_key: null,
      detail: { registered: true, registered_at: new Date().toISOString() },
      source: 'founder_set',
      confidence: 1.0
    });

    const report = await runOutboundOnce(h.deps);

    assert.equal(report.alerts_sent, 1);
    assert.equal(report.alerts_contact_not_registered, 0);
    assert.equal(await h.transitions.currentState(ALERT_ID), 'sent');
  });

  it('does NOT block when no sendblue_contact_status signal exists (preserves founder + pre-v0.5.3 friends)', async () => {
    // Missing-signal returns false (= allow) so the gate doesn't
    // retroactively block the founder (env-equality routed, no
    // memory signal) or v0.5.1/v0.5.2 friends onboarded before this
    // gate existed.
    const sendBlueFetch = mockFetch(async () => ({
      status: 200,
      body: { status: 'QUEUED', message_handle: 'mh-no-signal-1' }
    }));
    const gmailFetch = mockFetch(async () => ({ status: 200, body: FAKE_GMAIL_MSG }));
    const h = await buildHarness({ sendBlueFetch, gmailFetch });
    // No memoryStore.upsert — no signal at all.

    const report = await runOutboundOnce(h.deps);

    assert.equal(report.alerts_sent, 1);
    assert.equal(report.alerts_contact_not_registered, 0);
    assert.equal(await h.transitions.currentState(ALERT_ID), 'sent');
  });

  it('contact_not_registered gate fires BEFORE stop_enforced if both signals are present (stop wins because it is the more privacy-load-bearing block)', async () => {
    // Ordering choice: STOP enforcement runs first (compliance-load-bearing).
    // The contact_not_registered gate only fires when stop is NOT
    // active. This tests that the gate ordering is correct AND that
    // we don't double-process an alert.
    let sendBlueCalls = 0;
    const sendBlueFetch = mockFetch(async () => {
      sendBlueCalls++;
      return { status: 200, body: { status: 'QUEUED' } };
    });
    const gmailFetch = mockFetch(async () => ({ status: 200, body: FAKE_GMAIL_MSG }));
    const h = await buildHarness({ sendBlueFetch, gmailFetch });

    await h.memoryStore.upsert({
      user_id: FOUNDER_USER,
      kind: 'stop_active',
      scope_key: null,
      detail: { active: true, recorded_at: new Date().toISOString() },
      source: 'user_confirmed',
      confidence: 1.0
    });
    await h.memoryStore.upsert({
      user_id: FOUNDER_USER,
      kind: 'sendblue_contact_status',
      scope_key: null,
      detail: { registered: false, error_reason: 'test', attempted_at: new Date().toISOString() },
      source: 'founder_set',
      confidence: 1.0
    });

    const report = await runOutboundOnce(h.deps);

    // STOP wins because it's checked first.
    assert.equal(report.alerts_stop_enforced, 1);
    assert.equal(report.alerts_contact_not_registered, 0);
    assert.equal(sendBlueCalls, 0, 'SendBlue must NOT be called when either gate blocks');
  });

  it('canary privacy: registration_error_reason NEVER contains raw SendBlue response body or full destination phone', async () => {
    const gmailFetch = mockFetch(async () => ({ status: 200, body: FAKE_GMAIL_MSG }));
    const h = await buildHarness({ gmailFetch });

    // Adversarial error_reason that includes content that should NOT
    // leak (the registration code that writes this signal only ever
    // sets error_reason from SendBlueClient.registerContact's bounded
    // `reason` field — never from the raw response body — but defense-
    // in-depth: even if a future caller passes an unsafe value, the
    // outbound audit row bounds it to 120 chars and the destination
    // phone never appears in this audit path).
    const adversarial =
      'should-not-leak-canary-aaa raw_body_canary_bbb +15551234567';
    await h.memoryStore.upsert({
      user_id: FOUNDER_USER,
      kind: 'sendblue_contact_status',
      scope_key: null,
      detail: { registered: false, error_reason: adversarial, attempted_at: new Date().toISOString() },
      source: 'founder_set',
      confidence: 1.0
    });

    await runOutboundOnce(h.deps);

    const events = await h.auditStore.recent(FOUNDER_USER, 500);
    const gateAudit = events.find((e) => e.action === 'fomo.send.contact_not_registered');
    assert.ok(gateAudit);
    const detail = gateAudit?.detail as { registration_error_reason: string };
    // The reason field IS surfaced (operator-visible) but bounded.
    assert.ok(detail.registration_error_reason.length <= 120);
    // The phone digits in the adversarial string would appear in the
    // reason field (since this audit row intentionally surfaces what
    // the signal recorded). But the audit row does NOT include
    // destination_slug or any other phone field on this code path —
    // we never reach the destinationFor step. Confirm that:
    const serialized = JSON.stringify(gateAudit?.detail ?? {});
    assert.equal(/destination_slug|to_number|from_number/.test(serialized), false);
  });
});

/* ============================================================== */
/* Phase v0.5.6 — iMessage Tone + Summary Length                  */
/* ============================================================== */
//
// Covers the new runtime contract:
//   - rank.reason is threaded through to renderFounderText
//   - fomo.send.attempted audit detail carries reason_source
//   - When rank.reason fails the body-render schema (empty or >180),
//     fomo.alert.drafter_schema_failed audit fires BEFORE
//     fomo.send.attempted; the send still proceeds with the
//     deterministic fallback body (Q6: best-effort audit, no retry).
//   - Founder isolation: only founder's audit rows in this run.
//   - v0.5.5 STOP enforcement remains untouched: when stop_active=true,
//     stop_enforced wins and the v0.5.6 fallback path never fires.

describe('runOutboundOnce — v0.5.6 rank.reason wiring (happy path)', () => {
  it('fomo.send.attempted detail carries reason_source=rank when reason is within bounds', async () => {
    const sendBlueFetch = mockFetch(async () => ({
      status: 200,
      body: { status: 'QUEUED', message_handle: 'mh-1' }
    }));
    const gmailFetch = mockFetch(async () => ({ status: 200, body: FAKE_GMAIL_MSG }));
    const h = await buildHarness({
      sendBlueFetch,
      gmailFetch,
      reasonOverride: 'Counselor flagged a time-sensitive dorm deposit deadline tonight.'
    });

    await runOutboundOnce(h.deps);

    const events = await h.auditStore.recent(FOUNDER_USER, 500);
    const attempted = events.find((e) => e.action === 'fomo.send.attempted');
    assert.ok(attempted);
    const detail = attempted?.detail as { reason_source: string; template_version: string };
    assert.equal(detail.reason_source, 'rank');
    // Template version is the bumped v0.5.6 value.
    assert.equal(detail.template_version, 'human-message-v0.3.0');

    // No fallback audit when reason is within bounds.
    const fallback = events.find((e) => e.action === 'fomo.alert.drafter_schema_failed');
    assert.equal(fallback, undefined);
  });
});

describe('runOutboundOnce — v0.5.6 deterministic fallback (Q6: best-effort audit, no retry)', () => {
  it('empty rank.reason triggers fallback + fomo.alert.drafter_schema_failed audit + send still proceeds', async () => {
    const sendBlueFetch = mockFetch(async () => ({
      status: 200,
      body: { status: 'QUEUED', message_handle: 'mh-1' }
    }));
    const gmailFetch = mockFetch(async () => ({ status: 200, body: FAKE_GMAIL_MSG }));
    const h = await buildHarness({
      sendBlueFetch,
      gmailFetch,
      reasonOverride: ''
    });

    const report = await runOutboundOnce(h.deps);

    // Send still proceeds with the deterministic fallback body.
    assert.equal(report.alerts_sent, 1);
    assert.equal(await h.transitions.currentState(ALERT_ID), 'sent');

    const events = await h.auditStore.recent(FOUNDER_USER, 500);
    const fallback = events.find((e) => e.action === 'fomo.alert.drafter_schema_failed');
    assert.ok(fallback, 'drafter_schema_failed must fire on empty reason');
    const fbDetail = fallback?.detail as {
      reason_violation_kind: string;
      original_reason_length: number;
      template_version: string;
    };
    assert.equal(fbDetail.reason_violation_kind, 'empty');
    assert.equal(fbDetail.original_reason_length, 0);
    assert.equal(fbDetail.template_version, 'human-message-v0.3.0');

    // fomo.send.attempted detail.reason_source === 'fallback'.
    const attempted = events.find((e) => e.action === 'fomo.send.attempted');
    assert.ok(attempted);
    assert.equal(
      (attempted?.detail as { reason_source: string }).reason_source,
      'fallback'
    );
  });

  it('over-long rank.reason (>180) triggers fallback + reason_violation_kind=too_long', async () => {
    const sendBlueFetch = mockFetch(async () => ({
      status: 200,
      body: { status: 'QUEUED', message_handle: 'mh-1' }
    }));
    const gmailFetch = mockFetch(async () => ({ status: 200, body: FAKE_GMAIL_MSG }));
    const tooLong = 'x'.repeat(250);
    const h = await buildHarness({
      sendBlueFetch,
      gmailFetch,
      reasonOverride: tooLong
    });

    await runOutboundOnce(h.deps);

    const events = await h.auditStore.recent(FOUNDER_USER, 500);
    const fallback = events.find((e) => e.action === 'fomo.alert.drafter_schema_failed');
    assert.ok(fallback);
    const fbDetail = fallback?.detail as {
      reason_violation_kind: string;
      original_reason_length: number;
    };
    assert.equal(fbDetail.reason_violation_kind, 'too_long');
    assert.equal(fbDetail.original_reason_length, 250);
  });

  it('drafter_schema_failed fires BEFORE fomo.send.attempted (audit ordering)', async () => {
    const sendBlueFetch = mockFetch(async () => ({
      status: 200,
      body: { status: 'QUEUED', message_handle: 'mh-1' }
    }));
    const gmailFetch = mockFetch(async () => ({ status: 200, body: FAKE_GMAIL_MSG }));
    const h = await buildHarness({
      sendBlueFetch,
      gmailFetch,
      reasonOverride: ''
    });

    await runOutboundOnce(h.deps);

    const events = await h.auditStore.recent(FOUNDER_USER, 500);
    // recent() returns newest-first; the EARLIER event has the LARGER index.
    const fallbackIdx = events.findIndex((e) => e.action === 'fomo.alert.drafter_schema_failed');
    const attemptedIdx = events.findIndex((e) => e.action === 'fomo.send.attempted');
    assert.ok(fallbackIdx >= 0 && attemptedIdx >= 0);
    assert.ok(
      fallbackIdx > attemptedIdx,
      `drafter_schema_failed must precede send.attempted in audit log (fb idx=${fallbackIdx}, attempted idx=${attemptedIdx})`
    );
  });

  it('Q6 no-retry: cycle with fallback-fired alert does NOT re-process on a second cycle', async () => {
    const sendBlueFetch = mockFetch(async () => ({
      status: 200,
      body: { status: 'QUEUED', message_handle: 'mh-1' }
    }));
    const gmailFetch = mockFetch(async () => ({ status: 200, body: FAKE_GMAIL_MSG }));
    const h = await buildHarness({
      sendBlueFetch,
      gmailFetch,
      reasonOverride: ''
    });

    const r1 = await runOutboundOnce(h.deps);
    assert.equal(r1.alerts_sent, 1);
    const eventsAfterCycle1 = await h.auditStore.recent(FOUNDER_USER, 500);
    const fallbackCountAfter1 = eventsAfterCycle1.filter(
      (e) => e.action === 'fomo.alert.drafter_schema_failed'
    ).length;
    assert.equal(fallbackCountAfter1, 1);

    const r2 = await runOutboundOnce(h.deps);
    assert.equal(r2.alerts_considered, 0, 'second cycle must not re-find the already-sent alert');
    const eventsAfterCycle2 = await h.auditStore.recent(FOUNDER_USER, 500);
    const fallbackCountAfter2 = eventsAfterCycle2.filter(
      (e) => e.action === 'fomo.alert.drafter_schema_failed'
    ).length;
    // Critical: fallback audit must NOT fire a second time (no retry).
    assert.equal(fallbackCountAfter2, 1, 'fallback audit must NOT fire on a second cycle (Q6 no-retry)');
  });
});

describe('runOutboundOnce — v0.5.6 cross-tenant + founder isolation', () => {
  it('only founder-tagged audit rows appear in this run; no leakage to other actor_user_ids', async () => {
    const sendBlueFetch = mockFetch(async () => ({
      status: 200,
      body: { status: 'QUEUED', message_handle: 'mh-1' }
    }));
    const gmailFetch = mockFetch(async () => ({ status: 200, body: FAKE_GMAIL_MSG }));
    const h = await buildHarness({
      sendBlueFetch,
      gmailFetch,
      reasonOverride: ''
    });

    await runOutboundOnce(h.deps);

    // Sample a few OTHER user_ids and verify the audit store has zero
    // rows for them. The InMemoryAuditStore filters by actor_user_id.
    const otherUsers = ['friend-morris', 'friend-gm3258', 'sheila-residual'];
    for (const u of otherUsers) {
      const events = await h.auditStore.recent(u, 100);
      assert.equal(events.length, 0, `actor_user_id=${u} must have zero audit rows`);
    }
    // And founder must have ≥ the expected rows (drafter_schema_failed +
    // send.attempted + send.succeeded at minimum).
    const founderEvents = await h.auditStore.recent(FOUNDER_USER, 100);
    assert.ok(
      founderEvents.some((e) => e.action === 'fomo.alert.drafter_schema_failed')
    );
    assert.ok(founderEvents.some((e) => e.action === 'fomo.send.attempted'));
    assert.ok(founderEvents.some((e) => e.action === 'fomo.send.succeeded'));
  });
});

describe('runOutboundOnce — v0.5.6 preserves v0.5.5 STOP enforcement', () => {
  it('stop_active=true wins over the v0.5.6 fallback path: stop_enforced fires; drafter_schema_failed does NOT', async () => {
    const sendBlueFetch = mockFetch(async () => {
      throw new Error('SendBlue must NOT be called when stop_enforced fires');
    });
    const gmailFetch = mockFetch(async () => ({ status: 200, body: FAKE_GMAIL_MSG }));
    const h = await buildHarness({
      sendBlueFetch,
      gmailFetch,
      // An empty reason WOULD trigger the v0.5.6 fallback, but
      // stop_enforced must short-circuit before rendering.
      reasonOverride: ''
    });

    // Flip stop_active=true on the founder BEFORE the cycle.
    await h.memoryStore.upsert({
      user_id: FOUNDER_USER,
      kind: 'stop_active',
      scope_key: null,
      detail: { active: true, recorded_at: new Date().toISOString() },
      source: 'user_confirmed',
      confidence: 1.0
    });

    const report = await runOutboundOnce(h.deps);
    assert.equal(report.alerts_stop_enforced, 1, 'stop_enforced counter must increment');
    assert.equal(report.alerts_sent, 0);
    assert.equal(await h.transitions.currentState(ALERT_ID), 'failed');

    const events = await h.auditStore.recent(FOUNDER_USER, 500);
    // v0.5.5 stop_enforced audit fires.
    assert.ok(events.find((e) => e.action === 'fomo.send.stop_enforced'));
    // v0.5.6 fallback audit does NOT fire because we never reached the
    // body-render step.
    assert.equal(
      events.find((e) => e.action === 'fomo.alert.drafter_schema_failed'),
      undefined,
      'drafter_schema_failed must NOT fire when stop_enforced short-circuits the send'
    );
    // fomo.send.attempted does NOT fire either (stop_enforced is checked
    // before the render).
    assert.equal(events.find((e) => e.action === 'fomo.send.attempted'), undefined);
  });
});

/* ================================================================== */
/* v0.5.7 — Human Message Renderer wiring                              */
/* ================================================================== */

describe('runOutboundOnce — v0.5.7 HMR audit-field population (Q6.A)', () => {
  it('fomo.send.attempted detail carries all 4 new HMR audit fields with structural enum values', async () => {
    const sendBlueFetch = mockFetch(async () => ({
      status: 202,
      body: { status: 'ENQUEUED', message_handle: 'mh-test' }
    }));
    const gmailFetch = mockFetch(async () => ({ status: 200, body: FAKE_GMAIL_MSG }));
    const h = await buildHarness({ sendBlueFetch, gmailFetch });
    await runOutboundOnce(h.deps);

    const events = await h.auditStore.recent(FOUNDER_USER, 100);
    const attempted = events.find((e) => e.action === 'fomo.send.attempted');
    assert.ok(attempted, 'fomo.send.attempted must fire');
    const detail = attempted.detail as {
      sender_resolution_path: string;
      subject_strip_applied: string;
      reason_voice: string;
      template_shape: string;
    };
    // Harness sender_name="Sarah" → first_name path.
    assert.equal(detail.sender_resolution_path, 'first_name');
    // Harness subject="lunch?" → no prefix to strip → 'none'.
    assert.equal(detail.subject_strip_applied, 'none');
    // Harness prompt_version='ranker-v0.1.0' → reason_voice='legacy_3p'.
    assert.equal(detail.reason_voice, 'legacy_3p');
    // Both sender + subject present → two_sentence shape.
    assert.equal(detail.template_shape, 'two_sentence');
  });

  it('fomo.alert.hmr_degradation_applied fires when ANY Q5.A degradation rule triggers (legacy_3p case)', async () => {
    // Harness prompt_version='ranker-v0.1.0' → legacy_3p → degradation_applied=true.
    const sendBlueFetch = mockFetch(async () => ({
      status: 202,
      body: { status: 'ENQUEUED', message_handle: 'mh-test' }
    }));
    const gmailFetch = mockFetch(async () => ({ status: 200, body: FAKE_GMAIL_MSG }));
    const h = await buildHarness({ sendBlueFetch, gmailFetch });
    await runOutboundOnce(h.deps);

    const events = await h.auditStore.recent(FOUNDER_USER, 100);
    const degrade = events.find((e) => e.action === 'fomo.alert.hmr_degradation_applied');
    assert.ok(degrade, 'fomo.alert.hmr_degradation_applied must fire on legacy_3p (any Q5.A path)');
    const detail = degrade.detail as {
      sender_resolution_path: string;
      subject_strip_applied: string;
      reason_voice: string;
      template_shape: string;
    };
    assert.equal(detail.reason_voice, 'legacy_3p');
    // STRUCTURAL audit only — no raw subject/body/header in detail.
    const detailStr = JSON.stringify(degrade.detail);
    assert.ok(!detailStr.includes('lunch?'), `audit must not leak subject; got: ${detailStr}`);
    assert.ok(!detailStr.includes('Sarah'), `audit must not leak sender_name; got: ${detailStr}`);
    assert.ok(!detailStr.includes('@example.com'), `audit must not leak raw email; got: ${detailStr}`);
  });

  it('hmr_degradation_applied fires BEFORE fomo.send.attempted (audit ordering)', async () => {
    const sendBlueFetch = mockFetch(async () => ({
      status: 202,
      body: { status: 'ENQUEUED', message_handle: 'mh-test' }
    }));
    const gmailFetch = mockFetch(async () => ({ status: 200, body: FAKE_GMAIL_MSG }));
    const h = await buildHarness({ sendBlueFetch, gmailFetch });
    await runOutboundOnce(h.deps);
    const events = await h.auditStore.recent(FOUNDER_USER, 100);
    // recent() returns newest-first per existing fixture; reverse to scan
    // chronologically.
    const chronological = [...events].reverse();
    const degradeIdx = chronological.findIndex((e) => e.action === 'fomo.alert.hmr_degradation_applied');
    const attemptedIdx = chronological.findIndex((e) => e.action === 'fomo.send.attempted');
    assert.ok(degradeIdx >= 0 && attemptedIdx >= 0);
    assert.ok(
      degradeIdx < attemptedIdx,
      `hmr_degradation_applied (idx ${degradeIdx}) must precede send.attempted (idx ${attemptedIdx})`
    );
  });

  it('over-long reason → BOTH drafter_schema_failed AND hmr_degradation_applied fire (companion audits)', async () => {
    const sendBlueFetch = mockFetch(async () => ({
      status: 202,
      body: { status: 'ENQUEUED', message_handle: 'mh-test' }
    }));
    const gmailFetch = mockFetch(async () => ({ status: 200, body: FAKE_GMAIL_MSG }));
    const h = await buildHarness({
      sendBlueFetch,
      gmailFetch,
      reasonOverride: 'x'.repeat(250)
    });
    await runOutboundOnce(h.deps);

    const events = await h.auditStore.recent(FOUNDER_USER, 100);
    const drafter = events.find((e) => e.action === 'fomo.alert.drafter_schema_failed');
    const degrade = events.find((e) => e.action === 'fomo.alert.hmr_degradation_applied');
    assert.ok(drafter, 'drafter_schema_failed must fire on reason violation (v0.5.6 carry-forward)');
    assert.ok(degrade, 'hmr_degradation_applied must fire (companion to drafter_schema_failed)');
    const dd = degrade.detail as { reason_voice: string };
    assert.equal(dd.reason_voice, 'fallback');
  });

  it('Q5.A no-retry: second cycle does NOT re-fire hmr_degradation_applied for an already-sent alert', async () => {
    const sendBlueFetch = mockFetch(async () => ({
      status: 202,
      body: { status: 'ENQUEUED', message_handle: 'mh-test' }
    }));
    const gmailFetch = mockFetch(async () => ({ status: 200, body: FAKE_GMAIL_MSG }));
    const h = await buildHarness({ sendBlueFetch, gmailFetch });
    await runOutboundOnce(h.deps);
    await runOutboundOnce(h.deps);

    const events = await h.auditStore.recent(FOUNDER_USER, 100);
    const degradeCount = events.filter((e) => e.action === 'fomo.alert.hmr_degradation_applied').length;
    assert.equal(
      degradeCount,
      1,
      `hmr_degradation_applied must fire exactly once across two cycles (no retry); got ${degradeCount}`
    );
  });
});
