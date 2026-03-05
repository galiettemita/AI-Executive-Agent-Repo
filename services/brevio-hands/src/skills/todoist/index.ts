import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { TodoistInput, TodoistOutput } from './types.js';

const adapter: ISkillAdapter = {
  id: 'todoist',
  plane: 'hands',
  requiredScopes: ['task:add', 'data:read'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['list', 'create', 'complete', 'delete'] },
      project_id: { type: 'string' },
      task: { type: 'object' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action'],
    properties: {
      provider: { type: 'string', enum: ['todoist_mock'] },
      action: { type: 'string', enum: ['list', 'create', 'complete', 'delete'] },
      task_id: { type: 'string' },
      tasks: { type: 'array' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'todoist',
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
      const data = await runClient(parsed.data as TodoistInput);
      const output = OutputSchema.parse(data) as TodoistOutput;
      return {
        skill_id: 'todoist',
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
      const isValidationError =
        err instanceof Error &&
        (err.message === 'TODOIST_CONTENT_REQUIRED' || err.message === 'TODOIST_TASK_ID_REQUIRED');

      return {
        skill_id: 'todoist',
        status: 'FAILED',
        error: {
          code: isValidationError ? 'VALIDATION_FAILED' : 'EXTERNAL_ERROR',
          message: isValidationError
            ? 'Todoist action requires the expected task fields (content or task_id).'
            : err instanceof Error
              ? err.message
              : 'todoist execution failed',
          retryable: !isValidationError,
          http_status: isValidationError ? 400 : 502
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
