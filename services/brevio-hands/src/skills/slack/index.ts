import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { SlackInput, SlackOutput } from './types.js';

const adapter: ISkillAdapter = {
  id: 'slack',
  plane: 'hands',
  requiredScopes: ['channels:read', 'chat:write', 'reactions:write', 'users:read'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['list_channels', 'post_message', 'add_reaction'] },
      channel_id: { type: 'string', minLength: 3, maxLength: 50 },
      text: { type: 'string', minLength: 1, maxLength: 4000 },
      message_ts: { type: 'string', minLength: 3, maxLength: 50 },
      emoji: { type: 'string', minLength: 2, maxLength: 40 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action'],
    properties: {
      provider: { type: 'string', enum: ['slack'] },
      action: { type: 'string', enum: ['list_channels', 'post_message', 'add_reaction'] },
      channels: { type: 'array' },
      post: { type: 'object' },
      reacted: { type: 'boolean' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'slack',
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
      const data = await runClient(parsed.data as SlackInput);
      const output = OutputSchema.parse(data) as SlackOutput;
      return {
        skill_id: 'slack',
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
        skill_id: 'slack',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'slack execution failed',
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
