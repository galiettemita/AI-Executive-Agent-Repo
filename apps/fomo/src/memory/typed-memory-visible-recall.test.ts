import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { InMemoryMemorySignalStore } from './memory-signals.ts';
import {
  InMemoryTypedMemoryStore,
  MemorySignalsBackedTypedMemoryStore,
  type NewTypedMemoryRow
} from './typed-memory.ts';
import { recallVisibleExplicitPreference } from './typed-memory-visible-recall.ts';

function preference(overrides: Record<string, unknown> = {}): NewTypedMemoryRow {
  return {
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
    updated_at: '2026-06-25T12:00:00.000Z',
    ...overrides
  } as NewTypedMemoryRow;
}

describe('Memory V1 visible explicit-preference recall', () => {
  it('returns a safe user-visible explanation with source and audit metadata', async () => {
    const store = new InMemoryTypedMemoryStore();
    await store.write(preference());

    const recall = await recallVisibleExplicitPreference(store, 'u1', { attribute: 'alert_timing' });

    assert.deepEqual(recall, {
      user_id: 'u1',
      memory_id: 1,
      attribute: 'alert_timing',
      preference_summary: 'alert timing: evening',
      visible_explanation:
        'I used your saved alert timing preference because it was explicitly stored for this user. (alert timing: evening)',
      why_used:
        'I used your saved alert timing preference because it was explicitly stored for this user.',
      source_metadata: {
        source: 'user_stated',
        source_ref_type: 'reply',
        confidence: 'high',
        updated_at: '2026-06-25T12:00:00.000Z'
      },
      audit_metadata: {
        memory_kind: 'preference',
        row_id: 1,
        scope_key: 'preference:alert_timing'
      }
    });
    assert.equal(Object.isFrozen(recall), true);
    assert.equal(Object.isFrozen(recall?.source_metadata), true);
    assert.equal(Object.isFrozen(recall?.audit_metadata), true);
  });

  it('preserves cross-user isolation and does not leak other-user/private preference content', async () => {
    const store = new InMemoryTypedMemoryStore();
    await store.write(preference({ user_id: 'u1', value: 'morning' }));
    await store.write(
      preference({
        user_id: 'u2',
        value: 'private other user value alice@example.com',
        source_ref: 'reply:other-private'
      })
    );

    const recall = await recallVisibleExplicitPreference(store, 'u1', { attribute: 'alert_timing' });
    const json = JSON.stringify(recall);

    assert.equal(recall?.user_id, 'u1');
    assert.equal(recall?.preference_summary, 'alert timing: morning');
    assert.equal(json.includes('u2'), false);
    assert.equal(json.includes('alice@example.com'), false);
    assert.equal(json.includes('other-private'), false);
  });

  it('redacts raw email-like primitive preference values and hides structured preference payloads', async () => {
    const store = new InMemoryTypedMemoryStore();
    await store.write(preference({ value: 'alice@example.com' }));
    await store.write(
      preference({
        scope_key: 'preference:quietness',
        attribute: 'quietness_preference',
        value: { max_per_day: 2, hidden_note: 'do not leak nested private text' },
        updated_at: '2026-06-25T13:00:00.000Z'
      })
    );

    const redacted = await recallVisibleExplicitPreference(store, 'u1', { attribute: 'alert_timing' });
    const structured = await recallVisibleExplicitPreference(store, 'u1', {
      attribute: 'quietness_preference'
    });

    assert.equal(redacted?.preference_summary, 'alert timing: [redacted]');
    assert.equal(JSON.stringify(redacted).includes('alice@example.com'), false);
    assert.equal(structured?.preference_summary, 'quietness preference: saved structured preference');
    assert.equal(JSON.stringify(structured).includes('hidden_note'), false);
    assert.equal(JSON.stringify(structured).includes('do not leak nested private text'), false);
  });

  it('recalls explicit preferences from the memory_signals bridge without runtime activation', async () => {
    const memoryStore = new InMemoryMemorySignalStore();
    await memoryStore.upsert({
      user_id: 'u1',
      kind: 'quietness_preference',
      scope_key: null,
      detail: { max_per_day: 4, private_note: 'do not leak bridged payload' },
      source: 'user_confirmed',
      updated_at: '2026-06-25T14:00:00.000Z'
    });
    await memoryStore.upsert({
      user_id: 'u2',
      kind: 'quietness_preference',
      scope_key: null,
      detail: { max_per_day: 1, private_note: 'other user private payload' },
      source: 'user_confirmed',
      updated_at: '2026-06-25T15:00:00.000Z'
    });
    const store = new MemorySignalsBackedTypedMemoryStore(memoryStore);

    const recall = await recallVisibleExplicitPreference(store, 'u1', {
      attribute: 'quietness_preference'
    });
    const json = JSON.stringify(recall);

    assert.equal(recall?.preference_summary, 'quietness preference: saved structured preference');
    assert.equal(recall?.source_metadata.source, 'user_stated');
    assert.equal(recall?.source_metadata.source_ref_type, 'memory_signal');
    assert.equal(recall?.audit_metadata.memory_kind, 'preference');
    assert.equal(json.includes('do not leak bridged payload'), false);
    assert.equal(json.includes('other user private payload'), false);
    assert.equal(json.includes('u2'), false);
  });

  it('does not recall feedback-derived, low-confidence, stale, or retracted preferences', async () => {
    const store = new InMemoryTypedMemoryStore();
    await store.write(preference({ source: 'feedback_derived' }));
    await store.write(preference({ scope_key: 'preference:low', attribute: 'low', confidence: 'low' }));
    await store.write(
      preference({
        scope_key: 'preference:stale',
        attribute: 'stale',
        stale_marked_at: '2026-06-25T00:00:00.000Z'
      })
    );
    await store.write(preference({ scope_key: 'preference:retracted', attribute: 'retracted', retracted: true }));

    assert.equal(await recallVisibleExplicitPreference(store, 'u1'), null);
  });
});
