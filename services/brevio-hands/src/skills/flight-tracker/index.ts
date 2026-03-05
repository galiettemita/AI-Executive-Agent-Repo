import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { FlightTrackerInput, FlightTrackerOutput } from './types.js';

const FLIGHT_TRACKER_IDENTIFIER_REQUIRED = 'FLIGHT_TRACKER_IDENTIFIER_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'flight-tracker',
  plane: 'hands',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    properties: {
      callsign: { type: 'string', pattern: '^[A-Z0-9]{3,8}$' },
      icao24: { type: 'string', pattern: '^[a-f0-9]{6}$' },
      origin_iata: { type: 'string', pattern: '^[A-Z]{3}$' },
      destination_iata: { type: 'string', pattern: '^[A-Z]{3}$' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'flights', 'queried_at_utc'],
    properties: {
      provider: { type: 'string', enum: ['opensky'] },
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
        skill_id: 'flight-tracker',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            validationMessage || FLIGHT_TRACKER_IDENTIFIER_REQUIRED,
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
      const data = await runClient(parsed.data as FlightTrackerInput);
      const output = OutputSchema.parse(data) as FlightTrackerOutput;
      return {
        skill_id: 'flight-tracker',
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
        skill_id: 'flight-tracker',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'flight-tracker execution failed',
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
