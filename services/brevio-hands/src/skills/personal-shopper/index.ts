import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { PersonalShopperInput, PersonalShopperOutput } from './types.js';

const PERSONAL_SHOPPER_QUERY_REQUIRED = 'PERSONAL_SHOPPER_QUERY_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'personal-shopper',
  plane: 'brain',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['research_product', 'rank_options', 'purchase_plan'] },
      query: { type: 'string', minLength: 2, maxLength: 200 },
      budget_cents: { type: 'integer', minimum: 1, maximum: 10000000 },
      constraints: { type: 'array' },
      candidates: { type: 'array' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'summary', 'ranked_candidates', 'recommendation'],
    properties: {
      provider: { type: 'string', enum: ['personal-shopper'] },
      action: { type: 'string', enum: ['research_product', 'rank_options', 'purchase_plan'] },
      summary: { type: 'string' },
      ranked_candidates: { type: 'array' },
      recommendation: { type: 'string' },
      purchase_steps: { type: 'array' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'personal-shopper',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') ||
            PERSONAL_SHOPPER_QUERY_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as PersonalShopperInput);
      const output = OutputSchema.parse(data) as PersonalShopperOutput;
      return {
        skill_id: 'personal-shopper',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'personal-shopper',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'personal-shopper execution failed',
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
