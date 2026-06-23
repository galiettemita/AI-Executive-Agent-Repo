import { type AuditStore } from '../core/audit.js';
import { redact } from '../core/safe-logger.js';

// M1 no-migration typed-memory facade.
//
// This module intentionally ships only the typed TypeScript surface plus an
// in-memory dormant store. It does NOT create tables, wire consumers, feed the
// ranker/HMR, or persist business data. The full M1 migration path remains
// governed by docs/BREVIO_MEMORY_AND_SKILL_OS.md §11. This facade lets the code
// name the memory contracts now without sneaking an untyped JSON dump or a
// half-wired runtime feature into production.

export const TYPED_MEMORY_KINDS = [
  'semantic',
  'preference',
  'project',
  'contact',
  'repeated_behavior'
] as const;

export type TypedMemoryKind = (typeof TYPED_MEMORY_KINDS)[number];

export const TYPED_MEMORY_CONFIDENCE_LEVELS = ['low', 'medium', 'high'] as const;
export type TypedMemoryConfidence = (typeof TYPED_MEMORY_CONFIDENCE_LEVELS)[number];

export const TYPED_MEMORY_RETRIEVAL_PACK_KINDS = [
  'ranker',
  'hmr',
  'explain',
  'drafter',
  'ops'
] as const;
export type TypedMemoryRetrievalPackKind = (typeof TYPED_MEMORY_RETRIEVAL_PACK_KINDS)[number];

export const TYPED_MEMORY_SOURCES = [
  'user_provided',
  'user_stated',
  'founder_default',
  'feedback_derived',
  'consolidation_proposed',
  'ops_injected'
] as const;

export type TypedMemorySource = (typeof TYPED_MEMORY_SOURCES)[number];

export function isTypedMemoryKind(value: unknown): value is TypedMemoryKind {
  return typeof value === 'string' && (TYPED_MEMORY_KINDS as readonly string[]).includes(value);
}

export function isTypedMemoryConfidence(value: unknown): value is TypedMemoryConfidence {
  return (
    typeof value === 'string' &&
    (TYPED_MEMORY_CONFIDENCE_LEVELS as readonly string[]).includes(value)
  );
}

export function isTypedMemoryRetrievalPackKind(
  value: unknown
): value is TypedMemoryRetrievalPackKind {
  return (
    typeof value === 'string' &&
    (TYPED_MEMORY_RETRIEVAL_PACK_KINDS as readonly string[]).includes(value)
  );
}

export function isTypedMemorySource(value: unknown): value is TypedMemorySource {
  return typeof value === 'string' && (TYPED_MEMORY_SOURCES as readonly string[]).includes(value);
}

export interface TypedMemoryBase {
  readonly id?: number;
  readonly user_id: string;
  readonly kind: TypedMemoryKind;
  readonly scope_key: string;
  readonly source: TypedMemorySource;
  readonly source_ref: string;
  readonly confidence: TypedMemoryConfidence;
  readonly created_at: string;
  readonly updated_at: string;
  readonly stale_marked_at: string | null;
  readonly retracted: boolean;
  readonly superseded_by: number | null;
}

export interface UserProfileFactMemory extends TypedMemoryBase {
  readonly kind: 'semantic';
  readonly attribute: string;
  readonly value: Record<string, unknown>;
}

export interface UserPreferenceMemory extends TypedMemoryBase {
  readonly kind: 'preference';
  readonly attribute: string;
  readonly value: string | number | boolean | Record<string, unknown>;
}

export interface UserProjectMemory extends TypedMemoryBase {
  readonly kind: 'project';
  readonly project_id: string;
  readonly title: string;
  readonly status: 'active' | 'paused' | 'done';
  readonly deadline_iso: string | null;
}

export interface UserContactMemory extends TypedMemoryBase {
  readonly kind: 'contact';
  readonly contact_hmac: string;
  readonly label: string;
  readonly role: string;
}

export interface UserBehaviorPatternMemory extends TypedMemoryBase {
  readonly kind: 'repeated_behavior';
  readonly pattern_id: string;
  readonly trigger: string;
  readonly action_observed: string;
  readonly occurrences: number;
  readonly last_observed_at: string;
}

export type TypedMemoryRow =
  | UserProfileFactMemory
  | UserPreferenceMemory
  | UserProjectMemory
  | UserContactMemory
  | UserBehaviorPatternMemory;

export type NewTypedMemoryRow = Omit<TypedMemoryRow, 'id' | 'created_at' | 'updated_at'> & {
  readonly created_at?: string;
  readonly updated_at?: string;
};

export interface TypedMemoryRetrievalAudit {
  readonly user_id: string;
  readonly pack_kind: TypedMemoryRetrievalPackKind;
  readonly kinds: readonly TypedMemoryKind[];
  readonly returned_ids: readonly number[];
  readonly suppressions_applied?: number;
  readonly preferences_applied?: number;
}

export interface TypedMemoryStore {
  write(row: NewTypedMemoryRow): Promise<TypedMemoryRow>;
  get(userId: string, kind: TypedMemoryKind, scopeKey: string): Promise<TypedMemoryRow | null>;
  listActive(userId: string, kinds?: readonly TypedMemoryKind[]): Promise<readonly TypedMemoryRow[]>;
  markRetrieved(audit: TypedMemoryRetrievalAudit): Promise<void>;
  retract(userId: string, kind: TypedMemoryKind, scopeKey: string, supersededBy?: number | null): Promise<boolean>;
}

function assertSafeScopeKey(scopeKey: string): void {
  if (scopeKey.trim().length === 0) {
    throw new Error('typed memory scope_key must be non-empty');
  }
  if (scopeKey.includes('@')) {
    throw new Error('typed memory scope_key must not contain raw email addresses');
  }
}

function assertTypedMemoryKindValue(kind: string): void {
  if (!isTypedMemoryKind(kind)) {
    throw new Error(`typed memory kind must be one of ${TYPED_MEMORY_KINDS.join(', ')}`);
  }
}

function assertTypedMemoryConfidenceValue(confidence: string): void {
  if (!isTypedMemoryConfidence(confidence)) {
    throw new Error(
      `typed memory confidence must be one of ${TYPED_MEMORY_CONFIDENCE_LEVELS.join(', ')}`
    );
  }
}

function assertTypedMemorySourceValue(source: string): void {
  if (!isTypedMemorySource(source)) {
    throw new Error(`typed memory source must be one of ${TYPED_MEMORY_SOURCES.join(', ')}`);
  }
}

function assertRetrievalPackKind(packKind: string): void {
  if (!isTypedMemoryRetrievalPackKind(packKind)) {
    throw new Error(
      `typed memory retrieval pack_kind must be one of ${TYPED_MEMORY_RETRIEVAL_PACK_KINDS.join(', ')}`
    );
  }
}

function assertRetrievalKinds(kinds: readonly string[]): void {
  for (const kind of kinds) {
    assertTypedMemoryKindValue(kind);
  }
}

function cloneAndFreeze<T extends TypedMemoryRow>(row: T): T {
  return Object.freeze({
    ...row,
    ...(row.kind === 'semantic' ? { value: Object.freeze({ ...row.value }) } : {}),
    ...(row.kind === 'preference' && typeof row.value === 'object' && row.value !== null
      ? { value: Object.freeze({ ...row.value }) }
      : {})
  }) as unknown as T;
}

function rowKey(userId: string, kind: TypedMemoryKind, scopeKey: string): string {
  return `${userId}|${kind}|${scopeKey}`;
}

function isActiveRetrievable(row: TypedMemoryRow): boolean {
  return !row.retracted && row.confidence !== 'low' && row.stale_marked_at === null;
}

function safeRowForStorage(row: NewTypedMemoryRow, id: number): TypedMemoryRow {
  assertSafeScopeKey(row.scope_key);
  const now = new Date().toISOString();
  const base = {
    ...row,
    id,
    created_at: row.created_at ?? now,
    updated_at: row.updated_at ?? now,
    stale_marked_at: row.stale_marked_at ?? null,
    retracted: row.retracted ?? false,
    superseded_by: row.superseded_by ?? null
  } as TypedMemoryRow;

  return cloneAndFreeze(redact(base) as TypedMemoryRow);
}

export class InMemoryTypedMemoryStore implements TypedMemoryStore {
  private readonly rows = new Map<string, TypedMemoryRow>();
  private nextId = 1;
  private readonly audit?: AuditStore;

  constructor(audit?: AuditStore) {
    this.audit = audit;
  }

  async write(input: NewTypedMemoryRow): Promise<TypedMemoryRow> {
    assertTypedMemoryKindValue(input.kind);
    assertTypedMemorySourceValue(input.source);
    assertTypedMemoryConfidenceValue(input.confidence);
    const id = this.rows.get(rowKey(input.user_id, input.kind, input.scope_key))?.id ?? this.nextId++;
    const row = safeRowForStorage(input, id);
    this.rows.set(rowKey(row.user_id, row.kind, row.scope_key), row);
    return row;
  }

  async get(userId: string, kind: TypedMemoryKind, scopeKey: string): Promise<TypedMemoryRow | null> {
    const row = this.rows.get(rowKey(userId, kind, scopeKey));
    if (!row || !isActiveRetrievable(row)) return null;
    return row;
  }

  async listActive(
    userId: string,
    kinds: readonly TypedMemoryKind[] = TYPED_MEMORY_KINDS
  ): Promise<readonly TypedMemoryRow[]> {
    const kindSet = new Set<TypedMemoryKind>(kinds);
    const out: TypedMemoryRow[] = [];
    for (const row of this.rows.values()) {
      if (row.user_id !== userId) continue;
      if (!isActiveRetrievable(row)) continue;
      if (!kindSet.has(row.kind)) continue;
      out.push(row);
    }
    out.sort((a, b) => b.updated_at.localeCompare(a.updated_at));
    return Object.freeze(out);
  }

  async markRetrieved(audit: TypedMemoryRetrievalAudit): Promise<void> {
    assertRetrievalPackKind(audit.pack_kind);
    assertRetrievalKinds(audit.kinds);
    await this.audit?.write({
      actor_user_id: audit.user_id,
      actor_ip: null,
      actor_user_agent: null,
      action: 'brevio.memory.retrieved',
      target: 'typed_memory',
      result: 'success',
      detail: {
        pack_kind: audit.pack_kind,
        row_kinds: [...audit.kinds],
        row_ids: [...audit.returned_ids],
        suppressions_applied: audit.suppressions_applied ?? 0,
        preferences_applied: audit.preferences_applied ?? 0
      }
    });
  }

  async retract(
    userId: string,
    kind: TypedMemoryKind,
    scopeKey: string,
    supersededBy: number | null = null
  ): Promise<boolean> {
    const existing = this.rows.get(rowKey(userId, kind, scopeKey));
    if (!existing || existing.retracted) return false;
    const retracted = cloneAndFreeze({
      ...existing,
      retracted: true,
      superseded_by: supersededBy,
      updated_at: new Date().toISOString()
    } as TypedMemoryRow);
    this.rows.set(rowKey(userId, kind, scopeKey), retracted);
    await this.audit?.write({
      actor_user_id: userId,
      actor_ip: null,
      actor_user_agent: null,
      action: 'brevio.memory.retraction_recorded',
      target: 'typed_memory',
      result: 'success',
      detail: {
        kind,
        retracted_id: existing.id,
        superseded_by: supersededBy
      }
    });
    return true;
  }
}
