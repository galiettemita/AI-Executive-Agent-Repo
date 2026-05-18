import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { YahooFinanceInput, YahooFinanceOutput } from './types.js';

const YAHOO_FINANCE_SYMBOLS_REQUIRED = 'YAHOO_FINANCE_SYMBOLS_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'yahoo-finance',
  plane: 'hands',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['quotes', 'fundamentals', 'news'] },
      symbols: { type: 'array' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'disclaimer'],
    properties: {
      provider: { type: 'string', enum: ['yahoo-finance'] },
      action: { type: 'string', enum: ['quotes', 'fundamentals', 'news'] },
      quotes: { type: 'array' },
      fundamentals: { type: 'array' },
      news: { type: 'array' },
      disclaimer: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'yahoo-finance',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') || YAHOO_FINANCE_SYMBOLS_REQUIRED,
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
      const data = await runClient(parsed.data as YahooFinanceInput);
      const output = OutputSchema.parse(data) as YahooFinanceOutput;
      return {
        skill_id: 'yahoo-finance',
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
        skill_id: 'yahoo-finance',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'yahoo-finance execution failed',
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
