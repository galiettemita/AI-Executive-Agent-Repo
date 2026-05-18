import { z } from 'zod';

const ActionSchema = z.enum(['scan_subscriptions', 'draft_cancellation']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    transactions_csv: z.string().min(10).max(200000).optional(),
    merchant_name: z.string().min(2).max(200).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'scan_subscriptions' && !value.transactions_csv) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'JUST_FUCKING_CANCEL_INPUT_REQUIRED' });
    }
    if (value.action === 'draft_cancellation' && !value.merchant_name) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'JUST_FUCKING_CANCEL_MERCHANT_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('just-fucking-cancel'),
    action: ActionSchema,
    findings: z.array(
      z
        .object({
          merchant: z.string().min(2).max(200),
          amount_usd: z.number().min(0),
          cadence: z.string().min(2).max(80)
        })
        .strict()
    ),
    draft_message: z.string().min(5).max(2000).optional(),
    summary: z.string().min(10).max(4096)
  })
  .strict();
