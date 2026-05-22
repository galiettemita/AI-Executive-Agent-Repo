import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { InMemoryPendingMessageStore } from './pending-message-store.ts';

describe('InMemoryPendingMessageStore', () => {
  it('round-trips put -> peek -> consume', async () => {
    const store = new InMemoryPendingMessageStore();
    await store.put({
      pending_message_id: 'pm-1',
      user_id: 'u1',
      original_text: 'send my email',
      channel: 'API',
      session_id: 'sess-1'
    });
    const peeked = await store.peek('pm-1', 'u1');
    assert.equal(peeked?.original_text, 'send my email');
    const consumed = await store.consume('pm-1', 'u1');
    assert.equal(consumed?.original_text, 'send my email');
    const second = await store.consume('pm-1', 'u1');
    assert.equal(second, null);
  });

  it('rejects cross-user access', async () => {
    const store = new InMemoryPendingMessageStore();
    await store.put({
      pending_message_id: 'pm-2',
      user_id: 'alice',
      original_text: 'private',
      channel: null,
      session_id: null
    });
    assert.equal(await store.peek('pm-2', 'bob'), null);
    assert.equal(await store.consume('pm-2', 'bob'), null);
    // Alice can still consume
    assert.ok(await store.consume('pm-2', 'alice'));
  });

  it('returns null for missing or expired rows', async () => {
    const store = new InMemoryPendingMessageStore();
    assert.equal(await store.peek('nope', 'u1'), null);
    assert.equal(await store.consume('nope', 'u1'), null);
  });

  it('atomic consume prevents double-redeem under concurrent attempts', async () => {
    const store = new InMemoryPendingMessageStore();
    await store.put({
      pending_message_id: 'pm-race',
      user_id: 'u1',
      original_text: 'msg',
      channel: null,
      session_id: null
    });
    // Two concurrent consumes; only one should win
    const [a, b] = await Promise.all([
      store.consume('pm-race', 'u1'),
      store.consume('pm-race', 'u1')
    ]);
    const wins = [a, b].filter(Boolean);
    assert.equal(wins.length, 1, `expected exactly one consumer to win, got ${wins.length}`);
  });

  it('prune removes stale rows', async () => {
    const store = new InMemoryPendingMessageStore();
    await store.put({
      pending_message_id: 'old',
      user_id: 'u1',
      original_text: 'msg',
      channel: null,
      session_id: null
    });
    // Force the row to look old by manipulating internals via a small future-now
    const future = Date.now() + 60 * 60 * 1000;
    const pruned = await store.prune(future);
    assert.equal(pruned, 1);
    assert.equal(await store.peek('old', 'u1'), null);
  });
});
