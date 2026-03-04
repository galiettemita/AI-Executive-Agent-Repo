import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { KidsFamilyManagementInput, KidsFamilyManagementOutput } from './types.js';

const adapter: ISkillAdapter = {
  id: 'kids-family-management',
  plane: 'hands',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['family_schedule', 'pickup_plan', 'location_checkin'] },
      child_name: { type: 'string', minLength: 2, maxLength: 80 },
      date: { type: 'string', pattern: '^\\d{4}-\\d{2}-\\d{2}$' },
      location: { type: 'string', minLength: 2, maxLength: 120 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'partnership_status'],
    properties: {
      provider: { type: 'string', enum: ['kids-family-management'] },
      action: { type: 'string', enum: ['family_schedule', 'pickup_plan', 'location_checkin'] },
      events: { type: 'array' },
      checkin_status: { type: 'string', enum: ['on_time', 'delayed', 'arrived'] },
      partnership_status: { type: 'string', enum: ['awaiting_api_partnership'] }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'kids-family-management',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message: parsed.error.issues.map((issue) => issue.message).join('; '),
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      // CUSTOM_BUILD_REQUIRED: Awaiting API partnership
      const data = await runClient(parsed.data as KidsFamilyManagementInput);
      const output = OutputSchema.parse(data) as KidsFamilyManagementOutput;
      return {
        skill_id: 'kids-family-management',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'kids-family-management',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'kids-family-management execution failed',
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
