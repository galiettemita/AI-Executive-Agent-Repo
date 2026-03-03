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

export type SkillResult = z.infer<typeof SkillResultSchema>;
