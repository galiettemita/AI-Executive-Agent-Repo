import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { BuyAnythingInput, BuyAnythingOutput } from './types.js';

const BUY_ANYTHING_ORDER_CONFIRMATION_REQUIRED = 'BUY_ANYTHING_ORDER_CONFIRMATION_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'buy-anything',
  plane: 'hands',
  requiredScopes: ['amazon.checkout.write', 'amazon.orders.read'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['search_product', 'prepare_checkout', 'place_order', 'order_status'] },
      query: { type: 'string', minLength: 2, maxLength: 200 },
      amazon_url: { type: 'string', format: 'uri', pattern: '^https://' },
      quantity: { type: 'integer', minimum: 1, maximum: 50 },
      max_total_cents: { type: 'integer', minimum: 1, maximum: 5000000 },
      shipping_address_id: { type: 'string', minLength: 2, maxLength: 120 },
      line_items: { type: 'array' },
      order_id: { type: 'string', minLength: 2, maxLength: 120 },
      confirmed: { type: 'boolean' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action'],
    properties: {
      provider: { type: 'string', enum: ['buy-anything'] },
      action: { type: 'string', enum: ['search_product', 'prepare_checkout', 'place_order', 'order_status'] },
      product_options: { type: 'array' },
      checkout_preview: { type: 'object' },
      order_id: { type: 'string' },
      order_status: { type: 'string', enum: ['pending', 'confirmed', 'shipped', 'delivered', 'cancelled'] }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'buy-anything',
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

    const request = parsed.data as BuyAnythingInput;
    if (request.action === 'place_order' && request.confirmed !== true) {
      return {
        skill_id: 'buy-anything',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message: BUY_ANYTHING_ORDER_CONFIRMATION_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(request);
      const output = OutputSchema.parse(data) as BuyAnythingOutput;
      return {
        skill_id: 'buy-anything',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'buy-anything',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'buy-anything execution failed',
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
