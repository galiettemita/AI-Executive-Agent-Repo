import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { PharmacyPrescriptionInput, PharmacyPrescriptionOutput } from './types.js';

const PHARMACY_REFILL_CONFIRMATION_REQUIRED = 'PHARMACY_REFILL_CONFIRMATION_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'pharmacy-prescription',
  plane: 'hands',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['medication_lookup', 'refill_request', 'refill_status'] },
      medication_name: { type: 'string', minLength: 2, maxLength: 120 },
      prescription_id: { type: 'string', minLength: 2, maxLength: 120 },
      pharmacy_name: { type: 'string', minLength: 2, maxLength: 120 },
      confirmed: { type: 'boolean' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'partnership_status'],
    properties: {
      provider: { type: 'string', enum: ['pharmacy-prescription'] },
      action: { type: 'string', enum: ['medication_lookup', 'refill_request', 'refill_status'] },
      medications: { type: 'array' },
      prescription_id: { type: 'string' },
      status: { type: 'string', enum: ['pending', 'processing', 'ready', 'cancelled'] },
      partnership_status: { type: 'string', enum: ['awaiting_api_partnership'] }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'pharmacy-prescription',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message: parsed.error.issues.map((issue) => issue.message).join('; '),
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    const request = parsed.data as PharmacyPrescriptionInput;

    if (request.action === 'refill_request' && request.confirmed !== true) {
      return {
        skill_id: 'pharmacy-prescription',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message: PHARMACY_REFILL_CONFIRMATION_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      // CUSTOM_BUILD_REQUIRED: Awaiting API partnership
      const data = await runClient(request);
      const output = OutputSchema.parse(data) as PharmacyPrescriptionOutput;
      return {
        skill_id: 'pharmacy-prescription',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'pharmacy-prescription',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'pharmacy-prescription execution failed',
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
