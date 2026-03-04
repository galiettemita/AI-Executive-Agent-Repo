import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { MorningManifestoInput, MorningManifestoOutput } from './types.js';

const MORNING_MANIFESTO_GOALS_REQUIRED = 'MORNING_MANIFESTO_GOALS_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'morning-manifesto',
  plane: 'brain',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['generate_manifesto', 'sync_actions'] },
      goals: { type: 'array' },
      gratitude: { type: 'array' },
      blockers: { type: 'array' },
      tone: { type: 'string', enum: ['direct', 'supportive'] },
      sync_targets: { type: 'array' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'manifesto', 'affirmations', 'action_items', 'sync_targets'],
    properties: {
      provider: { type: 'string', enum: ['morning-manifesto'] },
      action: { type: 'string', enum: ['generate_manifesto', 'sync_actions'] },
      manifesto: { type: 'string' },
      affirmations: { type: 'array' },
      action_items: { type: 'array' },
      sync_targets: { type: 'array' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'morning-manifesto',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') ||
            MORNING_MANIFESTO_GOALS_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as MorningManifestoInput);
      const output = OutputSchema.parse(data) as MorningManifestoOutput;
      return {
        skill_id: 'morning-manifesto',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'morning-manifesto',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'morning-manifesto execution failed',
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
