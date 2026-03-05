import { z } from 'zod';

export const InputSchema = z.object({
  entity_id: z.string().min(3).max(200),
  action: z.string().min(2).max(100),
  value: z.union([z.string(), z.number(), z.boolean()]).optional(),
  two_factor_code: z.string().min(4).max(12).optional()
});

export const OutputSchema = z.object({
  state: z.string(),
  attributes: z.record(z.union([z.string(), z.number(), z.boolean()]))
});
