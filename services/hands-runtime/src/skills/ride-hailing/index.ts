import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { RideHailingInput, RideHailingOutput } from './types.js';

const RIDE_HAILING_CONFIRMATION_REQUIRED = 'RIDE_HAILING_CONFIRMATION_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'ride-hailing',
  plane: 'hands',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['estimate', 'request_ride', 'ride_status', 'cancel_ride'] },
      origin: { type: 'string', minLength: 3, maxLength: 200 },
      destination: { type: 'string', minLength: 3, maxLength: 200 },
      service_tier: { type: 'string', enum: ['economy', 'comfort', 'xl'] },
      ride_id: { type: 'string', minLength: 2, maxLength: 120 },
      confirmed: { type: 'boolean' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'partnership_status'],
    properties: {
      provider: { type: 'string', enum: ['ride-hailing'] },
      action: { type: 'string', enum: ['estimate', 'request_ride', 'ride_status', 'cancel_ride'] },
      estimates: { type: 'array' },
      ride_id: { type: 'string' },
      status: {
        type: 'string',
        enum: ['requested', 'driver_assigned', 'arriving', 'in_progress', 'completed', 'cancelled']
      },
      partnership_status: { type: 'string', enum: ['awaiting_api_partnership'] }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'ride-hailing',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message: parsed.error.issues.map((issue) => issue.message).join('; '),
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    const request = parsed.data as RideHailingInput;

    if (request.action === 'request_ride' && request.confirmed !== true) {
      return {
        skill_id: 'ride-hailing',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message: RIDE_HAILING_CONFIRMATION_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      // CUSTOM_BUILD_REQUIRED: Awaiting API partnership
      const data = await runClient(request);
      const output = OutputSchema.parse(data) as RideHailingOutput;
      return {
        skill_id: 'ride-hailing',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'ride-hailing',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'ride-hailing execution failed',
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
