import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { TickTickInput, TickTickOutput } from './types.js';

const TICKTICK_TASK_CONTENT_REQUIRED = 'TICKTICK_TASK_CONTENT_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'ticktick',
  plane: 'hands',
  requiredScopes: ['tasks:write', 'tasks:read'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['add_task', 'list_tasks', 'complete_task', 'delete_task'] },
      task_content: { type: 'string', minLength: 2, maxLength: 500 },
      task_id: { type: 'string', minLength: 3, maxLength: 120 },
      project_id: { type: 'string', minLength: 1, maxLength: 120 },
      due_date: { type: 'string', format: 'date-time' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'tasks', 'total_tasks', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['ticktick'] },
      action: { type: 'string', enum: ['add_task', 'list_tasks', 'complete_task', 'delete_task'] },
      tasks: { type: 'array', items: { type: 'object' } },
      total_tasks: { type: 'integer', minimum: 0 },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'ticktick',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message: parsed.error.issues.map((issue) => issue.message).join('; ') || TICKTICK_TASK_CONTENT_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as TickTickInput);
      const output = OutputSchema.parse(data) as TickTickOutput;
      return {
        skill_id: 'ticktick',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'ticktick',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'ticktick execution failed',
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
