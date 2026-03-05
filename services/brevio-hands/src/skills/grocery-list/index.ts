import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { GroceryListInput, GroceryListOutput } from './types.js';

const GROCERY_LIST_CLEAR_CONFIRMATION_REQUIRED = 'GROCERY_LIST_CLEAR_CONFIRMATION_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'grocery-list',
  plane: 'hands',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: {
        type: 'string',
        enum: ['add_items', 'remove_items', 'list_items', 'organize_by_section', 'clear_list']
      },
      list_id: { type: 'string', minLength: 2, maxLength: 80 },
      items: { type: 'array' },
      item_ids: { type: 'array' },
      confirmed: { type: 'boolean' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'list_id', 'items', 'total_items'],
    properties: {
      provider: { type: 'string', enum: ['grocery-list'] },
      action: {
        type: 'string',
        enum: ['add_items', 'remove_items', 'list_items', 'organize_by_section', 'clear_list']
      },
      list_id: { type: 'string' },
      items: { type: 'array' },
      total_items: { type: 'integer', minimum: 0 }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'grocery-list',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message: parsed.error.issues.map((issue) => issue.message).join('; '),
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    const request = parsed.data as GroceryListInput;
    if (request.action === 'clear_list' && request.confirmed !== true) {
      return {
        skill_id: 'grocery-list',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message: GROCERY_LIST_CLEAR_CONFIRMATION_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(request);
      const output = OutputSchema.parse(data) as GroceryListOutput;
      return {
        skill_id: 'grocery-list',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'grocery-list',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'grocery-list execution failed',
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
