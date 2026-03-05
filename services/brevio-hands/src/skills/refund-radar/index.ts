import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { RefundRadarInput, RefundRadarOutput } from './types.js';

const REFUND_RADAR_DRAFT_FIELDS_REQUIRED = 'REFUND_RADAR_DRAFT_FIELDS_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'refund-radar',
  plane: 'brain',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['scan_recurring_charges', 'draft_refund_request'] },
      merchant: { type: 'string', minLength: 1, maxLength: 160 },
      amount_cents: { type: 'integer', minimum: 1, maximum: 100000000 },
      reason: { type: 'string', minLength: 2, maxLength: 400 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'flagged_charges', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['refund-radar'] },
      action: { type: 'string', enum: ['scan_recurring_charges', 'draft_refund_request'] },
      flagged_charges: { type: 'array' },
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
        skill_id: 'refund-radar',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') ||
            REFUND_RADAR_DRAFT_FIELDS_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as RefundRadarInput);
      const output = OutputSchema.parse(data) as RefundRadarOutput;
      return {
        skill_id: 'refund-radar',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'refund-radar',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'refund-radar execution failed',
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
