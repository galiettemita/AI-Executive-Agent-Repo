import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { SamsungSmartTVInput, SamsungSmartTVOutput } from './types.js';

const SAMSUNG_SMART_TV_APP_REQUIRED = 'SAMSUNG_SMART_TV_APP_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'samsung-smart-tv',
  plane: 'hands',
  requiredScopes: ['r:devices:*', 'x:devices:*'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['power_on', 'power_off', 'launch_app', 'set_volume', 'status'] },
      device_id: { type: 'string', minLength: 3, maxLength: 120 },
      app_id: { type: 'string', minLength: 2, maxLength: 120 },
      volume_pct: { type: 'integer', minimum: 0, maximum: 100 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'device_id', 'power_state', 'current_app', 'volume_pct', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['samsung-smart-tv'] },
      action: { type: 'string', enum: ['power_on', 'power_off', 'launch_app', 'set_volume', 'status'] },
      device_id: { type: 'string' },
      power_state: { type: 'string', enum: ['on', 'off'] },
      current_app: { type: 'string' },
      volume_pct: { type: 'integer', minimum: 0, maximum: 100 },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'samsung-smart-tv',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') || SAMSUNG_SMART_TV_APP_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as SamsungSmartTVInput);
      const output = OutputSchema.parse(data) as SamsungSmartTVOutput;
      return {
        skill_id: 'samsung-smart-tv',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'samsung-smart-tv',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'samsung-smart-tv execution failed',
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
