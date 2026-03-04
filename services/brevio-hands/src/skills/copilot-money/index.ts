import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { CopilotMoneyInput, CopilotMoneyOutput } from './types.js';

const COPILOT_MONEY_ACCOUNT_REQUIRED = 'COPILOT_MONEY_ACCOUNT_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'copilot-money',
  plane: 'hands',
  requiredScopes: ['copilot.accounts.read', 'copilot.transactions.read'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['accounts', 'transactions', 'net_worth'] },
      account_id: { type: 'string', minLength: 2, maxLength: 100 },
      from_date: { type: 'string', pattern: '^\\d{4}-\\d{2}-\\d{2}$' },
      to_date: { type: 'string', pattern: '^\\d{4}-\\d{2}-\\d{2}$' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action'],
    properties: {
      provider: { type: 'string', enum: ['copilot-money'] },
      action: { type: 'string', enum: ['accounts', 'transactions', 'net_worth'] },
      accounts: { type: 'array' },
      transactions: { type: 'array' },
      net_worth_cents: { type: 'integer' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'copilot-money',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') || COPILOT_MONEY_ACCOUNT_REQUIRED,
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
      const data = await runClient(parsed.data as CopilotMoneyInput);
      const output = OutputSchema.parse(data) as CopilotMoneyOutput;
      return {
        skill_id: 'copilot-money',
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
        skill_id: 'copilot-money',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'copilot-money execution failed',
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
