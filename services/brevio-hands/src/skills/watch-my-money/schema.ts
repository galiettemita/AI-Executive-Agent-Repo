import { z } from 'zod';

const ActionSchema = z.enum(['analyze_statement', 'budget_alerts']);

const TransactionSchema = z.object({
  merchant: z.string().min(1).max(160),
  amount_cents: z.number().int().positive().max(100000000),
  category: z.string().min(2).max(80)
});

export const InputSchema = z
  .object({
    action: ActionSchema,
    monthly_income_cents: z.number().int().positive().max(1000000000).optional(),
    transactions: z.array(TransactionSchema).max(2000).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (!value.transactions?.length) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'WATCH_MY_MONEY_TRANSACTIONS_REQUIRED' });
    }
    if (!value.monthly_income_cents) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'WATCH_MY_MONEY_INCOME_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('watch-my-money'),
    action: ActionSchema,
    category_totals_cents: z.record(z.number().int().nonnegative()),
    spend_rate_pct_of_income: z.number().min(0),
    alerts: z.array(z.string().min(2).max(240)).max(20),
    summary: z.string().min(10).max(4096)
  })
  .strict();
