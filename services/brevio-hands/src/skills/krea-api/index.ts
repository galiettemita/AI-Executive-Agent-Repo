import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { KreaApiInput, KreaApiOutput } from './types.js';

const KREA_API_PROMPT_REQUIRED = 'KREA_API_PROMPT_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'krea-api',
  plane: 'hands',
  requiredScopes: ['krea.generate'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['generate_image', 'upscale_image', 'list_models'] },
      prompt: { type: 'string', minLength: 2, maxLength: 600 },
      image_url: { type: 'string', format: 'uri' },
      model: { type: 'string', minLength: 2, maxLength: 120 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'image_url', 'model', 'quality_score', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['krea-api'] },
      action: { type: 'string', enum: ['generate_image', 'upscale_image', 'list_models'] },
      image_url: { type: 'string' },
      model: { type: 'string' },
      quality_score: { type: 'number', minimum: 0, maximum: 1 },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'krea-api',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message: parsed.error.issues.map((issue) => issue.message).join('; ') || KREA_API_PROMPT_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as KreaApiInput);
      const output = OutputSchema.parse(data) as KreaApiOutput;
      return {
        skill_id: 'krea-api',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'krea-api',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'krea-api execution failed',
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
