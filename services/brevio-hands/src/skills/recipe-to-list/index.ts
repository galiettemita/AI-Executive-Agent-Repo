import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { RecipeToListInput, RecipeToListOutput } from './types.js';

const RECIPE_TO_LIST_TEXT_REQUIRED = 'RECIPE_TO_LIST_TEXT_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'recipe-to-list',
  plane: 'hands',
  requiredScopes: ['task:add', 'data:read', 'data:read_write'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['parse_recipe', 'sync_todoist'] },
      recipe_title: { type: 'string', minLength: 2, maxLength: 200 },
      recipe_text: { type: 'string', minLength: 10, maxLength: 12000 },
      recipe_items: { type: 'array' },
      project_id: { type: 'string', minLength: 2, maxLength: 120 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'recipe_title', 'normalized_items'],
    properties: {
      provider: { type: 'string', enum: ['recipe-to-list'] },
      action: { type: 'string', enum: ['parse_recipe', 'sync_todoist'] },
      recipe_title: { type: 'string' },
      normalized_items: { type: 'array' },
      task_ids: { type: 'array' },
      project_name: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'recipe-to-list',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') || RECIPE_TO_LIST_TEXT_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as RecipeToListInput);
      const output = OutputSchema.parse(data) as RecipeToListOutput;
      return {
        skill_id: 'recipe-to-list',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'recipe-to-list',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'recipe-to-list execution failed',
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
