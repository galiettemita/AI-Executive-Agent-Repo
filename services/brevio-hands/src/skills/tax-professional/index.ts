import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { TaxProfessionalInput, TaxProfessionalOutput } from './types.js';

const TAX_PROFESSIONAL_TAX_YEAR_REQUIRED = 'TAX_PROFESSIONAL_TAX_YEAR_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'tax-professional',
  plane: 'hands',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    required: ['action', 'tax_year'],
    properties: {
      action: { type: 'string', enum: ['estimate_deductions', 'filing_checklist'] },
      tax_year: { type: 'integer', minimum: 2000, maximum: 2100 },
      filing_status: {
        type: 'string',
        enum: ['single', 'married_filing_jointly', 'married_filing_separately', 'head_of_household']
      },
      deductible_expenses_cents: { type: 'array' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'tax_year', 'estimated_deductions_cents', 'checklist', 'disclaimer', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['tax-professional'] },
      action: { type: 'string', enum: ['estimate_deductions', 'filing_checklist'] },
      tax_year: { type: 'integer', minimum: 2000, maximum: 2100 },
      estimated_deductions_cents: { type: 'integer', minimum: 0 },
      checklist: { type: 'array' },
      disclaimer: { type: 'string', enum: ['not_tax_advice'] },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'tax-professional',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') ||
            TAX_PROFESSIONAL_TAX_YEAR_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as TaxProfessionalInput);
      const output = OutputSchema.parse(data) as TaxProfessionalOutput;
      return {
        skill_id: 'tax-professional',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'tax-professional',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'tax-professional execution failed',
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
