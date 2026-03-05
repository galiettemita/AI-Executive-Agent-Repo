import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { YNABInput, YNABOutput } from './types.js';

const adapter: ISkillAdapter = {
  id: 'ynab',
  plane: 'hands',
  requiredScopes: ['read-only'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['summary', 'accounts', 'transactions'] },
      budget_id: { type: 'string' },
      account_id: { type: 'string' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'budget_id'],
    properties: {
      provider: { type: 'string', enum: ['ynab'] },
      action: { type: 'string', enum: ['summary', 'accounts', 'transactions'] },
      budget_id: { type: 'string' },
      total_budget_cents: { type: 'integer' },
      accounts: { type: 'array' },
      transactions: { type: 'array' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'ynab',
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
      const data = await runClient(parsed.data as YNABInput);
      const output = OutputSchema.parse(data) as YNABOutput;
      return {
        skill_id: 'ynab',
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
      const accountMissing = err instanceof Error && err.message === 'YNAB_ACCOUNT_NOT_FOUND';
      return {
        skill_id: 'ynab',
        status: 'FAILED',
        error: {
          code: accountMissing ? 'VALIDATION_FAILED' : 'EXTERNAL_ERROR',
          message: accountMissing
            ? 'Requested YNAB account_id was not found.'
            : err instanceof Error
              ? err.message
              : 'ynab execution failed',
          retryable: !accountMissing,
          http_status: accountMissing ? 404 : 502
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
