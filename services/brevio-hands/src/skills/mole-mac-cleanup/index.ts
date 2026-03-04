import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { MoleMacCleanupInput, MoleMacCleanupOutput } from './types.js';

const MOLE_MAC_CLEANUP_CONFIRMATION_REQUIRED = 'MOLE_MAC_CLEANUP_CONFIRMATION_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'mole-mac-cleanup',
  plane: 'hands',
  requiredScopes: ['macos.cleanup'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['scan_cleanup', 'run_cleanup'] },
      mode: { type: 'string', enum: ['quick', 'deep'] },
      confirmed: { type: 'boolean' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'reclaimable_mb', 'categories', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['mole-mac-cleanup'] },
      action: { type: 'string', enum: ['scan_cleanup', 'run_cleanup'] },
      reclaimable_mb: { type: 'integer', minimum: 0 },
      cleaned_mb: { type: 'integer', minimum: 0 },
      categories: { type: 'array', items: { type: 'string' } },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'mole-mac-cleanup',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') || MOLE_MAC_CLEANUP_CONFIRMATION_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as MoleMacCleanupInput);
      const output = OutputSchema.parse(data) as MoleMacCleanupOutput;
      return {
        skill_id: 'mole-mac-cleanup',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'mole-mac-cleanup',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'mole-mac-cleanup execution failed',
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
