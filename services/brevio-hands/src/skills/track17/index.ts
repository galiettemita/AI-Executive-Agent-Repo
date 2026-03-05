import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { Track17Input, Track17Output } from './types.js';

const adapter: ISkillAdapter = {
  id: 'track17',
  plane: 'hands',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    required: ['tracking_number'],
    properties: {
      tracking_number: { type: 'string', minLength: 8, maxLength: 40 },
      carrier_code: { type: 'string', minLength: 2, maxLength: 10 },
      request_locale: { type: 'string', minLength: 2, maxLength: 10 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'tracking_number', 'carrier', 'status', 'checkpoints'],
    properties: {
      provider: { type: 'string', enum: ['17track'] },
      tracking_number: { type: 'string' },
      carrier: { type: 'string' },
      status: { type: 'string', enum: ['not_found', 'in_transit', 'out_for_delivery', 'delivered'] },
      checkpoints: { type: 'array' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'track17',
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
      const data = await runClient(parsed.data as Track17Input);
      const output = OutputSchema.parse(data) as Track17Output;
      return {
        skill_id: 'track17',
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
        skill_id: 'track17',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'track17 execution failed',
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
