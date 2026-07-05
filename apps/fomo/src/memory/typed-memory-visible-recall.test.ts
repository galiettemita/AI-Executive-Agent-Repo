import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { InMemoryMemorySignalStore } from './memory-signals.ts';
import {
  InMemoryTypedMemoryStore,
  MemorySignalsBackedTypedMemoryStore,
  type NewTypedMemoryRow
} from './typed-memory.ts';
import {
  explainVisibleExplicitPreferenceUse,
  recallVisibleExplicitPreference
} from './typed-memory-visible-recall.ts';

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

  it('explains why a visible explicit preference was used in bounded human-readable language', async () => {
    const store = new InMemoryTypedMemoryStore();
    await store.write(preference());

    const recall = await recallVisibleExplicitPreference(store, 'u1', { attribute: 'alert_timing' });
    assert.ok(recall);
    const explanation = explainVisibleExplicitPreferenceUse(recall);

    assert.deepEqual(explanation, {
      memory_used: 'your saved alert timing preference',
      answer:
        'I used it because this request matched the saved alert timing preference for this user. This came from a user-stated preference recorded through a prior user reply. The recall used high-confidence preference metadata last updated 2026-06...',
      relevance:
        'I used it because this request matched the saved alert timing preference for this user.',
      source: 'This came from a user-stated preference recorded through a prior user reply.',
      audit:
        'The recall used high-confidence preference metadata last updated 2026-06-25T12:00:00.000Z; raw preference content is not needed to explain why it was used.',
      safety:
        'The explanation is scoped to this user and summarizes memory metadata without exposing raw private values.'
    });
    assert.equal(Object.isFrozen(explanation), true);
    assert.ok(explanation.answer.length <= 240);
  });

  it('uses safe source metadata labels without dumping raw internals', async () => {
    const store = new InMemoryTypedMemoryStore();
    await store.write(preference({ source: 'founder_default', source_ref: 'memory_signal:99' }));

    const recall = await recallVisibleExplicitPreference(store, 'u1', {
      attribute: 'alert_timing',
      sources: ['founder_default']
    });
    assert.ok(recall);
    const explanation = explainVisibleExplicitPreferenceUse(recall);
    const json = JSON.stringify(explanation);

    assert.equal(
      explanation.source,
      'This came from a founder-set default preference recorded through the memory-signals substrate.'
    );
    assert.equal(json.includes('memory_signal:99'), false);
    assert.equal(json.includes('scope_key'), false);
    assert.equal(json.includes('row_id'), false);
  });

  it('does not leak raw private, email-like, or structured preference values into why-used explanation', async () => {
    const store = new InMemoryTypedMemoryStore();
    await store.write(preference({ value: 'alice@example.com' }));
    await store.write(
      preference({
        scope_key: 'preference:quietness',
        attribute: 'quietness_preference',
        value: { private_note: 'never reveal nested text' },
        updated_at: '2026-06-25T15:00:00.000Z'
      })
    );

    const emailRecall = await recallVisibleExplicitPreference(store, 'u1', { attribute: 'alert_timing' });
    const structuredRecall = await recallVisibleExplicitPreference(store, 'u1', {
      attribute: 'quietness_preference'
    });
    assert.ok(emailRecall);
    assert.ok(structuredRecall);

    const emailExplanation = JSON.stringify(explainVisibleExplicitPreferenceUse(emailRecall));
    const structuredExplanation = JSON.stringify(explainVisibleExplicitPreferenceUse(structuredRecall));

    assert.equal(emailExplanation.includes('alice@example.com'), false);
    assert.equal(emailExplanation.includes('[redacted]'), false);
    assert.equal(structuredExplanation.includes('never reveal nested text'), false);
    assert.equal(structuredExplanation.includes('private_note'), false);
    assert.match(structuredExplanation, /quietness preference/);
  });

  it('keeps why-used explanations cross-user isolated', async () => {
    const store = new InMemoryTypedMemoryStore();
    await store.write(preference({ user_id: 'u1', value: 'evening' }));
    await store.write(
      preference({
        user_id: 'u2',
        value: 'other user secret alice@example.com',
        source_ref: 'reply:other-user-secret',
        updated_at: '2026-06-26T12:00:00.000Z'
      })
    );

    const recall = await recallVisibleExplicitPreference(store, 'u1', { attribute: 'alert_timing' });
    assert.ok(recall);
    const json = JSON.stringify(explainVisibleExplicitPreferenceUse(recall));

    assert.equal(json.includes('u2'), false);
    assert.equal(json.includes('other user secret'), false);
    assert.equal(json.includes('alice@example.com'), false);
    assert.equal(json.includes('other-user-secret'), false);
  });

  it('does not explain low-confidence, stale, retracted, deleted, or tombstoned memory', async () => {
    const store = new InMemoryTypedMemoryStore();
    await store.write(preference({ confidence: 'low' }));
    await store.write(
      preference({
        scope_key: 'preference:stale',
        attribute: 'stale',
        stale_marked_at: '2026-06-25T00:00:00.000Z'
      })
    );
    await store.write(preference({ scope_key: 'preference:retracted', attribute: 'retracted', retracted: true }));
    await store.write(
      preference({
        scope_key: 'preference:tombstoned',
        attribute: 'tombstoned',
        superseded_by: 999
      })
    );

    const memoryStore = new InMemoryMemorySignalStore();
    await memoryStore.upsert({
      user_id: 'u1',
      kind: 'quietness_preference',
      scope_key: null,
      detail: { max_per_day: 1 },
      source: 'user_confirmed',
      updated_at: '2026-06-25T14:00:00.000Z'
    });
    await memoryStore.delete('u1', 'quietness_preference', null);
    const bridgedStore = new MemorySignalsBackedTypedMemoryStore(memoryStore);

    assert.equal(await recallVisibleExplicitPreference(store, 'u1'), null);
    assert.equal(await recallVisibleExplicitPreference(bridgedStore, 'u1'), null);
  });

  it('selects the newest matching candidate deterministically before explaining why it was used', async () => {
    const store = new InMemoryTypedMemoryStore();
    await store.write(preference({ value: 'morning', updated_at: '2026-06-25T10:00:00.000Z' }));
    await store.write(preference({ value: 'evening', updated_at: '2026-06-25T12:00:00.000Z' }));

    const firstRecall = await recallVisibleExplicitPreference(store, 'u1', { attribute: 'alert_timing' });
    const secondRecall = await recallVisibleExplicitPreference(store, 'u1', { attribute: 'alert_timing' });
    assert.ok(firstRecall);
    assert.ok(secondRecall);

    assert.equal(firstRecall.preference_summary, 'alert timing: evening');
    assert.deepEqual(
      explainVisibleExplicitPreferenceUse(firstRecall),
      explainVisibleExplicitPreferenceUse(secondRecall)
    );
  });
});
