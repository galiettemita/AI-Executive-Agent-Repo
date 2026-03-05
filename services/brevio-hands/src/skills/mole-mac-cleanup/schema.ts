import { z } from 'zod';

const ActionSchema = z.enum(['scan_cleanup', 'run_cleanup']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    mode: z.enum(['quick', 'deep']).optional(),
    confirmed: z.boolean().optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'run_cleanup' && value.confirmed !== true) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'MOLE_MAC_CLEANUP_CONFIRMATION_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('mole-mac-cleanup'),
    action: ActionSchema,
    reclaimable_mb: z.number().int().min(0),
    cleaned_mb: z.number().int().min(0).optional(),
    categories: z.array(z.string().min(2).max(120)).min(1).max(20),
    summary: z.string().min(10).max(4096)
  })
  .strict();
