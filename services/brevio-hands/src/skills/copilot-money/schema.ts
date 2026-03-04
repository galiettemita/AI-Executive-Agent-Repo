import { z } from 'zod';

const ActionSchema = z.enum(['accounts', 'transactions', 'net_worth']);

const AccountSchema = z.object({
  account_id: z.string(),
  name: z.string(),
  balance_cents: z.number().int(),
  account_type: z.enum(['checking', 'savings', 'credit'])
});

const TransactionSchema = z.object({
  transaction_id: z.string(),
  account_id: z.string(),
  merchant: z.string(),
  amount_cents: z.number().int(),
  category: z.string(),
  posted_at: z.string().datetime()
});

export const InputSchema = z
  .object({
    action: ActionSchema,
    account_id: z.string().min(2).max(100).optional(),
    from_date: z.string().regex(/^\d{4}-\d{2}-\d{2}$/u).optional(),
    to_date: z.string().regex(/^\d{4}-\d{2}-\d{2}$/u).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'transactions' && !value.account_id) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'COPILOT_MONEY_ACCOUNT_REQUIRED'
      });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('copilot-money'),
    action: ActionSchema,
    accounts: z.array(AccountSchema).optional(),
    transactions: z.array(TransactionSchema).optional(),
    net_worth_cents: z.number().int().optional()
  })
  .strict();
