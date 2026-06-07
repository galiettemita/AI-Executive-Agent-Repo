// Phase v0.5.13 — Worker-level PIL allowlist gate tests.
//
// Locked 4-case truth table (per memory project_v05-13-scope):
//   | FOMO_PIL_LIVE_ENABLED | FOMO_PIL_LIVE_USER_ALLOWLIST | Behavior                          |
//   |----------------------|------------------------------|------------------------------------|
//   | false                | (any)                        | Bit-identical v0.5.11 for all users; pilLive=null in prod, but here we mimic by enabled=false on the mock |
//   | true                 | []                           | All users baseline-only (fail-closed at the gate); fomo.pil_live.allowlist_empty WARN at boot (covered by index.ts test, not this file) |
//   | true                 | [founder]                    | Only founder hybrid; non-founder users baseline-only |
//   | true                 | [a, b]                       | Both hybrid (generic mechanism; one assertion suffices here) |
//
// Founder correction #1 (trim-only, no-lowercase): a 5th case asserts
// that case-different user_id strings do NOT match via ===.
//
// These tests construct minimal mocks for the PIL dep (`rankWithPil` and
// `buildLivePilContext`) and the `ranker` dep so we can observe which path
// the worker took for each (kill-switch, allowlist, user_id) combination,
// plus verify the new `messages_pil_skipped_not_in_allowlist` counter.

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { GMAIL_READONLY_SCOPE, GmailClient } from '../adapters/gmail/client.ts';
import { InMemoryAuditStore } from '../core/audit.ts';
import { SAFE_DEFAULT_KILL_SWITCHES } from '../core/kill-switches.ts';
import { type PolicyGateDeps } from '../core/policy-gate.ts';
import { InMemoryToolInvocationStore } from '../core/tool-invocations.ts';
import { createToolRegistry } from '../core/tool-registry.ts';
import { createDispatchTable } from '../dispatch/dispatcher.ts';
import { wireExternalExecutors } from '../dispatch/external-executors.ts';
import { InMemoryGmailCursorStore } from '../memory/gmail-cursors.ts';
import { InMemoryRankResultStore } from '../memory/rank-results.ts';
import { type RankerResult } from '../ranker/index.ts';
import { loadCryptoConfig } from '../security/token-crypto.ts';
import { InMemoryTokenStore } from '../security/oauth/token-store.ts';

import { runOnce, type GmailPollPilLiveDep } from './gmail-poll.ts';

const TEST_KEK = Buffer.alloc(32, 13).toString('base64');
const SENDER_HASH_KEY = Buffer.alloc(32, 7);

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

function fakeMessageJSON(id: string): unknown {
  return {
    id,
    threadId: `thr-${id}`,
    internalDate: '1700000000000',
    payload: {
      headers: [
        { name: 'From', value: 'Sarah <sarah@example.com>' },
        { name: 'Subject', value: 'subject ' + id }
      ],
      mimeType: 'text/plain',
      body: { data: 'SGkgU2FyYWg' }
    }
  };
}

function rankerSuccess(over: Partial<Extract<RankerResult, { ok: true }>> = {}): RankerResult {
  return Object.freeze({
    ok: true as const,
    decision: { label: 'not_important' as const, score: 0.2, reason: 'baseline' },
    model_name: 'gpt-5-mini',
    prompt_version: 'ranker-v0.2.0',
    latency_ms: 100,
    input_tokens: 200,
    output_tokens: 10,
    estimated_cost_usd: 0.0001,
    ...over
  });
}

interface ScenarioOpts {
  pil_live_enabled: boolean;
  allowlist: readonly string[];
  user_id: string;
}

interface ScenarioResult {
  pilCalls: number;
  baselineCalls: number;
  outcomeStatus: string;
  pilSkippedCounter: number;
  cyclePilSkippedAggregate: number;
}

async function runScenario(opts: ScenarioOpts): Promise<ScenarioResult> {
  const historyMap = new Map<string, { status: number; body: unknown }>();
  historyMap.set('h-100', {
    status: 200,
    body: {
      history: [{ id: 'h-101', messagesAdded: [{ message: { id: 'm-1', threadId: 't-1' } }] }],
      historyId: 'h-200'
    }
  });

  const fetchImpl: typeof fetch = (async (input: string | URL | Request) => {
    const url = typeof input === 'string' ? input : input.toString();
    const historyMatch = /\/users\/me\/history\?.*startHistoryId=([^&]+)/.exec(url);
    if (historyMatch) {
      const id = decodeURIComponent(historyMatch[1] ?? '');
      const resp = historyMap.get(id);
      if (!resp) {
        return new Response(JSON.stringify({ historyId: id }), {
          status: 200,
          headers: { 'content-type': 'application/json' }
        });
      }
      return new Response(JSON.stringify(resp.body), {
        status: resp.status,
        headers: { 'content-type': 'application/json' }
      });
    }
    const msgMatch = /\/users\/me\/messages\/([^?]+)/.exec(url);
    if (msgMatch) {
      const messageId = decodeURIComponent(msgMatch[1] ?? '');
      return new Response(JSON.stringify(fakeMessageJSON(messageId)), {
        status: 200,
        headers: { 'content-type': 'application/json' }
      });
    }
    return new Response(JSON.stringify({}), {
      status: 404,
      headers: { 'content-type': 'application/json' }
    });
  }) as typeof fetch;

  const gmailClient = new GmailClient({ fetchImpl });
  const tokenStore = new InMemoryTokenStore(cryptoConfig);
  const cursorStore = new InMemoryGmailCursorStore();
  const auditStore = new InMemoryAuditStore();
  const toolInvocationStore = new InMemoryToolInvocationStore();
  const dispatch = createDispatchTable();
  wireExternalExecutors(dispatch, { gmailClient, tokenStore });
  const registry = createToolRegistry();
  const gateDeps: PolicyGateDeps = {
    registry,
    switches: SAFE_DEFAULT_KILL_SWITCHES,
    hasConsent: () => true,
    hasOAuth: () => true
  };
  await tokenStore.save({
    user_id: opts.user_id,
    provider: 'google',
    scopes: [GMAIL_READONLY_SCOPE],
    access_token: 'tok-' + opts.user_id
  });
  await cursorStore.upsert({ user_id: opts.user_id, history_id: 'h-100' });

  const rankResultStore = new InMemoryRankResultStore();
  let baselineCalls = 0;
  let pilCalls = 0;
  const ranker = {
    rank: async (_req: { raw: unknown; user_id: string }): Promise<RankerResult> => {
      baselineCalls++;
      return rankerSuccess();
    },
    store: rankResultStore
  };

  const pilLive: GmailPollPilLiveDep = {
    enabled: opts.pil_live_enabled,
    allowlist: opts.allowlist,
    buildLivePilContext: async () => {
      pilCalls++;
      // Returning null is fine — the assertion is "was the PIL path entered",
      // which we count via this call. The downstream rankWithPil tracking
      // would double-count, so we keep PIL invocation observability here.
      return null;
    },
    rankWithPil: async () => {
      // The worker enters this path only when enabled AND allowlist match
      // AND senderHashKey present. Returning a ranker-like result keeps the
      // worker happy. We don't double-count here; pilCalls (from
      // buildLivePilContext above) is the canonical "PIL path entered" count.
      return {
        result: rankerSuccess({ prompt_version: 'ranker-v0.3.0' }),
        audit_emitted_for_rank_result_id: null,
        audit_payload: null,
        baseline_result: null,
        pil_result: null
      };
    }
  };

  const r = await runOnce({
    gmailClient,
    tokenStore,
    cursorStore,
    dispatch,
    auditStore,
    toolInvocationStore,
    gateDeps,
    ranker: ranker as Parameters<typeof runOnce>[0]['ranker'],
    pilLive,
    senderHashKey: SENDER_HASH_KEY,
    newInvocationId: () => 'inv-1',
    now: () => 1_700_000_000_000
  });

  const outcome = r.outcomes[0];
  const outcomeStatus = outcome?.status ?? 'no_outcome';
  const pilSkippedCounter =
    outcome && outcome.status === 'polled'
      ? outcome.messages_pil_skipped_not_in_allowlist
      : 0;
  return {
    pilCalls,
    baselineCalls,
    outcomeStatus,
    pilSkippedCounter,
    cyclePilSkippedAggregate: r.messages_pil_skipped_not_in_allowlist
  };
}

/* ====================================================================== */
/* 4-case truth table                                                     */
/* ====================================================================== */

describe('worker-level allowlist gate — case (a): pil_live_enabled=false', () => {
  it('takes baseline path for ALL users regardless of allowlist content', async () => {
    // Even with founder in the allowlist, the global kill switch off keeps
    // the worker on the bit-identical-v0.5.11 path.
    const r = await runScenario({
      pil_live_enabled: false,
      allowlist: ['founder', 'u-other'],
      user_id: 'founder'
    });
    assert.equal(r.pilCalls, 0);
    assert.equal(r.baselineCalls, 1);
    assert.equal(
      r.pilSkippedCounter,
      0,
      'counter does NOT tick when global is off (different reason for skipping; existing counters cover it)'
    );
    assert.equal(r.cyclePilSkippedAggregate, 0);
  });
});

describe('worker-level allowlist gate — case (b): pil_live_enabled=true + empty allowlist (fail-closed)', () => {
  it('takes baseline path for ALL users; counter ticks per skipped rank', async () => {
    const r = await runScenario({
      pil_live_enabled: true,
      allowlist: [],
      user_id: 'founder'
    });
    assert.equal(r.pilCalls, 0);
    assert.equal(r.baselineCalls, 1);
    assert.equal(
      r.pilSkippedCounter,
      1,
      'counter MUST tick — PIL was eligible (enabled + senderHashKey present) but allowlist gate blocked the user'
    );
    assert.equal(r.cyclePilSkippedAggregate, 1);
  });
});

describe('worker-level allowlist gate — case (c): pil_live_enabled=true + allowlist=[founder]', () => {
  it('founder gets PIL hybrid path; non-founder users get baseline + counter ticks', async () => {
    // 7c.1 — founder IS in the allowlist → PIL hybrid path.
    const founderResult = await runScenario({
      pil_live_enabled: true,
      allowlist: ['founder'],
      user_id: 'founder'
    });
    assert.equal(founderResult.pilCalls, 1, 'founder hits the PIL path');
    assert.equal(founderResult.baselineCalls, 0, 'founder does NOT hit the baseline path');
    assert.equal(founderResult.pilSkippedCounter, 0, 'no skip — founder is in the list');

    // 7c.2 — non-founder user NOT in the allowlist → baseline path + counter ticks.
    const otherResult = await runScenario({
      pil_live_enabled: true,
      allowlist: ['founder'],
      user_id: 'u-other'
    });
    assert.equal(otherResult.pilCalls, 0, 'non-founder user does NOT hit the PIL path');
    assert.equal(otherResult.baselineCalls, 1, 'non-founder user takes the baseline path');
    assert.equal(otherResult.pilSkippedCounter, 1, 'counter ticks for the skipped rank');
  });
});

describe('worker-level allowlist gate — case (d): pil_live_enabled=true + allowlist=[a, b] (multi-user generic mechanism)', () => {
  it('both listed users get PIL hybrid path; users NOT in the list get baseline', async () => {
    const a = await runScenario({
      pil_live_enabled: true,
      allowlist: ['userA', 'userB'],
      user_id: 'userA'
    });
    assert.equal(a.pilCalls, 1);
    assert.equal(a.baselineCalls, 0);

    const b = await runScenario({
      pil_live_enabled: true,
      allowlist: ['userA', 'userB'],
      user_id: 'userB'
    });
    assert.equal(b.pilCalls, 1);
    assert.equal(b.baselineCalls, 0);

    const c = await runScenario({
      pil_live_enabled: true,
      allowlist: ['userA', 'userB'],
      user_id: 'userC'
    });
    assert.equal(c.pilCalls, 0);
    assert.equal(c.baselineCalls, 1);
    assert.equal(c.pilSkippedCounter, 1);
  });
});

/* ====================================================================== */
/* Founder correction #1: trim-only, no-lowercase exact-match              */
/* ====================================================================== */

describe('worker-level allowlist gate — founder correction #1 (trim-only, no-lowercase)', () => {
  it('does NOT match a case-different user_id against the allowlist (strict === comparison)', async () => {
    // The allowlist contains 'Founder' (uppercase F). The polled user is
    // 'founder' (lowercase f). Per founder correction #1, the parser
    // preserves exact case so the worker gate's === comparison catches the
    // mismatch. The expected behavior is: PIL path is NOT taken; baseline
    // path is; counter ticks.
    const r = await runScenario({
      pil_live_enabled: true,
      allowlist: ['Founder'],
      user_id: 'founder'
    });
    assert.equal(
      r.pilCalls,
      0,
      'case-different user_id MUST NOT take the PIL path (no silent lowercase fallback)'
    );
    assert.equal(r.baselineCalls, 1);
    assert.equal(r.pilSkippedCounter, 1);
  });

  it('DOES match exact-case user_id even with leading/trailing whitespace in env (parser trimmed)', async () => {
    // This case is exercised at the parser level (kill-switches.test.ts),
    // but we re-assert at the worker level: when the allowlist entry was
    // 'founder' (post-trim), a user_id 'founder' matches.
    const r = await runScenario({
      pil_live_enabled: true,
      allowlist: ['founder'], // post-parser shape
      user_id: 'founder'
    });
    assert.equal(r.pilCalls, 1);
    assert.equal(r.baselineCalls, 0);
    assert.equal(r.pilSkippedCounter, 0);
  });
});
