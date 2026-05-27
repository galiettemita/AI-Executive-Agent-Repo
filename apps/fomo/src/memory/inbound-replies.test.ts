import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { InMemoryInboundReplyStore } from './inbound-replies.ts';

describe('InMemoryInboundReplyStore — record (idempotency)', () => {
  it('inserts on first record + returns inserted: true', async () => {
    const store = new InMemoryInboundReplyStore();
    const out = await store.record({ provider_message_id: 'sb-msg-1', user_id: 'founder' });
    assert.equal(out.inserted, true);
    assert.equal(out.record.provider_message_id, 'sb-msg-1');
    assert.equal(out.record.user_id, 'founder');
    assert.ok(out.record.received_at);
  });

  it('LOAD-BEARING: second record with same provider_message_id returns inserted: false (SendBlue retry safety)', async () => {
    const store = new InMemoryInboundReplyStore();
    const first = await store.record({ provider_message_id: 'sb-msg-1', user_id: 'founder' });
    const dup = await store.record({ provider_message_id: 'sb-msg-1', user_id: 'founder' });
    assert.equal(first.inserted, true);
    assert.equal(dup.inserted, false);
    // The returned record is the ORIGINAL, not the duplicate input.
    assert.equal(dup.record.id, first.record.id);
    assert.equal(dup.record.received_at, first.record.received_at);
  });

  it('allows multiple rows for the same user with different provider_message_ids', async () => {
    const store = new InMemoryInboundReplyStore();
    const a = await store.record({ provider_message_id: 'sb-msg-A', user_id: 'founder' });
    const b = await store.record({ provider_message_id: 'sb-msg-B', user_id: 'founder' });
    assert.equal(a.inserted, true);
    assert.equal(b.inserted, true);
    assert.notEqual(a.record.id, b.record.id);
  });

  it('LOAD-BEARING: idempotency is GLOBAL on provider_message_id (not per-user)', async () => {
    // SendBlue's provider_message_id is unique across all messages, so
    // if two different users somehow shared one (impossible in real
    // SendBlue, but defensive), the second would still be rejected.
    const store = new InMemoryInboundReplyStore();
    const a = await store.record({ provider_message_id: 'sb-msg-shared', user_id: 'founder' });
    const b = await store.record({ provider_message_id: 'sb-msg-shared', user_id: 'other-user' });
    assert.equal(a.inserted, true);
    assert.equal(b.inserted, false);
  });
});

describe('InMemoryInboundReplyStore — getByProviderMessageId', () => {
  it('returns the row for a known provider_message_id', async () => {
    const store = new InMemoryInboundReplyStore();
    await store.record({ provider_message_id: 'sb-msg-1', user_id: 'founder' });
    const r = await store.getByProviderMessageId('sb-msg-1');
    assert.ok(r);
    assert.equal(r?.user_id, 'founder');
  });

  it('returns null for unknown provider_message_id', async () => {
    const store = new InMemoryInboundReplyStore();
    assert.equal(await store.getByProviderMessageId('does-not-exist'), null);
  });
});

describe('InMemoryInboundReplyStore — count + recent', () => {
  it('counts per user', async () => {
    const store = new InMemoryInboundReplyStore();
    await store.record({ provider_message_id: 'a', user_id: 'u1' });
    await store.record({ provider_message_id: 'b', user_id: 'u1' });
    await store.record({ provider_message_id: 'c', user_id: 'u2' });
    assert.equal(await store.count('u1'), 2);
    assert.equal(await store.count('u2'), 1);
    assert.equal(await store.count('u3'), 0);
  });

  it('recent returns newest first', async () => {
    const store = new InMemoryInboundReplyStore();
    await store.record({ provider_message_id: 'a', user_id: 'u1' });
    await new Promise((r) => setTimeout(r, 2));
    await store.record({ provider_message_id: 'b', user_id: 'u1' });
    const out = await store.recent('u1', 5);
    assert.equal(out.length, 2);
    assert.equal(out[0]?.provider_message_id, 'b');
    assert.equal(out[1]?.provider_message_id, 'a');
  });

  it('recent respects limit', async () => {
    const store = new InMemoryInboundReplyStore();
    for (let i = 0; i < 5; i++) {
      await store.record({ provider_message_id: `m${i}`, user_id: 'u1' });
    }
    const out = await store.recent('u1', 3);
    assert.equal(out.length, 3);
  });

  it('recent returns empty for non-positive limit', async () => {
    const store = new InMemoryInboundReplyStore();
    await store.record({ provider_message_id: 'a', user_id: 'u1' });
    assert.equal((await store.recent('u1', 0)).length, 0);
    assert.equal((await store.recent('u1', -1)).length, 0);
  });
});

describe('InMemoryInboundReplyStore — immutability', () => {
  it('returned records are frozen', async () => {
    const store = new InMemoryInboundReplyStore();
    const out = await store.record({ provider_message_id: 'a', user_id: 'u1' });
    assert.throws(() => {
      (out.record as unknown as { user_id: string }).user_id = 'mutated';
    });
  });
});
