import assert from 'node:assert/strict';
import { readdir, readFile } from 'node:fs/promises';
import path from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import { InMemoryAuditStore, FOMO_AUDIT_ACTIONS } from '../core/audit.ts';

import {
  InMemoryTypedMemoryStore,
  MemorySignalsBackedTypedMemoryStore,
  TYPED_MEMORY_CONFIDENCE_LEVELS,
  TYPED_MEMORY_KINDS,
  TYPED_MEMORY_RETRIEVAL_PACK_KINDS,
  TYPED_MEMORY_SOURCES,
  isTypedMemoryConfidence,
  isTypedMemoryKind,
  isTypedMemoryRetrievalPackKind,
  isTypedMemorySource,
  typedMemoryScopeKeyForBridgedMemorySignal,
  type NewTypedMemoryRow
} from './typed-memory.ts';
import { InMemoryMemorySignalStore } from './memory-signals.ts';

const SRC_ROOT = path.resolve(fileURLToPath(new URL('..', import.meta.url)));

async function listSourceFiles(dir: string): Promise<readonly string[]> {
  const entries = await readdir(dir, { withFileTypes: true });
  const files: string[] = [];
  for (const entry of entries) {
    const fullPath = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      files.push(...(await listSourceFiles(fullPath)));
    } else if (entry.isFile() && fullPath.endsWith('.ts')) {
      files.push(fullPath);
    }
  }
  return files.sort();
}

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

describe('typed memory facade constants', () => {
  it('declares M1 no-migration typed memory kinds without an untyped catch-all', () => {
    assert.deepEqual([...TYPED_MEMORY_KINDS], [
      'semantic',
      'preference',
      'project',
      'contact',
      'repeated_behavior'
    ]);
    assert.equal(isTypedMemoryKind('semantic'), true);
    assert.equal(isTypedMemoryKind('json_dump'), false);
  });

  it('declares explicit confidence and source enums', () => {
    assert.deepEqual([...TYPED_MEMORY_CONFIDENCE_LEVELS], ['low', 'medium', 'high']);
    assert.deepEqual([...TYPED_MEMORY_RETRIEVAL_PACK_KINDS], [
      'ranker',
      'hmr',
      'explain',
      'drafter',
      'ops'
    ]);
    assert.deepEqual([...TYPED_MEMORY_SOURCES], [
      'user_provided',
      'user_stated',
      'founder_default',
      'feedback_derived',
      'consolidation_proposed',
      'ops_injected'
    ]);
    assert.equal(isTypedMemoryConfidence('medium'), true);
    assert.equal(isTypedMemoryConfidence('certain'), false);
    assert.equal(isTypedMemoryRetrievalPackKind('ranker'), true);
    assert.equal(isTypedMemoryRetrievalPackKind('m1_facade_test'), false);
    assert.equal(isTypedMemorySource('user_stated'), true);
    assert.equal(isTypedMemorySource('model_guessed'), false);
  });

  it('registers dormant memory audit actions in the runtime audit action list', () => {
    assert.ok(FOMO_AUDIT_ACTIONS.includes('brevio.memory.retrieved'));
    assert.ok(FOMO_AUDIT_ACTIONS.includes('brevio.memory.retraction_recorded'));
  });

  it('keeps the M1 facade dormant: no non-test production module imports typed-memory', async () => {
    const files = await listSourceFiles(SRC_ROOT);
    const importRegex = /import\s+[^;]+?from\s+['"]([^'"]+)['"]/g;

    for (const file of files) {
      if (file.endsWith('typed-memory.ts') || file.endsWith('.test.ts')) continue;

      const source = await readFile(file, 'utf8');
      let match: RegExpExecArray | null;
      while ((match = importRegex.exec(source)) !== null) {
        const specifier = match[1] ?? '';
        assert.equal(
          specifier.endsWith('/typed-memory.js') || specifier.endsWith('/typed-memory.ts'),
          false,
          `M1 no-migration facade must remain dormant; unexpected production import in ${path.relative(SRC_ROOT, file)} from ${specifier}`
        );
      }
    }
  });
});

describe('InMemoryTypedMemoryStore', () => {
  it('writes and reads a typed semantic row', async () => {
    const store = new InMemoryTypedMemoryStore();
    const written = await store.write(semantic());
    assert.equal(written.kind, 'semantic');
    assert.equal(written.user_id, 'u1');
    assert.equal(written.scope_key, 'profile:working_hours');
    assert.equal(written.confidence, 'high');
    assert.deepEqual(written.value, { tz: 'America/New_York', start: '09:00', end: '18:00' });

    const read = await store.get('u1', 'semantic', 'profile:working_hours');
    assert.equal(read?.id, written.id);
  });

  it('preserves legitimate typed-memory value keys instead of applying log redaction to stored rows', async () => {
    const store = new InMemoryTypedMemoryStore();
    const written = await store.write(
      semantic({
        scope_key: 'profile:state_code',
        attribute: 'state_code',
        value: { state: 'NY', code: 'EST', note: 'allowed typed memory payload' }
      })
    );

    assert.deepEqual(written.value, {
      state: 'NY',
      code: 'EST',
      note: 'allowed typed memory payload'
    });

    const read = await store.get('u1', 'semantic', 'profile:state_code');
    assert.deepEqual(read?.kind === 'semantic' ? read.value : null, written.value);
  });

  it('deep-clones and freezes typed-memory values so callers cannot mutate stored rows', async () => {
    const store = new InMemoryTypedMemoryStore();
    const inputValue = { prefs: { start: '09:00' }, tags: ['morning'] };
    const written = await store.write(
      semantic({
        scope_key: 'profile:nested_value',
        attribute: 'nested_value',
        value: inputValue
      })
    );

    inputValue.prefs.start = '12:00';
    inputValue.tags.push('mutated');
    assert.throws(() => {
      if (written.kind === 'semantic') {
        (written.value.prefs as { start: string }).start = '13:00';
      }
    }, /Cannot assign to read only property/);

    const read = await store.get('u1', 'semantic', 'profile:nested_value');
    assert.deepEqual(read?.kind === 'semantic' ? read.value : null, {
      prefs: { start: '09:00' },
      tags: ['morning']
    });
  });

  it('rejects invalid typed enum values at write time', async () => {
    const store = new InMemoryTypedMemoryStore();

    await assert.rejects(
      () => store.write(semantic({ kind: 'json_dump' as never })),
      /kind must be one of/
    );
    await assert.rejects(
      () => store.write(semantic({ source: 'model_guessed' as never })),
      /source must be one of/
    );
    await assert.rejects(
      () => store.write(semantic({ confidence: 'certain' as never })),
      /confidence must be one of/
    );
  });

  it('preserves cross-tenant isolation by user_id', async () => {
    const store = new InMemoryTypedMemoryStore();
    await store.write(semantic({ user_id: 'u1', scope_key: 'profile:timezone' }));
    await store.write(semantic({ user_id: 'u2', scope_key: 'profile:timezone' }));

    const u1Rows = await store.listActive('u1');
    const u2Rows = await store.listActive('u2');
    assert.equal(u1Rows.length, 1);
    assert.equal(u2Rows.length, 1);
    assert.equal(u1Rows[0]?.user_id, 'u1');
    assert.equal(u2Rows[0]?.user_id, 'u2');
  });

  it('excludes low-confidence, stale, and retracted rows from active retrieval', async () => {
    const store = new InMemoryTypedMemoryStore();
    await store.write(semantic({ scope_key: 'profile:confirmed', confidence: 'high' }));
    await store.write(semantic({ scope_key: 'profile:weak', confidence: 'low' }));
    await store.write(
      semantic({
        scope_key: 'profile:stale',
        confidence: 'high',
        stale_marked_at: '2026-06-01T00:00:00.000Z'
      })
    );
    await store.write(semantic({ scope_key: 'profile:retracted', confidence: 'high', retracted: true }));

    const rows = await store.listActive('u1');
    assert.deepEqual(rows.map((r) => r.scope_key), ['profile:confirmed']);
    assert.equal(await store.get('u1', 'semantic', 'profile:weak'), null);
    assert.equal(await store.get('u1', 'semantic', 'profile:stale'), null);
    assert.equal(await store.get('u1', 'semantic', 'profile:retracted'), null);
  });

  it('records retrieval audit without persisting memory content in audit detail', async () => {
    const audit = new InMemoryAuditStore();
    const store = new InMemoryTypedMemoryStore(audit);
    const row = await store.write(semantic());
    await store.markRetrieved({
      user_id: 'u1',
      pack_kind: 'ranker',
      kinds: ['semantic'],
      returned_ids: [row.id ?? 0],
      suppressions_applied: 0,
      preferences_applied: 0
    });

    const [entry] = await audit.recent('u1');
    assert.equal(entry?.action, 'brevio.memory.retrieved');
    assert.deepEqual(entry?.detail, {
      pack_kind: 'ranker',
      row_kinds: ['semantic'],
      row_ids: [row.id],
      suppressions_applied: 0,
      preferences_applied: 0
    });
    assert.equal(JSON.stringify(entry?.detail).includes('working_hours'), false);
    assert.equal(JSON.stringify(entry?.detail).includes('America/New_York'), false);
  });

  it('rejects non-structural retrieval pack kinds before they can enter audit detail', async () => {
    const audit = new InMemoryAuditStore();
    const store = new InMemoryTypedMemoryStore(audit);
    const row = await store.write(semantic());

    await assert.rejects(
      () =>
        store.markRetrieved({
          user_id: 'u1',
          pack_kind: 'memory-212-555-1212' as never,
          kinds: ['semantic'],
          returned_ids: [row.id ?? 0]
        }),
      /pack_kind must be one of/
    );

    assert.deepEqual(await audit.recent('u1'), []);
  });

  it('rejects non-typed retrieval row kinds before they can enter audit detail', async () => {
    const audit = new InMemoryAuditStore();
    const store = new InMemoryTypedMemoryStore(audit);
    const row = await store.write(semantic());

    await assert.rejects(
      () =>
        store.markRetrieved({
          user_id: 'u1',
          pack_kind: 'ranker',
          kinds: ['semantic', 'json_dump' as never],
          returned_ids: [row.id ?? 0]
        }),
      /kind must be one of/
    );

    assert.deepEqual(await audit.recent('u1'), []);
  });

  it('retracts rows and emits structural retraction audit', async () => {
    const audit = new InMemoryAuditStore();
    const store = new InMemoryTypedMemoryStore(audit);
    await store.write(semantic({ scope_key: 'profile:role' }));

    assert.equal(await store.retract('u1', 'semantic', 'profile:role', 42), true);
    assert.equal(await store.get('u1', 'semantic', 'profile:role'), null);

    const [entry] = await audit.recent('u1');
    assert.equal(entry?.action, 'brevio.memory.retraction_recorded');
    assert.equal(entry?.target, 'typed_memory');
    assert.deepEqual(entry?.detail, {
      kind: 'semantic',
      retracted_id: 1,
      superseded_by: 42
    });
    assert.equal(JSON.stringify(entry?.detail).includes('profile:role'), false);
  });

  it('rejects raw email-like scope keys as a privacy canary', async () => {
    const store = new InMemoryTypedMemoryStore();
    await assert.rejects(
      () => store.write(semantic({ scope_key: 'person:alice@example.com' })),
      /must not contain raw email/
    );
  });
});

describe('MemorySignalsBackedTypedMemoryStore', () => {
  it('bridges existing user-wide preference signals into typed-memory preference rows', async () => {
    const memoryStore = new InMemoryMemorySignalStore();
    await memoryStore.upsert({ user_id: 'u1', kind: 'quietness_preference', scope_key: null, detail: { max_per_day: 5 }, source: 'user_confirmed', updated_at: '2026-06-23T12:00:00.000Z' });
    await memoryStore.upsert({ user_id: 'u1', kind: 'timing_preference', scope_key: null, detail: { window: 'evening' }, source: 'founder_set', updated_at: '2026-06-23T13:00:00.000Z' });
    await memoryStore.upsert({ user_id: 'u1', kind: 'stop_active', scope_key: null, detail: { active: true }, source: 'user_confirmed', updated_at: '2026-06-23T14:00:00.000Z' });
    const store = new MemorySignalsBackedTypedMemoryStore(memoryStore);
    const rows = await store.listActive('u1');
    assert.deepEqual(rows.map((row) => ({ kind: row.kind, scope_key: row.scope_key, source: row.source, confidence: row.confidence, attribute: row.kind === 'preference' ? row.attribute : null })), [
      { kind: 'preference', scope_key: typedMemoryScopeKeyForBridgedMemorySignal('timing_preference'), source: 'founder_default', confidence: 'high', attribute: 'timing_preference' },
      { kind: 'preference', scope_key: typedMemoryScopeKeyForBridgedMemorySignal('quietness_preference'), source: 'user_stated', confidence: 'high', attribute: 'quietness_preference' }
    ]);
    assert.equal(rows.some((row) => row.scope_key.includes('stop_active')), false);
  });

  it('reads a bridged preference by its fixed safe typed scope key', async () => {
    const memoryStore = new InMemoryMemorySignalStore();
    await memoryStore.upsert({ user_id: 'u1', kind: 'quietness_preference', scope_key: null, detail: { max_per_day: 3 }, source: 'founder_set', updated_at: '2026-06-23T15:00:00.000Z' });
    const store = new MemorySignalsBackedTypedMemoryStore(memoryStore);
    const row = await store.get('u1', 'preference', typedMemoryScopeKeyForBridgedMemorySignal('quietness_preference'));
    assert.equal(row?.kind, 'preference');
    assert.equal(row?.scope_key, 'signal:quietness_preference');
    assert.equal(row?.source, 'founder_default');
    assert.equal(row?.source_ref, 'memory_signal:quietness_preference:1');
    assert.equal(row?.kind === 'preference' ? row.attribute : null, 'quietness_preference');
    assert.deepEqual(row?.kind === 'preference' ? row.value : null, { max_per_day: 3 });
  });

  it('excludes bridged rows when the underlying memory signal confidence maps to low', async () => {
    const memoryStore = new InMemoryMemorySignalStore();
    await memoryStore.upsert({ user_id: 'u1', kind: 'timing_preference', scope_key: null, detail: { window: 'night' }, source: 'user_confirmed', confidence: 0.55 });
    const store = new MemorySignalsBackedTypedMemoryStore(memoryStore);
    assert.deepEqual(await store.listActive('u1'), []);
    assert.equal(await store.get('u1', 'preference', typedMemoryScopeKeyForBridgedMemorySignal('timing_preference')), null);
  });

  it('excludes behavior-derived preference signals so the bridge cannot normalize a non-user-approved preference', async () => {
    const memoryStore = new InMemoryMemorySignalStore();
    await memoryStore.upsert({ user_id: 'u1', kind: 'quietness_preference', scope_key: null, detail: { max_per_day: 2 }, source: 'feedback_derived', updated_at: '2026-06-23T12:00:00.000Z' });
    await memoryStore.upsert({ user_id: 'u1', kind: 'timing_preference', scope_key: null, detail: { window: 'night' }, source: 'inferred', updated_at: '2026-06-23T13:00:00.000Z', confidence: 0.9 });
    const store = new MemorySignalsBackedTypedMemoryStore(memoryStore);

    assert.deepEqual(await store.listActive('u1'), []);
    assert.equal(
      await store.get(
        'u1',
        'preference',
        typedMemoryScopeKeyForBridgedMemorySignal('quietness_preference')
      ),
      null
    );
    assert.equal(
      await store.get(
        'u1',
        'preference',
        typedMemoryScopeKeyForBridgedMemorySignal('timing_preference')
      ),
      null
    );
  });

  it('preserves cross-tenant isolation while bridging', async () => {
    const memoryStore = new InMemoryMemorySignalStore();
    await memoryStore.upsert({ user_id: 'u1', kind: 'quietness_preference', scope_key: null, detail: { max_per_day: 5 }, source: 'user_confirmed' });
    await memoryStore.upsert({ user_id: 'u2', kind: 'quietness_preference', scope_key: null, detail: { max_per_day: 2 }, source: 'user_confirmed' });
    const store = new MemorySignalsBackedTypedMemoryStore(memoryStore);
    const u1Rows = await store.listActive('u1');
    const u2Rows = await store.listActive('u2');
    assert.equal(u1Rows.length, 1);
    assert.equal(u2Rows.length, 1);
    assert.deepEqual(u1Rows[0]?.kind === 'preference' ? u1Rows[0].value : null, { max_per_day: 5 });
    assert.deepEqual(u2Rows[0]?.kind === 'preference' ? u2Rows[0].value : null, { max_per_day: 2 });
  });

  it('emits structural retrieval audit without leaking bridged preference values', async () => {
    const audit = new InMemoryAuditStore();
    const memoryStore = new InMemoryMemorySignalStore();
    await memoryStore.upsert({ user_id: 'u1', kind: 'quietness_preference', scope_key: null, detail: { max_per_day: 4 }, source: 'user_confirmed' });
    const store = new MemorySignalsBackedTypedMemoryStore(memoryStore, audit);
    const row = await store.get('u1', 'preference', typedMemoryScopeKeyForBridgedMemorySignal('quietness_preference'));
    await store.markRetrieved({ user_id: 'u1', pack_kind: 'ops', kinds: ['preference'], returned_ids: row?.id ? [row.id] : [], preferences_applied: 1 });
    const [entry] = await audit.recent('u1');
    assert.equal(entry?.action, 'brevio.memory.retrieved');
    assert.deepEqual(entry?.detail, { pack_kind: 'ops', row_kinds: ['preference'], row_ids: [1], suppressions_applied: 0, preferences_applied: 1 });
    assert.equal(JSON.stringify(entry?.detail).includes('max_per_day'), false);
  });

  it('rejects write and retract because the bridge is intentionally read-only', async () => {
    const store = new MemorySignalsBackedTypedMemoryStore(new InMemoryMemorySignalStore());
    await assert.rejects(() => store.write({ user_id: 'u1', kind: 'preference', scope_key: 'signal:quietness_preference', source: 'user_stated', source_ref: 'reply:123', confidence: 'high', stale_marked_at: null, retracted: false, superseded_by: null, attribute: 'quietness_preference', value: { max_per_day: 5 } }), /read-only/);
    await assert.rejects(() => store.retract('u1', 'preference', 'signal:quietness_preference'), /read-only/);
  });
});
