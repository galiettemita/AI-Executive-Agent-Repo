import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { ShortcutsGeneratorInput, ShortcutsGeneratorOutput } from './types.js';

const SHORTCUTS_GENERATOR_STEPS_REQUIRED = 'SHORTCUTS_GENERATOR_STEPS_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'shortcuts-generator',
  plane: 'hands',
  requiredScopes: ['shortcuts.local.write'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['generate_shortcut', 'list_shortcuts', 'install_shortcut'] },
      shortcut_name: { type: 'string', minLength: 2, maxLength: 180 },
      description: { type: 'string', minLength: 2, maxLength: 500 },
      steps: { type: 'array', items: { type: 'string' }, minItems: 1, maxItems: 30 },
      shortcut_id: { type: 'string', minLength: 3, maxLength: 120 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'shortcuts', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['shortcuts-generator'] },
      action: { type: 'string', enum: ['generate_shortcut', 'list_shortcuts', 'install_shortcut'] },
      shortcuts: { type: 'array', items: { type: 'object' } },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'shortcuts-generator',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') || SHORTCUTS_GENERATOR_STEPS_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as ShortcutsGeneratorInput);
      const output = OutputSchema.parse(data) as ShortcutsGeneratorOutput;
      return {
        skill_id: 'shortcuts-generator',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'shortcuts-generator',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'shortcuts-generator execution failed',
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
