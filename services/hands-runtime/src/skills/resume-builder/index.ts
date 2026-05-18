import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { ResumeBuilderInput, ResumeBuilderOutput } from './types.js';

const RESUME_BUILDER_ROLE_REQUIRED = 'RESUME_BUILDER_ROLE_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'resume-builder',
  plane: 'hands',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['generate', 'tailor', 'score'] },
      role: { type: 'string', minLength: 2, maxLength: 200 },
      experience_bullets: { type: 'array' },
      job_description: { type: 'string', minLength: 20, maxLength: 5000 },
      resume_markdown: { type: 'string', minLength: 20, maxLength: 20000 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action'],
    properties: {
      provider: { type: 'string', enum: ['resume-builder'] },
      action: { type: 'string', enum: ['generate', 'tailor', 'score'] },
      resume_markdown: { type: 'string' },
      score: { type: 'number' },
      recommendations: { type: 'array' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'resume-builder',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') || RESUME_BUILDER_ROLE_REQUIRED,
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
      const data = await runClient(parsed.data as ResumeBuilderInput);
      const output = OutputSchema.parse(data) as ResumeBuilderOutput;
      return {
        skill_id: 'resume-builder',
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
      return {
        skill_id: 'resume-builder',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'resume-builder execution failed',
          retryable: true,
          http_status: 502
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
