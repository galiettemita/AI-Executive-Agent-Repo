import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { CraftInput, CraftOutput } from './types.js';

const CRAFT_DOC_TITLE_REQUIRED = 'CRAFT_DOC_TITLE_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'craft',
  plane: 'hands',
  requiredScopes: ['notes.write'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['create_doc', 'append_doc', 'search_docs'] },
      doc_title: { type: 'string', minLength: 2, maxLength: 240 },
      doc_id: { type: 'string', minLength: 2, maxLength: 120 },
      content: { type: 'string', minLength: 2, maxLength: 20000 },
      query: { type: 'string', minLength: 2, maxLength: 500 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'docs', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['craft'] },
      action: { type: 'string', enum: ['create_doc', 'append_doc', 'search_docs'] },
      docs: { type: 'array', items: { type: 'object' } },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'craft',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message: parsed.error.issues.map((issue) => issue.message).join('; ') || CRAFT_DOC_TITLE_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as CraftInput);
      const output = OutputSchema.parse(data) as CraftOutput;
      return {
        skill_id: 'craft',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'craft',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'craft execution failed',
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
