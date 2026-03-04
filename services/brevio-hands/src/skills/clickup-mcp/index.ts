import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { ClickupInput, ClickupOutput } from './types.js';

const adapter: ISkillAdapter = {
  id: 'clickup-mcp',
  plane: 'hands',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: {
        type: 'string',
        enum: ['task_list', 'task_create', 'doc_create', 'time_start', 'time_stop']
      },
      list_id: { type: 'string' },
      task_id: { type: 'string' },
      title: { type: 'string' },
      content: { type: 'string' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action'],
    properties: {
      provider: { type: 'string', enum: ['clickup-mcp'] },
      action: {
        type: 'string',
        enum: ['task_list', 'task_create', 'doc_create', 'time_start', 'time_stop']
      },
      task_id: { type: 'string' },
      doc_id: { type: 'string' },
      timer_started: { type: 'boolean' },
      tasks: { type: 'array' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'clickup-mcp',
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
      const data = await runClient(parsed.data as ClickupInput);
      const output = OutputSchema.parse(data) as ClickupOutput;
      return {
        skill_id: 'clickup-mcp',
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
        (err.message === 'CLICKUP_TITLE_REQUIRED' ||
          err.message === 'CLICKUP_DOC_TITLE_REQUIRED' ||
          err.message === 'CLICKUP_TASK_ID_REQUIRED');

      return {
        skill_id: 'clickup-mcp',
        status: 'FAILED',
        error: {
          code: validationError ? 'VALIDATION_FAILED' : 'EXTERNAL_ERROR',
          message: validationError
            ? 'ClickUp action missing required title/task_id fields.'
            : err instanceof Error
              ? err.message
              : 'clickup-mcp execution failed',
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
