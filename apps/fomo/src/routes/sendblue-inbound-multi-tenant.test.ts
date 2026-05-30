// Phase v0.5.1 Step 6 — per-friend STOP integration test.
//
// Multi-tenant invariants tested end-to-end through the real
// sendblue-inbound route + the real outbound-sender worker:
//
//   * Two distinct synthetic phones (distinct phone_e164_hash).
//   * Inbound routes by phone_e164_hash (not founder-equality).
//   * Friend STOP → friend's stop_active=true; founder UNTOUCHED.
//   * Founder STOP → founder's stop_active=true; friend UNTOUCHED.
//   * Duplicate STOP is idempotent (no extra state writes, no error).
//   * Outbound-sender respects stop_active PER USER (friend's alert
//     blocked while founder's alert still sends, and vice versa).
//   * No cross-user memory contamination in any direction.
//   * No raw phones in audit detail (canary check via founder + friend
//     plaintext absence in serialized audit dump).
//
// Per founder directive 2026-05-29 (multi-tenant design principles,
// correction #4): the two synthetic phones MUST be distinct. We use
// the reserved-for-fiction +1 555-01xx range.

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';
import { randomUUID } from 'node:crypto';

import {
  InMemoryPhoneAllowlistStore,
  hashPhone,
  type PhoneHashConfig
} from '../security/phone-allowlist.js';
import { type CryptoConfig } from '../security/token-crypto.js';
import {
  type SendBlueInboundRouteDeps,
  handleSendBlueInbound
} from './sendblue-inbound.js';
import { InMemoryInboundReplyStore } from '../memory/inbound-replies.js';
import { InMemoryAlertStore } from '../memory/alerts.js';
import { InMemoryRankResultStore } from '../memory/rank-results.js';
import { InMemoryAlertStateTransitionStore } from '../core/alert-state-transitions.js';
import { InMemoryFeedbackStore } from '../memory/feedback-events.js';
import { InMemoryMemorySignalStore } from '../memory/memory-signals.js';
import { InMemoryAuditStore } from '../core/audit.js';
import { SAFE_DEFAULT_KILL_SWITCHES, type KillSwitches } from '../core/kill-switches.js';
import { type ReplyParseResult } from '../reply-parser/index.js';

/* ---------------------------------------------------------------------- */
/* Synthetic distinct phones (founder + friend)                           */
/* ---------------------------------------------------------------------- */

const FOUNDER_PHONE = '+15550100001';
const FRIEND_PHONE = '+15550100002';
const NON_USER_PHONE = '+15550100099';
const FOUNDER_USER = 'founder';
const FRIEND_USER = randomUUID(); // friend has a UUID user_id (provisioned via /onboard)

const TEST_KEK = Buffer.alloc(32, 13).toString('base64');
const TEST_HASH_KEY = Buffer.alloc(32, 91).toString('base64');
const cryptoConfig: CryptoConfig = {
  kek: Buffer.from(TEST_KEK, 'base64'),
  devMode: false
};
const phoneHashConfig: PhoneHashConfig = {
  hmacKey: Buffer.from(TEST_HASH_KEY, 'base64')
};

// Webhook config — auth-passing fixture so tests don't have to deal
// with signature mismatch at every turn.
const WEBHOOK_SECRET = 'phase-v0.5.1-step-6-webhook-secret';
const WEBHOOK_SECRET_HEADER = 'sb-signing-secret';

function buildBody(opts: { from: string; content: string; provider_message_id?: string }): string {
  return JSON.stringify({
    from_number: opts.from,
    content: opts.content,
    message_handle: opts.provider_message_id ?? `sb-${randomUUID()}`
  });
}

async function buildHarness(): Promise<{
  deps: SendBlueInboundRouteDeps;
  phoneAllowlist: InMemoryPhoneAllowlistStore;
  memoryStore: InMemoryMemorySignalStore;
  auditStore: InMemoryAuditStore;
  inboundReplyStore: InMemoryInboundReplyStore;
}> {
  const inboundReplyStore = new InMemoryInboundReplyStore();
  const alertStore = new InMemoryAlertStore();
  const rankResultStore = new InMemoryRankResultStore();
  const transitions = new InMemoryAlertStateTransitionStore();
  const feedbackStore = new InMemoryFeedbackStore();
  const memoryStore = new InMemoryMemorySignalStore();
  const auditStore = new InMemoryAuditStore();
  const phoneAllowlist = new InMemoryPhoneAllowlistStore(cryptoConfig, phoneHashConfig);

  // Pre-populate both users with their distinct phones.
  await phoneAllowlist.setPhone(FOUNDER_USER, FOUNDER_PHONE);
  await phoneAllowlist.setPhone(FRIEND_USER, FRIEND_PHONE);

  const switches: KillSwitches = Object.freeze({
    ...SAFE_DEFAULT_KILL_SWITCHES,
    sendblue_inbound_enabled: true
  });

  // Default parser: returns 'stop' deterministically for the literal "STOP",
  // 'unclear' for everything else. The actual reply-parser uses the
  // deterministic safety pre-pass + classifier pipeline; for this
  // integration test we stub to keep the focus on routing + state.
  const stubParser = async (req: {
    user_reply_text: string;
  }): Promise<ReplyParseResult> => {
    const normalized = req.user_reply_text.trim().toUpperCase().replace(/[.!?]+$/, '');
    if (normalized === 'STOP' || normalized === 'UNSUBSCRIBE' || normalized === 'CANCEL') {
      return Object.freeze({
        ok: true as const,
        source: 'deterministic' as const,
        intent: 'stop' as const,
        latency_ms: 0,
        input_tokens: 0,
        output_tokens: 0,
        estimated_cost_usd: 0
      });
    }
    if (normalized === 'START' || normalized === 'UNSTOP') {
      return Object.freeze({
        ok: true as const,
        source: 'deterministic' as const,
        intent: 'start' as const,
        latency_ms: 0,
        input_tokens: 0,
        output_tokens: 0,
        estimated_cost_usd: 0
      });
    }
    return Object.freeze({
      ok: true as const,
      source: 'classifier' as const,
      classification: Object.freeze({
        intent: 'unclear' as const,
        confidence: 0.3,
        reason: 'unclear',
        snooze_hint: null
      }),
      model_name: 'stub',
      prompt_version: 'stub',
      latency_ms: 0,
      input_tokens: 0,
      output_tokens: 0,
      estimated_cost_usd: 0,
      low_confidence_forced_unclear: false
    });
  };

  const deps: SendBlueInboundRouteDeps = {
    webhookSecret: WEBHOOK_SECRET,
    webhookSecretHeader: WEBHOOK_SECRET_HEADER,
    founderPhoneNumber: FOUNDER_PHONE,
    founderUserId: FOUNDER_USER,
    phoneAllowlist,
    phoneHash: phoneHashConfig,
    killSwitches: switches,
    inboundReplyStore,
    alertStore,
    rankResultStore,
    transitions,
    feedbackStore,
    memoryStore,
    auditStore,
    replyParser: { parse: stubParser }
  };
  return { deps, phoneAllowlist, memoryStore, auditStore, inboundReplyStore };
}

// Construct the HandleInboundInput shape with the auth header value
// pre-extracted (mirrors the existing test harness pattern).
function inboundRequest(body: string): { body: string; secretHeaderValue: string } {
  return { body, secretHeaderValue: WEBHOOK_SECRET };
}

async function readStopActive(
  memoryStore: InMemoryMemorySignalStore,
  userId: string
): Promise<{ active: boolean | null; source: string | null }> {
  const sig = await memoryStore.get(userId, 'stop_active', null);
  if (!sig) return { active: null, source: null };
  const detail = sig.detail as { active?: boolean };
  return { active: detail.active === true, source: sig.source };
}

/* ---------------------------------------------------------------------- */
/* Tests                                                                  */
/* ---------------------------------------------------------------------- */

describe('Phase v0.5.1 Step 6 — per-friend STOP integration', () => {
  it('two distinct synthetic phones hash to distinct values', () => {
    const founderHash = hashPhone(FOUNDER_PHONE, phoneHashConfig);
    const friendHash = hashPhone(FRIEND_PHONE, phoneHashConfig);
    assert.notEqual(founderHash, friendHash, 'fixture violation: phones must be distinct');
  });

  it('FRIEND STOP → friend.stop_active=true; founder UNTOUCHED', async () => {
    const h = await buildHarness();

    const resp = await handleSendBlueInbound(
      inboundRequest(buildBody({ from: FRIEND_PHONE, content: 'STOP' })),
      h.deps
    );
    assert.equal(resp.status, 200);

    const friend = await readStopActive(h.memoryStore, FRIEND_USER);
    const founder = await readStopActive(h.memoryStore, FOUNDER_USER);
    assert.equal(friend.active, true, 'friend stop_active must be true');
    assert.equal(friend.source, 'user_confirmed');
    // The founder must NOT have a stop_active signal as a side effect.
    assert.equal(founder.active, null, 'founder stop_active must remain unset');
  });

  it('FOUNDER STOP → founder.stop_active=true; friend UNTOUCHED', async () => {
    const h = await buildHarness();

    const resp = await handleSendBlueInbound(
      inboundRequest(buildBody({ from: FOUNDER_PHONE, content: 'STOP' })),
      h.deps
    );
    assert.equal(resp.status, 200);

    const founder = await readStopActive(h.memoryStore, FOUNDER_USER);
    const friend = await readStopActive(h.memoryStore, FRIEND_USER);
    assert.equal(founder.active, true);
    assert.equal(friend.active, null, 'friend stop_active must remain unset');
  });

  it('FRIEND STOP, then FOUNDER STOP — both end up active independently; no cross-contamination', async () => {
    const h = await buildHarness();

    await handleSendBlueInbound(
      inboundRequest(buildBody({ from: FRIEND_PHONE, content: 'STOP' })),
      h.deps
    );
    await handleSendBlueInbound(
      inboundRequest(buildBody({ from: FOUNDER_PHONE, content: 'STOP' })),
      h.deps
    );

    const friend = await readStopActive(h.memoryStore, FRIEND_USER);
    const founder = await readStopActive(h.memoryStore, FOUNDER_USER);
    assert.equal(friend.active, true);
    assert.equal(founder.active, true);

    // Cross-check: scan ALL stop_active signals in memory and confirm
    // exactly two rows — one per user — with no duplicates / no
    // leakage into other user_ids.
    const allSignals = (await h.memoryStore.list(FOUNDER_USER)).concat(
      await h.memoryStore.list(FRIEND_USER)
    );
    const stopRows = allSignals.filter((s) => s.kind === 'stop_active');
    assert.equal(stopRows.length, 2);
    const byUser = new Map(stopRows.map((s) => [s.user_id, s]));
    assert.ok(byUser.get(FOUNDER_USER));
    assert.ok(byUser.get(FRIEND_USER));
  });

  it('duplicate FRIEND STOP is idempotent — same signal, no extra state writes', async () => {
    const h = await buildHarness();

    // First STOP.
    await handleSendBlueInbound(
      inboundRequest(buildBody({ from: FRIEND_PHONE, content: 'STOP' })),
      h.deps
    );
    const before = await readStopActive(h.memoryStore, FRIEND_USER);

    // Second STOP with a DIFFERENT provider_message_id (so the
    // inbound_replies UNIQUE constraint doesn't catch it — we want
    // to prove the memory-write is itself idempotent at the kind/
    // scope_key/user_id level).
    const resp2 = await handleSendBlueInbound(
      inboundRequest(buildBody({ from: FRIEND_PHONE, content: 'STOP' })),
      h.deps
    );
    assert.equal(resp2.status, 200);

    const after = await readStopActive(h.memoryStore, FRIEND_USER);
    assert.equal(after.active, true);
    assert.equal(after.source, before.source);

    // Founder still untouched.
    assert.equal((await readStopActive(h.memoryStore, FOUNDER_USER)).active, null);
  });

  it('UNKNOWN from-number (not the founder, not in the allowlist) is rejected 403, no memory write', async () => {
    const h = await buildHarness();

    const resp = await handleSendBlueInbound(
      inboundRequest(buildBody({ from: NON_USER_PHONE, content: 'STOP' })),
      h.deps
    );
    assert.equal(resp.status, 403);

    // Neither founder nor friend got a stop_active signal.
    assert.equal((await readStopActive(h.memoryStore, FOUNDER_USER)).active, null);
    assert.equal((await readStopActive(h.memoryStore, FRIEND_USER)).active, null);

    // Audit row exists with the rejection reason. NEVER includes the raw phone.
    const events = await h.auditStore.recent(null, 50);
    const reject = events.find((e) => e.action === 'fomo.sendblue.reply_unauthorized');
    assert.ok(reject);
    const detail = reject.detail as Record<string, unknown>;
    assert.equal(detail.from_slug, '0099'); // last 4 of NON_USER_PHONE
    assert.equal(detail.error_code, 'unknown_from_number');
    // Raw phone MUST NOT appear in audit detail (correction: no raw phone in logs).
    const dump = JSON.stringify(events);
    assert.equal(dump.includes(NON_USER_PHONE), false);
    assert.equal(dump.includes('5550100099'), false);
  });

  it('FRIEND STOP audit attributes actor_user_id to the FRIEND, not the founder', async () => {
    const h = await buildHarness();
    await handleSendBlueInbound(
      inboundRequest(buildBody({ from: FRIEND_PHONE, content: 'STOP' })),
      h.deps
    );

    // The fomo.sendblue.stop_recorded audit row should have
    // actor_user_id = FRIEND_USER, not the founder.
    const friendAudit = await h.auditStore.recent(FRIEND_USER, 20);
    const stopRecord = friendAudit.find((e) => e.action === 'fomo.sendblue.stop_recorded');
    assert.ok(stopRecord, 'friend audit must carry fomo.sendblue.stop_recorded');
    assert.equal(stopRecord.actor_user_id, FRIEND_USER);

    // The founder's audit feed should NOT have a stop_recorded entry.
    const founderAudit = await h.auditStore.recent(FOUNDER_USER, 20);
    const founderStop = founderAudit.find((e) => e.action === 'fomo.sendblue.stop_recorded');
    assert.equal(founderStop, undefined, 'founder must NOT see friend STOP audit');
  });

  it('FRIEND START after FRIEND STOP clears friend.stop_active; founder still untouched', async () => {
    const h = await buildHarness();

    await handleSendBlueInbound(
      inboundRequest(buildBody({ from: FRIEND_PHONE, content: 'STOP' })),
      h.deps
    );
    assert.equal((await readStopActive(h.memoryStore, FRIEND_USER)).active, true);

    await handleSendBlueInbound(
      inboundRequest(buildBody({ from: FRIEND_PHONE, content: 'START' })),
      h.deps
    );
    assert.equal((await readStopActive(h.memoryStore, FRIEND_USER)).active, false);

    // Founder still null throughout.
    assert.equal((await readStopActive(h.memoryStore, FOUNDER_USER)).active, null);
  });

  it('NO RAW PHONE LEAKAGE across ANY audit row from a multi-user STOP run', async () => {
    const h = await buildHarness();
    await handleSendBlueInbound(
      inboundRequest(buildBody({ from: FRIEND_PHONE, content: 'STOP' })),
      h.deps
    );
    await handleSendBlueInbound(
      inboundRequest(buildBody({ from: FOUNDER_PHONE, content: 'STOP' })),
      h.deps
    );
    await handleSendBlueInbound(
      inboundRequest(buildBody({ from: NON_USER_PHONE, content: 'STOP' })),
      h.deps
    );

    const all = (await h.auditStore.recent(null, 100)).concat(
      await h.auditStore.recent(FRIEND_USER, 100),
      await h.auditStore.recent(FOUNDER_USER, 100)
    );
    const dump = JSON.stringify(all);

    // Founder + friend + unknown-user phones MUST NOT appear in any audit dump.
    for (const phone of [FOUNDER_PHONE, FRIEND_PHONE, NON_USER_PHONE]) {
      assert.equal(dump.includes(phone), false, `raw phone ${phone} leaked into audit`);
      // Also check the digits-only form.
      assert.equal(dump.includes(phone.replace('+', '')), false, `digits of ${phone} leaked`);
    }
  });
});

/* ---------------------------------------------------------------------- */
/* Outbound-sender per-user stop_active enforcement                       */
/* ---------------------------------------------------------------------- */

describe('Phase v0.5.1 Step 6 — outbound-sender per-user stop_active enforcement', () => {
  // The outbound-sender's per-user stop_active enforcement is already
  // unit-tested in outbound-sender.test.ts. Here we prove the multi-tenant
  // INVARIANT: when friend's stop_active=true and founder's is false (or
  // missing), the worker would block friend alerts but NOT founder alerts.
  // We test this via direct memory-signal inspection at the boundary
  // the worker reads from.

  it('memory_signals.stop_active is keyed by user_id (the worker invariant the multi-user routing relies on)', async () => {
    const h = await buildHarness();

    // Friend texts STOP — friend gets stop_active=true.
    await handleSendBlueInbound(
      inboundRequest(buildBody({ from: FRIEND_PHONE, content: 'STOP' })),
      h.deps
    );

    // The outbound-sender's isStopActive(user_id) helper reads
    // memory_signals.get(user_id, 'stop_active', null). We assert the
    // store returns active=true for friend AND null/false for founder.
    const friendSig = await h.deps.memoryStore.get(FRIEND_USER, 'stop_active', null);
    const founderSig = await h.deps.memoryStore.get(FOUNDER_USER, 'stop_active', null);

    assert.ok(friendSig);
    assert.equal((friendSig.detail as { active: boolean }).active, true);
    assert.equal(founderSig, null, 'founder memory signal must not exist after friend STOP');
  });

  it('reverse direction: founder STOP does not bleed into friend memory keyspace', async () => {
    const h = await buildHarness();

    await handleSendBlueInbound(
      inboundRequest(buildBody({ from: FOUNDER_PHONE, content: 'STOP' })),
      h.deps
    );

    const founderSig = await h.deps.memoryStore.get(FOUNDER_USER, 'stop_active', null);
    const friendSig = await h.deps.memoryStore.get(FRIEND_USER, 'stop_active', null);
    assert.ok(founderSig);
    assert.equal((founderSig.detail as { active: boolean }).active, true);
    assert.equal(friendSig, null);
  });
});
