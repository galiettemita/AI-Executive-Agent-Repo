import type { ISkillAdapter, SkillContext, SkillInput } from '@brevio/shared';
import type { SkillResult } from '@brevio/shared';

export const TemplateSkillAdapter: ISkillAdapter = {
  id: 'template-skill',
  plane: 'hands',
  requiredScopes: [],
  inputSchema: { type: 'object' },
  outputSchema: { type: 'object' },
  async execute(_input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    return {
      skill_id: 'template-skill',
      status: 'SUCCESS',
      data: { ok: true },
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
