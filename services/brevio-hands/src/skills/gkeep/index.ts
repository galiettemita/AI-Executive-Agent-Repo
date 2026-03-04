import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type {
  GKEEPInput,
  GKEEPOutput
} from './types.js';

const adapter: ISkillAdapter = {
  id: 'gkeep',
  plane: 'hands',
  requiredScopes: ['keep'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['list', 'create', 'search', 'update'] },
      note_id: { type: 'string' },
      title: { type: 'string' },
      content: { type: 'string' },
      query: { type: 'string' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action'],
    properties: {
      provider: { type: 'string', enum: ['gkeep'] },
      action: { type: 'string', enum: ['list', 'create', 'search', 'update'] },
      note_id: { type: 'string' },
      notes: { type: 'array' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'gkeep',
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
      const data = await runClient(parsed.data as GKEEPInput);
      const output = OutputSchema.parse(data) as GKEEPOutput;
      return {
        skill_id: 'gkeep',
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
        (err.message === 'GKEEP_CREATE_FIELDS_REQUIRED' ||
          err.message === 'GKEEP_UPDATE_FIELDS_REQUIRED');

      return {
        skill_id: 'gkeep',
        status: 'FAILED',
        error: {
          code: validationError ? 'VALIDATION_FAILED' : 'EXTERNAL_ERROR',
          message: validationError
            ? 'Note action missing required create/update fields.'
            : err instanceof Error
              ? err.message
              : 'gkeep execution failed',
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
