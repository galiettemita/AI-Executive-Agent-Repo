import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { ThingsMacInput, ThingsMacOutput } from './types.js';

const THINGS_MAC_TITLE_REQUIRED = 'THINGS_MAC_TITLE_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'things-mac',
  plane: 'hands',
  requiredScopes: ['things.local.write'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['create_todo', 'list_today', 'complete_todo', 'move_to_project'] },
      title: { type: 'string', minLength: 2, maxLength: 240 },
      todo_id: { type: 'string', minLength: 3, maxLength: 120 },
      project: { type: 'string', minLength: 1, maxLength: 120 },
      due_date: { type: 'string', format: 'date-time' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'todos', 'inbox_count', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['things-mac'] },
      action: { type: 'string', enum: ['create_todo', 'list_today', 'complete_todo', 'move_to_project'] },
      todos: { type: 'array', items: { type: 'object' } },
      inbox_count: { type: 'integer', minimum: 0 },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'things-mac',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message: parsed.error.issues.map((issue) => issue.message).join('; ') || THINGS_MAC_TITLE_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as ThingsMacInput);
      const output = OutputSchema.parse(data) as ThingsMacOutput;
      return {
        skill_id: 'things-mac',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'things-mac',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'things-mac execution failed',
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
