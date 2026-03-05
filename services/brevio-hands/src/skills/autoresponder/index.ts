import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { AutoresponderInput, AutoresponderOutput } from './types.js';

const AUTORESPONDER_INTERCEPT_TEXT_REQUIRED = 'AUTORESPONDER_INTERCEPT_TEXT_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'autoresponder',
  plane: 'gateway',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['enable', 'disable', 'intercept'] },
      ruleset_name: { type: 'string', minLength: 2, maxLength: 120 },
      incoming_text: { type: 'string', minLength: 1, maxLength: 4096 },
      channel: { type: 'string', enum: ['whatsapp', 'imessage'] },
      delegation_enabled: { type: 'boolean' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'status', 'delegated_to_brain', 'latency_budget_ms'],
    properties: {
      provider: { type: 'string', enum: ['autoresponder'] },
      action: { type: 'string', enum: ['enable', 'disable', 'intercept'] },
      status: { type: 'string', enum: ['enabled', 'disabled', 'responded'] },
      delegated_to_brain: { type: 'boolean' },
      response_text: { type: 'string' },
      latency_budget_ms: { type: 'integer', enum: [8000] }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'autoresponder',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') ||
            AUTORESPONDER_INTERCEPT_TEXT_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as AutoresponderInput);
      const output = OutputSchema.parse(data) as AutoresponderOutput;
      return {
        skill_id: 'autoresponder',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'autoresponder',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'autoresponder execution failed',
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
