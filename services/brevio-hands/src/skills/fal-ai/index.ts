import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { FalAIInput, FalAIOutput } from './types.js';

const adapter: ISkillAdapter = {
  id: 'fal-ai',
  plane: 'hands',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    required: ['prompt'],
    properties: {
      prompt: { type: 'string', minLength: 3, maxLength: 1000 },
      model: { type: 'string' },
      size: { type: 'string', enum: ['square', 'portrait', 'landscape'] }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'image_url', 'model_used', 'size'],
    properties: {
      provider: { type: 'string', enum: ['fal-ai'] },
      image_url: { type: 'string', format: 'uri' },
      model_used: { type: 'string' },
      size: { type: 'string', enum: ['square', 'portrait', 'landscape'] }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'fal-ai',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message: parsed.error.issues.map((issue) => issue.message).join('; '),
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: {
          retries: 0,
          circuit_breaker_state: 'CLOSED',
          cache_hit: false
        }
      };
    }

    try {
      const data = await runClient(parsed.data as FalAIInput);
      const output = OutputSchema.parse(data) as FalAIOutput;
      return {
        skill_id: 'fal-ai',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: {
          retries: 0,
          circuit_breaker_state: 'CLOSED',
          cache_hit: false
        }
      };
    } catch (err) {
      const blocked = err instanceof Error && err.message === 'FAL_CONTENT_POLICY_BLOCKED';
      return {
        skill_id: 'fal-ai',
        status: 'FAILED',
        error: {
          code: blocked ? 'VALIDATION_FAILED' : 'EXTERNAL_ERROR',
          message: blocked
            ? 'Prompt blocked by content policy.'
            : err instanceof Error
              ? err.message
              : 'fal-ai execution failed',
          retryable: !blocked,
          http_status: blocked ? 422 : 502
        },
        latency_ms: Date.now() - started,
        metadata: {
          retries: 0,
          circuit_breaker_state: 'CLOSED',
          cache_hit: false
        }
      };
    }
  },
  async healthCheck(): Promise<boolean> {
    return true;
  }
};

export default adapter;
