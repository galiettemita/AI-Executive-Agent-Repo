import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { AsanaInput, AsanaOutput } from './types.js';

const adapter: ISkillAdapter = {
  id: 'asana',
  plane: 'hands',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['task_list', 'task_create', 'task_update'] },
      project_id: { type: 'string' },
      task_id: { type: 'string' },
      name: { type: 'string' },
      notes: { type: 'string' },
      status: { type: 'string', enum: ['todo', 'in_progress', 'done'] }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action'],
    properties: {
      provider: { type: 'string', enum: ['asana'] },
      action: { type: 'string', enum: ['task_list', 'task_create', 'task_update'] },
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
        skill_id: 'asana',
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
      const data = await runClient(parsed.data as AsanaInput);
      const output = OutputSchema.parse(data) as AsanaOutput;
      return {
        skill_id: 'asana',
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
        (err.message === 'ASANA_CREATE_FIELDS_REQUIRED' || err.message === 'ASANA_TASK_ID_REQUIRED');

      return {
        skill_id: 'asana',
        status: 'FAILED',
        error: {
          code: validationError ? 'VALIDATION_FAILED' : 'EXTERNAL_ERROR',
          message: validationError
            ? 'Asana action requires create fields (project_id/name) or task_id.'
            : err instanceof Error
              ? err.message
              : 'asana execution failed',
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
