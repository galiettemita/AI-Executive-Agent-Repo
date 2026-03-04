import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { MarketplaceInput, MarketplaceOutput } from './types.js';

const MARKETPLACE_TITLE_REQUIRED = 'MARKETPLACE_TITLE_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'marketplace',
  plane: 'hands',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['evaluate_listing', 'compare_prices', 'draft_listing'] },
      title: { type: 'string', minLength: 2, maxLength: 200 },
      listing_url: { type: 'string', format: 'uri', pattern: '^https://' },
      condition: { type: 'string', enum: ['new', 'like_new', 'good', 'fair'] },
      asking_price_cents: { type: 'integer', minimum: 1, maximum: 5000000 },
      comparable_prices_cents: { type: 'array' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'fair_price_cents', 'confidence', 'scam_risk', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['marketplace'] },
      action: { type: 'string', enum: ['evaluate_listing', 'compare_prices', 'draft_listing'] },
      fair_price_cents: { type: 'integer', minimum: 1 },
      confidence: { type: 'number', minimum: 0, maximum: 1 },
      scam_risk: { type: 'string', enum: ['low', 'medium', 'high'] },
      summary: { type: 'string' },
      draft_listing_copy: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'marketplace',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') || MARKETPLACE_TITLE_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as MarketplaceInput);
      const output = OutputSchema.parse(data) as MarketplaceOutput;
      return {
        skill_id: 'marketplace',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'marketplace',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'marketplace execution failed',
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
