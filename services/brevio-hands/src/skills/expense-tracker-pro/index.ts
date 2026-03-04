import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { ExpenseTrackerProInput, ExpenseTrackerProOutput } from './types.js';

const EXPENSE_TRACKER_PRO_ADD_FIELDS_REQUIRED = 'EXPENSE_TRACKER_PRO_ADD_FIELDS_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'expense-tracker-pro',
  plane: 'hands',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['add_expense', 'monthly_summary', 'category_breakdown'] },
      merchant: { type: 'string', minLength: 1, maxLength: 160 },
      amount_cents: { type: 'integer', minimum: 1, maximum: 100000000 },
      category: { type: 'string', minLength: 2, maxLength: 80 },
      occurred_on: { type: 'string', pattern: '^\\d{4}-\\d{2}-\\d{2}$' },
      month: { type: 'string', pattern: '^\\d{4}-\\d{2}$' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'entries', 'totals_by_category', 'total_cents', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['expense-tracker-pro'] },
      action: { type: 'string', enum: ['add_expense', 'monthly_summary', 'category_breakdown'] },
      entries: { type: 'array' },
      totals_by_category: { type: 'object' },
      total_cents: { type: 'integer', minimum: 0 },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'expense-tracker-pro',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') ||
            EXPENSE_TRACKER_PRO_ADD_FIELDS_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as ExpenseTrackerProInput);
      const output = OutputSchema.parse(data) as ExpenseTrackerProOutput;
      return {
        skill_id: 'expense-tracker-pro',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'expense-tracker-pro',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'expense-tracker-pro execution failed',
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
