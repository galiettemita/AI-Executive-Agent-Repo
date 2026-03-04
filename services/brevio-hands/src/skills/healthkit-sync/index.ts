import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { HealthkitSyncInput, HealthkitSyncOutput } from './types.js';

const HEALTHKIT_SYNC_ALIAS_RANGE_REQUIRED = 'HEALTHKIT_SYNC_ALIAS_RANGE_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'healthkit-sync',
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
    required: ['provider', 'action', 'alias_target', 'deprecated_alias', 'forwarded', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['healthkit-sync'] },
      action: { type: 'string', enum: ['sync_steps', 'sync_sleep', 'sync_heart_rate', 'sync_all'] },
      alias_target: { type: 'string', enum: ['healthkit-sync-apple'] },
      deprecated_alias: { type: 'boolean', enum: [true] },
      forwarded: { type: 'boolean' },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'healthkit-sync',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') ||
            HEALTHKIT_SYNC_ALIAS_RANGE_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as HealthkitSyncInput);
      const output = OutputSchema.parse(data) as HealthkitSyncOutput;
      return {
        skill_id: 'healthkit-sync',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'healthkit-sync',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'healthkit-sync execution failed',
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
