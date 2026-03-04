import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { ReactEmailSkillsInput, ReactEmailSkillsOutput } from './types.js';

const REACT_EMAIL_TEMPLATE_REQUIRED = 'REACT_EMAIL_TEMPLATE_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'react-email-skills',
  plane: 'hands',
  requiredScopes: ['email.template'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['render_template', 'preview_message'] },
      template_id: { type: 'string', minLength: 2, maxLength: 120 },
      subject: { type: 'string', minLength: 2, maxLength: 240 },
      variables: { type: 'object' },
      preview_to: { type: 'string', format: 'email' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'html', 'text', 'preview_id', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['react-email-skills'] },
      action: { type: 'string', enum: ['render_template', 'preview_message'] },
      html: { type: 'string' },
      text: { type: 'string' },
      preview_id: { type: 'string' },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'react-email-skills',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') || REACT_EMAIL_TEMPLATE_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as ReactEmailSkillsInput);
      const output = OutputSchema.parse(data) as ReactEmailSkillsOutput;
      return {
        skill_id: 'react-email-skills',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'react-email-skills',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'react-email-skills execution failed',
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
