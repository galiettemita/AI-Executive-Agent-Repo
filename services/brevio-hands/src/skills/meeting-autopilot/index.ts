import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { MeetingAutopilotInput, MeetingAutopilotOutput } from './types.js';

const MEETING_AUTOPILOT_TRANSCRIPT_REQUIRED = 'MEETING_AUTOPILOT_TRANSCRIPT_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'meeting-autopilot',
  plane: 'brain',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['summarize_meeting', 'extract_actions', 'draft_follow_up'] },
      meeting_title: { type: 'string', minLength: 2, maxLength: 180 },
      transcript: { type: 'string', minLength: 20, maxLength: 16000 },
      participants: { type: 'array' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'summary', 'decisions', 'action_items'],
    properties: {
      provider: { type: 'string', enum: ['meeting-autopilot'] },
      action: { type: 'string', enum: ['summarize_meeting', 'extract_actions', 'draft_follow_up'] },
      summary: { type: 'string' },
      decisions: { type: 'array' },
      action_items: { type: 'array' },
      follow_up_email: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'meeting-autopilot',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') ||
            MEETING_AUTOPILOT_TRANSCRIPT_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as MeetingAutopilotInput);
      const output = OutputSchema.parse(data) as MeetingAutopilotOutput;
      return {
        skill_id: 'meeting-autopilot',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'meeting-autopilot',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'meeting-autopilot execution failed',
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
