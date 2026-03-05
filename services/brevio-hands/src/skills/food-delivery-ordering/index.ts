import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { FoodDeliveryOrderingInput, FoodDeliveryOrderingOutput } from './types.js';

const FOOD_DELIVERY_CHECKOUT_CONFIRMATION_REQUIRED =
  'FOOD_DELIVERY_CHECKOUT_CONFIRMATION_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'food-delivery-ordering',
  plane: 'hands',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['search_restaurants', 'build_cart', 'checkout', 'order_status'] },
      address: { type: 'string', minLength: 5, maxLength: 200 },
      cuisine: { type: 'string', minLength: 2, maxLength: 80 },
      restaurant_id: { type: 'string', minLength: 2, maxLength: 120 },
      items: { type: 'array' },
      cart_id: { type: 'string', minLength: 2, maxLength: 120 },
      order_id: { type: 'string', minLength: 2, maxLength: 120 },
      confirmed: { type: 'boolean' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'partnership_status'],
    properties: {
      provider: { type: 'string', enum: ['food-delivery-ordering'] },
      action: { type: 'string', enum: ['search_restaurants', 'build_cart', 'checkout', 'order_status'] },
      restaurants: { type: 'array' },
      cart_id: { type: 'string' },
      order_id: { type: 'string' },
      status: { type: 'string', enum: ['pending', 'confirmed', 'delivered', 'cancelled'] },
      estimated_total_cents: { type: 'integer' },
      partnership_status: { type: 'string', enum: ['awaiting_api_partnership'] }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'food-delivery-ordering',
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

    const request = parsed.data as FoodDeliveryOrderingInput;

    if (request.action === 'checkout' && request.confirmed !== true) {
      return {
        skill_id: 'food-delivery-ordering',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message: FOOD_DELIVERY_CHECKOUT_CONFIRMATION_REQUIRED,
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
      const output = OutputSchema.parse(data) as FoodDeliveryOrderingOutput;
      return {
        skill_id: 'food-delivery-ordering',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'food-delivery-ordering',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'food-delivery-ordering execution failed',
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
