import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { GoogleMapsInput, GoogleMapsOutput } from './types.js';

const adapter: ISkillAdapter = {
  id: 'google-maps',
  plane: 'hands',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    required: ['origin', 'destination'],
    properties: {
      origin: { type: 'string', minLength: 2, maxLength: 200 },
      destination: { type: 'string', minLength: 2, maxLength: 200 },
      mode: { type: 'string', enum: ['driving', 'walking', 'bicycling', 'transit'] }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['distance_m', 'duration_s', 'mode', 'steps'],
    properties: {
      distance_m: { type: 'integer' },
      duration_s: { type: 'integer' },
      mode: { type: 'string', enum: ['driving', 'walking', 'bicycling', 'transit'] },
      steps: {
        type: 'array',
        items: {
          type: 'object',
          required: ['instruction', 'distance_m'],
          properties: {
            instruction: { type: 'string' },
            distance_m: { type: 'integer' }
          },
          additionalProperties: false
        }
      }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'google-maps',
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
      const data = await runClient(parsed.data as GoogleMapsInput);
      const output = OutputSchema.parse(data) as GoogleMapsOutput;
      return {
        skill_id: 'google-maps',
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
        skill_id: 'google-maps',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'google-maps execution failed',
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
