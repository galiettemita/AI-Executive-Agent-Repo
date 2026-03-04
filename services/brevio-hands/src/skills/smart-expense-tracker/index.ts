import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { SmartExpenseTrackerInput, SmartExpenseTrackerOutput } from './types.js';

const SMART_EXPENSE_TRACKER_LOG_FIELDS_REQUIRED = 'SMART_EXPENSE_TRACKER_LOG_FIELDS_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'smart-expense-tracker',
  plane: 'brain',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['log_expense', 'daily_briefing', 'budget_status'] },
      merchant: { type: 'string', minLength: 1, maxLength: 160 },
      amount_cents: { type: 'integer', minimum: 1, maximum: 100000000 },
      category: { type: 'string', minLength: 2, maxLength: 80 },
      occurred_on: { type: 'string', pattern: '^\\d{4}-\\d{2}-\\d{2}$' },
      note: { type: 'string', minLength: 1, maxLength: 400 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'entries', 'today_spend_cents', 'month_spend_cents', 'budget_alerts', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['smart-expense-tracker'] },
      action: { type: 'string', enum: ['log_expense', 'daily_briefing', 'budget_status'] },
      entries: { type: 'array' },
      today_spend_cents: { type: 'integer', minimum: 0 },
      month_spend_cents: { type: 'integer', minimum: 0 },
      budget_alerts: { type: 'array' },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'smart-expense-tracker',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') ||
            SMART_EXPENSE_TRACKER_LOG_FIELDS_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as SmartExpenseTrackerInput);
      const output = OutputSchema.parse(data) as SmartExpenseTrackerOutput;
      return {
        skill_id: 'smart-expense-tracker',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'smart-expense-tracker',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'smart-expense-tracker execution failed',
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
