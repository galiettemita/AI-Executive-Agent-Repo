import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { GeminiDeepResearchInput, GeminiDeepResearchOutput } from './types.js';

const GEMINI_DEEP_RESEARCH_TOPIC_REQUIRED = 'GEMINI_DEEP_RESEARCH_TOPIC_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'gemini-deep-research',
  plane: 'hands',
  requiredScopes: ['research.deep'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['run_research'] },
      topic: { type: 'string', minLength: 2, maxLength: 500 },
      depth: { type: 'string', enum: ['standard', 'deep'] }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'report_sections', 'citations', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['gemini-deep-research'] },
      action: { type: 'string', enum: ['run_research'] },
      report_sections: { type: 'array', items: { type: 'string' } },
      citations: { type: 'array', items: { type: 'string' } },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'gemini-deep-research',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') || GEMINI_DEEP_RESEARCH_TOPIC_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as GeminiDeepResearchInput);
      const output = OutputSchema.parse(data) as GeminiDeepResearchOutput;
      return {
        skill_id: 'gemini-deep-research',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'gemini-deep-research',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'gemini-deep-research execution failed',
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
