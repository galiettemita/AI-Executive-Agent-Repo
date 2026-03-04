import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { BillPayP2PInput, BillPayP2POutput } from './types.js';

const BILL_PAY_CONFIRMATION_REQUIRED = 'BILL_PAY_CONFIRMATION_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'bill-pay-p2p',
  plane: 'hands',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['list_payees', 'create_payment', 'payment_status', 'cancel_payment'] },
      payee_id: { type: 'string', minLength: 2, maxLength: 120 },
      amount_cents: { type: 'integer', minimum: 1, maximum: 500000000 },
      memo: { type: 'string', minLength: 1, maxLength: 280 },
      payment_id: { type: 'string', minLength: 2, maxLength: 120 },
      confirmed: { type: 'boolean' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'partnership_status'],
    properties: {
      provider: { type: 'string', enum: ['bill-pay-p2p'] },
      action: { type: 'string', enum: ['list_payees', 'create_payment', 'payment_status', 'cancel_payment'] },
      payees: { type: 'array' },
      payment_id: { type: 'string' },
      status: { type: 'string', enum: ['pending', 'scheduled', 'completed', 'cancelled'] },
      partnership_status: { type: 'string', enum: ['awaiting_api_partnership'] }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'bill-pay-p2p',
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

    const request = parsed.data as BillPayP2PInput;

    if (request.action === 'create_payment' && request.confirmed !== true) {
      return {
        skill_id: 'bill-pay-p2p',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message: BILL_PAY_CONFIRMATION_REQUIRED,
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
      const output = OutputSchema.parse(data) as BillPayP2POutput;
      return {
        skill_id: 'bill-pay-p2p',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'bill-pay-p2p',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'bill-pay-p2p execution failed',
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
