import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { DeAiIfyInput, DeAiIfyOutput } from './types.js';

const DE_AI_IFY_TEXT_REQUIRED = 'DE_AI_IFY_TEXT_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'de-ai-ify',
  plane: 'hands',
  requiredScopes: ['text.rewrite'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['rewrite_text', 'tone_check'] },
      text: { type: 'string', minLength: 10, maxLength: 8000 },
      target_tone: { type: 'string', enum: ['casual', 'professional', 'direct'] }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'rewritten_text', 'detected_ai_markers', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['de-ai-ify'] },
      action: { type: 'string', enum: ['rewrite_text', 'tone_check'] },
      rewritten_text: { type: 'string' },
      detected_ai_markers: { type: 'array', items: { type: 'string' } },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'de-ai-ify',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message: parsed.error.issues.map((issue) => issue.message).join('; ') || DE_AI_IFY_TEXT_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as DeAiIfyInput);
      const output = OutputSchema.parse(data) as DeAiIfyOutput;
      return {
        skill_id: 'de-ai-ify',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'de-ai-ify',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'de-ai-ify execution failed',
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
