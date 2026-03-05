import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type {
  FinancialMarketAnalysisInput,
  FinancialMarketAnalysisOutput
} from './types.js';

const FINANCIAL_MARKET_ANALYSIS_SYMBOLS_REQUIRED = 'FINANCIAL_MARKET_ANALYSIS_SYMBOLS_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'financial-market-analysis',
  plane: 'hands',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    required: ['action', 'symbols'],
    properties: {
      action: { type: 'string', enum: ['sentiment', 'volatility', 'correlation'] },
      symbols: { type: 'array' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action'],
    properties: {
      provider: { type: 'string', enum: ['financial-market-analysis'] },
      action: { type: 'string', enum: ['sentiment', 'volatility', 'correlation'] },
      metrics: { type: 'array' },
      correlation: { type: 'object' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'financial-market-analysis',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') ||
            FINANCIAL_MARKET_ANALYSIS_SYMBOLS_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: {
          retries: 0,
          circuit_breaker_state: 'CLOSED',
          cache_hit: false
        }
      };
    }

    try {
      const data = await runClient(parsed.data as FinancialMarketAnalysisInput);
      const output = OutputSchema.parse(data) as FinancialMarketAnalysisOutput;
      return {
        skill_id: 'financial-market-analysis',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: {
          retries: 0,
          circuit_breaker_state: 'CLOSED',
          cache_hit: false
        }
      };
    } catch (err) {
      return {
        skill_id: 'financial-market-analysis',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'financial-market-analysis execution failed',
          retryable: true,
          http_status: 502
        },
        latency_ms: Date.now() - started,
        metadata: {
          retries: 0,
          circuit_breaker_state: 'CLOSED',
          cache_hit: false
        }
      };
    }
  },
  async healthCheck(): Promise<boolean> {
    return true;
  }
};

export default adapter;
