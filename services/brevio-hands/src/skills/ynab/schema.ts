import { z } from 'zod';

export const InputSchema = z
  .object({
    action: z.enum(['summary', 'accounts', 'transactions']),
    budget_id: z.string().min(3).max(120).optional(),
    account_id: z.string().min(3).max(120).optional()
  })
  .strict();

const AccountSchema = z.object({
  account_id: z.string(),
  name: z.string(),
  balance_cents: z.number().int()
});

const TransactionSchema = z.object({
  transaction_id: z.string(),
  account_id: z.string(),
  payee: z.string(),
  amount_cents: z.number().int(),
  date: z.string().date()
});

export const OutputSchema = z
  .object({
    provider: z.literal('ynab'),
    action: z.enum(['summary', 'accounts', 'transactions']),
    budget_id: z.string(),
    total_budget_cents: z.number().int().optional(),
    accounts: z.array(AccountSchema).optional(),
    transactions: z.array(TransactionSchema).optional()
  })
  .strict();
