import type { JSONSchema7 } from 'json-schema';
import { z } from 'zod';

export const SkillResultSchema = z.object({
  skill_id: z.string(),
  status: z.enum(['SUCCESS', 'PARTIAL', 'FAILED', 'TIMEOUT']),
  data: z.unknown().optional(),
  error: z
    .object({
      code: z.enum([
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
        'IDEMPOTENCY_CONFLICT'
      ]),
      message: z.string(),
      retryable: z.boolean(),
      http_status: z.number()
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
    skill_id: { type: 'string' },
    status: { type: 'string', enum: ['SUCCESS', 'PARTIAL', 'FAILED', 'TIMEOUT'] },
    data: {},
    error: {
      type: 'object',
      additionalProperties: false,
      required: ['code', 'message', 'retryable', 'http_status'],
      properties: {
        code: {
          type: 'string',
          enum: [
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
            'IDEMPOTENCY_CONFLICT'
          ]
        },
        message: { type: 'string' },
        retryable: { type: 'boolean' },
        http_status: { type: 'number' }
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
