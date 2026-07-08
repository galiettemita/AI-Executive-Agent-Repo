import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { InMemoryMemorySignalStore } from './memory-signals.ts';
import {
  InMemoryTypedMemoryStore,
  MemorySignalsBackedTypedMemoryStore,
  type NewTypedMemoryRow
} from './typed-memory.ts';
import {
  answerVisibleMemoryExplanationCommand,
  answerVisibleMemoryReviewCommand,
  correctVisibleExplicitPreference,
  explainVisibleExplicitPreferenceMemoryUse,
  explainVisibleExplicitPreferenceUse,
  forgetVisibleExplicitPreference,
  isVisibleMemoryExplanationCommandText,
  isVisibleMemoryReviewCommandText,
  rememberVisibleExplicitPreference,
  recallVisibleExplicitPreference,
  reviewVisibleExplicitPreferences
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
  it('reviews active explicit preferences without leaking private raw values or cross-user data', async () => {
    const store = new InMemoryTypedMemoryStore();
    await store.write(
      preference({
        user_id: 'u1',
        value: 'alice@example.com evenings only',
        source_ref: 'reply:private-source-ref'
      })
    );
    await store.write(
      preference({
        user_id: 'u1',
        scope_key: 'preference:quietness_preference',
        attribute: 'quietness_preference',
        value: { max_per_day: 2, hidden_note: 'do not leak nested private text' },
        source_ref: 'memory_signal:quietness_preference:secret-raw-ref',
        updated_at: '2026-06-26T12:00:00.000Z'
      })
    );
    await store.write(
      preference({
        user_id: 'u2',
        value: 'other user secret alice@example.com',
        source_ref: 'reply:other-user-private-source',
        updated_at: '2026-06-27T12:00:00.000Z'
      })
    );

    const review = await reviewVisibleExplicitPreferences(store, 'u1');
    const json = JSON.stringify(review);

    assert.deepEqual(review.preferences.map((item) => item.preference_summary), [
      'quietness preference: saved structured preference',
      'alert timing: [redacted]'
    ]);
    assert.match(
      review.answer,
      /I remember these active explicit preferences for you: quietness preference: saved structured preference; alert timing: \[redacted\]\./
    );
    assert.deepEqual(review.audit_metadata, {
      memory_kind: 'preference',
      returned_count: 2,
      row_ids: [2, 1],
      scope_keys: ['preference:quietness_preference', 'preference:alert_timing']
    });
    assert.equal(review.preferences[0]?.source_metadata.source_ref_type, 'memory_signal');
    assert.equal(review.preferences[1]?.source_metadata.source_ref_type, 'reply');
    assert.equal(Object.isFrozen(review), true);
    assert.equal(Object.isFrozen(review.preferences), true);
    assert.equal(Object.isFrozen(review.audit_metadata), true);
    assert.equal(json.includes('alice@example.com'), false);
    assert.equal(json.includes('private-source-ref'), false);
    assert.equal(json.includes('secret-raw-ref'), false);
    assert.equal(json.includes('hidden_note'), false);
    assert.equal(json.includes('do not leak nested private text'), false);
    assert.equal(json.includes('u2'), false);
    assert.equal(json.includes('other user secret'), false);
    assert.equal(json.includes('other-user-private-source'), false);
  });

  it('excludes unsafe inactive and non-explicit memories from visible preference review', async () => {
    const store = new InMemoryTypedMemoryStore();
    await store.write(preference({ value: 'morning', updated_at: '2026-06-25T10:00:00.000Z' }));
    await store.write(
      preference({
        scope_key: 'preference:stale',
        attribute: 'stale',
        value: 'should not review',
        stale_marked_at: '2026-06-25T00:00:00.000Z'
      })
    );
    await store.write(
      preference({
        scope_key: 'preference:retracted',
        attribute: 'retracted',
        value: 'should not review',
        retracted: true
      })
    );
    await store.write(
      preference({
        scope_key: 'preference:tombstoned',
        attribute: 'tombstoned',
        value: 'should not review',
        superseded_by: 99
      })
    );
    await store.write(
      preference({
        scope_key: 'preference:low',
        attribute: 'low',
        value: 'should not review',
        confidence: 'low'
      })
    );
    await store.write(
      preference({
        scope_key: 'preference:feedback',
        attribute: 'feedback',
        value: 'should not review',
        source: 'feedback_derived'
      })
    );

    const review = await reviewVisibleExplicitPreferences(store, 'u1');
    const json = JSON.stringify(review);

    assert.deepEqual(review.preferences.map((item) => item.preference_summary), ['alert timing: morning']);
    assert.equal(review.audit_metadata.returned_count, 1);
    assert.equal(json.includes('should not review'), false);
    assert.equal(json.includes('stale'), false);
    assert.equal(json.includes('retracted'), false);
    assert.equal(json.includes('tombstoned'), false);
    assert.equal(json.includes('feedback'), false);
  });

  it('reviews explicit preferences from memory_signals bridge without raw source refs or private payloads', async () => {
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
      user_id: 'u1',
      kind: 'timing_preference',
      scope_key: null,
      detail: { preferred_window: 'late', private_note: 'do not leak timing payload' },
      source: 'feedback_derived',
      updated_at: '2026-06-25T15:00:00.000Z'
    });
    await memoryStore.upsert({
      user_id: 'u2',
      kind: 'quietness_preference',
      scope_key: null,
      detail: { max_per_day: 1, private_note: 'other user private payload' },
      source: 'user_confirmed',
      updated_at: '2026-06-25T16:00:00.000Z'
    });
    const store = new MemorySignalsBackedTypedMemoryStore(memoryStore);

    const review = await reviewVisibleExplicitPreferences(store, 'u1');
    const json = JSON.stringify(review);

    assert.deepEqual(review.preferences.map((item) => item.preference_summary), [
      'quietness preference: saved structured preference'
    ]);
    assert.equal(review.preferences[0]?.source_metadata.source_ref_type, 'memory_signal');
    assert.deepEqual(review.audit_metadata.scope_keys, ['signal:quietness_preference']);
    assert.equal(json.includes('do not leak bridged payload'), false);
    assert.equal(json.includes('do not leak timing payload'), false);
    assert.equal(json.includes('memory_signal:quietness_preference'), false);
    assert.equal(json.includes('other user private payload'), false);
    assert.equal(json.includes('u2'), false);
  });

  it('returns a safe empty review answer when no explicit preferences are available', async () => {
    const store = new InMemoryTypedMemoryStore();

    const review = await reviewVisibleExplicitPreferences(store, 'u1');

    assert.deepEqual(review, {
      user_id: 'u1',
      answer: 'I do not have any active explicit preferences saved for you that are safe to show here.',
      preferences: [],
      audit_metadata: {
        memory_kind: 'preference',
        returned_count: 0,
        row_ids: [],
        scope_keys: []
      }
    });
    assert.equal(Object.isFrozen(review), true);
    assert.equal(Object.isFrozen(review.preferences), true);
    assert.equal(Object.isFrozen(review.audit_metadata), true);
  });

  it('answers what-do-you-remember style commands through the explicit-preference review helper', async () => {
    const store = new InMemoryTypedMemoryStore();
    await store.write(
      preference({
        user_id: 'u1',
        value: 'alice@example.com evenings only',
        source_ref: 'reply:private-review-command-ref'
      })
    );
    await store.write(
      preference({
        user_id: 'u1',
        scope_key: 'preference:quietness_preference',
        attribute: 'quietness_preference',
        value: { max_per_day: 2, hidden_note: 'do not leak command payload' },
        source_ref: 'memory_signal:quietness-command-secret-ref',
        updated_at: '2026-06-26T12:00:00.000Z'
      })
    );
    await store.write(
      preference({
        user_id: 'u2',
        value: 'other-user private command value alice@example.com',
        source_ref: 'reply:other-user-command-secret',
        updated_at: '2026-06-27T12:00:00.000Z'
      })
    );

    const result = await answerVisibleMemoryReviewCommand(store, 'u1', 'What do you remember about me?');
    assert.ok(result);
    const json = JSON.stringify(result);

    assert.deepEqual(result.review.preferences.map((item) => item.preference_summary), [
      'quietness preference: saved structured preference',
      'alert timing: [redacted]'
    ]);
    assert.equal(result.action, 'review_visible_explicit_preferences');
    assert.equal(result.user_id, 'u1');
    assert.equal(result.answer, result.review.answer);
    assert.deepEqual(result.audit_metadata, {
      memory_kind: 'preference',
      matched_intent: 'memory_review',
      returned_count: 2,
      row_ids: [2, 1],
      scope_keys: ['preference:quietness_preference', 'preference:alert_timing']
    });
    assert.equal(result.review.preferences[0]?.source_metadata.source_ref_type, 'memory_signal');
    assert.equal(result.review.preferences[1]?.source_metadata.source_ref_type, 'reply');
    assert.equal(Object.isFrozen(result), true);
    assert.equal(Object.isFrozen(result.audit_metadata), true);
    assert.equal(json.includes('alice@example.com'), false);
    assert.equal(json.includes('private-review-command-ref'), false);
    assert.equal(json.includes('quietness-command-secret-ref'), false);
    assert.equal(json.includes('do not leak command payload'), false);
    assert.equal(json.includes('u2'), false);
    assert.equal(json.includes('other-user private command value'), false);
    assert.equal(json.includes('other-user-command-secret'), false);
  });

  it('only routes review-style memory commands and leaves unknown or remember-this text alone', async () => {
    const store = new InMemoryTypedMemoryStore();
    await store.write(preference({ value: 'morning' }));

    assert.equal(isVisibleMemoryReviewCommandText('What have you saved about me?'), true);
    assert.equal(isVisibleMemoryReviewCommandText('List my saved preferences'), true);
    assert.equal(isVisibleMemoryReviewCommandText('Do you remember anything about me?'), true);
    assert.equal(isVisibleMemoryReviewCommandText('Remember this: I prefer mornings'), false);
    assert.equal(isVisibleMemoryReviewCommandText('please save that I prefer mornings'), false);
    assert.equal(isVisibleMemoryReviewCommandText('Can you help me with my inbox?'), false);

    assert.equal(await answerVisibleMemoryReviewCommand(store, 'u1', 'Can you help me with my inbox?'), null);
    assert.equal(await answerVisibleMemoryReviewCommand(store, 'u1', 'Remember this: I prefer mornings'), null);
  });

  it('keeps the review command adapter on the same inactive-memory and cross-user exclusion path', async () => {
    const store = new InMemoryTypedMemoryStore();
    await store.write(preference({ value: 'morning', updated_at: '2026-06-25T10:00:00.000Z' }));
    await store.write(
      preference({
        user_id: 'u2',
        value: 'other-user command secret alice@example.com',
        source_ref: 'reply:other-user-review-command'
      })
    );
    await store.write(preference({ scope_key: 'preference:low', attribute: 'low', confidence: 'low' }));
    await store.write(
      preference({
        scope_key: 'preference:stale',
        attribute: 'stale',
        value: 'should not route through command',
        stale_marked_at: '2026-06-25T00:00:00.000Z'
      })
    );
    await store.write(preference({ scope_key: 'preference:retracted', attribute: 'retracted', retracted: true }));
    await store.write(preference({ scope_key: 'preference:tombstoned', attribute: 'tombstoned', superseded_by: 99 }));

    const result = await answerVisibleMemoryReviewCommand(store, 'u1', 'What does Brevio remember about me?');
    assert.ok(result);
    const json = JSON.stringify(result);

    assert.deepEqual(result.review.preferences.map((item) => item.preference_summary), ['alert timing: morning']);
    assert.equal(result.audit_metadata.returned_count, 1);
    assert.equal(json.includes('u2'), false);
    assert.equal(json.includes('other-user command secret'), false);
    assert.equal(json.includes('alice@example.com'), false);
    assert.equal(json.includes('other-user-review-command'), false);
    assert.equal(json.includes('should not route through command'), false);
    assert.equal(json.includes('stale'), false);
    assert.equal(json.includes('retracted'), false);
    assert.equal(json.includes('tombstoned'), false);
  });

  it('answers why-remembered or why-used commands through the explicit-preference explanation helper', async () => {
    const store = new InMemoryTypedMemoryStore();
    await store.write(
      preference({
        user_id: 'u1',
        value: 'alice@example.com evenings only',
        source_ref: 'reply:private-explain-command-ref'
      })
    );
    await store.write(
      preference({
        user_id: 'u2',
        value: 'other-user private command value alice@example.com',
        source_ref: 'reply:other-user-explain-command-secret',
        updated_at: '2026-06-27T12:00:00.000Z'
      })
    );

    const result = await answerVisibleMemoryExplanationCommand(
      store,
      'u1',
      'Why did you remember that?',
      { attribute: 'alert_timing' }
    );
    assert.ok(result);
    const json = JSON.stringify(result);

    assert.equal(result.action, 'explain_visible_explicit_preference_use');
    assert.equal(result.user_id, 'u1');
    assert.equal(result.answer, result.explanation.answer);
    assert.deepEqual(result.audit_metadata, {
      memory_kind: 'preference',
      matched_intent: 'memory_explanation'
    });
    assert.match(result.explanation.answer, /matched the saved alert timing preference/);
    assert.match(result.explanation.source, /prior user reply/);
    assert.match(result.explanation.audit, /high-confidence preference metadata/);
    assert.match(result.explanation.safety, /scoped to this user/);
    assert.equal(Object.isFrozen(result), true);
    assert.equal(Object.isFrozen(result.audit_metadata), true);
    assert.equal(json.includes('alice@example.com'), false);
    assert.equal(json.includes('private-explain-command-ref'), false);
    assert.equal(json.includes('u2'), false);
    assert.equal(json.includes('other-user private command value'), false);
    assert.equal(json.includes('other-user-explain-command-secret'), false);
  });

  it('only routes explanation-style memory commands and leaves unknown or remember-this text alone', async () => {
    const store = new InMemoryTypedMemoryStore();
    await store.write(preference({ value: 'morning' }));

    assert.equal(isVisibleMemoryExplanationCommandText('Why did you remember that?'), true);
    assert.equal(isVisibleMemoryExplanationCommandText('Why did Brevio use my preference?'), true);
    assert.equal(isVisibleMemoryExplanationCommandText('Explain why you used it'), true);
    assert.equal(isVisibleMemoryExplanationCommandText('Remember this: I prefer mornings'), false);
    assert.equal(isVisibleMemoryExplanationCommandText('please save that I prefer mornings'), false);
    assert.equal(isVisibleMemoryExplanationCommandText('Can you help me with my inbox?'), false);

    assert.equal(await answerVisibleMemoryExplanationCommand(store, 'u1', 'Can you help me with my inbox?'), null);
    assert.equal(await answerVisibleMemoryExplanationCommand(store, 'u1', 'Remember this: I prefer mornings'), null);
  });

  it('keeps the explanation command adapter on the same inactive-memory and cross-user exclusion path', async () => {
    const store = new InMemoryTypedMemoryStore();
    await store.write(preference({ value: 'morning', updated_at: '2026-06-25T10:00:00.000Z' }));
    await store.write(
      preference({
        user_id: 'u2',
        value: 'other-user explanation command secret alice@example.com',
        source_ref: 'reply:other-user-explain-command'
      })
    );
    await store.write(preference({ scope_key: 'preference:low', attribute: 'low', confidence: 'low' }));
    await store.write(
      preference({
        scope_key: 'preference:stale',
        attribute: 'stale',
        value: 'should not explain through command',
        stale_marked_at: '2026-06-25T00:00:00.000Z'
      })
    );
    await store.write(preference({ scope_key: 'preference:retracted', attribute: 'retracted', retracted: true }));
    await store.write(preference({ scope_key: 'preference:tombstoned', attribute: 'tombstoned', superseded_by: 99 }));

    const result = await answerVisibleMemoryExplanationCommand(
      store,
      'u1',
      'Tell me why Brevio used my preference',
      { attribute: 'alert_timing' }
    );
    assert.ok(result);
    const json = JSON.stringify(result);

    assert.match(result.explanation.answer, /matched the saved alert timing preference/);
    assert.equal(json.includes('u2'), false);
    assert.equal(json.includes('other-user explanation command secret'), false);
    assert.equal(json.includes('alice@example.com'), false);
    assert.equal(json.includes('other-user-explain-command'), false);
    assert.equal(json.includes('should not explain through command'), false);
    assert.equal(json.includes('stale'), false);
    assert.equal(json.includes('retracted'), false);
    assert.equal(json.includes('tombstoned'), false);
    assert.equal(
      await answerVisibleMemoryExplanationCommand(store, 'u1', 'Why did you use it?', { attribute: 'stale' }),
      null
    );
    assert.equal(
      await answerVisibleMemoryExplanationCommand(store, 'u1', 'Why did you use it?', { attribute: 'tombstoned' }),
      null
    );
  });

  it('remembers a user-stated explicit preference, then recalls, explains, corrects, and forgets it', async () => {
    const store = new InMemoryTypedMemoryStore();

    const remembered = await rememberVisibleExplicitPreference(store, 'u1', {
      attribute: 'alert_timing',
      value: 'alice@example.com evenings only',
      updatedAt: '2026-06-28T09:00:00.000Z',
      sourceRef: 'reply:remember-this-123'
    });

    assert.deepEqual(remembered, {
      action: 'remembered',
      user_id: 'u1',
      attribute: 'alert_timing',
      message:
        'I remembered that saved alert timing preference. I can use it in future memory recall.',
      audit_metadata: {
        memory_kind: 'preference',
        scope_key: 'preference:alert_timing',
        remembered_row_id: 1,
        source: 'user_stated',
        confidence: 'high',
        updated_at: '2026-06-28T09:00:00.000Z'
      }
    });
    assert.equal(Object.isFrozen(remembered), true);
    assert.equal(Object.isFrozen(remembered.audit_metadata), true);
    assert.equal(JSON.stringify(remembered).includes('alice@example.com'), false);
    assert.equal(JSON.stringify(remembered).includes('remember-this-123'), false);

    const recalled = await recallVisibleExplicitPreference(store, 'u1', { attribute: 'alert_timing' });
    assert.ok(recalled);
    assert.equal(recalled.preference_summary, 'alert timing: [redacted]');
    assert.equal(recalled.source_metadata.source, 'user_stated');
    assert.equal(recalled.source_metadata.source_ref_type, 'reply');
    assert.equal(recalled.source_metadata.confidence, 'high');
    assert.equal(recalled.source_metadata.updated_at, '2026-06-28T09:00:00.000Z');
    assert.equal(recalled.audit_metadata.scope_key, 'preference:alert_timing');
    assert.match(
      explainVisibleExplicitPreferenceUse(recalled).answer,
      /matched the saved alert timing preference/
    );

    await correctVisibleExplicitPreference(
      store,
      'u1',
      { attribute: 'alert_timing' },
      {
        correctedValue: 'mornings',
        updatedAt: '2026-06-29T09:00:00.000Z',
        sourceRef: 'reply:correction-after-remember'
      }
    );

    const corrected = await recallVisibleExplicitPreference(store, 'u1', { attribute: 'alert_timing' });
    assert.ok(corrected);
    assert.equal(corrected.preference_summary, 'alert timing: mornings');
    assert.equal(corrected.source_metadata.updated_at, '2026-06-29T09:00:00.000Z');

    const forgotten = await forgetVisibleExplicitPreference(store, 'u1', { attribute: 'alert_timing' });
    assert.equal(forgotten?.forgotten, true);
    assert.equal(await recallVisibleExplicitPreference(store, 'u1', { attribute: 'alert_timing' }), null);
  });

  it('keeps remember-this explicit preferences user-scoped and rejects raw private scope material', async () => {
    const store = new InMemoryTypedMemoryStore();
    await rememberVisibleExplicitPreference(store, 'u1', {
      attribute: 'alert_timing',
      value: 'morning',
      updatedAt: '2026-06-28T09:00:00.000Z'
    });
    await rememberVisibleExplicitPreference(store, 'u2', {
      attribute: 'alert_timing',
      value: 'other user secret alice@example.com',
      updatedAt: '2026-06-28T10:00:00.000Z',
      sourceRef: 'reply:other-user-secret'
    });

    const recall = await recallVisibleExplicitPreference(store, 'u1', { attribute: 'alert_timing' });
    const json = JSON.stringify(recall);

    assert.equal(recall?.user_id, 'u1');
    assert.equal(recall?.preference_summary, 'alert timing: morning');
    assert.equal(json.includes('u2'), false);
    assert.equal(json.includes('other user secret'), false);
    assert.equal(json.includes('alice@example.com'), false);
    assert.equal(json.includes('other-user-secret'), false);
    await assert.rejects(
      rememberVisibleExplicitPreference(store, 'u1', {
        attribute: 'alice@example.com',
        value: 'never use private email as scope',
        updatedAt: '2026-06-28T11:00:00.000Z'
      }),
      /must not contain raw email addresses/
    );
  });

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

  it('answers why remembered/used through a scoped helper without explaining unsafe memories', async () => {
    const store = new InMemoryTypedMemoryStore();
    await store.write(
      preference({
        user_id: 'u1',
        value: 'alice@example.com evenings only',
        source_ref: 'reply:private-source-ref',
        updated_at: '2026-06-25T12:00:00.000Z'
      })
    );
    await store.write(
      preference({
        user_id: 'u2',
        value: 'other-user private value alice@example.com',
        source_ref: 'reply:other-user-private-source',
        updated_at: '2026-06-25T13:00:00.000Z'
      })
    );
    await store.write(
      preference({
        user_id: 'u1',
        scope_key: 'preference:deleted',
        attribute: 'deleted',
        value: 'should not explain',
        retracted: true
      })
    );
    await store.write(
      preference({
        user_id: 'u1',
        scope_key: 'preference:tombstoned',
        attribute: 'tombstoned',
        value: 'should not explain either',
        superseded_by: 999
      })
    );

    const explanation = await explainVisibleExplicitPreferenceMemoryUse(store, 'u1', {
      attribute: 'alert_timing'
    });
    assert.ok(explanation);
    const json = JSON.stringify(explanation);

    assert.match(explanation.answer, /matched the saved alert timing preference/);
    assert.match(explanation.source, /prior user reply/);
    assert.match(explanation.audit, /high-confidence preference metadata/);
    assert.match(explanation.safety, /scoped to this user/);
    assert.equal(json.includes('alice@example.com'), false);
    assert.equal(json.includes('private-source-ref'), false);
    assert.equal(json.includes('u2'), false);
    assert.equal(json.includes('other-user private value'), false);
    assert.equal(json.includes('other-user-private-source'), false);
    assert.equal(json.includes('should not explain'), false);
    assert.equal(
      await explainVisibleExplicitPreferenceMemoryUse(store, 'u1', { attribute: 'deleted' }),
      null
    );
    assert.equal(
      await explainVisibleExplicitPreferenceMemoryUse(store, 'u1', { attribute: 'tombstoned' }),
      null
    );
  });

  it('runs the visible loop: recall, explain, forget, then old preference no longer surfaces', async () => {
    const store = new InMemoryTypedMemoryStore();
    await store.write(preference({ value: 'evening' }));

    const before = await recallVisibleExplicitPreference(store, 'u1', { attribute: 'alert_timing' });
    assert.ok(before);
    assert.equal(before.preference_summary, 'alert timing: evening');
    assert.match(explainVisibleExplicitPreferenceUse(before).answer, /matched the saved alert timing preference/);

    const result = await forgetVisibleExplicitPreference(store, 'u1', { attribute: 'alert_timing' });
    assert.deepEqual(result, {
      action: 'forgot',
      user_id: 'u1',
      attribute: 'alert_timing',
      forgotten: true,
      message:
        'I forgot that saved alert timing preference. I will not use it in future memory recall.',
      audit_metadata: {
        memory_kind: 'preference',
        scope_key: 'preference:alert_timing',
        forgotten_row_id: 1
      }
    });
    assert.equal(Object.isFrozen(result), true);
    assert.equal(Object.isFrozen(result?.audit_metadata), true);
    assert.equal(JSON.stringify(result).includes('evening'), false);

    assert.equal(await recallVisibleExplicitPreference(store, 'u1', { attribute: 'alert_timing' }), null);
  });

  it('runs the visible loop: recall, explain, correct, then future recall uses corrected preference', async () => {
    const store = new InMemoryTypedMemoryStore();
    await store.write(preference({ value: 'evening' }));

    const before = await recallVisibleExplicitPreference(store, 'u1', { attribute: 'alert_timing' });
    assert.ok(before);
    assert.equal(before.preference_summary, 'alert timing: evening');
    assert.match(explainVisibleExplicitPreferenceUse(before).answer, /user-stated preference/);

    const result = await correctVisibleExplicitPreference(
      store,
      'u1',
      { attribute: 'alert_timing' },
      {
        correctedValue: 'morning',
        updatedAt: '2026-06-26T09:00:00.000Z',
        sourceRef: 'reply:correction-789'
      }
    );

    assert.deepEqual(result, {
      action: 'corrected',
      user_id: 'u1',
      attribute: 'alert_timing',
      message:
        'I updated that saved alert timing preference. I will use the corrected version going forward.',
      audit_metadata: {
        memory_kind: 'preference',
        scope_key: 'preference:alert_timing',
        previous_row_id: 1,
        corrected_row_id: 1,
        source: 'user_stated',
        updated_at: '2026-06-26T09:00:00.000Z'
      }
    });
    assert.equal(JSON.stringify(result).includes('evening'), false);
    assert.equal(JSON.stringify(result).includes('morning'), false);
    assert.equal(JSON.stringify(result).includes('correction-789'), false);

    const after = await recallVisibleExplicitPreference(store, 'u1', { attribute: 'alert_timing' });
    assert.ok(after);
    assert.equal(after.preference_summary, 'alert timing: morning');
    assert.equal(after.source_metadata.updated_at, '2026-06-26T09:00:00.000Z');
    assert.match(explainVisibleExplicitPreferenceUse(after).audit, /2026-06-26T09:00:00.000Z/);
  });

  it('keeps forget and correct scoped to one user', async () => {
    const store = new InMemoryTypedMemoryStore();
    await store.write(preference({ user_id: 'u1', value: 'evening' }));
    await store.write(preference({ user_id: 'u2', value: 'midnight' }));

    await forgetVisibleExplicitPreference(store, 'u1', { attribute: 'alert_timing' });
    assert.equal(await recallVisibleExplicitPreference(store, 'u1', { attribute: 'alert_timing' }), null);
    assert.equal(
      (await recallVisibleExplicitPreference(store, 'u2', { attribute: 'alert_timing' }))?.preference_summary,
      'alert timing: midnight'
    );

    await correctVisibleExplicitPreference(
      store,
      'u2',
      { attribute: 'alert_timing' },
      { correctedValue: 'morning', updatedAt: '2026-06-27T09:00:00.000Z' }
    );
    assert.equal(await recallVisibleExplicitPreference(store, 'u1', { attribute: 'alert_timing' }), null);
    assert.equal(
      (await recallVisibleExplicitPreference(store, 'u2', { attribute: 'alert_timing' }))?.preference_summary,
      'alert timing: morning'
    );
  });

  it('does not forget or correct inactive/unsafe preference memory', async () => {
    const store = new InMemoryTypedMemoryStore();
    await store.write(preference({ confidence: 'low' }));
    await store.write(preference({ scope_key: 'preference:retracted', attribute: 'retracted', retracted: true }));
    await store.write(
      preference({
        scope_key: 'preference:tombstoned',
        attribute: 'tombstoned',
        superseded_by: 99
      })
    );

    assert.equal(await forgetVisibleExplicitPreference(store, 'u1', { attribute: 'alert_timing' }), null);
    assert.equal(
      await correctVisibleExplicitPreference(
        store,
        'u1',
        { attribute: 'alert_timing' },
        { correctedValue: 'morning', updatedAt: '2026-06-27T09:00:00.000Z' }
      ),
      null
    );
  });
});
