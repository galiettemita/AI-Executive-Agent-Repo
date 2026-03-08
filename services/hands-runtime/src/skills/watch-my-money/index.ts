import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { WatchMyMoneyInput, WatchMyMoneyOutput } from './types.js';

const WATCH_MY_MONEY_TRANSACTIONS_REQUIRED = 'WATCH_MY_MONEY_TRANSACTIONS_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'watch-my-money',
  plane: 'hands',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    required: ['action', 'transactions', 'monthly_income_cents'],
    properties: {
      action: { type: 'string', enum: ['analyze_statement', 'budget_alerts'] },
      monthly_income_cents: { type: 'integer', minimum: 1, maximum: 1000000000 },
      transactions: { type: 'array' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'category_totals_cents', 'spend_rate_pct_of_income', 'alerts', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['watch-my-money'] },
      action: { type: 'string', enum: ['analyze_statement', 'budget_alerts'] },
      category_totals_cents: { type: 'object' },
      spend_rate_pct_of_income: { type: 'number', minimum: 0 },
      alerts: { type: 'array' },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'watch-my-money',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') ||
            WATCH_MY_MONEY_TRANSACTIONS_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as WatchMyMoneyInput);
      const output = OutputSchema.parse(data) as WatchMyMoneyOutput;
      return {
        skill_id: 'watch-my-money',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'watch-my-money',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'watch-my-money execution failed',
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
