import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { RelationshipSkillsInput, RelationshipSkillsOutput } from './types.js';

const RELATIONSHIP_SKILLS_CONTEXT_REQUIRED = 'RELATIONSHIP_SKILLS_CONTEXT_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'relationship-skills',
  plane: 'hands',
  requiredScopes: ['coaching.guidance'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['coach_message', 'conflict_plan'] },
      context: { type: 'string', minLength: 10, maxLength: 4000 },
      goal: { type: 'string', minLength: 5, maxLength: 500 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'talking_points', 'suggested_message', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['relationship-skills'] },
      action: { type: 'string', enum: ['coach_message', 'conflict_plan'] },
      talking_points: { type: 'array', items: { type: 'string' } },
      suggested_message: { type: 'string' },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'relationship-skills',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') || RELATIONSHIP_SKILLS_CONTEXT_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as RelationshipSkillsInput);
      const output = OutputSchema.parse(data) as RelationshipSkillsOutput;
      return {
        skill_id: 'relationship-skills',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'relationship-skills',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'relationship-skills execution failed',
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
