import { z } from 'zod';

const ActionSchema = z.enum(['fetch_accounts', 'analyze_transactions']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    account_id: z.string().min(2).max(120).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'analyze_transactions' && !value.account_id) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'GEORGE_ACCOUNT_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('george'),
    action: ActionSchema,
    accounts: z.array(
      z
        .object({
          account_id: z.string().min(2).max(120),
          balance_eur: z.number(),
          currency: z.literal('EUR')
        })
        .strict()
    ),
    summary: z.string().min(10).max(4096)
  })
  .strict();
