import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { HealthkitSyncAppleInput, HealthkitSyncAppleOutput } from './types.js';

const HEALTHKIT_SYNC_APPLE_RANGE_REQUIRED = 'HEALTHKIT_SYNC_APPLE_RANGE_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'healthkit-sync-apple',
  plane: 'hands',
  requiredScopes: ['healthkit.read'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['sync_steps', 'sync_sleep', 'sync_heart_rate', 'sync_all'] },
      start_date: { type: 'string', pattern: '^\\d{4}-\\d{2}-\\d{2}$' },
      end_date: { type: 'string', pattern: '^\\d{4}-\\d{2}-\\d{2}$' },
      days: { type: 'integer', minimum: 1, maximum: 365 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'snapshots', 'synced_metric_count', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['healthkit-sync-apple'] },
      action: { type: 'string', enum: ['sync_steps', 'sync_sleep', 'sync_heart_rate', 'sync_all'] },
      snapshots: { type: 'array' },
      synced_metric_count: { type: 'integer', minimum: 0 },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'healthkit-sync-apple',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') ||
            HEALTHKIT_SYNC_APPLE_RANGE_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as HealthkitSyncAppleInput);
      const output = OutputSchema.parse(data) as HealthkitSyncAppleOutput;
      return {
        skill_id: 'healthkit-sync-apple',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'healthkit-sync-apple',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'healthkit-sync-apple execution failed',
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
