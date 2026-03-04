import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { GranolaInput, GranolaOutput } from './types.js';

const GRANOLA_NOTE_TEXT_REQUIRED = 'GRANOLA_NOTE_TEXT_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'granola',
  plane: 'hands',
  requiredScopes: ['notes.read'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['summarize_note', 'extract_actions'] },
      note_text: { type: 'string', minLength: 20, maxLength: 20000 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'summary', 'action_items', 'decisions'],
    properties: {
      provider: { type: 'string', enum: ['granola'] },
      action: { type: 'string', enum: ['summarize_note', 'extract_actions'] },
      summary: { type: 'string' },
      action_items: { type: 'array', items: { type: 'string' } },
      decisions: { type: 'array', items: { type: 'string' } }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'granola',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message: parsed.error.issues.map((issue) => issue.message).join('; ') || GRANOLA_NOTE_TEXT_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as GranolaInput);
      const output = OutputSchema.parse(data) as GranolaOutput;
      return {
        skill_id: 'granola',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'granola',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'granola execution failed',
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
