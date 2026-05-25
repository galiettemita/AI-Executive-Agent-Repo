import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { InMemoryRankResultStore, type RankResultInput } from './rank-results.ts';

function fixture(over: Partial<RankResultInput> = {}): RankResultInput {
  return {
    user_id: 'u-1',
    message_id: 'msg-100',
    invocation_id: 'inv-1',
    model_name: 'gpt-5-mini',
    prompt_version: 'fomo-ranker-v1',
    label: 'important',
    score: 0.82,
    reason: 'Mentions deadline today.',
    latency_ms: 412,
    input_tokens: 380,
    output_tokens: 24,
    estimated_cost_usd: 0.0009,
    ...over
  };
}

describe('InMemoryRankResultStore — write + get', () => {
  it('returns null when nothing stored', async () => {
    const store = new InMemoryRankResultStore();
    assert.equal(await store.get('u-1', 'msg-1'), null);
  });

  it('write + get round-trip preserves every field', async () => {
    const store = new InMemoryRankResultStore();
    const outcome = await store.write(fixture());
    assert.equal(outcome.inserted, true);
    const row = await store.get('u-1', 'msg-100');
    assert.ok(row);
    assert.equal(row?.user_id, 'u-1');
    assert.equal(row?.message_id, 'msg-100');
    assert.equal(row?.invocation_id, 'inv-1');
    assert.equal(row?.model_name, 'gpt-5-mini');
    assert.equal(row?.prompt_version, 'fomo-ranker-v1');
    assert.equal(row?.label, 'important');
    assert.equal(row?.score, 0.82);
    assert.equal(row?.reason, 'Mentions deadline today.');
    assert.equal(row?.latency_ms, 412);
    assert.equal(row?.input_tokens, 380);
    assert.equal(row?.output_tokens, 24);
    assert.equal(row?.estimated_cost_usd, 0.0009);
    assert.ok(row?.created_at);
  });

  it('returned row is frozen', async () => {
    const store = new InMemoryRankResultStore();
    await store.write(fixture());
    const row = await store.get('u-1', 'msg-100');
    assert.ok(row);
    assert.throws(() => {
      (row as unknown as { label: string }).label = 'not_important';
    });
  });
});

describe('InMemoryRankResultStore — idempotency on (user_id, message_id)', () => {
  it('second write for same (user_id, message_id) reports inserted=false', async () => {
    const store = new InMemoryRankResultStore();
    const first = await store.write(fixture());
    assert.equal(first.inserted, true);
    const second = await store.write(fixture({ label: 'not_important', score: 0.1 }));
    assert.equal(second.inserted, false);
  });

  it('existing row is unchanged after a duplicate write', async () => {
    const store = new InMemoryRankResultStore();
    await store.write(fixture());
    await store.write(fixture({ label: 'not_important', score: 0.1, reason: 'Different reason.' }));
    const row = await store.get('u-1', 'msg-100');
    // First write wins.
    assert.equal(row?.label, 'important');
    assert.equal(row?.score, 0.82);
    assert.equal(row?.reason, 'Mentions deadline today.');
  });

  it('same message_id across different users does not collide', async () => {
    const store = new InMemoryRankResultStore();
    const a = await store.write(fixture({ user_id: 'u-1', message_id: 'msg-shared' }));
    const b = await store.write(fixture({ user_id: 'u-2', message_id: 'msg-shared' }));
    assert.equal(a.inserted, true);
    assert.equal(b.inserted, true);
  });
});

describe('InMemoryRankResultStore — count', () => {
  it('returns 0 for an empty user', async () => {
    const store = new InMemoryRankResultStore();
    assert.equal(await store.count('u-none'), 0);
    assert.equal(await store.count('u-none', 'important'), 0);
  });

  it('counts all rows for the user when label omitted', async () => {
    const store = new InMemoryRankResultStore();
    await store.write(fixture({ message_id: 'm-1', label: 'important' }));
    await store.write(fixture({ message_id: 'm-2', label: 'not_important' }));
    await store.write(fixture({ message_id: 'm-3', label: 'important' }));
    assert.equal(await store.count('u-1'), 3);
  });

  it('counts only matching label when given', async () => {
    const store = new InMemoryRankResultStore();
    await store.write(fixture({ message_id: 'm-1', label: 'important' }));
    await store.write(fixture({ message_id: 'm-2', label: 'not_important' }));
    await store.write(fixture({ message_id: 'm-3', label: 'important' }));
    assert.equal(await store.count('u-1', 'important'), 2);
    assert.equal(await store.count('u-1', 'not_important'), 1);
  });

  it('per-user isolation', async () => {
    const store = new InMemoryRankResultStore();
    await store.write(fixture({ user_id: 'u-1', message_id: 'm-1' }));
    await store.write(fixture({ user_id: 'u-2', message_id: 'm-1' }));
    await store.write(fixture({ user_id: 'u-2', message_id: 'm-2' }));
    assert.equal(await store.count('u-1'), 1);
    assert.equal(await store.count('u-2'), 2);
  });
});

describe('InMemoryRankResultStore — recent', () => {
  it('returns empty when no rows', async () => {
    const store = new InMemoryRankResultStore();
    assert.deepEqual([...(await store.recent('u-1', 10))], []);
  });

  it('returns up to limit rows, newest first by id (created_at ties)', async () => {
    const store = new InMemoryRankResultStore();
    await store.write(fixture({ message_id: 'm-1' }));
    await store.write(fixture({ message_id: 'm-2' }));
    await store.write(fixture({ message_id: 'm-3' }));
    const rows = await store.recent('u-1', 2);
    assert.equal(rows.length, 2);
    // Newest first (highest id wins on created_at ties)
    assert.equal(rows[0]?.message_id, 'm-3');
    assert.equal(rows[1]?.message_id, 'm-2');
  });

  it('returns empty on non-positive limits', async () => {
    const store = new InMemoryRankResultStore();
    await store.write(fixture());
    assert.deepEqual([...(await store.recent('u-1', 0))], []);
    assert.deepEqual([...(await store.recent('u-1', -1))], []);
    assert.deepEqual([...(await store.recent('u-1', 1.5))], []);
  });

  it('respects per-user scope', async () => {
    const store = new InMemoryRankResultStore();
    await store.write(fixture({ user_id: 'u-1', message_id: 'm-1' }));
    await store.write(fixture({ user_id: 'u-2', message_id: 'm-2' }));
    const rows = await store.recent('u-1', 10);
    assert.equal(rows.length, 1);
    assert.equal(rows[0]?.user_id, 'u-1');
  });

  it('returned list is frozen', async () => {
    const store = new InMemoryRankResultStore();
    await store.write(fixture());
    const rows = await store.recent('u-1', 1);
    assert.throws(() => {
      (rows as unknown as { length: number; push: (n: unknown) => void }).push({});
    });
  });
});
