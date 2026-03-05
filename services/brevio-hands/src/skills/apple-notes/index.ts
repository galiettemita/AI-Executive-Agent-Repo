import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { AppleNotesInput, AppleNotesOutput } from './types.js';

const APPLE_NOTES_CREATE_FIELDS_REQUIRED = 'APPLE_NOTES_CREATE_FIELDS_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'apple-notes',
  plane: 'hands',
  requiredScopes: ['notes.read', 'notes.write'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['create_note', 'search_notes', 'list_recent'] },
      title: { type: 'string', minLength: 1, maxLength: 200 },
      body: { type: 'string', minLength: 1, maxLength: 5000 },
      folder: { type: 'string', minLength: 1, maxLength: 120 },
      query: { type: 'string', minLength: 2, maxLength: 200 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'canonical_skill_id', 'deprecated_alias', 'notes', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['apple-notes'] },
      action: { type: 'string', enum: ['create_note', 'search_notes', 'list_recent'] },
      canonical_skill_id: { type: 'string', enum: ['apple-notes-skill'] },
      deprecated_alias: { type: 'boolean', enum: [true] },
      notes: { type: 'array' },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'apple-notes',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') ||
            APPLE_NOTES_CREATE_FIELDS_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as AppleNotesInput);
      const output = OutputSchema.parse(data) as AppleNotesOutput;
      return {
        skill_id: 'apple-notes',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'apple-notes',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'apple-notes execution failed',
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
