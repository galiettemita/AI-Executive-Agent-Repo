import {
  readTypedMemory,
  type TypedMemoryConfidence,
  type TypedMemorySource,
  type TypedMemoryStore,
  type UserPreferenceMemory
} from './typed-memory.js';

export interface VisibleExplicitPreferenceRecallQuery {
  readonly attribute?: string;
  readonly scopeKeys?: readonly string[];
  readonly sources?: readonly TypedMemorySource[];
  readonly minConfidence?: TypedMemoryConfidence;
}

export interface VisibleExplicitPreferenceRecall {
  readonly user_id: string;
  readonly memory_id?: number;
  readonly attribute: string;
  readonly preference_summary: string;
  readonly visible_explanation: string;
  readonly why_used: string;
  readonly source_metadata: {
    readonly source: TypedMemorySource;
    readonly source_ref_type: 'memory_signal' | 'reply' | 'unknown';
    readonly confidence: TypedMemoryConfidence;
    readonly updated_at: string;
  };
  readonly audit_metadata: {
    readonly memory_kind: 'preference';
    readonly row_id?: number;
    readonly scope_key: string;
  };
}

function humanizePreferenceAttribute(attribute: string): string {
  return attribute.replace(/[_:-]+/g, ' ').replace(/\s+/g, ' ').trim() || 'preference';
}

function safeUserVisiblePreferenceValue(value: UserPreferenceMemory['value']): string {
  if (typeof value === 'boolean') return value ? 'yes' : 'no';
  if (typeof value === 'number') return String(value);
  if (typeof value === 'string') {
    const trimmed = value.trim().replace(/\s+/g, ' ');
    if (trimmed.includes('@')) return '[redacted]';
    return trimmed.length > 80 ? `${trimmed.slice(0, 77)}...` : trimmed;
  }
  return 'saved structured preference';
}

function sourceRefType(sourceRef: string): VisibleExplicitPreferenceRecall['source_metadata']['source_ref_type'] {
  if (sourceRef.startsWith('memory_signal:')) return 'memory_signal';
  if (sourceRef.startsWith('reply:')) return 'reply';
  return 'unknown';
}

export async function recallVisibleExplicitPreference(
  store: Pick<TypedMemoryStore, 'listActive'>,
  userId: string,
  query: VisibleExplicitPreferenceRecallQuery = {}
): Promise<VisibleExplicitPreferenceRecall | null> {
  const rows = await readTypedMemory(store, userId, {
    kinds: ['preference'],
    scopeKeys: query.scopeKeys,
    sources: query.sources ?? ['user_provided', 'user_stated', 'founder_default'],
    minConfidence: query.minConfidence ?? 'medium',
    limit: 10
  });

  const preference = rows.find(
    (row): row is UserPreferenceMemory =>
      row.kind === 'preference' &&
      (query.attribute === undefined || row.attribute === query.attribute)
  );
  if (preference === undefined) return null;

  const label = humanizePreferenceAttribute(preference.attribute);
  const value = safeUserVisiblePreferenceValue(preference.value);
  const preferenceSummary = `${label}: ${value}`;
  const whyUsed = `I used your saved ${label} preference because it was explicitly stored for this user.`;

  return Object.freeze({
    user_id: userId,
    memory_id: preference.id,
    attribute: preference.attribute,
    preference_summary: preferenceSummary,
    visible_explanation: `${whyUsed} (${preferenceSummary})`,
    why_used: whyUsed,
    source_metadata: Object.freeze({
      source: preference.source,
      source_ref_type: sourceRefType(preference.source_ref),
      confidence: preference.confidence,
      updated_at: preference.updated_at
    }),
    audit_metadata: Object.freeze({
      memory_kind: 'preference' as const,
      row_id: preference.id,
      scope_key: preference.scope_key
    })
  });
}
