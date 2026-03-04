import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { ContractReviewerInput, ContractReviewerOutput } from './types.js';

const CONTRACT_REVIEWER_TEXT_REQUIRED = 'CONTRACT_REVIEWER_TEXT_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'contract-reviewer',
  plane: 'hands',
  requiredScopes: ['document.analysis'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['review_contract', 'summarize_risks'] },
      contract_text: { type: 'string', minLength: 50, maxLength: 40000 },
      contract_type: { type: 'string', enum: ['msa', 'nda', 'lease', 'employment', 'other'] },
      jurisdiction: { type: 'string', minLength: 2, maxLength: 120 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'overall_risk', 'risk_items', 'must_review_clauses', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['contract-reviewer'] },
      action: { type: 'string', enum: ['review_contract', 'summarize_risks'] },
      overall_risk: { type: 'string', enum: ['low', 'medium', 'high'] },
      risk_items: { type: 'array', items: { type: 'object' } },
      must_review_clauses: { type: 'array', items: { type: 'string' } },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'contract-reviewer',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') || CONTRACT_REVIEWER_TEXT_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as ContractReviewerInput);
      const output = OutputSchema.parse(data) as ContractReviewerOutput;
      return {
        skill_id: 'contract-reviewer',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'contract-reviewer',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'contract-reviewer execution failed',
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
