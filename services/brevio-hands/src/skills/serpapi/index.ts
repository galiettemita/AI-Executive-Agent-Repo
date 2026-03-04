import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { SerpAPIInput, SerpAPIOutput } from './types.js';

const adapter: ISkillAdapter = {
  id: 'serpapi',
  plane: 'hands',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    required: ['query'],
    properties: {
      query: { type: 'string', minLength: 2, maxLength: 500 },
      engine: { type: 'string', enum: ['google', 'amazon', 'yelp'] },
      max_results: { type: 'integer', minimum: 1, maximum: 20 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'engine', 'results'],
    properties: {
      provider: { type: 'string', enum: ['serpapi'] },
      engine: { type: 'string', enum: ['google', 'amazon', 'yelp'] },
      results: { type: 'array' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'serpapi',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message: parsed.error.issues.map((issue) => issue.message).join('; '),
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: {
          retries: 0,
          circuit_breaker_state: 'CLOSED',
          cache_hit: false
        }
      };
    }

    try {
      const data = await runClient(parsed.data as SerpAPIInput);
      const output = OutputSchema.parse(data) as SerpAPIOutput;
      return {
        skill_id: 'serpapi',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: {
          retries: 0,
          circuit_breaker_state: 'CLOSED',
          cache_hit: false
        }
      };
    } catch (err) {
      return {
        skill_id: 'serpapi',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'serpapi execution failed',
          retryable: true,
          http_status: 502
        },
        latency_ms: Date.now() - started,
        metadata: {
          retries: 0,
          circuit_breaker_state: 'CLOSED',
          cache_hit: false
        }
      };
    }
  },
  async healthCheck(): Promise<boolean> {
    return true;
  }
};

export default adapter;
