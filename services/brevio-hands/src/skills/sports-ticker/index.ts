import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { SportsTickerInput, SportsTickerOutput } from './types.js';

const SPORTS_TICKER_LEAGUE_REQUIRED = 'SPORTS_TICKER_LEAGUE_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'sports-ticker',
  plane: 'hands',
  requiredScopes: ['sports.read'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['get_score', 'get_schedule'] },
      league: { type: 'string', enum: ['nba', 'nfl', 'mlb', 'nhl', 'epl'] },
      team: { type: 'string', minLength: 2, maxLength: 120 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'items', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['sports-ticker'] },
      action: { type: 'string', enum: ['get_score', 'get_schedule'] },
      items: { type: 'array', items: { type: 'object' } },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'sports-ticker',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') || SPORTS_TICKER_LEAGUE_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as SportsTickerInput);
      const output = OutputSchema.parse(data) as SportsTickerOutput;
      return {
        skill_id: 'sports-ticker',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'sports-ticker',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'sports-ticker execution failed',
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
