import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { GammaInput, GammaOutput } from './types.js';

const GAMMA_TOPIC_REQUIRED = 'GAMMA_TOPIC_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'gamma',
  plane: 'hands',
  requiredScopes: ['gamma.deck.write'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['create_deck', 'update_deck', 'export_deck'] },
      topic: { type: 'string', minLength: 2, maxLength: 240 },
      deck_id: { type: 'string', minLength: 3, maxLength: 120 },
      slide_count: { type: 'integer', minimum: 1, maximum: 80 },
      format: { type: 'string', enum: ['pdf', 'pptx'] }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'deck_id', 'title', 'slide_count', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['gamma'] },
      action: { type: 'string', enum: ['create_deck', 'update_deck', 'export_deck'] },
      deck_id: { type: 'string' },
      title: { type: 'string' },
      slide_count: { type: 'integer', minimum: 1 },
      export_url: { type: 'string' },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'gamma',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message: parsed.error.issues.map((issue) => issue.message).join('; ') || GAMMA_TOPIC_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as GammaInput);
      const output = OutputSchema.parse(data) as GammaOutput;
      return {
        skill_id: 'gamma',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'gamma',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'gamma execution failed',
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
