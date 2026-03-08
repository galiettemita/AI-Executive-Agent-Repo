import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { LocalServiceBookingInput, LocalServiceBookingOutput } from './types.js';

const LOCAL_SERVICE_BOOKING_CONFIRMATION_REQUIRED =
  'LOCAL_SERVICE_BOOKING_CONFIRMATION_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'local-service-booking',
  plane: 'hands',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['search_providers', 'request_quote', 'book_service', 'booking_status'] },
      service_type: { type: 'string', minLength: 2, maxLength: 120 },
      zip_code: { type: 'string', pattern: '^\\d{5}$' },
      provider_id: { type: 'string', minLength: 2, maxLength: 120 },
      booking_id: { type: 'string', minLength: 2, maxLength: 120 },
      preferred_time: { type: 'string', minLength: 2, maxLength: 80 },
      confirmed: { type: 'boolean' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'partnership_status'],
    properties: {
      provider: { type: 'string', enum: ['local-service-booking'] },
      action: {
        type: 'string',
        enum: ['search_providers', 'request_quote', 'book_service', 'booking_status']
      },
      providers: { type: 'array' },
      booking_id: { type: 'string' },
      status: { type: 'string', enum: ['quote_pending', 'scheduled', 'completed', 'cancelled'] },
      partnership_status: { type: 'string', enum: ['awaiting_api_partnership'] }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'local-service-booking',
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

    const request = parsed.data as LocalServiceBookingInput;

    if (request.action === 'book_service' && request.confirmed !== true) {
      return {
        skill_id: 'local-service-booking',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message: LOCAL_SERVICE_BOOKING_CONFIRMATION_REQUIRED,
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
      const output = OutputSchema.parse(data) as LocalServiceBookingOutput;
      return {
        skill_id: 'local-service-booking',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'local-service-booking',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'local-service-booking execution failed',
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
