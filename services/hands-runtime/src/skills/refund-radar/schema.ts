import { z } from 'zod';

const ActionSchema = z.enum(['scan_recurring_charges', 'draft_refund_request']);

const FlaggedChargeSchema = z.object({
  merchant: z.string().min(1).max(160),
  amount_cents: z.number().int().positive(),
  frequency: z.enum(['weekly', 'monthly', 'annual']),
  confidence: z.number().min(0).max(1)
});

export const InputSchema = z
  .object({
    action: ActionSchema,
    merchant: z.string().min(1).max(160).optional(),
    amount_cents: z.number().int().positive().max(100000000).optional(),
    reason: z.string().min(2).max(400).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'draft_refund_request' && (!value.merchant || !value.amount_cents)) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'REFUND_RADAR_DRAFT_FIELDS_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('refund-radar'),
    action: ActionSchema,
    flagged_charges: z.array(FlaggedChargeSchema).max(100),
    draft_message: z.string().min(10).max(4096).optional(),
    summary: z.string().min(10).max(4096)
  })
  .strict();
