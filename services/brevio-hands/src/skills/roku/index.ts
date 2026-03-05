import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { RokuInput, RokuOutput } from './types.js';

const ROKU_ACTION_FIELDS_REQUIRED = 'ROKU_ACTION_FIELDS_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'roku',
  plane: 'hands',
  requiredScopes: ['roku.control'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['launch_app', 'key_press', 'status'] },
      device_id: { type: 'string', minLength: 2, maxLength: 120 },
      app_id: { type: 'string', minLength: 1, maxLength: 120 },
      key: { type: 'string', minLength: 1, maxLength: 40 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'device_id', 'current_app', 'power_state', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['roku'] },
      action: { type: 'string', enum: ['launch_app', 'key_press', 'status'] },
      device_id: { type: 'string' },
      current_app: { type: 'string' },
      power_state: { type: 'string', enum: ['on', 'off'] },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'roku',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message: parsed.error.issues.map((issue) => issue.message).join('; ') || ROKU_ACTION_FIELDS_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as RokuInput);
      const output = OutputSchema.parse(data) as RokuOutput;
      return {
        skill_id: 'roku',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'roku',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'roku execution failed',
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
