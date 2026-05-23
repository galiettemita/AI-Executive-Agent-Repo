import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { InMemoryGmailCursorStore } from './gmail-cursors.ts';

describe('InMemoryGmailCursorStore — upsert + get', () => {
  it('returns null when no cursor exists', async () => {
    const store = new InMemoryGmailCursorStore();
    assert.equal(await store.get('u-none'), null);
  });

  it('upsert + get round-trip', async () => {
    const store = new InMemoryGmailCursorStore();
    await store.upsert({ user_id: 'u-1', history_id: '12345' });
    const c = await store.get('u-1');
    assert.ok(c);
    assert.equal(c?.user_id, 'u-1');
    assert.equal(c?.history_id, '12345');
    assert.ok(c?.updated_at);
  });

  it('upsert replaces prior cursor for the same user', async () => {
    const store = new InMemoryGmailCursorStore();
    await store.upsert({ user_id: 'u-1', history_id: '100' });
    await store.upsert({ user_id: 'u-1', history_id: '200' });
    const c = await store.get('u-1');
    assert.equal(c?.history_id, '200');
  });

  it('per-user isolation', async () => {
    const store = new InMemoryGmailCursorStore();
    await store.upsert({ user_id: 'u-1', history_id: '100' });
    await store.upsert({ user_id: 'u-2', history_id: '999' });
    assert.equal((await store.get('u-1'))?.history_id, '100');
    assert.equal((await store.get('u-2'))?.history_id, '999');
  });

  it('uses provided updated_at when given', async () => {
    const store = new InMemoryGmailCursorStore();
    await store.upsert({
      user_id: 'u-1',
      history_id: '100',
      updated_at: '2026-05-22T00:00:00.000Z'
    });
    const c = await store.get('u-1');
    assert.equal(c?.updated_at, '2026-05-22T00:00:00.000Z');
  });

  it('returned cursor is frozen', async () => {
    const store = new InMemoryGmailCursorStore();
    await store.upsert({ user_id: 'u-1', history_id: '100' });
    const c = await store.get('u-1');
    assert.ok(c);
    assert.throws(() => {
      (c as unknown as { history_id: string }).history_id = 'mutated';
    });
  });
});

describe('InMemoryGmailCursorStore — delete', () => {
  it('returns true when removing an existing cursor', async () => {
    const store = new InMemoryGmailCursorStore();
    await store.upsert({ user_id: 'u-1', history_id: '100' });
    assert.equal(await store.delete('u-1'), true);
    assert.equal(await store.get('u-1'), null);
  });

  it('returns false when nothing to delete', async () => {
    const store = new InMemoryGmailCursorStore();
    assert.equal(await store.delete('u-none'), false);
  });

  it('is idempotent: second delete returns false', async () => {
    const store = new InMemoryGmailCursorStore();
    await store.upsert({ user_id: 'u-1', history_id: '100' });
    assert.equal(await store.delete('u-1'), true);
    assert.equal(await store.delete('u-1'), false);
  });
});

describe('InMemoryGmailCursorStore — listUserIds (Phase 3B.2)', () => {
  it('returns empty list when no cursors stored', async () => {
    const store = new InMemoryGmailCursorStore();
    assert.deepEqual([...(await store.listUserIds())], []);
  });

  it('returns each user_id with a stored cursor', async () => {
    const store = new InMemoryGmailCursorStore();
    await store.upsert({ user_id: 'u-1', history_id: '100' });
    await store.upsert({ user_id: 'u-2', history_id: '200' });
    await store.upsert({ user_id: 'u-3', history_id: '300' });
    const ids = [...(await store.listUserIds())].sort();
    assert.deepEqual(ids, ['u-1', 'u-2', 'u-3']);
  });

  it('deleted user does not appear', async () => {
    const store = new InMemoryGmailCursorStore();
    await store.upsert({ user_id: 'u-1', history_id: '100' });
    await store.upsert({ user_id: 'u-2', history_id: '200' });
    await store.delete('u-1');
    assert.deepEqual([...(await store.listUserIds())], ['u-2']);
  });

  it('returned list is frozen', async () => {
    const store = new InMemoryGmailCursorStore();
    await store.upsert({ user_id: 'u-1', history_id: '100' });
    const ids = await store.listUserIds();
    assert.throws(() => {
      (ids as unknown as string[]).push('mutated');
    });
  });
});
