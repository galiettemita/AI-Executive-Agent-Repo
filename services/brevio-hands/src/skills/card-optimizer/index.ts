import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { CardOptimizerInput, CardOptimizerOutput } from './types.js';

const CARD_OPTIMIZER_CATEGORY_REQUIRED = 'CARD_OPTIMIZER_CATEGORY_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'card-optimizer',
  plane: 'brain',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    required: ['action', 'purchase_category', 'amount_cents'],
    properties: {
      action: { type: 'string', enum: ['recommend_card', 'category_strategy'] },
      purchase_category: { type: 'string', minLength: 2, maxLength: 80 },
      amount_cents: { type: 'integer', minimum: 1, maximum: 100000000 },
      available_cards: { type: 'array' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'recommended_card', 'estimated_reward_cents', 'alternatives', 'rationale'],
    properties: {
      provider: { type: 'string', enum: ['card-optimizer'] },
      action: { type: 'string', enum: ['recommend_card', 'category_strategy'] },
      recommended_card: { type: 'string' },
      estimated_reward_cents: { type: 'integer', minimum: 0 },
      alternatives: { type: 'array' },
      rationale: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'card-optimizer',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') || CARD_OPTIMIZER_CATEGORY_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as CardOptimizerInput);
      const output = OutputSchema.parse(data) as CardOptimizerOutput;
      return {
        skill_id: 'card-optimizer',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'card-optimizer',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'card-optimizer execution failed',
          retryable: true,
          http_status: 502
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }
  },
  async healthCheck(): Promise<boolean> {
    return true;
  }
};

export default adapter;
