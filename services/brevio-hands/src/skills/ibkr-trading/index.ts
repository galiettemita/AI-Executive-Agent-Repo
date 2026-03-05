import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { IbkrTradingInput, IbkrTradingOutput } from './types.js';

const IBKR_TRADING_SYMBOL_REQUIRED = 'IBKR_TRADING_SYMBOL_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'ibkr-trading',
  plane: 'hands',
  requiredScopes: ['trading.execute'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['quote_symbol', 'place_order', 'order_status'] },
      symbol: { type: 'string', minLength: 1, maxLength: 12 },
      side: { type: 'string', enum: ['BUY', 'SELL'] },
      quantity: { type: 'integer', minimum: 1, maximum: 1000000 },
      order_id: { type: 'string', minLength: 2, maxLength: 120 },
      confirmed: { type: 'boolean' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'symbol', 'status', 'price_usd', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['ibkr-trading'] },
      action: { type: 'string', enum: ['quote_symbol', 'place_order', 'order_status'] },
      symbol: { type: 'string' },
      order_id: { type: 'string' },
      status: { type: 'string', enum: ['quoted', 'submitted', 'filled'] },
      price_usd: { type: 'number' },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'ibkr-trading',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') || IBKR_TRADING_SYMBOL_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as IbkrTradingInput);
      const output = OutputSchema.parse(data) as IbkrTradingOutput;
      return {
        skill_id: 'ibkr-trading',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'ibkr-trading',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'ibkr-trading execution failed',
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
