import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { PlaidInput, PlaidOutput } from './types.js';

const adapter: ISkillAdapter = {
  id: 'plaid',
  plane: 'hands',
  requiredScopes: ['transactions', 'balance', 'identity'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['accounts', 'transactions', 'balance'] },
      account_id: { type: 'string' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action'],
    properties: {
      provider: { type: 'string', enum: ['plaid'] },
      action: { type: 'string', enum: ['accounts', 'transactions', 'balance'] },
      accounts: { type: 'array' },
      transactions: { type: 'array' },
      balances: { type: 'array' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'plaid',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message: parsed.error.issues.map((issue) => issue.message).join('; '),
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
      const data = await runClient(parsed.data as PlaidInput);
      const output = OutputSchema.parse(data) as PlaidOutput;
      return {
        skill_id: 'plaid',
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
      const missingAccount = err instanceof Error && err.message === 'PLAID_ACCOUNT_NOT_FOUND';
      return {
        skill_id: 'plaid',
        status: 'FAILED',
        error: {
          code: missingAccount ? 'VALIDATION_FAILED' : 'EXTERNAL_ERROR',
          message: missingAccount
            ? 'account_id does not match linked Plaid accounts.'
            : err instanceof Error
              ? err.message
              : 'plaid execution failed',
          retryable: !missingAccount,
          http_status: missingAccount ? 404 : 502
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
