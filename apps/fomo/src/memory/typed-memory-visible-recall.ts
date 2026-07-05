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

export interface VisibleMemoryWhyUsedExplanation {
  readonly memory_used: string;
  readonly answer: string;
  readonly relevance: string;
  readonly source: string;
  readonly audit: string;
  readonly safety: string;
}

const MAX_WHY_USED_FIELD_LENGTH = 240;

function humanizePreferenceAttribute(attribute: string): string {
  return attribute.replace(/[_:-]+/g, ' ').replace(/\s+/g, ' ').trim() || 'preference';
}

function boundedHumanText(text: string): string {
  const normalized = text.replace(/\s+/g, ' ').trim();
  if (normalized.length <= MAX_WHY_USED_FIELD_LENGTH) return normalized;
  return `${normalized.slice(0, MAX_WHY_USED_FIELD_LENGTH - 3)}...`;
}

function sourceLabel(source: TypedMemorySource): string {
  switch (source) {
    case 'user_provided':
    case 'user_stated':
      return 'a user-stated preference';
    case 'founder_default':
      return 'a founder-set default preference';
    case 'feedback_derived':
      return 'a feedback-derived memory';
    case 'consolidation_proposed':
      return 'a proposed memory consolidation';
    case 'ops_injected':
      return 'an operator-provided memory';
  }
}

function sourceRefLabel(refType: VisibleExplicitPreferenceRecall['source_metadata']['source_ref_type']): string {
  switch (refType) {
    case 'memory_signal':
      return 'the memory-signals substrate';
    case 'reply':
      return 'a prior user reply';
    case 'unknown':
      return 'stored memory metadata';
  }
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
      row.superseded_by === null &&
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

export function explainVisibleExplicitPreferenceUse(
  recall: VisibleExplicitPreferenceRecall
): VisibleMemoryWhyUsedExplanation {
  const label = humanizePreferenceAttribute(recall.attribute);
  const source = sourceLabel(recall.source_metadata.source);
  const sourceRef = sourceRefLabel(recall.source_metadata.source_ref_type);
  const confidence = recall.source_metadata.confidence;
  const updatedAt = recall.source_metadata.updated_at;

  const memoryUsed = `your saved ${label} preference`;
  const relevance = `I used it because this request matched the saved ${label} preference for this user.`;
  const sourceText = `This came from ${source} recorded through ${sourceRef}.`;
  const auditText = `The recall used ${confidence}-confidence preference metadata last updated ${updatedAt}; raw preference content is not needed to explain why it was used.`;
  const safetyText = 'The explanation is scoped to this user and summarizes memory metadata without exposing raw private values.';
  const answer = `${relevance} ${sourceText} ${auditText}`;

  return Object.freeze({
    memory_used: boundedHumanText(memoryUsed),
    answer: boundedHumanText(answer),
    relevance: boundedHumanText(relevance),
    source: boundedHumanText(sourceText),
    audit: boundedHumanText(auditText),
    safety: boundedHumanText(safetyText)
  });
}
