import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { ColoringPageInput, ColoringPageOutput } from './types.js';

const COLORING_PAGE_PROMPT_REQUIRED = 'COLORING_PAGE_PROMPT_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'coloring-page',
  plane: 'hands',
  requiredScopes: ['image.generate'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['generate_from_prompt', 'generate_from_image'] },
      prompt: { type: 'string', minLength: 2, maxLength: 500 },
      image_url: { type: 'string', format: 'uri' },
      complexity: { type: 'string', enum: ['easy', 'medium', 'advanced'] }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'output_url', 'page_size', 'line_density', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['coloring-page'] },
      action: { type: 'string', enum: ['generate_from_prompt', 'generate_from_image'] },
      output_url: { type: 'string' },
      page_size: { type: 'string', enum: ['A4', 'Letter'] },
      line_density: { type: 'string', enum: ['low', 'medium', 'high'] },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'coloring-page',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message: parsed.error.issues.map((issue) => issue.message).join('; ') || COLORING_PAGE_PROMPT_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as ColoringPageInput);
      const output = OutputSchema.parse(data) as ColoringPageOutput;
      return {
        skill_id: 'coloring-page',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'coloring-page',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'coloring-page execution failed',
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
