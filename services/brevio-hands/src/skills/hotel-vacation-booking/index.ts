import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { HotelVacationBookingInput, HotelVacationBookingOutput } from './types.js';

const HOTEL_BOOKING_CONFIRMATION_REQUIRED = 'HOTEL_BOOKING_CONFIRMATION_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'hotel-vacation-booking',
  plane: 'hands',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['search_hotels', 'hold_room', 'book_room', 'reservation_status'] },
      city: { type: 'string', minLength: 2, maxLength: 120 },
      check_in: { type: 'string', pattern: '^\\d{4}-\\d{2}-\\d{2}$' },
      check_out: { type: 'string', pattern: '^\\d{4}-\\d{2}-\\d{2}$' },
      guests: { type: 'integer', minimum: 1, maximum: 20 },
      hotel_id: { type: 'string', minLength: 2, maxLength: 120 },
      hold_id: { type: 'string', minLength: 2, maxLength: 120 },
      reservation_id: { type: 'string', minLength: 2, maxLength: 120 },
      confirmed: { type: 'boolean' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'partnership_status'],
    properties: {
      provider: { type: 'string', enum: ['hotel-vacation-booking'] },
      action: { type: 'string', enum: ['search_hotels', 'hold_room', 'book_room', 'reservation_status'] },
      hotels: { type: 'array' },
      hold_id: { type: 'string' },
      reservation_id: { type: 'string' },
      status: { type: 'string', enum: ['pending', 'confirmed', 'cancelled'] },
      partnership_status: { type: 'string', enum: ['awaiting_api_partnership'] }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'hotel-vacation-booking',
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

    const request = parsed.data as HotelVacationBookingInput;

    if (request.action === 'book_room' && request.confirmed !== true) {
      return {
        skill_id: 'hotel-vacation-booking',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message: HOTEL_BOOKING_CONFIRMATION_REQUIRED,
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
      const output = OutputSchema.parse(data) as HotelVacationBookingOutput;
      return {
        skill_id: 'hotel-vacation-booking',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'hotel-vacation-booking',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'hotel-vacation-booking execution failed',
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
