import { type AuditStore } from '../core/audit.js';
import {
  type MemorySignal,
  type MemorySignalSource,
  type MemorySignalStore
} from './memory-signals.js';

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
  'correction',
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

export interface UserCorrectionMemory extends TypedMemoryBase {
  readonly kind: 'correction';
  readonly rule: 'sender_suppressed' | 'sender_feedback_ignored';
  readonly target_hmac: string;
  readonly value: Record<string, unknown>;
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
  | UserCorrectionMemory
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

export interface TypedMemoryQuery {
  readonly kinds?: readonly TypedMemoryKind[];
  readonly scopeKeys?: readonly string[];
  readonly sources?: readonly TypedMemorySource[];
  readonly minConfidence?: TypedMemoryConfidence;
  readonly limit?: number;
}

export interface TypedMemoryContextPack {
  readonly pack_kind: TypedMemoryRetrievalPackKind;
  readonly user_id: string;
  readonly rows: readonly TypedMemoryRow[];
  readonly row_ids: readonly number[];
  readonly row_kinds: readonly TypedMemoryKind[];
  readonly suppressions_applied: number;
  readonly preferences_applied: number;
}

export interface TypedMemoryStore {
  write(row: NewTypedMemoryRow): Promise<TypedMemoryRow>;
  get(userId: string, kind: TypedMemoryKind, scopeKey: string): Promise<TypedMemoryRow | null>;
  listActive(userId: string, kinds?: readonly TypedMemoryKind[]): Promise<readonly TypedMemoryRow[]>;
  markRetrieved(audit: TypedMemoryRetrievalAudit): Promise<void>;
  retract(userId: string, kind: TypedMemoryKind, scopeKey: string, supersededBy?: number | null): Promise<boolean>;
}

export const BRIDGED_MEMORY_SIGNAL_KINDS = ['timing_preference', 'quietness_preference'] as const;
export type BridgedMemorySignalKind = (typeof BRIDGED_MEMORY_SIGNAL_KINDS)[number];
export type BridgedPreferenceSignalKind = BridgedMemorySignalKind;

export const BRIDGED_CORRECTION_SIGNAL_KINDS = [
  'sender_suppressed',
  'sender_feedback_ignored'
] as const;
export type BridgedCorrectionSignalKind = (typeof BRIDGED_CORRECTION_SIGNAL_KINDS)[number];

const BRIDGED_TYPED_SCOPE_KEYS: Readonly<Record<BridgedPreferenceSignalKind, string>> = Object.freeze({
  timing_preference: 'signal:timing_preference',
  quietness_preference: 'signal:quietness_preference'
});

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

function cloneAndDeepFreeze<T>(value: T): T {
  if (Array.isArray(value)) {
    return Object.freeze(value.map((item) => cloneAndDeepFreeze(item))) as T;
  }
  if (typeof value === 'object' && value !== null) {
    const out: Record<string, unknown> = {};
    for (const [key, nested] of Object.entries(value)) {
      out[key] = cloneAndDeepFreeze(nested);
    }
    return Object.freeze(out) as T;
  }
  return value;
}

function cloneAndFreeze<T extends TypedMemoryRow>(row: T): T {
  return Object.freeze({
    ...row,
    ...(row.kind === 'semantic' ? { value: cloneAndDeepFreeze(row.value) } : {}),
    ...(row.kind === 'preference' && typeof row.value === 'object' && row.value !== null
      ? { value: cloneAndDeepFreeze(row.value) }
      : {}),
    ...(row.kind === 'correction' ? { value: cloneAndDeepFreeze(row.value) } : {})
  }) as unknown as T;
}

function rowKey(userId: string, kind: TypedMemoryKind, scopeKey: string): string {
  return `${userId}|${kind}|${scopeKey}`;
}

function isActiveRetrievable(row: TypedMemoryRow): boolean {
  return !row.retracted && row.confidence !== 'low' && row.stale_marked_at === null;
}

function confidenceRank(confidence: TypedMemoryConfidence): number {
  switch (confidence) {
    case 'low':
      return 0;
    case 'medium':
      return 1;
    case 'high':
      return 2;
  }
}

function compareTypedMemoryRows(a: TypedMemoryRow, b: TypedMemoryRow): number {
  const updated = b.updated_at.localeCompare(a.updated_at);
  if (updated !== 0) return updated;
  const aId = a.id ?? Number.MAX_SAFE_INTEGER;
  const bId = b.id ?? Number.MAX_SAFE_INTEGER;
  if (aId !== bId) return aId - bId;
  return `${a.kind}:${a.scope_key}`.localeCompare(`${b.kind}:${b.scope_key}`);
}

function queryMatches(row: TypedMemoryRow, query: TypedMemoryQuery): boolean {
  if (query.kinds !== undefined && !query.kinds.includes(row.kind)) return false;
  if (query.scopeKeys !== undefined && !query.scopeKeys.includes(row.scope_key)) return false;
  if (query.sources !== undefined && !query.sources.includes(row.source)) return false;
  if (
    query.minConfidence !== undefined &&
    confidenceRank(row.confidence) < confidenceRank(query.minConfidence)
  ) {
    return false;
  }
  return true;
}

function typedMemoryContextRowIds(rows: readonly TypedMemoryRow[]): readonly number[] {
  const rowIds: number[] = [];
  for (const row of rows) {
    if (typeof row.id === 'number') {
      rowIds.push(row.id);
    }
  }
  return Object.freeze(rowIds);
}

function typedMemoryContextRowKinds(rows: readonly TypedMemoryRow[]): readonly TypedMemoryKind[] {
  const seen = new Set<TypedMemoryKind>();
  const rowKinds: TypedMemoryKind[] = [];
  for (const row of rows) {
    if (seen.has(row.kind)) continue;
    seen.add(row.kind);
    rowKinds.push(row.kind);
  }
  return Object.freeze(rowKinds);
}

function typedMemorySuppressionsApplied(rows: readonly TypedMemoryRow[]): number {
  let suppressionsApplied = 0;
  for (const row of rows) {
    if (row.kind === 'correction' && row.rule === 'sender_suppressed') {
      suppressionsApplied += 1;
    }
  }
  return suppressionsApplied;
}

function typedMemoryPreferencesApplied(rows: readonly TypedMemoryRow[]): number {
  let preferencesApplied = 0;
  for (const row of rows) {
    if (row.kind === 'preference') {
      preferencesApplied += 1;
    }
  }
  return preferencesApplied;
}

export function queryTypedMemoryRows(
  rows: readonly TypedMemoryRow[],
  query: TypedMemoryQuery = {}
): readonly TypedMemoryRow[] {
  const limit = query.limit ?? rows.length;
  if (!Number.isInteger(limit) || limit < 0) {
    throw new Error('typed memory query limit must be a non-negative integer');
  }
  const out = rows.filter((row) => isActiveRetrievable(row) && queryMatches(row, query));
  out.sort(compareTypedMemoryRows);
  return Object.freeze(out.slice(0, limit));
}

export async function readTypedMemory(
  store: Pick<TypedMemoryStore, 'listActive'>,
  userId: string,
  query: TypedMemoryQuery = {}
): Promise<readonly TypedMemoryRow[]> {
  const rows = await store.listActive(userId, query.kinds ?? TYPED_MEMORY_KINDS);
  return queryTypedMemoryRows(rows, query);
}

export async function buildTypedMemoryContextPack(
  store: Pick<TypedMemoryStore, 'listActive' | 'markRetrieved'>,
  userId: string,
  packKind: TypedMemoryRetrievalPackKind,
  query: TypedMemoryQuery = {}
): Promise<TypedMemoryContextPack> {
  assertRetrievalPackKind(packKind);

  const rows = await readTypedMemory(store, userId, query);
  const rowIds = typedMemoryContextRowIds(rows);
  const rowKinds = typedMemoryContextRowKinds(rows);
  const suppressionsApplied = typedMemorySuppressionsApplied(rows);
  const preferencesApplied = typedMemoryPreferencesApplied(rows);

  await store.markRetrieved({
    user_id: userId,
    pack_kind: packKind,
    kinds: rowKinds,
    returned_ids: rowIds,
    suppressions_applied: suppressionsApplied,
    preferences_applied: preferencesApplied
  });

  return Object.freeze({
    pack_kind: packKind,
    user_id: userId,
    rows,
    row_ids: rowIds,
    row_kinds: rowKinds,
    suppressions_applied: suppressionsApplied,
    preferences_applied: preferencesApplied
  });
}

function assertReadOnlyBridgeMutation(): never {
  throw new Error(
    'memory_signals-backed typed memory bridge is read-only; typed writes still require dedicated M1 tables'
  );
}

function typedMemoryConfidenceFromSignal(confidence: number): TypedMemoryConfidence {
  if (confidence >= 0.85) return 'high';
  if (confidence >= 0.6) return 'medium';
  return 'low';
}

function typedMemorySourceFromSignal(source: MemorySignalSource): TypedMemorySource {
  switch (source) {
    case 'user_confirmed':
      return 'user_stated';
    case 'founder_set':
      return 'founder_default';
    case 'feedback_derived':
      return 'feedback_derived';
    case 'inferred':
      return 'consolidation_proposed';
    case 'opt_out_drift_carrier':
      return 'ops_injected';
  }
}

function sourceRefForSignal(signal: MemorySignal): string {
  return signal.id === undefined
    ? `memory_signal:${signal.kind}`
    : `memory_signal:${signal.kind}:${signal.id}`;
}

function isBridgeEligiblePreferenceSignalSource(source: MemorySignalSource): boolean {
  return source === 'user_confirmed' || source === 'founder_set';
}

function isDeletedOrTombstonedMemorySignal(signal: MemorySignal): boolean {
  const detail = signal.detail;
  return (
    detail.deleted === true ||
    detail.tombstoned === true ||
    typeof detail.deleted_at === 'string' ||
    typeof detail.tombstoned_at === 'string'
  );
}

function typedPreferenceFromSignal(signal: MemorySignal): UserPreferenceMemory | null {
  if (signal.kind !== 'timing_preference' && signal.kind !== 'quietness_preference') {
    return null;
  }
  if (!isBridgeEligiblePreferenceSignalSource(signal.source)) {
    return null;
  }
  if (isDeletedOrTombstonedMemorySignal(signal)) {
    return null;
  }

  return cloneAndFreeze({
    id: signal.id,
    user_id: signal.user_id,
    kind: 'preference',
    scope_key: BRIDGED_TYPED_SCOPE_KEYS[signal.kind],
    source: typedMemorySourceFromSignal(signal.source),
    source_ref: sourceRefForSignal(signal),
    confidence: typedMemoryConfidenceFromSignal(signal.confidence),
    created_at: signal.updated_at,
    updated_at: signal.updated_at,
    stale_marked_at: null,
    retracted: false,
    superseded_by: null,
    attribute: signal.kind,
    value: cloneAndDeepFreeze(signal.detail)
  });
}

function typedCorrectionScopeKey(kind: BridgedCorrectionSignalKind, scopeKey: string): string {
  assertSafeScopeKey(scopeKey);
  return `signal:${kind}:${scopeKey}`;
}

function typedCorrectionFromSignal(signal: MemorySignal): UserCorrectionMemory | null {
  if (signal.kind !== 'sender_suppressed' && signal.kind !== 'sender_feedback_ignored') {
    return null;
  }
  if (signal.scope_key === null || signal.scope_key.includes('@')) {
    return null;
  }
  if (isDeletedOrTombstonedMemorySignal(signal)) {
    return null;
  }

  return cloneAndFreeze({
    id: signal.id,
    user_id: signal.user_id,
    kind: 'correction',
    scope_key: typedCorrectionScopeKey(signal.kind, signal.scope_key),
    source: typedMemorySourceFromSignal(signal.source),
    source_ref: sourceRefForSignal(signal),
    confidence: typedMemoryConfidenceFromSignal(signal.confidence),
    created_at: signal.updated_at,
    updated_at: signal.updated_at,
    stale_marked_at: null,
    retracted: false,
    superseded_by: null,
    rule: signal.kind,
    target_hmac: signal.scope_key,
    value: cloneAndDeepFreeze(signal.detail)
  });
}

function typedRowFromBridgedSignal(signal: MemorySignal): TypedMemoryRow | null {
  return typedPreferenceFromSignal(signal) ?? typedCorrectionFromSignal(signal);
}

function bridgedSignalKindForTypedPreferenceScope(scopeKey: string): BridgedPreferenceSignalKind | null {
  for (const kind of ['timing_preference', 'quietness_preference'] as const) {
    if (BRIDGED_TYPED_SCOPE_KEYS[kind] === scopeKey) return kind;
  }
  return null;
}

function bridgedCorrectionSignalForTypedScope(
  scopeKey: string
): { readonly kind: BridgedCorrectionSignalKind; readonly signalScopeKey: string } | null {
  for (const kind of BRIDGED_CORRECTION_SIGNAL_KINDS) {
    const prefix = `signal:${kind}:`;
    if (!scopeKey.startsWith(prefix)) continue;
    const signalScopeKey = scopeKey.slice(prefix.length);
    if (signalScopeKey.length === 0 || signalScopeKey.includes('@')) return null;
    return { kind, signalScopeKey };
  }
  return null;
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

  return cloneAndFreeze(base);
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
    assertTypedMemoryKindValue(kind);
    assertSafeScopeKey(scopeKey);
    const row = this.rows.get(rowKey(userId, kind, scopeKey));
    if (!row || !isActiveRetrievable(row)) return null;
    return row;
  }

  async listActive(
    userId: string,
    kinds: readonly TypedMemoryKind[] = TYPED_MEMORY_KINDS
  ): Promise<readonly TypedMemoryRow[]> {
    assertRetrievalKinds(kinds);
    const kindSet = new Set<TypedMemoryKind>(kinds);
    const out: TypedMemoryRow[] = [];
    for (const row of this.rows.values()) {
      if (row.user_id !== userId) continue;
      if (!isActiveRetrievable(row)) continue;
      if (!kindSet.has(row.kind)) continue;
      out.push(row);
    }
    out.sort(compareTypedMemoryRows);
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
    assertTypedMemoryKindValue(kind);
    assertSafeScopeKey(scopeKey);
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

export function typedMemoryScopeKeyForBridgedMemorySignal(kind: BridgedMemorySignalKind): string {
  if (kind === 'timing_preference' || kind === 'quietness_preference') {
    return BRIDGED_TYPED_SCOPE_KEYS[kind];
  }
  return `signal:${kind}:<scope_key>`;
}

export function typedMemoryScopeKeyForBridgedCorrectionSignal(
  kind: BridgedCorrectionSignalKind,
  signalScopeKey: string
): string {
  return typedCorrectionScopeKey(kind, signalScopeKey);
}

export class MemorySignalsBackedTypedMemoryStore implements TypedMemoryStore {
  private readonly memoryStore: Pick<MemorySignalStore, 'get' | 'list'>;
  private readonly audit?: AuditStore;

  constructor(memoryStore: Pick<MemorySignalStore, 'get' | 'list'>, audit?: AuditStore) {
    this.memoryStore = memoryStore;
    this.audit = audit;
  }

  async write(input: NewTypedMemoryRow): Promise<TypedMemoryRow> {
    void input;
    assertReadOnlyBridgeMutation();
  }

  async get(userId: string, kind: TypedMemoryKind, scopeKey: string): Promise<TypedMemoryRow | null> {
    assertTypedMemoryKindValue(kind);
    assertSafeScopeKey(scopeKey);
    const preferenceKind = kind === 'preference' ? bridgedSignalKindForTypedPreferenceScope(scopeKey) : null;
    const correctionMatch = kind === 'correction' ? bridgedCorrectionSignalForTypedScope(scopeKey) : null;
    if (preferenceKind === null && correctionMatch === null) return null;
    const signal = preferenceKind
      ? await this.memoryStore.get(userId, preferenceKind, null)
      : correctionMatch
        ? await this.memoryStore.get(userId, correctionMatch.kind, correctionMatch.signalScopeKey)
        : null;
    const row = signal ? typedRowFromBridgedSignal(signal) : null;
    if (!row || !isActiveRetrievable(row)) return null;
    return row;
  }

  async listActive(
    userId: string,
    kinds: readonly TypedMemoryKind[] = TYPED_MEMORY_KINDS
  ): Promise<readonly TypedMemoryRow[]> {
    assertRetrievalKinds(kinds);
    const includePreferences = kinds.includes('preference');
    const includeCorrections = kinds.includes('correction');
    if (!includePreferences && !includeCorrections) {
      return Object.freeze([] as TypedMemoryRow[]);
    }

    const rows = (await this.memoryStore.list(userId))
      .map((signal) => typedRowFromBridgedSignal(signal))
      .filter(
        (row): row is TypedMemoryRow =>
          row !== null &&
          isActiveRetrievable(row) &&
          ((row.kind === 'preference' && includePreferences) ||
            (row.kind === 'correction' && includeCorrections))
      )
      .sort(compareTypedMemoryRows);

    return Object.freeze(rows);
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
    void userId;
    void kind;
    void scopeKey;
    void supersededBy;
    assertReadOnlyBridgeMutation();
  }
}
