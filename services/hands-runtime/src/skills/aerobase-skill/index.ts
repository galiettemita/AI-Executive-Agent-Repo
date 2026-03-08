import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { AerobaseInput, AerobaseOutput } from './types.js';

const AEROBASE_ROUTE_REQUIRED = 'AEROBASE_ROUTE_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'aerobase-skill',
  plane: 'hands',
  requiredScopes: ['travel.search'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['search_flights', 'compare_itineraries'] },
      origin: { type: 'string', minLength: 3, maxLength: 3 },
      destination: { type: 'string', minLength: 3, maxLength: 3 },
      depart_date: { type: 'string', format: 'date' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'itineraries', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['aerobase-skill'] },
      action: { type: 'string', enum: ['search_flights', 'compare_itineraries'] },
      itineraries: { type: 'array', items: { type: 'object' } },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'aerobase-skill',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message: parsed.error.issues.map((issue) => issue.message).join('; ') || AEROBASE_ROUTE_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as AerobaseInput);
      const output = OutputSchema.parse(data) as AerobaseOutput;
      return {
        skill_id: 'aerobase-skill',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'aerobase-skill',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'aerobase-skill execution failed',
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
