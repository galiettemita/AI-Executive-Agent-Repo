import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { InMemoryAuditStore } from '../core/audit.ts';
import { InMemoryMemorySignalStore } from './memory-signals.ts';
import {
  InMemoryTypedMemoryStore,
  MemorySignalsBackedTypedMemoryStore,
  type NewTypedMemoryRow
} from './typed-memory.ts';
import {
  answerVisibleMemoryCorrectCommand,
  answerVisibleMemoryExplanationCommand,
  answerVisibleMemoryForgetCommand,
  answerVisibleMemoryReviewCommand,
  correctVisibleExplicitPreference,
  createVisibleMemoryCommandAppAdapterAuditStoreRecorder,
  explainVisibleExplicitPreferenceMemoryUse,
  explainVisibleExplicitPreferenceUse,
  forgetVisibleExplicitPreference,
  handleVisibleMemoryCommandAppAdapterRequest,
  handleVisibleMemoryCommandFromCaller,
  isVisibleMemoryCorrectCommandText,
  isVisibleMemoryExplanationCommandText,
  isVisibleMemoryForgetCommandText,
  isVisibleMemoryRememberCommandText,
  isVisibleMemoryReviewCommandText,
  rememberVisibleExplicitPreference,
  rememberVisibleExplicitPreferenceFromCaller,
  recallVisibleExplicitPreference,
  reviewVisibleExplicitPreferences,
  routeUnifiedVisibleMemoryCommandFromCaller,
  routeVisibleMemoryCommand,
  routeVisibleMemoryCommandFromCaller,
  type VisibleMemoryCommandAppAdapterAuditEvent
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

  it('answers forget-that commands through the explicit-preference forget helper', async () => {
    const store = new InMemoryTypedMemoryStore();
    await store.write(
      preference({
        user_id: 'u1',
        value: 'alice@example.com evenings only',
        source_ref: 'reply:private-forget-command-ref'
      })
    );
    await store.write(
      preference({
        user_id: 'u2',
        value: 'other-user private forget value alice@example.com',
        source_ref: 'reply:other-user-forget-command-secret',
        updated_at: '2026-06-27T12:00:00.000Z'
      })
    );

    const result = await answerVisibleMemoryForgetCommand(store, 'u1', 'Forget that preference', {
      attribute: 'alert_timing'
    });
    assert.ok(result);
    const json = JSON.stringify(result);

    assert.equal(result.action, 'forget_visible_explicit_preference');
    assert.equal(result.user_id, 'u1');
    assert.equal(result.answer, result.forget.message);
    assert.deepEqual(result.audit_metadata, {
      memory_kind: 'preference',
      matched_intent: 'memory_forget',
      scope_key: 'preference:alert_timing',
      forgotten_row_id: 1
    });
    assert.equal(result.forget.forgotten, true);
    assert.equal(Object.isFrozen(result), true);
    assert.equal(Object.isFrozen(result.audit_metadata), true);
    assert.equal(await recallVisibleExplicitPreference(store, 'u1', { attribute: 'alert_timing' }), null);
    assert.equal(
      (await recallVisibleExplicitPreference(store, 'u2', { attribute: 'alert_timing' }))?.preference_summary,
      'alert timing: [redacted]'
    );
    assert.equal(json.includes('alice@example.com'), false);
    assert.equal(json.includes('private-forget-command-ref'), false);
    assert.equal(json.includes('u2'), false);
    assert.equal(json.includes('other-user private forget value'), false);
    assert.equal(json.includes('other-user-forget-command-secret'), false);
  });

  it('answers correct-that commands through the explicit-preference correct helper', async () => {
    const store = new InMemoryTypedMemoryStore();
    await store.write(
      preference({
        user_id: 'u1',
        value: 'alice@example.com evenings only',
        source_ref: 'reply:private-correct-command-ref'
      })
    );
    await store.write(
      preference({
        user_id: 'u2',
        value: 'other-user private correct value alice@example.com',
        source_ref: 'reply:other-user-correct-command-secret',
        updated_at: '2026-06-27T12:00:00.000Z'
      })
    );

    const result = await answerVisibleMemoryCorrectCommand(
      store,
      'u1',
      'Correct that preference',
      { attribute: 'alert_timing' },
      {
        correctedValue: 'morning',
        updatedAt: '2026-06-28T09:00:00.000Z',
        sourceRef: 'reply:private-correction-command-ref'
      }
    );
    assert.ok(result);
    const json = JSON.stringify(result);

    assert.equal(result.action, 'correct_visible_explicit_preference');
    assert.equal(result.user_id, 'u1');
    assert.equal(result.answer, result.correction.message);
    assert.deepEqual(result.audit_metadata, {
      memory_kind: 'preference',
      matched_intent: 'memory_correct',
      scope_key: 'preference:alert_timing',
      previous_row_id: 1,
      corrected_row_id: 1,
      source: 'user_stated',
      updated_at: '2026-06-28T09:00:00.000Z'
    });
    assert.equal((await recallVisibleExplicitPreference(store, 'u1', { attribute: 'alert_timing' }))?.preference_summary, 'alert timing: morning');
    assert.equal(
      (await recallVisibleExplicitPreference(store, 'u2', { attribute: 'alert_timing' }))?.preference_summary,
      'alert timing: [redacted]'
    );
    assert.equal(Object.isFrozen(result), true);
    assert.equal(Object.isFrozen(result.audit_metadata), true);
    assert.equal(json.includes('alice@example.com'), false);
    assert.equal(json.includes('private-correct-command-ref'), false);
    assert.equal(json.includes('private-correction-command-ref'), false);
    assert.equal(json.includes('morning'), false);
    assert.equal(json.includes('u2'), false);
    assert.equal(json.includes('other-user private correct value'), false);
    assert.equal(json.includes('other-user-correct-command-secret'), false);
  });

  it('only routes forget/correct memory commands and leaves unknown or remember-this text alone', async () => {
    const store = new InMemoryTypedMemoryStore();
    await store.write(preference({ value: 'morning' }));

    assert.equal(isVisibleMemoryForgetCommandText('Forget that preference'), true);
    assert.equal(isVisibleMemoryForgetCommandText('Please forget my memory'), true);
    assert.equal(isVisibleMemoryForgetCommandText('Stop remembering it'), true);
    assert.equal(isVisibleMemoryForgetCommandText('Remember this: I prefer mornings'), false);
    assert.equal(isVisibleMemoryForgetCommandText('please save that I prefer mornings'), false);
    assert.equal(isVisibleMemoryForgetCommandText('Can you help me with my inbox?'), false);

    assert.equal(isVisibleMemoryCorrectCommandText('Correct that preference'), true);
    assert.equal(isVisibleMemoryCorrectCommandText('Update my memory'), true);
    assert.equal(isVisibleMemoryCorrectCommandText('That saved preference is wrong'), true);
    assert.equal(isVisibleMemoryCorrectCommandText('Remember this: I prefer mornings'), false);
    assert.equal(isVisibleMemoryCorrectCommandText('please save that I prefer mornings'), false);
    assert.equal(isVisibleMemoryCorrectCommandText('Can you help me with my inbox?'), false);

    assert.equal(
      await answerVisibleMemoryForgetCommand(store, 'u1', 'Can you help me with my inbox?', {
        attribute: 'alert_timing'
      }),
      null
    );
    assert.equal(
      await answerVisibleMemoryForgetCommand(store, 'u1', 'Remember this: I prefer mornings', {
        attribute: 'alert_timing'
      }),
      null
    );
    assert.equal(
      await answerVisibleMemoryCorrectCommand(
        store,
        'u1',
        'Can you help me with my inbox?',
        { attribute: 'alert_timing' },
        { correctedValue: 'evening' }
      ),
      null
    );
    assert.equal(
      await answerVisibleMemoryCorrectCommand(
        store,
        'u1',
        'Remember this: I prefer mornings',
        { attribute: 'alert_timing' },
        { correctedValue: 'evening' }
      ),
      null
    );
  });

  it('keeps the forget/correct command adapters on the same inactive-memory and cross-user exclusion path', async () => {
    const store = new InMemoryTypedMemoryStore();
    await store.write(preference({ value: 'morning', updated_at: '2026-06-25T10:00:00.000Z' }));
    await store.write(
      preference({
        user_id: 'u2',
        value: 'other-user forget-correct command secret alice@example.com',
        source_ref: 'reply:other-user-forget-correct-command'
      })
    );
    await store.write(preference({ scope_key: 'preference:low', attribute: 'low', confidence: 'low' }));
    await store.write(
      preference({
        scope_key: 'preference:stale',
        attribute: 'stale',
        value: 'should not change through command',
        stale_marked_at: '2026-06-25T00:00:00.000Z'
      })
    );
    await store.write(preference({ scope_key: 'preference:retracted', attribute: 'retracted', retracted: true }));
    await store.write(preference({ scope_key: 'preference:tombstoned', attribute: 'tombstoned', superseded_by: 99 }));

    const forget = await answerVisibleMemoryForgetCommand(store, 'u1', 'Forget that preference', {
      attribute: 'stale'
    });
    assert.equal(forget, null);
    const correct = await answerVisibleMemoryCorrectCommand(
      store,
      'u1',
      'Correct that preference',
      { attribute: 'tombstoned' },
      { correctedValue: 'evening', updatedAt: '2026-06-28T09:00:00.000Z' }
    );
    assert.equal(correct, null);
    assert.equal(
      await answerVisibleMemoryForgetCommand(store, 'u1', 'Forget that preference', {
        attribute: 'low'
      }),
      null
    );
    assert.equal(
      await answerVisibleMemoryCorrectCommand(
        store,
        'u1',
        'Correct that preference',
        { attribute: 'retracted' },
        { correctedValue: 'evening', updatedAt: '2026-06-28T09:00:00.000Z' }
      ),
      null
    );

    const activeCorrection = await answerVisibleMemoryCorrectCommand(
      store,
      'u1',
      'Correct that preference',
      { attribute: 'alert_timing' },
      { correctedValue: 'evening', updatedAt: '2026-06-28T09:00:00.000Z' }
    );
    assert.ok(activeCorrection);
    const json = JSON.stringify(activeCorrection);

    assert.equal(activeCorrection.audit_metadata.scope_key, 'preference:alert_timing');
    assert.equal(json.includes('u2'), false);
    assert.equal(json.includes('other-user forget-correct command secret'), false);
    assert.equal(json.includes('alice@example.com'), false);
    assert.equal(json.includes('other-user-forget-correct-command'), false);
    assert.equal(json.includes('should not change through command'), false);
    assert.equal(json.includes('stale'), false);
    assert.equal(json.includes('retracted'), false);
    assert.equal(json.includes('tombstoned'), false);
  });

  it('routes visible memory commands to review, explanation, forget, and correct adapters', async () => {
    const reviewStore = new InMemoryTypedMemoryStore();
    await reviewStore.write(preference({ value: 'alice@example.com evenings only' }));

    const review = await routeVisibleMemoryCommand(reviewStore, 'u1', 'What do you remember about me?', {
      query: { attribute: 'alert_timing' }
    });
    assert.ok(review);
    assert.equal(review.action, 'review_visible_explicit_preferences');
    assert.equal(review.answer.includes('[redacted]'), true);

    const explanation = await routeVisibleMemoryCommand(reviewStore, 'u1', 'Why did you use that?', {
      query: { attribute: 'alert_timing' }
    });
    assert.ok(explanation);
    assert.equal(explanation.action, 'explain_visible_explicit_preference_use');
    assert.match(explanation.answer, /matched the saved alert timing preference/);

    const forgetStore = new InMemoryTypedMemoryStore();
    await forgetStore.write(preference({ value: 'evening' }));
    const forget = await routeVisibleMemoryCommand(forgetStore, 'u1', 'Forget that preference', {
      query: { attribute: 'alert_timing' }
    });
    assert.ok(forget);
    assert.equal(forget.action, 'forget_visible_explicit_preference');
    assert.equal(await recallVisibleExplicitPreference(forgetStore, 'u1', { attribute: 'alert_timing' }), null);

    const correctStore = new InMemoryTypedMemoryStore();
    await correctStore.write(preference({ value: 'evening' }));
    const correction = await routeVisibleMemoryCommand(correctStore, 'u1', 'Correct that preference', {
      query: { attribute: 'alert_timing' },
      correction: {
        correctedValue: 'morning',
        updatedAt: '2026-06-28T09:00:00.000Z',
        sourceRef: 'reply:private-router-correction-ref'
      }
    });
    assert.ok(correction);
    assert.equal(correction.action, 'correct_visible_explicit_preference');
    assert.equal(
      (await recallVisibleExplicitPreference(correctStore, 'u1', { attribute: 'alert_timing' }))?.preference_summary,
      'alert timing: morning'
    );
    const json = JSON.stringify(correction);
    assert.equal(json.includes('private-router-correction-ref'), false);
    assert.equal(json.includes('morning'), false);
  });

  it('returns null for unknown or non-memory text without writing or retracting', async () => {
    class CountingStore extends InMemoryTypedMemoryStore {
      writeCount = 0;
      retractCount = 0;

      override async write(input: NewTypedMemoryRow) {
        this.writeCount += 1;
        return super.write(input);
      }

      override async retract(
        userId: string,
        kind: Parameters<InMemoryTypedMemoryStore['retract']>[1],
        scopeKey: string,
        supersededBy: number | null = null
      ) {
        this.retractCount += 1;
        return super.retract(userId, kind, scopeKey, supersededBy);
      }
    }

    const store = new CountingStore();
    await store.write(preference({ value: 'morning' }));
    store.writeCount = 0;

    assert.equal(await routeVisibleMemoryCommand(store, 'u1', 'Can you help me with my inbox?'), null);
    assert.equal(await routeVisibleMemoryCommand(store, 'u1', 'Remember this: I prefer mornings'), null);
    assert.equal(await routeVisibleMemoryCommand(store, 'u1', 'Forget that preference'), null);

    assert.equal(store.writeCount, 0);
    assert.equal(store.retractCount, 0);
    assert.equal(
      (await recallVisibleExplicitPreference(store, 'u1', { attribute: 'alert_timing' }))?.preference_summary,
      'alert timing: morning'
    );
  });

  it('does not trigger correction through the router when correction options are missing', async () => {
    class CountingStore extends InMemoryTypedMemoryStore {
      writeCount = 0;

      override async write(input: NewTypedMemoryRow) {
        this.writeCount += 1;
        return super.write(input);
      }
    }

    const store = new CountingStore();
    await store.write(preference({ value: 'evening' }));
    store.writeCount = 0;

    const result = await routeVisibleMemoryCommand(store, 'u1', 'Correct that preference', {
      query: { attribute: 'alert_timing' }
    });

    assert.equal(result, null);
    assert.equal(store.writeCount, 0);
    assert.equal(
      (await recallVisibleExplicitPreference(store, 'u1', { attribute: 'alert_timing' }))?.preference_summary,
      'alert timing: evening'
    );
  });

  it('keeps router results on the same unsafe-memory and cross-user exclusion path', async () => {
    const store = new InMemoryTypedMemoryStore();
    await store.write(preference({ value: 'morning', updated_at: '2026-06-25T10:00:00.000Z' }));
    await store.write(
      preference({
        user_id: 'u2',
        value: 'other-user router secret alice@example.com',
        source_ref: 'reply:other-user-router-secret'
      })
    );
    await store.write(preference({ scope_key: 'preference:low', attribute: 'low', confidence: 'low' }));
    await store.write(
      preference({
        scope_key: 'preference:stale',
        attribute: 'stale',
        value: 'should not route',
        stale_marked_at: '2026-06-25T00:00:00.000Z'
      })
    );
    await store.write(preference({ scope_key: 'preference:retracted', attribute: 'retracted', retracted: true }));
    await store.write(preference({ scope_key: 'preference:tombstoned', attribute: 'tombstoned', superseded_by: 99 }));

    const review = await routeVisibleMemoryCommand(store, 'u1', 'List my saved preferences');
    assert.ok(review);
    assert.equal(review.action, 'review_visible_explicit_preferences');
    const reviewJson = JSON.stringify(review);
    assert.deepEqual(review.review.preferences.map((item) => item.preference_summary), ['alert timing: morning']);
    assert.equal(reviewJson.includes('u2'), false);
    assert.equal(reviewJson.includes('other-user router secret'), false);
    assert.equal(reviewJson.includes('alice@example.com'), false);
    assert.equal(reviewJson.includes('other-user-router-secret'), false);
    assert.equal(reviewJson.includes('should not route'), false);
    assert.equal(reviewJson.includes('stale'), false);
    assert.equal(reviewJson.includes('retracted'), false);
    assert.equal(reviewJson.includes('tombstoned'), false);

    assert.equal(
      await routeVisibleMemoryCommand(store, 'u1', 'Why did you use it?', { query: { attribute: 'stale' } }),
      null
    );
    assert.equal(
      await routeVisibleMemoryCommand(store, 'u1', 'Forget that preference', { query: { attribute: 'retracted' } }),
      null
    );
    assert.equal(
      await routeVisibleMemoryCommand(store, 'u1', 'Correct that preference', {
        query: { attribute: 'tombstoned' },
        correction: { correctedValue: 'evening', updatedAt: '2026-06-28T09:00:00.000Z' }
      }),
      null
    );
  });

  it('routes caller-supplied visible memory command context through the dormant integration seam', async () => {
    const reviewStore = new InMemoryTypedMemoryStore();
    await reviewStore.write(preference({ value: 'alice@example.com evenings only' }));

    const review = await routeVisibleMemoryCommandFromCaller(reviewStore, {
      userId: 'u1',
      text: 'What do you remember about me?',
      query: { attribute: 'alert_timing' }
    });
    assert.ok(review);
    assert.equal(review.action, 'review_visible_explicit_preferences');
    assert.equal(review.answer.includes('[redacted]'), true);

    const explanation = await routeVisibleMemoryCommandFromCaller(reviewStore, {
      userId: 'u1',
      text: 'Why did you use that?',
      query: { attribute: 'alert_timing' }
    });
    assert.ok(explanation);
    assert.equal(explanation.action, 'explain_visible_explicit_preference_use');
    assert.match(explanation.answer, /matched the saved alert timing preference/);

    const forgetStore = new InMemoryTypedMemoryStore();
    await forgetStore.write(preference({ value: 'evening' }));
    const forget = await routeVisibleMemoryCommandFromCaller(forgetStore, {
      userId: 'u1',
      text: 'Forget that preference',
      query: { attribute: 'alert_timing' }
    });
    assert.ok(forget);
    assert.equal(forget.action, 'forget_visible_explicit_preference');
    assert.equal(await recallVisibleExplicitPreference(forgetStore, 'u1', { attribute: 'alert_timing' }), null);

    const correctStore = new InMemoryTypedMemoryStore();
    await correctStore.write(preference({ value: 'evening' }));
    const correction = await routeVisibleMemoryCommandFromCaller(correctStore, {
      userId: 'u1',
      text: 'Correct that preference',
      query: { attribute: 'alert_timing' },
      correction: {
        correctedValue: 'morning',
        updatedAt: '2026-06-28T09:00:00.000Z',
        sourceRef: 'reply:private-caller-correction-ref'
      }
    });
    assert.ok(correction);
    assert.equal(correction.action, 'correct_visible_explicit_preference');
    assert.equal(
      (await recallVisibleExplicitPreference(correctStore, 'u1', { attribute: 'alert_timing' }))?.preference_summary,
      'alert timing: morning'
    );
    const json = JSON.stringify(correction);
    assert.equal(json.includes('private-caller-correction-ref'), false);
    assert.equal(json.includes('morning'), false);
  });

  it('keeps caller seam unknown text and destructive commands inert without explicit caller context', async () => {
    class CountingStore extends InMemoryTypedMemoryStore {
      writeCount = 0;
      retractCount = 0;

      override async write(input: NewTypedMemoryRow) {
        this.writeCount += 1;
        return super.write(input);
      }

      override async retract(
        userId: string,
        kind: Parameters<InMemoryTypedMemoryStore['retract']>[1],
        scopeKey: string,
        supersededBy: number | null = null
      ) {
        this.retractCount += 1;
        return super.retract(userId, kind, scopeKey, supersededBy);
      }
    }

    const store = new CountingStore();
    await store.write(preference({ value: 'morning' }));
    store.writeCount = 0;

    assert.equal(await routeVisibleMemoryCommandFromCaller(store, { userId: 'u1', text: 'Can you help me with my inbox?' }), null);
    assert.equal(await routeVisibleMemoryCommandFromCaller(store, { userId: 'u1', text: 'Remember this: I prefer mornings' }), null);
    assert.equal(await routeVisibleMemoryCommandFromCaller(store, { userId: 'u1', text: 'Forget that preference' }), null);
    assert.equal(
      await routeVisibleMemoryCommandFromCaller(store, {
        userId: 'u1',
        text: 'Correct that preference',
        query: { attribute: 'alert_timing' }
      }),
      null
    );

    assert.equal(store.writeCount, 0);
    assert.equal(store.retractCount, 0);
    assert.equal(
      (await recallVisibleExplicitPreference(store, 'u1', { attribute: 'alert_timing' }))?.preference_summary,
      'alert timing: morning'
    );
  });

  it('keeps caller seam results on the same unsafe-memory and cross-user exclusion path', async () => {
    const store = new InMemoryTypedMemoryStore();
    await store.write(preference({ value: 'morning', updated_at: '2026-06-25T10:00:00.000Z' }));
    await store.write(
      preference({
        user_id: 'u2',
        value: 'other-user caller seam secret alice@example.com',
        source_ref: 'reply:other-user-caller-seam-secret'
      })
    );
    await store.write(preference({ scope_key: 'preference:low', attribute: 'low', confidence: 'low' }));
    await store.write(
      preference({
        scope_key: 'preference:stale',
        attribute: 'stale',
        value: 'should not route through caller seam',
        stale_marked_at: '2026-06-25T00:00:00.000Z'
      })
    );
    await store.write(preference({ scope_key: 'preference:retracted', attribute: 'retracted', retracted: true }));
    await store.write(preference({ scope_key: 'preference:tombstoned', attribute: 'tombstoned', superseded_by: 99 }));

    const review = await routeVisibleMemoryCommandFromCaller(store, {
      userId: 'u1',
      text: 'List my saved preferences'
    });
    assert.ok(review);
    assert.equal(review.action, 'review_visible_explicit_preferences');
    const reviewJson = JSON.stringify(review);
    assert.deepEqual(review.review.preferences.map((item) => item.preference_summary), ['alert timing: morning']);
    assert.equal(reviewJson.includes('u2'), false);
    assert.equal(reviewJson.includes('other-user caller seam secret'), false);
    assert.equal(reviewJson.includes('alice@example.com'), false);
    assert.equal(reviewJson.includes('other-user-caller-seam-secret'), false);
    assert.equal(reviewJson.includes('should not route through caller seam'), false);
    assert.equal(reviewJson.includes('stale'), false);
    assert.equal(reviewJson.includes('retracted'), false);
    assert.equal(reviewJson.includes('tombstoned'), false);

    assert.equal(
      await routeVisibleMemoryCommandFromCaller(store, {
        userId: 'u1',
        text: 'Why did you use it?',
        query: { attribute: 'stale' }
      }),
      null
    );
    assert.equal(
      await routeVisibleMemoryCommandFromCaller(store, {
        userId: 'u1',
        text: 'Forget that preference',
        query: { attribute: 'retracted' }
      }),
      null
    );
    assert.equal(
      await routeVisibleMemoryCommandFromCaller(store, {
        userId: 'u1',
        text: 'Correct that preference',
        query: { attribute: 'tombstoned' },
        correction: { correctedValue: 'evening', updatedAt: '2026-06-28T09:00:00.000Z' }
      }),
      null
    );
  });

  it('routes caller-supplied remember-this context through the dormant explicit-preference seam', async () => {
    const store = new InMemoryTypedMemoryStore();

    assert.equal(isVisibleMemoryRememberCommandText('Remember this: I prefer mornings'), true);
    assert.equal(isVisibleMemoryRememberCommandText('Please save that I prefer mornings'), true);
    assert.equal(isVisibleMemoryRememberCommandText('Can you remember my alert timing?'), true);
    assert.equal(isVisibleMemoryRememberCommandText('What do you remember about me?'), false);
    assert.equal(isVisibleMemoryRememberCommandText('Forget that preference'), false);

    const remembered = await rememberVisibleExplicitPreferenceFromCaller(store, {
      userId: 'u1',
      text: 'Remember this: I prefer mornings',
      parsedPreference: {
        attribute: 'alert_timing',
        value: 'alice@example.com mornings only',
        updatedAt: '2026-06-30T09:00:00.000Z',
        sourceRef: 'reply:private-remember-caller-ref'
      }
    });
    assert.ok(remembered);
    const json = JSON.stringify(remembered);

    assert.equal(remembered.action, 'remember_visible_explicit_preference');
    assert.equal(remembered.user_id, 'u1');
    assert.equal(remembered.answer, remembered.remember.message);
    assert.deepEqual(remembered.audit_metadata, {
      memory_kind: 'preference',
      matched_intent: 'memory_remember',
      scope_key: 'preference:alert_timing',
      remembered_row_id: 1,
      source: 'user_stated',
      confidence: 'high',
      updated_at: '2026-06-30T09:00:00.000Z'
    });
    assert.equal(Object.isFrozen(remembered), true);
    assert.equal(Object.isFrozen(remembered.audit_metadata), true);
    assert.equal(json.includes('alice@example.com'), false);
    assert.equal(json.includes('mornings only'), false);
    assert.equal(json.includes('private-remember-caller-ref'), false);

    const recalled = await recallVisibleExplicitPreference(store, 'u1', { attribute: 'alert_timing' });
    assert.equal(recalled?.preference_summary, 'alert timing: [redacted]');
    assert.equal(recalled?.source_metadata.source_ref_type, 'reply');
  });

  it('keeps remember caller seam inert without caller-supplied parsed preference context', async () => {
    class CountingStore extends InMemoryTypedMemoryStore {
      writeCount = 0;

      override async write(input: NewTypedMemoryRow) {
        this.writeCount += 1;
        return super.write(input);
      }
    }

    const store = new CountingStore();

    assert.equal(
      await rememberVisibleExplicitPreferenceFromCaller(store, {
        userId: 'u1',
        text: 'Can you help me with my inbox?',
        parsedPreference: { attribute: 'alert_timing', value: 'morning' }
      }),
      null
    );
    assert.equal(
      await rememberVisibleExplicitPreferenceFromCaller(store, {
        userId: 'u1',
        text: 'What do you remember about me?',
        parsedPreference: { attribute: 'alert_timing', value: 'morning' }
      }),
      null
    );
    assert.equal(
      await rememberVisibleExplicitPreferenceFromCaller(store, {
        userId: 'u1',
        text: 'Why did you use that?',
        parsedPreference: { attribute: 'alert_timing', value: 'morning' }
      }),
      null
    );
    assert.equal(
      await rememberVisibleExplicitPreferenceFromCaller(store, {
        userId: 'u1',
        text: 'Forget that preference',
        parsedPreference: { attribute: 'alert_timing', value: 'morning' }
      }),
      null
    );
    assert.equal(
      await rememberVisibleExplicitPreferenceFromCaller(store, {
        userId: 'u1',
        text: 'Correct that preference',
        parsedPreference: { attribute: 'alert_timing', value: 'morning' }
      }),
      null
    );
    assert.equal(
      await rememberVisibleExplicitPreferenceFromCaller(store, {
        userId: 'u1',
        text: 'Remember this: I prefer mornings'
      }),
      null
    );

    assert.equal(store.writeCount, 0);
    assert.equal(await recallVisibleExplicitPreference(store, 'u1', { attribute: 'alert_timing' }), null);
  });

  it('keeps remember caller seam scoped to one user without leaking other-user/private material', async () => {
    const store = new InMemoryTypedMemoryStore();

    await rememberVisibleExplicitPreferenceFromCaller(store, {
      userId: 'u1',
      text: 'Remember this: I prefer mornings',
      parsedPreference: {
        attribute: 'alert_timing',
        value: 'morning',
        updatedAt: '2026-06-30T09:00:00.000Z'
      }
    });
    await rememberVisibleExplicitPreferenceFromCaller(store, {
      userId: 'u2',
      text: 'Remember this: I prefer midnight',
      parsedPreference: {
        attribute: 'alert_timing',
        value: 'other user secret alice@example.com',
        updatedAt: '2026-06-30T10:00:00.000Z',
        sourceRef: 'reply:other-user-remember-caller-ref'
      }
    });

    const recall = await recallVisibleExplicitPreference(store, 'u1', { attribute: 'alert_timing' });
    const json = JSON.stringify(recall);

    assert.equal(recall?.user_id, 'u1');
    assert.equal(recall?.preference_summary, 'alert timing: morning');
    assert.equal(json.includes('u2'), false);
    assert.equal(json.includes('other user secret'), false);
    assert.equal(json.includes('alice@example.com'), false);
    assert.equal(json.includes('other-user-remember-caller-ref'), false);
  });

  it('routes remember, review, explanation, forget, and correct through one unified dormant caller seam', async () => {
    const rememberStore = new InMemoryTypedMemoryStore();
    const remembered = await routeUnifiedVisibleMemoryCommandFromCaller(rememberStore, {
      userId: 'u1',
      text: 'Remember this: I prefer mornings',
      parsedPreference: {
        attribute: 'alert_timing',
        value: 'alice@example.com mornings only',
        updatedAt: '2026-07-01T09:00:00.000Z',
        sourceRef: 'reply:private-unified-remember-ref'
      }
    });
    assert.ok(remembered);
    assert.equal(remembered.action, 'remember_visible_explicit_preference');
    assert.equal(remembered.answer.includes('alert timing'), true);
    assert.equal(JSON.stringify(remembered).includes('alice@example.com'), false);
    assert.equal(JSON.stringify(remembered).includes('mornings only'), false);
    assert.equal(JSON.stringify(remembered).includes('private-unified-remember-ref'), false);
    assert.equal(
      (await recallVisibleExplicitPreference(rememberStore, 'u1', { attribute: 'alert_timing' }))?.preference_summary,
      'alert timing: [redacted]'
    );

    const review = await routeUnifiedVisibleMemoryCommandFromCaller(rememberStore, {
      userId: 'u1',
      text: 'What do you remember about me?',
      query: { attribute: 'alert_timing' }
    });
    assert.ok(review);
    assert.equal(review.action, 'review_visible_explicit_preferences');
    assert.equal(review.answer.includes('[redacted]'), true);

    const explanation = await routeUnifiedVisibleMemoryCommandFromCaller(rememberStore, {
      userId: 'u1',
      text: 'Why did you use that?',
      query: { attribute: 'alert_timing' }
    });
    assert.ok(explanation);
    assert.equal(explanation.action, 'explain_visible_explicit_preference_use');
    assert.match(explanation.answer, /matched the saved alert timing preference/);

    const correct = await routeUnifiedVisibleMemoryCommandFromCaller(rememberStore, {
      userId: 'u1',
      text: 'Correct that preference',
      query: { attribute: 'alert_timing' },
      correction: {
        correctedValue: 'evening',
        updatedAt: '2026-07-01T10:00:00.000Z',
        sourceRef: 'reply:private-unified-correction-ref'
      }
    });
    assert.ok(correct);
    assert.equal(correct.action, 'correct_visible_explicit_preference');
    assert.equal(JSON.stringify(correct).includes('private-unified-correction-ref'), false);
    assert.equal(JSON.stringify(correct).includes('evening'), false);
    assert.equal(
      (await recallVisibleExplicitPreference(rememberStore, 'u1', { attribute: 'alert_timing' }))?.preference_summary,
      'alert timing: evening'
    );

    const forget = await routeUnifiedVisibleMemoryCommandFromCaller(rememberStore, {
      userId: 'u1',
      text: 'Forget that preference',
      query: { attribute: 'alert_timing' }
    });
    assert.ok(forget);
    assert.equal(forget.action, 'forget_visible_explicit_preference');
    assert.equal(await recallVisibleExplicitPreference(rememberStore, 'u1', { attribute: 'alert_timing' }), null);
  });

  it('keeps the unified caller seam inert for unknown text and missing caller-supplied context', async () => {
    class CountingStore extends InMemoryTypedMemoryStore {
      writeCount = 0;
      retractCount = 0;

      override async write(input: NewTypedMemoryRow) {
        this.writeCount += 1;
        return super.write(input);
      }

      override async retract(
        userId: string,
        kind: Parameters<InMemoryTypedMemoryStore['retract']>[1],
        scopeKey: string,
        supersededBy: number | null = null
      ) {
        this.retractCount += 1;
        return super.retract(userId, kind, scopeKey, supersededBy);
      }
    }

    const store = new CountingStore();
    await store.write(preference({ value: 'morning' }));
    store.writeCount = 0;

    assert.equal(
      await routeUnifiedVisibleMemoryCommandFromCaller(store, {
        userId: 'u1',
        text: 'Can you help me with my inbox?',
        parsedPreference: { attribute: 'alert_timing', value: 'should not write' },
        query: { attribute: 'alert_timing' },
        correction: { correctedValue: 'should not correct' }
      }),
      null
    );
    assert.equal(
      await routeUnifiedVisibleMemoryCommandFromCaller(store, {
        userId: 'u1',
        text: 'Remember this: I prefer afternoons'
      }),
      null
    );
    assert.equal(
      await routeUnifiedVisibleMemoryCommandFromCaller(store, {
        userId: 'u1',
        text: 'Forget that preference'
      }),
      null
    );
    assert.equal(
      await routeUnifiedVisibleMemoryCommandFromCaller(store, {
        userId: 'u1',
        text: 'Correct that preference',
        query: { attribute: 'alert_timing' }
      }),
      null
    );

    assert.equal(store.writeCount, 0);
    assert.equal(store.retractCount, 0);
    assert.equal(
      (await recallVisibleExplicitPreference(store, 'u1', { attribute: 'alert_timing' }))?.preference_summary,
      'alert timing: morning'
    );
  });

  it('keeps unified caller seam user-scoped and excludes unsafe memory results', async () => {
    const store = new InMemoryTypedMemoryStore();
    await store.write(preference({ value: 'morning', updated_at: '2026-06-25T10:00:00.000Z' }));
    await store.write(
      preference({
        user_id: 'u2',
        value: 'other-user unified seam secret alice@example.com',
        source_ref: 'reply:other-user-unified-seam-secret'
      })
    );
    await store.write(preference({ scope_key: 'preference:low', attribute: 'low', confidence: 'low' }));
    await store.write(
      preference({
        scope_key: 'preference:stale',
        attribute: 'stale',
        value: 'should not route through unified caller seam',
        stale_marked_at: '2026-06-25T00:00:00.000Z'
      })
    );
    await store.write(preference({ scope_key: 'preference:retracted', attribute: 'retracted', retracted: true }));
    await store.write(preference({ scope_key: 'preference:tombstoned', attribute: 'tombstoned', superseded_by: 99 }));

    const review = await routeUnifiedVisibleMemoryCommandFromCaller(store, {
      userId: 'u1',
      text: 'List my saved preferences'
    });
    assert.ok(review);
    assert.equal(review.action, 'review_visible_explicit_preferences');
    const reviewJson = JSON.stringify(review);
    assert.deepEqual(review.review.preferences.map((item) => item.preference_summary), ['alert timing: morning']);
    assert.equal(reviewJson.includes('u2'), false);
    assert.equal(reviewJson.includes('other-user unified seam secret'), false);
    assert.equal(reviewJson.includes('alice@example.com'), false);
    assert.equal(reviewJson.includes('other-user-unified-seam-secret'), false);
    assert.equal(reviewJson.includes('should not route through unified caller seam'), false);
    assert.equal(reviewJson.includes('stale'), false);
    assert.equal(reviewJson.includes('retracted'), false);
    assert.equal(reviewJson.includes('tombstoned'), false);

    assert.equal(
      await routeUnifiedVisibleMemoryCommandFromCaller(store, {
        userId: 'u1',
        text: 'Why did you use it?',
        query: { attribute: 'stale' }
      }),
      null
    );
    assert.equal(
      await routeUnifiedVisibleMemoryCommandFromCaller(store, {
        userId: 'u1',
        text: 'Forget that preference',
        query: { attribute: 'retracted' }
      }),
      null
    );
    assert.equal(
      await routeUnifiedVisibleMemoryCommandFromCaller(store, {
        userId: 'u1',
        text: 'Correct that preference',
        query: { attribute: 'tombstoned' },
        correction: { correctedValue: 'evening', updatedAt: '2026-07-01T10:00:00.000Z' }
      }),
      null
    );
  });

  it('keeps the visible memory command handler disabled by default without reading, writing, or retracting', async () => {
    class CountingStore extends InMemoryTypedMemoryStore {
      listCount = 0;
      writeCount = 0;
      retractCount = 0;

      override async listActive(...args: Parameters<InMemoryTypedMemoryStore['listActive']>) {
        this.listCount += 1;
        return super.listActive(...args);
      }

      override async write(input: NewTypedMemoryRow) {
        this.writeCount += 1;
        return super.write(input);
      }

      override async retract(
        userId: string,
        kind: Parameters<InMemoryTypedMemoryStore['retract']>[1],
        scopeKey: string,
        supersededBy: number | null = null
      ) {
        this.retractCount += 1;
        return super.retract(userId, kind, scopeKey, supersededBy);
      }
    }

    const store = new CountingStore();
    await store.write(preference({ value: 'morning' }));
    store.listCount = 0;
    store.writeCount = 0;

    const result = await handleVisibleMemoryCommandFromCaller(store, {
      userId: 'u1',
      text: 'Remember this: I prefer afternoons',
      parsedPreference: {
        attribute: 'alert_timing',
        value: 'alice@example.com afternoons only',
        sourceRef: 'reply:private-disabled-handler-ref'
      }
    });

    assert.deepEqual(result, {
      handled: false,
      status: 'disabled',
      user_id: 'u1',
      response: { text: null },
      command_result: null,
      audit_metadata: {
        memory_kind: 'preference',
        handler: 'visible_memory_command_handler',
        enabled: false
      }
    });
    assert.equal(Object.isFrozen(result), true);
    assert.equal(Object.isFrozen(result.response), true);
    assert.equal(Object.isFrozen(result.audit_metadata), true);
    assert.equal(store.listCount, 0);
    assert.equal(store.writeCount, 0);
    assert.equal(store.retractCount, 0);
    assert.equal(
      (await recallVisibleExplicitPreference(store, 'u1', { attribute: 'alert_timing' }))?.preference_summary,
      'alert timing: morning'
    );
    const json = JSON.stringify(result);
    assert.equal(json.includes('alice@example.com'), false);
    assert.equal(json.includes('afternoons only'), false);
    assert.equal(json.includes('private-disabled-handler-ref'), false);
  });

  it('routes enabled visible memory command handler calls through explicit caller-supplied context only', async () => {
    const store = new InMemoryTypedMemoryStore();

    const remembered = await handleVisibleMemoryCommandFromCaller(store, {
      enabled: true,
      userId: 'u1',
      text: 'Remember this: I prefer mornings',
      parsedPreference: {
        attribute: 'alert_timing',
        value: 'alice@example.com mornings only',
        updatedAt: '2026-07-02T09:00:00.000Z',
        sourceRef: 'reply:private-handler-remember-ref'
      }
    });
    assert.equal(remembered.handled, true);
    assert.equal(remembered.status, 'handled');
    assert.equal(remembered.command_result?.action, 'remember_visible_explicit_preference');
    assert.equal(remembered.response.text, remembered.command_result?.answer);
    assert.deepEqual(remembered.audit_metadata, {
      memory_kind: 'preference',
      handler: 'visible_memory_command_handler',
      enabled: true,
      matched_action: 'remember_visible_explicit_preference',
      matched_intent: 'memory_remember'
    });
    assert.equal(JSON.stringify(remembered).includes('alice@example.com'), false);
    assert.equal(JSON.stringify(remembered).includes('mornings only'), false);
    assert.equal(JSON.stringify(remembered).includes('private-handler-remember-ref'), false);

    const review = await handleVisibleMemoryCommandFromCaller(store, {
      enabled: true,
      userId: 'u1',
      text: 'What do you remember about me?',
      query: { attribute: 'alert_timing' }
    });
    assert.equal(review.handled, true);
    assert.equal(review.command_result?.action, 'review_visible_explicit_preferences');
    assert.equal(review.response.text?.includes('[redacted]'), true);
    assert.deepEqual(review.audit_metadata, {
      memory_kind: 'preference',
      handler: 'visible_memory_command_handler',
      enabled: true,
      matched_action: 'review_visible_explicit_preferences',
      matched_intent: 'memory_review',
      returned_count: 1,
      row_ids: [1],
      scope_keys: ['preference:alert_timing']
    });

    const explanation = await handleVisibleMemoryCommandFromCaller(store, {
      enabled: true,
      userId: 'u1',
      text: 'Why did you use that?',
      query: { attribute: 'alert_timing' }
    });
    assert.equal(explanation.handled, true);
    assert.equal(explanation.command_result?.action, 'explain_visible_explicit_preference_use');

    const correction = await handleVisibleMemoryCommandFromCaller(store, {
      enabled: true,
      userId: 'u1',
      text: 'Correct that preference',
      query: { attribute: 'alert_timing' },
      correction: {
        correctedValue: 'evening',
        updatedAt: '2026-07-02T10:00:00.000Z',
        sourceRef: 'reply:private-handler-correction-ref'
      }
    });
    assert.equal(correction.handled, true);
    assert.equal(correction.command_result?.action, 'correct_visible_explicit_preference');
    assert.equal(JSON.stringify(correction).includes('private-handler-correction-ref'), false);
    assert.equal(JSON.stringify(correction).includes('evening'), false);

    const forget = await handleVisibleMemoryCommandFromCaller(store, {
      enabled: true,
      userId: 'u1',
      text: 'Forget that preference',
      query: { attribute: 'alert_timing' }
    });
    assert.equal(forget.handled, true);
    assert.equal(forget.command_result?.action, 'forget_visible_explicit_preference');
    assert.equal(await recallVisibleExplicitPreference(store, 'u1', { attribute: 'alert_timing' }), null);
  });

  it('keeps enabled handler unknown text and missing parsed context as no-ops', async () => {
    class CountingStore extends InMemoryTypedMemoryStore {
      writeCount = 0;
      retractCount = 0;

      override async write(input: NewTypedMemoryRow) {
        this.writeCount += 1;
        return super.write(input);
      }

      override async retract(
        userId: string,
        kind: Parameters<InMemoryTypedMemoryStore['retract']>[1],
        scopeKey: string,
        supersededBy: number | null = null
      ) {
        this.retractCount += 1;
        return super.retract(userId, kind, scopeKey, supersededBy);
      }
    }

    const store = new CountingStore();
    await store.write(preference({ value: 'morning' }));
    store.writeCount = 0;

    const unknown = await handleVisibleMemoryCommandFromCaller(store, {
      enabled: true,
      userId: 'u1',
      text: 'Can you help me with my inbox?',
      parsedPreference: { attribute: 'alert_timing', value: 'should not write' },
      query: { attribute: 'alert_timing' },
      correction: { correctedValue: 'should not correct' }
    });
    assert.equal(unknown.handled, false);
    assert.equal(unknown.status, 'no_memory_command');
    assert.deepEqual(unknown.audit_metadata, {
      memory_kind: 'preference',
      handler: 'visible_memory_command_handler',
      enabled: true
    });

    assert.equal(
      await handleVisibleMemoryCommandFromCaller(store, {
        enabled: true,
        userId: 'u1',
        text: 'Remember this: I prefer afternoons'
      }).then((handlerResult) => handlerResult.command_result),
      null
    );
    assert.equal(
      await handleVisibleMemoryCommandFromCaller(store, {
        enabled: true,
        userId: 'u1',
        text: 'Forget that preference'
      }).then((handlerResult) => handlerResult.command_result),
      null
    );
    assert.equal(
      await handleVisibleMemoryCommandFromCaller(store, {
        enabled: true,
        userId: 'u1',
        text: 'Correct that preference',
        query: { attribute: 'alert_timing' }
      }).then((handlerResult) => handlerResult.command_result),
      null
    );

    assert.equal(store.writeCount, 0);
    assert.equal(store.retractCount, 0);
    assert.equal(
      (await recallVisibleExplicitPreference(store, 'u1', { attribute: 'alert_timing' }))?.preference_summary,
      'alert timing: morning'
    );
  });

  it('keeps enabled handler user-scoped and excludes unsafe or cross-user memory results', async () => {
    const store = new InMemoryTypedMemoryStore();
    await store.write(preference({ value: 'morning', updated_at: '2026-06-25T10:00:00.000Z' }));
    await store.write(
      preference({
        user_id: 'u2',
        value: 'other-user handler secret alice@example.com',
        source_ref: 'reply:other-user-handler-secret'
      })
    );
    await store.write(preference({ scope_key: 'preference:low', attribute: 'low', confidence: 'low' }));
    await store.write(
      preference({
        scope_key: 'preference:stale',
        attribute: 'stale',
        value: 'should not route through handler',
        stale_marked_at: '2026-06-25T00:00:00.000Z'
      })
    );
    await store.write(preference({ scope_key: 'preference:retracted', attribute: 'retracted', retracted: true }));
    await store.write(preference({ scope_key: 'preference:tombstoned', attribute: 'tombstoned', superseded_by: 99 }));

    const review = await handleVisibleMemoryCommandFromCaller(store, {
      enabled: true,
      userId: 'u1',
      text: 'List my saved preferences'
    });
    assert.equal(review.handled, true);
    assert.equal(review.command_result?.action, 'review_visible_explicit_preferences');
    const reviewJson = JSON.stringify(review);
    assert.equal(reviewJson.includes('u2'), false);
    assert.equal(reviewJson.includes('other-user handler secret'), false);
    assert.equal(reviewJson.includes('alice@example.com'), false);
    assert.equal(reviewJson.includes('other-user-handler-secret'), false);
    assert.equal(reviewJson.includes('should not route through handler'), false);
    assert.equal(reviewJson.includes('stale'), false);
    assert.equal(reviewJson.includes('retracted'), false);
    assert.equal(reviewJson.includes('tombstoned'), false);

    assert.equal(
      await handleVisibleMemoryCommandFromCaller(store, {
        enabled: true,
        userId: 'u1',
        text: 'Why did you use it?',
        query: { attribute: 'stale' }
      }).then((handlerResult) => handlerResult.command_result),
      null
    );
    assert.equal(
      await handleVisibleMemoryCommandFromCaller(store, {
        enabled: true,
        userId: 'u1',
        text: 'Forget that preference',
        query: { attribute: 'retracted' }
      }).then((handlerResult) => handlerResult.command_result),
      null
    );
    assert.equal(
      await handleVisibleMemoryCommandFromCaller(store, {
        enabled: true,
        userId: 'u1',
        text: 'Correct that preference',
        query: { attribute: 'tombstoned' },
        correction: { correctedValue: 'evening', updatedAt: '2026-07-02T10:00:00.000Z' }
      }).then((handlerResult) => handlerResult.command_result),
      null
    );
  });

  it('keeps the app-level visible memory command adapter disabled by default without reading, writing, or retracting', async () => {
    class CountingStore extends InMemoryTypedMemoryStore {
      listCount = 0;
      writeCount = 0;
      retractCount = 0;

      override async listActive(...args: Parameters<InMemoryTypedMemoryStore['listActive']>) {
        this.listCount += 1;
        return super.listActive(...args);
      }

      override async write(input: NewTypedMemoryRow) {
        this.writeCount += 1;
        return super.write(input);
      }

      override async retract(
        userId: string,
        kind: Parameters<InMemoryTypedMemoryStore['retract']>[1],
        scopeKey: string,
        supersededBy: number | null = null
      ) {
        this.retractCount += 1;
        return super.retract(userId, kind, scopeKey, supersededBy);
      }
    }

    const store = new CountingStore();
    await store.write(preference({ value: 'morning' }));
    store.listCount = 0;
    store.writeCount = 0;

    const result = await handleVisibleMemoryCommandAppAdapterRequest(store, {
      userId: 'u1',
      text: 'Remember this: I prefer afternoons',
      parsedPreference: {
        attribute: 'alert_timing',
        value: 'alice@example.com afternoons only',
        sourceRef: 'reply:private-disabled-app-adapter-ref'
      }
    });

    assert.deepEqual(result, {
      handled: false,
      status: 'disabled',
      user_id: 'u1',
      response: { text: null },
      command_result: null,
      audit_metadata: {
        memory_kind: 'preference',
        adapter: 'visible_memory_command_app_adapter',
        enabled: false,
        handler_status: 'disabled'
      }
    });
    assert.equal(Object.isFrozen(result), true);
    assert.equal(Object.isFrozen(result.response), true);
    assert.equal(Object.isFrozen(result.audit_metadata), true);
    assert.equal(store.listCount, 0);
    assert.equal(store.writeCount, 0);
    assert.equal(store.retractCount, 0);
    assert.equal(
      (await recallVisibleExplicitPreference(store, 'u1', { attribute: 'alert_timing' }))?.preference_summary,
      'alert timing: morning'
    );
    const json = JSON.stringify(result);
    assert.equal(json.includes('alice@example.com'), false);
    assert.equal(json.includes('afternoons only'), false);
    assert.equal(json.includes('private-disabled-app-adapter-ref'), false);
  });

  it('routes enabled app-level adapter requests through the disabled handler seam with explicit caller context only', async () => {
    const store = new InMemoryTypedMemoryStore();

    const remembered = await handleVisibleMemoryCommandAppAdapterRequest(store, {
      enabled: true,
      userId: 'u1',
      text: 'Remember this: I prefer mornings',
      parsedPreference: {
        attribute: 'alert_timing',
        value: 'alice@example.com mornings only',
        updatedAt: '2026-07-03T09:00:00.000Z',
        sourceRef: 'reply:private-app-adapter-remember-ref'
      }
    });
    assert.equal(remembered.handled, true);
    assert.equal(remembered.status, 'handled');
    assert.equal(remembered.command_result?.action, 'remember_visible_explicit_preference');
    assert.equal(remembered.response.text, remembered.command_result?.answer);
    assert.deepEqual(remembered.audit_metadata, {
      memory_kind: 'preference',
      adapter: 'visible_memory_command_app_adapter',
      enabled: true,
      handler_status: 'handled',
      matched_action: 'remember_visible_explicit_preference',
      matched_intent: 'memory_remember'
    });
    assert.equal(JSON.stringify(remembered).includes('alice@example.com'), false);
    assert.equal(JSON.stringify(remembered).includes('mornings only'), false);
    assert.equal(JSON.stringify(remembered).includes('private-app-adapter-remember-ref'), false);

    const review = await handleVisibleMemoryCommandAppAdapterRequest(store, {
      enabled: true,
      userId: 'u1',
      text: 'What do you remember about me?',
      query: { attribute: 'alert_timing' }
    });
    assert.equal(review.handled, true);
    assert.equal(review.command_result?.action, 'review_visible_explicit_preferences');
    assert.equal(review.response.text?.includes('[redacted]'), true);
    assert.deepEqual(review.audit_metadata, {
      memory_kind: 'preference',
      adapter: 'visible_memory_command_app_adapter',
      enabled: true,
      handler_status: 'handled',
      matched_action: 'review_visible_explicit_preferences',
      matched_intent: 'memory_review',
      returned_count: 1,
      row_ids: [1],
      scope_keys: ['preference:alert_timing']
    });

    const explanation = await handleVisibleMemoryCommandAppAdapterRequest(store, {
      enabled: true,
      userId: 'u1',
      text: 'Why did you use that?',
      query: { attribute: 'alert_timing' }
    });
    assert.equal(explanation.handled, true);
    assert.equal(explanation.command_result?.action, 'explain_visible_explicit_preference_use');

    const correction = await handleVisibleMemoryCommandAppAdapterRequest(store, {
      enabled: true,
      userId: 'u1',
      text: 'Correct that preference',
      query: { attribute: 'alert_timing' },
      correction: {
        correctedValue: 'evening',
        updatedAt: '2026-07-03T10:00:00.000Z',
        sourceRef: 'reply:private-app-adapter-correction-ref'
      }
    });
    assert.equal(correction.handled, true);
    assert.equal(correction.command_result?.action, 'correct_visible_explicit_preference');
    assert.equal(JSON.stringify(correction).includes('private-app-adapter-correction-ref'), false);
    assert.equal(JSON.stringify(correction).includes('evening'), false);

    const forget = await handleVisibleMemoryCommandAppAdapterRequest(store, {
      enabled: true,
      userId: 'u1',
      text: 'Forget that preference',
      query: { attribute: 'alert_timing' }
    });
    assert.equal(forget.handled, true);
    assert.equal(forget.command_result?.action, 'forget_visible_explicit_preference');
    assert.equal(await recallVisibleExplicitPreference(store, 'u1', { attribute: 'alert_timing' }), null);
  });

  it('keeps enabled app-level adapter unknown text and missing parsed context as no-ops', async () => {
    class CountingStore extends InMemoryTypedMemoryStore {
      writeCount = 0;
      retractCount = 0;

      override async write(input: NewTypedMemoryRow) {
        this.writeCount += 1;
        return super.write(input);
      }

      override async retract(
        userId: string,
        kind: Parameters<InMemoryTypedMemoryStore['retract']>[1],
        scopeKey: string,
        supersededBy: number | null = null
      ) {
        this.retractCount += 1;
        return super.retract(userId, kind, scopeKey, supersededBy);
      }
    }

    const store = new CountingStore();
    await store.write(preference({ value: 'morning' }));
    store.writeCount = 0;

    const unknown = await handleVisibleMemoryCommandAppAdapterRequest(store, {
      enabled: true,
      userId: 'u1',
      text: 'Can you help me with my inbox?',
      parsedPreference: { attribute: 'alert_timing', value: 'should not write' },
      query: { attribute: 'alert_timing' },
      correction: { correctedValue: 'should not correct' }
    });
    assert.equal(unknown.handled, false);
    assert.equal(unknown.status, 'no_memory_command');
    assert.deepEqual(unknown.audit_metadata, {
      memory_kind: 'preference',
      adapter: 'visible_memory_command_app_adapter',
      enabled: true,
      handler_status: 'no_memory_command'
    });

    assert.equal(
      await handleVisibleMemoryCommandAppAdapterRequest(store, {
        enabled: true,
        userId: 'u1',
        text: 'Remember this: I prefer afternoons'
      }).then((adapterResult) => adapterResult.command_result),
      null
    );
    assert.equal(
      await handleVisibleMemoryCommandAppAdapterRequest(store, {
        enabled: true,
        userId: 'u1',
        text: 'Forget that preference'
      }).then((adapterResult) => adapterResult.command_result),
      null
    );
    assert.equal(
      await handleVisibleMemoryCommandAppAdapterRequest(store, {
        enabled: true,
        userId: 'u1',
        text: 'Correct that preference',
        query: { attribute: 'alert_timing' }
      }).then((adapterResult) => adapterResult.command_result),
      null
    );

    assert.equal(store.writeCount, 0);
    assert.equal(store.retractCount, 0);
    assert.equal(
      (await recallVisibleExplicitPreference(store, 'u1', { attribute: 'alert_timing' }))?.preference_summary,
      'alert timing: morning'
    );
  });

  it('keeps the app-level adapter audit-event seam disabled by default without recording audit events', async () => {
    class CountingStore extends InMemoryTypedMemoryStore {
      listCount = 0;
      writeCount = 0;
      retractCount = 0;

      override async listActive(...args: Parameters<InMemoryTypedMemoryStore['listActive']>) {
        this.listCount += 1;
        return super.listActive(...args);
      }

      override async write(input: NewTypedMemoryRow) {
        this.writeCount += 1;
        return super.write(input);
      }

      override async retract(
        userId: string,
        kind: Parameters<InMemoryTypedMemoryStore['retract']>[1],
        scopeKey: string,
        supersededBy: number | null = null
      ) {
        this.retractCount += 1;
        return super.retract(userId, kind, scopeKey, supersededBy);
      }
    }

    const store = new CountingStore();
    const events: VisibleMemoryCommandAppAdapterAuditEvent[] = [];
    await store.write(preference({ value: 'morning' }));
    store.listCount = 0;
    store.writeCount = 0;

    const result = await handleVisibleMemoryCommandAppAdapterRequest(store, {
      userId: 'u1',
      text: 'Remember this: I prefer afternoons',
      parsedPreference: {
        attribute: 'alert_timing',
        value: 'alice@example.com afternoons only',
        sourceRef: 'reply:private-audit-disabled-ref'
      },
      audit: {
        record: (event) => events.push(event)
      }
    });

    assert.equal(result.status, 'disabled');
    assert.deepEqual(events, []);
    assert.equal(store.listCount, 0);
    assert.equal(store.writeCount, 0);
    assert.equal(store.retractCount, 0);
    assert.equal(
      (await recallVisibleExplicitPreference(store, 'u1', { attribute: 'alert_timing' }))?.preference_summary,
      'alert timing: morning'
    );
  });

  it('records enabled app-level adapter audit events with sanitized structural outcome metadata only', async () => {
    const store = new InMemoryTypedMemoryStore();
    const events: VisibleMemoryCommandAppAdapterAuditEvent[] = [];

    const remembered = await handleVisibleMemoryCommandAppAdapterRequest(store, {
      enabled: true,
      userId: 'u1',
      text: 'Remember this: I prefer mornings',
      parsedPreference: {
        attribute: 'alert_timing',
        value: 'alice@example.com mornings only',
        updatedAt: '2026-07-04T09:00:00.000Z',
        sourceRef: 'reply:private-audit-remember-ref'
      },
      audit: {
        enabled: true,
        record: (event) => events.push(event)
      }
    });

    assert.equal(remembered.handled, true);
    assert.equal(events.length, 1);
    assert.deepEqual(events[0], {
      action: 'visible_memory_command.app_adapter.outcome',
      actor_user_id: 'u1',
      target: 'visible_memory_command_app_adapter',
      result: 'success',
      detail: {
        memory_kind: 'preference',
        adapter: 'visible_memory_command_app_adapter',
        enabled: true,
        handled: true,
        status: 'handled',
        matched_action: 'remember_visible_explicit_preference',
        matched_intent: 'memory_remember'
      }
    });
    assert.equal(Object.isFrozen(events[0]), true);
    assert.equal(Object.isFrozen(events[0]?.detail), true);

    await store.write(
      preference({
        user_id: 'u2',
        value: 'other-user audit seam secret alice@example.com',
        source_ref: 'reply:other-user-audit-seam-secret',
        updated_at: '2026-07-04T10:00:00.000Z'
      })
    );

    await handleVisibleMemoryCommandAppAdapterRequest(store, {
      enabled: true,
      userId: 'u1',
      text: 'What do you remember about me?',
      query: { attribute: 'alert_timing' },
      audit: {
        enabled: true,
        record: (event) => events.push(event)
      }
    });

    assert.deepEqual(events[1], {
      action: 'visible_memory_command.app_adapter.outcome',
      actor_user_id: 'u1',
      target: 'visible_memory_command_app_adapter',
      result: 'success',
      detail: {
        memory_kind: 'preference',
        adapter: 'visible_memory_command_app_adapter',
        enabled: true,
        handled: true,
        status: 'handled',
        matched_action: 'review_visible_explicit_preferences',
        matched_intent: 'memory_review',
        returned_count: 1,
        row_ids: [1],
        scope_keys: ['preference:alert_timing']
      }
    });

    const json = JSON.stringify(events);
    assert.equal(json.includes('Remember this'), false);
    assert.equal(json.includes('What do you remember'), false);
    assert.equal(json.includes('alice@example.com'), false);
    assert.equal(json.includes('mornings only'), false);
    assert.equal(json.includes('private-audit-remember-ref'), false);
    assert.equal(json.includes('u2'), false);
    assert.equal(json.includes('other-user audit seam secret'), false);
    assert.equal(json.includes('other-user-audit-seam-secret'), false);
  });

  it('records safe no-op audit statuses for unknown text and missing parsed context only when explicitly enabled', async () => {
    class CountingStore extends InMemoryTypedMemoryStore {
      writeCount = 0;
      retractCount = 0;

      override async write(input: NewTypedMemoryRow) {
        this.writeCount += 1;
        return super.write(input);
      }

      override async retract(
        userId: string,
        kind: Parameters<InMemoryTypedMemoryStore['retract']>[1],
        scopeKey: string,
        supersededBy: number | null = null
      ) {
        this.retractCount += 1;
        return super.retract(userId, kind, scopeKey, supersededBy);
      }
    }

    const store = new CountingStore();
    const events: VisibleMemoryCommandAppAdapterAuditEvent[] = [];
    await store.write(preference({ value: 'morning' }));
    store.writeCount = 0;

    await handleVisibleMemoryCommandAppAdapterRequest(store, {
      enabled: true,
      userId: 'u1',
      text: 'Can you help me with my inbox?',
      parsedPreference: { attribute: 'alert_timing', value: 'should not write' },
      query: { attribute: 'alert_timing' },
      correction: { correctedValue: 'should not correct' },
      audit: {
        enabled: true,
        record: (event) => events.push(event)
      }
    });
    await handleVisibleMemoryCommandAppAdapterRequest(store, {
      enabled: true,
      userId: 'u1',
      text: 'Remember this: I prefer afternoons',
      audit: {
        enabled: true,
        record: (event) => events.push(event)
      }
    });

    assert.deepEqual(events.map((event) => event.detail.status), ['no_memory_command', 'no_memory_command']);
    assert.deepEqual(events.map((event) => event.result), ['noop', 'noop']);
    assert.deepEqual(events.map((event) => event.detail.handled), [false, false]);
    assert.equal(store.writeCount, 0);
    assert.equal(store.retractCount, 0);
    const json = JSON.stringify(events);
    assert.equal(json.includes('Can you help me'), false);
    assert.equal(json.includes('Remember this'), false);
    assert.equal(json.includes('should not write'), false);
    assert.equal(json.includes('should not correct'), false);
  });

  it('keeps the app-adapter audit-store recorder disabled by default without writing audit rows', async () => {
    const memoryStore = new InMemoryTypedMemoryStore();
    const auditStore = new InMemoryAuditStore();
    await memoryStore.write(preference({ value: 'morning' }));

    await handleVisibleMemoryCommandAppAdapterRequest(memoryStore, {
      enabled: true,
      userId: 'u1',
      text: 'What do you remember about me?',
      query: { attribute: 'alert_timing' },
      audit: {
        enabled: true,
        record: createVisibleMemoryCommandAppAdapterAuditStoreRecorder({ auditStore })
      }
    });

    assert.deepEqual(await auditStore.recent('u1'), []);
  });

  it('writes enabled app-adapter outcomes to the audit store with sanitized structural metadata only', async () => {
    const memoryStore = new InMemoryTypedMemoryStore();
    const auditStore = new InMemoryAuditStore();

    await handleVisibleMemoryCommandAppAdapterRequest(memoryStore, {
      enabled: true,
      userId: 'u1',
      text: 'Remember this: I prefer mornings',
      parsedPreference: {
        attribute: 'alert_timing',
        value: 'alice@example.com mornings only',
        updatedAt: '2026-07-04T09:00:00.000Z',
        sourceRef: 'reply:private-audit-store-remember-ref'
      },
      audit: {
        enabled: true,
        record: createVisibleMemoryCommandAppAdapterAuditStoreRecorder({
          enabled: true,
          auditStore,
          occurredAt: '2026-07-04T09:01:00.000Z'
        })
      }
    });

    await memoryStore.write(
      preference({
        user_id: 'u2',
        value: 'other-user audit-store seam secret alice@example.com',
        source_ref: 'reply:other-user-audit-store-seam-secret',
        updated_at: '2026-07-04T10:00:00.000Z'
      })
    );

    await handleVisibleMemoryCommandAppAdapterRequest(memoryStore, {
      enabled: true,
      userId: 'u1',
      text: 'What do you remember about me?',
      query: { attribute: 'alert_timing' },
      audit: {
        enabled: true,
        record: createVisibleMemoryCommandAppAdapterAuditStoreRecorder({
          enabled: true,
          auditStore,
          occurredAt: '2026-07-04T09:02:00.000Z'
        })
      }
    });

    assert.deepEqual(await auditStore.recent('u2'), []);
    const entries = await auditStore.recent('u1');
    assert.equal(entries.length, 2);
    assert.deepEqual(
      entries.map((entry) => ({
        occurred_at: entry.occurred_at,
        actor_user_id: entry.actor_user_id,
        actor_ip: entry.actor_ip,
        actor_user_agent: entry.actor_user_agent,
        action: entry.action,
        target: entry.target,
        result: entry.result,
        detail: entry.detail
      })),
      [
        {
          occurred_at: '2026-07-04T09:02:00.000Z',
          actor_user_id: 'u1',
          actor_ip: null,
          actor_user_agent: null,
          action: 'visible_memory_command.app_adapter.outcome',
          target: 'visible_memory_command_app_adapter',
          result: 'success',
          detail: {
            memory_kind: 'preference',
            adapter: 'visible_memory_command_app_adapter',
            enabled: true,
            handled: true,
            status: 'handled',
            matched_action: 'review_visible_explicit_preferences',
            matched_intent: 'memory_review',
            returned_count: 1,
            row_ids: [1],
            scope_keys: ['preference:alert_timing']
          }
        },
        {
          occurred_at: '2026-07-04T09:01:00.000Z',
          actor_user_id: 'u1',
          actor_ip: null,
          actor_user_agent: null,
          action: 'visible_memory_command.app_adapter.outcome',
          target: 'visible_memory_command_app_adapter',
          result: 'success',
          detail: {
            memory_kind: 'preference',
            adapter: 'visible_memory_command_app_adapter',
            enabled: true,
            handled: true,
            status: 'handled',
            matched_action: 'remember_visible_explicit_preference',
            matched_intent: 'memory_remember'
          }
        }
      ]
    );

    const json = JSON.stringify(entries);
    assert.equal(json.includes('Remember this'), false);
    assert.equal(json.includes('What do you remember'), false);
    assert.equal(json.includes('alice@example.com'), false);
    assert.equal(json.includes('mornings only'), false);
    assert.equal(json.includes('private-audit-store-remember-ref'), false);
    assert.equal(json.includes('u2'), false);
    assert.equal(json.includes('other-user audit-store seam secret'), false);
    assert.equal(json.includes('other-user-audit-store-seam-secret'), false);
  });

  it('writes safe no-op audit-store statuses for unknown text and missing parsed context only when explicitly enabled', async () => {
    class CountingStore extends InMemoryTypedMemoryStore {
      writeCount = 0;
      retractCount = 0;

      override async write(input: NewTypedMemoryRow) {
        this.writeCount += 1;
        return super.write(input);
      }

      override async retract(
        userId: string,
        kind: Parameters<InMemoryTypedMemoryStore['retract']>[1],
        scopeKey: string,
        supersededBy: number | null = null
      ) {
        this.retractCount += 1;
        return super.retract(userId, kind, scopeKey, supersededBy);
      }
    }

    const memoryStore = new CountingStore();
    const auditStore = new InMemoryAuditStore();
    await memoryStore.write(preference({ value: 'morning' }));
    memoryStore.writeCount = 0;

    await handleVisibleMemoryCommandAppAdapterRequest(memoryStore, {
      enabled: true,
      userId: 'u1',
      text: 'Can you help me with my inbox?',
      parsedPreference: { attribute: 'alert_timing', value: 'should not write' },
      query: { attribute: 'alert_timing' },
      correction: { correctedValue: 'should not correct' },
      audit: {
        enabled: true,
        record: createVisibleMemoryCommandAppAdapterAuditStoreRecorder({ enabled: true, auditStore })
      }
    });
    await handleVisibleMemoryCommandAppAdapterRequest(memoryStore, {
      enabled: true,
      userId: 'u1',
      text: 'Remember this: I prefer afternoons',
      audit: {
        enabled: true,
        record: createVisibleMemoryCommandAppAdapterAuditStoreRecorder({ enabled: true, auditStore })
      }
    });

    const entries = await auditStore.recent('u1');
    assert.deepEqual(
      entries.map((entry) => entry.detail),
      [
        {
          memory_kind: 'preference',
          adapter: 'visible_memory_command_app_adapter',
          enabled: true,
          handled: false,
          status: 'no_memory_command'
        },
        {
          memory_kind: 'preference',
          adapter: 'visible_memory_command_app_adapter',
          enabled: true,
          handled: false,
          status: 'no_memory_command'
        }
      ]
    );
    assert.equal(memoryStore.writeCount, 0);
    assert.equal(memoryStore.retractCount, 0);
    const json = JSON.stringify(entries);
    assert.equal(json.includes('Can you help me'), false);
    assert.equal(json.includes('Remember this'), false);
    assert.equal(json.includes('should not write'), false);
    assert.equal(json.includes('should not correct'), false);
  });

  it('keeps enabled app-level adapter user-scoped and excludes unsafe or cross-user memory results', async () => {
    const store = new InMemoryTypedMemoryStore();
    await store.write(preference({ value: 'morning', updated_at: '2026-06-25T10:00:00.000Z' }));
    await store.write(
      preference({
        user_id: 'u2',
        value: 'other-user app adapter secret alice@example.com',
        source_ref: 'reply:other-user-app-adapter-secret'
      })
    );
    await store.write(preference({ scope_key: 'preference:low', attribute: 'low', confidence: 'low' }));
    await store.write(
      preference({
        scope_key: 'preference:stale',
        attribute: 'stale',
        value: 'should not route through app adapter',
        stale_marked_at: '2026-06-25T00:00:00.000Z'
      })
    );
    await store.write(preference({ scope_key: 'preference:retracted', attribute: 'retracted', retracted: true }));
    await store.write(preference({ scope_key: 'preference:tombstoned', attribute: 'tombstoned', superseded_by: 99 }));

    const review = await handleVisibleMemoryCommandAppAdapterRequest(store, {
      enabled: true,
      userId: 'u1',
      text: 'List my saved preferences'
    });
    assert.equal(review.handled, true);
    assert.equal(review.command_result?.action, 'review_visible_explicit_preferences');
    const reviewJson = JSON.stringify(review);
    assert.equal(reviewJson.includes('u2'), false);
    assert.equal(reviewJson.includes('other-user app adapter secret'), false);
    assert.equal(reviewJson.includes('alice@example.com'), false);
    assert.equal(reviewJson.includes('other-user-app-adapter-secret'), false);
    assert.equal(reviewJson.includes('should not route through app adapter'), false);
    assert.equal(reviewJson.includes('stale'), false);
    assert.equal(reviewJson.includes('retracted'), false);
    assert.equal(reviewJson.includes('tombstoned'), false);

    assert.equal(
      await handleVisibleMemoryCommandAppAdapterRequest(store, {
        enabled: true,
        userId: 'u1',
        text: 'Why did you use it?',
        query: { attribute: 'stale' }
      }).then((adapterResult) => adapterResult.command_result),
      null
    );
    assert.equal(
      await handleVisibleMemoryCommandAppAdapterRequest(store, {
        enabled: true,
        userId: 'u1',
        text: 'Forget that preference',
        query: { attribute: 'retracted' }
      }).then((adapterResult) => adapterResult.command_result),
      null
    );
    assert.equal(
      await handleVisibleMemoryCommandAppAdapterRequest(store, {
        enabled: true,
        userId: 'u1',
        text: 'Correct that preference',
        query: { attribute: 'tombstoned' },
        correction: { correctedValue: 'evening', updatedAt: '2026-07-03T10:00:00.000Z' }
      }).then((adapterResult) => adapterResult.command_result),
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
