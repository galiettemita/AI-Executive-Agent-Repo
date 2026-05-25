import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { InMemoryAlertStore, type AlertInput } from './alerts.ts';

function fixture(over: Partial<AlertInput> = {}): AlertInput {
  return {
    alert_id: 'alert-001',
    user_id: 'u-1',
    message_id: 'msg-100',
    rank_result_id: 1,
    label: 'important',
    score: 0.85,
    ...over
  };
}

describe('InMemoryAlertStore — create + get', () => {
  it('returns null when nothing stored', async () => {
    const store = new InMemoryAlertStore();
    assert.equal(await store.get('alert-none'), null);
    assert.equal(await store.getByRankResult(999), null);
  });

  it('create + get round-trip preserves every field', async () => {
    const store = new InMemoryAlertStore();
    const outcome = await store.create(fixture());
    assert.equal(outcome.inserted, true);
    assert.equal(outcome.alert.alert_id, 'alert-001');
    const row = await store.get('alert-001');
    assert.ok(row);
    assert.equal(row?.user_id, 'u-1');
    assert.equal(row?.message_id, 'msg-100');
    assert.equal(row?.rank_result_id, 1);
    assert.equal(row?.label, 'important');
    assert.equal(row?.score, 0.85);
    assert.ok(row?.created_at);
  });

  it('returned alert is frozen', async () => {
    const store = new InMemoryAlertStore();
    const out = await store.create(fixture());
    assert.throws(() => {
      (out.alert as unknown as { label: string }).label = 'not_important';
    });
  });

  it('getByRankResult finds the alert', async () => {
    const store = new InMemoryAlertStore();
    await store.create(fixture({ alert_id: 'alert-A', rank_result_id: 7 }));
    const row = await store.getByRankResult(7);
    assert.equal(row?.alert_id, 'alert-A');
  });
});

describe('InMemoryAlertStore — idempotency on rank_result_id (CORE Phase 3D.1 invariant)', () => {
  it('second create for SAME rank_result_id reports inserted=false', async () => {
    const store = new InMemoryAlertStore();
    const first = await store.create(fixture({ alert_id: 'alert-A', rank_result_id: 1 }));
    assert.equal(first.inserted, true);
    const second = await store.create(fixture({ alert_id: 'alert-B-different-id', rank_result_id: 1 }));
    assert.equal(second.inserted, false);
  });

  it('returns the EXISTING row (not the new input) on idempotency hit', async () => {
    const store = new InMemoryAlertStore();
    await store.create(fixture({ alert_id: 'alert-original', rank_result_id: 1, score: 0.85 }));
    const second = await store.create(
      fixture({ alert_id: 'alert-attempted-overwrite', rank_result_id: 1, score: 0.1 })
    );
    // The original alert wins; the second create returns the original.
    assert.equal(second.inserted, false);
    assert.equal(second.alert.alert_id, 'alert-original');
    assert.equal(second.alert.score, 0.85);
  });

  it('existing row is unchanged after a duplicate create', async () => {
    const store = new InMemoryAlertStore();
    await store.create(fixture({ alert_id: 'alert-A', rank_result_id: 1 }));
    await store.create(fixture({ alert_id: 'alert-B', rank_result_id: 1, score: 0.1 }));
    const row = await store.get('alert-A');
    assert.equal(row?.score, 0.85);
    // The second attempted alert_id was NOT inserted:
    assert.equal(await store.get('alert-B'), null);
  });

  it('different rank_result_ids → different alerts (per-rank scoping holds)', async () => {
    const store = new InMemoryAlertStore();
    const a = await store.create(fixture({ alert_id: 'alert-A', rank_result_id: 1 }));
    const b = await store.create(fixture({ alert_id: 'alert-B', rank_result_id: 2 }));
    assert.equal(a.inserted, true);
    assert.equal(b.inserted, true);
  });
});

describe('InMemoryAlertStore — count + recent', () => {
  it('count: 0 for an empty user', async () => {
    const store = new InMemoryAlertStore();
    assert.equal(await store.count('u-none'), 0);
  });

  it('count + recent reflect inserts', async () => {
    const store = new InMemoryAlertStore();
    await store.create(fixture({ alert_id: 'a1', rank_result_id: 1, message_id: 'm1' }));
    await store.create(fixture({ alert_id: 'a2', rank_result_id: 2, message_id: 'm2' }));
    await store.create(fixture({ alert_id: 'a3', rank_result_id: 3, message_id: 'm3' }));
    assert.equal(await store.count('u-1'), 3);
    const recent = await store.recent('u-1', 2);
    assert.equal(recent.length, 2);
  });

  it('per-user isolation', async () => {
    const store = new InMemoryAlertStore();
    await store.create(fixture({ user_id: 'u-1', alert_id: 'a1', rank_result_id: 1 }));
    await store.create(fixture({ user_id: 'u-2', alert_id: 'a2', rank_result_id: 2 }));
    assert.equal(await store.count('u-1'), 1);
    assert.equal(await store.count('u-2'), 1);
    const r1 = await store.recent('u-1', 10);
    assert.equal(r1[0]?.user_id, 'u-1');
  });

  it('recent: rejects non-positive limits', async () => {
    const store = new InMemoryAlertStore();
    await store.create(fixture());
    assert.deepEqual([...(await store.recent('u-1', 0))], []);
    assert.deepEqual([...(await store.recent('u-1', -1))], []);
    assert.deepEqual([...(await store.recent('u-1', 1.5))], []);
  });

  it('returned list is frozen', async () => {
    const store = new InMemoryAlertStore();
    await store.create(fixture());
    const rows = await store.recent('u-1', 1);
    assert.throws(() => {
      (rows as unknown as { push: (n: unknown) => void }).push({});
    });
  });
});
