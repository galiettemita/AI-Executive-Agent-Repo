import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { InMemoryAuditStore } from '../core/audit.ts';

import {
  buildTypedMemoryContextPack,
  InMemoryTypedMemoryStore,
  type NewTypedMemoryRow
} from './typed-memory.ts';

function semantic(overrides: Partial<NewTypedMemoryRow> = {}): NewTypedMemoryRow {
  return {
    user_id: 'u1',
    kind: 'semantic',
    scope_key: 'profile:working_hours',
    source: 'user_provided',
    source_ref: 'reply:123',
    confidence: 'high',
    stale_marked_at: null,
    retracted: false,
    superseded_by: null,
    attribute: 'working_hours',
    value: { tz: 'America/New_York', start: '09:00', end: '18:00' },
    ...overrides
  } as NewTypedMemoryRow;
}

describe('typed memory dormant context pack builder', () => {
  it('builds a deterministic dormant retrieval pack with structural audit metadata only', async () => {
    const audit = new InMemoryAuditStore();
    const store = new InMemoryTypedMemoryStore(audit);
    await store.write(
      semantic({
        scope_key: 'profile:older',
        updated_at: '2026-06-23T11:00:00.000Z'
      })
    );
    await store.write({
      user_id: 'u1',
      kind: 'preference',
      scope_key: 'preference:alert_timing',
      source: 'user_stated',
      source_ref: 'reply:456',
      confidence: 'high',
      stale_marked_at: null,
      retracted: false,
      superseded_by: null,
      attribute: 'alert_timing',
      value: 'evening',
      updated_at: '2026-06-23T13:00:00.000Z'
    } as NewTypedMemoryRow);
    await store.write({
      user_id: 'u1',
      kind: 'correction',
      scope_key: 'signal:sender_suppressed:abc123hmac',
      source: 'user_stated',
      source_ref: 'reply:789',
      confidence: 'high',
      stale_marked_at: null,
      retracted: false,
      superseded_by: null,
      rule: 'sender_suppressed',
      target_hmac: 'abc123hmac',
      value: { suppressed: true, hidden_reason: 'private sender context' },
      updated_at: '2026-06-23T13:00:00.000Z'
    } as NewTypedMemoryRow);
    await store.write(
      semantic({
        scope_key: 'profile:newer',
        updated_at: '2026-06-23T13:00:00.000Z'
      })
    );
    await store.write(
      semantic({
        user_id: 'u2',
        scope_key: 'profile:other_user',
        updated_at: '2026-06-24T13:00:00.000Z'
      })
    );

    const pack = await buildTypedMemoryContextPack(store, 'u1', 'ops', {
      kinds: ['semantic', 'preference', 'correction'],
      minConfidence: 'high',
      limit: 3
    });

    assert.deepEqual(pack.rows.map((row) => row.user_id), ['u1', 'u1', 'u1']);
    assert.deepEqual(pack.row_ids, [2, 3, 4]);
    assert.deepEqual(pack.row_kinds, ['preference', 'correction', 'semantic']);
    assert.deepEqual(pack.rows.map((row) => row.scope_key), [
      'preference:alert_timing',
      'signal:sender_suppressed:abc123hmac',
      'profile:newer'
    ]);
    assert.equal(pack.preferences_applied, 1);
    assert.equal(pack.suppressions_applied, 1);

    const [entry] = await audit.recent('u1');
    assert.deepEqual(entry?.detail, {
      pack_kind: 'ops',
      row_kinds: ['preference', 'correction', 'semantic'],
      row_ids: [2, 3, 4],
      suppressions_applied: 1,
      preferences_applied: 1
    });
    const auditJson = JSON.stringify(entry?.detail);
    assert.equal(auditJson.includes('evening'), false);
    assert.equal(auditJson.includes('private sender context'), false);
  });
});
