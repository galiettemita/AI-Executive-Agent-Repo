import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { JustFuckingCancelInput, JustFuckingCancelOutput } from './types.js';

const JUST_FUCKING_CANCEL_INPUT_REQUIRED = 'JUST_FUCKING_CANCEL_INPUT_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'just-fucking-cancel',
  plane: 'hands',
  requiredScopes: ['finance.read'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['scan_subscriptions', 'draft_cancellation'] },
      transactions_csv: { type: 'string', minLength: 10, maxLength: 200000 },
      merchant_name: { type: 'string', minLength: 2, maxLength: 200 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'findings', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['just-fucking-cancel'] },
      action: { type: 'string', enum: ['scan_subscriptions', 'draft_cancellation'] },
      findings: { type: 'array', items: { type: 'object' } },
      draft_message: { type: 'string' },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'just-fucking-cancel',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') || JUST_FUCKING_CANCEL_INPUT_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as JustFuckingCancelInput);
      const output = OutputSchema.parse(data) as JustFuckingCancelOutput;
      return {
        skill_id: 'just-fucking-cancel',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'just-fucking-cancel',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'just-fucking-cancel execution failed',
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
