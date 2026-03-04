import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { AlterActionsInput, AlterActionsOutput } from './types.js';

const ALTER_ACTIONS_ACTION_KEY_REQUIRED = 'ALTER_ACTIONS_ACTION_KEY_REQUIRED';
const ALTER_ACTIONS_CONFIRMATION_REQUIRED = 'ALTER_ACTIONS_CONFIRMATION_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'alter-actions',
  plane: 'hands',
  requiredScopes: ['local.app_actions.execute'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['list_actions', 'trigger_action'] },
      action_key: { type: 'string', minLength: 2, maxLength: 120 },
      app_name: { type: 'string', minLength: 2, maxLength: 120 },
      parameters: { type: 'object' },
      confirmed: { type: 'boolean' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'actions', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['alter-actions'] },
      action: { type: 'string', enum: ['list_actions', 'trigger_action'] },
      actions: { type: 'array' },
      triggered_action: { type: 'string' },
      callback_url: { type: 'string', format: 'uri' },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'alter-actions',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') ||
            ALTER_ACTIONS_ACTION_KEY_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    const request = parsed.data as AlterActionsInput;
    if (request.action === 'trigger_action' && request.confirmed !== true) {
      return {
        skill_id: 'alter-actions',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message: ALTER_ACTIONS_CONFIRMATION_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(request);
      const output = OutputSchema.parse(data) as AlterActionsOutput;
      return {
        skill_id: 'alter-actions',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'alter-actions',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'alter-actions execution failed',
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
