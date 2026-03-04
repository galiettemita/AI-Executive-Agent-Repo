import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { DoingTasksInput, DoingTasksOutput } from './types.js';

const DOING_TASKS_TASK_REQUIRED = 'DOING_TASKS_TASK_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'doing-tasks',
  plane: 'hands',
  requiredScopes: ['task.orchestration'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['route_task', 'status_report'] },
      task: { type: 'string', minLength: 5, maxLength: 2000 },
      skill_hint: { type: 'string', minLength: 2, maxLength: 120 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'routed_skill', 'execution_plan', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['doing-tasks'] },
      action: { type: 'string', enum: ['route_task', 'status_report'] },
      routed_skill: { type: 'string' },
      execution_plan: { type: 'array', items: { type: 'string' } },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'doing-tasks',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message: parsed.error.issues.map((issue) => issue.message).join('; ') || DOING_TASKS_TASK_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as DoingTasksInput);
      const output = OutputSchema.parse(data) as DoingTasksOutput;
      return {
        skill_id: 'doing-tasks',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'doing-tasks',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'doing-tasks execution failed',
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
