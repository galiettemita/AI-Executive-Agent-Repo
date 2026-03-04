import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { HomeAssistantInput, HomeAssistantOutput } from './types.js';

const adapter: ISkillAdapter = {
  id: 'home-assistant',
  plane: 'hands',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    required: ['entity_id', 'action'],
    properties: {
      entity_id: { type: 'string', minLength: 3, maxLength: 200 },
      action: { type: 'string', minLength: 2, maxLength: 100 },
      value: { type: ['string', 'number', 'boolean'] },
      two_factor_code: { type: 'string' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['state', 'attributes'],
    properties: {
      state: { type: 'string' },
      attributes: {
        type: 'object',
        additionalProperties: { type: ['string', 'number', 'boolean'] }
      }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'home-assistant',
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
      const data = await runClient(parsed.data as HomeAssistantInput);
      const output = OutputSchema.parse(data) as HomeAssistantOutput;
      return {
        skill_id: 'home-assistant',
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
      const isSafety = err instanceof Error && err.message === 'SAFETY_2FA_REQUIRED';
      return {
        skill_id: 'home-assistant',
        status: 'FAILED',
        error: {
          code: isSafety ? 'VALIDATION_FAILED' : 'EXTERNAL_ERROR',
          message: isSafety
            ? 'Action requires 2FA confirmation (two_factor_code).'
            : err instanceof Error
              ? err.message
              : 'home-assistant execution failed',
          retryable: !isSafety,
          http_status: isSafety ? 403 : 502
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
