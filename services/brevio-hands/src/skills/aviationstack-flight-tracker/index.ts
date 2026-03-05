import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type {
  AviationstackFlightTrackerInput,
  AviationstackFlightTrackerOutput
} from './types.js';

const AVIATIONSTACK_FLIGHT_IDENTIFIER_REQUIRED = 'AVIATIONSTACK_FLIGHT_IDENTIFIER_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'aviationstack-flight-tracker',
  plane: 'hands',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    properties: {
      flight_iata: { type: 'string', pattern: '^[A-Z0-9]{2,8}$' },
      flight_icao: { type: 'string', pattern: '^[A-Z0-9]{3,8}$' },
      airline_iata: { type: 'string', pattern: '^[A-Z0-9]{2,3}$' },
      date: { type: 'string', pattern: '^\\d{4}-\\d{2}-\\d{2}$' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'flights', 'queried_at_utc'],
    properties: {
      provider: { type: 'string', enum: ['aviationstack'] },
      flights: { type: 'array' },
      queried_at_utc: { type: 'string', format: 'date-time' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      const validationMessage = parsed.error.issues.map((issue) => issue.message).join('; ');
      return {
        skill_id: 'aviationstack-flight-tracker',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            validationMessage || AVIATIONSTACK_FLIGHT_IDENTIFIER_REQUIRED,
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
      const data = await runClient(parsed.data as AviationstackFlightTrackerInput);
      const output = OutputSchema.parse(data) as AviationstackFlightTrackerOutput;
      return {
        skill_id: 'aviationstack-flight-tracker',
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
        skill_id: 'aviationstack-flight-tracker',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message:
            err instanceof Error ? err.message : 'aviationstack-flight-tracker execution failed',
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
