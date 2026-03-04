import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { ParcelPackageTrackingInput, ParcelPackageTrackingOutput } from './types.js';

const adapter: ISkillAdapter = {
  id: 'parcel-package-tracking',
  plane: 'hands',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    required: ['tracking_number'],
    properties: {
      tracking_number: { type: 'string', minLength: 8, maxLength: 40 },
      carrier: { type: 'string', enum: ['auto', 'ups', 'usps', 'fedex', 'dhl'] },
      locale: { type: 'string', minLength: 2, maxLength: 10 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'tracking_number', 'carrier', 'status', 'history'],
    properties: {
      provider: { type: 'string', enum: ['parcel'] },
      tracking_number: { type: 'string' },
      carrier: { type: 'string', enum: ['ups', 'usps', 'fedex', 'dhl'] },
      status: { type: 'string', enum: ['label_created', 'in_transit', 'out_for_delivery', 'delivered'] },
      eta: { type: 'string', format: 'date-time' },
      history: { type: 'array' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'parcel-package-tracking',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message: parsed.error.issues.map((issue) => issue.message).join('; '),
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
      const data = await runClient(parsed.data as ParcelPackageTrackingInput);
      const output = OutputSchema.parse(data) as ParcelPackageTrackingOutput;
      return {
        skill_id: 'parcel-package-tracking',
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
        skill_id: 'parcel-package-tracking',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'parcel-package-tracking execution failed',
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
