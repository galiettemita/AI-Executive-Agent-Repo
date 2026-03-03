import { z } from 'zod';

export const InputSchema = z.object({
  payload: z.record(z.unknown()).optional()
});

export const OutputSchema = z.object({
  ok: z.boolean(),
  skill_id: z.string()
});
