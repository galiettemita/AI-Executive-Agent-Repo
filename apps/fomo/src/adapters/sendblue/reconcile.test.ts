// Phase v0.5.3 item #4 — SendBlue reconciliation regression tests.
//
// v0.5.2 incident: a SendBlue-confirmed STOP iMessage from a real
// friend never reached our webhook (server was down + retries
// exhausted), and we had no visibility into the gap until manually
// querying SendBlue's /api/v2/messages 11h later. These tests pin
// the gap-detection contract:
//   - SendBlue inbound NOT in our audit → flagged as gap + audited
//   - SendBlue inbound IN our audit → NOT flagged
//   - Outbound messages → never flagged (gap concept doesn't apply)
//   - Messages older than the window boundary → filtered out
//   - Pagination is bounded; non-2xx aborts with a clear error
//   - Audit detail surfaces ONLY safe fields (no body content, no full E.164)

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import {
  reconcileSendBlue,
  phoneSlugFromMessage,
  type ReconcileDeps,
  type SendBlueMessage
} from './reconcile.ts';

function fakeMessage(overrides: Partial<SendBlueMessage> = {}): SendBlueMessage {
  return Object.freeze({
    message_handle: overrides.message_handle ?? 'sb-handle-default',
    is_outbound: overrides.is_outbound ?? false,
    status: overrides.status ?? 'RECEIVED',
    service: overrides.service ?? 'iMessage',
    from_number: overrides.from_number ?? '+19295558367',
    to_number: overrides.to_number ?? '+12143547196',
    date_sent: overrides.date_sent ?? '2026-06-01T20:00:00Z'
  });
}

function mockSendBlue(pages: SendBlueMessage[][]): typeof fetch {
  let callIndex = 0;
  return (async () => {
    const page = pages[callIndex] ?? [];
    const hasMore = callIndex + 1 < pages.length;
    callIndex++;
    return new Response(
      JSON.stringify({
        status: 'OK',
        data: page,
        pagination: { total: pages.flat().length, limit: 50, offset: 0, hasMore }
      }),
      { status: 200, headers: { 'content-type': 'application/json' } }
    );
  }) as typeof fetch;
}

function harness(opts: {
  pages?: SendBlueMessage[][];
  auditHandles?: Set<string>;
  windowHours?: number;
  now?: () => number;
}): {
  deps: ReconcileDeps;
  recordedGaps: SendBlueMessage[];
} {
  const recordedGaps: SendBlueMessage[] = [];
  const deps: ReconcileDeps = {
    apiKeyId: 'fake-key',
    apiSecretKey: 'fake-secret',
    windowHours: opts.windowHours ?? 24,
    now: opts.now ?? (() => Date.parse('2026-06-01T22:00:00Z')),
    fetchImpl: mockSendBlue(opts.pages ?? []),
    fetchAuditedHandles: async () => opts.auditHandles ?? new Set<string>(),
    recordGap: async (msg) => {
      recordedGaps.push(msg);
    }
  };
  return { deps, recordedGaps };
}

describe('reconcileSendBlue — gap detection (Phase v0.5.3 item #4)', () => {
  it('detects an inbound that SendBlue has but our audit does not (the v0.5.2 incident shape)', async () => {
    // SendBlue knows about Morris's STOP message. Our audit log has
    // NO trace because the server was down at delivery time.
    const stopMsg = fakeMessage({
      message_handle: '73179539-F023-4B99-ACE4-0FB1CBBB6A12',
      from_number: '+19293818367',
      date_sent: '2026-06-01T11:08:50Z'
    });
    const h = harness({
      pages: [[stopMsg]],
      auditHandles: new Set() // empty — the gap
    });

    const result = await reconcileSendBlue(h.deps);

    assert.equal(result.sendblue_inbound_count, 1);
    assert.equal(result.audit_handles_in_window, 0);
    assert.equal(result.gaps_found, 1);
    assert.deepEqual([...result.gap_handles], ['73179539-F023-4B99-ACE4-0FB1CBBB6A12']);
    // Audit row was recorded for the gap.
    assert.equal(h.recordedGaps.length, 1);
    assert.equal(h.recordedGaps[0].message_handle, '73179539-F023-4B99-ACE4-0FB1CBBB6A12');
  });

  it('does NOT flag an inbound that IS in our audit (happy path — webhook fired)', async () => {
    const msg = fakeMessage({ message_handle: 'sb-handle-ok' });
    const h = harness({
      pages: [[msg]],
      auditHandles: new Set(['sb-handle-ok'])
    });

    const result = await reconcileSendBlue(h.deps);

    assert.equal(result.sendblue_inbound_count, 1);
    assert.equal(result.audit_handles_in_window, 1);
    assert.equal(result.gaps_found, 0);
    assert.equal(h.recordedGaps.length, 0);
  });

  it('ignores outbound messages (gap concept only applies to inbound webhook deliveries)', async () => {
    const inboundMsg = fakeMessage({ message_handle: 'sb-in-1' });
    const outboundMsg = fakeMessage({ message_handle: 'sb-out-1', is_outbound: true });
    const h = harness({
      pages: [[inboundMsg, outboundMsg]],
      auditHandles: new Set() // both would be "gaps" if we didn't filter
    });

    const result = await reconcileSendBlue(h.deps);

    // Only the inbound counts toward sendblue_inbound_count + gaps.
    assert.equal(result.sendblue_inbound_count, 1);
    assert.equal(result.gaps_found, 1);
    assert.deepEqual([...result.gap_handles], ['sb-in-1']);
  });

  it('filters out messages older than the window boundary', async () => {
    // Window = 24h ending at 2026-06-01T22:00Z, so messages before
    // 2026-05-31T22:00Z are out.
    const insideWindow = fakeMessage({
      message_handle: 'recent',
      date_sent: '2026-06-01T10:00:00Z'
    });
    const outsideWindow = fakeMessage({
      message_handle: 'too-old',
      date_sent: '2026-05-29T10:00:00Z'
    });
    const h = harness({
      pages: [[insideWindow, outsideWindow]],
      auditHandles: new Set()
    });

    const result = await reconcileSendBlue(h.deps);

    assert.equal(result.sendblue_inbound_count, 1);
    assert.equal(result.gaps_found, 1);
    assert.deepEqual([...result.gap_handles], ['recent']);
  });

  it('aborts with a clear error when SendBlue API returns non-2xx', async () => {
    const deps: ReconcileDeps = {
      apiKeyId: 'fake',
      apiSecretKey: 'fake',
      windowHours: 24,
      now: () => Date.parse('2026-06-01T22:00:00Z'),
      fetchImpl: (async () =>
        new Response('forbidden', { status: 403 })) as typeof fetch,
      fetchAuditedHandles: async () => new Set(),
      recordGap: async () => undefined
    };

    await assert.rejects(reconcileSendBlue(deps), /HTTP 403/);
  });

  it('recordGap failure does NOT abort reconciliation; gap_handles still complete', async () => {
    const msgA = fakeMessage({ message_handle: 'sb-a' });
    const msgB = fakeMessage({ message_handle: 'sb-b' });
    let recordCalls = 0;
    const deps: ReconcileDeps = {
      apiKeyId: 'fake',
      apiSecretKey: 'fake',
      windowHours: 24,
      now: () => Date.parse('2026-06-01T22:00:00Z'),
      fetchImpl: mockSendBlue([[msgA, msgB]]),
      fetchAuditedHandles: async () => new Set(),
      recordGap: async () => {
        recordCalls++;
        throw new Error('audit-store write failed');
      }
    };

    const result = await reconcileSendBlue(deps);

    // Both attempts were made; result still reports both gaps.
    assert.equal(recordCalls, 2);
    assert.equal(result.gaps_found, 2);
    assert.deepEqual([...result.gap_handles].sort(), ['sb-a', 'sb-b']);
  });

  it('phoneSlugFromMessage returns the last 4 digits, never the full E.164', () => {
    assert.equal(phoneSlugFromMessage(fakeMessage({ from_number: '+19293818367' })), '8367');
    assert.equal(phoneSlugFromMessage(fakeMessage({ from_number: '+12143547196' })), '7196');
    assert.equal(phoneSlugFromMessage(fakeMessage({ from_number: '' })), '');
  });

  it('paginates: keeps fetching while hasMore=true and inside the window; stops on boundary crossing', async () => {
    // Page 1: 2 recent inbounds. Page 2: 1 inbound at the boundary.
    // Should stop after page 2 even though there are no more messages
    // anyway.
    const recent1 = fakeMessage({ message_handle: 'p1-a', date_sent: '2026-06-01T20:00:00Z' });
    const recent2 = fakeMessage({ message_handle: 'p1-b', date_sent: '2026-06-01T19:00:00Z' });
    const tooOld = fakeMessage({ message_handle: 'p2-old', date_sent: '2026-05-28T00:00:00Z' });
    const h = harness({
      pages: [[recent1, recent2], [tooOld]],
      auditHandles: new Set()
    });
    const result = await reconcileSendBlue(h.deps);
    // 2 recent inbounds; the old one is filtered.
    assert.equal(result.sendblue_inbound_count, 2);
    assert.equal(result.gaps_found, 2);
  });
});
