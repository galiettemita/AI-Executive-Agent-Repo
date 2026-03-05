import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { ProactiveResearchInput, ProactiveResearchOutput } from './types.js';

const PROACTIVE_RESEARCH_TOPIC_REQUIRED = 'PROACTIVE_RESEARCH_TOPIC_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'proactive-research',
  plane: 'hands',
  requiredScopes: ['research.monitor'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['monitor_topic', 'summarize_updates'] },
      topic: { type: 'string', minLength: 2, maxLength: 500 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'alerts', 'next_check_at', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['proactive-research'] },
      action: { type: 'string', enum: ['monitor_topic', 'summarize_updates'] },
      alerts: { type: 'array', items: { type: 'string' } },
      next_check_at: { type: 'string' },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'proactive-research',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') || PROACTIVE_RESEARCH_TOPIC_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as ProactiveResearchInput);
      const output = OutputSchema.parse(data) as ProactiveResearchOutput;
      return {
        skill_id: 'proactive-research',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'proactive-research',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'proactive-research execution failed',
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
