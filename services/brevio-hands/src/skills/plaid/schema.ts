import { z } from 'zod';

export const InputSchema = z
  .object({
    action: z.enum(['accounts', 'transactions', 'balance']),
    account_id: z.string().min(3).max(120).optional()
  })
  .strict();

const AccountSchema = z.object({
  account_id: z.string(),
  name: z.string(),
  mask: z.string(),
  subtype: z.string()
});

const TransactionSchema = z.object({
  transaction_id: z.string(),
  account_id: z.string(),
  name: z.string(),
  amount: z.number(),
  date: z.string().date()
});

const BalanceSchema = z.object({
  account_id: z.string(),
  available: z.number(),
  current: z.number()
});

export const OutputSchema = z
  .object({
    provider: z.literal('plaid'),
    action: z.enum(['accounts', 'transactions', 'balance']),
    accounts: z.array(AccountSchema).optional(),
    transactions: z.array(TransactionSchema).optional(),
    balances: z.array(BalanceSchema).optional()
  })
  .strict();
