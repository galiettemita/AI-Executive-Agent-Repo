// Cost Tracking — per-model-call accounting that the Model Router writes
// after every backend invocation. Lets the system attribute spend per user,
// per model, per period; flags schema-invalid outputs as separate cost
// because they consumed tokens without producing useful output.
//
// FOMO_PLAN §9.9. Phase 2D ships substrate only — no caller yet beyond the
// Model Router's own writes. A future cost-digest worker (Phase 3+) will
// aggregate periods and alert when spend crosses a budget.
//
// MODEL_PRICING is the v0.1 reference table for known models. Real backends
// can compute their own cost and pass estimated_cost_usd directly; the
// helper is here for the mock backend and any caller that wants to back
// into a number from token counts.

export type CapabilityTag = 'classification';

export const CAPABILITY_TAGS: readonly CapabilityTag[] = Object.freeze(['classification']);

export function isCapabilityTag(value: unknown): value is CapabilityTag {
  return typeof value === 'string' && (CAPABILITY_TAGS as readonly string[]).includes(value);
}

export interface CostRecord {
  readonly id?: number;
  // ISO 8601.
  readonly occurred_at: string;
  readonly user_id: string;
  readonly capability: CapabilityTag;
  readonly model_name: string;
  readonly prompt_version: string;
  readonly latency_ms: number;
  readonly input_tokens: number;
  readonly output_tokens: number;
  readonly estimated_cost_usd: number;
  // false when the backend produced output but it failed schema validation —
  // we still consumed tokens, so the cost record exists, but the call is
  // attributed as wasted spend.
  readonly schema_valid: boolean;
}

export interface CostRecordInput {
  user_id: string;
  capability: CapabilityTag;
  model_name: string;
  prompt_version: string;
  latency_ms: number;
  input_tokens: number;
  output_tokens: number;
  estimated_cost_usd: number;
  schema_valid: boolean;
  occurred_at?: string;
}

export interface CostStore {
  write(input: CostRecordInput): Promise<void>;
  recent(userId: string, limit?: number): Promise<readonly CostRecord[]>;
  sumByModel(userId: string, modelName: string): Promise<number>;
  sumByPeriod(userId: string, fromIso: string, toIso: string): Promise<number>;
}

export class InMemoryCostStore implements CostStore {
  private records: CostRecord[] = [];
  private nextId = 1;
  private readonly capacity: number;

  constructor(capacity = 50_000) {
    this.capacity = capacity;
  }

  async write(input: CostRecordInput): Promise<void> {
    this.records.push(
      Object.freeze({
        id: this.nextId++,
        occurred_at: input.occurred_at ?? new Date().toISOString(),
        user_id: input.user_id,
        capability: input.capability,
        model_name: input.model_name,
        prompt_version: input.prompt_version,
        latency_ms: input.latency_ms,
        input_tokens: input.input_tokens,
        output_tokens: input.output_tokens,
        estimated_cost_usd: input.estimated_cost_usd,
        schema_valid: input.schema_valid
      })
    );
    if (this.records.length > this.capacity) {
      this.records.splice(0, this.records.length - this.capacity);
    }
  }

  async recent(userId: string, limit = 100): Promise<readonly CostRecord[]> {
    const filtered = this.records.filter((r) => r.user_id === userId);
    return filtered.slice(-limit).reverse();
  }

  async sumByModel(userId: string, modelName: string): Promise<number> {
    let sum = 0;
    for (const r of this.records) {
      if (r.user_id === userId && r.model_name === modelName) sum += r.estimated_cost_usd;
    }
    return sum;
  }

  async sumByPeriod(userId: string, fromIso: string, toIso: string): Promise<number> {
    let sum = 0;
    for (const r of this.records) {
      if (r.user_id !== userId) continue;
      if (r.occurred_at < fromIso) continue;
      if (r.occurred_at > toIso) continue;
      sum += r.estimated_cost_usd;
    }
    return sum;
  }
}

// Cost per 1M tokens for known model names. The Mock backend uses two
// fake models so tests can exercise multi-model accounting. Real backends
// in Phase 3+ should compute cost from the provider's actual pricing and
// pass estimated_cost_usd directly to the cost store; this table is only a
// fallback / mock helper.
// Per-1M-token USD pricing. Mock entries seed the Phase 2D tests; the
// Anthropic entries land in Phase 3C.1 alongside the AnthropicBackend.
//
// Anthropic pricing (Phase 3C.1, list price as of model release). These
// are approximate and SHOULD be re-verified before any production
// billing rollout — Anthropic publishes the authoritative numbers at
// https://www.anthropic.com/pricing. The Phase 3C.2 bake-off uses these
// to compute cost-per-1k-emails for the founder demo.
export const MODEL_PRICING: Readonly<Record<string, { readonly input_per_1m_usd: number; readonly output_per_1m_usd: number }>> =
  Object.freeze({
    'mock-classifier-tiny': Object.freeze({ input_per_1m_usd: 0.10, output_per_1m_usd: 0.40 }),
    'mock-classifier-small': Object.freeze({ input_per_1m_usd: 0.30, output_per_1m_usd: 1.20 }),
    // Anthropic Haiku 4.5 — small/fast tier.
    'claude-haiku-4-5-20251001': Object.freeze({ input_per_1m_usd: 1.00, output_per_1m_usd: 5.00 }),
    // Anthropic Sonnet 4.6 — frontier tier.
    'claude-sonnet-4-6': Object.freeze({ input_per_1m_usd: 3.00, output_per_1m_usd: 15.00 }),
    // OpenAI GPT-5 family — list-price approximations as of model
    // release. Founder picked gpt-5-mini as the initial Brevio ranker
    // in Phase 3C.2; nano + frontier rows are included so a future
    // model swap via FOMO_OPENAI_MODEL doesn't return 0 cost. Re-verify
    // at https://openai.com/api/pricing/ before any production billing
    // rollout.
    'gpt-5-mini': Object.freeze({ input_per_1m_usd: 0.25, output_per_1m_usd: 2.00 }),
    'gpt-5-nano': Object.freeze({ input_per_1m_usd: 0.05, output_per_1m_usd: 0.40 }),
    'gpt-5': Object.freeze({ input_per_1m_usd: 1.25, output_per_1m_usd: 10.00 }),
    // OpenAI prior-gen mini, kept as a safe fallback if gpt-5-mini is
    // not yet available on the founder's account.
    'gpt-4o-mini': Object.freeze({ input_per_1m_usd: 0.15, output_per_1m_usd: 0.60 })
  });

export function computeEstimatedCost(modelName: string, inputTokens: number, outputTokens: number): number {
  const pricing = MODEL_PRICING[modelName];
  if (!pricing) return 0;
  return (
    (inputTokens / 1_000_000) * pricing.input_per_1m_usd +
    (outputTokens / 1_000_000) * pricing.output_per_1m_usd
  );
}
