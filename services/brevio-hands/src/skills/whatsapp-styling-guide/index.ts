import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type {
  WhatsAppStylingGuideInput,
  WhatsAppStylingGuideOutput
} from './types.js';

const WHATSAPP_STYLING_TEXT_REQUIRED = 'WHATSAPP_STYLING_TEXT_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'whatsapp-styling-guide',
  plane: 'gateway',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    required: ['text'],
    properties: {
      text: { type: 'string', minLength: 1, maxLength: 4096 },
      style: { type: 'string', enum: ['default', 'bullet', 'numbered', 'emphasis'] }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'formatted_text', 'applied_rules', 'char_count', 'latency_budget_ms'],
    properties: {
      provider: { type: 'string', enum: ['whatsapp-styling-guide'] },
      formatted_text: { type: 'string' },
      applied_rules: { type: 'array' },
      char_count: { type: 'integer', minimum: 1, maximum: 4096 },
      latency_budget_ms: { type: 'integer', enum: [10] }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'whatsapp-styling-guide',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') ||
            WHATSAPP_STYLING_TEXT_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as WhatsAppStylingGuideInput);
      const output = OutputSchema.parse(data) as WhatsAppStylingGuideOutput;
      return {
        skill_id: 'whatsapp-styling-guide',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'whatsapp-styling-guide',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'whatsapp-styling-guide execution failed',
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
