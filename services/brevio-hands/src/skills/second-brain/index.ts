import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';

const adapter: ISkillAdapter = {
  id: 'second-brain',
  plane: 'hands',
  requiredScopes: [],
  inputSchema: { type: 'object' },
  outputSchema: { type: 'object' },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const data = await runClient({ payload: input });
    return {
      skill_id: 'second-brain',
      status: 'SUCCESS',
      data,
      latency_ms: 1,
      metadata: {
        retries: 0,
        circuit_breaker_state: 'CLOSED',
        cache_hit: false
      }
    };
  },
  async healthCheck(): Promise<boolean> {
    return true;
  }
};

export default adapter;
