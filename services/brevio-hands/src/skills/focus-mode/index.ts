import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { FocusModeInput, FocusModeOutput } from './types.js';

const FOCUS_MODE_SESSION_REQUIRED = 'FOCUS_MODE_SESSION_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'focus-mode',
  plane: 'brain',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['start_session', 'check_in', 'end_session'] },
      goal: { type: 'string', minLength: 5, maxLength: 240 },
      duration_minutes: { type: 'integer', minimum: 15, maximum: 240 },
      session_id: { type: 'string', minLength: 6, maxLength: 80 },
      distraction_note: { type: 'string', minLength: 2, maxLength: 240 },
      completed_tasks: { type: 'array' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'session_id', 'status', 'check_in_schedule', 'next_prompt'],
    properties: {
      provider: { type: 'string', enum: ['focus-mode'] },
      action: { type: 'string', enum: ['start_session', 'check_in', 'end_session'] },
      session_id: { type: 'string' },
      status: { type: 'string', enum: ['active', 'checking_in', 'completed'] },
      check_in_schedule: { type: 'array' },
      next_prompt: { type: 'string' },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'focus-mode',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') || FOCUS_MODE_SESSION_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as FocusModeInput);
      const output = OutputSchema.parse(data) as FocusModeOutput;
      return {
        skill_id: 'focus-mode',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'focus-mode',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'focus-mode execution failed',
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
