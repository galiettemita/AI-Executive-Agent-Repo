import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { PollinationsInput, PollinationsOutput } from './types.js';

const POLLINATIONS_PROMPT_REQUIRED = 'POLLINATIONS_PROMPT_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'pollinations',
  plane: 'hands',
  requiredScopes: ['pollinations.generate'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['generate_image', 'generate_video', 'generate_audio'] },
      prompt: { type: 'string', minLength: 2, maxLength: 600 },
      model: { type: 'string', minLength: 2, maxLength: 120 },
      size: { type: 'string', minLength: 2, maxLength: 80 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'asset_url', 'model_used', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['pollinations'] },
      action: { type: 'string', enum: ['generate_image', 'generate_video', 'generate_audio'] },
      asset_url: { type: 'string' },
      model_used: { type: 'string' },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'pollinations',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message: parsed.error.issues.map((issue) => issue.message).join('; ') || POLLINATIONS_PROMPT_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as PollinationsInput);
      const output = OutputSchema.parse(data) as PollinationsOutput;
      return {
        skill_id: 'pollinations',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'pollinations',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'pollinations execution failed',
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
