import assert from 'node:assert/strict';
import { readdir, readFile } from 'node:fs/promises';
import path from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import { InMemoryAuditStore, FOMO_AUDIT_ACTIONS } from '../core/audit.ts';

import {
  buildTypedMemoryRetrievalEvidence,
  buildTypedMemoryContextPack,
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
  queryTypedMemoryRows,
  readTypedMemory,
  typedMemoryScopeKeyForBridgedCorrectionSignal,
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
      'correction',
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

  it('keeps the M1 facade dormant except the approved visible recall helper', async () => {
    const files = await listSourceFiles(SRC_ROOT);
    const importRegex = /import\s+[^;]+?from\s+['"]([^'"]+)['"]/g;
    const allowedTypedMemoryImporters = new Set(['memory/typed-memory-visible-recall.ts']);

    for (const file of files) {
      if (file.endsWith('typed-memory.ts') || file.endsWith('.test.ts')) continue;
      const relativeFile = path.relative(SRC_ROOT, file);

      const source = await readFile(file, 'utf8');
      let match: RegExpExecArray | null;
      while ((match = importRegex.exec(source)) !== null) {
        const specifier = match[1] ?? '';
        const importsTypedMemory =
          specifier.endsWith('/typed-memory.js') || specifier.endsWith('/typed-memory.ts');
        assert.equal(
          importsTypedMemory && !allowedTypedMemoryImporters.has(relativeFile),
          false,
          `M1 no-migration facade must remain dormant outside approved helpers; unexpected production import in ${relativeFile} from ${specifier}`
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

  it('rejects invalid read/list/retract inputs before lookup', async () => {
    const store = new InMemoryTypedMemoryStore();
    await store.write(semantic());

    await assert.rejects(
      () => store.get('u1', 'json_dump' as never, 'profile:working_hours'),
      /kind must be one of/
    );
    await assert.rejects(
      () => store.get('u1', 'semantic', 'person:alice@example.com'),
      /must not contain raw email/
    );
    await assert.rejects(() => store.listActive('u1', ['json_dump' as never]), /kind must be one of/);
    await assert.rejects(
      () => store.retract('u1', 'semantic', 'person:alice@example.com'),
      /must not contain raw email/
    );
  });
});

describe('typed memory query helpers', () => {
  it('filters typed rows by kind, scope, source, confidence, and limit with stable ordering', async () => {
    const store = new InMemoryTypedMemoryStore();
    await store.write(
      semantic({
        scope_key: 'profile:alpha',
        source: 'founder_default',
        confidence: 'medium',
        updated_at: '2026-06-23T12:00:00.000Z'
      })
    );
    await store.write(
      semantic({
        scope_key: 'profile:beta',
        source: 'user_stated',
        confidence: 'high',
        updated_at: '2026-06-23T12:00:00.000Z'
      })
    );
    await store.write(
      semantic({
        scope_key: 'profile:gamma',
        source: 'user_stated',
        confidence: 'high',
        updated_at: '2026-06-23T13:00:00.000Z'
      })
    );

    const rows = await readTypedMemory(store, 'u1', {
      kinds: ['semantic'],
      sources: ['user_stated'],
      minConfidence: 'high',
      limit: 2
    });

    assert.deepEqual(rows.map((row) => row.scope_key), ['profile:gamma', 'profile:beta']);
    assert.deepEqual(
      queryTypedMemoryRows(rows, { scopeKeys: ['profile:beta'] }).map((row) => row.scope_key),
      ['profile:beta']
    );
  });

  it('rejects invalid helper limits before producing a retrieval pack', async () => {
    const store = new InMemoryTypedMemoryStore();
    await store.write(semantic());
    await assert.rejects(() => readTypedMemory(store, 'u1', { limit: -1 }), /non-negative integer/);
  });

  it('builds structural retrieval evidence for included and excluded rows without leaking content', async () => {
    const store = new InMemoryTypedMemoryStore();
    const rows = [
      await store.write(
        semantic({
          scope_key: 'profile:included_newer',
          updated_at: '2026-06-23T13:00:00.000Z',
          value: { private_note: 'do not leak included value' }
        })
      ),
      await store.write(
        semantic({
          scope_key: 'profile:limit_excluded',
          updated_at: '2026-06-23T12:00:00.000Z',
          value: { private_note: 'do not leak limit value' }
        })
      ),
      await store.write(
        semantic({
          scope_key: 'profile:low_confidence',
          confidence: 'low',
          updated_at: '2026-06-23T11:00:00.000Z',
          value: { private_note: 'do not leak low value' }
        })
      ),
      await store.write(
        semantic({
          scope_key: 'profile:stale',
          stale_marked_at: '2026-06-23T00:00:00.000Z',
          updated_at: '2026-06-23T10:00:00.000Z',
          value: { private_note: 'do not leak stale value' }
        })
      ),
      await store.write(
        semantic({
          scope_key: 'profile:retracted',
          retracted: true,
          updated_at: '2026-06-23T09:00:00.000Z',
          value: { private_note: 'do not leak retracted value' }
        })
      ),
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
        value: 'private preference value',
        updated_at: '2026-06-23T14:00:00.000Z'
      } as NewTypedMemoryRow)
    ];

    const evidence = buildTypedMemoryRetrievalEvidence(rows, {
      kinds: ['semantic'],
      minConfidence: 'medium',
      limit: 1
    });

    assert.equal(Object.isFrozen(evidence), true);
    assert.deepEqual(evidence.returned_ids, [1]);
    assert.deepEqual(evidence.returned_kinds, ['semantic']);
    assert.equal(evidence.excluded_count, 5);
    assert.deepEqual(
      evidence.considered.map((row) => ({ id: row.id, kind: row.kind, decision: row.decision, reasons: row.reasons })),
      [
        { id: 6, kind: 'preference', decision: 'excluded', reasons: ['kind_not_requested'] },
        { id: 1, kind: 'semantic', decision: 'included', reasons: ['included'] },
        { id: 2, kind: 'semantic', decision: 'excluded', reasons: ['limit_exceeded'] },
        { id: 3, kind: 'semantic', decision: 'excluded', reasons: ['low_confidence', 'below_min_confidence'] },
        { id: 4, kind: 'semantic', decision: 'excluded', reasons: ['inactive_stale'] },
        { id: 5, kind: 'semantic', decision: 'excluded', reasons: ['inactive_retracted'] }
      ]
    );
    const evidenceJson = JSON.stringify(evidence);
    assert.equal(evidenceJson.includes('scope_key'), false);
    assert.equal(evidenceJson.includes('private'), false);
    assert.equal(evidenceJson.includes('included_newer'), false);
    assert.equal(evidenceJson.includes('alert_timing'), false);
  });

  it('keeps retrieval evidence scoped to rows already isolated for one user', async () => {
    const store = new InMemoryTypedMemoryStore();
    await store.write(
      semantic({
        user_id: 'u1',
        scope_key: 'profile:u1_private_context',
        value: { private_note: 'u1 private value' }
      })
    );
    await store.write(
      semantic({
        user_id: 'u2',
        scope_key: 'profile:u2_private_context',
        value: { private_note: 'u2 private value' }
      })
    );

    const rows = await readTypedMemory(store, 'u1', { kinds: ['semantic'] });
    const evidence = buildTypedMemoryRetrievalEvidence(rows, { kinds: ['semantic'] });

    assert.deepEqual(evidence.returned_ids, [1]);
    assert.deepEqual(
      evidence.considered.map((row) => ({ id: row.id, kind: row.kind, decision: row.decision, reasons: row.reasons })),
      [{ id: 1, kind: 'semantic', decision: 'included', reasons: ['included'] }]
    );
    const evidenceJson = JSON.stringify(evidence);
    assert.equal(evidenceJson.includes('u2'), false);
    assert.equal(evidenceJson.includes('private'), false);
    assert.equal(evidenceJson.includes('profile:'), false);
  });

  it('keeps M1 no-migration scope: no typed-memory table or migration exists', async () => {
    const migrationDir = path.join(SRC_ROOT, 'db', 'migrations');
    const migrationFiles = await readdir(migrationDir);
    assert.equal(migrationFiles.some((name) => /typed[_-]?memory/i.test(name)), false);

    for (const file of migrationFiles.filter((name) => name.endsWith('.sql'))) {
      const sql = await readFile(path.join(migrationDir, file), 'utf8');
      assert.equal(/create\s+table\s+[^;]*typed[_-]?memory/i.test(sql), false, file);
    }

    const schemaSource = await readFile(path.join(SRC_ROOT, 'db', 'schema.ts'), 'utf8');
    assert.equal(/typed[_-]?memory/i.test(schemaSource), false);

    const storeFactorySource = await readFile(path.join(SRC_ROOT, 'db', 'store-factory.ts'), 'utf8');
    assert.equal(/typed-memory/.test(storeFactorySource), false);
    assert.equal(/typedMemory/.test(storeFactorySource), false);
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

  it('preserves null and unknown metadata while bridging user-approved preferences', async () => {
    const memoryStore = new InMemoryMemorySignalStore();
    await memoryStore.upsert({
      user_id: 'u1',
      kind: 'timing_preference',
      scope_key: null,
      detail: { window: null, unknown_nested: { value: null }, unrecognized: ['kept'] },
      source: 'user_confirmed',
      updated_at: '2026-06-23T15:30:00.000Z'
    });
    const store = new MemorySignalsBackedTypedMemoryStore(memoryStore);
    const row = await store.get(
      'u1',
      'preference',
      typedMemoryScopeKeyForBridgedMemorySignal('timing_preference')
    );
    assert.deepEqual(row?.kind === 'preference' ? row.value : null, {
      window: null,
      unknown_nested: { value: null },
      unrecognized: ['kept']
    });
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

  it('excludes deleted and tombstoned memory_signals from typed reads', async () => {
    const memoryStore = new InMemoryMemorySignalStore();
    await memoryStore.upsert({
      user_id: 'u1',
      kind: 'quietness_preference',
      scope_key: null,
      detail: { max_per_day: 2, deleted: true },
      source: 'user_confirmed',
      updated_at: '2026-06-23T12:00:00.000Z'
    });
    await memoryStore.upsert({
      user_id: 'u1',
      kind: 'timing_preference',
      scope_key: null,
      detail: { window: 'morning', tombstoned_at: '2026-06-23T12:30:00.000Z' },
      source: 'founder_set',
      updated_at: '2026-06-23T13:00:00.000Z'
    });
    const store = new MemorySignalsBackedTypedMemoryStore(memoryStore);

    assert.deepEqual(await readTypedMemory(store, 'u1', { kinds: ['preference'] }), []);
    assert.equal(
      await store.get(
        'u1',
        'preference',
        typedMemoryScopeKeyForBridgedMemorySignal('timing_preference')
      ),
      null
    );
  });

  it('does not surface deleted memory_signals after the existing store delete path removes them', async () => {
    const memoryStore = new InMemoryMemorySignalStore();
    await memoryStore.upsert({
      user_id: 'u1',
      kind: 'quietness_preference',
      scope_key: null,
      detail: { max_per_day: 5 },
      source: 'user_confirmed'
    });
    assert.equal(await memoryStore.delete('u1', 'quietness_preference', null), true);
    const store = new MemorySignalsBackedTypedMemoryStore(memoryStore);
    assert.deepEqual(await store.listActive('u1'), []);
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

  it('bridges existing sender correction signals into typed correction rows without raw sender data', async () => {
    const memoryStore = new InMemoryMemorySignalStore();
    await memoryStore.upsert({
      user_id: 'u1',
      kind: 'sender_suppressed',
      scope_key: 'abc123hmac',
      detail: {
        suppressed: true,
        set_by: 'explicit_ignore_sender',
        source_feedback_event_ids: [10]
      },
      source: 'user_confirmed',
      updated_at: '2026-06-23T12:00:00.000Z'
    });
    await memoryStore.upsert({
      user_id: 'u1',
      kind: 'sender_feedback_ignored',
      scope_key: 'def456hmac',
      detail: {
        ignored_count: 2,
        first_ignored_at: '2026-06-22T12:00:00.000Z',
        last_ignored_at: '2026-06-23T13:00:00.000Z',
        unknown_metadata: null
      },
      source: 'feedback_derived',
      updated_at: '2026-06-23T13:00:00.000Z'
    });
    const store = new MemorySignalsBackedTypedMemoryStore(memoryStore);

    const rows = await readTypedMemory(store, 'u1', { kinds: ['correction'] });
    assert.deepEqual(
      rows.map((row) => ({
        kind: row.kind,
        scope_key: row.scope_key,
        source: row.source,
        rule: row.kind === 'correction' ? row.rule : null,
        target_hmac: row.kind === 'correction' ? row.target_hmac : null
      })),
      [
        {
          kind: 'correction',
          scope_key: typedMemoryScopeKeyForBridgedCorrectionSignal('sender_feedback_ignored', 'def456hmac'),
          source: 'feedback_derived',
          rule: 'sender_feedback_ignored',
          target_hmac: 'def456hmac'
        },
        {
          kind: 'correction',
          scope_key: typedMemoryScopeKeyForBridgedCorrectionSignal('sender_suppressed', 'abc123hmac'),
          source: 'user_stated',
          rule: 'sender_suppressed',
          target_hmac: 'abc123hmac'
        }
      ]
    );
    assert.equal(JSON.stringify(rows).includes('@'), false);
    assert.deepEqual(rows[0]?.kind === 'correction' ? rows[0].value.unknown_metadata : 'missing', null);
  });

  it('reads a bridged correction by its fixed safe typed scope key', async () => {
    const memoryStore = new InMemoryMemorySignalStore();
    await memoryStore.upsert({
      user_id: 'u1',
      kind: 'sender_suppressed',
      scope_key: 'abc123hmac',
      detail: { suppressed: true },
      source: 'user_confirmed',
      updated_at: '2026-06-23T12:00:00.000Z'
    });
    const store = new MemorySignalsBackedTypedMemoryStore(memoryStore);
    const row = await store.get(
      'u1',
      'correction',
      typedMemoryScopeKeyForBridgedCorrectionSignal('sender_suppressed', 'abc123hmac')
    );

    assert.equal(row?.kind, 'correction');
    assert.equal(row?.scope_key, 'signal:sender_suppressed:abc123hmac');
    assert.equal(row?.source_ref, 'memory_signal:sender_suppressed:1');
    assert.deepEqual(row?.kind === 'correction' ? row.value : null, { suppressed: true });
  });

  it('preserves cross-tenant isolation for bridged corrections', async () => {
    const memoryStore = new InMemoryMemorySignalStore();
    await memoryStore.upsert({ user_id: 'u1', kind: 'sender_suppressed', scope_key: 'sharedhash', detail: { suppressed: true }, source: 'user_confirmed' });
    await memoryStore.upsert({ user_id: 'u2', kind: 'sender_suppressed', scope_key: 'sharedhash', detail: { suppressed: false }, source: 'user_confirmed' });
    const store = new MemorySignalsBackedTypedMemoryStore(memoryStore);

    const u1Rows = await store.listActive('u1', ['correction']);
    const u2Rows = await store.listActive('u2', ['correction']);
    assert.deepEqual(u1Rows[0]?.kind === 'correction' ? u1Rows[0].value : null, { suppressed: true });
    assert.deepEqual(u2Rows[0]?.kind === 'correction' ? u2Rows[0].value : null, { suppressed: false });
  });

  it('excludes deleted, tombstoned, low-confidence, and raw-email-looking correction signals', async () => {
    const memoryStore = new InMemoryMemorySignalStore();
    await memoryStore.upsert({ user_id: 'u1', kind: 'sender_suppressed', scope_key: 'deletedhash', detail: { deleted: true }, source: 'user_confirmed' });
    await memoryStore.upsert({ user_id: 'u1', kind: 'sender_feedback_ignored', scope_key: 'tombstonedhash', detail: { tombstoned_at: '2026-06-23T12:00:00.000Z' }, source: 'feedback_derived', confidence: 1 });
    await memoryStore.upsert({ user_id: 'u1', kind: 'sender_feedback_ignored', scope_key: 'weakhash', detail: { ignored_count: 1 }, source: 'feedback_derived', confidence: 0.55 });
    await memoryStore.upsert({ user_id: 'u1', kind: 'sender_suppressed', scope_key: 'alice@example.com', detail: { suppressed: true }, source: 'user_confirmed' });
    const store = new MemorySignalsBackedTypedMemoryStore(memoryStore);

    assert.deepEqual(await readTypedMemory(store, 'u1', { kinds: ['correction'] }), []);
  });

  it('rejects write and retract because the bridge is intentionally read-only', async () => {
    const store = new MemorySignalsBackedTypedMemoryStore(new InMemoryMemorySignalStore());
    await assert.rejects(() => store.write({ user_id: 'u1', kind: 'preference', scope_key: 'signal:quietness_preference', source: 'user_stated', source_ref: 'reply:123', confidence: 'high', stale_marked_at: null, retracted: false, superseded_by: null, attribute: 'quietness_preference', value: { max_per_day: 5 } }), /read-only/);
    await assert.rejects(() => store.retract('u1', 'preference', 'signal:quietness_preference'), /read-only/);
  });

  it('rejects malformed bridge queries before reading memory_signals', async () => {
    const store = new MemorySignalsBackedTypedMemoryStore(new InMemoryMemorySignalStore());

    await assert.rejects(() => store.listActive('u1', ['json_dump' as never]), /kind must be one of/);
    await assert.rejects(
      () => store.get('u1', 'preference', 'person:alice@example.com'),
      /must not contain raw email/
    );
    assert.throws(
      () => typedMemoryScopeKeyForBridgedCorrectionSignal('sender_suppressed', 'alice@example.com'),
      /must not contain raw email/
    );
  });

  it('builds a frozen structural context pack over bridged memory_signals and audits without content', async () => {
    const audit = new InMemoryAuditStore();
    const memoryStore = new InMemoryMemorySignalStore();
    await memoryStore.upsert({
      user_id: 'u1',
      kind: 'quietness_preference',
      scope_key: null,
      detail: { max_per_day: 4, unknown_metadata: null },
      source: 'user_confirmed',
      updated_at: '2026-06-23T12:00:00.000Z'
    });
    await memoryStore.upsert({
      user_id: 'u1',
      kind: 'sender_suppressed',
      scope_key: 'abc123hmac',
      detail: { suppressed: true, hidden_reason: 'do not leak this content' },
      source: 'user_confirmed',
      updated_at: '2026-06-23T13:00:00.000Z'
    });
    await memoryStore.upsert({
      user_id: 'u2',
      kind: 'sender_suppressed',
      scope_key: 'abc123hmac',
      detail: { suppressed: false },
      source: 'user_confirmed',
      updated_at: '2026-06-23T14:00:00.000Z'
    });

    const store = new MemorySignalsBackedTypedMemoryStore(memoryStore, audit);
    const pack = await buildTypedMemoryContextPack(store, 'u1', 'ranker', {
      kinds: ['preference', 'correction']
    });

    assert.equal(Object.isFrozen(pack), true);
    assert.equal(Object.isFrozen(pack.rows), true);
    assert.equal(Object.isFrozen(pack.row_ids), true);
    assert.equal(Object.isFrozen(pack.row_kinds), true);
    assert.deepEqual(pack.rows.map((row) => row.user_id), ['u1', 'u1']);
    assert.deepEqual(pack.rows.map((row) => row.scope_key), [
      typedMemoryScopeKeyForBridgedCorrectionSignal('sender_suppressed', 'abc123hmac'),
      typedMemoryScopeKeyForBridgedMemorySignal('quietness_preference')
    ]);
    assert.deepEqual(pack.row_ids, [2, 1]);
    assert.deepEqual(pack.row_kinds, ['correction', 'preference']);
    assert.equal(pack.suppressions_applied, 1);
    assert.equal(pack.preferences_applied, 1);
    assert.deepEqual(pack.rows[1]?.kind === 'preference' ? pack.rows[1].value : null, {
      max_per_day: 4,
      unknown_metadata: null
    });

    assert.throws(() => {
      (pack.row_ids as number[]).push(999);
    }, /Cannot add property/);

    const [entry] = await audit.recent('u1');
    assert.equal(entry?.action, 'brevio.memory.retrieved');
    assert.deepEqual(entry?.detail, {
      pack_kind: 'ranker',
      row_kinds: ['correction', 'preference'],
      row_ids: [2, 1],
      suppressions_applied: 1,
      preferences_applied: 1
    });
    const auditJson = JSON.stringify(entry?.detail);
    assert.equal(auditJson.includes('max_per_day'), false);
    assert.equal(auditJson.includes('do not leak this content'), false);
  });

  it('excludes deleted and tombstoned bridged signals before building a context pack', async () => {
    const audit = new InMemoryAuditStore();
    const memoryStore = new InMemoryMemorySignalStore();
    await memoryStore.upsert({
      user_id: 'u1',
      kind: 'quietness_preference',
      scope_key: null,
      detail: { max_per_day: 3, deleted: true },
      source: 'user_confirmed',
      updated_at: '2026-06-23T12:00:00.000Z'
    });
    await memoryStore.upsert({
      user_id: 'u1',
      kind: 'sender_feedback_ignored',
      scope_key: 'tombstonedhash',
      detail: { ignored_count: 2, tombstoned_at: '2026-06-23T13:00:00.000Z' },
      source: 'feedback_derived',
      confidence: 1,
      updated_at: '2026-06-23T13:00:00.000Z'
    });

    const store = new MemorySignalsBackedTypedMemoryStore(memoryStore, audit);
    const pack = await buildTypedMemoryContextPack(store, 'u1', 'ops', {
      kinds: ['preference', 'correction']
    });

    assert.deepEqual(pack.rows, []);
    assert.deepEqual(pack.row_ids, []);
    assert.deepEqual(pack.row_kinds, []);
    assert.equal(pack.suppressions_applied, 0);
    assert.equal(pack.preferences_applied, 0);
    const [entry] = await audit.recent('u1');
    assert.deepEqual(entry?.detail, {
      pack_kind: 'ops',
      row_kinds: [],
      row_ids: [],
      suppressions_applied: 0,
      preferences_applied: 0
    });
  });

  it('rejects invalid context-pack kind before writing retrieval audit', async () => {
    const audit = new InMemoryAuditStore();
    const store = new MemorySignalsBackedTypedMemoryStore(new InMemoryMemorySignalStore(), audit);

    await assert.rejects(
      () => buildTypedMemoryContextPack(store, 'u1', 'memory-212-555-1212' as never),
      /pack_kind must be one of/
    );

    assert.deepEqual(await audit.recent('u1'), []);
  });
});
