import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { LinearInput, LinearOutput } from './types.js';

const adapter: ISkillAdapter = {
  id: 'linear',
  plane: 'hands',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['issue_list', 'issue_create', 'issue_update'] },
      team_id: { type: 'string' },
      issue_id: { type: 'string' },
      title: { type: 'string' },
      description: { type: 'string' },
      status: { type: 'string' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action'],
    properties: {
      provider: { type: 'string', enum: ['linear'] },
      action: { type: 'string', enum: ['issue_list', 'issue_create', 'issue_update'] },
      issue_id: { type: 'string' },
      issues: { type: 'array' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'linear',
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
      const data = await runClient(parsed.data as LinearInput);
      const output = OutputSchema.parse(data) as LinearOutput;
      return {
        skill_id: 'linear',
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
      const validationError =
        err instanceof Error &&
        (err.message === 'LINEAR_CREATE_FIELDS_REQUIRED' || err.message === 'LINEAR_ISSUE_ID_REQUIRED');

      return {
        skill_id: 'linear',
        status: 'FAILED',
        error: {
          code: validationError ? 'VALIDATION_FAILED' : 'EXTERNAL_ERROR',
          message: validationError
            ? 'Linear action requires issue_id or create fields (team_id/title).'
            : err instanceof Error
              ? err.message
              : 'linear execution failed',
          retryable: !validationError,
          http_status: validationError ? 400 : 502
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
