import type { JSONSchema7 } from 'json-schema';
import { z } from 'zod';

export const SkillExecutionStatusValues = [
  'SUCCESS',
  'PARTIAL',
  'FAILED',
  'TIMEOUT',
  'NEEDS_CONSENT',
  'NOT_EXECUTED',
  'SIMULATED'
] as const;

export const SkillErrorCodeValues = [
  'SKILL_NOT_FOUND',
  'SKILL_DISABLED',
  'AUTH_EXPIRED',
  'AUTH_REVOKED',
  'RATE_LIMITED',
  'EXTERNAL_TIMEOUT',
  'EXTERNAL_ERROR',
  'VALIDATION_FAILED',
  'CIRCUIT_OPEN',
  'LLM_HALLUCINATION',
  'BUDGET_EXCEEDED',
  'TASK_GRAPH_INVALID',
  'IDEMPOTENCY_CONFLICT',
  'CONSENT_REQUIRED',
  'HUMAN_REVIEW_REQUIRED',
  'RECIPIENT_VERIFICATION_REQUIRED',
  'POLICY_REQUIRED',
  'UNSUPPORTED_OPERATION',
  'QUEUE_DECRYPT_FAILED',
  'STALE_DISPATCH',
  'RESULT_PROVENANCE_MISMATCH',
  'PAYLOAD_TOO_LARGE',
  'LEGACY_TOOL_CALL_DEPRECATED'
] as const;

export const SkillResultSchema = z.object({
  request_id: z.string().optional(),
  run_id: z.string().optional(),
  task_id: z.string().optional(),
  step_id: z.string().optional(),
  attempt: z.number().int().positive().optional(),
  skill_id: z.string(),
  status: z.enum(SkillExecutionStatusValues),
  data: z.unknown().optional(),
  error: z
    .object({
      code: z.enum(SkillErrorCodeValues),
      message: z.string(),
      retryable: z.boolean(),
      http_status: z.number()
    })
    .optional(),
  execution_receipt: z
    .object({
      executor: z.string().min(1),
      mode: z.enum(['direct', 'delegated', 'local', 'simulated']),
      issued_at: z.string().datetime(),
      receipt_id: z.string().min(1)
    })
    .optional(),
  latency_ms: z.number().int(),
  tokens_used: z.number().int().optional(),
  cost_cents: z.number().optional(),
  metadata: z.object({
    retries: z.number().int(),
    circuit_breaker_state: z.enum(['CLOSED', 'HALF_OPEN', 'OPEN']),
    cache_hit: z.boolean()
  })
});

export const SkillResultJsonSchema: JSONSchema7 = {
  $schema: 'http://json-schema.org/draft-07/schema#',
  $id: 'https://schemas.brevio.app/skill-result.v1.json',
  type: 'object',
  additionalProperties: false,
  required: ['skill_id', 'status', 'latency_ms', 'metadata'],
  properties: {
    request_id: { type: 'string' },
    run_id: { type: 'string' },
    task_id: { type: 'string' },
    step_id: { type: 'string' },
    attempt: { type: 'integer', minimum: 1 },
    skill_id: { type: 'string' },
    status: { type: 'string', enum: [...SkillExecutionStatusValues] },
    data: {},
    error: {
      type: 'object',
      additionalProperties: false,
      required: ['code', 'message', 'retryable', 'http_status'],
      properties: {
        code: {
          type: 'string',
          enum: [...SkillErrorCodeValues]
        },
        message: { type: 'string' },
        retryable: { type: 'boolean' },
        http_status: { type: 'number' }
      }
    },
    execution_receipt: {
      type: 'object',
      additionalProperties: false,
      required: ['executor', 'mode', 'issued_at', 'receipt_id'],
      properties: {
        executor: { type: 'string', minLength: 1 },
        mode: { type: 'string', enum: ['direct', 'delegated', 'local', 'simulated'] },
        issued_at: { type: 'string', format: 'date-time' },
        receipt_id: { type: 'string', minLength: 1 }
      }
    },
    latency_ms: { type: 'integer' },
    tokens_used: { type: 'integer' },
    cost_cents: { type: 'number' },
    metadata: {
      type: 'object',
      additionalProperties: false,
      required: ['retries', 'circuit_breaker_state', 'cache_hit'],
      properties: {
        retries: { type: 'integer' },
        circuit_breaker_state: { type: 'string', enum: ['CLOSED', 'HALF_OPEN', 'OPEN'] },
        cache_hit: { type: 'boolean' }
      }
    }
  }
};

export type SkillResult = z.infer<typeof SkillResultSchema>;
