import { z } from 'zod';

const ActionSchema = z.enum(['log_expense', 'daily_briefing', 'budget_status']);

const EntrySchema = z.object({
  entry_id: z.string().min(2).max(120),
  merchant: z.string().min(1).max(160),
  amount_cents: z.number().int().positive(),
  category: z.string().min(2).max(80),
  occurred_on: z.string().regex(/^\d{4}-\d{2}-\d{2}$/)
});

export const InputSchema = z
  .object({
    action: ActionSchema,
    merchant: z.string().min(1).max(160).optional(),
    amount_cents: z.number().int().positive().max(100000000).optional(),
    category: z.string().min(2).max(80).optional(),
    occurred_on: z.string().regex(/^\d{4}-\d{2}-\d{2}$/).optional(),
    note: z.string().min(1).max(400).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (
      value.action === 'log_expense' &&
      (!value.merchant || !value.amount_cents || !value.category || !value.occurred_on)
    ) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'SMART_EXPENSE_TRACKER_LOG_FIELDS_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('smart-expense-tracker'),
    action: ActionSchema,
    entries: z.array(EntrySchema).max(500),
    today_spend_cents: z.number().int().nonnegative(),
    month_spend_cents: z.number().int().nonnegative(),
    budget_alerts: z.array(z.string().min(2).max(240)).max(20),
    summary: z.string().min(10).max(4096)
  })
  .strict();
