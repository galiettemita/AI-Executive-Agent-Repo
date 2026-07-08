import {
  readTypedMemory,
  type NewTypedMemoryRow,
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

export interface VisibleExplicitPreferenceReview {
  readonly user_id: string;
  readonly answer: string;
  readonly preferences: readonly VisibleExplicitPreferenceRecall[];
  readonly audit_metadata: {
    readonly memory_kind: 'preference';
    readonly returned_count: number;
    readonly row_ids: readonly number[];
    readonly scope_keys: readonly string[];
  };
}

export interface VisibleMemoryReviewCommandResult {
  readonly action: 'review_visible_explicit_preferences';
  readonly user_id: string;
  readonly answer: string;
  readonly review: VisibleExplicitPreferenceReview;
  readonly audit_metadata: {
    readonly memory_kind: 'preference';
    readonly matched_intent: 'memory_review';
    readonly returned_count: number;
    readonly row_ids: readonly number[];
    readonly scope_keys: readonly string[];
  };
}

export interface VisibleMemoryExplanationCommandResult {
  readonly action: 'explain_visible_explicit_preference_use';
  readonly user_id: string;
  readonly answer: string;
  readonly explanation: VisibleMemoryWhyUsedExplanation;
  readonly audit_metadata: {
    readonly memory_kind: 'preference';
    readonly matched_intent: 'memory_explanation';
  };
}

export interface VisibleMemoryForgetCommandResult {
  readonly action: 'forget_visible_explicit_preference';
  readonly user_id: string;
  readonly answer: string;
  readonly forget: VisiblePreferenceForgetResult;
  readonly audit_metadata: {
    readonly memory_kind: 'preference';
    readonly matched_intent: 'memory_forget';
    readonly scope_key: string;
    readonly forgotten_row_id?: number;
  };
}

export interface VisibleMemoryCorrectCommandResult {
  readonly action: 'correct_visible_explicit_preference';
  readonly user_id: string;
  readonly answer: string;
  readonly correction: VisiblePreferenceCorrectResult;
  readonly audit_metadata: {
    readonly memory_kind: 'preference';
    readonly matched_intent: 'memory_correct';
    readonly scope_key: string;
    readonly previous_row_id?: number;
    readonly corrected_row_id?: number;
    readonly source: Extract<TypedMemorySource, 'user_provided' | 'user_stated'>;
    readonly updated_at: string;
  };
}

export interface VisiblePreferenceRememberOptions {
  readonly attribute: string;
  readonly value: UserPreferenceMemory['value'];
  readonly updatedAt?: string;
  readonly sourceRef?: string;
  readonly source?: Extract<TypedMemorySource, 'user_provided' | 'user_stated'>;
  readonly confidence?: Extract<TypedMemoryConfidence, 'medium' | 'high'>;
}

export interface VisiblePreferenceRememberResult {
  readonly action: 'remembered';
  readonly user_id: string;
  readonly attribute: string;
  readonly message: string;
  readonly audit_metadata: {
    readonly memory_kind: 'preference';
    readonly scope_key: string;
    readonly remembered_row_id?: number;
    readonly source: Extract<TypedMemorySource, 'user_provided' | 'user_stated'>;
    readonly confidence: Extract<TypedMemoryConfidence, 'medium' | 'high'>;
    readonly updated_at: string;
  };
}

export interface VisiblePreferenceForgetResult {
  readonly action: 'forgot';
  readonly user_id: string;
  readonly attribute: string;
  readonly forgotten: boolean;
  readonly message: string;
  readonly audit_metadata: {
    readonly memory_kind: 'preference';
    readonly scope_key: string;
    readonly forgotten_row_id?: number;
  };
}

export interface VisiblePreferenceCorrectOptions {
  readonly correctedValue: UserPreferenceMemory['value'];
  readonly updatedAt?: string;
  readonly sourceRef?: string;
  readonly source?: Extract<TypedMemorySource, 'user_provided' | 'user_stated'>;
}

export interface VisiblePreferenceCorrectResult {
  readonly action: 'corrected';
  readonly user_id: string;
  readonly attribute: string;
  readonly message: string;
  readonly audit_metadata: {
    readonly memory_kind: 'preference';
    readonly scope_key: string;
    readonly previous_row_id?: number;
    readonly corrected_row_id?: number;
    readonly source: Extract<TypedMemorySource, 'user_provided' | 'user_stated'>;
    readonly updated_at: string;
  };
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

function visiblePreferenceScopeKey(attribute: string): string {
  const normalized = attribute.trim();
  if (normalized.length === 0) {
    throw new Error('visible preference attribute must be non-empty');
  }
  if (normalized.includes('@')) {
    throw new Error('visible preference attribute must not contain raw email addresses');
  }
  return `preference:${normalized}`;
}

function visibleRecallFromPreference(
  userId: string,
  preference: UserPreferenceMemory
): VisibleExplicitPreferenceRecall {
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

  return visibleRecallFromPreference(userId, preference);
}

export async function reviewVisibleExplicitPreferences(
  store: Pick<TypedMemoryStore, 'listActive'>,
  userId: string,
  query: VisibleExplicitPreferenceRecallQuery = {}
): Promise<VisibleExplicitPreferenceReview> {
  const rows = await readTypedMemory(store, userId, {
    kinds: ['preference'],
    scopeKeys: query.scopeKeys,
    sources: query.sources ?? ['user_provided', 'user_stated', 'founder_default'],
    minConfidence: query.minConfidence ?? 'medium',
    limit: 25
  });
  const preferences = rows
    .filter(
      (row): row is UserPreferenceMemory =>
        row.kind === 'preference' &&
        row.superseded_by === null &&
        (query.attribute === undefined || row.attribute === query.attribute)
    )
    .map((preference) => visibleRecallFromPreference(userId, preference));
  const summaries = preferences.map((preference) => preference.preference_summary);
  const answer =
    preferences.length === 0
      ? 'I do not have any active explicit preferences saved for you that are safe to show here.'
      : `I remember these active explicit preferences for you: ${summaries.join('; ')}.`;

  return Object.freeze({
    user_id: userId,
    answer,
    preferences: Object.freeze(preferences),
    audit_metadata: Object.freeze({
      memory_kind: 'preference' as const,
      returned_count: preferences.length,
      row_ids: Object.freeze(
        preferences
          .map((preference) => preference.audit_metadata.row_id)
          .filter((rowId): rowId is number => typeof rowId === 'number')
      ),
      scope_keys: Object.freeze(preferences.map((preference) => preference.audit_metadata.scope_key))
    })
  });
}

function normalizedVisibleMemoryCommandText(text: string): string {
  return text
    .toLowerCase()
    .replace(/[’']/g, '')
    .replace(/[^a-z0-9\s]/g, ' ')
    .replace(/\s+/g, ' ')
    .trim();
}

export function isVisibleMemoryReviewCommandText(text: string): boolean {
  const normalized = normalizedVisibleMemoryCommandText(text);
  if (normalized.length === 0) return false;

  if (/^(please )?(remember|save) (this|that)\b/.test(normalized)) return false;
  if (/^(can you|could you|please) (remember|save)\b/.test(normalized)) return false;

  return [
    /^what (do|can) you remember( about me)?$/,
    /^what (does|can) brevio remember( about me)?$/,
    /^what have you (remembered|saved)( about me)?$/,
    /^what (memories|preferences) do you have( for me| about me)?$/,
    /^(show|list|tell me) (my )?(memories|saved preferences)$/,
    /^(show|list|tell me) what you (remember|saved)( about me)?$/,
    /^do you remember anything about me$/
  ].some((pattern) => pattern.test(normalized));
}

export function isVisibleMemoryExplanationCommandText(text: string): boolean {
  const normalized = normalizedVisibleMemoryCommandText(text);
  if (normalized.length === 0) return false;

  if (/^(please )?(remember|save) (this|that)\b/.test(normalized)) return false;
  if (/^(can you|could you|please) (remember|save)\b/.test(normalized)) return false;

  return [
    /^why did you (remember|save|use) (that|this|it|my memory|my preference)$/,
    /^why did brevio (remember|save|use) (that|this|it|my memory|my preference)$/,
    /^why do you remember (that|this|it|my preference)$/,
    /^why does brevio remember (that|this|it|my preference)$/,
    /^why was (that|this|it|my memory|my preference) (remembered|saved|used)$/,
    /^(explain|tell me) why you (remembered|saved|used) (that|this|it|my memory|my preference)$/,
    /^(explain|tell me) why brevio (remembered|saved|used) (that|this|it|my memory|my preference)$/
  ].some((pattern) => pattern.test(normalized));
}

export function isVisibleMemoryForgetCommandText(text: string): boolean {
  const normalized = normalizedVisibleMemoryCommandText(text);
  if (normalized.length === 0) return false;

  if (/^(please )?(remember|save) (this|that)\b/.test(normalized)) return false;
  if (/^(can you|could you|please) (remember|save)\b/.test(normalized)) return false;

  return [
    /^forget (that|this|it|my memory|my preference)$/,
    /^forget that (saved )?(memory|preference)$/,
    /^please forget (that|this|it|my memory|my preference)$/,
    /^(delete|remove) (that|this|it|my memory|my preference)$/,
    /^(delete|remove) that (saved )?(memory|preference)$/,
    /^stop remembering (that|this|it|my memory|my preference)$/,
    /^can you forget (that|this|it|my memory|my preference)$/
  ].some((pattern) => pattern.test(normalized));
}

export function isVisibleMemoryCorrectCommandText(text: string): boolean {
  const normalized = normalizedVisibleMemoryCommandText(text);
  if (normalized.length === 0) return false;

  if (/^(please )?(remember|save) (this|that)\b/.test(normalized)) return false;
  if (/^(can you|could you|please) (remember|save)\b/.test(normalized)) return false;

  return [
    /^correct (that|this|it|my memory|my preference)$/,
    /^correct that (saved )?(memory|preference)$/,
    /^please correct (that|this|it|my memory|my preference)$/,
    /^(update|change) (that|this|it|my memory|my preference)$/,
    /^(update|change) that (saved )?(memory|preference)$/,
    /^that (memory|preference) is wrong$/,
    /^that saved (memory|preference) is wrong$/
  ].some((pattern) => pattern.test(normalized));
}

export async function answerVisibleMemoryReviewCommand(
  store: Pick<TypedMemoryStore, 'listActive'>,
  userId: string,
  text: string,
  query: VisibleExplicitPreferenceRecallQuery = {}
): Promise<VisibleMemoryReviewCommandResult | null> {
  if (!isVisibleMemoryReviewCommandText(text)) return null;

  const review = await reviewVisibleExplicitPreferences(store, userId, query);
  return Object.freeze({
    action: 'review_visible_explicit_preferences' as const,
    user_id: userId,
    answer: review.answer,
    review,
    audit_metadata: Object.freeze({
      memory_kind: 'preference' as const,
      matched_intent: 'memory_review' as const,
      returned_count: review.audit_metadata.returned_count,
      row_ids: review.audit_metadata.row_ids,
      scope_keys: review.audit_metadata.scope_keys
    })
  });
}

export async function answerVisibleMemoryExplanationCommand(
  store: Pick<TypedMemoryStore, 'listActive'>,
  userId: string,
  text: string,
  query: VisibleExplicitPreferenceRecallQuery = {}
): Promise<VisibleMemoryExplanationCommandResult | null> {
  if (!isVisibleMemoryExplanationCommandText(text)) return null;

  const explanation = await explainVisibleExplicitPreferenceMemoryUse(store, userId, query);
  if (explanation === null) return null;

  return Object.freeze({
    action: 'explain_visible_explicit_preference_use' as const,
    user_id: userId,
    answer: explanation.answer,
    explanation,
    audit_metadata: Object.freeze({
      memory_kind: 'preference' as const,
      matched_intent: 'memory_explanation' as const
    })
  });
}

export async function answerVisibleMemoryForgetCommand(
  store: Pick<TypedMemoryStore, 'listActive' | 'retract'>,
  userId: string,
  text: string,
  query: VisibleExplicitPreferenceRecallQuery
): Promise<VisibleMemoryForgetCommandResult | null> {
  if (!isVisibleMemoryForgetCommandText(text)) return null;

  const forget = await forgetVisibleExplicitPreference(store, userId, query);
  if (forget === null) return null;

  return Object.freeze({
    action: 'forget_visible_explicit_preference' as const,
    user_id: userId,
    answer: forget.message,
    forget,
    audit_metadata: Object.freeze({
      memory_kind: 'preference' as const,
      matched_intent: 'memory_forget' as const,
      scope_key: forget.audit_metadata.scope_key,
      forgotten_row_id: forget.audit_metadata.forgotten_row_id
    })
  });
}

export async function answerVisibleMemoryCorrectCommand(
  store: Pick<TypedMemoryStore, 'listActive' | 'write'>,
  userId: string,
  text: string,
  query: VisibleExplicitPreferenceRecallQuery,
  options: VisiblePreferenceCorrectOptions
): Promise<VisibleMemoryCorrectCommandResult | null> {
  if (!isVisibleMemoryCorrectCommandText(text)) return null;

  const correction = await correctVisibleExplicitPreference(store, userId, query, options);
  if (correction === null) return null;

  return Object.freeze({
    action: 'correct_visible_explicit_preference' as const,
    user_id: userId,
    answer: correction.message,
    correction,
    audit_metadata: Object.freeze({
      memory_kind: 'preference' as const,
      matched_intent: 'memory_correct' as const,
      scope_key: correction.audit_metadata.scope_key,
      previous_row_id: correction.audit_metadata.previous_row_id,
      corrected_row_id: correction.audit_metadata.corrected_row_id,
      source: correction.audit_metadata.source,
      updated_at: correction.audit_metadata.updated_at
    })
  });
}

export async function rememberVisibleExplicitPreference(
  store: Pick<TypedMemoryStore, 'write'>,
  userId: string,
  options: VisiblePreferenceRememberOptions
): Promise<VisiblePreferenceRememberResult> {
  const updatedAt = options.updatedAt ?? new Date().toISOString();
  const source = options.source ?? 'user_stated';
  const confidence = options.confidence ?? 'high';
  const attribute = options.attribute.trim();
  const scopeKey = visiblePreferenceScopeKey(attribute);
  const row = await store.write({
    user_id: userId,
    kind: 'preference',
    scope_key: scopeKey,
    source,
    source_ref: options.sourceRef ?? 'reply:memory-v1-remember-this',
    confidence,
    created_at: updatedAt,
    updated_at: updatedAt,
    stale_marked_at: null,
    retracted: false,
    superseded_by: null,
    attribute,
    value: options.value
  } as NewTypedMemoryRow);
  const label = humanizePreferenceAttribute(attribute);

  return Object.freeze({
    action: 'remembered' as const,
    user_id: userId,
    attribute,
    message: `I remembered that saved ${label} preference. I can use it in future memory recall.`,
    audit_metadata: Object.freeze({
      memory_kind: 'preference' as const,
      scope_key: scopeKey,
      remembered_row_id: row.id,
      source,
      confidence,
      updated_at: updatedAt
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

export async function explainVisibleExplicitPreferenceMemoryUse(
  store: Pick<TypedMemoryStore, 'listActive'>,
  userId: string,
  query: VisibleExplicitPreferenceRecallQuery = {}
): Promise<VisibleMemoryWhyUsedExplanation | null> {
  const recall = await recallVisibleExplicitPreference(store, userId, query);
  if (recall === null) return null;
  return explainVisibleExplicitPreferenceUse(recall);
}

export async function forgetVisibleExplicitPreference(
  store: Pick<TypedMemoryStore, 'listActive' | 'retract'>,
  userId: string,
  query: VisibleExplicitPreferenceRecallQuery
): Promise<VisiblePreferenceForgetResult | null> {
  const recall = await recallVisibleExplicitPreference(store, userId, query);
  if (recall === null) return null;

  const forgotten = await store.retract(userId, 'preference', recall.audit_metadata.scope_key);
  const label = humanizePreferenceAttribute(recall.attribute);
  return Object.freeze({
    action: 'forgot' as const,
    user_id: userId,
    attribute: recall.attribute,
    forgotten,
    message: forgotten
      ? `I forgot that saved ${label} preference. I will not use it in future memory recall.`
      : `I could not find an active saved ${label} preference to forget.`,
    audit_metadata: Object.freeze({
      memory_kind: 'preference' as const,
      scope_key: recall.audit_metadata.scope_key,
      forgotten_row_id: recall.audit_metadata.row_id
    })
  });
}

export async function correctVisibleExplicitPreference(
  store: Pick<TypedMemoryStore, 'listActive' | 'write'>,
  userId: string,
  query: VisibleExplicitPreferenceRecallQuery,
  options: VisiblePreferenceCorrectOptions
): Promise<VisiblePreferenceCorrectResult | null> {
  const recall = await recallVisibleExplicitPreference(store, userId, query);
  if (recall === null) return null;

  const updatedAt = options.updatedAt ?? new Date().toISOString();
  const source = options.source ?? 'user_stated';
  const corrected = {
    user_id: userId,
    kind: 'preference',
    scope_key: recall.audit_metadata.scope_key,
    source,
    source_ref: options.sourceRef ?? 'reply:memory-v1-correction',
    confidence: 'high',
    created_at: updatedAt,
    updated_at: updatedAt,
    stale_marked_at: null,
    retracted: false,
    superseded_by: null,
    attribute: recall.attribute,
    value: options.correctedValue
  } as NewTypedMemoryRow;
  const row = await store.write(corrected);
  const label = humanizePreferenceAttribute(recall.attribute);

  return Object.freeze({
    action: 'corrected' as const,
    user_id: userId,
    attribute: recall.attribute,
    message: `I updated that saved ${label} preference. I will use the corrected version going forward.`,
    audit_metadata: Object.freeze({
      memory_kind: 'preference' as const,
      scope_key: recall.audit_metadata.scope_key,
      previous_row_id: recall.audit_metadata.row_id,
      corrected_row_id: row.id,
      source,
      updated_at: updatedAt
    })
  });
}
