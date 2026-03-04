import { z } from 'zod';

const ActionSchema = z.enum(['accounts', 'transactions', 'budgets']);

const AccountSchema = z.object({
  account_id: z.string(),
  name: z.string(),
  balance_cents: z.number().int()
});

const TransactionSchema = z.object({
  transaction_id: z.string(),
  account_id: z.string(),
  merchant: z.string(),
  amount_cents: z.number().int(),
  category: z.string(),
  posted_at: z.string().datetime()
});

const BudgetSchema = z.object({
  category: z.string(),
  budget_cents: z.number().int().nonnegative(),
  spent_cents: z.number().int()
});

export const InputSchema = z
  .object({
    action: ActionSchema,
    account_id: z.string().min(2).max(100).optional(),
    month: z.string().regex(/^\d{4}-\d{2}$/u).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'transactions' && !value.account_id) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'MONARCH_MONEY_ACCOUNT_REQUIRED'
      });
    }

    if (value.action === 'budgets' && !value.month) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'MONARCH_MONEY_MONTH_REQUIRED'
      });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('monarch-money'),
    action: ActionSchema,
    accounts: z.array(AccountSchema).optional(),
    transactions: z.array(TransactionSchema).optional(),
    budgets: z.array(BudgetSchema).optional()
  })
  .strict();
